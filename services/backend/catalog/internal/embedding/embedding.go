// Package embedding computes vector embeddings for the curated corpus and for
// SearchInspiration query text. The embedding model + dimension are the pinned
// contract with the Surprise service: both sides must embed with the same model
// into the same vector space, or similarity search is meaningless.
//
// The computation is hidden behind the Embedder interface so the rest of the
// service (and its tests) never depends on a live embedding API. A deterministic
// Fake is used in tests and for local runs with no endpoint configured; a real
// HTTP client (OpenAI-compatible) is used when an endpoint is set.
package embedding

import (
	"context"
	"fmt"
)

// Embedder turns text into fixed-dimension vectors. Model and Dimension pin the
// embedding space; callers validate that any caller-supplied embedding matches
// Dimension before searching.
type Embedder interface {
	// Embed returns one vector per input text, each of length Dimension().
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Model is the pinned model identifier (recorded/compared across services).
	Model() string
	// Dimension is the pinned vector length.
	Dimension() int
}

// New selects the embedder from config. An empty endpoint yields the deterministic
// Fake (so the service boots and can be exercised without an external API); a
// non-empty endpoint yields the real HTTP client. apiKey is the resolved secret
// value (read from the environment by the caller), not an env var name.
func New(model string, dimension int, endpoint, apiKey string) (Embedder, error) {
	if dimension <= 0 {
		return nil, fmt.Errorf("embedding dimension must be positive, got %d", dimension)
	}
	if model == "" {
		return nil, fmt.Errorf("embedding model must be set")
	}
	if endpoint == "" {
		return NewFake(model, dimension), nil
	}
	return NewHTTPClient(endpoint, model, apiKey, dimension), nil
}
