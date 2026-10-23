package offline

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Regression tests of the M5 server review (findings W1 to W4 and K1 to K11); W5 is in package
// admin, W6 and K9 in package api, K3 in package playback, K12 in package store.

// kfEncoder fails the keyframe index while failKF is set (ffprobe hung once).
type kfEncoder struct {
	*fakeEncoder
	failKF atomic.Bool
	kf     []time.Duration
}

func (k *kfEncoder) Keyframes(context.Context, string) ([]time.Duration, error) {
	if k.failKF.Load() {
		return nil, errors.New("ffprobe failed (transient)")
	}
	return k.kf, nil
}

// contiguous checks that the index entries cover the clip from 0 without gap or overlap and that
// media.mp4 is the init segment followed by the segments.
func contiguous(t *testing.T, e *env, id, init string) {
	t.Helper()
	_, entries := readEntries(t, e, id)
	var at int64
	want := []byte(init)
	for n, en := range entries {
		if en.N != n || en.StartMs != at {
			t.Errorf("entry %d = %+v, want start %d", n, en, at)
		}
		at += en.DurationMs
		want = append(want, segmentData(n)...)
	}
	if at != clipDuration.Milliseconds() {
		t.Errorf("entries cover %d ms of %d", at, clipDuration.Milliseconds())
	}
	if got := e.file(id, mediaFile); !bytes.Equal(got, want) {
		t.Errorf("media.mp4 = %q, want %q", got, want)
	}
}

// W1: attempt 1 has no keyframe index (4-s transcode) and stops after two segments; attempt 2
// finds the index and plans a remux with other boundaries. The copy starts over instead of
// appending remux segments to transcode segments.
func TestPrepareStartsOverForAnotherPlan(t *testing.T) {
	t.Parallel()
	enc := &kfEncoder{fakeEncoder: newFakeEncoder(), kf: []time.Duration{0, 6 * time.Second, 12 * time.Second}}
	e := newEnv(t, WithEncoder(enc))
	m := e.f.Rated(18)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), nil)
	enc.failKF.Store(true)
	st := e.create(e.f.P18, e.devA, m.ID, model.DownloadHigh)
	enc.failAt, enc.failCalls = 2, 1
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadQueued || r.done != 2 {
		t.Fatalf("after attempt 1: %+v", r)
	}
	enc.failKF.Store(false)
	e.clock.Advance(jobs.Backoff(1))
	e.runJobs()
	r := e.row(st.ID)
	if r.state != model.DownloadReady || r.done != 3 || r.total != 3 {
		t.Fatalf("after attempt 2: %+v", r)
	}
	if !slices.Equal(enc.modes, []string{"transcode", "remux"}) || !slices.Equal(enc.calls(), []int{0, 0}) {
		t.Errorf("modes %v, start segments %v; want the remux from segment 0", enc.modes, enc.calls())
	}
	contiguous(t, e, st.ID, initData)
	if h, _ := readEntries(t, e, st.ID); h.Plan == "" {
		t.Error("index header without plan fingerprint")
	}
}

// W1: the resumed run delivers another init segment (another backend since the last attempt): the
// copy starts over at segment 0 in the same attempt.
func TestPrepareStartsOverForAnotherInit(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.failAt, e.enc.failCalls = 3, 1
	e.runJobs()
	const other = "ftyp-moov-of-another-backend"
	e.enc.mu.Lock()
	e.enc.init = other
	e.enc.mu.Unlock()
	e.clock.Advance(jobs.Backoff(1))
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || r.done != 5 {
		t.Fatalf("row = %+v", r)
	}
	if calls := e.enc.calls(); !slices.Equal(calls, []int{0, 3, 0}) {
		t.Errorf("start segments = %v, want [0 3 0]", calls)
	}
	contiguous(t, e, st.ID, other)
}

