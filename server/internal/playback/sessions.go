package playback

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
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
// last token request and are revoked on stop and logout; the sweep (Run) stops what they left
// running and removes them a day later.

const (
	// SessionLifetime is the sliding expiry of a playback session after its last token request.
	SessionLifetime = 12 * time.Hour
	// KeyframeTimeout bounds the on-demand keyframe index of Start (about 19 s for a 6.5 GB file
	// over NFS); a longer scan falls back to transcoding while the index job runs.
	KeyframeTimeout = 20 * time.Second
	// touchInterval limits how often token requests write last_activity_at/expires_at.
	touchInterval = time.Minute
	// sweepInterval is the period of the background sweep (Run).
	sweepInterval = time.Minute
	// endedRetention keeps revoked and expired rows for a day before the sweep deletes them.
	endedRetention = 24 * time.Hour
)

// ErrNoKeyframes is returned by Transcoder.Start for a remux without a keyframe index; Start then
// transcodes instead.
var ErrNoKeyframes = errors.New("remux needs a keyframe index")

// HLSRequest is what a Transcoder needs to start the HLS output of a playback session.
type HLSRequest struct {
	ID        string          // playback session id (ULID), also the transcode session id
	Path      string          // absolute path of the media file, resolved through package paths
	Probe     transcode.Probe // stored ffprobe result
	Keyframes []time.Duration // keyframe index; nil only for transcode
	Decision  Decision        // ModeRemux or ModeTranscode
	// Resume is the resume position: the encoder starts at the segment that contains it. Without
	// a position (0) it starts at StartSegment.
	Resume       time.Duration
	StartSegment int
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
	// exhausted and ErrNoKeyframes for a remux without a keyframe index.
	Start(ctx context.Context, req HLSRequest) (HLSSession, error)
	// Get returns the output of a playback session while the transcoder still has it.
	Get(id string) (HLSSession, bool)
	// Stop ends the output and removes its files; unknown ids are ignored.
	Stop(id string)
}

// Start decides how the client plays a visible title, starts its HLS output if needed, records the
// playback session and returns the playback description with the token URL. mediaFileID may be
// empty for the primary file. The resume position is the profile's position unless the title was
// completed; the HLS output starts at its segment.
func (s *Service) Start(ctx context.Context, session model.Session, itemID string, req model.PlaybackRequest) (model.PlaybackInfo, error) {
	if len(req.DeviceID) > maxClientIDLength {
		return model.PlaybackInfo{}, (&model.ValidationError{}).Add("deviceId", "too_long")
	}
	if !session.HasProfile() {
		return model.PlaybackInfo{}, model.ErrNotFound // a playback session always belongs to a profile
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

	file := loc.File
	file.MimeType = MimeType(file.Container, file.VideoCodec, file.AudioCodec)
	hls, decision, err := s.hlsRequest(ctx, scope, loc, normalizeCapabilities(req.Capabilities))
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	var resume int64
	if p := item.Progress; p != nil && !p.Completed && p.PositionMs > 0 {
		resume = p.PositionMs
	}

	id := ulid.Make().String()
	if hls != nil {
		// The output starts before the row exists, so the row records the final decision (a remux
		// may turn into a transcode); a logout during the admission wait is caught below.
		hls.ID = id
		hls.Resume = time.Duration(resume) * time.Millisecond
		if _, decision, err = s.startHLS(ctx, hls, loc.File); err != nil {
			return model.PlaybackInfo{}, err
		}
	}
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
		s.stopOutput(id)
		return model.PlaybackInfo{}, fmt.Errorf("insert playback session: %w", err)
	}
	if hls != nil {
		// The browser session may have ended (logout) while the output waited for admission.
		if _, err := s.read.GetPlaybackSession(ctx, appq.GetPlaybackSessionParams{ID: id, Now: s.now().UnixMilli()}); err != nil {
			s.stopOutput(id)
			return model.PlaybackInfo{}, unauthenticatedOr(err, "recheck playback session")
		}
	}

	st := url.QueryEscape(SignToken(s.key, id, item.ID))
	info := model.PlaybackInfo{
		Mode:              ModeDirect,
		Item:              item,
		MediaFile:         file,
		StreamURL:         "/api/v1/items/" + url.PathEscape(item.ID) + "/stream?st=" + st,
		PlaybackSessionID: id,
		ResumePositionMs:  resume,
	}
	if hls != nil {
		info.Mode = "hls"
		info.StreamURL = "/api/v1/playback/sessions/" + id + "/master.m3u8?st=" + st
		info.Transcoding = &model.Transcoding{Mode: decision.Mode, Backend: backend, Reason: decision.Reason}
	}
	s.log.InfoContext(ctx, "playback started", "playback_id", id, "item_id", item.ID, "media_file_id", file.ID,
		"mode", decision.Mode, "backend", backend, "reason", decision.Reason)
	return info, nil
}

