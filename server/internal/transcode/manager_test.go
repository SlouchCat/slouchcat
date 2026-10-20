package transcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newTestManager returns a manager whose "ffmpeg" is the given script.
func newTestManager(t *testing.T, script string, budget float64, idle time.Duration) *Manager {
	t.Helper()
	m, err := NewManager(ManagerOptions{
		Runner: Runner{FFmpegPath: fakeBinary(t, script)}, WorkDir: filepath.Join(t.TempDir(), "transcode"),
		Backend: BackendSoftware, Budget: budget, IdleTimeout: idle,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.admitWait = 300 * time.Millisecond
	t.Cleanup(m.Close)
	return m
}

func testPlan(t *testing.T, src Source) Plan {
	t.Helper()
	p, err := NewPlan(src, Target{Mode: ModeTranscode})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerAdmission(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 3, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	p := testPlan(t, src) // cost 2
	ctx := context.Background()

	if _, err := m.Start(ctx, "A", src, p, 0); err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(); st.Used != 2 || st.Running != 1 || st.Sessions != 1 || st.Budget != 3 || st.Backend != BackendSoftware {
		t.Errorf("stats %+v", st)
	}
	start := time.Now()
	if _, err := m.Start(ctx, "B", src, p, 0); !errors.Is(err, ErrBusy) {
		t.Fatalf("second session: %v, want ErrBusy", err)
	}
	if d := time.Since(start); d < 250*time.Millisecond {
		t.Errorf("ErrBusy after %v, want after the admission wait", d)
	}
	if _, err := m.Start(ctx, "A", src, p, 0); err == nil {
		t.Error("want error for a duplicate id")
	}
	if _, err := m.Start(ctx, "../evil", src, p, 0); err == nil {
		t.Error("want error for an unsafe id")
	}

	// A waiting Start is admitted as soon as budget is released.
	m.admitWait = 5 * time.Second
	admitted := make(chan error, 1)
	go func() {
		_, err := m.Start(ctx, "C", src, p, 0)
		admitted <- err
	}()
	time.Sleep(100 * time.Millisecond)
	m.Stop("A")
	if err := <-admitted; err != nil {
		t.Fatalf("waiting session: %v", err)
	}
	if _, ok := m.Get("A"); ok {
		t.Error("stopped session still registered")
	}
	if s, ok := m.Get("C"); !ok || s.Mode() != ModeTranscode || s.Backend() != BackendSoftware {
		t.Error("session C not registered")
	}

	// A canceled context ends the wait.
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := m.Start(cctx, "D", src, p, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("canceled wait: %v", err)
	}
}

func TestManagerAdmitsOversizedSessionAlone(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 1, time.Minute)
	src := videoSource("hevc", 3840, 2160, "smpte2084", sec(20))
	p, err := NewPlan(src, Target{Mode: ModeTranscode, Tonemap: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), "big", src, p, 0); err != nil {
		t.Fatalf("cost %v alone on budget 1: %v", p.Cost, err)
	}
}

func TestManagerIdleStopAndResume(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 0, 100*time.Millisecond)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "idle", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(); st.Budget != DefaultBudget(BackendSoftware) {
		t.Errorf("default budget %v", st.Budget)
	}

	// ffmpeg runs with nice 10 and oom_score_adj 500.
	s.mu.Lock()
	pid := s.run.cmd.Process.Pid
	s.mu.Unlock()
	if adj, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/oom_score_adj"); err == nil && strings.TrimSpace(string(adj)) != "500" {
		t.Errorf("oom_score_adj %q, want 500", adj)
	}
	if stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
		if len(fields) > 16 && fields[16] != "10" {
			t.Errorf("nice %s, want 10", fields[16])
		}
	}

	waitFor(t, "idle stop", func() bool { st := m.Stats(); return st.Running == 0 && st.Used == 0 })
	if _, err := os.Stat(s.dir); err != nil {
		t.Errorf("session dir removed by the idle stop: %v", err)
	}

	// A request resumes ffmpeg (it never delivers here) and takes the budget again.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.OpenSegment(ctx, 3); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("OpenSegment: %v", err)
	}
	if st := m.Stats(); st.Running != 1 || st.Used != 2 {
		t.Errorf("after resume: %+v", st)
	}
	if _, err := s.OpenSegment(context.Background(), 99); !errors.Is(err, ErrNoSegment) {
		t.Errorf("segment out of range: %v", err)
	}

	m.Stop("idle")
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Errorf("session dir after Stop: %v", err)
	}
	if st := m.Stats(); st.Used != 0 || st.Sessions != 0 {
		t.Errorf("after Stop: %+v", st)
	}
	if _, err := s.OpenInit(context.Background()); !errors.Is(err, ErrClosed) {
		t.Errorf("OpenInit after Stop: %v", err)
	}
}

