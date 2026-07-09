// Package token issues and verifies the service's access tokens and publishes
// the JWK Set used by every other service to verify them locally.
//
// Access tokens are JWTs signed with EdDSA (Ed25519) — allowed by the contract,
// with tiny keys, instant generation, and a standard JWK "OKP" representation
// (RFC 8037). The Manager keeps a current and a previous key so verifiers
// tolerate rotation without downtime: after Rotate() the old key is still
// published and still verifies, until the next rotation retires it.
package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the decoded, verified content of an access token.
type Claims struct {
	Subject   string
	SessionID string
	Issuer    string
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	JTI       string
}

// JWK is one public key in JWK form (RFC 7517 / RFC 8037 for OKP/Ed25519).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
}

type keyPair struct {
	kid  string
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// Manager owns the signing keys and the issue/verify/rotate logic. It is safe
// for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	current  *keyPair
	previous *keyPair

	issuer   string
	audience string
	ttl      time.Duration

	// now is the clock, swappable in tests.
	now func() time.Time
}

// NewManager creates a Manager with a freshly generated current key.
func NewManager(issuer, audience string, ttl time.Duration) (*Manager, error) {
	kp, err := newKeyPair()
	if err != nil {
		return nil, err
	}
	return &Manager{
		current:  kp,
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
		now:      time.Now,
	}, nil
}

func newKeyPair() (*keyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	sum := sha256.Sum256(pub)
	kid := base64.RawURLEncoding.EncodeToString(sum[:16])
	return &keyPair{kid: kid, priv: priv, pub: pub}, nil
}

type jwtClaims struct {
	Sid string `json:"sid"`
	jwt.RegisteredClaims
}

// Issue signs a new access token for subject bound to sessionID. It returns the
// token and its lifetime in seconds.
func (m *Manager) Issue(subject, sessionID string) (string, int, error) {
	m.mu.RLock()
	kp := m.current
	m.mu.RUnlock()

	now := m.now()
	exp := now.Add(m.ttl)

	jti, err := randomID()
	if err != nil {
		return "", 0, err
	}

	claims := jwtClaims{
		Sid: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kp.kid
	signed, err := tok.SignedString(kp.priv)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}
	return signed, int(m.ttl.Seconds()), nil
}

// Verify checks the token's signature (against the current or previous key,
// selected by the `kid` header), algorithm, issuer, audience, and expiry.
func (m *Manager) Verify(raw string) (*Claims, error) {
	var claims jwtClaims
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		pub := m.publicKeyByKid(kid)
		if pub == nil {
			return nil, errors.New("unknown key id")
		}
		return pub, nil
	},
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}

	out := &Claims{
		Subject:   claims.Subject,
		SessionID: claims.Sid,
		Issuer:    claims.Issuer,
		JTI:       claims.ID,
	}
	if len(claims.Audience) > 0 {
		out.Audience = claims.Audience[0]
	}
	if claims.IssuedAt != nil {
		out.IssuedAt = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		out.ExpiresAt = claims.ExpiresAt.Time
	}
	return out, nil
}

func (m *Manager) publicKeyByKid(kid string) ed25519.PublicKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current != nil && m.current.kid == kid {
		return m.current.pub
	}
	if m.previous != nil && m.previous.kid == kid {
		return m.previous.pub
	}
	return nil
}

// JWKS returns the public keys to publish: current first, then previous.
func (m *Manager) JWKS() []JWK {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []JWK{jwkFor(m.current)}
	if m.previous != nil {
		out = append(out, jwkFor(m.previous))
	}
	return out
}

func jwkFor(kp *keyPair) JWK {
	return JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(kp.pub),
		Kid: kp.kid,
		Use: "sig",
		Alg: "EdDSA",
	}
}

// Rotate mints a new current key and demotes the old current to previous,
// retiring whatever was previous. Verifiers keep working across the change
// because both keys are published in the JWKS.
func (m *Manager) Rotate() error {
	kp, err := newKeyPair()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.previous = m.current
	m.current = kp
	m.mu.Unlock()
	return nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
