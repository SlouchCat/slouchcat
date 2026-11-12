package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// playingWindow is the activity window of a stream, as the stream overview counts one
// (playback.ActiveWindow): a playback without a request for longer is no longer playing.
const playingWindow = 2 * time.Minute

// WithDeleting sets the set of titles a delete works on, shared with package playback, which
// refuses to start one of them (review finding 5).
func WithDeleting(d *model.DeletingSet) Option {
	return func(s *Service) { s.deleting = d }
}

// WithBackups sets the config directory and the server version for the backup that DeleteLibrary
// writes before it deletes (store.BackupBeforeLibraryDelete); without it no backup is written.
func WithBackups(configDir, version string) Option {
	return func(s *Service) { s.backupConfigDir, s.version = configDir, version }
}

// JobLibraryDelete deletes a library in the background (concept 20 section 9.3 after the
// acceptance on CT 190); payload libraryDeletePayload, key library-delete:<id>.
const JobLibraryDelete = "library.delete"

// prioLibraryDelete is above the scans it cancels and below a playback.
const prioLibraryDelete = 7

// libraryDeletePayload is the payload of library.delete: the library, its name for the jobs page
// once the row is gone, and who asked (the activity log).
type libraryDeletePayload struct {
	LibraryID   int64  `json:"libraryId"`
	LibraryName string `json:"libraryName"`
	AccountID   int64  `json:"accountId,omitempty"`
	ProfileID   int64  `json:"profileId,omitempty"`
}

func libraryDeleteKey(libraryID int64) string { return fmt.Sprintf("library-delete:%d", libraryID) }

