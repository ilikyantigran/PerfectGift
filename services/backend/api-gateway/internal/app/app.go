// Package app is the gateway's self-contained runtime. main builds the App and calls
// Run; this is the single place that knows how to assemble and shut down the service.
//
// The gateway deviates from the standard house App in one deliberate way: it is
// HTTP-only. It exposes NO gRPC server — it is a gRPC *client* to the five domain
// services. So Run dials the downstreams, builds the REST edge (routing + JWT +
// rate limiting + CORS + error envelope), and serves a single HTTP port carrying the
// public API, /swagger/, and /metrics.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/auth"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/clients"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/infra/docs"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/infra/telemetry"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/ratelimit"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/transport/rest"
)

// App is the gateway runtime.
type App struct {
	config     *config.Config
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

// Run assembles the gateway (telemetry → JWKS verifier → rate limiters → downstream
// gRPC clients → REST edge → HTTP transport), serves the HTTP port, and blocks until
// ctx is cancelled, then shuts down gracefully.
func (a *App) Run(ctx context.Context) error {
	// 1. Telemetry first so gRPC/HTTP handlers capture the global providers.
	tel, err := telemetry.Setup(ctx, "api-gateway")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer tel.Shutdown(context.Background())

	// 2. Local JWT verifier, backed by Identity's JWKS endpoint (fetched + cached).
	verifier, err := auth.New(auth.Config{
		Issuer:     a.config.Auth.Issuer,
		Audience:   a.config.Auth.Audience,
		Source:     auth.NewHTTPSource(a.config.Auth.JWKSURL, nil),
		RefreshTTL: a.config.Auth.JWKSRefresh,
	})
	if err != nil {
		return fmt.Errorf("auth verifier: %w", err)
	}

	// 3. Downstream gRPC clients (lazy dials; do not require the services to be up).
	cl, err := clients.Dial(a.config)
	if err != nil {
		return fmt.Errorf("dial downstreams: %w", err)
	}
	defer cl.Close()

	// 4. REST edge.
	srv := rest.New(rest.Options{
		Identity:       cl.Identity,
		Poll:           cl.Poll,
		Surprise:       cl.Surprise,
		Catalog:        cl.Catalog,
		Notification:   cl.Notification,
		Verifier:       verifier,
		GlobalLimiter:  a.limiter(a.config.RateLimit.GlobalPerMin),
		PerIPLimiter:   a.limiter(a.config.RateLimit.PerIPPerMin),
		PerUserLimiter: a.limiter(a.config.RateLimit.PerUserPerMin),
		RefreshLimiter: a.limiter(a.config.RateLimit.RefreshPerMin),
		CORSOrigins:    a.config.CORS.PollOrigins,
	})

	// 5. HTTP mux: public API + Swagger + metrics, with the gateway as the OTel root span.
	mux := http.NewServeMux()
	mux.Handle("/metrics", tel.MetricsHandler())
	mux.Handle("/swagger/", http.StripPrefix("/swagger", docs.Handler()))
	mux.Handle("/", srv.Handler())

	a.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%s", a.config.Service.HttpPort),
		Handler:           otelhttp.NewHandler(mux, "gateway"),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 6. Serve; first error or ctx cancellation wins.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP listening (public API, /swagger/, /metrics)", "addr", a.httpServer.Addr)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping gateway")
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

// limiter builds a per-minute fixed-window limiter, or a no-op when the budget is 0.
func (a *App) limiter(perMinute int) ratelimit.Limiter {
	if perMinute <= 0 {
		return ratelimit.Noop{}
	}
	return ratelimit.NewWindow(perMinute, time.Minute)
}

func (a *App) Config() config.Config { return *a.config }
