package library

import (
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

// chapterFolderNames lists the folders below cache/chapters.
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
	return out
}

// Concept 13 stage 3: the files of the first scan get no chapter images job, a file that appears
// after it does (through its probe); the job writes one WebP per chapter into the folder of the
// probe, does nothing for an existing folder, replaces the folder of an older probe; the backfill
// queues the files without images; the sweep removes the folders of deleted files and old
// temporary folders.
func TestChapterImagesJob(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" || ffmpegPath() == "" {
		t.Fatal("ffmpeg and ffprobe are required for the chapter image tests (install ffmpeg)")
	}
	e := newEnv(t)
	cache := t.TempDir()
	e.svc = New(e.db, e.queue, transcode.Runner{FFprobePath: ffprobePath(), FFmpegPath: ffmpegPath()}, discard,
		WithClock(e.clock.Now), WithCacheDir(cache))
	src := filepath.Join(testMedia(t), "Slouchcat Test Chapters (2024) {tmdb-7004}", "Slouchcat Test Chapters (2024) {tmdb-7004}.mkv")
	dir := t.TempDir()
	copyTestFile(t, src, dir, "Erster Film (2020)/Erster Film (2020).mkv")
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("first scan = %+v", got.Scan)
	}
	first := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Erster Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(first)); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Fatalf("first scan queued %d chapter image jobs", n)
	}

	e.clock.Advance(time.Minute)
	copyTestFile(t, src, dir, "Zweiter Film (2021)/Zweiter Film (2021).mkv")
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("second scan = %+v", got.Scan)
	}
	second := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Zweiter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(second)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 5`, JobChapterImages, "chapters:"+second); n != 1 {
		t.Fatalf("new file: %d chapter image jobs at priority 5, want 1", n)
	}

	// The job: three chapters, three WebP files in the folder of the probe.
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, second)); err != nil {
		t.Fatal(err)
	}
	probedAt := int64(e.count(`SELECT probed_at FROM media_files WHERE id = ?`, second))
	folder := filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(second, probedAt)))
	for i := range 3 {
		b, err := os.ReadFile(filepath.Join(folder, ChapterImageName(i)))
		if err != nil {
			t.Fatalf("chapter %d: %v", i, err)
		}
		if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
			t.Errorf("chapter %d is no WebP (%d bytes)", i, len(b))
		}
	}
	if names := chapterFolderNames(t, cache); len(names) != 1 {
		t.Errorf("folders = %v, want one", names)
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
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobChapterImages, "chapters:"+first); n != 1 {
		t.Errorf("backfill queued %d jobs for the file without images", n)
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

	// A new probe: a new folder, the old one goes.
	if _, err := e.db.Write.Exec(`UPDATE media_files SET probed_at = probed_at + 1 WHERE id = ?`, second); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleChapterImages(e.ctx, mediaJob(t, JobChapterImages, second)); err != nil {
		t.Fatal(err)
	}
	if names := chapterFolderNames(t, cache); !slices.Equal(names, []string{filepath.Base(ChapterImagesRel(second, probedAt+1))}) {
		t.Errorf("folders after the new probe = %v", names)
	}

	// The sweep: the folder of a deleted file goes, a stale temporary folder goes, a fresh one and
	// the folder of a known file stay.
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
	names := chapterFolderNames(t, cache)
	slices.Sort(names)
	want := []string{chapterTempPrefix + "fresh", filepath.Base(ChapterImagesRel(second, probedAt+1))}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("folders after the sweep = %v, want %v", names, want)
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
	if err := QueueChapterImagesAfterPlayback(e.ctx, e.db.Read, e.queue, e.svc.cacheDir, "01JZZZZZZZZZZZZZZZZZZZZZZZ"); err != nil {
		t.Errorf("after playback, unknown file: %v", err)
	}
	if n := e.jobCount(JobChapterImages); n != 0 {
		t.Errorf("queued %d jobs", n)
	}
}

// A file whose frames all fail (here: gone from the disk after its probe) fails the attempt
// without folder; the last attempt writes the folder empty, and the backfill skips it from then on.
func TestChapterImagesJobLastAttempt(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" || ffmpegPath() == "" {
		t.Fatal("ffmpeg and ffprobe are required for the chapter image tests (install ffmpeg)")
	}
	e := newEnv(t)
	cache := t.TempDir()
	e.svc = New(e.db, e.queue, transcode.Runner{FFprobePath: ffprobePath(), FFmpegPath: ffmpegPath()}, discard,
		WithClock(e.clock.Now), WithCacheDir(cache))
	src := filepath.Join(testMedia(t), "Slouchcat Test Chapters (2024) {tmdb-7004}", "Slouchcat Test Chapters (2024) {tmdb-7004}.mkv")
	dir := t.TempDir()
	copyTestFile(t, src, dir, "Kaputter Film (2020)/Kaputter Film (2020).mkv")
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan = %+v", got.Scan)
	}
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Kaputter Film%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Kaputter Film (2020)", "Kaputter Film (2020).mkv")); err != nil {
		t.Fatal(err)
	}
	job := mediaJob(t, JobChapterImages, id)
	job.Attempts, job.MaxAttempts = 1, jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err == nil {
		t.Error("first attempt without any frame succeeded")
	}
	if names := chapterFolderNames(t, cache); len(names) != 0 {
		t.Errorf("folders after a failed attempt = %v", names)
	}
	job.Attempts = jobs.DefaultMaxAttempts
	if err := e.svc.handleChapterImages(e.ctx, job); err != nil {
		t.Errorf("last attempt: %v", err)
	}
	probedAt := int64(e.count(`SELECT probed_at FROM media_files WHERE id = ?`, id))
	entries, err := os.ReadDir(filepath.Join(cache, filepath.FromSlash(ChapterImagesRel(id, probedAt))))
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
