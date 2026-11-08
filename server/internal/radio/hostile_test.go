package radio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Hostile and failing sources (review S-M2, S-M3, S-L3, CR-1, CR-5): every limit a source could
// push against holds.

func TestICYRefusesTinyMetaint(t *testing.T) {
	st := startStation(t)
	st.Set("/tiny", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, MetaInt: 100})
	if _, err := testClient().openStream(context.Background(), st.URL("/tiny")); !errors.Is(err, model.ErrRadioNotAudio) {
		t.Fatalf("icy-metaint 100: %v, want ErrRadioNotAudio", err)
	}
	st.Set("/ok", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, MetaInt: minMetaInt})
	if s := openTest(t, testClient(), st.URL("/ok")); s.Info().MetaInt != minMetaInt {
		t.Fatalf("icy-metaint %d: got %d", minMetaInt, s.Info().MetaInt)
	}
}

// collect keeps at most maxCollectEvents events, however many metadata blocks the source sends.
func TestICYCollectBounded(t *testing.T) {
	var body bytes.Buffer
	for range 4096 {
		body.Write(make([]byte, minMetaInt))
		body.WriteByte(1)
		body.WriteString("StreamTitle='x';")
	}
	s := &icyStream{inf: streamInfo{MetaInt: minMetaInt}, left: minMetaInt,
		resp: &icyResponse{body: bufio.NewReader(bytes.NewReader(body.Bytes()))}}
	audio, err := s.collect(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.pending) > maxCollectEvents || len(audio) >= 1<<20 {
		t.Fatalf("%d events, %d audio bytes kept", len(s.pending), len(audio))
	}
}

func TestTitleTextAndTimelineCapped(t *testing.T) {
	e, ok := cleanTitle(strings.Repeat("ä", 3000) + " - Lied")
	if !ok || utf8.RuneCountInString(e.Text) != maxTitleRunes || !utf8.ValidString(e.Text) {
		t.Fatalf("title of %d runes, ok %v", utf8.RuneCountInString(e.Text), ok)
	}
	r := bufferRelay(t, "", 30*time.Minute, 1<<40, 1, 4*time.Second, nil)
	r.s, r.showsTitles = &Service{now: time.Now}, true
	for i := range 3 * maxRelayTitles {
		r.title(context.Background(), streamEvent{Title: "Artist - Song " + strconv.Itoa(i), TitleSet: true}, nil, &feedState{})
	}
	if n := len(r.titles); n != maxRelayTitles || r.titles[n-1].Title != "Song "+strconv.Itoa(3*maxRelayTitles-1) {
		t.Fatalf("%d titles, last %+v", n, r.titles[len(r.titles)-1])
	}
}

// oggBOS is a page that opens logical stream serial and carries one byte of its first packet.
func oggBOS(serial uint32) []byte {
	p := make([]byte, 27, 29)
	copy(p, "OggS")
	p[5] = 0x02
	binary.LittleEndian.PutUint32(p[14:], serial)
	p[26] = 1
	return append(p, 1, 0x01)
}

func TestOggStreamsBounded(t *testing.T) {
	var o oggTitles
	for serial := range uint32(100) {
		o.push(oggBOS(serial))
		if len(o.streams) > oggMaxStreams {
			t.Fatalf("%d logical streams tracked", len(o.streams))
		}
	}
}

// A reload whose media sequence jumps more than hlsMaxJump queues the last three segments only,
// like the first load, with a discontinuity.
func TestHLSJumpKeepsLastSegments(t *testing.T) {
	media, _ := url.Parse("http://127.0.0.1/live.m3u8")
	s := &hlsStream{c: testClient(), media: media, lastSeq: 100}
	var pl strings.Builder
	pl.WriteString("#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:101\n")
	for i := range 500 {
		pl.WriteString("#EXTINF:4,\ns" + strconv.Itoa(i) + ".ts\n")
	}
	if err := s.apply(context.Background(), parseHLS([]byte(pl.String()))); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != hlsStartSegments || s.queue[0].seq != 598 || !s.queue[0].discontinuity || s.lastSeq != 600 {
		t.Fatalf("queue %+v, last %d", s.queue, s.lastSeq)
	}
	// A normal step is followed segment by segment.
	s = &hlsStream{c: testClient(), media: media, lastSeq: 100}
	if err := s.apply(context.Background(), parseHLS([]byte("#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:96\n"+
		strings.Repeat("#EXTINF:4,\ns.ts\n", 10)))); err != nil {
		t.Fatal(err)
	}
	if len(s.queue) != 5 || s.queue[0].seq != 101 || s.queue[0].discontinuity {
		t.Fatalf("queue %+v", s.queue)
	}
}

