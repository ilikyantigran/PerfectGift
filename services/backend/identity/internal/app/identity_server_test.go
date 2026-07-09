package app

import (
	"context"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/oauth"
	"github.com/ilikyantigran/PerfectGift/services/backend/identity/internal/token"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/identity/pkg/api/identity/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type harness struct {
	srv      *Server
	users    *fakeUsers
	sessions *fakeSessions
	limiter  *fakeLimiter
	verifier *oauth.FakeVerifier
	tokens   *token.Manager
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	tm, err := token.NewManager("https://issuer.test", "aud.test", 15*time.Minute)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	users := newFakeUsers()
	sessions := newFakeSessions()
	limiter := newFakeLimiter(3)
	verifier := oauth.NewFakeVerifier()
	srv := NewServer(Deps{
		Users:      users,
		Sessions:   sessions,
		Limiter:    limiter,
		Verifier:   verifier,
		Tokens:     tm,
		RefreshTTL: 720 * time.Hour,
	})
	return &harness{srv, users, sessions, limiter, verifier, tm}
}

func codeOf(err error) codes.Code { return status.Code(err) }

func TestSignInGoogleCreatesUser(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g-token", oauth.Identity{Provider: "google", Subject: "g-sub-1", Email: "user@example.com", EmailVerified: true})

	resp, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{
		Provider: identityv1.Provider_PROVIDER_GOOGLE,
		IdToken:  "g-token",
	})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if resp.User.GetEmail() != "user@example.com" {
		t.Errorf("email = %q", resp.User.GetEmail())
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("expires_in = %d", resp.ExpiresIn)
	}
	// Access token must verify and carry the user id.
	claims, err := h.tokens.Verify(resp.AccessToken)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}
	if claims.Subject != resp.User.GetId() {
		t.Errorf("subject %q != user id %q", claims.Subject, resp.User.GetId())
	}
}

func TestSignInGoogleSameSubjectReturnsSameUser(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g-token", oauth.Identity{Provider: "google", Subject: "g-sub-1", Email: "user@example.com"})
	r1, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g-token"})
	r2, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g-token"})
	if err != nil {
		t.Fatalf("second SignIn: %v", err)
	}
	if r1.User.GetId() != r2.User.GetId() {
		t.Errorf("expected same user id, got %q and %q", r1.User.GetId(), r2.User.GetId())
	}
}

func TestSignInGoogleBadToken(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "nope"})
	if codeOf(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestSignInEmailRegistersThenAuthenticates(t *testing.T) {
	h := newHarness(t)
	// First sign-in registers the account.
	r1, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{
		Provider: identityv1.Provider_PROVIDER_EMAIL,
		Email:    "e@example.com",
		Password: "hunter2horse",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	// Second sign-in with correct password authenticates the same user.
	r2, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{
		Provider: identityv1.Provider_PROVIDER_EMAIL,
		Email:    "e@example.com",
		Password: "hunter2horse",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if r1.User.GetId() != r2.User.GetId() {
		t.Errorf("different user ids across register/login")
	}
}

func TestSignInEmailWrongPassword(t *testing.T) {
	h := newHarness(t)
	_, _ = h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_EMAIL, Email: "e@example.com", Password: "right-password"})
	_, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_EMAIL, Email: "e@example.com", Password: "wrong-password"})
	if codeOf(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
	if got := status.Convert(err).Message(); got == "wrong password" {
		t.Errorf("error message leaks which factor failed: %q", got)
	}
}

func TestSignInEmailRateLimited(t *testing.T) {
	h := newHarness(t) // limiter max = 3
	req := &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_EMAIL, Email: "e@example.com", Password: "x"}
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = h.srv.SignIn(context.Background(), req)
	}
	if codeOf(lastErr) != codes.ResourceExhausted {
		t.Fatalf("want ResourceExhausted after too many attempts, got %v", lastErr)
	}
}

func TestSignInUnsupportedProvider(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_UNSPECIFIED})
	if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestRefreshRotatesAndInvalidatesOld(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s", Email: "e@x.com"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})

	refreshed, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: signIn.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == signIn.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if refreshed.AccessToken == "" {
		t.Fatal("no new access token")
	}
	// New refresh token works.
	if _, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: refreshed.RefreshToken}); err != nil {
		t.Fatalf("new refresh should work: %v", err)
	}
}

