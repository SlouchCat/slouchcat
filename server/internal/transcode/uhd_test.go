package transcode

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Tests of the 2160p rung (ADR 0021): the height limit per backend, the H.264 level and codec
// string of the output height, the admission cost, the availability rule and the argument lists.

func TestMaxOutputHeight(t *testing.T) {
	cases := []struct {
		b          Backend
		cpuTonemap bool
		want       int
	}{
		{BackendSoftware, false, 1080},
		{BackendSoftware, true, 1080},
		{"", false, 1080}, // unknown backend counts as software
		{"amf", false, 1080},
		{BackendNVENC, false, 2160},
		{BackendQSV, false, 2160},
		{BackendVAAPI, false, 2160},
		{BackendNVENC, true, 1080},
		{BackendQSV, true, 1080},
		{BackendVAAPI, true, 1080},
	}
	for _, c := range cases {
		if got := MaxOutputHeight(c.b, c.cpuTonemap); got != c.want {
			t.Errorf("MaxOutputHeight(%q, cpuTonemap %v) = %d, want %d", c.b, c.cpuTonemap, got, c.want)
		}
	}
}

func TestHeightLimitPerBackend(t *testing.T) {
	cases := []struct {
		name string
		t    Target
		want int
	}{
		{"software keeps 1080", Target{Backend: BackendSoftware, MaxHeight: 2160}, 1080},
		{"software with an explicit 2160p", Target{Backend: BackendSoftware, MaxHeight: 2160, Quality: "2160p"}, 1080},
		{"no backend keeps 1080", Target{MaxHeight: 2160, Quality: "2160p"}, 1080},
		{"qsv 2160", Target{Backend: BackendQSV, MaxHeight: 2160}, 2160},
		{"nvenc 2160p rung", Target{Backend: BackendNVENC, MaxHeight: 2160, Quality: "2160p"}, 2160},
		{"vaapi 2160", Target{Backend: BackendVAAPI, MaxHeight: 2160}, 2160},
		{"hardware caps above 2160", Target{Backend: BackendNVENC, MaxHeight: 4320}, 2160},
		{"hardware auto stays at 1080", Target{Backend: BackendQSV}, 1080},
		{"hardware auto keyword stays at 1080", Target{Backend: BackendQSV, Quality: "auto"}, 1080},
		{"2160p rung without a raised MaxHeight", Target{Backend: BackendQSV, Quality: "2160p"}, 1080},
		{"hardware lower rung", Target{Backend: BackendQSV, MaxHeight: 2160, Quality: "720p"}, 720},
		{"hardware 1440 lines", Target{Backend: BackendQSV, MaxHeight: 1440}, 1440},
		{"cpu tonemap on qsv", Target{Backend: BackendQSV, MaxHeight: 2160, Tonemap: true}, 1080},
		{"cpu tonemap on nvenc", Target{Backend: BackendNVENC, MaxHeight: 2160, Quality: "2160p", Tonemap: true}, 1080},
		{"gpu tonemap on nvenc", Target{Backend: BackendNVENC, MaxHeight: 2160, Tonemap: true, HWTonemap: true}, 2160},
		{"gpu tonemap on qsv", Target{Backend: BackendQSV, MaxHeight: 2160, Tonemap: true, HWTonemap: true}, 2160},
		{"gpu tonemap flag on software", Target{Backend: BackendSoftware, MaxHeight: 2160, Tonemap: true, HWTonemap: true}, 1080},
		{"software tonemap", Target{Backend: BackendSoftware, Tonemap: true}, 1080},
		{"software below 1080", Target{Backend: BackendSoftware, MaxHeight: 480}, 480},
	}
	for _, c := range cases {
		got, err := heightLimit(c.t)
		if err != nil || got != c.want {
			t.Errorf("%s: heightLimit = %d, %v, want %d", c.name, got, err, c.want)
		}
	}
	if _, err := heightLimit(Target{Backend: BackendQSV, MaxHeight: 2160, Quality: "4320p"}); err == nil {
		t.Error("want error for an unknown quality")
	}
}