// startHLS starts the output of req. A remux the transcoder rejects for a missing keyframe index
// (a safety net behind hlsRequest) is transcoded instead and the index job is queued at top
// priority. It returns the output and the decision that runs.
func (s *Service) startHLS(ctx context.Context, req *HLSRequest, file model.MediaFile) (HLSSession, Decision, error) {
	hs, err := s.tc.Start(ctx, *req)
	if errors.Is(err, ErrNoKeyframes) && req.Decision.Mode == ModeRemux {
		s.queueKeyframes(ctx, file.ID)
		req.Decision = transcodeFallback(file, isHDR(mainVideo(&req.Probe)))
		req.Keyframes = nil
		hs, err = s.tc.Start(ctx, *req)
	}
	return hs, req.Decision, err
}

// transcodeFallback is the transcode that replaces a remux while the keyframe index is pending.
// Tonemapping follows the source (a Safari HDR passthrough is tonemapped as well).
func transcodeFallback(file model.MediaFile, hdr bool) Decision {
	return Decision{Mode: ModeTranscode, MaxHeight: MaxTranscodeHeight, Tonemap: hdr,
		AudioCopy: strings.EqualFold(file.AudioCodec, "aac"), Reason: ReasonKeyframesPending}
}

// storedProbe reads the stored probe and keyframe blob of a media file; probe is nil when the file
// is not probed yet or the JSON is unreadable.
func (s *Service) storedProbe(ctx context.Context, scope visibility.Scope, loc visibility.MediaFileLocation) (*transcode.Probe, []byte, error) {
	pf, err := s.vis.MediaFileProbe(ctx, scope, loc.ItemID, loc.File.ID)
	if err != nil {
		return nil, nil, err
	}
	if pf.Probe == "" {
		return nil, pf.Keyframes, nil
	}
	var p transcode.Probe
	if err := json.Unmarshal([]byte(pf.Probe), &p); err != nil {
		s.log.WarnContext(ctx, "stored probe unreadable", "media_file_id", loc.File.ID, "err", err)
		return nil, pf.Keyframes, nil
	}
	return &p, pf.Keyframes, nil
}

// sourcePath resolves the absolute path of a media file through package paths.
func sourcePath(loc visibility.MediaFileLocation) (string, error) {
	root, err := paths.NewRoot(loc.RootPath)
	if err != nil {
		return "", err
	}
	abs, err := paths.Resolve(root, loc.RelPath)
	if err != nil {
		return "", fmt.Errorf("resolve media file %s: %w", loc.File.ID, err)
	}
	return abs, nil
}

