package offline

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Download profiles (M5 design 3.1): the output is always H.264 (8 bit, SDR) and AAC stereo in
// fragmented MP4 with only the chosen audio track. The video is copied when the source already
// is H.264 8 bit SDR within the profile's height and close to its bit rate; otherwise it is encoded
// at a constant bit rate, so the size matches the estimate (O-1: ±10 %). HDR and Dolby Vision are
// tonemapped like a playback transcode (playback.Decide); a source is never scaled up.

// copyHeadroom is how far the source video bit rate may exceed the profile's and still be copied.
const copyHeadroom = 1.1

// containerOverhead is the share the fragmented MP4 adds to the elementary streams.
const containerOverhead = 1.02

// choice is the decision for one download quality of one media file.
type choice struct {
	Profile        model.DownloadProfile
	Target         transcode.Target
	Copy           bool // video copied (remux)
	AudioCopy      bool // AAC stereo copied
	Width, Height  int
	VideoKbps      int // output video bit rate (the source's when copied)
	AudioKbps      int
	DurationMs     int64
	EstimatedBytes int64
}

// parseProbe decodes a stored probe; nil when the file is not probed or the JSON is unreadable.
func parseProbe(raw string) *transcode.Probe {
	if raw == "" {
		return nil
	}
	var p transcode.Probe
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	return &p
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

// selectedAudio returns the audio stream with the index, else the default one, or nil.
func selectedAudio(p *transcode.Probe, index int) *transcode.Stream {
	if p == nil {
		return nil
	}
	def := defaultAudio(p)
	for _, want := range []int{index, def} {
		for i := range p.Streams {
			if s := &p.Streams[i]; s.Type == "audio" && s.Index == want && want >= 0 {
				return s
			}
		}
	}
	return nil
}

// audioKbpsGuess estimates the bit rate of a source audio stream; the stored probe has the
// container bit rate only.
func audioKbpsGuess(a transcode.Stream) int {
	ch := max(a.Channels, 2)
	switch {
	case a.Codec == "truehd" || a.Codec == "mlp" || a.Codec == "flac" || a.Codec == "alac" || strings.HasPrefix(a.Codec, "pcm_"):
		return 384 * ch // lossless
	case a.Codec == "dts":
		return 1536
	default:
		return 80 * ch // lossy: AAC, MP3 or Opus stereo 160, AC-3 or E-AC-3 5.1 480
	}
}

// eightBit reports whether the video is 8-bit 4:2:0 (an unknown pixel format counts as such, like
// playback's browserDecodable).
func eightBit(v *transcode.Stream) bool {
	profile := strings.ToLower(v.Profile)
	if strings.Contains(profile, "10") || strings.Contains(profile, "4:2:2") || strings.Contains(profile, "4:4:4") {
		return false
	}
	switch strings.ToLower(v.PixFmt) {
	case "", "yuv420p", "yuvj420p", "nv12":
		return true
	}
	return false
}

// durationOf returns the duration of the source: the probe's, else the media file's.
func durationOf(file model.MediaFile, p *transcode.Probe) time.Duration {
	if p != nil && p.Format.Duration > 0 {
		return p.Format.Duration
	}
	return time.Duration(file.DurationMs) * time.Millisecond
}

// sourceVideoKbps derives the video bit rate of the source: the container bit rate (else the size
// over the duration) minus the estimated audio streams; 0 when it cannot be derived.
func sourceVideoKbps(file model.MediaFile, p *transcode.Probe, d time.Duration) int {
	if p == nil || d <= 0 {
		return 0
	}
	total := p.Format.BitRate
	if total <= 0 {
		size := p.Format.Size
		if size <= 0 {
			size = file.SizeBytes
		}
		total = int64(float64(size) * 8 / d.Seconds())
	}
	kbps := int(total / 1000)
	for _, s := range p.Streams {
		if s.Type == "audio" {
			kbps -= audioKbpsGuess(s)
		}
	}
	return max(kbps, 0)
}

// decide picks the target of a download of file in profile prof with the audio stream audio (-1 =
// the default track) and estimates its size.
func decide(file model.MediaFile, p *transcode.Probe, prof model.DownloadProfile, audio int) choice {
	return decideWith(file, p, prof, audio, true)
}

// decideWith is decide; without allowCopy the video is always encoded (a remux whose keyframe
// index cannot be computed).
func decideWith(file model.MediaFile, p *transcode.Probe, prof model.DownloadProfile, audio int, allowCopy bool) choice {
	c := choice{Profile: prof}
	d := durationOf(file, p)
	c.DurationMs = d.Milliseconds()
	v := mainVideo(p)
	srcW, srcH := file.Width, file.Height
	if v != nil && v.Height > 0 {
		srcW, srcH = v.Width, v.Height
	}
	if a := selectedAudio(p, audio); a != nil {
		audio = a.Index
		c.AudioCopy = a.Codec == "aac" && a.Channels >= 1 && a.Channels <= 2
		c.AudioKbps = prof.AudioBitrate
		if c.AudioCopy {
			c.AudioKbps = audioKbpsGuess(*a)
		}
	} else if p == nil && file.AudioCodec != "" {
		c.AudioKbps = prof.AudioBitrate // not probed yet: assume one track that is encoded
	}

	videoKbps := sourceVideoKbps(file, p, d)
	hdr := v != nil && v.IsHDR()
	c.Copy = allowCopy && v != nil && v.Codec == "h264" && eightBit(v) && !hdr && srcH > 0 && srcH <= prof.MaxHeight &&
		videoKbps > 0 && float64(videoKbps) <= copyHeadroom*float64(prof.VideoBitrate)
	if c.Copy {
		c.Target = transcode.Target{
			Mode: transcode.ModeRemux, VideoCopy: true, AudioCopy: c.AudioCopy, SubtitleBurnIn: -1, AudioStream: audio,
			AudioBitrate: prof.AudioBitrate,
		}
		c.Width, c.Height, c.VideoKbps = srcW, srcH, videoKbps
	} else {
		c.Target = transcode.Target{
			Mode: transcode.ModeTranscode, AudioCopy: c.AudioCopy, MaxHeight: prof.MaxHeight, Tonemap: hdr,
			SubtitleBurnIn: -1, AudioStream: audio, MaxBitrate: prof.VideoBitrate * 1000, ConstantRate: true,
			AudioBitrate: prof.AudioBitrate,
		}
		c.Width, c.Height = outputSize(srcW, srcH, prof.MaxHeight)
		c.VideoKbps = transcodeKbps(prof, c.Height)
	}
	c.EstimatedBytes = estimate(c.VideoKbps+c.AudioKbps, d)
	return c
}

// outputSize is the size of an encode capped at maxHeight lines: never scaled up, the width by the
// source's aspect ratio (16:9 when unknown), both even, like transcode.NewPlan.
func outputSize(srcW, srcH, maxHeight int) (w, h int) {
	h = maxHeight
	if srcH > 0 {
		h = min(h, srcH)
	}
	if srcW > 0 && srcH > 0 {
		w = int(math.Round(float64(srcW) * float64(h) / float64(srcH)))
	} else {
		w = h * 16 / 9
	}
	return w &^ 1, h &^ 1
}

// transcodeKbps is the constant video bit rate of an encode of the given output height: the
// profile's rate, capped by the ladder rung of that height (transcode.NewPlan: a 360-line source
// gets the 360p rung's rate).
func transcodeKbps(prof model.DownloadProfile, height int) int {
	rate := prof.VideoBitrate
	if rungs := transcode.Rungs(height); len(rungs) > 0 {
		rate = min(rate, rungs[0].Bitrate/1000)
	}
	return rate
}

// estimate is the size of a file of kbps over d with the container overhead.
func estimate(kbps int, d time.Duration) int64 {
	if kbps <= 0 || d <= 0 {
		return 0
	}
	return int64(float64(kbps) * 1000 / 8 * d.Seconds() * containerOverhead)
}
