package transcode

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/trackpick"
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
	Path string // absolute path, resolved by the caller through package paths
	// Parts are the ordered absolute paths of a multi-part audiobook (M6). When set, Path is
	// ignored for the input: ffmpeg reads a concat list (-f concat -safe 0) written into the work
	// directory of the run, or each part with the concat filter when the parts differ
	// (Probe.Mixed), and Probe describes the whole book (ConcatProbe: Duration is the sum of the
	// parts, Streams are the first part's audio).
	Parts     []string
	Probe     Probe           // stored ffprobe result (media_files.probe)
	Keyframes []time.Duration // decoded keyframe index (library.DecodeKeyframes), may be nil
}

// Target describes the output of an HLS session.
type Target struct {
	Mode      Mode
	VideoCopy bool   // remux keeps video; transcode encodes H.264
	AudioCopy bool   // keep the chosen audio track or encode it to AAC stereo
	MaxHeight int    // output height limit of a transcode; 0 = 1080 (auto never goes above 1080p)
	Tonemap   bool   // HDR10/HLG/Dolby Vision → SDR, with MaxHeight <= 1080 unless HWTonemap
	VideoTag  string // "hvc1" when remuxing HEVC for Safari
	// Backend and HWTonemap describe the host of a transcode (ADR 0021, Capabilities.Backend and
	// HWTonemap): the output goes above 1080 lines only on a hardware backend, and with Tonemap
	// only when the GPU tonemaps. The zero value (no backend) keeps the 1080 limit of AN-17.
	Backend   Backend
	HWTonemap bool
	// SubtitleBurnIn is the stream index of an embedded subtitle to burn into the video, or -1 for
	// none; a sidecar file is named by BurnInPath instead (M4). An index that does not name a
	// subtitle stream burns nothing.
	SubtitleBurnIn int
	BurnInPath     string // absolute path of a sidecar text subtitle to burn in (resolved through paths)
	// AudioStream is the stream index of the audio track to output; -1 (or an index that does not
	// name an audio stream) picks the default track.
	AudioStream int
	// Quality names the ladder rung (model.Quality*); empty or "auto" = the rung of MaxHeight.
	// NewPlan replaces it with the rung the output actually gets.
	Quality string
	// MaxBitrate caps the video bit rate in bit/s (the profile's bandwidth limit); 0 = the rung's rate.
	MaxBitrate int
	// ConstantRate encodes at a constant bit rate (MaxBitrate) so the size of a download is
	// predictable (M5, O-1: ±10 % of the estimate).
	ConstantRate bool
	// AudioBitrate is the AAC bit rate in kbit/s of an audio encode; 0 = the HLS default (M5 downloads).
	AudioBitrate int
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
	// VideoBitrate is the encoder's peak video bit rate in bit/s (maxrate): the rate of the rung,
	// capped by Target.MaxBitrate; 0 when the video is copied.
	VideoBitrate int
}

// Admission costs of plan 3.5 (start values, calibrated in docs/perf/M1b.md).
const (
	cost4KHDR = 4
	// cost2160p is a transcode with an output above 1080 lines (ADR 0021): it decodes and encodes
	// in 4K, so the QSV budget of 8 admits one such stream next to one 1080p transcode (5 + 2).
	cost2160p = 5
	// costHDRCPU replaces the HDR cost when the tonemapping runs on the CPU (software backend,
	// QSV hybrid chain): on the reference NAS (2 cores) one 1080p stream uses 1.2 cores
	// (docs/perf/M4.md), so the QSV budget of 8 admits one such stream plus one SDR transcode.
	costHDRCPU     = 6
	cost4KSDR      = 3
	costHD         = 2
	costHDSmallOut = 1.5
	costRemux      = 0.5
	costAudioOnly  = 0.25
)

// Encoding defaults: the output height of auto and the limit of the software backend and of CPU
// tonemapping (AN-17), the limit of the hardware backends (ADR 0021) and the AAC bit rate.
const (
	defaultMaxHeight  = 1080
	hardwareMaxHeight = 2160
	audioBitrate      = 160_000
)

