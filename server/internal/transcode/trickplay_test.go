package transcode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestTrickplayTileHeight(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		v    Stream
		want int
	}{
		{Stream{Width: 1920, Height: 1080}, 180},
		{Stream{Width: 3840, Height: 2160}, 180},
		{Stream{Width: 1920, Height: 800}, 132},             // 2.40:1: 133.3 down to even
		{Stream{Width: 720, Height: 480, SAR: "32:27"}, 180}, // 16:9 DVD
		{Stream{Width: 720, Height: 576, SAR: "16:15"}, 240}, // 4:3 PAL
		{Stream{Width: 720, Height: 480, SAR: "0:1"}, 212},  // unknown: square pixels
		{Stream{Width: 720, Height: 480, SAR: "x"}, 212},
		{Stream{}, 180},
		{Stream{Width: 100000, Height: 1}, 2},
	} {
		if got := TrickplayTileHeight(c.v); got != c.want {
			t.Errorf("TrickplayTileHeight(%+v) = %d, want %d", c.v, got, c.want)
		}
	}
	if n, d := (Stream{SAR: "4:3"}).SampleAspect(); n != 4 || d != 3 {
		t.Errorf("SampleAspect = %d:%d", n, d)
	}
}

func TestVideoSAR(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ typ, in, want string }{
		{"video", "32:27", "32:27"}, {"video", "1:1", ""}, {"video", "0:1", ""}, {"video", "", ""}, {"audio", "32:27", ""},
	} {
		if got := videoSAR(c.typ, c.in); got != c.want {
			t.Errorf("videoSAR(%q, %q) = %q, want %q", c.typ, c.in, got, c.want)
		}
	}
}

func TestBlackTrickplayTile(t *testing.T) {
	t.Parallel()
	tile := BlackTrickplayTile(180)
	if len(tile) != TrickplayTileBytes(180) || tile[0] != 16 || tile[320*180-1] != 16 || tile[320*180] != 128 || tile[len(tile)-1] != 128 {
		t.Errorf("black tile: %d bytes, %d %d", len(tile), tile[0], tile[len(tile)-1])
	}
}

