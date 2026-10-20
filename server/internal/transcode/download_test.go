package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Rate control and AAC bit rate of a download (M5 design 3.1/3.2): CBR at the plan's VideoBitrate
// per backend. Compared up to the output seek like TestFFmpegArgsGoldenM4; the HLS lists without
// ConstantRate and AudioBitrate stay those of the golden tests above.
func TestFFmpegArgsConstantRate(t *testing.T) {
	src := Source{
		Path: "/media/Film.mkv",
		Probe: Probe{Format: Format{Duration: 10 * time.Second, BitRate: 8_800_000}, Streams: []Stream{
			{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 1920, Height: 1080, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
			{Index: 1, Type: "audio", Codec: "eac3", Channels: 6, Default: true},
		}},
		Keyframes: []time.Duration{0, 5 * time.Second},
	}
	medium := Target{Mode: ModeTranscode, MaxHeight: 720, MaxBitrate: 3_000_000, ConstantRate: true, AudioBitrate: 128,
		SubtitleBurnIn: -1, AudioStream: -1}
	const (
		head  = "-nostdin -hide_banner -loglevel warning -copyts"
		maps  = "-i /media/Film.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1"
		kf    = "-force_key_frames 4.000000,8.000000"
		aac   = "-c:a aac -b:a 128k -ac 2"
		cudaI = "-hwaccel cuda -hwaccel_output_format cuda"
		qsvI  = "-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv"
		vaI   = "-init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device va"
	)
	cases := []struct {
		name string
		caps Capabilities
		t    Target
		want string
	}{
		{"x264 cbr", Capabilities{Backend: BackendSoftware}, medium,
			head + " " + maps + " -vf scale=w=1280:h=720:flags=bicubic,format=yuv420p -c:v libx264 -preset veryfast -b:v 3000000 -minrate 3000000 -maxrate 3000000 -bufsize 6000000 -x264-params nal-hrd=cbr -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 " + kf + " " + aac},
		{"nvenc cbr", Capabilities{Backend: BackendNVENC}, medium,
			head + " " + cudaI + " " + maps + " -vf scale_cuda=w=1280:h=720:format=nv12 -c:v h264_nvenc -preset p4 -tune hq -rc cbr -b:v 3000000 -maxrate 3000000 -bufsize 6000000 -profile:v high -level:v 4.1 -g 9999 -no-scenecut 1 -forced-idr 1 " + kf + " " + aac},
		{"qsv cbr", Capabilities{Backend: BackendQSV}, medium,
			head + " " + qsvI + " " + maps + " -vf vpp_qsv=w=1280:h=720:format=nv12 -c:v h264_qsv -preset veryfast -b:v 3000000 -look_ahead 0 -maxrate 3000000 -bufsize 6000000 -profile:v high -level:v 41 -g 9999 -forced_idr 1 " + kf + " " + aac},
		{"vaapi cbr", Capabilities{Backend: BackendVAAPI}, medium,
			head + " " + vaI + " " + maps + " -vf scale_vaapi=w=1280:h=720:format=nv12 -c:v h264_vaapi -rc_mode CBR -b:v 3000000 -maxrate 3000000 -bufsize 6000000 -profile:v high -level:v 4.1 -g 9999 " + kf + " " + aac},
		{"cbr at the rung of a smaller source", Capabilities{Backend: BackendSoftware},
			Target{Mode: ModeTranscode, MaxHeight: 1080, MaxBitrate: 6_000_000, ConstantRate: true, Quality: "480p", SubtitleBurnIn: -1, AudioStream: -1},
			head + " " + maps + " -vf scale=w=852:h=480:flags=bicubic,format=yuv420p -c:v libx264 -preset veryfast -b:v 2000000 -minrate 2000000 -maxrate 2000000 -bufsize 4000000 -x264-params nal-hrd=cbr -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 " + kf + " -c:a aac -b:a 160k -ac 2"},
		{"audio bit rate without cbr", Capabilities{Backend: BackendSoftware},
			Target{Mode: ModeTranscode, MaxHeight: 720, AudioBitrate: 96, SubtitleBurnIn: -1, AudioStream: -1},
			head + " " + maps + " -vf scale=w=1280:h=720:flags=bicubic,format=yuv420p -c:v libx264 -preset veryfast -crf 23 -maxrate 4000000 -bufsize 8000000 -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 " + kf + " -c:a aac -b:a 96k -ac 2"},
		{"remux encodes the audio at the bit rate", Capabilities{Backend: BackendSoftware},
			Target{Mode: ModeRemux, VideoCopy: true, ConstantRate: true, AudioBitrate: 96, SubtitleBurnIn: -1, AudioStream: -1},
			head + " " + maps + " -c:v copy -c:a aac -b:a 96k -ac 2"},
		{"copied audio ignores the bit rate", Capabilities{Backend: BackendSoftware},
			Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, ConstantRate: true, AudioBitrate: 96, SubtitleBurnIn: -1, AudioStream: -1},
			head + " " + maps + " -c:v copy -c:a copy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(src, c.t)
			if err != nil {
				t.Fatal(err)
			}
			args, err := ffmpegArgs(c.caps, src, p, 0, "/w/d/encode")
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(args, " ")
			got = got[:strings.Index(got, " -output_ts_offset")]
			if got != c.want {
				t.Errorf("args differ\ngot:  %s\nwant: %s", got, c.want)
			}
		})
	}
}

