// Package store persists log records in a pure-Go SQLite database
// (modernc.org/sqlite, so the build stays CGO-free).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection pool.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS logs (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	ts       TEXT,
	level    TEXT,
	service  TEXT,
	message  TEXT,
	trace_id TEXT,
	span_id  TEXT,
	fields   TEXT
);
CREATE INDEX IF NOT EXISTS idx_logs_ts       ON logs(ts);
CREATE INDEX IF NOT EXISTS idx_logs_service  ON logs(service);
CREATE INDEX IF NOT EXISTS idx_logs_level    ON logs(level);
CREATE INDEX IF NOT EXISTS idx_logs_trace_id ON logs(trace_id);
`

// Open opens (creating if needed) the SQLite database at path with WAL mode
// and a busy timeout, then ensures the schema exists.
func Open(path string) (*Store, error) {
	// WAL + busy_timeout keep the hot ingest path from blocking readers.
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Insert stores a batch of records in a single transaction and returns the
// number accepted. Missing optional fields are defaulted by the caller.
func (s *Store) Insert(ctx context.Context, recs []Record) (int, error) {
	if len(recs) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO logs(ts, level, service, message, trace_id, span_id, fields)
		 VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for i := range recs {
		r := &recs[i]
		fields := "{}"
		if len(r.Fields) > 0 {
			fields = string(r.Fields)
		}
		if _, err := stmt.ExecContext(ctx,
			r.TS, r.Level, r.Service, r.Message, r.TraceID, r.SpanID, fields); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(recs), nil
}

// Query returns log rows matching q, newest-first (highest id first).
func (s *Store) Query(ctx context.Context, q Query) ([]LogRow, error) {
	var (
		where []string
		args  []any
	)

	if q.Service != "" {
		where = append(where, "service = ?")
		args = append(args, q.Service)
	}
	if q.Level != "" {
		where = append(where, "level = ?")
		args = append(args, q.Level)
	}
	if q.From != "" {
		where = append(where, "ts >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "ts <= ?")
		args = append(args, q.To)
	}
	if q.After > 0 {
		where = append(where, "id > ?")
		args = append(args, q.After)
	}
	if q.Q != "" {
		where = append(where, `LOWER(message) LIKE LOWER(?) ESCAPE '\'`)
		args = append(args, likePattern(q.Q))
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	sqlStr := `SELECT id, ts, level, service, message, trace_id, span_id, fields FROM logs`
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LogRow, 0, limit)
	for rows.Next() {
		var lr LogRow
		var fields string
		if err := rows.Scan(&lr.ID, &lr.TS, &lr.Level, &lr.Service,
			&lr.Message, &lr.TraceID, &lr.SpanID, &fields); err != nil {
			return nil, err
		}
		lr.Fields = []byte(fields)
		out = append(out, lr)
	}
	return out, rows.Err()
}

// Services returns the distinct set of service names, sorted.
func (s *Store) Services(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT service FROM logs WHERE service <> '' ORDER BY service`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var svc string
		if err := rows.Scan(&svc); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

// Prune deletes rows whose ts is older than the given cutoff and returns the
// number removed. Rows with an unparseable/empty ts are left untouched.
func (s *Store) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM logs WHERE ts <> '' AND ts < ?`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// likePattern converts a user query into a SQL LIKE pattern.
//   - Existing % and _ (and the escape char \) are escaped so they match
//     literally.
//   - '*' becomes the LIKE wildcard %.
//   - A query with no '*' is treated as a substring match (wrapped in %…%).
func likePattern(q string) string {
	var b strings.Builder
	hasStar := strings.Contains(q, "*")
	for _, r := range q {
		switch r {
		case '\\', '%', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '*':
			b.WriteByte('%')
		default:
			b.WriteRune(r)
		}
	}
	if hasStar {
		return b.String()
	}
	return "%" + b.String() + "%"
}
