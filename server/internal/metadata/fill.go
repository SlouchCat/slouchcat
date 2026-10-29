package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// metadata.fill (ADR 0015, concept 4.4): after a successful match the title carries the values
// of its identifier, the first provider of its list. This job walks the whole list, identifier
// included, fetches each provider's record once and decides field by field (and per language):
//
//  1. A field in items.locked_fields is never touched (B-6).
//  2. Otherwise the first provider in the list with a non-empty value wins, and
//     item_field_sources records who won. A stored source that stands further front in the
//     list, or is the same provider, keeps the field; one further back (or no longer in the
//     list) is replaced. The job is therefore idempotent and may run any number of times:
//     after another match, after a list change, after a failure.
//  3. Fields: title, overview, tagline per language; original_title; genres (the union of every
//     provider's names, source "union"); runtime_minutes; release_date (with the year);
//     poster and backdrop. Ratings are the exception (K-3): own_rating is only raised, never
//     lowered, whatever the provider's place; rating_source names the strictest.
//  4. A provider without key (Configured false) is skipped and logged as a warning, like the
//     interface warns when it is ticked without key. A provider the title has no id for (a TVDB
//     filler on a series TMDB's record names no tvdb_id for) is skipped quietly.
//
// A list with one entry needs no request: the job only records the identifier as the source of
// every field the title carries, so the next run with a longer list knows what it may replace.
//
// Seasons and episodes get the same decision when more than one provider of the list can
// describe them (TMDB and TVDB): one request per provider and season, one transaction per
// season as in matchSeasons. AniList fills the title level only (concept 4.4).
//
// The job is queued in the transaction of the match (applyTitle) and by UpdateRoot after a list
// change; QueuePendingMetadata re-queues titles whose provenance is missing or stale
// (ListFillItems). A fill of a series that finds its match job still active waits for it: the
// season transactions of the match would otherwise overwrite what the fill decided.
const (
	JobFill      = "metadata.fill"
	fillPriority = imageFetchPriority
)

// genreUnion is the item_field_sources provider of genres: every provider's list, merged.
const genreUnion = "union"

// errFillWaitsForMatch ends a fill whose match job is still queued or running; the queue retries.
var errFillWaitsForMatch = errors.New("the match of the title is still running")

// fieldKey names a field and its language ("" for fields without one).
type fieldKey struct{ field, lang string }

// fieldValues holds the non-empty values of one provider per field.
type fieldValues map[fieldKey]string

// providerFields is what one provider offers for a title, season or episode.
type providerFields struct {
	provider     string
	values       fieldValues
	genres       map[string][]string
	rating       *int
	ratingSource string // rating_source when this provider's rating wins ("tmdb:DE", "anilist")
}

func (pf *providerFields) set(field, lang, value string) {
	if value != "" {
		pf.values[fieldKey{field, lang}] = value
	}
}

// titleFields maps a TMDB or TVDB record onto the fields.
func titleFields(rec TitleRecord) providerFields {
	pf := providerFields{provider: rec.Provider, values: fieldValues{}, genres: map[string][]string{}}
	for lang, t := range rec.Texts {
		pf.set("title", lang, t.Title)
		pf.set("overview", lang, t.Overview)
		pf.set("tagline", lang, t.Tagline)
	}
	pf.set("original_title", "", rec.OriginalTitle)
	pf.set("release_date", "", rec.Date)
	if rec.Runtime > 0 {
		pf.set("runtime_minutes", "", strconv.Itoa(rec.Runtime))
	}
	pf.set("poster", "", rec.PosterPath)
	pf.set("backdrop", "", rec.BackdropPath)
	for lang, names := range rec.Genres {
		if len(names) > 0 {
			pf.genres[lang] = names
		}
	}
	if rating, country := FSKFromCertifications(rec.Certifications); rating != nil {
		pf.rating, pf.ratingSource = rating, rec.Provider+":"+country
	}
	return pf
}

// seasonFields maps a season record onto the fields of a season item.
func seasonFields(provider string, rec SeasonRecord) providerFields {
	pf := providerFields{provider: provider, values: fieldValues{}}
	for lang, t := range rec.Texts {
		pf.set("title", lang, t.Title)
		pf.set("overview", lang, t.Overview)
	}
	pf.set("release_date", "", rec.AirDate)
	pf.set("poster", "", rec.PosterPath)
	return pf
}

