package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// emitIdeasInput is the tool schema payload Claude fills in. Ideas is decoded
// leniently: the tool schema declares "ideas" as a JSON array, but Claude has
// been observed to occasionally return it as a JSON-encoded string containing
// that array instead (a stringified array). Both shapes decode to the same
// []Idea via the custom UnmarshalJSON below.
type emitIdeasInput struct {
	Ideas []Idea `json:"-"`
}

func (in *emitIdeasInput) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		Ideas json.RawMessage `json:"ideas"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	ideas, err := decodeIdeasField(wrapper.Ideas)
	if err != nil {
		return err
	}
	in.Ideas = ideas
	return nil
}

// maxIdeasDecodeDepth bounds recursive unwrapping of nested wrapper objects so a
// pathological payload (e.g. {"ideas":{"ideas":{"ideas":...}}}) can't recurse
// without end.
const maxIdeasDecodeDepth = 3

// decodeIdeasField decodes the "ideas" field of the emit_ideas tool input,
// tolerating every shape the model has been observed to emit:
//
//	1. a plain JSON array                        [{...},{...}]           (happy path)
//	2. a JSON-encoded string of that array       "[{...},{...}]"         (stringified)
//	3. a wrapper object carrying the array        {"ideas":[{...}]}       (unwrap one level)
//	4. a single idea object                       {"title":...,...}       (wrap as one element)
//
// The array and stringified-array paths are tried first and unchanged. If none
// match, the original array-decode error is returned (it best describes the
// expected shape) — and the raw payload is logged by the caller on failure.
func decodeIdeasField(raw json.RawMessage) ([]Idea, error) {
	return decodeIdeasFieldDepth(raw, 0)
}

func decodeIdeasFieldDepth(raw json.RawMessage, depth int) ([]Idea, error) {
	// Absent ("ideas" key missing) or explicit null: no ideas, no error. Matches
	// the pre-tolerance struct-tag behavior and avoids a misleading "unexpected end
	// of JSON input" from json.Unmarshal(nil, ...) on a partial tool input.
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var ideas []Idea
	arrErr := json.Unmarshal(raw, &ideas)
	if arrErr == nil {
		return ideas, nil
	}

	// Not a bare array — try the stringified-array case: raw is a JSON string that
	// itself contains the array. If the unquote succeeds, the inner decode error
	// (if any) is the actionable one, so surface it rather than the outer arrErr.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if innerErr := json.Unmarshal([]byte(s), &ideas); innerErr != nil {
			return nil, innerErr
		}
		return ideas, nil
	}

	// Object shapes: the model has been observed returning "ideas" as a JSON
	// object rather than an array. Two variants are handled, in order:
	//   (a) a wrapper object carrying the array, e.g. {"ideas":[...]} — unwrap one
	//       level and decode its inner "ideas" field (recursively, depth-bounded);
	//   (b) a single idea object, e.g. {"title":...,"why_it_fits":...} — wrap it as
	//       a one-element []Idea.
	if depth < maxIdeasDecodeDepth && isJSONObject(raw) {
		// (a) wrapper object.
		var wrapper struct {
			Ideas json.RawMessage `json:"ideas"`
		}
		if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Ideas) > 0 {
			return decodeIdeasFieldDepth(wrapper.Ideas, depth+1)
		}
		// (b) single idea object — only accept a non-empty idea so an unrelated
		// empty/foreign object still falls through to the error below.
		var single Idea
		if err := json.Unmarshal(raw, &single); err == nil && single != (Idea{}) {
			return []Idea{single}, nil
		}
	}

	return nil, arrErr
}

// isJSONObject reports whether raw is a JSON object (first non-space byte is '{').
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// maxLoggedInputLen caps how many bytes of a raw tool payload are logged on a
// decode failure, so a large model output can't flood the logs.
const maxLoggedInputLen = 2048

// truncateForLog renders raw tool-input bytes as a string for diagnostic logging,
// truncating to maxLoggedInputLen and appending an ellipsis marker when cut. Only
// the model's own idea output is ever passed here (no secrets/keys), but the cap
// is applied regardless.
func truncateForLog(b []byte) string {
	if len(b) <= maxLoggedInputLen {
		return string(b)
	}
	return string(b[:maxLoggedInputLen]) + "…(truncated)"
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
				// Log the raw tool payload (truncated) so the actual shape is always
				// diagnosable instead of guessed at from the error alone. Fires only on
				// the failure path — never on success.
				slog.WarnContext(ctx, "emit_ideas decode failed", "raw", truncateForLog(block.Input), "error", err)
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