// DeleteLibrary asks for a library to be deleted for good (concept 20 P-8, section 9). In the
// request: confirmName must repeat the library's name (trimmed; compared against the stored row,
// never against anything else of the request), no profile may play one of its titles (O-13), and
// a library already being deleted answers model.ErrDeleting. Then, in one transaction, the library
// is marked (libraries.deleting_at: from now on it is gone for every view) and the job
// library.delete is queued; the answer is its id. The job does the delete (handleLibraryDelete).
//
// Measured on CT 190 (docs/perf/20-bedienung.md §5): 9.5 s for 304 titles and 27.4 s for 31 641
// inside the request, the backup of the 1.5 GB database alone about 9 s; too long for a request.
//
// Errors: model.ErrNotFound, model.ErrConfirmMismatch, model.ErrDeleting, *model.PlayingError.
func (s *Service) DeleteLibrary(ctx context.Context, libraryID int64, confirmName string, by model.Actor) (model.LibraryDeletionQueued, error) {
	lib, err := appq.New(s.db.Read).GetLibraryForDeletion(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.LibraryDeletionQueued{}, model.ErrNotFound
	}
	if err != nil {
		return model.LibraryDeletionQueued{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	if lib.DeletingAt.Valid {
		return model.LibraryDeletionQueued{}, model.ErrDeleting
	}
	if strings.TrimSpace(confirmName) != lib.Name {
		return model.LibraryDeletionQueued{}, model.ErrConfirmMismatch
	}
	if err := s.refuseIfLibraryPlays(ctx, libraryID); err != nil {
		return model.LibraryDeletionQueued{}, err
	}
	payload, err := json.Marshal(libraryDeletePayload{LibraryID: libraryID, LibraryName: lib.Name, AccountID: by.AccountID, ProfileID: by.ProfileID})
	if err != nil {
		return model.LibraryDeletionQueued{}, fmt.Errorf("encode %s payload: %w", JobLibraryDelete, err)
	}
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.LibraryDeletionQueued{}, err
	}
	defer func() { _ = tx.Rollback() }()
	n, err := appq.New(tx).MarkLibraryDeleting(ctx, appq.MarkLibraryDeletingParams{Now: sql.NullInt64{Int64: now, Valid: true}, ID: libraryID})
	if err != nil {
		return model.LibraryDeletionQueued{}, fmt.Errorf("mark library %d: %w", libraryID, err)
	}
	if n == 0 {
		return model.LibraryDeletionQueued{}, model.ErrDeleting // a second request was faster
	}
	q := systemq.New(tx)
	jobID, err := q.InsertJob(ctx, systemq.InsertJobParams{
		Type: JobLibraryDelete, Priority: prioLibraryDelete,
		IdempotencyKey: sql.NullString{String: libraryDeleteKey(libraryID), Valid: true},
		Payload:        string(payload), RunAfter: now, MaxAttempts: jobs.DefaultMaxAttempts, Now: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		jobID, err = q.GetActiveJobIDByKey(ctx, sql.NullString{String: libraryDeleteKey(libraryID), Valid: true})
	}
	if err != nil {
		return model.LibraryDeletionQueued{}, fmt.Errorf("queue %s: %w", JobLibraryDelete, err)
	}
	if err := tx.Commit(); err != nil {
		return model.LibraryDeletionQueued{}, fmt.Errorf("mark library %d: %w", libraryID, err)
	}
	s.log.WarnContext(ctx, "library delete queued", "library_id", libraryID, "library", lib.Name, "job_id", jobID)
	return model.LibraryDeletionQueued{JobID: jobID, Name: lib.Name}, nil
}

// handleLibraryDelete is the job library.delete: the delete of a marked library (deleteMarked).
// A library that is gone was deleted by an earlier run (the server stopped after the commit),
// one without the mark was given back: both end the job. A failure before the transaction gives
// the library back (the mark goes) and is final: the run shows on the jobs page and nothing
// retries it. A run that was cut off by a stop of the server is claimed again and runs once more
// from the start: every step before the transaction repeats without harm, the transaction is
// all or nothing.
func (s *Service) handleLibraryDelete(ctx context.Context, j *jobs.Job) error {
	var p libraryDeletePayload
	if err := decodePayload(j, &p); err != nil {
		return jobs.Final(err)
	}
	lib, err := appq.New(s.db.Read).GetLibraryForDeletion(ctx, p.LibraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get library %d: %w", p.LibraryID, err)
	}
	if !lib.DeletingAt.Valid {
		return nil
	}
	if _, err := s.deleteMarked(ctx, lib, model.Actor{AccountID: p.AccountID, ProfileID: p.ProfileID}, j.ID); err != nil {
		bg := context.WithoutCancel(ctx)
		if _, gone := appq.New(s.db.Read).GetLibraryForDeletion(bg, p.LibraryID); errors.Is(gone, sql.ErrNoRows) {
			return nil // the transaction went through; what failed after it is logged
		}
		if cerr := appq.New(s.db.Write).ClearLibraryDeleting(bg, p.LibraryID); cerr != nil {
			return jobs.Final(errors.Join(err, cerr))
		}
		s.log.WarnContext(ctx, "library delete failed, the library is back", "library_id", p.LibraryID, "library", lib.Name, "err", err)
		return jobs.Final(err)
	}
	return nil
}

// cancelLibraryDelete is the cancel hook of library.delete: a delete cancelled on the jobs page
// before its transaction gives the library back.
func (s *Service) cancelLibraryDelete(ctx context.Context, j jobs.Job) error {
	var p libraryDeletePayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return fmt.Errorf("decode %s payload: %w", j.Type, err)
	}
	return appq.New(s.db.Write).ClearLibraryDeleting(ctx, p.LibraryID)
}

// requeueLibraryDeletes queues library.delete again for every marked library without an active
// job (a job that was dismissed or lost): no library stays hidden for good. Called at start.
func (s *Service) requeueLibraryDeletes(ctx context.Context) {
	ids, err := appq.New(s.db.Read).ListDeletingLibraries(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "list libraries being deleted", "err", err)
		return
	}
	for _, id := range ids {
		lib, err := appq.New(s.db.Read).GetLibraryForDeletion(ctx, id)
		if err != nil {
			continue
		}
		if err := enqueueTx(ctx, systemq.New(s.db.Write), s.now(), newJob{
			JobLibraryDelete, prioLibraryDelete, libraryDeleteKey(id), libraryDeletePayload{LibraryID: id, LibraryName: lib.Name},
		}); err != nil {
			s.log.WarnContext(ctx, "queue library delete again", "library_id", id, "err", err)
		}
	}
}

