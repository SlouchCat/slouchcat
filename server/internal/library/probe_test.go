package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// testMedia returns the generated test movies (`task testdata:media`).
func testMedia(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Movies"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("test media missing (run `task testdata:media` in the repository root): %v", err)
	}
	return dir
}

func mediaJob(t *testing.T, typ, mediaFileID string) *jobs.Job {
	t.Helper()
	payload, err := json.Marshal(mediaFilePayload{mediaFileID})
	if err != nil {
		t.Fatal(err)
	}
	return &jobs.Job{Type: typ, Payload: payload}
}

// Not parallel: this test waits on the clock (an interval, a debounce window or a
// cancellation), and parallel load would stretch the wait past what it asserts.
func TestProbeJobsOnTestMedia(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	lib := e.create(testMedia(t))
	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.ItemCount < 8 {
		t.Fatalf("scan of the test media = %+v", got.Scan)
	}

	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Slouchcat Test Sidecar Subtitles (2024)/%'`)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT container || '/' || video_codec || '/' || (width > 0) || '/' || (height > 0) || '/' || (duration_ms > 5000) || '/' || (probed_at IS NOT NULL) FROM media_files WHERE id = ?`, id); v != "mkv/h264/1/1/1/1" &&
		v != "mkv/hevc/1/1/1/1" {
		t.Errorf("probe columns = %q", v)
	}
	var p transcode.Probe
	if err := json.Unmarshal([]byte(e.str(`SELECT probe FROM media_files WHERE id = ?`, id)), &p); err != nil || len(p.Streams) < 2 {
		t.Errorf("stored probe = %+v, %v", p, err)
	}
	if n := e.count(`SELECT count(*) FROM tracks WHERE media_file_id = ? AND source = 'embedded' AND kind IN ('video', 'audio')`, id); n < 2 {
		t.Errorf("embedded tracks = %d, want video and audio", n)
	}
	if v := e.str(`SELECT group_concat(language || ':' || codec, ',') FROM (SELECT language, codec FROM tracks
	               WHERE media_file_id = ? AND source = 'sidecar' ORDER BY rel_path)`, id); v != "de:ass,de:subrip,en:ass,en:subrip" {
		t.Errorf("sidecar tracks = %q", v)
	}
	if n := e.jobCount(JobProbeKeyframes); n != 1 {
		t.Errorf("keyframe jobs = %d, want 1", n)
	}
	// Probing again replaces the tracks instead of adding them.
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM tracks WHERE media_file_id = ? AND source = 'sidecar'`, id); n != 4 {
		t.Errorf("sidecar tracks after the second probe = %d, want 4", n)
	}

	if err := e.svc.handleKeyframes(e.ctx, mediaJob(t, JobProbeKeyframes, id)); err != nil {
		t.Fatal(err)
	}
	var blob []byte
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT keyframes FROM media_files WHERE id = ? AND keyframes_at IS NOT NULL`, id).Scan(&blob); err != nil {
		t.Fatal(err)
	}
	kf, err := DecodeKeyframes(blob)
	if err != nil || len(kf) < 2 || kf[0] > time.Second || kf[1] <= kf[0] {
		t.Fatalf("keyframes = %v, %v", kf, err)
	}
	t.Logf("keyframe index: %d keyframes in %d bytes", len(kf), len(blob))

	// Chapters end up in the stored probe.
	chap := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Slouchcat Test Chapters (2024) {tmdb-7004}/%'`)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, chap)); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(e.str(`SELECT probe FROM media_files WHERE id = ?`, chap)), &p); err != nil || len(p.Chapters) < 2 {
		t.Errorf("chapters in probe = %+v, %v", p.Chapters, err)
	}

	// A media file that vanished from the database or is missing ends the job quietly.
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, "01ZZZZZZZZZZZZZZZZZZZZZZZZ")); err != nil {
		t.Errorf("probe of an unknown file = %v, want nil", err)
	}
}

// M3 review, finding 4: a probe that fails because of the environment (ffprobe not configured or
// not found) is returned for a retry and stores no probe_error; only a failure of the file does.
func TestProbeEnvironmentErrorsAreRetried(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Alien (1979)/Alien (1979).mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	id := e.str(`SELECT id FROM media_files`)
	for name, runner := range map[string]transcode.Runner{
		"not configured": {},
		"not found":      {FFprobePath: filepath.Join(t.TempDir(), "ffprobe")},
	} {
		e.svc.probe = runner
		if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err == nil {
			t.Errorf("%s: probe = nil, want an error for a retry", name)
		}
		if v := e.str(`SELECT coalesce(probe_error, '') || '/' || coalesce(probed_at, '') FROM media_files WHERE id = ?`, id); v != "/" {
			t.Errorf("%s: probe_error/probed_at = %q, want none", name, v)
		}
	}
}

