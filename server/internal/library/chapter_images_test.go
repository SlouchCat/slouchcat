package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// copyTestFile copies the file src to dir/rel.
func copyTestFile(t *testing.T, src, dir, rel string) {
	t.Helper()
	if err := copyFile(src, filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

// probeJobOf returns the queued probe.file job of a media file as the scan stored it.
func (e *env) probeJobOf(fileID string) *jobs.Job {
	e.t.Helper()
	payload := e.str(`SELECT payload FROM jobs WHERE type = ? AND idempotency_key = ?`, JobProbeFile, "probe:"+fileID)
	if payload == "" {
		e.t.Fatalf("no probe job of %s", fileID)
	}
	return &jobs.Job{Type: JobProbeFile, Payload: []byte(payload)}
}

// chapterVersion returns the current version of the chapter images of a media file.
func (e *env) chapterVersion(fileID string) string {
	e.t.Helper()
	row, err := systemq.New(e.db.Read).GetChapterImageFile(e.ctx, fileID)
	if err != nil {
		e.t.Fatal(err)
	}
	_, _, version, ok := storedChapterImages(row.Probe, row.Kind, row.DurationMs.Int64, row.ProbedAt.Int64)
	if !ok {
		e.t.Fatalf("probe of %s unreadable", fileID)
	}
	return version
}

// chapterFolderNames lists the folders below cache/chapters, sorted.
func chapterFolderNames(t *testing.T, cache string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(cache, ChapterImagesDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}

// chapterEnv is an env with ffmpeg, a cache directory and a library of copies of the generated
// chapters film (three chapters, 10 s), scanned once.
func chapterEnv(t *testing.T, films ...string) (e *env, cache, dir string, lib int64) {
	t.Helper()
	if ffprobePath() == "" || ffmpegPath() == "" {
		t.Fatal("ffmpeg and ffprobe are required for the chapter image tests (install ffmpeg)")
	}
	e = newEnv(t)
	cache = t.TempDir()
	e.svc = New(e.db, e.queue, transcode.Runner{FFprobePath: ffprobePath(), FFmpegPath: ffmpegPath()}, discard,
		WithClock(e.clock.Now), WithCacheDir(cache))
	dir = t.TempDir()
	for _, name := range films {
		copyTestFile(t, chaptersFilm(t), dir, name+"/"+name+".mkv")
	}
	l := e.create(dir)
	if got := e.scan(l.ID); got.Scan.State != "done" {
		t.Fatalf("first scan = %+v", got.Scan)
	}
	return e, cache, dir, l.ID
}

func chaptersFilm(t *testing.T) string {
	return filepath.Join(testMedia(t), "Slouchcat Test Chapters (2024) {tmdb-7004}", "Slouchcat Test Chapters (2024) {tmdb-7004}.mkv")
}

// Concept 13 stage 3: the files of the first scan get no chapter images job, a file that appears
// after a complete scan does (through its probe); the job writes one WebP per chapter into the
// folder of the version, does nothing for an existing folder; a re-probe of a file with images
// queues the job again and gets a new folder; the backfill queues the files without images; the
// sweep removes the folders of other versions, of deleted files and old temporary folders.
func TestChapterImagesJob(t *testing.T) {
	t.Parallel()
	e, cache, dir, lib := chapterEnv(t, "Erster Film (2020)")
	first := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Erster Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(first)); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Fatalf("first scan queued %d chapter image jobs", n)
	}

	e.clock.Advance(time.Minute)
	copyTestFile(t, chaptersFilm(t), dir, "Zweiter Film (2021)/Zweiter Film (2021).mkv")
	if got := e.scan(lib); got.Scan.State != "done" {
		t.Fatalf("second scan = %+v", got.Scan)
	}
	second := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Zweiter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(second)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 5`, JobChapterImages, "chapters:"+second); n != 1 {
		t.Fatalf("new file: %d chapter image jobs at priority 5, want 1", n)
	}

	// The job: three chapters, three WebP files in the folder of the version.
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, second)); err != nil {
		t.Fatal(err)
	}
	version := e.chapterVersion(second)
	folder := filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(second, version)))
	for i := range 3 {
		b, err := os.ReadFile(filepath.Join(folder, ChapterImageName(i)))
		if err != nil {
			t.Fatalf("chapter %d: %v", i, err)
		}
		if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
			t.Errorf("chapter %d is no WebP (%d bytes)", i, len(b))
		}
	}
	if names := chapterFolderNames(t, cache); !slices.Equal(names, []string{second + "-" + version}) {
		t.Errorf("folders = %v, want the one of %s", names, version)
	}
	// A second run finds the folder and does nothing.
	before, err := os.Stat(filepath.Join(folder, ChapterImageName(0)))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, second)); err != nil {
		t.Fatal(err)
	}
	if after, err := os.Stat(filepath.Join(folder, ChapterImageName(0))); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("second run rewrote the images: %v", err)
	}

	// The backfill queues the first file (no images), not the second.
	if err := e.svc.handleChapterImagesBackfill(e.ctx, &jobs.Job{Type: JobChapterImagesBackfill}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 1`, JobChapterImages, "chapters:"+first); n != 1 {
		t.Errorf("backfill queued %d jobs at priority 1 for the file without images", n)
	}
	if n := e.jobCount(JobChapterImages); n != 2 {
		t.Errorf("chapter image jobs = %d, want 2 (one per file)", n)
	}
	ids, err := e.svc.runChapterImagesBackfill(e.ctx, e.queue, jobs.ManualPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run backfill = %v, %v", ids, err)
	}
	if j, err := e.queue.Get(e.ctx, ids[0]); err != nil || j.Type != JobChapterImagesBackfill || j.Priority != jobs.ManualPriority {
		t.Errorf("backfill job = %+v, %v", j, err)
	}

	// A re-probe of the file with images ("Metadaten aktualisieren") queues the job of the new
	// probe; it writes a new folder, the old one waits for the sweep.
	e.exec(`DELETE FROM jobs WHERE type = ?`, JobChapterImages) // the earlier runs are done
	e.clock.Advance(time.Minute)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, second)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobChapterImages, "chapters:"+second); n != 1 {
		t.Fatalf("re-probe of a file with images queued %d jobs, want 1", n)
	}
	newVersion := e.chapterVersion(second)
	if newVersion == version {
		t.Fatalf("version unchanged after the re-probe: %s", version)
	}
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, second)); err != nil {
		t.Fatal(err)
	}
	want := []string{second + "-" + newVersion, second + "-" + version}
	slices.Sort(want)
	if names := chapterFolderNames(t, cache); !slices.Equal(names, want) {
		t.Errorf("folders after the new probe = %v, want %v", names, want)
	}

	// The sweep: the folders of the old version and of a deleted file go, a stale temporary folder
	// goes, a fresh one and the current folder stay.
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, first)); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(cache, ChapterImagesDir, chapterTempPrefix+"stale")
	fresh := filepath.Join(cache, ChapterImagesDir, chapterTempPrefix+"fresh")
	for _, d := range []string{stale, fresh} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	old := e.clock.Now().Add(-7 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fresh, e.clock.Now(), e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := systemq.New(e.db.Write).DeleteMediaFile(e.ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.sweepChapterImages(e.ctx); err != nil {
		t.Fatal(err)
	}
	want = []string{chapterTempPrefix + "fresh", second + "-" + newVersion}
	slices.Sort(want)
	if names := chapterFolderNames(t, cache); !slices.Equal(names, want) {
		t.Errorf("folders after the sweep = %v, want %v", names, want)
	}
}

// While a playback of the file plays, the job waits without an attempt; it runs once the playback
// had no request for two minutes.
func TestChapterImagesJobWaitsForPlayback(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	item := e.str(`SELECT item_id FROM media_files WHERE id = ?`, id)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	e.profiles(1)
	now := e.clock.Now().UnixMilli()
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01JPLAYBACKSESSION00000001', 1, 1, 1, ?, ?, 'direct', ?, ?, ?)`, item, id, now, now, now+3_600_000)
	err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, id))
	var later *jobs.RetryLaterError
	if !errors.As(err, &later) || later.After != chapterPlayingRetry {
		t.Fatalf("playing file: %v, want RetryLater(%v)", err, chapterPlayingRetry)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders while playing = %v", names)
	}
	// A session the client never stopped counts as playing until two minutes after its last
	// request (the rule of the stream limits).
	e.exec(`UPDATE playback_sessions SET last_activity_at = ? WHERE id = '01JPLAYBACKSESSION00000001'`, now-int64(chapterPlayingWindow/time.Millisecond)+1000)
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, id)); !errors.As(err, &later) {
		t.Fatalf("request 1:59 ago: %v, want RetryLater", err)
	}
	e.exec(`UPDATE playback_sessions SET last_activity_at = ? WHERE id = '01JPLAYBACKSESSION00000001'`, now-int64(chapterPlayingWindow/time.Millisecond)-1000)
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, id)); err != nil {
		t.Fatal(err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 1 {
		t.Errorf("folders after the playback = %v", names)
	}
}

// A media file that cannot be read waits an hour without folder; a readable file whose frames
// ffmpeg cannot make fails its attempts and gets the empty «done» folder on the last one, which
// the backfill then skips.
func TestChapterImagesJobFailures(t *testing.T) {
	t.Parallel()
	e, cache, dir, _ := chapterEnv(t, "Kaputter Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Kaputter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Kaputter Film (2020)", "Kaputter Film (2020).mkv")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	job := mediaJob(t, JobChapterImages, id)
	job.Attempts, job.MaxAttempts = jobs.DefaultMaxAttempts, jobs.DefaultMaxAttempts
	err := e.svc.handleChapterImages(e.ctx, job)
	var later *jobs.RetryLaterError
	if !errors.As(err, &later) || later.After != chapterUnreadableRetry {
		t.Fatalf("unreadable file: %v, want RetryLater(%v)", err, chapterUnreadableRetry)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Fatalf("folders for an unreadable file = %v", names)
	}

	// Readable, but no video ffmpeg can read: the attempts fail, the last one writes the folder empty.
	if err := os.WriteFile(path, []byte("no video at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	job.Attempts = 1
	if err := e.svc.handleChapterImages(e.ctx, job); err == nil || errors.As(err, &later) {
		t.Errorf("first attempt without any frame: %v, want an error", err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders after a failed attempt = %v", names)
	}
	job.Attempts = jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err != nil {
		t.Errorf("last attempt: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(id, e.chapterVersion(id)))))
	if err != nil || len(entries) != 0 {
		t.Fatalf("folder after the last attempt: %v, %d entries", err, len(entries))
	}
	if err := e.svc.handleChapterImagesBackfill(e.ctx, &jobs.Job{Type: JobChapterImagesBackfill}); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Errorf("backfill queued %d jobs for a file marked without images", n)
	}
}

// Without ffmpeg the last attempt writes no «done» folder: the frames may well exist.
func TestChapterImagesJobWithoutFFmpeg(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	e.svc.probe.FFmpegPath = ""
	job := mediaJob(t, JobChapterImages, id)
	job.Attempts, job.MaxAttempts = jobs.DefaultMaxAttempts, jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err == nil {
		t.Error("last attempt without ffmpeg succeeded")
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders without ffmpeg = %v", names)
	}
}

// A job without cache directory, for an unknown file or a file that is not a film or an episode
// does nothing and does not fail.
func TestChapterImagesJobNoWork(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, "01JZZZZZZZZZZZZZZZZZZZZZZZ")); err != nil {
		t.Errorf("without cache dir: %v", err)
	}
	e.svc.cacheDir = t.TempDir()
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, "01JZZZZZZZZZZZZZZZZZZZZZZZ")); err != nil {
		t.Errorf("unknown file: %v", err)
	}
	if err := QueueChapterImagesAfterPlayback(e.ctx, e.db.Read, e.queue, e.svc.cacheDir, []string{"01JZZZZZZZZZZZZZZZZZZZZZZZ"}); err != nil {
		t.Errorf("after playback, unknown file: %v", err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Errorf("queued %d jobs", n)
	}
}

// Concept 13 11.6, «after the first complete scan»: a scan after a failed one does not count its
// new files as news (the backfill covers them); after a done scan it does.
func TestChapterImagesNewFilesNeedADoneScan(t *testing.T) {
	t.Parallel()
	e, _, dir, lib := chapterEnv(t, "Erster Film (2020)")
	e.exec(`UPDATE libraries SET last_scan_error = 'boom' WHERE id = ?`, lib) // as a failed scan leaves it
	copyTestFile(t, chaptersFilm(t), dir, "Zweiter Film (2021)/Zweiter Film (2021).mkv")
	e.clock.Advance(time.Minute)
	if got := e.scan(lib); got.Scan.State != "done" {
		t.Fatalf("scan = %+v", got.Scan)
	}
	second := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Zweiter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(second)); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Errorf("after a failed scan: %d chapter image jobs", n)
	}
	copyTestFile(t, chaptersFilm(t), dir, "Dritter Film (2022)/Dritter Film (2022).mkv")
	e.clock.Advance(time.Minute)
	if got := e.scan(lib); got.Scan.State != "done" {
		t.Fatalf("scan = %+v", got.Scan)
	}
	third := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Dritter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(third)); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobChapterImages); n != 1 {
		t.Errorf("after a done scan: %d chapter image jobs, want 1", n)
	}
}

