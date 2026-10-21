package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// introShow is the intro season of testdata/generate-media.sh: intro start (s) and duration (s)
// per episode, the intro lasts 20 s, the outro the last 25 s; E03 has the chapters Intro and Credits.
const introShow = "Slouchcat Test Intro Show (2024) {tmdb-7102}"

var (
	introStarts    = []int64{5, 12, 8, 15, 10}
	introDurations = []int64{98, 100, 102, 104, 106}
)

// ffmpegPath finds ffmpeg next to ffprobe in PATH (os/exec is reserved for package transcode).
func ffmpegPath() string {
	if p := ffprobePath(); p != "" {
		if fi, err := os.Stat(filepath.Join(filepath.Dir(p), "ffmpeg")); err == nil && fi.Mode().IsRegular() {
			return filepath.Join(filepath.Dir(p), "ffmpeg")
		}
	}
	return ""
}

func seasonJob(t *testing.T, seasonID string) *jobs.Job {
	t.Helper()
	payload, err := json.Marshal(seasonPayload{seasonID})
	if err != nil {
		t.Fatal(err)
	}
	return &jobs.Job{Type: JobSegmentsDetect, Payload: payload}
}

// probeQueued runs every queued probe.file job of the test and marks it done.
func (e *env) probeQueued() {
	e.t.Helper()
	for _, id := range e.queuedJobs(JobProbeFile) {
		payload := e.str(`SELECT payload FROM jobs WHERE id = ?`, id)
		if err := e.svc.handleProbe(e.ctx, &jobs.Job{Type: JobProbeFile, Payload: json.RawMessage(payload)}); err != nil {
			e.t.Fatal(err)
		}
		if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE id = ?`, id); err != nil {
			e.t.Fatal(err)
		}
	}
}

// queuedJobs returns the ids of the queued jobs of a type.
func (e *env) queuedJobs(typ string) []int64 {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT id FROM jobs WHERE type = ? AND state = 'queued' ORDER BY id`, typ)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			e.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return ids
}

