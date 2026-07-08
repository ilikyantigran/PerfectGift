package embedding

import (
	"context"
	"hash/fnv"
	"math"
	"math/rand"
)

// Fake is a deterministic, offline embedder. It maps text to a stable unit vector
// of the configured dimension by seeding a PRNG from a hash of the text. The same
// text always yields the same vector; different texts almost always differ. It has
// no semantic meaning — it exists so the service and its tests never need a live
// embedding API — but it is a valid vector space, so cosine similarity behaves
// (identical text scores 1.0), which is enough to exercise the pgvector path.
type Fake struct {
	model string
	dim   int
}

// NewFake returns a deterministic fake embedder producing dim-length unit vectors.
func NewFake(model string, dim int) *Fake {
	return &Fake{model: model, dim: dim}
}

func (f *Fake) Model() string  { return f.model }
func (f *Fake) Dimension() int { return f.dim }

// Embed returns one deterministic unit vector per input text.
func (f *Fake) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = f.vector(t)
	}
	return out, nil
}

func (f *Fake) vector(text string) []float32 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	r := rand.New(rand.NewSource(int64(h.Sum64()))) //nolint:gosec // deterministic, non-cryptographic by design

	v := make([]float32, f.dim)
	var norm float64
	for i := range v {
		x := r.NormFloat64()
		v[i] = float32(x)
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		norm = 1
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}
