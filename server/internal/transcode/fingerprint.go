package transcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// fingerprintTimeout bounds one fingerprint run. Ten minutes of audio decode in a few seconds;
// a slow network share or a damaged file must not hold a job worker for longer.
const fingerprintTimeout = 3 * time.Minute

// Fingerprint returns the raw Chromaprint fingerprint of the first audio stream of the file at
// path, from from for length (W-9): ffmpeg's chromaprint muxer downmixes to mono at 11025 Hz and
// writes one unsigned 32-bit little-endian value per 1365 samples (about 8 per second; package
// segments interprets them). path is absolute and resolved through package paths.
func (r Runner) Fingerprint(ctx context.Context, path string, from, length time.Duration) ([]uint32, error) {
	if r.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	if from < 0 || length <= 0 {
		return nil, fmt.Errorf("fingerprint range %v+%v is invalid", from, length)
	}
	ctx, cancel := context.WithTimeout(ctx, fingerprintTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFmpegPath, FingerprintArgs(path, from, length)...) //nolint:gosec // G204: configured binary, fixed argument list, path resolved by package paths
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("fingerprint: %w", ctx.Err())
		}
		return nil, fmt.Errorf("fingerprint: ffmpeg: %w: %s", err, imageErrTail(strings.TrimSpace(stderr.String()), 2048))
	}
	out := stdout.Bytes()
	if len(out)%4 != 0 {
		return nil, fmt.Errorf("fingerprint: %d bytes are not a sequence of 32-bit values", len(out))
	}
	fp := make([]uint32, len(out)/4)
	for i := range fp {
		fp[i] = binary.LittleEndian.Uint32(out[4*i:])
	}
	return fp, nil
}

// FingerprintArgs returns the ffmpeg arguments of Fingerprint: input seeking, the first audio
// stream only, the raw fingerprint on stdout.
func FingerprintArgs(path string, from, length time.Duration) []string {
	return []string{
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-ss", fingerprintSeconds(from), "-t", fingerprintSeconds(length), "-i", path,
		"-map", "0:a:0", "-ac", "1", "-ar", "11025",
		"-f", "chromaprint", "-fp_format", "raw", "-",
	}
}

// fingerprintSeconds formats d as seconds with millisecond precision ("12.345").
func fingerprintSeconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}
