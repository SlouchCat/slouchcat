package transcode

import (
	"context"
	"errors"
	"time"
)

// Offline downloads (M5 design 3.2): a download is encoded into the same fragmented MP4 segments as
// an HLS output, and its encode yields to live playback in the admission control. This file holds
// the API that package offline builds on; M5 stream A implements it.

// errDownloadNotImplemented is returned until stream A replaces the bodies below.
var errDownloadNotImplemented = errors.New("transcode: download encode not implemented")

// Download encodes src with plan into workDir as fragmented MP4 segments, the same ones the HLS
// path produces (4-s segments for transcodes, keyframe segments for remuxes), starting at
// startSegment. onInit receives the init segment (ftyp+moov) once per call; onSegment every
// finished media segment in order with its index and its start and duration on the playlist
// timeline. It returns when the last segment was delivered, ctx ended (ctx.Err()) or ffmpeg failed.
func (r Runner) Download(ctx context.Context, caps Capabilities, src Source, plan Plan, workDir string,
	startSegment int, onInit func(init []byte) error,
	onSegment func(n int, data []byte, start, dur time.Duration) error) error {
	return errDownloadNotImplemented
}

// DownloadLease holds admission budget for one download encode.
type DownloadLease struct {
	preempted chan struct{}
}

// Preempted is closed when live playback needs the units; the holder stops its encoder and calls
// Release.
func (l *DownloadLease) Preempted() <-chan struct{} { return l.preempted }

// Release returns the units; it is idempotent.
func (l *DownloadLease) Release() {}

// AcquireDownload reserves cost units for a download encode. It waits (ctx) until the budget has
// room and no live start is waiting; a cost above the whole budget is capped at the budget, so a
// download always runs eventually. Live playback has priority (plan 3.5): when a Start lacks
// budget, the manager preempts download leases (newest first) before it waits its 10 s.
func (m *Manager) AcquireDownload(ctx context.Context, id string, cost float64) (*DownloadLease, error) {
	return nil, errDownloadNotImplemented
}
