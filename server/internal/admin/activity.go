package admin

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

// Activity log "Letzte Aktivität" of the admin overview (concept 11 section 11.4, ADR 0019,
// migration 00031). The hooks in playback, library, jobs, admin and main hand their events to
// RecordActivity, which applies the flood rule of the kind and writes at most one row; the
// dashboard reads the newest six, and the cleanup hook of jobs.prune keeps 30 days and at most
// 2'000 rows. A failed write is logged and never aborts the action that caused it.

const (
	// dashboardActivityLimit is how many events the dashboard lists.
	dashboardActivityLimit = 6
	// activityStartWindow: a profile's start of the same title is written once in this window
	// (track, quality and version switches are new playback sessions).
	activityStartWindow = 30 * time.Minute
	// activityScanWindow: at most one scan_failed and one scan_aborted per library and reason in
	// this window.
	activityScanWindow = time.Hour
	// activityRetention and activityKeep bound the log; activityPruneBatch rows per statement.
	activityRetention  = 30 * 24 * time.Hour
	activityKeep       = 2000
	activityPruneBatch = jobs.PruneBatch
	// activityWriteTimeout bounds one write; the caller's context may end before it (a request
	// that answered already, the goroutine of a playback start).
	activityWriteTimeout = 5 * time.Second
)

// knownActivity are the kinds RecordActivity writes.
var knownActivity = map[string]bool{
	model.ActivityPlaybackStarted: true, model.ActivityPlaybackCompleted: true, model.ActivityPlaybackStopped: true,
	model.ActivityScanStarted: true, model.ActivityScanDone: true, model.ActivityScanFailed: true, model.ActivityScanAborted: true,
	model.ActivityJobFailed: true, model.ActivityBackupCreated: true, model.ActivityBackupRestored: true,
	model.ActivityServerStarted: true, model.ActivityServerRestart: true,
	model.ActivityLibraryDeleted: true, model.ActivityItemDeleted: true,
}

// Fixed codes of the detail (rule 10: no free error text in the log).
var (
	activityReasons  = map[string]bool{"root_missing": true, "root_empty": true, "loss_over_threshold": true, abortCancelled: true, "error": true}
	activityTriggers = map[string]bool{model.ScanTriggerManual: true, model.ScanTriggerSchedule: true, model.ScanTriggerWatcher: true}
	activityStoppers = map[string]bool{model.StoppedByAdmin: true, model.StoppedByRevoked: true}
	activityModes    = map[string]bool{"library": true, "files": true}
)

// RecordActivity writes an event to the activity log with the flood rule of its kind:
// playback_started once per profile and title in 30 minutes, scan_failed and scan_aborted once
// per library, reason and hour, job_failed one row per job type and clock hour whose count grows. An
// event of a deleted profile or account is dropped. Errors are logged as WARN, never returned.
func (s *Service) RecordActivity(ctx context.Context, e model.ActivityEvent) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), activityWriteTimeout)
	defer cancel()
	if err := s.recordActivity(ctx, e); err != nil {
		s.log.WarnContext(ctx, "activity not recorded", "kind", e.Kind, "err", err)
	}
}

// RecordJobFailure is the OnFinalFailure hook of the job workers: a job that failed its last
// attempt counts into the job_failed row of its type. The error text stays out of the log.
func (s *Service) RecordJobFailure(ctx context.Context, j jobs.Job, _ error) {
	s.RecordActivity(ctx, model.ActivityEvent{Kind: model.ActivityJobFailed, Detail: model.ActivityDetail{JobType: j.Type}})
}

