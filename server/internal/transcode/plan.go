package transcode

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// Mode is the kind of HLS session: remux copies the video, transcode encodes H.264.
type Mode string

// Session modes (plan 3.5).
const (
	ModeRemux     Mode = "remux"
	ModeTranscode Mode = "transcode"
)

// SegmentLength is the nominal HLS segment length: the transcode grid and the minimum length of a
// remux segment cut on source keyframes.
const SegmentLength = 4 * time.Second

// minTail is the shortest last segment of the transcode grid; a shorter remainder is merged into
// the previous segment, because it may contain no video frame at all and would never be written.
const minTail = time.Second

// Source is the input of an HLS session.
type Source struct {
	Path      string          // absolute path, resolved by the caller through package paths
	Probe     Probe           // stored ffprobe result (media_files.probe)
	Keyframes []time.Duration // decoded keyframe index (library.DecodeKeyframes), may be nil
}

// Target describes the output of an HLS session.
type Target struct {
	Mode           Mode
	VideoCopy      bool   // remux keeps video; transcode encodes H.264
	AudioCopy      bool   // keep audio (AAC) or encode to AAC stereo
	MaxHeight      int    // 1080 for M1b, 720 for phone later; 0 = 1080 when encoding
	Tonemap        bool   // HDR10 → SDR (hable), always with MaxHeight <= 1080
	VideoTag       string // "hvc1" when remuxing HEVC for Safari
	SubtitleBurnIn int    // -1 none (M1b has none)
}

// Segment is one HLS media segment on the source timeline.
type Segment struct {
	Start, Duration time.Duration
}

// Plan is the complete segment list of a session, computed once at the start so seeking can
// restart ffmpeg at any segment with the same boundaries.
type Plan struct {
	Target   Target
	Segments []Segment // transcode = 4-s grid; remux = keyframe boundaries >= 4 s
	Duration time.Duration
	Cost     float64 // admission cost, plan 3.5 table

	// Width and Height are the output video size (the source size when the video is copied),
	// Bandwidth the peak bit rate in bit/s for EXT-X-STREAM-INF. All three feed MasterPlaylist.
	Width, Height int
	Bandwidth     int
}

// Admission costs of plan 3.5 (start values, calibrated in docs/perf/M1b.md).
const (
	cost4KHDR      = 4
	cost4KSDR      = 3
	costHD         = 2
	costHDSmallOut = 1.5
	costRemux      = 0.5
	costAudioOnly  = 0.25
)

// Encoding defaults for M1b: one quality per output height.
const (
	defaultMaxHeight = 1080
	bitrate1080      = 8_000_000
	bitrate720       = 4_000_000
	audioBitrate     = 160_000
)

// NewPlan computes the segment list and the admission cost for src and t.
//
// Transcode cuts on a 4-s grid from 0 (keyframes are forced there); a remainder shorter than 1 s is
// merged into the last segment. Remux cuts on source keyframes: the next boundary is the first
// keyframe at least 4 s after the previous one. Without a keyframe index remux falls back to the
// 4-s grid; ffmpeg then cuts at the first keyframe after each grid point, so the playlist
// durations are approximate and a seek may land on an earlier keyframe (documented in ADR 0003).
func NewPlan(src Source, t Target) (Plan, error) {
	if t.Mode != ModeRemux && t.Mode != ModeTranscode {
		return Plan{}, fmt.Errorf("unknown mode %q", t.Mode)
	}
	if t.Mode == ModeTranscode && t.VideoCopy {
		return Plan{}, errors.New("transcode mode encodes the video; VideoCopy needs remux mode")
	}
	if t.Mode == ModeRemux && !t.VideoCopy {
		return Plan{}, errors.New("remux mode copies the video; set VideoCopy")
	}
	d := src.Probe.Format.Duration
	if d <= 0 {
		return Plan{}, errors.New("source duration unknown")
	}
	v, hasVideo := videoStream(src.Probe)
	_, hasAudio := audioStream(src.Probe)
	if !hasVideo && !hasAudio {
		return Plan{}, errors.New("source has neither video nor audio")
	}
	if t.Mode == ModeTranscode && hasVideo {
		if t.MaxHeight <= 0 || t.MaxHeight > defaultMaxHeight {
			t.MaxHeight = defaultMaxHeight
		}
		if !isHDR(v) {
			t.Tonemap = false
		}
	}

	var starts []time.Duration
	if t.Mode == ModeRemux && hasVideo && len(src.Keyframes) > 0 {
		starts = keyframeStarts(src.Keyframes, d)
	} else {
		starts = gridStarts(d)
	}
	p := Plan{Target: t, Duration: d, Segments: make([]Segment, len(starts))}
	for i, s := range starts {
		end := d
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		p.Segments[i] = Segment{Start: s, Duration: end - s}
	}
	p.Cost = Cost(src, t)
	p.Width, p.Height = outputSize(v, t)
	p.Bandwidth = bandwidth(src.Probe, t, p.Height)
	return p, nil
}

// gridStarts returns 0, 4 s, 8 s, ... below d; a tail shorter than minTail joins the previous segment.
func gridStarts(d time.Duration) []time.Duration {
	starts := []time.Duration{0}
	for s := SegmentLength; s < d; s += SegmentLength {
		starts = append(starts, s)
	}
	if n := len(starts); n > 1 && d-starts[n-1] < minTail {
		starts = starts[:n-1]
	}
	return starts
}