// MaxOutputHeight is the highest output height of a transcode on backend b (ADR 0021): 2160 on
// the hardware backends (NVENC, QSV, VAAPI), 1080 on the software backend (AN-17) and wherever
// the tonemapping runs on the CPU (cpuTonemap). An unknown backend counts as software.
func MaxOutputHeight(b Backend, cpuTonemap bool) int {
	switch {
	case cpuTonemap:
		return defaultMaxHeight
	case b == BackendNVENC || b == BackendQSV || b == BackendVAAPI:
		return hardwareMaxHeight
	default:
		return defaultMaxHeight
	}
}

// Rung2160Allowed reports whether a transcode of the video v to the 2160p rung may be offered
// (ADR 0021, all conditions): v has at least 2160 lines, the backend of caps is a hardware one,
// the profile allows 2160 lines (profileMaxHeight 0 = no limit), the device reports a short side
// of at least 1440 pixels (deviceHeight, 0 = unknown: not allowed) and v is SDR or caps tonemaps
// on the GPU. auto never picks the rung by itself.
func Rung2160Allowed(caps Capabilities, v Stream, profileMaxHeight, deviceHeight int) bool {
	return v.Height >= hardwareMaxHeight &&
		MaxOutputHeight(caps.Backend, isHDR(v) && !caps.HWTonemap) >= hardwareMaxHeight &&
		(profileMaxHeight == 0 || profileMaxHeight >= hardwareMaxHeight) &&
		deviceHeight >= min2160DeviceHeight
}

// min2160DeviceHeight is the smallest short side of a screen in device pixels that gets the 2160p
// rung offered (ADR 0021).
const min2160DeviceHeight = 1440

// h264Level is the H.264 level_idc of an encode with the given output height: 4.1 up to 1080
// lines (1920x1080 at 30 fps, 8192 macroblocks per frame), 5.1 above (3840x2160 and 4096x2160 at
// 30 fps, 36864 macroblocks per frame); an unknown height (0) counts as 1080.
func h264Level(height int) int {
	if height > defaultMaxHeight {
		return 51
	}
	return 41
}

// Rung is one step of the quality ladder (W-8, plan 3.5): the output height limit and the peak
// video bit rate in bit/s.
type Rung struct {
	Quality string // model.Quality2160, Quality1080, Quality720, Quality480 or Quality360
	Height  int
	Bitrate int
}

// ladder lists the rungs from the highest down.
var ladder = []Rung{
	{model.Quality2160, 2160, 25_000_000},
	{model.Quality1080, 1080, 8_000_000},
	{model.Quality720, 720, 4_000_000},
	{model.Quality480, 480, 2_000_000},
	{model.Quality360, 360, 1_000_000},
}

// QualityFor returns the rung a video of the given height plays at without upscaling: the
// smallest rung at least as high (a 1920x800 film is 1080p, 405 lines are 480p, 300 lines 360p).
// Heights above 1080 give 2160p, unknown heights (<= 0) 1080p.
func QualityFor(height int) string { return rungFor(height).Quality }

func rungFor(height int) Rung {
	if height <= 0 {
		height = defaultMaxHeight
	}
	for i := len(ladder) - 1; i >= 0; i-- {
		if ladder[i].Height >= height {
			return ladder[i]
		}
	}
	return ladder[0]
}

// Rungs returns the rungs a source of the given height can be transcoded to, highest first: the
// rung of the source (QualityFor) and every lower one. An unknown height (0) offers 1080p and
// every lower rung. Whether a host offers 2160p is Rung2160Allowed.
func Rungs(sourceHeight int) []Rung {
	top := rungFor(sourceHeight)
	i := slices.IndexFunc(ladder, func(r Rung) bool { return r.Quality == top.Quality })
	return slices.Clone(ladder[i:])
}

// rungByName returns the rung of a model.Quality* name; ok is false for "", "auto" and unknown names.
func rungByName(q string) (Rung, bool) {
	i := slices.IndexFunc(ladder, func(r Rung) bool { return r.Quality == q })
	if i < 0 {
		return Rung{}, false
	}
	return ladder[i], true
}

