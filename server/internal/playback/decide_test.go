package playback

import (
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Capability profiles as the web client reports them (canPlayType / MediaSource.isTypeSupported,
// screen size in device pixels, the dynamic-range media query).
var (
	capsChrome = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9", "av1"},
		Audio: []string{"aac", "mp3", "opus", "flac"}, HLS: "mse", MaxWidth: 2560, MaxHeight: 1440, Type: "desktop"}
	capsChromeHEVC = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "hevc", "vp9", "av1"},
		Audio: []string{"aac", "mp3", "opus", "flac"}, HLS: "mse", MaxWidth: 2560, MaxHeight: 1440, Type: "desktop"}
	capsFirefox = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9", "av1"},
		Audio: []string{"aac", "mp3", "opus", "flac"}, HLS: "mse", MaxWidth: 1920, MaxHeight: 1080, Type: "desktop"}
	capsSafari = model.PlaybackCapabilities{Containers: []string{"mp4"}, Video: []string{"h264", "hevc"},
		Audio: []string{"aac", "ac3", "eac3", "mp3", "flac"}, HLS: "native", MaxWidth: 3024, MaxHeight: 1964, HDR: true, Type: "desktop"}
	capsSafariSDR = model.PlaybackCapabilities{Containers: []string{"mp4"}, Video: []string{"h264", "hevc"},
		Audio: []string{"aac", "ac3", "eac3", "mp3", "flac"}, HLS: "native", MaxWidth: 1920, MaxHeight: 1080, Type: "desktop"}
	capsAndroid = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "hevc", "vp9"},
		Audio: []string{"aac", "opus"}, HLS: "mse", MaxWidth: 1080, MaxHeight: 2400, Type: "phone"}
	capsAndroidNoHEVC = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9"},
		Audio: []string{"aac", "opus"}, HLS: "mse", MaxWidth: 720, MaxHeight: 1600}
)

// probeOf is a probe with cover art, the main video stream and an AAC track.
func probeOf(codec, transfer, primaries string) *transcode.Probe {
	return &transcode.Probe{Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "mjpeg", AttachedPic: true},
		{Index: 1, Type: "video", Codec: codec, ColorTransfer: transfer, ColorPrimaries: primaries},
		{Index: 2, Type: "audio", Codec: "aac", Default: true},
	}}
}

func sizedProbe(codec, transfer string, w, h int, bitrate int64) *transcode.Probe {
	return &transcode.Probe{Format: transcode.Format{BitRate: bitrate}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: codec, Width: w, Height: h, PixFmt: "yuv420p", ColorTransfer: transfer},
		{Index: 1, Type: "audio", Codec: "aac", Default: true, Channels: 2},
		{Index: 2, Type: "audio", Codec: "ac3", Channels: 6},
	}}
}

func pixProbe(codec, profile, pixFmt string) *transcode.Probe {
	return &transcode.Probe{Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: codec, Profile: profile, PixFmt: pixFmt, ColorTransfer: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Default: true},
	}}
}

func dvProbe(profile int) *transcode.Probe {
	return &transcode.Probe{Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084",
			DolbyVisionProfile: profile, DolbyVisionBLCompat: map[int]int{8: 1}[profile]},
		{Index: 1, Type: "audio", Codec: "eac3", Default: true},
	}}
}

// dvSDRProbe is Dolby Vision 8.2: an SDR (BT.709) base layer, compatibility id 2, 1080p.
func dvSDRProbe() *transcode.Probe {
	return &transcode.Probe{Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "hevc", Width: 1920, Height: 1080, PixFmt: "yuv420p10le", ColorTransfer: "bt709",
			DolbyVisionProfile: 8, DolbyVisionBLCompat: 2},
		{Index: 1, Type: "audio", Codec: "eac3", Default: true},
	}}
}

// req is a decision request with the capabilities and no limits, tracks or quality wish.
func req(caps model.PlaybackCapabilities) Request { return Request{Caps: caps, AudioStream: -1} }

