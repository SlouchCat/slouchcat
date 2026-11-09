package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Trickplay (concept 18 part A, ADR 0027): preview tiles on the seek bar, one tile every 10 s,
// gathered into sheets of 10 x 10 tiles. A tile is one frame per ffmpeg run: an input seek with
// -noaccurate_seek lands on the keyframe before the time point and -fps_mode passthrough keeps
// ffmpeg from decoding on to the time point, so a point reads about 1 MB and decodes 3 to 4 frames
// (measured in docs/perf/18-trickplay.md). The frame comes back as raw yuv420p of a fixed size on
// standard output; package library collects the tiles of a sheet and TrickplaySheet encodes them
// as one WebP.

// Trickplay tile and sheet geometry (concept 18, section 7).
const (
	// TrickplayTileWidth is the width of a tile in pixels (F-4).
	TrickplayTileWidth = 320
	// TrickplayColumns and TrickplayRows are the tiles of a full sheet (100 points = 16 min 40 s).
	TrickplayColumns = 10
	TrickplayRows    = 10
	// trickplaySheetQuality is the WebP quality of a sheet (measurement 3.6: 1.3 to 1.8 MB per hour).
	trickplaySheetQuality = 75
	// defaultTileHeight is the height of a tile of a stream without dimensions (16:9).
	defaultTileHeight = 180
)

// TrickplayTileHeight returns the height of a tile of the video stream v: TrickplayTileWidth
// divided by the display aspect ratio (width and height of the stream with its sample aspect ratio,
// so an anamorphic source comes out undistorted), rounded down to an even number, at least 2. A
// stream without dimensions gets 16:9.
func TrickplayTileHeight(v Stream) int {
	if v.Width <= 0 || v.Height <= 0 {
		return defaultTileHeight
	}
	num, den := v.SampleAspect()
	h := TrickplayTileWidth * v.Height * den / (v.Width * num)
	return max(h/2*2, 2)
}

// SampleAspect returns the sample aspect ratio of a video stream as numerator and denominator
// (ffprobe sample_aspect_ratio, "32:27"); 1:1 when the probe has none, it is unknown ("0:1") or
// malformed (probes stored before contract 1.23.0 carry none).
func (v Stream) SampleAspect() (num, den int) {
	a, b, ok := strings.Cut(v.SAR, ":")
	if !ok {
		return 1, 1
	}
	n, err1 := strconv.Atoi(a)
	d, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil || n <= 0 || d <= 0 || n > 1000 || d > 1000 {
		return 1, 1
	}
	return n, d
}

// TrickplayTileBytes is the size of a raw yuv420p tile of the given height.
func TrickplayTileBytes(height int) int { return TrickplayTileWidth * height * 3 / 2 }

// BlackTrickplayTile returns a black raw yuv420p tile of the given height (TV range: luma 16,
// chroma 128), the filler of a point whose frame ffmpeg could not give.
func BlackTrickplayTile(height int) []byte {
	luma := TrickplayTileWidth * height
	tile := make([]byte, TrickplayTileBytes(height))
	for i := range tile {
		if i < luma {
			tile[i] = 16
		} else {
			tile[i] = 128
		}
	}
	return tile
}

// TrickplayFrameArgs builds the ffmpeg argument list of TrickplayFrame: one decoder thread, an
// input seek without accurate seeking to at, the video stream v (its index; cover art never), one
// frame without frame rate conversion, scaled to TrickplayTileWidth x height with square pixels,
// tonemapped to BT.709 on the CPU when v is HDR (the chain of GrabFrameArgs), as raw yuv420p on
// standard output.
func TrickplayFrameArgs(inPath string, at time.Duration, height int, v Stream) ([]string, error) {
	if inPath == "" || at < 0 || height <= 0 || height%2 != 0 {
		return nil, fmt.Errorf("invalid trickplay frame request: %q at %v, height %d", inPath, at, height)
	}
	w, h := strconv.Itoa(TrickplayTileWidth), strconv.Itoa(height)
	var chain []string
	if v.IsHDR() {
		if v.DolbyVisionProfile > 0 && v.ColorTransfer == "" {
			chain = append(chain, dvAsHDR10) // profile 5 carries no transfer tag (pipeline.go)
		}
		chain = append(chain, "zscale=w="+w+":h="+h, "setsar=1")
		chain = append(chain, cpuTonemap()...)
	} else {
		chain = []string{"scale=w=" + w + ":h=" + h + ":flags=lanczos", "setsar=1", "format=yuv420p"}
	}
	return []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-threads", "1",
		"-noaccurate_seek", "-ss", strconv.FormatFloat(at.Seconds(), 'f', 3, 64), "-i", inPath,
		"-map", "0:" + strconv.Itoa(v.Index), "-frames:v", "1", "-fps_mode", "passthrough",
		"-vf", strings.Join(chain, ","), "-f", "rawvideo", "-pix_fmt", "yuv420p", "pipe:1"}, nil
}

