package transcode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// realRunner returns a Runner with the ffprobe from PATH. The probe tests need the real binary
// and the generated test media (`task testdata:media`); they fail instead of skipping.
func realRunner(t *testing.T) Runner {
	t.Helper()
	bin, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatalf("ffprobe is required for the probe tests (install ffmpeg): %v", err)
	}
	return Runner{FFprobePath: bin}
}

func testMovie(t *testing.T, folder, file string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Movies", folder, file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("test media missing (run `task testdata:media` in the repository root): %v", err)
	}
	return path
}

func TestProbeH264(t *testing.T) {
	r := realRunner(t)
	path := testMovie(t, "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4")
	p, err := r.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Format.Duration < 5*time.Second || p.Format.Size <= 0 || p.Format.BitRate <= 0 {
		t.Errorf("format = %+v, want duration, size and bit rate", p.Format)
	}
	var video, audio *Stream
	for i := range p.Streams {
		switch p.Streams[i].Type {
		case "video":
			video = &p.Streams[i]
		case "audio":
			audio = &p.Streams[i]
		}
	}
	if video == nil || video.Codec != "h264" || video.Width == 0 || video.Height == 0 || video.PixFmt == "" {
		t.Errorf("video stream = %+v", video)
	}
	if audio == nil || audio.Codec == "" || audio.Channels == 0 {
		t.Errorf("audio stream = %+v", audio)
	}
	t.Logf("format %s, %v, video %s %dx%d", p.Format.Name, p.Format.Duration, video.Codec, video.Width, video.Height)
}

func TestProbeChapters(t *testing.T) {
	r := realRunner(t)
	path := testMovie(t, "Slouchcat Test Chapters (2024)", "Slouchcat Test Chapters (2024).mkv")
	p, err := r.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Chapters) < 2 {
		t.Fatalf("chapters = %+v, want at least 2", p.Chapters)
	}
	first := p.Chapters[0]
	if first.Start != 0 || first.End != 3*time.Second || first.Title == "" {
		t.Errorf("first chapter = %+v", first)
	}
	for i := 1; i < len(p.Chapters); i++ {
		if p.Chapters[i].Start < p.Chapters[i-1].Start {
			t.Errorf("chapters not ordered: %+v", p.Chapters)
		}
	}
}

func TestKeyframes(t *testing.T) {
	r := realRunner(t)
	path := testMovie(t, "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4")
	kf, err := r.Keyframes(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(kf) < 2 || kf[0] != 0 {
		t.Fatalf("keyframes = %v, want at least two starting at 0", kf)
	}
	for i := 1; i < len(kf); i++ {
		if kf[i] <= kf[i-1] {
			t.Fatalf("keyframes not strictly increasing: %v", kf)
		}
	}
	t.Logf("%d keyframes: %v", len(kf), kf)
}

func TestProbeErrors(t *testing.T) {
	r := realRunner(t)
	if _, err := r.Probe(context.Background(), filepath.Join(t.TempDir(), "missing.mkv")); err == nil {
		t.Error("want error for a missing file")
	}
	junk := filepath.Join(t.TempDir(), "junk.mkv")
	if err := os.WriteFile(junk, []byte("not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Probe(context.Background(), junk); err == nil {
		t.Error("want error for a file that is not a video")
	}
	if _, err := (Runner{}).Probe(context.Background(), junk); err == nil {
		t.Error("want error for an unconfigured ffprobe")
	}
}

func TestParseKeyframes(t *testing.T) {
	in := "2.000000,K__\n0.041667,___\n0.000000,K__\nN/A,K__\n4.000000,K_D\n2.000000,K__\n\n"
	got := ParseKeyframes([]byte(in))
	want := []time.Duration{0, 2 * time.Second, 4 * time.Second}
	if len(got) != len(want) {
		t.Fatalf("ParseKeyframes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseKeyframes = %v, want %v", got, want)
		}
	}
}

func TestParseProbeRejectsEmpty(t *testing.T) {
	if _, err := ParseProbe([]byte(`{}`)); err == nil {
		t.Error("want error for an empty probe")
	}
	if _, err := ParseProbe([]byte(`not json`)); err == nil {
		t.Error("want error for invalid JSON")
	}
}
