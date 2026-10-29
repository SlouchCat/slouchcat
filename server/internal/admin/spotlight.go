package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/trakt"
)

// Spotlight administration (M11, plan 05-plan-spotlight.md 5.4 to 5.7, ADR 0011). The ranges are
// those of the CHECK constraints of spotlight_config (migration 00021); the validation names the
// field before the database could refuse the row with a 500.
const (
	minSpotlightItems    = 5
	maxSpotlightItems    = 20
	minSpotlightHits     = 0
	maxSpotlightHits     = 10
	minSpotlightInterval = 6
	maxSpotlightInterval = 48
	minSpotlightRotation = 3 // seconds per title on the stage (concept 08, F-10, migration 00026)
	maxSpotlightRotation = 15
	maxSpotlightListURL  = 500 // contract SpotlightConfigUpdate.listUrl
	maxSpotlightNote     = 280 // like a pick's note (spotlight_overrides.note)

	// spotlightRefreshGap is the least time between two manual fetches (refreshSpotlight 429).
	spotlightRefreshGap = time.Minute
)

// SpotlightConfig returns the stored configuration with the catalogue of list kinds.
func (s *Service) SpotlightConfig(ctx context.Context) (model.SpotlightConfig, error) {
	row, err := s.read.GetSpotlightConfig(ctx)
	if err != nil {
		return model.SpotlightConfig{}, fmt.Errorf("get spotlight config: %w", err)
	}
	return toSpotlightConfig(row), nil
}

// SetSpotlightConfig validates and stores the writable values: itemCount 5..20, minHits 0..10,
// intervalHours 6..48 and rotationSeconds 3..15 (invalid_value), a listKey of the catalogue, a
// media the kind offers, and for the kind url a public user list address (a watchlist address is
// refused, plan 5.7, rule 11). For the other kinds listUrl is stored as NULL. A missing
// rotationSeconds keeps the stored value (concept 08 5.2: an older administration client must not
// reset it). The address may name a Trakt user and never reaches the log (rule 10).
func (s *Service) SetSpotlightConfig(ctx context.Context, in model.SpotlightConfigUpdate) (model.SpotlightConfig, error) {
	var verr *model.ValidationError
	if in.ItemCount < minSpotlightItems || in.ItemCount > maxSpotlightItems {
		verr = verr.Add("itemCount", "invalid_value")
	}
	if in.MinHits < minSpotlightHits || in.MinHits > maxSpotlightHits {
		verr = verr.Add("minHits", "invalid_value")
	}
	if in.IntervalHours < minSpotlightInterval || in.IntervalHours > maxSpotlightInterval {
		verr = verr.Add("intervalHours", "invalid_value")
	}
	if r := in.RotationSeconds; r != nil && (*r < minSpotlightRotation || *r > maxSpotlightRotation) {
		verr = verr.Add("rotationSeconds", "invalid_value")
	}
	listURL := sql.NullString{}
	kind, ok := trakt.KindOf(in.ListKey)
	switch {
	case !ok:
		verr = verr.Add("listKey", "invalid_value")
	default:
		if !slices.Contains(kind.Media, in.Media) {
			verr = verr.Add("media", "invalid_value")
		}
		if kind.NeedsURL {
			raw := strings.TrimSpace(in.ListURL)
			switch {
			case raw == "":
				verr = verr.Add("listUrl", "required")
			case utf8.RuneCountInString(raw) > maxSpotlightListURL:
				verr = verr.Add("listUrl", "too_long")
			default:
				// ErrWatchlist and ErrInvalidList alike: the field is invalid; the error text is not
				// passed on, so the address stays out of answers and logs.
				if _, _, err := trakt.ParseListURL(raw); err != nil {
					verr = verr.Add("listUrl", "invalid_value")
				}
			}
			listURL = sql.NullString{String: raw, Valid: true}
		}
	}
	if verr != nil {
		return model.SpotlightConfig{}, verr
	}
	var rotation int64
	if in.RotationSeconds != nil {
		rotation = int64(*in.RotationSeconds)
	} else {
		stored, err := s.write.GetSpotlightConfig(ctx)
		if err != nil {
			return model.SpotlightConfig{}, fmt.Errorf("get spotlight config: %w", err)
		}
		rotation = stored.RotationSeconds
	}
	if err := s.write.SetSpotlightConfig(ctx, appq.SetSpotlightConfigParams{
		Enabled: int64(boolInt(in.Enabled)), ListKey: in.ListKey, ListUrl: listURL, Media: in.Media,
		ItemCount: int64(in.ItemCount), MinHits: int64(in.MinHits), IntervalHours: int64(in.IntervalHours),
		RotationSeconds: rotation, Now: s.now().UnixMilli(),
	}); err != nil {
		return model.SpotlightConfig{}, fmt.Errorf("set spotlight config: %w", err)
	}
	row, err := s.write.GetSpotlightConfig(ctx)
	if err != nil {
		return model.SpotlightConfig{}, fmt.Errorf("get spotlight config: %w", err)
	}
	s.log.InfoContext(ctx, "spotlight config set", "list_key", in.ListKey, "media", in.Media)
	return toSpotlightConfig(row), nil
}

