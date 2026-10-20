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
	// Tags are the container tags (ffprobe format.tags) with lower-case keys: title, album,
	// artist, album_artist, composer, show, episode_id, track, date, genre, description, comment
	// and the like; nil when the file has none. Audiobooks take their metadata from them (M6).
	Tags map[string]string `json:"tags,omitempty"`
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
	// Dolby Vision (M4): the profile of the DOVI configuration record (5, 7, 8; 0 = none) and the
	// base-layer compatibility id (1 = HDR10 base layer, profile 8.1; 2 = SDR, 8.2/9.2; 4 = HLG,
	// 8.4). Profile 8 plays its base layer like HDR10 (or SDR, HLG); profiles 5 and 7 are
	// transcoded.
	DolbyVisionProfile  int `json:"dvProfile,omitempty"`
	DolbyVisionBLCompat int `json:"dvBLCompat,omitempty"`
	// HDR10Plus is set when the first frame carries SMPTE ST 2094-40 dynamic metadata (M4).
	HDR10Plus bool `json:"hdr10plus,omitempty"`
}

// HDR formats of HDRFormat.
const (
	HDRNone        = ""
	HDR10          = "hdr10"
	HDR10Plus      = "hdr10plus"
	HDRDolbyVision = "dolby_vision"
	HDRHLG         = "hlg"
)

// dvCompatSDR is the Dolby Vision base-layer compatibility id of an SDR (BT.709) base layer
// (profiles 8.2 and 9.2); 0 is none (profile 5), 1 HDR10, 4 HLG, 6 Blu-ray HDR10 (profile 7).
const dvCompatSDR = 2

// IsHDR reports whether the stream is HDR: PQ (HDR10, HDR10+, a Dolby Vision HDR10 base layer) or
// HLG transfer, or Dolby Vision without such a tag unless its base layer is SDR (compatibility id
// 2). Profiles 5 and 7 count as HDR whatever they signal. BT.2020 primaries alone do not count.
func (s Stream) IsHDR() bool {
	switch s.ColorTransfer {
	case "smpte2084", "arib-std-b67":
		return true
	}
	if s.DolbyVisionProfile == 0 {
		return false
	}
	return s.DolbyVisionProfile == 5 || s.DolbyVisionProfile == 7 || s.DolbyVisionBLCompat != dvCompatSDR
}

// HDRFormat classifies the stream: Dolby Vision when a DOVI record exists and the stream is HDR
// (a Dolby Vision stream with an SDR base layer is SDR), HLG by transfer, HDR10+ by dynamic
// metadata, HDR10 for any other PQ stream, empty for SDR.
func (s Stream) HDRFormat() string {
	switch {
	case s.DolbyVisionProfile > 0 && s.IsHDR():
		return HDRDolbyVision
	case s.ColorTransfer == "arib-std-b67":
		return HDRHLG
	case s.ColorTransfer != "smpte2084":
		return HDRNone
	case s.HDR10Plus:
		return HDR10Plus
	default:
		return HDR10
	}
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
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		Size       string            `json:"size"`
		BitRate    string            `json:"bit_rate"`
		Tags       map[string]string `json:"tags"`
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
		SideData []struct {
			Type       string `json:"side_data_type"`
			DVProfile  int    `json:"dv_profile"`
			DVBLCompat int    `json:"dv_bl_signal_compatibility_id"`
		} `json:"side_data_list"`
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
//
// A failure that is a property of the file (ffprobe ran and exited with an error, or its output
// does not parse) is a *FileError; any other failure (ffprobe missing or not configured, the
// process could not start, the context ended) is returned as is, so the caller can retry it.
//
// HDR10+ metadata is per frame (SEI), invisible in the stream list: for a PQ main video stream a
// second ffprobe call reads the side data of its first frame (ParseFrameSideData) and sets
// Stream.HDR10Plus. A file error of that call leaves HDR10Plus false.
func (r Runner) Probe(ctx context.Context, path string) (Probe, error) {
	out, err := r.ffprobe(ctx, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "-show_chapters", path)
	if err != nil {
		return Probe{}, err
	}
	p, err := ParseProbe(out)
	if err != nil {
		return Probe{}, &FileError{err: err}
	}
	i := slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Type == "video" && !s.AttachedPic })
	if i < 0 || p.Streams[i].ColorTransfer != "smpte2084" {
		return p, nil
	}
	out, err = r.ffprobe(ctx, "-v", "error", "-print_format", "json", "-select_streams", strconv.Itoa(p.Streams[i].Index),
		"-read_intervals", "%+#1", "-show_entries", "frame=side_data_list:frame_side_data=side_data_type", path)
	var fe *FileError
	switch {
	case errors.As(err, &fe):
		return p, nil
	case err != nil:
		return Probe{}, err
	}
	hdr10Plus, perr := ParseFrameSideData(out) // unreadable side data only means no HDR10+
	p.Streams[i].HDR10Plus = perr == nil && hdr10Plus
	return p, nil
}