func TestNewPlan2160(t *testing.T) {
	uhd := videoSource("hevc", 3840, 2160, "bt709", sec(10))
	scope := videoSource("hevc", 3840, 1600, "bt709", sec(10))
	hdr := videoSource("hevc", 3840, 2160, "smpte2084", sec(10))
	hd := videoSource("hevc", 1920, 1080, "bt709", sec(10))
	cases := []struct {
		name     string
		src      Source
		t        Target
		w, h, br int
		quality  string
		cost     float64
	}{
		{"qsv 4K SDR", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Quality: "2160p"}, 3840, 2160, 25_000_000, "2160p", 5},
		{"qsv 4K SDR with a stray tonemap flag", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Tonemap: true}, 3840, 2160, 25_000_000, "2160p", 5},
		{"qsv scope 4K", scope, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160}, 3840, 1600, 25_000_000, "2160p", 5},
		{"qsv 4K bandwidth cap", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, MaxBitrate: 12_000_000}, 3840, 2160, 12_000_000, "2160p", 5},
		{"qsv 4K auto", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV}, 1920, 1080, 8_000_000, "1080p", 3},
		{"qsv 4K HDR cpu tonemap", hdr, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Quality: "2160p", Tonemap: true}, 1920, 1080, 8_000_000, "1080p", 4},
		{"nvenc 4K HDR gpu tonemap", hdr, Target{Mode: ModeTranscode, Backend: BackendNVENC, HWTonemap: true, MaxHeight: 2160, Quality: "2160p", Tonemap: true}, 3840, 2160, 25_000_000, "2160p", 5},
		{"software 4K SDR", uhd, Target{Mode: ModeTranscode, Backend: BackendSoftware, MaxHeight: 2160, Quality: "2160p"}, 1920, 1080, 8_000_000, "1080p", 3},
		{"qsv 1080p source never upscales", hd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Quality: "2160p"}, 1920, 1080, 8_000_000, "1080p", 2},
	}
	for _, c := range cases {
		p, err := NewPlan(c.src, c.t)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if p.Width != c.w || p.Height != c.h || p.VideoBitrate != c.br || p.Bandwidth != c.br+audioBitrate ||
			p.Target.Quality != c.quality || p.Cost != c.cost {
			t.Errorf("%s: %dx%d rate %d bw %d quality %s cost %v", c.name, p.Width, p.Height, p.VideoBitrate,
				p.Bandwidth, p.Target.Quality, p.Cost)
		}
		if got := Cost(c.src, c.t); got != c.cost {
			t.Errorf("%s: Cost before planning = %v, want %v", c.name, got, c.cost)
		}
	}
}

// The software backend never encodes above 1080 lines, and neither does tonemapping on the CPU,
// whatever the target asks for.
func TestNo2160OnSoftwareOrCPUTonemap(t *testing.T) {
	sources := map[string]Source{
		"4K SDR": videoSource("hevc", 3840, 2160, "bt709", sec(10)),
		"4K PQ":  videoSource("hevc", 3840, 2160, "smpte2084", sec(10)),
		"4K HLG": videoSource("hevc", 3840, 2160, "arib-std-b67", sec(10)),
		"8K SDR": videoSource("hevc", 7680, 4320, "bt709", sec(10)),
	}
	dv5 := videoSource("hevc", 3840, 2160, "", sec(10))
	dv5.Probe.Streams[0].DolbyVisionProfile = 5
	sources["4K DV5"] = dv5
	for name, src := range sources {
		for _, maxHeight := range []int{0, 1080, 1440, 2160, 4320} {
			for _, q := range []string{"", "auto", "2160p", "1080p"} {
				for _, tonemap := range []bool{false, true} {
					for _, hw := range []bool{false, true} {
						sw := Target{Mode: ModeTranscode, Backend: BackendSoftware, MaxHeight: maxHeight, Quality: q, Tonemap: tonemap, HWTonemap: hw}
						if p, err := NewPlan(src, sw); err != nil || p.Height > 1080 || p.Cost == cost2160p {
							t.Errorf("software %s %+v: height %d cost %v, %v", name, sw, p.Height, p.Cost, err)
						}
						if !tonemap || !isHDR(src.Probe.Streams[0]) {
							continue
						}
						for _, b := range []Backend{BackendNVENC, BackendQSV, BackendVAAPI} {
							cpu := Target{Mode: ModeTranscode, Backend: b, MaxHeight: maxHeight, Quality: q, Tonemap: true}
							if p, err := NewPlan(src, cpu); err != nil || p.Height > 1080 || p.Cost == cost2160p {
								t.Errorf("cpu tonemap %s %+v: height %d cost %v, %v", name, cpu, p.Height, p.Cost, err)
							}
						}
					}
				}
			}
		}
	}
}

