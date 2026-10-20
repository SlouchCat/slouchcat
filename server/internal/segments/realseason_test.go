package segments_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/segments"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// TestRealSeason runs the detection on a real season for the W-9 measurement
// (docs/perf/M4.md): SLOUCHCAT_REAL_SEASON names a directory with the episode files
// (*.mkv, *.mp4), optionally SLOUCHCAT_FFMPEG / SLOUCHCAT_FFPROBE the binaries. It prints the
// detected spans per file and only fails when ffmpeg fails; the comparison with hand-measured
// times is done by the reader. Without the variable the test is skipped (like the test-media
// tests without generated media).
func TestRealSeason(t *testing.T) {
	dir := os.Getenv("SLOUCHCAT_REAL_SEASON")
	if dir == "" {
		t.Skip("SLOUCHCAT_REAL_SEASON not set")
	}
	r := transcode.Runner{FFmpegPath: envOr("SLOUCHCAT_FFMPEG", "ffmpeg"), FFprobePath: envOr("SLOUCHCAT_FFPROBE", "ffprobe")}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if ext := strings.ToLower(filepath.Ext(e.Name())); !e.IsDir() && (ext == ".mkv" || ext == ".mp4") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) < 2 {
		t.Fatalf("need at least two episodes in %s, found %d", dir, len(names))
	}
	opts := segments.DefaultOptions()
	ctx := context.Background()
	var eps []segments.Episode
	for _, name := range names {
		path := filepath.Join(dir, name)
		started := time.Now()
		p, err := r.Probe(ctx, path)
		if err != nil {
			t.Fatalf("probe %s: %v", name, err)
		}
		ep := segments.Episode{ID: name, Duration: p.Format.Duration}
		for _, c := range p.Chapters {
			ep.Chapters = append(ep.Chapters, segments.Chapter{Start: c.Start, End: c.End, Title: c.Title})
		}
		if ep.Head, err = r.Fingerprint(ctx, path, 0, opts.HeadLength(ep.Duration)); err != nil {
			t.Fatalf("head fingerprint %s: %v", name, err)
		}
		from, length := opts.TailRange(ep.Duration)
		if ep.Tail, err = r.Fingerprint(ctx, path, from, length); err != nil {
			t.Fatalf("tail fingerprint %s: %v", name, err)
		}
		t.Logf("%s: duration %s, head %d values, tail %d values, chapters %d, fingerprinted in %s",
			name, ep.Duration.Round(time.Second), len(ep.Head), len(ep.Tail), len(ep.Chapters), time.Since(started).Round(100*time.Millisecond))
		eps = append(eps, ep)
	}
	started := time.Now()
	res := segments.Detect(eps, opts)
	t.Logf("detect: %s for %d episodes", time.Since(started).Round(time.Millisecond), len(eps))
	for _, ep := range eps {
		r := res[ep.ID]
		t.Logf("%s: intro %s, outro %s", ep.ID, span(r.Intro), span(r.Outro))
	}
}

func span(s *segments.Span) string {
	if s == nil {
		return "none"
	}
	return fmt.Sprintf("%s–%s (%s, %.2f)", s.Start.Round(100*time.Millisecond), s.End.Round(100*time.Millisecond), s.Source, s.Confidence)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
