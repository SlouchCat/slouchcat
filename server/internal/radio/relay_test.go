package radio

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The relay tests run the real ffmpeg packager against the test station (radiotest).

var testTokenKey = []byte("radio-test-key-radio-test-key-32")

// fakeClock is a settable clock (WithClock).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// fakeWatch is the K-5 watch clock of the tests: limit 0 means no daily limit.
type fakeWatch struct {
	mu               sync.Mutex
	limit, remaining int
}

func (w *fakeWatch) WatchTime(context.Context, model.Profile, time.Time) (*model.WatchTime, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.limit == 0 {
		return nil, nil
	}
	return &model.WatchTime{LimitMinutes: w.limit, UsedMinutes: w.limit - w.remaining, RemainingMinutes: w.remaining}, nil
}

func (w *fakeWatch) set(remaining int) {
	w.mu.Lock()
	w.remaining = remaining
	w.mu.Unlock()
}

type relayEnv struct {
	t       *testing.T
	f       *vistest.Fixture
	s       *Service
	st      *radiotest.Station
	dir     string
	runner  transcode.Runner
	clock   *fakeClock // nil: wall clock
	watch   *fakeWatch
	changed []int64 // onStationChanged calls
	mu      sync.Mutex
	tokens  map[string]string // listen tokens by listen id (fetch)
}

type envOption func(*relayEnv, *[]Option)

func withFakeClock(at time.Time) envOption {
	return func(e *relayEnv, opts *[]Option) {
		e.clock = &fakeClock{t: at}
		*opts = append(*opts, WithClock(e.clock.Now))
	}
}

func testRunner() transcode.Runner {
	ff, fp := os.Getenv("SLOUCHCAT_FFMPEG"), os.Getenv("SLOUCHCAT_FFPROBE")
	if ff == "" {
		ff = "ffmpeg"
	}
	if fp == "" {
		fp = "ffprobe"
	}
	return transcode.Runner{FFmpegPath: ff, FFprobePath: fp}
}

func newRelayEnv(t *testing.T, eo ...envOption) *relayEnv {
	t.Helper()
	e := &relayEnv{t: t, f: vistest.New(t), st: startStation(t), dir: t.TempDir(), runner: testRunner(), watch: &fakeWatch{}}
	opts := []Option{WithRunner(e.runner), WithDir(e.dir), WithTokenKey(testTokenKey), WithWatchClock(e.watch)}
	for _, o := range eo {
		o(e, &opts)
	}
	e.s = withDialControl(New(e.f.DB, testLogger(), opts...), loopbackOnly)
	e.s.rel.tmpfs = func(string) bool { return true }
	e.s.rel.knobs.backoff = []time.Duration{20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond}
	reload := e.s.onStationChanged // the index reload of the wiring stays behind the recorder
	e.s.onStationChanged = func(ctx context.Context, id int64) {
		e.mu.Lock()
		e.changed = append(e.changed, id)
		e.mu.Unlock()
		reload(ctx, id)
	}
	t.Cleanup(e.s.stopRelays)
	return e
}

// exec runs SQL on the fixture database.
func (e *relayEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.f.DB.Write.Exec(query, args...); err != nil {
		e.t.Fatal(err)
	}
}

