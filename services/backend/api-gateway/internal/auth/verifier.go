// Package auth verifies Identity-issued JWT access tokens LOCALLY, against a JWKS
// public key set — no per-request call to Identity on the hot path (SERVICE.md §4).
//
// The key set is fetched from Identity's JWKS URL and cached; on a fetch failure the
// last-known good key set is served (fail-soft on JWKS), while an unverifiable or
// expired token is always rejected (fail-closed on auth, per §7). Verification is
// implemented in-house (crypto/rsa + crypto/ed25519) so it has no external JWT
// dependency and the whole thing is unit-testable offline.
package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// Claims is the validated subset of JWT claims the gateway cares about.
type Claims struct {
	Subject string
	Issuer  string
	JTI     string
}

// JWK / JWKS model the public JSON Web Key Set published by Identity.
type JWK struct {
	Kty string `json:"kty"`           // RSA | OKP
	Kid string `json:"kid"`           // key id (matches the token header `kid`)
	Alg string `json:"alg,omitempty"` // RS256 | EdDSA
	Use string `json:"use,omitempty"`
	N   string `json:"n,omitempty"` // RSA modulus (base64url)
	E   string `json:"e,omitempty"` // RSA exponent (base64url)
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"` // Ed25519 public key (base64url)
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

// Source loads a JWKS. HTTPSource fetches from Identity's JWKS URL; StaticKeySet is
// used in tests. A Source that returns an error is tolerated by the Verifier as long
// as a previously-fetched key set is still cached.
type Source interface {
	Fetch(ctx context.Context) (*JWKS, error)
}

// StaticKeySet is a Source that always returns the same in-memory JWKS (tests/dev).
func StaticKeySet(j *JWKS) Source { return staticSource{j} }

type staticSource struct{ j *JWKS }

func (s staticSource) Fetch(context.Context) (*JWKS, error) { return s.j, nil }

// Config configures a Verifier.
type Config struct {
	Issuer     string        // expected `iss`
	Audience   string        // expected `aud`
	Source     Source        // where the JWKS comes from
	RefreshTTL time.Duration // how long a fetched JWKS is trusted before re-fetch (default 10m)
}

// Verifier validates access tokens against a cached JWKS.
type Verifier struct {
	issuer   string
	audience string
	source   Source
	ttl      time.Duration

	mu        sync.Mutex
	keys      map[string]crypto.PublicKey
	fetchedAt time.Time
	haveKeys  bool
}

// New builds a Verifier. The initial JWKS is fetched lazily on the first Verify.
func New(cfg Config) (*Verifier, error) {
	if cfg.Source == nil {
		return nil, errors.New("auth: nil JWKS source")
	}
	ttl := cfg.RefreshTTL
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	return &Verifier{
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		source:   cfg.Source,
		ttl:      ttl,
		keys:     map[string]crypto.PublicKey{},
	}, nil
}

var (
	ErrMalformedToken = errors.New("auth: malformed token")
	ErrUnknownKey     = errors.New("auth: unknown signing key")
	ErrBadSignature   = errors.New("auth: signature verification failed")
	ErrExpired        = errors.New("auth: token expired")
	ErrClaims         = errors.New("auth: claim validation failed")
	ErrNoKeys         = errors.New("auth: no signing keys available")
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// Verify parses, checks the signature against the JWKS, and validates the standard
// claims. It returns validated Claims or an error. Any error means "reject".
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrMalformedToken
	}

	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformedToken
	}
	var hdr jwtHeader
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, ErrMalformedToken
	}

	key, err := v.keyFor(ctx, hdr.Kid)
	if err != nil {
		return nil, err
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrMalformedToken
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(hdr.Alg, key, signingInput, sig); err != nil {
		return nil, err
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformedToken
	}
	return v.validateClaims(payload)
}

// keyFor returns the public key for kid, refreshing the JWKS if the cache is stale
// or the kid is unknown. On a refresh failure it keeps serving the last-known keys.
func (v *Verifier) keyFor(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	stale := !v.haveKeys || time.Since(v.fetchedAt) >= v.ttl
	_, known := v.keys[kid]
	if stale || !known {
		if err := v.refreshLocked(ctx); err != nil && !v.haveKeys {
			// No cached keys and fetch failed → fail closed.
			return nil, fmt.Errorf("%w: %v", ErrNoKeys, err)
		}
		// If fetch failed but we still have cached keys, fall through to them.
	}
	k, ok := v.keys[kid]
	if !ok {
		return nil, ErrUnknownKey
	}
	return k, nil
}

