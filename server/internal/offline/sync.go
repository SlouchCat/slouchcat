package offline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Sync of a device, the revocation list and the auto-download rules (M5 design 3.4 and 3.5).

// validateIDs checks reported download ids (at most MaxHeld, each a ULID).
func validateIDs(field string, ids []string) error {
	var verr *model.ValidationError
	if len(ids) > MaxHeld {
		verr = verr.Add(field, "too_many")
	}
	for _, id := range ids {
		if !downloadID.MatchString(id) {
			verr = verr.Add(field, "invalid_id")
			break
		}
	}
	if verr != nil {
		return verr
	}
	return nil
}

// Sync records the device's complete fetches, answers the revocation list of the reported ids,
// applies the auto-download rules of the account's profiles on the device, prepares evicted
// copies the device lacks, recomputes the allowed profiles of every download of the device and
// account and returns their states, the offline profile data and the rule of the active profile.
// Errors: model.ErrDeviceRequired, *model.ValidationError.
func (s *Service) Sync(ctx context.Context, session model.Session, req model.OfflineSyncRequest) (model.OfflineSync, error) {
	held := make([]string, 0, len(req.Held))
	complete := map[string]bool{}
	for _, h := range req.Held {
		held = append(held, h.ID)
		if h.Complete {
			complete[h.ID] = true
		}
	}
	if err := validateIDs("held", held); err != nil {
		return model.OfflineSync{}, err
	}
	switch req.Network {
	case "", "wifi", "cellular", "unknown":
	default:
		return model.OfflineSync{}, (&model.ValidationError{}).Add("network", "invalid")
	}
	if session.DeviceID == "" {
		return model.OfflineSync{}, model.ErrDeviceRequired
	}
	if s.pins == nil {
		return model.OfflineSync{}, ErrNoPins
	}
	now := s.now()
	account, device := session.Account.ID, session.DeviceID

	// (1) Complete fetches: the server copy goes 24 h later (AN-20).
	for chunk := range slices.Chunk(slices.Sorted(maps.Keys(complete)), writeBatch) {
		if _, err := s.write.SetDownloadsFetched(ctx, systemq.SetDownloadsFetchedParams{
			Now: ms(now), DeviceID: device, AccountID: account, Ids: chunk,
		}); err != nil {
			return model.OfflineSync{}, fmt.Errorf("record fetched downloads: %w", err)
		}
	}
	// (2) Revocation list: reported ids with a tombstone of this device (AN-4, K-7).
	revoked, err := s.tombstones(ctx, device, held)
	if err != nil {
		return model.OfflineSync{}, err
	}
	profiles, err := s.pins.ListMyProfiles(ctx, account)
	if err != nil {
		return model.OfflineSync{}, fmt.Errorf("list profiles: %w", err)
	}
	// (3) Auto-download rules of the account's profiles on this device (O-3), best effort: a
	// failing rule must not cost the device its revocation list and profile data.
	s.applyRules(ctx, account, device, profiles, now)
	// (4) Evicted copies the device does not hold completely are prepared again; unfinished
	// downloads without a preparation job get one.
	rows, err := s.read.ListDeviceDownloads(ctx, systemq.ListDeviceDownloadsParams{DeviceID: device, AccountID: account})
	if err != nil {
		return model.OfflineSync{}, fmt.Errorf("list device downloads: %w", err)
	}
	for _, row := range rows {
		switch row.State {
		case model.DownloadEvicted:
			if !complete[row.ID] {
				if err := s.requeue(ctx, row.ID); err != nil {
					return model.OfflineSync{}, err
				}
			}
		case model.DownloadQueued, model.DownloadPreparing, model.DownloadPaused:
			if _, err := s.ensureJob(ctx, row.ID, row.State); err != nil {
				return model.OfflineSync{}, err
			}
		}
	}
	// (5) Allowed profiles of every download of the device (plan 3.3).
	if err := s.recomputeAllowed(ctx, account, rows, now); err != nil {
		return model.OfflineSync{}, err
	}
	if rows, err = s.read.ListDeviceDownloads(ctx, systemq.ListDeviceDownloadsParams{DeviceID: device, AccountID: account}); err != nil {
		return model.OfflineSync{}, fmt.Errorf("list device downloads: %w", err)
	}
	// (6) The answer.
	out := model.OfflineSync{ServerTime: now, Revoked: revoked, Downloads: make([]model.DownloadState, 0, len(rows))}
	active := session.Profile.ID
	wanted := map[int64]bool{active: true}
	// AN-19: PIN material only for profiles of downloads the device holds (reported in held) or
	// can fetch or has fetched (ready, evicted), never because a profile is active.
	pinned := map[int64]bool{}
	reported := map[string]bool{}
	for _, id := range held {
		reported[id] = true
	}
	var mine []string
	for _, row := range rows {
		st := toState(row)
		holds := reported[row.ID] || row.State == model.DownloadReady || row.State == model.DownloadEvicted
		for _, pid := range st.AllowedProfileIDs {
			wanted[pid] = true
			if holds {
				pinned[pid] = true
			}
		}
		if slices.Contains(st.AllowedProfileIDs, active) {
			mine = append(mine, row.ItemID)
		}
		out.Downloads = append(out.Downloads, st)
	}
	items, err := s.vis.Summaries(ctx, visibility.Profile(session.Profile), mine)
	if err != nil {
		return model.OfflineSync{}, err
	}
	for i, row := range rows {
		if it, ok := items[row.ItemID]; ok && slices.Contains(out.Downloads[i].AllowedProfileIDs, active) {
			out.Downloads[i].Item = &it
		}
	}
	if out.Profiles, err = s.offlineProfiles(ctx, account, profiles, wanted, pinned, now); err != nil {
		return model.OfflineSync{}, err
	}
	if out.Rules, err = s.rule(ctx, active, device); err != nil {
		return model.OfflineSync{}, err
	}
	return out, nil
}

