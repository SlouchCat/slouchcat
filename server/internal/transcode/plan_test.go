package transcode

import (
	"slices"
	"testing"
	"time"
)

func sec(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

func videoSource(codec string, w, h int, transfer string, d time.Duration, kf ...time.Duration) Source {
	return Source{
		Path: "/m/f.mkv",
		Probe: Probe{Format: Format{Duration: d, BitRate: 10_000_000}, Streams: []Stream{
			{Index: 0, Type: "video", Codec: codec, Profile: "Main 10", Level: 150, Width: w, Height: h, PixFmt: "yuv420p10le", ColorTransfer: transfer},
			{Index: 1, Type: "audio", Codec: "aac", Profile: "LC", Channels: 6},
		}},
		Keyframes: kf,
	}
}

func starts(p Plan) []time.Duration {
	var s []time.Duration
	var total time.Duration
	for _, seg := range p.Segments {
		s = append(s, seg.Start)
		total += seg.Duration
	}
	if total != p.Duration {
		panic("segment durations do not add up to the plan duration")
	}
	return s
}

func TestNewPlanTranscodeGrid(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want []time.Duration
	}{
		{sec(10.021), []time.Duration{0, sec(4), sec(8)}},           // tail 2.021 s stays
		{sec(12.5), []time.Duration{0, sec(4), sec(8)}},             // tail 0.5 s merged
		{sec(13), []time.Duration{0, sec(4), sec(8), sec(12)}},      // tail of exactly 1 s stays
		{sec(16), []time.Duration{0, sec(4), sec(8), sec(12)}},      // exact multiple
		{sec(0.5), []time.Duration{0}},                              // shorter than one segment
		{sec(5941.2), nil},                                          // checked by length below
		{sec(3.9), []time.Duration{0}},                              // single segment
		{sec(4.5), []time.Duration{0}},                              // 0.5 s tail merged into the only segment
		{sec(8), []time.Duration{0, sec(4)}},                        // no empty last segment
		{sec(9), []time.Duration{0, sec(4), sec(8)}},                // 1 s tail stays
		{sec(4) + time.Millisecond, []time.Duration{0}},             // 1 ms tail merged
		{sec(8) + 999*time.Millisecond, []time.Duration{0, sec(4)}}, // 0.999 s tail merged
	}
	for _, c := range cases {
		p, err := NewPlan(videoSource("hevc", 1920, 1080, "bt709", c.d), Target{Mode: ModeTranscode})
		if err != nil {
			t.Fatal(err)
		}
		got := starts(p)
		if c.want == nil {
			if len(got) != 1486 || got[1485] != sec(5940) {
				t.Errorf("%v: %d segments, last at %v", c.d, len(got), got[len(got)-1])
			}
			continue
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%v: starts %v, want %v", c.d, got, c.want)
		}
	}
}

