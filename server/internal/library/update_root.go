package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// UpdateRoot changes the content, the provider list or the path of a folder (updateLibraryRoot,
// ADR 0015, concept 6.3, ADR 0023 point 4). The four effects differ in weight, and the weight
// decides what the request has to carry:
//
//   - A change of the list that keeps the identifier (["tmdb"] → ["tmdb","anilist"], or the
//     order of the fillers) writes the new list on every title with a file in the folder, its
//     ancestors and their descendants, and queues one metadata.fill per title. Nothing is
//     deleted, no scan runs, no confirmation is needed.
//   - A change of the identifier (["tmdb","tvdb"] → ["tvdb","tmdb"] on a series folder) does the
//     same, but queues metadata.match: the job identifies the title with the new first provider
//     and describes its seasons and episodes by that provider's numbering, and its fill follows.
//     The episode items stay -- their identity is the file name -- so progress and watchlist stay
//     attached. No confirmation either.
//   - A change of the content (movie ↔ series) is, in effect, "remove the folder and add it
//     again": the folder is read with another parser, and a film that becomes an episode is a
//     new title. It runs the removal steps of RemoveRoot in one transaction under the scan lock
//     (soft-delete the titles that live only here, move the downloads of the survivors, drop the
//     media files and the ignored files of the folder), keeps the root row and the watcher, clears
//     what the scanner holds for the folder and queues a scan that reads it afresh. It deletes
//     offline packages and detaches progress from titles, exactly like a removal, and so it must
//     not be easier to reach: confirmPath has to repeat the stored path (model.ErrConfirmMismatch),
//     and a running scan refuses it (model.ErrScanRunning).
//   - A new path (the folder moved) passes the rules of a new folder (prepareRootPath: absolute,
//     readable, below a browsable root, no overlap with another folder) and is taken over under
//     the scan lock by changeRootPath: the folder keeps its media files, so the queued scan finds
//     them again by name, inode or partial hash, and titles, ids and user data stay. Files it does
//     not find count towards the loss threshold. No confirmation; it cannot be combined with a
//     content change (path_with_content), only with a provider list. The same path is no change.
//
// Errors: model.ErrNotFound (unknown library, or the root belongs to another one),
// *model.ValidationError (content_not_allowed, providers_not_allowed, path library_path_invalid,
// root_overlap or path_with_content), model.ErrConfirmMismatch, model.ErrScanRunning.
func (s *Service) UpdateRoot(ctx context.Context, libraryID, rootID int64, upd model.LibraryRootUpdate) (model.Library, error) {
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

	var verr *model.ValidationError
	content := root.Content
	if upd.Content != nil {
		switch {
		case !model.ValidRootContent(*upd.Content):
			verr = verr.Add("content", "invalid_value")
		case !model.ContentAllowed(lib.Kind, *upd.Content):
			verr = verr.Add("content", model.CodeContentNotAllowed)
		default:
			content = *upd.Content
		}
	}
	old := model.ParseProviders(root.Providers)
	providers := old
	if upd.Providers != nil {
		providers = upd.Providers
	}
	if !model.ValidRootProviders(content, providers) {
		verr = verr.Add("providers", model.CodeProvidersNotAllowed)
	}
	var moved *preparedRoot
	if upd.Path != nil {
		p, err := s.prepareMovedRoot(ctx, root, *upd.Path, upd.Content != nil)
		var field *model.ValidationError
		switch {
		case errors.As(err, &field):
			verr = verr.Add("path", field.Fields[0].Code)
		case err != nil:
			return model.Library{}, err
		}
		moved = p
	}
	if verr != nil {
		return model.Library{}, verr
	}
	if content != root.Content {
		return s.changeRootContent(ctx, lib, root, content, providers, upd.ConfirmPath)
	}
	if moved != nil {
		var err error
		if root, err = s.changeRootPath(ctx, lib, root, *moved); err != nil {
			return model.Library{}, err
		}
	}
	if err := s.changeRootProviders(ctx, root, old, providers); err != nil {
		return model.Library{}, err
	}
	if moved != nil {
		return s.QueueScan(ctx, libraryID)
	}
	return s.Get(ctx, libraryID)
}

