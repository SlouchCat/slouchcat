package transcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Audiobooks (M6, design 3.2): cover art embedded in a file, and multi-part books that play as
// one timeline through ffmpeg's concat demuxer (or the concat filter when the parts differ).

// ErrNoCover is returned by ExtractCover for a file without an attached picture.
var ErrNoCover = errors.New("no embedded cover")

// concatList is the file name of the concat list a multi-part source reads (Source.Parts),
// written into the directory of each ffmpeg run.
const concatList = "parts.txt"

// ExtractCover returns the first attached picture of the file at the absolute path (resolved
// through package paths by the caller): JPEG or PNG bytes as stored, any other picture format
// converted to JPEG. A file without one returns ErrNoCover.
func (r Runner) ExtractCover(ctx context.Context, path string) ([]byte, error) {
	p, err := r.Probe(ctx, path)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(p.Streams, func(s Stream) bool { return s.Type == "video" && s.AttachedPic })
	if i < 0 {
		return nil, ErrNoCover
	}
	pic := p.Streams[i]
	codec := []string{"-c:v", "copy"}
	if pic.Codec != "mjpeg" && pic.Codec != "png" {
		codec = []string{"-c:v", "mjpeg", "-q:v", "2", "-pix_fmt", "yuvj420p"}
	}
	args := slices.Concat([]string{"-nostdin", "-hide_banner", "-loglevel", "error", "-i", path,
		"-map", "0:" + strconv.Itoa(pic.Index), "-frames:v", "1"}, codec, []string{"-f", "image2pipe", "pipe:1"})
	out, err := r.runImageFFmpeg(ctx, args...)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrNoCover
	}
	return out, nil
}

// ConcatProbe merges the probes of the parts of an audiobook, in order, into one book-wide probe:
// the format of the first part with the summed duration and size (and the average bit rate), the
// audio streams of the first part (cover art and other streams dropped, so the plan is audio-only),
// and the chapters of every part shifted by the duration of the parts before it (PartChapters). A
// part with fewer than two chapter marks contributes one chapter over the whole part, titled by its
// title tag.
//
// ffmpeg's concat demuxer maps the packets of every part by stream index and copies their format,
// so it needs parts with the same streams (concatCompatible). When they differ (cover art in one
// part only, another codec, sample rate or channel count), the merged probe is Mixed: the HLS
// pipeline then decodes each part and joins them with the concat filter, and the audio cannot be
// copied.
func ConcatProbe(parts []Probe) Probe {
	out := Probe{Streams: []Stream{}, Chapters: []Chapter{}}
	if len(parts) == 0 {
		return out
	}
	out.Format = parts[0].Format
	out.Format.Duration, out.Format.Size = 0, 0
	for _, s := range parts[0].Streams {
		if s.Type == "audio" {
			out.Streams = append(out.Streams, s)
		}
	}
	var offset time.Duration
	for _, p := range parts {
		d := p.Format.Duration
		if marks := PartChapters(p, d); marks != nil {
			for _, c := range marks {
				out.Chapters = append(out.Chapters, Chapter{Start: offset + c.Start, End: offset + c.End, Title: c.Title})
			}
		} else {
			out.Chapters = append(out.Chapters, Chapter{Start: offset, End: offset + d, Title: p.Format.Tags["title"]})
		}
		offset += d
		out.Format.Size += p.Format.Size
	}
	out.Format.Duration = offset
	if offset > 0 && out.Format.Size > 0 {
		out.Format.BitRate = int64(float64(out.Format.Size*8) / offset.Seconds())
	}
	if !concatCompatible(parts) {
		out.partDurations = make([]time.Duration, 0, len(parts))
		for _, p := range parts {
			out.partDurations = append(out.partDurations, p.Format.Duration)
		}
	}
	return out
}

// PartChapters returns the chapter marks of one part of a book on the part's own timeline, each
// clamped to the part's duration d (an end never before its start), or nil when the part has fewer
// than two marks (it counts as one chapter then). ConcatProbe and the chapter list of package
// library share it.
func PartChapters(p Probe, d time.Duration) []Chapter {
	if len(p.Chapters) < 2 {
		return nil
	}
	out := make([]Chapter, 0, len(p.Chapters))
	for _, c := range p.Chapters {
		start := min(max(c.Start, 0), d)
		out = append(out, Chapter{Start: start, End: min(max(c.End, start), d), Title: c.Title})
	}
	return out
}

// Mixed reports whether p is the merged probe of book parts that differ in their streams
// (ConcatProbe): they are joined by the concat filter and their audio is always encoded.
func (p Probe) Mixed() bool { return len(p.partDurations) > 0 }

// concatCompatible reports whether the concat demuxer can read the parts as one stream: the same
// streams in the same order, with the same type, codec and cover flag, and audio of the same sample
// rate and channel count.
func concatCompatible(parts []Probe) bool {
	for _, p := range parts[1:] {
		if !slices.EqualFunc(p.Streams, parts[0].Streams, func(a, b Stream) bool {
			return a.Index == b.Index && a.Type == b.Type && a.Codec == b.Codec && a.AttachedPic == b.AttachedPic &&
				(a.Type != "audio" || (a.SampleRate == b.SampleRate && a.Channels == b.Channels))
		}) {
			return false
		}
	}
	return true
}

// concatFilterInputs returns the inputs of a Mixed book and the filter graph that joins their audio
// with the concat filter into audioOut. A run starting at book time b reads from the part that
// holds b: an input seek to b inside it, and -itsoffset puts the part on the book's timeline (the
// filter appends the later parts after it); the output -ss trims to b exactly. a is the chosen audio
// stream of the first part; every part contributes its audio stream at the same position.
func concatFilterInputs(src Source, a Stream, b time.Duration) (args []string, graph string) {
	durs := src.Probe.partDurations
	first, offset := 0, time.Duration(0)
	for first < len(src.Parts)-1 && first < len(durs) && offset+durs[first] <= b {
		offset += durs[first]
		first++
	}
	n := max(slices.IndexFunc(src.Probe.Streams, func(s Stream) bool { return s.Index == a.Index }), 0)
	var g strings.Builder
	for i := first; i < len(src.Parts); i++ {
		if i == first && b > offset {
			args = append(args, "-ss", seconds6(b-offset))
		}
		if i == first && offset > 0 {
			args = append(args, "-itsoffset", seconds6(offset))
		}
		args = append(args, "-i", src.Parts[i])
		fmt.Fprintf(&g, "[%d:a:%d]", i-first, n)
	}
	fmt.Fprintf(&g, "concat=n=%d:v=0:a=1%s", len(src.Parts)-first, audioOut)
	return args, g.String()
}

// writeConcatList writes the concat list of a multi-part source into dir; a source with a single
// file needs none.
func writeConcatList(src Source, dir string) error {
	if len(src.Parts) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("ffconcat version 1.0\n")
	for _, p := range src.Parts {
		if !filepath.IsAbs(p) || strings.ContainsAny(p, "\n\r") {
			return fmt.Errorf("concat part %q is not an absolute single-line path", p)
		}
		// Single quotes are closed, escaped and reopened (ffmpeg's quoting rules).
		b.WriteString("file '" + strings.ReplaceAll(p, "'", `'\''`) + "'\n")
	}
	if err := os.WriteFile(filepath.Join(dir, concatList), []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}
	return nil
}
