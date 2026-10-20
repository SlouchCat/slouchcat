package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Test environment: the vistest fixture (movies rated 0 to 18, the series tree, the guest account,
// tracks) with stored probes, devices, a fake encoder, a fake admission control, fake PIN and
// watch-time ports and one clock for the service and the job queue.

var testKey = []byte("offline-test-download-token-key!")

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// clipDuration is the duration of every probed fixture file: five 4-s segments.
const clipDuration = 20 * time.Second

// testProbe is an H.264 1080p SDR source at 4 Mbit/s with a default AAC stereo track (index 1),
// an AC-3 5.1 track (2), a SubRip (3) and a PGS (4) subtitle, like vistest.AddTracks.
func testProbe() transcode.Probe {
	return transcode.Probe{Format: transcode.Format{Name: "mov,mp4,m4a", Duration: clipDuration, BitRate: 4_000_000}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "h264", Profile: "High", Width: 1920, Height: 1080, PixFmt: "yuv420p"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2, Default: true, Language: "de"},
		{Index: 2, Type: "audio", Codec: "ac3", Channels: 6, Language: "en"},
		{Index: 3, Type: "subtitle", Codec: "subrip", Language: "en"},
		{Index: 4, Type: "subtitle", Codec: "hdmv_pgs_subtitle", Language: "de"},
	}}
}

func probeJSON(t testing.TB, p transcode.Probe) string {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// keyframeBlob is a keyframe every 2 s over the clip.
func keyframeBlob() []byte {
	var kf []time.Duration
	for d := time.Duration(0); d < clipDuration; d += 2 * time.Second {
		kf = append(kf, d)
	}
	return library.EncodeKeyframes(kf)
}

type env struct {
	t      *testing.T
	f      *vistest.Fixture
	svc    *Service
	enc    *fakeEncoder
	adm    *fakeAdmission
	pins   *fakePins
	watch  *fakeWatch
	queue  *jobs.Queue
	clock  *clock
	dir    string
	tracks map[string]vistest.FileTracks
	devA   string // devices of the owner account
	devB   string
}

func newEnv(t *testing.T, opts ...Option) *env {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddGuest(t)
	tracks := f.AddTracks(t)
	for _, m := range f.Playable() {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), keyframeBlob())
	}
	c := &clock{now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	e := &env{t: t, f: f, enc: newFakeEncoder(), adm: &fakeAdmission{}, pins: &fakePins{f: f}, watch: &fakeWatch{p6: f.P6.ID},
		queue: jobs.NewQueue(f.DB.Write, c.Now), clock: c, dir: filepath.Join(t.TempDir(), "downloads"), tracks: tracks}
	e.devA, e.devB = e.device("a"), e.device("b")
	all := append([]Option{WithClock(c.Now), WithDir(e.dir), WithTokenKey(testKey), WithEncoder(e.enc), WithAdmission(e.adm),
		WithPins(e.pins), WithPlayback(e.watch)}, opts...)
	e.svc = New(f.DB, visibility.New(f.DB, vistest.Logger()), e.queue, transcode.Runner{}, vistest.Logger(), all...)
	return e
}

// device inserts a device row; its id repeats c 32 times.
func (e *env) device(c string) string {
	e.t.Helper()
	id := strings.Repeat(c, 32)
	e.exec(`INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1)`, id)
	return id
}

func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.f.DB.Write.ExecContext(context.Background(), query, args...); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
}

// session is a browser session of profile p on device (the account follows the profile).
func (e *env) session(p model.Profile, device string) model.Session {
	account := e.f.Account
	if p.AccountID == e.f.Guest.ID {
		account = e.f.Guest
	}
	return model.Session{ID: 1, Account: account, Profile: p, DeviceID: device}
}

// create downloads a title for p on device and returns the single state.
func (e *env) create(p model.Profile, device, itemID, quality string) model.DownloadState {
	e.t.Helper()
	list, err := e.svc.Create(context.Background(), e.session(p, device), itemID, model.DownloadRequest{Quality: quality})
	if err != nil {
		e.t.Fatalf("create %s for %s: %v", itemID, p.Name, err)
	}
	if len(list) != 1 {
		e.t.Fatalf("create %s: %d downloads", itemID, len(list))
	}
	return list[0]
}

