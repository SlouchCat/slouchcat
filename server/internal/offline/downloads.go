package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Download options, creation and deletion (getDownloadOptions, createDownloads, deleteDownload).
// Every read of a title runs with the active profile's scope: an invisible title is
// model.ErrNotFound (404), never 403.

// maxClientID caps ids that clients send (media file ids, like playback).
const maxClientID = 64

// maxLanguage caps DownloadRequest.audioLanguage (contract).
const maxLanguage = 16

// errOtherAccount marks a title that another account already holds on the same device (the
// table allows one download per device and title).
var errOtherAccount = errors.New("offline: another account holds the title on this device")

// source is a media file of a visible title with what the decision and the options need.
type source struct {
	item   model.ItemSummary
	loc    visibility.MediaFileLocation
	probe  *transcode.Probe
	tracks model.TrackList
}

// sourceOf reads the media file (empty fileID: the primary one), its probe and its tracks for the
// scope; a missing file is model.ErrNotFound.
func (s *Service) sourceOf(ctx context.Context, scope visibility.Scope, item model.ItemSummary, fileID string) (source, error) {
	if len(fileID) > maxClientID {
		return source{}, model.ErrNotFound
	}
	loc, err := s.vis.MediaFile(ctx, scope, item.ID, fileID)
	if err != nil {
		return source{}, err
	}
	if !loc.File.Available {
		return source{}, model.ErrNotFound
	}
	pf, err := s.vis.MediaFileProbe(ctx, scope, item.ID, loc.File.ID)
	if err != nil {
		return source{}, err
	}
	tracks, err := s.vis.Tracks(ctx, scope, item.ID, loc.File.ID)
	if err != nil {
		return source{}, err
	}
	return source{item: item, loc: loc, probe: parseProbe(pf.Probe), tracks: tracks}, nil
}

