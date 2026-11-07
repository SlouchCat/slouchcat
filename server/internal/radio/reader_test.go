package radio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

// loopbackOnly is the dial control of the tests: the test station on 127.0.0.1 is allowed, every
// other local address is refused as in production (the twin of dialer_radiotest.go).
func loopbackOnly(network, address string, c syscall.RawConn) error {
	if ap, err := netip.ParseAddrPort(address); err == nil && ap.Addr().Unmap().IsLoopback() {
		return nil
	}
	return netguard.RefuseLocal(network, address, c)
}

func testLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func testClient() *netClient { return newNetClient(loopbackOnly, "Slouchcat/test", testLogger()) }

// testAudio reads a file of testdata/radio.
func testAudio(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "radio", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func startStation(t testing.TB) *radiotest.Station {
	t.Helper()
	st, err := radiotest.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

func openTest(t *testing.T, c *netClient, rawURL string) stream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	s, err := c.openStream(ctx, rawURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// drain reads a stream to its end and returns the audio and the titles with the audio offset at
// which they arrived.
func drain(t *testing.T, s stream, maxEvents int) ([]byte, map[int]string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var audio []byte
	titles := map[int]string{}
	for range maxEvents {
		ev, err := s.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if ev.TitleSet {
			titles[len(audio)] = ev.Title
		}
		audio = append(audio, ev.Audio...)
	}
	return audio, titles
}

func TestICYShoutcastV1StatusLine(t *testing.T) {
	st := startStation(t)
	mp3 := testAudio(t, "tone-mp3-128.mp3")
	st.Set("/sc", radiotest.Mount{Kind: radiotest.Shoutcast, Audio: [][]byte{mp3}, Once: true, Name: "Test FM", Bitrate: 128})
	s := openTest(t, testClient(), st.URL("/sc"))
	inf := s.Info()
	if inf.Input.Format != "mp3" || inf.Input.Codec != "mp3" || inf.Input.SampleRate != 44100 || inf.Input.BitrateKbps != 128 {
		t.Fatalf("input %+v, want mp3 44.1 kHz 128 kbit/s", inf.Input)
	}
	if inf.ICYName != "Test FM" || inf.ICYBitrate != 128 {
		t.Fatalf("icy headers %+v", inf)
	}
	audio, _ := drain(t, s, 10000)
	if !bytes.Equal(audio, mp3) {
		t.Fatalf("got %d audio bytes, want the %d of the file", len(audio), len(mp3))
	}
}

func TestICYMetaintDemux(t *testing.T) {
	st := startStation(t)
	mp3 := testAudio(t, "tone-mp3-128.mp3")
	st.Set("/ice", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{mp3}, Once: true, MetaInt: 8192,
		Titles: []radiotest.Title{{At: 0, Text: "Artist A - One"}, {At: 50000, Text: "Rock 'n' Roll - Two"}}})
	s := openTest(t, testClient(), st.URL("/ice"))
	if s.Info().MetaInt != 8192 {
		t.Fatalf("metaint %d", s.Info().MetaInt)
	}
	audio, titles := drain(t, s, 10000)
	if !bytes.Equal(audio, mp3) {
		t.Fatalf("got %d audio bytes, want the %d of the file unchanged", len(audio), len(mp3))
	}
	want := map[int]string{8192: "Artist A - One", 7 * 8192: "Rock 'n' Roll - Two"}
	if len(titles) != len(want) {
		t.Fatalf("titles %v, want %v", titles, want)
	}
	for at, text := range want {
		if titles[at] != text {
			t.Fatalf("titles %v, want %v", titles, want)
		}
	}
}

func TestICYRedirectLimit(t *testing.T) {
	st := startStation(t)
	mp3 := testAudio(t, "tone-mp3-128.mp3")
	st.Set("/s", radiotest.Mount{Audio: [][]byte{mp3}, Once: true})
	chain := func(prefix string, n int) string {
		for i := 1; i <= n; i++ {
			target := "/s"
			if i < n {
				target = prefix + string(rune('0'+i+1))
			}
			st.Set(prefix+string(rune('0'+i)), radiotest.Mount{Kind: radiotest.Redirect, Target: target})
		}
		return st.URL(prefix + "1")
	}
	ok := chain("/ok", 5)
	s := openTest(t, testClient(), ok)
	if s.Info().Input.Codec != "mp3" {
		t.Fatalf("after 5 redirects: %+v", s.Info().Input)
	}
	_, err := testClient().openStream(context.Background(), chain("/no", 6))
	if !errors.Is(err, model.ErrRadioUnreachable) || !strings.Contains(err.Error(), "more than 5") {
		t.Fatalf("6 redirects: %v, want ErrRadioUnreachable", err)
	}
}

func TestICYRefusesLocalTargets(t *testing.T) {
	st := startStation(t)
	st.Set("/s", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}})
	st.Set("/lan", radiotest.Mount{Kind: radiotest.Redirect, Target: "http://10.0.0.1/stream"})
	prod := newNetClient(netguard.RefuseLocal, "Slouchcat/test", testLogger())
	_, err := prod.openStream(context.Background(), st.URL("/s"))
	if !errors.Is(err, netguard.ErrRefused) || !errors.Is(err, model.ErrRadioUnreachable) {
		t.Fatalf("production control on 127.0.0.1: %v, want ErrRefused", err)
	}
	_, err = testClient().openStream(context.Background(), st.URL("/lan"))
	if !errors.Is(err, netguard.ErrRefused) {
		t.Fatalf("redirect to 10.0.0.1: %v, want ErrRefused", err)
	}
	if st.Connections("/s") != 0 {
		t.Fatal("the production client reached the station")
	}
}

