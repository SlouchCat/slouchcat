package transcode

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

// Concept 26 stage 1: codec strings, bit rates and encoder options of the audio of HLS outputs.

// dolbySource is a 13-s HEVC file with a stereo AAC track (default, stream 1) and a second track
// (stream 2) of codec with 6 channels and the probed rate.
func dolbySource(codec string, bitrate int64) Source {
	return Source{
		Path: "/media/Film.mkv",
		Probe: Probe{Format: Format{Duration: 13 * time.Second, BitRate: 8_800_000}, Streams: []Stream{
			{Index: 0, Type: "video", Codec: "h264", Profile: "High", Level: 41, Width: 1920, Height: 1080, PixFmt: "yuv420p"},
			{Index: 1, Type: "audio", Codec: "aac", Channels: 2, Default: true},
			{Index: 2, Type: "audio", Codec: codec, Channels: 6, BitRate: bitrate},
		}},
		Keyframes: []time.Duration{0, 4 * time.Second, 8 * time.Second, 12 * time.Second},
	}
}

func TestAudioCodecStrings(t *testing.T) {
	for _, tc := range []struct {
		codec string
		t     Target
		want  string
	}{
		{"ac3", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,ac-3"},
		{"eac3", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,ec-3"},
		{"dts", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,mp4a.a9"},
		{"flac", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,fLaC"},
		{"opus", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,Opus"},
		// Not carried by HLS: NewPlan encodes them (AAC), so the string is AAC's and true.
		{"truehd", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,mp4a.40.2"},
		{"mp3", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,mp4a.40.2"},
		{"pcm_s24le", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "avc1.640029,mp4a.40.2"},
		{"dts", Target{Mode: ModeTranscode, AudioCodec: AudioAC3, AudioChannels: 6}, "avc1.640029,ac-3"},
		{"dts", Target{Mode: ModeTranscode, AudioChannels: 6}, "avc1.640029,mp4a.40.2"},
	} {
		tc.t.AudioStream = 2
		src := dolbySource(tc.codec, 0)
		p, err := NewPlan(src, tc.t)
		if err != nil {
			t.Fatal(err)
		}
		if got := CodecsString(p, src); got != tc.want {
			t.Errorf("%s %+v: codecs %q, want %q", tc.codec, tc.t, got, tc.want)
		}
	}
}

func TestAudioBitrates(t *testing.T) {
	for _, tc := range []struct {
		s    Stream
		want int
	}{
		{Stream{Codec: "eac3", BitRate: 768_000}, 768_000},
		{Stream{Codec: "eac3"}, 1_024_000},
		{Stream{Codec: "ac3"}, 640_000},
		{Stream{Codec: "dts"}, 1_536_000},
		{Stream{Codec: "opus"}, 510_000},
		{Stream{Codec: "flac", Channels: 6}, 4_500_000},
		{Stream{Codec: "aac", Channels: 6}, 160_000}, // as before without a probed rate
		{Stream{Codec: "aac", BitRate: 128_000}, 128_000},
	} {
		if got := CopyAudioBitrate(tc.s); got != tc.want {
			t.Errorf("CopyAudioBitrate(%+v) = %d, want %d", tc.s, got, tc.want)
		}
	}
	for _, tc := range []struct {
		codec    string
		channels int
		want     int
	}{{"", 0, 160_000}, {"", 2, 160_000}, {"", 6, 384_000}, {"", 8, 512_000}, {AudioAC3, 6, 640_000}, {AudioAC3, 4, 640_000}} {
		if got := EncodeAudioBitrate(tc.codec, tc.channels); got != tc.want {
			t.Errorf("EncodeAudioBitrate(%q, %d) = %d, want %d", tc.codec, tc.channels, got, tc.want)
		}
	}
	// The master playlist of a transcode counts the copied track with its own rate, the encode
	// with the encoder's.
	src := dolbySource("eac3", 768_000)
	p, err := NewPlan(src, Target{Mode: ModeTranscode, AudioCopy: true, AudioStream: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p.Bandwidth != 8_000_000+768_000 {
		t.Errorf("bandwidth %d, want %d", p.Bandwidth, 8_000_000+768_000)
	}
	p, err = NewPlan(src, Target{Mode: ModeTranscode, AudioCodec: AudioAC3, AudioChannels: 6, AudioStream: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p.Bandwidth != 8_000_000+640_000 {
		t.Errorf("bandwidth %d, want %d", p.Bandwidth, 8_000_000+640_000)
	}
}

// The audio options of the ffmpeg run and the delayed moov for AC-3 and E-AC-3 only.
func TestFFmpegArgsAudio(t *testing.T) {
	delay := "movflags=+empty_moov+frag_custom+default_base_moof+frag_discont+skip_trailer+delay_moov+frag_keyframe" +
		":avoid_negative_ts=disabled:use_editlist=0"
	for _, tc := range []struct {
		name  string
		codec string
		t     Target
		audio string // the audio options
		mux   string
	}{
		{"ac3 copy", "ac3", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "-c:a copy", delay},
		{"eac3 copy", "eac3", Target{Mode: ModeTranscode, AudioCopy: true}, "-c:a copy", delay},
		{"dts copy", "dts", Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}, "-c:a copy", movFlags},
		{"ac3 5.1 encode", "dts", Target{Mode: ModeTranscode, AudioCodec: AudioAC3, AudioChannels: 6}, "-c:a ac3 -b:a 640k -ac 6", delay},
		{"aac 5.1 encode", "dts", Target{Mode: ModeRemux, VideoCopy: true, AudioChannels: 6}, "-c:a aac -b:a 384k -ac 6", movFlags},
		{"aac 7.1 encode", "truehd", Target{Mode: ModeRemux, VideoCopy: true, AudioChannels: 8}, "-c:a aac -b:a 512k -ac 8", movFlags},
		{"stereo as before", "dts", Target{Mode: ModeTranscode}, "-c:a aac -b:a 160k -ac 2", movFlags},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.t.AudioStream = 2
			src := dolbySource(tc.codec, 0)
			p, err := NewPlan(src, tc.t)
			if err != nil {
				t.Fatal(err)
			}
			args, err := ffmpegArgs(Capabilities{Backend: BackendSoftware}, src, p, 1, "/w/s/run0001")
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(args, " ")
			if !strings.Contains(got, " -map 0:2 ") || !strings.Contains(got, " "+tc.audio+" -ss ") ||
				!strings.Contains(got, " -segment_format_options "+tc.mux+" ") {
				t.Errorf("args %s\nwant %q and %q", got, tc.audio, tc.mux)
			}
		})
	}
}

func TestParseProbeAudioBitRate(t *testing.T) {
	p, err := ParseProbe([]byte(`{"format":{"format_name":"matroska,webm","duration":"10.0"},"streams":[
		{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"5000000"},
		{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6,"bit_rate":"640000"},
		{"index":2,"codec_type":"audio","codec_name":"dts","channels":6,"tags":{"BPS":"1509000"}},
		{"index":3,"codec_type":"audio","codec_name":"ac3","channels":6,"tags":{"BPS-eng":"448000"}},
		{"index":4,"codec_type":"audio","codec_name":"truehd","channels":8}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{0, 640_000, 1_509_000, 448_000, 0} {
		if got := p.Streams[i].BitRate; got != want {
			t.Errorf("stream %d: bit rate %d, want %d", i, got, want)
		}
	}
}

// segmentOf assigns a fragment by the presentation time of its first video sample with the cut
// tolerance of the segment muxer; firstCompositionOffset reads the signed offset of trun v1.
func TestSplitHelpers(t *testing.T) {
	p := Plan{Segments: []Segment{{Start: 0}, {Start: 5 * time.Second}, {Start: 9 * time.Second}}}
	for _, tc := range []struct {
		t    time.Duration
		cut  time.Duration
		want int
	}{
		{-20 * time.Millisecond, remuxCutDelta, 0},
		{2 * time.Second, remuxCutDelta, 0}, // a keyframe inside segment 0 (remux)
		{4999 * time.Millisecond, remuxCutDelta, 1},
		{4900 * time.Millisecond, remuxCutDelta, 0},
		{4900 * time.Millisecond, transcodeCutDelta, 1},
		{9 * time.Second, remuxCutDelta, 2},
		{30 * time.Second, remuxCutDelta, 2},
	} {
		if got := segmentOf(p, tc.t, tc.cut); got != tc.want {
			t.Errorf("segmentOf(%v, cut %v) = %d, want %d", tc.t, tc.cut, got, tc.want)
		}
	}
	// trun v1 with data offset, sample duration, size, flags and composition offset -1024.
	trun := make([]byte, 28)
	binary.BigEndian.PutUint32(trun, 1<<24|0x1|0x100|0x200|0x400|0x800)
	binary.BigEndian.PutUint32(trun[4:], 1)
	cto := int32(-1024)
	binary.BigEndian.PutUint32(trun[24:], uint32(cto))
	if got := firstCompositionOffset(trun); got != -1024 {
		t.Errorf("composition offset %d, want -1024", got)
	}
}