// A probe that arrives while the job runs (its key is taken) makes the job run again a minute later
// for the new version.
func TestChapterImagesJobReprobedWhileRunning(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	e.svc.onChapterImagesStored = func() { e.exec(`UPDATE media_files SET probed_at = probed_at + 1 WHERE id = ?`, id) }
	err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, id))
	var later *jobs.RetryLaterError
	if !errors.As(err, &later) || later.After != chapterChangedRetry {
		t.Fatalf("re-probed while running: %v, want RetryLater(%v)", err, chapterChangedRetry)
	}
	e.svc.onChapterImagesStored = nil
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, id)); err != nil {
		t.Fatal(err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 2 || !slices.Contains(names, id+"-"+e.chapterVersion(id)) {
		t.Errorf("folders = %v, want the old and the new version", names)
	}
}

// A chapters.images job that waits for its start after a playback may run now after the backfill,
// with its own priority 5 (not the backfill's 1) and the same job row.
func TestChapterImagesBackfillPullsDelayedJob(t *testing.T) {
	t.Parallel()
	e, _, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	if err := QueueChapterImagesAfterPlayback(e.ctx, e.db.Read, e.queue, e.svc.cacheDir, []string{id}); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now().UnixMilli()
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND run_after > ?`, JobChapterImages, now); n != 1 {
		t.Fatalf("delayed jobs after the playback = %d, want 1", n)
	}
	// After a playback the job may start outside the time window of its type, not during a pause
	// (concept 15, 4.3).
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND override_window = 1 AND override_pause = 0`, JobChapterImages); n != 1 {
		t.Errorf("chapter image jobs of the playback with override_window only = %d, want 1", n)
	}
	if err := e.svc.handleChapterImagesBackfill(e.ctx, &jobs.Job{Type: JobChapterImagesBackfill}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND run_after <= ? AND priority = 5`, JobChapterImages, now); n != 1 {
		t.Errorf("runnable jobs at priority 5 = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ?`, JobChapterImages); n != 1 {
		t.Errorf("chapter image job rows = %d, want the one of the playback", n)
	}
}