// episodeFields maps an episode record onto the fields of an episode item.
func episodeFields(provider string, rec EpisodeRecord) providerFields {
	pf := providerFields{provider: provider, values: fieldValues{}}
	for lang, t := range rec.Texts {
		pf.set("title", lang, t.Title)
		pf.set("overview", lang, t.Overview)
	}
	pf.set("release_date", "", rec.AirDate)
	if rec.Runtime > 0 {
		pf.set("runtime_minutes", "", strconv.Itoa(rec.Runtime))
	}
	pf.set("backdrop", "", rec.StillPath)
	return pf
}

// fieldWrite is the decision for one field: the provider that won and its value.
type fieldWrite struct {
	provider, value string
}

// decideFields applies rule 2 of the job: per field the first provider in order with a value
// wins, unless the field is locked or a stored source stands further front. candidates are in
// list order. The lock name of a text field is the field name (both languages).
func decideFields(order []string, candidates []providerFields, stored map[fieldKey]string, locked map[string]bool) map[fieldKey]fieldWrite {
	rank := func(p string) int {
		if i := slices.Index(order, p); i >= 0 {
			return i
		}
		return len(order)
	}
	out := map[fieldKey]fieldWrite{}
	for _, c := range candidates {
		for key, value := range c.values {
			if _, done := out[key]; done || locked[key.field] {
				continue
			}
			if prev, ok := stored[key]; ok && rank(prev) < rank(c.provider) {
				out[key] = fieldWrite{} // the stored source stands further front: keep the value
				continue
			}
			out[key] = fieldWrite{provider: c.provider, value: value}
		}
	}
	for key, w := range out {
		if w.provider == "" {
			delete(out, key)
		}
	}
	return out
}

