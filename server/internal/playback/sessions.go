package playback

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Playback sessions (plan 5.4): every started playback has a row in playback_sessions; its stream
// token authorises the Range endpoint (direct) or the HLS endpoints. Rows expire 12 h after the
// last token request and are revoked on stop and logout; a sweep removes them a day later.

const (
	// SessionLifetime is the sliding expiry of a playback session after its last token request.
	SessionLifetime = 12 * time.Hour
	// touchInterval limits how often token requests write last_activity_at/expires_at.
	touchInterval = time.Minute
	// sweepInterval limits how often Start looks for ended sessions.
	sweepInterval = 10 * time.Minute
	// endedRetention keeps revoked and expired rows for a day before the sweep deletes them.
	endedRetention = 24 * time.Hour
)

// HLSRequest is what a Transcoder needs to start the HLS output of a playback session.
type HLSRequest struct {
	ID        string          // playback session id (ULID), also the transcode session id
	Path      string          // absolute path of the media file, resolved through package paths
	Probe     transcode.Probe // stored ffprobe result
	Keyframes []time.Duration // keyframe index, nil when not indexed yet
	Decision  Decision        // ModeRemux or ModeTranscode
}

// HLSSession is a running HLS output.
type HLSSession interface {
	model.HLSStream
	// Touch marks activity (idle timer of the encoder).
	Touch()
}

// Transcoder produces HLS output (package transcode, adapted in transcoder.go; tests use a fake).
type Transcoder interface {
	// Backend names the encoder backend of transcodes: nvenc, qsv, vaapi or software.
	Backend() string
	// Start admits and starts the output; it returns model.ErrTranscodeBusy when the budget stays
	// exhausted.
	Start(ctx context.Context, req HLSRequest) (HLSSession, error)
	// Get returns the running output of a playback session.
	Get(id string) (HLSSession, bool)
	// Stop ends the output and removes its files; unknown ids are ignored.
	Stop(id string)
}

// Start decides how the client plays a visible title, records the playback session and returns
// the playback description with the token URL. mediaFileID may be empty for the primary file. The
// resume position is the profile's position unless the title was completed.
func (s *Service) Start(ctx context.Context, session model.Session, itemID string, req model.PlaybackRequest) (model.PlaybackInfo, error) {
	if len(req.DeviceID) > maxClientIDLength {
		return model.PlaybackInfo{}, (&model.ValidationError{}).Add("deviceId", "too_long")
	}
	scope := visibility.Profile(session.Profile)
	loc, err := s.vis.MediaFile(ctx, scope, itemID, req.MediaFileID)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	if !loc.File.Available {
		return model.PlaybackInfo{}, model.ErrNotFound
	}
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	s.sweep(ctx)

	file := loc.File
	file.MimeType = MimeType(file.Container, file.VideoCodec, file.AudioCodec)
	hls, decision, err := s.hlsRequest(ctx, scope, loc, normalizeCapabilities(req.Capabilities))
	if err != nil {
		return model.PlaybackInfo{}, err
	}

	id := ulid.Make().String()
	now := s.now()
	backend := ""
	if decision.Mode == ModeTranscode {
		backend = s.tc.Backend()
	}
	err = s.write.InsertPlaybackSession(ctx, appq.InsertPlaybackSessionParams{
		ID: id, SessionID: session.ID, AccountID: session.Account.ID, ProfileID: session.Profile.ID,
		ItemID: item.ID, MediaFileID: file.ID, DeviceID: nullString(req.DeviceID), Mode: decision.Mode,
		Backend: nullString(backend), Reason: nullString(decision.Reason),
		Now: now.UnixMilli(), ExpiresAt: now.Add(SessionLifetime).UnixMilli(),
	})
	if err != nil {
		return model.PlaybackInfo{}, fmt.Errorf("insert playback session: %w", err)
	}

	st := url.QueryEscape(SignToken(s.key, id, item.ID))
	info := model.PlaybackInfo{
		Mode:              ModeDirect,
		Item:              item,
		MediaFile:         file,
		StreamURL:         "/api/v1/items/" + url.PathEscape(item.ID) + "/stream?st=" + st,
		PlaybackSessionID: id,
	}
	if hls != nil {
		hls.ID = id
		if _, err := s.tc.Start(ctx, *hls); err != nil {
			s.revoke(ctx, id, session.ID)
			return model.PlaybackInfo{}, err
		}
		info.Mode = "hls"
		info.StreamURL = "/api/v1/playback/sessions/" + id + "/master.m3u8?st=" + st
		info.Transcoding = &model.Transcoding{Mode: decision.Mode, Backend: backend, Reason: decision.Reason}
	}
	if p := item.Progress; p != nil && !p.Completed && p.PositionMs > 0 {
		info.ResumePositionMs = p.PositionMs
	}
	s.log.InfoContext(ctx, "playback started", "playback_id", id, "item_id", item.ID, "media_file_id", file.ID,
		"mode", decision.Mode, "backend", backend, "reason", decision.Reason)
	return info, nil
}

