// Package httpapi wires the ingest/query REST endpoints plus the embedded UI.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/infra/metrics"
	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/store"
)

// API holds the dependencies for the HTTP handlers.
type API struct {
	store *store.Store
	log   *slog.Logger
}

// New builds an API bound to the given store.
func New(st *store.Store, log *slog.Logger) *API {
	return &API{store: st, log: log}
}

// Routes registers the API routes and the SPA handler onto a mux.
func (a *API) Routes(ui http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/ingest", a.handleIngest)
	mux.HandleFunc("GET /api/logs", a.handleLogs)
	mux.HandleFunc("GET /api/services", a.handleServices)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// SPA + static assets on everything else (non-/api paths).
	mux.Handle("/", ui)
	return mux
}

type ingestReq struct {
	Records []store.Record `json:"records"`
}

// handleIngest stores a batch of records. It is lenient: missing level
// defaults to INFO and missing ts defaults to now. This is the hot path.
func (a *API) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range req.Records {
		rec := &req.Records[i]
		if rec.TS == "" {
			rec.TS = now
		}
		if rec.Level == "" {
			rec.Level = "INFO"
		}
	}

	n, err := a.store.Insert(r.Context(), req.Records)
	if err != nil {
		a.log.Error("ingest insert", "err", err)
		http.Error(w, `{"error":"store"}`, http.StatusInternalServerError)
		return
	}
	metrics.IngestedRecords.Add(float64(n))
	writeJSON(w, http.StatusOK, map[string]int{"accepted": n})
}

type logsResp struct {
	Logs []store.LogRow `json:"logs"`
}

// handleLogs runs a filtered query and returns rows newest-first.
func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	q := store.Query{
		Service: v.Get("service"),
		Level:   v.Get("level"),
		Q:       v.Get("q"),
		From:    v.Get("from"),
		To:      v.Get("to"),
	}
	if s := v.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			q.Limit = n
		}
	}
	if s := v.Get("after"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			q.After = n
		}
	}

	rows, err := a.store.Query(r.Context(), q)
	if err != nil {
		a.log.Error("logs query", "err", err)
		http.Error(w, `{"error":"store"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, logsResp{Logs: rows})
}

type servicesResp struct {
	Services []string `json:"services"`
}

// handleServices returns the distinct service names for the UI dropdown.
func (a *API) handleServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := a.store.Services(r.Context())
	if err != nil {
		a.log.Error("services query", "err", err)
		http.Error(w, `{"error":"store"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, servicesResp{Services: svcs})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
