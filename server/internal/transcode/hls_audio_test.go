package transcode

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// These tests run the real ffmpeg on the clip of testClip (irregular keyframes 0, 2, 5, 7, 9, 13,
// 14, 18 s, B-frames) with multichannel audio (concept 26 stage 1, ADR 0041): AC-3 and E-AC-3 in
// fMP4 need the delayed moov and the split layout of the session (split.go); multichannel encodes
// keep 5.1 and 7.1. Each test checks the segments with ffprobe: codec, channel count, a keyframe at
// every planned start, an audio timeline without gaps, and that ffmpeg decodes them all.

// audioClip is testClip with the audio encoded by audioArgs (codec, rate, channels; the source is
// a 440 Hz sine upmixed to the channel count).
func audioClip(t *testing.T, r Runner, audioArgs ...string) Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.mkv")
	args := []string{"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "20", "-c:v", "libx264", "-preset", "veryfast", "-bf", "3", "-force_key_frames", "0,2,5,7,9,13,14,18",
		"-x264-params", "keyint=1000:scenecut=0"}
	runFFmpeg(t, r, slices.Concat(args, audioArgs, []string{path})...)
	ctx := context.Background()
	probe, err := r.Probe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	kf, err := r.Keyframes(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	return Source{Path: path, Probe: probe, Keyframes: kf}
}

// audioInfo is what ffprobe reports about the audio of init + segments.
type audioInfo struct {
	codec    string
	channels int
	layout   string
	maxGap   float64 // largest gap between the end of one audio packet and the start of the next, s
	gapAt    float64 // where it begins
	start    float64 // first audio packet
	end      float64 // end of the last audio packet
}

