// Package ports declares the interfaces and shared types that decouple the
// server (internal/app) from its backing stores (internal/domain/*). The server
// depends on these interfaces; the concrete stores implement them. Keeping them
// in their own package avoids an import cycle between app and the stores.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/domain/model"
)

// Sentinel errors the Repo returns; the server maps them to gRPC codes (and, for
// the anonymous path, to a uniform NotFound so expired/invalid/consumed are
// indistinguishable — no enumeration oracle).
var (
	ErrNotFound         = errors.New("not found")
	ErrAlreadyCompleted = errors.New("poll already completed")
)

// LinkedPoll is the raw result of resolving a link token hash: the poll plus the
// link row's own state. The server applies the expiry/revoked/status rules so
// they stay in one place.
type LinkedPoll struct {
	Poll          model.Poll
	LinkRevoked   bool
	LinkExpiresAt time.Time
}

// Repo is the Postgres port. Implemented by internal/domain/postgres; faked in
// tests. The service owns the `poll` schema; no cross-schema access.
type Repo interface {
	// CreatePoll persists the poll and its (hashed) link atomically.
	CreatePoll(ctx context.Context, p model.Poll, tokenHash string, linkExpiresAt time.Time) error
	// GetByTokenHash resolves a link token hash to its poll. ErrNotFound when no
	// such link exists.
	GetByTokenHash(ctx context.Context, tokenHash string) (LinkedPoll, error)
	// CompleteWithResponse atomically records the one response and flips the poll
	// active→completed. Returns ErrAlreadyCompleted if the poll is not active
	// (one-response guard). Returns the new response id.
	CompleteWithResponse(ctx context.Context, pollID string, answers []model.Answer, fingerprint string, at time.Time) (string, error)
	// GetPollByID fetches a poll for owner-scoped reads. ErrNotFound if absent.
	GetPollByID(ctx context.Context, pollID string) (model.Poll, error)
	// GetResponses returns a poll's responses (owner-scoped read).
	GetResponses(ctx context.Context, pollID string) ([]model.Response, error)
}

// RateLimiter is the Valkey port for anonymous abuse control: a fixed-window
// counter. Allow returns false when the key has exhausted its budget in the
// current window. Implemented by internal/domain/valkey; faked in tests.
type RateLimiter interface {
	Allow(ctx context.Context, key string, budget int, window time.Duration) (bool, error)
}

// PollCompleted is the event payload published on completion.
type PollCompleted struct {
	PollID            string    `json:"poll_id"`
	SurpriseRequestID string    `json:"surprise_request_id,omitempty"`
	OwnerUserID       string    `json:"owner_user_id"`
	CompletedAt       time.Time `json:"completed_at"`
}

// Publisher is the NATS JetStream port. Implemented by internal/domain/events;
// faked in tests so the suite needs no live broker.
type Publisher interface {
	PublishPollCompleted(ctx context.Context, ev PollCompleted) error
}