// deleteMarked deletes a marked library for good (self is the id of the job that runs it): its folders, titles, files and everything on
// them -- progress, watchlist, picks, own images and texts, its home row, the grants of the
// profiles, the downloads of its titles (the devices drop them). It never deletes a file on the
// disk.
//
// The run, under the scan lock of the library: the running scan is cancelled and the lock waited
// for, the queued and running jobs of its titles are cancelled (they would fail after the delete
// and fill «Fehlgeschlagen»), the database is backed up (store.BackupBeforeLibraryDelete, its own
// name and retention), then one transaction deletes the grants and the library row, takes the
// library out of the activity log (the events keep its name as subject) and deletes the TMDB
// collections that lost their last member; manual collections stay, empty or not. The image
// folders of the titles go right after; the hourly sweeps take trickplay and chapter images.
// While a profile plays one of its titles the delete is refused (O-13; the library is hidden
// since the mark, so none can start).
func (s *Service) deleteMarked(ctx context.Context, lib appq.Library, by model.Actor, self int64) (model.LibraryDeletion, error) {
	libraryID := lib.ID
	if err := s.cancelScans(ctx, libraryID); err != nil {
		return model.LibraryDeletion{}, err
	}
	unlock, err := s.lockScan(ctx, libraryID)
	if err != nil {
		return model.LibraryDeletion{}, err
	}
	defer unlock()
	titles, err := systemq.New(s.db.Read).ListLibraryItemIDs(ctx, libraryID)
	if err != nil {
		return model.LibraryDeletion{}, fmt.Errorf("list titles of library %d: %w", libraryID, err)
	}
	// No playback starts from here to the commit (playback answers deleting), then again under
	// the lock: a playback may have started before the mark.
	release := s.deleting.Hold(titles)
	defer release()
	if err := s.refuseIfLibraryPlays(ctx, libraryID); err != nil {
		return model.LibraryDeletion{}, err
	}
	ids, err := systemq.New(s.db.Read).ListActiveJobsOfLibrary(ctx, libraryID)
	if err != nil {
		return model.LibraryDeletion{}, fmt.Errorf("list jobs of library %d: %w", libraryID, err)
	}
	// Not the job that runs this delete (its payload names the library too).
	s.cancelJobs(ctx, slices.DeleteFunc(ids, func(id int64) bool { return id == self }))
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return model.LibraryDeletion{}, fmt.Errorf("list roots of library %d: %w", libraryID, err)
	}
	out := model.LibraryDeletion{Name: lib.Name, Roots: len(roots)}
	if s.backupConfigDir != "" {
		name, err := store.BackupBeforeLibraryDelete(ctx, s.db.Read, s.backupConfigDir, s.version, s.now())
		if err != nil {
			return model.LibraryDeletion{}, err
		}
		out.Backup = name
	}
	if out.Items, err = s.deleteLibraryRows(ctx, libraryID); err != nil {
		return model.LibraryDeletion{}, err
	}
	for _, r := range roots {
		s.forgetRoot(libraryID, r.ID)
	}
	s.removeItemImages(ctx, titles)
	// Warn and not Info, like the removal of a folder: the log line and the activity entry are
	// the trace that a whole library left on one click.
	s.log.WarnContext(ctx, "library deleted", "library_id", libraryID, "library", lib.Name,
		"roots", out.Roots, "items_deleted", out.Items, "backup", out.Backup)
	s.recordActivity(ctx, model.ActivityEvent{
		Kind: model.ActivityLibraryDeleted, AccountID: by.AccountID, ProfileID: by.ProfileID, Subject: lib.Name,
		Detail: model.ActivityDetail{Count: out.Items},
	})
	return out, nil
}

// deleteLibraryRows is the transaction of DeleteLibrary; it returns how many rows of items went.
func (s *Service) deleteLibraryRows(ctx context.Context, libraryID int64) (int, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	items, err := q.CountLibraryItemsAll(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("count titles of library %d: %w", libraryID, err)
	}
	collections, err := q.ListTMDBCollectionsOfLibrary(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list collections of library %d: %w", libraryID, err)
	}
	if err := q.DeleteGrantsOfLibrary(ctx, libraryID); err != nil {
		return 0, fmt.Errorf("delete grants of library %d: %w", libraryID, err)
	}
	if err := q.ClearActivityLibrary(ctx, sql.NullInt64{Int64: libraryID, Valid: true}); err != nil {
		return 0, fmt.Errorf("clear activity of library %d: %w", libraryID, err)
	}
	n, err := q.DeleteLibraryRow(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("delete library %d: %w", libraryID, err)
	}
	if n == 0 {
		return 0, model.ErrNotFound
	}
	if len(collections) > 0 {
		if _, err := q.DeleteEmptyTMDBCollections(ctx, collections); err != nil {
			return 0, fmt.Errorf("delete empty collections: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("delete library %d: %w", libraryID, err)
	}
	return int(items), nil
}

// refuseIfLibraryPlays answers *model.PlayingError while a profile plays a title of the library.
func (s *Service) refuseIfLibraryPlays(ctx context.Context, libraryID int64) error {
	now := s.now()
	names, err := systemq.New(s.db.Read).ListPlayingProfilesOfLibrary(ctx, systemq.ListPlayingProfilesOfLibraryParams{
		LibraryID: libraryID, Now: now.UnixMilli(), ActiveAfter: now.Add(-playingWindow).UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("playing profiles of library %d: %w", libraryID, err)
	}
	if len(names) > 0 {
		return &model.PlayingError{Profiles: names}
	}
	return nil
}

// cancelJobs cancels the jobs through the registry (a running one stops at once) or, before
// RegisterTypes, the queue. A failure is logged: a job that still runs fails on its own.
func (s *Service) cancelJobs(ctx context.Context, ids []int64) {
	for _, id := range ids {
		var err error
		if s.registry != nil {
			_, err = s.registry.CancelJob(ctx, id)
		} else {
			_, err = s.queue.Cancel(ctx, id)
		}
		if err != nil {
			s.log.WarnContext(ctx, "cancel job of a deleted title", "job_id", id, "err", err)
		}
	}
}
