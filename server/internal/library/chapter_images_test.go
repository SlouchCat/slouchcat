package library

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
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

// A chapters.images job that waits for its start after a playback is replaced by the backfill with
// one that may run now (at the backfill's priority).
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
	if err := e.svc.handleChapterImagesBackfill(e.ctx, &jobs.Job{Type: JobChapterImagesBackfill}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND run_after <= ? AND priority = 1`, JobChapterImages, now); n != 1 {
		t.Errorf("runnable backfill jobs = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND run_after > ?`, JobChapterImages, now); n != 0 {
		t.Errorf("delayed jobs after the backfill = %d, want 0", n)
	}
}