func (v *Verifier) refreshLocked(ctx context.Context) error {
	jwks, err := v.source.Fetch(ctx)
	if err != nil {
		return err
	}
	parsed := map[string]crypto.PublicKey{}
	for _, jwk := range jwks.Keys {
		pk, err := jwk.publicKey()
		if err != nil {
			continue // skip keys we can't parse rather than failing the whole set
		}
		parsed[jwk.Kid] = pk
	}
	if len(parsed) == 0 {
		return errors.New("auth: JWKS contained no usable keys")
	}
	v.keys = parsed
	v.fetchedAt = time.Now()
	v.haveKeys = true
	return nil
}

func (v *Verifier) validateClaims(payload []byte) (*Claims, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, ErrMalformedToken
	}

	now := time.Now()
	if exp, ok := numericClaim(raw, "exp"); ok {
		if now.After(time.Unix(exp, 0)) {
			return nil, ErrExpired
		}
	} else {
		return nil, fmt.Errorf("%w: missing exp", ErrClaims)
	}
	if nbf, ok := numericClaim(raw, "nbf"); ok {
		if now.Add(30 * time.Second).Before(time.Unix(nbf, 0)) {
			return nil, fmt.Errorf("%w: token not yet valid", ErrClaims)
		}
	}
	if v.issuer != "" {
		if iss := stringClaim(raw, "iss"); iss != v.issuer {
			return nil, fmt.Errorf("%w: issuer mismatch", ErrClaims)
		}
	}
	if v.audience != "" {
		if !audienceContains(raw, v.audience) {
			return nil, fmt.Errorf("%w: audience mismatch", ErrClaims)
		}
	}
	sub := stringClaim(raw, "sub")
	if sub == "" {
		return nil, fmt.Errorf("%w: missing sub", ErrClaims)
	}
	return &Claims{Subject: sub, Issuer: stringClaim(raw, "iss"), JTI: stringClaim(raw, "jti")}, nil
}

// --- signature verification ---

func verifySignature(alg string, key crypto.PublicKey, signingInput, sig []byte) error {
	switch alg {
	case "RS256":
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return ErrBadSignature
		}
		h := sha256.Sum256(signingInput)
		if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, h[:], sig); err != nil {
			return ErrBadSignature
		}
		return nil
	case "EdDSA":
		edKey, ok := key.(ed25519.PublicKey)
		if !ok {
			return ErrBadSignature
		}
		if !ed25519.Verify(edKey, signingInput, sig) {
			return ErrBadSignature
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported alg %q", ErrBadSignature, alg)
	}
}

// rsaSign is a test helper (used from verifier_test.go) to sign with RS256.
func rsaSign(key *rsa.PrivateKey, signingInput []byte) ([]byte, error) {
	h := sha256.Sum256(signingInput)
	return rsa.SignPKCS1v15(nil, key, crypto.SHA256, h[:])
}

// --- JWK → public key ---

func (j JWK) publicKey() (crypto.PublicKey, error) {
	switch j.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(j.N, "="))
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(j.E, "="))
		if err != nil {
			return nil, err
		}
		n := new(big.Int).SetBytes(nBytes)
		var e int
		switch len(eBytes) {
		case 0:
			return nil, errors.New("auth: empty RSA exponent")
		default:
			padded := make([]byte, 8)
			copy(padded[8-len(eBytes):], eBytes)
			e = int(binary.BigEndian.Uint64(padded))
		}
		return &rsa.PublicKey{N: n, E: e}, nil
	case "OKP":
		if j.Crv != "Ed25519" {
			return nil, fmt.Errorf("auth: unsupported OKP curve %q", j.Crv)
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(j.X, "="))
		if err != nil {
			return nil, err
		}
		if len(xBytes) != ed25519.PublicKeySize {
			return nil, errors.New("auth: bad Ed25519 key size")
		}
		return ed25519.PublicKey(xBytes), nil
	default:
		return nil, fmt.Errorf("auth: unsupported kty %q", j.Kty)
	}
}

// --- claim helpers ---

func numericClaim(raw map[string]json.RawMessage, k string) (int64, bool) {
	v, ok := raw[k]
	if !ok {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(v, &f); err != nil {
		return 0, false
	}
	return int64(f), true
}

func stringClaim(raw map[string]json.RawMessage, k string) string {
	v, ok := raw[k]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}

// audienceContains supports both string and []string `aud` per RFC 7519.
func audienceContains(raw map[string]json.RawMessage, want string) bool {
	v, ok := raw["aud"]
	if !ok {
		return false
	}
	var single string
	if err := json.Unmarshal(v, &single); err == nil {
		return single == want
	}
	var many []string
	if err := json.Unmarshal(v, &many); err == nil {
		for _, a := range many {
			if a == want {
				return true
			}
		}
	}
	return false
}