func TestContentTypeAllowlistAndSniff(t *testing.T) {
	st := startStation(t)
	st.Set("/none", radiotest.Mount{ContentType: "-", Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}})
	st.Set("/octet", radiotest.Mount{ContentType: "application/octet-stream", Audio: [][]byte{testAudio(t, "tone-aac-64.aac")}})
	st.Set("/aacp", radiotest.Mount{ContentType: "audio/aacp", Audio: [][]byte{testAudio(t, "tone-aac-64.aac")}})
	st.Set("/html", radiotest.Mount{Kind: radiotest.HTML, Body: []byte("<!doctype html><html><body>Radio</body></html>")})
	st.Set("/fake", radiotest.Mount{ContentType: "audio/mpeg", Audio: [][]byte{[]byte("<html>" + strings.Repeat("x", 20000))}, Once: true})
	c := testClient()
	for path, codec := range map[string]string{"/none": "mp3", "/octet": "aac", "/aacp": "he-aac"} {
		if got := openTest(t, c, st.URL(path)).Info().Input.Codec; got != codec {
			t.Errorf("%s: codec %q, want %q", path, got, codec)
		}
	}
	for _, path := range []string{"/html", "/fake"} {
		if _, err := c.openStream(context.Background(), st.URL(path)); !errors.Is(err, model.ErrRadioNotAudio) {
			t.Errorf("%s: %v, want ErrRadioNotAudio", path, err)
		}
	}
}

func TestPlaylistResolution(t *testing.T) {
	st := startStation(t)
	st.Set("/s", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}})
	st.Set("/x.pls", radiotest.Mount{Kind: radiotest.PLS, Target: "/s"})
	st.Set("/x.m3u", radiotest.Mount{Kind: radiotest.M3U, Target: "/s"})
	st.Set("/sniffed.pls", radiotest.Mount{Kind: radiotest.PLS, ContentType: "text/plain", Target: "/s"})
	st.Set("/empty.m3u", radiotest.Mount{Kind: radiotest.HTML, ContentType: "audio/x-mpegurl", Body: []byte("# nothing\n")})
	c := testClient()
	for _, p := range []string{"/x.pls", "/x.m3u", "/sniffed.pls"} {
		if got := openTest(t, c, st.URL(p)).Info(); got.Input.Codec != "mp3" || !strings.HasSuffix(got.URL, "/s") {
			t.Errorf("%s: %+v", p, got)
		}
	}
	if _, err := c.openStream(context.Background(), st.URL("/empty.m3u")); !errors.Is(err, model.ErrRadioNotAudio) {
		t.Errorf("empty playlist: %v", err)
	}
}

