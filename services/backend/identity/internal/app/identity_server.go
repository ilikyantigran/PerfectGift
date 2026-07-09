package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/oauth"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/password"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/token"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/identity/pkg/api/identity/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// ErrNotFound is returned by the Users store when a user does not exist.
var ErrNotFound = errors.New("user not found")

// User and Session are the value types shared with the domain stores. They are
// aliases to the model package so a store there can satisfy the interfaces
// below without an import cycle.
type (
	User    = model.User
	Session = model.Session
)

// Users owns the identity Postgres schema (users, credentials, oauth_links).
type Users interface {
	// UpsertOAuthUser resolves (provider, subject) to a user, creating the user
	// and oauth link on first login. When email matches an existing user the link
	// is attached to that user instead of creating a duplicate.
	UpsertOAuthUser(ctx context.Context, provider, subject, email, displayName string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, bool, error)
	// CreateEmailUser creates a user together with a password credential.
	CreateEmailUser(ctx context.Context, email, displayName, passwordHash string) (User, error)
	// GetPasswordHash returns the stored password hash for a user, ok=false when
	// the user has no password credential (e.g. a social-only account).
	GetPasswordHash(ctx context.Context, userID string) (string, bool, error)
}

// Sessions owns login sessions / refresh tokens in Valkey.
type Sessions interface {
	Create(ctx context.Context, s Session) error
	Get(ctx context.Context, sessionID string) (Session, bool, error)
	Update(ctx context.Context, s Session) error
	Delete(ctx context.Context, sessionID string) error
}

// RateLimiter guards email/password sign-in from brute force. Allow records an
// attempt for key and reports whether it is within the configured limit.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Deps are the collaborators a Server needs.
type Deps struct {
	Users      Users
	Sessions   Sessions
	Limiter    RateLimiter
	Verifier   oauth.Verifier
	Tokens     *token.Manager
	RefreshTTL time.Duration
}

// Server implements the identity.v1 gRPC service.
type Server struct {
	identityv1.UnimplementedIdentityServiceServer

	users      Users
	sessions   Sessions
	limiter    RateLimiter
	verifier   oauth.Verifier
	tokens     *token.Manager
	refreshTTL time.Duration

	now func() time.Time
}

func NewServer(d Deps) *Server {
	return &Server{
		users:      d.Users,
		sessions:   d.Sessions,
		limiter:    d.Limiter,
		verifier:   d.Verifier,
		tokens:     d.Tokens,
		refreshTTL: d.RefreshTTL,
		now:        time.Now,
	}
}

// SignIn verifies a social ID token or an email+password and returns a fresh
// token pair. First social login creates the user; first email sign-in for an
// unregistered address registers it (there is no separate Register RPC).
func (s *Server) SignIn(ctx context.Context, req *identityv1.SignInRequest) (*identityv1.SignInResponse, error) {
	switch req.GetProvider() {
	case identityv1.Provider_PROVIDER_GOOGLE, identityv1.Provider_PROVIDER_APPLE:
		return s.signInSocial(ctx, req)
	case identityv1.Provider_PROVIDER_EMAIL:
		return s.signInEmail(ctx, req)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported provider")
	}
}

func (s *Server) signInSocial(ctx context.Context, req *identityv1.SignInRequest) (*identityv1.SignInResponse, error) {
	if req.GetIdToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "id_token is required for social sign-in")
	}
	provider := "google"
	if req.GetProvider() == identityv1.Provider_PROVIDER_APPLE {
		provider = "apple"
	}

	ext, err := s.verifier.Verify(ctx, provider, req.GetIdToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid id token")
	}

	user, err := s.users.UpsertOAuthUser(ctx, provider, ext.Subject, ext.Email, req.GetDisplayName())
	if err != nil {
		return nil, status.Error(codes.Internal, "could not resolve account")
	}

	access, refresh, expiresIn, err := s.issuePair(ctx, user, req.GetDevice())
	if err != nil {
		return nil, err
	}
	return &identityv1.SignInResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(expiresIn),
		User:         toProtoUser(user),
	}, nil
}