// direct, remux and transcode build the expected decisions.
func direct(w, h, bitrate, audio int) Decision {
	return Decision{Mode: ModeDirect, Width: w, Height: h, Bitrate: bitrate, AudioStream: audio, BurnStream: -1}
}

func remux(reason string, audioCopy bool, tag string, w, h, bitrate, audio int) Decision {
	return Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: audioCopy, VideoTag: tag, Reason: reason, Quality: model.QualityAuto,
		Width: w, Height: h, Bitrate: bitrate, AudioStream: audio, BurnStream: -1}
}

func transcodeTo(reason, quality string, w, h, bitrate int, tonemap, audioCopy bool, audio int) Decision {
	return Decision{Mode: ModeTranscode, AudioCopy: audioCopy, MaxHeight: ladder[rungIndex(quality)].Height, Tonemap: tonemap, Reason: reason,
		Quality: quality, Width: w, Height: h, Bitrate: bitrate, AudioStream: audio, BurnStream: -1}
}

// Plan 10.3 per client (Chrome, Firefox, Safari, Android), rows 1–4; the subtitle rows 5 and 6
// follow in TestDecideSubtitles.
func TestDecideMatrix(t *testing.T) {
	h264MP4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	hevcSDR := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	hevcHDR := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	h264DTS := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "dts", Width: 1920, Height: 1080}
	h264MKV := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}
	sdr := sizedProbe("hevc", "bt709", 1920, 1080, 6_000_000)
	sdr.Streams[0].PixFmt = "yuv420p10le"
	hdr := sizedProbe("hevc", "smpte2084", 3840, 2160, 40_000_000)
	hdr.Streams[0].PixFmt = "yuv420p10le"

	for _, tc := range []struct {
		name  string
		file  model.MediaFile
		probe *transcode.Probe
		caps  model.PlaybackCapabilities
		want  Decision
	}{
		// Row 1: H.264/AAC MP4 1080p → direct play everywhere (a phone too: the device cap only
		// shapes transcodes).
		{"h264 mp4 chrome", h264MP4, nil, capsChrome, direct(1920, 1080, 0, -1)},
		{"h264 mp4 firefox", h264MP4, nil, capsFirefox, direct(1920, 1080, 0, -1)},
		{"h264 mp4 safari", h264MP4, nil, capsSafari, direct(1920, 1080, 0, -1)},
		{"h264 mp4 android", h264MP4, nil, capsAndroid, direct(1920, 1080, 0, -1)},
		// Row 2: HEVC 10-bit SDR MKV → remux where the client decodes HEVC, else transcode.
		{"hevc sdr chrome", hevcSDR, sdr, capsChrome, transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"hevc sdr chrome with hevc", hevcSDR, sdr, capsChromeHEVC, remux(ReasonContainer, true, "hvc1", 1920, 1080, 6_000_000, 1)},
		{"hevc sdr firefox", hevcSDR, sdr, capsFirefox, transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"hevc sdr safari", hevcSDR, sdr, capsSafari, remux(ReasonContainer, true, "hvc1", 1920, 1080, 6_000_000, 1)},
		{"hevc sdr android", hevcSDR, sdr, capsAndroid, remux(ReasonContainer, true, "hvc1", 1920, 1080, 6_000_000, 1)},
		{"hevc sdr android without hevc", hevcSDR, sdr, capsAndroidNoHEVC, transcodeTo(ReasonVideoCodec, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		// Row 3: HEVC HDR10 4K → tonemap to <= 1080p SDR; an HDR display with native HLS
		// (Safari) passes HDR through; the phone gets the 720p rung.
		{"hevc hdr chrome", hevcHDR, hdr, capsChromeHEVC, transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 1)},
		{"hevc hdr firefox", hevcHDR, hdr, capsFirefox, transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 1)},
		{"hevc hdr safari hdr display", hevcHDR, hdr, capsSafari, remux(ReasonContainer, true, "hvc1", 3840, 2160, 40_000_000, 1)},
		{"hevc hdr safari sdr display", hevcHDR, hdr, capsSafariSDR, transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 1)},
		{"hevc hdr android phone", hevcHDR, hdr, capsAndroid, transcodeTo(ReasonHDR, model.Quality720, 1280, 720, 4_000_000, true, true, 1)},
		{"hlg counts as hdr", hevcHDR, probeOf("hevc", "arib-std-b67", "bt2020"), capsFirefox,
			transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 2)},
		// The rule of transcode.NewPlan: only the PQ/HLG transfer is HDR, BT.2020 primaries alone are SDR.
		{"bt2020 primaries alone are sdr", hevcHDR, probeOf("hevc", "bt709", "bt2020"), capsFirefox,
			transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 2)},
		// Row 4: H.264 + DTS → audio transcode (remux with AAC) everywhere.
		{"h264 dts chrome", h264DTS, nil, capsChrome, remux(ReasonAudioCodec, false, "", 1920, 1080, 0, -1)},
		{"h264 dts firefox", h264DTS, nil, capsFirefox, remux(ReasonAudioCodec, false, "", 1920, 1080, 0, -1)},
		{"h264 dts safari", h264DTS, nil, capsSafari, remux(ReasonAudioCodec, false, "", 1920, 1080, 0, -1)},
		{"h264 dts android", h264DTS, nil, capsAndroid, remux(ReasonAudioCodec, false, "", 1920, 1080, 0, -1)},
		// H.264/AAC in MKV → remux in every browser; a client that plays MKV itself plays it directly.
		{"h264 mkv chrome", h264MKV, nil, capsChrome, remux(ReasonContainer, true, "", 0, 0, 0, -1)},
		{"h264 mkv safari", h264MKV, nil, capsSafari, remux(ReasonContainer, true, "", 0, 0, 0, -1)},
		{"h264 mkv on a client that plays mkv", h264MKV, nil, model.PlaybackCapabilities{Containers: []string{"mp4", "mkv"},
			Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}, direct(0, 0, 0, -1)},
		// Surround audio: listed codecs are copied.
		{"h264 ac3 safari", model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "ac3"}, nil, capsSafari,
			remux(ReasonContainer, true, "", 0, 0, 0, -1)},
		// Without a report: H.264 and AAC in MP4 through MSE.
		{"defaults play mp4", h264MP4, nil, defaultCapabilities, direct(1920, 1080, 0, -1)},
		{"defaults remux webm", model.MediaFile{Container: "webm", VideoCodec: "h264", AudioCodec: "aac"}, nil, defaultCapabilities,
			remux(ReasonContainer, true, "", 0, 0, 0, -1)},
		// Other codecs the client lists play directly in a container it plays; HEVC on Safari
		// needs the hvc1 tag of a remux.
		{"vp9 webm chrome", model.MediaFile{Container: "webm", VideoCodec: "vp9", AudioCodec: "opus"}, nil, capsChrome, direct(0, 0, 0, -1)},
		{"hevc mp4 chrome with hevc", model.MediaFile{Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}, nil, capsChromeHEVC, direct(0, 0, 0, -1)},
		{"hevc mp4 safari", model.MediaFile{Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}, sdr, capsSafari,
			remux(ReasonVideoCodec, true, "hvc1", 1920, 1080, 6_000_000, 1)},
		{"mpeg2 transcodes, ac3 encoded", model.MediaFile{Container: "mkv", VideoCodec: "mpeg2video", AudioCodec: "ac3"}, nil, capsSafari,
			transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, false, -1)},
		{"no hls support plays directly", hevcSDR, sdr, model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "none"},
			direct(1920, 1080, 6_000_000, 1)},
		// Pixel formats browsers do not decode: H.264 High 10 and 4:2:2/4:4:4 transcode.
		{"h264 high 10 mp4 not direct", h264MP4, pixProbe("h264", "High 10", "yuv420p10le"), capsChrome,
			transcodeTo(ReasonPixelFormat, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"h264 4:2:2 mkv", h264MKV, pixProbe("h264", "High 4:2:2", "yuv422p"), capsChrome,
			transcodeTo(ReasonPixelFormat, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"hevc 4:4:4 on safari", hevcSDR, pixProbe("hevc", "Rext", "yuv444p10le"), capsSafari,
			transcodeTo(ReasonPixelFormat, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"hevc main 10 remuxes", hevcSDR, pixProbe("hevc", "Main 10", "yuv420p10le"), capsSafari, remux(ReasonContainer, true, "hvc1", 0, 0, 0, 1)},
		{"mp4 without audio", model.MediaFile{Container: "m4v", VideoCodec: "h264"}, nil, capsFirefox, direct(0, 0, 0, -1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.file, tc.probe, req(tc.caps)); got != tc.want {
				t.Errorf("Decide =\n%+v, want\n%+v", got, tc.want)
			}
		})
	}
}

