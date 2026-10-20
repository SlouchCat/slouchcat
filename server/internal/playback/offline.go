package playback

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Offline playback (M5 design 3.6, O-5, AN-10, AN-18): the remaining watch time a device takes
// along into the offline library, and the upload of the progress events it buffered there.

const (
	// MaxOfflineEvents is the largest batch of uploadOfflineEvents (contract OfflineEventBatch).
	MaxOfflineEvents = 500
	// firstOfflinePlayedMs bounds the played time of the first event of an offline playback: the
	// client reports every 10 s, plus playedSlack.
	firstOfflinePlayedMs = 11_000
	// offlineEventMaxAge is the oldest corrected time an offline event may book: a device clock far
	// in the past cannot move watch time to arbitrary earlier days.
	offlineEventMaxAge = 30 * 24 * time.Hour
)

// OfflineWatchTime returns what a device needs to enforce the daily limit of a profile offline
// (AN-18): the limit, the minutes remaining today at now and the next midnight of the server's
// time zone (the day of startOfDay), after which the full limit applies again; nil when the
// profile has no limit.
func (s *Service) OfflineWatchTime(ctx context.Context, profile model.Profile, now time.Time) (*model.OfflineWatchTime, error) {
	wt, err := s.WatchTime(ctx, profile, now)
	if err != nil || wt == nil {
		return nil, err
	}
	return &model.OfflineWatchTime{
		LimitMinutes: wt.LimitMinutes, RemainingMinutes: wt.RemainingMinutes, ResetsAt: startOfDay(now).AddDate(0, 0, 1),
	}, nil
}

