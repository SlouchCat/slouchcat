package offline

import (
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

func profile(q string) model.DownloadProfile {
	p, ok := model.DownloadProfileFor(q)
	if !ok {
		panic(q)
	}
	return p
}

// src is a media file with its probe.
type src struct {
	file  model.MediaFile
	probe *transcode.Probe
}

// media builds a probe: one video stream and one audio stream of the given formats.
func media(codec, pix, transfer string, dv, w, h int, bitRate int64, audio string, channels int) src {
	p := &transcode.Probe{Format: transcode.Format{Duration: time.Hour, BitRate: bitRate}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: codec, PixFmt: pix, ColorTransfer: transfer, DolbyVisionProfile: dv, Width: w, Height: h},
		{Index: 1, Type: "audio", Codec: audio, Channels: channels, Default: true},
	}}
	return src{model.MediaFile{VideoCodec: codec, AudioCodec: audio, Width: w, Height: h, DurationMs: time.Hour.Milliseconds()}, p}
}

func TestDecidePerProfileAndSource(t *testing.T) {
	type want struct {
		copy, audioCopy, tonemap bool
		w, h, videoKbps          int
	}
	for _, tc := range []struct {
		name    string
		src     src
		quality string
		want    want
	}{
		// 1080p H.264 at 5 Mbit/s (video 4840): copied in high, encoded below; AAC stereo copied.
		{"h264 1080p high", media("h264", "yuv420p", "bt709", 0, 1920, 1080, 5_000_000, "aac", 2), model.DownloadHigh,
			want{copy: true, audioCopy: true, w: 1920, h: 1080, videoKbps: 4840}},
		{"h264 1080p medium", media("h264", "yuv420p", "bt709", 0, 1920, 1080, 5_000_000, "aac", 2), model.DownloadMedium,
			want{audioCopy: true, w: 1280, h: 720, videoKbps: 3000}},
		{"h264 1080p low", media("h264", "yuv420p", "bt709", 0, 1920, 1080, 5_000_000, "aac", 2), model.DownloadLow,
			want{audioCopy: true, w: 852, h: 480, videoKbps: 1200}},
		// Above 1.1 × the profile rate: encoded at the profile's constant rate.
		{"h264 1080p 12 Mbit/s high", media("h264", "yuv420p", "", 0, 1920, 1080, 12_000_000, "aac", 2), model.DownloadHigh,
			want{audioCopy: true, w: 1920, h: 1080, videoKbps: 6000}},
		// Within 1.1 × (6400 ≤ 6600) still copied.
		{"h264 1080p 6.4 Mbit/s high", media("h264", "yuv420p", "", 0, 1920, 1080, 6_560_000, "aac", 2), model.DownloadHigh,
			want{copy: true, audioCopy: true, w: 1920, h: 1080, videoKbps: 6400}},
		// A small source in a big profile: never scaled up, the 480p rung's rate (transcode.NewPlan).
		{"h264 480p 8 Mbit/s high", media("h264", "yuv420p", "", 0, 854, 480, 8_000_000, "aac", 2), model.DownloadHigh,
			want{audioCopy: true, w: 854, h: 480, videoKbps: 2000}},
		// 10 bit, HEVC, HDR10, Dolby Vision 5: always encoded; HDR and DV tonemapped.
		{"h264 high 10", media("h264", "yuv420p10le", "", 0, 1920, 1080, 4_000_000, "aac", 2), model.DownloadHigh,
			want{audioCopy: true, w: 1920, h: 1080, videoKbps: 6000}},
		{"hevc sdr 1080p", media("hevc", "yuv420p", "", 0, 1920, 1080, 4_000_000, "aac", 2), model.DownloadHigh,
			want{audioCopy: true, w: 1920, h: 1080, videoKbps: 6000}},
		{"hevc hdr10 4k medium", media("hevc", "yuv420p10le", "smpte2084", 0, 3840, 2160, 40_000_000, "eac3", 6), model.DownloadMedium,
			want{tonemap: true, w: 1280, h: 720, videoKbps: 3000}},
		{"dolby vision 5", media("hevc", "yuv420p10le", "", 5, 3840, 2160, 20_000_000, "aac", 2), model.DownloadLow,
			want{tonemap: true, audioCopy: true, w: 852, h: 480, videoKbps: 1200}},
		// AC-3 5.1 and AAC 5.1 are encoded to AAC stereo.
		{"ac3 5.1", media("h264", "yuv420p", "", 0, 1280, 720, 3_000_000, "ac3", 6), model.DownloadMedium,
			want{copy: true, w: 1280, h: 720, videoKbps: 2520}},
		{"aac 5.1", media("h264", "yuv420p", "", 0, 1280, 720, 3_000_000, "aac", 6), model.DownloadMedium,
			want{copy: true, w: 1280, h: 720, videoKbps: 2520}},
		// Not probed yet: no verified pixel format or bit rate, encoded.
		{"not probed", src{model.MediaFile{VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080, DurationMs: 3_600_000}, nil}, model.DownloadHigh,
			want{w: 1920, h: 1080, videoKbps: 6000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := decide(tc.src.file, tc.src.probe, profile(tc.quality), -1)
			got := want{copy: c.Copy, audioCopy: c.AudioCopy, tonemap: c.Target.Tonemap, w: c.Width, h: c.Height, videoKbps: c.VideoKbps}
			if got != tc.want {
				t.Fatalf("decide = %+v, want %+v", got, tc.want)
			}
			wantMode := transcode.ModeTranscode
			if c.Copy {
				wantMode = transcode.ModeRemux
			}
			if c.Target.Mode != wantMode || c.Target.VideoCopy != c.Copy || c.Target.AudioCopy != c.AudioCopy ||
				c.Target.AudioBitrate != profile(tc.quality).AudioBitrate || c.Target.SubtitleBurnIn != -1 {
				t.Errorf("target = %+v", c.Target)
			}
			if !c.Copy && (!c.Target.ConstantRate || c.Target.MaxBitrate != profile(tc.quality).VideoBitrate*1000 ||
				c.Target.MaxHeight != profile(tc.quality).MaxHeight) {
				t.Errorf("transcode target without the profile's constant rate: %+v", c.Target)
			}
			audio := profile(tc.quality).AudioBitrate
			if c.AudioCopy {
				audio = 160
			}
			if tc.src.probe == nil && c.AudioKbps != profile(tc.quality).AudioBitrate {
				t.Errorf("audio kbit/s without probe = %d", c.AudioKbps)
			}
			if tc.src.probe != nil && c.AudioKbps != audio {
				t.Errorf("audio kbit/s = %d, want %d", c.AudioKbps, audio)
			}
			// (video + audio) kbit/s × 1 h / 8 × 1.02.
			want := int64(float64(c.VideoKbps+c.AudioKbps) * 1000 / 8 * 3600 * 1.02)
			if c.EstimatedBytes != want || c.DurationMs != 3_600_000 {
				t.Errorf("estimate = %d bytes over %d ms, want %d", c.EstimatedBytes, c.DurationMs, want)
			}
		})
	}
}

