package transcode

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Chapter images (concept 13 stage 3, 11.6, V-19): one frame per chapter of a film or an episode,
// grabbed with an input seek, so ffmpeg reads only the few MB from the keyframe before the time
// point on, scaled to ChapterImageWidth and stored as WebP. HDR and Dolby Vision sources are
// tonemapped on the CPU (cpuTonemap; one 480-px frame is cheap).

// ChapterImageWidth is the width of a chapter image in pixels.
const ChapterImageWidth = 480

// Time points of a chapter image (11.6, 11.14): the start plus a tenth of the chapter, at most
// 30 s after the start; when that frame is black, a second try 10 s later within the chapter.
const (
	chapterFrameMaxOffset = 30 * time.Second
	chapterFrameRetry     = 10 * time.Second
)

// Black test of a grabbed frame: the frame is also scaled to a lumaWidth x lumaHeight grey image
// (area average) and written raw to standard output in the same ffmpeg run. It counts as black
// when at least blackAmount percent of its pixels have a luma of at most blackThreshold: the
// defaults of ffmpeg's blackframe filter (amount 98, threshold 32), computed here from the pixels
// instead of parsing log lines.
const (
	lumaWidth      = 64
	lumaHeight     = 36
	blackThreshold = 32
	blackAmount    = 98
)

// ChapterFrameTimes returns the time points of the image of chapter c: the first try and, when it
// still lies inside the chapter, the second one 10 s later (for a black first frame).
func ChapterFrameTimes(c model.Chapter) []time.Duration {
	start, end := time.Duration(c.StartMs)*time.Millisecond, time.Duration(c.EndMs)*time.Millisecond
	first := start + min(max(end-start, 0)/10, chapterFrameMaxOffset)
	out := []time.Duration{first}
	if second := first + chapterFrameRetry; second < end {
		out = append(out, second)
	}
	return out
}

// GrabFrameArgs builds the ffmpeg argument list of GrabFrame: an input seek to at, the video
// stream v of the file (its index; cover art never), scaled to width with the height of the
// display aspect ratio (dar, so an anamorphic source with a sample aspect ratio other than 1 comes
// out undistorted; even, square pixels), tonemapped to BT.709 on the CPU when v is HDR, one frame
// as WebP to outPath and the same frame as a raw grey lumaWidth x lumaHeight image to standard
// output.
func GrabFrameArgs(inPath string, at time.Duration, width int, v Stream, outPath string) ([]string, error) {
	if width <= 0 || width%2 != 0 || inPath == "" || outPath == "" || at < 0 {
		return nil, fmt.Errorf("invalid frame request: %q at %v, width %d", inPath, at, width)
	}
	w := strconv.Itoa(width)
	h := "trunc(" + w + "/dar/2)*2" // the display aspect ratio, even
	var body []string
	if v.IsHDR() {
		if v.DolbyVisionProfile > 0 && v.ColorTransfer == "" {
			body = append(body, dvAsHDR10) // profile 5 carries no transfer tag (pipeline.go)
		}
		body = append(body, "zscale=w="+w+":h="+h, "setsar=1")
		body = append(body, cpuTonemap()...)
	} else {
		body = []string{"scale=w=" + w + ":h=" + h + ":flags=lanczos", "setsar=1", "format=yuv420p"}
	}
	graph := "[0:" + strconv.Itoa(v.Index) + "]" + strings.Join(body, ",") + ",split=2[img][probe];" +
		"[probe]scale=w=" + strconv.Itoa(lumaWidth) + ":h=" + strconv.Itoa(lumaHeight) + ":flags=area,format=gray[luma]"
	enc, err := imageEncoderArgs(ImageWebP)
	if err != nil {
		return nil, err
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-ss", strconv.FormatFloat(at.Seconds(), 'f', 3, 64), "-i", inPath, "-filter_complex", graph,
		"-map", "[img]", "-frames:v", "1"}
	args = append(args, enc...)
	args = append(args, outPath, "-map", "[luma]", "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "gray", "pipe:1")
	return args, nil
}

// ErrNoFrame is the result of a time point past the last frame of the file.
var ErrNoFrame = errors.New("ffmpeg wrote no frame")

// FrameFailed reports whether err of GrabFrame says that ffmpeg ran on the input and gave no
// usable frame: it wrote none (ErrNoFrame) or exited with an error (a broken stream). An ffmpeg
// that did not run (not configured, not found), a cancelled or timed-out context and an ffmpeg
// that could not read its input («Input/output error», «Permission denied»: a share offline, a
// right missing) are no frame failure: the frame may well exist.
func FrameFailed(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if msg := err.Error(); strings.Contains(msg, "Input/output error") || strings.Contains(msg, "Permission denied") {
		return false
	}
	var exit *exec.ExitError
	return errors.Is(err, ErrNoFrame) || errors.As(err, &exit)
}

// GrabFrame writes the frame of the video stream v of the file at inPath at the time point at as
// WebP of the given width to outPath (an existing file is overwritten) and reports whether the
// frame is black (see blackAmount). Callers pass absolute paths resolved through package paths and
// write outPath under a temporary name.
func (r Runner) GrabFrame(ctx context.Context, inPath string, at time.Duration, width int, v Stream, outPath string) (black bool, err error) {
	args, err := GrabFrameArgs(inPath, at, width, v, outPath)
	if err != nil {
		return false, err
	}
	luma, err := r.runImageFFmpeg(ctx, args...)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return false, fmt.Errorf("%w: %w", cerr, err)
		}
		return false, err
	}
	if len(luma) != lumaWidth*lumaHeight {
		return false, fmt.Errorf("%w at %v (%d bytes of luma)", ErrNoFrame, at, len(luma))
	}
	return blackLuma(luma), nil
}

// blackLuma reports whether at least blackAmount percent of the grey pixels are at most
// blackThreshold.
func blackLuma(luma []byte) bool {
	dark := 0
	for _, y := range luma {
		if y <= blackThreshold {
			dark++
		}
	}
	return dark*100 >= len(luma)*blackAmount
}