// The level follows the macroblocks per frame and per second (MaxFS and MaxMBPS of H.264 table
// A-1), not the height alone: 2592x1080 exceeds the 8192 macroblocks of level 4.1 and needs 5.1;
// 1080p at 50 or 60 fps gets 4.2 (final review stage 2, G2-M2). Without a frame rate the frame
// size decides alone, but above 1080 lines it is at least 5.2 (G2-m2).
func TestH264Level(t *testing.T) {
	for _, c := range []struct {
		w, h int
		fps  float64
		want int
	}{
		{0, 0, 0, 41}, {640, 360, 0, 41}, {852, 480, 25, 41}, {1280, 720, 60, 41}, {1920, 800, 0, 41},
		{1728, 720, 24, 41}, {2048, 1024, 0, 41}, {1920, 1080, 0, 41}, {1920, 1080, 23.976, 41},
		{1920, 1080, 30, 41}, {1920, 1080, 50, 42}, {1920, 1080, 59.94, 42}, {1920, 1080, 60, 42},
		{2048, 1040, 0, 42}, {2048, 1080, 60, 42}, {2112, 1080, 60, 51}, {2592, 1080, 0, 51}, {2592, 1080, 24, 51}, {2560, 1440, 30, 51},
		{3840, 1600, 24, 51}, {3840, 2160, 30, 51}, {4096, 2160, 24, 51},
		{2560, 1440, 0, 52}, {3840, 1600, 0, 52}, {3840, 2160, 0, 52},
		{3840, 2160, 50, 52}, {3840, 2160, 60, 52}, {5120, 2160, 0, 52},
	} {
		if got := h264Level(c.w, c.h, c.fps); got != c.want {
			t.Errorf("h264Level(%d, %d, %v) = %d, want %d", c.w, c.h, c.fps, got, c.want)
		}
	}
	src := videoSource("hevc", 3840, 2160, "bt709", sec(10))
	fast := videoSource("hevc", 3840, 2160, "bt709", sec(10))
	fast.Probe.Streams[0].FrameRate = 59.94
	for _, c := range []struct {
		src  Source
		w, h int
		want string
	}{
		{src, 0, 0, "avc1.640029"}, {src, 1280, 720, "avc1.640029"}, {src, 1920, 1080, "avc1.640029"},
		{src, 2592, 1080, "avc1.640033"}, {src, 3840, 1600, "avc1.640034"}, {src, 3840, 2160, "avc1.640034"},
		{fast, 1920, 1080, "avc1.64002A"}, {fast, 3840, 1600, "avc1.640034"}, {fast, 3840, 2160, "avc1.640034"},
	} {
		p := Plan{Target: Target{Mode: ModeTranscode}, Width: c.w, Height: c.h}
		if got := CodecsString(p, c.src); got != c.want+",mp4a.40.2" {
			t.Errorf("CodecsString at %dx%d = %q, want %s,mp4a.40.2", c.w, c.h, got, c.want)
		}
	}
	// A copied video keeps the codec string of the source, also at 2160 lines.
	p := Plan{Target: Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, VideoTag: "hvc1"}, Height: 2160}
	if got := CodecsString(p, src); got != "hvc1.2.4.L150.B0,mp4a.40.2" {
		t.Errorf("CodecsString of a 2160p remux = %q", got)
	}
}

