package playback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

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

// Start plans the HLS output of req and starts it; an exhausted budget returns
// model.ErrTranscodeBusy.
func (t *ManagerTranscoder) Start(ctx context.Context, req HLSRequest) (HLSSession, error) {
	d := req.Decision
	src := transcode.Source{Path: req.Path, Probe: req.Probe, Keyframes: req.Keyframes}
	target := transcode.Target{
		Mode: transcode.Mode(d.Mode), VideoCopy: d.VideoCopy, AudioCopy: d.AudioCopy,
		MaxHeight: d.MaxHeight, Tonemap: d.Tonemap, VideoTag: d.VideoTag, SubtitleBurnIn: -1,
	}
	plan, err := transcode.NewPlan(src, target)
	if err != nil {
		return nil, fmt.Errorf("plan hls output %s: %w", req.ID, err)
	}
	sess, err := t.m.Start(ctx, req.ID, src, plan)
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

// Get returns the running output of a playback session.
func (t *ManagerTranscoder) Get(id string) (HLSSession, bool) {
	t.mu.Lock()
	ms, ok := t.sessions[id]
	t.mu.Unlock()
	if !ok {
		return nil, false
	}
	if _, running := t.m.Get(id); !running {
		return nil, false
	}
	return ms, true
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

// notFoundIfEnded maps a segment outside the plan and a stopped session to model.ErrNotFound.
func notFoundIfEnded(err error) error {
	if errors.Is(err, transcode.ErrNoSegment) || errors.Is(err, transcode.ErrClosed) {
		return errors.Join(model.ErrNotFound, err)
	}
	return err
}
