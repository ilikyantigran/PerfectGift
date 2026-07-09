package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpSource fetches a JWKS document over HTTP from Identity's JWKS endpoint. It is
// used as the Verifier's Source in production; the Verifier caches the result and
// tolerates transient fetch failures by serving the last-known key set.
type httpSource struct {
	url    string
	client *http.Client
}

// NewHTTPSource returns a Source that GETs the JWKS JSON from url.
func NewHTTPSource(url string, client *http.Client) Source {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &httpSource{url: url, client: client}
}

func (s *httpSource) Fetch(ctx context.Context) (*JWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS fetch %s: status %d", s.url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("auth: decode JWKS: %w", err)
	}
	return &jwks, nil
}
