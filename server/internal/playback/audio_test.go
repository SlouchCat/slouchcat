package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Audiobooks (M6 design 3.3): audio-only decisions, one timeline over the parts of a book, the
// playback speed per title.

func audioProbe(codec string, channels int, bitrate int64, cover bool) *transcode.Probe {
	p := &transcode.Probe{Format: transcode.Format{Duration: 60 * time.Second, BitRate: bitrate}, Streams: []transcode.Stream{
		{Index: 0, Type: "audio", Codec: codec, Channels: channels, Default: true},
	}}
	if cover {
		p.Streams = append(p.Streams, transcode.Stream{Index: 1, Type: "video", Codec: "mjpeg", AttachedPic: true})
	}
	return p
}

func TestDecideAudio(t *testing.T) {
	mixedAAC := transcode.ConcatProbe([]transcode.Probe{*audioProbe("aac", 2, 96_000, true), *audioProbe("aac", 2, 96_000, false)})
	file := func(container, codec string) model.MediaFile {
		return model.MediaFile{Container: container, AudioCodec: codec}
	}
	direct := func(bitrate int) Decision {
		return Decision{Mode: ModeDirect, Audio: true, AudioStream: 0, BurnStream: -1, Bitrate: bitrate}
	}
	hls := func(reason string, copyAAC bool, bitrate int) Decision {
		return Decision{Mode: ModeTranscode, Audio: true, AudioCopy: copyAAC, Reason: reason, Quality: model.QualityAuto,
			AudioStream: 0, BurnStream: -1, Bitrate: bitrate}
	}
	capsNone := capsChrome
	capsNone.HLS = "none"
	cases := []struct {
		name  string
		file  model.MediaFile
		probe *transcode.Probe
		req   Request
		want  Decision
	}{
		{"m4b aac with cover", file("m4b", "aac"), audioProbe("aac", 1, 64_000, true), req(capsChrome), direct(64_000)},
		{"mp4 container aac", file("mp4", "aac"), audioProbe("aac", 2, 128_000, false), req(capsSafari), direct(128_000)},
		{"mp3", file("mp3", "mp3"), audioProbe("mp3", 1, 64_000, false), req(capsFirefox), direct(64_000)},
		{"mp3 without mp3 decoder", file("mp3", "mp3"), audioProbe("mp3", 1, 64_000, false), req(capsAndroid), hls(ReasonAudioCodec, false, 128_000)},
		{"flac is not a direct format", file("flac", "flac"), audioProbe("flac", 2, 900_000, false), req(capsChrome), hls(ReasonContainer, false, 128_000)},
		{"ogg opus", file("ogg", "opus"), audioProbe("opus", 2, 96_000, false), req(capsChrome), hls(ReasonContainer, false, 128_000)},
		{"no hls: the file", file("flac", "flac"), audioProbe("flac", 2, 900_000, false), req(capsNone), direct(900_000)},
		{"multi-part mp3", file("mp3", "mp3"), audioProbe("mp3", 1, 64_000, false), Request{Caps: capsChrome, AudioStream: -1, MultiPart: true},
			hls(ReasonContainer, false, 128_000)},
		{"multi-part m4a copies", file("m4a", "aac"), audioProbe("aac", 2, 96_000, false), Request{Caps: capsChrome, AudioStream: -1, MultiPart: true},
			hls(ReasonContainer, true, 96_000)},
		{"multi-part without hls stays hls", file("mp3", "mp3"), audioProbe("mp3", 1, 64_000, false), Request{Caps: capsNone, AudioStream: -1, MultiPart: true},
			hls(ReasonContainer, false, 128_000)},
		{"bandwidth limit", file("m4b", "aac"), audioProbe("aac", 2, 256_000, false),
			Request{Caps: capsChrome, AudioStream: -1, Limits: model.PlaybackLimits{MaxBitrateKbps: 200}}, hls(ReasonBandwidth, false, 128_000)},
		{"rebuild of a transcode", file("m4b", "aac"), audioProbe("aac", 2, 64_000, false), Request{Caps: capsChrome, AudioStream: -1, ForceTranscode: true},
			hls(ReasonContainer, true, 64_000)},
		// M6 review n-6: a limit below the AAC rate caps the encode.
		{"bandwidth limit below the aac rate", file("m4b", "aac"), audioProbe("aac", 2, 256_000, false),
			Request{Caps: capsChrome, AudioStream: -1, Limits: model.PlaybackLimits{MaxBitrateKbps: 96}}, hls(ReasonBandwidth, false, 96_000)},
		// M6 review M-4: m4a parts that differ (cover art in one) go through the concat filter, no copy.
		{"multi-part m4a parts that differ", file("m4a", "aac"), &mixedAAC, Request{Caps: capsChrome, AudioStream: -1, MultiPart: true},
			hls(ReasonContainer, false, 128_000)},
	}
	for _, c := range cases {
		if got := Decide(c.file, c.probe, c.req); got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
	// A transcode of a video file is unchanged by the audio rules.
	if d := Decide(model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080},
		sizedProbe("h264", "bt709", 1920, 1080, 8_000_000), req(capsChrome)); d.Audio || d.Mode != ModeRemux {
		t.Errorf("video decision %+v", d)
	}

	opts := QualityOptions(hls(ReasonContainer, false, 128_000), 0, 0, model.PlaybackLimits{}, capsChrome, "720p")
	if len(opts) != 1 || opts[0] != (model.QualityOption{Quality: model.QualityAuto, Bitrate: 128_000, Available: true, Current: true}) {
		t.Errorf("audio qualities %+v", opts)
	}
	target := hlsTarget(HLSRequest{Decision: hls(ReasonContainer, false, 128_000)})
	if target.Mode != transcode.ModeTranscode || target.AudioBitrate != audioOnlyKbps || target.MaxBitrate != 0 || target.Quality != "" {
		t.Errorf("audio target %+v", target)
	}
	if target := hlsTarget(HLSRequest{Decision: hls(ReasonBandwidth, false, 96_000)}); target.AudioBitrate != 96 {
		t.Errorf("audio target under a bandwidth limit %+v", target)
	}
}

