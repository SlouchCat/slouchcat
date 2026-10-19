package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

func TestHLSBenchUsage(t *testing.T) {
	for _, args := range [][]string{
		{"hlsbench"},
		{"hlsbench", "--file", "x.mkv", "--mode", "dash"},
		{"hlsbench", "--file", "x.mkv", "--streams", "0"},
		{"hlsbench", "--unknown"},
	} {
		if code := run(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
	if code := run([]string{"hlsbench", "--file", filepath.Join(t.TempDir(), "missing.mkv"), "--ffprobe", "/nonexistent/ffprobe"}); code != 1 {
		t.Errorf("missing file: exit %d, want 1", code)
	}
}

// TestHLSBenchRemux runs one short remux stream on the generated test media (task testdata:media).
func TestHLSBenchRemux(t *testing.T) {
	file, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Movies", "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Skip("test media missing (run `task testdata:media`)")
	}
	runner := transcode.Runner{FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}
	if _, err := runner.Version(context.Background()); err != nil {
		t.Skip("ffmpeg not installed")
	}
	rep, err := runHLSBench(context.Background(), benchOptions{file: file, streams: 2, mode: transcode.ModeRemux, hw: "software",
		maxHeight: 1080, seconds: 8 * time.Second, budget: 1000, runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 2 || rep.Backend != "copy" || rep.Cost != 0.5 {
		t.Fatalf("report %+v", rep)
	}
	for _, r := range rep.Results {
		if r.Error != "" || r.Segments < 2 || r.FirstSegMS <= 0 || r.ReadMBytes <= 0 {
			t.Errorf("stream %+v", r)
		}
	}
}
