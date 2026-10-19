package transcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// segmentScript is a fake ffmpeg that writes the segments from -segment_start_number to total-1
// the way the segment muxer lays them out (first file of a run ftyp+moov+moof+mdat, then
// moof+mdat), one every interval, and lists each after its data.
func segmentScript(total int, interval string) string {
	return fmt.Sprintf(`exec 2>/dev/null
start=0; prev=; last=
for a in "$@"; do [ "$prev" = "-segment_start_number" ] && start=$a; prev=$a; last=$a; done
dir=$(dirname "$last")
n=$start; first=1
while [ $n -lt %d ]; do
  f=$(printf '%%s/seg%%05d.m4s' "$dir" $n)
  if [ $first = 1 ]; then printf '\0\0\0\010ftyp\0\0\0\010moov' > "$f"; first=0; else : > "$f"; fi
  printf '\0\0\0\010moof\0\0\0\011mdatX' >> "$f"
  printf 'seg%%05d.m4s\n' $n >> "$dir/list.txt"
  n=$((n+1)); sleep %s
done
`, total, interval)
}

var (
	fakeInit  = slices.Concat(mkBox("ftyp"), mkBox("moov"))
	fakeMedia = slices.Concat(mkBox("moof"), mkBox("mdat", 'X'))
)

func publishedSegments(s *Session) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var idx []int
	for k, d := range s.done {
		if d {
			idx = append(idx, k)
		}
	}
	return idx
}

func currentRun(t *testing.T, s *Session) *run {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run == nil {
		t.Fatal("no ffmpeg runs")
	}
	return s.run
}

func TestCheckComplete(t *testing.T) {
	dir := t.TempDir()
	full := slices.Concat(fakeInit, fakeMedia)
	large := append(binary64Box("mdat", 3), 1, 2, 3)
	cases := []struct {
		name       string
		data       []byte // nil: file missing
		incomplete bool   // errIncomplete (retry) rather than a hard error
		ok         bool
	}{
		{"missing", nil, true, false},
		{"empty", []byte{}, true, false},
		{"partial header", full[:5], true, false},
		{"ftyp only", fakeInit[:8], true, false},
		{"init without media", fakeInit, true, false},
		{"partial mdat", full[:len(full)-1], true, false},
		{"moof without mdat", mkBox("moof"), true, false},
		{"first file", full, false, true},
		{"later file", fakeMedia, false, true},
		{"two fragments", slices.Concat(fakeMedia, fakeMedia), false, true},
		{"large mdat", slices.Concat(mkBox("moof"), large), false, true},
		{"size zero", slices.Concat(mkBox("moof"), []byte{0, 0, 0, 0, 'm', 'd', 'a', 't', 1}), true, false},
		{"garbage", mkBox("free"), false, false},
		{"ftyp then moof", slices.Concat(mkBox("ftyp"), fakeMedia), false, false},
		{"bad size", []byte{0, 0, 0, 4, 'm', 'o', 'o', 'f'}, false, false},
	}
	for i, c := range cases {
		path := filepath.Join(dir, fmt.Sprintf("seg%05d.m4s", i))
		if c.data != nil {
			if err := os.WriteFile(path, c.data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		err := checkComplete(path)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: %v, want complete", c.name, err)
		case !c.ok && err == nil:
			t.Errorf("%s: complete, want an error", c.name)
		case !c.ok && errors.Is(err, errIncomplete) != c.incomplete:
			t.Errorf("%s: %v, want incomplete=%v", c.name, err, c.incomplete)
		}
	}
}

// binary64Box returns the header of a box with a 64-bit size and n payload bytes to follow.
func binary64Box(typ string, n int) []byte {
	b := []byte{0, 0, 0, 1}
	b = append(b, typ...)
	size := uint64(16 + n)
	for i := 7; i >= 0; i-- {
		b = append(b, byte(size>>(8*i)))
	}
	return b
}

// ffmpeg 6.1.1 lists a segment before it writes its data: a listed 0-byte or partial file is not
// published, does not stop the run, and is published once the data is complete.
func TestSessionWaitsForListedSegmentData(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 0, time.Minute)
	m.incompleteWait = time.Second
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "partial", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	r := currentRun(t, s)
	seg := filepath.Join(r.dir, "seg00000.m4s")
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := make(chan []byte, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rc, err := s.OpenSegment(ctx, 0)
		if err != nil {
			t.Error(err)
			got <- nil
			return
		}
		defer rc.Close()
		b := make([]byte, 64)
		n, _ := rc.Read(b)
		got <- b[:n]
	}()

	full := slices.Concat(fakeInit, fakeMedia)
	write(seg, nil)
	write(filepath.Join(r.dir, segmentList), []byte("seg00000.m4s\n"))
	time.Sleep(4 * pollInterval)
	write(seg, full[:len(fakeInit)+10]) // header and part of the moof
	time.Sleep(4 * pollInterval)
	if p := publishedSegments(s); len(p) != 0 || !s.isRunning() {
		t.Fatalf("published %v, running %v before the data was complete", p, s.isRunning())
	}
	write(seg, full)
	if b := <-got; !slices.Equal(b, fakeMedia) {
		t.Errorf("segment 0 = %v, want %v", b, fakeMedia)
	}
	rc, err := s.OpenInit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 64)
	n, _ := rc.Read(b)
	_ = rc.Close()
	if !slices.Equal(b[:n], fakeInit) {
		t.Errorf("init = %v, want %v", b[:n], fakeInit)
	}
	if !s.isRunning() {
		t.Error("ffmpeg stopped")
	}

	// A listed segment that stays incomplete fails the run after incompleteWait.
	write(filepath.Join(r.dir, "seg00001.m4s"), fakeMedia[:9])
	write(filepath.Join(r.dir, segmentList), []byte("seg00000.m4s\nseg00001.m4s\n"))
	waitFor(t, "run stopped", func() bool { return !s.isRunning() })
	s.mu.Lock()
	err = s.err
	s.mu.Unlock()
	if !errors.Is(err, errIncomplete) {
		t.Errorf("session error %v, want incomplete segment", err)
	}
}

