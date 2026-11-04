package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Chapter images (concept 13 stage 3, 11.6, V-19): one WebP of 480 px per chapter of a film or an
// episode (transcode.Chapters, transcode.GrabFrame), stored below the cache directory in
// chapters/<mediaFileId>-<probedAt>/<idx>-w480.webp. The folder of a probe is written under a
// temporary name and renamed when every chapter is done, so an existing folder is a complete set
// (a chapter whose frame failed has no file). A new probe has a new probed_at and thus a new
// folder; the job removes the folders of older probes of its file, the hourly library.sweep
// removes the folders of deleted files and older probes (sweepChapterImages: media files leave
// through several deletes and foreign-key cascades, a sweep catches every one of them).
//
//   - chapters.images, payload {"mediaFileId": "<ULID>"}, key chapters:<id>, priority 5 (below
//     the manual runs, 7, and the keyframe index of a playback, 9): an event type, queued for a
//     file that is probed after its library finished a scan (probeJob's chapterImages, a new or
//     changed file) and for a file without images after its playback stopped
//     (QueueChapterImagesAfterPlayback).
//   - chapters.images_backfill, key chapters_images_backfill: a manual type (Run), started only by
//     hand on the page Hintergrundjobs; it queues chapters.images for every probed film and episode
//     file without images.
const (
	JobChapterImages         = "chapters.images"
	JobChapterImagesBackfill = "chapters.images_backfill"
	prioChapterImages        = 5
	chapterImagesBackfillKey = "chapters_images_backfill"
	// ChapterImagesDir is the folder of the chapter images below the cache directory.
	ChapterImagesDir = "chapters"
	// chapterTempPrefix starts the name of a folder a job is writing; the sweep removes one that
	// is older than chapterTempMaxAge (a job that died with the server).
	chapterTempPrefix = ".tmp-"
	chapterTempMaxAge = 6 * time.Hour
)

// ChapterImagesRel returns the folder of the chapter images of a media file at a probe, relative to
// the cache directory (slash-separated).
func ChapterImagesRel(mediaFileID string, probedAt int64) string {
	return ChapterImagesDir + "/" + mediaFileID + "-" + strconv.FormatInt(probedAt, 10)
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

// hasChapterImages reports whether the folder of the probe exists below the cache root.
func hasChapterImages(cache paths.Root, mediaFileID string, probedAt int64) bool {
	abs, err := paths.Resolve(cache, ChapterImagesRel(mediaFileID, probedAt))
	if err != nil {
		return false
	}
	_, err = os.Stat(abs)
	return err == nil
}

// QueueChapterImagesAfterPlayback queues chapters.images for a probed film or episode file without
// images (11.6 (2)): playback calls it after a playback stopped, never while it plays, so the job
// does not compete with the transcode of that playback. Anything else (no cache directory, another
// kind, an unprobed, missing or unknown file, images present) queues nothing.
func QueueChapterImagesAfterPlayback(ctx context.Context, db *sql.DB, q *jobs.Queue, cacheDir, mediaFileID string) error {
	if cacheDir == "" || mediaFileID == "" {
		return nil
	}
	row, err := systemq.New(db).GetChapterImageFile(ctx, mediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load media file: %w", err)
	}
	if !chapterKind(row.Kind) || !row.ProbedAt.Valid || !row.Probe.Valid || row.MissingSince.Valid {
		return nil
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return err
	}
	if hasChapterImages(cache, mediaFileID, row.ProbedAt.Int64) {
		return nil
	}
	return enqueue(ctx, q, chapterImagesJob(mediaFileID))
}

// enqueue adds j through the queue (outside a transaction); an active job with its key wins.
func enqueue(ctx context.Context, q *jobs.Queue, j newJob) error {
	payload, err := json.Marshal(j.payload)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", j.typ, err)
	}
	_, err = q.Enqueue(ctx, jobs.NewJob{Type: j.typ, Priority: j.priority, IdempotencyKey: j.key, Payload: payload})
	return err
}

