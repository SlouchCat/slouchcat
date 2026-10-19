package playback

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Capability sets as the web client reports them (canPlayType / MediaSource.isTypeSupported).
var (
	capsChrome        = model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}
	capsChromeHEVC    = model.PlaybackCapabilities{Video: []string{"h264", "hevc"}, Audio: []string{"aac"}, HLS: "mse"}
	capsFirefox       = model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}
	capsSafari        = model.PlaybackCapabilities{Video: []string{"h264", "hevc"}, Audio: []string{"aac", "ac3", "eac3"}, HLS: "native"}
	capsAndroid       = model.PlaybackCapabilities{Video: []string{"h264", "hevc"}, Audio: []string{"aac"}, HLS: "mse"}
	capsAndroidNoHEVC = model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}
)

func probeOf(codec, transfer, primaries string) *transcode.Probe {
	return &transcode.Probe{Streams: []transcode.Stream{
		{Type: "video", Codec: "mjpeg", AttachedPic: true},
		{Type: "video", Codec: codec, ColorTransfer: transfer, ColorPrimaries: primaries},
		{Type: "audio", Codec: "aac"},
	}}
}

// Plan 10.3 rows 1–3 per client, plus the H.264 MKV (remux) and the surround audio files.
func TestDecideMatrix(t *testing.T) {
	h264MP4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}
	hevcSDR := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	hevcHDR := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	h264MKV := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}
	h264DTS := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "dts"}
	h264AC3 := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "ac3"}
	sdr := probeOf("hevc", "bt709", "bt709")
	hdr := probeOf("hevc", "smpte2084", "bt2020")

	remuxHEVC := Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, VideoTag: "hvc1", Reason: ReasonContainer}
	transcodeSDR := Decision{Mode: ModeTranscode, MaxHeight: 1080, AudioCopy: true, Reason: ReasonVideoCodec}
	tonemap := Decision{Mode: ModeTranscode, MaxHeight: 1080, Tonemap: true, AudioCopy: true, Reason: ReasonHDR}
	remuxH264 := Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, Reason: ReasonContainer}
	remuxAudioEncode := Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: false, Reason: ReasonContainer}

	for _, tc := range []struct {
		name  string
		file  model.MediaFile
		probe *transcode.Probe
		caps  model.PlaybackCapabilities
		want  Decision
	}{
		// Row 1: H.264/AAC MP4 → direct play everywhere.
		{"h264 mp4 chrome", h264MP4, nil, capsChrome, Decision{Mode: ModeDirect}},
		{"h264 mp4 firefox", h264MP4, nil, capsFirefox, Decision{Mode: ModeDirect}},
		{"h264 mp4 safari", h264MP4, nil, capsSafari, Decision{Mode: ModeDirect}},
		{"h264 mp4 android", h264MP4, nil, capsAndroid, Decision{Mode: ModeDirect}},
		// Row 2: HEVC 10-bit SDR MKV → remux where the client decodes HEVC, else transcode.
		{"hevc sdr chrome", hevcSDR, sdr, capsChrome, transcodeSDR},
		{"hevc sdr chrome with hevc", hevcSDR, sdr, capsChromeHEVC, remuxHEVC},
		{"hevc sdr firefox", hevcSDR, sdr, capsFirefox, transcodeSDR},
		{"hevc sdr safari", hevcSDR, sdr, capsSafari, remuxHEVC},
		{"hevc sdr android", hevcSDR, sdr, capsAndroid, remuxHEVC},
		{"hevc sdr android without hevc", hevcSDR, sdr, capsAndroidNoHEVC, transcodeSDR},
		// Row 3: HEVC HDR10 4K → tonemap to <= 1080p SDR; Safari passes HDR through (remux).
		{"hevc hdr chrome", hevcHDR, hdr, capsChromeHEVC, tonemap},
		{"hevc hdr firefox", hevcHDR, hdr, capsFirefox, tonemap},
		{"hevc hdr safari", hevcHDR, hdr, capsSafari, remuxHEVC},
		{"hevc hdr android", hevcHDR, hdr, capsAndroid, tonemap},
		{"hlg counts as hdr", hevcHDR, probeOf("hevc", "arib-std-b67", "bt2020"), capsFirefox, tonemap},
		{"bt2020 primaries count as hdr", hevcHDR, probeOf("hevc", "", "bt2020"), capsFirefox, tonemap},
		// H.264/AAC in MKV → remux in every browser.
		{"h264 mkv chrome", h264MKV, nil, capsChrome, remuxH264},
		{"h264 mkv firefox", h264MKV, nil, capsFirefox, remuxH264},
		{"h264 mkv safari", h264MKV, nil, capsSafari, remuxH264},
		// Surround audio the client cannot decode is encoded to AAC; listed codecs are copied.
		{"h264 dts firefox", h264DTS, nil, capsFirefox, remuxAudioEncode},
		{"h264 ac3 chrome", h264AC3, nil, capsChrome, remuxAudioEncode},
		{"h264 ac3 safari", h264AC3, nil, capsSafari, remuxH264},
		// Edge cases.
		{"mp4 with ac3 on chrome", model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "ac3"}, nil, capsChrome,
			Decision{Mode: ModeRemux, VideoCopy: true, Reason: ReasonAudioCodec}},
		{"hevc mp4 on safari", model.MediaFile{Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}, sdr, capsSafari,
			Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, VideoTag: "hvc1", Reason: ReasonVideoCodec}},
		{"mpeg2 transcodes, ac3 encoded", model.MediaFile{Container: "mkv", VideoCodec: "mpeg2video", AudioCodec: "ac3"}, nil, capsSafari,
			Decision{Mode: ModeTranscode, MaxHeight: 1080, Reason: ReasonVideoCodec}},
		{"no hls support plays directly", hevcSDR, sdr, model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "none"},
			Decision{Mode: ModeDirect}},
		{"mp4 without audio", model.MediaFile{Container: "m4v", VideoCodec: "h264"}, nil, capsFirefox, Decision{Mode: ModeDirect}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.file, tc.probe, tc.caps); got != tc.want {
				t.Errorf("Decide = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNormalizeCapabilities(t *testing.T) {
	if got := normalizeCapabilities(nil); got.HLS != "mse" || len(got.Video) != 1 || got.Video[0] != "h264" || got.Audio[0] != "aac" {
		t.Errorf("absent report = %+v", got)
	}
	got := normalizeCapabilities(&model.PlaybackCapabilities{Video: []string{"H264", "HEVC"}, Audio: []string{"AAC"}, HLS: "bogus"})
	if got.HLS != "mse" || got.Video[1] != "hevc" || got.Audio[0] != "aac" {
		t.Errorf("normalized = %+v", got)
	}
}
