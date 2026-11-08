package radio

import (
	"bufio"
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

func TestMetaCleanup(t *testing.T) {
	if got := decodeMetaText([]byte("K\xfcnstler - Lied")); got != "Künstler - Lied" {
		t.Errorf("CP1252: %q", got)
	}
	if got := decodeMetaText([]byte("Künstler - Lied\x00\x00")); got != "Künstler - Lied" {
		t.Errorf("UTF-8 must stay UTF-8 (no «KÃ¼nstler»): %q", got)
	}
	for _, tc := range []struct {
		text, artist, title string
		ok                  bool
	}{
		{"  Künstler   -  Lied  ", "Künstler", "Lied", true},
		{"Band – Song - Remix", "Band", "Song - Remix", true},
		{"Only a title", "", "", true},
		{"Radio Test FM", "", "", false}, // the station name
		{"radio test fm", "", "", false},
		{" - ", "", "", false},
		{"", "", "", false},
		{"Werbung", "", "", false},
		{"unknown", "", "", false},
	} {
		e, ok := cleanTitle(tc.text, "Radio Test FM")
		if ok != tc.ok || e.Artist != tc.artist || e.Title != tc.title {
			t.Errorf("cleanTitle(%q) = %+v %v", tc.text, e, ok)
		}
	}
	if title, ok, u := parseICYMeta([]byte("StreamTitle='Rock 'n' Roll - It''s';StreamUrl='http://x/y';\x00\x00")); !ok ||
		title != "Rock 'n' Roll - It''s" || u != "http://x/y" {
		t.Errorf("parseICYMeta: %q %v %q", title, ok, u)
	}
}

// oggTitlesOf pushes an Ogg file in chunks and returns every title in order.
func oggTitlesOf(t *testing.T, name string, chunk int) []string {
	t.Helper()
	data := testAudio(t, name)
	var o oggTitles
	var got []string
	for b := data; len(b) > 0; {
		n := min(len(b), chunk)
		if title, ok := o.push(b[:n]); ok {
			got = append(got, title)
		}
		b = b[n:]
	}
	return got
}

func TestOggVorbisComments(t *testing.T) {
	if got := oggTitlesOf(t, "tone-ogg.ogg", 1000); len(got) != 1 || got[0] != "Slouchcat - Test Tone" {
		t.Fatalf("titles %q", got)
	}
}

func TestOggOpusTags(t *testing.T) {
	if got := oggTitlesOf(t, "tone-opus.ogg", 333); len(got) != 1 || got[0] != "Slouchcat - Opus Tone" {
		t.Fatalf("titles %q", got)
	}
}

func TestOggChainedStreamNewTitle(t *testing.T) {
	got := oggTitlesOf(t, "tone-ogg-chained.ogg", 4096)
	if len(got) != 2 || got[0] != "Slouchcat - First Tone" || got[1] != "Slouchcat - Second Tone" {
		t.Fatalf("titles %q", got)
	}
	// Through the reader: an Icecast Ogg mount without icy-metaint.
	st := startStation(t)
	st.Set("/ogg", radiotestOgg(t))
	s := openTest(t, testClient(), st.URL("/ogg"))
	if in := s.Info().Input; in.Format != "ogg" || in.Codec != "vorbis" {
		t.Fatalf("input %+v", in)
	}
	_, titles := drain(t, s, 1000)
	if len(titles) != 2 {
		t.Fatalf("titles through the reader %v", titles)
	}
}

func radiotestOgg(t *testing.T) radiotest.Mount {
	return radiotest.Mount{ContentType: "audio/ogg", Audio: [][]byte{testAudio(t, "tone-ogg-chained.ogg")}, Once: true}
}

// The StreamUrl of an ICY block reaches the timeline entry of its title (the cover chain reads it).
func TestStreamURLReachesTheTimeline(t *testing.T) {
	text := []byte("StreamTitle='Band - Song';StreamUrl='https://img.example/c.jpg';")
	n := (len(text) + 15) / 16
	block := append([]byte{byte(n)}, append(text, make([]byte, n*16-len(text))...)...)
	body := append(append(make([]byte, minMetaInt), block...), make([]byte, 16)...)
	s := &icyStream{inf: streamInfo{MetaInt: minMetaInt}, left: minMetaInt,
		resp: &icyResponse{body: bufio.NewReader(bytes.NewReader(body))}}
	var ev streamEvent
	for !ev.TitleSet {
		var err error
		if ev, err = s.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if ev.Title != "Band - Song" || ev.StreamURL != "https://img.example/c.jpg" {
		t.Fatalf("event %+v", ev)
	}
	r := bufferRelay(t, "", 30*time.Minute, 1<<40, 1, 4*time.Second, nil)
	r.s, r.showsTitles = &Service{now: time.Now}, true
	r.title(context.Background(), ev, nil, &feedState{})
	if len(r.titles) != 1 || r.titles[0].streamURL != ev.StreamURL || r.titles[0].Artist != "Band" {
		t.Fatalf("timeline %+v", r.titles)
	}
}
