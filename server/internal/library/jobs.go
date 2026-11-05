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
	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Job types. The names are a contract with the packages that enqueue or handle them.
const (
	JobScanLibrary    = "scan.library"    // payload {"libraryId": n}, key scan:<id>
	JobScanPath       = "scan.path"       // payload pathScanPayload, key scan:<id>:path
	JobProbeFile      = "probe.file"      // payload {"mediaFileId": "<ULID>"}, key probe:<id>
	JobProbeKeyframes = "probe.keyframes" // payload {"mediaFileId": "<ULID>"}, key keyframes:<id>
	JobSweep          = "library.sweep"   // key sweep
	JobRelink         = "library.relink"  // payload {"itemId": "<ULID>"}, key relink:<id> (relink.go)
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
	// Trigger says what queued a full scan (model.ScanTrigger*, for the activity log of concept
	// 11): the first one wins while the job waits under its key. Empty in older jobs.
	Trigger string `json:"trigger,omitempty"`
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
	// ChapterImages asks probe.file to queue chapters.images after the probe (a film or an
	// episode file that appeared or changed after its library finished a scan, concept 13 11.6).
	ChapterImages bool `json:"chapterImages,omitempty"`
	// UnreadableSince is when chapters.images first found its media file unreadable (Unix ms; 0
	// while readable): the job gives up a week after it, not a week after it was queued.
	UnreadableSince int64 `json:"unreadableSince,omitempty"`
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

// probeJob probes a media file; chapterImages queues chapters.images after the probe.
func probeJob(mediaFileID string, chapterImages bool) newJob {
	return newJob{JobProbeFile, prioProbe, "probe:" + mediaFileID, mediaFilePayload{MediaFileID: mediaFileID, ChapterImages: chapterImages}}
}

func keyframesJob(mediaFileID string) newJob {
	return newJob{JobProbeKeyframes, prioKeyframes, "keyframes:" + mediaFileID, mediaFilePayload{MediaFileID: mediaFileID}}
}

// QueueKeyframesForPlayback queues the keyframe index of a media file at top priority (a playback
// is transcoding because the index is missing). A queued index job of a scan (priority 1) is
// replaced; a running one is left alone.
func QueueKeyframesForPlayback(ctx context.Context, q *jobs.Queue, mediaFileID string) error {
	payload, err := json.Marshal(mediaFilePayload{MediaFileID: mediaFileID})
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

// Keyframe sources: the values of media_files.keyframes_source (concept 14, 4.1).
const (
	KeyframesCues = string(mediaindex.SourceCues) // Matroska and WebM cues
	KeyframesMP4  = string(mediaindex.SourceMP4)  // MP4 sample tables
	KeyframesScan = "scan"                        // the full read (transcode.Runner.Keyframes)
	KeyframesNone = "none"                        // no index possible for this file version
)

// FileVersion is the size and mtime of a media file row, remembered before its file is read: a
// probe or an index is only written for the version it was computed from (concept 14, 4.2).
type FileVersion struct {
	Size  int64
	Mtime int64
}

// VersionOf returns the version of a media file row.
func VersionOf(f systemq.MediaFile) FileVersion { return FileVersion{Size: f.Size, Mtime: f.Mtime} }

// StoreKeyframes is the one writer of the keyframe index (concept 14, 4.2): it stores kf with its
// source only while the file has no index and the row still has version v (I-1, Z-9), on db (the
// write connection or the caller's transaction). An empty kf is stored as KeyframesNone without a
// blob. stored is false when an index was there already or the file changed meanwhile, which is
// not an error.
func StoreKeyframes(ctx context.Context, db systemq.DBTX, mediaFileID string, kf []time.Duration, source string, v FileVersion, now time.Time) (stored bool, err error) {
	var blob []byte
	if len(kf) == 0 {
		source = KeyframesNone
	} else {
		blob = EncodeKeyframes(kf)
	}
	n, err := systemq.New(db).SetMediaFileKeyframes(ctx, systemq.SetMediaFileKeyframesParams{
		Keyframes: blob, Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true},
		Source: sql.NullString{String: source, Valid: true}, ID: mediaFileID, Size: v.Size, Mtime: v.Mtime,
	})
	if err != nil {
		return false, fmt.Errorf("store keyframe index of %s: %w", mediaFileID, err)
	}
	return n > 0, nil
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

// RegisterTypes registers the job types of the library on r (concept 09, ADR 0017): the full
// scan and the sweep are schedule types (every 6 h and hourly, both at the start of the server),
// chapters.images_backfill is a manual type (concept 13 stage 3), the rest are event types queued
// by scans, watchers, webhooks, playbacks and matches (library.relink).
func (s *Service) RegisterTypes(r *jobs.Registry) {
	atStart := time.Duration(0)
	r.Register(jobs.Type{
		Name: JobScanLibrary, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioScan, Handler: s.handleScan, Run: s.runScans, Cancel: s.cancelScanJob, Every: fullScanInterval, StartDelay: &atStart,
	})
	r.Register(jobs.Type{Name: JobScanPath, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioPathScan, Handler: s.handlePathScan, Cancel: s.cancelScanJob})
	r.Register(jobs.Type{Name: JobProbeFile, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioProbe, Handler: s.handleProbe})
	r.Register(jobs.Type{Name: JobProbeKeyframes, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioKeyframes, Handler: s.handleKeyframes})
	r.Register(jobs.Type{
		Name: JobSweep, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioSweep, Handler: s.handleSweep, Run: s.runSweep, Every: sweepInterval, StartDelay: &atStart,
	})
	r.Register(jobs.Type{Name: JobSegmentsDetect, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioSegments, Handler: s.handleSegments})
	r.Register(jobs.Type{Name: JobRelink, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioMatch, Handler: s.handleRelink, Cancel: s.cancelRelink})
	r.Register(jobs.Type{Name: JobChapterImages, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioChapterImages, Handler: s.handleChapterImages})
	r.Register(jobs.Type{
		Name: JobChapterImagesBackfill, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerManual,
		Priority: prioChapterImages, Handler: s.handleChapterImagesBackfill, Run: s.runChapterImagesBackfill,
	})
}

// cancelScanJob is the cancel hook of scan.library and scan.path (jobs.Type.Cancel, concept 09
// 6.3): the administration's cancel of a scan job runs through CancelScan, so the library ends
// aborted/cancelled like with the button on the library page instead of staying "queued". A
// deleted library is no error: the queue cancels the job alone.
func (s *Service) cancelScanJob(ctx context.Context, j jobs.Job) error {
	var p libraryPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("decode %s payload: %w", j.Type, err)
	}
	_, err := s.CancelScan(ctx, p.LibraryID)
	if errors.Is(err, model.ErrNotFound) {
		return nil
	}
	return err
}

func decodePayload(j *jobs.Job, v any) error {
	if err := json.Unmarshal(j.Payload, v); err != nil {
		return fmt.Errorf("decode %s payload: %w", j.Type, err)
	}
	return nil
}
