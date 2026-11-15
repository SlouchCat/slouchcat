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
	"github.com/slouchcat/slouchcat/server/internal/store/certq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// metadata.ratings_reread «Freigaben neu lesen» (concept 22 section 4.11, F-16, F-17, ADR 0034):
// the ratings of the providers are read anew, once after the update to minimum ages (F-17: PG-13
// was 12 and is 13 now) and after every change of the rating country (F-16). Unlike the scan,
// which only raises a rating (K-3, decisions 13 and 16), this job may lower it too: that is the
// exception ADR 0034 makes.
//
// What is read: films and series whose rating_source is tmdb:* or tvdb:* or that have no rating
// at all, without the lock own_rating (ListRatingRereadItems). Ratings from an NFO, by hand, from
// the library, the tags, the feed or AniList stay as they are (review 2, M2-1).
//
// Every provider of the list that has the title's id must be read; when one of them is not
// readable (no key, a rejected key, no stored certificates), the title keeps its value: the
// strictest over the providers is unknown, and the readable rest could only lower it.
//
// How: per title and provider of its list (TMDB, TVDB) the job takes the certificates stored in
// item_certifications. A provider without rows is asked once (one request with the first text
// language; the rate limiters of the providers apply), and what it delivers is stored, an empty
// answer as the row "asked, nothing", so the next run computes locally. A provider without the
// title's id does not count. The age of each provider follows ageFromCertifications (rating
// country first, then DE, AT, CH, GB, US); the strictest over the providers wins, as in the fill.
// Every provider with the title's id must be read: when one is not readable (no key, a rejected
// key, no stored certificates), the title keeps its value, because the readable rest could only
// lower the strictest. A title whose providers were all read but know no certificate becomes
// unrated (NULL, AN-1).
//
// A title whose read fails (a provider error) keeps its value, is logged and counted as passed;
// the batch goes on.
//
// One run takes a batch of ratingsRereadBatch titles in id order after the cursor of its payload
// and queues its successor with the cursor (key ratings_reread:<last id>), so a restart goes on
// where the last batch stopped. After a batch that changed a rating the open streams are checked
// once (WithVisibilityRecheck); downloads follow at the next sync (allowed_profile_ids, rule 12).
// Until the chain is done the old values apply, the administration shows the job as running.
const (
	JobRatingsReread   = model.JobRatingsReread
	ratingsRereadKey   = "ratings_reread"
	ratingsRereadBatch = 200
	// SettingRatingsRereadDone marks that a chain read every title to the end (written by its last
	// batch); until then the first start after migration 00064 queues the chain again.
	SettingRatingsRereadDone = "metadata.ratings_reread_done"
)

// certificationProviders are the providers whose certificates the table keeps.
var certificationProviders = []string{model.ProviderTMDB, model.ProviderTVDB}

// ratingsRereadPayload is the payload of metadata.ratings_reread: the id the batch starts after
// and the progress of the chain (titles read so far, titles at the start of the chain).
type ratingsRereadPayload struct {
	After string `json:"after,omitempty"`
	Done  int64  `json:"done,omitempty"`
	Total int64  `json:"total,omitempty"`
}

// QueueRatingsRereadOnce queues the chain at a start after migration 00064 until one chain has
// read every title (SettingRatingsRereadDone, written by its last batch): a chain that failed or
// was cancelled starts again at the next start. A chain that is still queued or running is not
// doubled. It reports whether it queued.
func (s *Service) QueueRatingsRereadOnce(ctx context.Context) (bool, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingRatingsRereadDone); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read %s: %w", SettingRatingsRereadDone, err)
	}
	if running, err := s.RatingsRereading(ctx); err != nil || running {
		return false, err
	}
	return s.QueueRatingsReread(ctx)
}

// markRatingsRereadDone remembers that a chain read every title.
func (s *Service) markRatingsRereadDone(ctx context.Context) error {
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingRatingsRereadDone, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("write %s: %w", SettingRatingsRereadDone, err)
	}
	return nil
}

