package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/trackpick"
)

// The original language of a title (concept 13 11.7, F-4): items.original_language is NULL while
// it was never looked up, "" when it was looked up and no provider knows it, otherwise an ISO
// 639-1 code. It is a field of the fill order of ADR 0015 like the others: the match writes the
// identifier's value (a list with one provider never runs the field decision), metadata.fill
// decides it in list order with its provenance in item_field_sources, and the lock
// original_language in items.locked_fields keeps it from every writer. The sources are TMDB's
// original_language, TVDB's originalLanguage (ISO 639-3 through the language table) and AniList's
// countryOfOrigin (JP, KR, CN). An unknown value "" never gets a provenance row, so a provider
// that knows the language later may still write it.
//
// metadata.original_language_backfill fills the titles matched before migration 00032: a manual
// job type (Run) in batches of originalLanguageBatch, in id order after the cursor of its payload
// (ListOriginalLanguageBackfillItems: NULL, not locked). Per title it asks the providers of the
// list through fetchTitleRecords (their rate limiters apply, a provider without key is skipped)
// and decides the field with decideFields; one transaction per title writes the value (or "")
// and its provenance. A title whose identifier is not configured or whose match job is active is
// passed over and stays NULL. Before a full batch ends, the job queues its successor with the
// cursor (key original_language_backfill:<last id>). QueueOriginalLanguageBackfillOnce queues the
// chain once at the first start after the migration (the setting
// metadata.original_language_backfill_queued remembers it); later runs start by hand on the page
// Hintergrundjobs.
const (
	JobOriginalLanguageBackfill = "metadata.original_language_backfill"
	fieldOriginalLanguage       = "original_language"
	originalLanguageKey         = "original_language_backfill"
	originalLanguageBatch       = 200
	// SettingOriginalLanguageQueued marks that the backfill was queued once after migration 00032.
	SettingOriginalLanguageQueued = "metadata.original_language_backfill_queued"
)

// languageProviders are the providers that deliver an original language (not the local files).
var languageProviders = []string{model.ProviderTMDB, model.ProviderTVDB, model.ProviderAniList}

// normalizeOriginalLanguage maps a provider's language code onto ISO 639-1 through the language
// table of the track choice (trackpick.Normalize), so the original language compares with the
// tracks and the profiles' lists: the primary subtag in lower case, ISO 639-2 codes onto their
// ISO 639-1 code (eng → en), TMDB's cn (Cantonese) onto Chinese, zh; TMDB's xx (no language) and
// any code trackpick.Valid refuses are "" (unknown).
func normalizeOriginalLanguage(code string) string {
	code = trackpick.Normalize(code)
	if !trackpick.Valid(code) {
		return ""
	}
	return code
}

// countryLanguage maps AniList's countryOfOrigin onto the original language: Japan, Korea and
// China; any other country names no language.
func countryLanguage(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "JP":
		return "ja"
	case "KR":
		return "ko"
	case "CN":
		return "zh"
	}
	return ""
}

// ValidOriginalLanguage reports whether code may be stored as an original language by hand: ""
// (unknown) or an ISO 639-1 code that normalizeOriginalLanguage keeps as it is.
func ValidOriginalLanguage(code string) bool {
	return code == "" || normalizeOriginalLanguage(code) == code
}

// originalLanguagePayload is the payload of metadata.original_language_backfill: the id the batch
// starts after.
type originalLanguagePayload struct {
	After string `json:"after,omitempty"`
}

// runOriginalLanguageBackfill is the Run of the job type: one chain from the start with prio. A
// chain that is already active (its first batch or a successor original_language_backfill:<id>,
// waiting or running) is not doubled: its waiting batch is raised to prio (RaiseQueuedPriority)
// and its ids are returned.
func (s *Service) runOriginalLanguageBackfill(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	active, err := s.queue.ListActive(ctx, JobOriginalLanguageBackfill)
	if err != nil {
		return nil, err
	}
	if len(active) > 0 {
		ids := make([]int64, 0, len(active))
		for _, j := range active {
			if j.State == jobs.StateQueued {
				if _, err := s.queue.RaiseQueuedPriority(ctx, j.ID, prio); err != nil {
					return nil, fmt.Errorf("raise %s: %w", JobOriginalLanguageBackfill, err)
				}
			}
			ids = append(ids, j.ID)
		}
		return ids, nil
	}
	id, err := s.enqueueOriginalLanguageBackfill(ctx, "", prio)
	if err != nil {
		return nil, err
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobOriginalLanguageBackfill, err)
	}
	return []int64{id}, nil
}

