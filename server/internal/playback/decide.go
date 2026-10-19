package playback

import (
	"slices"
	"strings"

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
)

// MaxTranscodeHeight caps transcode output in M1b (AN-17: software transcode up to 1080p SDR).
const MaxTranscodeHeight = 1080

// Decision is how a media file is played on a client.
type Decision struct {
	Mode      string // ModeDirect, ModeRemux, ModeTranscode
	VideoCopy bool   // remux: keep the video stream
	AudioCopy bool   // keep the audio stream (else encode AAC)
	MaxHeight int    // transcode output height cap, 0 = keep
	Tonemap   bool   // HDR10/HLG → SDR
	VideoTag  string // "hvc1" when remuxing HEVC (Safari)
	Reason    string // empty for direct play
}

// defaultCapabilities stand for a client that reports nothing: a browser with MSE that plays
// H.264 and AAC (every browser the M1a client supports).
var defaultCapabilities = model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}

// normalizeCapabilities lower-cases the codec names and fills in the defaults of an absent report.
func normalizeCapabilities(c *model.PlaybackCapabilities) model.PlaybackCapabilities {
	if c == nil {
		return defaultCapabilities
	}
	out := model.PlaybackCapabilities{HLS: strings.ToLower(c.HLS)}
	for _, v := range c.Video {
		out.Video = append(out.Video, strings.ToLower(v))
	}
	for _, a := range c.Audio {
		out.Audio = append(out.Audio, strings.ToLower(a))
	}
	switch out.HLS {
	case "native", "mse", "none":
	default:
		out.HLS = "mse"
	}
	return out
}

// isHDR reports whether the main video stream is HDR10/HLG: PQ or HLG transfer. It is the rule of
// transcode.NewPlan (plan.go isHDR), so the decision and the plan agree on tonemapping; BT.2020
// primaries alone (an SDR wide-gamut master) do not count.
func isHDR(v *transcode.Stream) bool {
	if v == nil {
		return false
	}
	switch strings.ToLower(v.ColorTransfer) {
	case "smpte2084", "arib-std-b67":
		return true
	}
	return false
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

func mp4Family(container string) bool {
	switch strings.ToLower(container) {
	case "mp4", "m4v", "mov":
		return true
	}
	return false
}

// Decide picks the playback mode for a media file on a client (plan 10.3 rows 1–3; subtitles,
// rows 4–6, follow in M4). probe may be nil when the file is not probed yet; the codec columns of
// the file are used then and the source counts as SDR.
//
//   - MP4/M4V/MOV with 8-bit 4:2:0 H.264 and AAC (or no audio) on a client that plays H.264 →
//     direct.
//   - A client without HLS support (neither native nor MSE) → direct; the client checks the file
//     itself and shows that it cannot play it.
//   - H.264 High 10 and 4:2:2/4:4:4 sources (probe profile and pix_fmt) → transcode, reason
//     pixel_format: no browser decodes them.
//   - HDR source → transcode with tonemapping to SDR (<= 1080p), except on a native-HLS client
//     (Safari) that decodes the codec: remux, the HDR stream passes through.
//   - H.264 or HEVC video the client decodes → remux (video copied, HEVC tagged hvc1); the audio
//     is copied when the client lists its codec, else encoded to AAC.
//   - Everything else → transcode to H.264/AAC, at most 1080p; AAC audio is copied.
func Decide(file model.MediaFile, probe *transcode.Probe, caps model.PlaybackCapabilities) Decision {
	video := strings.ToLower(file.VideoCodec)
	audio := strings.ToLower(file.AudioCodec)
	videoOK := video != "" && slices.Contains(caps.Video, video)
	audioOK := audio == "" || slices.Contains(caps.Audio, audio)
	v := mainVideo(probe)
	pixelsOK := browserDecodable(video, v)

	if mp4Family(file.Container) && video == "h264" && (audio == "" || audio == "aac") && videoOK && audioOK && pixelsOK {
		return Decision{Mode: ModeDirect}
	}
	if caps.HLS == "none" {
		return Decision{Mode: ModeDirect}
	}
	hdr := isHDR(v)
	remuxable := video == "h264" || video == "hevc"
	passthrough := caps.HLS == "native" && videoOK && remuxable
	if hdr && !passthrough {
		return Decision{Mode: ModeTranscode, MaxHeight: MaxTranscodeHeight, Tonemap: true,
			AudioCopy: audio == "aac" && audioOK, Reason: ReasonHDR}
	}
	if !pixelsOK {
		return Decision{Mode: ModeTranscode, MaxHeight: MaxTranscodeHeight, Tonemap: hdr,
			AudioCopy: audio == "aac" && audioOK, Reason: ReasonPixelFormat}
	}
	if videoOK && remuxable {
		d := Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: audioOK}
		if video == "hevc" {
			d.VideoTag = "hvc1"
		}
		switch {
		case !mp4Family(file.Container):
			d.Reason = ReasonContainer
		case video != "h264":
			d.Reason = ReasonVideoCodec
		default:
			d.Reason = ReasonAudioCodec
		}
		return d
	}
	return Decision{Mode: ModeTranscode, MaxHeight: MaxTranscodeHeight, AudioCopy: audio == "aac" && audioOK, Reason: ReasonVideoCodec}
}
