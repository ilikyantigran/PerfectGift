package app

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/domain/memory"
	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/surprise/pkg/api/surprise/v1"
)

func newServer() (*Server, *memory.Store, *events.Bus) {
	store := memory.New()
	bus := events.NewBus(false)
	return NewServer(store, store, bus, ServerConfig{}), store, bus
}

func asUser(id string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id))
}

func codeOf(err error) codes.Code { return status.Code(err) }

func validRequest(key string) *surprisev1.RequestGenerationRequest {
	return &surprisev1.RequestGenerationRequest{
		UserId: "u1", HolidayId: "valentine", BudgetBand: "mid",
		PreferencesText: "cozy", IdempotencyKey: key,
	}
}

func TestRequestGenerationValidation(t *testing.T) {
	s, _, _ := newServer()
	_, err := s.RequestGeneration(context.Background(), &surprisev1.RequestGenerationRequest{HolidayId: "x", BudgetBand: "y", IdempotencyKey: "k"})
	if codeOf(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing user_id, got %v", err)
	}
}

func TestRequestGenerationEnqueuesAndReturns202(t *testing.T) {
	s, store, bus := newServer()
	resp, err := s.RequestGeneration(context.Background(), validRequest("k1"))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED || resp.RequestId == "" {
		t.Fatalf("expected queued + id, got %+v", resp)
	}
	if len(bus.Jobs) != 1 || bus.Jobs[0].RequestID != resp.RequestId {
		t.Fatalf("expected one enqueued job for the request, got %+v", bus.Jobs)
	}
	if r, _ := store.GetRequest(context.Background(), resp.RequestId); r == nil || r.Status != domain.StatusQueued {
		t.Fatal("request not persisted as queued")
	}
	if info, _ := store.GetStatus(context.Background(), resp.RequestId); info.Status != domain.StatusQueued {
		t.Fatal("status not seeded in cache")
	}
}

func TestRequestGenerationIdempotent(t *testing.T) {
	s, _, bus := newServer()
	first, err := s.RequestGeneration(context.Background(), validRequest("same-key"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.RequestGeneration(context.Background(), validRequest("same-key"))
	if err != nil {
		t.Fatal(err)
	}
	if first.RequestId != second.RequestId {
		t.Fatalf("same idempotency key should return same request id: %s vs %s", first.RequestId, second.RequestId)
	}
	if len(bus.Jobs) != 1 {
		t.Fatalf("duplicate submit must enqueue exactly one job, got %d", len(bus.Jobs))
	}
}

func TestGetGenerationStatusOwnerScoping(t *testing.T) {
	s, store, _ := newServer()
	resp, _ := s.RequestGeneration(context.Background(), validRequest("k2"))
	_ = store

	// Wrong caller -> PermissionDenied.
	_, err := s.GetGenerationStatus(asUser("intruder"), &surprisev1.GetGenerationStatusRequest{RequestId: resp.RequestId})
	if codeOf(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for non-owner, got %v", err)
	}
	// Owner -> ok.
	got, err := s.GetGenerationStatus(asUser("u1"), &surprisev1.GetGenerationStatusRequest{RequestId: resp.RequestId})
	if err != nil {
		t.Fatalf("owner should read status: %v", err)
	}
	if got.Status != surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED {
		t.Fatalf("expected queued, got %v", got.Status)
	}
}

func seedReadyRequest(t *testing.T, store *memory.Store) (reqID, ideaID string) {
	t.Helper()
	r := &domain.Request{ID: "r-ready", UserID: "u1", HolidayID: "h", BudgetBand: "mid", IdempotencyKey: "seed", Status: domain.StatusReady, Tier: domain.TierSonnet}
	if err := store.CreateRequest(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	ideas := []domain.Idea{
		{ID: "idea-2", RequestID: r.ID, Title: "second", Rank: 2, Moderation: domain.ModerationApproved},
		{ID: "idea-1", RequestID: r.ID, Title: "first", Rank: 1, Moderation: domain.ModerationApproved},
	}
	if err := store.SaveIdeas(context.Background(), r.ID, ideas); err != nil {
		t.Fatal(err)
	}
	return r.ID, "idea-1"
}

func TestGetIdeasReturnsRankedOwnerOnly(t *testing.T) {
	s, store, _ := newServer()
	reqID, _ := seedReadyRequest(t, store)

	if _, err := s.GetIdeas(asUser("intruder"), &surprisev1.GetIdeasRequest{RequestId: reqID}); codeOf(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for non-owner, got %v", err)
	}
	resp, err := s.GetIdeas(asUser("u1"), &surprisev1.GetIdeasRequest{RequestId: reqID})
	if err != nil {
		t.Fatalf("owner get ideas: %v", err)
	}
	if len(resp.Ideas) != 2 || resp.Ideas[0].Rank != 1 || resp.Ideas[1].Rank != 2 {
		t.Fatalf("expected ideas ranked ascending, got %+v", resp.Ideas)
	}
}

func TestSaveIdeaOwnerScoping(t *testing.T) {
	s, store, _ := newServer()
	_, ideaID := seedReadyRequest(t, store)

	// Idea belongs to u1; u2 cannot save it.
	if _, err := s.SaveIdea(asUser("u2"), &surprisev1.SaveIdeaRequest{UserId: "u2", IdeaId: ideaID}); codeOf(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied saving another user's idea, got %v", err)
	}
	resp, err := s.SaveIdea(asUser("u1"), &surprisev1.SaveIdeaRequest{UserId: "u1", IdeaId: ideaID})
	if err != nil || !resp.Ok {
		t.Fatalf("owner save should succeed: %v", err)
	}
	if !store.SavedFor("u1", ideaID) {
		t.Fatal("idea not recorded as saved")
	}
}

func TestRefineRequeues(t *testing.T) {
	s, store, bus := newServer()
	created, _ := s.RequestGeneration(context.Background(), validRequest("k3"))
	before := len(bus.Jobs)

	resp, err := s.Refine(asUser("u1"), &surprisev1.RefineRequest{RequestId: created.RequestId, Refinement: "make it cheaper"})
	if err != nil {
		t.Fatalf("refine: %v", err)
	}
	if resp.Status != surprisev1.GenerationStatus_GENERATION_STATUS_QUEUED {
		t.Fatalf("expected queued after refine, got %v", resp.Status)
	}
	if len(bus.Jobs) != before+1 {
		t.Fatalf("refine should enqueue a new job")
	}
	r, _ := store.GetRequest(context.Background(), created.RequestId)
	if r.Refinement != "make it cheaper" || r.Status != domain.StatusQueued {
		t.Fatalf("refinement not recorded / not requeued: %+v", r)
	}
}