// seasonSources returns the sources of the visible episodes of a season that have an available
// file, in episode order.
func (s *Service) seasonSources(ctx context.Context, scope visibility.Scope, seasonID string) ([]source, error) {
	eps, err := s.vis.Episodes(ctx, scope, seasonID)
	if err != nil {
		return nil, err
	}
	out := make([]source, 0, len(eps))
	for _, ep := range eps {
		src, err := s.sourceOf(ctx, scope, ep, "")
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}

// defaultIndex is the stream index of the default audio track of a source (the probe's, else the
// track flagged default, else the first embedded audio track), -1 without audio.
func defaultIndex(src source) int {
	if i := defaultAudio(src.probe); i >= 0 {
		return i
	}
	first := -1
	for _, t := range src.tracks.Audio {
		if t.Index < 0 {
			continue
		}
		if t.Default {
			return t.Index
		}
		if first < 0 {
			first = t.Index
		}
	}
	return first
}

// hasAudio reports whether the source has an embedded audio stream with the index.
func hasAudio(src source, index int) bool {
	if index < 0 {
		return false
	}
	if slices.ContainsFunc(src.tracks.Audio, func(t model.Track) bool { return t.Index == index }) {
		return true
	}
	if src.probe != nil {
		return slices.ContainsFunc(src.probe.Streams, func(st transcode.Stream) bool { return st.Type == "audio" && st.Index == index })
	}
	return false
}

// languageAudio returns the first embedded audio track in the language, else the default one.
func languageAudio(src source, lang string) int {
	if lang != "" {
		for _, t := range src.tracks.Audio {
			if t.Index >= 0 && strings.EqualFold(t.Language, lang) {
				return t.Index
			}
		}
	}
	return defaultIndex(src)
}

// textTracks keeps the subtitle tracks that travel along as WebVTT.
func textTracks(tracks []model.Track) []model.Track {
	out := make([]model.Track, 0, len(tracks))
	for _, t := range tracks {
		if t.Text {
			out = append(out, t)
		}
	}
	return out
}

// Options returns the download profiles of a movie, an episode or a season with size estimates
// (O-1). Errors: model.ErrNotFound (unknown, invisible, a file of another title),
// model.ErrNotDownloadable (another kind).
func (s *Service) Options(ctx context.Context, session model.Session, itemID, mediaFileID string) (model.DownloadOptions, error) {
	scope := visibility.Profile(session.Profile)
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return model.DownloadOptions{}, err
	}
	switch item.Kind {
	case "movie", "episode":
		src, err := s.sourceOf(ctx, scope, item, mediaFileID)
		if err != nil {
			return model.DownloadOptions{}, err
		}
		out := model.DownloadOptions{
			MediaFileID: src.loc.File.ID, AudioTracks: src.tracks.Audio, Subtitles: textTracks(src.tracks.Subtitles),
			Qualities: make([]model.DownloadQualityOption, 0, len(model.DownloadProfiles)),
		}
		audio := defaultIndex(src)
		for _, prof := range model.DownloadProfiles {
			c := decide(src.loc.File, src.probe, prof, audio)
			out.DurationMs = c.DurationMs
			out.Qualities = append(out.Qualities, model.DownloadQualityOption{
				Quality: prof.Quality, Width: c.Width, Height: c.Height, VideoBitrate: c.VideoKbps, AudioBitrate: c.AudioKbps,
				EstimatedBytes: c.EstimatedBytes, Copy: c.Copy,
			})
		}
		if session.DeviceID != "" {
			row, err := s.read.GetDeviceDownload(ctx, systemq.GetDeviceDownloadParams{DeviceID: session.DeviceID, ItemID: item.ID})
			switch {
			case err == nil && row.AccountID == session.Account.ID:
				st := toState(row)
				st.Item = &item
				out.Existing = &st
			case err != nil && !errors.Is(err, sql.ErrNoRows):
				return model.DownloadOptions{}, fmt.Errorf("get device download: %w", err)
			}
		}
		return out, nil
	case "season":
		srcs, err := s.seasonSources(ctx, scope, item.ID)
		if err != nil {
			return model.DownloadOptions{}, err
		}
		out := model.DownloadOptions{EpisodeCount: len(srcs), AudioTracks: []model.Track{}, Subtitles: []model.Track{},
			Qualities: make([]model.DownloadQualityOption, 0, len(model.DownloadProfiles))}
		for i, prof := range model.DownloadProfiles {
			opt := model.DownloadQualityOption{Quality: prof.Quality, Copy: len(srcs) > 0}
			for _, src := range srcs {
				c := decide(src.loc.File, src.probe, prof, defaultIndex(src))
				opt.EstimatedBytes += c.EstimatedBytes
				opt.Copy = opt.Copy && c.Copy
				opt.Width, opt.Height = max(opt.Width, c.Width), max(opt.Height, c.Height)
				opt.VideoBitrate, opt.AudioBitrate = max(opt.VideoBitrate, c.VideoKbps), max(opt.AudioBitrate, c.AudioKbps)
				if i == 0 {
					out.DurationMs += c.DurationMs
				}
			}
			out.Qualities = append(out.Qualities, opt)
		}
		return out, nil
	}
	return model.DownloadOptions{}, model.ErrNotDownloadable
}

// validateRequest checks a DownloadRequest.
func validateRequest(req model.DownloadRequest) error {
	var verr *model.ValidationError
	if _, ok := model.DownloadProfileFor(req.Quality); !ok {
		verr = verr.Add("quality", "invalid")
	}
	if len(req.AudioLanguage) > maxLanguage {
		verr = verr.Add("audioLanguage", "too_long")
	}
	if req.AudioTrack != nil && *req.AudioTrack < 0 {
		verr = verr.Add("audioTrack", "invalid")
	}
	if verr != nil {
		return verr
	}
	return nil
}

