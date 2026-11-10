package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// «Vereinheitlichen» (concept 19 stage D3, D6.4): items.author_display holds the unified author
// name of an audiobook (NULL: like items.author, which stays the value of the files); display,
// sort by author and search take COALESCE(author_display, author). The series names follow the
// switch metadata.books.normalize_series (audioseries.Names.Shown). One job recomputes both,
// library.author_display, queued after a scan that touched audiobooks, after a probe that changed
// a book's author, once at the first start after the update, and by package metadata after a match
// and a change of either switch (QueueAuthorDisplay).

// JobAuthorDisplay recomputes the unified author and series names (RecomputeAuthorDisplay); no
// payload, key author_display.
const JobAuthorDisplay = "library.author_display"

// prioAuthorDisplay is that of a match: the job is one short transaction over the audiobooks, and
// at priority 1 it waited behind the image backlog of a large library for hours (CT 190, 10 Nov 2026).
const prioAuthorDisplay = prioMatch

// authorDisplayKey is the idempotency key of JobAuthorDisplay; a request while the job runs
// queues one more under authorDisplayNextKey, so a change written after the running job read the
// books is not lost.
const (
	authorDisplayKey     = "author_display"
	authorDisplayNextKey = "author_display:next"
)

// SettingAuthorDisplayQueued marks that library.author_display was queued once after migration
// 00046 (the first start after the update).
const SettingAuthorDisplayQueued = "library.author_display_queued"

// QueueAuthorDisplay queues library.author_display. A job that waits already covers the request
// (raised to prioAuthorDisplay, should it wait at a lower one from an older release); one that runs
// may have read the books before the change, so one more is queued behind it.
func (s *Service) QueueAuthorDisplay(ctx context.Context) error {
	for _, key := range []string{authorDisplayKey, authorDisplayNextKey} {
		id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobAuthorDisplay, Priority: prioAuthorDisplay, IdempotencyKey: key})
		if err != nil {
			return err
		}
		j, err := s.queue.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("load %s job: %w", JobAuthorDisplay, err)
		}
		if j.State != jobs.StateRunning {
			if _, err := s.queue.RaiseQueuedPriority(ctx, id, prioAuthorDisplay); err != nil {
				return fmt.Errorf("raise %s job: %w", JobAuthorDisplay, err)
			}
			return nil
		}
	}
	return nil // both run; the later one started after this request's change was written
}

// queueAuthorDisplayOnce queues library.author_display at the first start after migration 00046
// (SettingAuthorDisplayQueued), in one transaction with the flag. Every start raises a job that
// still waits at a lower priority of an older build to prioAuthorDisplay.
func (s *Service) queueAuthorDisplayOnce(ctx context.Context) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	now := s.now()
	for _, key := range []string{authorDisplayKey, authorDisplayNextKey} {
		if _, err := q.RaiseQueuedJobPriorityByKey(ctx, systemq.RaiseQueuedJobPriorityByKeyParams{
			Priority: prioAuthorDisplay, Now: now.UnixMilli(), IdempotencyKey: sql.NullString{String: key, Valid: true},
		}); err != nil {
			return fmt.Errorf("raise %s job: %w", JobAuthorDisplay, err)
		}
	}
	if _, err := q.GetSystemSetting(ctx, SettingAuthorDisplayQueued); err == nil {
		return tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read %s: %w", SettingAuthorDisplayQueued, err)
	}
	if err := enqueueTx(ctx, q, now, newJob{JobAuthorDisplay, prioAuthorDisplay, authorDisplayKey, struct{}{}}); err != nil {
		return err
	}
	if err := q.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{Key: SettingAuthorDisplayQueued, Value: "1", Now: now.UnixMilli()}); err != nil {
		return fmt.Errorf("write %s: %w", SettingAuthorDisplayQueued, err)
	}
	return tx.Commit()
}

// handleAuthorDisplay runs RecomputeAuthorDisplay in one transaction.
func (s *Service) handleAuthorDisplay(ctx context.Context, _ *jobs.Job) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := RecomputeAuthorDisplay(ctx, systemq.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unified names: %w", err)
	}
	return nil
}

// switchOn reads a switch of «Vereinheitlichen» (absent: on).
func switchOn(ctx context.Context, q *systemq.Queries, key string) (bool, error) {
	v, err := q.GetSystemSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", key, err)
	}
	return audioseries.SwitchOn(v, true), nil
}

// shownSeriesNames returns the Names that name series under the switch normalize_series.
func shownSeriesNames(ctx context.Context, q *systemq.Queries, names audioseries.Names) (audioseries.Names, error) {
	on, err := switchOn(ctx, q, audioseries.SettingNormalizeSeries)
	if err != nil {
		return audioseries.Names{}, err
	}
	return names.Shown(on), nil
}

