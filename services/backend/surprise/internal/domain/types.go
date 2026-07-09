// Package domain holds the Surprise service's core entities and the storage
// interfaces its handlers and worker depend on. Concrete stores live in
// subpackages (postgres, valkey); an in-memory implementation for tests lives in
// domain/memory. Keeping the interfaces here (consumer-neutral) lets the server
// and the pipeline share them without importing a concrete store.
package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by stores when a lookup misses.
var ErrNotFound = errors.New("surprise: not found")

// Status is the lifecycle of a generation request.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusReady   Status = "ready"
	StatusFailed  Status = "failed"
)

// Tier selects the generation model (Sonnet default, Opus premium).
type Tier string

const (
	TierSonnet Tier = "sonnet"
	TierOpus   Tier = "opus"
)

// Moderation is the outcome of the Haiku moderation pass on an idea.
type Moderation string

const (
	ModerationApproved Moderation = "approved"
	ModerationRejected Moderation = "rejected"
	ModerationPending  Moderation = "pending"
)

// Request is a persisted generation request (surprise_requests).
type Request struct {
	ID              string
	UserID          string
	HolidayID       string
	BudgetBand      string
	PreferencesText string
	PollID          string // optional
	IdempotencyKey  string
	Status          Status
	Tier            Tier
	Refinement      string // set by Refine
	CreatedAt       time.Time
}

// Idea is a persisted generated idea (generated_ideas). Embedding supports
// dedup/similarity across ideas.
type Idea struct {
	ID         string
	RequestID  string
	Title      string
	WhyItFits  string
	RoughCost  string
	HowTo      string
	Rank       int
	Moderation Moderation
	Embedding  []float32
	CreatedAt  time.Time
}

// StatusInfo is the cheap poll payload cached in Valkey.
type StatusInfo struct {
	Status   Status `json:"status"`
	Progress int    `json:"progress"`
}

// Repository is the transactional idea ledger (Postgres).
type Repository interface {
	// CreateRequest persists a new request. Returns ErrDuplicate if the
	// idempotency_key already exists.
	CreateRequest(ctx context.Context, r *Request) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	GetRequestByIdempotencyKey(ctx context.Context, key string) (*Request, error)
	SetRequestStatus(ctx context.Context, id string, s Status) error
	// MarkRefinement records the refinement text, bumps status back to queued.
	MarkRefinement(ctx context.Context, id, refinement string) error
	// SaveIdeas replaces the request's ideas with the given ranked set.
	SaveIdeas(ctx context.Context, requestID string, ideas []Idea) error
	GetIdeas(ctx context.Context, requestID string) ([]Idea, error)
	GetIdea(ctx context.Context, ideaID string) (*Idea, error)
	SaveIdeaForUser(ctx context.Context, userID, ideaID string) error
}

// ErrDuplicate is returned by CreateRequest when the idempotency_key collides.
var ErrDuplicate = errors.New("surprise: duplicate idempotency key")

// Cache is the ephemeral store (Valkey): job status, idempotency, LLM cache.
type Cache interface {
	SetStatus(ctx context.Context, requestID string, info StatusInfo, ttl time.Duration) error
	GetStatus(ctx context.Context, requestID string) (StatusInfo, error)
	// SetIdempotencyIfAbsent stores key->requestID only if absent. Returns
	// (stored, existingRequestID). If stored is false, existing holds the winner.
	SetIdempotencyIfAbsent(ctx context.Context, key, requestID string, ttl time.Duration) (bool, string, error)
	GetLLMCache(ctx context.Context, hash string) ([]Idea, error)
	SetLLMCache(ctx context.Context, hash string, ideas []Idea, ttl time.Duration) error
}
