package playback

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Playback modes of a decision. ModeDirect plays the file as it is through the Range endpoint;
// ModeRemux copies the video into fMP4-HLS; ModeTranscode encodes H.264/AAC (plan 10.3).
const (
	ModeDirect    = "direct"
	ModeRemux     = "remux"
	ModeTranscode = "transcode"
)

// Reasons why a file is not played directly (stable codes, stored in playback_sessions.reason and
// returned in PlaybackInfo.transcoding.reason).
const (
	ReasonContainer  = "container"   // the container needs remuxing (MKV and others)
	ReasonVideoCodec = "video_codec" // the client cannot decode the video codec directly
	ReasonAudioCodec = "audio_codec" // the client cannot decode the audio codec, audio is encoded to AAC
	ReasonHDR        = "hdr"         // HDR source on a client without HDR passthrough: tonemap to SDR
	// ReasonPixelFormat: H.264 High 10 or a 4:2:2/4:4:4 source that browsers do not decode.
	ReasonPixelFormat = "pixel_format"
	// ReasonKeyframesPending: a remux candidate whose keyframe index is not computed yet; it is
	// transcoded on the 4-s grid while the index job runs (see Service.ensureKeyframes).
	ReasonKeyframesPending = "keyframes_pending"
	// ReasonDolbyVision: Dolby Vision profile 5 or 7 has no base layer a browser can show; it is
	// transcoded with tonemapping (profile 8 remuxes its HDR10 base layer).
	ReasonDolbyVision = "dolby_vision"
	ReasonBandwidth   = "bandwidth"     // the source bit rate exceeds the profile's bandwidth limit
	ReasonMaxHeight   = "max_height"    // the source is higher than the profile's maximum height
	ReasonQuality     = "quality"       // a manual rung below the source
	ReasonAudioTrack  = "audio_track"   // a non-default audio track: remuxed with that track
	ReasonBurnIn      = "subtitle_burn" // burned-in subtitles need a transcode
	// ReasonBudgetLowered: the transcode budget was exhausted, an SDR source runs one rung lower
	// (plan 3.5).
	ReasonBudgetLowered = "budget_lowered"
)

// MaxTranscodeHeight caps transcode output (AN-17: up to 1080p, 4K sources are scaled).
const MaxTranscodeHeight = 1080

// Decision is how a media file is played on a client. It is comparable, so tests compare it as a
// whole.
type Decision struct {
	Mode      string // ModeDirect, ModeRemux, ModeTranscode
	VideoCopy bool   // remux: keep the video stream
	AudioCopy bool   // keep the audio stream (else encode AAC)
	MaxHeight int    // transcode output height cap (the rung), 0 = keep
	Tonemap   bool   // HDR10/HLG/Dolby Vision → SDR
	VideoTag  string // "hvc1" when remuxing HEVC (Safari)
	Reason    string // empty for direct play

	// Quality is the rung of a transcode, model.QualityAuto for a remux and empty for direct play
	// (playback_sessions.quality). Width, Height and Bitrate describe the output: the rung's size
	// and peak video bit rate for a transcode, the source's for direct play and remux (0 unknown).
	Quality          string
	Width, Height    int
	Bitrate          int
	AudioStream      int    // stream index of the audio track to output; -1 = default (or no audio)
	SubtitleTrack    int64  // tracks.id of the selected subtitle track, 0 = none
	SubtitleDelivery string // model.SubtitleText or model.SubtitleBurn; empty without subtitles
	BurnStream       int    // stream index of an embedded subtitle to burn in, -1 = none
	BurnSidecar      string // sidecar subtitle to burn in, relative to the library root
	// Audio marks an audio-only source (no video stream besides cover art, M6): direct play of
	// the file, or an audio-only HLS output (AAC copy or audioOnlyKbps); Quality is auto and the
	// output has no size.
	Audio bool
}