// delivered is what a Download call handed to its callbacks.
type delivered struct {
	inits  [][]byte
	idx    []int
	segs   map[int][]byte
	starts map[int]Segment
}

// collectDownload runs Download and records the callbacks; stopAfter > 0 cancels ctx once that
// many segments arrived.
func collectDownload(ctx context.Context, r Runner, src Source, p Plan, workDir string, start int,
	onSeg func(n int) error) (*delivered, error) {
	d := &delivered{segs: map[int][]byte{}, starts: map[int]Segment{}}
	err := r.Download(ctx, Capabilities{Backend: BackendSoftware}, src, p, workDir, start,
		func(init []byte) error {
			d.inits = append(d.inits, init)
			return nil
		},
		func(n int, data []byte, start, dur time.Duration) error {
			d.idx = append(d.idx, n)
			d.segs[n] = data
			d.starts[n] = Segment{Start: start, Duration: dur}
			if onSeg != nil {
				return onSeg(n)
			}
			return nil
		})
	return d, err
}

func span(from, to int) []int {
	var s []int
	for i := from; i < to; i++ {
		s = append(s, i)
	}
	return s
}

// checkDelivered asserts one init, the segments from..len in order with the plan's start and
// duration, each starting with a keyframe at its planned time, and an empty work directory.
func checkDelivered(t *testing.T, p Plan, d *delivered, from int, workDir string) map[uint32]track {
	t.Helper()
	if len(d.inits) != 1 {
		t.Fatalf("%d init segments, want 1", len(d.inits))
	}
	if want := span(from, len(p.Segments)); !slices.Equal(d.idx, want) {
		t.Fatalf("segments %v, want %v", d.idx, want)
	}
	tracks := parseInit(t, d.inits[0])
	for _, i := range d.idx {
		if d.starts[i] != p.Segments[i] {
			t.Errorf("segment %d at %+v, want %+v", i, d.starts[i], p.Segments[i])
		}
		checkSegment(t, p, tracks, i, d.segs[i])
	}
	if _, err := os.Stat(filepath.Join(workDir, downloadDir)); !os.IsNotExist(err) {
		t.Errorf("encode dir after Download: %v", err)
	}
	return tracks
}

func concat(d *delivered, idx ...int) []byte {
	var b []byte
	for _, i := range idx {
		b = append(b, d.segs[i]...)
	}
	return b
}