// ReportOffline stores the progress events a device buffered during offline playback, for any
// profile of the session's account, and returns one result per event in the order of events
// (O-5, AN-10).
//
// The device clock is corrected by the difference between sentAt (device clock at sending) and the
// server's clock; a corrected time later than now becomes now, one older than 30 days becomes
// now − 30 days. The events are processed in the order of their corrected times:
//
//   - rejected: invalid fields, a profile outside the session's account (a guest uploads only for
//     its own profiles), a title the profile may not see, or a downloadId of an existing download
//     the event cannot come from (another account or device, another title, a profile not in its
//     allowed profiles, a corrected time before the download was created); a downloadId of a
//     download deleted since counts like none;
//   - duplicate: the same playback id and seq for the same profile and title is already stored;
//   - accepted: stored with server_time = corrected time (watch time and report count the right
//     day, AN-18), offline = 1, uploaded_at = now, the session's device and the playback id as
//     playback_session_id. Its played time counts at most the corrected time since the previous
//     event of the playback plus playedSlack percent, 11 s for the first one. An event newer than
//     the stored progress replaces it (position, completed by the 90 % rule, updated_at = corrected
//     time); an older one only counts for the watch time, so "completed" stays until a newer event
//     below 90 % comes.
//
// Afterwards every profile with accepted events gets the watch-limit check of Report: at the
// limit its running playback ends (stopped_by limit). An empty batch, more than MaxOfflineEvents
// events or a missing sentAt is a *model.ValidationError.
func (s *Service) ReportOffline(ctx context.Context, session model.Session, sentAt time.Time, events []model.OfflineEvent) (model.OfflineEventResult, error) {
	var verr *model.ValidationError
	if sentAt.IsZero() {
		verr = verr.Add("sentAt", "required")
	}
	switch {
	case len(events) == 0:
		verr = verr.Add("events", "required")
	case len(events) > MaxOfflineEvents:
		verr = verr.Add("events", "too_long")
	}
	if verr != nil {
		return model.OfflineEventResult{}, verr
	}
	now := s.now()
	offset := now.Sub(sentAt)
	profiles, err := s.accountProfiles(ctx, session.Account)
	if err != nil {
		return model.OfflineEventResult{}, err
	}

	results := make([]string, len(events))
	corrected := make([]int64, len(events))
	order := make([]int, 0, len(events))
	visible := map[profileItem]bool{}
	downloads := map[string]*appq.GetOfflineEventDownloadRow{}
	oldest := now.Add(-offlineEventMaxAge).UnixMilli()
	for i, ev := range events {
		results[i] = model.OfflineEventRejected
		p, ok := profiles[ev.ProfileID]
		if !ok || validateOffline(ev) != nil {
			continue
		}
		dl, err := s.eventDownload(ctx, downloads, ev.DownloadID)
		if err != nil {
			return model.OfflineEventResult{}, err
		}
		if dl != nil && !fitsDownload(dl, session, ev) {
			continue
		}
		key := profileItem{profileID: ev.ProfileID, itemID: ev.ItemID}
		seen, known := visible[key]
		if !known {
			err := s.vis.Visible(ctx, visibility.Profile(p), ev.ItemID)
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				return model.OfflineEventResult{}, err
			}
			seen = err == nil
			visible[key] = seen
		}
		if !seen {
			continue
		}
		at := ev.ClientTime.Add(offset).UnixMilli()
		if dl != nil && at < dl.CreatedAt {
			continue // played before the download existed
		}
		corrected[i] = min(max(at, oldest), now.UnixMilli())
		order = append(order, i)
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(corrected[a], corrected[b]) })

	affected := map[int64]bool{}
	if len(order) > 0 {
		tx, err := s.db.Write.BeginTx(ctx, nil)
		if err != nil {
			return model.OfflineEventResult{}, fmt.Errorf("begin offline events: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		q := appq.New(tx)
		for _, i := range order {
			stored, err := storeOfflineEvent(ctx, q, session.DeviceID, events[i], corrected[i], now)
			if err != nil {
				return model.OfflineEventResult{}, err
			}
			if !stored {
				results[i] = model.OfflineEventDuplicate
				continue
			}
			results[i] = model.OfflineEventAccepted
			affected[events[i].ProfileID] = true
		}
		if err := tx.Commit(); err != nil {
			return model.OfflineEventResult{}, fmt.Errorf("commit offline events: %w", err)
		}
	}

	out := model.OfflineEventResult{Results: results}
	for _, r := range results {
		switch r {
		case model.OfflineEventAccepted:
			out.Accepted++
		case model.OfflineEventDuplicate:
			out.Duplicates++
		default:
			out.Rejected++
		}
	}
	s.log.InfoContext(ctx, "offline events uploaded", "account_id", session.Account.ID, "accepted", out.Accepted,
		"duplicates", out.Duplicates, "rejected", out.Rejected, "clock_offset_ms", offset.Milliseconds())
	ids := make([]int64, 0, len(affected))
	for id := range affected {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if _, err := s.checkWatchLimit(ctx, profiles[id]); err != nil {
			return model.OfflineEventResult{}, err
		}
	}
	return out, nil
}

// eventDownload returns the download an offline event names, read once per upload in cache; nil
// when the event names none or the download is gone (deleted after the playback: its events still
// count, like those of an event without downloadId).
func (s *Service) eventDownload(ctx context.Context, cache map[string]*appq.GetOfflineEventDownloadRow, id string) (*appq.GetOfflineEventDownloadRow, error) {
	if id == "" {
		return nil, nil
	}
	if row, ok := cache[id]; ok {
		return row, nil
	}
	row, err := s.read.GetOfflineEventDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		cache[id] = nil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get download of offline event: %w", err)
	}
	cache[id] = &row
	return &row, nil
}

// fitsDownload reports whether an offline event can come from the download it names: a download
// of the session's account on the session's device, of the event's title, that the event's
// profile may play (allowed_profile_ids, computed by the server at every sync).
func fitsDownload(dl *appq.GetOfflineEventDownloadRow, session model.Session, ev model.OfflineEvent) bool {
	if dl.AccountID != session.Account.ID || dl.DeviceID != session.DeviceID || dl.ItemID != ev.ItemID {
		return false
	}
	var allowed []int64
	if err := json.Unmarshal([]byte(dl.AllowedProfileIds), &allowed); err != nil {
		return false
	}
	return slices.Contains(allowed, ev.ProfileID)
}

// profileItem keys the visibility checks of one upload.
type profileItem struct {
	profileID int64
	itemID    string
}