// addStation inserts a station with one stream per URL (codec and bitrate as the catalog says).
func (e *relayEnv) addStation(name, country, codec string, bitrate int, urls ...string) int64 {
	e.t.Helper()
	res, err := e.f.DB.Write.Exec(`INSERT INTO radio_stations (group_key, name, sort_key, search_text, country, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, 1)`, name+"|"+country, name, strings.ToLower(name), strings.ToLower(name), country)
	if err != nil {
		e.t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	for _, u := range urls {
		e.exec(`INSERT INTO radio_streams (station_id, url, codec, bitrate, updated_at) VALUES (?, ?, ?, ?, 1)`, id, u, codec, bitrate)
	}
	return id
}

func (e *relayEnv) listen(p model.Profile, stationID int64) (model.Session, model.RadioListen) {
	e.t.Helper()
	sess := e.f.Session(e.t, p)
	l, err := e.s.Listen(context.Background(), sess, stationID)
	if err != nil {
		e.t.Fatal(err)
	}
	return sess, l
}

func (e *relayEnv) relay(stationID int64) *relay {
	e.t.Helper()
	e.s.rel.mu.Lock()
	defer e.s.rel.mu.Unlock()
	r := e.s.rel.relays[stationID]
	if r == nil {
		e.t.Fatalf("no relay of station %d", stationID)
	}
	return r
}

// waitSegments waits until the relay has published n segments in all.
func waitSegments(t *testing.T, r *relay, n int64) {
	t.Helper()
	deadline := time.After(60 * time.Second)
	for {
		r.mu.Lock()
		have, ch := r.buf.nextSeq, r.changed
		r.mu.Unlock()
		if have >= n {
			return
		}
		select {
		case <-ch:
		case <-r.done:
			t.Fatalf("relay ended after %d of %d segments: %v", have, n, relayError(r))
		case <-deadline:
			t.Fatalf("relay has %d of %d segments after 60 s", have, n)
		}
	}
}

// waitGenSegment waits until the window holds a segment of generation gen.
func waitGenSegment(t *testing.T, r *relay, gen int) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		r.mu.Lock()
		ch := r.changed
		r.mu.Unlock()
		for _, s := range segments(r) {
			if s.gen == gen {
				return
			}
		}
		select {
		case <-ch:
		case <-r.done:
			t.Fatalf("relay ended before generation %d: %v", gen, relayError(r))
		case <-deadline:
			t.Fatalf("no segment of generation %d after 30 s", gen)
		}
	}
}

// segments returns a copy of the window.
func segments(r *relay) []segment {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]segment, len(r.buf.segs))
	for i, s := range r.buf.segs {
		out[i] = *s
	}
	return out
}

