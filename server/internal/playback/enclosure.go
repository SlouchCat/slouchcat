package playback

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Podcast episodes without a library file stream through the server from their enclosure URL
// (M6, design 3.4): the same stream token and visibility rules as every other stream, the
// client's address never reaches the podcast host, and the download job reuses the fetch.
// startEnclosure records a direct playback session with media_file_id "" (the column has no
// foreign key; the empty string means "no file"); ProxyEnclosure answers the stream endpoint for
// such a session with a Range pass-through to the enclosure.

// Limits of the enclosure fetch: the response headers within 30 s, at most 5 redirects, http and
// https only; the body has no limit (an episode may be long).
const (
	enclosureHeaderTimeout = 30 * time.Second
	maxEnclosureRedirects  = 5
)

// defaultEnclosureType is the MIME type of a playback when the feed names none.
const defaultEnclosureType = "audio/mpeg"

// errEnclosureRedirect stops a redirect chain that is too long or leaves http and https.
var errEnclosureRedirect = errors.New("enclosure redirects too often or to another scheme")

// enclosureClients fetch enclosures for the stream proxy: header timeout 30 s, at most 5
// redirects, http and https only, no address in the local network unless the podcast's feed is
// there itself (package netguard).
var enclosureClients = netguard.NewClients(http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > maxEnclosureRedirects || !httpScheme(req.URL) {
			return errEnclosureRedirect
		}
		return nil
	},
}, enclosureHeaderTimeout)

func httpScheme(u *url.URL) bool { return u.Scheme == "http" || u.Scheme == "https" }

// startEnclosure starts the playback of a podcast episode from its enclosure: the limits of the
// profile apply like for any start, the resume position is the profile's, and the session is a
// direct playback without media file whose stream URL is the proxy (MediaKind audio, duration
// from items.duration_ms). An episode without enclosure is model.ErrNotFound.
func (s *Service) startEnclosure(ctx context.Context, session model.Session, item model.ItemSummary, req model.PlaybackRequest) (model.PlaybackInfo, error) {
	enc, err := s.vis.Enclosure(ctx, visibility.Profile(session.Profile), item.ID)
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	limits := session.Profile.Effective
	watch, slot, err := s.admit(ctx, session, item.ID, limits, s.now())
	if err != nil {
		return model.PlaybackInfo{}, err
	}
	defer slot.release()
	var resume int64
	switch p := item.Progress; {
	case req.PositionMs != nil:
		resume = *req.PositionMs
	case p != nil && !p.Completed && p.PositionMs > 0:
		resume = p.PositionMs
	}
	caps := normalizeCapabilities(req.Capabilities)
	now := s.now()
	device, rowDevice := s.recordDevice(ctx, session, req, caps, now)
	capsJSON, err := json.Marshal(caps)
	if err != nil {
		return model.PlaybackInfo{}, fmt.Errorf("encode capabilities: %w", err)
	}
	id := ulid.Make().String()
	err = slot.insert(ctx, func() error {
		return s.write.InsertPlaybackSession(ctx, appq.InsertPlaybackSessionParams{
			ID: id, SessionID: session.ID, AccountID: session.Account.ID, ProfileID: session.Profile.ID,
			ItemID: item.ID, MediaFileID: "", DeviceID: nullString(rowDevice), Mode: ModeDirect,
			Capabilities: sql.NullString{String: string(capsJSON), Valid: true},
			PositionMs:   sql.NullInt64{Int64: resume, Valid: true}, Now: now.UnixMilli(), ExpiresAt: now.Add(SessionLifetime).UnixMilli(),
		})
	})
	if err != nil {
		return model.PlaybackInfo{}, fmt.Errorf("insert playback session: %w", err)
	}
	if limits.WatchLimitMinutes > 0 {
		if err := s.recheckWatchLimit(context.WithoutCancel(ctx), session.Profile.ID, limits.WatchLimitMinutes, id); err != nil {
			return model.PlaybackInfo{}, err
		}
	}
	st := url.QueryEscape(SignToken(s.key, id, item.ID))
	mime := cmp.Or(enc.Type, defaultEnclosureType)
	info := model.PlaybackInfo{
		MediaKind: model.MediaKindAudio,
		Mode:      ModeDirect,
		Item:      item,
		MediaFile: model.MediaFile{
			Container: containerOfType(mime), MimeType: mime, DurationMs: enc.DurationMs, SizeBytes: enc.Length,
			AudioCodec: codecOfType(mime), Available: true,
		},
		StreamURL:         "/api/v1/items/" + url.PathEscape(item.ID) + "/stream?st=" + st,
		PlaybackSessionID: id,
		ResumePositionMs:  resume,
		Tracks:            model.TrackList{Audio: []model.Track{}, Subtitles: []model.Track{}},
		Selection:         model.TrackSelection{AudioTrack: -1},
		Qualities:         []model.QualityOption{},
		Chapters:          []model.Chapter{},
		WatchTime:         watch,
		Device:            device,
	}
	s.log.InfoContext(ctx, "playback started", "playback_id", id, "item_id", item.ID, "mode", ModeDirect, "source", "enclosure")
	return info, nil
}

