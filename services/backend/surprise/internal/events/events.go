// Package events is the NATS boundary. The Surprise service PRODUCES
// GenerationRequested (a durable work-queue job) and IdeasReady (a pub/sub event),
// and CONSUMES GenerationRequested with its own worker pool. The interfaces here
// let the server and pipeline stay decoupled from JetStream; the real
// implementation is in nats.go and an in-memory Bus fake drives the tests.
package events

import (
	"context"
	"encoding/json"
	"sync"
)

// GenerationRequested is the durable job payload consumed by the worker pool.
type GenerationRequested struct {
	RequestID string `json:"request_id"`
	Tier      string `json:"tier"`
}

// IdeasReady is the pub/sub event emitted when ideas are persisted.
type IdeasReady struct {
	RequestID string `json:"request_id"`
	UserID    string `json:"user_id"`
	IdeaCount int    `json:"idea_count"`
}

// Publisher emits jobs and events.
type Publisher interface {
	PublishGenerationRequested(ctx context.Context, job GenerationRequested) error
	PublishIdeasReady(ctx context.Context, evt IdeasReady) error
}

// Handler processes one GenerationRequested job.
type Handler func(ctx context.Context, job GenerationRequested) error

// Consumer subscribes the worker pool to the durable GenerationRequested queue.
type Consumer interface {
	ConsumeGenerationRequested(ctx context.Context, h Handler) error
}

// Bus is an in-memory Publisher+Consumer for tests. It records everything
// published and, when a handler is registered, dispatches GenerationRequested
// jobs to it synchronously on publish (so a test can drive the full loop).
type Bus struct {
	mu       sync.Mutex
	handler  Handler
	Jobs     []GenerationRequested
	Ready    []IdeasReady
	dispatch bool
}

// NewBus builds an in-memory bus. If dispatch is true, publishing a job invokes
// the registered handler immediately (useful for end-to-end pipeline tests).
func NewBus(dispatch bool) *Bus { return &Bus{dispatch: dispatch} }

func (b *Bus) PublishGenerationRequested(ctx context.Context, job GenerationRequested) error {
	b.mu.Lock()
	b.Jobs = append(b.Jobs, job)
	h := b.handler
	dispatch := b.dispatch
	b.mu.Unlock()
	if dispatch && h != nil {
		return h(ctx, job)
	}
	return nil
}

func (b *Bus) PublishIdeasReady(_ context.Context, evt IdeasReady) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Ready = append(b.Ready, evt)
	return nil
}

func (b *Bus) ConsumeGenerationRequested(_ context.Context, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handler = h
	return nil
}

// marshal/unmarshal helpers shared with the real implementation.
func marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
