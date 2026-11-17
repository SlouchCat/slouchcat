package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The series catalog (concept 25 step 4.1, 08-datenmodell-api.md section 2.10, 03-architektur.md
// section 13): for every series of the library with a TMDB id, the seasons TMDB lists with their
// episode counts and air dates, the status, whether it is in production and the last and next
// episode. visibility reads it for the missing seasons of the series page and the episode count
// that makes a wished season complete; package metadata only fetches and stores it.
//
//   - series.catalog is one job type with two kinds of run. The schedule (daily, priority 7: a
//     short daily job above the bulk rungs, ladder of jobs/queue.go) queues the sweep under the key
//     series.catalog: it fetches every series whose row is missing, was fetched for another TMDB
//     id or is older than SeriesCatalogMaxAge, the missing ones first, at most seriesCatalogBatch
//     per run. QueueSeriesCatalog queues the run of one series (priority 4, like metadata.match of
//     series) under series.catalog:<itemId>; the series match calls it after its seasons, so a
//     newly matched series and one whose TMDB id changed get their catalog at once.
//   - Without a TMDB key a run ends without work (skipped). A key TMDB rejects ends it with an
//     error, so does seriesCatalogFailures transport failures in a row of the sweep (and the one
//     failure of a single run): the queue retries after its backoff.
//   - A series TMDB does not know (404) keeps its row (fetched_at moves on, TouchSeriesCatalog); a
//     series without row gets an empty one. Neither run asks for it again before it is due, so
//     there is no retry storm.
//   - The seasons are stored as TMDB lists them, season 0 (specials) included; the readers leave
//     it out (Anhang I, I-40: season 0 is never offered).
const (
	JobSeriesCatalog = "series.catalog"
	// seriesCatalogKey is the key of the sweep; the run of one series appends ":<itemId>".
	seriesCatalogKey = "series.catalog"
	// seriesCatalogPriority is the scheduled sweep, seriesCatalogEventPriority the run of one series.
	seriesCatalogPriority      = 7
	seriesCatalogEventPriority = 4
	seriesCatalogEvery         = 24 * time.Hour
	// SeriesCatalogMaxAge is the age after which the sweep fetches a series again.
	SeriesCatalogMaxAge   = 7 * 24 * time.Hour
	seriesCatalogBatch    = 1000
	seriesCatalogFailures = 3
)

// QueueSeriesCatalog queues series.catalog for one series of the library (priority 4). A series
// without TMDB id, another kind or a deleted title ends that job without work.
func (s *Service) QueueSeriesCatalog(ctx context.Context, itemID string) error {
	return s.enqueueJob(ctx, JobSeriesCatalog, seriesCatalogKey+":", itemID, seriesCatalogEventPriority)
}

// runSeriesCatalog is the Run of the schedule series.catalog: one sweep under its key.
func (s *Service) runSeriesCatalog(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobSeriesCatalog, Priority: prio, IdempotencyKey: seriesCatalogKey})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobSeriesCatalog, err)
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobSeriesCatalog, err)
	}
	return []int64{id}, nil
}

// handleSeriesCatalog runs the sweep (a job without itemId) or the run of one series.
func (s *Service) handleSeriesCatalog(ctx context.Context, j *jobs.Job) error {
	var p itemPayload
	if len(j.Payload) > 0 {
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return jobs.Final(fmt.Errorf("%s payload: %w", JobSeriesCatalog, err))
		}
	}
	if !s.tmdb.Configured(ctx) {
		jobs.Skip(ctx) // without a TMDB key there is nothing to fetch
		return nil
	}
	if p.ItemID != "" {
		return s.seriesCatalogOne(ctx, p.ItemID)
	}
	return s.seriesCatalogSweep(ctx)
}

// seriesCatalogOne fetches the catalog of one series, whatever the age of its row.
func (s *Service) seriesCatalogOne(ctx context.Context, itemID string) error {
	target, err := s.read.GetSeriesCatalogTarget(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		jobs.Skip(ctx) // gone, not a series or without TMDB id
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: series %s: %w", JobSeriesCatalog, itemID, err)
	}
	if err := s.RefreshSeriesCatalog(ctx, target.ID, target.TmdbID); err != nil && !errors.Is(err, model.ErrNotFound) {
		return fmt.Errorf("%s: %w", JobSeriesCatalog, err)
	}
	return nil
}

