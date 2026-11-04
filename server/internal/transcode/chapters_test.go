package transcode

import (
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestCleanChapterTitle(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"00:12:36.172":      "",
		"00:23:29.283":      "",
		"00:23:29":          "",
		"1:05:00":           "",
		"12:36":             "",
		"00:12:36,172":      "",
		"Chapter 12":        "",
		"chapter 05":        "",
		"CHAPTER 1":         "",
		"Kapitel 12":        "",
		"kapitel 3":         "",
		" Chapter 7 ":       "",
		"Chapter 12 – Ende": "Chapter 12 – Ende",
		"The Escape":        "The Escape",
		"Kapitel":           "Kapitel",
		"12":                "12",
		"  Prolog ":         "Prolog",
		"Opening Credits":   "Opening Credits",
		"":                  "",
	} {
		if got := CleanChapterTitle(in); got != want {
			t.Errorf("CleanChapterTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func mins(m int) int64 { return int64(m) * 60_000 }

// chaptersProbe is a probe with the given chapter marks (minutes) and duration.
func chaptersProbe(duration time.Duration, marks ...Chapter) *Probe {
	return &Probe{Format: Format{Duration: duration}, Chapters: marks}
}

func chap(from, to int, title string) Chapter {
	return Chapter{Start: time.Duration(from) * time.Minute, End: time.Duration(to) * time.Minute, Title: title}
}

func auto(from, to int64) model.Chapter {
	return model.Chapter{StartMs: from, EndMs: to, Auto: true}
}

// grid returns auto marks every 10 minutes from 0 up to the given minute marks (exclusive ends).
func grid(bounds ...int) []model.Chapter {
	var out []model.Chapter
	for i := 0; i+1 < len(bounds); i++ {
		out = append(out, auto(mins(bounds[i]), mins(bounds[i+1])))
	}
	return out
}

func TestChapters(t *testing.T) {
	t.Parallel()
	four := []Chapter{chap(0, 10, "00:00:00.000"), chap(10, 20, "Chapter 2"), chap(20, 30, "Die Flucht"), chap(30, 40, "Kapitel 4")}
	cases := []struct {
		name       string
		probe      *Probe
		kind       string
		durationMs int64
		want       []model.Chapter
	}{
		{
			name: "titles cleaned, exactly four chapters keep their marks", kind: "movie", durationMs: mins(40),
			probe: chaptersProbe(40*time.Minute, four...),
			want: []model.Chapter{{StartMs: 0, EndMs: mins(10)}, {StartMs: mins(10), EndMs: mins(20)},
				{StartMs: mins(20), EndMs: mins(30), Title: "Die Flucht"}, {StartMs: mins(30), EndMs: mins(40)}},
		},
		{
			name: "episode cleaned like a film", kind: "episode", durationMs: mins(40),
			probe: chaptersProbe(40*time.Minute, four...),
			want: []model.Chapter{{StartMs: 0, EndMs: mins(10)}, {StartMs: mins(10), EndMs: mins(20)},
				{StartMs: mins(20), EndMs: mins(30), Title: "Die Flucht"}, {StartMs: mins(30), EndMs: mins(40)}},
		},
		{
			name: "audiobook untouched", kind: "audiobook", durationMs: mins(600),
			probe: chaptersProbe(600*time.Minute, chap(0, 300, "Chapter 1"), chap(300, 600, "00:05:00")),
			want:  []model.Chapter{{StartMs: 0, EndMs: mins(300), Title: "Chapter 1"}, {StartMs: mins(300), EndMs: mins(600), Title: "00:05:00"}},
		},
		{
			name: "podcast episode untouched", kind: "podcast_episode", durationMs: mins(60),
			probe: chaptersProbe(60*time.Minute, chap(0, 60, "Kapitel 1")),
			want:  []model.Chapter{{StartMs: 0, EndMs: mins(60), Title: "Kapitel 1"}},
		},
		{
			name: "extra untouched", kind: "extra", durationMs: mins(45),
			probe: chaptersProbe(45 * time.Minute),
			want:  []model.Chapter{},
		},
		{
			name: "under 30 minutes keeps few chapters", kind: "episode", durationMs: mins(29),
			probe: chaptersProbe(29*time.Minute, chap(0, 20, "Chapter 1"), chap(20, 29, "Abspann")),
			want:  []model.Chapter{{StartMs: 0, EndMs: mins(20)}, {StartMs: mins(20), EndMs: mins(29), Title: "Abspann"}},
		},
		{
			name: "under 30 minutes without chapters", kind: "episode", durationMs: mins(22),
			probe: chaptersProbe(22 * time.Minute),
			want:  []model.Chapter{},
		},
		{
			name: "no chapters: grid every 10 minutes", kind: "movie", durationMs: mins(45),
			probe: chaptersProbe(45 * time.Minute),
			want:  grid(0, 10, 20, 30, 40, 45),
		},
		{
			name: "three chapters are replaced", kind: "movie", durationMs: mins(60),
			probe: chaptersProbe(60*time.Minute, chap(0, 20, "Anfang"), chap(20, 40, "Mitte"), chap(40, 60, "Ende")),
			want:  grid(0, 10, 20, 30, 40, 50, 60),
		},
		{
			name: "last section under 3 minutes merges", kind: "movie", durationMs: mins(42),
			probe: chaptersProbe(42 * time.Minute),
			want:  grid(0, 10, 20, 30, 42),
		},
		{
			name: "last section of exactly 3 minutes stays", kind: "movie", durationMs: mins(33),
			probe: chaptersProbe(33 * time.Minute),
			want:  grid(0, 10, 20, 30, 33),
		},
		{
			name: "exactly 30 minutes", kind: "episode", durationMs: mins(30),
			probe: chaptersProbe(30*time.Minute, chap(0, 30, "Chapter 1")),
			want:  grid(0, 10, 20, 30),
		},
		{
			name: "the probe's duration wins over the file's", kind: "movie", durationMs: mins(20),
			probe: chaptersProbe(35 * time.Minute),
			want:  grid(0, 10, 20, 30, 35),
		},
		{
			name: "the file's duration when the probe has none", kind: "movie", durationMs: mins(31),
			probe: chaptersProbe(0),
			want:  grid(0, 10, 20, 31),
		},
		{
			name: "no probe, no chapters", kind: "movie", durationMs: mins(120),
			want: []model.Chapter{},
		},
		{
			name: "credits chapter kept, grid up to its start", kind: "movie", durationMs: mins(166),
			probe: chaptersProbe(166*time.Minute, chap(0, 158, "Chapter 1"), chap(158, 166, "Credits")),
			want: append(grid(0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 158),
				model.Chapter{StartMs: mins(158), EndMs: mins(166), Title: "Credits"}),
		},
		{
			name: "abspann kept, short scene after it merges into it", kind: "movie", durationMs: mins(100),
			probe: chaptersProbe(100*time.Minute, chap(0, 45, "Teil 1"), chap(45, 90, "Teil 2"), chap(90, 98, "Abspann")),
			want: append(grid(0, 10, 20, 30, 40, 50, 60, 70, 80, 90),
				model.Chapter{StartMs: mins(90), EndMs: mins(100), Title: "Abspann"}),
		},
		{
			name: "long scene after the credits gets its own grid", kind: "movie", durationMs: mins(115),
			probe: chaptersProbe(115*time.Minute, chap(0, 90, "Film"), chap(90, 98, "End Credits")),
			want: append(append(grid(0, 10, 20, 30, 40, 50, 60, 70, 80, 90),
				model.Chapter{StartMs: mins(90), EndMs: mins(98), Title: "End Credits"}),
				auto(mins(98), mins(108)), auto(mins(108), mins(115))),
		},
		{
			name: "opening credits are no credits mark", kind: "movie", durationMs: mins(40),
			probe: chaptersProbe(40*time.Minute, chap(0, 3, "Opening Credits"), chap(3, 40, "Film")),
			want:  grid(0, 10, 20, 30, 40),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Chapters(tc.probe, tc.kind, tc.durationMs)
			if got == nil || !slices.Equal(got, tc.want) {
				t.Errorf("Chapters() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}
