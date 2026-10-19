package playback

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// ManagerTranscoder adapts the transcode.Manager of the process to Transcoder: it turns a
// decision into a transcode target and plan, maps the manager's errors to model errors and renders
// the master playlist with the codecs of the plan.
type ManagerTranscoder struct {
	m       *transcode.Manager
	backend transcode.Backend

	mu       sync.Mutex
	sessions map[string]*managerSession
}

// NewManagerTranscoder returns the Transcoder over m; backend is the one transcode.Detect chose.
func NewManagerTranscoder(m *transcode.Manager, backend transcode.Backend) *ManagerTranscoder {
	return &ManagerTranscoder{m: m, backend: backend, sessions: map[string]*managerSession{}}
}

// Backend names the encoder backend of transcodes.
func (t *ManagerTranscoder) Backend() string { return string(t.backend) }

// Start plans the HLS output of req and starts the encoder at the segment of req.Resume (else
// req.StartSegment); an exhausted budget returns model.ErrTranscodeBusy, a remux without keyframe
// index ErrNoKeyframes.
func (t *ManagerTranscoder) Start(ctx context.Context, req HLSRequest) (HLSSession, error) {
	d := req.Decision
	src := transcode.Source{Path: req.Path, Probe: req.Probe, Keyframes: req.Keyframes}
	target := transcode.Target{
		Mode: transcode.Mode(d.Mode), VideoCopy: d.VideoCopy, AudioCopy: d.AudioCopy,
		MaxHeight: d.MaxHeight, Tonemap: d.Tonemap, VideoTag: d.VideoTag, SubtitleBurnIn: -1,
	}
	plan, err := transcode.NewPlan(src, target)
	if errors.Is(err, transcode.ErrNoKeyframes) {
		return nil, errors.Join(ErrNoKeyframes, err)
	}
	if err != nil {
		return nil, fmt.Errorf("plan hls output %s: %w", req.ID, err)
	}
	start := req.StartSegment
	if req.Resume > 0 {
		start = segmentAt(plan.Segments, req.Resume)
	}
	start = max(0, min(start, len(plan.Segments)-1))
	sess, err := t.m.Start(ctx, req.ID, src, plan, start)
	if errors.Is(err, transcode.ErrBusy) {
		return nil, model.ErrTranscodeBusy
	}
	if err != nil {
		return nil, err
	}
	ms := &managerSession{s: sess, codecs: transcode.CodecsString(plan, src)}
	t.mu.Lock()
	t.sessions[req.ID] = ms
	t.mu.Unlock()
	return ms, nil
}

// Get returns the output of a playback session while the manager has it; the entry of a session
// the manager dropped is removed.
func (t *ManagerTranscoder) Get(id string) (HLSSession, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ms, ok := t.sessions[id]
	if !ok {
		return nil, false
	}
	if sess, alive := t.m.Get(id); !alive || sess != ms.s {
		delete(t.sessions, id)
		return nil, false
	}
	return ms, true
}

// segmentAt returns the index of the segment that contains pos (the last one beyond the end).
// On the 4-s transcode grid that is floor(pos / 4 s); a remux plan cuts on keyframes.
func segmentAt(segs []transcode.Segment, pos time.Duration) int {
	i, _ := slices.BinarySearchFunc(segs, pos, func(s transcode.Segment, p time.Duration) int {
		return cmp.Compare(s.Start, p)
	})
	if i < len(segs) && segs[i].Start == pos {
		return i
	}
	return max(0, i-1)
}

// Stop ends the output and removes its files.
func (t *ManagerTranscoder) Stop(id string) {
	t.mu.Lock()
	delete(t.sessions, id)
	t.mu.Unlock()
	t.m.Stop(id)
}

// managerSession is a transcode.Session with the CODECS string of its master playlist.
type managerSession struct {
	s      *transcode.Session
	codecs string
}

func (ms *managerSession) MasterPlaylist(mediaURL string) []byte {
	p := ms.s.Plan()
	return transcode.MasterPlaylist(mediaURL, ms.codecs, p.Width, p.Height, p.Bandwidth)
}

func (ms *managerSession) MediaPlaylist(initURL string, segmentURL func(int) string) []byte {
	return ms.s.MediaPlaylist(initURL, segmentURL)
}

func (ms *managerSession) OpenInit(ctx context.Context) (io.ReadSeekCloser, error) {
	f, err := ms.s.OpenInit(ctx)
	return f, notFoundIfEnded(err)
}

func (ms *managerSession) OpenSegment(ctx context.Context, i int) (io.ReadSeekCloser, error) {
	f, err := ms.s.OpenSegment(ctx, i)
	return f, notFoundIfEnded(err)
}

func (ms *managerSession) Touch() { ms.s.Touch() }

// notFoundIfEnded maps a segment outside the plan and a stopped session to model.ErrNotFound and
// a wait that a newer seek superseded to model.ErrSegmentRestarted (503, retry in a second).
func notFoundIfEnded(err error) error {
	switch {
	case errors.Is(err, transcode.ErrNoSegment) || errors.Is(err, transcode.ErrClosed):
		return errors.Join(model.ErrNotFound, err)
	case errors.Is(err, transcode.ErrRestarted):
		return errors.Join(model.ErrSegmentRestarted, err)
	}
	return err
}
