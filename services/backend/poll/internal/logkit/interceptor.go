package logkit

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary interceptor that emits exactly one
// access-log line per request, using the request's context. That context carries
// the active OpenTelemetry server span (installed by otelgrpc.NewServerHandler),
// so the emitted log is stamped with the request's trace_id/span_id and becomes
// clickable through to Jaeger. No request/response payloads are logged (no PII):
// only the RPC method, resulting gRPC code, and wall-clock duration.
//
// Chain it after the otel stats handler so the span is in scope, e.g.:
//
//	grpc.NewServer(
//	    grpc.StatsHandler(otelgrpc.NewServerHandler()),
//	    grpc.ChainUnaryInterceptor(logkit.UnaryServerInterceptor(), authn.Unary()),
//	)
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)

		level := slog.LevelInfo
		if code != codes.OK {
			level = slog.LevelWarn
		}
		attrs := []any{
			slog.String("rpc.method", info.FullMethod),
			slog.String("rpc.code", code.String()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		// ctx carries the otelgrpc server span → the log gets a real trace_id.
		slog.Default().Log(ctx, level, "grpc "+info.FullMethod, attrs...)
		return resp, err
	}
}

// HTTPMiddleware wraps an http.Handler so every request emits exactly one
// access-log line using the request's context. Place it INSIDE otelhttp so the
// otel server span is in scope and the log carries the request's trace_id, e.g.:
//
//	otelhttp.NewHandler(logkit.HTTPMiddleware(mux), "gateway")
//
// Only method, path, status, and duration are logged (no headers or bodies).
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		} else if rec.status >= 400 {
			level = slog.LevelWarn
		}
		// r.Context() carries the otelhttp server span → real trace_id on the log.
		slog.Default().Log(r.Context(), level, "http "+r.Method+" "+r.URL.Path,
			slog.String("http.method", r.Method),
			slog.String("http.path", r.URL.Path),
			slog.Int("http.status", rec.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

// statusRecorder captures the response status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true // an implicit 200 is now committed
	return r.ResponseWriter.Write(b)
}

// Flush propagates http.Flusher so streaming/SSE handlers keep working.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
