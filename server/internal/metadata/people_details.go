package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The facts of a person for the person page (concept 21 P-8, migration 00058). The job
// people.details asks TMDB /person/{id} with combined_credits, through the API limiter, for every
// person with a TMDB id and a role on a title that is not deleted, in the main metadata language
// (the first text language): the biography, birthday, deathday, place of birth, known-for
// department and the number of titles of the combined credits (O-14). When the biography is empty
// in the main language, a second request asks for the English one (O-13), stored with biography_lang
// en. queries/system/people_details.sql says what is open: never fetched, fetched before
// peopleDetailsMaxAge, or in another main language.
//
// Without a TMDB key, or with one TMDB rejected (key_rejection.go, concept 21 P-2), a run asks
// nothing and ends skipped; a 401 during the run remembers the rejection and ends the run without
// error. A 404 stores the person without facts; another answer that would come again the same way
// keeps the facts and only marks the fetch. A failure of the transport (5xx, 429 after the retries,
// a timeout, the network) writes nothing; peopleDetailsFailures of them in a row end the run with
// an error and the queue retries it after its backoff. After the pass the run starts another while
// open people are left that did not fail in this run, at most peopleDetailsPasses passes.
//
// The job is queued like people.images: after a match that wrote credits, at the end of
// QueuePendingMetadata, once at the first start after the update (QueuePeopleRefreshOnce) and when
// a rejected key passes its check again.
const (
	JobPeopleDetails      = "people.details"
	peopleDetailsKey      = "people.details"
	peopleDetailsBatch    = 200
	peopleDetailsPasses   = 3
	peopleDetailsFailures = 3
	// peopleDetailsPriority is the one of people.images: bulk work.
	peopleDetailsPriority = peopleImagesPriority
	// peopleDetailsMaxAge is how long the facts of a fetch stand before a run asks again (a
	// deathday, a new biography).
	peopleDetailsMaxAge = 90 * 24 * time.Hour
)

// errPeopleDetailsFailures: peopleDetailsFailures fetches in a row failed.
var errPeopleDetailsFailures = errors.New("person detail fetches fail")

// enqueuePeopleDetails queues people.details; a queued or running job holds the key.
func (s *Service) enqueuePeopleDetails(ctx context.Context) error {
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobPeopleDetails, Priority: peopleDetailsPriority, IdempotencyKey: peopleDetailsKey}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobPeopleDetails, err)
	}
	return nil
}

// QueuePeopleDetails queues people.details when a person is open and TMDB has a key it does not
// reject, and reports whether it did.
func (s *Service) QueuePeopleDetails(ctx context.Context) (bool, error) {
	if !s.tmdb.Configured(ctx) || s.KeyRejected(ctx, ProviderTMDB) {
		return false, nil
	}
	lang, err := s.mainLanguage(ctx)
	if err != nil {
		return false, err
	}
	open, err := s.peopleDetailsLeft(ctx, lang, nil)
	if err != nil || !open {
		return false, err
	}
	return true, s.enqueuePeopleDetails(ctx)
}

// mainLanguage is the first text language of the server's metadata settings, the language of
// the facts.
func (s *Service) mainLanguage(ctx context.Context) (string, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return "", err
	}
	if len(st.TextLangs) == 0 {
		return "en", nil
	}
	return st.TextLangs[0], nil
}

func (s *Service) detailsStaleBefore() int64 {
	return s.now().Add(-peopleDetailsMaxAge).UnixMilli()
}

// peopleDetailsLeft reports whether an open person is left that is not in failed.
func (s *Service) peopleDetailsLeft(ctx context.Context, lang string, failed map[int64]bool) (bool, error) {
	ids, err := s.read.ListOpenPersonDetails(ctx, systemq.ListOpenPersonDetailsParams{
		StaleBefore: s.detailsStaleBefore(), Lang: lang, LimitCount: int64(len(failed) + 1),
	})
	if err != nil {
		return false, fmt.Errorf("list open person details: %w", err)
	}
	for _, r := range ids {
		if !failed[r.ID] {
			return true, nil
		}
	}
	return false, nil
}