// SpotlightEntries returns the fetched entries of the configured list in fetch order, with and
// without a library hit (S-4), and the overrides of that list whose title is not fetched any more.
func (s *Service) SpotlightEntries(ctx context.Context) (model.SpotlightEntries, error) {
	cfg, err := s.read.GetSpotlightConfig(ctx)
	if err != nil {
		return model.SpotlightEntries{}, fmt.Errorf("get spotlight config: %w", err)
	}
	entries, orphans, err := spotlightEntries(ctx, s.sys, cfg.ListKey)
	if err != nil {
		return model.SpotlightEntries{}, err
	}
	return model.SpotlightEntries{ListKey: cfg.ListKey, Entries: entries, Orphans: orphans}, nil
}

// SetSpotlightEntryOverride writes the override of one title (media, tmdbID) of the configured
// list. A nil field keeps the stored value, a Clear flag (JSON null) removes it, an empty update
// removes the whole override. An override left with no value is removed as well. Errors:
// *model.ValidationError (media, pinnedRank below 1, note over 280 characters), model.ErrNotFound
// when the list has neither an entry nor an override of the title, model.ErrRankTaken when another
// title of the list holds the rank. The answer is the title as getSpotlightEntries shows it; after
// removing an orphan's override it is the bare orphan without override values.
func (s *Service) SetSpotlightEntryOverride(ctx context.Context, media string, tmdbID int64, upd model.SpotlightOverrideUpdate) (model.SpotlightEntry, error) {
	if media != trakt.MediaMovie && media != trakt.MediaShow {
		return model.SpotlightEntry{}, (&model.ValidationError{}).Add("media", "invalid_value")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.SpotlightEntry{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	cfg, err := appq.New(tx).GetSpotlightConfig(ctx)
	if err != nil {
		return model.SpotlightEntry{}, fmt.Errorf("get spotlight config: %w", err)
	}
	q := systemq.New(tx)
	n, err := q.CountSpotlightEntry(ctx, systemq.CountSpotlightEntryParams{ListKey: cfg.ListKey, Media: media, TmdbID: tmdbID})
	if err != nil {
		return model.SpotlightEntry{}, fmt.Errorf("count spotlight entry: %w", err)
	}
	cur, err := q.GetSpotlightOverride(ctx, systemq.GetSpotlightOverrideParams{ListKey: cfg.ListKey, Media: media, TmdbID: tmdbID})
	hasOverride := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.SpotlightEntry{}, fmt.Errorf("get spotlight override: %w", err)
	}
	if n == 0 && !hasOverride {
		return model.SpotlightEntry{}, model.ErrNotFound
	}

	hidden, rank, note := cur.Hidden != 0, cur.PinnedRank, cur.Note
	if !upd.Empty {
		if upd.Hidden != nil {
			hidden = *upd.Hidden
		}
		var verr *model.ValidationError
		switch {
		case upd.ClearPinnedRank:
			rank = sql.NullInt64{}
		case upd.PinnedRank != nil && *upd.PinnedRank < 1:
			verr = verr.Add("pinnedRank", "invalid_value")
		case upd.PinnedRank != nil:
			rank = sql.NullInt64{Int64: int64(*upd.PinnedRank), Valid: true}
		}
		switch {
		case upd.ClearNote:
			note = sql.NullString{}
		case upd.Note != nil && utf8.RuneCountInString(*upd.Note) > maxSpotlightNote:
			verr = verr.Add("note", "too_long")
		case upd.Note != nil:
			note = nullText(*upd.Note)
		}
		if verr != nil {
			return model.SpotlightEntry{}, verr
		}
	}
	key := systemq.DeleteSpotlightOverrideParams{ListKey: cfg.ListKey, Media: media, TmdbID: tmdbID}
	if upd.Empty || (!hidden && !rank.Valid && !note.Valid) {
		if _, err := q.DeleteSpotlightOverride(ctx, key); err != nil {
			return model.SpotlightEntry{}, fmt.Errorf("delete spotlight override: %w", err)
		}
	} else if err := q.UpsertSpotlightOverride(ctx, systemq.UpsertSpotlightOverrideParams{
		ListKey: cfg.ListKey, Media: media, TmdbID: tmdbID, Hidden: int64(boolInt(hidden)), PinnedRank: rank, Note: note,
		Now: s.now().UnixMilli(),
	}); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return model.SpotlightEntry{}, model.ErrRankTaken
		}
		return model.SpotlightEntry{}, fmt.Errorf("upsert spotlight override: %w", err)
	}
	entries, orphans, err := spotlightEntries(ctx, q, cfg.ListKey)
	if err != nil {
		return model.SpotlightEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.SpotlightEntry{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "spotlight override set", "list_key", cfg.ListKey, "media", media, "tmdb_id", tmdbID)
	match := func(e model.SpotlightEntry) bool { return e.Media == media && e.TmdbID == tmdbID }
	if i := slices.IndexFunc(entries, match); i >= 0 {
		return entries[i], nil
	}
	if i := slices.IndexFunc(orphans, match); i >= 0 {
		return orphans[i], nil
	}
	return model.SpotlightEntry{Media: media, TmdbID: tmdbID, Orphaned: true}, nil
}