// tombstones returns the reported ids that have a tombstone of the device.
func (s *Service) tombstones(ctx context.Context, device string, ids []string) ([]string, error) {
	out := []string{}
	for chunk := range slices.Chunk(ids, writeBatch) {
		found, err := s.read.ListDeviceTombstones(ctx, systemq.ListDeviceTombstonesParams{DeviceID: device, Ids: chunk})
		if err != nil {
			return nil, fmt.Errorf("list tombstones: %w", err)
		}
		out = append(out, found...)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// recomputeAllowed stores the profiles of the account that see each download's title, in
// transactions of at most writeBatch rows.
func (s *Service) recomputeAllowed(ctx context.Context, account int64, rows []systemq.Download, now time.Time) error {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ItemID)
	}
	allowed, err := s.vis.AllowedProfiles(ctx, visibility.System("offline: allowed profiles at sync"), account, ids)
	if err != nil {
		return fmt.Errorf("allowed profiles: %w", err)
	}
	type change struct{ id, allowed string }
	var changes []change
	for _, row := range rows {
		if v := encodeIDs(allowed[row.ItemID]); v != row.AllowedProfileIds {
			changes = append(changes, change{row.ID, v})
		}
	}
	for chunk := range slices.Chunk(changes, writeBatch) {
		tx, err := s.db.Write.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin: %w", err)
		}
		q := s.write.WithTx(tx)
		for _, c := range chunk {
			if err := q.SetDownloadAllowed(ctx, systemq.SetDownloadAllowedParams{
				AllowedProfileIds: c.allowed, Now: sql.NullInt64{Int64: ms(now), Valid: true}, ID: c.id,
			}); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("set allowed profiles: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit allowed profiles: %w", err)
		}
	}
	return nil
}

