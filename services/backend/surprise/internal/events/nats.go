package events

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// NATSConfig configures the JetStream producer/consumer.
type NATSConfig struct {
	URL            string
	Stream         string // e.g. SURPRISE
	RequestSubject string // e.g. surprise.generation.requested
	ReadySubject   string // e.g. surprise.ideas.ready
	DurableName    string // durable consumer name for the work queue
}

// NATSBus is the production Publisher+Consumer backed by NATS JetStream. The
// GenerationRequested subject is a durable work queue (survives worker restarts);
// IdeasReady is a plain pub/sub event.
type NATSBus struct {
	cfg NATSConfig
	nc  *nats.Conn
	js  nats.JetStreamContext
}

// NewNATSBus connects to NATS, ensures the JetStream stream exists, and returns
// the bus. Call Close on shutdown.
func NewNATSBus(cfg NATSConfig) (*NATSBus, error) {
	nc, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	if _, err := js.StreamInfo(cfg.Stream); err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{
			Name:      cfg.Stream,
			Subjects:  []string{cfg.RequestSubject, cfg.ReadySubject},
			Retention: nats.WorkQueuePolicy,
		}); err != nil {
			nc.Close()
			return nil, fmt.Errorf("add stream: %w", err)
		}
	}
	return &NATSBus{cfg: cfg, nc: nc, js: js}, nil
}

// Close drains the connection.
func (b *NATSBus) Close() {
	if b.nc != nil {
		_ = b.nc.Drain()
	}
}

func (b *NATSBus) PublishGenerationRequested(ctx context.Context, job GenerationRequested) error {
	data, err := marshal(job)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: b.cfg.RequestSubject, Data: data, Header: nats.Header{}}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))
	_, err = b.js.PublishMsg(msg)
	return err
}

func (b *NATSBus) PublishIdeasReady(ctx context.Context, evt IdeasReady) error {
	data, err := marshal(evt)
	if err != nil {
		return err
	}
	msg := &nats.Msg{Subject: b.cfg.ReadySubject, Data: data, Header: nats.Header{}}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))
	_, err = b.js.PublishMsg(msg)
	return err
}

// ConsumeGenerationRequested binds a durable queue subscription so the worker
// pool pulls jobs off the request path. Messages are acked on handler success and
// nak'd on failure for redelivery. The W3C trace context carried in the message
// headers (injected by PublishGenerationRequested) is extracted and used to start
// a consumer span, so the handler — and every log it emits — is linked back to the
// request that produced the job instead of running under a fresh, span-less context.
func (b *NATSBus) ConsumeGenerationRequested(ctx context.Context, h Handler) error {
	_, err := b.js.QueueSubscribe(b.cfg.RequestSubject, b.cfg.DurableName, func(msg *nats.Msg) {
		var job GenerationRequested
		if err := unmarshal(msg.Data, &job); err != nil {
			_ = msg.Term()
			return
		}
		// Extract into the live subscription ctx (not context.Background()) so the
		// handler keeps the app's cancellation/deadline for graceful shutdown while
		// still being parented to the remote span carried in the message headers.
		mctx := otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(msg.Header))
		mctx, span := otel.Tracer("nats").Start(mctx, "consume "+b.cfg.RequestSubject, trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End() // ended even if the handler panics, so the span never leaks
		if err := h(mctx, job); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}, nats.Durable(b.cfg.DurableName), nats.ManualAck(), nats.AckExplicit())
	return err
}