// unionGenres merges the genre lists of the candidates per language, in list order, without
// duplicates; nil when no candidate names a genre.
func unionGenres(candidates []providerFields) map[string][]string {
	out := map[string][]string{}
	for _, c := range candidates {
		for lang, names := range c.genres {
			for _, n := range names {
				if n != "" && !slices.Contains(out[lang], n) {
					out[lang] = append(out[lang], n)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// strictestRating returns the highest rating among the candidates and the source of the first
// provider that names it; nil when none has one.
func strictestRating(candidates []providerFields) (*int, string) {
	var best *int
	source := ""
	for _, c := range candidates {
		if c.rating != nil && (best == nil || *c.rating > *best) {
			best, source = c.rating, c.ratingSource
		}
	}
	return best, source
}

func (s *Service) handleFill(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.fill(ctx, itemID)
}

// fill runs the job for one title. Transient provider errors are returned so the queue retries;
// a title that is gone, not matched or of another kind ends the job quietly.
func (s *Service) fill(ctx context.Context, itemID string) error {
	item, err := s.read.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("metadata.fill: load item %s: %w", itemID, err)
	}
	if item.DeletedAt.Valid || item.MetadataState != stateMatched || (item.Kind != "movie" && item.Kind != "series") {
		return nil
	}
	if _, err := s.read.GetActiveJobIDByKey(ctx, sql.NullString{String: "match:" + itemID, Valid: true}); err == nil {
		return fmt.Errorf("metadata.fill %s: %w", itemID, errFillWaitsForMatch)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("metadata.fill: check match job of %s: %w", itemID, err)
	}
	order := model.ParseProviders(item.Providers)
	if len(order) == 0 {
		order = []string{model.ProviderTMDB}
	}
	if len(order) == 1 {
		return s.recordIdentifier(ctx, item, order[0])
	}

	candidates, anilistID, skipped, err := s.fetchTitleRecords(ctx, item, order)
	if err != nil {
		return err
	}
	if len(candidates) == 0 || candidates[0].provider != order[0] {
		// Without the identifier's record the fillers would replace what the match wrote (the
		// title has no provenance to defend it): nothing to decide, the next match brings it.
		s.log.Warn("fill skipped: the identifier gave no record", "item_id", item.ID, "identifier", order[0])
		return nil
	}
	// The list this run stands for: the providers that were configured (a switched-off AniList
	// is left out, so switching it on queues the title again, QueuePendingFills).
	filled := make([]string, 0, len(order))
	for _, p := range order {
		if !slices.Contains(skipped, p) {
			filled = append(filled, p)
		}
	}
	changedImages, err := s.applyFill(ctx, item, order, candidates, anilistID, filled)
	if err != nil {
		return err
	}
	if changedImages {
		if err := s.enqueueImages(ctx, item.ID, imageFetchPriority); err != nil {
			return fmt.Errorf("metadata.fill: %w", err)
		}
	}
	if item.Kind == "series" {
		if err := s.fillSeasons(ctx, item, order, candidates); err != nil {
			return err
		}
	}
	s.log.Info("item filled", "item_id", item.ID, "providers", model.ProvidersJSON(order),
		"fetched", len(candidates), "skipped", skipped)
	return nil
}

// fetchedRecord is one provider's title record with the id it was fetched by.
type fetchedRecord struct {
	provider string
	id       string
	fields   providerFields
}

// fetchTitleRecords fetches the record of every provider in the list that is configured and
// whose id the title carries, in list order; AniList through the mapping list. It returns the
// records, the AniList id to store (-1 when AniList was not asked, 0 when the list has no entry)
// and the providers it skipped. A provider that no longer knows the id contributes nothing.
func (s *Service) fetchTitleRecords(ctx context.Context, item systemq.Item, order []string) ([]fetchedRecord, int64, []string, error) {
	var out []fetchedRecord
	var skipped []string
	anilistID := int64(-1)
	for _, p := range order {
		switch p {
		case model.ProviderTMDB:
			if !item.TmdbID.Valid || item.TmdbID.Int64 <= 0 {
				continue
			}
			if !s.tmdb.Configured(ctx) {
				skipped = append(skipped, p)
				continue
			}
			id := strconv.FormatInt(item.TmdbID.Int64, 10)
			rec, err := s.titleRecord(ctx, s.tmdb, item.Kind, id)
			if err != nil {
				return nil, 0, nil, err
			}
			if rec != nil {
				out = append(out, fetchedRecord{p, id, titleFields(*rec)})
			}
		case model.ProviderTVDB:
			if item.Kind != "series" || !item.TvdbID.Valid || item.TvdbID.Int64 <= 0 {
				continue
			}
			if !s.tvdb.Configured(ctx) {
				skipped = append(skipped, p)
				continue
			}
			id := strconv.FormatInt(item.TvdbID.Int64, 10)
			rec, err := s.titleRecord(ctx, s.tvdb, item.Kind, id)
			if err != nil {
				return nil, 0, nil, err
			}
			if rec != nil {
				out = append(out, fetchedRecord{p, id, titleFields(*rec)})
			}
		case model.ProviderAniList:
			if !s.anilist.Configured(ctx) {
				skipped = append(skipped, p)
				continue
			}
			id, err := s.anilistIDOf(ctx, item)
			if err != nil {
				return nil, 0, nil, err
			}
			anilistID = id
			if id <= 0 {
				continue
			}
			rec, err := s.anilist.Media(ctx, id)
			if errors.Is(err, ErrNotFound) {
				s.log.Warn("anilist does not know the id of the mapping list", "item_id", item.ID, "anilist_id", id)
				continue
			}
			if err != nil {
				return nil, 0, nil, fmt.Errorf("metadata.fill %s: %w", item.ID, err)
			}
			out = append(out, fetchedRecord{p, strconv.FormatInt(id, 10), rec.fields()})
		}
	}
	for _, p := range skipped {
		s.log.Warn("metadata provider in the list of a title but not configured, skipped",
			"item_id", item.ID, "provider", p)
	}
	return out, anilistID, skipped, nil
}

// titleRecord fetches the movie or series record of one provider; nil when the provider does
// not know the id any more.
func (s *Service) titleRecord(ctx context.Context, prov Provider, kind, id string) (*TitleRecord, error) {
	var rec TitleRecord
	var err error
	if kind == "movie" {
		var m MovieRecord
		m, err = prov.Movie(ctx, id, textLangs)
		rec = m.TitleRecord
	} else {
		var sr SeriesRecord
		sr, err = prov.Series(ctx, id, textLangs)
		rec = sr.TitleRecord
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, nil
	case errors.Is(err, errNotConfigured):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("metadata.fill: %s %s: %w", prov.Name(), id, err)
	}
	return &rec, nil
}

// anilistIDOf looks the AniList id of a title up in the mapping list: a film by its TMDB id,
// else its IMDb id; a series by its TVDB id (the title level is season 1). The stored id is
// used when it is set; 0 ("looked up, no entry") is looked up again, because the list grows.
func (s *Service) anilistIDOf(ctx context.Context, item systemq.Item) (int64, error) {
	if item.AnilistID.Valid && item.AnilistID.Int64 > 0 {
		return item.AnilistID.Int64, nil
	}
	var id int64
	var err error
	if item.Kind == "movie" {
		id, err = s.animeLists.MovieAniListID(ctx, item.TmdbID.Int64, item.ImdbID.String)
	} else {
		id, err = s.animeLists.SeriesAniListID(ctx, item.TvdbID.Int64)
	}
	if err != nil {
		return 0, fmt.Errorf("metadata.fill %s: %w", item.ID, err)
	}
	return id, nil
}

// applyFill decides and writes the title-level fields in one transaction. It reports whether
// poster or backdrop changed (image.fetch follows after the commit).
func (s *Service) applyFill(ctx context.Context, snap systemq.Item, order []string, recs []fetchedRecord, anilistID int64, filled []string) (bool, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("metadata.fill: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := waitForMatch(ctx, q, snap.ID); err != nil {
		return false, err
	}
	cur, ok, err := reloadItem(ctx, q, snap.ID)
	if err != nil || !ok {
		return false, err
	}
	if !sameIdentity(cur, snap) {
		return false, fmt.Errorf("metadata.fill %s: %w", snap.ID, errItemChanged)
	}
	candidates := make([]providerFields, 0, len(recs))
	for _, r := range recs {
		candidates = append(candidates, r.fields)
	}
	changedImages, err := s.writeFill(ctx, q, cur, order, candidates, true, anilistID, filled)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("metadata.fill: commit %s: %w", snap.ID, err)
	}
	if changedImages.rating && s.recheck != nil {
		if err := s.recheck(ctx); err != nil {
			s.log.Warn("visibility recheck after a rating change failed", "item_id", snap.ID, "err", err)
		}
	}
	return changedImages.images, nil
}

// fillOutcome tells what a write changed.
type fillOutcome struct {
	images bool // poster or backdrop path changed
	rating bool // own_rating changed
}

// writeFill decides and writes the fields of one item (title, season or episode) inside q. title
// says whether the item is a movie or series (the other columns and the rating); anilistID is
// stored on a title when it is not -1. Multi-episode files keep the composite title the match
// wrote.
func (s *Service) writeFill(ctx context.Context, q *systemq.Queries, cur systemq.Item, order []string, candidates []providerFields, title bool, anilistID int64, filled []string) (fillOutcome, error) {
	var out fillOutcome
	storedTexts, err := q.ListItemTextsByItem(ctx, cur.ID)
	if err != nil {
		return out, fmt.Errorf("metadata.fill: load texts %s: %w", cur.ID, err)
	}
	sources, err := q.ListItemFieldSources(ctx, cur.ID)
	if err != nil {
		return out, fmt.Errorf("metadata.fill: load sources %s: %w", cur.ID, err)
	}
	stored := make(map[fieldKey]string, len(sources))
	for _, src := range sources {
		stored[fieldKey{src.Field, src.Lang}] = src.Provider
	}
	locked := lockedFields(cur.LockedFields)
	if cur.EpisodeEnd.Valid {
		locked["title"] = true
	}
	writes := decideFields(order, candidates, stored, locked)
	now := s.now().UnixMilli()

	// Texts: the stored row of each language, with the decided fields replaced.
	old := map[string]systemq.ItemText{}
	for _, t := range storedTexts {
		old[t.Lang] = t
	}
	for _, lang := range textLangs {
		title, ok1 := writes[fieldKey{"title", lang}]
		overview, ok2 := writes[fieldKey{"overview", lang}]
		tagline, ok3 := writes[fieldKey{"tagline", lang}]
		if !ok1 && !ok2 && !ok3 {
			continue
		}
		prev, had := old[lang]
		p := systemq.UpsertItemTextParams{ItemID: cur.ID, Lang: lang, Title: cur.Title}
		if had {
			p.Title, p.Overview, p.Tagline = prev.Title, prev.Overview, prev.Tagline
		}
		if ok1 {
			p.Title = title.value
		}
		if ok2 {
			p.Overview = nullString(overview.value)
		}
		if ok3 {
			p.Tagline = nullString(tagline.value)
		}
		if err := q.UpsertItemText(ctx, p); err != nil {
			return out, fmt.Errorf("metadata.fill: write %s text of %s: %w", lang, cur.ID, err)
		}
	}

	// Columns.
	releaseDate, runtime := cur.ReleaseDate, cur.RuntimeMinutes
	year := cur.Year
	if w, ok := writes[fieldKey{"release_date", ""}]; ok {
		releaseDate = nullString(w.value)
		if y, ok := yearOf(w.value); ok && !locked["year"] {
			year = sql.NullInt64{Int64: int64(y), Valid: true}
		}
	}
	if w, ok := writes[fieldKey{"runtime_minutes", ""}]; ok {
		if n, err := strconv.Atoi(w.value); err == nil && n > 0 {
			runtime = sql.NullInt64{Int64: int64(n), Valid: true}
		}
	}
	poster, backdrop := cur.TmdbPosterPath, cur.TmdbBackdropPath
	if w, ok := writes[fieldKey{"poster", ""}]; ok {
		poster = nullString(w.value)
	}
	if w, ok := writes[fieldKey{"backdrop", ""}]; ok {
		backdrop = nullString(w.value)
	}
	out.images = poster != cur.TmdbPosterPath || backdrop != cur.TmdbBackdropPath

	if title {
		originalTitle := cur.OriginalTitle
		if w, ok := writes[fieldKey{"original_title", ""}]; ok {
			originalTitle = nullString(w.value)
		}
		genresJSON := cur.Genres
		genreSource := ""
		if union := unionGenres(candidates); union != nil && !locked["genres"] {
			raw, err := json.Marshal(genres{DE: nonNil(union["de"]), EN: nonNil(union["en"])})
			if err != nil {
				return out, fmt.Errorf("metadata.fill: genres: %w", err)
			}
			genresJSON, genreSource = string(raw), genreUnion
		}
		ownRating, ratingSource := cur.OwnRating, cur.RatingSource
		ratingProvider := ""
		src := cur.RatingSource.String
		if best, source := strictestRating(candidates); best != nil && !locked["own_rating"] && src != "nfo" && src != "manual" {
			switch {
			case !cur.OwnRating.Valid || int64(*best) > cur.OwnRating.Int64:
				// Raised, never lowered (K-3); the source names the strictest provider.
				ownRating = sql.NullInt64{Int64: int64(*best), Valid: true}
				ratingSource = nullString(source)
				ratingProvider, _, _ = strings.Cut(source, ":")
				out.rating = true
			case int64(*best) == cur.OwnRating.Int64:
				// The same rating: the first provider in the list that names it is its source,
				// so a run after another match ends with the same row.
				ratingSource = nullString(source)
				ratingProvider, _, _ = strings.Cut(source, ":")
			}
		}
		anilist := cur.AnilistID
		if anilistID >= 0 {
			anilist = sql.NullInt64{Int64: anilistID, Valid: true}
		}
		if err := q.UpdateItemFill(ctx, systemq.UpdateItemFillParams{
			OriginalTitle: originalTitle, Year: year, ReleaseDate: releaseDate, RuntimeMinutes: runtime,
			Genres: genresJSON, OwnRating: ownRating, RatingSource: ratingSource,
			TmdbPosterPath: poster, TmdbBackdropPath: backdrop, AnilistID: anilist,
			FilledProviders: nullString(model.ProvidersJSON(filled)),
			Now:             now, ID: cur.ID,
		}); err != nil {
			return out, fmt.Errorf("metadata.fill: update %s: %w", cur.ID, err)
		}
		if genreSource != "" {
			writes[fieldKey{"genres", ""}] = fieldWrite{provider: genreSource}
		}
		if ratingProvider != "" {
			writes[fieldKey{"own_rating", ""}] = fieldWrite{provider: ratingProvider}
		}
	} else {
		if err := q.UpdateChildFill(ctx, systemq.UpdateChildFillParams{
			ReleaseDate: releaseDate, RuntimeMinutes: runtime, TmdbPosterPath: poster, TmdbBackdropPath: backdrop,
			Now: now, ID: cur.ID,
		}); err != nil {
			return out, fmt.Errorf("metadata.fill: update %s: %w", cur.ID, err)
		}
	}
	for key, w := range writes {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
			ItemID: cur.ID, Field: key.field, Lang: key.lang, Provider: w.provider,
		}); err != nil {
			return out, fmt.Errorf("metadata.fill: write source of %s: %w", cur.ID, err)
		}
	}
	return out, nil
}

// recordIdentifier is the run of a title whose list holds only its identifier: no request, no
// change of a value; every field the title carries that has no source yet is attributed to the
// identifier, so a later run with a longer list knows what it may replace.
func (s *Service) recordIdentifier(ctx context.Context, item systemq.Item, identifier string) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("metadata.fill: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	cur, ok, err := reloadItem(ctx, q, item.ID)
	if err != nil || !ok {
		return err
	}
	texts, err := q.ListItemTextsByItem(ctx, cur.ID)
	if err != nil {
		return fmt.Errorf("metadata.fill: load texts %s: %w", cur.ID, err)
	}
	sources, err := q.ListItemFieldSources(ctx, cur.ID)
	if err != nil {
		return fmt.Errorf("metadata.fill: load sources %s: %w", cur.ID, err)
	}
	have := map[fieldKey]bool{}
	for _, src := range sources {
		have[fieldKey{src.Field, src.Lang}] = true
	}
	var keys []fieldKey
	add := func(field, lang, value string) {
		if value != "" && !have[fieldKey{field, lang}] {
			keys = append(keys, fieldKey{field, lang})
		}
	}
	for _, t := range texts {
		add("title", t.Lang, t.Title)
		add("overview", t.Lang, t.Overview.String)
		add("tagline", t.Lang, t.Tagline.String)
	}
	add("original_title", "", cur.OriginalTitle.String)
	add("release_date", "", cur.ReleaseDate.String)
	if cur.RuntimeMinutes.Valid && cur.RuntimeMinutes.Int64 > 0 {
		add("runtime_minutes", "", "set")
	}
	if cur.Genres != "" && cur.Genres != "[]" && cur.Genres != `{"de":[],"en":[]}` {
		add("genres", "", "set")
	}
	add("poster", "", cur.TmdbPosterPath.String)
	add("backdrop", "", cur.TmdbBackdropPath.String)
	if cur.OwnRating.Valid && strings.HasPrefix(cur.RatingSource.String, identifier) {
		add("own_rating", "", "set")
	}
	for _, k := range keys {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
			ItemID: cur.ID, Field: k.field, Lang: k.lang, Provider: identifier,
		}); err != nil {
			return fmt.Errorf("metadata.fill: write source of %s: %w", cur.ID, err)
		}
	}
	if err := q.SetItemFilledProviders(ctx, systemq.SetItemFilledProvidersParams{
		FilledProviders: nullString(model.ProvidersJSON([]string{identifier})), Now: s.now().UnixMilli(), ID: cur.ID,
	}); err != nil {
		return fmt.Errorf("metadata.fill: record list of %s: %w", cur.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("metadata.fill: commit %s: %w", cur.ID, err)
	}
	s.log.Info("item provenance recorded", "item_id", cur.ID, "provider", identifier, "fields", len(keys))
	return nil
}

