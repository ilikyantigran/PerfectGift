package logkit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	testTraceID = "0af7651916cd43dd8448eb211c80319c"
	testSpanID  = "b7ad6b7169203331"
)

// ingestServer is a hermetic stand-in for the log-server's /api/ingest
// endpoint. It captures every received Record and can be toggled unhealthy.
type ingestServer struct {
	*httptest.Server
	mu      sync.Mutex
	records []Record
	healthy bool
}

func newIngestServer(t *testing.T) *ingestServer {
	t.Helper()
	is := &ingestServer{healthy: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ingest", func(w http.ResponseWriter, r *http.Request) {
		is.mu.Lock()
		healthy := is.healthy
		is.mu.Unlock()
		if !healthy {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		var body ingestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		is.mu.Lock()
		is.records = append(is.records, body.Records...)
		n := len(body.Records)
		is.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": n})
	})
	is.Server = httptest.NewServer(mux)
	t.Cleanup(is.Close)
	return is
}

func (is *ingestServer) setHealthy(v bool) {
	is.mu.Lock()
	is.healthy = v
	is.mu.Unlock()
}

func (is *ingestServer) got() []Record {
	is.mu.Lock()
	defer is.mu.Unlock()
	out := make([]Record, len(is.records))
	copy(out, is.records)
	return out
}

// ctxWithSpan returns a context carrying a valid (non-recording) span context
// so trace_id/span_id propagate exactly as they would in a real service.
func ctxWithSpan(t *testing.T) context.Context {
	t.Helper()
	tid, err := trace.TraceIDFromHex(testTraceID)
	if err != nil {
		t.Fatal(err)
	}
	sid, err := trace.SpanIDFromHex(testSpanID)
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), sc)
}

// testOptions returns Options wired for fast, deterministic tests: large batch
// and long tickers so nothing ships until we call Flush explicitly.
func testOptions(t *testing.T, serverURL string, stdout *bytes.Buffer) Options {
	t.Helper()
	return Options{
		ServerURL:     serverURL,
		SpoolDir:      t.TempDir(),
		Level:         slog.LevelInfo,
		BatchSize:     1000,
		FlushInterval: time.Hour,
		RetryInterval: time.Hour,
		QueueSize:     1024,
		HTTPTimeout:   2 * time.Second,
		Stdout:        stdout,
	}
}

// TestShipsToServerWhenUp proves records reach the server with the exact
// contract Record shape, including trace_id/span_id from the context.
func TestShipsToServerWhenUp(t *testing.T) {
	srv := newIngestServer(t)
	var stdout bytes.Buffer
	h := NewHandler("identity", testOptions(t, srv.URL, &stdout))
	t.Cleanup(func() { h.Close(context.Background()) })

	log := slog.New(h)
	log.InfoContext(ctxWithSpan(t), "gRPC listening", "addr", ":9090")

	h.Flush(context.Background())

	recs := srv.got()
	if len(recs) != 1 {
		t.Fatalf("want 1 record shipped, got %d", len(recs))
	}
	r := recs[0]
	if r.Level != "INFO" {
		t.Errorf("level: want INFO, got %q", r.Level)
	}
	if r.Service != "identity" {
		t.Errorf("service: want identity, got %q", r.Service)
	}
	if r.Message != "gRPC listening" {
		t.Errorf("message: want %q, got %q", "gRPC listening", r.Message)
	}
	if r.TraceID != testTraceID {
		t.Errorf("trace_id: want %q, got %q", testTraceID, r.TraceID)
	}
	if r.SpanID != testSpanID {
		t.Errorf("span_id: want %q, got %q", testSpanID, r.SpanID)
	}
	if got := r.Fields["addr"]; got != ":9090" {
		t.Errorf("fields.addr: want :9090, got %v", got)
	}
	// ts must be RFC3339 UTC with fractional seconds.
	if _, err := time.Parse(time.RFC3339Nano, r.Ts); err != nil {
		t.Errorf("ts not RFC3339: %q (%v)", r.Ts, err)
	}
	if !strings.HasSuffix(r.Ts, "Z") {
		t.Errorf("ts not UTC (no trailing Z): %q", r.Ts)
	}
}

