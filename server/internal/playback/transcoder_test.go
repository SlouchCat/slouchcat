package playback

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The ffmpeg pipeline itself is tested in package transcode and end to end by web/e2e/hls.spec.ts;
// here: the error mapping and the admission error of the adapter.

func TestNotFoundIfEnded(t *testing.T) {
	for _, err := range []error{fmt.Errorf("%w: 99", transcode.ErrNoSegment), transcode.ErrClosed} {
		if got := notFoundIfEnded(err); !errors.Is(got, model.ErrNotFound) {
			t.Errorf("%v → %v, want ErrNotFound", err, got)
		}
	}
	if got := notFoundIfEnded(context.Canceled); !errors.Is(got, context.Canceled) || errors.Is(got, model.ErrNotFound) {
		t.Errorf("canceled → %v", got)
	}
	if notFoundIfEnded(nil) != nil {
		t.Error("nil error mapped")
	}
	if got := notFoundIfEnded(fmt.Errorf("%w: segment 3", transcode.ErrRestarted)); !errors.Is(got, model.ErrSegmentRestarted) || errors.Is(got, model.ErrNotFound) {
		t.Errorf("restarted → %v, want ErrSegmentRestarted", got)
	}
}

func TestSegmentAt(t *testing.T) {
	sec := func(n float64) time.Duration { return time.Duration(n * float64(time.Second)) }
	grid := []transcode.Segment{{Start: 0, Duration: sec(4)}, {Start: sec(4), Duration: sec(4)}, {Start: sec(8), Duration: sec(5)}}
	keyframes := []transcode.Segment{{Start: 0, Duration: sec(5.2)}, {Start: sec(5.2), Duration: sec(4.1)}, {Start: sec(9.3), Duration: sec(6)}}
	for _, tc := range []struct {
		segs []transcode.Segment
		pos  time.Duration
		want int
	}{
		{grid, 0, 0}, {grid, sec(3.999), 0}, {grid, sec(4), 1}, {grid, sec(7.5), 1}, {grid, sec(12.9), 2}, {grid, sec(99), 2},
		{keyframes, sec(5.1), 0}, {keyframes, sec(5.2), 1}, {keyframes, sec(9.29), 1}, {keyframes, sec(10), 2},
		{nil, sec(10), 0},
	} {
		if got := segmentAt(tc.segs, tc.pos); got != tc.want {
			t.Errorf("segmentAt(%v) = %d, want %d", tc.pos, got, tc.want)
		}
	}
}

func TestManagerTranscoderRejectsUnplannableSource(t *testing.T) {
	m, err := transcode.NewManager(transcode.ManagerOptions{
		Runner: transcode.Runner{FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, WorkDir: filepath.Join(t.TempDir(), "transcode"),
		Backend: transcode.BackendSoftware,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	tc := NewManagerTranscoder(m, transcode.BackendSoftware)
	if tc.Backend() != "software" {
		t.Errorf("backend = %q", tc.Backend())
	}
	// No duration in the probe: NewPlan fails before any ffmpeg starts.
	_, err = tc.Start(context.Background(), HLSRequest{ID: "01JAAAAAAAAAAAAAAAAAAAAAAA", Path: "/nonexistent.mkv",
		Probe: transcode.Probe{Streams: []transcode.Stream{{Type: "video", Codec: "hevc"}}}, Decision: Decision{Mode: ModeTranscode, MaxHeight: 1080}})
	if err == nil {
		t.Fatal("start without duration succeeded")
	}
	if _, ok := tc.Get("01JAAAAAAAAAAAAAAAAAAAAAAA"); ok {
		t.Error("failed start left a session")
	}
	tc.Stop("01JAAAAAAAAAAAAAAAAAAAAAAA") // unknown ids are ignored

	// A remux without keyframe index is refused before any ffmpeg starts.
	_, err = tc.Start(context.Background(), HLSRequest{ID: "01JAAAAAAAAAAAAAAAAAAAAAAB", Path: "/nonexistent.mkv",
		Probe:    transcode.Probe{Format: transcode.Format{Duration: 90 * time.Second}, Streams: []transcode.Stream{{Type: "video", Codec: "h264"}}},
		Decision: Decision{Mode: ModeRemux, VideoCopy: true}})
	if !errors.Is(err, ErrNoKeyframes) {
		t.Errorf("remux without index: %v, want ErrNoKeyframes", err)
	}
}