// TestSegmentsDetectGeneratedSeason runs segments.detect on the generated intro season (W-9: intro
// within ±2 s on at least 80 % of the episodes) and checks the hooks: probes queue the job, chapters
// win, manual segments stay, a changed episode sets the season back to pending.
func TestSegmentsDetectGeneratedSeason(t *testing.T) {
	tv := filepath.Join(filepath.Dir(testMedia(t)), "TV")
	if _, err := os.Stat(filepath.Join(tv, introShow)); err != nil {
		t.Skipf("intro season missing (run `task testdata:media` in the repository root): %v", err)
	}
	if ffmpegPath() == "" {
		t.Fatal("ffmpeg is required for the fingerprint tests (install ffmpeg)")
	}
	e := newEnv(t)
	e.svc.probe.FFmpegPath = ffmpegPath()
	lib := e.createSeries(tv)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("series scan = %+v", got.Scan)
	}
	season := e.str(`SELECT s.id FROM items AS s JOIN items AS r ON r.id = s.parent_id
	                 WHERE s.kind = 'season' AND r.title = 'Slouchcat Test Intro Show'`)
	if n, err := e.svc.QueuePendingSegments(e.ctx, lib.ID); err != nil || n != 0 {
		t.Fatalf("QueuePendingSegments before any probe = %d, %v; want 0", n, err)
	}

	// The probes queue the job once per season with two or more probed episodes.
	e.probeQueued()
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobSegmentsDetect, "segments:"+season); n != 1 {
		t.Fatalf("segments.detect jobs of the intro season = %d, want 1", n)
	}
	if v := e.str(`SELECT priority FROM jobs WHERE idempotency_key = ?`, "segments:"+season); v != "2" {
		t.Errorf("priority = %s, want 2", v)
	}

	start := time.Now()
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, season)); err != nil {
		t.Fatal(err)
	}
	t.Logf("segments.detect on 5 episodes: %v", time.Since(start).Round(time.Millisecond))
	if v := e.str(`SELECT segments_state || '/' || (segments_checked_at IS NOT NULL) FROM items WHERE id = ?`, season); v != "done/1" {
		t.Fatalf("season state = %s, want done/1", v)
	}

	type seg struct {
		start, end int64
		source     string
		confidence float64
	}
	load := func(ep int, kind string) (seg, bool) {
		t.Helper()
		var s seg
		err := e.db.Read.QueryRowContext(e.ctx, `SELECT g.start_ms, g.end_ms, g.source, coalesce(g.confidence, 0) FROM media_segments AS g
			JOIN items AS i ON i.id = g.item_id WHERE i.parent_id = ? AND i.episode_number = ? AND g.kind = ?`, season, ep, kind).
			Scan(&s.start, &s.end, &s.source, &s.confidence)
		return s, err == nil
	}
	within := func(got, want int64) bool { return got >= want-2000 && got <= want+2000 }
	introOK, outroOK := 0, 0
	for i := range 5 {
		ep := i + 1
		in, okIn := load(ep, "intro")
		out, okOut := load(ep, "outro")
		wantIn, wantOut := introStarts[i]*1000, (introDurations[i]-25)*1000
		t.Logf("E%02d intro %+v (want %d-%d, Δ %+d/%+d ms), outro %+v (want %d-%d, Δ %+d/%+d ms)", ep,
			in, wantIn, wantIn+20000, in.start-wantIn, in.end-wantIn-20000,
			out, wantOut, introDurations[i]*1000, out.start-wantOut, out.end-introDurations[i]*1000)
		if okIn && within(in.start, wantIn) && within(in.end, wantIn+20000) {
			introOK++
		}
		if okOut && within(out.start, wantOut) && within(out.end, introDurations[i]*1000) {
			outroOK++
		}
		wantSource := "fingerprint"
		if ep == 3 {
			wantSource = "chapter"
		}
		if (okIn && in.source != wantSource) || (okOut && out.source != wantSource) {
			t.Errorf("E%02d sources = %s/%s, want %s", ep, in.source, out.source, wantSource)
		}
	}
	if introOK < 4 || outroOK < 4 {
		t.Errorf("intro within ±2 s for %d, outro for %d of 5 episodes; want at least 4 each", introOK, outroOK)
	}
	if in, _ := load(3, "intro"); in.start != 8000 || in.end != 28000 || in.confidence != 1 {
		t.Errorf("E03 chapter intro = %+v, want 8000-28000", in)
	}

	// Manual segments stay; a second run replaces only the detected ones.
	e1 := e.str(`SELECT id FROM items WHERE parent_id = ? AND episode_number = 1`, season)
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_segments SET source = 'manual', start_ms = 1000, end_ms = 2000 WHERE item_id = ? AND kind = 'intro'`, e1); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, season)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT source || ':' || start_ms || '-' || end_ms FROM media_segments WHERE item_id = ? AND kind = 'intro'`, e1); v != "manual:1000-2000" {
		t.Errorf("manual intro after a new run = %s", v)
	}
	if n := e.count(`SELECT count(*) FROM media_segments AS g JOIN items AS i ON i.id = g.item_id WHERE i.parent_id = ?`, season); n != 10 {
		t.Errorf("segments of the season = %d, want 10", n)
	}

	// Season 1 of the other test show (three 10-s episodes with the same tone) was queued too: its
	// heads of 4 s are too short for an intro, the shared tone does not reach the outro minimum.
	other := e.str(`SELECT s.id FROM items AS s JOIN items AS r ON r.id = s.parent_id
	                WHERE s.kind = 'season' AND s.season_number = 1 AND r.title = 'Slouchcat Test Show'`)
	if n := e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ?`, "segments:"+other); n != 1 {
		t.Errorf("segments.detect jobs of the test show = %d, want 1", n)
	}
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, other)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, other); v != "none" {
		t.Errorf("test show season = %s, want none", v)
	}
	// The download show (six 8-s episodes, a different tone each) was queued too and has none.
	download := e.str(`SELECT s.id FROM items AS s JOIN items AS r ON r.id = s.parent_id
	                   WHERE s.kind = 'season' AND r.title = 'Slouchcat Test Download Show'`)
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, download)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, download); v != "none" {
		t.Errorf("download show season = %s, want none", v)
	}

	// Done seasons are not queued again; a changed episode sets the season back to pending and
	// queues the job.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE type = ?`, JobSegmentsDetect); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'season' AND segments_state = 'pending' AND id = ?`, season); n != 0 {
		t.Fatal("season still pending")
	}
	file := e.str(`SELECT m.id FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE i.parent_id = ? AND i.episode_number = 2`, season)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, file)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, season); v != "pending" {
		t.Errorf("season after a new probe = %s, want pending", v)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, "segments:"+season); n != 1 {
		t.Errorf("queued segments.detect after a new probe = %d, want 1", n)
	}
	// With the job queued, the scan hook adds nothing; without it, it queues the pending season.
	if n, err := e.svc.QueuePendingSegments(e.ctx, lib.ID); err != nil || n != 0 {
		t.Errorf("QueuePendingSegments with an active job = %d, %v", n, err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE type = ?`, JobSegmentsDetect); err != nil {
		t.Fatal(err)
	}
	if n, err := e.svc.QueuePendingSegments(e.ctx, lib.ID); err != nil || n != 1 {
		t.Errorf("QueuePendingSegments = %d, %v; want 1 (the intro season)", n, err)
	}
	// A probe during the run leaves the season pending and keeps the stored segments: the stale
	// result is rolled back (the intros deleted here are not written again).
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM media_segments WHERE kind = 'intro'`); err != nil {
		t.Fatal(err)
	}
	stored := e.count(`SELECT count(*) FROM media_segments`)
	before := e.count(`SELECT COALESCE(max(created_at), 0) FROM media_segments`)
	e.clock.Advance(time.Second)
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET probed_at = ? WHERE id = ?`, e.clock.Now().Add(time.Hour).UnixMilli(), file); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, season)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, season); v != "pending" {
		t.Errorf("season changed during the run = %s, want pending", v)
	}
	if n, newest := e.count(`SELECT count(*) FROM media_segments`), e.count(`SELECT COALESCE(max(created_at), 0) FROM media_segments`); n != stored || newest != before {
		t.Errorf("segments after the stale run: %d rows (want %d), newest %d (want %d)", n, stored, newest, before)
	}
}

// TestSegmentsDetectWithoutFiles: a season whose episodes have no probed file ends as none, and a
// failing ffmpeg as failed.
func TestSegmentsDetectWithoutFiles(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	for _, rel := range []string{"Show (2020)/Season 01/Show (2020) S01E01.mkv", "Show (2020)/Season 01/Show (2020) S01E02.mkv"} {
		writeFile(t, dir, rel, "not a video")
	}
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	season := e.str(`SELECT id FROM items WHERE kind = 'season'`)
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, season)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, season); v != "none" {
		t.Errorf("state without probed files = %s, want none", v)
	}
	// Pretend both files were probed: ffmpeg is not configured, so fingerprinting fails.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET probed_at = 1, duration_ms = 60000, probe = '{}'`); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleSegments(e.ctx, seasonJob(t, season)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT segments_state FROM items WHERE id = ?`, season); v != "failed" {
		t.Errorf("state when every fingerprint fails = %s, want failed", v)
	}
}