// QueueRatingsReread starts the chain from the first title when a title waits (a change of the
// rating country, F-16); without titles nothing is queued (the wizard before the first scan, m2-11).
// A start that still waits is not doubled (idempotency key ratings_reread). It reports whether
// it queued.
func (s *Service) QueueRatingsReread(ctx context.Context) (bool, error) {
	total, err := s.read.CountRatingRereadItems(ctx)
	if err != nil {
		return false, fmt.Errorf("count titles to read anew: %w", err)
	}
	if total == 0 {
		return false, s.markRatingsRereadDone(ctx) // nothing to read: every title is read
	}
	return true, s.enqueueRatingsReread(ctx, ratingsRereadPayload{Total: total})
}

// RatingsRereading reports whether a run of the chain is queued or running (the hint «Freigaben
// werden neu gelesen» of the administration).
func (s *Service) RatingsRereading(ctx context.Context) (bool, error) {
	if _, err := s.read.GetActiveJobIDByType(ctx, JobRatingsReread); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check %s: %w", JobRatingsReread, err)
	}
	return false, nil
}

func (s *Service) enqueueRatingsReread(ctx context.Context, p ratingsRereadPayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobRatingsReread, err)
	}
	key := ratingsRereadKey
	if p.After != "" {
		key += ":" + p.After
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobRatingsReread, Priority: fillPriority, IdempotencyKey: key, Payload: payload,
	}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobRatingsReread, err)
	}
	return nil
}

func (s *Service) handleRatingsReread(ctx context.Context, j *jobs.Job) error {
	var p ratingsRereadPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobRatingsReread, err)
	}
	if p.After != "" && !isULID(p.After) {
		return fmt.Errorf("%s payload: invalid after %q", JobRatingsReread, p.After)
	}
	next, err := s.ratingsReread(ctx, p)
	if err != nil || next == nil {
		return err
	}
	return s.enqueueRatingsReread(ctx, *next)
}

// rereadOutcome tells what the job did with one title.
type rereadOutcome int

const (
	rereadPassed    rereadOutcome = iota // no provider could be read: the value stays
	rereadUnchanged                      // read, the same rating
	rereadChanged                        // read, another rating (lower, higher or unknown)
)

// ratingsReread runs one batch. It returns the payload of the successor, nil after the last batch.
func (s *Service) ratingsReread(ctx context.Context, p ratingsRereadPayload) (*ratingsRereadPayload, error) {
	rows, err := s.read.ListRatingRereadItems(ctx, systemq.ListRatingRereadItemsParams{AfterID: p.After, LimitCount: ratingsRereadBatch})
	if err != nil {
		return nil, fmt.Errorf("%s: list titles: %w", JobRatingsReread, err)
	}
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", JobRatingsReread, err)
	}
	var changed, unchanged, passed int
	for i, item := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out, err := s.rereadItem(ctx, item, settings)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			// One title the provider keeps failing (an error answer, a broken record) must not stop
			// the chain: it keeps its value and is read again by the next run.
			s.log.Warn("rating not read anew", "item_id", item.ID, "err", err)
			out = rereadPassed
		}
		switch out {
		case rereadChanged:
			changed++
		case rereadUnchanged:
			unchanged++
		default:
			passed++
		}
		done := p.Done + int64(i) + 1
		jobs.Progress(ctx, done, max(p.Total, done), "")
	}
	s.log.Info("ratings read anew", "titles", len(rows), "changed", changed, "unchanged", unchanged, "passed", passed,
		"rating_country", settings.RatingCountry)
	if changed > 0 && s.recheck != nil {
		if err := s.recheck(ctx); err != nil {
			s.log.Warn("visibility recheck after reading the ratings anew failed", "err", err)
		}
	}
	if len(rows) < ratingsRereadBatch {
		s.log.Info("ratings read anew: finished")
		return nil, s.markRatingsRereadDone(ctx)
	}
	return &ratingsRereadPayload{After: rows[len(rows)-1].ID, Done: p.Done + int64(len(rows)), Total: p.Total}, nil
}