// A file that comes back unchanged after missing_since is probed again, so a probe that failed
// while the file was away does not stay.
func TestReturningFileIsProbedAgain(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:10])
	lib := e.create(dir)
	e.scan(lib.ID)
	rel := twentyMovies[0]
	id := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, rel)
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE type = ?`, JobProbeFile); err != nil {
		t.Fatal(err)
	}
	away := filepath.Join(t.TempDir(), "away.mkv")
	if err := os.Rename(filepath.Join(dir, filepath.FromSlash(rel)), away); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM media_files WHERE id = ? AND missing_since IS NOT NULL`, id); n != 1 {
		t.Fatal("the file is not missing")
	}
	if err := os.Rename(away, filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND idempotency_key = ?`, JobProbeFile, "probe:"+id); n != 1 {
		t.Errorf("probe jobs for the returned file = %d, want 1", n)
	}
}

func TestKeyframeEncoding(t *testing.T) {
	t.Parallel()
	in := []time.Duration{4 * time.Second, 0, 2*time.Second + 2*time.Millisecond, -time.Millisecond, 2 * time.Hour}
	b := EncodeKeyframes(in)
	got, err := DecodeKeyframes(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{0, 0, 2*time.Second + 2*time.Millisecond, 4 * time.Second, 2 * time.Hour}
	if len(got) != len(want) {
		t.Fatalf("decoded %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decoded %v, want %v", got, want)
		}
	}
	if b[0] != keyframesFormat || len(b) > 1+3*len(in) {
		t.Errorf("encoding %x too large or without version", b)
	}
	if _, err := DecodeKeyframes([]byte{9, 1}); err == nil {
		t.Error("unknown format accepted")
	}
	if _, err := DecodeKeyframes([]byte{keyframesFormat, 0x80}); err == nil {
		t.Error("truncated varint accepted")
	}
	if kf, err := DecodeKeyframes(nil); err != nil || kf != nil {
		t.Errorf("empty blob = %v, %v", kf, err)
	}
}

func TestParseSidecar(t *testing.T) {
	t.Parallel()
	stem := "Hollow Man (2000) [imdb-tt0164052] - [German DL][Bluray-1080p][AC3 5.1][x264]-c0nFuSed"
	cases := []struct {
		name string
		ok   bool
		want sidecar
	}{
		{stem + ".de.srt", true, sidecar{lang: "de", codec: "subrip"}},
		{stem + ".de.forced.srt", true, sidecar{lang: "de", codec: "subrip", forced: true}},
		{stem + ".de.hi.srt", true, sidecar{lang: "de", codec: "subrip", hearingImpaired: true}},
		{stem + ".en.sdh.vtt", true, sidecar{lang: "en", codec: "webvtt", hearingImpaired: true}},
		{stem + ".pt-br.ass", true, sidecar{lang: "pt-br", codec: "ass"}},
		{stem + ".German.ssa", true, sidecar{lang: "german", codec: "ssa"}},
		{stem + ".srt", true, sidecar{codec: "subrip"}},
		{stem + ".EN.CC.SRT", true, sidecar{lang: "en", codec: "subrip", hearingImpaired: true}},
		{stem + ".mkv", false, sidecar{}},
		{stem + ".nfo", false, sidecar{}},
		{stem + "-320-10.bif", false, sidecar{}},
		{"Other Movie.de.srt", false, sidecar{}},
		{stem + ".de.en.extra words.srt", false, sidecar{}},
	}
	for _, c := range cases {
		got, ok := parseSidecar(stem, c.name)
		if ok != c.ok || got != c.want {
			t.Errorf("parseSidecar(%q) = %+v, %v; want %+v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestCanonicalContainer(t *testing.T) {
	t.Parallel()
	cases := []struct{ format, ext, want string }{
		{"matroska,webm", "mkv", "mkv"}, {"matroska,webm", "webm", "webm"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "mp4", "mp4"}, {"mov,mp4,m4a,3gp,3g2,mj2", "m4v", "m4v"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "mov", "mov"}, {"mov,mp4,m4a,3gp,3g2,mj2", "mkv", "mp4"},
		{"avi", "avi", "avi"}, {"mpegts", "ts", "ts"}, {"mpegts", "m2ts", "m2ts"}, {"asf", "wmv", "wmv"},
		{"mpeg", "mpg", "mpg"}, {"flv", "mkv", "mkv"},
	}
	for _, c := range cases {
		if got := canonicalContainer(c.format, c.ext); got != c.want {
			t.Errorf("canonicalContainer(%q, %q) = %q, want %q", c.format, c.ext, got, c.want)
		}
	}
}
