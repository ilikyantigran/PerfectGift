package rest

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
)

// requireJWT validates the bearer token locally (JWKS) and, on success, injects the
// subject into the context and applies the per-user rate limit. It fails CLOSED:
// any missing/invalid/expired token is rejected with 401 (SERVICE.md §7).
func (s *Server) requireJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "missing or malformed bearer token", nil)
			return
		}
		claims, err := s.opts.Verifier.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "invalid token", nil)
			return
		}
		// Per-user rate limit, keyed by the JWT subject.
		if !s.opts.PerUserLimiter.Allow("user:" + claims.Subject) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "per-user rate limit exceeded", nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxSubject, claims.Subject)
		next(w, r.WithContext(ctx))
	}
}

// limitRefresh applies a strict per-IP limit to the sensitive refresh route, which
// is not JWT-protected but must be defended against brute force (SERVICE.md §3.1 note).
func (s *Server) limitRefresh(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.RefreshLimiter.Allow("refresh:" + clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "refresh rate limit exceeded", nil)
			return
		}
		next(w, r)
	}
}

// globalLimitMW applies the global budget and the per-IP budget to every request.
func (s *Server) globalLimitMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.GlobalLimiter.Allow("global") {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "global rate limit exceeded", nil)
			return
		}
		if !s.opts.PerIPLimiter.Allow("ip:" + clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "per-IP rate limit exceeded", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverMW turns a panic in any handler into a 500 envelope instead of a dropped
// connection.
func (s *Server) recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic in handler", "err", rec, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors applies CORS ONLY to the anonymous poll routes it wraps (SERVICE.md §5): the
// Poll Web Page origin is allowed to fetch/submit polls from the browser. No other
// route is CORS-enabled.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		next.ServeHTTP(w, r)
	})
}

// handleCORSPreflight answers an OPTIONS preflight (headers are set by the cors mw).
func (s *Server) handleCORSPreflight(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) originAllowed(origin string) bool {
	if slices.Contains(s.opts.CORSOrigins, "*") {
		return true
	}
	return slices.Contains(s.opts.CORSOrigins, origin)
}