// runJobs claims and runs runnable preparations like a worker and returns how many ran.
func (e *env) runJobs() int {
	e.t.Helper()
	ctx := context.Background()
	n := 0
	for ; n < 50; n++ {
		j, err := e.queue.Claim(ctx, "test-worker", []string{JobPrepare})
		if err != nil {
			e.t.Fatal(err)
		}
		if j == nil {
			return n
		}
		if err := e.svc.handlePrepare(ctx, j); err != nil {
			if ferr := e.queue.Fail(ctx, "test-worker", j, err); ferr != nil {
				e.t.Fatal(ferr)
			}
		} else if err := e.queue.Complete(ctx, "test-worker", j.ID); err != nil {
			e.t.Fatal(err)
		}
	}
	e.t.Fatal("jobs keep coming")
	return n
}

// row reads a download row.
func (e *env) row(id string) downloadRow {
	e.t.Helper()
	var r downloadRow
	err := e.f.DB.Read.QueryRow(`SELECT state, COALESCE(error, ''), segments_done, COALESCE(segments_total, 0), COALESCE(size_bytes, 0),
	       fetched_at, files_removed_at, allowed_profile_ids, auto FROM downloads WHERE id = ?`, id).
		Scan(&r.state, &r.err, &r.done, &r.total, &r.size, &r.fetched, &r.removed, &r.allowed, &r.auto)
	if err != nil {
		e.t.Fatalf("download %s: %v", id, err)
	}
	return r
}

type downloadRow struct {
	state, err, allowed string
	done, total, size   int64
	fetched, removed    sql.NullInt64
	auto                int64
}

// pendingJobs lists the queued preparation jobs: key and run_after.
func (e *env) pendingJobs() map[string]time.Time {
	e.t.Helper()
	rows, err := e.f.DB.Read.Query(`SELECT idempotency_key, run_after FROM jobs WHERE type = ? AND state = 'queued'`, JobPrepare)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var key string
		var at int64
		if err := rows.Scan(&key, &at); err != nil {
			e.t.Fatal(err)
		}
		out[key] = time.UnixMilli(at)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// exists reports whether the directory of a download exists.
func exists(t *testing.T, e *env, id string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(e.dir, id))
	return err == nil
}

func (e *env) file(id, name string) []byte {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, id, name))
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

// fakeEncoder stands for transcode.Runner: it delivers an init segment and per media segment n
// the bytes of segmentData(n), in the order of the plan.
type fakeEncoder struct {
	mu     sync.Mutex
	starts []int    // start segment of every Download call
	modes  []string // plan mode of every call
	// fail returns an error instead of delivering segment n (-1: never) for the next failCalls calls.
	failAt    int
	failCalls int
	// after runs after segment n was delivered (preemption, deletion); block then waits for ctx.
	after func(n int)
	block atomic.Bool
}

func newFakeEncoder() *fakeEncoder { return &fakeEncoder{failAt: -1} }

const initData = "ftyp-moov-init"

func segmentData(n int) []byte { return []byte(fmt.Sprintf("moof-mdat-segment-%02d|", n)) }

var errEncode = errors.New("fake ffmpeg failed")

func (e *fakeEncoder) Download(ctx context.Context, _ transcode.Capabilities, _ transcode.Source, plan transcode.Plan, _ string,
	start int, onInit func([]byte) error, onSegment func(int, []byte, time.Duration, time.Duration) error) error {
	e.mu.Lock()
	e.starts = append(e.starts, start)
	e.modes = append(e.modes, string(plan.Target.Mode))
	failAt := -1
	if e.failCalls > 0 {
		e.failCalls--
		failAt = e.failAt
	}
	after := e.after
	e.mu.Unlock()
	if err := onInit([]byte(initData)); err != nil {
		return fmt.Errorf("download init: %w", err)
	}
	for n := start; n < len(plan.Segments); n++ {
		if n == failAt {
			return errEncode
		}
		seg := plan.Segments[n]
		if err := onSegment(n, segmentData(n), seg.Start, seg.Duration); err != nil {
			return fmt.Errorf("download segment %d: %w", n, err)
		}
		if after != nil {
			after(n)
		}
		if e.block.Load() {
			<-ctx.Done()
			return ctx.Err()
		}
	}
	return nil
}

