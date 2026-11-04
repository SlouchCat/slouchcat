// Package trackpick chooses the audio and subtitle track of a playback or a download when the
// request names none (concept 13 section 11.3, F-3, F-4, F-11). The rule lives here once: package
// playback uses it for every start, package offline for downloads and auto-downloads, so both get
// the same language. The server decides, the client only sorts its panels (rule 12).
//
// The package is a leaf: it depends on model only and reads nothing. Callers hand in the tracks of
// the file (model.Track), the profile's preferences, the original language of the title and what
// the profile remembered for the series.
package trackpick

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Why an audio track plays (TrackSelection.audioReason).
const (
	AudioRequest  = "request"  // the request named it
	AudioSeries   = "series"   // the language the profile chose by hand for the series
	AudioProfile  = "profile"  // a language of the profile (audio_languages)
	AudioOriginal = "original" // the original language of the title
	AudioFile     = "file"     // the file's default track, else its first
)

// Why subtitles show, or none (TrackSelection.subtitleReason).
const (
	SubtitleRequest = "request" // the request named a track or switched subtitles off
	SubtitleSeries  = "series"  // the language (or "off") the profile chose by hand for the series
	SubtitleMode    = "mode"    // the profile's subtitle rule picked the track
	SubtitleOff     = "off"     // no subtitles: the rule picked none
	// SubtitleForcedBitmap: the rule would show a forced track that exists only as a bitmap, and the
	// playback is not transcoded anyway, so none shows (F-11).
	SubtitleForcedBitmap = "forced_bitmap_available"
)

// Remembered is what a profile last chose by hand for a series (profile_series_tracks). An empty
// language counts as none.
type Remembered struct {
	AudioLanguage    string
	SubtitleLanguage string
	SubtitleOff      bool
}

// Context is what the rules need besides the tracks: the profile's preferences, the original
// language of the title (of the series for an episode; "" unknown) and the choice remembered for
// the series of an episode (nil for none and for every other title).
type Context struct {
	Prefs            model.PlaybackPrefs
	OriginalLanguage string
	Series           *Remembered
}

// FileDefault returns the stream index of the file's default audio track: the first one flagged
// default, else the first one; -1 without audio. Tracks without a stream index (Index < 0) are
// skipped.
func FileDefault(tracks []model.Track) int {
	first := -1
	for _, t := range tracks {
		if t.Index < 0 {
			continue
		}
		if t.Default {
			return t.Index
		}
		if first < 0 {
			first = t.Index
		}
	}
	return first
}

// Audio returns the stream index of the audio track to play when the request names none, and why
// (concept 13 11.3):
//
//  1. series: the language the profile chose by hand for the series, if the file has it;
//  2. profile: the first language of audio_languages the file has;
//  3. original: the original language of the title;
//  4. file: the file's default track, else its first (FileDefault).
//
// Among several tracks of the language a track the device decodes comes first (decodes; nil
// counts every codec as decodable), then the default one, then more channels, then the file's
// order; commentary tracks come last. Tracks with "und" or no language match no language and only
// play in step 4. It returns -1 and AudioFile without audio.
func Audio(tracks []model.Track, c Context, decodes func(codec string) bool) (int, string) {
	if c.Series != nil {
		if i := audioIn(tracks, c.Series.AudioLanguage, decodes); i >= 0 {
			return i, AudioSeries
		}
	}
	for _, lang := range c.Prefs.AudioLanguages {
		if i := audioIn(tracks, lang, decodes); i >= 0 {
			return i, AudioProfile
		}
	}
	if i := audioIn(tracks, c.OriginalLanguage, decodes); i >= 0 {
		return i, AudioOriginal
	}
	return FileDefault(tracks), AudioFile
}

// audioIn returns the stream index of the best audio track in the language, or -1.
func audioIn(tracks []model.Track, language string, decodes func(codec string) bool) int {
	want := Normalize(language)
	if want == "" {
		return -1
	}
	var match []model.Track
	for _, t := range tracks {
		if t.Index >= 0 && Normalize(t.Language) == want {
			match = append(match, t)
		}
	}
	if len(match) == 0 {
		return -1
	}
	decodable := func(t model.Track) bool { return decodes == nil || decodes(strings.ToLower(t.Codec)) }
	// SortStableFunc keeps the file's order among equal tracks.
	slices.SortStableFunc(match, func(a, b model.Track) int {
		return cmp.Or(
			boolFirst(!commentary(a), !commentary(b)),
			boolFirst(decodable(a), decodable(b)),
			boolFirst(a.Default, b.Default),
			cmp.Compare(b.Channels, a.Channels),
		)
	})
	return match[0].Index
}

// commentaryRe matches the title of a commentary track.
var commentaryRe = regexp.MustCompile(`(?i)commentary|kommentar`)

func commentary(t model.Track) bool { return commentaryRe.MatchString(t.Title) }

// sdhRe matches the title of a subtitle track for the deaf and hard of hearing; the web client
// marks the same titles (isSdh in web/src/features/player/trackOrder.ts).
var sdhRe = regexp.MustCompile(`(?i)\b(sdh|cc)\b|hearing|hörgeschädigt|gehörlos`)