func TestRefreshReuseRevokesSession(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})

	// Rotate once; the original token is now stale.
	if _, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: signIn.RefreshToken}); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	// Presenting the stale original triggers reuse detection.
	_, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: signIn.RefreshToken})
	if codeOf(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated on reuse, got %v", err)
	}
	// Session is revoked: even the last-good rotated token no longer works.
	// (We can't hold it here, but the reused session id must be gone.)
	sid := sessionIDFromRefresh(signIn.RefreshToken)
	if _, ok, _ := h.sessions.Get(context.Background(), sid); ok {
		t.Fatal("session should have been deleted on reuse")
	}
}

func TestRefreshInvalidToken(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: "garbage-no-dot"})
	if codeOf(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestRevokeByRefreshToken(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})

	if _, err := h.srv.Revoke(context.Background(), &identityv1.RevokeRequest{RefreshToken: signIn.RefreshToken}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := h.srv.RefreshToken(context.Background(), &identityv1.RefreshTokenRequest{RefreshToken: signIn.RefreshToken}); codeOf(err) != codes.Unauthenticated {
		t.Fatalf("refresh after revoke should fail Unauthenticated, got %v", err)
	}
}

func TestRevokeBySessionID(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})
	sid := sessionIDFromRefresh(signIn.RefreshToken)

	if _, err := h.srv.Revoke(context.Background(), &identityv1.RevokeRequest{SessionId: sid}); err != nil {
		t.Fatalf("revoke by session id: %v", err)
	}
	if _, ok, _ := h.sessions.Get(context.Background(), sid); ok {
		t.Fatal("session not deleted")
	}
}

func TestRevokeRequiresAnInput(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.Revoke(context.Background(), &identityv1.RevokeRequest{})
	if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

func TestGetMeWithBearer(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s", Email: "me@x.com"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signIn.AccessToken))
	resp, err := h.srv.GetMe(ctx, &identityv1.GetMeRequest{})
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if resp.User.GetEmail() != "me@x.com" {
		t.Errorf("email = %q", resp.User.GetEmail())
	}
}

func TestGetMeWithoutToken(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.GetMe(context.Background(), &identityv1.GetMeRequest{})
	if codeOf(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestValidateToken(t *testing.T) {
	h := newHarness(t)
	h.verifier.Register("g", oauth.Identity{Provider: "google", Subject: "s"})
	signIn, _ := h.srv.SignIn(context.Background(), &identityv1.SignInRequest{Provider: identityv1.Provider_PROVIDER_GOOGLE, IdToken: "g"})

	resp, err := h.srv.ValidateToken(context.Background(), &identityv1.ValidateTokenRequest{AccessToken: signIn.AccessToken})
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !resp.Valid {
		t.Fatal("expected valid=true")
	}
	if resp.Subject != signIn.User.GetId() {
		t.Errorf("subject = %q", resp.Subject)
	}
	if resp.Claims == nil || resp.Claims.Fields["sub"] == nil {
		t.Error("claims struct missing sub")
	}

	bad, err := h.srv.ValidateToken(context.Background(), &identityv1.ValidateTokenRequest{AccessToken: "not-a-token"})
	if err != nil {
		t.Fatalf("ValidateToken(bad) should not error: %v", err)
	}
	if bad.Valid {
		t.Error("expected valid=false for garbage token")
	}
}

func TestGetJWKS(t *testing.T) {
	h := newHarness(t)
	resp, err := h.srv.GetJWKS(context.Background(), &identityv1.GetJWKSRequest{})
	if err != nil {
		t.Fatalf("GetJWKS: %v", err)
	}
	if len(resp.Keys) == 0 {
		t.Fatal("no keys published")
	}
	k := resp.Keys[0]
	if k.GetKty() != "OKP" || k.GetCrv() != "Ed25519" || k.GetKid() == "" {
		t.Errorf("unexpected JWK: %+v", k)
	}
}
