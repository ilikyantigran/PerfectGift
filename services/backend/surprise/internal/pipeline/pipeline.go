// Package pipeline implements the generation algorithm (SERVICE.md §5) that runs
// inside the worker pool, off the request path. It is pure orchestration over
// interfaces (Repository, Cache, llm.Client, Poll, Catalog, Publisher) so the
// whole flow is unit-tested with fakes — no DB, NATS, network, or API key.
package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/clients"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/llm"
)

// Config carries the pipeline tuning knobs.
type Config struct {
	IdeasWanted int
	StatusTTL   time.Duration
	LLMCacheTTL time.Duration
}

// Pipeline runs a single generation job end to end.
type Pipeline struct {
	repo    domain.Repository
	cache   domain.Cache
	llm     llm.Client
	poll    clients.Poll
	catalog clients.Catalog
	pub     events.Publisher
	cfg     Config
}

// New builds a Pipeline.
func New(repo domain.Repository, cache domain.Cache, model llm.Client, poll clients.Poll, catalog clients.Catalog, pub events.Publisher, cfg Config) *Pipeline {
	if cfg.IdeasWanted <= 0 {
		cfg.IdeasWanted = 5
	}
	return &Pipeline{repo: repo, cache: cache, llm: model, poll: poll, catalog: catalog, pub: pub, cfg: cfg}
}

// Run executes steps 2–8 of the generation pipeline for one job. Business
// failures (LLM exhausted, nothing survives moderation) mark the request FAILED
// and return nil so the job is not redelivered forever; only lookup/infra errors
// are returned to the caller.
func (p *Pipeline) Run(ctx context.Context, job events.GenerationRequested) error {
	req, err := p.repo.GetRequest(ctx, job.RequestID)
	if err != nil {
		return fmt.Errorf("load request %s: %w", job.RequestID, err)
	}

	// Step 2: running.
	p.setStatus(ctx, req.ID, domain.StatusRunning, 10)

	// Step 3: LLM cache check (hash of normalized inputs).
	hash := hashInputs(req, p.cfg.IdeasWanted)
	candidates, cacheHit := p.cachedCandidates(ctx, hash)

	if !cacheHit {
		// Step 4: gather grounding (best-effort; degrade if unavailable).
		pollAnswers := p.gatherPoll(ctx, req.PollID)
		embedding, _ := p.llm.Embed(ctx, groundingQuery(req))
		inspiration := p.gatherCatalog(ctx, req, embedding)
		p.setStatus(ctx, req.ID, domain.StatusRunning, 40)

		// Step 5: structured prompt -> Claude (tool use), tier-selected.
		gen, gerr := p.llm.GenerateIdeas(ctx, llm.GenerateParams{
			Holiday:     req.HolidayID,
			BudgetBand:  req.BudgetBand,
			Preferences: req.PreferencesText,
			PollAnswers: pollAnswers,
			Inspiration: inspiration,
			Refinement:  req.Refinement,
			N:           p.cfg.IdeasWanted,
			Tier:        tierFor(req.Tier),
		})
		if gerr != nil {
			slog.Error("generation failed", "request_id", req.ID, "err", gerr)
			p.fail(ctx, req.ID)
			return nil
		}
		candidates = toCandidates(gen)
		// cache the raw response keyed by input hash (main cost lever).
		_ = p.cache.SetLLMCache(ctx, hash, candidates, p.cfg.LLMCacheTTL)
	}
	p.setStatus(ctx, req.ID, domain.StatusRunning, 70)

	// Step 6: moderate (Haiku) + validate + rank.
	ideas := p.moderateValidateRank(ctx, req, candidates)
	if len(ideas) == 0 {
		slog.Warn("no ideas survived moderation/validation", "request_id", req.ID)
		p.fail(ctx, req.ID)
		return nil
	}

	// Step 7: persist (+ embeddings) and set ready.
	if err := p.repo.SaveIdeas(ctx, req.ID, ideas); err != nil {
		return fmt.Errorf("save ideas: %w", err)
	}
	p.setStatus(ctx, req.ID, domain.StatusReady, 100)
	if err := p.repo.SetRequestStatus(ctx, req.ID, domain.StatusReady); err != nil {
		return fmt.Errorf("set ready: %w", err)
	}

	// Step 8: publish IdeasReady.
	if err := p.pub.PublishIdeasReady(ctx, events.IdeasReady{
		RequestID: req.ID,
		UserID:    req.UserID,
		IdeaCount: len(ideas),
	}); err != nil {
		return fmt.Errorf("publish ideas ready: %w", err)
	}
	return nil
}

