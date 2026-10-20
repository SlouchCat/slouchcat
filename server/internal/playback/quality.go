package playback

import "github.com/slouchcat/slouchcat/server/internal/model"

// Quality ladder (W-8, plan 3.5, M4 design section 3.1). Transcodes encode H.264 on one rung; the
// rung caps the output height and the peak video bit rate, audio is AAC at audioBitrate. The
// output never exceeds 1080p (AN-17): 4K sources are scaled when transcoded.
//
// The table is the playback package's own until package transcode exposes its rungs (stream A of
// M4: transcode.Rungs); both carry the same values.

// Rung is one quality rung: the output height cap and the peak video bit rate in bit/s.
type Rung struct {
	Quality string
	Height  int
	Bitrate int
}

// ladder lists the rungs from the highest to the lowest.
var ladder = []Rung{
	{Quality: model.Quality1080, Height: 1080, Bitrate: 8_000_000},
	{Quality: model.Quality720, Height: 720, Bitrate: 4_000_000},
	{Quality: model.Quality480, Height: 480, Bitrate: 2_000_000},
	{Quality: model.Quality360, Height: 360, Bitrate: 1_000_000},
}

// audioBitrate is the AAC bit rate of transcoded audio.
const audioBitrate = 160_000

// phoneRung is the highest rung of a device whose screen is smaller than 1080 lines (phone
// profile, plan 3.5).
const phoneRung = 720

// rungIndex returns the position of a rung in the ladder, or -1 for an unknown name (auto).
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
	return q == "" || q == model.QualityAuto || rungIndex(q) >= 0
}

// sourceTop returns the index of the highest rung that a source of height h fills: the smallest
// rung at least as high as the source, so a 1920×800 film keeps its size on the 1080p rung. An
// unknown height (0) and anything above 1080 lines start at the top.
func sourceTop(h int) int {
	top := 0
	for i, r := range ladder {
		if h > 0 && r.Height >= h {
			top = i
		}
	}
	return top
}

// deviceCap is the height cap a device sets on transcodes: phoneRung for a phone and for a screen
// whose short side is below 1080 device pixels, else none (0). Direct play and remux ignore it.
func deviceCap(c model.PlaybackCapabilities) int {
	if c.Type == "phone" {
		return phoneRung
	}
	short := c.MaxHeight
	if c.MaxWidth > 0 && (short == 0 || c.MaxWidth < short) {
		short = c.MaxWidth
	}
	if short > 0 && short < 1080 {
		return phoneRung
	}
	return 0
}

// rungAllowed reports whether rung i fits the limits: the profile's maximum height, the device
// cap and the bandwidth limit (video bit rate of the rung). The lowest rung is always allowed by
// bandwidth (the limit then caps its bit rate).
func rungAllowed(i int, limits model.PlaybackLimits, devCap int) bool {
	r := ladder[i]
	if limits.MaxHeight > 0 && r.Height > limits.MaxHeight {
		return false
	}
	if devCap > 0 && r.Height > devCap {
		return false
	}
	if limits.MaxBitrateKbps > 0 && r.Bitrate > limits.MaxBitrateKbps*1000 && i < len(ladder)-1 {
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

// outputSize is the size of a transcode on a rung: the source scaled to the rung height (never
// up), the width by the source's aspect ratio and even; 16:9 when the source size is unknown.
func outputSize(srcW, srcH int, r Rung) (w, h int) {
	h = r.Height
	if srcH > 0 && srcH < h {
		h = srcH
	}
	switch {
	case srcW > 0 && srcH > 0:
		w = srcW * h / srcH
	default:
		w = h * 16 / 9
	}
	return w &^ 1, h &^ 1
}

// rungBitrate is the peak video bit rate of a transcode on rung r under the bandwidth limit.
func rungBitrate(r Rung, limits model.PlaybackLimits) int {
	if limits.MaxBitrateKbps > 0 {
		return min(r.Bitrate, limits.MaxBitrateKbps*1000)
	}
	return r.Bitrate
}

// QualityOptions lists the qualities a playback offers (PlaybackInfo.qualities, highest first):
// auto with the size and bit rate of the running output, then every rung with its size and peak
// bit rate, available when the source fills it and the limits and the device allow it. current
// marks what the client asked for: auto, or for a manual choice the rung that runs (the requested
// one, or the next lower one that the limits or the budget allowed).
func QualityOptions(d Decision, srcW, srcH int, limits model.PlaybackLimits, caps model.PlaybackCapabilities, requested string) []model.QualityOption {
	devCap := deviceCap(caps)
	top := sourceTop(srcH)
	manual := rungIndex(requested) >= 0
	if manual && d.Mode == ModeTranscode {
		requested = d.Quality
	}
	out := make([]model.QualityOption, 0, len(ladder)+1)
	out = append(out, model.QualityOption{
		Quality: model.QualityAuto, Width: d.Width, Height: d.Height, Bitrate: d.Bitrate, Available: true, Current: !manual,
	})
	for i, r := range ladder {
		w, h := outputSize(srcW, srcH, r)
		out = append(out, model.QualityOption{
			Quality: r.Quality, Width: w, Height: h, Bitrate: rungBitrate(r, limits),
			Available: i >= top && rungAllowed(i, limits, devCap),
			Current:   manual && requested == r.Quality,
		})
	}
	return out
}