// rereadItem computes and writes the rating of one title.
func (s *Service) rereadItem(ctx context.Context, item systemq.Item, settings Settings) (rereadOutcome, error) {
	stored, err := s.certificationsOf(ctx, item.ID)
	if err != nil {
		return rereadPassed, err
	}
	order := model.ParseProviders(item.Providers)
	if len(order) == 0 {
		order = []string{model.ProviderTMDB}
	}
	type candidate struct {
		age    int
		source string
	}
	var best *candidate
	read := false
	for _, provider := range order {
		if !slices.Contains(certificationProviders, provider) {
			continue
		}
		certs, ok := stored[provider]
		if !ok {
			if !certificationApplies(item, provider) {
				continue // no id at this provider: it cannot rate the title
			}
			certs, ok, err = s.fetchCertifications(ctx, item, provider, settings.TextLangs)
			if err != nil {
				return rereadPassed, err
			}
		}
		if !ok {
			// A provider that could rate the title is not readable now (no key, a rejected key):
			// the strictest value over the providers cannot be known, so the title keeps its
			// value rather than lowering it from the rest of the list.
			return rereadPassed, nil
		}
		read = true
		if age, country := ageFromCertifications(certs, settings.RatingCountry); age != nil && (best == nil || *age > best.age) {
			best = &candidate{*age, provider + ":" + country}
		}
	}
	if !read {
		return rereadPassed, nil
	}
	rating, source := sql.NullInt64{}, sql.NullString{}
	if best != nil {
		rating, source = sql.NullInt64{Int64: int64(best.age), Valid: true}, nullString(best.source)
	}
	if rating == item.OwnRating && source == item.RatingSource {
		return rereadUnchanged, nil
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return rereadPassed, fmt.Errorf("%s: begin: %w", JobRatingsReread, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	cur, found, err := reloadItem(ctx, q, item.ID)
	if err != nil || !found {
		return rereadPassed, err
	}
	// A match, a fill or the administrator came in meanwhile: they decide themselves.
	if !sameIdentity(cur, item) || cur.OwnRating != item.OwnRating || cur.RatingSource != item.RatingSource ||
		lockedFields(cur.LockedFields)["own_rating"] {
		return rereadPassed, nil
	}
	if err := q.SetItemProviderRating(ctx, systemq.SetItemProviderRatingParams{
		OwnRating: rating, RatingSource: source, Now: s.now().UnixMilli(), ID: cur.ID,
	}); err != nil {
		return rereadPassed, fmt.Errorf("%s: write rating of %s: %w", JobRatingsReread, cur.ID, err)
	}
	if best != nil {
		provider, _, _ := strings.Cut(best.source, ":")
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
			ItemID: cur.ID, Field: "own_rating", Provider: provider,
		}); err != nil {
			return rereadPassed, fmt.Errorf("%s: write source of %s: %w", JobRatingsReread, cur.ID, err)
		}
	} else if err := q.DeleteItemFieldSource(ctx, systemq.DeleteItemFieldSourceParams{ItemID: cur.ID, Field: "own_rating"}); err != nil {
		// Unrated now: no provider gave the rating, so none is its source.
		return rereadPassed, fmt.Errorf("%s: delete source of %s: %w", JobRatingsReread, cur.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return rereadPassed, fmt.Errorf("%s: commit %s: %w", JobRatingsReread, cur.ID, err)
	}
	s.log.Debug("rating read anew", "item_id", cur.ID, "rating_source", source.String)
	return rereadChanged, nil
}

// certificationApplies reports whether provider can rate the title: it has the title's id (TVDB
// for series only, as in the fill).
func certificationApplies(item systemq.Item, provider string) bool {
	switch provider {
	case model.ProviderTMDB:
		return item.TmdbID.Valid && item.TmdbID.Int64 > 0
	case model.ProviderTVDB:
		return item.Kind == "series" && item.TvdbID.Valid && item.TvdbID.Int64 > 0
	}
	return false
}

// certificationsOf returns the stored certificates of a title per provider; a provider with the
// row "asked, nothing" maps to an empty map.
func (s *Service) certificationsOf(ctx context.Context, itemID string) (map[string]map[string]string, error) {
	rows, err := certq.New(s.db.Read).ListItemCertifications(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("%s: list certificates of %s: %w", JobRatingsReread, itemID, err)
	}
	out := map[string]map[string]string{}
	for _, r := range rows {
		if out[r.Provider] == nil {
			out[r.Provider] = map[string]string{}
		}
		if r.Country != "" {
			out[r.Provider][r.Country] = r.Certification
		}
	}
	return out, nil
}

// fetchCertifications asks one provider for the certificates of a title and stores them. ok is
// false when the provider cannot be asked (no id, no key, a rejected key); a provider that no
// longer knows the id counts as asked, without certificates.
func (s *Service) fetchCertifications(ctx context.Context, item systemq.Item, provider string, langs []string) (map[string]string, bool, error) {
	var prov Provider
	var id int64
	switch provider {
	case model.ProviderTMDB:
		prov, id = s.tmdb, item.TmdbID.Int64
	case model.ProviderTVDB:
		if item.Kind != "series" {
			return nil, false, nil
		}
		prov, id = s.tvdb, item.TvdbID.Int64
	default:
		return nil, false, nil
	}
	if id <= 0 || !prov.Configured(ctx) || s.KeyRejected(ctx, provider) {
		return nil, false, nil
	}
	rec, err := s.titleRecord(ctx, prov, item.Kind, strconv.FormatInt(id, 10), langs[:min(1, len(langs))])
	if errors.Is(err, model.ErrTMDBKeyInvalid) {
		s.rejectKey(ctx, provider)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", JobRatingsReread, err)
	}
	certs := map[string]string{}
	if rec != nil {
		certs = rec.Certifications
	}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("%s: begin: %w", JobRatingsReread, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := storeCertifications(ctx, tx, item.ID, provider, certs, s.now().UnixMilli()); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("%s: commit certificates of %s: %w", JobRatingsReread, item.ID, err)
	}
	return certs, true, nil
}

// storeCertifications replaces the certificates one provider delivered for a title inside tx:
// one row per rating country with a value, or the row "asked, nothing" (country ”). Other
// providers and countries outside model.RatingSystems are not stored.
func storeCertifications(ctx context.Context, tx *sql.Tx, itemID, provider string, certs map[string]string, now int64) error {
	if !slices.Contains(certificationProviders, provider) {
		return nil
	}
	q := certq.New(tx)
	if err := q.DeleteItemCertifications(ctx, certq.DeleteItemCertificationsParams{ItemID: itemID, Provider: provider}); err != nil {
		return fmt.Errorf("delete certificates of %s: %w", itemID, err)
	}
	wrote := false
	for _, country := range model.RatingCountries() {
		cert := strings.TrimSpace(certs[country])
		if cert == "" || len(cert) > 40 {
			continue
		}
		if err := q.InsertItemCertification(ctx, certq.InsertItemCertificationParams{
			ItemID: itemID, Provider: provider, Country: country, Certification: cert, Now: now,
		}); err != nil {
			return fmt.Errorf("store certificate %s of %s: %w", country, itemID, err)
		}
		wrote = true
	}
	if !wrote {
		if err := q.InsertItemCertification(ctx, certq.InsertItemCertificationParams{
			ItemID: itemID, Provider: provider, Now: now,
		}); err != nil {
			return fmt.Errorf("mark certificates of %s: %w", itemID, err)
		}
	}
	return nil
}