func (e *relayEnv) listenRow(id string) appq.RadioListenSession {
	e.t.Helper()
	row, err := appq.New(e.f.DB.Read).GetRadioListenSession(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

func (e *relayEnv) stationRow(id int64) appq.RadioStation {
	e.t.Helper()
	row, err := appq.New(e.f.DB.Read).GetRadioStation(context.Background(), id)
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

// grant authorizes the live routes of a listen session.
func (e *relayEnv) grant(l model.RadioListen) model.RadioGrant {
	e.t.Helper()
	g, err := e.s.AuthorizeLive(context.Background(), l.ListenID, l.Token)
	if err != nil {
		e.t.Fatal(err)
	}
	return g
}

// stream mounts: MP3 128 kbit/s and ADTS 64 kbit/s, paced like a live station after a 4-s burst
// when paced, else as fast as the relay reads.
func mp3Mount(t testing.TB, paced bool) radiotest.Mount {
	m := radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, Name: "Test FM", Bitrate: 128}
	if paced {
		m.BytesPerSecond = 16000
	}
	return m
}

func aacMount(t testing.TB) radiotest.Mount {
	return radiotest.Mount{Kind: radiotest.Icecast, ContentType: "audio/aac", Audio: [][]byte{testAudio(t, "tone-aac-64.aac")}, Bitrate: 64}
}

func TestRelayReadyAfterTwoSegments(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	start := time.Now()
	_, l := e.listen(e.f.P18, id)
	if d := time.Since(start); d > 6*time.Second {
		t.Fatalf("listen took %v, want ≤ 6 s on an Icecast burst", d)
	}
	if l.HeartbeatSeconds != 30 || l.StartOffsetSeconds != -8 || l.Station.ID != id || l.Token == "" {
		t.Fatalf("listen %+v", l)
	}
	segs := segments(e.relay(id))
	if len(segs) < 2 {
		t.Fatalf("%d segments at the listen, want ≥ 2", len(segs))
	}
	for _, s := range segs {
		if s.dur < 1900*time.Millisecond || s.dur > 2100*time.Millisecond {
			t.Fatalf("start segment lasts %v, want 2 s", s.dur)
		}
	}
}

func TestRelaySegmentsBecomeFourSeconds(t *testing.T) {
	e := newRelayEnv(t)
	e.s.rel.knobs.startPhase = time.Second
	m := mp3Mount(t, true)
	m.BytesPerSecond = 64000 // four times real time
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	waitSegments(t, r, 6)
	segs := segments(r)
	if segs[0].dur > 2100*time.Millisecond {
		t.Fatalf("first segment lasts %v, want 2 s", segs[0].dur)
	}
	last := segs[len(segs)-1]
	if last.dur < 3900*time.Millisecond || last.dur > 4200*time.Millisecond {
		t.Fatalf("segment after the start phase lasts %v, want 4 s", last.dur)
	}
	if bytes.Count(r.playlist("x", false), []byte("#EXT-X-DISCONTINUITY\n")) != 0 {
		t.Fatal("the change to 4-s segments added a discontinuity")
	}
}

func TestRelayOneConnectionManyListeners(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	for _, p := range []model.Profile{e.f.P18, e.f.P12, e.f.P6} {
		e.listen(p, id)
	}
	if n := e.st.Connections("/live"); n != 1 {
		t.Fatalf("%d connections to the station, want 1", n)
	}
	rel, err := e.s.Relays(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rel.Relays) != 1 || len(rel.Relays[0].Listeners) != 3 || !rel.Relays[0].Transcoded || rel.Relays[0].BitrateKbps != 128 ||
		rel.Load.Relays != 1 || rel.Load.BufferMode != model.RadioBufferTmpfs || rel.Load.UploadKbps != 3*128 {
		t.Fatalf("relays %+v", rel)
	}
}

func TestRelayReconnectSameCodecKeepsPackager(t *testing.T) {
	e := newRelayEnv(t)
	e.s.rel.knobs.backoff = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second}
	m := mp3Mount(t, false)
	m.CutAfter = []int{150_001}
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	waitSegments(t, r, 20)
	if n := e.st.Connections("/live"); n != 2 {
		t.Fatalf("%d connections, want 2 (one reconnect)", n)
	}
	r.mu.Lock()
	gen := r.gen
	r.mu.Unlock()
	if gen != 0 || bytes.Contains(r.playlist("x", false), []byte("DISCONTINUITY\n")) {
		t.Fatalf("generation %d after a reconnect with the same codec, want 0 and no discontinuity", gen)
	}
}

// joined returns the init of generation 0 and every segment of the window as one file.
func joined(t *testing.T, r *relay) string {
	t.Helper()
	init, err := r.initData(0)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.Clone(init)
	for _, s := range segments(r) {
		if s.gen != 0 {
			t.Fatalf("segment %d of generation %d", s.seq, s.gen)
		}
		data, _, err := r.segmentData(s.seq)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, data...)
	}
	path := filepath.Join(t.TempDir(), "joined.mp4")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRelayReconnectMidFrameDecodesClean(t *testing.T) {
	for name, m := range map[string]radiotest.Mount{"mp3-transcode": mp3Mount(t, false), "adts-copy": aacMount(t)} {
		t.Run(name, func(t *testing.T) {
			e := newRelayEnv(t)
			m.CutAfter = []int{60_001, 33_333, 47_777} // in the middle of frames
			e.st.Set("/live", m)
			id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
			e.listen(e.f.P18, id)
			r := e.relay(id)
			for e.st.Connections("/live") < 4 {
				waitSegments(t, r, segments(r)[len(segments(r))-1].seq+2)
			}
			waitSegments(t, r, segments(r)[len(segments(r))-1].seq+3)
			if err := e.runner.CheckRadioAudio(context.Background(), joined(t, r)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// hlsPacked sets an HLS media mount of packed ADTS segments (4 parts of the AAC tone).
func hlsPacked(t *testing.T, st *radiotest.Station, path string, discAt int) [][]byte {
	t.Helper()
	parts, err := radiotest.SplitADTS(testAudio(t, "tone-aac-64.aac"), 4)
	if err != nil {
		t.Fatal(err)
	}
	var segs []radiotest.Segment
	for i, p := range parts {
		segs = append(segs, radiotest.Segment{Data: p, Discontinuity: i == discAt})
	}
	st.Set(path, radiotest.Mount{Kind: radiotest.HLSMedia})
	st.SetHLS(path, 0, 4, 6, segs...)
	return parts
}

func TestRelayCopyModeDiscontinuityNewGeneration(t *testing.T) {
	e := newRelayEnv(t)
	parts := hlsPacked(t, e.st, "/hls", -1)
	id := e.addStation("HLS FM", "CH", "AAC", 64, e.st.URL("/hls"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	r.mu.Lock()
	copyMode := r.out.Copy
	r.mu.Unlock()
	if !copyMode {
		t.Fatal("AAC is not copied")
	}
	e.st.AppendHLS("/hls", radiotest.Segment{Data: parts[0], Discontinuity: true}, radiotest.Segment{Data: parts[1]},
		radiotest.Segment{Data: parts[2]})
	waitGenSegment(t, r, 1)
	pl := string(r.playlist("x", false))
	if !strings.Contains(pl, "#EXT-X-DISCONTINUITY\n") || !strings.Contains(pl, `init/1?rt=x`) {
		t.Fatalf("playlist without the new generation:\n%s", pl)
	}
}

func TestRelayOggReconnectNewGeneration(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/ogg", radiotest.Mount{ContentType: "audio/ogg", Audio: [][]byte{testAudio(t, "tone-ogg.ogg")}, CutAfter: []int{30_000}})
	id := e.addStation("Ogg FM", "CH", "OGG", 112, e.st.URL("/ogg"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	waitGenSegment(t, r, 1) // Ogg is not framed by Go: a reconnect always starts g+1
	if n := e.st.Connections("/ogg"); n != 2 {
		t.Fatalf("%d connections, want 2", n)
	}
}

func TestRelayCodecChangeAddsDiscontinuity(t *testing.T) {
	e := newRelayEnv(t)
	m := mp3Mount(t, false)
	m.Audio = append(m.Audio, testAudio(t, "tone-aac-64.aac"))
	m.CutAfter = []int{80_000}
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	waitGenSegment(t, r, 1)
	r.mu.Lock()
	gen, out := r.gen, r.out
	r.mu.Unlock()
	if gen != 1 || !out.Copy {
		t.Fatalf("generation %d copy %v after the change to AAC, want 1 and copy", gen, out.Copy)
	}
	pl := string(r.playlist("x", false))
	if strings.Count(pl, "#EXT-X-MAP:") != 2 || strings.Count(pl, "#EXT-X-PROGRAM-DATE-TIME:") != 2 ||
		strings.Count(pl, "#EXT-X-DISCONTINUITY\n") != 1 {
		t.Fatalf("playlist:\n%s", pl)
	}
	if _, err := r.initData(1); err != nil {
		t.Fatal(err)
	}
}

func TestRelayEndsAfterFiveFailures(t *testing.T) {
	e := newRelayEnv(t)
	m := mp3Mount(t, true)
	m.CutAfter = []int{100_000} // about 2 s after the burst
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	r := e.relay(id)
	e.st.Set("/live", radiotest.Mount{Kind: radiotest.Icecast, Down: true}) // every reconnect fails
	select {
	case <-r.done:
	case <-time.After(30 * time.Second):
		t.Fatal("the relay did not end")
	}
	if n := e.st.Connections("/live"); n != 6 {
		t.Fatalf("%d connections, want 1 and 5 attempts", n)
	}
	if row := e.listenRow(l.ListenID); row.EndedBy.String != model.RadioEndedStation {
		t.Fatalf("listen session ended by %q, want station", row.EndedBy.String)
	}
	if _, err := e.s.Heartbeat(context.Background(), sess, l.ListenID, true); !errors.Is(err, model.ErrRadioUnreachable) {
		t.Fatalf("heartbeat after the relay ended: %v", err)
	}
	if _, err := e.s.AuthorizeLive(context.Background(), l.ListenID, l.Token); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("live request after the relay ended: %v", err)
	}
	if st := e.stationRow(id); st.Failures != 1 || st.BrokenAt.Valid {
		t.Fatalf("station failures %d broken %v, want 1 and not broken", st.Failures, st.BrokenAt.Valid)
	}
}

func TestRelayBrokenAfterThreeAborts(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/down", radiotest.Mount{Kind: radiotest.Icecast, Down: true})
	id := e.addStation("Down FM", "CH", "MP3", 128, e.st.URL("/down"))
	sess := e.f.Session(t, e.f.P18)
	for i := 1; i <= 3; i++ {
		if _, err := e.s.Listen(context.Background(), sess, id); !errors.Is(err, model.ErrRadioUnreachable) {
			t.Fatalf("listen %d: %v, want ErrRadioUnreachable", i, err)
		}
		if st := e.stationRow(id); st.Failures != int64(i) || st.BrokenAt.Valid != (i == 3) {
			t.Fatalf("after abort %d: failures %d broken %v", i, st.Failures, st.BrokenAt.Valid)
		}
	}
	// A station that plays again is no longer broken.
	e.st.Set("/down", mp3Mount(t, false))
	e.listen(e.f.P18, id)
	if st := e.stationRow(id); st.BrokenAt.Valid {
		t.Fatal("broken_at stays after a successful start")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.changed) < 4 {
		t.Fatalf("onStationChanged called %d times, want after every abort and the recovery", len(e.changed))
	}
}

func TestRelayGraceStopsAndFrees(t *testing.T) {
	e := newRelayEnv(t, withFakeClock(time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)))
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	r := e.relay(id)
	ctx := context.Background()
	if err := e.s.EndListen(ctx, sess, l.ListenID); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(119 * time.Second)
	e.s.sweep(ctx)
	if e.relay(id) != r {
		t.Fatal("relay stopped before the grace time")
	}
	e.clock.Add(2 * time.Second)
	e.s.sweep(ctx)
	select {
	case <-r.done:
	default:
		t.Fatal("relay still running after the grace time")
	}
	e.s.rel.mu.Lock()
	n := len(e.s.rel.relays)
	e.s.rel.mu.Unlock()
	if n != 0 {
		t.Fatal("the relay keeps its reservation")
	}
	if _, err := os.Stat(filepath.Join(e.dir, "1")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("station directory left: %v", err)
	}
	if row := e.listenRow(l.ListenID); row.EndedBy.String != model.StoppedByClient {
		t.Fatalf("ended by %q", row.EndedBy.String)
	}
}

func TestBudgetRefusesRelay(t *testing.T) {
	e := newRelayEnv(t)
	e.exec(`UPDATE radio_config SET budget_mb = 64, buffer_minutes = 60`)
	e.st.Set("/a", mp3Mount(t, true))
	e.st.Set("/b", mp3Mount(t, true))
	a := e.addStation("A FM", "CH", "MP3", 128, e.st.URL("/a"))
	b := e.addStation("B FM", "CH", "MP3", 128, e.st.URL("/b"))
	e.listen(e.f.P18, a) // 128 kbit/s × 3600 s × 1.03 ≈ 59 MB
	_, err := e.s.Listen(context.Background(), e.f.Session(t, e.f.P18), b)
	var ce *model.RadioCapacityError
	if !errors.As(err, &ce) || !errors.Is(err, model.ErrRadioCapacity) || ce.Running != 1 || ce.Max != 10 {
		t.Fatalf("second relay over the budget: %v", err)
	}
	if e.st.Connections("/b") != 0 {
		t.Fatal("a refused relay connected")
	}
}

func TestMaxRelays(t *testing.T) {
	e := newRelayEnv(t)
	e.exec(`UPDATE radio_config SET max_relays = 1`)
	e.st.Set("/a", mp3Mount(t, true))
	e.st.Set("/b", mp3Mount(t, true))
	a := e.addStation("A FM", "CH", "MP3", 128, e.st.URL("/a"))
	b := e.addStation("B FM", "CH", "MP3", 128, e.st.URL("/b"))
	e.listen(e.f.P18, a)
	e.listen(e.f.P12, a) // joining a running relay needs no capacity
	_, err := e.s.Listen(context.Background(), e.f.Session(t, e.f.P18), b)
	var ce *model.RadioCapacityError
	if !errors.As(err, &ce) || ce.Running != 1 || ce.Max != 1 {
		t.Fatalf("second relay over max_relays: %v", err)
	}
}

func TestMemoryModeWithoutTmpfs(t *testing.T) {
	e := newRelayEnv(t)
	e.s.rel.tmpfs = func(string) bool { return false }
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	waitSegments(t, r, 20)
	if n := len(segments(r)); n > memorySegments {
		t.Fatalf("%d segments in memory mode, want ≤ %d", n, memorySegments)
	}
	if e.s.bufferMode() != model.RadioBufferMemory {
		t.Fatal(e.s.bufferMode())
	}
	err := filepath.WalkDir(e.dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && path != e.dir {
			t.Errorf("written in memory mode: %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.segmentData(segments(r)[0].seq); err != nil {
		t.Fatal(err)
	}
}

func TestNowEntriesFromICY(t *testing.T) {
	e := newRelayEnv(t)
	m := mp3Mount(t, false)
	m.MetaInt = 8192
	m.Titles = []radiotest.Title{{At: 0, Text: "Test FM"}, {At: 40_000, Text: "Künstler - Lied"}, {At: 120_000, Text: "Band - Song"}}
	m.Once = true
	e.st.Set("/live", m)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, _ := e.listen(e.f.P18, id)
	deadline := time.Now().Add(20 * time.Second)
	var now model.RadioNow
	for {
		var err error
		if now, err = e.s.Now(context.Background(), sess, id, time.Time{}); err != nil {
			t.Fatal(err)
		}
		if len(now.Entries) >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !now.Live || len(now.Entries) != 2 || now.Entries[0].Artist != "Künstler" || now.Entries[0].Title != "Lied" ||
		now.Entries[1].Text != "Band - Song" || now.Entries[1].At.Before(now.Entries[0].At) {
		t.Fatalf("now %+v (the station name is no title)", now)
	}
	later, err := e.s.Now(context.Background(), sess, id, now.Entries[1].At)
	if err != nil || len(later.Entries) != 1 || later.Entries[0].Text != "Band - Song" {
		t.Fatalf("since the last entry: %+v %v", later, err)
	}
	if e.stationRow(id).ShowsTitles != 1 {
		t.Fatal("shows_titles not set")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.changed) != 1 || e.changed[0] != id {
		t.Fatalf("onStationChanged calls %v, want one for the station", e.changed)
	}
}

func TestRadioIgnoresStreamLimits(t *testing.T) {
	e := newRelayEnv(t)
	one := e.f.AddLimitedProfile(t, "One", model.PlaybackLimits{MaxStreams: 1})
	pb := playback.New(e.f.DB, testLogger(), playback.WithTokenKey(testTokenKey))
	if _, err := pb.Start(context.Background(), e.f.Session(t, one), e.f.Rated(0).ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pb.Start(context.Background(), e.f.Session(t, one), e.f.Rated(6).ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrStreamLimit) {
		t.Fatalf("second playback of the profile: %v, want the stream limit", err)
	}
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(one, id)
	e.listen(one, id) // a second listen of the same profile as well (F-19)
}
