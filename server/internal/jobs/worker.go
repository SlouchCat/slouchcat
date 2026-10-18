package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"
)

// Handler runs one job. It must be idempotent (see the package documentation) and return when
// ctx is cancelled.
type Handler func(ctx context.Context, j *Job) error

// finishTimeout bounds the database update after a handler returned, also during shutdown.
const finishTimeout = 5 * time.Second

// Worker claims jobs of its registered types and runs them one at a time. Run several workers
// for parallelism.
type Worker struct {
	queue     *Queue
	id        string
	log       *slog.Logger
	handlers  map[string]Handler
	poll      time.Duration
	heartbeat time.Duration
}

// NewWorker returns a worker with the unique id that claims from q.
func NewWorker(q *Queue, id string, log *slog.Logger) *Worker {
	return &Worker{
		queue:     q,
		id:        id,
		log:       log.With("worker", id),
		handlers:  map[string]Handler{},
		poll:      time.Second,
		heartbeat: HeartbeatInterval,
	}
}

// Handle registers h for jobs of type typ. Call it before Run.
func (w *Worker) Handle(typ string, h Handler) {
	w.handlers[typ] = h
}

// Run claims and runs jobs until ctx is cancelled. On cancellation the running handler sees its
// context cancelled; an interrupted job goes back to the queue without counting the attempt.
func (w *Worker) Run(ctx context.Context) {
	types := slices.Sorted(maps.Keys(w.handlers))
	for ctx.Err() == nil {
		job, err := w.queue.Claim(ctx, w.id, types)
		if err != nil && ctx.Err() == nil {
			w.log.Error("claim job", "err", err)
		}
		if job == nil {
			select {
			case <-ctx.Done():
			case <-time.After(w.poll):
			}
			continue
		}
		w.run(ctx, job)
	}
}

func (w *Worker) run(ctx context.Context, job *Job) {
	log := w.log.With("job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts)
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var leaseLost atomic.Bool
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(w.heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				err := w.queue.Heartbeat(jobCtx, w.id, job.ID)
				if errors.Is(err, ErrLeaseLost) {
					log.Info("job lease lost, stopping handler")
					leaseLost.Store(true)
					cancel()
					return
				}
				if err != nil && jobCtx.Err() == nil {
					log.Warn("heartbeat", "err", err)
				}
			}
		}
	}()

	start := time.Now()
	err := w.call(jobCtx, job)
	cancel()
	<-heartbeatDone

	fctx, fcancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer fcancel()
	var ferr error
	switch {
	case leaseLost.Load():
		return
	case err != nil && ctx.Err() != nil:
		log.Info("job interrupted by shutdown, released")
		ferr = w.queue.Release(fctx, w.id, job.ID)
	case err != nil:
		log.Warn("job failed", "err", err, "duration", time.Since(start))
		ferr = w.queue.Fail(fctx, w.id, job, err)
	default:
		log.Info("job done", "duration", time.Since(start))
		ferr = w.queue.Complete(fctx, w.id, job.ID)
	}
	if ferr != nil && !errors.Is(ferr, ErrLeaseLost) {
		log.Error("finish job", "err", ferr)
	}
}

// call runs the handler and turns a panic into an error.
func (w *Worker) call(ctx context.Context, job *Job) (err error) {
	h, ok := w.handlers[job.Type]
	if !ok {
		return fmt.Errorf("no handler for job type %q", job.Type)
	}
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("job handler panicked", "job_id", job.ID, "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return h(ctx, job)
}
