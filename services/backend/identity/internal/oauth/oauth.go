// Package oauth verifies Apple and Google ID tokens behind a small interface so
// the sign-in logic can be tested without live external credentials or network:
// production uses ProviderVerifier (fetches the provider's JWKS over HTTPS),
// tests use FakeVerifier.
package oauth

import (
	"context"
	"fmt"
)

// Identity is the external identity proven by a verified provider ID token.
type Identity struct {
	Provider      string // "google" | "apple"
	Subject       string // stable per-provider user id (the token's `sub`)
	Email         string // may be empty (some Apple relay flows omit it)
	EmailVerified bool
}

// Verifier verifies a provider ID token and returns the external identity.
type Verifier interface {
	Verify(ctx context.Context, provider, idToken string) (*Identity, error)
}

// FakeVerifier is an in-memory Verifier for tests. Register the tokens you want
// to be considered valid; anything else fails verification.
type FakeVerifier struct {
	tokens map[string]Identity
}

func NewFakeVerifier() *FakeVerifier {
	return &FakeVerifier{tokens: map[string]Identity{}}
}

// Register makes idToken verify to id.
func (f *FakeVerifier) Register(idToken string, id Identity) {
	f.tokens[idToken] = id
}

func (f *FakeVerifier) Verify(_ context.Context, provider, idToken string) (*Identity, error) {
	id, ok := f.tokens[idToken]
	if !ok {
		return nil, fmt.Errorf("oauth: token not recognized")
	}
	if id.Provider != provider {
		return nil, fmt.Errorf("oauth: token was issued for provider %q, not %q", id.Provider, provider)
	}
	out := id
	return &out, nil
}
