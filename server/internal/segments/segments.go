// Package segments finds the intro and the end credits (outro) of the episodes of a season (W-9,
// design M4 section 7). It is pure: the caller (job segments.detect in package library) computes
// the audio fingerprints with transcode.Runner.Fingerprint and stores the result.
//
// Two sources, chapter names first:
//
//   - Chapter: a chapter titled like an intro (intro, opening, vorspann, titles) or like the end
//     credits (credits, abspann, ending, outro, end titles) is taken as it is, source "chapter",
//     when its length is within the bounds of the fingerprint segments.
//   - Fingerprint: every pair of episodes is aligned on the Chromaprint values of the start of the
//     episodes (Options.HeadLength) and of their end (Options.TailRange). The shift with the most
//     values within MaxBitErrors bits of each other wins; at that shift (and its best neighbours)
//     the longest contiguous match of MinIntro..MaxIntro is an intro candidate of both episodes,
//     one of MinOutro..MaxOutro that ends at most OutroEndSlack before both file ends an outro
//     candidate. Each episode takes the candidate that most of its pairings agree on (median of the
//     agreeing starts and ends), if that is more than half of its pairings; its confidence is the
//     agreeing fraction.
//
// Similar to the approach of Jellyfin's intro-skipper plugin, without its silence detection.
package segments

import (
	"cmp"
	"math/bits"
	"regexp"
	"slices"
	"time"
)

// Step is the time between two fingerprint values: ffmpeg's chromaprint muxer resamples to
// 11025 Hz and Chromaprint hops 1365 samples (frames of 4096 with an overlap of two thirds).
// Value i of a fingerprint taken from time from describes the audio from from + i*Step on.
const Step = time.Second * 1365 / 11025

// Sources of a Span, the values of media_segments.source.
const (
	SourceFingerprint = "fingerprint"
	SourceChapter     = "chapter"
)

// Chapter is a chapter mark of an episode (transcode.Chapter).
type Chapter struct {
	Start time.Duration
	End   time.Duration
	Title string
}

// Episode is the input of Detect for one episode. Head is the fingerprint of the first
// Options.HeadLength(Duration), Tail the one of Options.TailRange(Duration); either may be empty
// (no audio, fingerprinting failed), then the episode takes part with its chapters only.
type Episode struct {
	ID       string
	Head     []uint32
	Tail     []uint32
	Duration time.Duration
	Chapters []Chapter
}

// Span is a detected segment, times from the start of the file.
type Span struct {
	Start  time.Duration
	End    time.Duration
	Source string // SourceFingerprint or SourceChapter
	// Confidence is the fraction of the pairings of the episode that agree on the span (1 for a
	// chapter).
	Confidence float64
}

// Result holds the segments of one episode; nil means not found.
type Result struct {
	Intro *Span
	Outro *Span
}

// Options are the parameters of the detection; DefaultOptions returns those of the design.
type Options struct {
	HeadMax      time.Duration // the head is the first min(HeadMax, HeadFraction × duration)
	HeadFraction float64
	TailLength   time.Duration // the tail is the last TailLength (the whole file when shorter)
	MinIntro     time.Duration
	MaxIntro     time.Duration
	MinOutro     time.Duration
	MaxOutro     time.Duration
	// OutroEndSlack is how far before the end of the file an outro may end.
	OutroEndSlack time.Duration
	// MaxBitErrors is the Hamming distance up to which two values match.
	MaxBitErrors int
	// MaxGap is the longest run of unmatched values inside one match.
	MaxGap time.Duration
	// Agreement is how far starts and ends of two candidates may differ to count as the same span.
	Agreement time.Duration
}

// DefaultOptions returns the parameters of the design (M4 section 7).
func DefaultOptions() Options {
	return Options{
		HeadMax: 10 * time.Minute, HeadFraction: 0.4, TailLength: 6 * time.Minute,
		MinIntro: 15 * time.Second, MaxIntro: 3 * time.Minute,
		MinOutro: 20 * time.Second, MaxOutro: 3 * time.Minute,
		OutroEndSlack: 5 * time.Second, MaxBitErrors: 6,
		MaxGap: 2 * time.Second, Agreement: 3 * time.Second,
	}
}

// HeadLength is the length of the fingerprint at the start of an episode of duration d.
func (o Options) HeadLength(d time.Duration) time.Duration {
	return min(o.HeadMax, time.Duration(o.HeadFraction*float64(d)))
}

// TailRange is the start and the length of the fingerprint at the end of an episode of duration d.
func (o Options) TailRange(d time.Duration) (from, length time.Duration) {
	from = max(0, d-o.TailLength)
	return from, d - from
}

