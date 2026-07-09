package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// downstreamTimeout bounds every gRPC call the gateway makes to a domain service.
const downstreamTimeout = 15 * time.Second

// reqCtx derives a timeout-bounded context from the request for a downstream call.
func reqCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), downstreamTimeout)
}

// ctxKey is an unexported context key type to avoid collisions.
type ctxKey int

const (
	ctxSubject ctxKey = iota // authenticated user id (JWT subject)
)

// subjectFrom returns the authenticated user id put on the context by requireJWT.
func subjectFrom(ctx context.Context) string {
	s, _ := ctx.Value(ctxSubject).(string)
	return s
}

// decodeJSON strictly decodes a JSON body into v, rejecting unknown fields and
// oversized payloads. Returns false (and writes a 400 envelope) on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "request body is required", nil)
		return false
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "invalid JSON body: "+err.Error(), nil)
		return false
	}
	return true
}

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// bearerToken extracts the token from an `Authorization: Bearer <t>` header.
func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("malformed Authorization header")
	}
	return parts[1], nil
}

// clientIP returns a best-effort client IP for per-IP rate limiting, honoring
// X-Forwarded-For (the gateway sits behind a load balancer / ingress).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