// hdr10PlusSideData is the prefix ffprobe prints for SMPTE ST 2094-40 dynamic metadata, for
// example "HDR Dynamic Metadata SMPTE2094-40 (HDR10+)".
const hdr10PlusSideData = "HDR Dynamic Metadata SMPTE2094-40"

// ParseFrameSideData parses the JSON of `ffprobe -show_entries
// frame=side_data_list:frame_side_data=side_data_type` and reports whether a frame carries HDR10+
// (SMPTE ST 2094-40) dynamic metadata.
func ParseFrameSideData(data []byte) (hdr10Plus bool, err error) {
	var raw struct {
		Frames []struct {
			SideData []struct {
				Type string `json:"side_data_type"`
			} `json:"side_data_list"`
		} `json:"frames"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parse ffprobe frame side data: %w", err)
	}
	for _, f := range raw.Frames {
		for _, sd := range f.SideData {
			if strings.HasPrefix(sd.Type, hdr10PlusSideData) {
				return true, nil
			}
		}
	}
	return false, nil
}

// FileError is a probe failure caused by the file itself: ffprobe exited with an error (not a
// media file, damaged) or printed output that does not parse.
type FileError struct{ err error }

// systemFailures are the operating system messages ffprobe prints when it cannot open or read its
// input (a network share that dropped, a file gone for a moment); such an exit says nothing about
// the file, so it is not a FileError.
var systemFailures = []string{
	"Input/output error", "No such file or directory", "Permission denied", "Resource temporarily unavailable",
	"Stale file handle", "Host is down", "Connection timed out",
}

func (e *FileError) Error() string { return e.err.Error() }

func (e *FileError) Unwrap() error { return e.err }

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
			Tags:     lowerKeys(raw.Format.Tags),
		},
		Streams:  make([]Stream, 0, len(raw.Streams)),
		Chapters: make([]Chapter, 0, len(raw.Chapters)),
	}
	for _, s := range raw.Streams {
		dvProfile, dvCompat := 0, 0
		for _, sd := range s.SideData {
			if strings.HasPrefix(sd.Type, "DOVI configuration") {
				dvProfile, dvCompat = sd.DVProfile, sd.DVBLCompat
			}
		}
		p.Streams = append(p.Streams, Stream{
			DolbyVisionProfile:  dvProfile,
			DolbyVisionBLCompat: dvCompat,
			Index:               s.Index,
			Type:                s.CodecType,
			Codec:               s.CodecName,
			Profile:             s.Profile,
			Level:               s.Level,
			Width:               s.Width,
			Height:              s.Height,
			PixFmt:              s.PixFmt,
			ColorTransfer:       s.ColorTransfer,
			ColorPrimaries:      s.ColorPrimaries,
			Channels:            s.Channels,
			ChannelLayout:       s.ChannelLayout,
			Language:            s.Tags.Language,
			Title:               s.Tags.Title,
			Default:             s.Disposition.Default == 1,
			Forced:              s.Disposition.Forced == 1,
			AttachedPic:         s.Disposition.AttachedPic == 1,
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
		msg := strings.TrimSpace(stderr.String())
		wrapped := fmt.Errorf("ffprobe: %w: %s", err, msg)
		var exit *exec.ExitError
		if errors.As(err, &exit) && ctx.Err() == nil && !slices.ContainsFunc(systemFailures, func(s string) bool { return strings.Contains(msg, s) }) {
			return nil, &FileError{err: wrapped}
		}
		return nil, wrapped
	}
	return out, nil
}

// lowerKeys returns the tags with lower-case keys and trimmed values, without empty values; nil
// for none. Of two keys that differ in case only, the first in key order wins.
func lowerKeys(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v := strings.TrimSpace(tags[k])
		lk := strings.ToLower(k)
		if _, dup := out[lk]; v == "" || dup {
			continue
		}
		out[lk] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