// seriesCatalogSweep fetches the due series (ListDueSeriesCatalog).
func (s *Service) seriesCatalogSweep(ctx context.Context) error {
	cutoff := s.now().Add(-SeriesCatalogMaxAge).UnixMilli()
	rows, err := s.read.ListDueSeriesCatalog(ctx, systemq.ListDueSeriesCatalogParams{Cutoff: cutoff, LimitCount: seriesCatalogBatch})
	if err != nil {
		return fmt.Errorf("%s: list due series: %w", JobSeriesCatalog, err)
	}
	failures := 0
	for i, r := range rows {
		jobs.Progress(ctx, int64(i), int64(len(rows)), "")
		err := s.RefreshSeriesCatalog(ctx, r.ID, r.TmdbID)
		switch {
		case err == nil, errors.Is(err, model.ErrNotFound):
			failures = 0 // a series TMDB does not know keeps its row until the next due date
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, model.ErrTMDBKeyInvalid), errors.Is(err, model.ErrProviderNotConfigured):
			return fmt.Errorf("%s: %w", JobSeriesCatalog, err)
		default:
			failures++
			s.log.Warn("series catalog refresh failed", "item_id", r.ID, "tmdb_id", r.TmdbID, "err", err)
			if failures >= seriesCatalogFailures {
				return fmt.Errorf("%s: %d failures in a row: %w", JobSeriesCatalog, failures, err)
			}
		}
	}
	jobs.Progress(ctx, int64(len(rows)), int64(len(rows)), "")
	return nil
}

// RefreshSeriesCatalog fetches the TMDB details of series itemID (TMDB id tmdbID) and stores its
// catalog. A series TMDB does not know keeps its row with fetched_at moved on (a row of another
// TMDB id is emptied) and the answer is model.ErrNotFound; without a key it is
// model.ErrProviderNotConfigured, else the errors of the fetch (model.ErrTMDBKeyInvalid,
// model.ErrTMDBUnreachable).
func (s *Service) RefreshSeriesCatalog(ctx context.Context, itemID string, tmdbID int64) error {
	if tmdbID <= 0 {
		return model.ErrNotFound
	}
	c, err := s.tmdb.keyed(ctx)
	if errors.Is(err, errNotConfigured) {
		return model.ErrProviderNotConfigured
	}
	if err != nil {
		return err
	}
	lang := "en"
	if st, err := s.Settings(ctx); err != nil {
		return err
	} else if len(st.TextLangs) > 0 {
		lang = st.TextLangs[0]
	}
	q := systemq.New(s.db.Write)
	now := s.now().UnixMilli()
	tv, err := c.TV(ctx, tmdbID, tmdbLang(lang), nil)
	if errors.Is(err, ErrNotFound) {
		if terr := q.TouchSeriesCatalog(ctx, systemq.TouchSeriesCatalogParams{ItemID: itemID, TmdbID: tmdbID, FetchedAt: now}); terr != nil {
			return fmt.Errorf("touch series catalog %s: %w", itemID, terr)
		}
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("series catalog %s (tmdb %d): %w", itemID, tmdbID, err)
	}
	p := systemq.UpsertSeriesCatalogParams{ItemID: itemID, TmdbID: tmdbID, Status: trimmedNull(tv.Status), FetchedAt: now}
	if tv.InProduction {
		p.InProduction = 1
	}
	seasons := make([]ExternalSeason, 0, len(tv.Seasons))
	for _, se := range tv.Seasons {
		if se.SeasonNumber < 0 {
			continue
		}
		seasons = append(seasons, ExternalSeason{Number: se.SeasonNumber, EpisodeCount: max(se.EpisodeCount, 0), AirDate: validDate(se.AirDate)})
	}
	if p.Seasons, err = marshalJSON(seasons); err != nil {
		return err
	}
	if p.LastEpisode, err = episodeJSON(tv.LastEpisodeToAir); err != nil {
		return err
	}
	if p.NextEpisode, err = episodeJSON(tv.NextEpisodeToAir); err != nil {
		return err
	}
	if err := q.UpsertSeriesCatalog(ctx, p); err != nil {
		return fmt.Errorf("store series catalog %s: %w", itemID, err)
	}
	return nil
}