// heightLimit is the output height limit of a transcode: MaxHeight (0 = 1080), at most
// MaxOutputHeight of the target's backend (1080 on software and for tonemapping on the CPU, 2160
// on hardware), lowered to the rung of an explicit Quality. An unknown quality name is an error.
func heightLimit(t Target) (int, error) {
	limit := t.MaxHeight
	if limit <= 0 {
		limit = defaultMaxHeight
	}
	limit = min(limit, MaxOutputHeight(t.Backend, t.Tonemap && !t.HWTonemap))
	if t.Quality == "" || t.Quality == model.QualityAuto {
		return limit, nil
	}
	r, ok := rungByName(t.Quality)
	if !ok {
		return 0, fmt.Errorf("unknown quality %q", t.Quality)
	}
	return min(limit, r.Height), nil
}

// ErrNoKeyframes is returned by NewPlan for a remux of a video without a keyframe index: on a grid
// the segment muxer cuts once per source keyframe, so with GOPs longer than 4 s the segment
// numbers drift away from the playlist. The caller computes the index first (Runner.Keyframes)
// or transcodes.
var ErrNoKeyframes = errors.New("remux needs the keyframe index of the source")

// NewPlan computes the segment list and the admission cost for src and t.
//
// An audio-only source (no video stream; cover art does not count) is planned in transcode mode
// with AudioCopy (AAC within 1.1 × the target rate) or AudioBitrate (kbit/s); MaxHeight, Quality
// and the video fields are ignored, the plan has no size and costs costAudioOnly.
//
// Transcode cuts on a 4-s grid from 0 (keyframes are forced there); a remainder shorter than 1 s is
// merged into the last segment. Remux cuts on source keyframes: the next boundary is the first
// keyframe at least 4 s after the previous one; without src.Keyframes it returns ErrNoKeyframes.
// Audio-only sources use the 4-s grid in both modes.
//
// A transcode gets the rung of min(height limit, source height): the limit is MaxHeight lowered
// to an explicit Quality, the bit rate that of the rung, capped by MaxBitrate. Plan.Target carries
// the resolved MaxHeight and Quality. Burning in subtitles needs a video transcode and a format
// ffmpeg can render (text through libass, PGS and DVD bitmaps through overlay).
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
	if src.Probe.Mixed() {
		t.AudioCopy = false // the concat filter decodes the parts
	}
	v, hasVideo := videoStream(src.Probe)
	_, hasAudio := audioStream(src.Probe)
	if !hasVideo && !hasAudio {
		return Plan{}, errors.New("source has neither video nor audio")
	}
	burn, err := burnInOf(src.Probe, t)
	if err != nil {
		return Plan{}, err
	}
	videoBitrate := 0
	if t.Mode == ModeTranscode && hasVideo {
		if !isHDR(v) {
			t.Tonemap = false // before heightLimit: an SDR source never tonemaps on the CPU
		}
		limit, err := heightLimit(t)
		if err != nil {
			return Plan{}, err
		}
		t.MaxHeight = limit
		// The rung of the output: the limit, or the source when it is smaller (no upscaling).
		h := limit
		if v.Height > 0 {
			h = min(h, v.Height)
		}
		r := rungFor(h)
		t.Quality = r.Quality
		videoBitrate = r.Bitrate
		if t.MaxBitrate > 0 && t.MaxBitrate < videoBitrate {
			videoBitrate = t.MaxBitrate
		}
	} else if burn.kind != burnNone {
		return Plan{}, errors.New("burning in subtitles needs a video transcode")
	}

	var starts []time.Duration
	if t.Mode == ModeRemux && hasVideo {
		if len(src.Keyframes) == 0 {
			return Plan{}, ErrNoKeyframes
		}
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
	p.VideoBitrate = videoBitrate
	p.Bandwidth = bandwidth(src.Probe, t, videoBitrate)
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
// quality, except for an output above 1080 lines (2160p, ADR 0021) at 5. 4K HDR with tonemapping
// 4, other 4K 3, up to 1080p 2 (output <= 720p: 1.5, by MaxHeight, Quality or a source of at most
// 720 lines), remux 0.5, audio only 0.25. Remux includes an AAC encode of the audio.
func Cost(src Source, t Target) float64 {
	v, ok := videoStream(src.Probe)
	if !isHDR(v) {
		t.Tonemap = false // as NewPlan: an SDR source keeps the limit of its backend
	}
	limit, err := heightLimit(t)
	if err != nil {
		limit = defaultMaxHeight
	}
	if v.Height > 0 {
		limit = min(limit, v.Height)
	}
	switch {
	case !ok:
		return costAudioOnly
	case t.Mode == ModeRemux || t.VideoCopy:
		return costRemux
	case limit > defaultMaxHeight:
		return cost2160p
	case is4K(v) && t.Tonemap && isHDR(v):
		return cost4KHDR
	case is4K(v):
		return cost4KSDR
	case limit <= 720:
		return costHDSmallOut
	default:
		return costHD
	}
}

func is4K(v Stream) bool { return v.Width > 2560 || v.Height > 1440 }

// isHDR reports PQ (HDR10, HDR10+, Dolby Vision base layer) and HLG sources, and Dolby Vision
// without a transfer tag (profile 5 signals IPT-PQ-c2 only in its configuration record) unless its
// base layer is SDR (Stream.IsHDR).
func isHDR(v Stream) bool { return v.IsHDR() }

// videoStream returns the first real video stream (cover art excluded).
func videoStream(p Probe) (Stream, bool) {
	for _, s := range p.Streams {
		if s.Type == "video" && !s.AttachedPic {
			return s, true
		}
	}
	return Stream{}, false
}

// selectedAudio returns the stream t.AudioStream names when it is an audio stream, else the
// default one (audioStream); a zero Target (AudioStream 0) thus keeps the default unless stream 0
// is audio.
func selectedAudio(p Probe, t Target) (Stream, bool) {
	if s, ok := streamByIndex(p, t.AudioStream); ok && s.Type == "audio" {
		return s, true
	}
	return audioStream(p)
}

// streamByIndex returns the stream with the ffprobe index i.
func streamByIndex(p Probe, i int) (Stream, bool) {
	if i < 0 {
		return Stream{}, false
	}
	j := slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Index == i })
	if j < 0 {
		return Stream{}, false
	}
	return p.Streams[j], true
}