// processesWith returns the pids of the processes whose command line contains s.
func processesWith(t *testing.T, s string) []int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("list processes: %v", err)
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if cmd, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline")); err == nil && bytes.Contains(cmd, []byte(s)) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// A CBR transcode of a moving 720p source lands within ±10 % of bit rate × duration (O-1); a
// cancel mid-run returns ctx.Err() and leaves no ffmpeg behind.
func TestDownloadTranscodeConstantRate(t *testing.T) {
	r := hlsRunner(t)
	path := filepath.Join(t.TempDir(), "720p.mkv")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "20", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "128k", path)
	probe, err := r.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	src := Source{Path: path, Probe: probe}
	p, err := NewPlan(src, Target{Mode: ModeTranscode, MaxHeight: 720, MaxBitrate: 3_000_000, ConstantRate: true,
		AudioBitrate: 128, SubtitleBurnIn: -1, AudioStream: -1})
	if err != nil {
		t.Fatal(err)
	}
	if p.VideoBitrate != 3_000_000 || p.Width != 1280 || p.Height != 720 {
		t.Fatalf("plan %+v", p)
	}

	t.Run("size", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		work := t.TempDir()
		d, err := collectDownload(ctx, r, src, p, work, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		checkDelivered(t, p, d, 0, work)
		size := len(d.inits[0]) + len(concat(d, d.idx...))
		want := float64(p.VideoBitrate+128_000) * p.Duration.Seconds() / 8
		t.Logf("size %d bytes, bit rate × duration %.0f bytes (%+.1f %%)", size, want, (float64(size)/want-1)*100)
		if ratio := float64(size) / want; ratio < 0.9 || ratio > 1.1 {
			t.Errorf("size %d bytes is %.2f × the estimate %.0f, want within ±10 %%", size, ratio, want)
		}
		if n := decodes(t, r, d.inits[0], concat(d, d.idx...)); n != 480 {
			t.Errorf("decoded %d frames, want 480", n)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		work := t.TempDir()
		var running []int
		d, err := collectDownload(ctx, r, src, p, work, 0, func(int) error {
			running = processesWith(t, work)
			for _, pid := range running {
				// The encoder runs with nice 10 and oom_score_adj 500, like HLS runs.
				if adj, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/oom_score_adj"); err == nil && strings.TrimSpace(string(adj)) != "500" {
					t.Errorf("oom_score_adj %q, want 500", adj)
				}
				if stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
					fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))
					if len(fields) > 16 && fields[16] != "10" {
						t.Errorf("nice %s, want 10", fields[16])
					}
				}
			}
			cancel()
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Download after cancel: %v, want context.Canceled", err)
		}
		if len(running) == 0 {
			t.Log("ffmpeg had already finished when the first segment arrived")
		}
		if !slices.Equal(d.idx, []int{0}) {
			t.Errorf("segments %v delivered, want only 0 (none after the cancel)", d.idx)
		}
		if left := processesWith(t, work); len(left) > 0 {
			t.Errorf("ffmpeg still running after Download returned: pids %v", left)
		}
		if _, err := os.Stat(filepath.Join(work, downloadDir)); !os.IsNotExist(err) {
			t.Errorf("encode dir after the cancel: %v", err)
		}
	})
}

