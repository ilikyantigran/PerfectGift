// Package notify holds the notification service's core domain: the device and
// outbox types, the Store / Pusher / Subscription interfaces, the event
// consumers, and the outbox dispatcher. Everything external (Postgres, NATS,
// APNs, FCM) sits behind an interface in here so the logic is unit-testable
// without any live dependency.
package notify

import (
	"encoding/json"
	"fmt"
	"time"
)

// Platform is the device platform we push to.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Valid reports whether p is a platform we can route a push for.
func (p Platform) Valid() bool { return p == PlatformIOS || p == PlatformAndroid }

// Type is the kind of notification (matches the outbox `type` column).
type Type string

const (
	TypePollCompleted Type = "poll_completed"
	TypeIdeasReady    Type = "ideas_ready"
)

// Status is the lifecycle of an outbox row.
type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

// Device is a registered push target owned by this service's `devices` table.
type Device struct {
	ID           string
	UserID       string
	Platform     Platform
	PushToken    string
	AppVersion   string
	RegisteredAt time.Time
	LastSeenAt   time.Time
	Active       bool
}

// Outbox is one row of the transactional outbox (`notifications` table). It
// represents a single logical notification to a user; the dispatcher fans it
// out to all of that user's active devices at send time.
type Outbox struct {
	ID            string
	UserID        string
	Type          Type
	Payload       json.RawMessage
	DedupeKey     string
	Status        Status
	Attempts      int
	NextAttemptAt time.Time
	CreatedAt     time.Time
	SentAt        *time.Time
}

// --- Consumed events (JSON shapes emitted by Poll and Surprise) -------------

// PollCompletedEvent is emitted by the Poll service when the Subject finishes.
// Shape per poll/SERVICE.md §3.2.
type PollCompletedEvent struct {
	PollID             string    `json:"poll_id"`
	SurpriseRequestID  string    `json:"surprise_request_id,omitempty"`
	OwnerUserID        string    `json:"owner_user_id"`
	CompletedAt        time.Time `json:"completed_at"`
}

// IdeasReadyEvent is emitted by the Surprise service when generation finishes.
// Shape per surprise/SERVICE.md §3.2.
type IdeasReadyEvent struct {
	RequestID string `json:"request_id"`
	UserID    string `json:"user_id"`
	IdeaCount int    `json:"idea_count"`
}

// --- Outbox construction from events ----------------------------------------

// outbox builds a pending outbox row for a decoded event. The dedupe_key is the
// idempotency anchor: the same logical event, redelivered, produces the same
// key and therefore at most one row (see Store.EnqueueOutbox).
func (e PollCompletedEvent) toOutbox(now time.Time) (Outbox, error) {
	if e.PollID == "" || e.OwnerUserID == "" {
		return Outbox{}, fmt.Errorf("poll_completed: missing poll_id or owner_user_id")
	}
	payload, err := json.Marshal(map[string]any{
		"title":               "Your poll is done",
		"body":                "Your partner finished the poll — tap to sharpen your ideas.",
		"poll_id":             e.PollID,
		"surprise_request_id": e.SurpriseRequestID,
	})
	if err != nil {
		return Outbox{}, err
	}
	return Outbox{
		UserID:        e.OwnerUserID,
		Type:          TypePollCompleted,
		Payload:       payload,
		DedupeKey:     fmt.Sprintf("poll_completed:%s:%s", e.PollID, e.OwnerUserID),
		Status:        StatusPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}

func (e IdeasReadyEvent) toOutbox(now time.Time) (Outbox, error) {
	if e.RequestID == "" || e.UserID == "" {
		return Outbox{}, fmt.Errorf("ideas_ready: missing request_id or user_id")
	}
	payload, err := json.Marshal(map[string]any{
		"title":      "Your surprise ideas are ready",
		"body":       "We generated some ideas for your surprise — tap to see them.",
		"request_id": e.RequestID,
		"idea_count": e.IdeaCount,
	})
	if err != nil {
		return Outbox{}, err
	}
	return Outbox{
		UserID:        e.UserID,
		Type:          TypeIdeasReady,
		Payload:       payload,
		DedupeKey:     fmt.Sprintf("ideas_ready:%s:%s", e.RequestID, e.UserID),
		Status:        StatusPending,
		NextAttemptAt: now,
		CreatedAt:     now,
	}, nil
}
