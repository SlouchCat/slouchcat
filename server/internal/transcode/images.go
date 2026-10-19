package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"strconv"
	"strings"
)

// Image pipeline (plan 3.1, M1b): ffmpeg scales the stored TMDB original into the delivered
// sizes and formats and computes the palette for the accent colour. Callers pass absolute paths
// resolved through package paths.

// ImageFormat is an output format of the image pipeline.
type ImageFormat string

// Delivered image formats: WebP (libwebp) for browsers that accept it, JPEG as the fallback.
const (
	ImageWebP ImageFormat = "webp"
	ImageJPEG ImageFormat = "jpeg"
)

// ImageOutput is one scaled copy of an image: the width in pixels (the height follows the aspect
// ratio, rounded to an even number) and the format, written to the absolute Path.
type ImageOutput struct {
	Path   string
	Width  int
	Format ImageFormat
}

// imageEncoderArgs returns the encoder and muxer options of a format. The muxer is named explicitly,
// so the output path needs no extension (callers write to a temporary name and rename).
func imageEncoderArgs(f ImageFormat) ([]string, error) {
	switch f {
	case ImageWebP:
		return []string{"-c:v", "libwebp", "-quality", "82", "-compression_level", "4", "-f", "webp"}, nil
	case ImageJPEG:
		return []string{"-c:v", "mjpeg", "-q:v", "3", "-pix_fmt", "yuvj420p", "-f", "mjpeg"}, nil
	default:
		return nil, fmt.Errorf("unknown image format %q", f)
	}
}

// ScaleImageArgs builds the ffmpeg argument list of ScaleImage. One run decodes the input once,
// scales it once per distinct width (lanczos) and encodes every output from that frame.
func ScaleImageArgs(inPath string, outs []ImageOutput) ([]string, error) {
	if len(outs) == 0 {
		return nil, errors.New("no image outputs")
	}
	var widths []int
	perWidth := map[int][]int{} // width → indexes into outs
	for i, o := range outs {
		if o.Width <= 0 || o.Path == "" {
			return nil, fmt.Errorf("invalid image output %+v", o)
		}
		if _, ok := perWidth[o.Width]; !ok {
			widths = append(widths, o.Width)
		}
		perWidth[o.Width] = append(perWidth[o.Width], i)
	}
	var graph strings.Builder
	graph.WriteString("[0:v]split=" + strconv.Itoa(len(widths)))
	for wi := range widths {
		graph.WriteString("[s" + strconv.Itoa(wi) + "]")
	}
	for wi, w := range widths {
		idx := perWidth[w]
		graph.WriteString(";[s" + strconv.Itoa(wi) + "]scale=" + strconv.Itoa(w) + ":-2:flags=lanczos,split=" + strconv.Itoa(len(idx)))
		for _, i := range idx {
			graph.WriteString("[o" + strconv.Itoa(i) + "]")
		}
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", inPath, "-filter_complex", graph.String()}
	for i, o := range outs {
		enc, err := imageEncoderArgs(o.Format)
		if err != nil {
			return nil, err
		}
		args = append(args, "-map", "[o"+strconv.Itoa(i)+"]", "-frames:v", "1")
		args = append(args, enc...)
		args = append(args, o.Path)
	}
	return args, nil
}

// ScaleImage writes every output of the image at inPath in a single ffmpeg run (the original is
// decoded once). Existing files at the output paths are overwritten.
func (r Runner) ScaleImage(ctx context.Context, inPath string, outs []ImageOutput) error {
	args, err := ScaleImageArgs(inPath, outs)
	if err != nil {
		return err
	}
	_, err = r.runImageFFmpeg(ctx, args...)
	return err
}

// Palette returns up to maxColors representative colours of the image at inPath, computed by
// ffmpeg's palettegen (median cut). palettegen writes a 16x16 palette image whose first pixels are
// the colours; the rest repeats the last colour, so duplicates are dropped.
func (r Runner) Palette(ctx context.Context, inPath string, maxColors int) ([]color.NRGBA, error) {
	if maxColors < 2 || maxColors > 256 {
		return nil, fmt.Errorf("palette size %d out of range 2..256", maxColors)
	}
	out, err := r.runImageFFmpeg(ctx, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", inPath,
		"-vf", "palettegen=max_colors="+strconv.Itoa(maxColors)+":reserve_transparent=0",
		"-frames:v", "1", "-c:v", "png", "-f", "image2pipe", "pipe:1")
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("decode palette: %w", err)
	}
	return paletteColors(img, maxColors), nil
}

// paletteColors reads the first n pixels of a palette image in row order, without duplicates.
func paletteColors(img image.Image, n int) []color.NRGBA {
	b := img.Bounds()
	seen := map[color.NRGBA]bool{}
	var res []color.NRGBA
	for i := 0; i < n && i < b.Dx()*b.Dy(); i++ {
		c := color.NRGBAModel.Convert(img.At(b.Min.X+i%b.Dx(), b.Min.Y+i/b.Dx())).(color.NRGBA)
		if !seen[c] {
			seen[c] = true
			res = append(res, c)
		}
	}
	return res
}

// Lightness range of the preferred accent colour: darker colours vanish on the dark UI, lighter
// ones wash out the tint.
const (
	accentMinLightness = 0.25
	accentMaxLightness = 0.65
)

// AccentColor picks the accent colour of a title from its poster palette (plan 3.1): the colour
// with the highest HSL saturation among those with a lightness between 0.25 and 0.65; when none
// lies in that range, the most saturated colour overall. Fully transparent colours are ignored;
// ties keep the earlier colour. The result is lower-case hex (#rrggbb); ok is false for an empty
// palette.
func AccentColor(colors []color.NRGBA) (hex string, ok bool) {
	best, bestInRange := -1, -1
	var bestSat, bestInRangeSat float64
	for i, c := range colors {
		if c.A == 0 {
			continue
		}
		s, l := saturationLightness(c)
		if best < 0 || s > bestSat {
			best, bestSat = i, s
		}
		if l >= accentMinLightness && l <= accentMaxLightness && (bestInRange < 0 || s > bestInRangeSat) {
			bestInRange, bestInRangeSat = i, s
		}
	}
	if bestInRange >= 0 {
		best = bestInRange
	}
	if best < 0 {
		return "", false
	}
	c := colors[best]
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B), true
}

// saturationLightness returns the HSL saturation and lightness of c, both in 0..1.
func saturationLightness(c color.NRGBA) (s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := max(r, g, b), min(r, g, b)
	l = (hi + lo) / 2
	if hi == lo {
		return 0, l
	}
	d := hi - lo
	if l > 0.5 {
		return d / (2 - hi - lo), l
	}
	return d / (hi + lo), l
}

// runImageFFmpeg runs the configured ffmpeg with an argument list and returns its standard output.
func (r Runner) runImageFFmpeg(ctx context.Context, args ...string) ([]byte, error) {
	if r.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFmpegPath, args...) //nolint:gosec // G204: configured binary, fixed argument list, paths resolved by package paths
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, imageErrTail(strings.TrimSpace(stderr.String()), 2048))
	}
	return stdout.Bytes(), nil
}

// imageErrTail keeps the tail of a long error output.
func imageErrTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