// accountProfiles returns the profiles of an account by id with their effective limits (the
// account's caps of the session applied).
func (s *Service) accountProfiles(ctx context.Context, account model.Account) (map[int64]model.Profile, error) {
	rows, err := s.read.ListProfilesByAccount(ctx, account.ID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	out := make(map[int64]model.Profile, len(rows))
	for _, r := range rows {
		limits := auth.ProfileLimits(r)
		out[r.ID] = model.Profile{
			ID: r.ID, AccountID: r.AccountID, Name: r.Name, MaxRating: int(r.MaxRating), Language: r.Language,
			AllLibraries: r.AllLibraries == 1, Accent: r.Accent.String, HasPin: r.PinHash.Valid, HasOfflinePin: r.OfflinePin.Valid,
			IsPrimary: r.IsDefault == 1, Limits: limits, Effective: limits.Cap(account.Limits),
		}
	}
	return out, nil
}

// validateOffline checks the fields of an offline event with the rules of an online one; the
// playback id is required and the download id, when sent, is a ULID.
func validateOffline(ev model.OfflineEvent) error {
	err := validate(model.ProgressEvent{
		MediaFileID: ev.MediaFileID, PlaybackSessionID: ev.PlaybackID, PositionMs: ev.PositionMs, DurationMs: ev.DurationMs,
		PlayedMs: ev.PlayedMs, Seq: ev.Seq, State: ev.State, ClientTime: ev.ClientTime,
	})
	var verr *model.ValidationError
	errors.As(err, &verr)
	if ev.PlaybackID == "" {
		verr = verr.Add("playbackId", "required")
	}
	if ev.DownloadID != "" && !playbackID.MatchString(ev.DownloadID) {
		verr = verr.Add("downloadId", "invalid_value")
	}
	if ev.ClientTime.IsZero() {
		verr = verr.Add("clientTime", "required")
	}
	if verr != nil {
		return verr
	}
	return nil
}

// storeOfflineEvent stores one accepted offline event at its corrected time at (Unix ms) and merges
// the progress; false means a duplicate that was not stored.
func storeOfflineEvent(ctx context.Context, q *appq.Queries, deviceID string, ev model.OfflineEvent, at int64, now time.Time) (bool, error) {
	pid := nullString(ev.PlaybackID)
	dup, err := q.CountOfflineDuplicateEvents(ctx, appq.CountOfflineDuplicateEventsParams{
		PlaybackID: pid, Seq: ev.Seq, ProfileID: ev.ProfileID, ItemID: ev.ItemID,
	})
	if err != nil {
		return false, fmt.Errorf("check duplicate offline event: %w", err)
	}
	if dup > 0 {
		return false, nil
	}
	prev, err := q.GetPreviousPlaybackEventTime(ctx, appq.GetPreviousPlaybackEventTimeParams{
		PlaybackID: pid, ProfileID: ev.ProfileID, ItemID: ev.ItemID, At: at,
	})
	if err != nil {
		return false, fmt.Errorf("get previous offline event: %w", err)
	}
	bound := int64(firstOfflinePlayedMs)
	if prev > 0 {
		elapsed := max(at-prev, 0)
		bound = elapsed + elapsed*playedSlack/100
	}
	err = q.InsertOfflineProgressEvent(ctx, appq.InsertOfflineProgressEventParams{
		ProfileID:   ev.ProfileID,
		ItemID:      ev.ItemID,
		MediaFileID: nullString(ev.MediaFileID),
		DeviceID:    nullString(deviceID),
		PlaybackID:  pid,
		PositionMs:  ev.PositionMs,
		DurationMs:  ev.DurationMs,
		PlayedMs:    min(ev.PlayedMs, bound),
		Seq:         ev.Seq,
		State:       ev.State,
		ClientTime:  ev.ClientTime.UnixMilli(),
		ServerTime:  at,
		UploadedAt:  sql.NullInt64{Int64: now.UnixMilli(), Valid: true},
		DownloadID:  nullString(ev.DownloadID),
	})
	if err != nil {
		return false, fmt.Errorf("insert offline progress event: %w", err)
	}

	// AN-10: the event with the latest time wins; an older one only books watch time.
	cur, err := q.GetProgress(ctx, appq.GetProgressParams{ProfileID: ev.ProfileID, ItemID: ev.ItemID})
	switch {
	case err == nil && at <= cur.UpdatedAt:
		return true, nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return false, fmt.Errorf("get progress: %w", err)
	}
	position := min(ev.PositionMs, ev.DurationMs)
	err = q.UpsertProgress(ctx, appq.UpsertProgressParams{
		ProfileID: ev.ProfileID, ItemID: ev.ItemID, PositionMs: position, DurationMs: ev.DurationMs,
		Completed: boolInt(Completed(position, ev.DurationMs)), Now: at,
	})
	if err != nil {
		return false, fmt.Errorf("upsert progress: %w", err)
	}
	return true, nil
}
