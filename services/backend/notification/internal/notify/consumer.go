package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Message is one delivery from the bus. The real implementation wraps a NATS
// JetStream message (internal/events); tests use a fake. Ack removes the message
// from the durable consumer; Nak asks for redelivery.
type Message interface {
	Data() []byte
	Ack() error
	Nak() error
}

// MessageHandler decodes and processes one raw event body. Returning nil means
// the event was durably recorded (the outbox row is committed) and the message
// may be acked; a non-nil error means the message must be redelivered.
type MessageHandler func(ctx context.Context, data []byte) error

// Subscription is a durable JetStream consumer. Consume blocks, handing each
// message to deliver until ctx is cancelled. The real adapter lives in
// internal/events.
type Subscription interface {
	Consume(ctx context.Context, deliver func(Message)) error
}

// Consumer turns bus events into outbox rows. It is deliberately thin: decode →
// EnqueueOutbox (idempotent). The heavy lifting (device resolution, pushing,
// retries) is the dispatcher's job, decoupled through the outbox.
type Consumer struct {
	store OutboxStore
	now   func() time.Time
}

// NewConsumer builds a Consumer. now defaults to time.Now.
func NewConsumer(store OutboxStore, now func() time.Time) *Consumer {
	if now == nil {
		now = time.Now
	}
	return &Consumer{store: store, now: now}
}

// HandlePollCompleted decodes a PollCompleted event and enqueues its outbox row.
func (c *Consumer) HandlePollCompleted(ctx context.Context, data []byte) error {
	var e PollCompletedEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return fmt.Errorf("decode PollCompleted: %w", err)
	}
	o, err := e.toOutbox(c.now())
	if err != nil {
		return err
	}
	return c.enqueue(ctx, o)
}

// HandleIdeasReady decodes an IdeasReady event and enqueues its outbox row.
func (c *Consumer) HandleIdeasReady(ctx context.Context, data []byte) error {
	var e IdeasReadyEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return fmt.Errorf("decode IdeasReady: %w", err)
	}
	o, err := e.toOutbox(c.now())
	if err != nil {
		return err
	}
	return c.enqueue(ctx, o)
}

func (c *Consumer) enqueue(ctx context.Context, o Outbox) error {
	inserted, err := c.store.EnqueueOutbox(ctx, o)
	if err != nil {
		return err
	}
	if !inserted {
		slog.InfoContext(ctx, "duplicate event ignored", "dedupe_key", o.DedupeKey)
	}
	return nil
}

// Process runs handler over msg, then acks on success or naks on failure. Ack
// happens only after the outbox row is committed, so a crash between commit and
// ack simply causes a redelivery that the unique dedupe_key absorbs — the event
// is never lost and never produces a duplicate notification.
func Process(ctx context.Context, msg Message, handler MessageHandler) {
	if err := handler(ctx, msg.Data()); err != nil {
		slog.ErrorContext(ctx, "event handling failed, will redeliver", "error", err)
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

// Run consumes sub, processing every message with handler until ctx is done.
func Run(ctx context.Context, sub Subscription, handler MessageHandler) error {
	return sub.Consume(ctx, func(msg Message) {
		Process(ctx, msg, handler)
	})
}
