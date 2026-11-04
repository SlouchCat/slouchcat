package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Chapter images (concept 13 stage 3, 11.6, V-19): one WebP of 480 px per chapter of a film or an
// episode (transcode.Chapters, transcode.GrabFrame), stored below the cache directory in
// chapters/<mediaFileId>-<version>/<idx>-w480.webp. The version (ChapterImagesVersion) is the probe
// time of the file and the fingerprint of its chapter list, so a new probe and other chapter marks
// (also from other chapter rules) give a new folder and new URLs. The folder is written under a
// temporary name and renamed when the job is done, so an existing folder is a finished set; an
// empty one means «done, no images» (a file without chapters, or one whose frames ffmpeg could not
// make). The hourly library.sweep removes the folders of other versions, of files that are gone or
// whose title was deleted, and stale temporary folders (sweepChapterImages: media files leave
// through several deletes and foreign-key cascades, a sweep catches every one of them).
//
//   - chapters.images, payload {"mediaFileId": "<ULID>"}, key chapters:<id>, priority 5: an event
//     type, queued by the probe of a file that a scan added or changed after a complete scan of its
//     library (probeJob's chapterImages), by the probe of a file that had images for its previous
//     probe, and by the playback sweep for the files of playbacks that ended
//     (QueueChapterImagesAfterPlayback, 10 minutes later). It waits while the file plays.
//   - chapters.images_backfill, key chapters_images_backfill: a manual type (Run), started only by
//     hand on the page Hintergrundjobs; it queues chapters.images at priority 1 for every probed
//     film and episode file without images.
const (
	JobChapterImages         = "chapters.images"
	JobChapterImagesBackfill = "chapters.images_backfill"
	prioChapterImages        = 5
	// prioChapterImagesBackfill is the priority of the jobs the backfill queues: below every other
	// job (Marco's decision of 4 November 2026, m-5), so a backfill of the whole collection never
	// holds up probes, matches and images of new titles.
	prioChapterImagesBackfill = 1
	chapterImagesBackfillKey  = "chapters_images_backfill"
	// ChapterImagesDir is the folder of the chapter images below the cache directory.
	ChapterImagesDir = "chapters"
	// chapterTempPrefix starts the name of a folder a job is writing; the sweep removes one that
	// is older than chapterTempMaxAge (a job that died with the server).
	chapterTempPrefix = ".tmp-"
	chapterTempMaxAge = 6 * time.Hour
	// ChapterImagesAfterPlayback is how long after the end of a playback its chapter images job
	// waits: a track, quality or version switch ends a playback session and starts the next one
	// at once, and the title should have stopped playing for good.
	ChapterImagesAfterPlayback = 10 * time.Minute
	// chapterPlayingWindow: a playback session with a request (token or progress event) in this
	// window counts as playing, the same rule as playback.ActiveWindow for the stream limits and
	// the stream overview (an import of playback would be a cycle); a session the client never
	// stopped stops counting two minutes after its last request. While one plays the file, the job
	// waits chapterPlayingRetry.
	chapterPlayingWindow = 2 * time.Minute
	chapterPlayingRetry  = 10 * time.Minute
	// chapterUnreadableRetry is the wait when the media file cannot be read (a share that is
	// offline): no attempt is used and no «done» folder is written.
	chapterUnreadableRetry = time.Hour
	// chapterUnreadableGiveUp: a job whose file stays unreadable this long (from the first
	// unreadable run, mediaFilePayload.UnreadableSince) ends without folder (the next probe,
	// playback or backfill queues it again).
	chapterUnreadableGiveUp = 7 * 24 * time.Hour
	// chapterReadTimeout bounds the read test of the media file (a hanging share).
	chapterReadTimeout = 10 * time.Second
	// chapterChangedRetry is the wait after a new probe arrived while the job ran.
	chapterChangedRetry = time.Minute
	// chapterFrameTimeout bounds one GrabFrame run (one frame, a few MB read; a stuck read of a
	// network share must not hold the worker).
	chapterFrameTimeout = 60 * time.Second
)

