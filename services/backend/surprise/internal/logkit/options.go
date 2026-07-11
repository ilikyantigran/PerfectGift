package logkit

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Default configuration values. These are intentionally conservative so that
// logging never becomes a resource hog or a source of backpressure.
const (
	DefaultSpoolDir      = "/var/log/spool"
	DefaultBatchSize     = 100
	DefaultFlushInterval = 2 * time.Second
	DefaultQueueSize     = 4096
	DefaultHTTPTimeout   = 5 * time.Second
	DefaultRetryInterval = 10 * time.Second
	DefaultSpoolMaxBytes = 50 << 20 // 50 MiB
)

// Options configures a logkit handler. The zero value is not usable directly;
// build one with OptionsFromEnv (which fills defaults) or set fields explicitly
// and rely on NewHandler to backfill any left at their zero value.
type Options struct {
	// ServerURL is the base URL of the log-server, e.g. "http://log-server:8080".
	// Empty disables shipping entirely (stdout-only); the server is optional.
	ServerURL string

	// SpoolDir is the directory for the on-disk store-and-forward spool.
	SpoolDir string

	// Level is the minimum level that is handled (defaults to Info, matching the
	// services' current telemetry setup).
	Level slog.Level

	// BatchSize flushes a batch to the server once this many records accumulate.
	BatchSize int
	// FlushInterval flushes a partial batch after at most this long.
	FlushInterval time.Duration
	// QueueSize bounds the in-memory queue. When full, records are dropped
	// (counted) rather than blocking the caller.
	QueueSize int
	// HTTPTimeout bounds a single POST /api/ingest call.
	HTTPTimeout time.Duration
	// RetryInterval is how often the background loop retries draining the spool.
	RetryInterval time.Duration
	// SpoolMaxBytes caps the spool file; oldest lines are dropped beyond it.
	SpoolMaxBytes int64

	// Stdout is where the JSON log line is always written. Defaults to os.Stdout.
	// Injectable for tests.
	Stdout io.Writer

	// HTTPClient sends batches to the server. Defaults to a client using
	// HTTPTimeout. Injectable for tests.
	HTTPClient *http.Client
}

// OptionsFromEnv builds Options from environment variables, applying defaults:
//
//	LOG_SERVER_URL   base URL of the log-server ("" => shipping disabled)
//	LOG_SPOOL_DIR    spool directory        (default /var/log/spool)
//	LOG_BATCH_SIZE   records per batch       (default 100)
//	LOG_FLUSH_MS     flush interval, ms      (default 2000)
//	LOG_QUEUE_SIZE   in-memory queue depth   (default 4096)
//	LOG_HTTP_MS      per-request timeout, ms (default 5000)
//	LOG_RETRY_MS     spool retry interval,ms (default 10000)
//	LOG_SPOOL_MAX    spool cap in bytes      (default 52428800)
func OptionsFromEnv() Options {
	return Options{
		ServerURL:     os.Getenv("LOG_SERVER_URL"),
		SpoolDir:      envStr("LOG_SPOOL_DIR", DefaultSpoolDir),
		Level:         slog.LevelInfo,
		BatchSize:     envInt("LOG_BATCH_SIZE", DefaultBatchSize),
		FlushInterval: envDurMS("LOG_FLUSH_MS", DefaultFlushInterval),
		QueueSize:     envInt("LOG_QUEUE_SIZE", DefaultQueueSize),
		HTTPTimeout:   envDurMS("LOG_HTTP_MS", DefaultHTTPTimeout),
		RetryInterval: envDurMS("LOG_RETRY_MS", DefaultRetryInterval),
		SpoolMaxBytes: envInt64("LOG_SPOOL_MAX", DefaultSpoolMaxBytes),
	}
}

// withDefaults returns a copy of o with any zero-valued field replaced by its
// default, so callers can construct a partial Options.
func (o Options) withDefaults() Options {
	if o.SpoolDir == "" {
		o.SpoolDir = DefaultSpoolDir
	}
	if o.BatchSize <= 0 {
		o.BatchSize = DefaultBatchSize
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = DefaultFlushInterval
	}
	if o.QueueSize <= 0 {
		o.QueueSize = DefaultQueueSize
	}
	if o.HTTPTimeout <= 0 {
		o.HTTPTimeout = DefaultHTTPTimeout
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = DefaultRetryInterval
	}
	if o.SpoolMaxBytes <= 0 {
		o.SpoolMaxBytes = DefaultSpoolMaxBytes
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: o.HTTPTimeout}
	}
	return o
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envDurMS(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Millisecond
		}
	}
	return def
}
