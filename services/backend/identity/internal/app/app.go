package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/identity/pkg/api/identity/v1"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/domain/postgres"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/domain/valkey"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/infra/telemetry"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/oauth"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/token"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// App is the service's self-contained runtime. main builds it and calls Run;
// this is the single place that knows how to assemble and shut down the service.
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

// Run assembles the service (telemetry → stores → token manager → server),
// serves gRPC + the HTTP gateway (REST + /metrics + /swagger/), and blocks until
// ctx is cancelled, then shuts everything down gracefully.
func (a *App) Run(ctx context.Context) error {
	tel, err := telemetry.Setup(ctx, "identity")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// Postgres (users, credentials, oauth links) — connect and self-migrate.
	pg, err := postgres.New(ctx, a.config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pg.Close()
	if err := postgres.MigratePool(ctx, pg); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Valkey (sessions + rate limiting).
	rlWindow := time.Duration(a.config.RateLimit.WindowSeconds) * time.Second
	vk, err := valkey.NewStore(a.config.Valkey.Address, a.config.RateLimit.MaxAttempts, rlWindow)
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer vk.Close()
	if err := vk.Ping(ctx); err != nil {
		return err
	}

	// Token manager (Ed25519 signing keys + JWKS).
	tm, err := token.NewManager(
		a.config.Token.Issuer,
		a.config.Token.Audience,
		time.Duration(a.config.Token.AccessTTLSeconds)*time.Second,
	)
	if err != nil {
		return fmt.Errorf("token manager: %w", err)
	}

	// Provider verifier (real Apple/Google JWKS verification).
	verifier := oauth.NewProviderVerifier(a.config.OAuth.GoogleClientIDs, a.config.OAuth.AppleClientIDs)

	srv := NewServer(Deps{
		Users:      pg,
		Sessions:   vk,
		Limiter:    vk,
		Verifier:   verifier,
		Tokens:     tm,
		RefreshTTL: time.Duration(a.config.Token.RefreshTTLSeconds) * time.Second,
	})

	grpcAddr := fmt.Sprintf(":%s", a.config.Service.GrpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listener: %w", err)
	}
	a.grpcServer = grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	reflection.Register(a.grpcServer)
	identityv1.RegisterIdentityServiceServer(a.grpcServer, srv)

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

// httpHandler builds the REST gateway (dialing this service's own gRPC server)
// plus the /metrics and /swagger/ endpoints.
func (a *App) httpHandler(ctx context.Context, tel *telemetry.Provider) (http.Handler, error) {
	gwMux := runtime.NewServeMux()
	dialAddr := fmt.Sprintf("%s:%s", a.config.Service.Host, a.config.Service.GrpcPort)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	if err := identityv1.RegisterIdentityServiceHandlerFromEndpoint(ctx, gwMux, dialAddr, opts); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", gwMux)
	return mux, nil
}

func (a *App) Config() config.Config { return *a.config }