// W1: an index entry off the planned segment times is not continued.
func TestPrepareStartsOverForOtherSegmentTimes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.failAt, e.enc.failCalls = 3, 1
	e.runJobs()
	path := filepath.Join(e.dir, st.ID, indexFile)
	idx := bytes.Replace(e.file(st.ID, indexFile), []byte(`"startMs":4000`), []byte(`"startMs":4001`), 1)
	if err := os.WriteFile(path, idx, 0o640); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(jobs.Backoff(1))
	e.runJobs()
	if calls := e.enc.calls(); !slices.Equal(calls, []int{0, 0}) {
		t.Errorf("start segments = %v, want [0 0]", calls)
	}
	contiguous(t, e, st.ID, initData)
}

// subEncoder blocks the one-run WebVTT conversion until release is closed.
type subEncoder struct {
	*fakeEncoder
	entered chan struct{}
	release chan struct{}
}

func (s *subEncoder) SubtitleStreams(ctx context.Context, path string, indexes []int) (map[int][]byte, error) {
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.fakeEncoder.SubtitleStreams(ctx, path, indexes)
}

// W2: the lease goes back before the WebVTT conversion, so a live start never waits for the
// sidecars; a preemption during them changes nothing.
func TestPrepareReleasesTheLeaseBeforeTheSidecars(t *testing.T) {
	t.Parallel()
	enc := &subEncoder{fakeEncoder: newFakeEncoder(), entered: make(chan struct{}, 1), release: make(chan struct{})}
	e := newEnv(t, WithEncoder(enc))
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	var held atomic.Bool
	go func() {
		<-enc.entered
		l := e.adm.last()
		held.Store(!l.released.Load())
		l.preempt()
		close(enc.release)
	}()
	e.runJobs()
	if held.Load() {
		t.Error("the lease was still held during the WebVTT conversion")
	}
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Errorf("row = %+v", r)
	}
	if p := e.pendingJobs(); len(p) != 0 {
		t.Errorf("pending jobs = %v", p)
	}
}

// W2: a preemption right after the last segment does not pause a complete encode.
func TestPreparePreemptedAfterTheLastSegment(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.enc.after = func(n int) {
		if n == 4 {
			e.adm.last().preempt()
			e.enc.block.Store(true)
		}
	}
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || r.done != 5 {
		t.Fatalf("row = %+v", r)
	}
	if p := e.pendingJobs(); len(p) != 0 {
		t.Errorf("pending jobs = %v", p)
	}
	if l := e.adm.last(); !l.released.Load() {
		t.Error("lease not released")
	}
}

// zombie turns the queued preparation job of a download into one whose lease expired after the
// last attempt (five container restarts during a long encode) and lets the queue fail it.
func (e *env) zombie(id string) {
	e.t.Helper()
	ctx := context.Background()
	e.exec(`UPDATE downloads SET state = 'preparing', segments_done = 2 WHERE id = ?`, id)
	e.exec(`UPDATE jobs SET state = 'running', attempts = max_attempts, lease_owner = 'dead', lease_until = 1
	        WHERE type = ? AND state = 'queued'`, JobPrepare)
	if j, err := e.queue.Claim(ctx, "w", []string{JobPrepare}); err != nil || j != nil {
		e.t.Fatalf("claim: %v, %v", j, err)
	}
	if p := e.pendingJobs(); len(p) != 0 {
		e.t.Fatalf("pending jobs = %v", p)
	}
}

// W3: a download without a preparation job is queued again by the sync of its device and by the
// reconciliation of Run; a download whose chain still holds a key is left alone.
func TestUnfinishedDownloadWithoutJobIsQueuedAgain(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.zombie(st.ID)
	e.sync(e.f.P12, e.devA)
	if r := e.row(st.ID); r.state != model.DownloadQueued {
		t.Errorf("after the sync: %+v", r)
	}
	if e.pendingJobs()[primaryKey(st.ID)].IsZero() {
		t.Fatalf("no job after the sync: %v", e.pendingJobs())
	}
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Errorf("after the job: %+v", r)
	}

	other := e.create(e.f.P18, e.devB, e.f.Rated(0).ID, model.DownloadLow)
	e.zombie(other.ID)
	if err := e.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if r := e.row(other.ID); r.state != model.DownloadQueued || e.pendingJobs()[primaryKey(other.ID)].IsZero() {
		t.Errorf("after the reconciliation: %+v, jobs %v", r, e.pendingJobs())
	}
	e.runJobs()

	// A waiting chain (its successor holds the alternate key) gets no second job.
	third := e.create(e.f.P18, e.devB, e.f.Rated(16).ID, model.DownloadLow)
	e.exec(`UPDATE jobs SET state = 'done' WHERE idempotency_key = ?`, primaryKey(third.ID))
	if err := e.svc.enqueue(ctx, third.ID, alternateKey(third.ID), budgetWait); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if p := e.pendingJobs(); len(p) != 1 || p[alternateKey(third.ID)].IsZero() {
		t.Errorf("pending jobs = %v, want only the alternate key", p)
	}
}

