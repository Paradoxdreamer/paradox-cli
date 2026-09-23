package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
)

// Handler processes a job. Return nil on success; any error triggers retry/DLQ.
type Handler func(ctx context.Context, job *Job) error

// Worker polls the store and runs handlers.
type Worker struct {
	Store    *Store
	Handlers map[string]Handler // type → handler
	Interval time.Duration
	// DefaultHandler is used when no type-specific handler is registered.
	DefaultHandler Handler
}

// Run blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	if w.Interval <= 0 {
		w.Interval = 500 * time.Millisecond
	}
	logging.Info("queue worker started", "interval", w.Interval.String())

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Info("queue worker stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				logging.Error("worker tick failed", "error", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	job, err := w.Store.ClaimNext()
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	logging.Info("job claimed", "id", job.ID, "type", job.Type, "attempt", job.Attempts)

	handler := w.Handlers[job.Type]
	if handler == nil {
		handler = w.DefaultHandler
	}
	if handler == nil {
		handler = func(ctx context.Context, j *Job) error {
			return fmt.Errorf("no handler for job type %q", j.Type)
		}
	}

	runErr := handler(ctx, job)
	if runErr != nil {
		logging.Warn("job failed", "id", job.ID, "error", runErr, "attempt", job.Attempts)
		return w.Store.Fail(job, runErr.Error())
	}
	logging.Info("job succeeded", "id", job.ID, "type", job.Type)
	return w.Store.Complete(job)
}