// detailsRun is the state of one run of people.details.
type detailsRun struct {
	client   *Client
	lang     string
	failed   map[int64]bool // people that failed in this run
	failures int            // failures in a row
	fetched  int
	rejected bool // TMDB rejected the key: the run stops
}

// handlePeopleDetails is the handler of people.details.
func (s *Service) handlePeopleDetails(ctx context.Context, _ *jobs.Job) error {
	if s.KeyRejected(ctx, ProviderTMDB) {
		jobs.Skip(ctx) // rejectKey warned once for the key
		return nil
	}
	client, err := s.tmdb.keyed(ctx)
	if errors.Is(err, errNotConfigured) {
		jobs.Skip(ctx)
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", JobPeopleDetails, err)
	}
	lang, err := s.mainLanguage(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", JobPeopleDetails, err)
	}
	run := &detailsRun{client: client, lang: lang, failed: map[int64]bool{}}
	for range peopleDetailsPasses {
		if err := s.peopleDetailsPass(ctx, run); err != nil {
			return err
		}
		if run.rejected {
			break
		}
		left, err := s.peopleDetailsLeft(ctx, lang, run.failed)
		if err != nil {
			return err
		}
		if !left {
			break
		}
	}
	if run.fetched == 0 && len(run.failed) == 0 {
		jobs.Skip(ctx)
	}
	s.log.InfoContext(ctx, "person details done", "fetched", run.fetched, "failed", len(run.failed), "key_rejected", run.rejected)
	return nil
}

