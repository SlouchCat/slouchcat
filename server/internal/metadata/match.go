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
	"time"

	"github.com/oklog/ulid/v2"

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
	statePending   = "pending"

	errNoKey           = "no_tmdb_key"
	errNoTVDBKey       = "no_tvdb_key"
	errKeyInvalid      = "tmdb_key_invalid"
	errUnsupportedKind = "unsupported_kind"
	errSeasonViaSeries = "season_via_series"
	errSeriesUnmatched = "series_unmatched"
)

// Job priorities of the jobs this package enqueues (0 lowest, 9 highest).
const (
	matchPriority        = 4 // movies and series, like the library scan
	matchEpisodePriority = 3
	matchManualPriority  = 7 // manual match and refresh run before scans
	stillPriority        = 1 // episode stills come after posters
)

// errItemChanged ends a match whose title changed its provider ids or locked fields while the job
// resolved it (a manual match or a refresh in the meantime): the job writes nothing and returns
// this error, so the queue retries it and the next run resolves with the new ids.
var errItemChanged = errors.New("provider ids or locked fields changed during the match")

// sameIdentity reports whether cur still has the provider ids, locked fields and provider list of
// the snapshot a match started from (a changed list means another identifier or other fillers:
// the job resolved with the old one, ADR 0015).
func sameIdentity(cur, snap systemq.Item) bool {
	return cur.TmdbID == snap.TmdbID && cur.TvdbID == snap.TvdbID && cur.ImdbID == snap.ImdbID &&
		cur.LockedFields == snap.LockedFields && cur.Providers == snap.Providers
}

// maxEpisodesPerTx bounds a season transaction: every episode writes two texts and one row, so a
// transaction stays below 500 rows (jobs package rule).
const maxEpisodesPerTx = 150

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

// match enriches one item. Transient provider errors are returned so the queue retries; an
// invalid key, a missing key and a title the provider does not know end the job with a state on
// the item.
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
	switch item.Kind {
	case "movie":
		return s.matchMovie(ctx, item)
	case "series":
		return s.matchSeries(ctx, item)
	case "season":
		if item.MetadataState == stateMatched {
			return nil // the series job wrote it
		}
		return s.setState(ctx, item, stateSkipped, errSeasonViaSeries)
	case "episode":
		return s.matchEpisode(ctx, item)
	default:
		return s.setState(ctx, item, stateSkipped, errUnsupportedKind)
	}
}

// providerFailure ends a job after a provider error: a rejected key fails the item, an unknown ID
// leaves it unmatched, a missing key skips it; anything else is returned for a retry.
func (s *Service) providerFailure(ctx context.Context, item systemq.Item, provider string, err error) error {
	switch {
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.log.Warn("metadata provider key rejected", "provider", provider, "item_id", item.ID)
		return s.setState(ctx, item, stateFailed, provider+"_key_invalid")
	case errors.Is(err, ErrNotFound):
		return s.setState(ctx, item, stateUnmatched, "")
	case errors.Is(err, errNotConfigured):
		return s.setState(ctx, item, stateSkipped, "no_"+provider+"_key")
	default:
		return fmt.Errorf("metadata.match %s: %w", item.ID, err)
	}
}

func (s *Service) matchMovie(ctx context.Context, item systemq.Item) error {
	key, err := s.keys.TMDBKey(ctx)
	if err != nil {
		return fmt.Errorf("metadata.match: read tmdb key: %w", err)
	}
	if key == "" {
		return s.setState(ctx, item, stateSkipped, errNoKey)
	}
	rec, found, err := s.resolveMovie(ctx, item)
	if err != nil {
		return s.providerFailure(ctx, item, ProviderTMDB, err)
	}
	if !found || rec.TMDBID == 0 {
		return s.setState(ctx, item, stateUnmatched, "")
	}
	_, applied, err := s.applyTitle(ctx, item, rec.TitleRecord)
	if err != nil || !applied {
		return err
	}
	if err := s.enqueueImages(ctx, item.ID, imageFetchPriority); err != nil {
		return fmt.Errorf("metadata.match: %w", err)
	}
	return s.enqueueTrailerAfterMatch(ctx, item.ID)
}

// resolveMovie finds the TMDB movie of item: tmdb_id, else imdb_id through /find, else a title
// search with the parsed year ±1.
func (s *Service) resolveMovie(ctx context.Context, item systemq.Item) (MovieRecord, bool, error) {
	id := ""
	switch {
	case item.TmdbID.Valid && item.TmdbID.Int64 > 0:
		id = strconv.FormatInt(item.TmdbID.Int64, 10)
	case item.ImdbID.Valid && item.ImdbID.String != "":
		cs, err := s.tmdb.SearchMovie(ctx, MovieQuery{IMDBID: item.ImdbID.String})
		if err != nil || len(cs) == 0 {
			return MovieRecord{}, false, err
		}
		id = cs[0].ID
	default:
		year := itemYear(item)
		cs, err := s.tmdb.SearchMovie(ctx, MovieQuery{Title: item.Title, Year: year})
		if err != nil {
			return MovieRecord{}, false, err
		}
		c, ok := pickCandidate(cs, year)
		if !ok {
			return MovieRecord{}, false, nil
		}
		id = c.ID
	}
	rec, err := s.tmdb.Movie(ctx, id, textLangs)
	if errors.Is(err, ErrNotFound) {
		return MovieRecord{}, false, nil
	}
	return rec, err == nil, err
}

func itemYear(item systemq.Item) *int {
	if !item.Year.Valid {
		return nil
	}
	y := int(item.Year.Int64)
	return &y
}

