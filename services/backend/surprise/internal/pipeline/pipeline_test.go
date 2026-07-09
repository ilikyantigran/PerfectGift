package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/clients"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain/memory"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/llm"
)

type harness struct {
	store   *memory.Store
	fake    *llm.FakeClient
	poll    *clients.FakePoll
	catalog *clients.FakeCatalog
	bus     *events.Bus
	pipe    *Pipeline
}

func newHarness(t *testing.T, wanted int) *harness {
	t.Helper()
	store := memory.New()
	fake := &llm.FakeClient{}
	poll := &clients.FakePoll{Answers: []string{"likes hiking"}}
	catalog := &clients.FakeCatalog{Snippets: []string{"sunset picnic"}}
	bus := events.NewBus(false)
	pipe := New(store, store, fake, poll, catalog, bus, Config{IdeasWanted: wanted})
	return &harness{store: store, fake: fake, poll: poll, catalog: catalog, bus: bus, pipe: pipe}
}

func seedRequest(t *testing.T, store *memory.Store, id string) *domain.Request {
	t.Helper()
	r := &domain.Request{
		ID: id, UserID: "u1", HolidayID: "valentine", BudgetBand: "mid",
		PreferencesText: "cozy and outdoorsy", IdempotencyKey: "idem-" + id,
		Status: domain.StatusQueued, Tier: domain.TierSonnet,
	}
	if err := store.CreateRequest(context.Background(), r); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return r
}

func TestPipelineHappyPath(t *testing.T) {
	h := newHarness(t, 5)
	seedRequest(t, h.store, "req1")

	if err := h.pipe.Run(context.Background(), events.GenerationRequested{RequestID: "req1"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	ideas, _ := h.store.GetIdeas(context.Background(), "req1")
	if len(ideas) != 5 {
		t.Fatalf("expected 5 ideas persisted, got %d", len(ideas))
	}
	for i, idea := range ideas {
		if idea.Rank != i+1 {
			t.Fatalf("ideas not ranked contiguously: idx %d rank %d", i, idea.Rank)
		}
		if idea.Moderation != domain.ModerationApproved {
			t.Fatalf("idea %d not approved", i)
		}
		if len(idea.Embedding) == 0 {
			t.Fatalf("idea %d has no embedding", i)
		}
	}
	info, _ := h.store.GetStatus(context.Background(), "req1")
	if info.Status != domain.StatusReady || info.Progress != 100 {
		t.Fatalf("expected ready/100, got %v/%d", info.Status, info.Progress)
	}
	if len(h.bus.Ready) != 1 || h.bus.Ready[0].IdeaCount != 5 || h.bus.Ready[0].UserID != "u1" {
		t.Fatalf("expected one IdeasReady with count 5 for u1, got %+v", h.bus.Ready)
	}
}

func TestPipelineCacheHitSkipsLLM(t *testing.T) {
	h := newHarness(t, 3)
	r := seedRequest(t, h.store, "req2")
	// Pre-populate the LLM cache with the exact input hash.
	hash := hashInputs(r, 3)
	cached := []domain.Idea{
		{Title: "cached one", WhyItFits: "w", RoughCost: "$", HowTo: "h"},
		{Title: "cached two", WhyItFits: "w", RoughCost: "$", HowTo: "h"},
	}
	if err := h.store.SetLLMCache(context.Background(), hash, cached, 0); err != nil {
		t.Fatal(err)
	}
	h.fake.GenerateFunc = func(context.Context, llm.GenerateParams) ([]llm.Idea, error) {
		return nil, errors.New("LLM must not be called on cache hit")
	}
	if err := h.pipe.Run(context.Background(), events.GenerationRequested{RequestID: "req2"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if h.fake.Calls != 0 {
		t.Fatalf("expected 0 LLM calls on cache hit, got %d", h.fake.Calls)
	}
	ideas, _ := h.store.GetIdeas(context.Background(), "req2")
	if len(ideas) != 2 || ideas[0].Title != "cached one" {
		t.Fatalf("expected cached ideas persisted, got %+v", ideas)
	}
}

func TestPipelineDegradesWhenGroundingUnavailable(t *testing.T) {
	h := newHarness(t, 3)
	// Seed a request that DOES reference a poll, so the poll path is exercised.
	if err := h.store.CreateRequest(context.Background(), &domain.Request{
		ID: "req3", UserID: "u1", HolidayID: "valentine", BudgetBand: "mid",
		PreferencesText: "cozy", PollID: "poll-x", IdempotencyKey: "idem-req3",
		Status: domain.StatusQueued, Tier: domain.TierSonnet,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Force grounding sources to error; pipeline must still succeed.
	h.poll.Err = errors.New("poll down")
	h.catalog.Err = errors.New("catalog down")

	if err := h.pipe.Run(context.Background(), events.GenerationRequested{RequestID: "req3"}); err != nil {
		t.Fatalf("run should degrade gracefully, got: %v", err)
	}
	info, _ := h.store.GetStatus(context.Background(), "req3")
	if info.Status != domain.StatusReady {
		t.Fatalf("expected ready despite grounding failure, got %v", info.Status)
	}
	ideas, _ := h.store.GetIdeas(context.Background(), "req3")
	if len(ideas) == 0 {
		t.Fatal("expected ideas even with degraded grounding")
	}
}

func TestPipelineModerationAndValidationFilter(t *testing.T) {
	h := newHarness(t, 3)
	seedRequest(t, h.store, "req4")
	h.fake.GenerateFunc = func(context.Context, llm.GenerateParams) ([]llm.Idea, error) {
		return []llm.Idea{
			{Title: "good idea", WhyItFits: "fits", RoughCost: "$", HowTo: "do it"},
			{Title: "bad idea", WhyItFits: "this is UNSAFE content", RoughCost: "$", HowTo: "x"},
			{Title: "", WhyItFits: "no title", RoughCost: "$", HowTo: "x"}, // invalid
			{Title: "another good", WhyItFits: "fits too", RoughCost: "$", HowTo: "do it"},
		}, nil
	}
	if err := h.pipe.Run(context.Background(), events.GenerationRequested{RequestID: "req4"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	ideas, _ := h.store.GetIdeas(context.Background(), "req4")
	if len(ideas) != 2 {
		t.Fatalf("expected 2 survivors (1 unsafe + 1 empty dropped), got %d: %+v", len(ideas), ideas)
	}
	if ideas[0].Rank != 1 || ideas[1].Rank != 2 {
		t.Fatalf("ranks should be re-contiguous after filtering, got %d,%d", ideas[0].Rank, ideas[1].Rank)
	}
}

func TestPipelineGenerationFailureMarksFailed(t *testing.T) {
	h := newHarness(t, 3)
	seedRequest(t, h.store, "req5")
	h.fake.GenerateFunc = func(context.Context, llm.GenerateParams) ([]llm.Idea, error) {
		return nil, errors.New("llm exhausted")
	}
	if err := h.pipe.Run(context.Background(), events.GenerationRequested{RequestID: "req5"}); err != nil {
		t.Fatalf("business failure should not return infra error, got: %v", err)
	}
	info, _ := h.store.GetStatus(context.Background(), "req5")
	if info.Status != domain.StatusFailed {
		t.Fatalf("expected failed status, got %v", info.Status)
	}
	if len(h.bus.Ready) != 0 {
		t.Fatal("no IdeasReady should be published on failure")
	}
	req, _ := h.store.GetRequest(context.Background(), "req5")
	if req.Status != domain.StatusFailed {
		t.Fatalf("request row should be failed, got %v", req.Status)
	}
}
