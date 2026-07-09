package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Default provider JWKS endpoints and issuers.
const (
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	appleJWKSURL  = "https://appleid.apple.com/auth/keys"
)

// ProviderVerifier verifies Apple/Google ID tokens against each provider's
// published JWKS. Keys are cached in-memory with a short TTL. The JWKS URLs and
// accepted issuers are fields so tests can point them at an in-process server.
type ProviderVerifier struct {
	GoogleClientIDs []string
	AppleClientIDs  []string

	GoogleJWKSURL string
	AppleJWKSURL  string
	GoogleIssuers []string
	AppleIssuers  []string

	httpClient *http.Client
	cacheTTL   time.Duration

	mu    sync.Mutex
	cache map[string]*keyCacheEntry
}

type keyCacheEntry struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewProviderVerifier builds a verifier with production defaults. Pass the
// accepted client IDs (audiences) for each provider; either may be nil if that
// provider is unused.
func NewProviderVerifier(googleClientIDs, appleClientIDs []string) *ProviderVerifier {
	return &ProviderVerifier{
		GoogleClientIDs: googleClientIDs,
		AppleClientIDs:  appleClientIDs,
		GoogleJWKSURL:   googleJWKSURL,
		AppleJWKSURL:    appleJWKSURL,
		GoogleIssuers:   []string{"https://accounts.google.com", "accounts.google.com"},
		AppleIssuers:    []string{"https://appleid.apple.com"},
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		cacheTTL:        time.Hour,
		cache:           map[string]*keyCacheEntry{},
	}
}

func (v *ProviderVerifier) Verify(ctx context.Context, provider, idToken string) (*Identity, error) {
	var jwksURL string
	var audiences, issuers []string
	switch provider {
	case "google":
		jwksURL, audiences, issuers = v.GoogleJWKSURL, v.GoogleClientIDs, v.GoogleIssuers
	case "apple":
		jwksURL, audiences, issuers = v.AppleJWKSURL, v.AppleClientIDs, v.AppleIssuers
	default:
		return nil, fmt.Errorf("oauth: unsupported provider %q", provider)
	}

	keys, err := v.keysFor(ctx, jwksURL)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id %q", kid)
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(pick(audiences)),
	)
	if err != nil {
		return nil, fmt.Errorf("oauth: verify %s token: %w", provider, err)
	}

	// Audience: accept if any configured client id matches.
	if !audienceMatches(claims, audiences) {
		return nil, fmt.Errorf("oauth: token audience not accepted")
	}
	// Issuer: must be one of the provider's issuers.
	if iss, _ := claims["iss"].(string); !contains(issuers, iss) {
		return nil, fmt.Errorf("oauth: unexpected issuer %q", iss)
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("oauth: token missing subject")
	}
	email, _ := claims["email"].(string)
	verified := false
	switch ev := claims["email_verified"].(type) {
	case bool:
		verified = ev
	case string:
		verified = ev == "true"
	}

	return &Identity{Provider: provider, Subject: sub, Email: email, EmailVerified: verified}, nil
}

// pick returns the first audience (used only to satisfy the parser's single-
// audience option; the authoritative multi-audience check is audienceMatches).
func pick(auds []string) string {
	if len(auds) > 0 {
		return auds[0]
	}
	return ""
}

func audienceMatches(claims jwt.MapClaims, accepted []string) bool {
	aud, err := claims.GetAudience()
	if err != nil {
		return false
	}
	for _, a := range aud {
		if contains(accepted, a) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (v *ProviderVerifier) keysFor(ctx context.Context, jwksURL string) (map[string]*rsa.PublicKey, error) {
	v.mu.Lock()
	entry, ok := v.cache[jwksURL]
	v.mu.Unlock()
	if ok && time.Since(entry.fetchedAt) < v.cacheTTL {
		return entry.keys, nil
	}

	keys, err := v.fetchJWKS(ctx, jwksURL)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.cache[jwksURL] = &keyCacheEntry{keys: keys, fetchedAt: time.Now()}
	v.mu.Unlock()
	return keys, nil
}

type jwksDoc struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *ProviderVerifier) fetchJWKS(ctx context.Context, jwksURL string) (map[string]*rsa.PublicKey, error) {
	client := v.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth: jwks endpoint returned %d", resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("oauth: decode jwks: %w", err)
	}

	out := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k.N, k.E)
		if err != nil {
			return nil, err
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("oauth: jwks had no usable RSA keys")
	}
	return out, nil
}

// jwkToRSAPublicKey reconstructs an RSA public key from base64url-encoded modulus
// and exponent (the JWK `n` and `e` fields).
func jwkToRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("oauth: bad jwk modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("oauth: bad jwk exponent: %w", err)
	}
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(e.Int64()),
	}, nil
}