func (e *fakeEncoder) Keyframes(context.Context, string) ([]time.Duration, error) {
	return []time.Duration{0, 4 * time.Second, 8 * time.Second}, nil
}

func (e *fakeEncoder) Subtitles(_ context.Context, path string, index int) ([]byte, error) {
	return []byte(fmt.Sprintf("WEBVTT\n\nNOTE stream %d of %s\n", index, filepath.Base(path))), nil
}

func (e *fakeEncoder) SubtitlesFile(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return append([]byte("WEBVTT\n\n"), data...), nil
}

func (e *fakeEncoder) calls() []int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.starts)
}

// fakeAdmission hands out leases at once and records them.
type fakeAdmission struct {
	mu     sync.Mutex
	leases []*fakeLease
	costs  []float64
}

func (a *fakeAdmission) AdmissionCost(p transcode.Plan) float64 { return p.Cost + 0.25 }

func (a *fakeAdmission) AcquireDownload(_ context.Context, _ string, cost float64) (Lease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	l := &fakeLease{preempted: make(chan struct{})}
	a.leases, a.costs = append(a.leases, l), append(a.costs, cost)
	return l, nil
}

func (a *fakeAdmission) last() *fakeLease {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.leases) == 0 {
		return nil
	}
	return a.leases[len(a.leases)-1]
}

type fakeLease struct {
	preempted chan struct{}
	once      sync.Once
	released  atomic.Bool
}

func (l *fakeLease) Preempted() <-chan struct{} { return l.preempted }
func (l *fakeLease) Release()                   { l.released.Store(true) }
func (l *fakeLease) preempt()                   { l.once.Do(func() { close(l.preempted) }) }

// fakePins lists the fixture accounts' profiles through the store and gives every profile with
// a PIN a fixed salt; P16 has a separate offline PIN.
type fakePins struct {
	f     *vistest.Fixture
	mu    sync.Mutex
	asked [][]int64
}

func (p *fakePins) ListMyProfiles(ctx context.Context, accountID int64) ([]model.Profile, error) {
	rows, err := appq.New(p.f.DB.Read).ListProfilesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Profile, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.Profile{ID: r.ID, AccountID: r.AccountID, Name: r.Name, MaxRating: int(r.MaxRating), Language: r.Language,
			HasPin: r.PinHash.Valid, IsPrimary: r.IsDefault == 1, AllLibraries: r.AllLibraries == 1})
	}
	return out, nil
}

func (p *fakePins) OfflinePins(_ context.Context, _ int64, ids []int64) (map[int64]model.OfflinePin, error) {
	p.mu.Lock()
	p.asked = append(p.asked, slices.Clone(ids))
	p.mu.Unlock()
	out := map[int64]model.OfflinePin{}
	for _, id := range ids {
		pin := model.OfflinePin{HasPin: true, PinSalt: fmt.Sprintf("%032x", id)}
		if id == p.f.P16.ID {
			pin.OfflinePin = &model.PinVerifier{Salt: "c2FsdA", Iterations: model.OfflinePinIterations, Hash: "aGFzaA"}
		}
		out[id] = pin
	}
	return out, nil
}

// fakeWatch gives P6 a 60-minute limit with 45 minutes left.
type fakeWatch struct{ p6 int64 }

func (w *fakeWatch) OfflineWatchTime(_ context.Context, p model.Profile, now time.Time) (*model.OfflineWatchTime, error) {
	if p.ID != w.p6 {
		return nil, nil
	}
	return &model.OfflineWatchTime{LimitMinutes: 60, RemainingMinutes: 45, ResetsAt: now.Add(time.Hour)}, nil
}
