// Package events publishes the Poll service's domain events to NATS JetStream. It
// implements the ports.Publisher port. The only event is PollCompleted, consumed
// by Notification (and later analytics).
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ilikyantigran/PerfectGift/services/backend/poll/internal/ports"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Publisher struct {
	nc      *nats.Conn
	js      jetstream.JetStream
	subject string
}

// NewPublisher connects to NATS, ensures the stream exists, and returns a
// publisher bound to the completion subject.
func NewPublisher(ctx context.Context, url, stream, subject string) (*Publisher, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     stream,
		Subjects: []string{subject},
	}); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure stream %q: %w", stream, err)
	}
	return &Publisher{nc: nc, js: js, subject: subject}, nil
}

func (p *Publisher) PublishPollCompleted(ctx context.Context, ev ports.PollCompleted) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := p.js.Publish(ctx, p.subject, b); err != nil {
		return fmt.Errorf("publish %s: %w", p.subject, err)
	}
	return nil
}

func (p *Publisher) Close() {
	if p.nc != nil {
		_ = p.nc.Drain()
	}
}
