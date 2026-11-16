package model

import "testing"

// minute is the start of a minute in Unix milliseconds, n minutes after a fixed one.
func minute(n int64) int64 { return (29_333_334 + n) * 60_000 }

func TestFitPodcastEpisodes(t *testing.T) {
	t.Parallel()
	feed := PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "godzilla", DurationMs: 581_000}
	cases := []struct {
		name string
		file PodcastEpisodeTraits
		want PodcastPairFit
	}{
		// The measured case of 16 November 2026: the feed says 581 s, the file runs 548.5 s (5.6 %).
		{"same minute and title, duration off", PodcastEpisodeTraits{PublishedAt: minute(0) + 59_000, Dated: true, SortTitle: "godzilla", DurationMs: 548_544},
			PodcastPairFit{Grade: PodcastPairStrong}},
		{"same minute and title, duration agrees", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "godzilla", DurationMs: 580_100},
			PodcastPairFit{Grade: PodcastPairStrong, DurationsAgree: true}},
		{"same minute and title, duration unknown", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "godzilla"},
			PodcastPairFit{Grade: PodcastPairStrong}},
		{"only the title, duration off", PodcastEpisodeTraits{PublishedAt: minute(90), Dated: true, SortTitle: "godzilla", DurationMs: 548_544},
			PodcastPairFit{}},
		{"only the title, duration agrees", PodcastEpisodeTraits{PublishedAt: minute(90), Dated: true, SortTitle: "godzilla", DurationMs: 590_000},
			PodcastPairFit{Grade: PodcastPairWeak, DurationsAgree: true}},
		{"only the title, duration unknown", PodcastEpisodeTraits{SortTitle: "godzilla"},
			PodcastPairFit{Grade: PodcastPairWeak}},
		{"only the minute, duration off", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "mothra", DurationMs: 548_544},
			PodcastPairFit{}},
		{"only the minute, duration agrees", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "mothra", DurationMs: 581_900},
			PodcastPairFit{Grade: PodcastPairWeak, DurationsAgree: true}},
		{"two percent is the limit", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "mothra", DurationMs: 581_000 + 11_620},
			PodcastPairFit{Grade: PodcastPairWeak, DurationsAgree: true}},
		{"just over two percent", PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true, SortTitle: "mothra", DurationMs: 581_000 + 11_621},
			PodcastPairFit{}},
		{"nothing shared", PodcastEpisodeTraits{PublishedAt: minute(5), Dated: true, SortTitle: "mothra", DurationMs: 581_000},
			PodcastPairFit{DurationsAgree: true}},
		{"undated on one side, other title", PodcastEpisodeTraits{SortTitle: "mothra", DurationMs: 581_000},
			PodcastPairFit{DurationsAgree: true}},
	}
	for _, c := range cases {
		if got := FitPodcastEpisodes(feed, c.file); got != c.want {
			t.Errorf("%s: fit = %+v, want %+v", c.name, got, c.want)
		}
	}
	// Two episodes without a title never share one, and an undated feed episode shares no minute.
	blank := PodcastEpisodeTraits{PublishedAt: minute(0), Dated: false}
	if got := FitPodcastEpisodes(blank, PodcastEpisodeTraits{PublishedAt: minute(0), Dated: true}); got.Grade != PodcastPairNone {
		t.Errorf("no title and no date on the feed side: grade = %d, want none", got.Grade)
	}
}

func TestPickPodcastPair(t *testing.T) {
	t.Parallel()
	strong, strongAgrees := PodcastPairFit{Grade: PodcastPairStrong}, PodcastPairFit{Grade: PodcastPairStrong, DurationsAgree: true}
	weak, weakAgrees := PodcastPairFit{Grade: PodcastPairWeak}, PodcastPairFit{Grade: PodcastPairWeak, DurationsAgree: true}
	none := PodcastPairFit{}
	cases := []struct {
		name  string
		fits  []PodcastPairFit
		index int
		ok    bool
	}{
		{"no candidate", nil, -1, false},
		{"none fits", []PodcastPairFit{none, none}, -1, false},
		{"one strong, duration off", []PodcastPairFit{none, strong}, 1, true},
		{"one weak", []PodcastPairFit{weakAgrees, none}, 0, true},
		// A repeat of the episode: the same title on another day, the same audio.
		{"a strong one beats a weak one", []PodcastPairFit{weakAgrees, strong}, 1, true},
		{"two weak stay unclear", []PodcastPairFit{weakAgrees, weak}, -1, false},
		{"two weak that agree stay unclear", []PodcastPairFit{weakAgrees, weakAgrees}, -1, false},
		{"two strong: the duration decides", []PodcastPairFit{strong, strongAgrees, weakAgrees}, 1, true},
		{"two strong, both durations agree", []PodcastPairFit{strongAgrees, strongAgrees}, -1, false},
		{"two strong, no duration agrees", []PodcastPairFit{strong, strong}, -1, false},
	}
	for _, c := range cases {
		if index, ok := PickPodcastPair(c.fits); index != c.index || ok != c.ok {
			t.Errorf("%s: pick = %d, %v, want %d, %v", c.name, index, ok, c.index, c.ok)
		}
	}
}
