// Package playback decides how a title is played and records playback progress (plan 3.4 steps 6
// and 7, W-1, W-3, W-5). Start decides between direct play, remux and transcode from the stored
// probe and the client's capabilities (decide.go), records a playback session and returns URLs
// with the stream token (sessions.go, token.go, plan 5.4). HLS output comes from a Transcoder
// (package transcode through transcoder.go); direct play never starts ffmpeg (W-1).
//
// Every read of a title goes through the visibility package, so a title the profile may not see
// answers model.ErrNotFound for playback, streaming and progress alike. Files are opened only
// through the paths package (os.Root), relative to their library root. Progress lives in the user
// tables progress and progress_events (store/appq) and always hangs on the item, never on the
// media file, so renames and version changes keep it.
package playback

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// maxClientIDLength caps device and media file ids stored with progress events.
const maxClientIDLength = 64

// Service implements the playback service of the API.
type Service struct {
	db    *store.DB
	read  *appq.Queries
	write *appq.Queries
	vis   *visibility.Reader
	log   *slog.Logger
	now   func() time.Time
	key   []byte     // HMAC key of the stream tokens
	tc    Transcoder // nil: no HLS, every title plays directly
	queue *jobs.Queue

	// keyframes computes a missing keyframe index before a remux (nil: never on demand; the
	// file is transcoded until the index job ran), bounded by keyframeTimeout.
	keyframes       func(ctx context.Context, path string) ([]time.Duration, error)
	keyframeTimeout time.Duration
	// grantRecheck is how often an open direct-play stream re-reads its playback session.
	grantRecheck time.Duration

	streamsMu sync.Mutex
	streams   map[string]map[*grantFile]struct{} // open direct-play streams per playback id

	rebuildMu  sync.Mutex
	rebuilding map[string]chan struct{} // HLS outputs being rebuilt, closed when done

	subs       SubtitleConverter // WebVTT conversion; zero = no text subtitles
	cacheDir   string            // converted subtitles below <cacheDir>/subtitles; empty = uncached
	subMu      sync.Mutex
	subFlights map[string]*subtitleFlight // running conversions per playback id and track
	subSlots   chan struct{}              // maxSubtitleConversions slots
	starts     startLocks                 // stream counts of starts under a stream limit
}

// Option configures New.
type Option func(*Service)

// WithClock replaces the clock used for progress timestamps (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithTokenKey sets the HMAC key of the stream tokens (secrets.Keyring.Derive("stream-token")).
// Without it New draws a random key, so tokens do not survive a restart.
func WithTokenKey(key []byte) Option {
	return func(s *Service) { s.key = key }
}

// WithTranscoder enables HLS playback (remux and transcode) through t.
func WithTranscoder(t Transcoder) Option {
	return func(s *Service) { s.tc = t }
}

// WithKeyframeIndex lets Start compute a missing keyframe index before a remux
// (transcode.Runner.Keyframes); the index is stored for the next start.
func WithKeyframeIndex(f func(ctx context.Context, path string) ([]time.Duration, error)) Option {
	return func(s *Service) { s.keyframes = f }
}

// New returns the playback service; log is the redacting logger. Run sweeps ended playback
// sessions in the background.
func New(db *store.DB, log *slog.Logger, opts ...Option) *Service {
	s := &Service{
		db: db, read: appq.New(db.Read), write: appq.New(db.Write), vis: visibility.New(db, log), log: log, now: time.Now,
		keyframeTimeout: KeyframeTimeout, grantRecheck: grantRecheckInterval,
		streams: map[string]map[*grantFile]struct{}{}, rebuilding: map[string]chan struct{}{},
		subFlights: map[string]*subtitleFlight{}, subSlots: make(chan struct{}, maxSubtitleConversions),
	}
	for _, o := range opts {
		o(s)
	}
	s.queue = jobs.NewQueue(db.Write, func() time.Time { return s.now() })
	if len(s.key) == 0 {
		s.key = make([]byte, macLength)
		_, _ = rand.Read(s.key) // crypto/rand.Read never fails
	}
	return s
}