// The estimate of an encode uses the rate and size that transcode.NewPlan gives the encoder, so
// the file lands within O-1's ±10 % (stream A encodes at Plan.VideoBitrate).
func TestEstimateMatchesPlan(t *testing.T) {
	for _, s := range []struct{ w, h int }{{1920, 1080}, {1920, 800}, {1280, 720}, {720, 576}, {854, 480}, {640, 360}, {3840, 2160}} {
		for _, prof := range model.DownloadProfiles {
			in := media("hevc", "yuv420p10le", "smpte2084", 0, s.w, s.h, 30_000_000, "eac3", 6)
			c := decide(in.file, in.probe, prof, -1)
			plan, err := transcode.NewPlan(transcode.Source{Path: "/x", Probe: *in.probe}, c.Target)
			if err != nil {
				t.Fatal(err)
			}
			if plan.VideoBitrate != c.VideoKbps*1000 || plan.Width != c.Width || plan.Height != c.Height || !plan.Target.Tonemap {
				t.Errorf("%dx%d %s: plan %dx%d at %d bit/s, estimate %dx%d at %d kbit/s", s.w, s.h, prof.Quality,
					plan.Width, plan.Height, plan.VideoBitrate, c.Width, c.Height, c.VideoKbps)
			}
		}
	}
}

func TestSelectedAudioAndDefaults(t *testing.T) {
	p := testProbe()
	if got := defaultAudio(&p); got != 1 {
		t.Errorf("default audio = %d, want 1", got)
	}
	if a := selectedAudio(&p, 2); a == nil || a.Codec != "ac3" {
		t.Errorf("selected 2 = %+v", a)
	}
	if a := selectedAudio(&p, 9); a == nil || a.Index != 1 {
		t.Errorf("unknown index falls back to the default: %+v", a)
	}
	// The AC-3 track of the same source is encoded, the AAC one copied.
	file := model.MediaFile{DurationMs: clipDuration.Milliseconds()}
	if c := decide(file, &p, profile(model.DownloadHigh), 2); c.AudioCopy || c.Target.AudioStream != 2 {
		t.Errorf("ac3 track: %+v", c.Target)
	}
	if c := decide(file, &p, profile(model.DownloadHigh), -1); !c.AudioCopy || c.Target.AudioStream != 1 {
		t.Errorf("default track: %+v", c.Target)
	}
}
