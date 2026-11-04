package transcode

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/segments"
)

// Chapter rules of concept 13 (11.6, F-6, V-8, V-20) for films and episodes.
const (
	// ReplacementGrid is the distance of the replacement marks of a file with too few chapters.
	ReplacementGrid = 10 * time.Minute
	// replacementMinDuration is the shortest file that gets replacement marks.
	replacementMinDuration = 30 * time.Minute
	// replacementMinChapters is the number of real chapters from which a file keeps its own.
	replacementMinChapters = 4
	// replacementMinSection is the shortest section of a grid; a shorter one merges with a
	// neighbour.
	replacementMinSection = 3 * time.Minute
	// creditsMaxLength bounds a credits chapter that stays a mark among the replacement chapters:
	// plausible end credits are at most 15 minutes long (and start in the second half).
	creditsMaxLength = 15 * time.Minute
)

// Titles that only number a chapter: a time ("00:12:36.172", "1:05:00", "12:36") or "Chapter 12" /
// "Kapitel 05" (case-insensitive; also with a no-break space, "_", "." or "-" between word and
// number and a final dot: "Chapter-5", "Chapter 5."). The client numbers such chapters by position
// (V-8).
var (
	timeTitle   = regexp.MustCompile(`^\d+(:\d{2}){1,2}([.,]\d+)?$`)
	numberTitle = regexp.MustCompile(`(?i)^(chapter|kapitel)[\s\p{Zs}_.-]*\d+\.?$`)
)

// Chapters returns the chapters of a media file as playback and the download manifest send them,
// on the source timeline (ADR 0003). kind is the item kind of the file's title. The length of the
// timeline is the probe's duration, durationMs (the file's duration) only when the probe has none,
// so playback and a download of the same file agree. Empty, never nil; without a probe nothing is
// known about the file and the list stays empty.
//
// Audiobooks, podcast episodes and extras keep the marks of the probe unchanged. A film or an
// episode loses its zero-length marks and the titles that only number the chapter
// (CleanChapterTitle). With fewer than four chapters left and at least 30 minutes it gets
// replacement marks every 10 minutes instead (F-6, ReplacementChapters), Auto set.
func Chapters(p *Probe, kind string, durationMs int64) []model.Chapter {
	marks := p.ChapterMarks()
	if p == nil || (kind != "movie" && kind != "episode") {
		return marks
	}
	kept := marks[:0]
	for _, m := range marks {
		if m.EndMs > m.StartMs {
			m.Title = CleanChapterTitle(m.Title)
			kept = append(kept, m)
		}
	}
	duration := p.Format.Duration
	if duration <= 0 {
		duration = time.Duration(durationMs) * time.Millisecond
	}
	if len(kept) >= replacementMinChapters || duration < replacementMinDuration {
		return kept
	}
	return ReplacementChapters(kept, duration)
}

// CleanChapterTitle returns title without surrounding space, or "" when it only numbers the
// chapter (a time or "Chapter n" / "Kapitel n").
func CleanChapterTitle(title string) string {
	t := strings.TrimSpace(title)
	if timeTitle.MatchString(t) || numberTitle.MatchString(t) {
		return ""
	}
	return t
}

// ReplacementChapters replaces the chapters of a film of the given duration by marks every 10
// minutes (Auto, no title). A real credits chapter (segments.ChapterKind "outro": «Credits»,
// «Abspann», «End Titles», …) stays a mark with its title, start and end when it is plausible end
// credits: at most 15 minutes long and starting in the second half of the file. Any other chapter,
// «Happy Ending» over half the film or «Credits» at the start included, is replaced by the grid.
//
// The kept credits chapters (in start order, an overlapping one dropped) split the timeline into
// gaps. Each gap gets a mark at its start and at every full 10 minutes of the file inside it, so
// the panel reads «0:10, 0:20, …» also after a credits chapter. A section of a gap shorter than 3
// minutes merges with its neighbour in the gap: the last one into the section before it, any other
// one into the section after it. A gap that is shorter than 3 minutes as a whole merges into the
// credits chapter before it (a short scene after the credits) and stays its own mark only at the
// very start of the file.
func ReplacementChapters(real []model.Chapter, duration time.Duration) []model.Chapter {
	end := duration.Milliseconds()
	var candidates []model.Chapter
	for _, c := range real {
		if segments.ChapterKind(c.Title) != "outro" || c.EndMs <= c.StartMs || c.StartMs < 0 || c.StartMs >= end {
			continue
		}
		c.EndMs = min(c.EndMs, end)
		if c.EndMs-c.StartMs > creditsMaxLength.Milliseconds() || 2*c.StartMs < end {
			continue // no end credits: the grid replaces it
		}
		candidates = append(candidates, c)
	}
	slices.SortStableFunc(candidates, func(a, b model.Chapter) int { return cmp.Compare(a.StartMs, b.StartMs) })
	var credits []model.Chapter
	for _, c := range candidates {
		if n := len(credits); n > 0 && c.StartMs < credits[n-1].EndMs {
			continue // overlaps the credits chapter before it
		}
		credits = append(credits, c)
	}
	out := []model.Chapter{}
	from := int64(0)
	for _, c := range append(credits, model.Chapter{StartMs: end}) {
		out = gridGap(out, from, c.StartMs)
		if c.StartMs < end {
			out = append(out, c)
		}
		from = c.EndMs
	}
	return out
}

// gridGap appends the auto marks of the gap [from, to) to out (rules of ReplacementChapters).
func gridGap(out []model.Chapter, from, to int64) []model.Chapter {
	grid, minSection := ReplacementGrid.Milliseconds(), replacementMinSection.Milliseconds()
	if to <= from {
		return out
	}
	if to-from < minSection && len(out) > 0 {
		out[len(out)-1].EndMs = to
		return out
	}
	bounds := []int64{from}
	for m := (from/grid + 1) * grid; m < to; m += grid {
		bounds = append(bounds, m)
	}
	bounds = append(bounds, to)
	// Short sections merge: the last one into the one before it, any other into the one after it.
	for i := 0; i+1 < len(bounds) && len(bounds) > 2; {
		switch {
		case bounds[i+1]-bounds[i] >= minSection:
			i++
		case i+2 == len(bounds):
			bounds = slices.Delete(bounds, i, i+1) // the last section: drop its start
		default:
			bounds = slices.Delete(bounds, i+1, i+2) // drop the end of this section
		}
	}
	for i := 0; i+1 < len(bounds); i++ {
		out = append(out, model.Chapter{StartMs: bounds[i], EndMs: bounds[i+1], Auto: true})
	}
	return out
}
