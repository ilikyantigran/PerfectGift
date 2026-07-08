package token

import (
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager("https://issuer.test", "aud.test", 15*time.Minute)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	m := newTestManager(t)
	tok, expiresIn, err := m.Issue("user-123", "sess-abc")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if expiresIn != 900 {
		t.Fatalf("expiresIn = %d, want 900", expiresIn)
	}
	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("subject = %q, want user-123", claims.Subject)
	}
	if claims.SessionID != "sess-abc" {
		t.Errorf("sid = %q, want sess-abc", claims.SessionID)
	}
	if claims.Issuer != "https://issuer.test" {
		t.Errorf("issuer = %q", claims.Issuer)
	}
	if claims.JTI == "" {
		t.Error("jti empty")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	m := newTestManager(t)
	// Issue at a fixed past time so the token is already expired now.
	past := time.Now().Add(-time.Hour)
	m.now = func() time.Time { return past }
	tok, _, err := m.Issue("u", "s")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	m.now = time.Now // back to real time -> token from an hour ago is expired
	if _, err := m.Verify(tok); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	m := newTestManager(t)
	tok, _, _ := m.Issue("u", "s")
	// Flip the last character of the signature.
	b := []byte(tok)
	if b[len(b)-1] == 'a' {
		b[len(b)-1] = 'b'
	} else {
		b[len(b)-1] = 'a'
	}
	if _, err := m.Verify(string(b)); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestVerifyRejectsForeignKey(t *testing.T) {
	m1 := newTestManager(t)
	m2 := newTestManager(t)
	tok, _, _ := m1.Issue("u", "s")
	if _, err := m2.Verify(tok); err == nil {
		t.Fatal("token signed by another manager's key must not verify")
	}
}

func TestJWKSContainsCurrent(t *testing.T) {
	m := newTestManager(t)
	keys := m.JWKS()
	if len(keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(keys))
	}
	k := keys[0]
	if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Alg != "EdDSA" || k.Use != "sig" {
		t.Errorf("unexpected JWK shape: %+v", k)
	}
	if k.X == "" || k.Kid == "" {
		t.Errorf("JWK missing x/kid: %+v", k)
	}
}

func TestRotateKeepsPreviousKeyValid(t *testing.T) {
	m := newTestManager(t)
	old, _, _ := m.Issue("u", "s")
	if err := m.Rotate(); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	// Old token, signed by the now-previous key, must still verify.
	if _, err := m.Verify(old); err != nil {
		t.Fatalf("token from before rotation should still verify: %v", err)
	}
	// JWKS now advertises two keys with distinct kids.
	keys := m.JWKS()
	if len(keys) != 2 {
		t.Fatalf("want 2 keys after rotation, got %d", len(keys))
	}
	if keys[0].Kid == keys[1].Kid {
		t.Fatal("current and previous kids must differ")
	}
	// A second rotation drops the oldest key -> the original token no longer verifies.
	if err := m.Rotate(); err != nil {
		t.Fatalf("Rotate 2: %v", err)
	}
	if _, err := m.Verify(old); err == nil {
		t.Fatal("token should be invalid after two rotations")
	}
}

func TestVerifyChecksAudience(t *testing.T) {
	m, _ := NewManager("iss", "right-aud", time.Minute)
	tok, _, _ := m.Issue("u", "s")
	// A manager expecting a different audience but sharing the key would reject;
	// simulate by verifying against a manager with a different expected audience
	// but the same key set.
	other := &Manager{current: m.current, issuer: "iss", audience: "wrong-aud", ttl: time.Minute, now: time.Now}
	if _, err := other.Verify(tok); err == nil {
		t.Fatal("token with wrong audience must be rejected")
	}
}