// prepareMovedRoot checks the new path of root: nil when it is the stored one, else the folder
// with its file system facts, or a *model.ValidationError with one field error (its code is used
// for the field "path"): path_with_content when the request changes the content as well,
// library_path_invalid or root_overlap from the rules of a new folder, where root itself does not
// count as an overlap.
func (s *Service) prepareMovedRoot(ctx context.Context, root appq.LibraryRoot, path string, withContent bool) (*preparedRoot, error) {
	var verr *model.ValidationError
	if withContent {
		return nil, verr.Add("path", model.CodePathWithContent)
	}
	if p := strings.TrimSpace(path); filepath.IsAbs(p) && filepath.Clean(p) == root.Path {
		return nil, nil
	}
	clean, err := s.prepareRootPath(ctx, path, nil, root.ID)
	switch {
	case errors.Is(err, errRootOverlap):
		return nil, verr.Add("path", model.CodeRootOverlap)
	case errors.Is(err, model.ErrLibraryPathInvalid):
		return nil, verr.Add("path", "library_path_invalid")
	case err != nil:
		return nil, err
	}
	fsType, err := paths.DetectFS(clean)
	if err != nil {
		return nil, verr.Add("path", "library_path_invalid")
	}
	watch, _ := WatchMode(string(fsType))
	return &preparedRoot{path: clean, content: root.Content, fsType: string(fsType), watch: watch}, nil
}

// changeRootPath takes over the new path of a folder that moved (ADR 0023 point 4) under the scan
// lock, so no scan walks the old path while the row changes: it writes the path with the file
// system facts of the new place, keeps the media files, replaces the watcher and drops what the
// service and the queue hold for the old place. The caller queues the scan that finds the files
// again. A running scan refuses it (model.ErrScanRunning).
func (s *Service) changeRootPath(ctx context.Context, lib appq.Library, root appq.LibraryRoot, moved preparedRoot) (appq.LibraryRoot, error) {
	unlock, ok := s.tryLockScan(lib.ID)
	if !ok {
		return root, fmt.Errorf("%w: library %d", model.ErrScanRunning, lib.ID)
	}
	defer unlock()

	files, err := systemq.New(s.db.Read).CountMediaFilesOfRoot(ctx, root.ID)
	if err != nil {
		return root, fmt.Errorf("count media files of root %d: %w", root.ID, err)
	}
	row, err := appq.New(s.db.Write).UpdateLibraryRootPath(ctx, appq.UpdateLibraryRootPathParams{
		Path: moved.path, FsType: moved.fsType, Watch: moved.watch, ID: root.ID, LibraryID: root.LibraryID,
	})
	if err != nil {
		return root, fmt.Errorf("update path of library root %d: %w", root.ID, err)
	}
	s.stopWatcher(root.ID)
	s.resetRootState(lib.ID, root.ID)
	s.dropQueuedPathScan(ctx, lib.ID, root.ID)
	s.startWatcher(row)

	// Warn and not Info, as for a content change: the project keeps no audit table.
	s.log.WarnContext(ctx, "library root path changed", "library_id", lib.ID, "library", lib.Name,
		"root_id", root.ID, "from", root.Path, "to", row.Path, "fs_type", row.FsType, "watch", row.Watch,
		"media_files_kept", files)
	return row, nil
}

