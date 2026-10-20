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
// one timeline through ffmpeg's concat demuxer.

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
// and the chapters of every part shifted by the duration of the parts before it. A part with fewer
// than two chapter marks contributes one chapter over the whole part, titled by its title tag.
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
		if len(p.Chapters) >= 2 {
			for _, c := range p.Chapters {
				out.Chapters = append(out.Chapters, Chapter{Start: offset + c.Start, End: offset + min(c.End, d), Title: c.Title})
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
	return out
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
