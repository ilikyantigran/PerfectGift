package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// --- test helpers: build a JWKS + sign RS256 tokens with a local key ---

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func jwksFor(kid string, pub *rsa.PublicKey) *JWKS {
	eBytes := []byte{byte(pub.E >> 16), byte(pub.E >> 8), byte(pub.E)}
	// trim leading zero bytes
	for len(eBytes) > 1 && eBytes[0] == 0 {
		eBytes = eBytes[1:]
	}
	return &JWKS{Keys: []JWK{{
		Kty: "RSA",
		Kid: kid,
		Alg: "RS256",
		Use: "sig",
		N:   b64u(pub.N.Bytes()),
		E:   b64u(eBytes),
	}}}
}

func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	hdr := map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(hdr)
	cb, _ := json.Marshal(claims)
	signingInput := b64u(hb) + "." + b64u(cb)
	sig, err := rsaSign(key, []byte(signingInput))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signingInput + "." + b64u(sig)
}

func newVerifier(t *testing.T, jwks *JWKS) *Verifier {
	t.Helper()
	v, err := New(Config{
		Issuer:   "https://identity.perfectgift.test",
		Audience: "perfectgift",
		Source:   StaticKeySet(jwks),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func baseClaims() map[string]any {
	return map[string]any{
		"sub": "user-123",
		"iss": "https://identity.perfectgift.test",
		"aud": "perfectgift",
		"exp": time.Now().Add(15 * time.Minute).Unix(),
		"iat": time.Now().Add(-1 * time.Minute).Unix(),
		"jti": "jti-1",
	}
}

func TestVerify_ValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	tok := signRS256(t, key, "k1", baseClaims())

	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("subject = %q, want user-123", claims.Subject)
	}
}

func TestVerify_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	c := baseClaims()
	c["exp"] = time.Now().Add(-1 * time.Minute).Unix()
	tok := signRS256(t, key, "k1", c)

	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	c := baseClaims()
	c["iss"] = "https://evil.test"
	tok := signRS256(t, key, "k1", c)

	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected wrong issuer to fail")
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	c := baseClaims()
	c["aud"] = "someone-else"
	tok := signRS256(t, key, "k1", c)

	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected wrong audience to fail")
	}
}

func TestVerify_UnknownKid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	tok := signRS256(t, key, "unknown-kid", baseClaims())

	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected unknown kid to fail")
	}
}

func TestVerify_BadSignature(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	// JWKS advertises `key`, but the token is signed with `other`.
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	tok := signRS256(t, other, "k1", baseClaims())

	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected bad signature to fail")
	}
}

func TestVerify_Malformed(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newVerifier(t, jwksFor("k1", &key.PublicKey))
	for _, tok := range []string{"", "abc", "a.b", "a.b.c.d"} {
		if _, err := v.Verify(context.Background(), tok); err == nil {
			t.Fatalf("expected malformed token %q to fail", tok)
		}
	}
}

// A fetch source that fails after the first success must fall back to the
// last-known good key set (fail-soft on JWKS fetch, per SERVICE.md §4).
type flakySource struct {
	jwks  *JWKS
	calls int
}

func (f *flakySource) Fetch(context.Context) (*JWKS, error) {
	f.calls++
	if f.calls == 1 {
		return f.jwks, nil
	}
	return nil, context.DeadlineExceeded
}

func TestVerify_FallsBackToLastKnownJWKS(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	src := &flakySource{jwks: jwksFor("k1", &key.PublicKey)}
	v, err := New(Config{
		Issuer:      "https://identity.perfectgift.test",
		Audience:    "perfectgift",
		Source:      src,
		RefreshTTL:  0, // force a refresh attempt on every verify
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tok := signRS256(t, key, "k1", baseClaims())

	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatalf("first verify (fetch ok): %v", err)
	}
	// Second verify: fetch fails, but the cached key must still verify the token.
	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatalf("second verify should use last-known JWKS, got %v", err)
	}
}
