package pipeline

import (
	"context"
	"log/slog"

	"github.com/ilikyantigran/PerfectGift/services/backend/surprise/internal/events"
)

// Worker subscribes the pipeline to the durable GenerationRequested queue. The
// consumer's own concurrency (JetStream queue subscription) provides the pool;
// each delivered job runs Pipeline.Run.
type Worker struct {
	consumer events.Consumer
	pipe     *Pipeline
}

// NewWorker builds a Worker.
func NewWorker(consumer events.Consumer, pipe *Pipeline) *Worker {
	return &Worker{consumer: consumer, pipe: pipe}
}

// Start binds the consumer. It returns once the subscription is established; jobs
// are then processed as they arrive until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	return w.consumer.ConsumeGenerationRequested(ctx, func(ctx context.Context, job events.GenerationRequested) error {
		if err := w.pipe.Run(ctx, job); err != nil {
			slog.Error("pipeline run failed", "request_id", job.RequestID, "err", err)
			return err
		}
		return nil
	})
}