// handleChapterImages writes the chapter images of one media file at its current probe. A file
// that is gone, missing, not probed, of another kind or whose folder exists already ends the job
// without work. Per chapter it grabs the frame at transcode.ChapterFrameTimes; a black frame is
// tried once more 10 s later (11.14), and when that one is black too it stays. A frame that fails
// leaves its chapter without image; when every frame fails (ffmpeg missing, an unreadable file)
// the attempt fails without folder and the queue tries again, and the last attempt writes the
// folder empty. An empty folder (also for a file without chapters, under 30 minutes) means «done,
// no images»: the backfill skips the file until a new probe.
func (s *Service) handleChapterImages(ctx context.Context, j *jobs.Job) error {
	var p mediaFilePayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	if s.cacheDir == "" {
		return nil
	}
	row, err := systemq.New(s.db.Read).GetChapterImageFile(ctx, p.MediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load media file: %w", err)
	}
	if !chapterKind(row.Kind) || !row.ProbedAt.Valid || !row.Probe.Valid || row.MissingSince.Valid {
		return nil
	}
	var pr transcode.Probe
	if err := json.Unmarshal([]byte(row.Probe.String), &pr); err != nil {
		s.log.WarnContext(ctx, "stored probe unreadable, no chapter images", "media_file_id", row.ID, "err", err)
		return nil
	}
	video := mainVideo(pr.Streams)
	if video == nil {
		return nil
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
	final := ChapterImagesRel(row.ID, row.ProbedAt.Int64)
	if _, err := root.Stat(final); err == nil {
		s.pruneChapterImages(ctx, root, cache, row.ID, final)
		return nil
	}
	media, err := paths.NewRoot(row.RootPath)
	if err != nil {
		return err
	}
	in, err := paths.Resolve(media, row.RelPath)
	if err != nil {
		return err
	}
	if err := root.MkdirAll(ChapterImagesDir, 0o750); err != nil {
		return fmt.Errorf("create chapter image dir: %w", err)
	}
	tmp := ChapterImagesDir + "/" + chapterTempPrefix + row.ID + "-" + strconv.FormatInt(s.now().UnixNano(), 10)
	if err := root.Mkdir(tmp, 0o750); err != nil {
		return fmt.Errorf("create chapter image dir: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_ = root.RemoveAll(tmp)
		}
	}()
	chapters := transcode.Chapters(&pr, row.Kind, row.DurationMs.Int64)
	written, black := 0, 0
	var lastErr error
	for i, c := range chapters {
		jobs.Progress(ctx, int64(i), int64(len(chapters)), "")
		ok, isBlack, err := s.grabChapter(ctx, root, cache, in, *video, c, tmp+"/"+ChapterImageName(i))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch {
		case err != nil:
			lastErr = err
			s.log.WarnContext(ctx, "chapter image failed", "media_file_id", row.ID, "chapter", i, "err", err)
		case ok:
			written++
			if isBlack {
				black++
			}
		}
	}
	jobs.Progress(ctx, int64(len(chapters)), int64(len(chapters)), "")
	if len(chapters) > 0 && written == 0 {
		if j.Attempts < j.MaxAttempts {
			return fmt.Errorf("no chapter image of %s: %w", row.ID, lastErr)
		}
		// The last attempt: the empty folder records «done, no images» for this probe, so the
		// backfill does not queue the file on every run; a new probe tries again.
		s.log.WarnContext(ctx, "no chapter image after the last attempt, file stays without images", "media_file_id", row.ID, "err", lastErr)
	}
	if err := root.Rename(tmp, final); err != nil {
		if _, serr := root.Stat(final); serr != nil {
			return fmt.Errorf("store chapter images: %w", err)
		}
		// Another run of the same probe was faster; its folder stays, this one goes (deferred).
	} else {
		done = true
	}
	s.pruneChapterImages(ctx, root, cache, row.ID, final)
	s.log.InfoContext(ctx, "chapter images written", "media_file_id", row.ID, "chapters", len(chapters), "images", written, "black", black)
	return nil
}

// grabChapter writes the image of chapter c to rel (below the cache root): the first time point,
// and when that frame is black the second one if the chapter has room for it. Each try writes to a
// name of its own and replaces rel only when ffmpeg succeeded. ok reports whether rel exists.
func (s *Service) grabChapter(ctx context.Context, root *os.Root, cache paths.Root, in string, v transcode.Stream, c model.Chapter, rel string) (ok, black bool, err error) {
	for n, at := range transcode.ChapterFrameTimes(c) {
		try := rel + ".try" + strconv.Itoa(n)
		out, rerr := paths.Resolve(cache, try)
		if rerr != nil {
			return ok, black, rerr
		}
		isBlack, gerr := s.probe.GrabFrame(ctx, in, at, transcode.ChapterImageWidth, v, out)
		if gerr != nil {
			_ = root.Remove(try)
			if ok {
				return true, black, nil // keep the black first frame
			}
			return false, false, gerr
		}
		if err := root.Rename(try, rel); err != nil {
			_ = root.Remove(try)
			return ok, black, fmt.Errorf("store chapter image: %w", err)
		}
		ok, black = true, isBlack
		if !isBlack {
			break
		}
	}
	return ok, black, nil
}

// pruneChapterImages removes the folders of older probes of a media file (all but keep).
func (s *Service) pruneChapterImages(ctx context.Context, root *os.Root, cache paths.Root, mediaFileID, keep string) {
	names, err := paths.ReadDirNames(cache, ChapterImagesDir, 0)
	if err != nil {
		return
	}
	for _, name := range names {
		rel := ChapterImagesDir + "/" + name
		if rel == keep || !strings.HasPrefix(name, mediaFileID+"-") {
			continue
		}
		if err := root.RemoveAll(rel); err != nil {
			s.log.WarnContext(ctx, "remove old chapter images", "media_file_id", mediaFileID, "err", err)
		}
	}
}

// sweepChapterImages removes the chapter image folders of media files that are gone (or whose
// title was deleted) and of older probes, and the temporary folders of jobs that died. It runs in
// the hourly library.sweep; without a cache directory it does nothing.
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
	for _, r := range rows {
		keep[ChapterImagesRel(r.ID, r.ProbedAt.Int64)] = true
	}
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
		removed++
	}
	if removed > 0 {
		s.log.InfoContext(ctx, "stale chapter images removed", "folders", removed)
	}
	return nil
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

// handleChapterImagesBackfill queues chapters.images (priority 5) for every probed film and episode
// file that is not missing and has no images at its probe.
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
		if r.Missing || hasChapterImages(cache, r.ID, r.ProbedAt.Int64) {
			continue
		}
		if err := enqueue(ctx, s.queue, chapterImagesJob(r.ID)); err != nil {
			return err
		}
		queued++
	}
	jobs.Progress(ctx, int64(len(rows)), int64(len(rows)), "")
	s.log.InfoContext(ctx, "chapter images backfill queued", "files", len(rows), "queued", queued)
	return nil
}
