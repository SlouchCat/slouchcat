package offline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// wantMedia is the file of a complete preparation by the fake encoder: the init segment and the
// five segments.
func wantMedia(segments int) []byte {
	out := []byte(initData)
	for n := range segments {
		out = append(out, segmentData(n)...)
	}
	return out
}

// readEntries parses index.jsonl of a download.
func readEntries(t *testing.T, e *env, id string) (indexHeader, []indexEntry) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(e.file(id, indexFile)), "\n"), "\n")
	var h indexHeader
	if err := json.Unmarshal([]byte(lines[0]), &h); err != nil {
		t.Fatal(err)
	}
	var out []indexEntry
	for _, l := range lines[1:] {
		var en indexEntry
		if err := json.Unmarshal([]byte(l), &en); err != nil {
			t.Fatal(err)
		}
		out = append(out, en)
	}
	return h, out
}

func TestPrepareAppendsSegmentsIndexAndSubtitles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	f := e.f
	m := f.Rated(12)
	st := e.create(f.P12, e.devA, m.ID, model.DownloadLow)
	if n := e.runJobs(); n != 1 {
		t.Fatalf("%d jobs ran", n)
	}
	r := e.row(st.ID)
	media := wantMedia(5)
	if r.state != model.DownloadReady || r.done != 5 || r.total != 5 || r.size != int64(len(media)) || r.err != "" {
		t.Fatalf("row = %+v", r)
	}
	if got := e.file(st.ID, mediaFile); !bytes.Equal(got, media) {
		t.Errorf("media.mp4 = %q", got)
	}
	h, entries := readEntries(t, e, st.ID)
	if h.Init != int64(len(initData)) || h.Codecs != "avc1.640029,mp4a.40.2" || h.Width != 852 || h.Height != 480 || len(entries) != 5 {
		t.Fatalf("index header %+v with %d entries", h, len(entries))
	}
	offset := h.Init
	for n, en := range entries {
		want := indexEntry{N: n, StartMs: int64(n) * 4000, DurationMs: 4000, Offset: offset, Length: int64(len(segmentData(n)))}
		if en != want {
			t.Errorf("entry %d = %+v, want %+v", n, en, want)
		}
		offset += en.Length
	}
	// WebVTT sidecars of the text tracks (embedded SubRip, sidecar file), not of the PGS track.
	ft := e.tracks[m.FileID]
	if got := string(e.file(st.ID, "subs/"+strconv.FormatInt(ft.Text, 10)+".vtt")); !strings.HasPrefix(got, "WEBVTT") || !strings.Contains(got, "stream 3") {
		t.Errorf("embedded subtitle = %q", got)
	}
	if got := string(e.file(st.ID, "subs/"+strconv.FormatInt(ft.Sidecar, 10)+".vtt")); got != "WEBVTT\n\n"+vistest.SidecarContent {
		t.Errorf("sidecar subtitle = %q", got)
	}
	if _, err := os.Stat(filepath.Join(e.dir, st.ID, "subs", strconv.FormatInt(ft.Bitmap, 10)+".vtt")); err == nil {
		t.Error("the bitmap track was converted")
	}
	// The encode took admission budget at the manager's cost and gave it back.
	if l := e.adm.last(); l == nil || !l.released.Load() || e.adm.costs[0] != 1.5+0.25 {
		t.Errorf("lease %+v, costs %v", l, e.adm.costs)
	}
	// A further job for a ready download does nothing.
	if err := e.svc.enqueue(context.Background(), st.ID, primaryKey(st.ID), 0); err != nil {
		t.Fatal(err)
	}
	e.runJobs()
	if calls := e.enc.calls(); len(calls) != 1 {
		t.Errorf("encoder calls = %v", calls)
	}
}

func TestPrepareRemuxesCopyableSource(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P18, e.devA, e.f.Rated(18).ID, model.DownloadHigh)
	e.runJobs()
	r := e.row(st.ID)
	var mode string
	if err := e.f.DB.Read.QueryRow(`SELECT mode FROM downloads WHERE id = ?`, st.ID).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	// Keyframes every 2 s: remux segments of at least 4 s on keyframes, five again.
	if r.state != model.DownloadReady || mode != "remux" || r.total != 5 || e.enc.modes[0] != "remux" {
		t.Errorf("row %+v, mode %s", r, mode)
	}
	if h, _ := readEntries(t, e, st.ID); h.Codecs != "avc1.640029,mp4a.40.2" || h.Width != 1920 || h.Height != 1080 {
		t.Errorf("remux header = %+v", h)
	}
}

