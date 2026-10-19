package playback

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

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
}
