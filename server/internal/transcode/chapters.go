package transcode

import (
	"regexp"
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
	// replacementMinSection is the shortest last section of a grid; a shorter one merges into the
	// section before it.
	replacementMinSection = 3 * time.Minute
)

// Titles that only number a chapter: a time ("00:12:36.172", "1:05:00", "12:36") or "Chapter 12" /
// "Kapitel 05" (case-insensitive). The client numbers such chapters by position (V-8).
var (
	timeTitle   = regexp.MustCompile(`^\d+(:\d{2}){1,2}([.,]\d+)?$`)
	numberTitle = regexp.MustCompile(`(?i)^(chapter|kapitel)\s*\d+$`)
)

// Chapters returns the chapters of a media file as playback and the download manifest send them,
// on the source timeline (ADR 0003). kind is the item kind of the file's title. The length of the
// timeline is the probe's duration, durationMs (the file's duration) only when the probe has none,
// so playback and a download of the same file agree. Empty, never nil; without a probe nothing is
// known about the file and the list stays empty.
//
// Audiobooks, podcast episodes and extras keep the marks of the probe unchanged. A film or an
// episode loses titles that only number the chapter (CleanChapterTitle). With fewer than four
// chapters and at least 30 minutes it gets replacement marks every 10 minutes instead (F-6,
// ReplacementChapters), Auto set.
func Chapters(p *Probe, kind string, durationMs int64) []model.Chapter {
	marks := p.ChapterMarks()
	if p == nil || (kind != "movie" && kind != "episode") {
		return marks
	}
	for i := range marks {
		marks[i].Title = CleanChapterTitle(marks[i].Title)
	}
	duration := p.Format.Duration
	if duration <= 0 {
		duration = time.Duration(durationMs) * time.Millisecond
	}
	if len(marks) >= replacementMinChapters || duration < replacementMinDuration {
		return marks
	}
	return ReplacementChapters(marks, duration)
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
// «Abspann», «End Titles», …) stays a mark with its title, start and end.
//
// The kept credits chapters split the timeline into gaps; each gap gets marks at its start and then
// every 10 minutes after its start, so the first gap starts at 0:00 and the panel reads «0:10,
// 0:20, …». The last section of a gap shorter than 3 minutes merges into the section before it; a
// gap that is shorter than 3 minutes as a whole merges into the credits chapter before it (a short
// post-credits scene) and stays its own mark only at the very start of the file.
func ReplacementChapters(real []model.Chapter, duration time.Duration) []model.Chapter {
	end := duration.Milliseconds()
	var credits []model.Chapter
	for _, c := range real {
		if segments.ChapterKind(c.Title) != "outro" || c.EndMs <= c.StartMs || c.StartMs < 0 || c.StartMs >= end {
			continue
		}
		if n := len(credits); n > 0 && c.StartMs < credits[n-1].EndMs {
			continue // overlaps the credits chapter before it
		}
		c.EndMs = min(c.EndMs, end)
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
	first := len(out)
	for start := from; start < to; start += grid {
		out = append(out, model.Chapter{StartMs: start, EndMs: min(start+grid, to), Auto: true})
	}
	if last := len(out) - 1; last > first && out[last].EndMs-out[last].StartMs < minSection {
		out[last-1].EndMs = out[last].EndMs
		out = out[:last]
	}
	return out
}
