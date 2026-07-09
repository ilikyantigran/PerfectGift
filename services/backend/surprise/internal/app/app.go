// Package app is the Surprise service's self-contained runtime. NewApp loads
// config; Run assembles telemetry, downstream clients, stores, the LLM client
// (wrapped in retry + circuit breaker), the gRPC service, the HTTP/Swagger edge,
// and the worker pool that consumes GenerationRequested — then serves until the
// context is cancelled.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/clients"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain/postgres"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain/valkey"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/infra/telemetry"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/llm"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/pipeline"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/resilience"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/surprise/v1"
)

// App is the service runtime. main builds it and calls Run.
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

// Config exposes the loaded config (used by main for logging).
func (a *App) Config() config.Config { return *a.config }

// Run assembles and serves the service, blocking until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	tel, err := telemetry.Setup(ctx, "surprise")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// Stores.
	pg, err := postgres.New(ctx, a.config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pg.Close()
	if err := pg.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	cache, err := valkey.New(a.config.Valkey.Address)
	if err != nil {
		return fmt.Errorf("valkey: %w", err)
	}
	defer cache.Close()

	// Events (NATS JetStream): producer + durable consumer.
	bus, err := events.NewNATSBus(events.NATSConfig{
		URL:            a.config.NATS.URL,
		Stream:         a.config.NATS.Stream,
		RequestSubject: a.config.NATS.RequestSubject,
		ReadySubject:   a.config.NATS.ReadySubject,
		DurableName:    a.config.NATS.DurableName,
	})
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer bus.Close()

	// Downstream clients (Poll, Catalog).
	pollCli, err := clients.DialPoll(a.config.Downstreams.Poll)
	if err != nil {
		return fmt.Errorf("dial poll: %w", err)
	}
	defer pollCli.Close()
	catalogCli, err := clients.DialCatalog(a.config.Downstreams.Catalog)
	if err != nil {
		return fmt.Errorf("dial catalog: %w", err)
	}
	defer catalogCli.Close()

	// LLM client wrapped in retry + circuit breaker.
	anthropic := llm.NewAnthropicClient(llm.AnthropicConfig{
		BaseURL:        a.config.Anthropic.BaseURL,
		APIKey:         a.config.Anthropic.APIKey,
		Version:        a.config.Anthropic.Version,
		SonnetModel:    a.config.Anthropic.SonnetModel,
		OpusModel:      a.config.Anthropic.OpusModel,
		HaikuModel:     a.config.Anthropic.HaikuModel,
		EmbeddingModel: a.config.Anthropic.EmbeddingModel,
		EmbeddingDim:   a.config.Anthropic.EmbeddingDim,
		MaxTokens:      a.config.Anthropic.MaxTokens,
		Timeout:        a.config.Resilience.LLMTimeout,
	})
	breaker := resilience.NewBreaker(a.config.Resilience.BreakerMaxFailures, a.config.Resilience.BreakerOpenDuration)
	resilientLLM := llm.NewResilient(anthropic, breaker, resilience.RetryConfig{
		MaxAttempts: a.config.Resilience.RetryMaxAttempts,
		BaseBackoff: a.config.Resilience.RetryBaseBackoff,
	})

	// Worker pipeline.
	pipe := pipeline.New(pg, cache, resilientLLM, pollCli, catalogCli, bus, pipeline.Config{
		IdeasWanted: a.config.Worker.IdeasWanted,
		StatusTTL:   a.config.Cache.StatusTTL,
		LLMCacheTTL: a.config.Cache.LLMTTL,
	})
	worker := pipeline.NewWorker(bus, pipe)
	if err := worker.Start(ctx); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	// gRPC service (request path).
	srv := NewServer(pg, cache, bus, ServerConfig{
		StatusTTL:      a.config.Cache.StatusTTL,
		IdempotencyTTL: a.config.Cache.IdempotencyTTL,
	})

	grpcAddr := fmt.Sprintf(":%s", a.config.Service.GrpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("grpc listener: %w", err)
	}
	a.grpcServer = grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	reflection.Register(a.grpcServer)
	surprisev1.RegisterSurpriseServiceServer(a.grpcServer, srv)

	httpHandler, err := a.httpHandler(ctx, tel)
	if err != nil {
		return err
	}
	a.httpServer = &http.Server{Addr: fmt.Sprintf(":%s", a.config.Service.HttpPort), Handler: httpHandler}

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

func (a *App) httpHandler(ctx context.Context, tel *telemetry.Provider) (http.Handler, error) {
	gwMux := runtime.NewServeMux()
	dialAddr := fmt.Sprintf("%s:%s", a.config.Service.Host, a.config.Service.GrpcPort)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
	if err := surprisev1.RegisterSurpriseServiceHandlerFromEndpoint(ctx, gwMux, dialAddr, opts); err != nil {
		return nil, fmt.Errorf("register gateway: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", gwMux)
	return mux, nil
}
