package playback

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// chapterImageJobs counts the chapters.images jobs of a media file.
func chapterImageJobs(t *testing.T, f *vistest.Fixture, fileID string) int {
	t.Helper()
	var n int
	if err := f.DB.Read.QueryRow(`SELECT COUNT(*) FROM jobs WHERE type = ? AND idempotency_key = ?`,
		library.JobChapterImages, "chapters:"+fileID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Concept 13 stage 3: PlaybackInfo names the chapter images that exist for the file's version
// (probe time and chapter fingerprint). Neither the start nor the stop queue chapters.images; the
// sweep after the stop does, 10 minutes later, and not again once images exist.
func TestStartChapterImages(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddTracks(t)
	m := f.Rated(6)
	raw := trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000)
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", raw, testKeyframes)
	f.SetProbedAt(t, m.FileID, 4242)
	var probe transcode.Probe
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		t.Fatal(err)
	}
	version := library.ChapterImagesVersion(4242, transcode.Chapters(&probe, "movie", 0))
	cache := t.TempDir()
	s := New(f.DB, vistest.Logger(), WithTranscoder(newFakeTranscoder()), WithTokenKey(testKey), WithCacheDir(cache))
	ctx := context.Background()
	session := browserSession(t, f, f.P6)

	// No images yet: no URL; neither a running playback nor the stop request queue the job.
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &capsChromeLinux})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range info.Chapters {
		if c.ImageURL != "" {
			t.Errorf("chapter %d has an image URL without images: %q", i, c.ImageURL)
		}
	}
	s.sweepChapterImages(ctx, time.Now())
	if n := chapterImageJobs(t, f, m.FileID); n != 0 {
		t.Errorf("a running playback queued %d chapter image jobs", n)
	}
	if err := s.Stop(ctx, session, info.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if n := chapterImageJobs(t, f, m.FileID); n != 0 {
		t.Errorf("the stop request queued %d chapter image jobs", n)
	}
	// The sweep after the stop queues it, ChapterImagesAfterPlayback later.
	swept := time.Now()
	s.sweepChapterImages(ctx, swept)
	if n := chapterImageJobs(t, f, m.FileID); n != 1 {
		t.Fatalf("sweep after the stop queued %d chapter image jobs, want 1", n)
	}
	var runAfter int64
	if err := f.DB.Read.QueryRow(`SELECT run_after FROM jobs WHERE idempotency_key = ?`, "chapters:"+m.FileID).Scan(&runAfter); err != nil {
		t.Fatal(err)
	}
	if d := time.UnixMilli(runAfter).Sub(swept); d < library.ChapterImagesAfterPlayback-time.Minute || d > library.ChapterImagesAfterPlayback+time.Minute {
		t.Errorf("job runs %v after the sweep, want %v", d, library.ChapterImagesAfterPlayback)
	}

	// Images of chapters 0 and 3 at the current version, one of an older probe.
	folder := filepath.Join(cache, filepath.FromSlash(library.ChapterImagesRel(m.FileID, version)))
	old := filepath.Join(cache, filepath.FromSlash(library.ChapterImagesRel(m.FileID, "1-0123abcd")))
	for _, dir := range []string{folder, old} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{filepath.Join(folder, library.ChapterImageName(0)), filepath.Join(folder, library.ChapterImageName(3)),
		filepath.Join(old, library.ChapterImageName(1))} {
		if err := os.WriteFile(p, []byte("webp"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &capsChromeLinux})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Chapters) != 12 {
		t.Fatalf("chapters = %+v", info.Chapters)
	}
	for i, c := range info.Chapters {
		want := ""
		if i == 0 || i == 3 {
			want = ChapterImageURL(m.ID, m.FileID, i, version)
		}
		if c.ImageURL != want {
			t.Errorf("chapter %d image URL = %q, want %q", i, c.ImageURL, want)
		}
	}
	if want := "/api/v1/items/" + m.ID + "/chapters/3/image?mediaFileId=" + m.FileID + "&v=" + version; info.Chapters[3].ImageURL != want {
		t.Errorf("image URL = %q, want %q", info.Chapters[3].ImageURL, want)
	}
	// A playback that ends with images present queues nothing more (one job row of the first).
	if err := s.Stop(ctx, session, info.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	s.sweepChapterImages(ctx, time.Now())
	if n := chapterImageJobs(t, f, m.FileID); n != 1 {
		t.Errorf("chapter image jobs = %d, want 1", n)
	}
}