// Detect returns the segments of the episodes by ID; episodes without any segment are left out.
func Detect(episodes []Episode, o Options) map[string]Result {
	n := len(episodes)
	intros := make([][]Span, n)
	outros := make([][]Span, n)
	headPairs := make([]int, n)
	tailPairs := make([]int, n)
	for i := range episodes {
		a, rest := &episodes[i], episodes[i+1:]
		for k := range rest {
			j, b := i+1+k, &rest[k]
			if len(a.Head) > 0 && len(b.Head) > 0 {
				headPairs[i]++
				headPairs[j]++
				if sa, sb, ok := pairIntro(a, b, o); ok {
					intros[i] = append(intros[i], sa)
					intros[j] = append(intros[j], sb)
				}
			}
			if len(a.Tail) > 0 && len(b.Tail) > 0 {
				tailPairs[i]++
				tailPairs[j]++
				if sa, sb, ok := pairOutro(a, b, o); ok {
					outros[i] = append(outros[i], sa)
					outros[j] = append(outros[j], sb)
				}
			}
		}
	}
	out := map[string]Result{}
	for i, e := range episodes {
		ci, co := chapterSpans(e.Chapters, o)
		r := Result{Intro: ci, Outro: co}
		if r.Intro == nil {
			r.Intro = majority(intros[i], headPairs[i], o.Agreement)
		}
		if r.Outro == nil {
			r.Outro = majority(outros[i], tailPairs[i], o.Agreement)
		}
		if r.Intro != nil || r.Outro != nil {
			out[e.ID] = r
		}
	}
	return out
}

// window is a fingerprint and the part of the file it describes.
type window struct {
	fp       []uint32
	from     time.Duration
	end      time.Duration // from + the fingerprinted length
	duration time.Duration // of the file
}

func head(e *Episode, o Options) window {
	return window{fp: e.Head, from: 0, end: o.HeadLength(e.Duration), duration: e.Duration}
}

func tail(e *Episode, o Options) window {
	from, length := o.TailRange(e.Duration)
	return window{fp: e.Tail, from: from, end: from + length, duration: e.Duration}
}

// span converts the run of n values from index first to times in the file. Chromaprint's last
// value describes the audio to the end of the window, so a run that reaches it (within
// edgeWindow values) ends there.
func (w window) span(first, n int) Span {
	start := w.from + time.Duration(first)*Step
	end := w.from + time.Duration(first+n)*Step
	if first+n >= len(w.fp)-edgeWindow {
		end = w.end
	}
	return Span{Start: start, End: min(end, w.duration), Source: SourceFingerprint}
}

// pairIntro returns the intro candidate of a and b: the longest match of their heads.
func pairIntro(a, b *Episode, o Options) (Span, Span, bool) {
	wa, wb := head(a, o), head(b, o)
	var best match
	for _, m := range matches(wa.fp, wb.fp, o) {
		if l := m.length(); l >= o.MinIntro && l <= o.MaxIntro && l > best.length() {
			best = m
		}
	}
	if best.n == 0 {
		return Span{}, Span{}, false
	}
	return wa.span(best.a0, best.n), wb.span(best.b0, best.n), true
}

// pairOutro returns the outro candidate of a and b: the longest match of their tails that ends
// near both file ends.
func pairOutro(a, b *Episode, o Options) (Span, Span, bool) {
	wa, wb := tail(a, o), tail(b, o)
	var best match
	var bestA, bestB Span
	for _, m := range matches(wa.fp, wb.fp, o) {
		l := m.length()
		if l < o.MinOutro || l > o.MaxOutro || l <= best.length() {
			continue
		}
		sa, sb := wa.span(m.a0, m.n), wb.span(m.b0, m.n)
		if a.Duration-sa.End > o.OutroEndSlack || b.Duration-sb.End > o.OutroEndSlack {
			continue
		}
		best, bestA, bestB = m, sa, sb
	}
	if best.n == 0 {
		return Span{}, Span{}, false
	}
	return bestA, bestB, true
}

// match is a run of matching values: a[a0..a0+n) against b[b0..b0+n).
type match struct{ a0, b0, n int }

func (m match) length() time.Duration { return time.Duration(m.n) * Step }

const (
	// candidateShifts is how many of the best shifts of a pair are searched for matches.
	candidateShifts = 3
	// A run starts and ends where at least edgeDense of edgeWindow values (about 1 s) match.
	edgeWindow = 8
	edgeDense  = 6
)

// matches returns the runs of matching values of a and b at the shifts with the most matching
// values (shift s pairs a[i] with b[i-s]).
func matches(a, b []uint32, o Options) []match {
	minVals := int(min(o.MinIntro, o.MinOutro) / Step)
	type scored struct{ shift, count int }
	var shifts []scored
	for s := -(len(b) - minVals); s <= len(a)-minVals; s++ {
		lo, hi := max(0, s), min(len(a), len(b)+s)
		c := 0
		for i := lo; i < hi; i++ {
			if bits.OnesCount32(a[i]^b[i-s]) <= o.MaxBitErrors {
				c++
			}
		}
		if c > 0 {
			shifts = append(shifts, scored{s, c})
		}
	}
	slices.SortFunc(shifts, func(x, y scored) int { return cmp.Or(cmp.Compare(y.count, x.count), cmp.Compare(x.shift, y.shift)) })
	var chosen []int
	for _, sc := range shifts {
		if len(chosen) == candidateShifts {
			break
		}
		near := slices.ContainsFunc(chosen, func(c int) bool { return abs(c-sc.shift) <= 2 })
		if !near {
			chosen = append(chosen, sc.shift)
		}
	}
	maxGap := int(o.MaxGap / Step)
	var out []match
	for _, s := range chosen {
		out = append(out, runs(a, b, s, maxGap, o.MaxBitErrors)...)
	}
	return out
}

