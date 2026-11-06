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
	// prioKeyframesDownload: an offline download of a remux waits for the index (concept 14, 6.8).
	prioKeyframesDownload = 5
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

// keyframesFailedQuiet is how long a playback queues no full read again after the last one of the
// file failed (concept 14, 6.4, O-17): the backfill tries it again.
const keyframesFailedQuiet = 24 * time.Hour

// QueueKeyframesForPlayback asks for the keyframe index of a media file a playback transcodes
// because the index is missing (concept 14, 6.4): probe.keyframes at priority 9 under the key
// keyframes:<id>, through jobs.Queue.EnqueueOverridingPause, so the job runs while its type is
// paused (at most jobs.MaxPauseOverrides at a time). A job that waits under the key (one of a
// scan or the backfill at priority 1) is raised instead (RaiseQueuedJobForPlayback); a running
// one stays as it is. Nothing is queued for a file whose index is impossible (source none), whose
// row is gone, or whose last full read failed within keyframesFailedQuiet. db is a read
// connection. The result is zero when nothing was queued or raised.
func QueueKeyframesForPlayback(ctx context.Context, db *sql.DB, q *jobs.Queue, mediaFileID string, now time.Time) (jobs.PauseOverride, error) {
	j, wanted, err := keyframesWanted(ctx, db, mediaFileID, now)
	if err != nil || !wanted {
		return jobs.PauseOverride{}, err
	}
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return jobs.PauseOverride{}, fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	res, err := q.EnqueueOverridingPause(ctx, jobs.NewJob{Type: j.typ, Priority: prioKeyframesPlayback, IdempotencyKey: j.key, Payload: payload})
	if err != nil || res.Inserted {
		return res, err
	}
	return q.RaiseQueuedJobForPlayback(ctx, res.ID, prioKeyframesPlayback)
}

// QueueKeyframesForDownload asks for the keyframe index of a media file an offline download of a
// remux waits for (concept 14, 6.8): probe.keyframes at priority 5 under the key keyframes:<id>,
// without the pause override (that exception is for playback only, F-3). A job that waits under
// the key is raised to that priority; a running one stays as it is. Like
// QueueKeyframesForPlayback it queues nothing for a file whose index is there or impossible,
// whose row is gone, or whose last full read failed within keyframesFailedQuiet; a full read that
// is already queued or running under the key (one of the backfill, which does not look at O-17)
// counts all the same. db is a read connection. queued reports whether a full read is queued or
// running now: without one the download has nothing to wait for and encodes.
func QueueKeyframesForDownload(ctx context.Context, db *sql.DB, q *jobs.Queue, mediaFileID string, now time.Time) (queued bool, err error) {
	active, err := systemq.New(db).GetActiveJobIDByKey(ctx, sql.NullString{String: keyframesJob(mediaFileID).key, Valid: true})
	switch {
	case err == nil:
		if _, err = q.RaiseQueuedPriority(ctx, active, prioKeyframesDownload); err != nil {
			return false, err
		}
		return true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return false, fmt.Errorf("active full keyframe read of %s: %w", mediaFileID, err)
	}
	j, wanted, err := keyframesWanted(ctx, db, mediaFileID, now)
	if err != nil || !wanted {
		return false, err
	}
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return false, fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: prioKeyframesDownload, IdempotencyKey: j.key, Payload: payload})
	if err != nil {
		return false, err
	}
	if _, err = q.RaiseQueuedPriority(ctx, id, prioKeyframesDownload); err != nil {
		return false, err
	}
	return true, nil
}

// keyframesWanted reports whether a full read of a media file may be queued on demand: its row
// exists without an index (none: impossible for this version; else written meanwhile) and its
// last full read did not fail within keyframesFailedQuiet (O-17). j is the job of that read.
func keyframesWanted(ctx context.Context, db *sql.DB, mediaFileID string, now time.Time) (j newJob, wanted bool, err error) {
	read := systemq.New(db)
	mf, err := read.GetMediaFileSystem(ctx, mediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return newJob{}, false, nil
	}
	if err != nil {
		return newJob{}, false, fmt.Errorf("load media file: %w", err)
	}
	if mf.KeyframesSource.Valid {
		return newJob{}, false, nil
	}
	j = keyframesJob(mediaFileID)
	failed, err := read.LastJobFailedSince(ctx, systemq.LastJobFailedSinceParams{
		Type: j.typ, IdempotencyKey: sql.NullString{String: j.key, Valid: true},
		Since: sql.NullInt64{Int64: now.Add(-keyframesFailedQuiet).UnixMilli(), Valid: true},
	})
	if err != nil {
		return newJob{}, false, fmt.Errorf("last full keyframe read of %s: %w", mediaFileID, err)
	}
	return j, !failed, nil
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
// the rest are event types queued by scans, watchers, webhooks, playbacks and matches
// (library.relink). keyframes.backfill (concept 14, 6.6) and chapters.images_backfill (concept 13
// stage 3, schedulable since concept 15 P-3) are schedule types every 7 days with the schedule off
// by default; probe.keyframes runs in the pool index.
func (s *Service) RegisterTypes(r *jobs.Registry) {
	atStart := time.Duration(0)
	r.Register(jobs.Type{
		Name: JobScanLibrary, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioScan, Handler: s.handleScan, Run: s.runScans, Cancel: s.cancelScanJob, Every: fullScanInterval, StartDelay: &atStart,
	})
	r.Register(jobs.Type{Name: JobScanPath, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioPathScan, Handler: s.handlePathScan, Cancel: s.cancelScanJob})
	r.Register(jobs.Type{Name: JobProbeFile, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioProbe, Handler: s.handleProbe})
	r.Register(jobs.Type{
		Name: JobProbeKeyframes, Group: jobs.GroupLibrary, Pool: jobs.PoolIndex, Trigger: jobs.TriggerEvent,
		Priority: prioKeyframes, Handler: s.handleKeyframes, OverridesPauseForPlayback: true,
	})
	r.Register(jobs.Type{
		Name: JobSweep, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioSweep, Handler: s.handleSweep, Run: s.runSweep, Every: sweepInterval, StartDelay: &atStart,
	})
	r.Register(jobs.Type{Name: JobSegmentsDetect, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioSegments, Handler: s.handleSegments})
	r.Register(jobs.Type{Name: JobRelink, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioMatch, Handler: s.handleRelink, Cancel: s.cancelRelink})
	r.Register(jobs.Type{Name: JobChapterImages, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: prioChapterImages, Handler: s.handleChapterImages})
	r.Register(jobs.Type{
		Name: JobChapterImagesBackfill, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioChapterImages, Handler: s.handleChapterImagesBackfill, Run: s.runChapterImagesBackfill,
		Every: chapterImagesBackfillEvery, Seed: scheduleOffSeed,
	})
	r.Register(s.keyframesBackfillType())
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