// MimeType returns the media type of a file for the client's canPlayType check. MP4 files with
// H.264 video and AAC audio carry a codecs parameter.
//
// Simplification for M1a: the parameter names H.264 High profile level 3.1 and AAC-LC for every
// such file instead of the exact profile and level of the stream; the exact strings follow with
// the device capability profiles in M4.
func MimeType(container, videoCodec, audioCodec string) string {
	base := containerType(container)
	if base == "video/mp4" && strings.EqualFold(videoCodec, "h264") && strings.EqualFold(audioCodec, "aac") {
		return base + `; codecs="avc1.64001F, mp4a.40.2"`
	}
	return base
}

func containerType(container string) string {
	switch strings.ToLower(container) {
	case "mp4", "m4v", "mov":
		return "video/mp4"
	case "mkv":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

// Open opens a media file of a visible title for direct play. The caller closes the file. A file
// that is marked missing or no longer exists on disk returns model.ErrNotFound.
func (s *Service) Open(ctx context.Context, profile model.Profile, itemID, mediaFileID string) (model.StreamSource, error) {
	loc, err := s.vis.MediaFile(ctx, visibility.Profile(profile), itemID, mediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	if !loc.File.Available {
		return model.StreamSource{}, model.ErrNotFound
	}
	root, err := paths.NewRoot(loc.RootPath)
	if err != nil {
		return model.StreamSource{}, err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return model.StreamSource{}, notFoundIfMissing(err)
	}
	defer dir.Close() // files opened through the root stay usable after it is closed
	f, err := dir.Open(loc.RelPath)
	if err != nil {
		return model.StreamSource{}, notFoundIfMissing(err)
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat media file %s: %w", loc.File.ID, err), f.Close())
	}
	if info.IsDir() {
		return model.StreamSource{}, errors.Join(model.ErrNotFound, f.Close())
	}
	return model.StreamSource{
		File:      f,
		MediaType: containerType(loc.File.Container),
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Name:      loc.File.ID,
	}, nil
}

func notFoundIfMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return model.ErrNotFound
	}
	return err
}

// Completed reports whether a position completes a title: at or beyond 90 % of the duration. The
// rule applies to every event, so a later event before the 90 % mark reopens the title (AN-10).
func Completed(positionMs, durationMs int64) bool {
	return durationMs > 0 && positionMs*10 >= durationMs*9
}

// validate checks a progress event and returns nil or a *model.ValidationError.
func validate(ev model.ProgressEvent) error {
	var verr *model.ValidationError
	switch ev.State {
	case "playing", "paused", "ended":
	default:
		verr = verr.Add("state", "invalid_value")
	}
	if ev.PositionMs < 0 {
		verr = verr.Add("positionMs", "too_small")
	}
	if ev.DurationMs <= 0 {
		verr = verr.Add("durationMs", "too_small")
	}
	if ev.PlayedMs < 0 {
		verr = verr.Add("playedMs", "too_small")
	}
	if ev.Seq < 0 {
		verr = verr.Add("seq", "too_small")
	}
	if len(ev.MediaFileID) > maxClientIDLength {
		verr = verr.Add("mediaFileId", "too_long")
	}
	if len(ev.DeviceID) > maxClientIDLength {
		verr = verr.Add("deviceId", "too_long")
	}
	if ev.PlaybackSessionID != "" && !playbackID.MatchString(ev.PlaybackSessionID) {
		verr = verr.Add("playbackSessionId", "invalid_value")
	}
	if verr != nil {
		return verr
	}
	return nil
}

// Report stores a progress event of a visible title and returns the merged progress. M1a merges
// by taking the latest event; offline merging follows in M5. Every event is stored, but a late one
// does not move the merged position (see late); an exact duplicate (same playback session, device,
// media file, seq, client time, position and state) is not stored again, and the played time counts at most the
// server time since the previous event of the playback (boundPlayed). An event that names its playback session records
// the position and the activity of that session (W-6). For a profile with a daily limit (K-5) the
// answer carries the watch time of today; once it is used up, every playback session of the
// profile is revoked (stopped_by limit) and the answer says 0 minutes remain.
func (s *Service) Report(ctx context.Context, profile model.Profile, itemID string, ev model.ProgressEvent) (model.Progress, error) {
	p, err := s.report(ctx, profile, itemID, ev)
	if err != nil {
		return model.Progress{}, err
	}
	if ev.PlaybackSessionID != "" {
		now := s.now()
		_, err := s.write.RecordPlaybackPosition(ctx, appq.RecordPlaybackPositionParams{
			ID: ev.PlaybackSessionID, ProfileID: profile.ID, PositionMs: sql.NullInt64{Int64: ev.PositionMs, Valid: true},
			Now: now.UnixMilli(), ExpiresAt: now.Add(SessionLifetime).UnixMilli(),
		})
		if err != nil {
			s.log.WarnContext(ctx, "record playback position", "playback_id", ev.PlaybackSessionID, "err", err)
		}
	}
	if p.WatchTime, err = s.checkWatchLimit(ctx, profile); err != nil {
		return model.Progress{}, err
	}
	return p, nil
}

// report validates and stores the event and merges the progress.
func (s *Service) report(ctx context.Context, profile model.Profile, itemID string, ev model.ProgressEvent) (model.Progress, error) {
	if err := validate(ev); err != nil {
		return model.Progress{}, err
	}
	if _, err := s.vis.Summary(ctx, visibility.Profile(profile), itemID); err != nil {
		return model.Progress{}, err
	}
	now := s.now()
	position := min(ev.PositionMs, ev.DurationMs)
	completed := Completed(position, ev.DurationMs)
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.Progress{}, fmt.Errorf("begin progress: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	cur, err := q.GetProgress(ctx, appq.GetProgressParams{ProfileID: profile.ID, ItemID: itemID})
	hasCur := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.Progress{}, fmt.Errorf("get progress: %w", err)
	}
	merged := model.Progress{PositionMs: position, DurationMs: ev.DurationMs, Completed: completed, UpdatedAt: time.UnixMilli(now.UnixMilli()).UTC()}
	if hasCur {
		merged = model.Progress{PositionMs: cur.PositionMs, DurationMs: cur.DurationMs, Completed: cur.Completed == 1, UpdatedAt: time.UnixMilli(cur.UpdatedAt).UTC()}
	}
	dup, err := q.CountDuplicateProgressEvents(ctx, appq.CountDuplicateProgressEventsParams{
		PlaybackSessionID: nullString(ev.PlaybackSessionID), Seq: ev.Seq, ProfileID: profile.ID, ItemID: itemID,
		DeviceID: nullString(ev.DeviceID), MediaFileID: nullString(ev.MediaFileID), ClientTime: ev.ClientTime.UnixMilli(),
		PositionMs: ev.PositionMs, State: ev.State,
	})
	if err != nil {
		return model.Progress{}, fmt.Errorf("check duplicate progress event: %w", err)
	}
	if dup > 0 {
		return merged, nil // stored already; the answer is the merged progress
	}
	isLate := false
	if hasCur {
		if isLate, err = late(ctx, q, profile.ID, itemID, ev, cur, now); err != nil {
			return model.Progress{}, err
		}
	}
	played, err := boundPlayed(ctx, q, profile.ID, itemID, ev, now)
	if err != nil {
		return model.Progress{}, err
	}
	err = q.InsertProgressEvent(ctx, appq.InsertProgressEventParams{
		ProfileID:         profile.ID,
		ItemID:            itemID,
		MediaFileID:       nullString(ev.MediaFileID),
		DeviceID:          nullString(ev.DeviceID),
		PlaybackSessionID: nullString(ev.PlaybackSessionID),
		PositionMs:        ev.PositionMs,
		DurationMs:        ev.DurationMs,
		PlayedMs:          played,
		Seq:               ev.Seq,
		State:             ev.State,
		ClientTime:        ev.ClientTime.UnixMilli(),
		ServerTime:        now.UnixMilli(),
	})
	if err != nil {
		return model.Progress{}, fmt.Errorf("insert progress event: %w", err)
	}
	if isLate {
		if err := tx.Commit(); err != nil {
			return model.Progress{}, fmt.Errorf("commit progress: %w", err)
		}
		return merged, nil
	}
	err = q.UpsertProgress(ctx, appq.UpsertProgressParams{
		ProfileID:  profile.ID,
		ItemID:     itemID,
		PositionMs: position,
		DurationMs: ev.DurationMs,
		Completed:  boolInt(completed),
		Now:        now.UnixMilli(),
	})
	if err != nil {
		return model.Progress{}, fmt.Errorf("upsert progress: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Progress{}, fmt.Errorf("commit progress: %w", err)
	}
	return model.Progress{PositionMs: position, DurationMs: ev.DurationMs, Completed: completed, UpdatedAt: time.UnixMilli(now.UnixMilli()).UTC()}, nil
}

// playedSlack is the tolerance of boundPlayed on the server time between two events.
const playedSlack = 10 // percent

// boundPlayed returns the played time of ev the watch time counts (K-5): at most the server time
// since the previous event of the same playback (playback session, device and media file), or
// since the start of its playback session for the first event, plus playedSlack percent. A client
// cannot use up a daily limit with one event, and events that a retry delivers late count no more
// than the time that passed. An event without a reference (no previous event, no known playback
// session) counts as reported.
func boundPlayed(ctx context.Context, q *appq.Queries, profileID int64, itemID string, ev model.ProgressEvent, now time.Time) (int64, error) {
	last, err := q.GetLastPlaybackEventTime(ctx, appq.GetLastPlaybackEventTimeParams{
		PlaybackSessionID: nullString(ev.PlaybackSessionID), ProfileID: profileID, ItemID: itemID,
		DeviceID: nullString(ev.DeviceID), MediaFileID: nullString(ev.MediaFileID),
	})
	if err != nil {
		return 0, fmt.Errorf("get last progress event: %w", err)
	}
	if last == 0 && ev.PlaybackSessionID != "" {
		started, err := q.GetPlaybackSessionStart(ctx, appq.GetPlaybackSessionStartParams{ID: ev.PlaybackSessionID, ProfileID: profileID})
		switch {
		case err == nil:
			last = started
		case !errors.Is(err, sql.ErrNoRows):
			return 0, fmt.Errorf("get playback session start: %w", err)
		}
	}
	if last == 0 {
		return ev.PlayedMs, nil
	}
	elapsed := max(now.UnixMilli()-last, 0)
	return min(ev.PlayedMs, elapsed+elapsed*playedSlack/100), nil
}

// late reports whether ev must not replace the merged progress cur: the merged position is newer
// than the server time of ev, or ev belongs to a playback of the same device and media file whose
// stored newest event has a higher or equal seq at a client time not before that of ev (a request
// that overtook another). The client restarts seq with every playback, so a lower seq with a later
// client time is a new playback and counts. Events without a device id always count.
func late(ctx context.Context, q *appq.Queries, profileID int64, itemID string, ev model.ProgressEvent, cur appq.Progress, now time.Time) (bool, error) {
	if cur.UpdatedAt > now.UnixMilli() {
		return true, nil
	}
	if ev.DeviceID == "" {
		return false, nil
	}
	last, err := q.GetLatestDeviceProgressEvent(ctx, appq.GetLatestDeviceProgressEventParams{
		ProfileID: profileID, ItemID: itemID, DeviceID: nullString(ev.DeviceID), MediaFileID: nullString(ev.MediaFileID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get latest progress event: %w", err)
	}
	return ev.Seq <= last.Seq && ev.ClientTime.UnixMilli() <= last.ClientTime, nil
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