// fillSeasons runs the decision for every season of the library and its episodes when the list
// holds more than one provider that describes seasons (TMDB and TVDB, both fetched for the
// title); one request per provider and season, one transaction per season.
func (s *Service) fillSeasons(ctx context.Context, series systemq.Item, order []string, recs []fetchedRecord) error {
	type seasonProvider struct {
		prov Provider
		id   string
	}
	var provs []seasonProvider
	for _, r := range recs {
		switch r.provider {
		case model.ProviderTMDB:
			provs = append(provs, seasonProvider{s.tmdb, r.id})
		case model.ProviderTVDB:
			provs = append(provs, seasonProvider{s.tvdb, r.id})
		}
	}
	if len(provs) < 2 {
		return nil
	}
	children, err := s.read.ListChildItems(ctx, sql.NullString{String: series.ID, Valid: true})
	if err != nil {
		return fmt.Errorf("metadata.fill: list seasons of %s: %w", series.ID, err)
	}
	seasons := map[int]systemq.Item{}
	episodes := map[int][]systemq.Item{}
	for _, c := range children {
		if !c.SeasonNumber.Valid {
			continue
		}
		n := int(c.SeasonNumber.Int64)
		switch c.Kind {
		case "season":
			seasons[n] = c
			eps, err := s.read.ListChildItems(ctx, sql.NullString{String: c.ID, Valid: true})
			if err != nil {
				return fmt.Errorf("metadata.fill: list episodes of %s: %w", c.ID, err)
			}
			for _, e := range eps {
				if e.Kind == "episode" {
					episodes[n] = append(episodes[n], e)
				}
			}
		case "episode":
			episodes[n] = append(episodes[n], c)
		}
	}
	numbers := make([]int, 0, len(seasons)+len(episodes))
	for n := range seasons {
		numbers = append(numbers, n)
	}
	for n := range episodes {
		if _, ok := seasons[n]; !ok {
			numbers = append(numbers, n)
		}
	}
	slices.Sort(numbers)
	var images []imageJob
	for _, n := range numbers {
		var seasonCands []providerFields
		episodeCands := map[int][]providerFields{}
		// The identifier's record of the season: a filler describes an episode only when it
		// airs the same day there (numbers come from the identifier alone, concept 4.4; where
		// the providers count differently, the filler's episode 3 is another episode).
		var base *SeasonRecord
		for _, sp := range provs {
			srec, err := sp.prov.Season(ctx, sp.id, n, textLangs)
			if errors.Is(err, ErrNotFound) || errors.Is(err, errNotConfigured) {
				if base == nil {
					break // the identifier does not know the season: nothing to fill
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("metadata.fill: season %d of %s: %w", n, series.ID, err)
			}
			if base == nil {
				base = &srec
			} else if base.AirDate != "" && srec.AirDate != "" && base.AirDate != srec.AirDate {
				continue // another season under this number
			}
			seasonCands = append(seasonCands, seasonFields(sp.prov.Name(), srec))
			for _, e := range srec.Episodes {
				if base != &srec && !sameAirDate(base, e) {
					continue
				}
				episodeCands[e.Number] = append(episodeCands[e.Number], episodeFields(sp.prov.Name(), e))
			}
		}
		if len(seasonCands) == 0 {
			continue
		}
		var season *systemq.Item
		if it, ok := seasons[n]; ok {
			season = &it
		}
		eps := episodes[n]
		first := true
		for start := 0; first || start < len(eps); start += maxEpisodesPerTx {
			batch := eps[start:min(start+maxEpisodesPerTx, len(eps))]
			var withSeason *systemq.Item
			if first {
				withSeason = season
			}
			first = false
			written, err := s.fillSeasonBatch(ctx, series, order, withSeason, seasonCands, batch, episodeCands)
			if err != nil {
				return err
			}
			images = append(images, written...)
		}
	}
	for _, j := range images {
		if err := s.enqueueImages(ctx, j.itemID, j.priority); err != nil {
			return fmt.Errorf("metadata.fill: %w", err)
		}
	}
	return nil
}

// sameAirDate reports whether the identifier's season knows an episode of this number that
// aired the same day as e (or neither names a day).
func sameAirDate(base *SeasonRecord, e EpisodeRecord) bool {
	for _, b := range base.Episodes {
		if b.Number == e.Number {
			return b.AirDate == "" || e.AirDate == "" || b.AirDate == e.AirDate
		}
	}
	return false
}

// waitForMatch ends a fill transaction with errFillWaitsForMatch when a match of the title is
// queued or running: its writes would cross the fill's, and the fill of that match would be
// dropped (one active job per key). The queue retries the fill after the match.
func waitForMatch(ctx context.Context, q *systemq.Queries, itemID string) error {
	_, err := q.GetActiveJobIDByKey(ctx, sql.NullString{String: "match:" + itemID, Valid: true})
	switch {
	case err == nil:
		return fmt.Errorf("metadata.fill %s: %w", itemID, errFillWaitsForMatch)
	case errors.Is(err, sql.ErrNoRows):
		return nil
	default:
		return fmt.Errorf("metadata.fill: check match job of %s: %w", itemID, err)
	}
}

// fillSeasonBatch writes one season item and a batch of its episodes in one transaction. Only
// children the match wrote (state matched) are filled; the others wait for their match.
func (s *Service) fillSeasonBatch(ctx context.Context, series systemq.Item, order []string, season *systemq.Item,
	seasonCands []providerFields, eps []systemq.Item, episodeCands map[int][]providerFields,
) ([]imageJob, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("metadata.fill: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := waitForMatch(ctx, q, series.ID); err != nil {
		return nil, err
	}
	if err := checkSeries(ctx, q, series); err != nil {
		return nil, err
	}
	var out []imageJob
	if season != nil {
		cur, ok, err := reloadItem(ctx, q, season.ID)
		if err != nil {
			return nil, err
		}
		if ok && cur.MetadataState == stateMatched {
			o, err := s.writeFill(ctx, q, cur, order, seasonCands, false, -1, nil)
			if err != nil {
				return nil, err
			}
			if o.images {
				out = append(out, imageJob{cur.ID, imageFetchPriority})
			}
		}
	}
	for _, e := range eps {
		if !e.EpisodeNumber.Valid {
			continue
		}
		cands := episodeCands[int(e.EpisodeNumber.Int64)]
		if len(cands) == 0 {
			continue
		}
		cur, ok, err := reloadItem(ctx, q, e.ID)
		if err != nil {
			return nil, err
		}
		if !ok || cur.MetadataState != stateMatched {
			continue
		}
		o, err := s.writeFill(ctx, q, cur, order, cands, false, -1, nil)
		if err != nil {
			return nil, err
		}
		if o.images {
			out = append(out, imageJob{cur.ID, stillPriority})
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("metadata.fill: commit: %w", err)
	}
	return out, nil
}

// enqueueFillTx queues metadata.fill for a title inside the transaction of its match, so the
// match and its fill commit together (ADR 0015). An active fill job of the title keeps its key.
func enqueueFillTx(ctx context.Context, q *systemq.Queries, itemID string, now int64) error {
	payload, err := json.Marshal(itemPayload{ItemID: itemID})
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobFill, err)
	}
	_, err = q.InsertJob(ctx, systemq.InsertJobParams{
		Type: JobFill, Priority: fillPriority, IdempotencyKey: sql.NullString{String: "fill:" + itemID, Valid: true},
		Payload: string(payload), RunAfter: now, MaxAttempts: jobs.DefaultMaxAttempts, Now: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobFill, itemID, err)
	}
	return nil
}

