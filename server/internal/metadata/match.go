package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Values of items.metadata_state and items.metadata_error written by this package.
const (
	stateMatched   = "matched"
	stateUnmatched = "unmatched"
	stateFailed    = "failed"
	stateSkipped   = "skipped"

	errNoKey           = "no_tmdb_key"
	errKeyInvalid      = "tmdb_key_invalid"
	errUnsupportedKind = "unsupported_kind"
)

// genres is the JSON shape of items.genres written by metadata.match.
type genres struct {
	DE []string `json:"de"`
	EN []string `json:"en"`
}

func (s *Service) handleMatch(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.match(ctx, itemID)
}

// match enriches one item. Transient TMDB errors are returned so the queue retries; an invalid
// key, a missing key and a title TMDB does not know end the job with a state on the item.
func (s *Service) match(ctx context.Context, itemID string) error {
	item, err := s.read.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("metadata.match: load item %s: %w", itemID, err)
	}
	if item.DeletedAt.Valid {
		return nil
	}
	if item.Kind != "movie" {
		return s.setState(ctx, item, stateSkipped, errUnsupportedKind)
	}
	key, err := s.keys.TMDBKey(ctx)
	if err != nil {
		return fmt.Errorf("metadata.match: read tmdb key: %w", err)
	}
	if key == "" {
		return s.setState(ctx, item, stateSkipped, errNoKey)
	}
	c := s.client.WithKey(key)

	de, en, err := s.fetch(ctx, c, item)
	switch {
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.log.Warn("tmdb key rejected", "item_id", item.ID)
		return s.setState(ctx, item, stateFailed, errKeyInvalid)
	case errors.Is(err, ErrNotFound):
		return s.setState(ctx, item, stateUnmatched, "")
	case err != nil:
		return fmt.Errorf("metadata.match %s: %w", item.ID, err)
	case en.ID == 0:
		return s.setState(ctx, item, stateUnmatched, "")
	}

	applied, err := s.apply(ctx, item.ID, de, en)
	if err != nil || !applied {
		return err
	}
	payload, err := json.Marshal(itemPayload{ItemID: item.ID})
	if err != nil {
		return fmt.Errorf("metadata.match: image payload: %w", err)
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type:           JobImageFetch,
		Priority:       imageFetchPriority,
		IdempotencyKey: "image:" + item.ID,
		Payload:        payload,
	}); err != nil {
		return fmt.Errorf("metadata.match: %w", err)
	}
	return nil
}

// fetch resolves the TMDB ID of item (tmdb_id, else imdb_id, else title search) and returns the
// details in German and English. A zero en.ID means no match.
func (s *Service) fetch(ctx context.Context, c *Client, item systemq.Item) (de, en Movie, err error) {
	var id int64
	switch {
	case item.TmdbID.Valid && item.TmdbID.Int64 > 0:
		id = item.TmdbID.Int64
	case item.ImdbID.Valid && item.ImdbID.String != "":
		id, err = c.FindByIMDB(ctx, item.ImdbID.String)
	default:
		var year *int
		if item.Year.Valid {
			y := int(item.Year.Int64)
			year = &y
		}
		id, err = c.SearchMovie(ctx, item.Title, year)
	}
	if err != nil || id == 0 {
		return de, en, err
	}
	if de, err = c.Movie(ctx, id, langDE); err != nil {
		return de, en, err
	}
	en, err = c.Movie(ctx, id, langEN)
	return de, en, err
}