// hlsRequest decides the mode and, for remux and transcode, prepares the transcoder request. It
// falls back to direct play when no transcoder is configured or the file is not probed yet, so the
// client checks the file itself (the M1a behaviour).
func (s *Service) hlsRequest(ctx context.Context, scope visibility.Scope, loc visibility.MediaFileLocation, caps model.PlaybackCapabilities) (*HLSRequest, Decision, error) {
	pf, err := s.vis.MediaFileProbe(ctx, scope, loc.ItemID, loc.File.ID)
	if err != nil {
		return nil, Decision{}, err
	}
	var probe *transcode.Probe
	if pf.Probe != "" {
		var p transcode.Probe
		if err := json.Unmarshal([]byte(pf.Probe), &p); err != nil {
			s.log.WarnContext(ctx, "stored probe unreadable", "media_file_id", loc.File.ID, "err", err)
		} else {
			probe = &p
		}
	}
	d := Decide(loc.File, probe, caps)
	if d.Mode == ModeDirect {
		return nil, d, nil
	}
	if s.tc == nil || probe == nil {
		s.log.InfoContext(ctx, "hls unavailable, falling back to direct play", "media_file_id", loc.File.ID,
			"transcoder", s.tc != nil, "probed", probe != nil)
		return nil, Decision{Mode: ModeDirect}, nil
	}
	root, err := paths.NewRoot(loc.RootPath)
	if err != nil {
		return nil, Decision{}, err
	}
	abs, err := paths.Resolve(root, loc.RelPath)
	if err != nil {
		return nil, Decision{}, fmt.Errorf("resolve media file %s: %w", loc.File.ID, err)
	}
	kf, err := library.DecodeKeyframes(pf.Keyframes)
	if err != nil {
		s.log.WarnContext(ctx, "keyframe index unreadable, remux falls back to the 4-s grid", "media_file_id", loc.File.ID, "err", err)
		kf = nil
	}
	return &HLSRequest{Path: abs, Probe: *probe, Keyframes: kf, Decision: d}, d, nil
}

// Authorize resolves a stream token to its playback session. A missing, malformed or forged token
// and a revoked or expired session (or browser session) return model.ErrUnauthenticated. The row
// is read by primary key on every request, so a revocation applies to the next request.
func (s *Service) Authorize(ctx context.Context, token string) (model.PlaybackGrant, error) {
	id, mac, ok := parseToken(token)
	if !ok {
		return model.PlaybackGrant{}, model.ErrUnauthenticated
	}
	now := s.now()
	row, err := s.read.GetPlaybackSession(ctx, appq.GetPlaybackSessionParams{ID: id, Now: now.UnixMilli()})
	if errors.Is(err, sql.ErrNoRows) {
		return model.PlaybackGrant{}, model.ErrUnauthenticated
	}
	if err != nil {
		return model.PlaybackGrant{}, fmt.Errorf("get playback session: %w", err)
	}
	if !hmac.Equal(mac, tokenMAC(s.key, row.ID, row.ItemID)) {
		return model.PlaybackGrant{}, model.ErrUnauthenticated
	}
	acc, err := s.read.GetAccount(ctx, row.AccountID)
	if err != nil {
		return model.PlaybackGrant{}, unauthenticatedOr(err, "get account")
	}
	prof, err := s.read.GetProfile(ctx, row.ProfileID)
	if err != nil {
		return model.PlaybackGrant{}, unauthenticatedOr(err, "get profile")
	}
	if now.Sub(time.UnixMilli(row.LastActivityAt)) >= touchInterval {
		err := s.write.TouchPlaybackSession(ctx, appq.TouchPlaybackSessionParams{
			ID: row.ID, Now: now.UnixMilli(), ExpiresAt: now.Add(SessionLifetime).UnixMilli(),
		})
		if err != nil {
			// The session stays valid until its old expiry; a failed touch must not stop playback.
			s.log.WarnContext(ctx, "touch playback session", "playback_id", row.ID, "err", err)
		}
	}
	return model.PlaybackGrant{
		ID: row.ID,
		Session: model.Session{
			ID:      row.SessionID,
			Account: model.Account{ID: acc.ID, Email: acc.Email, Role: acc.Role},
			Profile: model.Profile{
				ID: prof.ID, AccountID: prof.AccountID, Name: prof.Name, MaxRating: int(prof.MaxRating),
				Language: prof.Language, AllLibraries: prof.AllLibraries == 1, Accent: prof.Accent.String,
			},
		},
		ItemID:      row.ItemID,
		MediaFileID: row.MediaFileID,
		Mode:        row.Mode,
	}, nil
}

