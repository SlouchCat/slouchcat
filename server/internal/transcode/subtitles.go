package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// subtitleTimeout bounds one WebVTT conversion: an embedded track needs a full read of the
// container. A variable for the tests.
var subtitleTimeout = 60 * time.Second

// textSubtitleCodecs are the subtitle codecs ffmpeg converts to WebVTT and libass renders.
var textSubtitleCodecs = []string{"subrip", "ass", "ssa", "mov_text", "webvtt", "text"}

// bitmapSubtitleCodecs are the image subtitles that can only be burned in (overlay).
var bitmapSubtitleCodecs = []string{"hdmv_pgs_subtitle", "dvd_subtitle"}

// TextSubtitleCodec reports whether codec (ffprobe codec_name) is a text subtitle: deliverable as
// WebVTT and burnable through libass. Image subtitles (PGS, DVD) are not.
func TextSubtitleCodec(codec string) bool { return slices.Contains(textSubtitleCodecs, codec) }

// BitmapSubtitleCodec reports whether codec is an embedded image subtitle that a transcode burns
// in as an overlay (PGS, DVD). Other image formats (VobSub sidecars, DVB) cannot be burned in.
func BitmapSubtitleCodec(codec string) bool { return slices.Contains(bitmapSubtitleCodecs, codec) }

type burnKind int

const (
	burnNone burnKind = iota
	burnText
	burnBitmap
)

// burnIn is the subtitle a transcode burns into the video.
type burnIn struct {
	kind   burnKind
	path   string // text: the file libass reads (sidecar, or the source itself for an embedded track)
	si     int    // text: index among the subtitle streams of path, -1 for a sidecar
	stream int    // bitmap: stream index of the subtitle in the source
}

// burnInOf resolves the burn-in of t: a sidecar (BurnInPath, text only), an embedded text track
// (path is filled in from the source by ffmpegArgs) or an embedded bitmap track. An index that
// does not name a subtitle stream burns nothing; a subtitle codec that is neither text nor PGS/DVD
// is an error.
func burnInOf(p Probe, t Target) (burnIn, error) {
	if t.BurnInPath != "" {
		return burnIn{kind: burnText, path: t.BurnInPath, si: -1}, nil
	}
	s, ok := streamByIndex(p, t.SubtitleBurnIn)
	if !ok || s.Type != "subtitle" {
		return burnIn{}, nil
	}
	switch {
	case TextSubtitleCodec(s.Codec):
		si := 0
		for _, o := range p.Streams {
			if o.Type == "subtitle" && o.Index < s.Index {
				si++
			}
		}
		return burnIn{kind: burnText, si: si}, nil
	case slices.Contains(bitmapSubtitleCodecs, s.Codec):
		return burnIn{kind: burnBitmap, stream: s.Index}, nil
	default:
		return burnIn{}, fmt.Errorf("subtitle codec %q cannot be burned in", s.Codec)
	}
}

// filter returns the libass subtitles filter of a text burn-in.
func (b burnIn) filter() string {
	f := "subtitles=filename=" + filterValue(b.path)
	if b.si >= 0 {
		f += ":si=" + strconv.Itoa(b.si)
	}
	return f
}

// filterValue escapes s as an option value inside a filter graph string. ffmpeg unescapes twice:
// the graph parser treats \ ' [ ] , ; as special, then the option parser \ ' : (and = for keys).
// Escaping a character that is not special is harmless, so both sets are escaped per level.
func filterValue(s string) string {
	return escapeChars(escapeChars(s, `\':=`), `\'[],;`)
}

func escapeChars(s, special string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Subtitles converts the embedded text subtitle stream streamIndex of path to WebVTT (`ffmpeg -i
// path -map 0:<idx> -f webvtt -`). Styling of ASS/SSA is lost. Like Probe, a failure caused by the
// file (ffmpeg exits with an error: an image subtitle, a stream that is no subtitle, a damaged
// file) is a *FileError; a missing ffmpeg, a cancelled context or the 60-s timeout are not.
func (r Runner) Subtitles(ctx context.Context, path string, streamIndex int) ([]byte, error) {
	if streamIndex < 0 {
		return nil, &FileError{err: fmt.Errorf("invalid subtitle stream index %d", streamIndex)}
	}
	return r.webVTT(ctx, path, "0:"+strconv.Itoa(streamIndex), false)
}

// SubtitlesFile converts a sidecar subtitle file (SRT, ASS, SSA, VTT) to WebVTT. A file that is
// not UTF-8 is read again as Windows-1252, the usual encoding of older European SRT files.
// Errors as for Subtitles.
func (r Runner) SubtitlesFile(ctx context.Context, path string) ([]byte, error) {
	return r.webVTT(ctx, path, "0:s:0", true)
}

func (r Runner) webVTT(ctx context.Context, path, mapSpec string, sidecar bool) ([]byte, error) {
	out, stderr, err := r.runWebVTT(ctx, path, mapSpec, nil)
	if sidecar && ctx.Err() == nil && strings.Contains(stderr, "Invalid UTF-8") {
		out, _, err = r.runWebVTT(ctx, path, mapSpec, []string{"-sub_charenc", "CP1252"})
	}
	return out, err
}

func (r Runner) runWebVTT(ctx context.Context, path, mapSpec string, inOpts []string) ([]byte, string, error) {
	if r.FFmpegPath == "" {
		return nil, "", errors.New("ffmpeg path is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, subtitleTimeout)
	defer cancel()
	args := slices.Concat([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, inOpts,
		[]string{"-i", path, "-map", mapSpec, "-c:s", "webvtt", "-f", "webvtt", "-"})
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFmpegPath, args...) //nolint:gosec // G204: configured binary, fixed argument list, path resolved by package paths
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	msg := strings.TrimSpace(stderr.String())
	if err != nil {
		wrapped := fmt.Errorf("ffmpeg webvtt %s: %w: %s", mapSpec, err, tail([]byte(msg), 512))
		var exit *exec.ExitError
		if errors.As(err, &exit) && ctx.Err() == nil && !slices.ContainsFunc(systemFailures, func(s string) bool { return strings.Contains(msg, s) }) {
			return nil, msg, &FileError{err: wrapped}
		}
		return nil, msg, wrapped
	}
	return out, msg, nil
}
