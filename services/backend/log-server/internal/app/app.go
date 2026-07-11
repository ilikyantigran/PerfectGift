// Package app wires config, store, retention pruner, and the HTTP server
// together and runs them until the context is cancelled.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/httpapi"
	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/infra/config"
	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/infra/metrics"
	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/store"
	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/web"
)

// App holds the wired-up runtime dependencies.
type App struct {
	cfg   *config.Config
	log   *slog.Logger
	store *store.Store
	srv   *http.Server
}

// NewApp loads config and opens the store.
func NewApp(configPath string) (*App, error) {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.InitConfig(configPath)
	if err != nil {
		return nil, err
	}

	st, err := store.Open(cfg.Store.Path)
	if err != nil {
		return nil, err
	}

	ui, err := web.Handler()
	if err != nil {
		st.Close()
		return nil, err
	}

	api := httpapi.New(st, log)
	mux := api.Routes(ui)
	mux.Handle("GET /metrics", promhttp.Handler())

	return &App{
		cfg:   cfg,
		log:   log,
		store: st,
		srv:   &http.Server{Addr: ":" + cfg.Service.HTTPPort, Handler: mux},
	}, nil
}

// Run starts the pruner and HTTP server, and shuts down gracefully on ctx done.
func (a *App) Run(ctx context.Context) error {
	defer a.store.Close()

	go a.runPruner(ctx)

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("log-server listening", "addr", a.srv.Addr, "db", a.cfg.Store.Path)
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

// runPruner periodically deletes rows older than the retention window.
func (a *App) runPruner(ctx context.Context) {
	prune := func() {
		cutoff := time.Now().Add(-a.cfg.Retention.Window)
		n, err := a.store.Prune(ctx, cutoff)
		if err != nil {
			a.log.Error("prune", "err", err)
			return
		}
		if n > 0 {
			metrics.PrunedRecords.Add(float64(n))
			a.log.Info("pruned old logs", "removed", n, "cutoff", cutoff.UTC().Format(time.RFC3339))
		}
	}

	prune() // sweep once at startup
	t := time.NewTicker(a.cfg.Retention.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}