// HearingImpairedTitle reports whether a track title names subtitles for the deaf and hard of
// hearing ("SDH", "CC", "Hearing Impaired", "für Hörgeschädigte"); the scan stores it in
// tracks.is_hearing_impaired.
func HearingImpairedTitle(title string) bool { return sdhRe.MatchString(title) }

// hearingImpaired reports whether a subtitle track is SDH: the stored flag or its title.
func hearingImpaired(t model.Track) bool { return t.HearingImpaired || HearingImpairedTitle(t.Title) }

// boolFirst orders true before false.
func boolFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return -1
	}
	return 1
}

// Subtitle returns the subtitle track to show when the request names none and does not switch
// subtitles off, or nil, and why (concept 13 11.3). tracks are the subtitle tracks the playback can
// show (the caller leaves out what it can neither deliver as text nor burn in); audioLanguage is
// the language of the chosen audio track; transcoding reports whether the playback is transcoded
// anyway, the only case in which a forced bitmap track is chosen (F-11).
//
//  1. series: the language the profile chose by hand for the series (or "off"), if the file has a
//     track in it; in the language of the audio a forced track comes first.
//  2. the profile's rule (subtitle_mode, "forced" when empty):
//     off: none; forced: the forced track in the language of the audio; foreign: like forced, and
//     when the audio is in none of the profile's audio languages the full track in the first
//     subtitle language the file has; always: the full track in the first subtitle language the file
//     has, otherwise like forced.
//
// Text tracks come before bitmap tracks of the same language, then the default one, then the
// file's order. SDH tracks show only under "always" (and for a remembered language) when the
// language has no other full track. A forced track that exists only as a bitmap is chosen only when
// transcoding; otherwise none shows and the reason is SubtitleForcedBitmap.
func Subtitle(tracks []model.Track, audioLanguage string, c Context, transcoding bool) (*model.Track, string) {
	audio := Normalize(audioLanguage)
	if r := c.Series; r != nil {
		if r.SubtitleOff {
			return nil, SubtitleSeries
		}
		if lang := Normalize(r.SubtitleLanguage); lang != "" {
			if lang == audio {
				// A forced track that exists only as a bitmap stands for the language here too: no
				// full track instead of it (F-11, review step 9 m-1).
				switch t, bitmapOnly := forcedIn(tracks, lang, transcoding); {
				case t != nil:
					return t, SubtitleSeries
				case bitmapOnly:
					return nil, SubtitleForcedBitmap
				}
			}
			if t := fullIn(tracks, lang, true); t != nil {
				return t, SubtitleSeries
			}
		}
	}
	switch c.Prefs.SubtitleMode {
	case model.SubtitleModeOff:
		return nil, SubtitleOff
	case model.SubtitleModeForeign:
		if audio != "" && !slices.ContainsFunc(c.Prefs.AudioLanguages, func(l string) bool { return Normalize(l) == audio }) {
			if t := fullInFirst(tracks, c.Prefs.SubtitleLanguages, false); t != nil {
				return t, SubtitleMode
			}
		}
	case model.SubtitleModeAlways:
		if t := fullInFirst(tracks, c.Prefs.SubtitleLanguages, true); t != nil {
			return t, SubtitleMode
		}
	}
	t, bitmapOnly := forcedIn(tracks, audio, transcoding)
	switch {
	case t != nil:
		return t, SubtitleMode
	case bitmapOnly:
		return nil, SubtitleForcedBitmap
	}
	return nil, SubtitleOff
}

// forcedIn returns the forced track in the language (text before bitmap, then default, then
// order); a bitmap track only when transcoding, otherwise nil and bitmapOnly true.
func forcedIn(tracks []model.Track, language string, transcoding bool) (t *model.Track, bitmapOnly bool) {
	if language == "" {
		return nil, false
	}
	best := pick(tracks, func(t model.Track) bool { return t.Forced && Normalize(t.Language) == language })
	if best == nil || best.Text || transcoding {
		return best, false
	}
	return nil, true
}

// fullInFirst returns the full track in the first of the languages the file has one in.
func fullInFirst(tracks []model.Track, languages []string, sdh bool) *model.Track {
	for _, l := range languages {
		if t := fullIn(tracks, Normalize(l), sdh); t != nil {
			return t
		}
	}
	return nil
}

// fullIn returns the full (not forced) track in the language: a track that is not SDH first, an
// SDH track only when sdh is set and the language has no other.
func fullIn(tracks []model.Track, language string, sdh bool) *model.Track {
	if language == "" {
		return nil
	}
	full := func(t model.Track) bool { return !t.Forced && Normalize(t.Language) == language }
	if t := pick(tracks, func(t model.Track) bool { return full(t) && !hearingImpaired(t) }); t != nil || !sdh {
		return t
	}
	return pick(tracks, full)
}

// pick returns the best track that matches: text before bitmap, then the default one, then the
// file's order; nil when none matches.
func pick(tracks []model.Track, match func(model.Track) bool) *model.Track {
	var best *model.Track
	for i := range tracks {
		t := &tracks[i]
		if !match(*t) {
			continue
		}
		if best == nil || cmp.Or(boolFirst(t.Text, best.Text), boolFirst(t.Default, best.Default)) < 0 {
			best = t
		}
	}
	return best
}