func TestCost2160(t *testing.T) {
	uhd := videoSource("hevc", 3840, 2160, "bt709", sec(10))
	hdr := videoSource("hevc", 3840, 2160, "smpte2084", sec(10))
	cases := []struct {
		name string
		src  Source
		t    Target
		want float64
	}{
		{"qsv 2160p", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160}, 5},
		{"nvenc 2160p", uhd, Target{Mode: ModeTranscode, Backend: BackendNVENC, MaxHeight: 2160, Quality: "2160p"}, 5},
		{"vaapi 2160p", uhd, Target{Mode: ModeTranscode, Backend: BackendVAAPI, MaxHeight: 2160}, 5},
		{"nvenc 2160p HDR gpu tonemap", hdr, Target{Mode: ModeTranscode, Backend: BackendNVENC, HWTonemap: true, MaxHeight: 2160, Tonemap: true}, 5},
		{"qsv 4K to 1080p", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 1080}, 3},
		{"qsv 4K to 720p", uhd, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Quality: "720p"}, 3},
		{"qsv 4K HDR cpu tonemap", hdr, Target{Mode: ModeTranscode, Backend: BackendQSV, MaxHeight: 2160, Tonemap: true}, 4},
		{"software 4K", uhd, Target{Mode: ModeTranscode, Backend: BackendSoftware, MaxHeight: 2160}, 3},
		{"qsv remux", uhd, Target{Mode: ModeRemux, VideoCopy: true, Backend: BackendQSV, MaxHeight: 2160}, 0.5},
	}
	for _, c := range cases {
		if got := Cost(c.src, c.t); got != c.want {
			t.Errorf("%s: Cost = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRung2160Allowed(t *testing.T) {
	qsvCPU := Capabilities{Backend: BackendQSV, Tonemapper: TonemapCPU} // CT 190
	sdr := Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 2160, ColorTransfer: "bt709"}
	pq := Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 2160, ColorTransfer: "smpte2084"}
	dv5 := Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 2160, DolbyVisionProfile: 5}
	cases := []struct {
		name            string
		caps            Capabilities
		v               Stream
		profile, device int
		want            bool
	}{
		{"all conditions hold", qsvCPU, sdr, 2160, 2160, true},
		{"no profile limit", qsvCPU, sdr, 0, 1440, true},
		{"nvenc", Capabilities{Backend: BackendNVENC}, sdr, 2160, 1440, true},
		{"vaapi", Capabilities{Backend: BackendVAAPI}, sdr, 2160, 1440, true},
		// A 4K source has 2160 lines or 3840 pixels of width (Marco's decision of 4 November 2026).
		{"scope 4K source 3840x1600", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 1600}, 2160, 2160, true},
		{"3840x2159", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 2159}, 2160, 2160, true},
		{"narrow 4K 2880x2160", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 2880, Height: 2160}, 2160, 2160, true},
		{"2560x1440", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 2560, Height: 1440}, 2160, 2160, false},
		{"3836x1600", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 3836, Height: 1600}, 2160, 2160, false},
		{"scope 4K HDR with cpu tonemap", qsvCPU, Stream{Type: "video", Codec: "hevc", Width: 3840, Height: 1600, ColorTransfer: "smpte2084"}, 2160, 2160, false},
		{"1080p source", qsvCPU, Stream{Type: "video", Codec: "h264", Width: 1920, Height: 1080}, 2160, 2160, false},
		{"software backend", Capabilities{Backend: BackendSoftware}, sdr, 2160, 2160, false},
		{"no backend", Capabilities{}, sdr, 2160, 2160, false},
		{"profile limit 1080", qsvCPU, sdr, 1080, 2160, false},
		{"device below 1440", qsvCPU, sdr, 2160, 1439, false},
		{"device size unknown", qsvCPU, sdr, 2160, 0, false},
		{"HDR with cpu tonemap", qsvCPU, pq, 2160, 2160, false},
		{"Dolby Vision 5 with cpu tonemap", qsvCPU, dv5, 2160, 2160, false},
		{"HDR with gpu tonemap", Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCUDA, HWTonemap: true}, pq, 2160, 2160, true},
		{"HDR with vpp_qsv tonemap", Capabilities{Backend: BackendQSV, Tonemapper: TonemapVPP, HWTonemap: true}, pq, 2160, 2160, true},
	}
	for _, c := range cases {
		if got := Rung2160Allowed(c.caps, c.v, c.profile, c.device); got != c.want {
			t.Errorf("%s: Rung2160Allowed = %v, want %v", c.name, got, c.want)
		}
	}
}