// probeAudio runs ffprobe on init followed by segs.
func probeAudio(t *testing.T, r Runner, init []byte, segs ...[]byte) audioInfo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cat.mp4")
	if err := os.WriteFile(path, slices.Concat(append([][]byte{init}, segs...)...), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.FFprobePath, "-v", "error", "-select_streams", "a:0", "-show_streams",
		"-show_entries", "packet=pts_time,duration_time", "-of", "json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Streams []struct {
			CodecName     string `json:"codec_name"`
			Channels      int    `json:"channels"`
			ChannelLayout string `json:"channel_layout"`
		} `json:"streams"`
		Packets []struct {
			PTS      string `json:"pts_time"`
			Duration string `json:"duration_time"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Streams) != 1 || len(raw.Packets) == 0 {
		t.Fatalf("no audio in init + %d segments", len(segs))
	}
	info := audioInfo{codec: raw.Streams[0].CodecName, channels: raw.Streams[0].Channels, layout: raw.Streams[0].ChannelLayout,
		start: math.Inf(1), end: math.Inf(-1)}
	type pkt struct{ pts, dur float64 }
	var pkts []pkt
	for _, p := range raw.Packets {
		pts, err := strconv.ParseFloat(p.PTS, 64)
		if err != nil {
			t.Fatalf("packet pts %q", p.PTS)
		}
		dur, _ := strconv.ParseFloat(p.Duration, 64)
		pkts = append(pkts, pkt{pts, dur})
	}
	slices.SortFunc(pkts, func(a, b pkt) int { return cmpFloat(a.pts, b.pts) })
	for i, p := range pkts {
		info.start, info.end = min(info.start, p.pts), max(info.end, p.pts+p.dur)
		// From the second packet on: ffprobe gives the priming packet of an AAC encode (negative
		// time) no duration.
		if i > 1 {
			prev := pkts[i-1]
			if g := p.pts - (prev.pts + prev.dur); g > info.maxGap {
				info.maxGap, info.gapAt = g, prev.pts+prev.dur
			}
		}
	}
	return info
}

// Audio frame lengths at 48 kHz: AAC 1024 samples, AC-3 and E-AC-3 1536.
const (
	aacFrame = 1024.0 / 48000
	ac3Frame = 1536.0 / 48000
)

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// checkAudio asserts codec and channels of init + segs and an audio timeline without gaps above
// gap seconds.
func checkAudio(t *testing.T, r Runner, what, codec string, channels int, gap float64, init []byte, segs ...[]byte) {
	t.Helper()
	a := probeAudio(t, r, init, segs...)
	if a.codec != codec || a.channels != channels {
		t.Errorf("%s: audio %s with %d channels (%s), want %s with %d", what, a.codec, a.channels, a.layout, codec, channels)
	}
	if a.maxGap > gap {
		t.Errorf("%s: audio timeline has a gap of %.1f ms at %.3f s", what, a.maxGap*1000, a.gapAt)
	}
}

// TestHLSRemuxDolbyCopy: a remux that copies AC-3 or E-AC-3 5.1 (the error of before: "Cannot
// write moov atom before AC3 packets", exit 234) from the start, after a restart at the last
// segment and after a seek back, with the codec string of the copied format.
func TestHLSRemuxDolbyCopy(t *testing.T) {
	r := hlsRunner(t)
	for _, tc := range []struct{ codec, codecs string }{{"ac3", "avc1.64001E,ac-3"}, {"eac3", "avc1.64001E,ec-3"}} {
		t.Run(tc.codec, func(t *testing.T) {
			src := audioClip(t, r, "-c:a", tc.codec, "-b:a", "448k", "-ac", "6")
			p, err := NewPlan(src, Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := starts(p), []time.Duration{0, sec(5), sec(9), sec(13), sec(18)}; !slices.Equal(got, want) {
				t.Fatalf("plan %v, want %v", got, want)
			}
			if got := CodecsString(p, src); got != tc.codecs {
				t.Errorf("codecs %q, want %q", got, tc.codecs)
			}
			m := newFFmpegManager(t, r, BackendSoftware)
			read := reader(t)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			s, err := m.Start(ctx, "dolby", src, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			init := read(s.OpenInit(ctx))
			tracks := parseInit(t, init)
			segs := make([][]byte, len(p.Segments))
			for i := range segs {
				segs[i] = read(s.OpenSegment(ctx, i))
				checkSegment(t, p, tracks, i, segs[i])
			}
			if n := decodes(t, r, init, segs...); n != 480 {
				t.Errorf("init + all segments decode to %d frames, want 480", n)
			}
			checkAudio(t, r, "one run", tc.codec, 6, 0.001, init, segs...)

			// Restarts: stop after segment 0, seek to the last segment, then back to 3.
			s2, err := m.Start(ctx, "dolby2", src, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			seg0 := read(s2.OpenSegment(ctx, 0))
			stopAndForget(t, s2, 1)
			last := read(s2.OpenSegment(ctx, 4))
			checkSegment(t, p, tracks, 4, last)
			seg3 := read(s2.OpenSegment(ctx, 3))
			checkSegment(t, p, tracks, 3, seg3)
			seg1 := read(s2.OpenSegment(ctx, 1))
			checkSegment(t, p, tracks, 1, seg1)
			seg2 := read(s2.OpenSegment(ctx, 2))
			checkSegment(t, p, tracks, 2, seg2)
			if n := decodes(t, r, init, seg0, seg1, seg2, seg3, last); n != 480 {
				t.Errorf("segments of several runs decode to %d frames, want 480", n)
			}
			checkAudio(t, r, "several runs", tc.codec, 6, 0.001, init, seg0, seg1, seg2, seg3, last)
		})
	}
}

// TestHLSTranscodeMultichannel: a transcode with copied E-AC-3, with AC-3 5.1 encoded from AAC
// 5.1, with AAC 5.1 and with AAC 7.1, each from the start and after restarts on the 4-s grid.
func TestHLSTranscodeMultichannel(t *testing.T) {
	r := hlsRunner(t)
	for _, tc := range []struct {
		name     string
		audio    []string
		target   Target
		codec    string
		channels int
		codecs   string
		frame    float64 // length of one audio frame, s
	}{
		{"aac stereo (before)", []string{"-c:a", "aac", "-b:a", "128k"}, Target{}, "aac", 2, "mp4a.40.2", aacFrame},
		{"eac3 copied", []string{"-c:a", "eac3", "-b:a", "640k", "-ac", "6"}, Target{AudioCopy: true}, "eac3", 6, "ec-3", ac3Frame},
		{"ac3 encoded", []string{"-c:a", "aac", "-b:a", "384k", "-ac", "6"}, Target{AudioCodec: AudioAC3, AudioChannels: 6}, "ac3", 6, "ac-3", ac3Frame},
		{"aac 5.1", []string{"-c:a", "ac3", "-b:a", "448k", "-ac", "6"}, Target{AudioChannels: 6}, "aac", 6, "mp4a.40.2", aacFrame},
		// Odd counts keep their number (ADR 0041): quad as AC-3, 6.1 as AAC.
		{"ac3 quad", []string{"-c:a", "pcm_s16le", "-ac", "4"}, Target{AudioCodec: AudioAC3, AudioChannels: 4}, "ac3", 4, "ac-3", ac3Frame},
		{"aac 6.1", []string{"-c:a", "pcm_s16le", "-ac", "7"}, Target{AudioChannels: 7}, "aac", 7, "mp4a.40.2", aacFrame},
		{"aac 7.1", []string{"-c:a", "flac", "-ac", "8"}, Target{AudioChannels: 8}, "aac", 8, "mp4a.40.2", aacFrame},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := audioClip(t, r, tc.audio...)
			target := tc.target
			target.Mode, target.MaxHeight = ModeTranscode, 360
			p, err := NewPlan(src, target)
			if err != nil {
				t.Fatal(err)
			}
			if got := CodecsString(p, src); got != "avc1.640029,"+tc.codecs {
				t.Errorf("codecs %q, want avc1.640029,%s", got, tc.codecs)
			}
			m := newFFmpegManager(t, r, BackendSoftware)
			read := reader(t)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			s, err := m.Start(ctx, "multi", src, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			init := read(s.OpenInit(ctx))
			tracks := parseInit(t, init)
			seg0 := read(s.OpenSegment(ctx, 0))
			checkSegment(t, p, tracks, 0, seg0)
			stopAndForget(t, s, 1)
			seg4 := read(s.OpenSegment(ctx, 4)) // restart at 16 s
			checkSegment(t, p, tracks, 4, seg4)
			seg2 := read(s.OpenSegment(ctx, 2)) // restart at 8 s
			checkSegment(t, p, tracks, 2, seg2)
			seg3 := read(s.OpenSegment(ctx, 3))
			checkSegment(t, p, tracks, 3, seg3)
			seg1 := read(s.OpenSegment(ctx, 1))
			checkSegment(t, p, tracks, 1, seg1)
			if runs := runCount(s); runs != 4 {
				t.Errorf("%d ffmpeg runs, want 4 (start, seek to 4, seek to 2, seek to 1)", runs)
			}
			if n := decodes(t, r, init, seg0, seg1, seg2, seg3, seg4); n != 480 {
				t.Errorf("segments of four runs decode to %d frames, want 480", n)
			}
			// A transcode restart drops the audio before its boundary that the previous run had not
			// muxed yet: up to about three audio frames at each seam, with stereo AAC as before
			// (53 ms measured); the seams of the new formats stay within that.
			checkAudio(t, r, "four runs", tc.codec, tc.channels, 3*tc.frame+0.001, init, seg0, seg1, seg2, seg3, seg4)

			// One run from the start: no gap at all.
			s2, err := m.Start(ctx, "multi2", src, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			segs := make([][]byte, len(p.Segments))
			for i := range segs {
				segs[i] = read(s2.OpenSegment(ctx, i))
				checkSegment(t, p, tracks, i, segs[i])
			}
			if runs := runCount(s2); runs != 1 {
				t.Errorf("%d ffmpeg runs, want 1", runs)
			}
			checkAudio(t, r, "one run", tc.codec, tc.channels, 0.001, init, segs...)
		})
	}
}