// An event (the end of a playback, a probe) that finds a waiting job of the backfill under its key
// raises it to the event priority 5.
func TestChapterImagesEventRaisesBackfillJob(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	priority := func() int {
		return e.count(`SELECT priority FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobChapterImages, "chapters:"+id)
	}
	if err := e.svc.handleChapterImagesBackfill(e.ctx, &jobs.Job{Type: JobChapterImagesBackfill}); err != nil {
		t.Fatal(err)
	}
	if p := priority(); p != prioChapterImagesBackfill {
		t.Fatalf("backfill job priority = %d", p)
	}
	if err := QueueChapterImagesAfterPlayback(e.ctx, e.db.Read, e.queue, cache, []string{id}); err != nil {
		t.Fatal(err)
	}
	if p := priority(); p != prioChapterImages {
		t.Errorf("after a playback the job has priority %d, want %d", p, prioChapterImages)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND override_window = 1 AND override_pause = 0`, "chapters:"+id); n != 1 {
		t.Errorf("the raised backfill job does not carry override_window alone")
	}
	// The same through a probe of a file with images: lower the job again, write a folder.
	e.exec(`UPDATE jobs SET priority = 1 WHERE idempotency_key = ?`, "chapters:"+id)
	if err := os.MkdirAll(filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(id, e.chapterVersion(id)))), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
		t.Fatal(err)
	}
	if p := priority(); p != prioChapterImages {
		t.Errorf("after a probe the job has priority %d, want %d", p, prioChapterImages)
	}
}