func (s *Server) signInEmail(ctx context.Context, req *identityv1.SignInRequest) (*identityv1.SignInResponse, error) {
	email := strings.TrimSpace(req.GetEmail())
	if email == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Brute-force protection per email.
	allowed, err := s.limiter.Allow(ctx, "signin:"+strings.ToLower(email))
	if err != nil {
		return nil, status.Error(codes.Internal, "rate limiter unavailable")
	}
	if !allowed {
		return nil, status.Error(codes.ResourceExhausted, "too many attempts, try again later")
	}

	existing, found, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, status.Error(codes.Internal, "lookup failed")
	}

	var user User
	if !found {
		// Sign-in-or-register: first use of an unregistered email creates it.
		hash, err := password.Hash(req.GetPassword())
		if err != nil {
			return nil, status.Error(codes.Internal, "could not create account")
		}
		user, err = s.users.CreateEmailUser(ctx, email, req.GetDisplayName(), hash)
		if err != nil {
			return nil, status.Error(codes.Internal, "could not create account")
		}
	} else {
		hash, ok, err := s.users.GetPasswordHash(ctx, existing.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "lookup failed")
		}
		// Generic error for any credential failure — no account-enumeration oracle.
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		match, err := password.Verify(req.GetPassword(), hash)
		if err != nil || !match {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		user = existing
	}

	access, refresh, expiresIn, err := s.issuePair(ctx, user, req.GetDevice())
	if err != nil {
		return nil, err
	}
	return &identityv1.SignInResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(expiresIn),
		User:         toProtoUser(user),
	}, nil
}

// RefreshToken performs a rotating refresh with reuse detection.
func (s *Server) RefreshToken(ctx context.Context, req *identityv1.RefreshTokenRequest) (*identityv1.RefreshTokenResponse, error) {
	sid, secret, ok := splitRefresh(req.GetRefreshToken())
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	sess, found, err := s.sessions.Get(ctx, sid)
	if err != nil {
		return nil, status.Error(codes.Internal, "session lookup failed")
	}
	if !found {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	// Reuse detection: a stale secret means the token was already rotated or
	// stolen — revoke the whole session.
	if !constantTimeEqual(hashSecret(secret), sess.RefreshHash) {
		_ = s.sessions.Delete(ctx, sid)
		return nil, status.Error(codes.Unauthenticated, "refresh token reuse detected")
	}

	if s.now().After(sess.ExpiresAt) {
		_ = s.sessions.Delete(ctx, sid)
		return nil, status.Error(codes.Unauthenticated, "session expired")
	}

	// Rotate the refresh secret in place; the access token is reissued.
	newSecret, err := randomSecret()
	if err != nil {
		return nil, status.Error(codes.Internal, "could not rotate token")
	}
	sess.RefreshHash = hashSecret(newSecret)
	sess.IssuedAt = s.now()
	if err := s.sessions.Update(ctx, sess); err != nil {
		return nil, status.Error(codes.Internal, "could not rotate token")
	}

	access, expiresIn, err := s.tokens.Issue(sess.UserID, sid)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not issue token")
	}
	return &identityv1.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: sid + "." + newSecret,
		ExpiresIn:    int64(expiresIn),
	}, nil
}

// Revoke deletes a session (sign-out) by refresh token or session id. It is
// idempotent — revoking an already-gone session is not an error.
func (s *Server) Revoke(ctx context.Context, req *identityv1.RevokeRequest) (*identityv1.RevokeResponse, error) {
	switch {
	case req.GetSessionId() != "":
		if err := s.sessions.Delete(ctx, req.GetSessionId()); err != nil {
			return nil, status.Error(codes.Internal, "revoke failed")
		}
	case req.GetRefreshToken() != "":
		sid, _, ok := splitRefresh(req.GetRefreshToken())
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "malformed refresh token")
		}
		if err := s.sessions.Delete(ctx, sid); err != nil {
			return nil, status.Error(codes.Internal, "revoke failed")
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "provide refresh_token or session_id")
	}
	return &identityv1.RevokeResponse{}, nil
}