// offlineProfiles returns the offline data of the account's profiles that may play a download on
// the device or are active, in the order of the profile list (primary first). The PIN material
// (salt, offline PIN verifier) goes only to the profiles in pinned, those of downloads the device
// holds (AN-19).
func (s *Service) offlineProfiles(ctx context.Context, account int64, profiles []model.Profile, wanted, pinned map[int64]bool, now time.Time) ([]model.OfflineProfile, error) {
	var ids []int64
	for _, p := range profiles {
		if wanted[p.ID] && pinned[p.ID] {
			ids = append(ids, p.ID)
		}
	}
	pins, err := s.pins.OfflinePins(ctx, account, ids)
	if err != nil {
		return nil, fmt.Errorf("offline pins: %w", err)
	}
	out := make([]model.OfflineProfile, 0, len(wanted))
	for _, p := range profiles {
		if !wanted[p.ID] {
			continue
		}
		op := model.OfflineProfile{ID: p.ID, Name: p.Name, Accent: p.Accent, Language: p.Language, IsPrimary: p.IsPrimary, HasPin: p.HasPin}
		if pin, ok := pins[p.ID]; ok {
			op.HasPin, op.OfflinePin = pin.HasPin, pin.OfflinePin
			if pin.PinSalt != "" {
				op.PinSalt, op.PinIterations = pin.PinSalt, model.OfflinePinIterations
			}
		}
		if s.playback != nil {
			if op.WatchTime, err = s.playback.OfflineWatchTime(ctx, p, now); err != nil {
				return nil, fmt.Errorf("offline watch time of profile %d: %w", p.ID, err)
			}
		}
		out = append(out, op)
	}
	return out, nil
}

// applyRules creates the auto-downloads of the enabled next-episode rules of the account's
// profiles on the device (O-3, AN-13): per series the profile watched in the last 60 days, the
// next visible episode after the last watched one, while fewer than maxCount auto-downloads the
// profile has not finished are on the device. Best effort: a failing rule is logged and the
// others still run.
func (s *Service) applyRules(ctx context.Context, account int64, device string, profiles []model.Profile, now time.Time) {
	rules, err := s.read.ListActiveDeviceRules(ctx, systemq.ListActiveDeviceRulesParams{DeviceID: device, AccountID: account})
	if err != nil {
		s.log.WarnContext(ctx, "list download rules", "account_id", account, "err", err)
		return
	}
	for _, r := range rules {
		i := slices.IndexFunc(profiles, func(p model.Profile) bool { return p.ID == r.ProfileID })
		if i < 0 {
			continue
		}
		if err := s.applyRule(ctx, account, device, profiles[i], r, now); err != nil {
			s.log.WarnContext(ctx, "auto-download rule not applied", "profile_id", r.ProfileID, "err", err)
		}
	}
}

func (s *Service) applyRule(ctx context.Context, account int64, device string, p model.Profile, r systemq.DownloadRule, now time.Time) error {
	pid := sql.NullInt64{Int64: p.ID, Valid: true}
	open, err := s.read.CountOpenAutoDownloads(ctx, systemq.CountOpenAutoDownloadsParams{ProfileID: pid, DeviceID: device})
	if err != nil {
		return fmt.Errorf("count auto-downloads: %w", err)
	}
	if open >= r.MaxCount {
		return nil
	}
	watched, err := s.read.ListRecentEpisodeProgress(ctx, systemq.ListRecentEpisodeProgressParams{ProfileID: p.ID, Since: ms(now.Add(-autoWindow))})
	if err != nil {
		return fmt.Errorf("list recent episodes: %w", err)
	}
	scope := visibility.Profile(p)
	seen := map[string]bool{}
	for _, w := range watched {
		if seen[w.SeriesID.String] {
			continue // only the last watched episode of a series counts
		}
		seen[w.SeriesID.String] = true
		next, err := s.vis.NextEpisode(ctx, scope, w.ItemID)
		if errors.Is(err, model.ErrNotFound) || (err == nil && next == nil) {
			continue // the watched episode is hidden now, or it was the last one
		}
		if err != nil {
			return err
		}
		if existing, err := s.read.GetDeviceDownload(ctx, systemq.GetDeviceDownloadParams{DeviceID: device, ItemID: next.ID}); err == nil {
			// Already on the device; a failed auto-download of the account is tried again after a
			// wait (a short outage of the source must not stop the rule for good).
			if existing.State != model.DownloadFailed || existing.Auto != 1 || existing.AccountID != account ||
				now.Sub(time.UnixMilli(existing.UpdatedAt)) < autoRetryWait {
				continue
			}
			if err := s.retry(ctx, existing.ID, p.ID); err != nil {
				return err
			}
			if open++; open >= r.MaxCount {
				return nil
			}
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get device download: %w", err)
		}
		src, err := s.sourceOf(ctx, scope, *next, "")
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := s.create(ctx, account, p.ID, device, src, r.Quality, defaultIndex(src), true); errors.Is(err, errOtherAccount) {
			continue
		} else if err != nil {
			return err
		}
		if open++; open >= r.MaxCount {
			return nil
		}
	}
	return nil
}