func TestAudioMimeTypes(t *testing.T) {
	for _, c := range []struct{ container, video, audio, want string }{
		{"mp3", "", "mp3", "audio/mpeg"},
		{"m4b", "", "aac", `audio/mp4; codecs="mp4a.40.2"`},
		{"m4a", "", "alac", "audio/mp4"},
		{"mp4", "", "aac", `audio/mp4; codecs="mp4a.40.2"`},
		{"mp4", "h264", "aac", `video/mp4; codecs="avc1.64001F, mp4a.40.2"`},
		{"flac", "", "flac", "audio/flac"},
		{"ogg", "", "vorbis", "audio/ogg"},
		{"opus", "", "opus", "audio/ogg"},
	} {
		if got := MimeType(c.container, c.video, c.audio); got != c.want {
			t.Errorf("MimeType(%s, %s, %s) = %q, want %q", c.container, c.video, c.audio, got, c.want)
		}
	}
}

// testPart is a part of a test audiobook.
type testPart struct {
	id, rel string
	probe   transcode.Probe
}

// addAudiobook inserts a visible audiobook (rating 0) with the parts and chapters.
func addAudiobook(t *testing.T, f *vistest.Fixture, id string, parts []testPart, chapters []model.Chapter) {
	t.Helper()
	var rows []vistest.BookPart
	for _, p := range parts {
		raw, err := json.Marshal(p.probe)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, vistest.BookPart{ID: p.id, RelPath: p.rel, Container: filepath.Ext(p.rel)[1:], AudioCodec: p.probe.Streams[0].Codec,
			Probe: string(raw), Size: p.probe.Format.Size, DurationMs: p.probe.Format.Duration.Milliseconds()})
	}
	f.AddBookWithParts(t, id, rows, chapters)
}

