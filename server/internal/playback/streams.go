package playback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
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
	s         *Service
	id        string
	accountID int64 // owner of the playback, so a revocation reaches the stream in memory
	profileID int64
	itemID    string
	ctx       context.Context
	cancel    context.CancelCauseFunc

	mu      sync.Mutex
	checked time.Time // last re-read of the playback session (wall clock)
	once    sync.Once
}

// track wraps f as a grantFile of the grant's playback session and registers it until Close.
func (s *Service) track(ctx context.Context, grant model.PlaybackGrant, f io.ReadSeekCloser) io.ReadSeekCloser {
	gctx, cancel := context.WithCancelCause(ctx)
	id := grant.ID
	g := &grantFile{
		ReadSeekCloser: f, s: s, id: id, accountID: grant.Session.Account.ID, profileID: grant.Session.Profile.ID,
		itemID: grant.ItemID, ctx: gctx, cancel: cancel, checked: time.Now(),
	}
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

// cancelWhere ends every open stream for which match is true.
func (s *Service) cancelWhere(match func(*grantFile) bool) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()
	for _, set := range s.streams {
		for g := range set {
			if match(g) {
				g.cancel(model.ErrUnauthenticated)
			}
		}
	}
}

// cancelAccount ends every open stream of an account, whatever its playback row says.
func (s *Service) cancelAccount(accountID int64) {
	s.cancelWhere(func(g *grantFile) bool { return g.accountID == accountID })
}

// cancelProfile ends every open stream of a profile.
func (s *Service) cancelProfile(profileID int64) {
	s.cancelWhere(func(g *grantFile) bool { return g.profileID == profileID })
}

// RecheckVisibility ends every open direct-play stream whose title the profile of the stream may
// no longer see (a rating override, a lowered ceiling, a removed library right). The caller runs it
// after such a change; HLS output needs no such call, because every playlist and segment request
// re-checks visibility. Streams of titles that stay visible are left alone.
func (s *Service) RecheckVisibility(ctx context.Context) error {
	type open struct {
		g         *grantFile
		profileID int64
		itemID    string
	}
	s.streamsMu.Lock()
	var streams []open
	for _, set := range s.streams {
		for g := range set {
			streams = append(streams, open{g: g, profileID: g.profileID, itemID: g.itemID})
		}
	}
	s.streamsMu.Unlock()
	for _, o := range streams {
		err := s.vis.Visible(ctx, visibility.Profile(model.Profile{ID: o.profileID}), o.itemID)
		switch {
		case err == nil:
		case errors.Is(err, model.ErrNotFound):
			o.g.cancel(model.ErrNotFound)
		default:
			return fmt.Errorf("recheck visibility of %s for profile %d: %w", o.itemID, o.profileID, err)
		}
	}
	return nil
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
