package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// AddRoot adds a further directory to a library (idea 02, E8): the path goes through the same
// rules as the first one -- absolute, readable, below a browsable root (E14) and overlapping no
// root of this or another library -- and the library's watcher and scan pick it up at once.
//
// Errors: model.ErrNotFound for an unknown library, *model.ValidationError for a podcast library
// (which has no directory) and model.ErrLibraryPathInvalid for a path the rules refuse.
func (s *Service) AddRoot(ctx context.Context, libraryID int64, path string) (model.Library, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	if lib.Kind == model.LibraryPodcasts {
		var verr *model.ValidationError
		return model.Library{}, verr.Add("path", "not_allowed")
	}
	if err := s.ValidateNewRoot(ctx, path); err != nil {
		return model.Library{}, err
	}
	clean := filepath.Clean(path)
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return model.Library{}, fmt.Errorf("list roots: %w", err)
	}
	for _, r := range roots {
		if nested(clean, r.Path) || nested(r.Path, clean) {
			return model.Library{}, fmt.Errorf("%w: overlaps the root of library %d", model.ErrLibraryPathInvalid, r.LibraryID)
		}
	}
	fsType, err := paths.DetectFS(clean)
	if err != nil {
		return model.Library{}, fmt.Errorf("%w: %w", model.ErrLibraryPathInvalid, err)
	}
	watch, _ := WatchMode(string(fsType))
	root, err := appq.New(s.db.Write).InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{
		LibraryID: libraryID, Path: clean, FsType: string(fsType), Watch: watch, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return model.Library{}, fmt.Errorf("insert library root: %w", err)
	}
	s.log.InfoContext(ctx, "library root added", "library_id", libraryID, "root_id", root.ID,
		"root", clean, "fs_type", string(fsType), "watch", watch)
	s.startWatcher(root)
	return s.QueueScan(ctx, libraryID)
}

// RemoveRoot removes a directory from a library and lets the titles that live only there leave
// it (idea 02, E12). confirmPath must repeat the stored path of the root; it is compared against
// the row, never against anything else the request carries.
//
// The order of the steps is the point of this function, not its style. media_files.root_id
// cascades on delete and SoftDeleteItemsMissingSince only ever looks at titles that still have a
// file, so a title whose last file goes with the root must be soft-deleted *before* the root row
// disappears -- otherwise it stays behind for good: invisible but not deleted, counted in every
// list. downloads.media_file_id cascades as well and its delete trigger writes a tombstone, so
// the downloads of the titles that survive move to a remaining file of theirs first; otherwise a
// device would drop a package whose title is still on screen.
//
// The scan lock is taken for the whole run, because scanLibrary loads the roots once and writes
// media_files with their ids afterwards: a root that vanished mid-scan would run that scan into a
// foreign key error. A scan that is already running answers model.ErrScanRunning.
//
// Errors: model.ErrNotFound (unknown library, or the root belongs to another one),
// model.ErrConfirmMismatch, model.ErrScanRunning.
func (s *Service) RemoveRoot(ctx context.Context, libraryID, rootID int64, confirmPath string) (model.Library, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, rootID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && root.LibraryID != libraryID) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, fmt.Errorf("get library root %d: %w", rootID, err)
	}
	if strings.TrimSpace(confirmPath) != root.Path {
		return model.Library{}, model.ErrConfirmMismatch
	}
	unlock, ok := s.tryLockScan(libraryID)
	if !ok {
		return model.Library{}, fmt.Errorf("%w: library %d", model.ErrScanRunning, libraryID)
	}
	defer unlock()

	items, files, err := s.removeRootRows(ctx, libraryID, rootID)
	if err != nil {
		return model.Library{}, err
	}
	s.forgetRoot(libraryID, rootID)
	s.dropQueuedPathScan(ctx, libraryID, rootID)

	// Warn and not Info: the project keeps no audit table, so this line and the notification are
	// the only trace that thousands of titles left the library on one click.
	s.log.WarnContext(ctx, "library root removed", "library_id", libraryID, "library", lib.Name,
		"root_id", rootID, "root", root.Path, "items_soft_deleted", items, "media_files_deleted", files)
	if s.rootRemoved != nil {
		if err := s.rootRemoved(ctx, lib.Name, root.Path, items, files); err != nil {
			s.log.WarnContext(ctx, "notify library root removed", "library_id", libraryID, "err", err)
		}
	}
	return s.Get(ctx, libraryID)
}

