package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// ADR 0015: a podcast episode with a file of its own plays from the file like an audiobook, the
// enclosure proxy is only the fallback. The fixture's episodes carry both, the file-only episode
// of a folder has no enclosure at all.
func TestPodcastEpisodeWithFilePlaysFromTheFile(t *testing.T) {
	t.Parallel()
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

// Concept 23 F-5: a podcast episode carries its older and its newer visible neighbour in the same
// podcast, whether it plays from its file or from its enclosure (startEnclosure).
func TestPodcastEpisodeNeighboursOnStart(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddAudio(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	a := f.Audio
	q0, q6, qc := a.Episodes[0], a.Episodes[1], a.Episodes[2]
	// The episode without a file moves between Q0 (2 October) and Q6 (3 October).
	between := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC).UnixMilli()
	f.SetPublishedAt(t, a.EnclosureOnly.ID, between)
	session := browserSession(t, f, f.P12)
	for _, tc := range []struct {
		name           string
		ep             vistest.Movie
		file           string
		next, previous string
	}{
		{"enclosure", a.EnclosureOnly, "", q6.ID, q0.ID},
		{"file", q6, q6.FileID, qc.ID, a.EnclosureOnly.ID},
		{"oldest visible", q0, q0.FileID, a.EnclosureOnly.ID, ""},
	} {
		info, err := s.Start(ctx, session, tc.ep.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if info.MediaFile.ID != tc.file {
			t.Errorf("%s: media file %q, want %q", tc.name, info.MediaFile.ID, tc.file)
		}
		var next, prev string
		if info.NextEpisode != nil {
			next = info.NextEpisode.ID
		}
		if info.PreviousEpisode != nil {
			prev = info.PreviousEpisode.ID
		}
		if next != tc.next || prev != tc.previous {
			t.Errorf("%s: next %q previous %q, want %q %q", tc.name, next, prev, tc.next, tc.previous)
		}
	}
}