func (p *Pipeline) cachedCandidates(ctx context.Context, hash string) ([]domain.Idea, bool) {
	cached, err := p.cache.GetLLMCache(ctx, hash)
	if err != nil || len(cached) == 0 {
		return nil, false
	}
	return cached, true
}

func (p *Pipeline) gatherPoll(ctx context.Context, pollID string) []string {
	if pollID == "" || p.poll == nil {
		return nil
	}
	answers, err := p.poll.GetResponses(ctx, pollID)
	if err != nil {
		slog.Warn("poll grounding unavailable, degrading", "poll_id", pollID, "err", err)
		return nil
	}
	return answers
}

func (p *Pipeline) gatherCatalog(ctx context.Context, req *domain.Request, embedding []float32) []string {
	if p.catalog == nil {
		return nil
	}
	snippets, err := p.catalog.SearchInspiration(ctx, groundingQuery(req), embedding, req.BudgetBand, p.cfg.IdeasWanted)
	if err != nil {
		slog.Warn("catalog grounding unavailable, degrading", "err", err)
		return nil
	}
	return snippets
}

// moderateValidateRank drops rejected/invalid candidates, ranks the survivors in
// order, embeds them, and stamps request-scoped identity.
func (p *Pipeline) moderateValidateRank(ctx context.Context, req *domain.Request, candidates []domain.Idea) []domain.Idea {
	out := make([]domain.Idea, 0, len(candidates))
	rank := 1
	for _, c := range candidates {
		if strings.TrimSpace(c.Title) == "" { // validate
			continue
		}
		approved, err := p.llm.Moderate(ctx, c.Title+". "+c.WhyItFits+". "+c.HowTo)
		if err != nil {
			slog.Warn("moderation error, dropping candidate", "err", err)
			continue
		}
		if !approved {
			continue
		}
		emb, _ := p.llm.Embed(ctx, c.Title+" "+c.WhyItFits)
		out = append(out, domain.Idea{
			ID:         uuid.NewString(),
			RequestID:  req.ID,
			Title:      c.Title,
			WhyItFits:  c.WhyItFits,
			RoughCost:  c.RoughCost,
			HowTo:      c.HowTo,
			Rank:       rank,
			Moderation: domain.ModerationApproved,
			Embedding:  emb,
			CreatedAt:  time.Now().UTC(),
		})
		rank++
	}
	return out
}

func (p *Pipeline) setStatus(ctx context.Context, id string, s domain.Status, progress int) {
	if err := p.cache.SetStatus(ctx, id, domain.StatusInfo{Status: s, Progress: progress}, p.cfg.StatusTTL); err != nil {
		slog.Warn("set status cache failed", "request_id", id, "err", err)
	}
	if s == domain.StatusRunning {
		_ = p.repo.SetRequestStatus(ctx, id, s)
	}
}

func (p *Pipeline) fail(ctx context.Context, id string) {
	p.setStatus(ctx, id, domain.StatusFailed, 0)
	_ = p.repo.SetRequestStatus(ctx, id, domain.StatusFailed)
}

func toCandidates(gen []llm.Idea) []domain.Idea {
	out := make([]domain.Idea, 0, len(gen))
	for _, g := range gen {
		out = append(out, domain.Idea{
			Title:     g.Title,
			WhyItFits: g.WhyItFits,
			RoughCost: g.RoughCost,
			HowTo:     g.HowTo,
		})
	}
	return out
}

func tierFor(t domain.Tier) llm.Tier {
	if t == domain.TierOpus {
		return llm.TierOpus
	}
	return llm.TierSonnet
}

func groundingQuery(req *domain.Request) string {
	return strings.TrimSpace(fmt.Sprintf("%s %s %s %s", req.HolidayID, req.BudgetBand, req.PreferencesText, req.Refinement))
}

// hashInputs is the normalized-input cache key. Normalization: lowercased,
// space-collapsed field values joined with a separator, then SHA-256.
func hashInputs(req *domain.Request, n int) string {
	norm := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
	parts := []string{
		norm(req.HolidayID),
		norm(req.BudgetBand),
		norm(req.PreferencesText),
		norm(req.PollID),
		norm(req.Refinement),
		string(req.Tier),
		fmt.Sprintf("n=%d", n),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}