// Request is the client's side of a decision: its capability profile, the effective limits of
// the profile and the wanted tracks and quality.
type Request struct {
	Caps   model.PlaybackCapabilities // normalized (normalizeCapabilities)
	Limits model.PlaybackLimits       // effective W-8 limits of the profile
	// AudioStream is the stream index of the wanted audio track; -1 = the file's default track.
	AudioStream int
	// Subtitle is the wanted subtitle track (nil = none) and Delivery how it is shown: auto, text
	// or burn. Text for a bitmap track is rejected before (Start validates).
	Subtitle *model.Track
	Delivery string
	// Quality is auto (or empty) or a rung; a rung below the source forces a transcode.
	Quality string
	// ForceTranscode rebuilds a recorded transcode (Service.rebuild): the decision stays a
	// transcode on the recorded rung even where the client could remux now.
	ForceTranscode bool
	// MultiPart is set for an audiobook of several files: it always plays as one HLS timeline over
	// its parts (transcode.Source.Parts).
	MultiPart bool
}

// defaultCapabilities stand for a client that reports nothing: a browser with MSE that plays
// H.264 and AAC in MP4 (every browser the M1a client supports).
var defaultCapabilities = model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}

// maxDeviceName caps the reported device name (devices.name CHECK).
const maxDeviceName = 80

var deviceTypes = []string{"desktop", "phone", "tablet", "tv", "other"}

// normalizeCapabilities lower-cases the codec and container names, fills in the defaults of an
// absent report and drops what the store would reject (an over-long name, an unknown type).
func normalizeCapabilities(c *model.PlaybackCapabilities) model.PlaybackCapabilities {
	if c == nil {
		return defaultCapabilities
	}
	out := model.PlaybackCapabilities{
		HLS: strings.ToLower(c.HLS), MaxWidth: max(c.MaxWidth, 0), MaxHeight: max(c.MaxHeight, 0), HDR: c.HDR,
		Name: strings.TrimSpace(c.Name), Type: strings.ToLower(c.Type),
	}
	lower := func(in []string) []string {
		var r []string
		for _, v := range in {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" && !slices.Contains(r, v) {
				r = append(r, v)
			}
		}
		return r
	}
	out.Containers, out.Video, out.Audio = lower(c.Containers), lower(c.Video), lower(c.Audio)
	switch out.HLS {
	case "native", "mse", "none":
	default:
		out.HLS = "mse"
	}
	if utf8.RuneCountInString(out.Name) > maxDeviceName {
		out.Name = string([]rune(out.Name)[:maxDeviceName])
	}
	if !slices.Contains(deviceTypes, out.Type) {
		out.Type = ""
	}
	return out
}

// isHDR reports whether the main video stream is HDR (transcode.Stream.IsHDR, the rule of
// transcode.NewPlan, so the decision and the plan agree on tonemapping): PQ or HLG transfer or a
// Dolby Vision configuration record whose base layer is not SDR (8.2/9.2 are SDR); BT.2020
// primaries alone (an SDR wide-gamut master) do not count.
func isHDR(v *transcode.Stream) bool {
	return v != nil && v.IsHDR()
}

// browserDecodable reports whether browsers decode the pixel format of the main video stream:
// H.264 only as 8-bit 4:2:0 (High 10, High 4:2:2 and High 4:4:4 fail in MSE and Safari), HEVC
// as 4:2:0 in 8 or 10 bit (Main, Main 10). Without a probe the stream counts as decodable.
func browserDecodable(codec string, v *transcode.Stream) bool {
	if v == nil {
		return true
	}
	pix := strings.ToLower(v.PixFmt)
	profile := strings.ToLower(v.Profile)
	if strings.Contains(profile, "4:2:2") || strings.Contains(profile, "4:4:4") || strings.Contains(profile, "rext") {
		return false
	}
	if pix != "" && !strings.Contains(pix, "420") && pix != "nv12" && pix != "p010le" {
		return false // 4:2:2, 4:4:4, gray, rgb
	}
	if codec == "h264" {
		if strings.Contains(profile, "high 10") {
			return false
		}
		if pix != "" && pix != "yuv420p" && pix != "yuvj420p" && pix != "nv12" {
			return false // 10 or 12 bit
		}
	}
	return true
}

// mainVideo returns the first video stream that is not cover art, or nil.
func mainVideo(p *transcode.Probe) *transcode.Stream {
	if p == nil {
		return nil
	}
	for i := range p.Streams {
		if p.Streams[i].Type == "video" && !p.Streams[i].AttachedPic {
			return &p.Streams[i]
		}
	}
	return nil
}