// writeFakeFrame is a frameGrabber result: it writes a tiny WebP-like file to out.
func writeFakeFrame(out string) error {
	return os.WriteFile(out, []byte("RIFF\x04\x00\x00\x00WEBP"), 0o600)
}

// With the frameGrabber seam: a chapter that fails makes the attempt fail while attempts remain,
// and the last attempt keeps the chapters that worked. (The chapters of the test film are 3 s
// long and have one time point each; TestGrabChapterSecondPoint covers the second one.)
func TestChapterImagesJobPartial(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	row, err := systemq.New(e.db.Read).GetChapterImageFile(e.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	_, chapters, version, _ := storedChapterImages(row.Probe, row.Kind, row.DurationMs.Int64, row.ProbedAt.Int64)
	if len(chapters) != 3 {
		t.Fatalf("chapters = %+v", chapters)
	}
	e.svc.frameGrabber = func(_ context.Context, _ string, at time.Duration, _ int, _ transcode.Stream, out string) (bool, error) {
		if slices.Contains(transcode.ChapterFrameTimes(chapters[2]), at) { // chapter 2 fails
			return false, transcode.ErrNoFrame
		}
		return false, writeFakeFrame(out)
	}
	job := mediaJob(t, JobChapterImages, id)
	job.Attempts, job.MaxAttempts = 1, jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err == nil || !strings.Contains(err.Error(), "1 of 3 chapter images") {
		t.Fatalf("partial attempt: %v, want 1 of 3 failed", err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Fatalf("folders after a partial attempt = %v", names)
	}
	job.Attempts = jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err != nil {
		t.Fatalf("last attempt: %v", err)
	}
	folder := filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(id, version)))
	for i, want := range []bool{true, true, false} {
		if _, err := os.Stat(filepath.Join(folder, ChapterImageName(i))); (err == nil) != want {
			t.Errorf("chapter %d image exists = %v, want %v", i, err == nil, want)
		}
	}
}