// A download resumed at segment n has the boundaries of an uninterrupted one, and init plus the
// segments before n of the first run plus those of the resumed run play as one file.
func TestDownloadResumeMatchesUninterrupted(t *testing.T) {
	r := hlsRunner(t)
	src := testClip(t, r)
	cases := []struct {
		name   string
		target Target
		resume int
	}{
		{"remux", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, SubtitleBurnIn: -1, AudioStream: -1}, 2},
		{"transcode", Target{Mode: ModeTranscode, MaxHeight: 360, MaxBitrate: 1_000_000, ConstantRate: true, AudioBitrate: 96,
			SubtitleBurnIn: -1, AudioStream: -1}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(src, c.target)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			work := t.TempDir()
			full, err := collectDownload(ctx, r, src, p, work, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			checkDelivered(t, p, full, 0, work)
			resumed, err := collectDownload(ctx, r, src, p, work, c.resume, nil)
			if err != nil {
				t.Fatal(err)
			}
			checkDelivered(t, p, resumed, c.resume, work)
			for i := c.resume; i < len(p.Segments); i++ {
				if full.starts[i] != resumed.starts[i] {
					t.Errorf("segment %d: resumed %+v, uninterrupted %+v", i, resumed.starts[i], full.starts[i])
				}
			}
			if !bytes.Equal(full.inits[0], resumed.inits[0]) {
				t.Errorf("the init segment of the resumed run differs (%d vs %d bytes)", len(resumed.inits[0]), len(full.inits[0]))
			}

			joined := slices.Concat(full.inits[0], concat(full, span(0, c.resume)...), concat(resumed, span(c.resume, len(p.Segments))...))
			if n := decodes(t, r, full.inits[0], joined[len(full.inits[0]):]); n != 480 {
				t.Errorf("joined download decodes to %d frames, want 480 (20 s at 24 fps)", n)
			}
			file := filepath.Join(t.TempDir(), "joined.mp4")
			if err := os.WriteFile(file, joined, 0o600); err != nil {
				t.Fatal(err)
			}
			probe, err := r.Probe(ctx, file)
			if err != nil {
				t.Fatalf("ffprobe of the joined download: %v", err)
			}
			// The fragments keep the HLS timeline, which starts at timelineOffset: ffprobe reports
			// the end of that timeline as the duration.
			if d := probe.Format.Duration - (p.Duration + timelineOffset); d < -500*time.Millisecond || d > 500*time.Millisecond {
				t.Errorf("ffprobe duration %v, want %v + the timeline offset %v", probe.Format.Duration, p.Duration, timelineOffset)
			}
			if v, ok := videoStream(probe); !ok || v.Codec != "h264" {
				t.Errorf("ffprobe streams %+v", probe.Streams)
			}
		})
	}
}

// fakeDownloadPlan is a 5-segment transcode plan (0, 4, 8, 12, 16 s) for fake ffmpeg scripts.
func fakeDownloadPlan(t *testing.T) (Source, Plan) {
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	return src, testPlan(t, src)
}

func noInit([]byte) error { return nil }