func (s *Service) matchSeries(ctx context.Context, item systemq.Item) error {
	tmdbKey, err := s.keys.TMDBKey(ctx)
	if err != nil {
		return fmt.Errorf("metadata.match: read tmdb key: %w", err)
	}
	tvdbKey, _, err := s.keys.TVDBKey(ctx)
	if err != nil {
		return fmt.Errorf("metadata.match: read tvdb key: %w", err)
	}
	locked := lockedFields(item.LockedFields)
	// A manual TVDB match locks tvdb_id and an empty tmdb_id: TVDB is the administrator's choice.
	manualTVDB := locked["tvdb_id"] && locked["tmdb_id"] && item.TvdbID.Valid && !item.TmdbID.Valid
	// The identifier of the title is the first entry of its provider list (ADR 0015): with TVDB
	// first, TVDB finds the series and numbers its seasons, and the tmdb_id is fetched on the side
	// through TMDB's external-id lookup so credits, trailers and images keep working.
	tvdbFirst := identifierOf(item) == model.ProviderTVDB

	var prov Provider
	var rec SeriesRecord
	found := false
	switch {
	case (manualTVDB || tvdbFirst) && tvdbKey == "":
		return s.setState(ctx, item, stateSkipped, errNoTVDBKey)
	case manualTVDB:
		prov = s.tvdb
		rec, found, err = s.resolveSeriesTVDB(ctx, item)
	case tvdbFirst:
		prov = s.tvdb
		rec, found, err = s.resolveSeriesTVDB(ctx, item)
		if err == nil && found && tmdbKey != "" && rec.TMDBID == 0 {
			rec.TMDBID, err = s.tmdbIDOfTVDB(ctx, rec.TVDBID)
		}
	case tmdbKey != "":
		prov = s.tmdb
		rec, found, err = s.resolveSeriesTMDB(ctx, item)
		if err == nil && !found && tvdbKey != "" {
			prov = s.tvdb
			rec, found, err = s.resolveSeriesTVDB(ctx, item)
		}
	case tvdbKey != "":
		prov = s.tvdb
		rec, found, err = s.resolveSeriesTVDB(ctx, item)
	default:
		return s.setState(ctx, item, stateSkipped, errNoKey)
	}
	if err != nil {
		return s.providerFailure(ctx, item, prov.Name(), err)
	}
	if !found {
		return s.setState(ctx, item, stateUnmatched, "")
	}
	series, applied, err := s.applyTitle(ctx, item, rec.TitleRecord)
	if err != nil || !applied {
		return err
	}
	if err := s.matchSeasons(ctx, series, prov, rec); err != nil {
		return err
	}
	if err := s.enqueueFill(ctx, item.ID); err != nil {
		return fmt.Errorf("metadata.match: %w", err)
	}
	return s.enqueueTrailerAfterMatch(ctx, item.ID)
}

// resolveSeriesTMDB finds the TMDB series of item: tmdb_id, else tvdb_id or imdb_id through /find,
// else a title search with the first air date year ±1 (a search with the year first, without it
// when nothing fits).
func (s *Service) resolveSeriesTMDB(ctx context.Context, item systemq.Item) (SeriesRecord, bool, error) {
	id := ""
	if item.TmdbID.Valid && item.TmdbID.Int64 > 0 {
		id = strconv.FormatInt(item.TmdbID.Int64, 10)
	}
	if id == "" && item.TvdbID.Valid && item.TvdbID.Int64 > 0 {
		cs, err := s.tmdb.SearchSeries(ctx, SeriesQuery{TVDBID: item.TvdbID.Int64})
		if err != nil {
			return SeriesRecord{}, false, err
		}
		if len(cs) > 0 {
			id = cs[0].ID
		}
	}
	if id == "" && item.ImdbID.Valid && item.ImdbID.String != "" {
		cs, err := s.tmdb.SearchSeries(ctx, SeriesQuery{IMDBID: item.ImdbID.String})
		if err != nil {
			return SeriesRecord{}, false, err
		}
		if len(cs) > 0 {
			id = cs[0].ID
		}
	}
	if id == "" {
		year := itemYear(item)
		cs, err := s.tmdb.SearchSeries(ctx, SeriesQuery{Title: item.Title, Year: year})
		if err != nil {
			return SeriesRecord{}, false, err
		}
		c, ok := pickCandidate(cs, year)
		if !ok && year != nil {
			if cs, err = s.tmdb.SearchSeries(ctx, SeriesQuery{Title: item.Title}); err != nil {
				return SeriesRecord{}, false, err
			}
			c, ok = pickCandidate(cs, year)
		}
		if !ok {
			return SeriesRecord{}, false, nil
		}
		id = c.ID
	}
	rec, err := s.tmdb.Series(ctx, id, textLangs)
	if errors.Is(err, ErrNotFound) {
		return SeriesRecord{}, false, nil
	}
	return rec, err == nil, err
}

// resolveSeriesTVDB finds the TVDB series of item: tvdb_id, else a title search with year ±1.
func (s *Service) resolveSeriesTVDB(ctx context.Context, item systemq.Item) (SeriesRecord, bool, error) {
	id := ""
	if item.TvdbID.Valid && item.TvdbID.Int64 > 0 {
		id = strconv.FormatInt(item.TvdbID.Int64, 10)
	} else {
		year := itemYear(item)
		cs, err := s.tvdb.SearchSeries(ctx, SeriesQuery{Title: item.Title, Year: year})
		if err != nil {
			return SeriesRecord{}, false, err
		}
		c, ok := pickCandidate(cs, year)
		if !ok {
			return SeriesRecord{}, false, nil
		}
		id = c.ID
	}
	rec, err := s.tvdb.Series(ctx, id, textLangs)
	if errors.Is(err, ErrNotFound) {
		return SeriesRecord{}, false, nil
	}
	return rec, err == nil, err
}

