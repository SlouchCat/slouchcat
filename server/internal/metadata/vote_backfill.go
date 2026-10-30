package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// metadata.vote_backfill (concept 08 4.2 and 12.6, ADR 0016): items.vote_average came with
// migration 00026. Every match (the identifier's vote, titleUpdate) and every fill (the field
// vote_average in the list order of ADR 0015) writes it from then on; this job fills the titles
// matched before.
//
// One run takes a batch of voteBackfillBatch matched movies and series in id order after the
// cursor of its payload (ListVoteBackfillItems: no vote yet, a list that names a provider with
// votes, TMDB or AniList, vote_average not locked, no provenance row for it). Per title it asks
// the vote providers of the list through fetchTitleRecords (the rate limiters of the providers
// apply, a provider without key is skipped) and decides the field with decideFields, so the fill
// rule holds: the first provider of the list with a vote wins. One transaction per title writes
// the vote and its item_field_sources row.
//
// A title whose providers give no vote gets the provenance row vote_average naming its
// identifier while the column stays NULL: "asked, no vote". The query skips such titles, so a
// later run does not ask again; writeFill ignores the row as long as the column is NULL, so a
// provider that has a vote later still writes it in the next fill, and a match that finds another
// title at the provider deletes the row with the rest of the provenance. Titles whose identifier
// is not configured or whose match job is active are passed over without a mark.
//
// Before the batch ends, the job queues its successor with the cursor (key
// vote_backfill:<last id>), so a restart continues where the last run stopped. QueueVoteBackfill
// starts a chain (key vote_backfill) only when no job of the type is active; QueuePendingMetadata
// calls it after every scan, the first of them right after the server start. A failed run is
// retried by the queue from its cursor; titles it already wrote are no longer in the query.
const (
	JobVoteBackfill   = "metadata.vote_backfill"
	voteBackfillKey   = "vote_backfill"
	voteBackfillBatch = 200
)

// voteProviders are the providers that deliver a vote average (TVDB v4 has none, tvdb.go).
var voteProviders = []string{model.ProviderTMDB, model.ProviderAniList}

// voteBackfillPayload is the payload of metadata.vote_backfill: the id the batch starts after.
type voteBackfillPayload struct {
	After string `json:"after,omitempty"`
}

// QueueVoteBackfill queues metadata.vote_backfill when no job of the type is queued or running
// and at least one title waits for a vote. It reports whether it queued.
func (s *Service) QueueVoteBackfill(ctx context.Context) (bool, error) {
	if _, err := s.read.GetActiveJobIDByType(ctx, JobVoteBackfill); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check %s: %w", JobVoteBackfill, err)
	}
	rows, err := s.read.ListVoteBackfillItems(ctx, systemq.ListVoteBackfillItemsParams{LimitCount: 1})
	if err != nil {
		return false, fmt.Errorf("list titles without vote: %w", err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	return true, s.enqueueVoteBackfill(ctx, "")
}

// enqueueVoteBackfill queues one batch after the id after ("" starts the chain).
func (s *Service) enqueueVoteBackfill(ctx context.Context, after string) error {
	payload, err := json.Marshal(voteBackfillPayload{After: after})
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobVoteBackfill, err)
	}
	key := voteBackfillKey
	if after != "" {
		key += ":" + after
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobVoteBackfill, Priority: fillPriority, IdempotencyKey: key, Payload: payload,
	}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobVoteBackfill, err)
	}
	return nil
}

func (s *Service) handleVoteBackfill(ctx context.Context, j *jobs.Job) error {
	var p voteBackfillPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("%s payload: %w", JobVoteBackfill, err)
	}
	if p.After != "" && !isULID(p.After) {
		return fmt.Errorf("%s payload: invalid after %q", JobVoteBackfill, p.After)
	}
	last, err := s.voteBackfill(ctx, p.After)
	if err != nil {
		return err
	}
	if last == "" {
		return nil
	}
	return s.enqueueVoteBackfill(ctx, last)
}

