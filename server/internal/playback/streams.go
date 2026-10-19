package playback

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Open direct-play streams (plan 5.4: a revoked playback answers 401 within a second, also for a
// response that is already streaming). Browsers keep a Range request "bytes=0-" open for the
// whole film, so a revocation must end the running response, not only the next request: every
// file OpenGrant returns is registered under its playback id, and Stop, EndSession and the sweep
// cancel it. The file also re-reads its playback session every grantRecheckInterval, which catches
// revocations from elsewhere (an ended browser session, an expiry).

// grantRecheckInterval is how often an open direct-play stream re-reads its playback session.
const grantRecheckInterval = 5 * time.Second

// grantFile is a media file served under a stream token. Its reads fail with
// model.ErrUnauthenticated once the playback session ended; Done is closed then (and when the
// request ends), so the HTTP layer can also abort a write that waits for a slow client.
type grantFile struct {
	io.ReadSeekCloser
	s      *Service
	id     string
	ctx    context.Context
	cancel context.CancelCauseFunc

	mu      sync.Mutex
	checked time.Time // last re-read of the playback session (wall clock)
	once    sync.Once
}

// track wraps f as a grantFile of playback id and registers it until Close.
func (s *Service) track(ctx context.Context, id string, f io.ReadSeekCloser) io.ReadSeekCloser {
	gctx, cancel := context.WithCancelCause(ctx)
	g := &grantFile{ReadSeekCloser: f, s: s, id: id, ctx: gctx, cancel: cancel, checked: time.Now()}
	s.streamsMu.Lock()
	if s.streams[id] == nil {
		s.streams[id] = map[*grantFile]struct{}{}
	}
	s.streams[id][g] = struct{}{}
	s.streamsMu.Unlock()
	return g
}

// cancelStreams ends every open stream of playback id.
func (s *Service) cancelStreams(id string) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	for g := range s.streams[id] {
		g.cancel(model.ErrUnauthenticated)
	}
}

func (s *Service) untrack(g *grantFile) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	delete(s.streams[g.id], g)
	if len(s.streams[g.id]) == 0 {
		delete(s.streams, g.id)
	}
}

// Done is closed when the playback session ended or the request is gone.
func (g *grantFile) Done() <-chan struct{} { return g.ctx.Done() }

func (g *grantFile) Read(p []byte) (int, error) {
	if err := g.err(); err != nil {
		return 0, err
	}
	if g.due() && !g.s.grantValid(g.ctx, g.id) {
		g.cancel(model.ErrUnauthenticated)
		return 0, model.ErrUnauthenticated
	}
	return g.ReadSeekCloser.Read(p)
}

func (g *grantFile) Seek(offset int64, whence int) (int64, error) {
	if err := g.err(); err != nil {
		return 0, err
	}
	return g.ReadSeekCloser.Seek(offset, whence)
}

func (g *grantFile) Close() error {
	g.once.Do(func() {
		g.s.untrack(g)
		g.cancel(nil)
	})
	return g.ReadSeekCloser.Close()
}

func (g *grantFile) err() error {
	if g.ctx.Err() == nil {
		return nil
	}
	return context.Cause(g.ctx)
}

// due reports whether the playback session must be re-read now.
func (g *grantFile) due() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if time.Since(g.checked) < g.s.grantRecheck {
		return false
	}
	g.checked = time.Now()
	return true
}

// grantValid re-reads a playback session by primary key. Only a missing (revoked, expired or
// logged out) row ends a stream; a database error is logged and playback goes on.
func (s *Service) grantValid(ctx context.Context, id string) bool {
	_, err := s.read.GetPlaybackSession(ctx, appq.GetPlaybackSessionParams{ID: id, Now: s.now().UnixMilli()})
	switch {
	case err == nil:
		return true
	case errors.Is(err, sql.ErrNoRows):
		return false
	default:
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "recheck playback session", "playback_id", id, "err", err)
		}
		return true
	}
}
