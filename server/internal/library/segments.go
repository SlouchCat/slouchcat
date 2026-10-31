package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/segments"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// JobSegmentsDetect finds intro and outro of the episodes of a season (W-9, design M4 section 7):
// payload {"seasonId": "<ULID>"}, key segments:<seasonId>.
const JobSegmentsDetect = "segments.detect"

// prioSegments: after probes (3), before keyframe indexes (1).
const prioSegments = 2

// fingerprintWorkers is how many ffmpeg processes one segments.detect job runs at a time.
const fingerprintWorkers = 2

type seasonPayload struct {
	SeasonID string `json:"seasonId"`
}

func segmentsJob(seasonID string) newJob {
	return newJob{JobSegmentsDetect, prioSegments, "segments:" + seasonID, seasonPayload{seasonID}}
}

// QueuePendingSegments queues segments.detect for every season of the library whose detection is
// pending and that has at least two probed episodes; it returns the number of queued jobs. A scan
// of a series library calls it when it is done.
func (s *Service) QueuePendingSegments(ctx context.Context, libraryID int64) (int, error) {
	ids, err := systemq.New(s.db.Read).ListPendingSegmentSeasons(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list seasons with pending segments: %w", err)
	}
	for _, id := range ids {
		j := segmentsJob(id)
		payload, err := json.Marshal(j.payload)
		if err != nil {
			return 0, err
		}
		if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: j.priority, IdempotencyKey: j.key, Payload: payload}); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// markSeasonPending runs in the transaction of a successful probe: a new or changed episode file
// sets its season back to pending and, with at least two probed episodes, queues segments.detect.
// A job that is running already keeps the key; it then leaves the season pending (see
// SetSeasonSegmentsState) and the next full scan queues it again.
func markSeasonPending(ctx context.Context, q *systemq.Queries, mediaFileID string, now time.Time) error {
	row, err := q.MarkFileSeasonSegmentsPending(ctx, systemq.MarkFileSeasonSegmentsPendingParams{Now: now.UnixMilli(), MediaFileID: mediaFileID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil // not an episode
	}
	if err != nil {
		return fmt.Errorf("mark season segments pending: %w", err)
	}
	if row.Count < 2 {
		return nil
	}
	return enqueueTx(ctx, q, now, segmentsJob(row.ID))
}

// handleSegments fingerprints the start and the end of every probed episode of the season (two
// ffmpeg processes at a time), runs segments.Detect and replaces the detected segments of the
// episodes; manual segments stay. The season ends as done (at least one segment), none, or failed
// (no episode could be fingerprinted and no chapter helped). A file whose fingerprint fails is
// skipped; an interrupted job is retried.
func (s *Service) handleSegments(ctx context.Context, j *jobs.Job) error {
	var p seasonPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	started := s.now()
	season := sql.NullString{String: p.SeasonID, Valid: true}
	rows, err := systemq.New(s.db.Read).ListSegmentEpisodes(ctx, season)
	if err != nil {
		return fmt.Errorf("list episodes of season %s: %w", p.SeasonID, err)
	}
	opts := segments.DefaultOptions()
	eps := make([]segments.Episode, len(rows))
	fingerprinted := make([]bool, len(rows))
	sem := make(chan struct{}, fingerprintWorkers)
	var wg sync.WaitGroup
	var fingerprintsDone atomic.Int64
	jobs.Progress(ctx, 0, int64(len(rows)), "")
	for i, r := range rows {
		eps[i] = segments.Episode{ID: r.ItemID, Duration: time.Duration(r.DurationMs.Int64) * time.Millisecond, Chapters: storedChapters(r.Probe)}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// One episode of the season done, with or without a fingerprint (concept 09, 6.2).
			defer func() { jobs.Progress(ctx, fingerprintsDone.Add(1), int64(len(rows)), "") }()
			head, tail, err := s.fingerprintEpisode(ctx, r, eps[i].Duration, opts)
			if err != nil {
				if ctx.Err() == nil {
					s.log.WarnContext(ctx, "fingerprint failed, episode skipped", "media_file_id", r.MediaFileID, "err", err)
				}
				return
			}
			eps[i].Head, eps[i].Tail, fingerprinted[i] = head, tail, true
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	res := segments.Detect(eps, opts)

	state := "none"
	switch {
	case len(res) > 0:
		state = "done"
	case len(rows) > 0 && !slices.Contains(fingerprinted, true):
		state = "failed"
	}
	now := s.now()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	if err := q.DeleteDetectedSegments(ctx, season); err != nil {
		return fmt.Errorf("delete detected segments: %w", err)
	}
	for id, r := range res {
		for _, seg := range [...]struct {
			kind string
			sp   *segments.Span
		}{{"intro", r.Intro}, {"outro", r.Outro}} {
			kind, sp := seg.kind, seg.sp
			if sp == nil {
				continue
			}
			if err := q.InsertSegment(ctx, systemq.InsertSegmentParams{
				ItemID: id, Kind: kind, StartMs: sp.Start.Milliseconds(), EndMs: sp.End.Milliseconds(), Source: sp.Source,
				Confidence: sql.NullFloat64{Float64: sp.Confidence, Valid: true}, Now: now.UnixMilli(),
			}); err != nil {
				return fmt.Errorf("insert %s of %s: %w", kind, id, err)
			}
		}
	}
	n, err := q.SetSeasonSegmentsState(ctx, systemq.SetSeasonSegmentsStateParams{
		State: state, Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, SeasonID: p.SeasonID,
		Started: sql.NullInt64{Int64: started.UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store segments state: %w", err)
	}
	if n == 0 {
		// The segments of this run are stale: keep the stored ones (rollback) for the next run.
		s.log.InfoContext(ctx, "season changed during segment detection, stays pending", "season_id", p.SeasonID)
		return nil
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "segments detected", "season_id", p.SeasonID, "episodes", len(rows), "with_segments", len(res),
		"state", state, "duration", s.now().Sub(started))
	return nil
}

// fingerprintEpisode returns the fingerprints of the start and the end of an episode file.
func (s *Service) fingerprintEpisode(ctx context.Context, r systemq.ListSegmentEpisodesRow, d time.Duration, opts segments.Options) (head, tail []uint32, err error) {
	root, err := paths.NewRoot(r.RootPath)
	if err != nil {
		return nil, nil, err
	}
	abs, err := paths.Resolve(root, r.RelPath)
	if err != nil {
		return nil, nil, err
	}
	head, err = s.probe.Fingerprint(ctx, abs, 0, opts.HeadLength(d))
	if err != nil {
		return nil, nil, err
	}
	from, length := opts.TailRange(d)
	tail, err = s.probe.Fingerprint(ctx, abs, from, length)
	if err != nil {
		return nil, nil, err
	}
	return head, tail, nil
}

// storedChapters reads the chapters of the stored probe JSON (transcode.Probe).
func storedChapters(probe sql.NullString) []segments.Chapter {
	if !probe.Valid {
		return nil
	}
	var pr transcode.Probe
	if err := json.Unmarshal([]byte(probe.String), &pr); err != nil {
		return nil
	}
	out := make([]segments.Chapter, 0, len(pr.Chapters))
	for _, c := range pr.Chapters {
		out = append(out, segments.Chapter{Start: c.Start, End: c.End, Title: c.Title})
	}
	return out
}
