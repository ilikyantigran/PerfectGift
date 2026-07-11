package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ilikyantigran/PerfectGift/services/backend/log-server/internal/store"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "logs.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	api := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Minimal UI stub for the "/" route.
	ui := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<h1>ui</h1>"))
	})
	return api.Routes(ui)
}

func TestIngestLeniencyAndLogsContract(t *testing.T) {
	srv := httptest.NewServer(newServer(t))
	defer srv.Close()

	// Missing level (=> INFO) and missing ts (=> now); fields omitted.
	body := `{"records":[
		{"service":"identity","message":"hello"},
		{"service":"poll","message":"world","level":"ERROR","ts":"2026-07-10T18:34:53.123456Z","fields":{"k":1}}
	]}`
	resp, err := http.Post(srv.URL+"/api/ingest", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	var acc struct {
		Accepted int `json:"accepted"`
	}
	json.NewDecoder(resp.Body).Decode(&acc)
	resp.Body.Close()
	if acc.Accepted != 2 {
		t.Fatalf("accepted want 2, got %d", acc.Accepted)
	}

	// Query back.
	resp, _ = http.Get(srv.URL + "/api/logs")
	var got struct {
		Logs []store.LogRow `json:"logs"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got.Logs) != 2 {
		t.Fatalf("logs want 2, got %d", len(got.Logs))
	}
	// Newest-first + monotonic ids.
	if got.Logs[0].ID != 2 || got.Logs[1].ID != 1 {
		t.Fatalf("ids/order wrong: %d,%d", got.Logs[0].ID, got.Logs[1].ID)
	}
	// Leniency defaults applied to row id 1.
	head := got.Logs[1]
	if head.Level != "INFO" {
		t.Fatalf("default level want INFO, got %q", head.Level)
	}
	if head.TS == "" {
		t.Fatalf("default ts not set")
	}
	if string(head.Fields) != "{}" {
		t.Fatalf("default fields want {}, got %s", head.Fields)
	}
}

func TestServicesEndpoint(t *testing.T) {
	srv := httptest.NewServer(newServer(t))
	defer srv.Close()

	http.Post(srv.URL+"/api/ingest", "application/json", strings.NewReader(
		`{"records":[{"service":"identity","message":"a"},{"service":"poll","message":"b"}]}`))

	resp, _ := http.Get(srv.URL + "/api/services")
	var got struct {
		Services []string `json:"services"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got.Services) != 2 || got.Services[0] != "identity" || got.Services[1] != "poll" {
		t.Fatalf("services wrong: %v", got.Services)
	}
}

func TestSPAFallback(t *testing.T) {
	srv := httptest.NewServer(newServer(t))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/some/spa/route")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "ui") {
		t.Fatalf("SPA fallback did not serve ui, got %q", b)
	}
}