// HLS returns the running HLS output of a grant after checking that its title is still visible
// to the profile; a title that became invisible, a direct-play grant and an output that no longer
// runs return model.ErrNotFound.
func (s *Service) HLS(ctx context.Context, g model.PlaybackGrant) (model.HLSStream, error) {
	if err := s.vis.Visible(ctx, visibility.Profile(g.Session.Profile), g.ItemID); err != nil {
		return nil, err
	}
	if g.Mode == ModeDirect || s.tc == nil {
		return nil, model.ErrNotFound
	}
	hs, ok := s.tc.Get(g.ID)
	if !ok {
		return nil, model.ErrNotFound
	}
	hs.Touch()
	return hs, nil
}

// OpenGrant opens the media file of a direct-play grant for the Range endpoint; HLS grants and
// titles that are no longer visible return model.ErrNotFound.
func (s *Service) OpenGrant(ctx context.Context, g model.PlaybackGrant) (model.StreamSource, error) {
	if g.Mode != ModeDirect {
		return model.StreamSource{}, model.ErrNotFound
	}
	return s.Open(ctx, g.Session.Profile, g.ItemID, g.MediaFileID)
}

// Stop revokes a playback session of the browser session and stops its HLS output. A playback
// session of another browser session, an unknown and an already ended one return
// model.ErrNotFound.
func (s *Service) Stop(ctx context.Context, session model.Session, id string) error {
	n, err := s.write.RevokePlaybackSession(ctx, appq.RevokePlaybackSessionParams{
		ID: id, SessionID: session.ID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke playback session: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	if s.tc != nil {
		s.tc.Stop(id)
	}
	return nil
}

// EndSession revokes every playback session of a browser session (logout) and stops their HLS
// output.
func (s *Service) EndSession(ctx context.Context, sessionID int64) error {
	ids, err := s.write.RevokePlaybackSessionsBySession(ctx, appq.RevokePlaybackSessionsBySessionParams{
		SessionID: sessionID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke playback sessions: %w", err)
	}
	if s.tc != nil {
		for _, id := range ids {
			s.tc.Stop(id)
		}
	}
	return nil
}

// revoke ends a playback session whose HLS output did not start.
func (s *Service) revoke(ctx context.Context, id string, sessionID int64) {
	_, err := s.write.RevokePlaybackSession(context.WithoutCancel(ctx), appq.RevokePlaybackSessionParams{
		ID: id, SessionID: sessionID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		s.log.WarnContext(ctx, "revoke failed playback session", "playback_id", id, "err", err)
	}
}

// sweep runs at most every sweepInterval from Start: it stops the HLS output of revoked and
// expired playback sessions (a client that never sent its stop) and deletes rows that ended more
// than a day ago.
func (s *Service) sweep(ctx context.Context) {
	now := s.now()
	s.sweepMu.Lock()
	due := now.Sub(s.lastSweep) >= sweepInterval
	if due {
		s.lastSweep = now
	}
	s.sweepMu.Unlock()
	if !due {
		return
	}
	if s.tc != nil {
		ids, err := s.read.ListEndedPlaybackSessions(ctx, now.UnixMilli())
		if err != nil {
			s.log.WarnContext(ctx, "list ended playback sessions", "err", err)
		}
		for _, id := range ids {
			if _, running := s.tc.Get(id); running {
				s.tc.Stop(id)
			}
		}
	}
	if _, err := s.write.DeleteExpiredPlaybackSessions(ctx, now.Add(-endedRetention).UnixMilli()); err != nil {
		s.log.WarnContext(ctx, "delete ended playback sessions", "err", err)
	}
}

func unauthenticatedOr(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrUnauthenticated
	}
	return fmt.Errorf("%s: %w", what, err)
}