func TestNewPlanRemuxKeyframes(t *testing.T) {
	kf := []time.Duration{0, sec(2), sec(5), sec(7), sec(9), sec(13), sec(14), sec(18)}
	p, err := NewPlan(videoSource("h264", 1920, 1080, "", sec(20), kf...), Target{Mode: ModeRemux, VideoCopy: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := starts(p), []time.Duration{0, sec(5), sec(9), sec(13), sec(18)}; !slices.Equal(got, want) {
		t.Errorf("starts %v, want %v", got, want)
	}
	if last := p.Segments[len(p.Segments)-1]; last.Duration != sec(2) {
		t.Errorf("last segment %v, want 2 s", last)
	}

	// A keyframe at the very end does not open an empty segment; long GOPs give long segments.
	p, _ = NewPlan(videoSource("h264", 1920, 1080, "", sec(30), 0, sec(10), sec(30)), Target{Mode: ModeRemux, VideoCopy: true})
	if got, want := starts(p), []time.Duration{0, sec(10)}; !slices.Equal(got, want) {
		t.Errorf("long GOP starts %v, want %v", got, want)
	}

	// Without a keyframe index remux falls back to the 4-s grid.
	p, _ = NewPlan(videoSource("h264", 1920, 1080, "", sec(10)), Target{Mode: ModeRemux, VideoCopy: true})
	if got, want := starts(p), []time.Duration{0, sec(4), sec(8)}; !slices.Equal(got, want) {
		t.Errorf("nil index starts %v, want %v", got, want)
	}
}

func TestNewPlanValidation(t *testing.T) {
	src := videoSource("hevc", 1920, 1080, "", sec(10))
	for _, tg := range []Target{
		{Mode: "dash"},
		{Mode: ModeTranscode, VideoCopy: true},
		{Mode: ModeRemux},
	} {
		if _, err := NewPlan(src, tg); err == nil {
			t.Errorf("NewPlan(%+v) = nil error", tg)
		}
	}
	if _, err := NewPlan(Source{Probe: Probe{Format: Format{Duration: sec(10)}}}, Target{Mode: ModeTranscode}); err == nil {
		t.Error("want error for a source without streams")
	}
	if _, err := NewPlan(videoSource("hevc", 1920, 1080, "", 0), Target{Mode: ModeTranscode}); err == nil {
		t.Error("want error for an unknown duration")
	}
}

func TestNewPlanOutput(t *testing.T) {
	cases := []struct {
		name          string
		src           Source
		t             Target
		w, h, bw      int
		tonemap       bool
		cost          float64
		maxHeightWant int
	}{
		{"4K HDR tonemap", videoSource("hevc", 3840, 2160, "smpte2084", sec(10)), Target{Mode: ModeTranscode, Tonemap: true}, 1920, 1080, 8_160_000, true, 4, 1080},
		{"4K HDR without tonemap request", videoSource("hevc", 3840, 2160, "smpte2084", sec(10)), Target{Mode: ModeTranscode}, 1920, 1080, 8_160_000, false, 3, 1080},
		{"4K SDR", videoSource("hevc", 3840, 1600, "bt709", sec(10)), Target{Mode: ModeTranscode, MaxHeight: 2160}, 2592, 1080, 8_160_000, false, 3, 1080},
		{"1080p SDR tonemap ignored", videoSource("hevc", 1920, 1080, "bt709", sec(10)), Target{Mode: ModeTranscode, Tonemap: true}, 1920, 1080, 8_160_000, false, 2, 1080},
		{"1080p to 720p", videoSource("hevc", 1920, 800, "", sec(10)), Target{Mode: ModeTranscode, MaxHeight: 720}, 1728, 720, 4_160_000, false, 1.5, 720},
		{"odd size", videoSource("mpeg4", 721, 405, "", sec(10)), Target{Mode: ModeTranscode}, 720, 404, 4_160_000, false, 2, 1080},
		{"remux", videoSource("hevc", 3840, 2160, "smpte2084", sec(10)), Target{Mode: ModeRemux, VideoCopy: true}, 3840, 2160, 12_000_000, false, 0.5, 0},
	}
	for _, c := range cases {
		p, err := NewPlan(c.src, c.t)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if p.Width != c.w || p.Height != c.h || p.Bandwidth != c.bw || p.Target.Tonemap != c.tonemap ||
			p.Cost != c.cost || p.Target.MaxHeight != c.maxHeightWant {
			t.Errorf("%s: %dx%d bw %d tonemap %v cost %v maxHeight %d", c.name, p.Width, p.Height, p.Bandwidth,
				p.Target.Tonemap, p.Cost, p.Target.MaxHeight)
		}
	}
}

func TestCost(t *testing.T) {
	audioOnly := Source{Probe: Probe{Streams: []Stream{{Type: "audio", Codec: "flac"}, {Type: "video", Codec: "mjpeg", AttachedPic: true}}}}
	if got := Cost(audioOnly, Target{Mode: ModeTranscode}); got != 0.25 {
		t.Errorf("audio only cost %v, want 0.25", got)
	}
	hdr := videoSource("hevc", 3840, 2160, "arib-std-b67", sec(10))
	if got := Cost(hdr, Target{Mode: ModeTranscode, Tonemap: true, MaxHeight: 720}); got != 4 {
		t.Errorf("4K HLG tonemap to 720p cost %v, want 4 (output size does not matter)", got)
	}
	if got := Cost(hdr, Target{Mode: ModeRemux, VideoCopy: true}); got != 0.5 {
		t.Errorf("remux cost %v, want 0.5", got)
	}
}

func TestCodecsString(t *testing.T) {
	h264 := Source{Probe: Probe{Streams: []Stream{
		{Type: "video", Codec: "h264", Profile: "High", Level: 40},
		{Type: "audio", Codec: "ac3", Default: true},
	}}}
	hevc := videoSource("hevc", 1920, 1080, "", sec(10))
	hevc.Probe.Streams[0].Level = 120
	main8 := Source{Probe: Probe{Streams: []Stream{{Type: "video", Codec: "hevc", Profile: "Main", Level: 93}}}}
	cases := []struct {
		name string
		p    Plan
		src  Source
		want string
	}{
		{"transcode", Plan{Target: Target{Mode: ModeTranscode}}, hevc, "avc1.640029,mp4a.40.2"},
		{"remux h264 ac3", Plan{Target: Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}}, h264, "avc1.640028,ac-3"},
		{"remux h264 aac encode", Plan{Target: Target{Mode: ModeRemux, VideoCopy: true}}, h264, "avc1.640028,mp4a.40.2"},
		{"remux hevc main10 hvc1", Plan{Target: Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, VideoTag: "hvc1"}}, hevc, "hvc1.2.4.L120.B0,mp4a.40.2"},
		{"remux hevc main hev1 no audio", Plan{Target: Target{Mode: ModeRemux, VideoCopy: true}}, main8, "hev1.1.6.L93.B0"},
	}
	for _, c := range cases {
		if got := CodecsString(c.p, c.src); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSeekMargin(t *testing.T) {
	kf := []time.Duration{0, sec(9.9), sec(10), sec(20)}
	cases := map[time.Duration]time.Duration{
		sec(10): 50 * time.Millisecond, // half the distance to 9.9 s
		sec(20): remuxSeekMargin,
		0:       remuxSeekMargin, // no earlier keyframe
		sec(15): remuxSeekMargin, // not a keyframe (grid fallback)
	}
	for b, want := range cases {
		if got := seekMargin(kf, b); got != want {
			t.Errorf("seekMargin(%v) = %v, want %v", b, got, want)
		}
	}
	if _, err := ffmpegArgs(Capabilities{}, videoSource("h264", 640, 360, "", sec(10)), Plan{Segments: []Segment{{0, sec(10)}}}, 1, "/x"); err == nil {
		t.Error("want error for a segment out of range")
	}
}