// Create downloads a movie or an episode (one download) or every visible episode of a season
// (O-2) to the session's device and queues their preparation. A title the device already has is
// returned as it is (an evicted copy is prepared again, a failed one from the start). Errors:
// model.ErrNotFound (unknown, invisible, a season without visible episodes),
// model.ErrNotDownloadable (another kind, or the device holds the title, or every episode of the
// season, for another account), *model.ValidationError, model.ErrDeviceRequired.
func (s *Service) Create(ctx context.Context, session model.Session, itemID string, req model.DownloadRequest) ([]model.DownloadState, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	scope := visibility.Profile(session.Profile)
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return nil, err
	}
	var srcs []source
	audio := map[string]int{}
	switch item.Kind {
	case "movie", "episode":
		src, err := s.sourceOf(ctx, scope, item, req.MediaFileID)
		if err != nil {
			return nil, err
		}
		a := defaultIndex(src)
		if req.AudioTrack != nil {
			if !hasAudio(src, *req.AudioTrack) {
				return nil, (&model.ValidationError{}).Add("audioTrack", "unknown_track")
			}
			a = *req.AudioTrack
		}
		srcs, audio[item.ID] = []source{src}, a
	case "season":
		if srcs, err = s.seasonSources(ctx, scope, item.ID); err != nil {
			return nil, err
		}
		if len(srcs) == 0 {
			return nil, model.ErrNotFound
		}
		for _, src := range srcs {
			audio[src.item.ID] = languageAudio(src, req.AudioLanguage)
		}
	default:
		return nil, model.ErrNotDownloadable
	}
	if session.DeviceID == "" {
		return nil, model.ErrDeviceRequired
	}
	out := make([]model.DownloadState, 0, len(srcs))
	for _, src := range srcs {
		row, err := s.create(ctx, session.Account.ID, session.Profile.ID, session.DeviceID, src, req.Quality, audio[src.item.ID], false)
		if errors.Is(err, errOtherAccount) && item.Kind == "season" {
			continue
		}
		if errors.Is(err, errOtherAccount) {
			return nil, model.ErrNotDownloadable
		}
		if err != nil {
			return nil, err
		}
		st := toState(row)
		st.Item = &src.item
		out = append(out, st)
	}
	if len(out) == 0 {
		return nil, model.ErrNotDownloadable // every episode is held by another account on the device
	}
	return out, nil
}

// create inserts the download of src on the device and queues its preparation, or returns the
// download the device already has (an evicted one is queued again, its allowed profiles are
// recomputed). audio is the source stream index of the audio track (-1: the default one).
func (s *Service) create(ctx context.Context, accountID, profileID int64, deviceID string, src source, quality string, audio int, auto bool) (systemq.Download, error) {
	existing, err := s.read.GetDeviceDownload(ctx, systemq.GetDeviceDownloadParams{DeviceID: deviceID, ItemID: src.item.ID})
	switch {
	case err == nil:
		return s.reuse(ctx, accountID, profileID, existing, auto)
	case !errors.Is(err, sql.ErrNoRows):
		return systemq.Download{}, fmt.Errorf("get device download: %w", err)
	}
	prof, _ := model.DownloadProfileFor(quality)
	c := decide(src.loc.File, src.probe, prof, audio)
	allowed, err := s.allowedJSON(ctx, accountID, src.item.ID)
	if err != nil {
		return systemq.Download{}, err
	}
	now := s.now()
	row, err := s.write.InsertDownload(ctx, systemq.InsertDownloadParams{
		ID: ulid.Make().String(), AccountID: accountID, ProfileID: sql.NullInt64{Int64: profileID, Valid: profileID > 0},
		DeviceID: deviceID, ItemID: src.item.ID, MediaFileID: src.loc.File.ID, Quality: prof.Quality,
		AudioStream: int64(max(audio, 0)), Auto: boolInt(auto),
		DurationMs: sql.NullInt64{Int64: c.DurationMs, Valid: c.DurationMs > 0}, EstimatedBytes: c.EstimatedBytes,
		AllowedProfileIds: allowed, Now: ms(now),
	})
	if errors.Is(err, sql.ErrNoRows) { // a concurrent request inserted it first
		existing, err := s.write.GetDeviceDownload(ctx, systemq.GetDeviceDownloadParams{DeviceID: deviceID, ItemID: src.item.ID})
		if err != nil {
			return systemq.Download{}, fmt.Errorf("get device download: %w", err)
		}
		return s.reuse(ctx, accountID, profileID, existing, auto)
	}
	if err != nil {
		return systemq.Download{}, fmt.Errorf("insert download: %w", err)
	}
	if err := s.enqueue(ctx, row.ID, primaryKey(row.ID), 0); err != nil {
		return systemq.Download{}, err
	}
	s.log.InfoContext(ctx, "download created", "download_id", row.ID, "item_id", row.ItemID, "quality", row.Quality,
		"copy", c.Copy, "estimated_bytes", c.EstimatedBytes, "auto", auto)
	return row, nil
}

