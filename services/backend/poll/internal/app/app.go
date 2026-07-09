package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/postgres"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/valkey"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/auth"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/infra/telemetry"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/poll/pkg/api/poll/v1"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// App is the Poll service's self-contained runtime. main builds it and calls Run;
// this is the single place that assembles and shuts down the service.
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

// Run assembles the service (telemetry → stores → server → transport), serves
// gRPC + the HTTP gateway (REST + /metrics + /swagger/), and blocks until ctx is
// cancelled, then shuts everything down gracefully.
func (a *App) Run(ctx context.Context) error {
	tel, err := telemetry.Setup(ctx, "poll")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// Backing stores (the service owns its Postgres schema + Valkey; NATS producer).
	pg, err := postgres.NewStore(ctx, a.config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pg.Close()
	if err := pg.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	rl, err := valkey.NewStore(a.config.Valkey.Address)
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer rl.Close()

	pub, err := events.NewPublisher(ctx, a.config.NATS.URL, a.config.NATS.Stream, a.config.NATS.Subject)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer pub.Close()

	srv := NewServer(pg, rl, pub, a.tuning())

	// gRPC server, with the auth interceptor resolving JWT subjects.
	authn := auth.New(a.config.Security.JWTSecret)
	grpcAddr := fmt.Sprintf(":%s", a.config.Service.GrpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listener: %w", err)
	}
	a.grpcServer = grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(authn.UnaryInterceptor()),
	)
	reflection.Register(a.grpcServer)
	pollv1.RegisterPollServiceServer(a.grpcServer, srv)

	httpHandler, err := a.httpHandler(ctx, tel)
	if err != nil {
		return err
	}
	a.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%s", a.config.Service.HttpPort),
		Handler: httpHandler,
	}

	errCh := make(chan error, 2)
	go func() {
		slog.Info("gRPC listening", "addr", grpcAddr)
		errCh <- a.grpcServer.Serve(lis)
	}()
	go func() {
		slog.Info("HTTP listening (REST gateway, /swagger/, /metrics)", "addr", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping servers")
		a.grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) tuning() Tuning {
	c := a.config
	return Tuning{
		DefaultTTL:     time.Duration(c.Tokens.DefaultTTLSeconds) * time.Second,
		PerTokenBudget: c.RateLimit.PerTokenBudget,
		PerTokenWindow: time.Duration(c.RateLimit.PerTokenWindow) * time.Second,
		PerIPBudget:    c.RateLimit.PerIPBudget,
		PerIPWindow:    time.Duration(c.RateLimit.PerIPWindow) * time.Second,
		AllowedOrigin:  c.Web.AllowedOrigin,
		LinkPath:       c.Web.LinkPath,
	}
}

// httpHandler builds the REST gateway (dialing this service's own gRPC server)
// plus /metrics and /swagger/. CORS is granted only to the configured Poll Web
// Page origin, and only for the public token routes.
func (a *App) httpHandler(ctx context.Context, tel *telemetry.Provider) (http.Handler, error) {
	gwMux := runtime.NewServeMux(
		// Forward the client IP and Authorization so the service can rate-limit and
		// resolve the owner subject.
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			switch strings.ToLower(key) {
			case "authorization", "x-forwarded-for", "x-real-ip", "user-agent":
				return key, true
			default:
				return runtime.DefaultHeaderMatcher(key)
			}
		}),
	)
	dialAddr := fmt.Sprintf("%s:%s", a.config.Service.Host, a.config.Service.GrpcPort)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	if err := pollv1.RegisterPollServiceHandlerFromEndpoint(ctx, gwMux, dialAddr, opts); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", a.cors(gwMux))
	return mux, nil
}

// cors grants the single configured web origin access to the public routes. This
// is the entire cross-origin surface — the anonymous token fetch/submit.
func (a *App) cors(next http.Handler) http.Handler {
	origin := a.config.Web.AllowedOrigin
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" && r.Header.Get("Origin") == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) Config() config.Config { return *a.config }
