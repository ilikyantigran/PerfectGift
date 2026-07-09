// Package clients holds the OUTbound gRPC clients Surprise calls during a
// generation job: Poll (GetResponses) and Catalog (SearchInspiration). Both are
// behind interfaces with fakes so the pipeline is tested without a live network.
// The real implementations dial the downstream services using stubs generated
// from a LOCAL copy of their proto (see api/poll, api/catalog) — Surprise does not
// depend on another service's module.
package clients

import (
	"context"
	"fmt"

	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/catalog/v1"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/poll/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Poll pulls a poll's answers as generation grounding. Missing/expired poll ->
// generate without it (the pipeline degrades gracefully).
type Poll interface {
	GetResponses(ctx context.Context, pollID string) ([]string, error)
}

// Catalog pulls pgvector grounding snippets. Degrade to weaker grounding if
// unavailable.
type Catalog interface {
	SearchInspiration(ctx context.Context, queryText string, embedding []float32, budget string, topK int) ([]string, error)
}

// --- real gRPC implementations ---

type pollClient struct {
	conn *grpc.ClientConn
	c    pollv1.PollServiceClient
}

// DialPoll connects to the Poll service. Close the returned client on shutdown.
func DialPoll(addr string) (*pollClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial poll: %w", err)
	}
	return &pollClient{conn: conn, c: pollv1.NewPollServiceClient(conn)}, nil
}

func (p *pollClient) Close() error { return p.conn.Close() }

func (p *pollClient) GetResponses(ctx context.Context, pollID string) ([]string, error) {
	resp, err := p.c.GetResponses(ctx, &pollv1.GetResponsesRequest{PollId: pollID})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, r := range resp.GetResponses() {
		for _, a := range r.GetAnswers() {
			out = append(out, fmt.Sprintf("%s: %s", a.GetQuestion(), a.GetAnswer()))
		}
	}
	return out, nil
}

type catalogClient struct {
	conn *grpc.ClientConn
	c    catalogv1.CatalogServiceClient
}

// DialCatalog connects to the Catalog service. Close the returned client on shutdown.
func DialCatalog(addr string) (*catalogClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial catalog: %w", err)
	}
	return &catalogClient{conn: conn, c: catalogv1.NewCatalogServiceClient(conn)}, nil
}

func (c *catalogClient) Close() error { return c.conn.Close() }

func (c *catalogClient) SearchInspiration(ctx context.Context, queryText string, embedding []float32, budget string, topK int) ([]string, error) {
	resp, err := c.c.SearchInspiration(ctx, &catalogv1.SearchInspirationRequest{
		QueryText:      queryText,
		QueryEmbedding: embedding,
		Budget:         budget,
		TopK:           uint32(topK),
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range resp.GetSnippets() {
		out = append(out, fmt.Sprintf("%s — %s", s.GetTitle(), s.GetBody()))
	}
	return out, nil
}

// --- fakes ---

// FakePoll is a deterministic Poll for tests.
type FakePoll struct {
	Answers []string
	Err     error
}

func (f *FakePoll) GetResponses(context.Context, string) ([]string, error) {
	return f.Answers, f.Err
}

// FakeCatalog is a deterministic Catalog for tests.
type FakeCatalog struct {
	Snippets []string
	Err      error
}

func (f *FakeCatalog) SearchInspiration(context.Context, string, []float32, string, int) ([]string, error) {
	return f.Snippets, f.Err
}