func (s *Service) recordActivity(ctx context.Context, e model.ActivityEvent) error {
	if !knownActivity[e.Kind] {
		return fmt.Errorf("unknown activity kind %q", e.Kind)
	}
	at := e.At
	if at.IsZero() {
		at = s.now()
	}
	e.Detail = cleanActivityDetail(e.Kind, e.Detail)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	if gone, err := activityOwnerGone(ctx, q, e); err != nil || gone {
		return err
	}
	switch e.Kind {
	case model.ActivityPlaybackStarted:
		n, err := q.CountRecentPlaybackStarts(ctx, systemq.CountRecentPlaybackStartsParams{
			Since: at.Add(-activityStartWindow).UnixMilli(), ProfileID: nullID(e.ProfileID), ItemID: nullText(e.ItemID),
		})
		if err != nil || n > 0 {
			return err
		}
	case model.ActivityScanFailed, model.ActivityScanAborted:
		n, err := q.CountRecentLibraryActivity(ctx, systemq.CountRecentLibraryActivityParams{
			Kind: e.Kind, Since: at.Add(-activityScanWindow).UnixMilli(), LibraryID: nullID(e.LibraryID), Reason: e.Detail.Reason,
		})
		if err != nil || n > 0 {
			return err
		}
	case model.ActivityJobFailed:
		n, err := q.BumpJobFailedActivity(ctx, systemq.BumpJobFailedActivityParams{
			At: at.UnixMilli(), Since: at.Truncate(time.Hour).UnixMilli(), JobType: e.Detail.JobType,
		})
		if err != nil {
			return fmt.Errorf("count job failure: %w", err)
		}
		if n > 0 {
			return tx.Commit()
		}
		e.Detail.Count = 1
	}
	if e.Subject == "" && e.ItemID != "" {
		title, err := q.ActivityItemTitle(ctx, e.ItemID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("title of %s: %w", e.ItemID, err)
		}
		e.Subject = title
	}
	detail, err := json.Marshal(e.Detail)
	if err != nil {
		return fmt.Errorf("encode detail: %w", err)
	}
	if _, err := q.InsertActivityEvent(ctx, systemq.InsertActivityEventParams{
		At: at.UnixMilli(), Kind: e.Kind, AccountID: nullID(e.AccountID), ProfileID: nullID(e.ProfileID),
		ItemID: nullText(e.ItemID), LibraryID: nullID(e.LibraryID), Subject: nullText(e.Subject), Detail: string(detail),
	}); err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	return tx.Commit()
}

// activityOwnerGone reports whether the profile or the account of the event no longer exists
// (deleted while the event was on its way): its rows were deleted with it, so none is added.
func activityOwnerGone(ctx context.Context, q *systemq.Queries, e model.ActivityEvent) (bool, error) {
	var n int64
	var err error
	switch {
	case e.ProfileID != 0:
		n, err = q.CountActivityProfile(ctx, e.ProfileID)
	case e.AccountID != 0:
		n, err = q.CountActivityAccount(ctx, e.AccountID)
	default:
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check owner: %w", err)
	}
	return n == 0, nil
}

// cleanActivityDetail keeps only the fixed codes in the detail: an unknown reason becomes error
// (a free error text never reaches the log), an unknown trigger or stopper is dropped. A failed
// scan without a reason says error.
func cleanActivityDetail(kind string, d model.ActivityDetail) model.ActivityDetail {
	if d.Reason != "" && !activityReasons[d.Reason] {
		d.Reason = "error"
	}
	if d.Reason == "" && kind == model.ActivityScanFailed {
		d.Reason = "error"
	}
	if !activityTriggers[d.Trigger] {
		d.Trigger = ""
	}
	if !activityStoppers[d.By] {
		d.By = ""
	}
	if !activityModes[d.Mode] {
		d.Mode = ""
	}
	return d
}

// recentActivity returns the newest events of the log for the dashboard, with the e-mail address
// and the profile name read now.
func (s *Service) recentActivity(ctx context.Context) ([]model.ActivityEntry, error) {
	rows, err := s.sys.ListActivity(ctx, dashboardActivityLimit)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	out := make([]model.ActivityEntry, 0, len(rows))
	for _, r := range rows {
		e := model.ActivityEntry{
			ID: r.ID, At: time.UnixMilli(r.At).UTC(), Kind: r.Kind,
			AccountEmail: r.AccountEmail.String, ProfileName: r.ProfileName.String,
		}
		switch {
		case r.ItemID.Valid:
			e.ItemTitle = r.Subject.String
		case r.LibraryID.Valid, r.Kind == model.ActivityLibraryDeleted:
			// A deleted library's own event has no library_id any more: the subject is its name.
			e.LibraryName = r.Subject.String
		}
		if err := json.Unmarshal([]byte(r.Detail), &e.Detail); err != nil {
			s.log.WarnContext(ctx, "activity detail unreadable", "id", r.ID, "err", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// PruneActivity is the cleanup admin registers in jobs.prune: it deletes the events older than 30
// days and every event beyond the newest 2'000, in batches, checks ctx between them and returns
// how many rows went.
func (s *Service) PruneActivity(ctx context.Context) (int64, error) {
	q := systemq.New(s.db)
	before := s.now().Add(-activityRetention).UnixMilli()
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := q.PruneActivity(ctx, systemq.PruneActivityParams{Before: before, Keep: activityKeep, Batch: activityPruneBatch})
		if err != nil {
			return total, fmt.Errorf("prune activity: %w", err)
		}
		total += n
		if n < activityPruneBatch {
			return total, nil
		}
	}
}

// nullID is an optional id column: 0 is none.
func nullID(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: id != 0} }