// A file that stays unreadable for more than seven days after the job first found it unreadable
// ends the job without folder. The clock starts with the first unreadable run (payload
// unreadableSince), not with the age of the job, and stops when the file is readable again.
func TestChapterImagesJobGivesUpOnUnreadable(t *testing.T) {
	t.Parallel()
	e, cache, dir, _ := chapterEnv(t, "Film (2020)")
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Film (2020)", "Film (2020).mkv")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := e.queue.Enqueue(e.ctx, jobs.NewJob{Type: JobChapterImages, Priority: 1, IdempotencyKey: "chapters:" + id,
		Payload: []byte(`{"mediaFileId":"` + id + `"}`)}); err != nil {
		t.Fatal(err)
	}
	job, err := e.queue.Claim(e.ctx, "w1", []string{JobChapterImages})
	if err != nil || job == nil {
		t.Fatalf("claim = %+v, %v", job, err)
	}
	// run runs the claimed job with the payload the queue holds now.
	run := func() error {
		job.Payload = []byte(e.str(`SELECT payload FROM jobs WHERE id = ?`, job.ID))
		return e.svc.handleChapterImages(e.ctx, job)
	}
	since := func() int {
		return e.count(`SELECT coalesce(json_extract(payload, '$.unreadableSince'), 0) FROM jobs WHERE id = ?`, job.ID)
	}
	var later *jobs.RetryLaterError
	job.CreatedAt = e.clock.Now().Add(-30 * 24 * time.Hour) // an old job of the backfill: no matter
	if err := run(); !errors.As(err, &later) {
		t.Fatalf("first unreadable run of an old job: %v, want RetryLater", err)
	}
	if got, want := since(), int(e.clock.Now().UnixMilli()); got != want {
		t.Fatalf("unreadableSince = %d, want %d", got, want)
	}
	e.clock.Advance(6 * 24 * time.Hour)
	if err := run(); !errors.As(err, &later) {
		t.Fatalf("six days unreadable: %v, want RetryLater", err)
	}
	// Readable again (the frames fail, the attempt fails): the clock stops.
	copyTestFile(t, chaptersFilm(t), dir, "Film (2020)/Film (2020).mkv")
	e.svc.frameGrabber = func(context.Context, string, time.Duration, int, transcode.Stream, string) (bool, error) {
		return false, transcode.ErrNoFrame
	}
	if err := run(); err == nil || errors.As(err, &later) {
		t.Fatalf("readable again: %v, want a failed attempt", err)
	}
	if got := since(); got != 0 {
		t.Errorf("unreadableSince after a readable run = %d, want 0", got)
	}
	// Unreadable again: a new week starts; after it the job gives up without folder.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(2 * 24 * time.Hour)
	if err := run(); !errors.As(err, &later) {
		t.Fatalf("unreadable again: %v, want RetryLater", err)
	}
	e.clock.Advance(6 * 24 * time.Hour)
	if err := run(); !errors.As(err, &later) {
		t.Fatalf("six days into the second outage: %v, want RetryLater", err)
	}
	e.clock.Advance(2 * 24 * time.Hour)
	if err := run(); err != nil {
		t.Fatalf("eight days unreadable: %v, want nil", err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders = %v, want none", names)
	}
}