// GetMe returns the current user's profile. The subject comes from the Bearer
// access token in request metadata, never from the request body.
func (s *Server) GetMe(ctx context.Context, _ *identityv1.GetMeRequest) (*identityv1.GetMeResponse, error) {
	raw := bearerFromContext(ctx)
	if raw == "" {
		return nil, status.Error(codes.Unauthenticated, "missing bearer token")
	}
	claims, err := s.tokens.Verify(raw)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	user, err := s.users.GetByID(ctx, claims.Subject)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unknown subject")
	}
	return &identityv1.GetMeResponse{User: toProtoUser(user)}, nil
}

// ValidateToken verifies an access token locally. An invalid token is reported
// as valid=false rather than an RPC error.
func (s *Server) ValidateToken(_ context.Context, req *identityv1.ValidateTokenRequest) (*identityv1.ValidateTokenResponse, error) {
	claims, err := s.tokens.Verify(req.GetAccessToken())
	if err != nil {
		return &identityv1.ValidateTokenResponse{Valid: false}, nil
	}
	st, err := structpb.NewStruct(map[string]any{
		"sub": claims.Subject,
		"sid": claims.SessionID,
		"iss": claims.Issuer,
		"aud": claims.Audience,
		"jti": claims.JTI,
		"iat": float64(claims.IssuedAt.Unix()),
		"exp": float64(claims.ExpiresAt.Unix()),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "could not encode claims")
	}
	return &identityv1.ValidateTokenResponse{
		Valid:   true,
		Subject: claims.Subject,
		Claims:  st,
	}, nil
}

// GetJWKS publishes the current + previous public keys.
func (s *Server) GetJWKS(_ context.Context, _ *identityv1.GetJWKSRequest) (*identityv1.GetJWKSResponse, error) {
	keys := s.tokens.JWKS()
	out := make([]*identityv1.JWK, 0, len(keys))
	for _, k := range keys {
		out = append(out, &identityv1.JWK{
			Kty: k.Kty, Crv: k.Crv, X: k.X, Kid: k.Kid, Use: k.Use, Alg: k.Alg,
		})
	}
	return &identityv1.GetJWKSResponse{Keys: out}, nil
}

// --- helpers ---

func (s *Server) issuePair(ctx context.Context, user User, device string) (access, refresh string, expiresIn int, err error) {
	sessionID := uuid.NewString()
	secret, err := randomSecret()
	if err != nil {
		return "", "", 0, status.Error(codes.Internal, "could not create session")
	}
	now := s.now()
	sess := Session{
		ID:          sessionID,
		UserID:      user.ID,
		RefreshHash: hashSecret(secret),
		Device:      device,
		IssuedAt:    now,
		ExpiresAt:   now.Add(s.refreshTTL),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return "", "", 0, status.Error(codes.Internal, "could not create session")
	}
	access, expiresIn, err = s.tokens.Issue(user.ID, sessionID)
	if err != nil {
		return "", "", 0, status.Error(codes.Internal, "could not issue token")
	}
	return access, sessionID + "." + secret, expiresIn, nil
}

func toProtoUser(u User) *identityv1.User {
	created := ""
	if !u.CreatedAt.IsZero() {
		created = u.CreatedAt.UTC().Format(time.RFC3339)
	}
	return &identityv1.User{
		Id:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Status:      u.Status,
		CreatedAt:   created,
	}
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// splitRefresh parses "{sessionID}.{secret}".
func splitRefresh(t string) (sid, secret string, ok bool) {
	i := strings.IndexByte(t, '.')
	if i <= 0 || i == len(t)-1 {
		return "", "", false
	}
	return t[:i], t[i+1:], true
}

// sessionIDFromRefresh extracts the session id from a refresh token (best effort).
func sessionIDFromRefresh(t string) string {
	sid, _, _ := splitRefresh(t)
	return sid
}

func bearerFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	h := vals[0]
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return h
}
