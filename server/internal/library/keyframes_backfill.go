package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The keyframe backfill (concept 14, 6.6, F-4): one long run over the indexworthy files without an
// index that reads the index from the container where it passes the checks and queues the full
// read (probe.keyframes) only for the rest. It is a schedule type whose schedule is off by default
// (O-6): «Jetzt ausführen» is the usual start, an administrator switches the interval or a one-off
// date on.
const (
	JobKeyframesBackfill = "keyframes.backfill" // key keyframes.backfill
	keyframesBackfillKey = "keyframes.backfill"
	// prioKeyframesBackfill is the priority of a scheduled run (a manual one runs at
	// jobs.ManualPriority); the full reads it queues run at prioKeyframes.
	prioKeyframesBackfill = 2
	// keyframesBackfillEvery is the default interval: the maximum of job_schedules.every_ms.
	keyframesBackfillEvery = 7 * 24 * time.Hour
	// keyframesBackfillBatch is how many files one query of the cursor returns.
	keyframesBackfillBatch = 200
	// keyframesBackfillTimeouts is how many container reads in a row may time out before the run
	// stops with an error (a hanging network share, 6.1); the queue retries it after its backoff.
	keyframesBackfillTimeouts = 3
)

// errBackfillTimeouts: keyframesBackfillTimeouts container reads in a row timed out.
var errBackfillTimeouts = errors.New("container reads time out")

// scheduleOffSeed is the Seed of the backfills: their first schedule row is off (O-6, concept 15
// P-3); a run starts by hand until the administrator turns the schedule on.
func scheduleOffSeed(context.Context) (jobs.ScheduleSeed, error) {
	return jobs.ScheduleSeed{Enabled: false}, nil
}

// keyframesBackfillType is the job type of the backfill (RegisterTypes).
func (s *Service) keyframesBackfillType() jobs.Type {
	return jobs.Type{
		Name: JobKeyframesBackfill, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioKeyframesBackfill, Every: keyframesBackfillEvery,
		Seed:    scheduleOffSeed,
		Run:     s.runKeyframesBackfill,
		Handler: s.handleKeyframesBackfill,
	}
}

// runKeyframesBackfill is the Run of keyframes.backfill: one run under its key at prio (a waiting
// one is raised to it).
func (s *Service) runKeyframesBackfill(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobKeyframesBackfill, Priority: prio, IdempotencyKey: keyframesBackfillKey})
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobKeyframesBackfill, err)
	}
	return []int64{id}, nil
}

// backfillRoot is a library root the backfill opened, nil dir when it is gone.
type backfillRoot struct {
	root paths.Root
	dir  *os.Root
}

// handleKeyframesBackfill walks the indexworthy files without an index by id
// (ListKeyframesBackfillFiles, keyframesBackfillBatch at a time). Per file it reads the container index outside any
// transaction (indexFromContainer, with the version remembered from the row) and then writes once:
// the index when it passed the checks, else probe.keyframes at prioKeyframes (an active job of the
// file holds the key, so nothing is doubled). A file whose container read timed out (a hanging
// share; the next run tries it again) and a missing file get no full read. The progress is done
// of total with the file relative to its root; it ends at done of done, since probe.file may index
// files of the count meanwhile. A pause or a cancel ends the run through ctx; the next run starts over and the
// condition keyframes_source IS NULL leaves out what is done.
func (s *Service) handleKeyframesBackfill(ctx context.Context, _ *jobs.Job) error {
	read := systemq.New(s.db.Read)
	total, err := read.CountKeyframesBackfillFiles(ctx)
	if err != nil {
		return fmt.Errorf("count files without a keyframe index: %w", err)
	}
	roots := map[int64]*backfillRoot{}
	defer func() {
		for _, r := range roots {
			if r.dir != nil {
				_ = r.dir.Close()
			}
		}
	}()
	var done, stored, queued int64
	timeouts := 0
	after := ""
	for {
		rows, err := read.ListKeyframesBackfillFiles(ctx, systemq.ListKeyframesBackfillFilesParams{AfterID: after, LimitCount: keyframesBackfillBatch})
		if err != nil {
			return fmt.Errorf("list files without a keyframe index: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, f := range rows {
			after = f.ID
			jobs.Progress(ctx, done, max(total, done), f.RelPath)
			r, err := s.backfillRoot(ctx, roots, f.RootID)
			if err != nil {
				return err
			}
			if r == nil || r.dir == nil {
				done++
				continue // the root is gone: the scan marks its files missing
			}
			abs, err := paths.Resolve(r.root, f.RelPath)
			if err != nil {
				return err
			}
			index, err := s.indexFromContainer(ctx, indexTarget{
				id: f.ID, root: r.root, dir: r.dir, rel: f.RelPath, abs: abs, container: f.Container,
				duration: time.Duration(f.DurationMs.Int64) * time.Millisecond, codec: f.VideoCodec.String, kind: f.Kind,
			})
			if err != nil {
				return err
			}
			switch {
			case len(index.keyframes) > 0:
				ok, err := StoreKeyframes(ctx, s.db.Write, f.ID, index.keyframes, index.source, FileVersion{Size: f.Size, Mtime: f.Mtime}, s.now())
				if err != nil {
					return err
				}
				if ok {
					stored++
				}
			case index.worthy && !index.timedOut && !index.missing:
				if err := s.enqueueFullRead(ctx, f.ID); err != nil {
					return err
				}
				queued++
			}
			done++
			if !index.timedOut {
				timeouts = 0
				continue
			}
			if timeouts++; timeouts >= keyframesBackfillTimeouts {
				return fmt.Errorf("keyframe backfill: %d %w in a row", timeouts, errBackfillTimeouts)
			}
		}
	}
	jobs.Progress(ctx, done, done, "")
	if done == 0 {
		jobs.Skip(ctx)
	}
	s.log.InfoContext(ctx, "keyframe backfill done", "files", done, "stored", stored, "full_reads_queued", queued)
	return nil
}

// backfillRoot returns the opened library root id from roots, opening it on first use; nil when
// the root row is gone, a nil dir when its directory is.
func (s *Service) backfillRoot(ctx context.Context, roots map[int64]*backfillRoot, id int64) (*backfillRoot, error) {
	if r, ok := roots[id]; ok {
		return r, nil
	}
	row, err := appq.New(s.db.Read).GetLibraryRoot(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		roots[id] = nil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load library root: %w", err)
	}
	root, err := paths.NewRoot(row.Path)
	if err != nil {
		return nil, err
	}
	r := &backfillRoot{root: root}
	if r.dir, err = paths.Open(root); err != nil {
		// The error of paths.Open names the absolute root; the log keeps only its cause (9.5).
		reason := "not readable"
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			reason = pathErr.Err.Error()
		}
		s.log.WarnContext(ctx, "library root not readable, keyframe backfill leaves its files", "root_id", id, "err", reason)
	}
	roots[id] = r
	return r, nil
}

// enqueueFullRead queues probe.keyframes of a file at prioKeyframes; an active job of the file
// holds the key and stays as it is.
func (s *Service) enqueueFullRead(ctx context.Context, mediaFileID string) error {
	j := keyframesJob(mediaFileID)
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	_, err = s.queue.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: j.priority, IdempotencyKey: j.key, Payload: payload})
	return err
}
