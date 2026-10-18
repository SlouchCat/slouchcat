// Package transcode is the only adapter around ffmpeg and ffprobe (plan section 3.8, point 5;
// CLAUDE.md rule 3). It is the only package allowed to import os/exec, and it always starts
// processes with exec.CommandContext and an argument list, never through a shell.
package transcode

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs the configured ffmpeg and ffprobe binaries.
type Runner struct {
	FFmpegPath  string
	FFprobePath string
}

// Version returns the first line of `ffprobe -version`, for example
// "ffprobe version 7.1.2-Jellyfin Copyright (c) 2007-2025 the FFmpeg developers".
func (r Runner) Version(ctx context.Context) (string, error) {
	if r.FFprobePath == "" {
		return "", errors.New("ffprobe path is not configured")
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFprobePath, "-version") //nolint:gosec // G204: configured binary, fixed argument list
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s -version: %w: %s", r.FFprobePath, err, strings.TrimSpace(stderr.String()))
	}
	line, _, _ := bufio.NewReader(bytes.NewReader(out)).ReadLine()
	if len(line) == 0 {
		return "", fmt.Errorf("%s -version printed nothing", r.FFprobePath)
	}
	return string(line), nil
}