// hlsRequest decides the mode and, for remux and transcode, prepares the transcoder request. It
// falls back to direct play when no transcoder is configured or the file is not probed yet, so the
// client checks the file itself (the M1a behaviour). A remux needs the keyframe index
// (ensureKeyframes); without it the file is transcoded.
func (s *Service) hlsRequest(ctx context.Context, scope visibility.Scope, loc visibility.MediaFileLocation, caps model.PlaybackCapabilities) (*HLSRequest, Decision, error) {
	probe, kfBlob, err := s.storedProbe(ctx, scope, loc)
	if err != nil {
		return nil, Decision{}, err
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
	return s.prepare(ctx, loc, probe, kfBlob, d)
}

// prepare resolves the source and, for a remux, the keyframe index of an HLS request.
func (s *Service) prepare(ctx context.Context, loc visibility.MediaFileLocation, probe *transcode.Probe, kfBlob []byte, d Decision) (*HLSRequest, Decision, error) {
	abs, err := sourcePath(loc)
	if err != nil {
		return nil, Decision{}, err
	}
	req := &HLSRequest{Path: abs, Probe: *probe, Decision: d}
	if d.Mode == ModeRemux {
		if req.Keyframes = s.ensureKeyframes(ctx, loc.File.ID, abs, kfBlob); req.Keyframes == nil {
			req.Decision = transcodeFallback(loc.File, isHDR(mainVideo(probe)))
		}
	}
	return req, req.Decision, nil
}

// ensureKeyframes returns the keyframe index of a media file for a remux. A missing index is
// computed on demand (bounded by keyframeTimeout) and stored for the next start; when that is not
// possible in time it returns nil and queues the index job at top priority, so the caller
// transcodes this playback and the next start remuxes.
func (s *Service) ensureKeyframes(ctx context.Context, fileID, abs string, blob []byte) []time.Duration {
	if len(blob) > 0 {
		kf, err := library.DecodeKeyframes(blob)
		if err == nil && len(kf) > 0 {
			return kf
		}
		s.log.WarnContext(ctx, "keyframe index unusable", "media_file_id", fileID, "err", err)
	}
	if s.keyframes != nil {
		kctx, cancel := context.WithTimeout(ctx, s.keyframeTimeout)
		started := time.Now()
		kf, err := s.keyframes(kctx, abs)
		cancel()
		if err == nil && len(kf) > 0 {
			if err := library.StoreKeyframes(ctx, s.db.Write, fileID, kf, s.now()); err != nil {
				s.log.WarnContext(ctx, "store keyframe index", "media_file_id", fileID, "err", err)
			}
			s.log.InfoContext(ctx, "keyframe index computed for playback", "media_file_id", fileID,
				"keyframes", len(kf), "ms", time.Since(started).Milliseconds())
			return kf
		}
		s.log.InfoContext(ctx, "keyframe index not ready, transcoding instead of remuxing", "media_file_id", fileID,
			"ms", time.Since(started).Milliseconds(), "err", err)
	}
	s.queueKeyframes(ctx, fileID)
	return nil
}

// queueKeyframes queues the keyframe index job of a media file at top priority.
func (s *Service) queueKeyframes(ctx context.Context, fileID string) {
	if err := library.QueueKeyframesForPlayback(context.WithoutCancel(ctx), s.queue, fileID); err != nil {
		s.log.WarnContext(ctx, "queue keyframe index", "media_file_id", fileID, "err", err)
	}
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
			Account: model.Account{ID: acc.ID, Email: acc.Email, Role: acc.Role, MaxRating: int(acc.MaxRating), AllLibraries: acc.AllLibraries == 1},
			Profile: model.Profile{
				ID: prof.ID, AccountID: prof.AccountID, Name: prof.Name, MaxRating: int(prof.MaxRating),
				Language: prof.Language, AllLibraries: prof.AllLibraries == 1, Accent: prof.Accent.String,
				HasPin: prof.PinHash.Valid, IsPrimary: prof.IsDefault == 1,
			},
		},
		ItemID:      row.ItemID,
		MediaFileID: row.MediaFileID,
		Mode:        row.Mode,
	}, nil
}

// HLS returns the HLS output of a grant after checking that its title is still visible to the
// profile; a title that became invisible and a direct-play grant return model.ErrNotFound. When
// the transcoder no longer has the output (a server restart, or it was stopped as idle), it is
// rebuilt from the grant and starts at segment.
func (s *Service) HLS(ctx context.Context, g model.PlaybackGrant, segment int) (model.HLSStream, error) {
	if err := s.vis.Visible(ctx, visibility.Profile(g.Session.Profile), g.ItemID); err != nil {
		return nil, err
	}
	if g.Mode == ModeDirect || s.tc == nil {
		return nil, model.ErrNotFound
	}
	hs, ok := s.tc.Get(g.ID)
	if !ok {
		var err error
		if hs, err = s.rebuild(ctx, g, segment); err != nil {
			return nil, err
		}
	}
	hs.Touch()
	return hs, nil
}

// rebuild starts the output of a valid HLS grant again, one rebuild per playback at a time. The
// mode is the recorded one; the client's capabilities are not stored, so a remux copies only AAC
// audio and encodes anything else (always playable, at worst a different audio codec than before).
func (s *Service) rebuild(ctx context.Context, g model.PlaybackGrant, segment int) (HLSSession, error) {
	for {
		s.rebuildMu.Lock()
		wait, busy := s.rebuilding[g.ID]
		if !busy {
			s.rebuilding[g.ID] = make(chan struct{})
			s.rebuildMu.Unlock()
			break
		}
		s.rebuildMu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if hs, ok := s.tc.Get(g.ID); ok {
			return hs, nil
		}
	}
	defer func() {
		s.rebuildMu.Lock()
		close(s.rebuilding[g.ID])
		delete(s.rebuilding, g.ID)
		s.rebuildMu.Unlock()
	}()
	if hs, ok := s.tc.Get(g.ID); ok {
		return hs, nil
	}

	scope := visibility.Profile(g.Session.Profile)
	loc, err := s.vis.MediaFile(ctx, scope, g.ItemID, g.MediaFileID)
	if err != nil {
		return nil, err
	}
	if !loc.File.Available {
		return nil, model.ErrNotFound
	}
	probe, kfBlob, err := s.storedProbe(ctx, scope, loc)
	if err != nil {
		return nil, err
	}
	if probe == nil {
		return nil, model.ErrNotFound
	}
	req, _, err := s.prepare(ctx, loc, probe, kfBlob, recordedDecision(loc.File, probe, g.Mode))
	if err != nil {
		return nil, err
	}
	req.ID = g.ID
	req.StartSegment = segment
	hs, _, err := s.startHLS(ctx, req, loc.File)
	if err != nil {
		return nil, err
	}
	// The playback may have been stopped while the output waited for admission.
	if _, err := s.read.GetPlaybackSession(ctx, appq.GetPlaybackSessionParams{ID: g.ID, Now: s.now().UnixMilli()}); err != nil {
		s.stopOutput(g.ID)
		return nil, unauthenticatedOr(err, "recheck playback session")
	}
	s.log.InfoContext(ctx, "hls output rebuilt", "playback_id", g.ID, "mode", req.Decision.Mode, "segment", segment)
	return hs, nil
}