// identifierOf is the first entry of the provider list of an item, the one that identifies it
// (ADR 0015); tmdb for an item without list.
func identifierOf(item systemq.Item) string {
	if ps := model.ParseProviders(item.Providers); len(ps) > 0 {
		return ps[0]
	}
	return model.ProviderTMDB
}

// tmdbIDOfTVDB looks the TMDB id of a series up by its TVDB id (TMDB's /find); 0 when TMDB does
// not know the series.
func (s *Service) tmdbIDOfTVDB(ctx context.Context, tvdbID int64) (int64, error) {
	if tvdbID == 0 {
		return 0, nil
	}
	cs, err := s.tmdb.SearchSeries(ctx, SeriesQuery{TVDBID: tvdbID})
	if err != nil || len(cs) == 0 {
		return 0, err
	}
	id, err := strconv.ParseInt(cs[0].ID, 10, 64)
	if err != nil {
		return 0, nil
	}
	return id, nil
}

// seriesProvider returns the provider and ID a matched series is described by: the identifier of
// its list when the series carries that provider's id (TVDB first means TVDB numbering, ADR 0015),
// else TMDB when it has a tmdb_id, else TVDB with its tvdb_id.
func (s *Service) seriesProvider(series systemq.Item) (Provider, string, bool) {
	tmdb := series.TmdbID.Valid && series.TmdbID.Int64 > 0
	tvdb := series.TvdbID.Valid && series.TvdbID.Int64 > 0
	switch {
	case identifierOf(series) == model.ProviderTVDB && tvdb:
		return s.tvdb, strconv.FormatInt(series.TvdbID.Int64, 10), true
	case tmdb:
		return s.tmdb, strconv.FormatInt(series.TmdbID.Int64, 10), true
	case tvdb:
		return s.tvdb, strconv.FormatInt(series.TvdbID.Int64, 10), true
	}
	return nil, "", false
}

