package feed_test

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/feed"
)

// testFeed reads testdata/podcasts/feed.xml.tmpl with {{BASE}} replaced.
func testFeed(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../testdata/podcasts/feed.xml.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(raw), "{{BASE}}", "http://feeds.test")
}

func TestParseTestFeed(t *testing.T) {
	f, err := feed.Parse(strings.NewReader(testFeed(t)))
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Slouchcat Test Podcast" || f.Author != "Slouchcat Test Author" || f.Language != "de" ||
		f.ImageURL != "http://feeds.test/cover.jpg" || f.Explicit {
		t.Fatalf("channel = %+v", f)
	}
	if !strings.HasPrefix(f.Description, "Ein Test-Podcast aus Sinustönen") {
		t.Errorf("description = %q", f.Description)
	}
	if len(f.Episodes) != 3 {
		t.Fatalf("episodes = %d", len(f.Episodes))
	}
	e1, e3 := f.Episodes[0], f.Episodes[2]
	if e1.GUID != "slouchcat-test-episode-1" || e1.Title != "Slouchcat Test Episode 1" || e1.Number != 1 ||
		e1.Duration != 8*time.Second || e1.Enclosure.URL != "http://feeds.test/episode-1.mp3" ||
		e1.Enclosure.Type != "audio/mpeg" || e1.Enclosure.Length != 0 ||
		!e1.PublishedAt.Equal(time.Date(2025, 10, 6, 6, 0, 0, 0, time.UTC)) {
		t.Errorf("episode 1 = %+v", e1)
	}
	if e1.Explicit == nil || *e1.Explicit {
		t.Errorf("episode 1 explicit = %v", e1.Explicit)
	}
	if e3.Explicit == nil || !*e3.Explicit || e3.Duration != 12*time.Second {
		t.Errorf("episode 3 = %+v", e3)
	}
}

const rssHead = `<?xml version="1.0"?><rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel><title>T</title>`