// voteBackfill runs one batch after the id after. It returns the last id of a full batch (the
// cursor of the successor), "" when the batch was the last one.
func (s *Service) voteBackfill(ctx context.Context, after string) (string, error) {
	rows, err := s.read.ListVoteBackfillItems(ctx, systemq.ListVoteBackfillItemsParams{AfterID: after, LimitCount: voteBackfillBatch})
	if err != nil {
		return "", fmt.Errorf("%s: list titles: %w", JobVoteBackfill, err)
	}
	configured := map[string]bool{
		model.ProviderTMDB: s.tmdb.Configured(ctx), model.ProviderTVDB: s.tvdb.Configured(ctx),
		model.ProviderAniList: s.anilist.Configured(ctx), model.ProviderLocal: true,
	}
	var votes, marked, passed int
	for _, item := range rows {
		wrote, err := s.voteBackfillItem(ctx, item, configured)
		if err != nil {
			return "", err
		}
		switch wrote {
		case voteWritten:
			votes++
		case voteMarked:
			marked++
		default:
			passed++
		}
	}
	s.log.Info("vote backfill batch", "titles", len(rows), "votes", votes, "without_vote", marked, "passed", passed)
	if len(rows) < voteBackfillBatch {
		s.log.Info("vote backfill finished")
		return "", nil
	}
	return rows[len(rows)-1].ID, nil
}

// voteOutcome tells what the backfill did with one title.
type voteOutcome int

const (
	votePassed  voteOutcome = iota // nothing written, the title comes again in a later chain
	voteWritten                    // vote_average and its provenance written
	voteMarked                     // no provider had a vote: the provenance row marks the title
)

// voteBackfillItem asks the vote providers of one title's list and writes the outcome.
func (s *Service) voteBackfillItem(ctx context.Context, item systemq.Item, configured map[string]bool) (voteOutcome, error) {
	order := model.ParseProviders(item.Providers)
	if len(order) == 0 {
		order = []string{model.ProviderTMDB}
	}
	identifier := order[0]
	if !configured[identifier] {
		return votePassed, nil
	}
	if _, err := s.read.GetActiveJobIDByKey(ctx, sql.NullString{String: "match:" + item.ID, Valid: true}); err == nil {
		return votePassed, nil // the match writes the identifier's vote itself
	} else if !errors.Is(err, sql.ErrNoRows) {
		return votePassed, fmt.Errorf("%s: check match job of %s: %w", JobVoteBackfill, item.ID, err)
	}
	var asked []string
	for _, p := range order {
		if slices.Contains(voteProviders, p) {
			asked = append(asked, p)
		}
	}
	recs, _, _, err := s.fetchTitleRecords(ctx, item, asked)
	if err != nil {
		return votePassed, err
	}
	candidates := make([]providerFields, 0, len(recs))
	for _, r := range recs {
		candidates = append(candidates, r.fields)
	}
	w, ok := decideFields(asked, candidates, nil, lockedFields(item.LockedFields))[fieldKey{fieldVoteAverage, ""}]
	vote, valid := 0.0, false
	if ok {
		vote, valid = parseVote(w.value)
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return votePassed, fmt.Errorf("%s: begin: %w", JobVoteBackfill, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	cur, found, err := reloadItem(ctx, q, item.ID)
	if err != nil || !found {
		return votePassed, err
	}
	if !sameIdentity(cur, item) || cur.VoteAverage.Valid {
		return votePassed, nil // a match or fill came in meanwhile and decides itself
	}
	out, provider := voteMarked, identifier
	if valid {
		out, provider = voteWritten, w.provider
		if err := q.SetItemVoteAverage(ctx, systemq.SetItemVoteAverageParams{
			VoteAverage: sql.NullFloat64{Float64: vote, Valid: true}, Now: s.now().UnixMilli(), ID: cur.ID,
		}); err != nil {
			return votePassed, fmt.Errorf("%s: write vote of %s: %w", JobVoteBackfill, cur.ID, err)
		}
	}
	if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
		ItemID: cur.ID, Field: fieldVoteAverage, Provider: provider,
	}); err != nil {
		return votePassed, fmt.Errorf("%s: write source of %s: %w", JobVoteBackfill, cur.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return votePassed, fmt.Errorf("%s: commit %s: %w", JobVoteBackfill, cur.ID, err)
	}
	return out, nil
}