// defaultAudio returns the stream index of the default audio track (the first flagged default,
// else the first audio stream), or -1 without audio or probe.
func defaultAudio(p *transcode.Probe) int {
	if p == nil {
		return -1
	}
	first := -1
	for _, s := range p.Streams {
		if s.Type != "audio" {
			continue
		}
		if s.Default {
			return s.Index
		}
		if first < 0 {
			first = s.Index
		}
	}
	return first
}

// audioStream returns the audio stream with the index, or nil.
func audioStream(p *transcode.Probe, index int) *transcode.Stream {
	if p == nil {
		return nil
	}
	for i := range p.Streams {
		if p.Streams[i].Type == "audio" && p.Streams[i].Index == index {
			return &p.Streams[i]
		}
	}
	return nil
}

func mp4Family(container string) bool {
	switch strings.ToLower(container) {
	case "mp4", "m4v", "mov", "m4a", "m4b":
		return true
	}
	return false
}

// containerPlayable reports whether the client's media element plays the container directly:
// the reported containers ("mp4" stands for the MP4 family, M4A and M4B included), MP4 only
// without a report. An MP3 file plays wherever the client decodes MP3 (clients report audio
// codecs, not audio containers).
func containerPlayable(container string, caps model.PlaybackCapabilities) bool {
	c := strings.ToLower(container)
	if mp4Family(c) {
		c = "mp4"
	}
	if c == "mp3" {
		return slices.Contains(caps.Audio, "mp3")
	}
	if len(caps.Containers) == 0 {
		return c == "mp4"
	}
	return slices.Contains(caps.Containers, c)
}

// directVideo reports whether the client plays the video codec without remuxing: H.264, and the
// other codecs it lists (HEVC, AV1, VP9), except HEVC on a native-HLS client (Safari), which needs
// the hvc1 tag of a remux.
func directVideo(codec string, caps model.PlaybackCapabilities) bool {
	switch codec {
	case "h264":
		return true
	case "hevc":
		return caps.HLS != "native"
	case "av1", "vp9":
		return true
	}
	return false
}

