package playback

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/auth"
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
	// BurnInPath is the absolute path of a sidecar subtitle to burn in (resolved through package
	// paths); embedded subtitles are named by Decision.BurnStream.
	BurnInPath string
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
// empty for the primary file. The limits of the profile are checked here (plan 3.5): a used-up
// daily watch time answers model.WatchLimitError, a profile running its maximum of streams
// model.StreamLimitError. The start position is req.PositionMs, else the profile's position unless
// the title was completed; the HLS output starts at its segment.
func (s *Service) Start(ctx context.Context, session model.Session, itemID string, req model.PlaybackRequest) (model.PlaybackInfo, error) {
	if err := validateRequest(req); err != nil {
		return model.PlaybackInfo{}, err
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
	limits := session.Profile.Effective
	if limits.MaxStreams > 0 {
		// Count and insert under one lock, so two starts of the profile cannot both take the
		// last stream.
		unlock := s.lockProfile(session.Profile.ID)
		defer unlock()
	}
	watch, err := s.admit(ctx, session, item.ID, limits, s.now())
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	tracks, err := s.vis.Tracks(ctx, scope, item.ID, loc.File.ID)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	probe, kfBlob, err := s.storedProbe(ctx, scope, loc)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	dreq, err := trackRequest(req, tracks)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	caps := normalizeCapabilities(req.Capabilities)
	dreq.Caps, dreq.Limits, dreq.Quality = caps, limits, req.Quality

	file := loc.File
	file.MimeType = MimeType(file.Container, file.VideoCodec, file.AudioCodec)
	file.Bitrate, file.HDR = probeDetails(probe)
	src := newSource(file, probe, limits, caps)
	d := Decide(file, probe, dreq)
	var hls *HLSRequest
	if d.Mode != ModeDirect {
		if s.tc == nil || probe == nil {
			s.log.InfoContext(ctx, "hls unavailable, falling back to direct play", "media_file_id", file.ID,
				"transcoder", s.tc != nil, "probed", probe != nil)
			d = directFallback(d)
		} else if hls, d, err = s.prepare(ctx, loc, src, kfBlob, d); err != nil {
			return model.PlaybackInfo{}, err
		}
	}
	var resume int64
	switch p := item.Progress; {
	case req.PositionMs != nil:
		resume = *req.PositionMs
	case p != nil && !p.Completed && p.PositionMs > 0:
		resume = p.PositionMs
	}

	id := ulid.Make().String()
	if hls != nil {
		// The output starts before the row exists, so the row records the final decision (a remux
		// may turn into a transcode, a transcode may run one rung lower); a logout during the
		// admission wait is caught below.
		hls.ID = id
		hls.Resume = time.Duration(resume) * time.Millisecond
		if _, d, err = s.startHLS(ctx, hls, src); err != nil {
			return model.PlaybackInfo{}, err
		}
	}
	now := s.now()
	backend, hardware := "", false
	if d.Mode == ModeTranscode {
		backend = s.tc.Backend()
		hardware = backend != "" && backend != "software"
	}
	device, rowDevice := s.recordDevice(ctx, session, req, caps, now)
	capsJSON, err := json.Marshal(caps)
	if err != nil {
		return model.PlaybackInfo{}, fmt.Errorf("encode capabilities: %w", err)
	}
	err = s.write.InsertPlaybackSession(ctx, appq.InsertPlaybackSessionParams{
		ID: id, SessionID: session.ID, AccountID: session.Account.ID, ProfileID: session.Profile.ID,
		ItemID: item.ID, MediaFileID: file.ID, DeviceID: nullString(rowDevice), Mode: d.Mode,
		Backend: nullString(backend), Reason: nullString(d.Reason), Quality: nullString(d.Quality),
		Width: nullPositive(d.Width), Height: nullPositive(d.Height), Bitrate: nullPositive(d.Bitrate),
		Hardware: boolInt(hardware), AudioTrack: sql.NullInt64{Int64: int64(d.AudioStream), Valid: d.AudioStream >= 0},
		SubtitleTrack: sql.NullInt64{Int64: d.SubtitleTrack, Valid: d.SubtitleTrack > 0}, SubtitleDelivery: nullString(d.SubtitleDelivery),
		Capabilities: sql.NullString{String: string(capsJSON), Valid: true}, PositionMs: sql.NullInt64{Int64: resume, Valid: true},
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
		Tracks:            tracks,
		Selection:         model.TrackSelection{AudioTrack: d.AudioStream, SubtitleTrack: d.SubtitleTrack, SubtitleDelivery: d.SubtitleDelivery},
		Qualities:         QualityOptions(d, src.w, src.h, limits, caps, req.Quality),
		Chapters:          chapters(probe),
		WatchTime:         watch,
		Device:            device,
	}
	if d.SubtitleDelivery == model.SubtitleText {
		info.SubtitleURL = fmt.Sprintf("/api/v1/playback/sessions/%s/subtitles/%d.vtt?st=%s", id, d.SubtitleTrack, st)
	}
	if hls != nil {
		info.Mode = "hls"
		info.StreamURL = "/api/v1/playback/sessions/" + id + "/master.m3u8?st=" + st
		info.Transcoding = &model.Transcoding{
			Mode: d.Mode, Backend: backend, Hardware: hardware, Quality: d.Quality, Width: d.Width, Height: d.Height,
			Bitrate: d.Bitrate, Tonemap: d.Tonemap, Reason: d.Reason,
		}
	}
	if info.Segments, err = s.vis.Segments(ctx, scope, item.ID); err != nil {
		return model.PlaybackInfo{}, err
	}
	if info.NextEpisode, err = s.vis.NextEpisode(ctx, scope, item.ID); err != nil {
		return model.PlaybackInfo{}, err
	}
	s.log.InfoContext(ctx, "playback started", "playback_id", id, "item_id", item.ID, "media_file_id", file.ID,
		"mode", d.Mode, "backend", backend, "reason", d.Reason, "quality", d.Quality, "audio_track", d.AudioStream,
		"subtitle_track", d.SubtitleTrack, "subtitle_delivery", d.SubtitleDelivery)
	return info, nil
}

// validateRequest checks the fields of a start request that need no lookup.
func validateRequest(req model.PlaybackRequest) error {
	var verr *model.ValidationError
	if len(req.DeviceID) > maxClientIDLength {
		verr = verr.Add("deviceId", "too_long")
	}
	if !validQuality(req.Quality) {
		verr = verr.Add("quality", "invalid_value")
	}
	switch req.SubtitleDelivery {
	case "", model.SubtitleAuto, model.SubtitleText, model.SubtitleBurn:
	default:
		verr = verr.Add("subtitleDelivery", "invalid_value")
	}
	if req.AudioTrack != nil && *req.AudioTrack < 0 {
		verr = verr.Add("audioTrack", "too_small")
	}
	if req.SubtitleTrack < 0 {
		verr = verr.Add("subtitleTrack", "too_small")
	}
	if req.PositionMs != nil && *req.PositionMs < 0 {
		verr = verr.Add("positionMs", "too_small")
	}
	if verr != nil {
		return verr
	}
	return nil
}

// trackRequest checks the wanted tracks against the tracks of the file: audioTrack must be the
// stream index of one of its audio tracks, subtitleTrack the id of one of its subtitle tracks,
// and a text delivery needs a text track (422 unknown_track, not_text).
func trackRequest(req model.PlaybackRequest, tracks model.TrackList) (Request, error) {
	out := Request{AudioStream: -1, Delivery: req.SubtitleDelivery}
	var verr *model.ValidationError
	if req.AudioTrack != nil {
		i := slices.IndexFunc(tracks.Audio, func(t model.Track) bool { return t.Index == *req.AudioTrack })
		if i < 0 {
			verr = verr.Add("audioTrack", "unknown_track")
		} else {
			out.AudioStream = tracks.Audio[i].Index
		}
	}
	if req.SubtitleTrack > 0 {
		i := slices.IndexFunc(tracks.Subtitles, func(t model.Track) bool { return t.ID == req.SubtitleTrack })
		switch {
		case i < 0:
			verr = verr.Add("subtitleTrack", "unknown_track")
		case req.SubtitleDelivery == model.SubtitleText && !tracks.Subtitles[i].Text:
			verr = verr.Add("subtitleDelivery", "not_text")
		default:
			out.Subtitle = &tracks.Subtitles[i]
		}
	}
	if verr != nil {
		return Request{}, verr
	}
	return out, nil
}

// directFallback is the decision when HLS is not possible (no transcoder, file not probed): direct
// play of the default audio track; a text subtitle stays, a burn-in is dropped.
func directFallback(d Decision) Decision {
	out := Decision{Mode: ModeDirect, AudioStream: -1, BurnStream: -1, Width: d.Width, Height: d.Height}
	if d.SubtitleDelivery == model.SubtitleText {
		out.SubtitleTrack, out.SubtitleDelivery = d.SubtitleTrack, d.SubtitleDelivery
	}
	return out
}

// source is what the fallbacks after Decide need about the file and the client: a remux whose
// keyframe index is missing and a transcode the budget lowers are decided again from it.
type source struct {
	file   model.MediaFile
	probe  *transcode.Probe
	w, h   int
	limits model.PlaybackLimits
	caps   model.PlaybackCapabilities
}

func newSource(file model.MediaFile, probe *transcode.Probe, limits model.PlaybackLimits, caps model.PlaybackCapabilities) source {
	src := source{file: file, probe: probe, w: file.Width, h: file.Height, limits: limits, caps: caps}
	if v := mainVideo(probe); v != nil && v.Height > 0 {
		src.w, src.h = v.Width, v.Height
	}
	return src
}

// transcode turns d into a transcode on its rung (a transcode) or on the highest rung the source
// and the limits allow (a remux), tonemapped for HDR sources; AAC audio stays copied.
func (src source) transcode(d Decision, reason string) Decision {
	audio := strings.ToLower(src.file.AudioCodec)
	if a := audioStream(src.probe, d.AudioStream); a != nil {
		audio = strings.ToLower(a.Codec)
	}
	from := sourceTop(src.h)
	if i := rungIndex(d.Quality); i >= 0 {
		from = i
	}
	audioCopy := audio == "aac" && slices.Contains(src.caps.Audio, audio)
	return transcodeOn(d, pickRung(from, src.limits, deviceCap(src.caps)), reason, isHDR(mainVideo(src.probe)), audioCopy, src.w, src.h, src.limits)
}

// startHLS starts the output of req. A remux the transcoder rejects for a missing keyframe index
// (a safety net behind prepare) is transcoded instead and the index job is queued at top priority.
// An exhausted budget lowers an SDR transcode above 720p once to the 720p rung (plan 3.5,
// budget_lowered); HDR sources and lower rungs answer model.ErrTranscodeBusy. It returns the output
// and the decision that runs.
func (s *Service) startHLS(ctx context.Context, req *HLSRequest, src source) (HLSSession, Decision, error) {
	hs, err := s.tc.Start(ctx, *req)
	if errors.Is(err, ErrNoKeyframes) && req.Decision.Mode == ModeRemux {
		s.queueKeyframes(ctx, src.file.ID)
		req.Decision = src.transcode(req.Decision, ReasonKeyframesPending)
		req.Keyframes = nil
		hs, err = s.tc.Start(ctx, *req)
	}
	if errors.Is(err, model.ErrTranscodeBusy) {
		if low, ok := lowered(req.Decision, src.w, src.h, src.limits); ok {
			s.log.InfoContext(ctx, "transcode budget exhausted, trying one rung lower", "playback_id", req.ID,
				"from", req.Decision.Quality, "to", low.Quality)
			req.Decision = low
			hs, err = s.tc.Start(ctx, *req)
		}
	}
	return hs, req.Decision, err
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

// probeDetails returns the container bit rate and the HDR format of the main video stream.
func probeDetails(p *transcode.Probe) (int64, string) {
	if p == nil {
		return 0, ""
	}
	hdr := ""
	if v := mainVideo(p); v != nil {
		hdr = v.HDRFormat()
	}
	return p.Format.BitRate, hdr
}

// chapters maps the chapter marks of the probe.
func chapters(p *transcode.Probe) []model.Chapter {
	out := []model.Chapter{}
	if p == nil {
		return out
	}
	for _, c := range p.Chapters {
		out = append(out, model.Chapter{StartMs: c.Start.Milliseconds(), EndMs: c.End.Milliseconds(), Title: c.Title})
	}
	return out
}

// sourcePath resolves the absolute path of a media file through package paths.
func sourcePath(loc visibility.MediaFileLocation) (string, error) {
	return rootPath(loc.RootPath, loc.RelPath, loc.File.ID)
}

// rootPath resolves rel below a library root through package paths.
func rootPath(rootDir, rel, what string) (string, error) {
	root, err := paths.NewRoot(rootDir)
	if err != nil {
		return "", err
	}
	abs, err := paths.Resolve(root, rel)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", what, err)
	}
	return abs, nil
}

// prepare resolves the source (and a sidecar subtitle to burn in) and, for a remux, the keyframe
// index of an HLS request. A remux without index is transcoded (ensureKeyframes).
func (s *Service) prepare(ctx context.Context, loc visibility.MediaFileLocation, src source, kfBlob []byte, d Decision) (*HLSRequest, Decision, error) {
	abs, err := sourcePath(loc)
	if err != nil {
		return nil, Decision{}, err
	}
	req := &HLSRequest{Path: abs, Probe: *src.probe, Decision: d}
	if d.SubtitleDelivery == model.SubtitleBurn && d.BurnSidecar != "" {
		if req.BurnInPath, err = rootPath(loc.RootPath, d.BurnSidecar, "sidecar subtitle"); err != nil {
			return nil, Decision{}, err
		}
	}
	if d.Mode == ModeRemux {
		if req.Keyframes = s.ensureKeyframes(ctx, loc.File.ID, abs, kfBlob); req.Keyframes == nil {
			req.Decision = src.transcode(d, ReasonKeyframesPending)
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
	account := model.Account{ID: acc.ID, Email: acc.Email, Role: acc.Role, MaxRating: int(acc.MaxRating), AllLibraries: acc.AllLibraries == 1,
		Limits: auth.AccountLimits(acc)}
	limits := auth.ProfileLimits(prof)
	g := model.PlaybackGrant{
		ID: row.ID,
		Session: model.Session{
			ID:      row.SessionID,
			Account: account,
			Profile: model.Profile{
				ID: prof.ID, AccountID: prof.AccountID, Name: prof.Name, MaxRating: int(prof.MaxRating),
				Language: prof.Language, AllLibraries: prof.AllLibraries == 1, Accent: prof.Accent.String,
				HasPin: prof.PinHash.Valid, IsPrimary: prof.IsDefault == 1, Limits: limits, Effective: limits.Cap(account.Limits),
			},
		},
		ItemID:      row.ItemID,
		MediaFileID: row.MediaFileID,
		Mode:        row.Mode,
	}
	if row.SubtitleDelivery.String == model.SubtitleText {
		g.SubtitleTrack = row.SubtitleTrack.Int64
	}
	return g, nil
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
// decision is made again from the recorded columns (M4 design 3.7): the reported capabilities,
// the quality rung, the audio track and the subtitle track with its delivery, under the profile's
// current limits; a recorded transcode stays a transcode on its rung.
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
	row, err := s.read.GetPlaybackSession(ctx, appq.GetPlaybackSessionParams{ID: g.ID, Now: s.now().UnixMilli()})
	if err != nil {
		return nil, unauthenticatedOr(err, "get playback session")
	}
	src, d, err := s.recordedDecision(ctx, scope, loc, probe, row, g.Session.Profile.Effective)
	if err != nil {
		return nil, err
	}
	req, _, err := s.prepare(ctx, loc, src, kfBlob, d)
	if err != nil {
		return nil, err
	}
	req.ID = g.ID
	req.StartSegment = segment
	hs, _, err := s.startHLS(ctx, req, src)
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

// recordedDecision decides an HLS playback again from its row: the stored capabilities (the
// defaults for a row without), quality rung, audio track and subtitle track with its delivery. A
// recorded transcode stays one; a decision that turns out direct (it cannot, with the same input)
// is transcoded, because the client fetches HLS.
func (s *Service) recordedDecision(ctx context.Context, scope visibility.Scope, loc visibility.MediaFileLocation, probe *transcode.Probe,
	row appq.PlaybackSession, limits model.PlaybackLimits,
) (source, Decision, error) {
	var reported *model.PlaybackCapabilities
	if row.Capabilities.Valid {
		var c model.PlaybackCapabilities
		if err := json.Unmarshal([]byte(row.Capabilities.String), &c); err == nil {
			reported = &c
		}
	}
	caps := normalizeCapabilities(reported)
	req := Request{Caps: caps, Limits: limits, AudioStream: -1, Quality: row.Quality.String, Delivery: row.SubtitleDelivery.String,
		ForceTranscode: row.Mode == ModeTranscode}
	if row.AudioTrack.Valid {
		req.AudioStream = int(row.AudioTrack.Int64)
	}
	if row.SubtitleTrack.Valid {
		tracks, err := s.vis.Tracks(ctx, scope, loc.ItemID, loc.File.ID)
		if err != nil {
			return source{}, Decision{}, err
		}
		if i := slices.IndexFunc(tracks.Subtitles, func(t model.Track) bool { return t.ID == row.SubtitleTrack.Int64 }); i >= 0 {
			req.Subtitle = &tracks.Subtitles[i]
		}
	}
	src := newSource(loc.File, probe, limits, caps)
	d := Decide(loc.File, probe, req)
	if d.Mode == ModeDirect {
		d = src.transcode(d, row.Reason.String)
	}
	return src, d, nil
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
		SessionID: sessionID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(model.StoppedByLogout),
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
		AccountID: accountID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(model.StoppedByRevoked),
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
	return s.endProfile(ctx, profileID, model.StoppedByRevoked)
}

// endProfile ends the playback of a profile; stoppedBy is revoked (profile deleted) or limit (K-5).
func (s *Service) endProfile(ctx context.Context, profileID int64, stoppedBy string) error {
	ids, err := s.write.RevokePlaybackSessionsByProfile(ctx, appq.RevokePlaybackSessionsByProfileParams{
		ProfileID: profileID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(stoppedBy),
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

// end closes the open streams of an ended playback session, stops its HLS output and removes its
// cached subtitles.
func (s *Service) end(id string) {
	s.cancelStreams(id)
	s.stopOutput(id)
	s.removeSubtitles(id)
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
		s.removeSubtitles(id)
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
	s.sweepSubtitles(time.Now()) // file times are wall-clock times
}

func unauthenticatedOr(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrUnauthenticated
	}
	return fmt.Errorf("%s: %w", what, err)
}