// rule returns the stored rule of a profile on a device, or the default rule.
func (s *Service) rule(ctx context.Context, profileID int64, device string) (model.DownloadRules, error) {
	r, err := s.read.GetDownloadRule(ctx, systemq.GetDownloadRuleParams{ProfileID: profileID, DeviceID: device})
	if errors.Is(err, sql.ErrNoRows) {
		return model.DefaultDownloadRules, nil
	}
	if err != nil {
		return model.DownloadRules{}, fmt.Errorf("get download rule: %w", err)
	}
	return model.DownloadRules{
		Enabled: r.Enabled == 1, NextEpisodes: r.NextEpisodes == 1, NewPodcastEpisodes: r.NewPodcastEpisodes == 1,
		WifiOnly: r.WifiOnly == 1, MaxCount: int(r.MaxCount), Quality: r.Quality,
	}, nil
}

// Rules returns the auto-download rule of the active profile on the session's device (the default
// rule, disabled, without a stored one). model.ErrDeviceRequired without a device.
func (s *Service) Rules(ctx context.Context, session model.Session) (model.DownloadRules, error) {
	if session.DeviceID == "" {
		return model.DownloadRules{}, model.ErrDeviceRequired
	}
	return s.rule(ctx, session.Profile.ID, session.DeviceID)
}

// SetRules stores the auto-download rule of the active profile on the session's device; it takes
// effect at the next sync. Errors: model.ErrDeviceRequired, *model.ValidationError.
func (s *Service) SetRules(ctx context.Context, session model.Session, r model.DownloadRules) (model.DownloadRules, error) {
	var verr *model.ValidationError
	if r.MaxCount < 1 || r.MaxCount > 20 {
		verr = verr.Add("maxCount", "out_of_range")
	}
	if _, ok := model.DownloadProfileFor(r.Quality); !ok {
		verr = verr.Add("quality", "invalid")
	}
	if verr != nil {
		return model.DownloadRules{}, verr
	}
	if session.DeviceID == "" {
		return model.DownloadRules{}, model.ErrDeviceRequired
	}
	err := s.write.UpsertDownloadRule(ctx, systemq.UpsertDownloadRuleParams{
		ProfileID: session.Profile.ID, DeviceID: session.DeviceID, Enabled: boolInt(r.Enabled), NextEpisodes: boolInt(r.NextEpisodes),
		NewPodcastEpisodes: boolInt(r.NewPodcastEpisodes), WifiOnly: boolInt(r.WifiOnly), MaxCount: int64(r.MaxCount),
		Quality: r.Quality, Now: ms(s.now()),
	})
	if err != nil {
		return model.DownloadRules{}, fmt.Errorf("store download rule: %w", err)
	}
	return s.rule(ctx, session.Profile.ID, session.DeviceID)
}

// Revocations returns the reported ids that a device must delete, identified by its device cookie
// only (checkOfflineRevocations, public): a revoked guest's sessions are gone, its tombstones stay.
// Without a device the list is empty. *model.ValidationError for more than MaxHeld or malformed ids.
func (s *Service) Revocations(ctx context.Context, deviceID string, ids []string) ([]string, error) {
	if err := validateIDs("ids", ids); err != nil {
		return nil, err
	}
	if deviceID == "" {
		return []string{}, nil
	}
	return s.tombstones(ctx, deviceID, ids)
}
