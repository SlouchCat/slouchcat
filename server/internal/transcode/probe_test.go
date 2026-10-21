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
	path := testMovie(t, "Slouchcat Test Chapters (2024) {tmdb-7004}", "Slouchcat Test Chapters (2024) {tmdb-7004}.mkv")
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

func TestParseFrameSideData(t *testing.T) {
	cases := map[string]bool{
		// Dutton Ranch S01E07 (HDR10+), shortened ffprobe output.
		`{"frames":[{"side_data_list":[{"side_data_type":"H.26[45] User Data Unregistered SEI message"},{"side_data_type":"Mastering display metadata"},{"side_data_type":"Content light level metadata"},{"side_data_type":"HDR Dynamic Metadata SMPTE2094-40 (HDR10+)"}]}]}`: true,
		// HDR10 only.
		`{"frames":[{"side_data_list":[{"side_data_type":"Mastering display metadata"},{"side_data_type":"Content light level metadata"}]}]}`: false,
		// Dolby Vision profile 8 without HDR10+.
		`{"frames":[{"side_data_list":[{"side_data_type":"Dolby Vision RPU Data"},{"side_data_type":"Dolby Vision Metadata","components":[{"pieces":[{}]}]}]}]}`: false,
		`{"frames":[]}`: false,
		`{}`:            false,
	}
	for in, want := range cases {
		got, err := ParseFrameSideData([]byte(in))
		if err != nil || got != want {
			t.Errorf("ParseFrameSideData(%s) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseFrameSideData([]byte("not json")); err == nil {
		t.Error("want error for invalid JSON")
	}
}

func TestProbeHDR(t *testing.T) {
	r := realRunner(t)
	for _, c := range []struct{ folder, file string }{
		{"Slouchcat Test HDR10 4K (2024)", "Slouchcat Test HDR10 4K (2024).mkv"},
		{"Slouchcat Test HDR10 Chart (2024)", "Slouchcat Test HDR10 Chart (2024).mkv"},
	} {
		p, err := r.Probe(context.Background(), testMovie(t, c.folder, c.file))
		if err != nil {
			t.Fatal(err)
		}
		v, ok := videoStream(p)
		if !ok || v.HDRFormat() != HDR10 || v.HDR10Plus || v.ColorPrimaries != "bt2020" || v.Width != 3840 {
			t.Errorf("%s: video %+v, format %q", c.file, v, v.HDRFormat())
		}
	}
	// SDR skips the side data call; the multi-audio file keeps its track metadata.
	p, err := r.Probe(context.Background(), testMovie(t, "Slouchcat Test Multi Audio (2024)", "Slouchcat Test Multi Audio (2024).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		typ, codec, lang string
		def              bool
	}{{"video", "h264", "", false}, {"audio", "aac", "ger", true}, {"audio", "ac3", "eng", false},
		{"subtitle", "subrip", "ger", false}, {"subtitle", "subrip", "eng", false}}
	if len(p.Streams) != len(want) {
		t.Fatalf("multi audio: %+v", p.Streams)
	}
	for i, w := range want {
		s := p.Streams[i]
		if s.Type != w.typ || s.Codec != w.codec || s.Language != w.lang || (w.typ == "audio" && s.Default != w.def) {
			t.Errorf("multi audio stream %d: %+v, want %+v", i, s, w)
		}
	}
	if p.Streams[2].Channels != 6 {
		t.Errorf("AC3 track has %d channels, want 6", p.Streams[2].Channels)
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