// W3: an error the last attempt meets outside the encoder (here the queue refuses the successor
// of a budget wait) leaves the download failed, not queued without a job.
func TestPrepareLastAttemptErrorFailsTheDownload(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	first := e.create(e.f.P18, e.devA, e.f.Rated(0).ID, model.DownloadLow)
	e.runJobs()
	e.svc.budget = e.row(first.ID).size + 100
	second := e.create(e.f.P18, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.exec(`UPDATE jobs SET attempts = max_attempts - 1 WHERE idempotency_key = ?`, primaryKey(second.ID))
	e.exec(`CREATE TRIGGER refuse_successor BEFORE INSERT ON jobs WHEN NEW.idempotency_key = '` + alternateKey(second.ID) + `'
	        BEGIN SELECT RAISE(ABORT, 'queue refused'); END`)
	e.runJobs()
	if r := e.row(second.ID); r.state != model.DownloadFailed || r.err != model.DownloadErrEncodeFailed {
		t.Errorf("row = %+v, want failed", r)
	}
}

// sourcePath is the absolute path of a fixture media file.
func (e *env) sourcePath(fileID string) string {
	e.t.Helper()
	var root, rel string
	if err := e.f.DB.Read.QueryRow(`SELECT r.path, m.rel_path FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id
	                                WHERE m.id = ?`, fileID).Scan(&root, &rel); err != nil {
		e.t.Fatal(err)
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

// W4: a source that is not probed yet or cannot be read waits like the budget, without using an
// attempt, and the preparation goes on once the source is there.
func TestPrepareWaitsForTheSource(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(12)
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", "", nil)
	st := e.create(e.f.P12, e.devA, m.ID, model.DownloadLow)
	waits := func(id string) {
		t.Helper()
		for i := range jobs.DefaultMaxAttempts + 1 {
			if n := e.runJobs(); n != 1 {
				t.Fatalf("wait %d: %d jobs ran", i, n)
			}
			if r := e.row(id); r.state != model.DownloadQueued || r.err != "" {
				t.Fatalf("wait %d: %+v", i, r)
			}
			p := e.pendingJobs()
			if len(p) != 1 {
				t.Fatalf("wait %d: pending jobs %v", i, p)
			}
			for _, at := range p {
				if !at.Equal(e.clock.Now().Add(sourceWait)) {
					t.Fatalf("wait %d: successor at %v", i, at)
				}
			}
			e.clock.Advance(sourceWait)
		}
	}
	waits(st.ID)
	if len(e.enc.calls()) != 0 {
		t.Fatal("an unprobed source was encoded")
	}
	e.f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), keyframeBlob())
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Fatalf("after the probe: %+v", r)
	}

	// The share is gone: the file cannot be opened.
	other := e.f.Rated(6)
	path := e.sourcePath(other.FileID)
	if err := os.Rename(path, path+".away"); err != nil {
		t.Fatal(err)
	}
	second := e.create(e.f.P12, e.devA, other.ID, model.DownloadLow)
	waits(second.ID)
	if err := os.Rename(path+".away", path); err != nil {
		t.Fatal(err)
	}
	e.runJobs()
	if r := e.row(second.ID); r.state != model.DownloadReady {
		t.Errorf("after the share came back: %+v", r)
	}
}

