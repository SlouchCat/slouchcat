package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// ffmpegFull returns a Runner with ffmpeg and ffprobe from PATH; the subtitle tests need the
// real binaries and the generated test media (`task testdata:media`) and fail without them.
func ffmpegFull(t *testing.T) Runner {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatalf("ffmpeg is required for the subtitle tests: %v", err)
	}
	r := realRunner(t)
	r.FFmpegPath = ff
	return r
}

func TestSubtitlesEmbedded(t *testing.T) {
	r := ffmpegFull(t)
	ctx := context.Background()
	multi := testMovie(t, "Slouchcat Test Multi Audio (2024)", "Slouchcat Test Multi Audio (2024).mkv")
	// Cue times are relative to the start of the file: this one starts at -21 ms (AAC priming),
	// so the captions at 1, 5, ... s are written as 00:01.021 and so on.
	for idx, want := range map[int][]string{
		3: {"WEBVTT", "00:01.021 --> 00:03.021\nDeutsch 1", "00:17.021 --> 00:19.021\nDeutsch 5"},
		4: {"WEBVTT", "00:09.021 --> 00:11.021\nEnglish 3"},
	} {
		vtt, err := r.Subtitles(ctx, multi, idx)
		if err != nil {
			t.Fatalf("stream %d: %v", idx, err)
		}
		for _, w := range want {
			if !strings.Contains(string(vtt), w) {
				t.Errorf("stream %d: WebVTT lacks %q:\n%s", idx, w, vtt)
			}
		}
	}
	// ASS loses its styling: no override tags in the cues.
	subs := testMovie(t, "Slouchcat Test Subtitles (2024)", "Slouchcat Test Subtitles (2024).mkv")
	vtt, err := r.Subtitles(ctx, subs, 4)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(vtt); !strings.Contains(s, "Erster Untertitel (ASS, Deutsch)") || strings.Contains(s, `{\`) {
		t.Errorf("ASS as WebVTT:\n%s", s)
	}

	// A bitmap track, a stream that is no subtitle and a negative index are file errors.
	pgs := testMovie(t, "Slouchcat Test PGS (2024)", "Slouchcat Test PGS (2024).mkv")
	for _, c := range []struct {
		path string
		idx  int
	}{{pgs, 2}, {multi, 1}, {multi, -1}} {
		var fe *FileError
		if _, err := r.Subtitles(ctx, c.path, c.idx); !errors.As(err, &fe) {
			t.Errorf("%s stream %d: %v, want a FileError", filepath.Base(c.path), c.idx, err)
		}
	}
	if _, err := (Runner{}).Subtitles(ctx, multi, 3); err == nil || errors.As(err, new(*FileError)) {
		t.Errorf("unconfigured ffmpeg: %v, want a plain error", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Subtitles(cancelled, multi, 3); err == nil || errors.As(err, new(*FileError)) {
		t.Errorf("cancelled context: %v, want a plain error", err)
	}
}

// M5 review K4: a download converts all its embedded text tracks in one ffmpeg run, with the
// same WebVTT as the per-track conversion and without the 60-s limit of Subtitles.
func TestSubtitleStreams(t *testing.T) {
	r := ffmpegFull(t)
	ctx := context.Background()
	multi := testMovie(t, "Slouchcat Test Multi Audio (2024)", "Slouchcat Test Multi Audio (2024).mkv")
	got, err := r.SubtitleStreams(ctx, multi, []int{4, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("streams = %d, want 2 (duplicates collapse)", len(got))
	}
	for _, idx := range []int{3, 4} {
		one, err := r.Subtitles(ctx, multi, idx)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[idx], one) {
			t.Errorf("stream %d in one run differs from the single conversion:\n%s\n---\n%s", idx, got[idx], one)
		}
	}
	// The per-track limit does not apply.
	old := subtitleTimeout
	subtitleTimeout = time.Nanosecond
	defer func() { subtitleTimeout = old }()
	if _, err := r.SubtitleStreams(ctx, multi, []int{3}); err != nil {
		t.Errorf("one run under a 1-ns per-track timeout: %v", err)
	}
	if empty, err := r.SubtitleStreams(ctx, multi, nil); err != nil || len(empty) != 0 {
		t.Errorf("no streams: %v, %v", empty, err)
	}
	// A bitmap stream among them fails the run as a file error; a cancelled context does not.
	pgs := testMovie(t, "Slouchcat Test PGS (2024)", "Slouchcat Test PGS (2024).mkv")
	var fe *FileError
	if _, err := r.SubtitleStreams(ctx, pgs, []int{2}); !errors.As(err, &fe) {
		t.Errorf("PGS stream: %v, want a FileError", err)
	}
	if _, err := r.SubtitleStreams(ctx, multi, []int{-1}); !errors.As(err, &fe) {
		t.Errorf("negative index: %v, want a FileError", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.SubtitleStreams(cancelled, multi, []int{3}); err == nil || errors.As(err, &fe) {
		t.Errorf("cancelled context: %v, want a plain error", err)
	}
}

func TestSubtitlesFile(t *testing.T) {
	r := ffmpegFull(t)
	ctx := context.Background()
	dir := filepath.Dir(testMovie(t, "Slouchcat Test Sidecar Subtitles (2024)", "Slouchcat Test Sidecar Subtitles (2024).mkv"))
	vtt, err := r.SubtitlesFile(ctx, filepath.Join(dir, "Slouchcat Test Sidecar Subtitles (2024).de.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(vtt); !strings.HasPrefix(s, "WEBVTT") || !strings.Contains(s, "Umlauten: äöü") {
		t.Errorf("SRT as WebVTT:\n%s", s)
	}
	vtt, err = r.SubtitlesFile(ctx, filepath.Join(dir, "Slouchcat Test Sidecar Subtitles (2024).en.ass"))
	if err != nil || !strings.Contains(string(vtt), "Top and italic") {
		t.Errorf("ASS as WebVTT: %v\n%s", err, vtt)
	}

	// An SRT in Windows-1252 is read again with that encoding.
	latin1 := filepath.Join(t.TempDir(), "old.de.srt")
	if err := os.WriteFile(latin1, []byte("1\n00:00:01,000 --> 00:00:02,000\nGr\xfcsse aus Z\xfcrich\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vtt, err = r.SubtitlesFile(ctx, latin1)
	if err != nil || !strings.Contains(string(vtt), "Grüsse aus Zürich") {
		t.Errorf("Windows-1252 SRT: %v\n%s", err, vtt)
	}

	junk := filepath.Join(t.TempDir(), "junk.srt")
	if err := os.WriteFile(junk, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	var fe *FileError
	if _, err := r.SubtitlesFile(ctx, junk); !errors.As(err, &fe) {
		t.Errorf("junk sidecar: %v, want a FileError", err)
	}
	if _, err := r.SubtitlesFile(ctx, filepath.Join(t.TempDir(), "missing.srt")); err == nil || errors.As(err, &fe) {
		t.Errorf("missing sidecar: %v, want a plain error (a vanished file says nothing about the file)", err)
	}
}

// The escaped path of a sidecar burn-in opens a file whose name has every character the filter
// graph treats specially.
func TestBurnInSidecarPath(t *testing.T) {
	r := ffmpegFull(t)
	dir := filepath.Dir(testMovie(t, "Slouchcat Test Sidecar Subtitles (2024)", "Slouchcat Test Sidecar Subtitles (2024).mkv"))
	data, err := os.ReadFile(filepath.Join(dir, "Slouchcat Test Sidecar Subtitles (2024).de.ass"))
	if err != nil {
		t.Fatal(err)
	}
	odd := filepath.Join(t.TempDir(), `it's [a], b; c:d=e\f.ass`)
	if err := os.WriteFile(odd, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f := burnIn{kind: burnText, path: odd, si: -1}.filter()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=gray:s=320x180:r=24:d=2", "-vf", f, "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg -vf %s: %v\n%s", f, err, out)
	}
}

// grayFrames decodes the video of path at 320x180 in 8-bit grey, one slice per frame: all frames,
// or with at >= 0 the five frames from at - 0.1 s.
func grayFrames(t *testing.T, r Runner, path string, at float64) [][]byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var seek, one []string
	if at >= 0 {
		seek, one = []string{"-ss", fmt.Sprintf("%.3f", at-0.1)}, []string{"-frames:v", "5"}
	}
	args := slices.Concat([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, seek,
		[]string{"-i", path, "-map", "0:v:0", "-vf", "scale=320:180,format=gray"}, one, []string{"-f", "rawvideo", "-"})
	out, err := exec.CommandContext(ctx, r.FFmpegPath, args...).Output()
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	const size = 320 * 180
	var frames [][]byte
	for len(out) >= size {
		frames = append(frames, out[:size])
		out = out[size:]
	}
	return frames
}

// subtitleCoverage is the share of pixels in the caption area (bottom centre, where libass and
// make-pgs.py put the text, away from the moving patterns of testsrc2) that differ strongly
// between a burned-in frame and the source frame: about 0 without a caption.
func subtitleCoverage(burned, source []byte) float64 {
	n, diff := 0, 0
	for y := 150; y < 178; y++ {
		for x := 40; x < 280; x++ {
			d := int(burned[y*320+x]) - int(source[y*320+x])
			if d > 48 || d < -48 {
				diff++
			}
			n++
		}
	}
	return float64(diff) / float64(n)
}

// TestBurnInSurvivesRestart burns subtitles into a transcode that starts at segment 1 (ffmpeg with
// -ss 4 and -copyts) and checks frames at known times against the caption timing: a filter that
// saw shifted timestamps would show the captions 4 s early (text) or late (PGS).
func TestBurnInSurvivesRestart(t *testing.T) {
	r := ffmpegFull(t)
	backends := []Capabilities{{Backend: BackendSoftware, Tonemapper: TonemapCPU}}
	if b, caps, err := Detect(context.Background(), r, "nvenc", nil); err == nil && b == BackendNVENC {
		backends = append(backends, caps)
	} else {
		t.Log("no NVENC on this machine: software only")
	}
	text := testMovie(t, "Slouchcat Test Subtitles (2024)", "Slouchcat Test Subtitles (2024).mkv")
	pgs := testMovie(t, "Slouchcat Test PGS (2024)", "Slouchcat Test PGS (2024).mkv")
	// Captions: SRT 1-3, 4-6, 7-9.5 s; PGS 2-6, 8-12 s. Probes: segment, frame time, caption shown.
	type probe struct {
		seg  int
		at   float64
		want bool
	}
	cases := []struct {
		name   string
		path   string
		stream int
		start  int
		probes []probe
	}{
		{"srt", text, 2, 1, []probe{{1, 4.5, true}, {1, 5.5, true}, {1, 6.5, false}, {1, 7.5, true}, {2, 9.0, true}}},
		{"pgs", pgs, 2, 1, []probe{{1, 7.0, false}, {2, 9.0, true}, {2, 11.0, true}, {3, 12.5, false}, {3, 13.0, false}}},
		{"pgs-from-start", pgs, 2, 0, []probe{{0, 1.0, false}, {0, 3.0, true}}},
	}
	for _, caps := range backends {
		for _, c := range cases {
			t.Run(string(caps.Backend)+"/"+c.name, func(t *testing.T) {
				probe, err := r.Probe(context.Background(), c.path)
				if err != nil {
					t.Fatal(err)
				}
				src := Source{Path: c.path, Probe: probe}
				p, err := NewPlan(src, Target{Mode: ModeTranscode, Quality: "360p", SubtitleBurnIn: c.stream, AudioStream: -1})
				if err != nil {
					t.Fatal(err)
				}
				m, err := NewManager(ManagerOptions{Runner: r, WorkDir: filepath.Join(t.TempDir(), "transcode"), Caps: caps})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(m.Close)
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				s, err := m.Start(ctx, "burn", src, p, c.start)
				if err != nil {
					t.Fatal(err)
				}
				read := reader(t)
				init := read(s.OpenInit(ctx))
				segs := map[int][]byte{}
				for _, pr := range c.probes {
					if _, ok := segs[pr.seg]; !ok {
						segs[pr.seg] = read(s.OpenSegment(ctx, pr.seg))
					}
				}
				for _, pr := range c.probes {
					path := filepath.Join(t.TempDir(), "seg.mp4")
					if err := os.WriteFile(path, slices.Concat(init, segs[pr.seg]), 0o600); err != nil {
						t.Fatal(err)
					}
					out := grayFrames(t, r, path, -1)
					k := int((pr.at - p.Segments[pr.seg].Start.Seconds()) * 24)
					if k >= len(out) {
						t.Fatalf("segment %d has %d frames, want frame %d", pr.seg, len(out), k)
					}
					// The best match of the source frames around the time absorbs a frame of offset.
					cov := 1.0
					for _, ref := range grayFrames(t, r, c.path, pr.at) {
						cov = min(cov, subtitleCoverage(out[k], ref))
					}
					if got := cov > 0.025; got != pr.want {
						t.Errorf("at %.1f s (segment %d frame %d): caption %v (coverage %.4f), want %v", pr.at, pr.seg, k, got, cov, pr.want)
					} else {
						t.Logf("at %.1f s: coverage %.4f", pr.at, cov)
					}
				}
			})
		}
	}
}