// ChapterImagesVersion is the version of the chapter images of a media file: its probe time and the
// fingerprint of its chapter list (transcode.ChapterFingerprint), "<probedAt>-<hash8>". It names
// the folder and is the v of the image URLs.
func ChapterImagesVersion(probedAt int64, chapters []model.Chapter) string {
	return strconv.FormatInt(probedAt, 10) + "-" + transcode.ChapterFingerprint(chapters)
}

// ChapterImagesRel returns the folder of the chapter images of a media file at a version, relative
// to the cache directory (slash-separated).
func ChapterImagesRel(mediaFileID, version string) string {
	return ChapterImagesDir + "/" + mediaFileID + "-" + version
}

// ChapterImageName is the file name of the image of chapter idx (0-based) in its folder.
func ChapterImageName(idx int) string {
	return strconv.Itoa(idx) + "-w" + strconv.Itoa(transcode.ChapterImageWidth) + ".webp"
}

func chapterImagesJob(mediaFileID string) newJob {
	return newJob{JobChapterImages, prioChapterImages, "chapters:" + mediaFileID, mediaFilePayload{MediaFileID: mediaFileID}}
}

// chapterKind reports whether titles of kind get chapter images: films and episodes (11.6).
func chapterKind(kind string) bool { return kind == "movie" || kind == "episode" }

// storedChapterImages parses a stored probe and returns it with the chapters of the file
// (transcode.Chapters) and the version of their images; ok is false for a probe that does not
// parse.
func storedChapterImages(probe sql.NullString, kind string, durationMs, probedAt int64) (pr transcode.Probe, chapters []model.Chapter, version string, ok bool) {
	if !probe.Valid || json.Unmarshal([]byte(probe.String), &pr) != nil {
		return transcode.Probe{}, nil, "", false
	}
	chapters = transcode.Chapters(&pr, kind, durationMs)
	return pr, chapters, ChapterImagesVersion(probedAt, chapters), true
}

// errReadHanging: an earlier read test of the same media root still hangs.
var errReadHanging = errors.New("an earlier read of this folder still hangs")

// readable opens the file at path (resolved through package paths below the library root root)
// and reads its first 64 KiB within timeout: a stat alone succeeds on a share whose data cannot be
// read. A read that hangs past timeout counts as unreadable; its goroutine stays in the kernel
// (and holds an OS thread) until the read returns. While one hangs, every further file of the same
// root counts as unreadable at once without a new goroutine, so a backfill over a hard-mounted
// share that went away leaves one stuck thread per root, not one per file.
func (s *Service) readable(root, path string, timeout time.Duration) error {
	if _, hanging := s.hangingReads.Load(root); hanging {
		return errReadHanging
	}
	var mu sync.Mutex // guards finished and timedOut
	finished, timedOut := false, false
	done := make(chan error, 1)
	go func() {
		err := readHead(path)
		mu.Lock()
		finished = true
		if timedOut {
			s.hangingReads.Delete(root) // the hanging read returned: the root may be tried again
		}
		mu.Unlock()
		done <- err
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		mu.Lock()
		if finished { // returned while the timer fired
			mu.Unlock()
			return <-done
		}
		timedOut = true
		s.hangingReads.Store(root, struct{}{})
		mu.Unlock()
		return fmt.Errorf("read of %s timed out after %v", filepath.Base(path), timeout)
	}
}

// readHead opens path and reads its first 64 KiB.
func readHead(path string) error {
	f, err := os.Open(path) //nolint:gosec // G304: path resolved through package paths below a library root
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Read(make([]byte, 64<<10))
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return err
}

