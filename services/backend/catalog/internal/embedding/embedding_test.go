package embedding

import (
	"context"
	"math"
	"testing"
)

func TestFakeEmbedIsDeterministicAndCorrectDimension(t *testing.T) {
	f := NewFake("test-model", 16)
	if f.Dimension() != 16 {
		t.Fatalf("Dimension() = %d, want 16", f.Dimension())
	}
	if f.Model() != "test-model" {
		t.Fatalf("Model() = %q, want test-model", f.Model())
	}

	v1, err := f.Embed(context.Background(), []string{"valentine's day dinner"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	v2, err := f.Embed(context.Background(), []string{"valentine's day dinner"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(v1) != 1 || len(v1[0]) != 16 {
		t.Fatalf("got %d vectors of len %d, want 1 of len 16", len(v1), len(v1[0]))
	}
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("embedding not deterministic at %d: %v vs %v", i, v1[0][i], v2[0][i])
		}
	}
}

func TestFakeEmbedProducesUnitVectors(t *testing.T) {
	f := NewFake("m", 32)
	vecs, err := f.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vectors, want 3", len(vecs))
	}
	for i, v := range vecs {
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Fatalf("vector %d norm = %v, want ~1.0", i, norm)
		}
	}
}

func TestFakeDifferentTextsDiffer(t *testing.T) {
	f := NewFake("m", 64)
	vecs, err := f.Embed(context.Background(), []string{"anniversary", "birthday"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	same := true
	for i := range vecs[0] {
		if vecs[0][i] != vecs[1][i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("distinct texts produced identical vectors")
	}
}

func TestNewSelectsFakeWhenNoEndpoint(t *testing.T) {
	e, err := New("text-embedding-3-small", 1536, "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := e.(*Fake); !ok {
		t.Fatalf("empty endpoint should yield *Fake, got %T", e)
	}
	if e.Dimension() != 1536 {
		t.Fatalf("Dimension() = %d, want 1536", e.Dimension())
	}
}

func TestNewSelectsHTTPWhenEndpointSet(t *testing.T) {
	e, err := New("m", 8, "https://example.com/v1/embeddings", "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := e.(*HTTPClient); !ok {
		t.Fatalf("non-empty endpoint should yield *HTTPClient, got %T", e)
	}
}

func TestNewRejectsBadDimension(t *testing.T) {
	if _, err := New("m", 0, "", ""); err == nil {
		t.Fatal("expected error for zero dimension")
	}
}