func TestICYHeaderCap(t *testing.T) {
	st := startStation(t)
	big := strings.Repeat("X-Padding: "+strings.Repeat("p", 340)+"\r\n", 50) // 17.5 KiB in 50 lines
	st.Set("/big", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, Header: big})
	many := strings.Repeat("X-A: b\r\n", 120)
	st.Set("/many", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, Header: many})
	for _, p := range []string{"/big", "/many"} {
		_, err := testClient().openStream(context.Background(), st.URL(p))
		if !errors.Is(err, errHeaderTooLarge) || !errors.Is(err, model.ErrRadioUnreachable) {
			t.Errorf("%s: %v, want errHeaderTooLarge", p, err)
		}
	}
	// One endless header line: the cap ends the read instead of buffering it.
	st.Set("/endless", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")},
		Header: "X-Endless: " + strings.Repeat("e", 1<<20) + "\r\n"})
	if _, err := testClient().openStream(context.Background(), st.URL("/endless")); !errors.Is(err, errHeaderTooLarge) {
		t.Errorf("endless line: %v", err)
	}
}

// hlsTS sets the media mount path to the six TS segments of testdata/radio/hls.
func hlsTS(t *testing.T, st *radiotest.Station, path string) {
	t.Helper()
	var segs []radiotest.Segment
	for i := range 6 {
		segs = append(segs, radiotest.Segment{Data: testAudio(t, filepath.Join("hls", "seg"+string(rune('0'+i))+".ts"))})
	}
	st.Set(path, radiotest.Mount{Kind: radiotest.HLSMedia})
	st.SetHLS(path, 100, 4, 6, segs...)
}