// keyframeJob reads the full keyframe read of a media file that waits or runs: priority and
// pause override.
func keyframeJob(t *testing.T, e *env, fileID string) (priority int, override bool) {
	t.Helper()
	err := e.f.DB.Read.QueryRow(`SELECT priority, override_pause FROM jobs WHERE type = ? AND idempotency_key = ? AND state IN ('queued', 'running')`,
		library.JobProbeKeyframes, "keyframes:"+fileID).Scan(&priority, &override)
	if err != nil {
		t.Fatalf("keyframe job of %s: %v", fileID, err)
	}
	return priority, override
}

// Concept 14, 6.8: a remux without keyframe index never reads the file itself. The download stays
// queued without an error, the full read is queued at priority 5 without the pause override (a
// waiting job of the scan is raised), and the preparation tries again in 10 minutes.
func TestPrepareWithoutKeyframesWaitsAndQueuesTheIndex(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	if _, err := e.queue.Enqueue(context.Background(), jobs.NewJob{Type: library.JobProbeKeyframes, Priority: 1,
		IdempotencyKey: "keyframes:" + m.FileID, Payload: []byte(`{"mediaFileId":"` + m.FileID + `"}`)}); err != nil {
		t.Fatal(err)
	}
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	if n := e.runJobs(); n != 1 {
		t.Fatalf("%d jobs ran", n)
	}
	if r := e.row(st.ID); r.state != model.DownloadQueued || r.err != "" || r.done != 0 {
		t.Errorf("row = %+v, want queued without error", r)
	}
	if calls := e.enc.calls(); len(calls) != 0 {
		t.Errorf("encoder calls = %v, want none", calls)
	}
	if prio, override := keyframeJob(t, e, m.FileID); prio != 5 || override {
		t.Errorf("keyframe job priority %d override %t, want 5 without the pause override", prio, override)
	}
	if p := e.pendingJobs(); len(p) != 1 || !p[alternateKey(st.ID)].Equal(e.clock.Now().Add(10*time.Minute)) {
		t.Errorf("pending preparations = %v, want one in 10 min", p)
	}
	// The full read stored the index: the next attempt remuxes.
	e.f.SetKeyframes(t, m.FileID, keyframeBlob(), library.KeyframesCues, e.clock.Now().UnixMilli())
	e.clock.Advance(10 * time.Minute)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || !slices.Equal(e.enc.modes, []string{"remux"}) {
		t.Errorf("after the index: row %+v, modes %v", r, e.enc.modes)
	}
}

// Concept 14, O-8: the attempts of a download waiting for its keyframe index move apart, 10 min,
// 30 min, then hourly, by the time since the download was created.
func TestPrepareKeyframesWaitGrows(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	for i, want := range []time.Duration{10 * time.Minute, 30 * time.Minute, time.Hour, time.Hour} {
		if n := e.runJobs(); n != 1 {
			t.Fatalf("wait %d: %d jobs ran", i, n)
		}
		p := e.pendingJobs()
		if len(p) != 1 {
			t.Fatalf("wait %d: pending %v", i, p)
		}
		for _, at := range p {
			if got := at.Sub(e.clock.Now()); got != want {
				t.Errorf("wait %d = %v, want %v", i, got, want)
			}
			e.clock.Advance(at.Sub(e.clock.Now()))
		}
	}
	if r := e.row(st.ID); r.state != model.DownloadQueued || r.err != "" || len(e.enc.calls()) != 0 {
		t.Errorf("row = %+v, encoder calls %v", r, e.enc.calls())
	}
	for _, c := range []struct{ waited, want time.Duration }{
		{0, 10 * time.Minute}, {9 * time.Minute, 10 * time.Minute}, {10 * time.Minute, 30 * time.Minute},
		{39 * time.Minute, 30 * time.Minute}, {40 * time.Minute, time.Hour}, {23 * time.Hour, time.Hour},
		{23*time.Hour + 30*time.Minute, 30 * time.Minute},
	} {
		if got := keyframesWait(c.waited); got != c.want {
			t.Errorf("keyframesWait(%v) = %v, want %v", c.waited, got, c.want)
		}
	}
}

