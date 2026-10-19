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
	JobProbeFile      = "probe.file"      // payload {"mediaFileId": "<ULID>"}, key probe:<id>
	JobProbeKeyframes = "probe.keyframes" // payload {"mediaFileId": "<ULID>"}, key keyframes:<id>
	JobSweep          = "library.sweep"   // key sweep
	JobMetadataMatch  = "metadata.match"  // handled by package metadata; payload {"itemId": "<ULID>"}
)

// Job priorities (0 lowest, 9 highest).
const (
	prioScan      = 5
	prioMatch     = 4
	prioProbe     = 3
	prioSweep     = 2
	prioKeyframes = 1
)

type libraryPayload struct {
	LibraryID int64 `json:"libraryId"`
}

type mediaFilePayload struct {
	MediaFileID string `json:"mediaFileId"`
}

type itemPayload struct {
	ItemID string `json:"itemId"`
}

func scanKey(libraryID int64) string { return "scan:" + strconv.FormatInt(libraryID, 10) }

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

func matchJob(itemID string) newJob {
	return newJob{JobMetadataMatch, prioMatch, "match:" + itemID, itemPayload{itemID}}
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
	w.Handle(JobProbeFile, s.handleProbe)
	w.Handle(JobProbeKeyframes, s.handleKeyframes)
	w.Handle(JobSweep, s.handleSweep)
}

func decodePayload(j *jobs.Job, v any) error {
	if err := json.Unmarshal(j.Payload, v); err != nil {
		return fmt.Errorf("decode %s payload: %w", j.Type, err)
	}
	return nil
}