// Dolby Vision: profile 8 plays its HDR10 base layer (remux on an HDR display, tonemap else),
// profiles 5 and 7 always transcode (reason dolby_vision).
func TestDecideDolbyVision(t *testing.T) {
	file := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "eac3"}
	for _, tc := range []struct {
		name    string
		profile int
		caps    model.PlaybackCapabilities
		want    Decision
	}{
		{"profile 8 on safari hdr", 8, capsSafari, remux(ReasonContainer, true, "hvc1", 3840, 2160, 0, 1)},
		{"profile 8 on chrome hevc with hdr", 8, func() model.PlaybackCapabilities { c := capsChromeHEVC; c.HDR = true; return c }(),
			remux(ReasonContainer, false, "hvc1", 3840, 2160, 0, 1)},
		{"profile 8 on firefox", 8, capsFirefox, transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, false, 1)},
		{"profile 5 on safari hdr", 5, capsSafari, transcodeTo(ReasonDolbyVision, model.Quality1080, 1920, 1080, 8_000_000, true, false, 1)},
		{"profile 7 on chrome", 7, capsChromeHEVC, transcodeTo(ReasonDolbyVision, model.Quality1080, 1920, 1080, 8_000_000, true, false, 1)},
		{"profile 5 on a phone", 5, capsAndroid, transcodeTo(ReasonDolbyVision, model.Quality720, 1280, 720, 4_000_000, true, false, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(file, dvProbe(tc.profile), req(tc.caps)); got != tc.want {
				t.Errorf("Decide =\n%+v, want\n%+v", got, tc.want)
			}
		})
	}
	// Profile 8.2 (SDR base layer) is SDR: remuxed for a client without HDR, never tonemapped.
	sdr := dvSDRProbe()
	if !isHDR(mainVideo(dvProbe(8))) || isHDR(mainVideo(sdr)) || sdr.Streams[0].HDRFormat() != "" {
		t.Errorf("hdr: 8.1 %v, 8.2 %v %q", isHDR(mainVideo(dvProbe(8))), isHDR(mainVideo(sdr)), sdr.Streams[0].HDRFormat())
	}
	if got, want := Decide(file, sdr, req(capsChromeHEVC)), remux(ReasonContainer, false, "hvc1", 1920, 1080, 0, 1); got != want {
		t.Errorf("8.2 on chrome hevc without hdr = %+v, want %+v", got, want)
	}
	if got := Decide(file, sdr, req(capsFirefox)); got.Mode != ModeTranscode || got.Tonemap || got.Reason != ReasonVideoCodec {
		t.Errorf("8.2 on firefox = %+v, want a transcode without tonemapping", got)
	}
}