// Concept 14, O-8: 24 h after the download was created it is encoded without the index.
func TestPrepareEncodesADayAfterCreationWithoutKeyframes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	e.runJobs()
	e.clock.Advance(keyframesPatience - time.Minute)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadQueued || len(e.enc.calls()) != 0 {
		t.Fatalf("before the day is over: %+v, encoder calls %v", r, e.enc.calls())
	}
	for _, at := range e.pendingJobs() {
		if !at.Equal(e.clock.Now().Add(time.Minute)) {
			t.Errorf("last wait until %v, want the end of the day %v", at, e.clock.Now().Add(time.Minute))
		}
	}
	e.clock.Advance(time.Minute)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || !slices.Equal(e.enc.modes, []string{"transcode"}) {
		t.Errorf("after a day: row %+v, modes %v", r, e.enc.modes)
	}
}

// Concept 14, 6.8: a file whose index is impossible (none) is encoded at once, nothing is queued.
func TestPrepareEncodesAtOnceWithoutPossibleKeyframes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	e.f.SetKeyframes(t, m.FileID, nil, library.KeyframesNone, e.clock.Now().UnixMilli())
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || !slices.Equal(e.enc.modes, []string{"transcode"}) {
		t.Errorf("row %+v, modes %v", r, e.enc.modes)
	}
	var n int
	if err := e.f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ?`, library.JobProbeKeyframes).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d keyframe jobs (%v), want none", n, err)
	}
}

// failKeyframeRead ends the full read of a media file as failed now (the state the job of
// package library leaves after its last attempt).
func failKeyframeRead(t *testing.T, e *env, fileID string) {
	t.Helper()
	id, err := e.queue.Enqueue(context.Background(), jobs.NewJob{Type: library.JobProbeKeyframes, Priority: 1,
		IdempotencyKey: "keyframes:" + fileID, Payload: []byte(`{"mediaFileId":"` + fileID + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE jobs SET state = 'failed', finished_at = ? WHERE id = ?`, e.clock.Now().UnixMilli(), id)
}

// Concept 14, 6.8 with O-17 (review of wave D): the last full read of the file failed within
// 24 h, so nothing is queued and nothing would end the wait; the first attempt encodes.
func TestPrepareEncodesAtOnceAfterAFailedKeyframeRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	failKeyframeRead(t, e, m.FileID)
	e.clock.Advance(time.Hour)
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	if n := e.runJobs(); n != 1 {
		t.Fatalf("%d jobs ran, want the one attempt", n)
	}
	if r := e.row(st.ID); r.state != model.DownloadReady || !slices.Equal(e.enc.modes, []string{"transcode"}) {
		t.Errorf("row %+v, modes %v; want encoded in the first attempt", r, e.enc.modes)
	}
	var n int
	if err := e.f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND state IN ('queued', 'running')`,
		library.JobProbeKeyframes).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d pending keyframe jobs (%v), want none", n, err)
	}
}

// Final review, m1: the last full read failed within 24 h, but the backfill (which does not look at
// O-17) has queued a new one meanwhile; the download waits for it instead of encoding, and the
// waiting read rises to priority 5.
func TestPrepareWaitsForAQueuedKeyframeReadAfterAFailedOne(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	failKeyframeRead(t, e, m.FileID)
	e.clock.Advance(time.Hour)
	queued, err := e.queue.Enqueue(context.Background(), jobs.NewJob{Type: library.JobProbeKeyframes, Priority: 1,
		IdempotencyKey: "keyframes:" + m.FileID, Payload: []byte(`{"mediaFileId":"` + m.FileID + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadQueued || len(e.enc.calls()) != 0 {
		t.Fatalf("row %+v, encoder calls %v; want waiting for the queued read", r, e.enc.calls())
	}
	var prio int
	if err := e.f.DB.Read.QueryRow(`SELECT priority FROM jobs WHERE id = ?`, queued).Scan(&prio); err != nil || prio != 5 {
		t.Errorf("priority of the queued read = %d (%v), want 5", prio, err)
	}
}

