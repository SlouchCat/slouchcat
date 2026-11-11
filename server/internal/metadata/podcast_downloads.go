package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The own setting of stored episodes of a podcast (concept 19 part A, 5.1 and 5.3; ADR 0029): the
// subscription belongs to package metadata, so updatePodcast stores the setting here and then
// calls the planner of package library through the hook of main.go. The effective setting comes
// from the one shared query PodcastEffectiveSettings (podcast before library).

// UpdatePodcastDownloads is updatePodcast: it stores the own setting of a podcast (a nil field
// stays, model.PodcastInherit follows the library) and runs the planner. A setting that makes the
// podcast store asks the folder check first (WithPodcastSettingHooks). Errors: model.ErrNotFound
// (unknown or deleted podcast), *model.ValidationError (mode, scope, lastN; mode not_allowed for a
// folder podcast without feed), the error of the folder check (model.ErrRootNotWritable,
// model.ErrRootRequired).
func (s *Service) UpdatePodcastDownloads(ctx context.Context, id string, upd model.PodcastDownloadsUpdate) (model.AdminPodcast, error) {
	if _, err := s.adminPodcast(ctx, id); err != nil {
		return model.AdminPodcast{}, err
	}
	sets, err := s.read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{PodcastID: id})
	if err != nil {
		return model.AdminPodcast{}, fmt.Errorf("setting of podcast %s: %w", id, err)
	}
	var verr *model.ValidationError
	if len(sets) != 1 {
		return model.AdminPodcast{}, verr.Add("mode", "not_allowed")
	}
	cur := sets[0]
	mode, scope, lastN := cur.OwnMode, cur.OwnScope, cur.OwnLastN
	if upd.Mode != nil {
		switch *upd.Mode {
		case model.PodcastInherit:
			mode = sql.NullString{}
		case model.PodcastSave, model.PodcastLink:
			mode = sql.NullString{String: *upd.Mode, Valid: true}
		default:
			verr = verr.Add("mode", "invalid")
		}
	}
	if upd.Scope != nil {
		switch *upd.Scope {
		case model.PodcastInherit:
			scope, lastN = sql.NullString{}, sql.NullInt64{}
		case model.PodcastScopeNew, model.PodcastScopeLastN, model.PodcastScopeAll:
			scope = sql.NullString{String: *upd.Scope, Valid: true}
		default:
			verr = verr.Add("scope", "invalid")
		}
	}
	if upd.LastN != nil {
		if *upd.LastN < 1 || *upd.LastN > model.PodcastLastNMax {
			verr = verr.Add("lastN", "out_of_range")
		} else {
			lastN = sql.NullInt64{Int64: int64(*upd.LastN), Valid: true}
		}
	}
	if verr != nil {
		return model.AdminPodcast{}, verr
	}
	if !scope.Valid {
		lastN = sql.NullInt64{} // the N belongs to an own scope
	}
	libraryMode := cur.EffectiveMode
	if cur.OwnMode.Valid {
		lib, err := s.read.PodcastLibraryMode(ctx, cur.LibraryID)
		if err != nil {
			return model.AdminPodcast{}, fmt.Errorf("setting of library %d: %w", cur.LibraryID, err)
		}
		libraryMode = lib
	}
	effective := libraryMode
	if mode.Valid {
		effective = mode.String
	}
	if effective == model.PodcastSave && s.podcastSavable != nil {
		if err := s.podcastSavable(ctx, id); err != nil {
			return model.AdminPodcast{}, err
		}
	}
	if err := s.write.PodcastUpsertSettings(ctx, systemq.PodcastUpsertSettingsParams{
		ItemID: id, Mode: mode, Scope: scope, LastN: lastN, Now: s.feeds.now().UnixMilli(),
	}); err != nil {
		return model.AdminPodcast{}, fmt.Errorf("store setting of podcast %s: %w", id, err)
	}
	if s.podcastChanged != nil {
		if err := s.podcastChanged(ctx, id); err != nil {
			s.log.WarnContext(ctx, "queue the podcast planner after a setting", "item_id", id, "err", err)
		}
	}
	return s.adminPodcast(ctx, id)
}

// withDownloads adds the setting and the state of the stored episodes to podcasts (AdminPodcast,
// section 11): own and effective setting from PodcastEffectiveSettings, stored bytes, downloads
// and the newest episode from PodcastDownloadStats. A folder podcast without feed keeps them empty.
func (s *Service) withDownloads(ctx context.Context, podcasts []model.AdminPodcast) ([]model.AdminPodcast, error) {
	sets, err := s.read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{})
	if err != nil {
		return nil, fmt.Errorf("podcast settings: %w", err)
	}
	stats, err := s.read.PodcastDownloadStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("podcast download stats: %w", err)
	}
	bySet := make(map[string]systemq.PodcastEffectiveSettingsRow, len(sets))
	for _, set := range sets {
		bySet[set.PodcastID] = set
	}
	byStat := make(map[string]systemq.PodcastDownloadStatsRow, len(stats))
	for _, st := range stats {
		byStat[st.PodcastID] = st
	}
	for i := range podcasts {
		p := &podcasts[i]
		if st, ok := byStat[p.ID]; ok {
			p.StoredBytes = st.StoredBytes
			p.DownloadsQueued, p.DownloadsRunning, p.DownloadsFailed = int(st.Queued), int(st.Running), int(st.Failed)
			if st.LatestEpisodeAt > 0 {
				t := time.UnixMilli(st.LatestEpisodeAt).UTC()
				p.LatestEpisodeAt = &t
			}
		}
		set, ok := bySet[p.ID]
		if !ok {
			continue
		}
		p.Mode, p.Scope = model.PodcastInherit, model.PodcastInherit
		if set.OwnMode.Valid {
			p.Mode = set.OwnMode.String
		}
		if set.OwnScope.Valid {
			p.Scope = set.OwnScope.String
			n := int(set.EffectiveLastN)
			p.LastN = &n
		}
		p.EffectiveMode, p.EffectiveScope, p.EffectiveLastN = set.EffectiveMode, set.EffectiveScope, int(set.EffectiveLastN)
	}
	return podcasts, nil
}
