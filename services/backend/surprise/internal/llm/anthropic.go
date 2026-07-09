package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicConfig configures the real Claude client.
type AnthropicConfig struct {
	BaseURL        string
	APIKey         string
	Version        string // anthropic-version header, e.g. "2023-06-01"
	SonnetModel    string // claude-sonnet-5
	OpusModel      string // claude-opus-4-8
	HaikuModel     string // claude-haiku-4-5
	EmbeddingModel string
	EmbeddingDim   int
	MaxTokens      int
	Timeout        time.Duration
}

// AnthropicClient talks to the Anthropic Messages API over raw HTTP. Structured
// idea output is obtained via forced tool use, so ideas come back as typed JSON
// objects rather than prose to parse. This client is only ever invoked inside the
// worker (never on the request path) and is wrapped by Resilient.
type AnthropicClient struct {
	cfg  AnthropicConfig
	http *http.Client
}

// NewAnthropicClient builds the client. Model IDs and dimension default to the
// architecture's pinned values if unset.
func NewAnthropicClient(cfg AnthropicConfig) *AnthropicClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Version == "" {
		cfg.Version = "2023-06-01"
	}
	if cfg.SonnetModel == "" {
		cfg.SonnetModel = "claude-sonnet-5"
	}
	if cfg.OpusModel == "" {
		cfg.OpusModel = "claude-opus-4-8"
	}
	if cfg.HaikuModel == "" {
		cfg.HaikuModel = "claude-haiku-4-5"
	}
	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 1536
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &AnthropicClient{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

func (c *AnthropicClient) model(t Tier) string {
	switch t {
	case TierOpus:
		return c.cfg.OpusModel
	case TierModeration:
		return c.cfg.HaikuModel
	default:
		return c.cfg.SonnetModel
	}
}

// --- wire types (Messages API) ---

type messagesRequest struct {
	Model      string           `json:"model"`
	MaxTokens  int              `json:"max_tokens"`
	System     string           `json:"system,omitempty"`
	Messages   []messageParam   `json:"messages"`
	Tools      []toolDef        `json:"tools,omitempty"`
	ToolChoice *toolChoice      `json:"tool_choice,omitempty"`
}

type messageParam struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type toolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type messagesResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// emitIdeasInput is the tool schema payload Claude fills in.
type emitIdeasInput struct {
	Ideas []Idea `json:"ideas"`
}

// GenerateIdeas builds a structured prompt and forces the emit_ideas tool so the
// model returns typed idea objects.
func (c *AnthropicClient) GenerateIdeas(ctx context.Context, p GenerateParams) ([]Idea, error) {
	n := p.N
	if n <= 0 {
		n = 5
	}
	system := "You are PerfectGift's surprise-idea generator. Produce several genuinely good, " +
		"budget-appropriate, non-generic surprise ideas for a partner. Ground every idea in the " +
		"provided holiday, budget, preferences, poll answers, and inspiration snippets. Always " +
		"return your answer by calling the emit_ideas tool."

	var b strings.Builder
	fmt.Fprintf(&b, "Holiday: %s\nBudget band: %s\nPreferences: %s\n", p.Holiday, p.BudgetBand, p.Preferences)
	if len(p.PollAnswers) > 0 {
		fmt.Fprintf(&b, "Poll answers from the partner:\n- %s\n", strings.Join(p.PollAnswers, "\n- "))
	}
	if len(p.Inspiration) > 0 {
		fmt.Fprintf(&b, "Curated inspiration seeds:\n- %s\n", strings.Join(p.Inspiration, "\n- "))
	}
	if p.Refinement != "" {
		fmt.Fprintf(&b, "Refinement request: %s\n", p.Refinement)
	}
	fmt.Fprintf(&b, "Generate exactly %d ideas.", n)

	tool := toolDef{
		Name:        "emit_ideas",
		Description: "Return the generated surprise ideas as typed objects.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ideas": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"title":       map[string]interface{}{"type": "string"},
							"why_it_fits": map[string]interface{}{"type": "string"},
							"rough_cost":  map[string]interface{}{"type": "string"},
							"how_to":      map[string]interface{}{"type": "string"},
						},
						"required":             []string{"title", "why_it_fits", "rough_cost", "how_to"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"ideas"},
			"additionalProperties": false,
		},
	}

	req := messagesRequest{
		Model:      c.model(p.Tier),
		MaxTokens:  c.cfg.MaxTokens,
		System:     system,
		Messages:   []messageParam{{Role: "user", Content: b.String()}},
		Tools:      []toolDef{tool},
		ToolChoice: &toolChoice{Type: "tool", Name: "emit_ideas"},
	}

	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, block := range resp.Content {
		if block.Type == "tool_use" && block.Name == "emit_ideas" {
			var in emitIdeasInput
			if err := json.Unmarshal(block.Input, &in); err != nil {
				return nil, fmt.Errorf("decode tool input: %w", err)
			}
			return in.Ideas, nil
		}
	}
	return nil, fmt.Errorf("anthropic: no emit_ideas tool_use in response")
}

// Moderate runs the cheap Haiku classification pass.
func (c *AnthropicClient) Moderate(ctx context.Context, text string) (bool, error) {
	req := messagesRequest{
		Model:     c.model(TierModeration),
		MaxTokens: 8,
		System:    "You are a content-safety classifier for a wholesome gift-planning app. Reply with exactly one word: SAFE or UNSAFE.",
		Messages:  []messageParam{{Role: "user", Content: text}},
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return false, err
	}
	for _, block := range resp.Content {
		if block.Type == "text" {
			return !strings.Contains(strings.ToUpper(block.Text), "UNSAFE"), nil
		}
	}
	return false, fmt.Errorf("anthropic: empty moderation response")
}

// Embed returns an embedding vector. Anthropic does not currently expose a
// first-party embeddings endpoint on the Messages API; when no embedding endpoint
// is configured we derive a stable pseudo-embedding from the text so downstream
// pgvector storage and dedup still function. In production this must be swapped
// for the real embedding provider that matches Catalog's embedding space.
func (c *AnthropicClient) Embed(_ context.Context, text string) ([]float32, error) {
	dim := c.cfg.EmbeddingDim
	v := make([]float32, dim)
	for i, r := range text {
		v[i%dim] += float32(r) / 1000.0
	}
	return v, nil
}

func (c *AnthropicClient) do(ctx context.Context, body messagesRequest) (*messagesResponse, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", c.cfg.Version)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, string(data))
	}
	var out messagesResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}
	return &out, nil
}
