package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

// DeleteLibrary deletes a library for good (concept 20 P-8, section 9): its folders, titles,
// files and everything on them -- progress, watchlist, picks, own images and texts, its home row,
// the grants of the profiles, the downloads of its titles (the devices drop them). It never
// deletes a file on the disk. confirmName must repeat the library's name (trimmed; compared
// against the stored row, never against anything else of the request).
//
// The run, under the scan lock of the library: the running scan is cancelled and the lock waited
// for, the queued and running jobs of its titles are cancelled (they would fail after the delete
// and fill «Fehlgeschlagen»), the database is backed up (store.BackupBeforeLibraryDelete, its own
// name and retention), then one transaction deletes the grants and the library row, takes the
// library out of the activity log (the events keep its name as subject) and deletes the TMDB
// collections that lost their last member; manual collections stay, empty or not. The image
// folders of the titles go right after; the hourly sweeps take trickplay and chapter images.
// While a profile plays one of its titles the delete is refused and the playback goes on (O-13).
//
// Measured (M-1, docs/perf/20-bedienung.md): 10 000 titles in under 3.5 s, so it runs inside the
// request and not as a job.
//
// Errors: model.ErrNotFound, model.ErrConfirmMismatch, *model.PlayingError (model.ErrPlaying),
// and the error of a failed backup, in which case nothing is deleted.
func (s *Service) DeleteLibrary(ctx context.Context, libraryID int64, confirmName string, by model.Actor) (model.LibraryDeletion, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.LibraryDeletion{}, model.ErrNotFound
	}
	if err != nil {
		return model.LibraryDeletion{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	if strings.TrimSpace(confirmName) != lib.Name {
		return model.LibraryDeletion{}, model.ErrConfirmMismatch
	}
	if err := s.refuseIfLibraryPlays(ctx, libraryID); err != nil {
		return model.LibraryDeletion{}, err
	}
	if _, err := s.CancelScan(ctx, libraryID); err != nil {
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
	// the lock: a playback may have started while the scan stopped.
	release := s.deleting.Hold(titles)
	defer release()
	if err := s.refuseIfLibraryPlays(ctx, libraryID); err != nil {
		return model.LibraryDeletion{}, err
	}
	ids, err := systemq.New(s.db.Read).ListActiveJobsOfLibrary(ctx, libraryID)
	if err != nil {
		return model.LibraryDeletion{}, fmt.Errorf("list jobs of library %d: %w", libraryID, err)
	}
	s.cancelJobs(ctx, ids)
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
