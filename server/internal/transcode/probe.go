package transcode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Probe is the parsed result of ffprobe for one media file (plan 3.4 step 3). It is stored as
// JSON in media_files.probe, so its field names are part of the stored format.
type Probe struct {
	Format   Format    `json:"format"`
	Streams  []Stream  `json:"streams"`
	Chapters []Chapter `json:"chapters"`
}

// Format describes the container.
type Format struct {
	Name     string        `json:"name"`     // ffprobe format_name, for example "matroska,webm"
	Duration time.Duration `json:"duration"` // nanoseconds in JSON
	Size     int64         `json:"size"`
	BitRate  int64         `json:"bitRate"`
}

// Stream is one elementary stream of the container.
type Stream struct {
	Index          int    `json:"index"`
	Type           string `json:"type"` // video, audio, subtitle, attachment, data
	Codec          string `json:"codec"`
	Profile        string `json:"profile,omitempty"`
	Level          int    `json:"level,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	PixFmt         string `json:"pixFmt,omitempty"`
	ColorTransfer  string `json:"colorTransfer,omitempty"`
	ColorPrimaries string `json:"colorPrimaries,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channelLayout,omitempty"`
	Language       string `json:"language,omitempty"`
	Title          string `json:"title,omitempty"`
	Default        bool   `json:"default,omitempty"`
	Forced         bool   `json:"forced,omitempty"`
	AttachedPic    bool   `json:"attachedPic,omitempty"` // cover art stored as a video stream
}

// Chapter is one chapter mark.
type Chapter struct {
	Start time.Duration `json:"start"`
	End   time.Duration `json:"end"`
	Title string        `json:"title,omitempty"`
}

// ffprobe JSON output; numbers that ffprobe prints as strings stay strings here.
type rawProbe struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index          int    `json:"index"`
		CodecType      string `json:"codec_type"`
		CodecName      string `json:"codec_name"`
		Profile        string `json:"profile"`
		Level          int    `json:"level"`
		Width          int    `json:"width"`
		Height         int    `json:"height"`
		PixFmt         string `json:"pix_fmt"`
		ColorTransfer  string `json:"color_transfer"`
		ColorPrimaries string `json:"color_primaries"`
		Channels       int    `json:"channels"`
		ChannelLayout  string `json:"channel_layout"`
		Tags           struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
		Disposition struct {
			Default     int `json:"default"`
			Forced      int `json:"forced"`
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Chapters []struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Tags      struct {
			Title string `json:"title"`
		} `json:"tags"`
	} `json:"chapters"`
}

// Probe runs `ffprobe -v error -print_format json -show_format -show_streams -show_chapters` on
// the absolute path (resolved through package paths by the caller) and parses the result.
func (r Runner) Probe(ctx context.Context, path string) (Probe, error) {
	out, err := r.ffprobe(ctx, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "-show_chapters", path)
	if err != nil {
		return Probe{}, err
	}
	return ParseProbe(out)
}

// ParseProbe parses the JSON output of ffprobe.
func ParseProbe(data []byte) (Probe, error) {
	var raw rawProbe
	if err := json.Unmarshal(data, &raw); err != nil {
		return Probe{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	p := Probe{
		Format: Format{
			Name:     raw.Format.FormatName,
			Duration: seconds(raw.Format.Duration),
			Size:     integer(raw.Format.Size),
			BitRate:  integer(raw.Format.BitRate),
		},
		Streams:  make([]Stream, 0, len(raw.Streams)),
		Chapters: make([]Chapter, 0, len(raw.Chapters)),
	}
	for _, s := range raw.Streams {
		p.Streams = append(p.Streams, Stream{
			Index:          s.Index,
			Type:           s.CodecType,
			Codec:          s.CodecName,
			Profile:        s.Profile,
			Level:          s.Level,
			Width:          s.Width,
			Height:         s.Height,
			PixFmt:         s.PixFmt,
			ColorTransfer:  s.ColorTransfer,
			ColorPrimaries: s.ColorPrimaries,
			Channels:       s.Channels,
			ChannelLayout:  s.ChannelLayout,
			Language:       s.Tags.Language,
			Title:          s.Tags.Title,
			Default:        s.Disposition.Default == 1,
			Forced:         s.Disposition.Forced == 1,
			AttachedPic:    s.Disposition.AttachedPic == 1,
		})
	}
	for _, c := range raw.Chapters {
		p.Chapters = append(p.Chapters, Chapter{Start: seconds(c.StartTime), End: seconds(c.EndTime), Title: c.Tags.Title})
	}
	if p.Format.Name == "" && len(p.Streams) == 0 {
		return Probe{}, errors.New("ffprobe found no format and no streams")
	}
	return p, nil
}

// Keyframes returns the presentation times of the key frames of the first video stream, sorted.
// It only demuxes (`-show_entries packet=pts_time,flags`), so it reads the file once without
// decoding; packets whose flags contain K are key frames. A remux plan needs this index
// (Source.Keyframes): NewPlan returns ErrNoKeyframes without it.
func (r Runner) Keyframes(ctx context.Context, path string) ([]time.Duration, error) {
	out, err := r.ffprobe(ctx, "-v", "error", "-select_streams", "v:0", "-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path)
	if err != nil {
		return nil, err
	}
	return ParseKeyframes(out), nil
}

// ParseKeyframes parses `pts_time,flags` CSV lines and keeps the key frames, sorted and without
// duplicates. Lines without a time (N/A) are skipped.
func ParseKeyframes(data []byte) []time.Duration {
	var res []time.Duration
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		pts, flags, ok := strings.Cut(strings.TrimSpace(sc.Text()), ",")
		if !ok || !strings.Contains(flags, "K") {
			continue
		}
		f, err := strconv.ParseFloat(pts, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		res = append(res, time.Duration(math.Round(f*float64(time.Second))))
	}
	slices.Sort(res)
	return slices.Compact(res)
}

func (r Runner) ffprobe(ctx context.Context, args ...string) ([]byte, error) {
	if r.FFprobePath == "" {
		return nil, errors.New("ffprobe path is not configured")
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFprobePath, args...) //nolint:gosec // G204: configured binary, fixed argument list, path resolved by package paths
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func seconds(s string) time.Duration {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0
	}
	return time.Duration(math.Round(f * float64(time.Second)))
}

func integer(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
