package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/domain/postgres"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/infra/telemetry"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"
	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/push"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/notification/pkg/api/notification/v1"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// App is the notification service's self-contained runtime. main builds it and
// calls Run; this is the single place that assembles and shuts down the service:
// the gRPC/HTTP device-registration edge, the two NATS event consumers, and the
// outbox dispatcher.
type App struct {
	config     *config.Config
	grpcServer *grpc.Server
	httpServer *http.Server
}

// NewApp loads config only — cheap, no connections to dependencies yet.
func NewApp(path string) (*App, error) {
	cfg, err := config.InitConfig(path)
	if err != nil {
		return nil, err
	}
	return &App{config: cfg}, nil
}

// Run assembles the service and serves until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	// 1. Telemetry.
	tel, err := telemetry.Setup(ctx, "notification")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// 2. Backing store (devices + outbox).
	store, err := postgres.New(ctx, a.config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer store.Close()

	// 3. Push providers → platform router.
	router, err := a.buildRouter()
	if err != nil {
		return fmt.Errorf("push providers: %w", err)
	}

	// 4. gRPC device-registration server.
	srv := NewServer(store, time.Now)
	grpcAddr := fmt.Sprintf(":%s", a.config.Service.GrpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listener: %w", err)
	}
	a.grpcServer = grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	reflection.Register(a.grpcServer)
	notificationv1.RegisterNotificationServiceServer(a.grpcServer, srv)

	// 5. HTTP edge (REST gateway + /metrics + /swagger/).
	httpHandler, err := a.httpHandler(ctx, tel)
	if err != nil {
		return err
	}
	a.httpServer = &http.Server{Addr: fmt.Sprintf(":%s", a.config.Service.HttpPort), Handler: httpHandler}

	// 6. Bus consumers + dispatcher.
	bus, err := events.Connect(ctx, a.config.NATS.URL, a.config.NATS.Stream,
		[]string{a.config.NATS.PollCompletedSub, a.config.NATS.IdeasReadySub})
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer bus.Close()

	consumer := notify.NewConsumer(store, time.Now)
	dispatcher := notify.NewDispatcher(store, router, notify.DispatcherConfig{
		Interval:    a.config.Dispatcher.Interval,
		Lease:       a.config.Dispatcher.Lease,
		Batch:       a.config.Dispatcher.Batch,
		MaxAttempts: a.config.Dispatcher.MaxAttempts,
		BaseBackoff: a.config.Dispatcher.BaseBackoff,
	}, time.Now)

	pollSub := bus.Subscription(a.config.NATS.Durable+"-poll-completed", a.config.NATS.PollCompletedSub)
	ideasSub := bus.Subscription(a.config.NATS.Durable+"-ideas-ready", a.config.NATS.IdeasReadySub)

	// 7. Run everything; the first hard error or ctx cancellation stops all.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		slog.Info("gRPC listening", "addr", grpcAddr)
		return a.grpcServer.Serve(lis)
	})
	g.Go(func() error {
		slog.Info("HTTP listening (REST gateway, /swagger/, /metrics)", "addr", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		slog.Info("dispatcher started")
		return dispatcher.Run(gctx)
	})
	g.Go(func() error {
		slog.Info("consuming PollCompleted", "subject", a.config.NATS.PollCompletedSub)
		return notify.Run(gctx, pollSub, consumer.HandlePollCompleted)
	})
	g.Go(func() error {
		slog.Info("consuming IdeasReady", "subject", a.config.NATS.IdeasReadySub)
		return notify.Run(gctx, ideasSub, consumer.HandleIdeasReady)
	})

	// Shutdown watcher: on ctx.Done, gracefully stop the transports so the
	// errgroup goroutines return.
	g.Go(func() error {
		<-gctx.Done()
		slog.Info("shutdown signal received, stopping servers")
		a.grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// buildRouter constructs the platform→Pusher router from config. Providers are
// opt-in via config; a disabled provider simply isn't registered (pushes to that
// platform then fail loudly rather than silently).
func (a *App) buildRouter() (*notify.Router, error) {
	m := map[notify.Platform]notify.Pusher{}
	if a.config.APNs.Enabled {
		p, err := push.NewAPNs(push.APNsConfig{
			KeyPath: a.config.APNs.KeyPath,
			KeyID:   a.config.APNs.KeyID,
			TeamID:  a.config.APNs.TeamID,
			Topic:   a.config.APNs.Topic,
			Sandbox: a.config.APNs.Sandbox,
		})
		if err != nil {
			return nil, err
		}
		m[notify.PlatformIOS] = p
	}
	if a.config.FCM.Enabled {
		p, err := push.NewFCM(push.FCMConfig{
			CredentialsPath: a.config.FCM.CredentialsPath,
			ProjectID:       a.config.FCM.ProjectID,
		})
		if err != nil {
			return nil, err
		}
		m[notify.PlatformAndroid] = p
	}
	return notify.NewRouter(m), nil
}

// httpHandler builds the REST gateway (dialing this service's own gRPC server)
// plus the /metrics and /swagger/ endpoints.
func (a *App) httpHandler(ctx context.Context, tel *telemetry.Provider) (http.Handler, error) {
	gwMux := runtime.NewServeMux()
	dialAddr := fmt.Sprintf("%s:%s", a.config.Service.Host, a.config.Service.GrpcPort)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	if err := notificationv1.RegisterNotificationServiceHandlerFromEndpoint(ctx, gwMux, dialAddr, opts); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", gwMux)
	return mux, nil
}

// Config returns a copy of the loaded config (handy for tests/tools).
func (a *App) Config() config.Config { return *a.config }