// apply writes texts and item columns in one transaction, keeping locked fields. It reports false
// when the item disappeared in the meantime.
func (s *Service) apply(ctx context.Context, itemID string, de, en Movie) (bool, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("metadata.match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)

	cur, err := q.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("metadata.match: reload item %s: %w", itemID, err)
	}
	if cur.DeletedAt.Valid {
		return false, nil
	}
	texts, err := q.ListItemTextsByItem(ctx, itemID)
	if err != nil {
		return false, fmt.Errorf("metadata.match: load texts %s: %w", itemID, err)
	}
	locked := lockedFields(cur.LockedFields)

	for _, t := range itemTexts(cur, texts, locked, de, en) {
		if err := q.UpsertItemText(ctx, t); err != nil {
			return false, fmt.Errorf("metadata.match: write %s text of %s: %w", t.Lang, itemID, err)
		}
	}
	p, err := itemUpdate(cur, locked, de, en)
	if err != nil {
		return false, err
	}
	p.Now = sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
	if err := q.UpdateItemFromTMDB(ctx, p); err != nil {
		return false, fmt.Errorf("metadata.match: update item %s: %w", itemID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("metadata.match: commit %s: %w", itemID, err)
	}
	s.log.Info("item matched", "item_id", itemID, "tmdb_id", en.ID,
		"rating_source", p.RatingSource.String)
	return true, nil
}

// itemUpdate computes the item columns from the TMDB details. Locked fields and empty TMDB values
// keep the current value.
func itemUpdate(cur systemq.Item, locked map[string]bool, de, en Movie) (systemq.UpdateItemFromTMDBParams, error) {
	p := systemq.UpdateItemFromTMDBParams{
		ID:             cur.ID,
		OriginalTitle:  cur.OriginalTitle,
		Year:           cur.Year,
		ReleaseDate:    cur.ReleaseDate,
		RuntimeMinutes: cur.RuntimeMinutes,
		Genres:         cur.Genres,
		TmdbID:         cur.TmdbID,
		ImdbID:         cur.ImdbID,
		OwnRating:      cur.OwnRating,
		RatingSource:   cur.RatingSource,
	}
	if !locked["original_title"] && en.OriginalTitle != "" {
		p.OriginalTitle = nullString(en.OriginalTitle)
	}
	if !locked["release_date"] && en.ReleaseDate != "" {
		p.ReleaseDate = nullString(en.ReleaseDate)
	}
	if y, ok := yearOf(en.ReleaseDate); ok && !locked["year"] {
		p.Year = sql.NullInt64{Int64: int64(y), Valid: true}
	}
	if !locked["runtime_minutes"] && en.Runtime > 0 {
		p.RuntimeMinutes = sql.NullInt64{Int64: int64(en.Runtime), Valid: true}
	}
	if !locked["genres"] {
		raw, err := json.Marshal(genres{DE: genreNames(de.Genres), EN: genreNames(en.Genres)})
		if err != nil {
			return p, fmt.Errorf("metadata.match: genres: %w", err)
		}
		p.Genres = string(raw)
	}
	if !locked["tmdb_id"] {
		p.TmdbID = sql.NullInt64{Int64: en.ID, Valid: true}
	}
	if !locked["imdb_id"] && en.IMDBID != "" {
		p.ImdbID = nullString(en.IMDBID)
	}
	src := cur.RatingSource.String
	if !locked["own_rating"] && src != "nfo" && src != "manual" {
		rd := en.ReleaseDates
		if len(rd.Results) == 0 {
			rd = de.ReleaseDates
		}
		rating, source := FSKFromReleaseDates(rd)
		p.OwnRating, p.RatingSource = sql.NullInt64{}, nullString(source)
		if rating != nil {
			p.OwnRating = sql.NullInt64{Int64: int64(*rating), Valid: true}
		}
	}
	return p, nil
}

// itemTexts returns the de and en texts. German falls back to the English values where TMDB has
// no translation; locked fields keep the stored text (or the parsed title).
func itemTexts(cur systemq.Item, stored []systemq.ItemText, locked map[string]bool, de, en Movie) []systemq.UpsertItemTextParams {
	old := map[string]systemq.ItemText{}
	for _, t := range stored {
		old[t.Lang] = t
	}
	build := func(lang string, m Movie) systemq.UpsertItemTextParams {
		t := systemq.UpsertItemTextParams{
			ItemID:   cur.ID,
			Lang:     lang,
			Title:    firstNonEmpty(m.Title, en.Title, en.OriginalTitle, cur.Title),
			Overview: nullString(firstNonEmpty(m.Overview, en.Overview)),
			Tagline:  nullString(firstNonEmpty(m.Tagline, en.Tagline)),
		}
		prev, had := old[lang]
		if locked["title"] {
			t.Title = cur.Title
			if had {
				t.Title = prev.Title
			}
		}
		if locked["overview"] {
			t.Overview = prev.Overview
		}
		if locked["tagline"] {
			t.Tagline = prev.Tagline
		}
		return t
	}
	return []systemq.UpsertItemTextParams{build("de", de), build("en", en)}
}

// setState records the outcome of a job that did not match. Unmatched items get matched_at = now
// (the time of the attempt); the other states keep it.
func (s *Service) setState(ctx context.Context, item systemq.Item, state, code string) error {
	now := s.now().UnixMilli()
	matchedAt := item.MatchedAt
	if state == stateUnmatched {
		matchedAt = sql.NullInt64{Int64: now, Valid: true}
	}
	err := s.write.SetItemMetadataState(ctx, systemq.SetItemMetadataStateParams{
		MetadataState: state,
		MetadataError: nullString(code),
		MatchedAt:     matchedAt,
		Now:           now,
		ID:            item.ID,
	})
	if err != nil {
		return fmt.Errorf("metadata.match: set state of %s: %w", item.ID, err)
	}
	s.log.Info("item not matched", "item_id", item.ID, "state", state, "reason", code)
	return nil
}

func genreNames(gs []Genre) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		if g.Name != "" {
			out = append(out, g.Name)
		}
	}
	return out
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