// changeRootProviders writes the new list on the root and its titles and queues one job per
// title: metadata.match when the identifier changed (the fill follows it), else metadata.fill.
func (s *Service) changeRootProviders(ctx context.Context, root appq.LibraryRoot, old, providers []string) error {
	if model.ProvidersJSON(old) == model.ProvidersJSON(providers) {
		return nil
	}
	identifierChanged := len(old) == 0 || old[0] != providers[0]
	job := fillJob
	if identifierChanged {
		job = matchJob
	}
	now := s.now()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	if _, err := appq.New(tx).UpdateLibraryRoot(ctx, appq.UpdateLibraryRootParams{
		Content: root.Content, Providers: model.ProvidersJSON(providers), ID: root.ID, LibraryID: root.LibraryID,
	}); err != nil {
		return fmt.Errorf("update library root %d: %w", root.ID, err)
	}
	titles, err := q.UpdateItemsProvidersByRoot(ctx, systemq.UpdateItemsProvidersByRootParams{
		Providers: model.ProvidersJSON(providers), Now: now.UnixMilli(), RootID: root.ID,
	})
	if err != nil {
		return fmt.Errorf("update providers of root %d: %w", root.ID, err)
	}
	ids, err := q.ListTitleIDsOfRoot(ctx, root.ID)
	if err != nil {
		return fmt.Errorf("list titles of root %d: %w", root.ID, err)
	}
	for _, id := range ids {
		if err := enqueueTx(ctx, q, now, job(id)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("change providers of root %d: %w", root.ID, err)
	}
	s.log.InfoContext(ctx, "library root providers changed", "library_id", root.LibraryID, "root_id", root.ID,
		"root", root.Path, "from", model.ProvidersJSON(old), "to", model.ProvidersJSON(providers),
		"items", titles, "titles_queued", len(ids), "identifier_changed", identifierChanged)
	return nil
}

// changeRootContent runs the removal steps of RemoveRoot without removing the row, writes the new
// content and list, clears the scanner's state of the folder and queues the scan that reads it
// afresh. See UpdateRoot for why it needs the typed path.
func (s *Service) changeRootContent(ctx context.Context, lib appq.Library, root appq.LibraryRoot, content string, providers []string, confirmPath string) (model.Library, error) {
	if strings.TrimSpace(confirmPath) != root.Path {
		return model.Library{}, model.ErrConfirmMismatch
	}
	unlock, ok := s.tryLockScan(lib.ID)
	if !ok {
		return model.Library{}, fmt.Errorf("%w: library %d", model.ErrScanRunning, lib.ID)
	}
	defer unlock()

	now := s.now().UnixMilli()
	ms := sql.NullInt64{Int64: now, Valid: true}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.Library{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	files, err := q.CountMediaFilesOfRoot(ctx, root.ID)
	if err != nil {
		return model.Library{}, fmt.Errorf("count media files of root %d: %w", root.ID, err)
	}
	items, err := q.SoftDeleteItemsOfRootWithoutOtherFiles(ctx, systemq.SoftDeleteItemsOfRootWithoutOtherFilesParams{Now: ms, RootID: root.ID})
	if err != nil {
		return model.Library{}, fmt.Errorf("soft delete items of root %d: %w", root.ID, err)
	}
	if _, err := q.SoftDeleteEmptySeasons(ctx, ms); err != nil {
		return model.Library{}, fmt.Errorf("soft delete empty seasons: %w", err)
	}
	if _, err := q.SoftDeleteEmptySeries(ctx, ms); err != nil {
		return model.Library{}, fmt.Errorf("soft delete empty series: %w", err)
	}
	if err := QueueRelinksTx(ctx, q, time.UnixMilli(now)); err != nil {
		return model.Library{}, err
	}
	if _, err := q.MoveDownloadsOffRoot(ctx, systemq.MoveDownloadsOffRootParams{RootID: root.ID, Now: now}); err != nil {
		return model.Library{}, fmt.Errorf("move downloads off root %d: %w", root.ID, err)
	}
	// The cascade clears the downloads of the soft-deleted titles (their tombstones tell the
	// devices), the triggers set items.available.
	if _, err := q.DeleteMediaFilesOfRoot(ctx, root.ID); err != nil {
		return model.Library{}, fmt.Errorf("delete media files of root %d: %w", root.ID, err)
	}
	if _, err := q.DeleteIgnoredFilesOfRoot(ctx, root.ID); err != nil {
		return model.Library{}, fmt.Errorf("delete ignored files of root %d: %w", root.ID, err)
	}
	if _, err := appq.New(tx).UpdateLibraryRoot(ctx, appq.UpdateLibraryRootParams{
		Content: content, Providers: model.ProvidersJSON(providers), ID: root.ID, LibraryID: root.LibraryID,
	}); err != nil {
		return model.Library{}, fmt.Errorf("update library root %d: %w", root.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return model.Library{}, fmt.Errorf("change content of root %d: %w", root.ID, err)
	}
	s.resetRootState(lib.ID, root.ID)
	s.dropQueuedPathScan(ctx, lib.ID, root.ID)

	// Warn and not Info, for the same reason as in RemoveRoot: the project keeps no audit table.
	s.log.WarnContext(ctx, "library root content changed", "library_id", lib.ID, "library", lib.Name,
		"root_id", root.ID, "root", root.Path, "from", root.Content, "to", content,
		"items_soft_deleted", items, "media_files_deleted", files)
	if s.rootContentChanged != nil {
		if err := s.rootContentChanged(ctx, lib.Name, root.Path, root.Content, content, items, files); err != nil {
			s.log.WarnContext(ctx, "notify library root content changed", "library_id", lib.ID, "err", err)
		}
	}
	return s.QueueScan(ctx, lib.ID)
}

// resetRootState drops what the service holds for a folder that is about to be read with
// another parser -- the directories waiting to become a scan.path job and the extras files a
// scan deferred, both of which carry ParsedName values of the old reading -- and keeps the
// watcher running, unlike forgetRoot: the folder stays.
func (s *Service) resetRootState(libraryID, rootID int64) {
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