// keyframeStarts cuts greedily on keyframes: segment 0 starts at 0, every next boundary is the
// first keyframe at least SegmentLength after the previous boundary and before the end.
func keyframeStarts(kf []time.Duration, d time.Duration) []time.Duration {
	starts := []time.Duration{0}
	prev := time.Duration(0)
	for _, k := range kf {
		if k >= prev+SegmentLength && k < d {
			starts = append(starts, k)
			prev = k
		}
	}
	return starts
}

// Cost is the admission cost of a session (plan 3.5): source and pipeline decide, not the output
// quality. 4K HDR with tonemapping 4, other 4K 3, up to 1080p 2 (output <= 720p 1.5), remux 0.5,
// audio only 0.25. Remux includes an AAC encode of the audio.
func Cost(src Source, t Target) float64 {
	v, ok := videoStream(src.Probe)
	switch {
	case !ok:
		return costAudioOnly
	case t.Mode == ModeRemux || t.VideoCopy:
		return costRemux
	case is4K(v) && t.Tonemap && isHDR(v):
		return cost4KHDR
	case is4K(v):
		return cost4KSDR
	case t.MaxHeight > 0 && t.MaxHeight <= 720:
		return costHDSmallOut
	default:
		return costHD
	}
}

func is4K(v Stream) bool { return v.Width > 2560 || v.Height > 1440 }

// isHDR reports PQ (HDR10, HDR10+, Dolby Vision base layer) and HLG sources.
func isHDR(v Stream) bool {
	return v.ColorTransfer == "smpte2084" || v.ColorTransfer == "arib-std-b67"
}

// videoStream returns the first real video stream (cover art excluded).
func videoStream(p Probe) (Stream, bool) {
	for _, s := range p.Streams {
		if s.Type == "video" && !s.AttachedPic {
			return s, true
		}
	}
	return Stream{}, false
}

// audioStream returns the default audio stream, else the first one.
func audioStream(p Probe) (Stream, bool) {
	i := slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Type == "audio" && s.Default })
	if i < 0 {
		i = slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Type == "audio" })
	}
	if i < 0 {
		return Stream{}, false
	}
	return p.Streams[i], true
}

// outputSize scales the video down to MaxHeight keeping the aspect ratio, with even sizes.
func outputSize(v Stream, t Target) (w, h int) {
	w, h = v.Width, v.Height
	if t.VideoCopy || w <= 0 || h <= 0 {
		return w, h
	}
	if t.MaxHeight > 0 && h > t.MaxHeight {
		w = int(math.Round(float64(w) * float64(t.MaxHeight) / float64(h)))
		h = t.MaxHeight
	}
	return w &^ 1, h &^ 1
}

func videoBitrate(height int) int {
	if height > 720 {
		return bitrate1080
	}
	return bitrate720
}

// bandwidth is the peak bit rate for the master playlist: the encoder cap plus audio when
// encoding, the container bit rate with 20 % headroom when copying.
func bandwidth(p Probe, t Target, height int) int {
	if _, ok := videoStream(p); !ok || t.VideoCopy {
		br := int(p.Format.BitRate)
		if br <= 0 {
			br = bitrate1080
		}
		return br * 6 / 5
	}
	return videoBitrate(height) + audioBitrate
}

// CodecsString returns the RFC 6381 CODECS attribute for the master playlist: H.264 High 4.1 and
// AAC LC when encoding, the source codecs when copying.
func CodecsString(plan Plan, src Source) string {
	var codecs []string
	if v, ok := videoStream(src.Probe); ok {
		if plan.Target.VideoCopy {
			if c := videoCodecString(v, plan.Target.VideoTag); c != "" {
				codecs = append(codecs, c)
			}
		} else {
			codecs = append(codecs, "avc1.640029")
		}
	}
	if a, ok := audioStream(src.Probe); ok {
		codecs = append(codecs, audioCodecString(a, plan.Target.AudioCopy))
	}
	return strings.Join(codecs, ",")
}

func videoCodecString(v Stream, tag string) string {
	switch v.Codec {
	case "h264":
		profiles := map[string]string{
			"Constrained Baseline": "42E0", "Baseline": "4200", "Main": "4D00", "Extended": "5800",
			"High": "6400", "High 10": "6E00", "High 4:2:2": "7A00", "High 4:4:4 Predictive": "F400",
		}
		pc, ok := profiles[v.Profile]
		if !ok {
			pc = "6400"
		}
		level := v.Level
		if level <= 0 {
			level = 41
		}
		return fmt.Sprintf("avc1.%s%02X", pc, level)
	case "hevc":
		prefix := "hev1"
		if tag == "hvc1" {
			prefix = "hvc1"
		}
		// general_profile_idc and the reversed compatibility flags: Main 1/6, Main 10 2/4.
		profile, compat := 1, "6"
		if v.Profile == "Main 10" {
			profile, compat = 2, "4"
		}
		level := v.Level // ffprobe reports general_level_idc (30 × level)
		if level <= 0 {
			level = 120
		}
		return fmt.Sprintf("%s.%d.%s.L%d.B0", prefix, profile, compat, level)
	default:
		return ""
	}
}

func audioCodecString(a Stream, copyAudio bool) string {
	if !copyAudio {
		return "mp4a.40.2"
	}
	switch a.Codec {
	case "ac3":
		return "ac-3"
	case "eac3":
		return "ec-3"
	case "aac":
		if strings.HasPrefix(a.Profile, "HE-AACv2") {
			return "mp4a.40.29"
		}
		if strings.HasPrefix(a.Profile, "HE-AAC") {
			return "mp4a.40.5"
		}
	}
	return "mp4a.40.2"
}