// QueueOriginalLanguageBackfillOnce queues the chain at the first start after migration 00032,
// when a title waits, and remembers in the settings that it did; later starts queue nothing. It
// reports whether it queued.
func (s *Service) QueueOriginalLanguageBackfillOnce(ctx context.Context) (bool, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingOriginalLanguageQueued); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read %s: %w", SettingOriginalLanguageQueued, err)
	}
	rows, err := s.read.ListOriginalLanguageBackfillItems(ctx, systemq.ListOriginalLanguageBackfillItemsParams{LimitCount: 1})
	if err != nil {
		return false, fmt.Errorf("list titles without original language: %w", err)
	}
	queued := len(rows) > 0
	if queued {
		if _, err := s.enqueueOriginalLanguageBackfill(ctx, "", fillPriority); err != nil {
			return false, err
		}
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingOriginalLanguageQueued, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return queued, fmt.Errorf("write %s: %w", SettingOriginalLanguageQueued, err)
	}
	return queued, nil
}

// enqueueOriginalLanguageBackfill queues one batch after the id after ("" starts the chain).
func (s *Service) enqueueOriginalLanguageBackfill(ctx context.Context, after string, prio int) (int64, error) {
	payload, err := json.Marshal(originalLanguagePayload{After: after})
	if err != nil {
		return 0, fmt.Errorf("%s payload: %w", JobOriginalLanguageBackfill, err)
	}
	key := originalLanguageKey
	if after != "" {
		key += ":" + after
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobOriginalLanguageBackfill, Priority: prio, IdempotencyKey: key, Payload: payload,
	})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobOriginalLanguageBackfill, err)
	}
	return id, nil
}

func (s *Service) handleOriginalLanguageBackfill(ctx context.Context, j *jobs.Job) error {
	var p originalLanguagePayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobOriginalLanguageBackfill, err)
	}
	if p.After != "" && !isULID(p.After) {
		return fmt.Errorf("%s payload: invalid after %q", JobOriginalLanguageBackfill, p.After)
	}
	last, err := s.originalLanguageBackfill(ctx, p.After)
	if err != nil {
		return err
	}
	if last == "" {
		return nil
	}
	_, err = s.enqueueOriginalLanguageBackfill(ctx, last, fillPriority)
	return err
}

// originalLanguageBackfill runs one batch after the id after. It returns the last id of a full
// batch (the cursor of the successor), "" when the batch was the last one.
func (s *Service) originalLanguageBackfill(ctx context.Context, after string) (string, error) {
	rows, err := s.read.ListOriginalLanguageBackfillItems(ctx, systemq.ListOriginalLanguageBackfillItemsParams{
		AfterID: after, LimitCount: originalLanguageBatch,
	})
	if err != nil {
		return "", fmt.Errorf("%s: list titles: %w", JobOriginalLanguageBackfill, err)
	}
	configured := map[string]bool{
		model.ProviderTMDB: s.tmdb.Configured(ctx), model.ProviderTVDB: s.tvdb.Configured(ctx),
		model.ProviderAniList: s.anilist.Configured(ctx), model.ProviderLocal: true,
	}
	var found, unknown, passed int
	for _, item := range rows {
		out, err := s.originalLanguageItem(ctx, item, configured)
		if err != nil {
			return "", err
		}
		switch out {
		case languageFound:
			found++
		case languageUnknown:
			unknown++
		default:
			passed++
		}
	}
	s.log.Info("original language backfill batch", "titles", len(rows), "found", found, "unknown", unknown, "passed", passed)
	if len(rows) < originalLanguageBatch {
		s.log.Info("original language backfill finished")
		return "", nil
	}
	return rows[len(rows)-1].ID, nil
}

// languageOutcome tells what the backfill did with one title.
type languageOutcome int

const (
	languagePassed  languageOutcome = iota // nothing written, the title stays NULL
	languageFound                          // a provider named the language
	languageUnknown                        // asked, no provider knows it: ''
)

