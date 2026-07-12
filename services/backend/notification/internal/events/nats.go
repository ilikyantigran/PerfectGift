// Package events adapts NATS JetStream to the notify.Subscription interface.
// It is the production event source; the consumer logic itself lives in
// internal/notify and is tested with a fake Subscription, so this package is
// compiled but not exercised by `go test ./...`.
package events

import (
	"context"
	"fmt"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Bus is a JetStream connection from which durable subscriptions are built.
type Bus struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	stream string
}

// Connect dials NATS and prepares JetStream, ensuring the events stream exists
// (covering both consumed subjects).
func Connect(ctx context.Context, url, stream string, subjects []string) (*Bus, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	// Idempotent: create the stream if a producer hasn't already.
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     stream,
		Subjects: subjects,
	}); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure stream: %w", err)
	}
	return &Bus{nc: nc, js: js, stream: stream}, nil
}

// Close drains and closes the underlying connection.
func (b *Bus) Close() { b.nc.Close() }

// Subscription returns a durable JetStream consumer bound to a single subject.
// durable must be unique per subject so each consumed event type has its own
// independent, redelivering cursor.
func (b *Bus) Subscription(durable, subject string) notify.Subscription {
	return &subscription{bus: b, durable: durable, subject: subject}
}

type subscription struct {
	bus     *Bus
	durable string
	subject string
}

// Consume creates/binds the durable consumer and delivers each message to
// deliver until ctx is cancelled. AckExplicit means the message is redelivered
// unless the handler acks it — the basis for the never-lost guarantee.
//
// Each delivery extracts the W3C trace context carried in the message headers
// (injected by the producing service, e.g. surprise's PublishIdeasReady) and
// starts a consumer span from it, so deliver — and everything it does — stays
// linked to the request that produced the event instead of running under a
// fresh, span-less context.
func (s *subscription) Consume(ctx context.Context, deliver func(context.Context, notify.Message)) error {
	cons, err := s.bus.js.CreateOrUpdateConsumer(ctx, s.bus.stream, jetstream.ConsumerConfig{
		Durable:       s.durable,
		FilterSubject: s.subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("consumer %q: %w", s.durable, err)
	}

	cc, err := cons.Consume(func(m jetstream.Msg) {
		mctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(m.Headers()))
		mctx, span := otel.Tracer("nats").Start(mctx, "consume "+s.subject, trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()
		deliver(mctx, &message{m: m})
	})
	if err != nil {
		return fmt.Errorf("consume %q: %w", s.durable, err)
	}
	defer cc.Stop()

	<-ctx.Done()
	return nil
}

// message adapts a JetStream message to notify.Message.
type message struct{ m jetstream.Msg }

func (a *message) Data() []byte { return a.m.Data() }
func (a *message) Ack() error   { return a.m.Ack() }
func (a *message) Nak() error   { return a.m.Nak() }
