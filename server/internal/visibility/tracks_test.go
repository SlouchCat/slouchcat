package visibility

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// M4: tracks, segments and the next episode go through visible_items like every title read.
func TestReaderTracksAndSegments(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	tracks := f.AddTracks(t)
	f.AddSegments(t, f.Show.E1.ID)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(6)
	ft := tracks[m.FileID]

	list, err := r.Tracks(ctx, Profile(f.P6), m.ID, m.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if list.MediaFileID != m.FileID || len(list.Audio) != 2 || len(list.Subtitles) != 3 || list.Audio[0].ID != ft.AudioDefault || !list.Audio[0].Default {
		t.Fatalf("tracks = %+v", list)
	}
	for _, s := range list.Subtitles {
		wantText := s.ID != ft.Bitmap
		if s.Text != wantText || (s.ID == ft.Sidecar && (s.Index != -1 || s.RelPath != ft.SidecarRel)) {
			t.Errorf("subtitle %+v", s)
		}
	}
	for _, tc := range []struct {
		p          model.Profile
		item, file string
	}{
		{f.P0, m.ID, m.FileID},          // above the profile's rating
		{f.P6, m.ID, f.Rated(0).FileID}, // a file of another title
		{f.P6, m.ID, "01J0000000000000000000NONE"},
	} {
		if _, err := r.Tracks(ctx, Profile(tc.p), tc.item, tc.file); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s %s %s: %v", tc.p.Name, tc.item, tc.file, err)
		}
	}
	if _, err := r.Tracks(ctx, System("test"), m.ID, m.FileID); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v", err)
	}

	d, err := r.Item(ctx, Profile(f.P6), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.MediaFiles) != 1 || d.MediaFiles[0].Tracks == nil || len(d.MediaFiles[0].Tracks.Subtitles) != 3 {
		t.Errorf("detail files = %+v", d.MediaFiles)
	}

	segs, err := r.Segments(ctx, Profile(f.P12), f.Show.E1.ID)
	if err != nil || segs.Intro == nil || segs.Outro == nil || segs.Intro.StartMs != 5_000 {
		t.Errorf("segments = %+v, %v", segs, err)
	}
	if segs, err := r.Segments(ctx, Profile(f.P6), f.Show.E1.ID); err != nil || segs.Intro != nil {
		t.Errorf("segments of an invisible episode = %+v, %v", segs, err)
	}
	next, err := r.NextEpisode(ctx, Profile(f.P18), f.Show.E1.ID)
	if err != nil || next == nil || next.ID != f.Show.E2.ID {
		t.Errorf("next of E1 for P18 = %+v, %v", next, err)
	}
	if next, err := r.NextEpisode(ctx, Profile(f.P18), m.ID); err != nil || next != nil {
		t.Errorf("next of a film = %+v, %v", next, err)
	}
	if _, err := r.NextEpisode(ctx, Profile(f.P6), f.Show.E1.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("next of an invisible episode: %v", err)
	}
}
