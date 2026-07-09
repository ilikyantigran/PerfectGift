package app

import (
	"context"
	"testing"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/clients"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain/memory"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/llm"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/pipeline"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/surprise/v1"
)

// TestEndToEndGenerationLoop wires the request path (Server), the bus, and the
// worker pipeline together with in-memory fakes, and drives the whole async loop
// from RequestGeneration through IdeasReady. The Bus runs in dispatch mode so a
// published job is handled synchronously — no goroutines/sleeps needed.
func TestEndToEndGenerationLoop(t *testing.T) {
	store := memory.New()
	bus := events.NewBus(true) // dispatch: publish -> handler runs inline
	fake := &llm.FakeClient{}

	pipe := pipeline.New(store, store, fake,
		&clients.FakePoll{Answers: []string{"loves the sea"}},
		&clients.FakeCatalog{Snippets: []string{"beach bonfire"}},
		bus, pipeline.Config{IdeasWanted: 3})
	worker := pipeline.NewWorker(bus, pipe)
	if err := worker.Start(context.Background()); err != nil {
		t.Fatalf("worker start: %v", err)
	}

	srv := NewServer(store, store, bus, ServerConfig{})

	resp, err := srv.RequestGeneration(context.Background(), &surprisev1.RequestGenerationRequest{
		UserId: "u1", HolidayId: "anniversary", BudgetBand: "high",
		PreferencesText: "romantic seaside evening", IdempotencyKey: "e2e-1",
		Tier: surprisev1.ModelTier_MODEL_TIER_OPUS,
	})
	if err != nil {
		t.Fatalf("request generation: %v", err)
	}

	// The job dispatched inline; the request should now be ready with ideas.
	got, err := srv.GetGenerationStatus(asUser("u1"), &surprisev1.GetGenerationStatusRequest{RequestId: resp.RequestId})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.Status != surprisev1.GenerationStatus_GENERATION_STATUS_READY {
		t.Fatalf("expected READY after inline generation, got %v", got.Status)
	}
	ideas, err := srv.GetIdeas(asUser("u1"), &surprisev1.GetIdeasRequest{RequestId: resp.RequestId})
	if err != nil {
		t.Fatalf("get ideas: %v", err)
	}
	if len(ideas.Ideas) != 3 {
		t.Fatalf("expected 3 ideas, got %d", len(ideas.Ideas))
	}
	if len(bus.Ready) != 1 || bus.Ready[0].IdeaCount != 3 {
		t.Fatalf("expected one IdeasReady with count 3, got %+v", bus.Ready)
	}
	// Opus tier should have been requested on the job.
	if len(bus.Jobs) == 0 || bus.Jobs[0].Tier != string(domain.TierOpus) {
		t.Fatalf("expected opus tier on job, got %+v", bus.Jobs)
	}
}
