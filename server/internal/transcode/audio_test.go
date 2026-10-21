package transcode

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg" // registers JPEG for image.DecodeConfig
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseProbeFormatTags(t *testing.T) {
	p, err := ParseProbe([]byte(`{"format":{"format_name":"mp3","duration":"6.0","tags":{"TITLE":"Kapitel 3","Artist":"A",
		"composer":" N ","track":"3/10","empty":"  "}},"streams":[{"index":0,"codec_type":"audio","codec_name":"mp3"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"title": "Kapitel 3", "artist": "A", "composer": "N", "track": "3/10"}
	if len(p.Format.Tags) != len(want) {
		t.Fatalf("tags %v, want %v", p.Format.Tags, want)
	}
	for k, v := range want {
		if p.Format.Tags[k] != v {
			t.Errorf("tag %s = %q, want %q", k, p.Format.Tags[k], v)
		}
	}
	none, err := ParseProbe([]byte(`{"format":{"format_name":"mp3"},"streams":[]}`))
	if err != nil || none.Format.Tags != nil {
		t.Errorf("no tags: %v %v", none.Format.Tags, err)
	}
}

func TestConcatProbe(t *testing.T) {
	audio := Stream{Index: 0, Type: "audio", Codec: "mp3", Channels: 1}
	cover := Stream{Index: 1, Type: "video", Codec: "mjpeg", AttachedPic: true}
	parts := []Probe{
		{Format: Format{Name: "mp3", Duration: sec(5), Size: 40_000, Tags: map[string]string{"title": "Eins"}}, Streams: []Stream{audio, cover}},
		// Two marks: kept and shifted by the first part.
		{Format: Format{Name: "mp3", Duration: sec(6), Size: 48_000}, Streams: []Stream{audio},
			Chapters: []Chapter{{Start: 0, End: sec(2), Title: "A"}, {Start: sec(2), End: sec(6.5), Title: "B"}}},
		// One mark counts as none.
		{Format: Format{Name: "mp3", Duration: sec(7), Size: 56_000, Tags: map[string]string{"title": "Drei"}}, Streams: []Stream{audio},
			Chapters: []Chapter{{Start: 0, End: sec(7), Title: "ignored"}}},
	}
	p := ConcatProbe(parts)
	if p.Format.Duration != sec(18) || p.Format.Size != 144_000 || p.Format.BitRate != 64_000 || p.Format.Tags["title"] != "Eins" {
		t.Errorf("format %+v", p.Format)
	}
	if len(p.Streams) != 1 || p.Streams[0].Type != "audio" {
		t.Errorf("streams %+v, want the first part's audio only", p.Streams)
	}
	want := []Chapter{
		{Start: 0, End: sec(5), Title: "Eins"},
		{Start: sec(5), End: sec(7), Title: "A"},
		{Start: sec(7), End: sec(11), Title: "B"},
		{Start: sec(11), End: sec(18), Title: "Drei"},
	}
	if !slices.Equal(p.Chapters, want) {
		t.Errorf("chapters %+v, want %+v", p.Chapters, want)
	}
	if empty := ConcatProbe(nil); empty.Format.Duration != 0 || len(empty.Chapters) != 0 {
		t.Errorf("empty %+v", empty)
	}
	// M6 review n-7: marks are clamped to their part (an end never before its start).
	marks := PartChapters(Probe{Chapters: []Chapter{{Start: -sec(1), End: sec(2), Title: "A"}, {Start: sec(8), End: sec(7), Title: "B"}}}, sec(5))
	if want := []Chapter{{Start: 0, End: sec(2), Title: "A"}, {Start: sec(5), End: sec(5), Title: "B"}}; !slices.Equal(marks, want) {
		t.Errorf("clamped marks %+v, want %+v", marks, want)
	}
	if PartChapters(Probe{Chapters: []Chapter{{End: sec(1)}}}, sec(1)) != nil {
		t.Error("one mark counts as none")
	}

	// M6 review M-4: parts whose streams differ are Mixed (concat filter); equal ones are not.
	if !p.Mixed() {
		t.Error("cover art in the first part only: not mixed")
	}
	same := []Probe{parts[1], parts[2]}
	for _, tc := range []struct {
		name  string
		edit  func(*Stream)
		mixed bool
	}{
		{"equal", func(*Stream) {}, false},
		{"codec", func(s *Stream) { s.Codec = "aac" }, true},
		{"channels", func(s *Stream) { s.Channels = 2 }, true},
		{"sample rate", func(s *Stream) { s.SampleRate = 22050 }, true},
		{"position", func(s *Stream) { s.Index = 1 }, true},
	} {
		second := same[1]
		second.Streams = []Stream{audio}
		tc.edit(&second.Streams[0])
		if got := ConcatProbe([]Probe{same[0], second}).Mixed(); got != tc.mixed {
			t.Errorf("%s: mixed %v, want %v", tc.name, got, tc.mixed)
		}
	}
}

// Mixed parts: one input per part with the concat filter; a restart reads from the part that holds
// its boundary, shifted onto the book's timeline, and the audio is encoded even when copy was asked.
func TestMixedPartsArgs(t *testing.T) {
	audio := Stream{Index: 0, Type: "audio", Codec: "aac", Channels: 2}
	book := ConcatProbe([]Probe{
		{Format: Format{Duration: sec(10)}, Streams: []Stream{{Index: 0, Type: "video", Codec: "mjpeg", AttachedPic: true}, {Index: 1, Type: "audio", Codec: "aac", Channels: 2}}},
		{Format: Format{Duration: sec(10)}, Streams: []Stream{audio}},
		{Format: Format{Duration: sec(10)}, Streams: []Stream{audio}},
	})
	src := Source{Parts: []string{"/m/01.m4a", "/m/02.m4a", "/m/03.m4a"}, Probe: book}
	p, err := NewPlan(src, Target{Mode: ModeTranscode, AudioCopy: true, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Target.AudioCopy {
		t.Error("mixed parts planned as a copy")
	}
	for _, tc := range []struct {
		start int
		want  string
	}{
		{0, "-i /m/01.m4a -i /m/02.m4a -i /m/03.m4a -filter_complex [0:a:0][1:a:0][2:a:0]concat=n=3:v=0:a=1[aout] -map [aout]"},
		{0, "-ac 2 -ar 48000"}, // encoded at one rate in every run, whichever part it starts in
		{3, "-ss 2.000000 -itsoffset 10.000000 -i /m/02.m4a -i /m/03.m4a -filter_complex [0:a:0][1:a:0]concat=n=2:v=0:a=1[aout] -map [aout]"},
		{5, "-itsoffset 20.000000 -i /m/03.m4a -filter_complex [0:a:0]concat=n=1:v=0:a=1[aout] -map [aout]"},
	} {
		args, err := ffmpegArgs(Capabilities{}, src, p, tc.start, "/w/run")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, tc.want) || strings.Contains(joined, "-f concat") || strings.Contains(joined, "-c:a copy") {
			t.Errorf("segment %d: %s\nwant %s", tc.start, joined, tc.want)
		}
	}
}

func TestAudioOnlyPlanAndArgs(t *testing.T) {
	mono := Source{Path: "/m/book.mp3", Probe: Probe{Format: Format{Name: "mp3", Duration: sec(10), BitRate: 64_000}, Streams: []Stream{
		{Index: 0, Type: "audio", Codec: "mp3", Channels: 1}, {Index: 1, Type: "video", Codec: "mjpeg", AttachedPic: true},
	}}}
	p, err := NewPlan(mono, Target{Mode: ModeTranscode, AudioBitrate: 128, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Cost != costAudioOnly || p.Width != 0 || p.Height != 0 || p.Bandwidth != 128_000 || len(p.Segments) != 3 {
		t.Errorf("plan %+v", p)
	}
	if c := CodecsString(p, mono); c != "mp4a.40.2" {
		t.Errorf("codecs %q", c)
	}
	args, err := ffmpegArgs(Capabilities{Backend: BackendNVENC}, mono, p, 1, "/w/run")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{"-ss 4.000000 -i /m/book.mp3", "-map 0:0", "-c:a aac -b:a 128k -ac 1"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q: %s", want, joined)
		}
	}
	for _, bad := range []string{"-vf", "-c:v", "hwaccel", "-map 0:1", "-noaccurate_seek"} {
		if strings.Contains(joined, bad) {
			t.Errorf("audio-only args contain %q: %s", bad, joined)
		}
	}

	parts := mono
	parts.Parts = []string{"/m/01.mp3", "/m/02.mp3"}
	args, err = ffmpegArgs(Capabilities{}, parts, p, 0, "/w/run")
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(args, " "); !strings.Contains(joined, "-f concat -safe 0 -i /w/run/parts.txt") || strings.Contains(joined, "/m/book.mp3") {
		t.Errorf("concat args: %s", joined)
	}

	aac := Source{Path: "/m/book.m4b", Probe: Probe{Format: Format{Name: "mov,mp4,m4a", Duration: sec(10), BitRate: 64_000}, Streams: []Stream{
		{Index: 0, Type: "audio", Codec: "aac", Profile: "LC", Channels: 2},
	}}}
	p, err = NewPlan(aac, Target{Mode: ModeTranscode, AudioCopy: true, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Bandwidth != 76_800 || CodecsString(p, aac) != "mp4a.40.2" {
		t.Errorf("copy plan bandwidth %d codecs %q", p.Bandwidth, CodecsString(p, aac))
	}
}

func TestWriteConcatList(t *testing.T) {
	dir := t.TempDir()
	src := Source{Parts: []string{"/books/It's/01.mp3", "/books/b/02.mp3"}}
	if err := writeConcatList(src, dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, concatList))
	if err != nil {
		t.Fatal(err)
	}
	want := "ffconcat version 1.0\nfile '/books/It'\\''s/01.mp3'\nfile '/books/b/02.mp3'\n"
	if string(data) != want {
		t.Errorf("list %q, want %q", data, want)
	}
	if err := writeConcatList(Source{Parts: []string{"rel.mp3"}}, dir); err == nil {
		t.Error("relative part accepted")
	}
	if err := writeConcatList(Source{Parts: []string{"/a\nfile '/etc/passwd'"}}, dir); err == nil {
		t.Error("part with a newline accepted")
	}
}

// audioParts generates mono MP3 parts of the given lengths (seconds) with title tags and returns
// their paths and probes.
func audioParts(t *testing.T, r Runner, lengths ...int) ([]string, []Probe) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	var probes []Probe
	for i, l := range lengths {
		p := filepath.Join(dir, "0"+strconv.Itoa(i+1)+" - Teil.mp3")
		runFFmpeg(t, r, "-f", "lavfi", "-i", "sine=frequency="+strconv.Itoa(300+100*i)+":sample_rate=44100", "-t", strconv.Itoa(l),
			"-ac", "1", "-c:a", "libmp3lame", "-b:a", "64k", "-metadata", "title=Teil "+strconv.Itoa(i+1), p)
		pr, err := r.Probe(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
		probes = append(probes, pr)
	}
	return paths, probes
}

// probeBytes writes init and segments to a file and probes it.
func probeBytes(t *testing.T, r Runner, init []byte, segs [][]byte) Probe {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(path, slices.Concat(append([][]byte{init}, segs...)...), 0o600); err != nil {
		t.Fatal(err)
	}
	runFFmpeg(t, r, "-xerror", "-i", path, "-f", "null", "-")
	p, err := r.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAudioHLSConcatEndToEnd(t *testing.T) {
	r := hlsRunner(t)
	paths, probes := audioParts(t, r, 5, 6, 7)
	book := ConcatProbe(probes)
	src := Source{Path: paths[0], Parts: paths, Probe: book}
	p, err := NewPlan(src, Target{Mode: ModeTranscode, AudioBitrate: 128, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 5 {
		t.Fatalf("segments %+v", p.Segments)
	}
	m := newFFmpegManager(t, r, BackendSoftware)
	ctx := context.Background()
	s, err := m.Start(ctx, "01J9AUDIOCONCAT00000000000", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	read := reader(t)
	init := read(s.OpenInit(ctx))
	tracks := parseInit(t, init)
	if len(tracks) != 1 {
		t.Fatalf("init tracks %+v, want one audio track", tracks)
	}
	var segs [][]byte
	for i := range p.Segments {
		seg := read(s.OpenSegment(ctx, i))
		f := parseFragment(t, seg, tracks)
		a, ok := f[false]
		want := (p.Segments[i].Start + timelineOffset).Seconds()
		if !ok || a.pts < want-0.3 || a.pts > want+0.3 {
			t.Errorf("segment %d: audio fragment %+v, want it near %.3f s", i, a, want)
		}
		if bytes.Contains(seg, []byte("ftyp")) {
			t.Errorf("segment %d repeats the init segment", i)
		}
		segs = append(segs, seg)
	}
	out := probeBytes(t, r, init, segs)
	a, ok := audioStream(out)
	if !ok || a.Codec != "aac" || a.Channels != 1 || len(out.Streams) != 1 {
		t.Errorf("output streams %+v, want one mono AAC stream", out.Streams)
	}
	// The fragments keep the HLS timeline, which starts at timelineOffset.
	if d := out.Format.Duration - timelineOffset - book.Format.Duration; d < -500*time.Millisecond || d > 500*time.Millisecond {
		t.Errorf("output duration %v, want %v ±0.5 s", out.Format.Duration-timelineOffset, book.Format.Duration)
	}

	// A restart in the middle of the second part keeps the book timeline.
	stopAndForget(t, s, 2)
	seg := read(s.OpenSegment(ctx, 2))
	if a, ok := parseFragment(t, seg, tracks)[false]; !ok || a.pts < 18-0.3 || a.pts > 18+0.3 {
		t.Errorf("restarted segment 2: %+v, want it near 18 s", a)
	}
}

// M6 review M-4: the first part carries its cover as an attached picture after the audio (AAC mono
// 44.1 kHz, stream 0), the second has the picture as its first stream and the audio (AAC stereo
// 22.05 kHz) as stream 1. The concat demuxer maps packets by stream index, so the second part's
// audio would land in the picture slot and its picture in the audio decoder; the concat filter
// plays the whole book on its timeline.
func TestAudioHLSMixedPartsEndToEnd(t *testing.T) {
	r := hlsRunner(t)
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.jpg")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "color=c=red:s=64x48", "-frames:v", "1", cover)
	first := filepath.Join(dir, "01 - Teil.m4a")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "sine=frequency=300:sample_rate=44100", "-i", cover, "-t", "5", "-map", "0:a", "-map", "1:v",
		"-c:v", "copy", "-disposition:v:0", "attached_pic", "-ac", "1", "-c:a", "aac", "-b:a", "64k", "-f", "mp4", first)
	second := filepath.Join(dir, "02 - Teil.mka")
	runFFmpeg(t, r, "-i", cover, "-f", "lavfi", "-i", "sine=frequency=500:sample_rate=22050", "-t", "6", "-map", "0:v", "-map", "1:a",
		"-c:v", "copy", "-ac", "2", "-c:a", "aac", "-b:a", "64k", "-f", "matroska", second)
	ctx := context.Background()
	var probes []Probe
	for _, path := range []string{first, second} {
		pr, err := r.Probe(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		probes = append(probes, pr)
	}
	if len(probes[0].Streams) != 2 || !probes[0].Streams[1].AttachedPic || len(probes[1].Streams) != 2 ||
		probes[1].Streams[0].Type != "video" || probes[1].Streams[1].Type != "audio" || probes[1].Streams[1].SampleRate != 22050 {
		t.Fatalf("fixture streams %+v / %+v", probes[0].Streams, probes[1].Streams)
	}
	book := ConcatProbe(probes)
	if !book.Mixed() {
		t.Fatal("parts with different streams are not mixed")
	}
	src := Source{Path: first, Parts: []string{first, second}, Probe: book}
	p, err := NewPlan(src, Target{Mode: ModeTranscode, AudioBitrate: 128, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	m := newFFmpegManager(t, r, BackendSoftware)
	s, err := m.Start(ctx, "01J9AUDIOMIXED000000000000", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	read := reader(t)
	init := read(s.OpenInit(ctx))
	tracks := parseInit(t, init)
	var segs [][]byte
	for i := range p.Segments {
		seg := read(s.OpenSegment(ctx, i))
		a, ok := parseFragment(t, seg, tracks)[false]
		if want := (p.Segments[i].Start + timelineOffset).Seconds(); !ok || a.pts < want-0.3 || a.pts > want+0.3 {
			t.Errorf("segment %d: audio fragment %+v, want it near %.3f s", i, a, want)
		}
		segs = append(segs, seg)
	}
	out := probeBytes(t, r, init, segs)
	if a, ok := audioStream(out); !ok || a.Codec != "aac" || a.Channels != 1 || len(out.Streams) != 1 {
		t.Errorf("output streams %+v, want one mono AAC stream", out.Streams)
	}
	if d := out.Format.Duration - timelineOffset - book.Format.Duration; d < -500*time.Millisecond || d > 500*time.Millisecond {
		t.Errorf("output duration %v, want %v ±0.5 s", out.Format.Duration-timelineOffset, book.Format.Duration)
	}
	// A restart inside the second part keeps the book timeline.
	stopAndForget(t, s, 2)
	if a, ok := parseFragment(t, read(s.OpenSegment(ctx, 2)), tracks)[false]; !ok || a.pts < 18-0.3 || a.pts > 18+0.3 {
		t.Errorf("restarted segment 2: %+v, want it near 18 s", a)
	}
}

// coveredAudio generates an AAC M4A with a JPEG cover (attached picture) and two chapter marks.
func coveredAudio(t *testing.T, r Runner) string {
	t.Helper()
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.jpg")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "color=c=red:s=64x48", "-frames:v", "1", cover)
	meta := filepath.Join(dir, "chapters.txt")
	if err := os.WriteFile(meta, []byte(";FFMETADATA1\ntitle=Buch\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=4000\ntitle=Eins\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=4000\nEND=9000\ntitle=Zwei\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "book.m4b")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-i", cover, "-i", meta, "-t", "9",
		"-map", "0:a", "-map", "1:v", "-map_metadata", "2", "-map_chapters", "2", "-ac", "1", "-c:a", "aac", "-b:a", "64k",
		"-c:v", "copy", "-disposition:v:0", "attached_pic", "-f", "mp4", path)
	return path
}

func TestExtractCover(t *testing.T) {
	r := hlsRunner(t)
	ctx := context.Background()
	data, err := r.ExtractCover(ctx, coveredAudio(t, r))
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || cfg.Width != 64 || cfg.Height != 48 {
		t.Errorf("cover %s %dx%d: %v", format, cfg.Width, cfg.Height, err)
	}
	paths, _ := audioParts(t, r, 1)
	if _, err := r.ExtractCover(ctx, paths[0]); !errors.Is(err, ErrNoCover) {
		t.Errorf("file without cover: %v, want ErrNoCover", err)
	}
}

func TestAudioDownloadCopiesAAC(t *testing.T) {
	r := hlsRunner(t)
	ctx := context.Background()
	path := coveredAudio(t, r)
	probe, err := r.Probe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(probe.Chapters) != 2 || probe.Format.Tags["title"] != "Buch" {
		t.Fatalf("fixture probe chapters %+v tags %v", probe.Chapters, probe.Format.Tags)
	}
	src := Source{Path: path, Probe: probe}
	p, err := NewPlan(src, Target{Mode: ModeTranscode, AudioCopy: true, AudioStream: -1, SubtitleBurnIn: -1})
	if err != nil {
		t.Fatal(err)
	}
	var init []byte
	var segs [][]byte
	err = r.Download(ctx, Capabilities{}, src, p, t.TempDir(), 0,
		func(b []byte) error { init = b; return nil },
		func(_ int, b []byte, _, _ time.Duration) error { segs = append(segs, b); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != len(p.Segments) {
		t.Fatalf("%d segments, want %d", len(segs), len(p.Segments))
	}
	out := probeBytes(t, r, init, segs)
	a, ok := audioStream(out)
	if !ok || a.Codec != "aac" || len(out.Streams) != 1 {
		t.Errorf("output streams %+v, want the AAC stream only (no cover)", out.Streams)
	}
	if d := out.Format.Duration - timelineOffset - probe.Format.Duration; d < -500*time.Millisecond || d > 500*time.Millisecond {
		t.Errorf("duration %v, want %v ±0.5 s", out.Format.Duration-timelineOffset, probe.Format.Duration)
	}
}