// matchSeasons writes every season of the series that exists in the library and every episode of
// it that exists, one transaction per season (and per maxEpisodesPerTx episodes), then enqueues
// image.fetch for the series, its seasons and episodes with an image path and metadata.match for
// the episodes still waiting. series is the series as applyTitle wrote it; a season transaction
// that finds other provider ids or locked fields on it writes nothing (errItemChanged).
func (s *Service) matchSeasons(ctx context.Context, series systemq.Item, prov Provider, rec SeriesRecord) error {
	seriesID := series.ID
	children, err := s.read.ListChildItems(ctx, sql.NullString{String: seriesID, Valid: true})
	if err != nil {
		return fmt.Errorf("metadata.match: list seasons of %s: %w", seriesID, err)
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
				return fmt.Errorf("metadata.match: list episodes of %s: %w", c.ID, err)
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
	seasonPoster := map[int]string{}
	for _, info := range rec.Seasons {
		seasonPoster[info.Number] = info.PosterPath
	}

	images := []imageJob{}
	if rec.PosterPath != "" || rec.BackdropPath != "" {
		images = append(images, imageJob{seriesID, imageFetchPriority})
	}
	for _, n := range numbers {
		srec, err := prov.Season(ctx, rec.ID, n, textLangs)
		if errors.Is(err, ErrNotFound) {
			continue // the episodes stay pending and are matched one by one below
		}
		if err != nil {
			return fmt.Errorf("metadata.match: season %d of %s: %w", n, seriesID, err)
		}
		if srec.PosterPath == "" {
			srec.PosterPath = seasonPoster[n]
		}
		var season *systemq.Item
		if it, ok := seasons[n]; ok {
			season = &it
		}
		written, err := s.applySeason(ctx, series, season, episodes[n], srec)
		if err != nil {
			return err
		}
		images = append(images, written...)
	}

	for _, j := range images {
		if err := s.enqueueImages(ctx, j.itemID, j.priority); err != nil {
			return fmt.Errorf("metadata.match: %w", err)
		}
	}
	waiting, err := s.read.ListEpisodesToMatch(ctx, sql.NullString{String: seriesID, Valid: true})
	if err != nil {
		return fmt.Errorf("metadata.match: list waiting episodes of %s: %w", seriesID, err)
	}
	for _, id := range waiting {
		if err := s.enqueueMatch(ctx, id, matchEpisodePriority); err != nil {
			return fmt.Errorf("metadata.match: %w", err)
		}
	}
	s.log.Info("series seasons matched", "item_id", seriesID, "provider", prov.Name(), "seasons", len(numbers),
		"waiting_episodes", len(waiting))
	return nil
}

// imageJob is an image.fetch to enqueue after the season transactions.
type imageJob struct {
	itemID   string
	priority int
}

// applySeason writes the season item (when the library has one) and its episodes in batches. It
// returns the image jobs for the written items with an image path.
func (s *Service) applySeason(ctx context.Context, series systemq.Item, season *systemq.Item, eps []systemq.Item, srec SeasonRecord) ([]imageJob, error) {
	byNumber := make(map[int]EpisodeRecord, len(srec.Episodes))
	for _, e := range srec.Episodes {
		byNumber[e.Number] = e
	}
	var out []imageJob
	first := true
	for start := 0; first || start < len(eps); start += maxEpisodesPerTx {
		batch := eps[start:min(start+maxEpisodesPerTx, len(eps))]
		var withSeason *systemq.Item
		if first {
			withSeason = season
		}
		first = false
		written, err := s.applySeasonBatch(ctx, series, withSeason, batch, srec, byNumber)
		if err != nil {
			return nil, err
		}
		out = append(out, written...)
	}
	return out, nil
}

func (s *Service) applySeasonBatch(ctx context.Context, series systemq.Item, season *systemq.Item, eps []systemq.Item, srec SeasonRecord, byNumber map[int]EpisodeRecord) ([]imageJob, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("metadata.match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	var out []imageJob
	if err := checkSeries(ctx, q, series); err != nil {
		return nil, err
	}

	if season != nil {
		cur, ok, err := reloadItem(ctx, q, season.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			n := srec.Number
			texts := map[string]Text{}
			for _, l := range textLangs {
				t := srec.Texts[l]
				t.Title = firstNonEmpty(t.Title, seasonTitle(l, n))
				texts[l] = t
			}
			if err := s.writeChild(ctx, q, cur, texts, srec.AirDate, 0, srec.PosterPath, "", false, now); err != nil {
				return nil, err
			}
			if srec.PosterPath != "" {
				out = append(out, imageJob{cur.ID, imageFetchPriority})
			}
		}
	}
	for _, e := range eps {
		if !e.EpisodeNumber.Valid {
			continue
		}
		first, ok := byNumber[int(e.EpisodeNumber.Int64)]
		if !ok {
			continue // unknown to the provider: matched alone later (and ends unmatched)
		}
		cur, ok, err := reloadItem(ctx, q, e.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		recs := []EpisodeRecord{first}
		if cur.EpisodeEnd.Valid {
			for k := int(cur.EpisodeNumber.Int64) + 1; k <= int(cur.EpisodeEnd.Int64); k++ {
				r, ok := byNumber[k]
				if !ok {
					break
				}
				recs = append(recs, r)
			}
		}
		if err := s.writeEpisode(ctx, q, cur, recs, now); err != nil {
			return nil, err
		}
		if first.StillPath != "" {
			out = append(out, imageJob{cur.ID, stillPriority})
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("metadata.match: commit season %d: %w", srec.Number, err)
	}
	return out, nil
}

// seasonTitle is the season title when the provider has none: "Staffel N" / "Season N",
// "Specials" for season 0.
func seasonTitle(lang string, n int) string {
	switch {
	case n == 0:
		return "Specials"
	case lang == "de":
		return "Staffel " + strconv.Itoa(n)
	default:
		return "Season " + strconv.Itoa(n)
	}
}

// writeEpisode writes one episode item from its provider records: the first one carries date,
// runtime and still; a multi-episode file gets "E01–E02: Title1 / Title2" when every title is
// known in the language.
func (s *Service) writeEpisode(ctx context.Context, q *systemq.Queries, cur systemq.Item, recs []EpisodeRecord, now int64) error {
	first := recs[0]
	texts := map[string]Text{}
	for _, l := range textLangs {
		t := first.Texts[l]
		t.Title = firstNonEmpty(t.Title, first.Texts["en"].Title)
		if len(recs) > 1 {
			titles := make([]string, 0, len(recs))
			for _, r := range recs {
				if title := firstNonEmpty(r.Texts[l].Title, r.Texts["en"].Title); title != "" {
					titles = append(titles, title)
				}
			}
			if len(titles) == len(recs) {
				t.Title = fmt.Sprintf("E%02d–E%02d: %s", first.Number, recs[len(recs)-1].Number, strings.Join(titles, " / "))
			}
		}
		texts[l] = t
	}
	return s.writeChild(ctx, q, cur, texts, first.AirDate, first.Runtime, "", first.StillPath, true, now)
}

// writeChild writes the texts and columns of a season or episode, keeping locked fields. German
// texts fall back to English when enFallback is set.
func (s *Service) writeChild(ctx context.Context, q *systemq.Queries, cur systemq.Item, texts map[string]Text,
	date string, runtime int, poster, backdrop string, enFallback bool, now int64,
) error {
	stored, err := q.ListItemTextsByItem(ctx, cur.ID)
	if err != nil {
		return fmt.Errorf("metadata.match: load texts %s: %w", cur.ID, err)
	}
	locked := lockedFields(cur.LockedFields)
	defTitle := func(string) string { return cur.Title }
	for _, t := range buildTexts(cur, stored, locked, texts, defTitle, enFallback) {
		if err := q.UpsertItemText(ctx, t); err != nil {
			return fmt.Errorf("metadata.match: write %s text of %s: %w", t.Lang, cur.ID, err)
		}
	}
	p := systemq.UpdateChildMatchParams{
		ID: cur.ID, ReleaseDate: cur.ReleaseDate, RuntimeMinutes: cur.RuntimeMinutes,
		TmdbPosterPath: cur.TmdbPosterPath, TmdbBackdropPath: cur.TmdbBackdropPath,
		Now: sql.NullInt64{Int64: now, Valid: true},
	}
	if !locked["release_date"] && date != "" {
		p.ReleaseDate = nullString(date)
	}
	if !locked["runtime_minutes"] && runtime > 0 {
		p.RuntimeMinutes = sql.NullInt64{Int64: int64(runtime), Valid: true}
	}
	if poster != "" {
		p.TmdbPosterPath = nullString(poster)
	}
	if backdrop != "" {
		p.TmdbBackdropPath = nullString(backdrop)
	}
	if err := q.UpdateChildMatch(ctx, p); err != nil {
		return fmt.Errorf("metadata.match: update %s: %w", cur.ID, err)
	}
	return nil
}

// checkSeries reloads the series inside the transaction of a season or episode write and returns
// errItemChanged when its provider ids or locked fields differ from the snapshot the job resolved
// the provider records with.
func checkSeries(ctx context.Context, q *systemq.Queries, series systemq.Item) error {
	cur, ok, err := reloadItem(ctx, q, series.ID)
	if err != nil {
		return err
	}
	if ok && !sameIdentity(cur, series) {
		return fmt.Errorf("metadata.match %s: %w", series.ID, errItemChanged)
	}
	return nil
}

// reloadItem reads an item inside a transaction; ok is false when it is gone or soft-deleted.
func reloadItem(ctx context.Context, q *systemq.Queries, id string) (systemq.Item, bool, error) {
	cur, err := q.GetItemSystem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return cur, false, nil
	}
	if err != nil {
		return cur, false, fmt.Errorf("metadata.match: reload item %s: %w", id, err)
	}
	return cur, !cur.DeletedAt.Valid, nil
}

// matchEpisode fills one episode from the provider of its series. An episode the series job
// already wrote makes no provider call.
func (s *Service) matchEpisode(ctx context.Context, item systemq.Item) error {
	if item.MetadataState == stateMatched {
		return nil
	}
	series, ok, err := s.seriesOf(ctx, item)
	if err != nil {
		return err
	}
	if !ok || series.MetadataState != stateMatched {
		return s.setState(ctx, item, stateSkipped, errSeriesUnmatched)
	}
	prov, pid, ok := s.seriesProvider(series)
	if !ok {
		return s.setState(ctx, item, stateSkipped, errSeriesUnmatched)
	}
	if !item.SeasonNumber.Valid || !item.EpisodeNumber.Valid {
		return s.setState(ctx, item, stateUnmatched, "")
	}
	sn, en := int(item.SeasonNumber.Int64), int(item.EpisodeNumber.Int64)
	first, err := prov.Episode(ctx, pid, sn, en, textLangs)
	if err != nil {
		return s.providerFailure(ctx, item, prov.Name(), err)
	}
	recs := []EpisodeRecord{first}
	if item.EpisodeEnd.Valid {
		for k := en + 1; k <= int(item.EpisodeEnd.Int64); k++ {
			r, err := prov.Episode(ctx, pid, sn, k, textLangs)
			if errors.Is(err, ErrNotFound) {
				break
			}
			if err != nil {
				return s.providerFailure(ctx, item, prov.Name(), err)
			}
			recs = append(recs, r)
		}
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("metadata.match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	if err := checkSeries(ctx, q, series); err != nil {
		return err
	}
	cur, ok, err := reloadItem(ctx, q, item.ID)
	if err != nil || !ok {
		return err
	}
	if err := s.writeEpisode(ctx, q, cur, recs, s.now().UnixMilli()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("metadata.match: commit %s: %w", item.ID, err)
	}
	if first.StillPath != "" {
		if err := s.enqueueImages(ctx, item.ID, stillPriority); err != nil {
			return fmt.Errorf("metadata.match: %w", err)
		}
	}
	return nil
}

// seriesOf returns the series of an episode: its parent, or the parent of its season.
func (s *Service) seriesOf(ctx context.Context, item systemq.Item) (systemq.Item, bool, error) {
	cur := item
	for range 2 {
		if !cur.ParentID.Valid {
			return systemq.Item{}, false, nil
		}
		p, err := s.read.GetItemSystem(ctx, cur.ParentID.String)
		if errors.Is(err, sql.ErrNoRows) {
			return systemq.Item{}, false, nil
		}
		if err != nil {
			return systemq.Item{}, false, fmt.Errorf("metadata.match: load parent of %s: %w", cur.ID, err)
		}
		if p.Kind == "series" {
			return p, !p.DeletedAt.Valid, nil
		}
		cur = p
	}
	return systemq.Item{}, false, nil
}

// applyTitle writes the texts, columns and credits of a movie or series in one transaction,
// keeping locked fields. snap is the item the job resolved rec with: when the reloaded row has
// other provider ids or locked fields (a manual match or refresh meanwhile), nothing is written
// and errItemChanged is returned for a retry. It returns the item with the provider ids as
// written, and false when the item disappeared in the meantime. A changed own rating triggers the
// visibility recheck of open streams after the commit.
func (s *Service) applyTitle(ctx context.Context, snap systemq.Item, rec TitleRecord) (systemq.Item, bool, error) {
	itemID := snap.ID
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return snap, false, fmt.Errorf("metadata.match: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)

	cur, ok, err := reloadItem(ctx, q, itemID)
	if err != nil || !ok {
		return snap, false, err
	}
	if !sameIdentity(cur, snap) {
		return snap, false, fmt.Errorf("metadata.match %s: %w", itemID, errItemChanged)
	}
	stored, err := q.ListItemTextsByItem(ctx, itemID)
	if err != nil {
		return snap, false, fmt.Errorf("metadata.match: load texts %s: %w", itemID, err)
	}
	locked := lockedFields(cur.LockedFields)
	defTitle := func(string) string { return firstNonEmpty(rec.OriginalTitle, cur.Title) }
	for _, t := range buildTexts(cur, stored, locked, rec.Texts, defTitle, true) {
		if err := q.UpsertItemText(ctx, t); err != nil {
			return snap, false, fmt.Errorf("metadata.match: write %s text of %s: %w", t.Lang, itemID, err)
		}
	}
	p, err := titleUpdate(cur, locked, rec)
	if err != nil {
		return snap, false, err
	}
	now := s.now().UnixMilli()
	p.Now = sql.NullInt64{Int64: now, Valid: true}
	if err := q.UpdateItemMatch(ctx, p); err != nil {
		return snap, false, fmt.Errorf("metadata.match: update item %s: %w", itemID, err)
	}
	if !locked["cast"] {
		if err := replaceCredits(ctx, q, itemID, rec.Credits, now); err != nil {
			return snap, false, err
		}
	}
	if cur.Kind == "movie" {
		if err := applyCollection(ctx, q, itemID, rec.Collection, p.ReleaseDate.String, p.Year, now, s.now); err != nil {
			return snap, false, err
		}
	}
	if p.TmdbID != cur.TmdbID || p.TvdbID != cur.TvdbID {
		// Another title at the provider: the provenance and the AniList entry of the old one
		// are worthless; the fill records both anew (review of 29 October 2026).
		if err := q.DeleteItemFieldSources(ctx, itemID); err != nil {
			return snap, false, fmt.Errorf("metadata.match: delete sources of %s: %w", itemID, err)
		}
		if err := q.UpdateItemAniListID(ctx, systemq.UpdateItemAniListIDParams{ID: itemID, Now: now}); err != nil {
			return snap, false, fmt.Errorf("metadata.match: clear anilist id of %s: %w", itemID, err)
		}
		if err := q.SetItemFilledProviders(ctx, systemq.SetItemFilledProvidersParams{ID: itemID, Now: now}); err != nil {
			return snap, false, fmt.Errorf("metadata.match: clear filled list of %s: %w", itemID, err)
		}
	}
	// The fill follows every match (ADR 0015): the title carries the identifier's values now,
	// the fillers of its list come with the job. A film's fill is queued in this transaction; a
	// series queues it after its seasons (matchSeries), because the fill would otherwise run
	// beside the season transactions and wait its attempts away.
	if cur.Kind == "movie" {
		if err := enqueueFillTx(ctx, q, itemID, now); err != nil {
			return snap, false, fmt.Errorf("metadata.match: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return snap, false, fmt.Errorf("metadata.match: commit %s: %w", itemID, err)
	}
	s.log.Info("item matched", "item_id", itemID, "provider", rec.Provider, "provider_id", rec.ID,
		"rating_source", p.RatingSource.String)
	if p.OwnRating != cur.OwnRating && s.recheck != nil {
		// The match is stored; a failed recheck only leaves streams open until the next change.
		if err := s.recheck(ctx); err != nil {
			s.log.Warn("visibility recheck after a rating change failed", "item_id", itemID, "err", err)
		}
	}
	written := cur
	written.TmdbID, written.TvdbID, written.ImdbID = p.TmdbID, p.TvdbID, p.ImdbID
	return written, true, nil
}

// applyCollection records the TMDB collection of a film (E-6) inside the match transaction: the
// collection is upserted by its TMDB id and the film becomes a member, ordered by its release
// date (year·10000 + month·100 + day, else the year alone). The film leaves every other TMDB
// collection, all of them when it has none any more; manual collections keep it. A TMDB
// collection left without members stays and is never visible.
func applyCollection(ctx context.Context, q *systemq.Queries, itemID string, c *CollectionRecord, date string,
	year sql.NullInt64, now int64, clock func() time.Time,
) error {
	keep := ""
	if c != nil && c.TMDBID > 0 {
		id, err := q.UpsertCollection(ctx, systemq.UpsertCollectionParams{
			ID:     ulid.MustNew(ulid.Timestamp(clock()), ulid.DefaultEntropy()).String(),
			TmdbID: sql.NullInt64{Int64: c.TMDBID, Valid: true},
			Name:   firstNonEmpty(c.Name["en"], c.Name["de"], "TMDB "+strconv.FormatInt(c.TMDBID, 10)),
			NameDe: nullString(c.Name["de"]), Overview: nullString(c.Overview["en"]),
			OverviewDe: nullString(c.Overview["de"]), PosterPath: nullString(c.PosterPath), Now: now,
		})
		if err != nil {
			return fmt.Errorf("metadata.match: upsert collection %d: %w", c.TMDBID, err)
		}
		if err := q.UpsertCollectionItem(ctx, systemq.UpsertCollectionItemParams{
			CollectionID: id, ItemID: itemID, Position: collectionPosition(date, year), Now: now,
		}); err != nil {
			return fmt.Errorf("metadata.match: add %s to collection %d: %w", itemID, c.TMDBID, err)
		}
		keep = id
	}
	if err := q.DeleteCollectionItemBySource(ctx, systemq.DeleteCollectionItemBySourceParams{ItemID: itemID, KeepID: keep}); err != nil {
		return fmt.Errorf("metadata.match: leave collections of %s: %w", itemID, err)
	}
	return nil
}

// collectionPosition orders the members of a TMDB collection by release date (YYYY-MM-DD), else
// by year; 0 when neither is known.
func collectionPosition(date string, year sql.NullInt64) int64 {
	if t, err := time.Parse(time.DateOnly, date); err == nil {
		return int64(t.Year())*10000 + int64(t.Month())*100 + int64(t.Day())
	}
	if year.Valid {
		return year.Int64 * 10000
	}
	return 0
}

// replaceCredits replaces the credits of an item; people are upserted by their TMDB ID, people
// without one (TVDB) are not stored.
func replaceCredits(ctx context.Context, q *systemq.Queries, itemID string, credits []Credit, now int64) error {
	if err := q.DeleteItemCredits(ctx, itemID); err != nil {
		return fmt.Errorf("metadata.match: delete credits of %s: %w", itemID, err)
	}
	people := map[int64]int64{}
	for _, c := range credits {
		if c.Person.TMDBID <= 0 || c.Person.Name == "" {
			continue
		}
		pid, ok := people[c.Person.TMDBID]
		if !ok {
			var err error
			pid, err = q.UpsertPerson(ctx, systemq.UpsertPersonParams{
				TmdbID: sql.NullInt64{Int64: c.Person.TMDBID, Valid: true}, Name: c.Person.Name,
				ProfilePath: nullString(c.Person.ProfilePath), Now: now,
			})
			if err != nil {
				return fmt.Errorf("metadata.match: upsert person %d: %w", c.Person.TMDBID, err)
			}
			people[c.Person.TMDBID] = pid
		}
		if err := q.InsertCredit(ctx, systemq.InsertCreditParams{
			ItemID: itemID, PersonID: pid, Role: c.Role, Character: nullString(c.Character), Ord: int64(c.Order),
		}); err != nil {
			return fmt.Errorf("metadata.match: insert credit of %s: %w", itemID, err)
		}
	}
	return nil
}

// titleUpdate computes the item columns from a provider record. Locked fields and empty provider
// values keep the current value.
func titleUpdate(cur systemq.Item, locked map[string]bool, rec TitleRecord) (systemq.UpdateItemMatchParams, error) {
	p := systemq.UpdateItemMatchParams{
		ID:               cur.ID,
		OriginalTitle:    cur.OriginalTitle,
		Year:             cur.Year,
		ReleaseDate:      cur.ReleaseDate,
		RuntimeMinutes:   cur.RuntimeMinutes,
		Genres:           cur.Genres,
		TmdbID:           cur.TmdbID,
		TvdbID:           cur.TvdbID,
		ImdbID:           cur.ImdbID,
		OwnRating:        cur.OwnRating,
		RatingSource:     cur.RatingSource,
		TmdbPosterPath:   cur.TmdbPosterPath,
		TmdbBackdropPath: cur.TmdbBackdropPath,
		VoteAverage:      cur.VoteAverage,
	}
	if !locked["original_title"] && rec.OriginalTitle != "" {
		p.OriginalTitle = nullString(rec.OriginalTitle)
	}
	if !locked["release_date"] && rec.Date != "" {
		p.ReleaseDate = nullString(rec.Date)
	}
	if y, ok := yearOf(rec.Date); ok && !locked["year"] {
		p.Year = sql.NullInt64{Int64: int64(y), Valid: true}
	}
	if !locked["runtime_minutes"] && rec.Runtime > 0 {
		p.RuntimeMinutes = sql.NullInt64{Int64: int64(rec.Runtime), Valid: true}
	}
	if !locked["genres"] {
		raw, err := json.Marshal(genres{DE: nonNil(rec.Genres["de"]), EN: nonNil(rec.Genres["en"])})
		if err != nil {
			return p, fmt.Errorf("metadata.match: genres: %w", err)
		}
		p.Genres = string(raw)
	}
	if !locked["tmdb_id"] && rec.TMDBID > 0 {
		p.TmdbID = sql.NullInt64{Int64: rec.TMDBID, Valid: true}
	}
	if !locked["tvdb_id"] && rec.TVDBID > 0 {
		p.TvdbID = sql.NullInt64{Int64: rec.TVDBID, Valid: true}
	}
	if !locked["imdb_id"] && rec.IMDBID != "" {
		p.ImdbID = nullString(rec.IMDBID)
	}
	src := cur.RatingSource.String
	if !locked["own_rating"] && src != "nfo" && src != "manual" {
		rating, country := FSKFromCertifications(rec.Certifications)
		// A rating another provider gave (a filler's isAdult, the other provider's
		// certification) is never lowered by a match of the same title (K-3): only the provider
		// that gave it may revise it, and another title at the provider starts afresh.
		sameTitle := p.TmdbID == cur.TmdbID && p.TvdbID == cur.TvdbID
		ownRating := strings.HasPrefix(src, rec.Provider+":")
		keep := cur.OwnRating.Valid && sameTitle && !ownRating && (rating == nil || int64(*rating) <= cur.OwnRating.Int64)
		if !keep {
			p.OwnRating, p.RatingSource = sql.NullInt64{}, sql.NullString{}
			if rating != nil {
				p.OwnRating = sql.NullInt64{Int64: int64(*rating), Valid: true}
				p.RatingSource = nullString(rec.Provider + ":" + country)
			}
		}
	}
	if !locked[fieldVoteAverage] {
		// The identifier's vote (concept 08 4.2): a list with one provider never runs the field
		// decision of metadata.fill, so the match carries it. Without a vote the value stays (a
		// filler may have given one), unless the match found another title at the provider.
		if v, ok := parseVote(formatVote(rec.VoteAverage)); ok {
			p.VoteAverage = sql.NullFloat64{Float64: v, Valid: true}
		} else if p.TmdbID != cur.TmdbID || p.TvdbID != cur.TvdbID {
			p.VoteAverage = sql.NullFloat64{}
		}
	}
	if rec.PosterPath != "" {
		p.TmdbPosterPath = nullString(rec.PosterPath)
	}
	if rec.BackdropPath != "" {
		p.TmdbBackdropPath = nullString(rec.BackdropPath)
	}
	return p, nil
}

// buildTexts returns the de and en texts. With enFallback, German falls back to the English
// values where the provider has no translation; defTitle names the title when the provider has
// none. Locked fields keep the stored text (or the parsed title).
func buildTexts(cur systemq.Item, stored []systemq.ItemText, locked map[string]bool, texts map[string]Text,
	defTitle func(lang string) string, enFallback bool,
) []systemq.UpsertItemTextParams {
	old := map[string]systemq.ItemText{}
	for _, t := range stored {
		old[t.Lang] = t
	}
	en := texts["en"]
	out := make([]systemq.UpsertItemTextParams, 0, len(textLangs))
	for _, lang := range textLangs {
		m := texts[lang]
		fb := Text{}
		if enFallback {
			fb = en
		}
		t := systemq.UpsertItemTextParams{
			ItemID:   cur.ID,
			Lang:     lang,
			Title:    firstNonEmpty(m.Title, fb.Title, defTitle(lang)),
			Overview: nullString(firstNonEmpty(m.Overview, fb.Overview)),
			Tagline:  nullString(firstNonEmpty(m.Tagline, fb.Tagline)),
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
		t.SortKey = sortKey(lang, t.Title)
		out = append(out, t)
	}
	return out
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

// enqueueJob adds a job with payload {"itemId": id}.
func (s *Service) enqueueJob(ctx context.Context, jobType, keyPrefix, itemID string, priority int) error {
	payload, err := json.Marshal(itemPayload{ItemID: itemID})
	if err != nil {
		return fmt.Errorf("%s payload: %w", jobType, err)
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type:           jobType,
		Priority:       priority,
		IdempotencyKey: keyPrefix + itemID,
		Payload:        payload,
	}); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", jobType, itemID, err)
	}
	return nil
}

func (s *Service) enqueueImages(ctx context.Context, itemID string, priority int) error {
	return s.enqueueJob(ctx, JobImageFetch, "image:", itemID, priority)
}

func (s *Service) enqueueMatch(ctx context.Context, itemID string, priority int) error {
	return s.enqueueJob(ctx, JobMatch, "match:", itemID, priority)
}

// enqueueManualMatch queues metadata.match at the priority of a manual match or refresh. A match
// job of the title that is already queued at a lower priority (a scan's) is cancelled and queued
// again at the manual priority; a running one is left alone, because its write sees the changed
// ids and retries (errItemChanged).
func (s *Service) enqueueManualMatch(ctx context.Context, itemID string) error {
	payload, err := json.Marshal(itemPayload{ItemID: itemID})
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobMatch, err)
	}
	nj := jobs.NewJob{Type: JobMatch, Priority: matchManualPriority, IdempotencyKey: "match:" + itemID, Payload: payload}
	id, err := s.queue.Enqueue(ctx, nj)
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, itemID, err)
	}
	j, err := s.queue.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, itemID, err)
	}
	if j.State != jobs.StateQueued || j.Priority >= matchManualPriority {
		return nil
	}
	if _, err := s.queue.Cancel(ctx, id); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, itemID, err)
	}
	if _, err := s.queue.Enqueue(ctx, nj); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, itemID, err)
	}
	return nil
}

