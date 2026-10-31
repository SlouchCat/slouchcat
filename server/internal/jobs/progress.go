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
// with every heartbeat, under the worker's lease, the note redacted. A report that comes too
// early is written by a timer ProgressInterval after the last write, so a phase that ends within
// the interval does not stay invisible until the heartbeat (docs/perf/hintergrundjobs.md, B-1).
// Skip marks a run that had nothing to do (a download waiting for budget, a scan of a library
// without a folder or without a change, an update check that is switched off): it does not count
// for the median duration of its type.

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
	if r := sinkOf(ctx); r != nil {
		r.report(ctx, RunProgress{Done: done, Total: total, Note: note})
	}
}

// Skip marks the running job behind ctx as a run that had nothing to do.
func Skip(ctx context.Context) {
	if r := sinkOf(ctx); r != nil {
		r.skip(ctx)
	}
}

type reporterKey struct{}

// progressSink receives what a handler reports: the worker's reporter, or a Recorder in tests.
type progressSink interface {
	report(ctx context.Context, p RunProgress)
	skip(ctx context.Context)
}

func sinkOf(ctx context.Context) progressSink {
	r, _ := ctx.Value(reporterKey{}).(progressSink)
	return r
}

// Recorder collects what a handler reports, for the tests of the handlers (nothing is written).
type Recorder struct {
	mu      sync.Mutex
	Reports []RunProgress
	Skipped bool
}

// WithRecorder returns a context whose Progress and Skip calls land in rec.
func WithRecorder(ctx context.Context, rec *Recorder) context.Context {
	return context.WithValue(ctx, reporterKey{}, rec)
}

func (rec *Recorder) report(_ context.Context, p RunProgress) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.Reports = append(rec.Reports, p)
}

func (rec *Recorder) skip(context.Context) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.Skipped = true
}

// Last returns the last report and whether there was one.
func (rec *Recorder) Last() (RunProgress, bool) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.Reports) == 0 {
		return RunProgress{}, false
	}
	return rec.Reports[len(rec.Reports)-1], true
}

// reporter throttles the progress writes of one run and remembers where the current progress
// series began (the measured estimate rates the work since then, B-3).
type reporter struct {
	q        *Queue
	worker   string
	id       int64
	log      *slog.Logger
	now      func() time.Time
	interval time.Duration

	mu      sync.Mutex
	pending RunProgress
	dirty   bool
	written time.Time
	timer   *time.Timer // writes a pending report when it is due; nil when none waits
	last    RunProgress // the last report, written or not
	// since and sinceDone mark the first report of the current series: the first report with
	// an extent, and again whenever the extent changes or done falls (a new phase).
	since     time.Time
	sinceDone int64
}

func newReporter(q *Queue, worker string, id int64, log *slog.Logger) *reporter {
	return &reporter{q: q, worker: worker, id: id, log: log, now: q.now, interval: ProgressInterval}
}

// report stores p and writes it when the last write is interval ago; otherwise a timer writes
// it when that moment comes, unless a later report or the heartbeat wrote first.
func (r *reporter) report(ctx context.Context, p RunProgress) {
	r.mu.Lock()
	now := r.now()
	switch {
	case p.Total <= 0:
		r.since, r.sinceDone = time.Time{}, 0
	case r.since.IsZero() || p.Total != r.last.Total || p.Done < r.last.Done:
		r.since, r.sinceDone = now, p.Done
	}
	r.last = p
	r.pending, r.dirty = p, true
	due := r.written.IsZero() || now.Sub(r.written) >= r.interval
	if !due && r.timer == nil {
		r.timer = time.AfterFunc(r.interval-now.Sub(r.written), func() { r.flush(ctx) })
	}
	r.mu.Unlock()
	if due {
		r.flush(ctx)
	}
}

// flush writes the pending progress when there is one (the heartbeat and the timer call it,
// and the worker once more when the handler returned). A lost lease is not reported: the worker
// learns it from its heartbeat.
func (r *reporter) flush(ctx context.Context) {
	r.mu.Lock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
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

// stop drops a waiting timer; the worker calls it after the final flush.
func (r *reporter) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// series returns when the current progress series began and the done count it began with;
// ok is false before the first report with an extent.
func (r *reporter) series() (since time.Time, done int64, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.since, r.sinceDone, !r.since.IsZero()
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