// TrickplayFrame returns the tile of the video stream v of the file at inPath at the time point at
// as raw yuv420p of TrickplayTileWidth x height (TrickplayTileBytes). The frame is the keyframe at
// or before at; callers seek to a keyframe plus 1 ms (concept 18, 5.4). A time point past the last
// frame is ErrNoFrame; FrameFailed tells a broken point from a file ffmpeg could not read. Callers
// pass an absolute path resolved through package paths and bound ctx (15 s per point).
func (r Runner) TrickplayFrame(ctx context.Context, inPath string, at time.Duration, height int, v Stream) ([]byte, error) {
	args, err := TrickplayFrameArgs(inPath, at, height, v)
	if err != nil {
		return nil, err
	}
	out, err := r.runFFmpeg(ctx, nil, args...)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, fmt.Errorf("%w: %w", cerr, err)
		}
		return nil, err
	}
	if len(out) != TrickplayTileBytes(height) {
		return nil, fmt.Errorf("%w at %v (%d bytes of %d)", ErrNoFrame, at, len(out), TrickplayTileBytes(height))
	}
	return out, nil
}

// TrickplaySheetArgs builds the ffmpeg argument list of TrickplaySheet: raw yuv420p tiles of
// TrickplayTileWidth x height from standard input, laid out TrickplayColumns wide and rows high,
// one WebP (quality 75) to outPath.
func TrickplaySheetArgs(height, rows int, outPath string) ([]string, error) {
	if height <= 0 || height%2 != 0 || rows <= 0 || rows > TrickplayRows || outPath == "" {
		return nil, fmt.Errorf("invalid trickplay sheet: height %d, rows %d", height, rows)
	}
	return []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "yuv420p", "-s", strconv.Itoa(TrickplayTileWidth) + "x" + strconv.Itoa(height),
		"-framerate", "1", "-i", "pipe:0",
		"-vf", "tile=layout=" + strconv.Itoa(TrickplayColumns) + "x" + strconv.Itoa(rows),
		"-frames:v", "1", "-c:v", "libwebp", "-quality", strconv.Itoa(trickplaySheetQuality), "-compression_level", "4",
		"-f", "webp", outPath}, nil
}

// TrickplaySheet encodes tiles (raw yuv420p of TrickplayTileWidth x height each, at most
// TrickplayColumns x TrickplayRows) into one WebP sheet at outPath: row by row, as many rows as the
// tiles need, the rest of the last row black. outPath is an absolute path resolved through package
// paths; callers write it under a temporary name and rename it.
func (r Runner) TrickplaySheet(ctx context.Context, tiles [][]byte, height int, outPath string) error {
	if len(tiles) == 0 || len(tiles) > TrickplayColumns*TrickplayRows {
		return fmt.Errorf("invalid trickplay sheet: %d tiles", len(tiles))
	}
	rows := (len(tiles) + TrickplayColumns - 1) / TrickplayColumns
	args, err := TrickplaySheetArgs(height, rows, outPath)
	if err != nil {
		return err
	}
	size := TrickplayTileBytes(height)
	var in bytes.Buffer
	in.Grow(rows * TrickplayColumns * size)
	for i, t := range tiles {
		if len(t) != size {
			return fmt.Errorf("trickplay tile %d has %d bytes, want %d", i, len(t), size)
		}
		in.Write(t)
	}
	if pad := rows*TrickplayColumns - len(tiles); pad > 0 {
		black := BlackTrickplayTile(height)
		for range pad {
			in.Write(black)
		}
	}
	_, err = r.runFFmpeg(ctx, in.Bytes(), args...)
	return err
}

// errNoFFmpeg is the error of a runner without ffmpeg.
var errNoFFmpeg = errors.New("ffmpeg path is not configured")