// recordedDecision rebuilds the decision of a recorded HLS mode from the stored probe.
// Tonemapping is requested for every transcode; transcode.NewPlan drops it for SDR sources.
func recordedDecision(file model.MediaFile, probe *transcode.Probe, mode string) Decision {
	audio := strings.ToLower(file.AudioCodec)
	if mode == ModeRemux {
		d := Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: audio == "" || audio == "aac"}
		if strings.EqualFold(file.VideoCodec, "hevc") {
			d.VideoTag = "hvc1"
		}
		return d
	}
	return Decision{Mode: ModeTranscode, MaxHeight: MaxTranscodeHeight, Tonemap: isHDR(mainVideo(probe)), AudioCopy: audio == "aac"}
}

// OpenGrant opens the media file of a direct-play grant for the Range endpoint; HLS grants and
// titles that are no longer visible return model.ErrNotFound. The returned file ends the stream
// when the playback session is revoked (see grantFile).
func (s *Service) OpenGrant(ctx context.Context, g model.PlaybackGrant) (model.StreamSource, error) {
	if g.Mode != ModeDirect {
		return model.StreamSource{}, model.ErrNotFound
	}
	src, err := s.Open(ctx, g.Session.Profile, g.ItemID, g.MediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	src.File = s.track(ctx, g, src.File)
	return src, nil
}

// Stop revokes a playback session of the browser session, ends its open streams and stops its HLS
// output. A playback session of another browser session, an unknown and an already ended one
// return model.ErrNotFound.
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
	s.end(id)
	return nil
}

// EndSession revokes every playback session of a browser session (logout), ends their open
// streams and stops their HLS output.
func (s *Service) EndSession(ctx context.Context, sessionID int64) error {
	ids, err := s.write.RevokePlaybackSessionsBySession(ctx, appq.RevokePlaybackSessionsBySessionParams{
		SessionID: sessionID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke playback sessions: %w", err)
	}
	for _, id := range ids {
		s.end(id)
	}
	return nil
}

// EndAccount revokes every playback session of an account (guest revocation, K-7), ends their
// open streams and stops their HLS output. The browser sessions are revoked by the caller; the
// stream tokens answer 401 from the next request on either way.
func (s *Service) EndAccount(ctx context.Context, accountID int64) error {
	ids, err := s.write.RevokePlaybackSessionsByAccount(ctx, appq.RevokePlaybackSessionsByAccountParams{
		AccountID: accountID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke playback sessions of account: %w", err)
	}
	for _, id := range ids {
		s.end(id)
	}
	s.cancelAccount(accountID) // streams whose row was revoked earlier or is not yet written
	return nil
}

// EndProfile revokes every playback session of a profile (profile deleted), ends their open
// streams and stops their HLS output.
func (s *Service) EndProfile(ctx context.Context, profileID int64) error {
	ids, err := s.write.RevokePlaybackSessionsByProfile(ctx, appq.RevokePlaybackSessionsByProfileParams{
		ProfileID: profileID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke playback sessions of profile: %w", err)
	}
	for _, id := range ids {
		s.end(id)
	}
	s.cancelProfile(profileID)
	return nil
}

// end closes the open streams of an ended playback session and stops its HLS output.
func (s *Service) end(id string) {
	s.cancelStreams(id)
	s.stopOutput(id)
}

func (s *Service) stopOutput(id string) {
	if s.tc != nil {
		s.tc.Stop(id)
	}
}

// Run sweeps ended playback sessions every minute until ctx ends (started by the server next to
// the job workers, never inside a request).
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		s.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sweep ends what revoked and expired playback sessions left behind, including those whose
// browser session ended (a client that never sent its stop, a revoked login): open streams and
// HLS output. It deletes rows that ended more than a day ago.
func (s *Service) sweep(ctx context.Context) {
	now := s.now()
	ids, err := s.read.ListEndedPlaybackSessions(ctx, now.UnixMilli())
	if err != nil {
		s.log.WarnContext(ctx, "list ended playback sessions", "err", err)
	}
	for _, id := range ids {
		s.cancelStreams(id)
		if s.tc == nil {
			continue
		}
		if _, running := s.tc.Get(id); running {
			s.tc.Stop(id)
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
