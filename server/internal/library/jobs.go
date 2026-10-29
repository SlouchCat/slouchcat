package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Job types. The names are a contract with the packages that enqueue or handle them.
const (
	JobScanLibrary    = "scan.library"    // payload {"libraryId": n}, key scan:<id>
	JobScanPath       = "scan.path"       // payload pathScanPayload, key scan:<id>:path
	JobProbeFile      = "probe.file"      // payload {"mediaFileId": "<ULID>"}, key probe:<id>
	JobProbeKeyframes = "probe.keyframes" // payload {"mediaFileId": "<ULID>"}, key keyframes:<id>
	JobSweep          = "library.sweep"   // key sweep
	JobMetadataMatch  = "metadata.match"  // handled by package metadata; payload {"itemId": "<ULID>"}
	JobMetadataFill   = "metadata.fill"   // handled by package metadata; payload {"itemId": "<ULID>"}
	JobImageFetch     = "image.fetch"     // handled by package metadata; payload {"itemId": "<ULID>"}, key image:<id>
)

// Job priorities (0 lowest, 9 highest).
const (
	prioPathScan = 6
	prioScan     = 5
	prioMatch    = 4
	prioProbe    = 3
	// prioEpisodeMatch: a new episode of a matched series (the series job fills the others).
	prioEpisodeMatch = 3
	prioSweep        = 2
	// prioImage is the priority of image.fetch for the cover of an audiobook (metadata's posters).
	prioImage     = 2
	prioKeyframes = 1
	// prioKeyframesPlayback: a playback fell back to transcoding because the index is missing;
	// the index runs before everything else so the next start remuxes.
	prioKeyframesPlayback = 9
)

type libraryPayload struct {
	LibraryID int64 `json:"libraryId"`
}

// pathScanPayload names the directories a path scan reads: per root of the library, directories
// slash-separated and relative to the root ("." is the root itself), sorted and unique.
// Example: {"libraryId": 1, "roots": [{"rootId": 1, "dirs": [".", "Alien (1979)"]}]}.
type pathScanPayload struct {
	LibraryID int64          `json:"libraryId"`
	Roots     []pathScanRoot `json:"roots"`
}

type pathScanRoot struct {
	RootID int64    `json:"rootId"`
	Dirs   []string `json:"dirs"`
}

type mediaFilePayload struct {
	MediaFileID string `json:"mediaFileId"`
}

type itemPayload struct {
	ItemID string `json:"itemId"`
}

func scanKey(libraryID int64) string { return "scan:" + strconv.FormatInt(libraryID, 10) }

func pathScanKey(libraryID int64) string { return scanKey(libraryID) + ":path" }

// newJob describes a job that is enqueued inside a scan or probe transaction.
type newJob struct {
	typ      string
	priority int
	key      string
	payload  any
}

func probeJob(mediaFileID string) newJob {
	return newJob{JobProbeFile, prioProbe, "probe:" + mediaFileID, mediaFilePayload{mediaFileID}}
}

func keyframesJob(mediaFileID string) newJob {
	return newJob{JobProbeKeyframes, prioKeyframes, "keyframes:" + mediaFileID, mediaFilePayload{mediaFileID}}
}

// QueueKeyframesForPlayback queues the keyframe index of a media file at top priority (a playback
// is transcoding because the index is missing). A queued index job of a scan (priority 1) is
// replaced; a running one is left alone.
func QueueKeyframesForPlayback(ctx context.Context, q *jobs.Queue, mediaFileID string) error {
	payload, err := json.Marshal(mediaFilePayload{mediaFileID})
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", JobProbeKeyframes, err)
	}
	nj := jobs.NewJob{Type: JobProbeKeyframes, Priority: prioKeyframesPlayback, IdempotencyKey: "keyframes:" + mediaFileID, Payload: payload}
	id, err := q.Enqueue(ctx, nj)
	if err != nil {
		return err
	}
	j, err := q.Get(ctx, id)
	if err != nil {
		return err
	}
	if j.State != jobs.StateQueued || j.Priority >= prioKeyframesPlayback {
		return nil
	}
	if _, err := q.Cancel(ctx, id); err != nil {
		return err
	}
	_, err = q.Enqueue(ctx, nj)
	return err
}

// StoreKeyframes stores a keyframe index that playback computed on demand, like the index job.
func StoreKeyframes(ctx context.Context, db *sql.DB, mediaFileID string, kf []time.Duration, now time.Time) error {
	err := systemq.New(db).UpdateMediaFileKeyframes(ctx, systemq.UpdateMediaFileKeyframesParams{
		Keyframes: EncodeKeyframes(kf), Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, ID: mediaFileID,
	})
	if err != nil {
		return fmt.Errorf("store keyframe index of %s: %w", mediaFileID, err)
	}
	return nil
}

func matchJob(itemID string) newJob {
	return newJob{JobMetadataMatch, prioMatch, "match:" + itemID, itemPayload{itemID}}
}

// fillJob queues metadata.fill at the priority of image.fetch, like the match does.
func fillJob(itemID string) newJob {
	return newJob{JobMetadataFill, prioImage, "fill:" + itemID, itemPayload{itemID}}
}

func imageJob(itemID string) newJob {
	return newJob{JobImageFetch, prioImage, "image:" + itemID, itemPayload{itemID}}
}

func episodeMatchJob(itemID string) newJob {
	return newJob{JobMetadataMatch, prioEpisodeMatch, "match:" + itemID, itemPayload{itemID}}
}

// enqueueTx adds a job in the caller's transaction, so rows and their follow-up jobs commit
// together and a crash cannot lose the job. Semantics match jobs.Queue.Enqueue: an active job
// with the same idempotency key wins, which makes repeated scans add no duplicates.
func enqueueTx(ctx context.Context, q *systemq.Queries, now time.Time, j newJob) error {
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	_, err = q.InsertJob(ctx, systemq.InsertJobParams{
		Type:           j.typ,
		Priority:       int64(j.priority),
		IdempotencyKey: sql.NullString{String: j.key, Valid: true},
		Payload:        string(payload),
		RunAfter:       now.UnixMilli(),
		MaxAttempts:    jobs.DefaultMaxAttempts,
		Now:            now.UnixMilli(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil // an active job holds the key
	}
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", j.typ, err)
	}
	return nil
}

// RegisterJobs registers the handlers of the library jobs on w.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobScanLibrary, s.handleScan)
	w.Handle(JobScanPath, s.handlePathScan)
	w.Handle(JobProbeFile, s.handleProbe)
	w.Handle(JobProbeKeyframes, s.handleKeyframes)
	w.Handle(JobSweep, s.handleSweep)
	w.Handle(JobSegmentsDetect, s.handleSegments)
}

func decodePayload(j *jobs.Job, v any) error {
	if err := json.Unmarshal(j.Payload, v); err != nil {
		return fmt.Errorf("decode %s payload: %w", j.Type, err)
	}
	return nil
}