// RefreshSpotlight queues a fetch of the spotlight list out of schedule, at most once a minute.
// The time of the last manual fetch lives in spotlight_config.last_manual_refresh, so the limit
// survives a restart. Errors: model.ErrRefreshTooSoon, model.ErrProviderNotConfigured without a
// client id (nothing is stamped then, so a refresh right after storing the id is not blocked),
// ErrNotConfigured without WithTrakt.
func (s *Service) RefreshSpotlight(ctx context.Context) error {
	cfg, err := s.write.GetSpotlightConfig(ctx)
	if err != nil {
		return fmt.Errorf("get spotlight config: %w", err)
	}
	now := s.now()
	if cfg.LastManualRefresh.Valid {
		// A stamp in the future (the clock was set back) does not block.
		if gap := now.Sub(time.UnixMilli(cfg.LastManualRefresh.Int64)); gap >= 0 && gap < spotlightRefreshGap {
			return model.ErrRefreshTooSoon
		}
	}
	if err := s.RefreshTrakt(ctx); err != nil {
		return err
	}
	if err := s.write.SetSpotlightManualRefresh(ctx, sql.NullInt64{Int64: now.UnixMilli(), Valid: true}); err != nil {
		return fmt.Errorf("stamp spotlight refresh: %w", err)
	}
	return nil
}

// spotlightEntries reads the entries (with the poster hash of each library hit) and the orphaned
// overrides of list key with q.
func spotlightEntries(ctx context.Context, q *systemq.Queries, listKey string) (entries, orphans []model.SpotlightEntry, err error) {
	rows, err := q.ListSpotlightEntriesAdmin(ctx, listKey)
	if err != nil {
		return nil, nil, fmt.Errorf("list spotlight entries: %w", err)
	}
	entries = make([]model.SpotlightEntry, 0, len(rows))
	for _, r := range rows {
		e := model.SpotlightEntry{
			Media: r.Media, TmdbID: r.TmdbID, Position: int(r.Position), Title: r.Title.String,
			Hidden: r.OverrideHidden.Valid && r.OverrideHidden.Int64 != 0, PinnedRank: nullIntPtr(r.PinnedRank),
			Note: r.Note.String,
		}
		if r.Year.Valid {
			y := int(r.Year.Int64)
			e.Year = &y
		}
		if r.ItemID.Valid {
			e.ItemID, e.ItemTitle = r.ItemID.String, r.ItemTitle
		}
		entries = append(entries, e)
	}
	hits := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ItemID != "" {
			hits = append(hits, e.ItemID)
		}
	}
	hashes, err := posterHashes(ctx, q, hits)
	if err != nil {
		return nil, nil, err
	}
	for i := range entries {
		entries[i].PosterHash = hashes[entries[i].ItemID]
	}
	orows, err := q.ListSpotlightOrphans(ctx, listKey)
	if err != nil {
		return nil, nil, fmt.Errorf("list spotlight orphans: %w", err)
	}
	orphans = make([]model.SpotlightEntry, 0, len(orows))
	for _, r := range orows {
		orphans = append(orphans, model.SpotlightEntry{
			Media: r.Media, TmdbID: r.TmdbID, Hidden: r.Hidden != 0, PinnedRank: nullIntPtr(r.PinnedRank),
			Note: r.Note.String, Orphaned: true,
		})
	}
	return entries, orphans, nil
}

func toSpotlightConfig(row appq.SpotlightConfig) model.SpotlightConfig {
	kinds := trakt.Kinds()
	out := model.SpotlightConfig{
		Enabled: row.Enabled != 0, ListKey: row.ListKey, ListURL: row.ListUrl.String, Media: row.Media,
		ItemCount: int(row.ItemCount), MinHits: int(row.MinHits), IntervalHours: int(row.IntervalHours),
		RotationSeconds: int(row.RotationSeconds), ListKinds: make([]model.SpotlightListKind, 0, len(kinds)),
	}
	for _, k := range kinds {
		out.ListKinds = append(out.ListKinds, model.SpotlightListKind{
			Key: k.Key, Media: k.Media, MediaFilters: k.MediaFilters, NeedsURL: k.NeedsURL,
		})
	}
	return out
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}
