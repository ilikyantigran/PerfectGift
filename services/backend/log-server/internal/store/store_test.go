package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// Temp-file DB (WAL needs a real file, not :memory:).
	path := filepath.Join(t.TempDir(), "logs.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func rec(svc, level, msg string) Record {
	return Record{
		TS:      time.Now().UTC().Format(time.RFC3339Nano),
		Level:   level,
		Service: svc,
		Message: msg,
		Fields:  json.RawMessage(`{}`),
	}
}

func TestIngestAndQueryBack(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	in := []Record{
		{TS: "2026-07-10T18:34:53.123456Z", Level: "INFO", Service: "identity",
			Message: "gRPC listening", TraceID: "0af7651916cd43dd8448eb211c80319c",
			SpanID: "b7ad6b7169203331", Fields: json.RawMessage(`{"addr":":9090"}`)},
		{TS: "2026-07-10T18:34:54.000000Z", Level: "ERROR", Service: "poll",
			Message: "boom", Fields: json.RawMessage(`{}`)},
	}
	n, err := st.Insert(ctx, in)
	if err != nil || n != 2 {
		t.Fatalf("insert: n=%d err=%v", n, err)
	}

	rows, err := st.Query(ctx, Query{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// Newest-first: highest id first. The second insert must lead.
	if rows[0].ID <= rows[1].ID {
		t.Fatalf("not newest-first: ids %d then %d", rows[0].ID, rows[1].ID)
	}
	if rows[0].Message != "boom" || rows[0].ID != 2 {
		t.Fatalf("unexpected head row: %+v", rows[0])
	}
	// LogRow shape: id assigned, fields preserved.
	if rows[1].ID != 1 || rows[1].TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("unexpected row1: %+v", rows[1])
	}
	if string(rows[1].Fields) != `{"addr":":9090"}` {
		t.Fatalf("fields not preserved: %s", rows[1].Fields)
	}
}

func TestQGlobAndSubstring(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	_, _ = st.Insert(ctx, []Record{
		rec("a", "INFO", "the auth problem: token expired"),
		rec("a", "INFO", "unrelated message"),
		rec("a", "INFO", "AUTH PROBLEM: mixed case"),
		rec("a", "INFO", "literal 50% off _underscore_"),
	})

	// Glob: *auth problem:* => contains "auth problem:" (case-insensitive).
	rows, _ := st.Query(ctx, Query{Q: "*auth problem:*"})
	if len(rows) != 2 {
		t.Fatalf("glob want 2, got %d: %+v", len(rows), msgs(rows))
	}

	// Plain substring (no '*').
	rows, _ = st.Query(ctx, Query{Q: "unrelated"})
	if len(rows) != 1 || rows[0].Message != "unrelated message" {
		t.Fatalf("substring want 1, got %+v", msgs(rows))
	}

	// Case-insensitivity.
	rows, _ = st.Query(ctx, Query{Q: "AUTH PROBLEM:"})
	if len(rows) != 2 {
		t.Fatalf("case-insensitive want 2, got %d", len(rows))
	}

	// % and _ are escaped literally: "50%" must not act as a wildcard.
	rows, _ = st.Query(ctx, Query{Q: "50%"})
	if len(rows) != 1 || rows[0].Message != "literal 50% off _underscore_" {
		t.Fatalf("literal %% want 1, got %+v", msgs(rows))
	}
	rows, _ = st.Query(ctx, Query{Q: "_underscore_"})
	if len(rows) != 1 {
		t.Fatalf("literal _ want 1, got %d", len(rows))
	}
	// A lone '_' would match any single char if unescaped; escaped it matches none.
	rows, _ = st.Query(ctx, Query{Q: "zzz_zzz"})
	if len(rows) != 0 {
		t.Fatalf("escaped _ want 0, got %d", len(rows))
	}
}

func TestFilters(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	_, _ = st.Insert(ctx, []Record{
		{TS: "2026-07-10T10:00:00Z", Level: "INFO", Service: "identity", Message: "a"},
		{TS: "2026-07-10T11:00:00Z", Level: "ERROR", Service: "identity", Message: "b"},
		{TS: "2026-07-10T12:00:00Z", Level: "INFO", Service: "poll", Message: "c"},
	})

	rows, _ := st.Query(ctx, Query{Service: "identity"})
	if len(rows) != 2 {
		t.Fatalf("service filter want 2, got %d", len(rows))
	}
	rows, _ = st.Query(ctx, Query{Level: "ERROR"})
	if len(rows) != 1 || rows[0].Message != "b" {
		t.Fatalf("level filter want [b], got %+v", msgs(rows))
	}
	// from/to inclusive.
	rows, _ = st.Query(ctx, Query{From: "2026-07-10T11:00:00Z", To: "2026-07-10T12:00:00Z"})
	if len(rows) != 2 {
		t.Fatalf("time range want 2, got %d", len(rows))
	}
}

func TestLimitCapAndAfterCursor(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	batch := make([]Record, 5)
	for i := range batch {
		batch[i] = rec("svc", "INFO", "m")
	}
	_, _ = st.Insert(ctx, batch)

	// Limit clamps to >=1; here explicit small limit.
	rows, _ := st.Query(ctx, Query{Limit: 2})
	if len(rows) != 2 {
		t.Fatalf("limit want 2, got %d", len(rows))
	}
	// Limit over max is capped at 1000 (can't overflow rows here, just ensure no error).
	if _, err := st.Query(ctx, Query{Limit: 999999}); err != nil {
		t.Fatalf("limit cap query err: %v", err)
	}

	// after cursor: only rows with id > after, newest-first.
	rows, _ = st.Query(ctx, Query{After: 3})
	if len(rows) != 2 {
		t.Fatalf("after want 2 rows (ids 4,5), got %d", len(rows))
	}
	for _, r := range rows {
		if r.ID <= 3 {
			t.Fatalf("after cursor leaked id %d", r.ID)
		}
	}
}

func TestServicesDistinct(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	_, _ = st.Insert(ctx, []Record{
		rec("identity", "INFO", "x"),
		rec("poll", "INFO", "y"),
		rec("identity", "INFO", "z"),
		rec("", "INFO", "no-service"),
	})
	svcs, err := st.Services(ctx)
	if err != nil {
		t.Fatalf("services: %v", err)
	}
	if len(svcs) != 2 || svcs[0] != "identity" || svcs[1] != "poll" {
		t.Fatalf("distinct services wrong: %v", svcs)
	}
}

func TestPrune(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	_, _ = st.Insert(ctx, []Record{
		{TS: "2026-07-01T00:00:00Z", Level: "INFO", Service: "s", Message: "old"},
		{TS: time.Now().UTC().Format(time.RFC3339Nano), Level: "INFO", Service: "s", Message: "new"},
	})
	n, err := st.Prune(ctx, time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC))
	if err != nil || n != 1 {
		t.Fatalf("prune n=%d err=%v", n, err)
	}
	rows, _ := st.Query(ctx, Query{})
	if len(rows) != 1 || rows[0].Message != "new" {
		t.Fatalf("prune kept wrong rows: %+v", msgs(rows))
	}
}

func msgs(rows []LogRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Message
	}
	return out
}