// TestServerDownSpoolsThenBackfills proves store-and-forward: while the server
// is down records land in the spool file; when it recovers the spool is drained
// and truncated, with nothing lost.
func TestServerDownSpoolsThenBackfills(t *testing.T) {
	srv := newIngestServer(t)
	srv.setHealthy(false)

	var stdout bytes.Buffer
	opts := testOptions(t, srv.URL, &stdout)
	spoolDir := opts.SpoolDir
	h := NewHandler("catalog", opts)
	t.Cleanup(func() { h.Close(context.Background()) })

	log := slog.New(h)
	ctx := ctxWithSpan(t)
	for i := 0; i < 3; i++ {
		log.InfoContext(ctx, "event", "n", i)
	}
	h.Flush(context.Background())

	// Server was down => nothing accepted, everything spooled.
	if got := srv.got(); len(got) != 0 {
		t.Fatalf("server was down but got %d records", len(got))
	}
	spoolPath := filepath.Join(spoolDir, "catalog.jsonl")
	data, err := os.ReadFile(spoolPath)
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	if lines := nonEmptyLines(data); lines != 3 {
		t.Fatalf("want 3 spooled lines, got %d\n%s", lines, data)
	}

	// Server recovers => next flush drains and truncates the spool.
	srv.setHealthy(true)
	h.Flush(context.Background())

	recs := srv.got()
	if len(recs) != 3 {
		t.Fatalf("want 3 backfilled records, got %d", len(recs))
	}
	for i, r := range recs {
		if r.TraceID != testTraceID {
			t.Errorf("record %d lost trace_id: %q", i, r.TraceID)
		}
	}
	if _, err := os.Stat(spoolPath); !os.IsNotExist(err) {
		t.Errorf("spool not truncated after backfill (stat err=%v)", err)
	}
}

// TestStdoutAlwaysWritten proves the JSON line hits stdout regardless of the
// server's state — logs are never lost even when shipping fails.
func TestStdoutAlwaysWritten(t *testing.T) {
	srv := newIngestServer(t)
	srv.setHealthy(false) // shipping will fail; stdout must still get the line

	var stdout bytes.Buffer
	h := NewHandler("surprise", testOptions(t, srv.URL, &stdout))
	t.Cleanup(func() { h.Close(context.Background()) })

	slog.New(h).InfoContext(ctxWithSpan(t), "hello", "k", "v")

	line := stdout.String()
	if line == "" {
		t.Fatal("stdout empty")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, line)
	}
	if m["service"] != "surprise" {
		t.Errorf("stdout service: got %v", m["service"])
	}
	if m["msg"] != "hello" {
		t.Errorf("stdout msg: got %v", m["msg"])
	}
	if m["trace_id"] != testTraceID {
		t.Errorf("stdout trace_id: got %v", m["trace_id"])
	}
}

// TestEmptyServerURLStdoutOnly proves that with no LOG_SERVER_URL, logkit is a
// pure stdout logger: no shipper, no spool, no error.
func TestEmptyServerURLStdoutOnly(t *testing.T) {
	var stdout bytes.Buffer
	opts := testOptions(t, "", &stdout)
	h := NewHandler("poll", opts)
	t.Cleanup(func() { h.Close(context.Background()) })

	if h.shp != nil {
		t.Fatal("shipper started despite empty LOG_SERVER_URL")
	}

	slog.New(h).Info("no shipping here", "x", 1)

	// stdout still works.
	if !strings.Contains(stdout.String(), "no shipping here") {
		t.Errorf("stdout missing line: %q", stdout.String())
	}
	// Flush/Close are safe no-ops.
	h.Flush(context.Background())
	h.Close(context.Background())

	// No spool file created.
	if entries, _ := os.ReadDir(opts.SpoolDir); len(entries) != 0 {
		t.Errorf("spool dir not empty: %v", entries)
	}
}

// TestFullQueueDropsWithoutBlocking proves a full queue never blocks the caller.
func TestFullQueueDropsWithoutBlocking(t *testing.T) {
	srv := newIngestServer(t)
	srv.setHealthy(false)
	var stdout bytes.Buffer
	opts := testOptions(t, srv.URL, &stdout)
	opts.QueueSize = 1
	h := NewHandler("identity", opts)
	t.Cleanup(func() { h.Close(context.Background()) })

	// Enqueue far more than the queue can hold; must return promptly, not hang.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10000; i++ {
			h.shp.enqueue(Record{Message: "x", Fields: map[string]any{}})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("enqueue blocked on a full queue")
	}
	if h.shp.droppedCount() == 0 {
		t.Error("expected some drops on an oversubscribed queue")
	}
}

func nonEmptyLines(b []byte) int {
	n := 0
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}
