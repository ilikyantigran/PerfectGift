package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/postgres"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/domain/valkey"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/embedding"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/catalog/internal/infra/telemetry"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/catalog/pkg/api/catalog/v1"

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

// Run assembles the service (telemetry → clients → stores → server → transport),
// serves gRPC + the HTTP gateway (REST + /metrics + /swagger/), and blocks until
// ctx is cancelled, then shuts everything down gracefully.
func (a *App) Run(ctx context.Context) error {
	// 1. Telemetry first: gRPC client/server handlers capture the global provider.
	tel, err := telemetry.Setup(ctx, "catalog")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// 2. Embedder. Empty endpoint → deterministic fake (service boots without an
	//    external embedding API). The API key is read from the env var named in config.
	apiKey := ""
	if a.config.Embedding.APIKeyEnv != "" {
		apiKey = os.Getenv(a.config.Embedding.APIKeyEnv)
	}
	embedder, err := embedding.New(a.config.Embedding.Model, a.config.Embedding.Dimension, a.config.Embedding.Endpoint, apiKey)
	if err != nil {
		return fmt.Errorf("embedding: %w", err)
	}
	slog.Info("embedder ready", "model", embedder.Model(), "dimension", embedder.Dimension(), "live", a.config.Embedding.Endpoint != "")

	// 3. Backing stores: Postgres (reference + corpus + pgvector) and Valkey (cache).
	pg, err := postgres.NewStore(ctx, a.config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pg.Close()

	cache, err := valkey.NewStore(a.config.Valkey.Address)
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer cache.Close()

	// 4. Construct the RPC implementation.
	srv := NewServer(pg, pg, cache, embedder, Tuning{
		ReferenceCacheTTL: time.Duration(a.config.Catalog.ReferenceCacheTTLSeconds) * time.Second,
		DefaultTopK:       a.config.Catalog.DefaultTopK,
		MaxTopK:           a.config.Catalog.MaxTopK,
	})

	// 5. gRPC server.
	grpcAddr := fmt.Sprintf(":%s", a.config.Service.GrpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listener: %w", err)
	}
	a.grpcServer = grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	reflection.Register(a.grpcServer)
	catalogv1.RegisterCatalogServiceServer(a.grpcServer, srv)

	// 6. HTTP edge: REST gateway + metrics + swagger.
	httpHandler, err := a.httpHandler(ctx, tel)
	if err != nil {
		return err
	}
	a.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%s", a.config.Service.HttpPort),
		Handler: httpHandler,
	}

	// 7. Serve both; first error or ctx cancellation wins.
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
	if err := catalogv1.RegisterCatalogServiceHandlerFromEndpoint(ctx, gwMux, dialAddr, opts); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", gwMux)
	return mux, nil
}

func (a *App) Config() config.Config { return *a.config }