// A playlist that stops listing new segments ends the connection instead of reloading forever.
func TestHLSStallEnds(t *testing.T) {
	st := startStation(t)
	var segs []radiotest.Segment
	for i := range 3 {
		segs = append(segs, radiotest.Segment{Data: testAudio(t, filepath.Join("hls", "seg"+strconv.Itoa(i)+".ts"))})
	}
	st.Set("/hls", radiotest.Mount{Kind: radiotest.HLSMedia})
	st.SetHLS("/hls", 0, 1, 3, segs...) // target duration 1 s
	s := openTest(t, testClient(), st.URL("/hls")).(*hlsStream)
	s.minStall = 0
	stall := 3 * s.target
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	var err error
	for err == nil {
		_, err = s.Next(ctx)
	}
	if !errors.Is(err, model.ErrRadioUnreachable) || ctx.Err() != nil {
		t.Fatalf("%v, want ErrRadioUnreachable", err)
	}
	if d := time.Since(start); d < stall-time.Second || d > stall+5*time.Second {
		t.Fatalf("stall after %v, want about %v", d, stall)
	}
}

// A station that connects but whose packager fails at once ends after the backoff with
// ErrRadioNotAudio and one abort, instead of spawning an ffmpeg per chunk and reconnecting forever.
func TestRelayEndsWhenPackagerFails(t *testing.T) {
	e := newRelayEnv(t)
	fake := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	e.s.o.runner = transcode.Runner{FFmpegPath: fake}
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	if _, err := e.s.Listen(context.Background(), e.f.Session(t, e.f.P18), id); !errors.Is(err, model.ErrRadioNotAudio) {
		t.Fatalf("listen: %v, want ErrRadioNotAudio", err)
	}
	if n := e.st.Connections("/live"); n != len(e.s.rel.knobs.backoff)+1 {
		t.Fatalf("%d connections, want %d", n, len(e.s.rel.knobs.backoff)+1)
	}
	deadline := time.Now().Add(5 * time.Second)
	for e.stationRow(id).Failures != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("failures %d, want one abort", e.stationRow(id).Failures)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A source faster than real time is read at real time after the start burst: the first segments
// come at once, then the packaged audio stays near the time since connect.
func TestRelayPacesFastSource(t *testing.T) {
	e := newRelayEnv(t)
	ahead := defaultRelayKnobs().paceAhead
	e.s.rel.knobs.paceAhead = ahead
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	start := time.Now()
	e.listen(e.f.P18, id)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("ready after %v: the start burst was held back", d)
	}
	r := e.relay(id)
	time.Sleep(6 * time.Second)
	r.mu.Lock()
	produced := r.produced
	r.mu.Unlock()
	// ffmpeg holds some input in its pipe and buffers (a few seconds at 128 kbit/s).
	if limit := time.Since(start) + ahead + 10*time.Second; produced > limit {
		t.Fatalf("%v of audio after %v, want at most %v", produced, time.Since(start), limit)
	}
}

// The logo chain reads at most maxManifestIcons icons of a web manifest: a manifest cannot make
// the job fetch without end.
func TestLogoManifestIconsCapped(t *testing.T) {
	e := newLogoEnv(t)
	serve(e.st, "/late.png", "image/png", pngOf(t, 512, 512))
	var icons []string
	for i := range maxManifestIcons {
		icons = append(icons, `{"src":"/missing`+strconv.Itoa(i)+`.png","sizes":"512x512"}`)
	}
	icons = append(icons, `{"src":"/late.png","sizes":"512x512"}`)
	serve(e.st, "/app.webmanifest", "application/manifest+json", []byte(`{"icons":[`+strings.Join(icons, ",")+`]}`))
	id := e.addStation("Manifest FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET homepage = ? WHERE id = ?`,
		serve(e.st, "/m.html", "text/html", []byte(`<link rel="manifest" href="/app.webmanifest">`)), id)
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState == logoStateWebsite {
		t.Fatalf("icon %d of the manifest was fetched", maxManifestIcons+1)
	}
	if n := e.st.Connections("/late.png"); n != 0 {
		t.Fatalf("%d fetches of the icon beyond the cap", n)
	}
}