// reuse returns an existing download of the device for the account: an evicted copy is queued
// again, a failed one that is requested explicitly (not by a rule) starts over for profileID, an
// unfinished one without a preparation job gets one, the allowed profiles are recomputed. A
// download of another account is errOtherAccount.
func (s *Service) reuse(ctx context.Context, accountID, profileID int64, row systemq.Download, auto bool) (systemq.Download, error) {
	if row.AccountID != accountID {
		return systemq.Download{}, errOtherAccount
	}
	switch row.State {
	case model.DownloadEvicted:
		if err := s.requeue(ctx, row.ID); err != nil {
			return systemq.Download{}, err
		}
	case model.DownloadFailed:
		if !auto {
			if err := s.retry(ctx, row.ID, profileID); err != nil {
				return systemq.Download{}, err
			}
		}
	case model.DownloadQueued, model.DownloadPreparing, model.DownloadPaused:
		if _, err := s.ensureJob(ctx, row.ID, row.State); err != nil {
			return systemq.Download{}, err
		}
	}
	allowed, err := s.allowedJSON(ctx, accountID, row.ItemID)
	if err != nil {
		return systemq.Download{}, err
	}
	if allowed != row.AllowedProfileIds {
		if err := s.write.SetDownloadAllowed(ctx, systemq.SetDownloadAllowedParams{
			AllowedProfileIds: allowed, Now: sql.NullInt64{Int64: ms(s.now()), Valid: true}, ID: row.ID,
		}); err != nil {
			return systemq.Download{}, fmt.Errorf("set allowed profiles: %w", err)
		}
	}
	row, err = s.write.GetDownload(ctx, row.ID)
	if err != nil {
		return systemq.Download{}, fmt.Errorf("get download: %w", err)
	}
	return row, nil
}

// requeue prepares an evicted download again.
func (s *Service) requeue(ctx context.Context, id string) error {
	n, err := s.write.RequeueDownload(ctx, systemq.RequeueDownloadParams{Now: ms(s.now()), ID: id})
	if err != nil {
		return fmt.Errorf("requeue download: %w", err)
	}
	if n == 0 {
		return nil
	}
	s.log.InfoContext(ctx, "evicted download queued again", "download_id", id)
	return s.enqueue(ctx, id, primaryKey(id), 0)
}

// retry prepares a failed download again from the start for profileID (its partial files go).
func (s *Service) retry(ctx context.Context, id string, profileID int64) error {
	s.removeFiles(ctx, id)
	n, err := s.write.ResetFailedDownload(ctx, systemq.ResetFailedDownloadParams{
		ProfileID: sql.NullInt64{Int64: profileID, Valid: profileID > 0}, Now: ms(s.now()), ID: id,
	})
	if err != nil {
		return fmt.Errorf("reset failed download: %w", err)
	}
	if n == 0 {
		return nil
	}
	s.log.InfoContext(ctx, "failed download queued again", "download_id", id)
	return s.enqueue(ctx, id, primaryKey(id), 0)
}

// allowedJSON returns the JSON list of the account's profiles that see the title.
func (s *Service) allowedJSON(ctx context.Context, accountID int64, itemID string) (string, error) {
	allowed, err := s.vis.AllowedProfiles(ctx, visibility.System("offline: allowed profiles of a download"), accountID, []string{itemID})
	if err != nil {
		return "", fmt.Errorf("allowed profiles: %w", err)
	}
	return encodeIDs(allowed[itemID]), nil
}