func (s *Service) enqueueFill(ctx context.Context, itemID string) error {
	return s.enqueueJob(ctx, JobFill, "fill:", itemID, fillPriority)
}

// QueuePendingFills enqueues metadata.fill for every matched title of the library with more
// than one provider whose provenance is missing or names a provider that left the list
// (ListFillItems); QueuePendingMetadata calls it after every scan. It returns the number of
// jobs queued.
func (s *Service) QueuePendingFills(ctx context.Context, libraryID int64) (int, error) {
	rows, err := s.read.ListFillItems(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list titles to fill: %w", err)
	}
	configured := map[string]bool{
		model.ProviderTMDB: s.tmdb.Configured(ctx), model.ProviderTVDB: s.tvdb.Configured(ctx),
		model.ProviderAniList: s.anilist.Configured(ctx), model.ProviderLocal: true,
	}
	n := 0
	for _, r := range rows {
		// The list a fill would run with now; a title whose last fill ran with it has nothing
		// to gain (a provider without key is left out on both sides).
		var effective []string
		for _, p := range model.ParseProviders(r.Providers) {
			if configured[p] {
				effective = append(effective, p)
			}
		}
		if r.FilledProviders.Valid && r.FilledProviders.String == model.ProvidersJSON(effective) {
			continue
		}
		if err := s.enqueueFill(ctx, r.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