// enqueueTrailerAfterMatch queues trailer.find after a successful movie or series match; the job
// itself skips titles without tmdb_id or with the trailer field locked.
func (s *Service) enqueueTrailerAfterMatch(ctx context.Context, itemID string) error {
	if err := s.enqueueTrailers(ctx, itemID); err != nil {
		return fmt.Errorf("metadata.match: %w", err)
	}
	return nil
}

// maxUpgradeMatches bounds the one-time matches of titles matched before M3 per check.
const maxUpgradeMatches = 200

// QueuePendingMetadata enqueues metadata.match for every title of the library whose metadata is
// still pending without an active match job: movies and series (priority 4) and episodes of
// matched series (priority 3). It also queues one match (priority 4, at most maxUpgradeMatches per
// run) for matched titles from before M3, which lack image paths, credits and trailers
// (ListUpgradeMatchItems), metadata.fill for matched titles with more than one provider whose
// provenance is missing or stale (QueuePendingFills, ADR 0015), and trailer.find for matched
// titles whose trailers are still pending
// (QueuePendingTrailers, for example after the YouTube quota ran out). It also starts
// metadata.vote_backfill once while titles wait for a vote (QueueVoteBackfill, concept 08 4.2; not
// counted, the job is not one of the library's). The library scan calls it after every complete
// run, so enrichment converges after aborted scans, failed jobs and the upgrade. It returns the
// number of jobs queued.
func (s *Service) QueuePendingMetadata(ctx context.Context, libraryID int64) (int, error) {
	rows, err := s.read.ListPendingMatchItems(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list pending metadata: %w", err)
	}
	n := 0
	for _, r := range rows {
		prio := matchPriority
		if r.Kind == "episode" {
			prio = matchEpisodePriority
		}
		if err := s.enqueueMatch(ctx, r.ID, prio); err != nil {
			return n, err
		}
		n++
	}
	// Before the trailers: a queued trailer job keeps a title out of the upgrade list.
	upgrade, err := s.read.ListUpgradeMatchItems(ctx, systemq.ListUpgradeMatchItemsParams{LibraryID: libraryID, LimitCount: maxUpgradeMatches})
	if err != nil {
		return n, fmt.Errorf("list titles matched before M3: %w", err)
	}
	for _, id := range upgrade {
		if err := s.enqueueMatch(ctx, id, matchPriority); err != nil {
			return n, err
		}
		n++
	}
	f, err := s.QueuePendingFills(ctx, libraryID)
	n += f
	if err != nil {
		return n, err
	}
	t, err := s.QueuePendingTrailers(ctx, libraryID)
	n += t
	if err != nil {
		return n, err
	}
	if _, err := s.QueueVoteBackfill(ctx); err != nil {
		return n, err
	}
	return n, nil
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

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
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

// sortKey is item_texts.sort_key of a text row: the shown title in its language, article
// stripped, folded (model.SortKey, concept 08, ADR 0016). Every writer of a text row sets it.
func sortKey(lang, title string) sql.NullString {
	return sql.NullString{String: model.SortKey(lang, title), Valid: true}
}