func TestSessionReportsFFmpegFailure(t *testing.T) {
	m := newTestManager(t, "echo 'Invalid data found when processing input' >&2\nexit 1\n", 0, time.Minute)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "fail", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.OpenInit(ctx); err == nil || !strings.Contains(err.Error(), "Invalid data found") {
		t.Fatalf("OpenInit: %v, want the ffmpeg error", err)
	}
	if _, err := s.OpenSegment(ctx, 2); err == nil || !strings.Contains(err.Error(), "Invalid data found") {
		t.Fatalf("OpenSegment: %v, want the ffmpeg error", err)
	}
	waitFor(t, "budget release", func() bool { return m.Stats().Used == 0 })
	log, err := os.ReadFile(filepath.Join(s.dir, logName))
	if err != nil || !strings.Contains(string(log), "--- run 1 from segment 0") {
		t.Errorf("ffmpeg.log: %q, %v", log, err)
	}
}

func TestNewManagerWipesWorkDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "transcode")
	if err := os.MkdirAll(filepath.Join(dir, "old"), 0o750); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(ManagerOptions{Runner: Runner{FFmpegPath: "ffmpeg"}, WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Errorf("old session dir survived: %v", err)
	}
	for _, bad := range []string{"", "relative/dir", "/"} {
		if _, err := NewManager(ManagerOptions{Runner: Runner{FFmpegPath: "ffmpeg"}, WorkDir: bad}); err == nil {
			t.Errorf("work dir %q: want error", bad)
		}
	}
	if _, err := NewManager(ManagerOptions{WorkDir: dir}); err == nil {
		t.Error("want error without ffmpeg path")
	}
	m.Close()
	if _, err := m.Start(context.Background(), "x", Source{}, Plan{Segments: []Segment{{0, sec(4)}}}, 0); !errors.Is(err, ErrClosed) {
		t.Errorf("Start after Close: %v", err)
	}
}

// An HDR transcode whose tonemapping runs on the CPU costs costHDRCPU on the manager; a GPU
// tonemapper and a remux keep the plan's cost.
func TestManagerCostOfCPUTonemap(t *testing.T) {
	hdr := Plan{Cost: cost4KHDR, Target: Target{Mode: ModeTranscode, Tonemap: true}}
	remux := Plan{Cost: costRemux, Target: Target{Mode: ModeRemux, VideoCopy: true, Tonemap: true}}
	sdr := Plan{Cost: costHD, Target: Target{Mode: ModeTranscode}}
	cpu := &Manager{caps: Capabilities{Backend: BackendQSV, Tonemapper: TonemapCPU}}
	gpu := &Manager{caps: Capabilities{Backend: BackendQSV, Tonemapper: TonemapVPP, HWTonemap: true}}
	if got := cpu.cost(hdr); got != costHDRCPU {
		t.Errorf("cpu tonemap: cost %v, want %v", got, costHDRCPU)
	}
	if got := gpu.cost(hdr); got != cost4KHDR {
		t.Errorf("gpu tonemap: cost %v, want %v", got, cost4KHDR)
	}
	if got := cpu.cost(remux); got != costRemux {
		t.Errorf("remux: cost %v, want %v", got, costRemux)
	}
	if got := cpu.cost(sdr); got != costHD {
		t.Errorf("sdr: cost %v, want %v", got, costHD)
	}
}
