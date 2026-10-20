package transcode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Offline downloads (M5 design 3.2): a download is encoded into the same fragmented MP4 segments as
// an HLS output, and its encode yields to live playback in the admission control. This file holds
// the API that package offline builds on.

// downloadDir is the subdirectory of a download's work directory that ffmpeg writes into.
const downloadDir = "encode"

// Download encodes src with plan into workDir as fragmented MP4 segments, the same ones the HLS
// path produces (4-s segments for transcodes, keyframe segments for remuxes), starting at
// startSegment. onInit receives the init segment (ftyp+moov) once per call; onSegment every
// finished media segment in order with its index and its start and duration on the playlist
// timeline. It returns when the last segment was delivered, ctx ended (ctx.Err()) or ffmpeg failed.
//
// ffmpeg runs with the arguments of an HLS run from startSegment (ffmpegArgs: the -ss,
// -segment_start_number and -output_ts_offset of a restart after a seek), so a download resumed at
// segment n gets the boundaries of an uninterrupted one; start and dur are Plan.Segments[n]. The
// fragments keep the HLS timeline (source time plus timelineOffset). ffmpeg writes into
// workDir/encode, which Download wipes before the start and removes on return; each segment file
// is deleted once onSegment accepted it, and nothing else in workDir is touched. One Download per
// workDir at a time. The byte slices passed to the callbacks are theirs to keep.
//
// Canceling ctx stops ffmpeg (SIGTERM, SIGKILL after killGrace) and Download returns ctx.Err()
// after it exited (also when a callback failed meanwhile); segments ffmpeg closes after the
// cancel are not delivered. A callback error stops ffmpeg the same way and is returned wrapped,
// like an ffmpeg failure (with its stderr tail). Segments that do not follow the plan (a gap, or
// ffmpeg ending early) are an error wrapping ErrNoSegment. Once the last segment is delivered
// Download returns nil. ffmpeg runs with nice 10 and oom_score_adj 500 like HLS runs.
func (r Runner) Download(ctx context.Context, caps Capabilities, src Source, plan Plan, workDir string,
	startSegment int, onInit func(init []byte) error,
	onSegment func(n int, data []byte, start, dur time.Duration) error) error {
	if r.FFmpegPath == "" {
		return errors.New("ffmpeg path is not configured")
	}
	if onInit == nil || onSegment == nil {
		return errors.New("download needs both callbacks")
	}
	if startSegment < 0 || startSegment >= len(plan.Segments) {
		return fmt.Errorf("%w: start segment %d of %d", ErrNoSegment, startSegment, len(plan.Segments))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Join(workDir, downloadDir)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("wipe download work dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create download work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	args, err := ffmpegArgs(caps, src, plan, startSegment, dir)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stderr := &tailWriter{}
	cmd, err := startFFmpeg(runCtx, r.FFmpegPath, args, stderr, slog.New(slog.DiscardHandler))
	if err != nil {
		return err
	}
	d := &download{dir: dir, plan: plan, next: startSegment, onInit: onInit, onSegment: onSegment}
	var fail error // first delivery error; ffmpeg is stopped after it
	waitErr := pollUntilExit(cmd, func() {
		if fail == nil && runCtx.Err() == nil {
			if fail = d.collect(runCtx, false); fail != nil {
				cancel()
			}
		}
	})
	if fail == nil && waitErr == nil && ctx.Err() == nil {
		fail = d.collect(runCtx, true)
	}
	switch {
	case d.next == len(plan.Segments): // a failure leaves at least one segment undelivered
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case fail != nil:
		return fail
	case waitErr != nil:
		return fmt.Errorf("ffmpeg from segment %d: %w: %s", startSegment, waitErr, stderr.String())
	default:
		return fmt.Errorf("%w: ffmpeg ended before segment %d of %d: %s", ErrNoSegment, d.next, len(plan.Segments), stderr.String())
	}
}

// download is the delivery state of one Download call.
type download struct {
	dir       string
	plan      Plan
	next      int       // segment to deliver next
	seen      int       // entries of the segment list already processed
	stalled   time.Time // when the next listed segment was first seen incomplete
	initSent  bool
	onInit    func(init []byte) error
	onSegment func(n int, data []byte, start, dur time.Duration) error
}

// collect delivers the segments ffmpeg has listed since the last call, with the incomplete-data
// rule of Session.collect (stallCheck). It stops without error once ctx ended, because ffmpeg
// closes the segment it is interrupted in as if it were complete.
func (d *download) collect(ctx context.Context, final bool) error {
	data, err := os.ReadFile(filepath.Join(d.dir, segmentList))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	idx := parseSegmentList(data)
	for ; d.seen < len(idx) && ctx.Err() == nil; d.seen++ {
		k := idx[d.seen]
		if k != d.next || k >= len(d.plan.Segments) {
			return fmt.Errorf("%w: ffmpeg wrote segment %d, want %d of %d", ErrNoSegment, k, d.next, len(d.plan.Segments))
		}
		path := filepath.Join(d.dir, fmt.Sprintf(segmentPattern, k))
		retry, err := stallCheck(checkComplete(path), final, &d.stalled, incompleteWait, k)
		if retry {
			return nil
		}
		if err != nil {
			return err
		}
		if err := d.deliver(k, path); err != nil {
			return err
		}
		d.next++
	}
	return nil
}

// deliver hands segment k to the callbacks, the header of the run's first file to onInit, and
// deletes the file.
func (d *download) deliver(k int, path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // G304: download work dir and a fixed file name
	if err != nil {
		return err
	}
	off, err := firstMoof(data)
	if err != nil {
		return fmt.Errorf("segment %s: %w", path, err)
	}
	switch {
	case off > 0 && !d.initSent:
		if err := d.onInit(data[:off:off]); err != nil {
			return fmt.Errorf("download init: %w", err)
		}
		d.initSent = true
	case !d.initSent:
		return fmt.Errorf("segment %d: the first file of the run has no init segment", k)
	}
	s := d.plan.Segments[k]
	if err := d.onSegment(k, data[off:], s.Start, s.Duration); err != nil {
		return fmt.Errorf("download segment %d: %w", k, err)
	}
	return os.Remove(path)
}

// DownloadLease holds admission budget for one download encode.
type DownloadLease struct {
	m         *Manager
	id        string
	cost      float64
	preempted chan struct{}
	stopping  bool // preempted is closed (under m.mu)
	released  bool // under m.mu
}

// Preempted is closed when live playback needs the units; the holder stops its encoder and calls
// Release.
func (l *DownloadLease) Preempted() <-chan struct{} { return l.preempted }

// Release returns the units; it is idempotent.
func (l *DownloadLease) Release() {
	m := l.m
	m.mu.Lock()
	defer m.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	if i := slices.Index(m.leases, l); i >= 0 {
		m.leases = slices.Delete(m.leases, i, i+1)
	}
	m.wakeLocked()
}

// AcquireDownload reserves cost units for a download encode. It waits (ctx) until the budget has
// room and no live start is waiting; a cost above the whole budget is capped at the budget, so a
// download always runs eventually. Live playback has priority (plan 3.5): when a Start lacks
// budget, the manager preempts download leases (newest first) before it waits its 10 s.
//
// A lease also counts when it was preempted, until Release; a session resuming after an idle stop
// preempts the leases that no longer fit next to it, and Close preempts all. Waiting downloads
// are not ordered among each other. id names the download in the log (a ULID); cost is the
// AdmissionCost of the download's plan.
func (m *Manager) AcquireDownload(ctx context.Context, id string, cost float64) (*DownloadLease, error) {
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("invalid download id %q", id)
	}
	if math.IsNaN(cost) || cost < 0 {
		return nil, fmt.Errorf("invalid download cost %v", cost)
	}
	cost = min(cost, m.budget)
	m.mu.Lock()
	for {
		if m.closed {
			m.mu.Unlock()
			return nil, ErrClosed
		}
		if len(m.waiting) == 0 && m.used+m.leaseUnitsLocked(false)+cost <= m.budget {
			l := &DownloadLease{m: m, id: id, cost: cost, preempted: make(chan struct{})}
			m.leases = append(m.leases, l)
			m.mu.Unlock()
			m.log.Info("download encode admitted", "download", id, "cost", cost)
			return l, nil
		}
		freed := m.freed
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-freed:
		}
		m.mu.Lock()
	}
}

// AdmissionCost is the budget a plan takes on this manager: Plan.Cost, raised for an HDR transcode
// whose tonemapping runs on the CPU. A download passes it to AcquireDownload.
func (m *Manager) AdmissionCost(p Plan) float64 { return m.cost(p) }

// leaseUnitsLocked returns the cost of the download leases, only of those not preempted yet when
// runningOnly is set.
func (m *Manager) leaseUnitsLocked(runningOnly bool) float64 {
	var units float64
	for _, l := range m.leases {
		if !runningOnly || !l.stopping {
			units += l.cost
		}
	}
	return units
}

// preemptLocked closes the Preempted channel of download leases, newest first, until fits accepts
// the cost of the leases that keep running. Preempted leases hold their units until Release.
func (m *Manager) preemptLocked(fits func(leases float64) bool) {
	running := m.leaseUnitsLocked(true)
	for i := len(m.leases) - 1; i >= 0 && !fits(running); i-- {
		l := m.leases[i]
		if l.stopping {
			continue
		}
		l.stopping = true
		close(l.preempted)
		running = max(0, running-l.cost)
		m.log.Info("download encode preempted for live playback", "download", l.id, "cost", l.cost)
	}
}