// The sweep removes the folders of a deleted title and of a file whose last probe failed; a folder
// of the current probe with another chapter list (other chapter rules) goes too, and the file gets
// a job at priority 1 to make the images for its list again.
func TestChapterImagesSweepCases(t *testing.T) {
	t.Parallel()
	e, cache, _, _ := chapterEnv(t, "Eins (2020)", "Zwei (2021)", "Drei (2022)")
	ids := map[string]string{}
	for _, name := range []string{"Eins", "Zwei", "Drei"} {
		ids[name] = e.str(`SELECT id FROM media_files WHERE rel_path LIKE ?`, name+"%")
		if err := e.svc.handleProbe(e.ctx, e.probeJobOf(ids[name])); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Eins", "Zwei"} {
		dir := filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(ids[name], e.chapterVersion(ids[name]))))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	probedAt := e.count(`SELECT probed_at FROM media_files WHERE id = ?`, ids["Drei"])
	otherList := filepath.Join(cache, ChapterImagesDir, ids["Drei"]+"-"+strconv.Itoa(probedAt)+"-0123abcd")
	if err := os.MkdirAll(otherList, 0o750); err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE items SET deleted_at = 1 WHERE id = (SELECT item_id FROM media_files WHERE id = ?)`, ids["Eins"])
	e.exec(`UPDATE media_files SET probe_error = 'broken' WHERE id = ?`, ids["Zwei"])
	if err := e.svc.sweepChapterImages(e.ctx); err != nil {
		t.Fatal(err)
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders after the sweep = %v, want none", names)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 1`, JobChapterImages, "chapters:"+ids["Drei"]); n != 1 {
		t.Errorf("rebuild jobs for the other chapter list = %d, want 1", n)
	}
	if n := e.jobCount(JobChapterImages); n != 1 {
		t.Errorf("chapter image jobs = %d, want only the rebuild", n)
	}
}

// grabChapter tries the second time point after an error at the first; a black first frame stays
// when the second fails.
func TestGrabChapterSecondPoint(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	cache := t.TempDir()
	cr, err := paths.NewRoot(cache)
	if err != nil {
		t.Fatal(err)
	}
	root, err := paths.Open(cr)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	c := model.Chapter{StartMs: 0, EndMs: 100_000} // points at 10 s and 20 s
	var calls []time.Duration
	e.svc.frameGrabber = func(_ context.Context, _ string, at time.Duration, _ int, _ transcode.Stream, out string) (bool, error) {
		calls = append(calls, at)
		if at == 10*time.Second {
			return false, errors.New("transient")
		}
		return false, writeFakeFrame(out)
	}
	ok, black, err := e.svc.grabChapter(e.ctx, root, cr, "/in.mkv", transcode.Stream{}, c, "0.webp")
	if !ok || black || err != nil || !slices.Equal(calls, []time.Duration{10 * time.Second, 20 * time.Second}) {
		t.Errorf("error first: ok %v black %v err %v calls %v", ok, black, err, calls)
	}
	if _, err := os.Stat(filepath.Join(cache, "0.webp")); err != nil {
		t.Errorf("image: %v", err)
	}
	calls = nil
	e.svc.frameGrabber = func(_ context.Context, _ string, at time.Duration, _ int, _ transcode.Stream, out string) (bool, error) {
		calls = append(calls, at)
		if at == 20*time.Second {
			return false, transcode.ErrNoFrame
		}
		return true, writeFakeFrame(out)
	}
	ok, black, err = e.svc.grabChapter(e.ctx, root, cr, "/in.mkv", transcode.Stream{}, c, "1.webp")
	if !ok || !black || err != nil || len(calls) != 2 {
		t.Errorf("black first, error second: ok %v black %v err %v calls %v", ok, black, err, calls)
	}
}