func TestHLSSourcePicksAudioVariant(t *testing.T) {
	v := pickVariant([]hlsVariant{
		{uri: "video", bandwidth: 2_000_000, codecs: "avc1.64001f,mp4a.40.2"},
		{uri: "hi", bandwidth: 400_000, codecs: "mp4a.40.2"},
		{uri: "mid", bandwidth: 128_000, codecs: "mp4a.40.2"},
		{uri: "low", bandwidth: 64_000, codecs: "mp4a.40.5"},
	})
	if v.uri != "mid" {
		t.Fatalf("picked %q, want mid", v.uri)
	}
	if v := pickVariant([]hlsVariant{{uri: "a", bandwidth: 900_000}, {uri: "b", bandwidth: 500_000}}); v.uri != "b" {
		t.Fatalf("without audio-only variant picked %q, want the lowest", v.uri)
	}

	st := startStation(t)
	hlsTS(t, st, "/hls/mid")
	st.Set("/hls/master.m3u8", radiotest.Mount{Kind: radiotest.HLSMaster, Variants: []radiotest.Variant{
		{Path: "video", Bandwidth: 2_000_000, Codecs: "avc1.64001f,mp4a.40.2"},
		{Path: "mid", Bandwidth: 128_000, Codecs: "mp4a.40.2"},
	}})
	s := openTest(t, testClient(), st.URL("/hls/master.m3u8"))
	inf := s.Info()
	if !inf.HLS || inf.Input.Format != "mpegts" || inf.Input.Codec != "aac" || !strings.HasSuffix(inf.URL, "/hls/mid") {
		t.Fatalf("info %+v", inf)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := range hlsStartSegments {
		ev, err := s.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := testAudio(t, filepath.Join("hls", "seg"+string(rune('0'+3+i))+".ts")); !bytes.Equal(ev.Audio, want) {
			t.Fatalf("event %d is not segment %d", i, 3+i)
		}
	}
	// The next segment appears after a reload.
	st.AppendHLS("/hls/mid", radiotest.Segment{Data: testAudio(t, "hls/seg0.ts")})
	ev, err := s.Next(ctx)
	if err != nil || !bytes.Equal(ev.Audio, testAudio(t, "hls/seg0.ts")) || ev.Discontinuity {
		t.Fatalf("after reload: %v, discontinuity %v", err, ev.Discontinuity)
	}
}

func TestHLSSourceRejectsEncrypted(t *testing.T) {
	st := startStation(t)
	hlsTS(t, st, "/enc")
	st.Set("/enc", radiotest.Mount{Kind: radiotest.HLSMedia, Header: "#EXT-X-KEY:METHOD=AES-128,URI=\"key\"\n"})
	_, err := testClient().openStream(context.Background(), st.URL("/enc"))
	if !errors.Is(err, model.ErrRadioNotAudio) {
		t.Fatalf("%v, want ErrRadioNotAudio", err)
	}
}

func TestHLSSourceID3Titles(t *testing.T) {
	st := startStation(t)
	parts, err := radiotest.SplitADTS(testAudio(t, "tone-aac-64.aac"), 4)
	if err != nil {
		t.Fatal(err)
	}
	var segs []radiotest.Segment
	for i, p := range parts {
		segs = append(segs, radiotest.Segment{Data: append(radiotest.ID3("Künstler", "Song "+string(rune('1'+i))), p...),
			Discontinuity: i == 3})
	}
	st.Set("/packed", radiotest.Mount{Kind: radiotest.HLSMedia})
	st.SetHLS("/packed", 7, 4, 6, segs...)
	s := openTest(t, testClient(), st.URL("/packed"))
	if in := s.Info().Input; in.Format != "aac" || in.Codec != "aac" {
		t.Fatalf("input %+v", in)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 1; i < 4; i++ { // the last three of four
		ev, err := s.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := "Künstler - Song " + string(rune('1'+i)); !ev.TitleSet || ev.Title != want {
			t.Fatalf("segment %d: title %q, want %q", i, ev.Title, want)
		}
		if !bytes.Equal(ev.Audio, parts[i]) {
			t.Fatalf("segment %d: the ID3 tag is not stripped", i)
		}
		if ev.Discontinuity != (i == 3) {
			t.Fatalf("segment %d: discontinuity %v", i, ev.Discontinuity)
		}
	}
	// A media sequence gap is a discontinuity as well.
	st.SetHLS("/packed", 20, 4, 6, radiotest.Segment{Data: parts[0]})
	ev, err := s.Next(ctx)
	if err != nil || !ev.Discontinuity {
		t.Fatalf("after a sequence gap: %v, discontinuity %v", err, ev.Discontinuity)
	}
}

// withDialControl replaces the dial control of s (the unexported test option of build plan S4):
// the tests reach the test station on 127.0.0.1, which the production control refuses.
func withDialControl(s *Service, control dialControl) *Service {
	s.rel.control = control
	return s
}

func TestServiceUsesGuardedClient(t *testing.T) {
	st := startStation(t)
	st.Set("/s", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}})
	if _, err := New(nil, testLogger()).netClient().openStream(context.Background(), st.URL("/s")); !errors.Is(err, netguard.ErrRefused) {
		t.Fatalf("default service on 127.0.0.1: %v, want ErrRefused", err)
	}
	s := withDialControl(New(nil, testLogger(), WithUserAgent("Slouchcat/1.17.0")), loopbackOnly)
	if c := s.netClient(); c.userAgent != "Slouchcat/1.17.0" {
		t.Fatalf("user agent %q", c.userAgent)
	}
	_ = openTest(t, s.netClient(), st.URL("/s"))
}
