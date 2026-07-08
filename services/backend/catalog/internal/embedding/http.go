package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient is an OpenAI-compatible embeddings client. It POSTs {model, input}
// to <endpoint> and expects {data: [{embedding: [...]}]} back. The API key is sent
// as a Bearer token when present. Dimension is validated against each returned
// vector so a model/config mismatch fails loudly instead of corrupting the corpus.
type HTTPClient struct {
	endpoint string
	model    string
	apiKey   string
	dim      int
	http     *http.Client
}

// NewHTTPClient builds a real embedding client. endpoint is the full URL of the
// embeddings API (e.g. https://api.openai.com/v1/embeddings).
func NewHTTPClient(endpoint, model, apiKey string, dim int) *HTTPClient {
	return &HTTPClient{
		endpoint: endpoint,
		model:    model,
		apiKey:   apiKey,
		dim:      dim,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *HTTPClient) Model() string  { return c.model }
func (c *HTTPClient) Dimension() int { return c.dim }

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed calls the embeddings endpoint once for the whole batch.
func (c *HTTPClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embed endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embed endpoint status %d: %s", resp.StatusCode, string(snippet))
	}

	var parsed embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embed endpoint returned %d vectors for %d inputs", len(parsed.Data), len(texts))
	}

	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		if len(d.Embedding) != c.dim {
			return nil, fmt.Errorf("embed vector %d has dimension %d, expected %d (model/config mismatch)", i, len(d.Embedding), c.dim)
		}
		out[i] = d.Embedding
	}
	return out, nil
}