// W-8: limits of the profile and the manual quality choice.
func TestDecideLimitsAndQuality(t *testing.T) {
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}
	p1080 := sizedProbe("h264", "bt709", 1920, 1080, 10_000_000)
	p720 := sizedProbe("h264", "bt709", 1280, 720, 3_000_000)
	scope := sizedProbe("h264", "bt709", 1920, 800, 5_000_000)
	uhd := sizedProbe("h264", "bt709", 3840, 2160, 30_000_000)
	with := func(c model.PlaybackCapabilities, l model.PlaybackLimits, q string) Request {
		return Request{Caps: c, Limits: l, AudioStream: -1, Quality: q}
	}
	for _, tc := range []struct {
		name  string
		probe *transcode.Probe
		req   Request
		want  Decision
	}{
		{"max 720p transcodes a 1080p source to 720p", p1080, with(capsChrome, model.PlaybackLimits{MaxHeight: 720}, ""),
			transcodeTo(ReasonMaxHeight, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		{"max 720p keeps a 720p source", p720, with(capsChrome, model.PlaybackLimits{MaxHeight: 720}, ""), direct(1280, 720, 3_000_000, 1)},
		{"4 mbit/s caps a 10 mbit/s source at 480p (720p video and audio exceed it)", p1080, with(capsChrome, model.PlaybackLimits{MaxBitrateKbps: 4000}, ""),
			transcodeTo(ReasonBandwidth, model.Quality480, 852, 480, 2_000_000, false, true, 1)},
		{"4.2 mbit/s fits 720p with audio", p1080, with(capsChrome, model.PlaybackLimits{MaxBitrateKbps: 4200}, ""),
			transcodeTo(ReasonBandwidth, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		{"3 mbit/s keeps a 3 mbit/s source", p720, with(capsChrome, model.PlaybackLimits{MaxBitrateKbps: 3000}, ""), direct(1280, 720, 3_000_000, 1)},
		{"500 kbit/s takes the lowest rung at the limit minus the audio", p1080, with(capsChrome, model.PlaybackLimits{MaxBitrateKbps: 500}, ""),
			transcodeTo(ReasonBandwidth, model.Quality360, 640, 360, 340_000, false, true, 1)},
		{"manual 480p transcodes", p1080, with(capsChrome, model.PlaybackLimits{}, model.Quality480),
			transcodeTo(ReasonQuality, model.Quality480, 852, 480, 2_000_000, false, true, 1)},
		{"manual 1080p on a 1080p source is auto", p1080, with(capsChrome, model.PlaybackLimits{}, model.Quality1080), direct(1920, 1080, 10_000_000, 1)},
		{"manual 1080p under a 720p limit", p1080, with(capsChrome, model.PlaybackLimits{MaxHeight: 720}, model.Quality1080),
			transcodeTo(ReasonMaxHeight, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		{"manual 480p of a remux candidate", p1080, with(capsChrome, model.PlaybackLimits{}, model.Quality480),
			transcodeTo(ReasonQuality, model.Quality480, 852, 480, 2_000_000, false, true, 1)},
		{"cinemascope keeps its size on the 1080p rung", scope, with(capsChrome, model.PlaybackLimits{MaxHeight: 1080}, model.Quality1080),
			direct(1920, 800, 5_000_000, 1)},
		{"4k sdr with a 1080p limit", uhd, with(capsChrome, model.PlaybackLimits{MaxHeight: 1080}, ""),
			transcodeTo(ReasonMaxHeight, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"4k sdr plays directly without limits", uhd, with(capsChrome, model.PlaybackLimits{}, ""), direct(3840, 2160, 30_000_000, 1)},
		{"limits of a phone: 1080p limit, phone cap 720p", p1080, with(capsAndroid, model.PlaybackLimits{MaxBitrateKbps: 5000}, ""),
			transcodeTo(ReasonBandwidth, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(mp4, tc.probe, tc.req); got != tc.want {
				t.Errorf("Decide =\n%+v, want\n%+v", got, tc.want)
			}
		})
	}
}

// Audio tracks: the default track plays directly, another one is remuxed with that track (reason
// audio_track); an undecodable track is encoded.
func TestDecideAudioTrack(t *testing.T) {
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	p := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	pick := func(c model.PlaybackCapabilities, idx int) Request { return Request{Caps: c, AudioStream: idx} }
	if got, want := Decide(mp4, p, pick(capsChrome, 1)), direct(1920, 1080, 6_000_000, 1); got != want {
		t.Errorf("default track = %+v", got)
	}
	if got, want := Decide(mp4, p, pick(capsSafari, 2)), remux(ReasonAudioTrack, true, "", 1920, 1080, 6_000_000, 2); got != want {
		t.Errorf("ac3 track on safari = %+v, want %+v", got, want)
	}
	if got, want := Decide(mp4, p, pick(capsChrome, 2)), remux(ReasonAudioTrack, false, "", 1920, 1080, 6_000_000, 2); got != want {
		t.Errorf("ac3 track on chrome = %+v, want %+v", got, want)
	}
	mkv := mp4
	mkv.Container = "mkv"
	if got, want := Decide(mkv, p, pick(capsSafari, 2)), remux(ReasonContainer, true, "", 1920, 1080, 6_000_000, 2); got != want {
		t.Errorf("ac3 track of an mkv = %+v, want %+v", got, want)
	}
}

// Plan 10.3 rows 5 and 6: bitmap subtitles are burned in (transcode), text subtitles go as
// WebVTT next to direct play or remux; a text track can be burned in on request.
func TestDecideSubtitles(t *testing.T) {
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	mkv := mp4
	mkv.Container = "mkv"
	p := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	pgs := &model.Track{ID: 11, Index: 3, Kind: "subtitle", Codec: "hdmv_pgs_subtitle", Source: "embedded"}
	srt := &model.Track{ID: 12, Index: 4, Kind: "subtitle", Codec: "subrip", Source: "embedded", Text: true}
	ass := &model.Track{ID: 13, Index: -1, Kind: "subtitle", Codec: "ass", Source: "sidecar", RelPath: "Film/Film.de.ass", Text: true}
	sub := func(c model.PlaybackCapabilities, t *model.Track, delivery string) Request {
		return Request{Caps: c, AudioStream: -1, Subtitle: t, Delivery: delivery}
	}
	for _, caps := range []model.PlaybackCapabilities{capsChrome, capsFirefox, capsSafari, capsAndroid} {
		rung, w, h, br := model.Quality1080, 1920, 1080, 8_000_000
		if caps.Type == "phone" {
			rung, w, h, br = model.Quality720, 1280, 720, 4_000_000
		}
		burn := transcodeTo(ReasonBurnIn, rung, w, h, br, false, true, 1)
		burn.SubtitleTrack, burn.SubtitleDelivery, burn.BurnStream = 11, model.SubtitleBurn, 3
		if got := Decide(mp4, p, sub(caps, pgs, model.SubtitleAuto)); got != burn {
			t.Errorf("%s pgs = %+v, want %+v", caps.Type, got, burn)
		}
		text := direct(1920, 1080, 6_000_000, 1)
		text.SubtitleTrack, text.SubtitleDelivery = 12, model.SubtitleText
		if got := Decide(mp4, p, sub(caps, srt, "")); got != text {
			t.Errorf("%s srt = %+v, want %+v", caps.Type, got, text)
		}
		textRemux := remux(ReasonContainer, true, "", 1920, 1080, 6_000_000, 1)
		textRemux.SubtitleTrack, textRemux.SubtitleDelivery = 13, model.SubtitleText
		if got := Decide(mkv, p, sub(caps, ass, model.SubtitleText)); got != textRemux {
			t.Errorf("%s ass sidecar of an mkv = %+v, want %+v", caps.Type, got, textRemux)
		}
	}
	burnSidecar := transcodeTo(ReasonBurnIn, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)
	burnSidecar.SubtitleTrack, burnSidecar.SubtitleDelivery, burnSidecar.BurnSidecar = 13, model.SubtitleBurn, "Film/Film.de.ass"
	if got := Decide(mp4, p, sub(capsChrome, ass, model.SubtitleBurn)); got != burnSidecar {
		t.Errorf("burned sidecar = %+v, want %+v", got, burnSidecar)
	}
}

// Budget (plan 3.5): an SDR transcode above 720p runs one rung lower; HDR, remux and 720p do not.
func TestLoweredRung(t *testing.T) {
	limits := model.PlaybackLimits{MaxBitrateKbps: 3000}
	d := transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 3_000_000, false, true, 1)
	got, ok := lowered(d, 1920, 1080, limits)
	want := transcodeTo(ReasonBudgetLowered, model.Quality720, 1280, 720, 2_840_000, false, true, 1)
	if !ok || got != want {
		t.Errorf("lowered = %+v %v, want %+v", got, ok, want)
	}
	for _, d := range []Decision{
		transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 1),
		transcodeTo(ReasonVideoCodec, model.Quality720, 1280, 720, 4_000_000, false, true, 1),
		remux(ReasonContainer, true, "", 1920, 1080, 0, 1),
	} {
		if _, ok := lowered(d, 1920, 1080, model.PlaybackLimits{}); ok {
			t.Errorf("lowered %+v", d)
		}
	}
}

func TestQualityOptions(t *testing.T) {
	p := sizedProbe("h264", "bt709", 1920, 800, 5_000_000)
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}
	limits := model.PlaybackLimits{MaxHeight: 720}
	d := Decide(mp4, p, Request{Caps: capsChrome, Limits: limits, AudioStream: -1})
	opts := QualityOptions(d, 1920, 800, limits, capsChrome, "")
	var got []string
	for _, o := range opts {
		got = append(got, o.Quality)
		if o.Quality == model.Quality1080 && (o.Available || o.Width != 1920 || o.Height != 800) {
			t.Errorf("1080p = %+v (the source keeps 1920x800, the limit rules it out)", o)
		}
		if o.Quality == model.Quality720 && (!o.Available || o.Current || o.Width != 1728) {
			t.Errorf("720p = %+v", o)
		}
		if o.Quality == model.QualityAuto && (!o.Current || o.Height != 720 || o.Bitrate != 4_000_000) {
			t.Errorf("auto = %+v", o)
		}
	}
	if !slices.Equal(got, []string{"auto", "1080p", "720p", "480p", "360p"}) {
		t.Errorf("qualities = %v", got)
	}
	// A manual rung is current; a rung the source does not fill is unavailable.
	small := sizedProbe("h264", "bt709", 640, 360, 1_000_000)
	d = Decide(mp4, small, Request{Caps: capsChrome, AudioStream: -1, Quality: model.Quality360})
	for _, o := range QualityOptions(d, 640, 360, model.PlaybackLimits{}, capsChrome, model.Quality360) {
		if want := o.Quality == model.Quality360; o.Current != want || (o.Quality == model.Quality480 && o.Available) {
			t.Errorf("small source option %+v", o)
		}
	}
}

func TestNormalizeCapabilities(t *testing.T) {
	if got := normalizeCapabilities(nil); got.HLS != "mse" || len(got.Video) != 1 || got.Video[0] != "h264" || got.Audio[0] != "aac" {
		t.Errorf("absent report = %+v", got)
	}
	long := ""
	for range 100 {
		long += "ä"
	}
	got := normalizeCapabilities(&model.PlaybackCapabilities{Video: []string{"H264", "HEVC", "hevc"}, Audio: []string{"AAC"}, HLS: "bogus",
		Containers: []string{"MP4"}, Name: long, Type: "toaster", MaxHeight: -5})
	if got.HLS != "mse" || !slices.Equal(got.Video, []string{"h264", "hevc"}) || got.Audio[0] != "aac" || got.Containers[0] != "mp4" ||
		len([]rune(got.Name)) != maxDeviceName || got.Type != "" || got.MaxHeight != 0 {
		t.Errorf("normalized = %+v", got)
	}
}

func TestDeviceCap(t *testing.T) {
	for _, tc := range []struct {
		caps model.PlaybackCapabilities
		want int
	}{
		{model.PlaybackCapabilities{}, 0},
		{model.PlaybackCapabilities{MaxWidth: 1920, MaxHeight: 1080}, 0},
		{model.PlaybackCapabilities{MaxWidth: 1366, MaxHeight: 768}, phoneRung},
		{model.PlaybackCapabilities{MaxWidth: 720, MaxHeight: 1600}, phoneRung},
		{model.PlaybackCapabilities{MaxWidth: 1440, MaxHeight: 3200, Type: "phone"}, phoneRung},
	} {
		if got := deviceCap(tc.caps); got != tc.want {
			t.Errorf("deviceCap(%+v) = %d, want %d", tc.caps, got, tc.want)
		}
	}
}
