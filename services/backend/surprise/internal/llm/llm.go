// Package llm is the boundary to the generation model. The Client interface is
// what the pipeline depends on; the real implementation talks to Anthropic Claude
// (anthropic.go), and the fake (FakeClient) is deterministic so the whole suite
// runs without a network or an API key. A resilient decorator wraps any Client
// with retry + circuit breaker.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/resilience"
)

// Tier selects the model. Sonnet is the default generation tier, Opus the
// premium/"deep" tier; Moderation maps to Haiku for the cheap safety pass.
type Tier string

const (
	TierSonnet     Tier = "sonnet"
	TierOpus       Tier = "opus"
	TierModeration Tier = "moderation"
)

// GenerateParams is the structured prompt input assembled by the pipeline.
type GenerateParams struct {
	Holiday     string
	BudgetBand  string
	Preferences string
	PollAnswers []string // grounding from Poll.GetResponses
	Inspiration []string // grounding snippets from Catalog.SearchInspiration
	Refinement  string   // set on Refine
	N           int      // number of candidate ideas wanted
	Tier        Tier
}

// Idea is a single typed idea returned by tool-use structured output.
type Idea struct {
	Title     string `json:"title"`
	WhyItFits string `json:"why_it_fits"`
	RoughCost string `json:"rough_cost"`
	HowTo     string `json:"how_to"`
}

// Client is the generation/moderation/embedding boundary.
type Client interface {
	// GenerateIdeas returns N candidate ideas as typed objects (tool use).
	GenerateIdeas(ctx context.Context, p GenerateParams) ([]Idea, error)
	// Moderate returns true if the text is wholesome/safe (Haiku pass).
	Moderate(ctx context.Context, text string) (bool, error)
	// Embed returns the embedding vector for text (dedup/similarity).
	Embed(ctx context.Context, text string) ([]float32, error)
}

// FakeClient is a deterministic Client for tests. Fields let a test inject
// failures or custom behavior.
type FakeClient struct {
	// GenerateFunc overrides GenerateIdeas when set.
	GenerateFunc func(ctx context.Context, p GenerateParams) ([]Idea, error)
	// ModerateFunc overrides Moderate when set. Default: approve unless the text
	// contains "unsafe".
	ModerateFunc func(ctx context.Context, text string) (bool, error)
	// Dim is the embedding dimension (default 8).
	Dim int

	Calls int // number of GenerateIdeas invocations (visibility for tests)
}

// GenerateIdeas returns N deterministic ideas derived from the params.
func (f *FakeClient) GenerateIdeas(ctx context.Context, p GenerateParams) ([]Idea, error) {
	f.Calls++
	if f.GenerateFunc != nil {
		return f.GenerateFunc(ctx, p)
	}
	n := p.N
	if n <= 0 {
		n = 3
	}
	out := make([]Idea, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Idea{
			Title:     fmt.Sprintf("%s idea %d", p.Holiday, i+1),
			WhyItFits: fmt.Sprintf("Fits %s within %s: %s", p.Holiday, p.BudgetBand, p.Preferences),
			RoughCost: p.BudgetBand,
			HowTo:     fmt.Sprintf("Step-by-step plan #%d", i+1),
		})
	}
	return out, nil
}

// Moderate approves text unless it contains the marker "unsafe".
func (f *FakeClient) Moderate(ctx context.Context, text string) (bool, error) {
	if f.ModerateFunc != nil {
		return f.ModerateFunc(ctx, text)
	}
	return !strings.Contains(strings.ToLower(text), "unsafe"), nil
}

// Embed returns a deterministic vector.
func (f *FakeClient) Embed(_ context.Context, text string) ([]float32, error) {
	dim := f.Dim
	if dim == 0 {
		dim = 8
	}
	v := make([]float32, dim)
	for i, r := range text {
		v[i%dim] += float32(r)
	}
	return v, nil
}

// Resilient wraps a Client with retry + circuit breaker around GenerateIdeas and
// Moderate (the paid, third-party calls). Embed is passed through.
type Resilient struct {
	inner   Client
	breaker *resilience.Breaker
	retry   resilience.RetryConfig
}

// NewResilient builds the decorator.
func NewResilient(inner Client, breaker *resilience.Breaker, retry resilience.RetryConfig) *Resilient {
	return &Resilient{inner: inner, breaker: breaker, retry: retry}
}

func (r *Resilient) GenerateIdeas(ctx context.Context, p GenerateParams) ([]Idea, error) {
	var out []Idea
	err := resilience.Retry(ctx, r.retry, func() error {
		return r.breaker.Do(func() error {
			var e error
			out, e = r.inner.GenerateIdeas(ctx, p)
			return e
		})
	})
	return out, err
}

func (r *Resilient) Moderate(ctx context.Context, text string) (bool, error) {
	var ok bool
	err := resilience.Retry(ctx, r.retry, func() error {
		return r.breaker.Do(func() error {
			var e error
			ok, e = r.inner.Moderate(ctx, text)
			return e
		})
	})
	return ok, err
}

func (r *Resilient) Embed(ctx context.Context, text string) ([]float32, error) {
	return r.inner.Embed(ctx, text)
}