// audioStream returns the file's default audio stream, else the first one (trackpick.FileDefault,
// the last step of the track choice).
func audioStream(p Probe) (Stream, bool) {
	i := trackpick.FileDefault(p.AudioTracks())
	j := slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Type == "audio" && s.Index == i })
	if i < 0 || j < 0 {
		return Stream{}, false
	}
	return p.Streams[j], true
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

// bandwidth is the peak bit rate for the master playlist: the encoder cap plus audio when
// encoding, the container bit rate with 20 % headroom when copying (and for copied audio-only
// sources), the AAC rate for encoded audio-only sources.
func bandwidth(p Probe, t Target, videoBitrate int) int {
	_, hasVideo := videoStream(p)
	if !hasVideo && !t.AudioCopy && t.Mode == ModeTranscode {
		if t.AudioBitrate > 0 {
			return t.AudioBitrate * 1000
		}
		return audioBitrate
	}
	if !hasVideo || t.VideoCopy {
		br := int(p.Format.BitRate)
		if br <= 0 {
			br = rungFor(defaultMaxHeight).Bitrate
		}
		return br * 6 / 5
	}
	return videoBitrate + audioBitrate
}

// CodecsString returns the RFC 6381 CODECS attribute for the master playlist: H.264 High at the
// level of the output height (h264Level: 4.1, above 1080 lines 5.1) and AAC LC when encoding, the
// source codecs when copying (the audio of the chosen track).
func CodecsString(plan Plan, src Source) string {
	var codecs []string
	if v, ok := videoStream(src.Probe); ok {
		if plan.Target.VideoCopy {
			if c := videoCodecString(v, plan.Target.VideoTag); c != "" {
				codecs = append(codecs, c)
			}
		} else {
			codecs = append(codecs, fmt.Sprintf("avc1.6400%02X", h264Level(plan.Height)))
		}
	}
	if a, ok := selectedAudio(src.Probe, plan.Target); ok {
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