// Concept 14, 6.8 (review of wave D): the full read the download waits for fails; the next attempt
// encodes instead of waiting for the rest of the day.
func TestPrepareEncodesWhenTheKeyframeReadFailsDuringTheWait(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadQueued || len(e.enc.calls()) != 0 {
		t.Fatalf("first attempt: %+v, encoder calls %v; want waiting", r, e.enc.calls())
	}
	e.exec(`UPDATE jobs SET state = 'failed', finished_at = ? WHERE type = ? AND idempotency_key = ?`,
		e.clock.Now().UnixMilli(), library.JobProbeKeyframes, "keyframes:"+m.FileID)
	e.clock.Advance(10 * time.Minute)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || !slices.Equal(e.enc.modes, []string{"transcode"}) {
		t.Errorf("after the failed read: row %+v, modes %v; want encoded", r, e.enc.modes)
	}
}

// A crash in the middle of a write: the next attempt cuts media.mp4 and index.jsonl back to the
// last complete segment and continues there.
func TestPrepareResumesAfterCrash(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.failAt, e.enc.failCalls = 3, 1
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadQueued || r.done != 3 {
		t.Fatalf("after the failed attempt: %+v", r)
	}
	// Half a segment and half an index line behind the third segment.
	appendTo(t, filepath.Join(e.dir, st.ID, mediaFile), []byte("moof-mdat-seg"))
	appendTo(t, filepath.Join(e.dir, st.ID, indexFile), []byte(`{"n":3,"startMs":12000,"dura`))
	e.clock.Advance(jobs.Backoff(1))
	e.runJobs()
	if calls := e.enc.calls(); !slices.Equal(calls, []int{0, 3}) {
		t.Errorf("encoder start segments = %v, want [0 3]", calls)
	}
	r := e.row(st.ID)
	if r.state != model.DownloadReady || r.done != 5 {
		t.Fatalf("after the resumed attempt: %+v", r)
	}
	if got := e.file(st.ID, mediaFile); !bytes.Equal(got, wantMedia(5)) {
		t.Errorf("media.mp4 after the resume = %q", got)
	}
	if _, entries := readEntries(t, e, st.ID); len(entries) != 5 {
		t.Errorf("index after the resume has %d entries", len(entries))
	}
}

