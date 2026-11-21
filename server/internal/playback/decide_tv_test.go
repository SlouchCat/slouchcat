package playback

import (
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Concept 26 stage 1 (ADR 0041): the three capability fields maxAudioChannels, selectsTracks and
// subtitles, and the audio of HLS outputs. Every rule has its row for a client that reports the
// same codecs without the new fields: it gets the decision of before.

// tvCaps is the Android TV app with a 5.1 sound system that passes AC-3 and E-AC-3 through
// (concept 26 5.2, stage 0 point 2); tvOld the same report without the new fields.
func tvCaps() model.PlaybackCapabilities {
	return model.PlaybackCapabilities{Containers: []string{"mp4", "mkv", "webm", "ts", "m2ts"}, Video: []string{"h264", "hevc"},
		Audio: []string{"aac", "ac3", "eac3", "opus", "flac"}, HLS: "mse", MaxWidth: 1920, MaxHeight: 1080, Type: "tv",
		MaxAudioChannels: 6, SelectsTracks: true, Subtitles: []string{"subrip", "ass", "webvtt", "hdmv_pgs_subtitle", "dvd_subtitle"}}
}

func tvOld() model.PlaybackCapabilities {
	c := tvCaps()
	c.MaxAudioChannels, c.SelectsTracks, c.Subtitles = 0, false, nil
	return c
}

// with returns caps changed by f.
func with(c model.PlaybackCapabilities, f func(*model.PlaybackCapabilities)) model.PlaybackCapabilities {
	f(&c)
	return c
}

// tvProbe is a 60-s 1080p H.264 file: a stereo AAC track (default) and a second track with the
// given codec, channels and bit rate (0 = not probed).
func tvProbe(transfer, codec string, channels int, bitrate int64) *transcode.Probe {
	return &transcode.Probe{Format: transcode.Format{Duration: time.Minute, BitRate: 6_000_000}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080, PixFmt: "yuv420p", ColorTransfer: transfer},
		{Index: 1, Type: "audio", Codec: "aac", Default: true, Channels: 2},
		{Index: 2, Type: "audio", Codec: codec, Channels: channels, BitRate: bitrate},
		{Index: 3, Type: "subtitle", Codec: "hdmv_pgs_subtitle"},
		{Index: 4, Type: "subtitle", Codec: "ass"},
	}}
}

// codecsOf is the CODECS attribute of the master playlist of decision d (hlsTarget, NewPlan).
func codecsOf(t *testing.T, p *transcode.Probe, d Decision) string {
	t.Helper()
	if d.Mode == ModeDirect {
		return ""
	}
	src := transcode.Source{Probe: *p, Keyframes: []time.Duration{0, 4 * time.Second, 8 * time.Second}}
	plan, err := transcode.NewPlan(src, hlsTarget(HLSRequest{Decision: d}, transcode.Capabilities{}))
	if err != nil {
		t.Fatal(err)
	}
	return transcode.CodecsString(plan, src)
}

