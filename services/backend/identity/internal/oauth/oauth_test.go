package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestFakeVerifier(t *testing.T) {
	f := NewFakeVerifier()
	f.Register("tok-1", Identity{Provider: "google", Subject: "g-123", Email: "a@example.com", EmailVerified: true})

	got, err := f.Verify(context.Background(), "google", "tok-1")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Subject != "g-123" || got.Email != "a@example.com" {
		t.Errorf("unexpected identity: %+v", got)
	}

	if _, err := f.Verify(context.Background(), "google", "unknown"); err == nil {
		t.Error("unknown token should error")
	}
	if _, err := f.Verify(context.Background(), "apple", "tok-1"); err == nil {
		t.Error("provider mismatch should error")
	}
}

// TestProviderVerifierWithLocalJWKS exercises the real RS256 verification path
// hermetically: we stand up an in-process JWKS server and sign a token with a
// matching key, so no external network or credentials are required.
func TestProviderVerifierWithLocalJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	const kid = "test-kid-1"

	jwks := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := NewProviderVerifier([]string{"my-google-client"}, nil)
	v.GoogleJWKSURL = srv.URL
	v.GoogleIssuers = []string{"https://accounts.google.com"}

	sign := func(claims jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = kid
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return s
	}

	good := sign(jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            "my-google-client",
		"sub":            "google-sub-42",
		"email":          "user@example.com",
		"email_verified": true,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Add(-time.Minute).Unix(),
	})
	id, err := v.Verify(context.Background(), "google", good)
	if err != nil {
		t.Fatalf("Verify good token: %v", err)
	}
	if id.Subject != "google-sub-42" || id.Email != "user@example.com" {
		t.Errorf("unexpected identity: %+v", id)
	}

	// Wrong audience must be rejected.
	badAud := sign(jwt.MapClaims{
		"iss": "https://accounts.google.com",
		"aud": "someone-else",
		"sub": "x",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), "google", badAud); err == nil {
		t.Error("wrong audience should be rejected")
	}

	// Expired token must be rejected.
	expired := sign(jwt.MapClaims{
		"iss": "https://accounts.google.com",
		"aud": "my-google-client",
		"sub": "x",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	if _, err := v.Verify(context.Background(), "google", expired); err == nil {
		t.Error("expired token should be rejected")
	}
}