// Delete removes a download of the session's account (any device) with its server copy; the
// trigger writes the tombstone the device reads at its next sync. The active profile may delete it
// when it may see the title (allowedProfileIds), requested the download, or is the account's
// primary profile, which looks after the devices (O-7). model.ErrNotFound for an unknown id,
// another account's download or one the profile may not delete (invisible is 404, never 403).
func (s *Service) Delete(ctx context.Context, session model.Session, id string) error {
	if !downloadID.MatchString(id) {
		return model.ErrNotFound
	}
	row, err := s.write.GetDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get download: %w", err)
	}
	if !mayDelete(session, row) {
		return model.ErrNotFound
	}
	n, err := s.write.DeleteAccountDownload(ctx, systemq.DeleteAccountDownloadParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return fmt.Errorf("delete download: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.removeFiles(ctx, id)
	s.log.InfoContext(ctx, "download deleted", "download_id", id)
	return nil
}

// mayDelete reports whether the session's profile may delete a download (Delete).
func mayDelete(session model.Session, row systemq.Download) bool {
	if row.AccountID != session.Account.ID {
		return false
	}
	p := session.Profile
	return p.IsPrimary || (row.ProfileID.Valid && row.ProfileID.Int64 == p.ID) || slices.Contains(decodeIDs(row.AllowedProfileIds), p.ID)
}

// preparePayload is the payload of download.prepare.
type preparePayload struct {
	DownloadID string `json:"downloadId"`
}

// primaryKey is the idempotency key of the preparation of a download. A preparation that waits
// (budget) or pauses (preemption) queues its successor under alternateKey while its own job still
// holds the primary key, and the successor alternates back (jobs.Queue keeps one active job per key).
func primaryKey(id string) string { return "download:" + id }

func alternateKey(id string) string { return "download:" + id + ":next" }

// nextKey is the key of the successor of a job with key.
func nextKey(id, key string) string {
	if key == primaryKey(id) {
		return alternateKey(id)
	}
	return primaryKey(id)
}

// enqueue queues the preparation of a download.
func (s *Service) enqueue(ctx context.Context, id, key string, delay time.Duration) error {
	payload, err := json.Marshal(preparePayload{DownloadID: id})
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", JobPrepare, err)
	}
	_, err = s.queue.Enqueue(context.WithoutCancel(ctx), jobs.NewJob{
		Type: JobPrepare, Priority: jobPriority, IdempotencyKey: key, Payload: payload, Delay: delay,
	})
	if err != nil {
		return fmt.Errorf("queue %s: %w", JobPrepare, err)
	}
	return nil
}

// toState maps a download row; Item is left to the caller (only for the active profile).
func toState(row systemq.Download) model.DownloadState {
	st := model.DownloadState{
		ID: row.ID, State: row.State, Quality: row.Quality, Auto: row.Auto == 1, AllowedProfileIDs: decodeIDs(row.AllowedProfileIds),
		EstimatedBytes: row.EstimatedBytes, SizeBytes: row.SizeBytes.Int64, SegmentsDone: int(row.SegmentsDone),
		SegmentsTotal: int(row.SegmentsTotal.Int64), Error: row.Error.String, CreatedAt: time.UnixMilli(row.CreatedAt),
	}
	if row.State != model.DownloadReady {
		st.SizeBytes = 0
	}
	if row.ReadyAt.Valid && row.State == model.DownloadReady {
		st.ReadyAt = time.UnixMilli(row.ReadyAt.Int64)
	}
	return st
}

// encodeIDs is the JSON array of profile ids ("[]" for none).
func encodeIDs(ids []int64) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]" // a slice of int64 always encodes
	}
	return string(b)
}

// decodeIDs reads downloads.allowed_profile_ids; unreadable JSON is no profile.
func decodeIDs(raw string) []int64 {
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil || ids == nil {
		return []int64{}
	}
	return ids
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