func TestTrickplayFrameArgs(t *testing.T) {
	t.Parallel()
	sdr := Stream{Index: 0, Type: "video", Codec: "hevc", Width: 1920, Height: 1080, PixFmt: "yuv420p"}
	args, err := TrickplayFrameArgs("/m/a.mkv", 10001*time.Millisecond, 180, sdr)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	// The measured call (concept 18, 3.2): no accurate seek before the input seek, one decoder
	// thread, one frame without frame rate conversion.
	for _, want := range []string{
		"-threads 1 -noaccurate_seek -ss 10.001 -i /m/a.mkv -map 0:0 -frames:v 1 -fps_mode passthrough",
		"-vf scale=w=320:h=180:flags=lanczos,setsar=1,format=yuv420p -f rawvideo -pix_fmt yuv420p pipe:1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q lack %q", joined, want)
		}
	}
	if slices.Contains(args, "-skip_frame") {
		t.Error("args skip non-key frames (the decoder then waits for the next keyframe)")
	}
	hdr := Stream{Index: 1, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084", ColorPrimaries: "bt2020"}
	args, err = TrickplayFrameArgs("/m/b.mkv", 0, 180, hdr)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(args, " ")
	if !strings.Contains(joined, "-map 0:1") || !strings.Contains(joined, "zscale=w=320:h=180,setsar=1,"+strings.Join(cpuTonemap(), ",")) {
		t.Errorf("hdr args %q", joined)
	}
	dv := Stream{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", DolbyVisionProfile: 5}
	if args, err = TrickplayFrameArgs("/m/c.mp4", 0, 180, dv); err != nil || !strings.Contains(strings.Join(args, " "), dvAsHDR10+",zscale") {
		t.Errorf("dv profile 5 args %q, %v", args, err)
	}
	for _, bad := range []struct {
		in     string
		at     time.Duration
		height int
	}{{"", 0, 180}, {"/m/a", -time.Second, 180}, {"/m/a", 0, 0}, {"/m/a", 0, 181}} {
		if _, err := TrickplayFrameArgs(bad.in, bad.at, bad.height, sdr); err == nil {
			t.Errorf("TrickplayFrameArgs(%+v) accepted", bad)
		}
	}
	if _, err := TrickplaySheetArgs(180, 11, "/c/s"); err == nil {
		t.Error("sheet with 11 rows accepted")
	}
	if args, err := TrickplaySheetArgs(180, 3, "/c/s"); err != nil || !strings.Contains(strings.Join(args, " "), "-s 320x180 -framerate 1 -i pipe:0 -vf tile=layout=10x3 -frames:v 1 -c:v libwebp -quality 75") {
		t.Errorf("sheet args %q, %v", args, err)
	}
}

// TrickplayFrame and TrickplaySheet on the generated test media (SDR MP4, HDR10 MKV) and on an
// anamorphic clip: each frame has the size of its tile, and a sheet of 15 tiles is 2 rows high.
func TestTrickplayFrameAndSheet(t *testing.T) {
	t.Parallel()
	r := ffmpegFull(t)
	ctx := context.Background()
	dir := t.TempDir()

	dvd := filepath.Join(dir, "dvd.mkv")
	gen := exec.CommandContext(ctx, r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=720x480:rate=24:duration=4,setsar=32/27", "-c:v", "libx264", "-preset", "ultrafast", "-g", "24", dvd)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate anamorphic clip: %v: %s", err, out)
	}
	for _, c := range []struct {
		name, path string
		height     int
		hdr        bool
	}{
		{"sdr-mp4", testMovie(t, "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4"), 180, false},
		{"hdr10-mkv", testMovie(t, "Slouchcat Test HDR10 4K (2024)", "Slouchcat Test HDR10 4K (2024).mkv"), 180, true},
		{"anamorphic", dvd, 180, false},
	} {
		p, err := r.Probe(ctx, c.path)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := videoStream(p)
		if !ok {
			t.Fatalf("%s: no video stream", c.name)
		}
		if v.IsHDR() != c.hdr {
			t.Fatalf("%s: IsHDR = %v", c.name, v.IsHDR())
		}
		h := TrickplayTileHeight(v)
		if h != c.height {
			t.Errorf("%s: tile height %d (sar %q), want %d", c.name, h, v.SAR, c.height)
		}
		started := time.Now()
		var tiles [][]byte
		for _, at := range []time.Duration{time.Millisecond, 1001 * time.Millisecond, 2001 * time.Millisecond} {
			tile, err := r.TrickplayFrame(ctx, c.path, at, h, v)
			if err != nil {
				t.Fatalf("%s at %v: %v", c.name, at, err)
			}
			tiles = append(tiles, tile)
		}
		t.Logf("%s: 3 frames in %v", c.name, time.Since(started))
		for len(tiles) < 15 {
			tiles = append(tiles, tiles[len(tiles)%3])
		}
		out := filepath.Join(dir, c.name+".webp")
		if err := r.TrickplaySheet(ctx, tiles, h, out); err != nil {
			t.Fatalf("%s sheet: %v", c.name, err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		sp, err := r.Probe(ctx, out)
		if err != nil {
			t.Fatal(err)
		}
		if sv, ok := videoStream(sp); !webpHeader(b) || !ok || sv.Width != TrickplayColumns*TrickplayTileWidth || sv.Height != 2*h {
			t.Errorf("%s sheet: WebP %v, %+v", c.name, webpHeader(b), sv)
		}
	}

	// Past the end the seek lands on the last keyframe (no accurate seek): a tile all the same.
	v := Stream{Index: 0, Type: "video", Codec: "h264", Width: 720, Height: 480, SAR: "32:27"}
	if tile, err := r.TrickplayFrame(ctx, dvd, time.Minute, 180, v); err != nil || len(tile) != TrickplayTileBytes(180) {
		t.Errorf("past the end: %d bytes, %v", len(tile), err)
	}
	// A file that is no video: ffmpeg runs and fails, a frame failure; an unreadable input is none.
	junk := filepath.Join(dir, "junk.mkv")
	if err := os.WriteFile(junk, []byte("no video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.TrickplayFrame(ctx, junk, 0, 180, v); err == nil || !FrameFailed(err) {
		t.Errorf("junk input: %v, want a frame failure", err)
	}
	locked := filepath.Join(dir, "locked.mkv")
	if err := os.WriteFile(locked, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := r.TrickplayFrame(ctx, locked, 0, 180, v); err == nil || (FrameFailed(err) && os.Geteuid() != 0) {
		t.Errorf("unreadable input: %v, want no frame failure", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.TrickplayFrame(cancelled, dvd, 0, 180, v); err == nil || FrameFailed(err) {
		t.Errorf("cancelled: %v", err)
	}
	if err := r.TrickplaySheet(ctx, [][]byte{{1, 2}}, 180, filepath.Join(dir, "bad.webp")); err == nil {
		t.Error("sheet with a short tile accepted")
	}
	if err := (Runner{}).TrickplaySheet(ctx, [][]byte{BlackTrickplayTile(180)}, 180, filepath.Join(dir, "none.webp")); err == nil {
		t.Error("sheet without ffmpeg accepted")
	}
}

func TestManagerCPUHeavy(t *testing.T) {
	ctx := context.Background()
	src := videoSource("hevc", 1920, 1080, "", sec(20))

	// Software backend: any video transcode is heavy, a remux is not.
	m := newTestManager(t, "exec sleep 30\n", 20, time.Minute)
	if m.CPUHeavy() {
		t.Error("idle manager is CPU heavy")
	}
	remux := testPlan(t, src)
	remux.Target.VideoCopy = true
	if _, err := m.Start(ctx, "R", src, remux, 0); err != nil {
		t.Fatal(err)
	}
	if m.CPUHeavy() {
		t.Error("a remux is CPU heavy")
	}
	if _, err := m.Start(ctx, "T", src, testPlan(t, src), 0); err != nil {
		t.Fatal(err)
	}
	if !m.CPUHeavy() {
		t.Error("a software transcode is not CPU heavy")
	}
	m.Stop("T")
	if m.CPUHeavy() {
		t.Error("CPU heavy after the transcode stopped")
	}
	l, err := m.AcquireDownload(ctx, "D", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !m.CPUHeavy() {
		t.Error("a download encode is not CPU heavy")
	}
	l.Release()

	// QSV without GPU tonemapping: only the tonemapping transcode is heavy.
	q, err := NewManager(ManagerOptions{
		Runner: Runner{FFmpegPath: fakeBinary(t, "exec sleep 30\n")}, WorkDir: filepath.Join(t.TempDir(), "transcode"),
		Backend: BackendQSV, Budget: 20, IdleTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	if _, err := q.Start(ctx, "H", src, testPlan(t, src), 0); err != nil {
		t.Fatal(err)
	}
	if q.CPUHeavy() {
		t.Error("a QSV transcode is CPU heavy")
	}
	tonemap := testPlan(t, src)
	tonemap.Target.Tonemap = true
	if _, err := q.Start(ctx, "M", src, tonemap, 0); err != nil {
		t.Fatal(err)
	}
	if !q.CPUHeavy() {
		t.Error("a CPU tonemapping transcode is not CPU heavy")
	}
}