func TestDownloadErrors(t *testing.T) {
	src, p := fakeDownloadPlan(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	caps := Capabilities{Backend: BackendSoftware}
	keep := func(int, []byte, time.Duration, time.Duration) error { return nil }

	t.Run("arguments", func(t *testing.T) {
		r := Runner{FFmpegPath: fakeBinary(t, "exit 0\n")}
		if err := r.Download(ctx, caps, src, p, t.TempDir(), len(p.Segments), noInit, keep); !errors.Is(err, ErrNoSegment) {
			t.Errorf("start beyond the plan: %v", err)
		}
		if err := r.Download(ctx, caps, src, p, t.TempDir(), 0, nil, keep); err == nil {
			t.Error("want error without callbacks")
		}
		if err := (Runner{}).Download(ctx, caps, src, p, t.TempDir(), 0, noInit, keep); err == nil {
			t.Error("want error without ffmpeg path")
		}
		done, stop := context.WithCancel(ctx)
		stop()
		if err := r.Download(done, caps, src, p, t.TempDir(), 0, noInit, keep); !errors.Is(err, context.Canceled) {
			t.Errorf("canceled context: %v", err)
		}
	})

	t.Run("ffmpeg fails", func(t *testing.T) {
		r := Runner{FFmpegPath: fakeBinary(t, "echo 'Invalid data found when processing input' >&2\nexit 1\n")}
		err := r.Download(ctx, caps, src, p, t.TempDir(), 0, noInit, keep)
		if err == nil || !strings.Contains(err.Error(), "Invalid data found") || errors.Is(err, ErrNoSegment) {
			t.Errorf("Download: %v, want the ffmpeg error", err)
		}
	})

	t.Run("ffmpeg ends early", func(t *testing.T) {
		r := Runner{FFmpegPath: fakeBinary(t, segmentScript(3, "0"))}
		var got []int
		err := r.Download(ctx, caps, src, p, t.TempDir(), 1, noInit, func(n int, _ []byte, _, _ time.Duration) error {
			got = append(got, n)
			return nil
		})
		if !errors.Is(err, ErrNoSegment) || !slices.Equal(got, []int{1, 2}) {
			t.Errorf("Download: %v after segments %v, want ErrNoSegment after 1, 2", err, got)
		}
	})

	t.Run("callback error stops ffmpeg", func(t *testing.T) {
		r := Runner{FFmpegPath: fakeBinary(t, segmentScript(5, "0.05"))}
		boom := errors.New("disk full")
		work := t.TempDir()
		var got []int
		err := r.Download(ctx, caps, src, p, work, 0, noInit, func(n int, data []byte, _, _ time.Duration) error {
			got = append(got, n)
			if !bytes.Equal(data, fakeMedia) {
				t.Errorf("segment %d = %v, want %v", n, data, fakeMedia)
			}
			if n == 1 {
				return boom
			}
			return nil
		})
		if !errors.Is(err, boom) || !slices.Equal(got, []int{0, 1}) {
			t.Errorf("Download: %v after segments %v, want the callback error after 0, 1", err, got)
		}
		if left := processesWith(t, work); len(left) > 0 {
			t.Errorf("fake ffmpeg still running: %v", left)
		}
		initErr := errors.New("init rejected")
		if err := r.Download(ctx, caps, src, p, work, 0, func([]byte) error { return initErr }, keep); !errors.Is(err, initErr) {
			t.Errorf("Download with a failing onInit: %v", err)
		}
		// A cancel wins over a callback error that comes with it.
		cctx, ccancel := context.WithCancel(ctx)
		defer ccancel()
		err = r.Download(cctx, caps, src, p, work, 0, noInit, func(int, []byte, time.Duration, time.Duration) error {
			ccancel()
			return boom
		})
		if !errors.Is(err, context.Canceled) || errors.Is(err, boom) {
			t.Errorf("callback failing after a cancel: %v, want context.Canceled", err)
		}
	})

	t.Run("init once, segments deleted", func(t *testing.T) {
		r := Runner{FFmpegPath: fakeBinary(t, segmentScript(5, "0"))}
		work := t.TempDir()
		if err := os.MkdirAll(filepath.Join(work, downloadDir), 0o750); err != nil {
			t.Fatal(err)
		}
		// Leftovers of a crashed run are wiped; files next to the encode dir stay.
		for _, f := range []string{filepath.Join(downloadDir, segmentList), "media.mp4"} {
			if err := os.WriteFile(filepath.Join(work, f), []byte("seg00000.m4s\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var inits [][]byte
		var got []int
		err := r.Download(ctx, caps, src, p, work, 2, func(b []byte) error {
			inits = append(inits, b)
			return nil
		}, func(n int, _ []byte, start, dur time.Duration) error {
			got = append(got, n)
			if start != p.Segments[n].Start || dur != p.Segments[n].Duration {
				t.Errorf("segment %d at %v+%v, want %+v", n, start, dur, p.Segments[n])
			}
			if _, err := os.Stat(filepath.Join(work, downloadDir, fmt.Sprintf(segmentPattern, n-1))); !os.IsNotExist(err) {
				t.Errorf("segment %d still on disk while %d is delivered: %v", n-1, n, err)
			}
			return nil
		})
		if err != nil || !slices.Equal(got, []int{2, 3, 4}) || len(inits) != 1 || !bytes.Equal(inits[0], fakeInit) {
			t.Errorf("Download: %v, segments %v, inits %v", err, got, inits)
		}
		if _, err := os.Stat(filepath.Join(work, "media.mp4")); err != nil {
			t.Errorf("file next to the encode dir: %v", err)
		}
	})
}

// newLeaseManager returns a manager with fake sessions (sleep) and the budget.
func newLeaseManager(t *testing.T, budget float64) *Manager {
	t.Helper()
	m := newTestManager(t, "exec sleep 30\n", budget, time.Minute)
	m.admitWait = 5 * time.Second
	return m
}

func acquireAsync(m *Manager, ctx context.Context, id string, cost float64) chan *DownloadLease {
	ch := make(chan *DownloadLease, 1)
	go func() {
		l, err := m.AcquireDownload(ctx, id, cost)
		if err != nil {
			l = nil
		}
		ch <- l
	}()
	return ch
}

func startAsync(m *Manager, ctx context.Context, id string, src Source, p Plan) chan error {
	ch := make(chan error, 1)
	go func() {
		_, err := m.Start(ctx, id, src, p, 0)
		ch <- err
	}()
	return ch
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestAcquireDownloadWaitsForBudget(t *testing.T) {
	m := newLeaseManager(t, 3)
	goroutines := runtime.NumGoroutine()
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	p := testPlan(t, src) // cost 2
	ctx := context.Background()
	if _, err := m.Start(ctx, "A", src, p, 0); err != nil {
		t.Fatal(err)
	}
	got := acquireAsync(m, ctx, "D1", 2)
	time.Sleep(150 * time.Millisecond)
	select {
	case <-got:
		t.Fatal("download admitted while the budget is full")
	default:
	}
	if st := m.Stats(); st.Downloads != 0 || st.DownloadUsed != 0 || st.Used != 2 {
		t.Errorf("stats while waiting: %+v", st)
	}
	m.Stop("A")
	l := <-got
	if l == nil {
		t.Fatal("download not admitted after the release")
	}
	if st := m.Stats(); st.Downloads != 1 || st.DownloadUsed != 2 || st.Used != 0 {
		t.Errorf("stats with the lease: %+v", st)
	}

	// A canceled wait returns the context error; invalid ids and costs are refused.
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := m.AcquireDownload(cctx, "D2", 2); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("canceled wait: %v", err)
	}
	if _, err := m.AcquireDownload(ctx, "../x", 1); err == nil {
		t.Error("want error for an unsafe id")
	}
	if _, err := m.AcquireDownload(ctx, "D3", -1); err == nil {
		t.Error("want error for a negative cost")
	}

	l.Release()
	l.Release()
	if st := m.Stats(); st.Downloads != 0 || st.DownloadUsed != 0 {
		t.Errorf("stats after Release: %+v", st)
	}
	if isClosed(l.Preempted()) {
		t.Error("a released lease was preempted")
	}
	waitFor(t, "goroutines to end", func() bool { return runtime.NumGoroutine() <= goroutines })
}

// A download does not overtake a live Start that waits, even when it would fit.
func TestAcquireDownloadYieldsToWaitingStart(t *testing.T) {
	m := newLeaseManager(t, 3)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	p := testPlan(t, src) // cost 2
	ctx := context.Background()
	if _, err := m.Start(ctx, "A", src, p, 0); err != nil {
		t.Fatal(err)
	}
	live := startAsync(m, ctx, "B", src, p)
	waitFor(t, "B in the queue", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.waiting) == 1
	})
	got := acquireAsync(m, ctx, "D", 0.5) // 2 + 0.5 fits, but B waits
	time.Sleep(150 * time.Millisecond)
	select {
	case <-got:
		t.Fatal("download admitted while a live start waits")
	default:
	}
	m.Stop("A")
	if err := <-live; err != nil {
		t.Fatalf("live start: %v", err)
	}
	l := <-got
	if l == nil {
		t.Fatal("download not admitted after the live start")
	}
	defer l.Release()
	if st := m.Stats(); st.Used != 2 || st.DownloadUsed != 0.5 || st.Downloads != 1 || st.Sessions != 1 {
		t.Errorf("stats %+v", st)
	}
}

// A live Start without budget preempts download leases, newest first and only as many as make
// room, and is admitted once they are released.
func TestStartPreemptsDownloads(t *testing.T) {
	m := newLeaseManager(t, 3)
	goroutines := runtime.NumGoroutine()
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	p := testPlan(t, src) // cost 2
	ctx := context.Background()
	older, err := m.AcquireDownload(ctx, "D1", 1)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := m.AcquireDownload(ctx, "D2", 1)
	if err != nil {
		t.Fatal(err)
	}
	live := startAsync(m, ctx, "A", src, p)
	select {
	case <-newer.Preempted():
	case <-time.After(5 * time.Second):
		t.Fatal("the newest lease was not preempted")
	}
	if isClosed(older.Preempted()) {
		t.Error("the older lease was preempted although one lease makes room")
	}
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-live:
		t.Fatalf("live start admitted before the preempted lease was released: %v", err)
	default:
	}
	if st := m.Stats(); st.Downloads != 2 || st.DownloadUsed != 2 {
		t.Errorf("a preempted lease keeps its units until Release: %+v", st)
	}
	newer.Release()
	if err := <-live; err != nil {
		t.Fatalf("live start after the release: %v", err)
	}
	if st := m.Stats(); st.Used != 2 || st.Downloads != 1 || st.DownloadUsed != 1 {
		t.Errorf("stats %+v", st)
	}

	// Preempting cannot make room next to session A: the second Start waits without preempting.
	m.admitWait = 200 * time.Millisecond
	if _, err := m.Start(ctx, "B", src, p, 0); !errors.Is(err, ErrBusy) {
		t.Errorf("second start: %v, want ErrBusy", err)
	}
	if isClosed(older.Preempted()) {
		t.Error("lease preempted although that did not make room")
	}
	older.Release()
	m.Stop("A")
	waitFor(t, "goroutines to end", func() bool { return runtime.NumGoroutine() <= goroutines })
}

// A cost above the budget is capped, so the download runs alone; an oversized live session
// preempts it.
func TestAcquireDownloadCapsCost(t *testing.T) {
	m := newLeaseManager(t, 3)
	ctx := context.Background()
	l, err := m.AcquireDownload(ctx, "big", 10)
	if err != nil {
		t.Fatal(err)
	}
	if st := m.Stats(); st.DownloadUsed != 3 || st.Downloads != 1 {
		t.Errorf("stats %+v, want the cost capped at the budget 3", st)
	}
	hdr := videoSource("hevc", 3840, 2160, "smpte2084", sec(20))
	p, err := NewPlan(hdr, Target{Mode: ModeTranscode, Tonemap: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.AdmissionCost(p); got != costHDRCPU {
		t.Errorf("admission cost of a CPU-tonemapped HDR plan %v, want %v", got, costHDRCPU)
	}
	live := startAsync(m, ctx, "huge", hdr, p) // 6 > 3: runs alone once the lease is gone
	<-l.Preempted()
	l.Release()
	if err := <-live; err != nil {
		t.Fatalf("oversized live start: %v", err)
	}
	// Downloads wait while the oversized session runs, and run once it is gone.
	got := acquireAsync(m, ctx, "next", 1)
	time.Sleep(100 * time.Millisecond)
	select {
	case <-got:
		t.Fatal("download admitted next to an oversized session")
	default:
	}
	m.Stop("huge")
	if l := <-got; l == nil {
		t.Fatal("download not admitted")
	} else {
		l.Release()
	}
}

// A session resuming after an idle stop takes its budget back and preempts the leases that no
// longer fit; Close preempts the rest and refuses new leases.
func TestResumeAndClosePreemptDownloads(t *testing.T) {
	m := newTestManager(t, "exec sleep 30\n", 3, 100*time.Millisecond)
	src := videoSource("hevc", 1920, 1080, "", sec(20))
	s, err := m.Start(context.Background(), "idle", src, testPlan(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "idle stop", func() bool { return m.Stats().Used == 0 })
	ctx := context.Background()
	big, err := m.AcquireDownload(ctx, "D1", 2)
	if err != nil {
		t.Fatal(err)
	}
	small, err := m.AcquireDownload(ctx, "D2", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	rctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := s.OpenSegment(rctx, 3); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("OpenSegment: %v", err)
	}
	// 2 (session) + 2.5 > 3: the newest lease goes first, then the next one.
	if !isClosed(small.Preempted()) || !isClosed(big.Preempted()) {
		t.Errorf("preempted: small %v, big %v; want both", isClosed(small.Preempted()), isClosed(big.Preempted()))
	}
	big.Release()
	small.Release()
	l, err := m.AcquireDownload(ctx, "D3", 1)
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	if !isClosed(l.Preempted()) {
		t.Error("Close did not preempt the lease")
	}
	l.Release()
	if _, err := m.AcquireDownload(ctx, "D4", 1); !errors.Is(err, ErrClosed) {
		t.Errorf("AcquireDownload after Close: %v", err)
	}
}
