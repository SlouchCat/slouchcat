package trackpick

import (
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Track layouts in the shape of real releases (stream order, languages, codecs, flags; no file
// names or tags): a UHD remux with English TrueHD first and a German commentary, a German release
// with the German track first, and a series episode with forced bitmap tracks only.
var (
	uhdAudio = []model.Track{
		{Index: 1, Codec: "truehd", Language: "eng", Title: "TrueHD Atmos 7.1", Default: true, Channels: 8},
		{Index: 2, Codec: "ac3", Language: "eng", Title: "AC3 5.1", Channels: 6},
		{Index: 3, Codec: "eac3", Language: "ger", Title: "Kommentar des Regisseurs", Channels: 2},
		{Index: 4, Codec: "dts", Language: "ger", Title: "DTS-HD MA 5.1", Channels: 6},
		{Index: 5, Codec: "ac3", Language: "ger", Title: "AC3 5.1", Channels: 6},
		{Index: 6, Codec: "aac", Language: "und", Channels: 2},
	}
	uhdSubs = []model.Track{
		{ID: 11, Index: 7, Codec: "hdmv_pgs_subtitle", Language: "eng"},
		{ID: 12, Index: 8, Codec: "hdmv_pgs_subtitle", Language: "ger", Forced: true},
		{ID: 13, Index: 9, Codec: "hdmv_pgs_subtitle", Language: "ger"},
		{ID: 14, Index: 10, Codec: "subrip", Language: "ger", Text: true, Forced: true},
		{ID: 15, Index: 11, Codec: "subrip", Language: "ger", Text: true, Title: "SDH"},
		{ID: 16, Index: 12, Codec: "subrip", Language: "eng", Text: true},
		{ID: 17, Index: -1, Codec: "subrip", Language: "en", Text: true, Source: "sidecar", HearingImpaired: true},
	}
	episodeSubs = []model.Track{
		{ID: 21, Index: 3, Codec: "hdmv_pgs_subtitle", Language: "ger", Forced: true},
		{ID: 22, Index: 4, Codec: "hdmv_pgs_subtitle", Language: "eng", Forced: true},
	}
)

func prefs(mode string, audio, subs []string) model.PlaybackPrefs {
	return model.PlaybackPrefs{AudioLanguages: audio, SubtitleMode: mode, SubtitleLanguages: subs}
}

func decodesOnly(codecs ...string) func(string) bool {
	return func(c string) bool { return slices.Contains(codecs, c) }
}

func TestAudio(t *testing.T) {
	browser := decodesOnly("aac", "ac3", "eac3")
	for _, tc := range []struct {
		name    string
		tracks  []model.Track
		c       Context
		decodes func(string) bool
		index   int
		reason  string
	}{
		{"series first", uhdAudio, Context{Prefs: prefs("", []string{"de"}, nil), Series: &Remembered{AudioLanguage: "en"}}, browser, 2, AudioSeries},
		{"series language missing falls to profile", uhdAudio, Context{Prefs: prefs("", []string{"de"}, nil), Series: &Remembered{AudioLanguage: "ja"}}, browser, 5, AudioProfile},
		{"series without language does not count", uhdAudio, Context{Prefs: prefs("", []string{"de"}, nil), Series: &Remembered{}}, browser, 5, AudioProfile},
		// German: DTS is not decodable, the commentary comes last, so AC3 5.1.
		{"profile decodable before default", uhdAudio, Context{Prefs: prefs("", []string{"de"}, nil)}, browser, 5, AudioProfile},
		// Every codec decodable: same channels, neither default, so the file's order (DTS).
		{"profile channels then order", uhdAudio, Context{Prefs: prefs("", []string{"de"}, nil)}, nil, 4, AudioProfile},
		{"profile second language", uhdAudio, Context{Prefs: prefs("", []string{"fr", "en"}, nil)}, browser, 2, AudioProfile},
		{"profile default among decodable", uhdAudio, Context{Prefs: prefs("", []string{"en"}, nil)}, nil, 1, AudioProfile},
		{"original", uhdAudio, Context{Prefs: prefs("", []string{"fr"}, nil), OriginalLanguage: "en"}, browser, 2, AudioOriginal},
		{"original in 639-2", uhdAudio, Context{OriginalLanguage: "ger"}, browser, 5, AudioOriginal},
		{"file default", uhdAudio, Context{Prefs: prefs("", []string{"fr"}, nil), OriginalLanguage: "it"}, browser, 1, AudioFile},
		{"und matches no language", uhdAudio, Context{Prefs: prefs("", []string{"und"}, nil), OriginalLanguage: "xx"}, browser, 1, AudioFile},
		{"file first without default", []model.Track{{Index: 2, Language: "fre"}, {Index: 3, Language: "spa"}}, Context{}, nil, 2, AudioFile},
		{"swiss german is german", []model.Track{{Index: 1, Language: "eng", Default: true}, {Index: 2, Language: "gsw"}}, Context{Prefs: prefs("", []string{"de"}, nil)}, nil, 2, AudioProfile},
		{"commentary only", []model.Track{{Index: 1, Language: "eng", Default: true}, {Index: 2, Language: "ger", Title: "Audio Commentary"}}, Context{Prefs: prefs("", []string{"de"}, nil)}, nil, 2, AudioProfile},
		{"no audio", nil, Context{Prefs: prefs("", []string{"de"}, nil)}, nil, -1, AudioFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index, reason := Audio(tc.tracks, tc.c, tc.decodes)
			if index != tc.index || reason != tc.reason {
				t.Errorf("Audio = %d %s, want %d %s", index, reason, tc.index, tc.reason)
			}
		})
	}
}

func TestFileDefault(t *testing.T) {
	if got := FileDefault([]model.Track{{Index: -1, Default: true}, {Index: 3}, {Index: 4, Default: true}}); got != 4 {
		t.Errorf("FileDefault = %d, want 4", got)
	}
	if got := FileDefault([]model.Track{{Index: 3}, {Index: 4}}); got != 3 {
		t.Errorf("FileDefault = %d, want 3", got)
	}
	if got := FileDefault(nil); got != -1 {
		t.Errorf("FileDefault = %d, want -1", got)
	}
}

func TestSubtitle(t *testing.T) {
	de := []string{"de"}
	for _, tc := range []struct {
		name        string
		tracks      []model.Track
		audio       string
		c           Context
		transcoding bool
		id          int64
		reason      string
	}{
		{"off", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeOff, de, de)}, false, 0, SubtitleOff},
		{"forced text in audio language", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, false, 14, SubtitleMode},
		{"empty mode is forced", uhdSubs, "de", Context{Prefs: prefs("", de, de)}, false, 14, SubtitleMode},
		{"forced none in audio language", uhdSubs, "eng", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, false, 0, SubtitleOff},
		{"forced unknown audio language", uhdSubs, "", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, false, 0, SubtitleOff},
		// F-11: the forced track exists only as a bitmap.
		{"forced bitmap without transcode", episodeSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, false, 0, SubtitleForcedBitmap},
		{"forced bitmap with transcode", episodeSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, true, 21, SubtitleMode},
		{"foreign audio in profile language", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForeign, de, de)}, false, 14, SubtitleMode},
		// English audio for a German profile: the full German track, text before bitmap, SDH never.
		{"foreign audio not in profile language", uhdSubs, "eng", Context{Prefs: prefs(model.SubtitleModeForeign, de, de)}, false, 13, SubtitleMode},
		{"foreign first subtitle language the file has", uhdSubs, "fre", Context{Prefs: prefs(model.SubtitleModeForeign, de, []string{"it", "en"})}, false, 16, SubtitleMode},
		{"foreign without full track like forced", episodeSubs, "eng", Context{Prefs: prefs(model.SubtitleModeForeign, de, de)}, false, 0, SubtitleForcedBitmap},
		{"foreign unknown audio is not foreign", uhdSubs, "und", Context{Prefs: prefs(model.SubtitleModeForeign, de, de)}, false, 0, SubtitleOff},
		{"always full track", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, []string{"en"})}, false, 16, SubtitleMode},
		{"always text before bitmap", uhdSubs, "eng", Context{Prefs: prefs(model.SubtitleModeAlways, de, de)}, false, 13, SubtitleMode},
		{"always without full track like forced", episodeSubs, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, []string{"fr"})}, true, 21, SubtitleMode},
		{"always sdh only without another", []model.Track{{ID: 31, Language: "eng", Text: true, Title: "English SDH"}}, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, []string{"en"})}, false, 31, SubtitleMode},
		{"always sdh flag only without another", []model.Track{{ID: 32, Language: "en", Text: true, HearingImpaired: true}, {ID: 33, Language: "en", Codec: "hdmv_pgs_subtitle"}}, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, []string{"en"})}, false, 33, SubtitleMode},
		{"foreign never sdh", []model.Track{{ID: 34, Language: "ger", Text: true, Title: "für Hörgeschädigte"}}, "eng", Context{Prefs: prefs(model.SubtitleModeForeign, de, de)}, false, 0, SubtitleOff},
		{"default among equal tracks", []model.Track{{ID: 35, Language: "en", Text: true}, {ID: 36, Language: "en", Text: true, Default: true}}, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, []string{"en"})}, false, 36, SubtitleMode},
		{"und matches no language", []model.Track{{ID: 37, Language: "und", Text: true, Forced: true}}, "und", Context{Prefs: prefs(model.SubtitleModeForced, de, de)}, false, 0, SubtitleOff},
		{"series off", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, de), Series: &Remembered{SubtitleOff: true}}, false, 0, SubtitleSeries},
		{"series full track", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeOff, de, de), Series: &Remembered{SubtitleLanguage: "en"}}, false, 16, SubtitleSeries},
		{"series in audio language forced first", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeOff, de, de), Series: &Remembered{SubtitleLanguage: "de"}}, false, 14, SubtitleSeries},
		{"series sdh when nothing else", []model.Track{{ID: 38, Language: "eng", Text: true, Title: "SDH"}}, "ger", Context{Series: &Remembered{SubtitleLanguage: "en"}}, false, 38, SubtitleSeries},
		{"series language missing falls to mode", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForced, de, de), Series: &Remembered{SubtitleLanguage: "ja"}}, false, 14, SubtitleMode},
		{"series empty language falls to mode", uhdSubs, "ger", Context{Prefs: prefs(model.SubtitleModeOff, de, de), Series: &Remembered{AudioLanguage: "de"}}, false, 0, SubtitleOff},
		{"series forced bitmap without transcode falls to mode", episodeSubs, "ger", Context{Prefs: prefs(model.SubtitleModeForced, de, de), Series: &Remembered{SubtitleLanguage: "de"}}, false, 0, SubtitleForcedBitmap},
		{"no tracks", nil, "ger", Context{Prefs: prefs(model.SubtitleModeAlways, de, de)}, true, 0, SubtitleOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := Subtitle(tc.tracks, tc.audio, tc.c, tc.transcoding)
			var id int64
			if got != nil {
				id = got.ID
			}
			if id != tc.id || reason != tc.reason {
				t.Errorf("Subtitle = %d %s, want %d %s", id, reason, tc.id, tc.reason)
			}
		})
	}
}

func TestHearingImpairedTitle(t *testing.T) {
	for title, want := range map[string]bool{
		"SDH": true, "English SDH": true, "CC": true, "Hearing Impaired": true, "für Hörgeschädigte": true,
		"Untertitel für Gehörlose": true, "Forced": false, "Deutsch": false, "": false, "Sdhx": false,
	} {
		if got := HearingImpairedTitle(title); got != want {
			t.Errorf("HearingImpairedTitle(%q) = %v, want %v", title, got, want)
		}
	}
}