// hasChapterImages reports whether the folder rel exists below the cache root.
func hasChapterImages(cache paths.Root, rel string) bool {
	abs, err := paths.Resolve(cache, rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(abs)
	return err == nil
}

// QueueChapterImagesAfterPlayback queues chapters.images, ChapterImagesAfterPlayback from now, for
// the probed film and episode files among mediaFileIDs that have no images (11.6 (2)): the playback
// sweep calls it with the files of the playbacks that ended. Anything else (no cache directory,
// another kind, an unprobed, missing, deleted or unknown file, images present) queues nothing. A
// waiting job of the backfill under the same key is raised to the event's priority 5. A file that
// fails does not hold up the others: the errors come back joined.
func QueueChapterImagesAfterPlayback(ctx context.Context, db *sql.DB, q *jobs.Queue, cacheDir string, mediaFileIDs []string) error {
	if cacheDir == "" || len(mediaFileIDs) == 0 {
		return nil
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return err
	}
	read := systemq.New(db)
	var errs []error
	for _, id := range mediaFileIDs {
		if err := queueAfterPlayback(ctx, read, q, cache, id); err != nil {
			errs = append(errs, fmt.Errorf("media file %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// queueAfterPlayback is QueueChapterImagesAfterPlayback for one file.
func queueAfterPlayback(ctx context.Context, read *systemq.Queries, q *jobs.Queue, cache paths.Root, id string) error {
	row, err := read.GetChapterImageFile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load media file: %w", err)
	}
	if !chapterKind(row.Kind) || !row.ProbedAt.Valid || row.MissingSince.Valid {
		return nil
	}
	_, _, version, ok := storedChapterImages(row.Probe, row.Kind, row.DurationMs.Int64, row.ProbedAt.Int64)
	if !ok || hasChapterImages(cache, ChapterImagesRel(id, version)) {
		return nil
	}
	j := chapterImagesJob(id)
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	jobID, err := q.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: j.priority, IdempotencyKey: j.key, Payload: payload,
		Delay: ChapterImagesAfterPlayback})
	if err != nil {
		return err
	}
	_, err = q.RaiseQueuedPriority(ctx, jobID, j.priority)
	return err
}

// enqueueChapterImagesTx queues chapters.images of a file in the caller's transaction (a probe) at
// the event priority 5; a waiting job of the backfill under the same key is raised to it.
func enqueueChapterImagesTx(ctx context.Context, q *systemq.Queries, now time.Time, mediaFileID string) error {
	j := chapterImagesJob(mediaFileID)
	if err := enqueueTx(ctx, q, now, j); err != nil {
		return err
	}
	if _, err := q.RaiseQueuedJobPriorityByKey(ctx, systemq.RaiseQueuedJobPriorityByKeyParams{
		Priority: int64(j.priority), Now: now.UnixMilli(), IdempotencyKey: sql.NullString{String: j.key, Valid: true},
	}); err != nil {
		return fmt.Errorf("raise %s: %w", j.typ, err)
	}
	return nil
}

// handleChapterImages writes the chapter images of one media file at its current version.
//
//   - A file that is gone, missing, deleted, not probed or failed its last probe, of another kind,
//     or whose folder exists already ends the job without work.
//   - While a playback of the file plays (CountActivePlaybacksOfFile), the job waits 10 minutes
//     without using an attempt (11.6: not during playback).
//   - A media file that cannot be read (a share offline) waits an hour, without folder and attempt.
//   - Per chapter it grabs the frame at the time points of transcode.ChapterFrameTimes: a black
//     frame or an error at the first one tries the second (11.14), a black second frame stays.
//   - When a chapter fails and attempts remain, the attempt fails (the queue retries). The last
//     attempt keeps what it has; with no image at all it writes the folder empty only when every
//     failure was ffmpeg giving no frame of a readable file (transcode.FrameFailed), otherwise it
//     fails without folder (ffmpeg not running).
//   - A file without chapters gets an empty folder at once.
//   - When a new probe arrived while the job ran, it runs again a minute later for that version.
func (s *Service) handleChapterImages(ctx context.Context, j *jobs.Job) error {
	var p mediaFilePayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	if s.cacheDir == "" {
		return nil
	}
	read := systemq.New(s.db.Read)
	row, err := read.GetChapterImageFile(ctx, p.MediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load media file: %w", err)
	}
	if !chapterKind(row.Kind) || !row.ProbedAt.Valid || row.MissingSince.Valid {
		return nil
	}
	pr, chapters, version, ok := storedChapterImages(row.Probe, row.Kind, row.DurationMs.Int64, row.ProbedAt.Int64)
	if !ok {
		s.log.WarnContext(ctx, "stored probe unreadable, no chapter images", "media_file_id", row.ID)
		return nil
	}
	video := mainVideo(pr.Streams)
	if video == nil {
		chapters = nil // nothing to grab: «done, no images»
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return err
	}
	root, err := paths.Open(cache)
	if err != nil {
		return err
	}
	defer root.Close()
	final := ChapterImagesRel(row.ID, version)
	if _, err := root.Stat(final); err == nil {
		return nil
	}
	now := s.now()
	playing, err := read.CountActivePlaybacksOfFile(ctx, systemq.CountActivePlaybacksOfFileParams{
		MediaFileID: row.ID, Now: now.UnixMilli(), ActiveAfter: now.Add(-chapterPlayingWindow).UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("count playbacks of the file: %w", err)
	}
	if playing > 0 {
		return jobs.RetryLater(chapterPlayingRetry, "the file is playing")
	}
	var in string
	if len(chapters) > 0 {
		media, err := paths.NewRoot(row.RootPath)
		if err != nil {
			return err
		}
		if in, err = paths.Resolve(media, row.RelPath); err != nil {
			return err
		}
		if err := s.readable(row.RootPath, in, chapterReadTimeout); err != nil {
			if p.UnreadableSince == 0 {
				p.UnreadableSince = now.UnixMilli()
				s.setRunningPayload(ctx, j, p)
			} else if now.Sub(time.UnixMilli(p.UnreadableSince)) > chapterUnreadableGiveUp {
				s.log.WarnContext(ctx, "media file unreadable for a week, chapter images given up", "media_file_id", row.ID, "err", err)
				return nil
			}
			s.log.WarnContext(ctx, "media file unreadable, chapter images wait", "media_file_id", row.ID, "err", err)
			return jobs.RetryLater(chapterUnreadableRetry, "the media file is unreadable")
		}
		if p.UnreadableSince != 0 {
			p.UnreadableSince = 0
			s.setRunningPayload(ctx, j, p)
		}
	}
	if err := root.MkdirAll(ChapterImagesDir, 0o750); err != nil {
		return fmt.Errorf("create chapter image dir: %w", err)
	}
	tmp := ChapterImagesDir + "/" + chapterTempPrefix + row.ID + "-" + strconv.FormatInt(now.UnixNano(), 10)
	if err := root.Mkdir(tmp, 0o750); err != nil {
		return fmt.Errorf("create chapter image dir: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_ = root.RemoveAll(tmp)
		}
	}()
	written, black, failed := 0, 0, 0
	onlyFrameFailures := true
	var lastErr error
	for i, c := range chapters {
		jobs.Progress(ctx, int64(i), int64(len(chapters)), "")
		ok, isBlack, err := s.grabChapter(ctx, root, cache, in, *video, c, tmp+"/"+ChapterImageName(i))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil:
			failed++
			lastErr = err
			onlyFrameFailures = onlyFrameFailures && transcode.FrameFailed(err)
			s.log.WarnContext(ctx, "chapter image failed", "media_file_id", row.ID, "chapter", i, "err", err)
		case ok:
			written++
			if isBlack {
				black++
			}
		}
	}
	jobs.Progress(ctx, int64(len(chapters)), int64(len(chapters)), "")
	if failed > 0 {
		switch {
		case j.Attempts < j.MaxAttempts:
			return fmt.Errorf("%d of %d chapter images of %s failed: %w", failed, len(chapters), row.ID, lastErr)
		case written == 0 && !onlyFrameFailures:
			return fmt.Errorf("no chapter image of %s: %w", row.ID, lastErr)
		}
		s.log.WarnContext(ctx, "chapter images incomplete after the last attempt", "media_file_id", row.ID,
			"chapters", len(chapters), "failed", failed)
	}
	if err := root.Rename(tmp, final); err != nil {
		if _, serr := root.Stat(final); serr != nil {
			return fmt.Errorf("store chapter images: %w", err)
		}
		// Another run of the same version was faster; its folder stays, this one goes (deferred).
	} else {
		done = true
	}
	s.log.InfoContext(ctx, "chapter images written", "media_file_id", row.ID, "chapters", len(chapters), "images", written,
		"black", black, "failed", failed)
	if s.onChapterImagesStored != nil {
		s.onChapterImagesStored()
	}
	// A probe that arrived while the job ran holds the key of this job; run again for its version.
	if cur, err := read.GetChapterImageFile(ctx, row.ID); err == nil {
		if _, _, v, ok := storedChapterImages(cur.Probe, cur.Kind, cur.DurationMs.Int64, cur.ProbedAt.Int64); ok && v != version {
			return jobs.RetryLater(chapterChangedRetry, "the file was probed again")
		}
	}
	return nil
}

// setRunningPayload stores the payload p in the running job j, for its next run; a failure only
// loses the note (logged).
func (s *Service) setRunningPayload(ctx context.Context, j *jobs.Job, p mediaFilePayload) {
	raw, err := json.Marshal(p)
	if err == nil {
		_, err = s.queue.SetRunningPayload(ctx, j.ID, raw)
	}
	if err != nil {
		s.log.WarnContext(ctx, "store chapter images job payload", "job_id", j.ID, "err", err)
	}
}

// grabFrame runs transcode.Runner.GrabFrame, or the test seam frameGrabber.
func (s *Service) grabFrame(ctx context.Context, in string, at time.Duration, width int, v transcode.Stream, out string) (bool, error) {
	if s.frameGrabber != nil {
		return s.frameGrabber(ctx, in, at, width, v, out)
	}
	return s.probe.GrabFrame(ctx, in, at, width, v, out)
}

// grabChapter writes the image of chapter c to rel (below the cache root). It tries the time points
// of transcode.ChapterFrameTimes in order: the next one after a black frame or an error, so a black
// first frame stays when the second fails. Each try runs at most chapterFrameTimeout, writes to a
// name of its own and replaces rel only when ffmpeg succeeded. ok reports whether rel exists; err
// is the last error when no frame was written.
func (s *Service) grabChapter(ctx context.Context, root *os.Root, cache paths.Root, in string, v transcode.Stream, c model.Chapter, rel string) (ok, black bool, err error) {
	for n, at := range transcode.ChapterFrameTimes(c) {
		try := rel + ".try" + strconv.Itoa(n)
		out, rerr := paths.Resolve(cache, try)
		if rerr != nil {
			return ok, black, rerr
		}
		frameCtx, cancel := context.WithTimeout(ctx, chapterFrameTimeout)
		isBlack, gerr := s.grabFrame(frameCtx, in, at, transcode.ChapterImageWidth, v, out)
		cancel()
		if gerr != nil {
			_ = root.Remove(try)
			if ctx.Err() != nil {
				return ok, black, ctx.Err()
			}
			err = gerr
			continue
		}
		if rerr := root.Rename(try, rel); rerr != nil {
			_ = root.Remove(try)
			err = fmt.Errorf("store chapter image: %w", rerr)
			continue
		}
		ok, black, err = true, isBlack, nil
		if !isBlack {
			break
		}
	}
	if ok {
		return true, black, nil
	}
	return false, false, err
}

// sweepChapterImages removes the chapter image folders of other versions than the current one of a
// file, of files that are gone, failed their last probe or whose title was deleted, and the
// temporary folders of jobs that died. It runs in the hourly library.sweep; without a cache
// directory it does nothing.
func (s *Service) sweepChapterImages(ctx context.Context) error {
	if s.cacheDir == "" {
		return nil
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return err
	}
	names, err := paths.ReadDirNames(cache, ChapterImagesDir, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list chapter images: %w", err)
	}
	rows, err := systemq.New(s.db.Read).ListChapterImageFiles(ctx)
	if err != nil {
		return fmt.Errorf("list chapter image files: %w", err)
	}
	keep := make(map[string]bool, len(rows))
	probedAt := make(map[string]int64, len(rows)) // current probe of each file with a version
	for _, r := range rows {
		if _, _, version, ok := storedChapterImages(r.Probe, r.Kind, r.DurationMs.Int64, r.ProbedAt.Int64); ok {
			keep[ChapterImagesRel(r.ID, version)] = true
			if !r.Missing {
				probedAt[r.ID] = r.ProbedAt.Int64
			}
		}
	}
	var rebuild []string
	root, err := paths.Open(cache)
	if err != nil {
		return err
	}
	defer root.Close()
	removed := 0
	for _, name := range names {
		rel := ChapterImagesDir + "/" + name
		if keep[rel] {
			continue
		}
		if strings.HasPrefix(name, chapterTempPrefix) {
			info, err := root.Stat(rel)
			if err != nil || s.now().Sub(info.ModTime()) < chapterTempMaxAge {
				continue
			}
		}
		if err := root.RemoveAll(rel); err != nil {
			s.log.WarnContext(ctx, "remove stale chapter images", "folder", name, "err", err)
			continue
		}
		// The folder of the current probe with another chapter list (chapter rules changed with
		// an update): the file had images, so it gets them again for the new list.
		if id, rest, ok := strings.Cut(name, "-"); ok {
			if at, known := probedAt[id]; known && strings.HasPrefix(rest, strconv.FormatInt(at, 10)+"-") {
				rebuild = append(rebuild, id)
			}
		}
		removed++
	}
	if removed > 0 {
		s.log.InfoContext(ctx, "stale chapter images removed", "folders", removed, "rebuilt", len(rebuild))
	}
	var errs []error
	for _, id := range rebuild {
		if err := s.enqueueBackfillImages(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("rebuild chapter images of %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// runChapterImagesBackfill is the Run of chapters.images_backfill: one backfill job at prio (a
// waiting one is raised to it).
func (s *Service) runChapterImagesBackfill(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobChapterImagesBackfill, Priority: prio, IdempotencyKey: chapterImagesBackfillKey, Payload: []byte("{}")})
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobChapterImagesBackfill, err)
	}
	return []int64{id}, nil
}

// enqueueBackfillImages queues chapters.images of a file at the backfill's priority. A job of the
// file that waits for a later start (queued ChapterImagesAfterPlayback after a playback) holds the
// key; it may run now (jobs.Queue.PullQueuedForward), because the backfill was started by hand,
// and keeps its own priority and attempts. The job itself still waits while the file plays.
func (s *Service) enqueueBackfillImages(ctx context.Context, mediaFileID string) error {
	j := chapterImagesJob(mediaFileID)
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: prioChapterImagesBackfill, IdempotencyKey: j.key, Payload: payload})
	if err != nil {
		return err
	}
	_, err = s.queue.PullQueuedForward(ctx, id)
	return err
}

// handleChapterImagesBackfill queues chapters.images (priority 1) for every probed film and episode
// file that is not missing and has no folder at its current version (an empty one counts: «done,
// no images»).
func (s *Service) handleChapterImagesBackfill(ctx context.Context, _ *jobs.Job) error {
	if s.cacheDir == "" {
		return nil
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return err
	}
	rows, err := systemq.New(s.db.Read).ListChapterImageFiles(ctx)
	if err != nil {
		return fmt.Errorf("list chapter image files: %w", err)
	}
	queued := 0
	for i, r := range rows {
		jobs.Progress(ctx, int64(i), int64(len(rows)), "")
		if r.Missing {
			continue
		}
		_, _, version, ok := storedChapterImages(r.Probe, r.Kind, r.DurationMs.Int64, r.ProbedAt.Int64)
		if !ok || hasChapterImages(cache, ChapterImagesRel(r.ID, version)) {
			continue
		}
		if err := s.enqueueBackfillImages(ctx, r.ID); err != nil {
			return err
		}
		queued++
	}
	jobs.Progress(ctx, int64(len(rows)), int64(len(rows)), "")
	s.log.InfoContext(ctx, "chapter images backfill queued", "files", len(rows), "queued", queued)
	return nil
}
