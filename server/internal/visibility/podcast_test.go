package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// M6: the audio titles follow the oracle of the fixture for every profile (explicit episode FSK 18,
// podcast without episodes invisible, unrated audiobook 18), the episode lists are paged newest
// first with their total, and an episode without images shows its podcast's cover.
func TestPodcastsAndAudioVisibility(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	f.AddAudio(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	a := f.Audio

	for _, p := range []model.Profile{f.P0, f.P6, f.P12, f.P16, f.P18, f.PX} {
		s := Profile(p)
		for _, m := range a.Titles() {
			_, err := r.Summary(ctx, s, m.ID)
			if want := f.VisibleTo(p, m); want != (err == nil) {
				t.Errorf("%s sees %s %s: %v, want %v", p.Name, m.Kind, m.Label, err, want)
			}
		}
		eps, total, err := r.PodcastEpisodes(ctx, s, a.Podcast.ID, 0, 0)
		if !f.VisibleTo(p, a.Podcast) {
			if !errors.Is(err, model.ErrNotFound) {
				t.Errorf("%s: episodes of the invisible podcast: %v", p.Name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		var want []string
		for _, m := range slices.Concat(a.Episodes, []vistest.Movie{a.Explicit, a.EnclosureOnly}) {
			if f.VisibleTo(p, m) {
				want = append(want, m.ID)
			}
		}
		slices.Reverse(want) // published one day apart in fixture order: newest first
		var got []string
		for _, e := range eps {
			got = append(got, e.ID)
		}
		if !slices.Equal(got, want) || total != len(want) {
			t.Errorf("%s: episodes %v (total %d), want %v", p.Name, got, total, want)
		}
		d, err := r.Item(ctx, s, a.Podcast.ID)
		if err != nil || len(d.Episodes) != len(want) || d.EpisodeCount == nil || *d.EpisodeCount != len(want) {
			t.Errorf("%s: podcast detail %d episodes, count %v, err %v", p.Name, len(d.Episodes), d.EpisodeCount, err)
		}
	}

	s := Profile(f.P18)
	// Paging and the clamps.
	page, total, err := r.PodcastEpisodes(ctx, s, a.Podcast.ID, 1, 2)
	if err != nil || total != 6 || len(page) != 2 || page[0].ID != a.Explicit.ID {
		t.Errorf("page 1..2 = %v total %d err %v", page, total, err)
	}
	if page, _, _ := r.PodcastEpisodes(ctx, s, a.Podcast.ID, -5, 1000); len(page) != 6 {
		t.Errorf("clamped page = %d episodes", len(page))
	}
	for _, id := range []string{a.Books[0].ID, a.Explicit.ID, a.Empty.ID, "01J0000000000000000000NONE"} {
		if _, _, err := r.PodcastEpisodes(ctx, s, id, 0, 10); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("episodes of %s: %v, want not found", id, err)
		}
	}
	// New episodes after a moment, newest first.
	first, err := r.Summary(ctx, s, a.Episodes[1].ID)
	if err != nil || first.PublishedAt == nil {
		t.Fatalf("episode summary %+v %v", first, err)
	}
	next, err := r.NextPodcastEpisodes(ctx, s, a.Podcast.ID, *first.PublishedAt, 2)
	if err != nil || len(next) != 2 || next[0].ID != a.EnclosureOnly.ID || next[1].ID != a.Explicit.ID {
		t.Errorf("next episodes = %v %v", next, err)
	}
	if next, _ := r.NextPodcastEpisodes(ctx, Profile(f.P16), a.Podcast.ID, *first.PublishedAt, 10); slices.ContainsFunc(next, func(m model.ItemSummary) bool { return m.ID == a.Explicit.ID }) {
		t.Error("P16 gets the explicit episode as new")
	}
	// The episode's detail names its podcast; the enclosure is readable for the visible episode only.
	d, err := r.Item(ctx, s, a.EnclosureOnly.ID)
	if err != nil || d.Podcast == nil || d.Podcast.ID != a.Podcast.ID || d.PublishedAt == nil || d.DurationMs == nil {
		t.Errorf("episode detail = %+v %v", d.Podcast, err)
	}
	enc, err := r.Enclosure(ctx, s, a.EnclosureOnly.ID)
	if err != nil || enc.URL != a.EnclosureOnly.Enclosure || enc.Type != vistest.EnclosureType || enc.DurationMs != vistest.DurationMs {
		t.Errorf("enclosure = %+v %v", enc, err)
	}
	if _, err := r.Enclosure(ctx, Profile(f.P16), a.Explicit.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("enclosure of the explicit episode for P16: %v", err)
	}
	if _, err := r.Enclosure(ctx, s, a.Books[0].ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("enclosure of an audiobook: %v", err)
	}
	// Images: the enclosure-only episode shows the podcast's poster, in lists and as a file.
	sum, err := r.Summary(ctx, s, a.EnclosureOnly.ID)
	pod, _ := r.Summary(ctx, s, a.Podcast.ID)
	if err != nil || sum.Poster == nil || pod.Poster == nil || sum.Poster.ContentHash != pod.Poster.ContentHash {
		t.Errorf("episode poster %+v, podcast poster %+v", sum.Poster, pod.Poster)
	}
	im, err := r.Image(ctx, s, a.EnclosureOnly.ID, "poster", "w400", false)
	if err != nil || im.RelPath != a.Podcast.PosterRel {
		t.Errorf("episode image = %+v %v, want %s", im, err, a.Podcast.PosterRel)
	}
	if im, err := r.Image(ctx, s, a.Episodes[0].ID, "poster", "w400", false); err != nil || im.RelPath != a.Episodes[0].PosterRel {
		t.Errorf("own episode image = %+v %v", im, err)
	}
	if _, err := r.Image(ctx, s, a.Books[0].ID, "poster", "w9999", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown size: %v", err)
	}
	// Recently added shows audiobooks and podcasts.
	recent, err := r.Recent(ctx, s, MaxRowItems)
	if err != nil || !slices.ContainsFunc(recent, func(m model.ItemSummary) bool { return m.ID == a.Podcast.ID }) ||
		!slices.ContainsFunc(recent, func(m model.ItemSummary) bool { return m.ID == a.Parts.ID }) {
		t.Errorf("recent lacks audio: %v", err)
	}
}
