package transcode

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestChapterFrameTimes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		ch   model.Chapter
		want []time.Duration
	}{
		// A tenth of a 10-minute chapter is 60 s: capped at 30 s, the retry 10 s later.
		{model.Chapter{StartMs: 600_000, EndMs: 1_200_000}, []time.Duration{630 * time.Second, 640 * time.Second}},
		// A tenth of 100 s is 10 s.
		{model.Chapter{StartMs: 0, EndMs: 100_000}, []time.Duration{10 * time.Second, 20 * time.Second}},
		// A 15-s chapter: 1.5 s, the retry at 11.5 s still inside.
		{model.Chapter{StartMs: 5_000, EndMs: 20_000}, []time.Duration{6500 * time.Millisecond, 16500 * time.Millisecond}},
		// A 10-s chapter has no room for the retry.
		{model.Chapter{StartMs: 0, EndMs: 10_000}, []time.Duration{time.Second}},
	} {
		if got := ChapterFrameTimes(c.ch); !slices.Equal(got, c.want) {
			t.Errorf("ChapterFrameTimes(%+v) = %v, want %v", c.ch, got, c.want)
		}
	}
}

func TestGrabFrameArgs(t *testing.T) {
	t.Parallel()
	sdr := Stream{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080, PixFmt: "yuv420p"}
	args, err := GrabFrameArgs("/m/a.mkv", 90500*time.Millisecond, 480, sdr, "/c/0.tmp")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	// Input seek: -ss before -i, so ffmpeg reads only from the keyframe before the time point on.
	if ss, in := slices.Index(args, "-ss"), slices.Index(args, "-i"); ss < 0 || ss > in || args[ss+1] != "90.500" || args[in+1] != "/m/a.mkv" {
		t.Errorf("seek args = %v", args)
	}
	for _, want := range []string{
		"[0:0]scale=w=480:h=trunc(480/dar/2)*2:flags=lanczos,setsar=1,format=yuv420p,split=2[img][probe]",
		"[probe]scale=w=64:h=36:flags=area,format=gray[luma]",
		"-map [img] -frames:v 1 -c:v libwebp -quality 82 -compression_level 4 -f webp /c/0.tmp",
		"-map [luma] -frames:v 1 -f rawvideo -pix_fmt gray pipe:1",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "tonemap") {
		t.Errorf("SDR source tonemapped: %s", joined)
	}

	// HDR10: CPU tonemapping, height from the display aspect ratio, the stream index.
	hdr := Stream{Index: 1, Type: "video", Codec: "hevc", Width: 3840, Height: 1608, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084"}
	args, err = GrabFrameArgs("/m/b.mkv", 0, 480, hdr, "/c/1.tmp")
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(args, " ")
	if !strings.Contains(joined, "[0:1]zscale=w=480:h=trunc(480/dar/2)*2,setsar=1,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,"+
		"tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p,split=2") {
		t.Errorf("HDR args = %s", joined)
	}
	if strings.Contains(joined, "setparams") {
		t.Errorf("HDR10 source got the Dolby Vision tags: %s", joined)
	}

	// Dolby Vision profile 5 carries no transfer tag: tagged as PQ first, then tonemapped.
	dv := Stream{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, DolbyVisionProfile: 5}
	args, err = GrabFrameArgs("/m/c.mp4", time.Minute, 480, dv, "/c/2.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if joined = strings.Join(args, " "); !strings.Contains(joined, "[0:0]"+dvAsHDR10+",zscale=w=480:h=trunc(480/dar/2)*2,setsar=1,") {
		t.Errorf("Dolby Vision args = %s", joined)
	}

	for _, bad := range []struct {
		width int
		at    time.Duration
	}{{0, 0}, {481, 0}, {480, -time.Second}} {
		if _, err := GrabFrameArgs("/m/a.mkv", bad.at, bad.width, sdr, "/c/x"); err == nil {
			t.Errorf("GrabFrameArgs(width %d, at %v) accepted", bad.width, bad.at)
		}
	}
}

func TestBlackLuma(t *testing.T) {
	t.Parallel()
	frame := bytes.Repeat([]byte{16}, lumaWidth*lumaHeight)
	if !blackLuma(frame) {
		t.Error("TV black is not black")
	}
	// 2 % bright pixels (credits text on black) still count as black, 3 % do not.
	n := len(frame) * 2 / 100
	for i := range n {
		frame[i] = 235
	}
	if !blackLuma(frame) {
		t.Error("2 % bright pixels: not black")
	}
	for i := n; i < len(frame)*3/100; i++ {
		frame[i] = 235
	}
	if blackLuma(frame) {
		t.Error("3 % bright pixels: black")
	}
}

// webpHeader reports whether b starts like a WebP file (RIFF....WEBP).
func webpHeader(b []byte) bool {
	return len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

// GrabFrame on the generated test media: an SDR frame of the H.264 file, a tonemapped frame of the
// 4K HDR10 file (both not black) and the black start of a clip that turns into test bars.
func TestGrabFrame(t *testing.T) {
	t.Parallel()
	r := ffmpegFull(t)
	ctx := context.Background()
	dir := t.TempDir()
	for _, c := range []struct {
		name, folder, file string
	}{
		{"sdr", "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4"},
		{"hdr10", "Slouchcat Test HDR10 4K (2024)", "Slouchcat Test HDR10 4K (2024).mkv"},
	} {
		path := testMovie(t, c.folder, c.file)
		p, err := r.Probe(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := videoStream(p)
		if !ok {
			t.Fatalf("%s: no video stream", c.name)
		}
		if (c.name == "hdr10") != v.IsHDR() {
			t.Fatalf("%s: IsHDR = %v", c.name, v.IsHDR())
		}
		out := filepath.Join(dir, c.name+".webp")
		started := time.Now()
		black, err := r.GrabFrame(ctx, path, 2*time.Second, ChapterImageWidth, v, out)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if black || !webpHeader(b) {
			t.Errorf("%s: black %v, %d bytes, WebP %v", c.name, black, len(b), webpHeader(b))
		}
		t.Logf("%s: %d bytes in %v", c.name, len(b), time.Since(started))
	}

	// Black for 3 s, then test bars: black at 1 s, not black at 4 s.
	clip := filepath.Join(dir, "black.mp4")
	gen := exec.CommandContext(ctx, r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:size=640x360:rate=24:duration=3",
		"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24:duration=3",
		"-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0[v]", "-map", "[v]", "-c:v", "libx264", "-preset", "ultrafast", "-g", "24", clip)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v: %s", err, out)
	}
	v := Stream{Index: 0, Type: "video", Codec: "h264", Width: 640, Height: 360, PixFmt: "yuv420p"}
	for _, c := range []struct {
		at    time.Duration
		black bool
	}{{time.Second, true}, {4 * time.Second, false}} {
		black, err := r.GrabFrame(ctx, clip, c.at, ChapterImageWidth, v, filepath.Join(dir, "clip.webp"))
		if err != nil || black != c.black {
			t.Errorf("clip at %v: black %v, %v; want %v", c.at, black, err, c.black)
		}
	}
	// Past the end of the clip ffmpeg writes no frame: a frame failure.
	_, err := r.GrabFrame(ctx, clip, time.Minute, ChapterImageWidth, v, filepath.Join(dir, "end.webp"))
	if !errors.Is(err, ErrNoFrame) || !FrameFailed(err) {
		t.Errorf("past the end: %v, want ErrNoFrame", err)
	}
	// A file that is no video: ffmpeg runs and fails, also a frame failure.
	junk := filepath.Join(dir, "junk.mkv")
	if err := os.WriteFile(junk, []byte("no video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GrabFrame(ctx, junk, 0, ChapterImageWidth, v, filepath.Join(dir, "junk.webp")); err == nil || !FrameFailed(err) {
		t.Errorf("junk input: %v, want a frame failure", err)
	}
	// No ffmpeg, a cancelled context: no frame failure.
	if _, err := (Runner{}).GrabFrame(ctx, clip, 0, ChapterImageWidth, v, filepath.Join(dir, "none.webp")); err == nil || FrameFailed(err) {
		t.Errorf("without ffmpeg: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.GrabFrame(cancelled, clip, 0, ChapterImageWidth, v, filepath.Join(dir, "cancel.webp")); err == nil || FrameFailed(err) {
		t.Errorf("cancelled: %v", err)
	}

	// Anamorphic 720x480 with a sample aspect ratio of 32:27 (16:9 DVD): 480x270, square pixels.
	dvd := filepath.Join(dir, "dvd.mkv")
	gen = exec.CommandContext(ctx, r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=720x480:rate=24:duration=2,setsar=32/27", "-c:v", "libx264", "-preset", "ultrafast", dvd)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate anamorphic clip: %v: %s", err, out)
	}
	out := filepath.Join(dir, "dvd.webp")
	if _, err := r.GrabFrame(ctx, dvd, time.Second, ChapterImageWidth, Stream{Index: 0, Type: "video", Codec: "h264", Width: 720, Height: 480}, out); err != nil {
		t.Fatal(err)
	}
	p, err := r.Probe(ctx, out)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := videoStream(p); !ok || v.Width != 480 || v.Height != 270 {
		t.Errorf("anamorphic frame = %+v, want 480x270", v)
	}
}