// runs returns the contiguous matches of a and b at shift s, a value of b one step off counting
// too (the frame grids of two files differ by up to half a step), bridging up to maxGap unmatched
// values. Runs of hardly changing values (silence, a held tone) are dropped: they match anything
// alike and say nothing about the position.
func runs(a, b []uint32, s, maxGap, maxBits int) []match {
	lo, hi := max(0, s), min(len(a), len(b)+s)
	ok := make([]bool, len(a))
	for i := lo; i < hi; i++ {
		j := i - s
		for _, k := range [...]int{j, j - 1, j + 1} {
			if k >= 0 && k < len(b) && bits.OnesCount32(a[i]^b[k]) <= maxBits {
				ok[i] = true
				break
			}
		}
	}
	// dense reports whether most of the edgeWindow values from i on (dir 1) or up to i (dir -1)
	// match: the edges of a run are trimmed to dense matching, so stray matches next to the shared
	// block do not stretch it.
	dense := func(i, dir int) bool {
		c := 0
		for k := range edgeWindow {
			if p := i + dir*k; p >= lo && p < hi && ok[p] {
				c++
			}
		}
		return c >= edgeDense
	}
	var out []match
	start, last := -1, -1
	flush := func() {
		for start >= 0 && start < last && !dense(start, 1) {
			start++
		}
		for start >= 0 && last > start && !dense(last, -1) {
			last--
		}
		if start >= 0 && distinct(a[start:last+1])*10 >= last+1-start {
			out = append(out, match{a0: start, b0: start - s, n: last + 1 - start})
		}
		start = -1
	}
	for i := lo; i < hi; i++ {
		if !ok[i] {
			continue
		}
		if start >= 0 && i-last-1 > maxGap {
			flush()
		}
		if start < 0 {
			start = i
		}
		last = i
	}
	flush()
	return out
}

func distinct(v []uint32) int {
	seen := make(map[uint32]struct{}, len(v))
	for _, x := range v {
		seen[x] = struct{}{}
	}
	return len(seen)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// majority returns the span most candidates agree on (starts and ends within tol), as the median
// of the agreeing ones, if more than half of the pairings agree.
func majority(cands []Span, pairings int, tol time.Duration) *Span {
	if pairings == 0 {
		return nil
	}
	var best []Span
	for _, c := range cands {
		var agree []Span
		for _, d := range cands {
			if absDur(c.Start-d.Start) <= tol && absDur(c.End-d.End) <= tol {
				agree = append(agree, d)
			}
		}
		if len(agree) > len(best) {
			best = agree
		}
	}
	if 2*len(best) <= pairings {
		return nil
	}
	starts := make([]time.Duration, len(best))
	ends := make([]time.Duration, len(best))
	for i, s := range best {
		starts[i], ends[i] = s.Start, s.End
	}
	return &Span{
		Start: median(starts), End: median(ends), Source: SourceFingerprint,
		Confidence: float64(len(best)) / float64(pairings),
	}
}

func median(v []time.Duration) time.Duration {
	slices.Sort(v)
	if len(v)%2 == 1 {
		return v[len(v)/2]
	}
	return (v[len(v)/2-1] + v[len(v)/2]) / 2
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// Chapter titles (de/en, case-insensitive). An intro word wins over "credits" ("Opening
// Credits"); "titles" alone is an intro ("Main Titles"), "end titles" an outro.
var (
	introTitle  = regexp.MustCompile(`(?i)\b(intro|opening|vorspann)\b`)
	outroTitle  = regexp.MustCompile(`(?i)\b(credits|abspann|ending|outro|end titles)\b`)
	titlesTitle = regexp.MustCompile(`(?i)\btitles\b`)
)

// ChapterKind classifies a chapter title as "intro", "outro" or "" (neither).
func ChapterKind(title string) string {
	switch {
	case introTitle.MatchString(title):
		return "intro"
	case outroTitle.MatchString(title):
		return "outro"
	case titlesTitle.MatchString(title):
		return "intro"
	}
	return ""
}

// chapterSpans returns the first intro chapter and the last outro chapter whose length is within
// the bounds of the fingerprint segments (MinIntro..MaxIntro, MinOutro..MaxOutro): an "Opening"
// chapter that also covers the prologue is no intro to skip; the fingerprints decide then.
func chapterSpans(chapters []Chapter, o Options) (intro, outro *Span) {
	for _, c := range chapters {
		if c.End <= c.Start || c.Start < 0 {
			continue
		}
		s := &Span{Start: c.Start, End: c.End, Source: SourceChapter, Confidence: 1}
		l := c.End - c.Start
		switch ChapterKind(c.Title) {
		case "intro":
			if intro == nil && l >= o.MinIntro && l <= o.MaxIntro {
				intro = s
			}
		case "outro":
			if l >= o.MinOutro && l <= o.MaxOutro {
				outro = s
			}
		}
	}
	return intro, outro
}