func TestDecideTVCapabilities(t *testing.T) {
	t.Parallel()
	mkv := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	avi := mkv
	avi.Container = "avi" // a container the TV does not play directly: remux
	pgs := &model.Track{ID: 11, Index: 3, Kind: "subtitle", Codec: "hdmv_pgs_subtitle", Source: "embedded"}
	ass := &model.Track{ID: 12, Index: 4, Kind: "subtitle", Codec: "ass", Source: "embedded", Text: true}
	sidecar := &model.Track{ID: 13, Index: -1, Kind: "subtitle", Codec: "ass", Source: "sidecar", RelPath: "F/F.de.ass", Text: true}
	hdr := "smpte2084"
	limit := func(kbps int) model.PlaybackLimits { return model.PlaybackLimits{MaxBitrateKbps: kbps} }

	type want struct {
		mode, reason  string
		copyAudio     bool
		codec         string // Decision.AudioCodec
		channels      int
		audioBitrate  int // Decision.AudioBitrate
		videoBitrate  int // Decision.Bitrate of a transcode, 0 = not checked
		delivery      string
		codecs        string // CODECS of the master playlist
		subtitleTrack int64
		burnStream    int
		audioStream   int
	}
	for _, tc := range []struct {
		name     string
		file     model.MediaFile
		probe    *transcode.Probe
		caps     model.PlaybackCapabilities
		audio    int // AudioStream of the request, -1 default
		sub      *model.Track
		delivery string
		limits   model.PlaybackLimits
		want     want
	}{
		// selectsTracks: another audio track plays directly; without the field it remuxes.
		{"other track, selects", mkv, tvProbe("bt709", "ac3", 6, 640_000), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeDirect, audioStream: 2, burnStream: -1}},
		{"other track, before", mkv, tvProbe("bt709", "ac3", 6, 640_000), tvOld(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonAudioTrack, copyAudio: true, codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},

		// subtitles: PGS and ASS of the file are drawn by the player in direct play; before, PGS
		// burned in (transcode) and ASS went as WebVTT. A sidecar, an explicit delivery and HLS
		// stay as before.
		{"pgs, drawn", mkv, tvProbe("bt709", "ac3", 6, 0), tvCaps(), -1, pgs, model.SubtitleAuto, model.PlaybackLimits{},
			want{mode: ModeDirect, subtitleTrack: 11, burnStream: -1, audioStream: 1}},
		{"pgs, before", mkv, tvProbe("bt709", "ac3", 6, 0), tvOld(), -1, pgs, model.SubtitleAuto, model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonBurnIn, copyAudio: true, delivery: model.SubtitleBurn, subtitleTrack: 11, burnStream: 3,
				audioStream: 1, codecs: "avc1.640029,mp4a.40.2"}},
		{"ass, drawn", mkv, tvProbe("bt709", "ac3", 6, 0), tvCaps(), -1, ass, "", model.PlaybackLimits{},
			want{mode: ModeDirect, subtitleTrack: 12, burnStream: -1, audioStream: 1}},
		{"ass, before", mkv, tvProbe("bt709", "ac3", 6, 0), tvOld(), -1, ass, "", model.PlaybackLimits{},
			want{mode: ModeDirect, delivery: model.SubtitleText, subtitleTrack: 12, burnStream: -1, audioStream: 1}},
		{"ass asked as text", mkv, tvProbe("bt709", "ac3", 6, 0), tvCaps(), -1, ass, model.SubtitleText, model.PlaybackLimits{},
			want{mode: ModeDirect, delivery: model.SubtitleText, subtitleTrack: 12, burnStream: -1, audioStream: 1}},
		{"ass sidecar", mkv, tvProbe("bt709", "ac3", 6, 0), tvCaps(), -1, sidecar, "", model.PlaybackLimits{},
			want{mode: ModeDirect, delivery: model.SubtitleText, subtitleTrack: 13, burnStream: -1, audioStream: 1}},
		{"pgs in a remux", avi, tvProbe("bt709", "ac3", 6, 0), tvCaps(), -1, pgs, "", model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonBurnIn, copyAudio: true, delivery: model.SubtitleBurn, subtitleTrack: 11, burnStream: 3,
				audioStream: 1, codecs: "avc1.640029,mp4a.40.2"}},

		// Audio copy next to encoded video (HDR source on an SDR TV): AC-3 and E-AC-3 are kept with
		// their bit rate (probed, else the format's bound); before, stereo AAC.
		{"ac3 transcode", mkv, tvProbe(hdr, "ac3", 6, 448_000), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonHDR, copyAudio: true, audioBitrate: 448_000, videoBitrate: 8_000_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},
		{"eac3 transcode, unprobed rate", mkv, tvProbe(hdr, "eac3", 6, 0), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonHDR, copyAudio: true, audioBitrate: 1_024_000, videoBitrate: 8_000_000,
				codecs: "avc1.640029,ec-3", audioStream: 2, burnStream: -1}},
		{"ac3 transcode, before", mkv, tvProbe(hdr, "ac3", 6, 448_000), tvOld(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonHDR, videoBitrate: 8_000_000, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"dts transcode", mkv, tvProbe(hdr, "dts", 6, 1_536_000), with(tvCaps(), func(c *model.PlaybackCapabilities) {
			c.Audio = append(c.Audio, "dts")
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeTranscode, reason: ReasonHDR, codec: transcode.AudioAC3, channels: 6, audioBitrate: 640_000, videoBitrate: 8_000_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},

		// Bandwidth: the copied track counts with its own rate. 8 Mbit/s of 1080p plus 640 kbit/s
		// of AC-3 exceed 8.5 Mbit/s: 720p; with stereo AAC (before) 1080p fits.
		{"ac3 transcode under a limit", mkv, tvProbe(hdr, "ac3", 6, 640_000), tvCaps(), 2, nil, "", limit(8500),
			want{mode: ModeTranscode, reason: ReasonHDR, copyAudio: true, audioBitrate: 640_000, videoBitrate: 4_000_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},
		{"ac3 transcode under a limit, before", mkv, tvProbe(hdr, "ac3", 6, 640_000), tvOld(), 2, nil, "", limit(8500),
			want{mode: ModeTranscode, reason: ReasonHDR, videoBitrate: 8_000_000, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"ac3 transcode under a tight limit", mkv, tvProbe(hdr, "ac3", 6, 640_000), tvCaps(), 2, nil, "", limit(2500),
			want{mode: ModeTranscode, reason: ReasonHDR, copyAudio: true, audioBitrate: 640_000, videoBitrate: 1_000_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},

		// Multichannel encode of what cannot be copied (a remux of DTS the TV does not report): AC-3
		// 5.1 where the TV reports ac3, else AAC 5.1; AAC 7.1 for 8 channels and a 7.1 source;
		// never more channels than the source; before, stereo AAC.
		{"dts remux, ac3", avi, tvProbe("bt709", "dts", 6, 0), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codec: transcode.AudioAC3, channels: 6, audioBitrate: 640_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},
		{"dts remux, aac 5.1", avi, tvProbe("bt709", "dts", 6, 0), with(tvCaps(), func(c *model.PlaybackCapabilities) {
			c.Audio = []string{"aac"}
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, channels: 6, audioBitrate: 384_000, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"truehd 7.1 remux, 8 channels", avi, tvProbe("bt709", "truehd", 8, 0), with(tvCaps(), func(c *model.PlaybackCapabilities) {
			c.Audio, c.MaxAudioChannels = append(c.Audio, "truehd"), 8
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, channels: 8, audioBitrate: 512_000, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"truehd 7.1 remux, 6 channels", avi, tvProbe("bt709", "truehd", 8, 0), with(tvCaps(), func(c *model.PlaybackCapabilities) {
			c.Audio = append(c.Audio, "truehd")
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codec: transcode.AudioAC3, channels: 6, audioBitrate: 640_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},
		{"quad remux, never more channels", avi, tvProbe("bt709", "pcm_s16le", 4, 0), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codec: transcode.AudioAC3, channels: 4, audioBitrate: 640_000,
				codecs: "avc1.640029,ac-3", audioStream: 2, burnStream: -1}},
		{"stereo source stays stereo", avi, tvProbe("bt709", "mp3", 2, 0), tvCaps(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"2 channels reported", avi, tvProbe("bt709", "dts", 6, 0), with(tvCaps(), func(c *model.PlaybackCapabilities) {
			c.MaxAudioChannels = 2
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
		{"dts remux, before", avi, tvProbe("bt709", "dts", 6, 0), tvOld(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},

		// Codec strings of copied formats in a remux (before: AAC for all but AC-3 and E-AC-3);
		// TrueHD is never copied into HLS, also for a client without the new fields.
		{"dts copied", avi, tvProbe("bt709", "dts", 6, 0), with(tvOld(), func(c *model.PlaybackCapabilities) {
			c.Audio = append(c.Audio, "dts")
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, copyAudio: true, codecs: "avc1.640029,mp4a.a9", audioStream: 2, burnStream: -1}},
		{"flac copied", avi, tvProbe("bt709", "flac", 2, 0), tvOld(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, copyAudio: true, codecs: "avc1.640029,fLaC", audioStream: 2, burnStream: -1}},
		{"opus copied", avi, tvProbe("bt709", "opus", 6, 0), tvOld(), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, copyAudio: true, codecs: "avc1.640029,Opus", audioStream: 2, burnStream: -1}},
		{"truehd never copied", avi, tvProbe("bt709", "truehd", 8, 0), with(tvOld(), func(c *model.PlaybackCapabilities) {
			c.Audio = append(c.Audio, "truehd")
		}), 2, nil, "", model.PlaybackLimits{},
			want{mode: ModeRemux, reason: ReasonContainer, codecs: "avc1.640029,mp4a.40.2", audioStream: 2, burnStream: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Request{Caps: normalizeCapabilities(&tc.caps), Limits: tc.limits, AudioStream: tc.audio, Subtitle: tc.sub, Delivery: tc.delivery}
			d := Decide(tc.file, tc.probe, r)
			w := tc.want
			got := want{mode: d.Mode, reason: d.Reason, copyAudio: d.AudioCopy, codec: d.AudioCodec, channels: d.AudioChannels,
				audioBitrate: d.AudioBitrate, delivery: d.SubtitleDelivery, codecs: codecsOf(t, tc.probe, d),
				subtitleTrack: d.SubtitleTrack, burnStream: d.BurnStream, audioStream: d.AudioStream}
			if w.videoBitrate > 0 {
				got.videoBitrate = d.Bitrate
			}
			if d.Mode == ModeDirect {
				got.copyAudio = false
			}
			if got != w {
				t.Errorf("Decide =\n%+v, want\n%+v", got, w)
			}
		})
	}
}

// The bandwidth of the master playlist of a transcode counts the copied track with its rate, as
// the decision does.
func TestTranscodeBandwidthWithCopiedAudio(t *testing.T) {
	t.Parallel()
	mkv := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	p := tvProbe("smpte2084", "eac3", 6, 768_000)
	caps := tvCaps()
	d := Decide(mkv, p, Request{Caps: normalizeCapabilities(&caps), AudioStream: 2})
	src := transcode.Source{Probe: *p}
	plan, err := transcode.NewPlan(src, hlsTarget(HLSRequest{Decision: d}, transcode.Capabilities{}))
	if err != nil {
		t.Fatal(err)
	}
	if want := d.Bitrate + 768_000; plan.Bandwidth != want || d.AudioBitrate != 768_000 {
		t.Errorf("bandwidth %d, audio %d, want %d and 768000", plan.Bandwidth, d.AudioBitrate, want)
	}
}

func TestNormalizeTVCapabilities(t *testing.T) {
	t.Parallel()
	got := normalizeCapabilities(&model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse",
		MaxAudioChannels: 6, SelectsTracks: true, Subtitles: []string{"PGS", "ass", "srt", "hdmv_pgs_subtitle", "VobSub", ""}})
	if got.MaxAudioChannels != 6 || !got.SelectsTracks ||
		!slices.Equal(got.Subtitles, []string{"hdmv_pgs_subtitle", "ass", "subrip", "dvd_subtitle"}) {
		t.Errorf("normalized = %+v", got)
	}
	for _, n := range []int{0, 1, 4, 7, 16} {
		if got := normalizeCapabilities(&model.PlaybackCapabilities{HLS: "mse", MaxAudioChannels: n}); got.MaxAudioChannels != 0 {
			t.Errorf("maxAudioChannels %d normalized to %d, want 0 (not reported)", n, got.MaxAudioChannels)
		}
	}
	if got := normalizeCapabilities(nil); got.MaxAudioChannels != 0 || got.SelectsTracks || got.Subtitles != nil {
		t.Errorf("absent report = %+v", got)
	}
}