// Decide picks the playback mode for a media file on a client (M4 design section 3, plan 10.3).
// probe may be nil when the file is not probed yet; the codec columns of the file are used then
// and the source counts as SDR with its default audio track.
//
//   - Direct play: the container is one the client plays, it decodes the video (H.264, or HEVC,
//     AV1, VP9 when listed; never HEVC on Safari) and the pixel format, the audio track is the
//     default one and decodable, subtitles are off or delivered as text, an HDR source only on an
//     HDR display (never Dolby Vision 5/7), the source within the profile's height and bandwidth
//     limits, and the quality auto (or a rung the source does not exceed).
//   - A client without HLS support (neither native nor MSE) → direct; the client checks the file
//     itself and shows that it cannot play it.
//   - Transcode, in the order of the reason codes: Dolby Vision 5/7 (dolby_vision); HDR without
//     passthrough (hdr: an HDR display and native HLS, or HEVC through MSE); H.264 High 10 and
//     4:2:2/4:4:4 (pixel_format); burned-in subtitles (subtitle_burn); a manual rung below the
//     source (quality); the source above the height limit (max_height) or the bandwidth limit
//     (bandwidth); a video codec the client cannot decode or remux (video_codec). The rung is the
//     highest the source fills and the limits and the device allow (a screen below 1080 lines:
//     720p), or the manual one; HDR is tonemapped; AAC audio is copied.
//   - Remux otherwise (H.264 or HEVC the client decodes, HEVC tagged hvc1, Dolby Vision 8 as its
//     HDR10 base layer): reason container, video_codec (HEVC for Safari), audio_track (a
//     non-default track) or audio_codec; the audio is copied when the client decodes it, else
//     encoded to AAC.
func Decide(file model.MediaFile, probe *transcode.Probe, req Request) Decision {
	caps, limits := req.Caps, req.Limits
	video := strings.ToLower(file.VideoCodec)
	v := mainVideo(probe)
	srcW, srcH := file.Width, file.Height
	if v != nil && v.Height > 0 {
		srcW, srcH = v.Width, v.Height
	}
	srcBitrate := int(file.Bitrate)
	if srcBitrate == 0 && probe != nil {
		srcBitrate = int(probe.Format.BitRate)
	}

	d := Decision{AudioStream: -1, BurnStream: -1, Width: srcW, Height: srcH, Bitrate: srcBitrate}

	// Audio: the wanted track or the default one.
	defAudio := defaultAudio(probe)
	audio, isDefaultAudio := strings.ToLower(file.AudioCodec), true
	if req.AudioStream >= 0 && req.AudioStream != defAudio {
		if a := audioStream(probe, req.AudioStream); a != nil {
			audio, isDefaultAudio = strings.ToLower(a.Codec), false
			d.AudioStream = a.Index
		}
	} else if defAudio >= 0 {
		d.AudioStream = defAudio
		if a := audioStream(probe, defAudio); a != nil && a.Codec != "" {
			audio = strings.ToLower(a.Codec)
		}
	}

	// Subtitles: text where the client renders the track, else burned in.
	if sub := req.Subtitle; sub != nil {
		d.SubtitleTrack = sub.ID
		d.SubtitleDelivery = model.SubtitleBurn
		switch {
		case sub.Text && req.Delivery != model.SubtitleBurn:
			d.SubtitleDelivery = model.SubtitleText
		case sub.Source == "sidecar":
			d.BurnSidecar = sub.RelPath
		default:
			d.BurnStream = sub.Index
		}
	}
	burn := d.SubtitleDelivery == model.SubtitleBurn

	if v == nil && video == "" && audio != "" {
		return decideAudio(d, file, req, audio, isDefaultAudio, probe != nil && probe.Mixed())
	}
	if caps.HLS == "none" && !req.ForceTranscode {
		d.Mode = ModeDirect
		return d
	}

	videoOK := video != "" && slices.Contains(caps.Video, video)
	audioOK := audio == "" || slices.Contains(caps.Audio, audio)
	pixelsOK := browserDecodable(video, v)
	remuxable := video == "h264" || video == "hevc"
	hdr := isHDR(v)
	dvNoBase := v != nil && (v.DolbyVisionProfile == 5 || v.DolbyVisionProfile == 7)
	hdrDirect := !hdr || (caps.HDR && videoOK && !dvNoBase)
	hdrRemux := !hdr || (caps.HDR && videoOK && !dvNoBase && (caps.HLS == "native" || video == "hevc"))
	heightOK := limits.MaxHeight == 0 || srcH == 0 || srcH <= limits.MaxHeight
	bitrateOK := limits.MaxBitrateKbps == 0 || srcBitrate == 0 || srcBitrate <= limits.MaxBitrateKbps*1000
	top := sourceTop(srcH)
	wanted := rungIndex(req.Quality)
	manual := wanted > top

	containerOK := containerPlayable(file.Container, caps)
	direct := containerOK && videoOK && directVideo(video, caps) && pixelsOK && isDefaultAudio && audioOK &&
		!burn && hdrDirect && heightOK && bitrateOK && !manual && !req.ForceTranscode
	if direct {
		d.Mode = ModeDirect
		return d
	}

	var reason string
	switch {
	case dvNoBase:
		reason = ReasonDolbyVision
	case !hdrRemux:
		reason = ReasonHDR
	case !pixelsOK:
		reason = ReasonPixelFormat
	case burn:
		reason = ReasonBurnIn
	case manual:
		reason = ReasonQuality
	case !heightOK:
		reason = ReasonMaxHeight
	case !bitrateOK:
		reason = ReasonBandwidth
	case !videoOK || !remuxable:
		reason = ReasonVideoCodec
	case req.ForceTranscode:
		reason = ReasonQuality
	}
	if reason == "" {
		d.Mode, d.VideoCopy, d.AudioCopy, d.Quality = ModeRemux, true, audioOK, model.QualityAuto
		if video == "hevc" {
			d.VideoTag = "hvc1"
		}
		switch {
		case !containerOK:
			d.Reason = ReasonContainer
		case !directVideo(video, caps):
			d.Reason = ReasonVideoCodec
		case !isDefaultAudio:
			d.Reason = ReasonAudioTrack
		default:
			d.Reason = ReasonAudioCodec
		}
		return d
	}
	from := top
	if wanted > top || (req.ForceTranscode && wanted >= 0) {
		from = wanted
	}
	return transcodeOn(d, pickRung(from, limits, deviceCap(caps)), reason, hdr, audio == "aac" && audioOK, srcW, srcH, limits)
}