// containerOfType maps the MIME type of an enclosure to a container name.
func containerOfType(mime string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0])) {
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/mp4", "audio/x-m4a", "audio/aac", "video/mp4":
		return "m4a"
	case "audio/ogg", "audio/opus":
		return "ogg"
	case "audio/flac", "audio/x-flac":
		return "flac"
	}
	return "unknown"
}

// codecOfType is the audio codec an enclosure of the MIME type usually carries ("" when unknown).
func codecOfType(mime string) string {
	switch containerOfType(mime) {
	case "mp3":
		return "mp3"
	case "m4a":
		return "aac"
	case "flac":
		return "flac"
	}
	return ""
}

// ProxyEnclosure answers the stream endpoint for a playback session without media file: after
// re-checking visibility it fetches the enclosure with the client's Range header and copies status
// (200 or 206; 416 for an unsatisfiable range), Content-Length, Content-Range and Accept-Ranges,
// with Cache-Control private, no-store. The Content-Type is an audio type or video/mp4 only
// (streamType), and nosniff with a sandboxing Content-Security-Policy keeps whatever the podcast
// host sends from running as a page on the server's origin. The response ends when the playback
// session is revoked. Errors before the response started: model.ErrNotFound (the title is not
// visible, the grant is no direct session), model.ErrEnclosureUnavailable (the podcast host failed,
// or the enclosure lies in the local network while the feed does not); later failures only end the
// response.
func (s *Service) ProxyEnclosure(ctx context.Context, g model.PlaybackGrant, w http.ResponseWriter, r *http.Request) error {
	if g.Mode != ModeDirect || g.MediaFileID != "" {
		return model.ErrNotFound
	}
	enc, err := s.vis.Enclosure(ctx, visibility.Profile(g.Session.Profile), g.ItemID)
	if err != nil {
		return err
	}
	u, err := url.Parse(enc.URL)
	if err != nil || !httpScheme(u) || u.Host == "" {
		return model.ErrEnclosureUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return model.ErrEnclosureUnavailable
	}
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}
	req.Header.Set("User-Agent", "Slouchcat")
	// The client's byte ranges count the stored representation: no transparent gzip.
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := enclosureClients.For(ctx, enc.FeedURL).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.log.InfoContext(ctx, "enclosure fetch failed", "playback_id", g.ID, "item_id", g.ItemID, "err", netguard.Clean(err))
		return model.ErrEnclosureUnavailable
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent, http.StatusRequestedRangeNotSatisfiable:
	default:
		_ = resp.Body.Close()
		s.log.InfoContext(ctx, "enclosure host answered an error", "playback_id", g.ID, "item_id", g.ItemID, "status", resp.StatusCode)
		return model.ErrEnclosureUnavailable
	}
	body := s.track(ctx, g, readOnly{resp.Body})
	defer body.Close()
	h := w.Header()
	h.Set("Content-Type", streamType(resp.Header.Get("Content-Type"), enc.Type))
	for _, k := range []string{"Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Content-Disposition", "inline")
	w.WriteHeader(resp.StatusCode)
	if rv, ok := body.(interface{ Done() <-chan struct{} }); ok {
		stop := abortWrites(w, rv.Done())
		defer stop()
	}
	if _, err := io.Copy(w, body); err != nil && ctx.Err() == nil {
		s.log.InfoContext(ctx, "enclosure stream ended", "playback_id", g.ID, "err", err)
	}
	return nil
}

// streamType is the Content-Type of a proxied enclosure: the podcast host's when it is an audio
// type or video/mp4, else the feed's when that is, else application/octet-stream. Any other type
// (text/html, image/svg+xml) could run as a page on the server's origin.
func streamType(upstream, feed string) string {
	for _, t := range []string{upstream, feed} {
		if mt, _, err := mime.ParseMediaType(t); err == nil && (strings.HasPrefix(mt, "audio/") || mt == "video/mp4") {
			return mt
		}
	}
	return "application/octet-stream"
}

// readOnly is an upstream body as the io.ReadSeekCloser the stream tracking takes; it cannot seek.
type readOnly struct{ io.ReadCloser }

func (readOnly) Seek(int64, int) (int64, error) { return 0, errors.New("enclosure body cannot seek") }

// abortWrites fails the writes of a response once done is closed (a revoked playback with a
// client that stopped reading), like the stream endpoint does for files.
func abortWrites(w http.ResponseWriter, done <-chan struct{}) func() {
	rc := http.NewResponseController(w)
	finished := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-done:
			_ = rc.SetWriteDeadline(time.Now())
		case <-finished:
		}
	}()
	return func() {
		close(finished)
		<-stopped
	}
}
