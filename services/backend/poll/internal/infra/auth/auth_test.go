package auth

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	testKid = "test-key-1"
	testIss = "https://identity.perfectgift.local"
	testAud = "perfectgift"
)

// mintEdDSA builds and signs an Ed25519 JWT the same way Identity does.
func mintEdDSA(t *testing.T, priv ed25519.PrivateKey, claims map[string]any) string {
	t.Helper()
	hdr := map[string]any{"alg": "EdDSA", "kid": testKid, "typ": "JWT"}
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signingInput := enc(hdr) + "." + enc(claims)
	sig := ed25519.Sign(priv, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// newTestVerifier returns a verifier backed by an in-memory Ed25519 keyset plus the
// private key to mint tokens with. Fully offline — no JWKS HTTP fetch.
func newTestVerifier(t *testing.T) (*Verifier, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	jwks := &JWKS{Keys: []JWK{{
		Kty: "OKP", Crv: "Ed25519", Alg: "EdDSA", Use: "sig", Kid: testKid,
		X: base64.RawURLEncoding.EncodeToString(pub),
	}}}
	v, err := NewVerifier(Config{Issuer: testIss, Audience: testAud, Source: StaticKeySet(jwks)})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v, priv
}

func validClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"sub": "user-123",
		"iss": testIss,
		"aud": testAud,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

func TestVerify_ValidEdDSAToken(t *testing.T) {
	v, priv := newTestVerifier(t)
	tok := mintEdDSA(t, priv, validClaims())
	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("subject = %q, want user-123", claims.Subject)
	}
}

func TestVerify_Rejects(t *testing.T) {
	v, priv := newTestVerifier(t)

	// expired
	c := validClaims()
	c["exp"] = time.Now().Add(-time.Minute).Unix()
	if _, err := v.Verify(context.Background(), mintEdDSA(t, priv, c)); err == nil {
		t.Fatal("expected expired token to be rejected")
	}

	// wrong issuer
	c = validClaims()
	c["iss"] = "https://evil.example.com"
	if _, err := v.Verify(context.Background(), mintEdDSA(t, priv, c)); err == nil {
		t.Fatal("expected issuer mismatch to be rejected")
	}

	// wrong audience
	c = validClaims()
	c["aud"] = "someone-else"
	if _, err := v.Verify(context.Background(), mintEdDSA(t, priv, c)); err == nil {
		t.Fatal("expected audience mismatch to be rejected")
	}

	// signed by a different key (bad signature)
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	if _, err := v.Verify(context.Background(), mintEdDSA(t, otherPriv, validClaims())); err == nil {
		t.Fatal("expected bad signature to be rejected")
	}

	// garbage
	if _, err := v.Verify(context.Background(), "not.a.jwt"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
}

func TestInterceptor_AttachesSubjectAndTolueratesAnon(t *testing.T) {
	v, priv := newTestVerifier(t)
	i := NewInterceptorWithVerifier(v)

	// A handler that records whatever subject the interceptor resolved.
	var gotSub string
	var gotOK bool
	handler := func(ctx context.Context, _ any) (any, error) {
		gotSub, gotOK = SubjectFrom(ctx)
		return nil, nil
	}
	call := func(ctx context.Context) {
		gotSub, gotOK = "", false
		_, _ = i.Unary()(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	}

	// valid token → subject attached
	tok := mintEdDSA(t, priv, validClaims())
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	call(ctx)
	if !gotOK || gotSub != "user-123" {
		t.Fatalf("valid token: subject=%q ok=%v, want user-123/true", gotSub, gotOK)
	}

	// no token → anonymous, handler still runs (no subject)
	call(context.Background())
	if gotOK {
		t.Fatalf("anonymous: expected no subject, got %q", gotSub)
	}

	// invalid token → NOT rejected here, just no subject
	badCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer not.a.jwt"))
	call(badCtx)
	if gotOK {
		t.Fatalf("invalid token: expected no subject, got %q", gotSub)
	}
}
