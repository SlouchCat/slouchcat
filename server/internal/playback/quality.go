package playback

import (
	"math"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Quality ladder (W-8, plan 3.5, M4 design section 3.1, ADR 0021). Transcodes encode H.264 on one
// rung; the rung caps the output height and the peak video bit rate, audio is AAC at 160 kbit/s.
// auto never goes above 1080p; 2160p is an explicit choice where transcode.Rung2160Allowed holds
// (hardware backend, 4K source, device and profile). original is no rung: it plays the file
// without re-encoding the video (direct play or remux) and falls back to auto where that is
// impossible (concept 13 11.5).

// ladder is the one quality ladder of package transcode (transcode.Rungs of a 2160-line source:
// every rung), from 2160p down to 360p.
var ladder = transcode.Rungs(2160)

// autoTop is the index of the highest rung auto picks (1080p, ADR 0021 point 4).
var autoTop = rungIndex(model.Quality1080)

// phoneRung is the highest rung of a device whose screen is smaller than 1080 lines (phone
// profile, plan 3.5).
const phoneRung = 720

// rungIndex returns the position of a rung in the ladder, or -1 for a name that is no rung: auto,
// original and unknown names. original is a quality of its own (Decide, QualityOptions), never
// read as auto.
func rungIndex(quality string) int {
	for i, r := range ladder {
		if r.Quality == quality {
			return i
		}
	}
	return -1
}

// validQuality reports whether q is a quality a client may request (empty = auto).
func validQuality(q string) bool {
	return q == "" || q == model.QualityAuto || q == model.QualityOriginal || rungIndex(q) >= 0
}

// validQualityCap reports whether q is a remembered rung a client may send as qualityCap (V-17):
// a rung below 2160p (contract QualityCap), empty = none.
func validQualityCap(q string) bool {
	return q == "" || rungIndex(q) > rungIndex(model.Quality2160)
}

// sourceTop returns the index of the highest rung that a source of height h fills: the smallest
// rung at least as high as the source, so a 1920×800 film keeps its size on the 1080p rung and a
// 3840×1600 film on the 2160p rung. An unknown height (0) counts as 1080p.
func sourceTop(h int) int {
	if h <= 0 {
		return autoTop
	}
	top := 0
	for i, r := range ladder {
		if r.Height >= h {
			top = i
		}
	}
	return top
}

// deviceShort is the short side of the device's screen in device pixels, 0 when not reported.
func deviceShort(c model.PlaybackCapabilities) int {
	short := c.MaxHeight
	if c.MaxWidth > 0 && (short == 0 || c.MaxWidth < short) {
		short = c.MaxWidth
	}
	return short
}

// deviceCap is the height cap a device sets on transcodes: phoneRung for a phone and for a screen
// whose short side is below 1080 device pixels, else none (0). Direct play and remux ignore it.
func deviceCap(c model.PlaybackCapabilities) int {
	if c.Type == "phone" {
		return phoneRung
	}
	if short := deviceShort(c); short > 0 && short < 1080 {
		return phoneRung
	}
	return 0
}

// uhdAllowed reports whether the 2160p rung may be offered for the video v on the transcode host
// (transcode.Rung2160Allowed: hardware backend, 4K source, a device of at least 1440 lines, a
// profile that allows 2160 lines, SDR or tonemapping on the GPU); never without a probe.
func uhdAllowed(host transcode.Capabilities, v *transcode.Stream, limits model.PlaybackLimits, caps model.PlaybackCapabilities) bool {
	return v != nil && transcode.Rung2160Allowed(host, *v, limits.MaxHeight, deviceShort(caps))
}

// heightCap is the height cap of the transcodes of a device: deviceCap, and 1080 lines unless the
// 2160p rung is allowed (uhd). Direct play and remux ignore it.
func heightCap(c model.PlaybackCapabilities, uhd bool) int {
	devCap := deviceCap(c)
	if top := ladder[autoTop].Height; !uhd && (devCap == 0 || devCap > top) {
		devCap = top
	}
	return devCap
}

// audioBitrate is the bit rate of the AAC audio of a transcode (transcode's 160 kbit/s); the
// bandwidth limit covers video and audio.
const audioBitrate = 160_000

// rungAllowed reports whether rung i fits the limits: the profile's maximum height, the height
// cap of the device (heightCap) and the bandwidth limit (video bit rate of the rung plus the
// audio). The lowest rung is always allowed by bandwidth (the limit then caps its bit rate).
func rungAllowed(i int, limits model.PlaybackLimits, devCap int) bool {
	r := ladder[i]
	if limits.MaxHeight > 0 && r.Height > limits.MaxHeight {
		return false
	}
	if devCap > 0 && r.Height > devCap {
		return false
	}
	if limits.MaxBitrateKbps > 0 && r.Bitrate+audioBitrate > limits.MaxBitrateKbps*1000 && i < len(ladder)-1 {
		return false
	}
	return true
}

// pickRung returns the highest rung at or below rung from that the limits allow; the lowest rung
// when none does (a height limit below 360 lines does not exist).
func pickRung(from int, limits model.PlaybackLimits, devCap int) int {
	for i := max(from, 0); i < len(ladder); i++ {
		if rungAllowed(i, limits, devCap) {
			return i
		}
	}
	return len(ladder) - 1
}

// outputSize is the size of a transcode on a rung: the source scaled into the rung's 16:9 box
// (the rung height, rung height × 16/9 pixels wide; never up) by the source's aspect ratio and
// even, as transcode's plan does, so a 3840x1600 film is 1920x800 on 1080p; the whole box when the
// source size is unknown.
func outputSize(srcW, srcH int, r transcode.Rung) (w, h int) {
	boxH := r.Height
	switch {
	case srcW <= 0 || srcH <= 0:
		w, h = boxH*16/9, boxH
	case srcW*9 > srcH*16 && srcW*9 > boxH*16: // wider than 16:9 and than the box: the box width limits
		w, h = boxH*16/9, int(math.Round(float64(srcH)*float64(boxH*16)/float64(srcW*9)))
	case srcH > boxH:
		w, h = int(math.Round(float64(srcW)*float64(boxH)/float64(srcH))), boxH
	default:
		w, h = srcW, srcH
	}
	return w &^ 1, h &^ 1
}

// rungBitrate is the peak video bit rate of a transcode on rung r under the bandwidth limit: the
// limit minus the audio (the smallest limit, 200 kbit/s, leaves 40 kbit/s of video).
func rungBitrate(r transcode.Rung, limits model.PlaybackLimits) int {
	if limits.MaxBitrateKbps > 0 {
		return max(min(r.Bitrate, limits.MaxBitrateKbps*1000-audioBitrate), 1)
	}
	return r.Bitrate
}

// originalAvailable reports whether quality original can play for the request req (concept 13
// 11.5): decided with original (without a cap), the video is not re-encoded (direct play or
// remux; not with burned-in subtitles, (b)), and the playback d that runs did not find the
// keyframe index of that remux missing ((a), reason keyframes_pending). Audio-only playbacks
// have no original.
func originalAvailable(d Decision, file model.MediaFile, probe *transcode.Probe, req Request) bool {
	if d.Audio || d.Reason == ReasonKeyframesPending {
		return false
	}
	if d.Quality == model.QualityOriginal {
		return true
	}
	req.Quality, req.QualityCap, req.ForceTranscode = model.QualityOriginal, "", false
	return Decide(file, probe, req).Mode != ModeTranscode
}

// transcodeCodec is the video codec of every transcode (ffprobe name).
const transcodeCodec = "h264"

// QualityOptions lists the qualities a playback offers (PlaybackInfo.qualities, highest first):
//
//   - auto with the size, bit rate and codec of the running output; direct when it does not
//     re-encode the video.
//   - original with the source's size, bit rate and codec, available when originalOK (the file
//     plays without re-encoding the video, concept 13 11.5).
//   - every rung from 2160p down with its size and peak bit rate, available when the source fills
//     it and the limits and the device allow it (2160p only where src.uhd); the rung of the
//     source, when available, is direct where original is possible and then shows the source's
//     bit rate and codec, every other rung is an H.264 transcode.
//
// current marks what the client asked for: auto (also for a remembered qualityCap and for an
// original that could not play), original when it plays, or for a manual rung the rung that runs
// (the requested one, or the next lower one that the limits or the budget allowed). A manual rung
// is current only when it is available and runs: a transcode on it, or the source's rung where the
// file plays without re-encoding the video. Otherwise (a remembered 2160p on a title that plays
// directly but does not offer 2160p) auto is current.
//
// An audio-only playback (Decision.Audio) offers one option: auto with the bit rate of the output
// and no size (the contract's Quality enum has no audio rung).
func QualityOptions(d Decision, src source, originalOK bool, requested string) []model.QualityOption {
	if d.Audio {
		return []model.QualityOption{{Quality: model.QualityAuto, Bitrate: d.Bitrate, Available: true, Current: true}}
	}
	devCap := heightCap(src.caps, src.uhd)
	top := sourceTop(src.h)
	codec := strings.ToLower(src.file.VideoCodec)
	original := d.Quality == model.QualityOriginal
	manual := rungIndex(requested) >= 0
	if manual && d.Mode == ModeTranscode {
		requested = d.Quality
	}
	autoCodec, autoDirect := codec, d.Mode != ModeTranscode
	if !autoDirect {
		autoCodec = transcodeCodec
	}
	out := make([]model.QualityOption, 0, len(ladder)+2)
	out = append(out,
		model.QualityOption{
			Quality: model.QualityAuto, Width: d.Width, Height: d.Height, Bitrate: d.Bitrate, Codec: autoCodec, Direct: autoDirect,
			Available: true,
		},
		model.QualityOption{
			Quality: model.QualityOriginal, Width: src.w, Height: src.h, Bitrate: src.bitrate, Codec: codec, Direct: true,
			Available: originalOK, Current: original,
		})
	rungCurrent := false
	for i, r := range ladder {
		o := model.QualityOption{
			Quality: r.Quality, Codec: transcodeCodec, Bitrate: rungBitrate(r, src.limits),
			Available: i >= top && rungAllowed(i, src.limits, devCap),
		}
		o.Current = manual && requested == r.Quality && o.Available && (d.Mode == ModeTranscode || i == top)
		rungCurrent = rungCurrent || o.Current
		o.Width, o.Height = outputSize(src.w, src.h, r)
		if i == top && originalOK && o.Available {
			o.Codec, o.Direct = codec, true
			if src.bitrate > 0 {
				o.Bitrate = src.bitrate
			}
		}
		out = append(out, o)
	}
	out[0].Current = !original && !rungCurrent
	return out
}
