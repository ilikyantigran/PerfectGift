// Package rest is the gateway's public HTTP/JSON edge. It owns the route table
// (SERVICE.md §3.1), maps each REST request to the correct downstream gRPC call, and
// enforces the edge concerns: JWT validation, rate limiting, CORS on the anonymous
// poll routes, and the uniform error envelope. It depends only on the generated
// <Svc>Client interfaces, so it is fully testable against fakes with no real
// service, DB, or network.
package rest

import (
	"context"
	"net/http"
	"strings"

	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/auth"
	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/ratelimit"

	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/catalog/v1"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/identity/v1"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/notification/v1"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/surprise/v1"
)

// TokenVerifier validates a bearer JWT locally and returns its claims. The real
// implementation is *auth.Verifier (JWKS-backed); tests inject a fake.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*auth.Claims, error)
}

// Options are the dependencies the REST edge needs. Every gRPC client is an
// interface (the generated <Svc>Client), so a fake can be substituted in tests.
type Options struct {
	Identity     identityv1.IdentityClient
	Poll         pollv1.PollClient
	Surprise     surprisev1.SurpriseClient
	Catalog      catalogv1.CatalogClient
	Notification notificationv1.NotificationClient

	Verifier TokenVerifier

	// Rate limiters. Any nil limiter is treated as disabled (Noop).
	GlobalLimiter  ratelimit.Limiter // all traffic
	PerIPLimiter   ratelimit.Limiter // per client IP (critical for anonymous poll routes)
	PerUserLimiter ratelimit.Limiter // per JWT subject
	RefreshLimiter ratelimit.Limiter // strict, per IP, for POST /v1/auth/refresh

	// CORSOrigins are the allowed origins for the two anonymous poll routes only.
	CORSOrigins []string
}

// Server holds the wired dependencies and builds the HTTP handler.
type Server struct {
	opts Options
}

// New builds a Server, defaulting any nil limiter to a no-op.
func New(opts Options) *Server {
	if opts.GlobalLimiter == nil {
		opts.GlobalLimiter = ratelimit.Noop{}
	}
	if opts.PerIPLimiter == nil {
		opts.PerIPLimiter = ratelimit.Noop{}
	}
	if opts.PerUserLimiter == nil {
		opts.PerUserLimiter = ratelimit.Noop{}
	}
	if opts.RefreshLimiter == nil {
		opts.RefreshLimiter = ratelimit.Noop{}
	}
	return &Server{opts: opts}
}

// Handler returns the fully-wired HTTP handler: the route table plus the global
// middleware chain (recover → request id → global+IP rate limit).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// --- Auth (Identity) ---
	mux.HandleFunc("POST /v1/auth/signin", s.handleSignIn)
	mux.HandleFunc("POST /v1/auth/refresh", s.limitRefresh(s.handleRefresh))
	mux.HandleFunc("POST /v1/auth/revoke", s.requireJWT(s.handleRevoke))
	mux.HandleFunc("GET /v1/me", s.requireJWT(s.handleGetMe))

	// --- Polls (owner-scoped, JWT) ---
	mux.HandleFunc("POST /v1/polls", s.requireJWT(s.handleCreatePoll))
	mux.HandleFunc("GET /v1/polls/{id}/responses", s.requireJWT(s.handleGetResponses))

	// --- Generations (Surprise) ---
	mux.HandleFunc("POST /v1/generations", s.requireJWT(s.handleRequestGeneration))
	mux.HandleFunc("GET /v1/generations/{id}", s.requireJWT(s.handleGetGeneration))
	mux.HandleFunc("POST /v1/generations/{id}/refine", s.requireJWT(s.handleRefine))
	mux.HandleFunc("POST /v1/ideas/{id}/save", s.requireJWT(s.handleSaveIdea))

	// --- Catalog (reference data, JWT) ---
	mux.HandleFunc("GET /v1/holidays", s.requireJWT(s.handleListHolidays))
	mux.HandleFunc("GET /v1/categories", s.requireJWT(s.handleGetCategories))

	// --- Devices (Notification) ---
	mux.HandleFunc("POST /v1/devices", s.requireJWT(s.handleRegisterDevice))

	// --- Health ---
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// The anonymous Subject routes live under the fixed prefix `/v1/polls/token/`.
	// They are kept in a SEPARATE sub-mux and dispatched by prefix (see routeRoot)
	// rather than registered as siblings of `GET /v1/polls/{id}/responses`: stdlib
	// ServeMux cannot disambiguate `GET /v1/polls/token/{t}` from
	// `GET /v1/polls/{id}/responses` (both match `/v1/polls/token/responses`). These
	// are the only CORS-enabled routes (SERVICE.md §5) and carry an opaque token, NOT
	// a JWT.
	tokenMux := http.NewServeMux()
	tokenMux.Handle("GET /v1/polls/token/{t}", s.cors(http.HandlerFunc(s.handleGetPollByToken)))
	tokenMux.Handle("OPTIONS /v1/polls/token/{t}", s.cors(http.HandlerFunc(s.handleCORSPreflight)))
	tokenMux.Handle("POST /v1/polls/token/{t}/responses", s.cors(http.HandlerFunc(s.handleSubmitResponse)))
	tokenMux.Handle("OPTIONS /v1/polls/token/{t}/responses", s.cors(http.HandlerFunc(s.handleCORSPreflight)))

	root := routeRoot(mux, tokenMux)
	return s.recoverMW(s.globalLimitMW(root))
}

// routeRoot sends anonymous poll-token traffic to tokenMux and everything else to
// the main mux, sidestepping ServeMux's inability to disambiguate the token routes
// from the owner `GET /v1/polls/{id}/responses` route.
func routeRoot(main, tokenMux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/polls/token/") {
			tokenMux.ServeHTTP(w, r)
			return
		}
		main.ServeHTTP(w, r)
	})
}
