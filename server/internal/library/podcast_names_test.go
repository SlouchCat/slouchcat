package library

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ADR 0029, decision 5: a stored episode is named by Audiobookshelf's sanitizeFilename. The
// expected names come from abs_sanitize in docs/perf/19-podcasts/m1.py (the reference of the
// measurement M-1), run on the same titles.
func TestAbsSanitizeMatchesTheReference(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ title, want string }{
		{"Folge 12: Der Anfang", "Folge 12 - Der Anfang.mp3"},
		{`AC/DC? <Live> "Best" *|*`, "ACDC Live Best .mp3"},
		{"Trailing dots... ", "Trailing dots... .mp3"},
		{"  Spaces   inside\tand\nbreaks  ", " Spaces insideandbreaks .mp3"},
		{"con", ""},
		{"Ärger über Öl", "Ärger über Öl.mp3"},
		{"Zürich: Teil 1: Teil 2", "Zürich - Teil 1 Teil 2.mp3"},
		{"NFD: Café", "NFD - Café.mp3"},
		{strings.Repeat("x", 300), strings.Repeat("x", 123) + ".mp3"},
		{"Emoji 😀 " + strings.Repeat("ü", 130), "Emoji 😀 " + strings.Repeat("ü", 114) + ".mp3"},
		{strings.Repeat("日本語", 90), strings.Repeat("日本語", 41) + ".mp3"},
	} {
		if got := absSanitize(tc.title + ".mp3"); got != tc.want {
			t.Errorf("absSanitize(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

// The name of a stored episode: Audiobookshelf's rule, at most 250 bytes in UTF-8 as well (at a
// character boundary), no leading dot or space, "Folge <date>" when nothing is left; a name that is
// taken gets the publication date in front, then " (2)", " (3)" at the stem, within both bounds.
func TestPodcastFileName(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)
	long := strings.Repeat("日本語", 90)
	for _, tc := range []struct{ title, ext, want string }{
		{"Folge 12: Der Anfang", "mp3", "Folge 12 - Der Anfang.mp3"},
		{"  Spaces   inside  ", "m4a", "Spaces inside .m4a"},
		{"...versteckt", "mp3", "versteckt.mp3"},
		{"con", "mp3", "Folge 2026-11-09.mp3"},
		{"", "mp3", "Folge 2026-11-09.mp3"},
		{`/\:*?"<>|`, "ogg", "Folge 2026-11-09.ogg"},
		{"Folge 7 . ", "mp3", "Folge 7 . .mp3"},
		{"...", "mp3", "Folge 2026-11-09.mp3"},
		{long, "mp3", strings.Repeat("日本語", 27) + "日" + ".mp3"},
	} {
		got := podcastFileName(tc.title, tc.ext, day)
		if got != tc.want {
			t.Errorf("podcastFileName(%q, %q) = %q, want %q", tc.title, tc.ext, got, tc.want)
		}
		if len(got) > maxNameUTF8 || !utf8.ValidString(got) {
			t.Errorf("podcastFileName(%q) = %d bytes or invalid UTF-8", tc.title, len(got))
		}
	}
	name := podcastFileName("Folge 12: Der Anfang", "mp3", day)
	for n, want := range []string{"Folge 12 - Der Anfang.mp3", "2026-11-09 Folge 12 - Der Anfang.mp3",
		"2026-11-09 Folge 12 - Der Anfang (2).mp3", "2026-11-09 Folge 12 - Der Anfang (3).mp3"} {
		if got := podcastNameVariant(name, day, n); got != want {
			t.Errorf("variant %d = %q, want %q", n, got, want)
		}
	}
	longName := podcastFileName(long, "mp3", day)
	for n := range 3 {
		if got := podcastNameVariant(longName, day, n); len(got) > maxNameUTF8 || !utf8.ValidString(got) || !strings.HasSuffix(got, ".mp3") {
			t.Errorf("variant %d of a long name = %q (%d bytes)", n, got, len(got))
		}
	}
}

// ADR 0029, decision 6: the key is NFC, lower case, letters and digits only (keys from key() in
// m1.py); a file's key leaves out Audiobookshelf's " (<uuid>)" suffix, and a feed episode has the
// key of its title and of its shortened name.
func TestPodcastKeys(t *testing.T) {
	t.Parallel()
	if got := podcastKey("Folge 12: Der Anfang"); got != "folge12deranfang" {
		t.Errorf("key = %q", got)
	}
	if got := podcastKey("Café – Ärger ²"); got != "caféärger²" {
		t.Errorf("key of a decomposed title = %q", got)
	}
	if got := podcastFileKey("Pod/Folge 12 - Der Anfang (0f8fad5b-d9cb-469f-a165-70867728950e).mp3"); got != "folge12deranfang" {
		t.Errorf("file key with a uuid suffix = %q", got)
	}
	if got := podcastFileKey("Pod/Folge 12 - Der Anfang (2).mp3"); got != "folge12deranfang2" {
		t.Errorf("file key with a number = %q", got)
	}
	long := strings.Repeat("x", 300)
	keys := podcastEpisodeKeys(long, "https://cdn.example.test/ep.mp3?x=1")
	if len(keys) != 2 || keys[0] != long || keys[1] != strings.Repeat("x", 123) {
		t.Errorf("keys of a long title = %v", keys)
	}
	if keys := podcastEpisodeKeys("Folge 1", "https://cdn.example.test/ep"); len(keys) != 1 || keys[0] != "folge1" {
		t.Errorf("keys of a short title = %v", keys)
	}
}

// The extension (decision 5) and the formats the planner leaves out (O-3).
func TestPodcastExtAndFormat(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ url, typ, format, codec, want string }{
		{"https://x.test/a/ep.MP3?k=1", "audio/mp4", "", "", "mp3"},
		{"https://x.test/ep", "audio/x-m4a", "", "", "m4a"},
		{"https://x.test/ep.php", "application/octet-stream", "mov,mp4,m4a,3gp,3g2,mj2", "aac", "m4a"},
		{"https://x.test/ep", "", "ogg", "opus", "opus"},
		{"https://x.test/ep.mp4", "video/mp4", "mov,mp4,m4a,3gp,3g2,mj2", "h264", "m4a"},
		{"https://x.test/ep", "text/html; charset=utf-8", "html", "", ""},
	} {
		if got := podcastExt(tc.url, tc.typ, tc.format, tc.codec); got != tc.want {
			t.Errorf("podcastExt(%q, %q, %q) = %q, want %q", tc.url, tc.typ, tc.format, got, tc.want)
		}
	}
	for _, tc := range []struct {
		url, typ string
		want     bool
	}{
		{"https://x.test/ep.mp3", "", true}, {"https://x.test/ep.mp4", "video/mp4", false}, {"https://x.test/ep.wav", "", false},
		{"https://x.test/ep", "audio/mpeg", true}, {"https://x.test/ep", "video/mp4", false}, {"https://x.test/ep", "", true},
		{"https://x.test/ep.php", "audio/mpeg", true},
	} {
		if got := podcastFormatKnown(tc.url, tc.typ); got != tc.want {
			t.Errorf("podcastFormatKnown(%q, %q) = %v", tc.url, tc.typ, got)
		}
	}
}
