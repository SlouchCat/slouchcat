package playback

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// ADR 0015: a podcast episode with a file of its own plays from the file like an audiobook, the
// enclosure proxy is only the fallback. The fixture's episodes carry both, the file-only episode
// of a folder has no enclosure at all.
func TestPodcastEpisodeWithFilePlaysFromTheFile(t *testing.T) {
	f := vistest.New(t)
	f.AddAudio(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	session := browserSession(t, f, f.P18)
	for _, ep := range []vistest.Movie{f.Audio.Episodes[0], f.Audio.FileOnlyExplicit} {
		info, err := s.Start(ctx, session, ep.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatalf("%s: %v", ep.Label, err)
		}
		if info.MediaKind != model.MediaKindAudio || info.Mode != ModeDirect || info.MediaFile.ID != ep.FileID {
			t.Fatalf("%s: info = %+v, want direct playback of file %s", ep.Label, info, ep.FileID)
		}
		g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
		if err != nil || g.MediaFileID != ep.FileID {
			t.Fatalf("%s: grant = %+v %v", ep.Label, g, err)
		}
	}
	// The explicit file episode is invisible below 18: no playback, never the enclosure path.
	if _, err := s.Start(ctx, browserSession(t, f, f.P12), f.Audio.FileOnlyExplicit.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("explicit file episode for P12: %v, want not found", err)
	}
}