func TestParseEdgeCases(t *testing.T) {
	src := rssHead + `<itunes:explicit>Yes</itunes:explicit><itunes:type>Serial</itunes:type>
<image><url>http://img.test/c.png</url></image>
<item><title>No enclosure</title><guid>a</guid></item>
<item><title>First</title><guid>dup</guid><enclosure url="http://x.test/1.mp3" type="audio/mpeg" length="123"/>
  <itunes:duration>1:02:03</itunes:duration><pubDate>Tue, 7 Oct 2025 08:30:00 GMT</pubDate>
  <itunes:image href="http://img.test/e.jpg"/><itunes:season>2</itunes:season></item>
<item><title>Duplicate</title><guid>dup</guid><enclosure url="http://x.test/2.mp3"/></item>
<item><title>No guid</title><enclosure url="http://x.test/3.mp3"/><itunes:duration>05:30</itunes:duration>
  <pubDate>2025-10-08T10:00:00+02:00</pubDate><itunes:explicit>clean</itunes:explicit>
  <description>plain</description>
  <content:encoded><![CDATA[<p>Hello <a href="http://x.test">world</a>&amp;more</p><p>Second&nbsp;para<br/>line</p><script>bad()</script>]]></content:encoded></item>
</channel></rss>`
	f, err := feed.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if !f.Explicit || f.Type != "serial" || f.ImageURL != "http://img.test/c.png" {
		t.Errorf("channel = %+v", f)
	}
	if len(f.Episodes) != 2 {
		t.Fatalf("episodes = %+v", f.Episodes)
	}
	a, b := f.Episodes[0], f.Episodes[1]
	if a.Title != "First" || a.Duration != time.Hour+2*time.Minute+3*time.Second || a.Enclosure.Length != 123 ||
		a.ImageURL != "http://img.test/e.jpg" || a.Season != 2 || a.Explicit != nil ||
		!a.PublishedAt.Equal(time.Date(2025, 10, 7, 8, 30, 0, 0, time.UTC)) {
		t.Errorf("first = %+v", a)
	}
	if b.GUID != "http://x.test/3.mp3" || b.Duration != 5*time.Minute+30*time.Second ||
		b.Explicit == nil || *b.Explicit || !b.PublishedAt.Equal(time.Date(2025, 10, 8, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("no guid = %+v", b)
	}
	if want := "Hello world&more\n\nSecond para\nline"; b.Description != want {
		t.Errorf("description = %q, want %q", b.Description, want)
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := feed.Parse(strings.NewReader(rssHead + `<item>`)); !errors.Is(err, feed.ErrInvalid) {
		t.Errorf("broken XML: %v", err)
	}
	if _, err := feed.Parse(strings.NewReader(`not xml at all`)); !errors.Is(err, feed.ErrInvalid) {
		t.Errorf("text: %v", err)
	}
	atom := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>A</title></feed>`
	if _, err := feed.Parse(strings.NewReader(atom)); !errors.Is(err, feed.ErrNotRSS) {
		t.Errorf("atom: %v", err)
	}
	if _, err := feed.Parse(strings.NewReader(`<rss version="2.0"></rss>`)); !errors.Is(err, feed.ErrNotRSS) {
		t.Errorf("no channel: %v", err)
	}
}

func TestParseLatin1(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><rss version=\"2.0\"><channel><title>Gr\xfcezi</title></channel></rss>"
	f, err := feed.Parse(strings.NewReader(src))
	if err != nil || f.Title != "Grüezi" {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"8": 8 * time.Second, "3600": time.Hour, "90.5": 90*time.Second + 500*time.Millisecond,
		"1:00:00": time.Hour, "01:02": time.Minute + 2*time.Second, "": 0, "abc": 0, "1:2:3:4": 0, "-5": 0,
	} {
		if got := feed.ParseDuration(in); got != want {
			t.Errorf("ParseDuration(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDate(t *testing.T) {
	want := time.Date(2025, 10, 6, 6, 0, 0, 0, time.UTC)
	for _, in := range []string{"Mon, 06 Oct 2025 06:00:00 +0000", "Mon, 06 Oct 2025 08:00:00 +0200",
		"Mon, 6 Oct 2025 06:00:00 GMT", "06 Oct 2025 06:00:00 +0000", "2025-10-06T06:00:00Z", "Mon, 06 Oct 2025 06:00 +0000"} {
		if got := feed.ParseDate(in); !got.Equal(want) {
			t.Errorf("ParseDate(%q) = %v", in, got)
		}
	}
	if !feed.ParseDate("gestern").IsZero() {
		t.Error("garbage date parsed")
	}
	// M6 review n-2: the named zones of RFC 822 keep their offset.
	for in, hours := range map[string]int{"EST": 1, "EDT": 2, "CST": 0, "CDT": 1, "MST": -1, "MDT": 0, "PST": -2, "PDT": -1} {
		if got := feed.ParseDate("Mon, 06 Oct 2025 0" + strconv.Itoa(5+hours) + ":00:00 " + in); !got.Equal(want.Add(5 * time.Hour)) {
			t.Errorf("ParseDate(%s) = %v, want %v", in, got, want.Add(5*time.Hour))
		}
	}
}

// M6 review m-10: titles, authors and guids are clipped, overlong URLs dropped.
func TestParseClipsFields(t *testing.T) {
	long := strings.Repeat("x", 3000)
	src := rssHead + `<itunes:author>` + strings.Repeat("A", 600) + `</itunes:author><itunes:image href="http://img.test/` + long + `"/>
<item><title>` + strings.Repeat("ü", 600) + `</title><guid>` + long + `</guid><enclosure url="http://x.test/1.mp3"/>
  <itunes:image href="http://img.test/` + long + `"/></item>
<item><title>Too long</title><enclosure url="http://x.test/` + long + `.mp3"/></item>
</channel></rss>`
	f, err := feed.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Author) != feed.MaxTitle || f.ImageURL != "" || len(f.Episodes) != 1 {
		t.Fatalf("channel author %d, image %q, %d episodes", len(f.Author), f.ImageURL, len(f.Episodes))
	}
	e := f.Episodes[0]
	if n := len([]rune(e.Title)); n != feed.MaxTitle || len(e.GUID) != feed.MaxURL || e.ImageURL != "" {
		t.Errorf("episode title %d runes, guid %d, image %q", n, len(e.GUID), e.ImageURL)
	}
}

func TestTextClips(t *testing.T) {
	long := strings.Repeat("ä", feed.MaxDescription+10)
	got := feed.Text(long)
	if n := len([]rune(got)); n != feed.MaxDescription || !strings.HasSuffix(got, "…") {
		t.Errorf("clipped to %d runes", n)
	}
	if got := feed.Text("x <unclosed"); got != "x <unclosed" {
		t.Errorf("lone bracket: %q", got)
	}
}
