package jobs

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Progress of a run (concept 09, 6.2). A handler calls Progress as often as it likes; the
// reporter the worker put into its context writes to the row at most every ProgressInterval and
// with every heartbeat, under the worker's lease, the note redacted. Skip marks a run that had
// nothing to do (a download waiting for budget, a scan of a library without a folder, an update
// check that is switched off): it does not count for the median duration of its type.

// Timing and bounds of the progress.
const (
	ProgressInterval = 2 * time.Second
	MaxProgressNote  = 200 // characters
)

// RunProgress is the progress a run reported: Done of Total (Total 0 = extent unknown) and a
// note on what it is at (a library and folder, a file relative to its root; never an absolute
// path, never a secret).
type RunProgress struct {
	Done  int64
	Total int64
	Note  string
}

// Progress reports the progress of the running job behind ctx; outside a job (tests, the
// synchronous scan of the library page) it does nothing.
func Progress(ctx context.Context, done, total int64, note string) {
	if r := reporterOf(ctx); r != nil {
		r.report(ctx, RunProgress{Done: done, Total: total, Note: note})
	}
}

// Skip marks the running job behind ctx as a run that had nothing to do.
func Skip(ctx context.Context) {
	if r := reporterOf(ctx); r != nil {
		r.skip(ctx)
	}
}

type reporterKey struct{}

func reporterOf(ctx context.Context) *reporter {
	r, _ := ctx.Value(reporterKey{}).(*reporter)
	return r
}

// reporter throttles the progress writes of one run.
type reporter struct {
	q      *Queue
	worker string
	id     int64
	log    *slog.Logger
	now    func() time.Time

	mu      sync.Mutex
	pending RunProgress
	dirty   bool
	written time.Time
}

func newReporter(q *Queue, worker string, id int64, log *slog.Logger) *reporter {
	return &reporter{q: q, worker: worker, id: id, log: log, now: q.now}
}

// report stores p and writes it when the last write is ProgressInterval ago.
func (r *reporter) report(ctx context.Context, p RunProgress) {
	r.mu.Lock()
	r.pending, r.dirty = p, true
	due := r.written.IsZero() || r.now().Sub(r.written) >= ProgressInterval
	r.mu.Unlock()
	if due {
		r.flush(ctx)
	}
}

// flush writes the pending progress when there is one (the heartbeat calls it, and the worker
// once more when the handler returned). A lost lease is not reported: the worker learns it from
// its heartbeat.
func (r *reporter) flush(ctx context.Context) {
	r.mu.Lock()
	if !r.dirty {
		r.mu.Unlock()
		return
	}
	p := r.pending
	r.dirty = false
	r.written = r.now()
	r.mu.Unlock()
	err := r.q.UpdateProgress(context.WithoutCancel(ctx), r.worker, r.id, p)
	if err != nil && !errors.Is(err, ErrLeaseLost) && ctx.Err() == nil {
		r.log.WarnContext(ctx, "write job progress", "err", err)
	}
}

func (r *reporter) skip(ctx context.Context) {
	err := r.q.MarkSkipped(context.WithoutCancel(ctx), r.worker, r.id)
	if err != nil && !errors.Is(err, ErrLeaseLost) && ctx.Err() == nil {
		r.log.WarnContext(ctx, "mark job skipped", "err", err)
	}
}

// UpdateProgress writes the progress of a running job under the worker's lease; the note is
// redacted (logging.RedactString) and cut to MaxProgressNote characters.
func (q *Queue) UpdateProgress(ctx context.Context, workerID string, id int64, p RunProgress) error {
	note := logging.RedactString(p.Note)
	if utf8.RuneCountInString(note) > MaxProgressNote {
		runes := []rune(note)
		note = string(runes[:MaxProgressNote-1]) + "…"
	}
	n, err := q.q.UpdateJobProgress(ctx, systemq.UpdateJobProgressParams{
		ProgressDone:  sql.NullInt64{Int64: max(p.Done, 0), Valid: true},
		ProgressTotal: sql.NullInt64{Int64: max(p.Total, 0), Valid: true},
		ProgressNote:  sql.NullString{String: note, Valid: note != ""},
		Now:           q.now().UnixMilli(), ID: id, WorkerID: sql.NullString{String: workerID, Valid: true},
	})
	return leaseResult("progress", n, err)
}

// MarkSkipped marks a running job as a run that had nothing to do, under the worker's lease.
func (q *Queue) MarkSkipped(ctx context.Context, workerID string, id int64) error {
	n, err := q.q.SetJobSkipped(ctx, systemq.SetJobSkippedParams{Now: q.now().UnixMilli(), ID: id, WorkerID: sql.NullString{String: workerID, Valid: true}})
	return leaseResult("skip", n, err)
}
