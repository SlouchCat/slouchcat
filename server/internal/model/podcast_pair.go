package model

// The pairing rule between a feed episode and a file episode of one podcast (ADR 0015, concept
// 4.5; changed by the decision of 16 November 2026, addendum to ADR 0029). The scanner's probe step
// (library.finishPodcastEpisode), a feed fetch that meets a new episode (metadata, adopt), the
// pairing after every feed check (library.PairPodcastEpisodes) and riegel 2 of the planner all
// read it here.
//
// Two episodes with the same publication minute and the same sort title are the same episode,
// whatever their durations say: the duration a feed states (itunes:duration, rounded to seconds)
// is often not the one of the file a provider hands out (inserted advertising: 2 to 14 percent off,
// measured on 16 November 2026). Where only one of the two agrees, the durations must not
// contradict: known on both sides, they agree within two percent of the feed's.

// PodcastPairGrade says how well a feed episode and a file episode fit.
type PodcastPairGrade int

const (
	// PodcastPairNone: neither the minute nor the title agrees, or only one of them does and the
	// durations contradict.
	PodcastPairNone PodcastPairGrade = iota
	// PodcastPairWeak: the publication minute or the sort title agrees, and the durations do not
	// contradict (within two percent, or unknown on a side).
	PodcastPairWeak
	// PodcastPairStrong: the publication minute and the sort title agree.
	PodcastPairStrong
)

// PodcastEpisodeTraits is what the pairing rule reads of an episode. Dated is false for an episode
// without a publication date, DurationMs is 0 for an unknown duration, SortTitle is "" for none.
type PodcastEpisodeTraits struct {
	PublishedAt int64 // Unix milliseconds
	Dated       bool
	SortTitle   string
	DurationMs  int64
}

// PodcastPairFit is the outcome of the rule for one feed episode and one file episode.
type PodcastPairFit struct {
	Grade PodcastPairGrade
	// DurationsAgree: both durations are known and agree within two percent of the feed's. It
	// decides between several strong candidates (PickPodcastPair).
	DurationsAgree bool
}

// FitPodcastEpisodes applies the pairing rule to a feed episode and a file episode of one podcast.
func FitPodcastEpisodes(feed, file PodcastEpisodeTraits) PodcastPairFit {
	sameMinute := feed.Dated && file.Dated && feed.PublishedAt/60000 == file.PublishedAt/60000
	sameTitle := feed.SortTitle != "" && feed.SortTitle == file.SortTitle
	known := feed.DurationMs > 0 && file.DurationMs > 0
	agree := false
	if known {
		diff := feed.DurationMs - file.DurationMs
		if diff < 0 {
			diff = -diff
		}
		agree = diff*50 <= feed.DurationMs
	}
	fit := PodcastPairFit{DurationsAgree: agree}
	switch {
	case sameMinute && sameTitle:
		fit.Grade = PodcastPairStrong
	case (sameMinute || sameTitle) && (!known || agree):
		fit.Grade = PodcastPairWeak
	}
	return fit
}

// PickPodcastPair chooses among the candidates of one episode (the feed episodes a file may stand
// for, or the files a feed episode may own) the one it is paired with, and returns its index; ok
// is false when there is none or the choice is not clear. A strong candidate beats every weak one
// (a repeat of an episode shares its title, not its minute). Between several strong candidates the
// duration decides: exactly one whose duration agrees wins. Several weak candidates, or several
// strong ones the duration does not tell apart, stay unclear: two episodes that stay apart are
// better than a file married to the wrong one.
func PickPodcastPair(fits []PodcastPairFit) (index int, ok bool) {
	only := func(match func(PodcastPairFit) bool) (int, int) {
		found, n := -1, 0
		for i, f := range fits {
			if match(f) {
				found, n = i, n+1
			}
		}
		return found, n
	}
	strong := func(f PodcastPairFit) bool { return f.Grade == PodcastPairStrong }
	i, n := only(strong)
	if n > 1 {
		i, n = only(func(f PodcastPairFit) bool { return strong(f) && f.DurationsAgree })
		if n != 1 {
			return -1, false
		}
	}
	if n == 0 {
		i, n = only(func(f PodcastPairFit) bool { return f.Grade == PodcastPairWeak })
	}
	if n != 1 {
		return -1, false
	}
	return i, true
}