// W4: an explicit createDownloads of a failed download prepares it again from the start.
func TestCreateRetriesAFailedDownload(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(6)
	st := e.create(e.f.P12, e.devA, m.ID, model.DownloadLow)
	e.runJobs()
	e.exec(`UPDATE downloads SET state = 'failed', error = 'encode_failed', segments_done = 3, profile_id = ? WHERE id = ?`, e.f.P16.ID, st.ID)
	again := e.create(e.f.P12, e.devA, m.ID, model.DownloadLow)
	if again.ID != st.ID || again.State != model.DownloadQueued || again.Error != "" || again.SegmentsDone != 0 {
		t.Fatalf("again = %+v", again)
	}
	if exists(t, e, st.ID) {
		t.Error("the partial files of the failed download remain")
	}
	var profile int64
	if err := e.f.DB.Read.QueryRow(`SELECT profile_id FROM downloads WHERE id = ?`, st.ID).Scan(&profile); err != nil || profile != e.f.P12.ID {
		t.Errorf("requesting profile = %d, %v; want P12", profile, err)
	}
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Errorf("row = %+v", r)
	}
}

// autoRule enables the next-episode rule of p on device with maxCount and marks episode itemID
// finished by p ago.
func (e *env) autoRule(p model.Profile, device string, maxCount int, itemID string, ago time.Duration) {
	e.t.Helper()
	if _, err := e.svc.SetRules(context.Background(), e.session(p, device), model.DownloadRules{Enabled: true, NextEpisodes: true,
		WifiOnly: true, MaxCount: maxCount, Quality: model.DownloadLow}); err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, 2000, 1, ?)
	        ON CONFLICT (profile_id, item_id) DO UPDATE SET completed = 1, updated_at = excluded.updated_at`,
		p.ID, itemID, e.clock.Now().Add(-ago).UnixMilli())
}

// W4: a failed auto-download takes no place in maxCount and is tried again after the wait.
func TestAutoDownloadRetriesAfterAFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	e.autoRule(f.P16, e.devA, 1, f.Show.E1.ID, time.Hour)
	s := e.sync(f.P16, e.devA)
	if len(s.Downloads) != 1 || !s.Downloads[0].Auto {
		t.Fatalf("auto-downloads = %+v", s.Downloads)
	}
	id := s.Downloads[0].ID
	e.exec(`UPDATE downloads SET state = 'failed', error = 'encode_failed', updated_at = ? WHERE id = ?`, e.clock.Now().UnixMilli(), id)
	e.exec(`DELETE FROM jobs`)
	open, err := e.svc.read.CountOpenAutoDownloads(ctx, systemq.CountOpenAutoDownloadsParams{ProfileID: nullID(f.P16.ID), DeviceID: e.devA})
	if err != nil || open != 0 {
		t.Errorf("open auto-downloads with a failed one = %d, %v; want 0", open, err)
	}
	e.sync(f.P16, e.devA)
	if r := e.row(id); r.state != model.DownloadFailed {
		t.Errorf("before the wait: %+v", r)
	}
	e.clock.Advance(autoRetryWait)
	e.sync(f.P16, e.devA)
	if r := e.row(id); r.state != model.DownloadQueued || r.err != "" || e.pendingJobs()[primaryKey(id)].IsZero() {
		t.Fatalf("after the wait: %+v, jobs %v", r, e.pendingJobs())
	}
	e.runJobs()
	if r := e.row(id); r.state != model.DownloadReady {
		t.Errorf("row = %+v", r)
	}
}

func nullID(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// K1: a profile deletes a download only when it may see the title, requested it or is the
// primary profile; any other download of the account is 404, like an invisible title.
func TestDeleteNeedsARightOnTheDownload(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	hidden := e.create(f.P18, e.devA, f.Rated(18).ID, model.DownloadLow)
	six := e.create(f.P12, e.devA, f.Rated(6).ID, model.DownloadLow)
	// P6 knows the id from its own sync (the download without title) but may not delete it.
	if err := e.svc.Delete(ctx, e.session(f.P6, e.devA), hidden.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 deletes P18's FSK-18 download: %v, want ErrNotFound", err)
	}
	// P0 does not see the 6 film, did not request it and is not primary.
	if err := e.svc.Delete(ctx, e.session(f.P0, e.devA), six.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P0 deletes P12's download: %v, want ErrNotFound", err)
	}
	if r := e.row(hidden.ID); r.state == "" {
		t.Fatal("download gone")
	}
	// P16 may see the 6 film (allowedProfileIds); P18 is the primary profile.
	if err := e.svc.Delete(ctx, e.session(f.P16, e.devA), six.ID); err != nil {
		t.Errorf("P16 deletes a download it may see: %v", err)
	}
	if err := e.svc.Delete(ctx, e.session(f.P18, e.devA), hidden.ID); err != nil {
		t.Errorf("primary profile deletes: %v", err)
	}
	// The requesting profile, also when its rights shrank since.
	twelve := e.create(f.P12, e.devB, f.Rated(12).ID, model.DownloadLow)
	sixteen := 16
	f.SetRating(t, f.Rated(12).ID, &sixteen)
	e.sync(f.P12, e.devB)
	if err := e.svc.Delete(ctx, e.session(f.P12, e.devB), twelve.ID); err != nil {
		t.Errorf("requesting profile deletes: %v", err)
	}
}

// gateAdmission blocks AcquireDownload until gate is closed (live playback holds the budget).
type gateAdmission struct {
	fakeAdmission
	gate chan struct{}
}

func (a *gateAdmission) AcquireDownload(ctx context.Context, id string, cost float64) (Lease, error) {
	select {
	case <-a.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return a.fakeAdmission.AcquireDownload(ctx, id, cost)
}

// K2: while admission waits the download is paused, preparing once the lease is held.
// Not parallel: this test waits on the clock (an interval, a debounce window or a
// cancellation), and parallel load would stretch the wait past what it asserts.
func TestPreparePausedWhileWaitingForAdmission(t *testing.T) {
	adm := &gateAdmission{gate: make(chan struct{})}
	e := newEnv(t, WithAdmission(adm))
	st := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	waiting := make(chan string, 1)
	go func() {
		state := ""
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if err := e.f.DB.Read.QueryRow(`SELECT state FROM downloads WHERE id = ?`, st.ID).Scan(&state); err != nil || state == model.DownloadPaused {
				break
			}
		}
		waiting <- state
		close(adm.gate)
	}()
	var during string
	e.enc.after = func(n int) {
		if n == 0 {
			during = e.row(st.ID).state
		}
	}
	e.runJobs()
	if got := <-waiting; got != model.DownloadPaused {
		t.Errorf("state while admission waits = %q, want paused", got)
	}
	if during != model.DownloadPreparing {
		t.Errorf("state during the encode = %q, want preparing", during)
	}
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Errorf("row = %+v", r)
	}
}

// K4: the embedded text tracks convert in one run; when that fails, each on its own.
func TestSidecarsConvertTheEmbeddedTracksInOneRun(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	m := e.f.Rated(12)
	st := e.create(e.f.P12, e.devA, m.ID, model.DownloadLow)
	e.runJobs()
	ft := e.tracks[m.FileID]
	if len(e.enc.streamRuns) != 1 || !slices.Equal(e.enc.streamRuns[0], []int{3}) || len(e.enc.single) != 0 {
		t.Errorf("runs %v, single conversions %v; want one run of stream 3", e.enc.streamRuns, e.enc.single)
	}
	if got := e.file(st.ID, "subs/"+strconv.FormatInt(ft.Text, 10)+".vtt"); !bytes.Contains(got, []byte("stream 3")) {
		t.Errorf("embedded subtitle = %q", got)
	}

	e.enc.failStreams = true
	other := e.create(e.f.P12, e.devA, e.f.Rated(6).ID, model.DownloadLow)
	e.runJobs()
	if !slices.Equal(e.enc.single, []int{3}) {
		t.Errorf("single conversions after a failed run = %v, want [3]", e.enc.single)
	}
	ot := e.tracks[e.f.Rated(6).FileID]
	if got := e.file(other.ID, "subs/"+strconv.FormatInt(ot.Text, 10)+".vtt"); !bytes.Contains(got, []byte("stream 3")) {
		t.Errorf("embedded subtitle after the fallback = %q", got)
	}
}

// K6: a failing auto-download rule does not cost the device its revocation list and profiles.
func TestSyncSurvivesAFailingRule(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	gone := e.create(f.P18, e.devA, f.Rated(0).ID, model.DownloadLow)
	if err := e.svc.Delete(ctx, e.session(f.P18, e.devA), gone.ID); err != nil {
		t.Fatal(err)
	}
	e.autoRule(f.P16, e.devA, 3, f.Show.E1.ID, time.Hour)
	e.exec(`CREATE TRIGGER refuse_auto BEFORE INSERT ON downloads WHEN NEW.auto = 1 BEGIN SELECT RAISE(ABORT, 'disk full'); END`)
	s, err := e.svc.Sync(ctx, e.session(f.P6, e.devA), model.OfflineSyncRequest{Held: []model.HeldDownload{{ID: gone.ID, Complete: true}}})
	if err != nil {
		t.Fatalf("sync with a failing rule: %v", err)
	}
	if !slices.Equal(s.Revoked, []string{gone.ID}) || len(s.Profiles) == 0 {
		t.Errorf("sync = revoked %v, profiles %v", s.Revoked, profileIDs(s))
	}
}

// K7: a season whose every episode another account holds on the device is not downloadable.
func TestCreateSeasonWithoutCreatableEpisode(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	if _, err := e.svc.Create(ctx, e.session(f.P18, e.devA), f.Show.Season.ID, model.DownloadRequest{Quality: model.DownloadLow}); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Create(ctx, e.session(f.PG, e.devA), f.Show.Season.ID, model.DownloadRequest{Quality: model.DownloadLow})
	if !errors.Is(err, model.ErrNotDownloadable) {
		t.Errorf("guest season on the owner's device: %v, want ErrNotDownloadable", err)
	}
}

// K10: only a missing copy or an unreadable index evicts, and then the files go; another read
// error leaves the copy alone.
func TestManifestEvictsOnlyAMissingCopy(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	st := e.create(e.f.P12, e.devA, e.f.Rated(12).ID, model.DownloadLow)
	e.runJobs()
	session := e.session(e.f.P12, e.devA)
	index := filepath.Join(e.dir, st.ID, indexFile)
	saved := e.file(st.ID, indexFile)
	// An I/O error (here: the index is a directory) passes; the copy stays.
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(index, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Manifest(ctx, session, st.ID); err == nil || errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("read error: %v, want a plain error", err)
	}
	if r := e.row(st.ID); r.state != model.DownloadReady || !exists(t, e, st.ID) {
		t.Fatalf("after a read error: %+v", r)
	}
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, saved, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Manifest(ctx, session, st.ID); err != nil {
		t.Fatalf("restored: %v", err)
	}
	// An unreadable index evicts and removes the files.
	if err := os.WriteFile(index, []byte("garbage\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Manifest(ctx, session, st.ID); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("unreadable index: %v, want ErrDownloadEvicted", err)
	}
	if r := e.row(st.ID); r.state != model.DownloadEvicted || exists(t, e, st.ID) {
		t.Errorf("after the eviction: %+v, files remain %v", r, exists(t, e, st.ID))
	}
}

// K11: the cleanup removes the files before the row turns evicted (here the update fails, the
// files are gone already and the manifest heals the row).
func TestCleanupRemovesTheFilesBeforeEvicting(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	st := e.create(e.f.P12, e.devA, e.f.Rated(12).ID, model.DownloadLow)
	e.runJobs()
	e.clock.Advance(unfetchedKeep + time.Minute)
	e.exec(`CREATE TRIGGER refuse_evict BEFORE UPDATE OF state ON downloads WHEN NEW.state = 'evicted'
	        BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`)
	if err := e.svc.Cleanup(ctx); err == nil {
		t.Fatal("cleanup without error")
	}
	if exists(t, e, st.ID) {
		t.Error("files remain while the row is still ready")
	}
	e.exec(`DROP TRIGGER refuse_evict`)
	if _, err := e.svc.Manifest(ctx, e.session(e.f.P12, e.devA), st.ID); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("manifest: %v, want ErrDownloadEvicted", err)
	}
}