// removeRootRows runs the four database steps of RemoveRoot in one transaction and returns how
// many titles were soft-deleted and how many media files went with the root.
func (s *Service) removeRootRows(ctx context.Context, libraryID, rootID int64) (items, files int64, err error) {
	now := s.now().UnixMilli()
	ms := sql.NullInt64{Int64: now, Valid: true}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)

	files, err = q.CountMediaFilesOfRoot(ctx, rootID)
	if err != nil {
		return 0, 0, fmt.Errorf("count media files of root %d: %w", rootID, err)
	}
	items, err = q.SoftDeleteItemsOfRootWithoutOtherFiles(ctx, systemq.SoftDeleteItemsOfRootWithoutOtherFilesParams{Now: ms, RootID: rootID})
	if err != nil {
		return 0, 0, fmt.Errorf("soft delete items of root %d: %w", rootID, err)
	}
	seasons, err := q.SoftDeleteEmptySeasons(ctx, ms)
	if err != nil {
		return 0, 0, fmt.Errorf("soft delete empty seasons: %w", err)
	}
	series, err := q.SoftDeleteEmptySeries(ctx, ms)
	if err != nil {
		return 0, 0, fmt.Errorf("soft delete empty series: %w", err)
	}
	if _, err := q.MoveDownloadsOffRoot(ctx, systemq.MoveDownloadsOffRootParams{RootID: rootID, Now: now}); err != nil {
		return 0, 0, fmt.Errorf("move downloads off root %d: %w", rootID, err)
	}
	n, err := q.DeleteLibraryRoot(ctx, systemq.DeleteLibraryRootParams{ID: rootID, LibraryID: libraryID})
	if err != nil {
		return 0, 0, fmt.Errorf("delete library root %d: %w", rootID, err)
	}
	if n == 0 {
		return 0, 0, model.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("remove library root %d: %w", rootID, err)
	}
	return items + seasons + series, files, nil
}

// forgetRoot stops the watcher of a removed root and drops what the service still holds for it:
// the directories waiting to become a scan.path job and the extras files a scan deferred.
func (s *Service) forgetRoot(libraryID, rootID int64) {
	s.mu.Lock()
	if stop := s.watcherStop[rootID]; stop != nil {
		stop()
	}
	delete(s.watcherStop, rootID)
	delete(s.watchers, rootID)
	s.mu.Unlock()

	s.pathMu.Lock()
	if byRoot := s.pathPending[libraryID]; byRoot != nil {
		delete(byRoot, rootID)
		if len(byRoot) == 0 {
			delete(s.pathPending, libraryID)
		}
	}
	s.pathMu.Unlock()

	s.orphanMu.Lock()
	delete(s.orphans, rootID)
	s.orphanMu.Unlock()
}

// dropQueuedPathScan takes the removed root out of a queued scan.path job of the library, and
// cancels the job when nothing else is left in it. A running job is left alone: it holds the
// roots it started with and ends on its own. Every failure is logged only -- the root is gone
// either way, and a path scan of a root that no longer exists finds nothing and stops.
func (s *Service) dropQueuedPathScan(ctx context.Context, libraryID, rootID int64) {
	id, err := systemq.New(s.db.Write).GetActiveJobIDByKey(ctx, sql.NullString{String: pathScanKey(libraryID), Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		s.log.WarnContext(ctx, "find path scan of removed root", "library_id", libraryID, "err", err)
		return
	}
	j, err := s.queue.Get(ctx, id)
	if err != nil || j.State != jobs.StateQueued {
		return
	}
	var held pathScanPayload
	if err := json.Unmarshal(j.Payload, &held); err != nil {
		s.log.WarnContext(ctx, "decode path scan of removed root", "job_id", id, "err", err)
		return
	}
	kept := slices.DeleteFunc(held.Roots, func(r pathScanRoot) bool { return r.RootID == rootID })
	if len(kept) == len(held.Roots) {
		return
	}
	if len(kept) == 0 {
		if _, err := s.queue.Cancel(ctx, id); err != nil {
			s.log.WarnContext(ctx, "cancel path scan of removed root", "job_id", id, "err", err)
		}
		return
	}
	held.Roots = kept
	data, err := json.Marshal(normalizePathScan(held))
	if err != nil {
		s.log.WarnContext(ctx, "encode path scan of removed root", "job_id", id, "err", err)
		return
	}
	if _, err := s.queue.SetQueuedPayload(ctx, id, data); err != nil {
		s.log.WarnContext(ctx, "rewrite path scan of removed root", "job_id", id, "err", err)
	}
}