// peopleDetailsPass walks the open people once by id.
func (s *Service) peopleDetailsPass(ctx context.Context, run *detailsRun) error {
	stale := s.detailsStaleBefore()
	total, err := s.read.CountOpenPersonDetails(ctx, systemq.CountOpenPersonDetailsParams{StaleBefore: stale, Lang: run.lang})
	if err != nil {
		return fmt.Errorf("count open person details: %w", err)
	}
	done := int64(0)
	var after int64
	for {
		rows, err := s.read.ListOpenPersonDetails(ctx, systemq.ListOpenPersonDetailsParams{
			AfterID: after, StaleBefore: stale, Lang: run.lang, LimitCount: peopleDetailsBatch,
		})
		if err != nil {
			return fmt.Errorf("list open person details: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			after = r.ID
			jobs.Progress(ctx, done, max(total, done), "")
			if !run.failed[r.ID] {
				if err := s.fetchPersonDetails(ctx, run, r.ID, r.TmdbID.Int64); err != nil {
					return err
				}
				if run.rejected {
					return nil
				}
			}
			done++
		}
	}
	jobs.Progress(ctx, done, done, "")
	return nil
}

// fetchPersonDetails fetches and stores the facts of one person.
func (s *Service) fetchPersonDetails(ctx context.Context, run *detailsRun, id, tmdbID int64) error {
	if err := ctx.Err(); err != nil {
		return err // a pause or a cancel
	}
	now := s.now().UnixMilli()
	d, bioLang, err := personDetailsIn(ctx, run.client, tmdbID, run.lang)
	switch {
	case err == nil:
		p := personDetailsRow(d, bioLang)
		p.DetailsLang, p.Now, p.ID = run.lang, now, id
		if err := s.write.StorePersonDetails(ctx, p); err != nil {
			return fmt.Errorf("%s: store person %d: %w", JobPeopleDetails, id, err)
		}
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.rejectKey(ctx, ProviderTMDB)
		run.rejected = true
		return nil
	case errors.Is(err, model.ErrTMDBUnreachable):
		run.failed[id] = true
		run.failures++
		s.log.Warn("person details failed", "person_id", id, "err", err)
		if run.failures >= peopleDetailsFailures {
			return fmt.Errorf("%s: %d %w in a row: %w", JobPeopleDetails, run.failures, errPeopleDetailsFailures, err)
		}
		return nil
	case errors.Is(err, ErrNotFound):
		// TMDB does not know the person (any more): no facts.
		if err := s.write.StorePersonDetails(ctx, systemq.StorePersonDetailsParams{DetailsLang: run.lang, Now: now, ID: id}); err != nil {
			return fmt.Errorf("%s: store person %d: %w", JobPeopleDetails, id, err)
		}
	default:
		// Another answer that would come again the same way: the facts stay, the fetch is marked.
		s.log.Warn("person details refused", "person_id", id, "err", err)
		if err := s.write.MarkPersonDetailsFetched(ctx, systemq.MarkPersonDetailsFetchedParams{DetailsLang: run.lang, Now: now, ID: id}); err != nil {
			return fmt.Errorf("%s: mark person %d: %w", JobPeopleDetails, id, err)
		}
	}
	run.failures = 0
	run.fetched++
	return nil
}

// personDetailsIn asks TMDB for the person in lang with the combined credits and, when the
// biography is empty and lang is not English, for the English biography (O-13). bioLang is the
// language of the biography: lang, or en for the fallback.
func personDetailsIn(ctx context.Context, c *Client, tmdbID int64, lang string) (d PersonDetail, bioLang string, err error) {
	d, err = c.Person(ctx, tmdbID, tmdbLang(lang), true)
	if err != nil {
		return PersonDetail{}, "", err
	}
	if strings.TrimSpace(d.Biography) != "" || PrimaryLanguage(lang) == "en" {
		return d, lang, nil
	}
	en, err := c.Person(ctx, tmdbID, langEN, false)
	if err != nil {
		return PersonDetail{}, "", err
	}
	d.Biography = en.Biography
	return d, "en", nil
}

// personDetailsRow maps a TMDB answer to the stored facts: empty texts and dates that are no
// YYYY-MM-DD become NULL.
func personDetailsRow(d PersonDetail, bioLang string) systemq.StorePersonDetailsParams {
	p := systemq.StorePersonDetailsParams{
		Biography: nullText(d.Biography), Birthday: nullDay(d.Birthday), Deathday: nullDay(d.Deathday),
		PlaceOfBirth: nullText(d.PlaceOfBirth), KnownForDepartment: nullText(d.KnownForDepartment),
	}
	if p.Biography.Valid {
		p.BiographyLang = sql.NullString{String: bioLang, Valid: true}
	}
	if n, ok := d.TotalCredits(); ok {
		p.TotalCredits = sql.NullInt64{Int64: int64(n), Valid: true}
	}
	return p
}

func nullText(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

func nullDay(s string) sql.NullString {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// SettingPeopleRefreshed marks that people.images and people.details were queued once after the
// update to concept 21 stage 4 (portraits w300 instead of w185, O-12; the facts of the person
// page).
const SettingPeopleRefreshed = "metadata.people_refreshed"

// QueuePeopleRefreshOnce queues, once at the first start after the update to concept 21 stage 4,
// people.images (its open condition takes every portrait of another size than w300, so the run
// replaces the w185 files, O-12) and people.details (every person is open, P-8), each when
// something is open; without it they wait for the next full scan. The setting remembers a complete
// call; a failed one starts again at the next start. It reports whether a job was queued.
func (s *Service) QueuePeopleRefreshOnce(ctx context.Context) (bool, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingPeopleRefreshed); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read %s: %w", SettingPeopleRefreshed, err)
	}
	images, err := s.QueuePeopleImages(ctx)
	if err != nil {
		return false, err
	}
	details, err := s.QueuePeopleDetails(ctx)
	if err != nil {
		return images, err
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingPeopleRefreshed, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return images || details, fmt.Errorf("write %s: %w", SettingPeopleRefreshed, err)
	}
	if images || details {
		s.log.InfoContext(ctx, "person portraits and facts queued after the update", "images", images, "details", details)
	}
	return images || details, nil
}
