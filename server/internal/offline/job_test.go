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