// originalLanguageItem asks the language providers of one title's list and writes the outcome.
func (s *Service) originalLanguageItem(ctx context.Context, item systemq.Item, configured map[string]bool) (languageOutcome, error) {
	order := model.ParseProviders(item.Providers)
	if len(order) == 0 {
		order = []string{model.ProviderTMDB}
	}
	if !configured[order[0]] {
		return languagePassed, nil
	}
	if _, err := s.read.GetActiveJobIDByKey(ctx, sql.NullString{String: "match:" + item.ID, Valid: true}); err == nil {
		return languagePassed, nil // the match writes the identifier's language itself
	} else if !errors.Is(err, sql.ErrNoRows) {
		return languagePassed, fmt.Errorf("%s: check match job of %s: %w", JobOriginalLanguageBackfill, item.ID, err)
	}
	var asked []string
	for _, p := range order {
		if slices.Contains(languageProviders, p) {
			asked = append(asked, p)
		}
	}
	var candidates []providerFields
	if len(asked) > 0 {
		recs, _, _, err := s.fetchTitleRecords(ctx, item, asked)
		if err != nil {
			return languagePassed, err
		}
		for _, r := range recs {
			candidates = append(candidates, r.fields)
		}
	}
	w, ok := decideFields(asked, candidates, nil, lockedFields(item.LockedFields))[fieldKey{fieldOriginalLanguage, ""}]

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return languagePassed, fmt.Errorf("%s: begin: %w", JobOriginalLanguageBackfill, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	cur, live, err := reloadItem(ctx, q, item.ID)
	if err != nil || !live {
		return languagePassed, err
	}
	if !sameIdentity(cur, item) || cur.OriginalLanguage.Valid {
		return languagePassed, nil // a match or fill came in meanwhile and decides itself
	}
	out, value := languageUnknown, ""
	if ok {
		out, value = languageFound, w.value
	}
	if err := q.SetItemOriginalLanguage(ctx, systemq.SetItemOriginalLanguageParams{
		OriginalLanguage: sql.NullString{String: value, Valid: true}, Now: s.now().UnixMilli(), ID: cur.ID,
	}); err != nil {
		return languagePassed, fmt.Errorf("%s: write language of %s: %w", JobOriginalLanguageBackfill, cur.ID, err)
	}
	if ok {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
			ItemID: cur.ID, Field: fieldOriginalLanguage, Provider: w.provider,
		}); err != nil {
			return languagePassed, fmt.Errorf("%s: write source of %s: %w", JobOriginalLanguageBackfill, cur.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return languagePassed, fmt.Errorf("%s: commit %s: %w", JobOriginalLanguageBackfill, cur.ID, err)
	}
	return out, nil
}

// SetItemOriginalLanguage is the administrator's original language of a movie or series
// (setItemOriginalLanguage): a code or "" (unknown) is stored and locked, its provenance row goes
// (the value is the administrator's); nil lifts the lock, clears the value to NULL and queues a
// manual match, which looks the language up again. Errors: model.ErrNotFound,
// *model.ValidationError (language invalid_value, kind invalid_value).
func (s *Service) SetItemOriginalLanguage(ctx context.Context, itemID string, lang *string) error {
	if lang != nil && !ValidOriginalLanguage(*lang) {
		var v *model.ValidationError
		return v.Add("language", "invalid_value")
	}
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.Kind != "movie" && item.Kind != "series" {
		var v *model.ValidationError
		return v.Add("kind", "invalid_value")
	}
	p := systemq.SetItemOriginalLanguageOverrideParams{ID: item.ID, Now: s.now().UnixMilli()}
	if lang != nil {
		p.OriginalLanguage = sql.NullString{String: *lang, Valid: true}
		p.LockedFields = addLocked(item.LockedFields, fieldOriginalLanguage)
	} else {
		p.LockedFields = removeLocked(item.LockedFields, fieldOriginalLanguage)
	}
	err = s.inTx(ctx, func(q *systemq.Queries) error {
		if err := q.SetItemOriginalLanguageOverride(ctx, p); err != nil {
			return err
		}
		return q.DeleteItemFieldSource(ctx, systemq.DeleteItemFieldSourceParams{ItemID: item.ID, Field: fieldOriginalLanguage})
	})
	if err != nil {
		return fmt.Errorf("set original language of %s: %w", item.ID, err)
	}
	s.log.Info("original language set by the admin", "item_id", item.ID, "locked", lang != nil)
	if lang != nil {
		return nil
	}
	return s.enqueueManualMatch(ctx, item.ID)
}
