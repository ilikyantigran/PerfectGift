package clients

import (
	"context"
	"testing"

	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/catalog/v1"
	"google.golang.org/grpc"
)

// stubCatalogServiceClient captures the last SearchInspirationRequest it was
// called with so tests can assert on exactly what the wire request looked like.
type stubCatalogServiceClient struct {
	gotReq *catalogv1.SearchInspirationRequest
}

func (s *stubCatalogServiceClient) SearchInspiration(_ context.Context, in *catalogv1.SearchInspirationRequest, _ ...grpc.CallOption) (*catalogv1.SearchInspirationResponse, error) {
	s.gotReq = in
	return &catalogv1.SearchInspirationResponse{}, nil
}

// Catalog's contract requires EXACTLY ONE of query_text / query_embedding, or
// it returns InvalidArgument (which the pipeline then degrades on, silently
// dropping catalog grounding). These tests pin down that we always send
// exactly one, preferring the embedding when present.
func TestCatalogClientSearchInspiration_PrefersEmbeddingWhenPresent(t *testing.T) {
	stub := &stubCatalogServiceClient{}
	c := &catalogClient{c: stub}

	embedding := []float32{0.1, 0.2, 0.3}
	if _, err := c.SearchInspiration(context.Background(), "birthday gifts for mom", embedding, "mid", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.gotReq == nil {
		t.Fatal("expected downstream request to be captured")
	}
	if stub.gotReq.GetQueryText() != "" {
		t.Errorf("expected empty QueryText when embedding is present, got %q", stub.gotReq.GetQueryText())
	}
	if len(stub.gotReq.GetQueryEmbedding()) != len(embedding) {
		t.Errorf("expected QueryEmbedding to be set, got %v", stub.gotReq.GetQueryEmbedding())
	}
}

func TestCatalogClientSearchInspiration_FallsBackToQueryTextWhenNoEmbedding(t *testing.T) {
	stub := &stubCatalogServiceClient{}
	c := &catalogClient{c: stub}

	if _, err := c.SearchInspiration(context.Background(), "birthday gifts for mom", nil, "mid", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.gotReq == nil {
		t.Fatal("expected downstream request to be captured")
	}
	if stub.gotReq.GetQueryText() != "birthday gifts for mom" {
		t.Errorf("expected QueryText to be set, got %q", stub.gotReq.GetQueryText())
	}
	if len(stub.gotReq.GetQueryEmbedding()) != 0 {
		t.Errorf("expected empty QueryEmbedding when no embedding given, got %v", stub.gotReq.GetQueryEmbedding())
	}
}

// An empty-but-non-nil embedding must be treated as "no embedding" (the check is
// len()>0, not != nil), so we fall through to the text query rather than sending
// an empty embedding.
func TestCatalogClientSearchInspiration_EmptyNonNilEmbeddingUsesText(t *testing.T) {
	stub := &stubCatalogServiceClient{}
	c := &catalogClient{c: stub}

	if _, err := c.SearchInspiration(context.Background(), "birthday gifts", []float32{}, "mid", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.gotReq.GetQueryText() != "birthday gifts" {
		t.Errorf("expected QueryText for empty non-nil embedding, got %q", stub.gotReq.GetQueryText())
	}
	if len(stub.gotReq.GetQueryEmbedding()) != 0 {
		t.Errorf("expected empty QueryEmbedding, got %v", stub.gotReq.GetQueryEmbedding())
	}
}

// With neither an embedding nor a text query, catalog would reject the request,
// so the client must skip the call entirely and return no grounding (no error).
func TestCatalogClientSearchInspiration_SkipsCallWhenNoQuerySignal(t *testing.T) {
	stub := &stubCatalogServiceClient{}
	c := &catalogClient{c: stub}

	out, err := c.SearchInspiration(context.Background(), "", nil, "mid", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil snippets when no query signal, got %v", out)
	}
	if stub.gotReq != nil {
		t.Error("expected NO downstream catalog call when both query_text and embedding are empty")
	}
}