// ffprobe's avg_frame_rate of a video stream becomes Stream.FrameRate for the H.264 level.
func TestFrameRate(t *testing.T) {
	for _, c := range []struct {
		kind, in string
		want     float64
	}{
		{"video", "24000/1001", 23.976}, {"video", "25/1", 25}, {"video", "60000/1001", 59.94}, {"video", "30", 30},
		{"video", "0/0", 0}, {"video", "", 0}, {"video", "x/y", 0}, {"video", "25/0", 0}, {"audio", "0/0", 0}, {"audio", "25/1", 0},
	} {
		if got := frameRate(c.kind, c.in); got != c.want {
			t.Errorf("frameRate(%q, %q) = %v, want %v", c.kind, c.in, got, c.want)
		}
	}
	p, err := ParseProbe([]byte(`{"format":{"duration":"10"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"50/1"}]}`))
	if err != nil || p.Streams[0].FrameRate != 50 {
		t.Errorf("parsed probe = %+v, %v", p.Streams, err)
	}
}

func TestIs4KSource(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want bool
	}{
		{3840, 1600, true}, {3840, 2160, true}, {4096, 1716, true}, {2880, 2160, true},
		{2560, 1440, false}, {3836, 1600, false}, {1920, 1080, false}, {0, 0, false},
	} {
		if got := Is4KSource(c.w, c.h); got != c.want {
			t.Errorf("Is4KSource(%d, %d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

// Golden argument lists of 2160p transcodes per hardware backend (up to the output seek, as in
// TestFFmpegArgsGoldenM4): the output size, the 25 Mbit/s rung and the level (5.2 for 2160p
// without a frame rate in the probe, G2-m2).
func TestFFmpegArgsGolden2160(t *testing.T) {
	sdr := Source{Path: "/media/UHD.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}}
	hdr := Source{Path: "/media/UHD.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084", ColorPrimaries: "bt2020"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}}
	scope := Source{Path: "/media/UHD.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 1600, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}}
	sdr60 := Source{Path: "/media/UHD.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, FrameRate: 59.94, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}}
	sw := Capabilities{Backend: BackendSoftware, Tonemapper: TonemapCPU}
	nvenc := Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCPU}
	cuda := Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCUDA, HWTonemap: true}
	qsvCPU := Capabilities{Backend: BackendQSV, Tonemapper: TonemapCPU}
	vaapi := Capabilities{Backend: BackendVAAPI, Tonemapper: TonemapCPU}
	// uhd is the target of an explicit 2160p choice on the host caps.
	uhd := func(caps Capabilities, tonemap bool) Target {
		return Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Tonemap: tonemap,
			Backend: caps.Backend, HWTonemap: caps.HWTonemap, MaxHeight: 2160, Quality: "2160p"}
	}
	const (
		x264  = "-c:v libx264 -preset veryfast -crf 23 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v %[3]s -g 9999 -sc_threshold 0"
		nvEnc = "-c:v h264_nvenc -preset p4 -tune hq -rc vbr -cq 23 -b:v 0 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v %[3]s -g 9999 -no-scenecut 1 -forced-idr 1"
		qsEnc = "-c:v h264_qsv -preset veryfast -global_quality 23 -look_ahead 0 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v %[3]s -g 9999 -forced_idr 1"
		vaEnc = "-c:v h264_vaapi -rc_mode VBR -b:v %[1]d -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v %[3]s -g 9999"
		aac   = "-c:a aac -b:a 160k -ac 2"
		head  = "-nostdin -hide_banner -loglevel warning -copyts"
		cudaI = "-hwaccel cuda -hwaccel_output_format cuda"
		qsvI  = "-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv"
		vaI   = "-init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device va"
		cpuTM = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
		maps  = " -i /media/UHD.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf "
		kf    = " -force_key_frames 4.000000,8.000000 " + aac
	)
	enc := func(f string, rate int, level string) string { return fmt.Sprintf(f, rate, 2*rate, level) }
	cases := []struct {
		name string
		caps Capabilities
		src  Source
		t    Target
		want string
	}{
		{"nvenc 2160p", nvenc, sdr, uhd(nvenc, false),
			head + " " + cudaI + maps + "scale_cuda=w=3840:h=2160:format=nv12 " + enc(nvEnc, 25_000_000, "5.2") + kf},
		{"nvenc 2160p hdr tonemap_cuda", cuda, hdr, uhd(cuda, true),
			head + " " + cudaI + maps + "scale_cuda=w=3840:h=2160," + cudaTonemapFilter + " " + enc(nvEnc, 25_000_000, "5.2") + kf},
		{"qsv 2160p", qsvCPU, sdr, uhd(qsvCPU, false),
			head + " " + qsvI + maps + "vpp_qsv=w=3840:h=2160:format=nv12 " + enc(qsEnc, 25_000_000, "52") + kf},
		{"vaapi 2160p", vaapi, sdr, uhd(vaapi, false),
			head + " " + vaI + maps + "scale_vaapi=w=3840:h=2160:format=nv12 " + enc(vaEnc, 25_000_000, "5.2") + kf},
		// The cpu tonemapping on QSV (CT 190) and the software backend stay at 1080p and level 4.1.
		{"qsv 2160p hdr cpu tonemap stays 1080p", qsvCPU, hdr, uhd(qsvCPU, true),
			head + " " + qsvI + maps + "vpp_qsv=w=1920:h=1080:format=p010,hwdownload,format=p010le," + cpuTM + ",format=nv12,hwupload=extra_hw_frames=64 " + enc(qsEnc, 8_000_000, "41") + kf},
		{"software 2160p stays 1080p", sw, sdr, uhd(sw, false),
			head + maps + "scale=w=1920:h=1080:flags=bicubic,format=yuv420p " + enc(x264, 8_000_000, "4.1") + kf},
		// A scope 4K film keeps its 3840x1600 on the 2160p rung (no upscaling), at level 5.2 without
		// a frame rate; its 1080p output fits the 16:9 box of 1920x1080 (1920x800, level 4.1, G2-m1).
		{"qsv scope 2160p", qsvCPU, scope, uhd(qsvCPU, false),
			head + " " + qsvI + maps + "vpp_qsv=w=3840:h=1600:format=nv12 " + enc(qsEnc, 25_000_000, "52") + kf},
		{"qsv scope 1080p", qsvCPU, scope, Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Backend: BackendQSV, MaxHeight: 1080, Quality: "1080p"},
			head + " " + qsvI + maps + "vpp_qsv=w=1920:h=800:format=nv12 " + enc(qsEnc, 8_000_000, "41") + kf},
		{"nvenc scope 1080p", nvenc, scope, Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Backend: BackendNVENC, MaxHeight: 1080, Quality: "1080p"},
			head + " " + cudaI + maps + "scale_cuda=w=1920:h=800:format=nv12 " + enc(nvEnc, 8_000_000, "4.1") + kf},
		{"vaapi scope 1080p", vaapi, scope, Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Backend: BackendVAAPI, MaxHeight: 1080, Quality: "1080p"},
			head + " " + vaI + maps + "scale_vaapi=w=1920:h=800:format=nv12 " + enc(vaEnc, 8_000_000, "4.1") + kf},
		{"software scope stays 1080p", sw, scope, uhd(sw, false),
			head + maps + "scale=w=1920:h=800:flags=bicubic,format=yuv420p " + enc(x264, 8_000_000, "4.1") + kf},
		// At 59.94 fps 2160p needs level 5.2 and 1080p level 4.2 (macroblocks per second, G2-M2).
		{"qsv 2160p60", qsvCPU, sdr60, uhd(qsvCPU, false),
			head + " " + qsvI + maps + "vpp_qsv=w=3840:h=2160:format=nv12 " + enc(qsEnc, 25_000_000, "52") + kf},
		{"software 1080p60", sw, sdr60, uhd(sw, false),
			head + maps + "scale=w=1920:h=1080:flags=bicubic,format=yuv420p " + enc(x264, 8_000_000, "4.2") + kf},
		{"qsv 1080p60", qsvCPU, sdr60, Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Backend: BackendQSV, MaxHeight: 1080, Quality: "1080p"},
			head + " " + qsvI + maps + "vpp_qsv=w=1920:h=1080:format=nv12 " + enc(qsEnc, 8_000_000, "42") + kf},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(c.src, c.t)
			if err != nil {
				t.Fatal(err)
			}
			args, err := ffmpegArgs(c.caps, c.src, p, 0, "/w/s/run0001")
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(args, " ")
			got = got[:strings.Index(got, " -output_ts_offset")]
			if got != c.want {
				t.Errorf("args differ\ngot:  %s\nwant: %s", got, c.want)
			}
		})
	}
}