func appendTo(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// AN-20: with the budget full a preparation waits; it starts once room is free.
func TestPrepareWaitsForBudget(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first := e.create(e.f.P18, e.devA, e.f.Rated(0).ID, model.DownloadLow)
	e.runJobs()
	used := e.row(first.ID).size
	// Room for the first copy only.
	e.svc.budget = used + 100
	second := e.create(e.f.P18, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.runJobs()
	if r := e.row(second.ID); r.state != model.DownloadQueued || r.err != model.DownloadErrBudgetFull {
		t.Fatalf("second = %+v", r)
	}
	jobsByKey := e.pendingJobs()
	if at := jobsByKey[alternateKey(second.ID)]; !at.Equal(e.clock.Now().Add(budgetWait)) || len(jobsByKey) != 1 {
		t.Fatalf("pending jobs = %v", jobsByKey)
	}
	// Before the delay nothing runs; after it the budget is still full: waits again (back on the
	// primary key).
	e.clock.Advance(budgetWait)
	e.runJobs()
	if jobsByKey := e.pendingJobs(); jobsByKey[primaryKey(second.ID)].IsZero() {
		t.Fatalf("pending jobs after the second wait = %v", jobsByKey)
	}
	// The first copy goes: the second runs.
	if err := e.svc.Delete(context.Background(), e.session(e.f.P18, e.devA), first.ID); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(budgetWait)
	e.runJobs()
	if r := e.row(second.ID); r.state != model.DownloadReady || r.err != "" {
		t.Errorf("second after the delete = %+v", r)
	}
	// A download larger than the whole budget runs when nothing else is stored.
	e.svc.budget = 10
	third := e.create(e.f.P18, e.devB, e.f.Rated(16).ID, model.DownloadLow)
	e.exec(`DELETE FROM downloads WHERE id = ?`, second.ID)
	e.runJobs()
	if r := e.row(third.ID); r.state != model.DownloadReady {
		t.Errorf("oversized download alone = %+v", r)
	}
}

// Plan 3.5: live playback preempts the lease; the preparation pauses, gives the budget back and
// continues 30 s later where it stopped.
func TestPreparePausesOnPreemption(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.after = func(n int) {
		if n == 1 {
			e.adm.last().preempt()
			e.enc.block.Store(true)
		}
	}
	e.runJobs()
	r := e.row(st.ID)
	if r.state != model.DownloadPaused || r.done != 2 {
		t.Fatalf("after the preemption: %+v", r)
	}
	if l := e.adm.last(); !l.released.Load() {
		t.Error("the preempted lease was not released")
	}
	if at := e.pendingJobs()[alternateKey(st.ID)]; !at.Equal(e.clock.Now().Add(preemptWait)) {
		t.Fatalf("pending jobs = %v", e.pendingJobs())
	}
	e.enc.after = nil
	e.enc.block.Store(false)
	e.clock.Advance(preemptWait)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || r.done != 5 {
		t.Fatalf("after the resume: %+v", r)
	}
	if calls := e.enc.calls(); !slices.Equal(calls, []int{0, 2}) {
		t.Errorf("encoder start segments = %v, want [0 2]", calls)
	}
	if got := e.file(st.ID, mediaFile); !bytes.Equal(got, wantMedia(5)) {
		t.Errorf("media.mp4 = %q", got)
	}
}

// A failing encode is retried by the queue; after the last attempt the download is failed.
func TestPrepareFailsAfterLastAttempt(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.failAt, e.enc.failCalls = 0, 100
	for attempt := 1; attempt <= jobs.DefaultMaxAttempts; attempt++ {
		if n := e.runJobs(); n != 1 {
			t.Fatalf("attempt %d: %d jobs ran", attempt, n)
		}
		want := model.DownloadQueued
		if attempt == jobs.DefaultMaxAttempts {
			want = model.DownloadFailed
		}
		if r := e.row(st.ID); r.state != want {
			t.Fatalf("attempt %d: state %s, want %s", attempt, r.state, want)
		}
		e.clock.Advance(jobs.Backoff(attempt))
	}
	if r := e.row(st.ID); r.err != model.DownloadErrEncodeFailed {
		t.Errorf("error = %q", r.err)
	}
	if n := e.runJobs(); n != 0 {
		t.Errorf("%d jobs after the last attempt", n)
	}
}

// The title becomes invisible for the requesting profile before the job runs: failed not_visible.
func TestPrepareFailsWhenNoLongerVisible(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(12)
	st := e.create(e.f.P12, e.devA, m.ID, model.DownloadLow)
	sixteen := 16
	e.f.SetRating(t, m.ID, &sixteen)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadFailed || r.err != model.DownloadErrNotVisible {
		t.Errorf("row = %+v", r)
	}
	if len(e.enc.calls()) != 0 {
		t.Error("an invisible title was encoded")
	}
}

// A download deleted during its preparation stops the encode and leaves no files.
func TestPrepareStopsWhenDeleted(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.after = func(n int) {
		if n == 1 {
			e.exec(`DELETE FROM downloads WHERE id = ?`, st.ID)
		}
	}
	e.runJobs()
	if exists(t, e, st.ID) {
		t.Error("files of the deleted download remain")
	}
	if calls := e.enc.calls(); len(calls) != 1 {
		t.Errorf("encoder calls = %v", calls)
	}
}

// The download worker of main.go: a worker of the pool download only claims download.prepare
// (RegisterTypes puts the type there).
// Not parallel: this test waits on the clock (an interval, a debounce window or a
// cancellation), and parallel load would stretch the wait past what it asserts.
func TestRegisterTypesClaimsOnlyPreparations(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := jobs.NewQueue(e.f.DB.Write, nil)
	e.svc.queue = q
	scan, err := q.Enqueue(ctx, jobs.NewJob{Type: "scan.library", Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	e.svc.now = time.Now
	st := e.create(e.f.P18, e.devA, e.f.Rated(0).ID, model.DownloadLow)
	w := jobs.NewWorker(q, "download-1", vistest.Logger())
	registry := jobs.NewRegistry(q, vistest.Logger())
	e.svc.RegisterTypes(registry)
	registry.Attach(w, jobs.PoolDownload)
	if ty, ok := registry.Lookup(JobPrepare); !ok || ty.Pool != jobs.PoolDownload || ty.Trigger != jobs.TriggerEvent || ty.Cancel == nil {
		t.Fatalf("download.prepare = %+v, want an event type of the pool download with a cancel hook", ty)
	}
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for e.row(st.ID).state != model.DownloadReady {
		if time.Now().After(deadline) {
			t.Fatalf("download not ready: %+v", e.row(st.ID))
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	j, err := q.Get(context.Background(), scan)
	if err != nil || j.State != jobs.StateQueued || j.Attempts != 0 {
		t.Errorf("scan job = %+v, %v", j, err)
	}
}
