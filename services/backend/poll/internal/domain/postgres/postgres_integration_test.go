package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/ports"

	"github.com/google/uuid"
)

// Integration test against a real Postgres. Hermetic by default: it skips unless
// POLL_TEST_POSTGRES_DSN points at a disposable database, so `go test ./...`
// stays green without any live dependency.
//
// Run it with, e.g.:
//
//	docker run --rm -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:16
//	POLL_TEST_POSTGRES_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
//	    go test ./internal/domain/postgres/ -run Integration -v
func TestIntegration_PollLifecycle(t *testing.T) {
	dsn := os.Getenv("POLL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set POLL_TEST_POSTGRES_DSN to run the Postgres integration test")
	}
	ctx := context.Background()
	store, err := NewStore(ctx, dsn)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	p := model.Poll{
		ID:                uuid.NewString(),
		OwnerUserID:       "owner-int",
		SurpriseRequestID: "sr-int",
		Title:             "Integration poll",
		Questions: []model.Question{
			{ID: "q1", Prompt: "Free text?", Type: model.TypeText, Required: true},
		},
		Status:    model.StatusActive,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}
	hash := "hash-" + p.ID
	if err := store.CreatePoll(ctx, p, hash, now.Add(time.Hour)); err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	lp, err := store.GetByTokenHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if lp.Poll.ID != p.ID || lp.Poll.OwnerUserID != "owner-int" || len(lp.Poll.Questions) != 1 {
		t.Fatalf("round-trip mismatch: %+v", lp.Poll)
	}

	answers := []model.Answer{{QuestionID: "q1", Text: "hello"}}
	if _, err := store.CompleteWithResponse(ctx, p.ID, answers, "fp", now); err != nil {
		t.Fatalf("CompleteWithResponse: %v", err)
	}
	// one-response guard: second attempt must fail
	if _, err := store.CompleteWithResponse(ctx, p.ID, answers, "fp", now); err != ports.ErrAlreadyCompleted {
		t.Fatalf("second complete: want ErrAlreadyCompleted, got %v", err)
	}

	resps, err := store.GetResponses(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetResponses: %v", err)
	}
	if len(resps) != 1 || len(resps[0].Answers) != 1 || resps[0].Answers[0].Text != "hello" {
		t.Fatalf("responses mismatch: %+v", resps)
	}

	// unknown token hash -> ErrNotFound
	if _, err := store.GetByTokenHash(ctx, "does-not-exist"); err != ports.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