// audioOnlyKbps is the AAC bit rate of an audio-only output that encodes (M6 design 3.3).
const audioOnlyKbps = 128

// decideAudio decides an audio-only source (M6 design 3.3): direct play of a single file in MP4
// (M4A, M4B) with AAC or of an MP3 file, when the client decodes the codec, the track is the
// default one and the bit rate fits the profile's bandwidth limit; a client without HLS also gets
// the file. Anything else, and every audiobook of several parts, is an audio-only HLS transcode:
// AAC is copied when the client decodes it (and the bit rate fits), other codecs are encoded to
// AAC at audioOnlyKbps (also for the parts of a book that differ in their streams, mixed: the
// concat filter decodes them; transcode.Probe.Mixed). Reasons: container (several parts, or a
// container the client does not play directly), audio_codec, audio_track, bandwidth.
func decideAudio(d Decision, file model.MediaFile, req Request, audio string, isDefaultAudio, mixed bool) Decision {
	caps, limits := req.Caps, req.Limits
	d.Audio, d.Width, d.Height = true, 0, 0
	container := strings.ToLower(file.Container)
	codecOK := slices.Contains(caps.Audio, audio)
	formatOK := (mp4Family(container) && audio == "aac") || (container == "mp3" && audio == "mp3")
	bitrateOK := limits.MaxBitrateKbps == 0 || d.Bitrate == 0 || d.Bitrate <= limits.MaxBitrateKbps*1000
	if !req.MultiPart && !req.ForceTranscode &&
		(caps.HLS == "none" || (formatOK && codecOK && isDefaultAudio && bitrateOK)) {
		d.Mode = ModeDirect
		return d
	}
	d.Mode, d.Quality = ModeTranscode, model.QualityAuto
	d.AudioCopy = audio == "aac" && codecOK && bitrateOK && !mixed
	switch {
	case req.MultiPart:
		d.Reason = ReasonContainer
	case !codecOK:
		d.Reason = ReasonAudioCodec
	case !isDefaultAudio:
		d.Reason = ReasonAudioTrack
	case !bitrateOK:
		d.Reason = ReasonBandwidth
	default:
		d.Reason = ReasonContainer
	}
	if !d.AudioCopy {
		// A bandwidth limit below the AAC rate caps the encode too.
		d.Bitrate = audioOnlyKbps * 1000
		if limits.MaxBitrateKbps > 0 {
			d.Bitrate = min(d.Bitrate, limits.MaxBitrateKbps*1000)
		}
	}
	return d
}

// transcodeOn turns d into a transcode on ladder rung i.
func transcodeOn(d Decision, i int, reason string, hdr, audioCopy bool, srcW, srcH int, limits model.PlaybackLimits) Decision {
	r := ladder[i]
	d.Mode, d.VideoCopy, d.VideoTag, d.Reason = ModeTranscode, false, "", reason
	d.AudioCopy, d.Tonemap = audioCopy, hdr
	d.Quality, d.MaxHeight = r.Quality, r.Height
	d.Width, d.Height = outputSize(srcW, srcH, r)
	d.Bitrate = rungBitrate(r, limits)
	return d
}

// lowered returns the decision one budget step lower (plan 3.5): an SDR transcode above 720p runs
// on the 720p rung instead (reason budget_lowered); ok is false when that saves nothing (HDR
// sources, remux, rungs at or below 720p).
func lowered(d Decision, srcW, srcH int, limits model.PlaybackLimits) (Decision, bool) {
	i := rungIndex(model.Quality720)
	if d.Mode != ModeTranscode || d.Tonemap || d.MaxHeight <= ladder[i].Height {
		return d, false
	}
	return transcodeOn(d, i, ReasonBudgetLowered, false, d.AudioCopy, srcW, srcH, limits), true
}