func (s *Session) isRunning() bool {
	running, _ := s.state()
	return running
}

// MaxAhead pauses ffmpeg once it is far enough ahead of the player, keeps the budget and the
// files, and a request for the next missing segment resumes there.
func TestSessionPausesAheadAndResumes(t *testing.T) {
	m, err := NewManager(ManagerOptions{
		Runner: Runner{FFmpegPath: fakeBinary(t, segmentScript(30, "0.1"))}, WorkDir: filepath.Join(t.TempDir(), "transcode"),
		Backend: BackendSoftware, Budget: 4, MaxAhead: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	src := videoSource("hevc", 1920, 1080, "", sec(120))
	s, err := m.Start(context.Background(), "ahead", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "pause", func() bool { return !s.isRunning() })
	if p := publishedSegments(s); !slices.Equal(p, []int{0, 1, 2, 3}) {
		t.Errorf("published %v before the pause, want 0..3 (MaxAhead 2 beyond segment 0)", p)
	}
	st := m.Stats()
	if st.Used != 2 || st.Running != 0 || st.DiskBytes <= 0 {
		t.Errorf("paused: %+v, want the budget kept and files on disk", st)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	read := reader(t)
	if b := read(s.OpenSegment(ctx, 2)); !slices.Equal(b, fakeMedia) {
		t.Errorf("segment 2 = %v", b)
	}
	if runCount(s) != 1 {
		t.Error("a segment on disk restarted ffmpeg")
	}
	if b := read(s.OpenSegment(ctx, 4)); !slices.Equal(b, fakeMedia) {
		t.Errorf("segment 4 = %v", b)
	}
	waitFor(t, "second pause", func() bool { return !s.isRunning() })
	if p := publishedSegments(s); !slices.Equal(p, []int{0, 1, 2, 3, 4, 5, 6, 7}) {
		t.Errorf("published %v, want 0..7", p)
	}
	if runCount(s) != 2 {
		t.Errorf("%d runs, want 2", runCount(s))
	}
}

// RemoveAfterIdle removes the files and releases the budget; the session stays and restarts at
// the requested segment.
func TestSessionTurnsColdAndRestarts(t *testing.T) {
	m, err := NewManager(ManagerOptions{
		Runner: Runner{FFmpegPath: fakeBinary(t, segmentScript(30, "0.02"))}, WorkDir: filepath.Join(t.TempDir(), "transcode"),
		Backend: BackendSoftware, Budget: 4, MaxAhead: 2, IdleTimeout: 100 * time.Millisecond, RemoveAfterIdle: 300 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	src := videoSource("hevc", 1920, 1080, "", sec(120))
	s, err := m.Start(context.Background(), "cold", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "cold", func() bool { return m.Stats().Cold == 1 })
	if st := m.Stats(); st.Used != 0 || st.Running != 0 || st.Sessions != 1 || st.DiskBytes != 0 {
		t.Errorf("cold: %+v, want no budget, no files, still registered", st)
	}
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Errorf("session dir after RemoveAfterIdle: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	read := reader(t)
	if b := read(s.OpenSegment(ctx, 10)); !slices.Equal(b, fakeMedia) {
		t.Errorf("segment 10 = %v", b)
	}
	if b := read(s.OpenInit(ctx)); !slices.Equal(b, fakeInit) {
		t.Errorf("init = %v", b)
	}
	s.mu.Lock()
	start := s.runs
	s.mu.Unlock()
	if p := publishedSegments(s); len(p) == 0 || p[0] != 10 || start != 2 {
		t.Errorf("after the cold restart: published %v, %d runs; want a run from segment 10", p, start)
	}
	if st := m.Stats(); st.Cold != 0 || st.Sessions != 1 {
		t.Errorf("after restart: %+v", st)
	}
}

// Start begins at the requested segment; OpenInit waits for the first file of that run.
func TestStartAtSegment(t *testing.T) {
	m := newTestManager(t, segmentScript(30, "0.02"), 4, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(120))
	p := testPlan(t, src)
	if _, err := m.Start(context.Background(), "bad", src, p, len(p.Segments)); !errors.Is(err, ErrNoSegment) {
		t.Errorf("start beyond the plan: %v", err)
	}
	s, err := m.Start(context.Background(), "resume", src, p, 12)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	read := reader(t)
	if b := read(s.OpenInit(ctx)); !slices.Equal(b, fakeInit) {
		t.Errorf("init = %v", b)
	}
	if b := read(s.OpenSegment(ctx, 12)); !slices.Equal(b, fakeMedia) {
		t.Errorf("segment 12 = %v", b)
	}
	if p := publishedSegments(s); p[0] != 12 || runCount(s) != 1 {
		t.Errorf("published %v in %d runs, want one run from 12", p, runCount(s))
	}
}

// A Stop or Close between registering and starting ffmpeg must not leave an ffmpeg behind.
func TestStartRacesWithStop(t *testing.T) {
	for _, stop := range []string{"Stop", "Close"} {
		t.Run(stop, func(t *testing.T) {
			m := newTestManager(t, "exec sleep 30\n", 4, time.Minute)
			src := videoSource("hevc", 1920, 1080, "", sec(20))
			m.afterRegister = func() {
				if stop == "Stop" {
					m.Stop("race")
				} else {
					m.Close()
				}
			}
			s, err := m.Start(context.Background(), "race", src, testPlan(t, src), 0)
			if !errors.Is(err, ErrClosed) || s != nil {
				t.Fatalf("Start: %v, want ErrClosed", err)
			}
			if st := m.Stats(); st.Used != 0 || st.Sessions != 0 || st.Running != 0 {
				t.Errorf("after the race: %+v", st)
			}
			if entries, err := os.ReadDir(m.dir); err == nil && len(entries) != 0 {
				t.Errorf("work dir not empty: %v", entries)
			}
		})
	}
}

// A request waiting for its run backs off with ErrRestarted when a newer request restarts
// ffmpeg far away, instead of restarting ffmpeg again (two readers killing each other's runs).
func TestNewestRequestWins(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 4, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(120))
	s, err := m.Start(context.Background(), "two", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	older := make(chan error, 1)
	go func() {
		_, err := s.OpenSegment(ctx, 2) // within lookahead of the run at 0: waits
		older <- err
	}()
	time.Sleep(100 * time.Millisecond)
	newer := make(chan error, 1)
	nctx, ncancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer ncancel()
	go func() {
		_, err := s.OpenSegment(nctx, 20) // restarts at 20
		newer <- err
	}()
	if err := <-older; !errors.Is(err, ErrRestarted) {
		t.Errorf("older request: %v, want ErrRestarted", err)
	}
	if err := <-newer; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("newer request: %v, want it to keep waiting for its run", err)
	}
	if n := runCount(s); n != 2 {
		t.Errorf("%d runs, want 2 (start, restart at 20)", n)
	}
	if r := currentRun(t, s); r.start != 20 {
		t.Errorf("run from %d, want 20", r.start)
	}
}

// After maxFailures failed runs from the same segment, requests get the error without a start.
func TestFailuresPerSegmentAreBounded(t *testing.T) {
	m := newTestManager(t, "echo 'broken' >&2\nexit 1\n", 0, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "fails", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range maxFailures {
		if _, err := s.OpenSegment(ctx, 2); err == nil {
			t.Fatal("want the ffmpeg error")
		}
	}
	runs := runCount(s)
	if _, err := s.OpenSegment(ctx, 2); err == nil || runCount(s) != runs {
		t.Errorf("request after %d failures: %v, %d runs (was %d); want the error without a start", maxFailures, err, runCount(s), runs)
	}
	// Another segment still gets its own attempts.
	if _, err := s.OpenSegment(ctx, 3); err == nil || runCount(s) != runs+1 {
		t.Errorf("segment 3: %v, %d runs", err, runCount(s))
	}
}

// A file that vanished because the session stopped is ErrClosed, otherwise ErrNoSegment.
func TestOpenVanishedFile(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 0, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "gone", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.open("seg00001.m4s"); !errors.Is(err, ErrNoSegment) {
		t.Errorf("missing file of a live session: %v", err)
	}
	m.Stop("gone")
	if _, err := s.open("seg00001.m4s"); !errors.Is(err, ErrClosed) {
		t.Errorf("missing file of a stopped session: %v", err)
	}
}

// Waiting Starts are admitted in arrival order: a cheap session does not overtake an expensive
// one that waits for budget.
func TestAdmissionIsFIFO(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 3, time.Minute)
	m.admitWait = 5 * time.Second
	video := videoSource("hevc", 1920, 1080, "", sec(20))
	audio := Source{Path: "/m/a.mka", Probe: Probe{Format: Format{Duration: sec(20)}, Streams: []Stream{{Type: "audio", Codec: "aac"}}}}
	vp, ap := testPlan(t, video), testPlan(t, audio) // cost 2 and 0.25
	ctx := context.Background()
	if _, err := m.Start(ctx, "A", video, vp, 0); err != nil {
		t.Fatal(err)
	}
	order := make(chan string, 2)
	start := func(id string, src Source, p Plan) {
		_, err := m.Start(ctx, id, src, p, 0)
		order <- fmt.Sprint(id, err)
	}
	go start("B", video, vp)
	time.Sleep(100 * time.Millisecond)
	go start("C", audio, ap)
	time.Sleep(200 * time.Millisecond)
	select {
	case who := <-order:
		t.Fatalf("%s admitted while B waits at the head of the queue", who)
	default:
	}
	m.Stop("A")
	got := []string{<-order, <-order}
	slices.Sort(got)
	if !slices.Equal(got, []string{"B<nil>", "C<nil>"}) {
		t.Errorf("after the release: %v, want B and C admitted", got)
	}
}