func mp3Parts(n int) []testPart {
	var parts []testPart
	for i := range n {
		parts = append(parts, testPart{
			id:  fmt.Sprintf("01J9000000000000000000PT%02d", i+1),
			rel: fmt.Sprintf("Autorin/Buch/%02d - Kapitel %d.mp3", i+1, i+1),
			probe: transcode.Probe{Format: transcode.Format{Name: "mp3", Duration: time.Duration(6+i) * time.Second, Size: 48_000, BitRate: 64_000},
				Streams: []transcode.Stream{{Index: 0, Type: "audio", Codec: "mp3", Channels: 1}}},
		})
	}
	return parts
}

func TestStartAudiobookParts(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	const book = "01J9000000000000000000BOOK"
	parts := mp3Parts(3)
	chapters := []model.Chapter{{StartMs: 0, EndMs: 6000, Title: "Kapitel 1"}, {StartMs: 6000, EndMs: 13000, Title: "Kapitel 2"},
		{StartMs: 13000, EndMs: 21000, Title: "Kapitel 3"}}
	addAudiobook(t, f, book, parts, chapters)
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, playback_rate)
	                                          VALUES (?, ?, 14500, 21000, 0, 1, 1.5)`, f.P6.ID, book); err != nil {
		t.Fatal(err)
	}

	info, err := s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{Capabilities: &capsChrome})
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []model.PlaybackPart{{MediaFileID: parts[0].id, StartMs: 0, DurationMs: 6000},
		{MediaFileID: parts[1].id, StartMs: 6000, DurationMs: 7000}, {MediaFileID: parts[2].id, StartMs: 13000, DurationMs: 8000}}
	if info.MediaKind != model.MediaKindAudio || info.Mode != "hls" || !slices.Equal(info.Parts, wantParts) ||
		!slices.Equal(info.Chapters, chapters) || info.ResumePositionMs != 14500 || info.MediaFile.DurationMs != 21000 ||
		info.Item.Progress == nil || info.Item.Progress.PlaybackRate == nil || *info.Item.Progress.PlaybackRate != 1.5 {
		t.Errorf("info %+v", info)
	}
	if len(info.Qualities) != 1 || info.Qualities[0].Quality != model.QualityAuto || info.Transcoding == nil || info.Transcoding.Reason != ReasonContainer {
		t.Errorf("qualities %+v transcoding %+v", info.Qualities, info.Transcoding)
	}
	starts := tc.starts()
	if len(starts) != 1 {
		t.Fatalf("starts %d", len(starts))
	}
	r := starts[0]
	var want []string
	for _, p := range parts {
		want = append(want, filepath.Join(f.MediaDir, filepath.FromSlash(p.rel)))
	}
	if !slices.Equal(r.Parts, want) || r.Probe.Format.Duration != 21*time.Second || !r.Decision.Audio || r.Resume != 14500*time.Millisecond {
		t.Errorf("hls request parts %v duration %v decision %+v resume %v", r.Parts, r.Probe.Format.Duration, r.Decision, r.Resume)
	}
	if src := hlsSource(r); !slices.Equal(src.Parts, want) {
		t.Errorf("source parts %v", src.Parts)
	}

	// Rebuild after a lost output keeps the book.
	tc.lose(info.PlaybackSessionID)
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.HLS(ctx, g, 2); err != nil {
		t.Fatal(err)
	}
	if starts := tc.starts(); len(starts) != 2 || !slices.Equal(starts[1].Parts, want) || starts[1].StartSegment != 2 {
		t.Errorf("rebuild %+v", starts[len(starts)-1])
	}

	// A part that is not probed yet: the book cannot play.
	f.SetMediaFile(t, parts[2].id, "mp3", "", "mp3", "", nil)
	if _, err := s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{Capabilities: &capsChrome}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unprobed part: %v", err)
	}
	// M6 review M-3: a part whose probe failed is left out, like in the chapters and the downloads;
	// the book plays its other parts.
	f.SetProbeError(t, parts[2].id, "invalid data")
	info, err = s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{Capabilities: &capsChrome})
	if err != nil || len(info.Parts) != 2 || info.MediaFile.DurationMs != 13000 {
		t.Errorf("book with a broken part: %+v %v", info.Parts, err)
	}
	// A book rated 18 is invisible for P6: 404.
	eighteen := 18
	f.SetRating(t, book, &eighteen)
	if _, err := s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible book: %v", err)
	}
}

func TestStartSingleFileAudiobookDirect(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithTranscoder(newFakeTranscoder()))
	ctx := context.Background()
	const book = "01J9000000000000000000M4BB"
	probe := *audioProbe("aac", 1, 64_000, true)
	probe.Format.Name = "mov,mp4,m4a,3gp,3g2,mj2"
	probe.Chapters = []transcode.Chapter{{Start: 0, End: 30 * time.Second, Title: "A"}, {Start: 30 * time.Second, End: 60 * time.Second, Title: "B"}}
	rel := "Autorin/Buch/Buch.m4b"
	if err := os.MkdirAll(filepath.Join(f.MediaDir, "Autorin", "Buch"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.MediaDir, filepath.FromSlash(rel)), []byte("m4b data"), 0o600); err != nil {
		t.Fatal(err)
	}
	addAudiobook(t, f, book, []testPart{{id: "01J9000000000000000000M4BF", rel: rel, probe: probe}},
		[]model.Chapter{{StartMs: 0, EndMs: 30000, Title: "Eins"}, {StartMs: 30000, EndMs: 60000, Title: "Zwei"}})
	info, err := s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{Capabilities: &capsChrome})
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != ModeDirect || info.MediaKind != model.MediaKindAudio || info.Parts != nil || info.MediaFile.MimeType != `audio/mp4; codecs="mp4a.40.2"` ||
		len(info.Chapters) != 2 || info.Chapters[0].Title != "Eins" || len(info.Qualities) != 1 {
		t.Errorf("info %+v", info)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	src, err := s.OpenGrant(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	defer src.File.Close()
	if src.MediaType != "audio/mp4" {
		t.Errorf("stream media type %q", src.MediaType)
	}
	// Without stored chapters the probe's marks apply.
	f.SetBookChapters(t, book, nil)
	info, err = s.Start(ctx, browserSession(t, f, f.P6), book, model.PlaybackRequest{Capabilities: &capsChrome})
	if err != nil || len(info.Chapters) != 2 || info.Chapters[1].Title != "B" {
		t.Errorf("probe chapters %+v %v", info.Chapters, err)
	}
}

// TestAudioHLSWithFFmpeg starts the HLS output of a book of three MP3 parts (copies of the
// generated fixture) with the real transcoder and reads its segments.
func TestAudioHLSWithFFmpeg(t *testing.T) {
	ffmpeg, ffprobe := lookPath("ffmpeg"), lookPath("ffprobe")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("ffmpeg not installed")
	}
	fixture, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Audiobooks", "Slouchcat Test Author",
		"Slouchcat Test Audio Series", "02 - Slouchcat Test Audiobook MP3"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("test media missing (run `task testdata:media` in the repository root): %v", err)
	}
	f := vistest.New(t)
	runner := transcode.Runner{FFmpegPath: ffmpeg, FFprobePath: ffprobe}
	ctx := context.Background()
	parts := mp3Parts(3)
	var total time.Duration
	for i := range parts {
		dst := filepath.Join(f.MediaDir, filepath.FromSlash(parts[i].rel))
		if err := copyFile(filepath.Join(fixture, filepath.Base(parts[i].rel)), dst); err != nil {
			t.Fatal(err)
		}
		if parts[i].probe, err = runner.Probe(ctx, dst); err != nil {
			t.Fatal(err)
		}
		total += parts[i].probe.Format.Duration
	}
	addAudiobook(t, f, "01J9000000000000000000BOOK", parts, nil)
	m, err := transcode.NewManager(transcode.ManagerOptions{Runner: runner, WorkDir: filepath.Join(t.TempDir(), "transcode"), Backend: transcode.BackendSoftware})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	tc := NewManagerTranscoder(m, transcode.BackendSoftware)
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	info, err := s.Start(ctx, browserSession(t, f, f.P6), "01J9000000000000000000BOOK", model.PlaybackRequest{Capabilities: &capsChrome})
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != "hls" || len(info.Chapters) != 3 || info.Chapters[2].StartMs < 11_000 {
		t.Fatalf("info %+v", info)
	}
	hs, ok := tc.Get(info.PlaybackSessionID)
	if !ok {
		t.Fatal("no hls session")
	}
	if master := string(hs.MasterPlaylist("media.m3u8")); !bytes.Contains([]byte(master), []byte(`CODECS="mp4a.40.2"`)) || bytes.Contains([]byte(master), []byte("RESOLUTION")) {
		t.Errorf("master playlist %s", master)
	}
	read := func(rc io.ReadSeekCloser, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	init := read(hs.OpenInit(ctx))
	n := int((total + transcode.SegmentLength - 1) / transcode.SegmentLength)
	if last := total - time.Duration(n-1)*transcode.SegmentLength; last < time.Second {
		n-- // a short tail joins the previous segment
	}
	var size int
	for i := range n {
		seg := read(hs.OpenSegment(ctx, i))
		if !bytes.Contains(seg[:min(len(seg), 64)], []byte("moof")) {
			t.Errorf("segment %d is not a fragment", i)
		}
		size += len(seg)
	}
	if !bytes.Contains(init, []byte("mp4a")) || size == 0 {
		t.Errorf("init %d bytes, segments %d bytes", len(init), size)
	}
	if _, err := hs.OpenSegment(ctx, n); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("segment after the end: %v", err)
	}
}

func lookPath(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func TestReportPlaybackRate(t *testing.T) {
	f := vistest.New(t)
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { now = now.Add(time.Second); return now }))
	ctx := context.Background()
	m := f.Rated(6)
	rate := func(v float64) *float64 { return &v }
	ev := func(seq int64, r *float64) model.ProgressEvent {
		return model.ProgressEvent{PositionMs: 60_000 + seq, DurationMs: vistest.DurationMs, PlayedMs: 1000, Seq: seq, State: "playing",
			ClientTime: now, PlaybackRate: r}
	}
	p, err := s.Report(ctx, f.P6, m.ID, ev(1, rate(2)))
	if err != nil || p.PlaybackRate == nil || *p.PlaybackRate != 2 {
		t.Fatalf("report with rate: %+v %v", p, err)
	}
	// An event without a rate keeps the stored one.
	p, err = s.Report(ctx, f.P6, m.ID, ev(2, nil))
	if err != nil || p.PlaybackRate == nil || *p.PlaybackRate != 2 {
		t.Errorf("report without rate: %+v %v", p, err)
	}
	var stored float64
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT playback_rate FROM progress WHERE profile_id = ? AND item_id = ?`, f.P6.ID, m.ID).Scan(&stored); err != nil || stored != 2 {
		t.Errorf("stored rate %v %v", stored, err)
	}
	if p, err = s.Report(ctx, f.P6, m.ID, ev(3, rate(0.75))); err != nil || *p.PlaybackRate != 0.75 {
		t.Errorf("changed rate: %+v %v", p, err)
	}
	item, err := s.vis.Summary(ctx, visibility.Profile(f.P6), m.ID)
	if err != nil || item.Progress == nil || item.Progress.PlaybackRate == nil || *item.Progress.PlaybackRate != 0.75 {
		t.Errorf("summary progress %+v %v", item.Progress, err)
	}
	for _, bad := range []float64{0.25, 3.5} {
		var verr *model.ValidationError
		if _, err := s.Report(ctx, f.P6, m.ID, ev(4, rate(bad))); !errors.As(err, &verr) || verr.Fields[0].Field != "playbackRate" {
			t.Errorf("rate %v: %v", bad, err)
		}
	}
}