// RecomputeAuthorDisplay sets items.author_display of every audiobook that is not deleted and the
// name of every series that is not locked, inside the transaction of q (D6.4).
//
// Authors: every name of an author field (split like audioseries.Author, so the first name's key
// is the first-author key of the series key) belongs to the group of its key (dots and hyphens
// as spaces, case-folded, NFC). The name of a group is the most common spelling among the books
// whose author a source wrote (provenance other than local) or the administrator locked, else the
// most common spelling of all its books; a tie goes to the spelling that sorts first. The initials
// of every spelling are written «J. R. R.» before they are counted (audioseries.FormatInitials),
// so «J R R Tolkien» and «J.R.R. Tolkien» count as one spelling, and a hyphen in a first name
// follows the group's name («Marc Uwe Kling» is «Marc-Uwe Kling» when that is the group's name).
// A field keeps its separators and order; only its names change. author_display is NULL when the
// unified field equals items.author, and NULL for every book with the switch
// metadata.books.normalize_authors off. Only changed rows are written.
//
// Series: the unlocked names follow the most common spelling of their books, without suffix with
// the switch metadata.books.normalize_series on, with it when off; the keys stay.
func RecomputeAuthorDisplay(ctx context.Context, q *systemq.Queries) error {
	now := time.Now()
	ms := now.UnixMilli()
	if err := recomputeAuthors(ctx, q, ms); err != nil {
		return err
	}
	names, err := shownSeriesNames(ctx, q, audioseries.Default())
	if err != nil {
		return err
	}
	ids, err := q.ListUnlockedAudioSeriesIDs(ctx)
	if err != nil {
		return fmt.Errorf("list audio series: %w", err)
	}
	for _, id := range ids {
		if err := refreshAudioSeriesName(ctx, q, names, id, ms); err != nil {
			return err
		}
	}
	return nil
}

func recomputeAuthors(ctx context.Context, q *systemq.Queries, ms int64) error {
	on, err := switchOn(ctx, q, audioseries.SettingNormalizeAuthors)
	if err != nil {
		return err
	}
	rows, err := q.ListAudiobookAuthors(ctx)
	if err != nil {
		return fmt.Errorf("list audiobook authors: %w", err)
	}
	want := make([]sql.NullString, len(rows))
	if on {
		want = unifiedAuthors(rows)
	}
	for i, r := range rows {
		if want[i] == r.AuthorDisplay {
			continue
		}
		if _, err := q.UpdateAuthorDisplay(ctx, systemq.UpdateAuthorDisplayParams{AuthorDisplay: want[i], Now: ms, ID: r.ID}); err != nil {
			return fmt.Errorf("unify author of %s: %w", r.ID, err)
		}
	}
	return nil
}

// unifiedAuthors returns the author_display of every row (see RecomputeAuthorDisplay).
func unifiedAuthors(rows []systemq.ListAudiobookAuthorsRow) []sql.NullString {
	type group struct{ all, preferred []string }
	groups := map[string]*group{}
	split := make([][]audioseries.AuthorPart, len(rows))
	for i, r := range rows {
		split[i] = audioseries.SplitAuthors(r.Author.String)
		preferred := (r.AuthorProvider.Valid && r.AuthorProvider.String != model.ProviderLocal) || lockedSet(r.LockedFields)["author"]
		for _, p := range split[i] {
			key := audioseries.AuthorKey(p.Name)
			if key == "" {
				continue
			}
			g := groups[key]
			if g == nil {
				g = &group{}
				groups[key] = g
			}
			spelling := audioseries.FormatInitials(p.Name)
			g.all = append(g.all, spelling)
			if preferred {
				g.preferred = append(g.preferred, spelling)
			}
		}
	}
	chosen := make(map[string]string, len(groups))
	for key, g := range groups {
		if len(g.preferred) > 0 {
			chosen[key] = audioseries.MostCommon(g.preferred)
		} else {
			chosen[key] = audioseries.MostCommon(g.all)
		}
	}
	out := make([]sql.NullString, len(rows))
	for i, r := range rows {
		var b strings.Builder
		for _, p := range split[i] {
			name := p.Name
			if n := chosen[audioseries.AuthorKey(p.Name)]; n != "" {
				name = n
			}
			b.WriteString(p.Lead + name + p.Trail + p.Sep)
		}
		if v := strings.TrimSpace(b.String()); v != r.Author.String {
			out[i] = sql.NullString{String: v, Valid: true}
		}
	}
	return out
}
