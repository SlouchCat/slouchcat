package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Deleting a title (concept 20 P-9, sections 10 and 11). The dialog asks first (ItemDeletion),
// then the administrator chooses: «Nur aus Slouchcat entfernen» takes the title and every title
// below it out of every list and keeps its files and user data (DeleteItem, mode library); the
// page «Entfernte Titel» lists those and brings them back (ListRemovedItems, RestoreRemovedItem).

// treeChunk bounds the ids per statement (SQLite allows 32 766 variables; a podcast may have
// thousands of episodes).
const treeChunk = 5000

// itemTree is a title with every title below it and their media files.
type itemTree struct {
	item  systemq.GetDeletionItemRow
	ids   []string
	files []systemq.ListMediaFilesOfItemsRow
}

// loadTree reads the title and its tree. A deleted or removed title has no delete
// (model.ErrNotFound, as for the administration of a title); an extra or a podcast episode that
// is only a reference from the feed is not deletable (model.ErrNotDeletable, O-5). lang names the
// text whose title the dialog shows and the administrator types.
func (s *Service) loadTree(ctx context.Context, q *systemq.Queries, itemID, lang string) (itemTree, error) {
	it, err := q.GetDeletionItem(ctx, systemq.GetDeletionItemParams{Lang: lang, ID: itemID})
	if errors.Is(err, sql.ErrNoRows) || err == nil && it.DeletedAt.Valid {
		return itemTree{}, model.ErrNotFound
	}
	if err != nil {
		return itemTree{}, fmt.Errorf("load title %s: %w", itemID, err)
	}
	rows, err := q.ListItemTree(ctx, itemID)
	if err != nil {
		return itemTree{}, fmt.Errorf("list titles below %s: %w", itemID, err)
	}
	t := itemTree{item: it, ids: make([]string, 0, len(rows))}
	for _, r := range rows {
		t.ids = append(t.ids, r.ID)
	}
	for chunk := range slices.Chunk(t.ids, treeChunk) {
		files, err := q.ListMediaFilesOfItems(ctx, chunk)
		if err != nil {
			return itemTree{}, fmt.Errorf("list files of %s: %w", itemID, err)
		}
		t.files = append(t.files, files...)
	}
	switch it.Kind {
	case "movie", "series", "season", "episode", "audiobook", "podcast":
	case "podcast_episode":
		// O-5: a feed episode without a file would come back with the next fetch of the feed.
		if len(t.files) == 0 {
			return itemTree{}, model.ErrNotDeletable
		}
	default:
		return itemTree{}, model.ErrNotDeletable
	}
	return t, nil
}

// ItemDeletion is the preview of deleteItem: every media file of the title and the titles below
// it with the folder of its library root and its size, the folder they share, the title to type
// when there is more than one file, and the profiles that play one of them (O-13: the delete
// waits for them). lang is the server's text language. Errors: model.ErrNotFound,
// model.ErrNotDeletable.
func (s *Service) ItemDeletion(ctx context.Context, itemID, lang string) (model.ItemDeletionPreview, error) {
	q := systemq.New(s.db.Read)
	t, err := s.loadTree(ctx, q, itemID, lang)
	if err != nil {
		return model.ItemDeletionPreview{}, err
	}
	playing, err := s.playingProfiles(ctx, q, t.ids)
	if err != nil {
		return model.ItemDeletionPreview{}, err
	}
	files, err := s.checkFiles(ctx, q, t)
	if err != nil {
		return model.ItemDeletionPreview{}, err
	}
	out := model.ItemDeletionPreview{
		ItemID: t.item.ID, Kind: t.item.Kind, ConfirmTitle: t.item.DisplayTitle, TypeTitle: len(t.files) > 1,
		Files: files, Folder: commonFolder(t.files), FileCount: len(t.files), PlayingProfiles: playing,
	}
	out.FilesDeletable, out.FilesReason = filesVerdict(files)
	for _, f := range t.files {
		out.TotalBytes += f.Size
	}
	return out, nil
}

// DeleteItem deletes a title (P-9). Mode files («Auch die Dateien löschen», section 11, ADR 0032)
// deletes the files first and then the title for good (deleteWithFiles); it is refused while a
// file cannot be deleted (*model.FilesNotDeletableError). Mode library («Nur aus Slouchcat
// entfernen», section 10.2):
// the title and every title below it get removed_at and deleted_at, so they leave every list of
// every profile; their media_files rows, progress, check marks and watchlist stay; the downloads
// on the devices go (their tombstones tell the devices). The scanner never brings them back and
// puts new files in their folders under them (itemIndex.find). With more than one file the
// administrator types the title as the preview shows it (compared without case and surrounding
// spaces). Under the scan lock of the library, without waiting for a running scan (model.ErrScanRunning,
// like the removal of a folder); refused while a profile plays one of the titles (O-13).
//
// Errors: model.ErrNotFound, model.ErrNotDeletable, model.ErrConfirmMismatch,
// *model.PlayingError, *model.ScanRunningError, *model.FilesNotDeletableError,
// *model.ValidationError (mode).
func (s *Service) DeleteItem(ctx context.Context, itemID, lang, mode, confirmTitle string, by model.Actor) (model.ItemDeletionResult, error) {
	if mode != model.DeleteModeLibrary && mode != model.DeleteModeFiles {
		return model.ItemDeletionResult{}, (&model.ValidationError{}).Add("mode", "invalid")
	}
	q := systemq.New(s.db.Read)
	t, err := s.loadTree(ctx, q, itemID, lang)
	if err != nil {
		return model.ItemDeletionResult{}, err
	}
	if len(t.files) > 1 && !strings.EqualFold(strings.TrimSpace(confirmTitle), strings.TrimSpace(t.item.DisplayTitle)) {
		return model.ItemDeletionResult{}, model.ErrConfirmMismatch
	}
	if err := s.refuseIfPlays(ctx, q, t.ids); err != nil {
		return model.ItemDeletionResult{}, err
	}
	unlock, ok := s.tryLockScan(t.item.LibraryID)
	if !ok {
		return model.ItemDeletionResult{}, s.scanRunning(ctx, t.item.LibraryID)
	}
	defer unlock()
	// Again under the lock: the tree may have grown, a playback may have started.
	if t, err = s.loadTree(ctx, q, itemID, lang); err != nil {
		return model.ItemDeletionResult{}, err
	}
	if err := s.refuseIfPlays(ctx, q, t.ids); err != nil {
		return model.ItemDeletionResult{}, err
	}
	if mode == model.DeleteModeFiles {
		return s.deleteWithFiles(ctx, q, t, by)
	}
	s.cancelJobsOfTree(ctx, t.ids)
	removed, err := s.removeTree(ctx, t.ids)
	if err != nil {
		return model.ItemDeletionResult{}, err
	}
	s.log.InfoContext(ctx, "title taken out of slouchcat", "item_id", itemID, "kind", t.item.Kind,
		"items", removed, "files_kept", len(t.files))
	s.recordActivity(ctx, model.ActivityEvent{
		Kind: model.ActivityItemDeleted, AccountID: by.AccountID, ProfileID: by.ProfileID, ItemID: itemID,
		LibraryID: t.item.LibraryID, Subject: t.item.DisplayTitle, Detail: model.ActivityDetail{Mode: mode},
	})
	return model.ItemDeletionResult{RemovedItems: removed, Leftover: []model.DeletionLeftover{}}, nil
}

// removeTree marks the titles as taken out and drops their downloads, in one transaction; it
// returns how many titles were marked.
func (s *Service) removeTree(ctx context.Context, ids []string) (int, error) {
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	var removed int64
	for chunk := range slices.Chunk(ids, treeChunk) {
		n, err := q.MarkItemsRemoved(ctx, systemq.MarkItemsRemovedParams{Now: now, ItemIds: chunk})
		if err != nil {
			return 0, fmt.Errorf("take titles out: %w", err)
		}
		removed += n
		if _, err := q.DeleteDownloadsOfItems(ctx, chunk); err != nil {
			return 0, fmt.Errorf("drop downloads of removed titles: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("take titles out: %w", err)
	}
	return int(removed), nil
}

// ListRemovedItems returns one page of the titles taken out of Slouchcat that can come back on
// their own, newest first, each with its library and the folder its files share (limit 0 = 50).
func (s *Service) ListRemovedItems(ctx context.Context, limit, offset int) (model.RemovedItemPage, error) {
	if limit <= 0 {
		limit = 50
	}
	q := systemq.New(s.db.Read)
	rows, err := q.ListRemovedItems(ctx, systemq.ListRemovedItemsParams{LimitCount: int64(limit), OffsetCount: int64(offset)})
	if err != nil {
		return model.RemovedItemPage{}, fmt.Errorf("list removed titles: %w", err)
	}
	total, err := q.CountRemovedItems(ctx)
	if err != nil {
		return model.RemovedItemPage{}, fmt.Errorf("count removed titles: %w", err)
	}
	out := model.RemovedItemPage{Items: make([]model.RemovedItem, 0, len(rows)), Total: int(total)}
	for _, r := range rows {
		item := model.RemovedItem{
			ID: r.ID, Kind: r.Kind, Title: r.Title, LibraryID: r.LibraryID, LibraryName: r.LibraryName,
			RemovedAt: time.UnixMilli(r.RemovedAt.Int64).UTC(), ParentTitle: parentTitle(r),
		}
		tree, err := q.ListItemTree(ctx, r.ID)
		if err != nil {
			return model.RemovedItemPage{}, fmt.Errorf("list titles below %s: %w", r.ID, err)
		}
		ids := make([]string, 0, len(tree))
		for _, t := range tree {
			ids = append(ids, t.ID)
		}
		var files []systemq.ListMediaFilesOfItemsRow
		for chunk := range slices.Chunk(ids, treeChunk) {
			f, err := q.ListMediaFilesOfItems(ctx, chunk)
			if err != nil {
				return model.RemovedItemPage{}, fmt.Errorf("list files of %s: %w", r.ID, err)
			}
			files = append(files, f...)
		}
		item.Folder = commonFolder(files)
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// parentTitle names what a removed season or episode belongs to: «Serie» or «Serie › Staffel».
func parentTitle(r systemq.ListRemovedItemsRow) string {
	switch {
	case r.GrandparentTitle != "" && r.ParentTitle != "":
		return r.GrandparentTitle + " › " + r.ParentTitle
	default:
		return r.ParentTitle
	}
}

// RestoreRemovedItem is «Wieder aufnehmen»: the title and every title below it taken out with it
// come back -- the same titles with all their user data, for every kind. A title below that was
// taken out on its own before stays out, and a season or series above that was soft-deleted
// meanwhile comes back with it (RestoreItem). Files that went missing meanwhile are handled by the
// next scan as always. It returns how many titles came back. Errors: model.ErrNotFound (unknown,
// not taken out, or below a title that is taken out too), *model.ScanRunningError.
func (s *Service) RestoreRemovedItem(ctx context.Context, itemID string) (int, error) {
	q := systemq.New(s.db.Read)
	it, err := q.GetDeletionItem(ctx, systemq.GetDeletionItemParams{ID: itemID})
	if errors.Is(err, sql.ErrNoRows) || err == nil && !it.RemovedAt.Valid {
		return 0, model.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("load title %s: %w", itemID, err)
	}
	if it.ParentID.Valid {
		parent, err := q.GetDeletionItem(ctx, systemq.GetDeletionItemParams{ID: it.ParentID.String})
		if err != nil {
			return 0, fmt.Errorf("load parent of %s: %w", itemID, err)
		}
		if parent.RemovedAt.Valid {
			return 0, model.ErrNotFound // it comes back with the title above
		}
	}
	unlock, ok := s.tryLockScan(it.LibraryID)
	if !ok {
		return 0, s.scanRunning(ctx, it.LibraryID)
	}
	defer unlock()
	tree, err := q.ListItemTree(ctx, itemID)
	if err != nil {
		return 0, fmt.Errorf("list titles below %s: %w", itemID, err)
	}
	ids := make([]string, 0, len(tree))
	for _, t := range tree {
		ids = append(ids, t.ID)
	}
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	w := systemq.New(tx)
	var restored int64
	for chunk := range slices.Chunk(ids, treeChunk) {
		n, err := w.RestoreRemovedItems(ctx, systemq.RestoreRemovedItemsParams{Now: now, ItemIds: chunk, RemovedAt: it.RemovedAt})
		if err != nil {
			return 0, fmt.Errorf("bring titles back: %w", err)
		}
		restored += n
	}
	if err := w.RestoreItem(ctx, systemq.RestoreItemParams{Now: now, ID: itemID}); err != nil {
		return 0, fmt.Errorf("bring the containers above back: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("bring titles back: %w", err)
	}
	s.log.InfoContext(ctx, "removed title taken back", "item_id", itemID, "items", restored)
	return int(restored), nil
}

// commonFolder is the folder below its library root that all files share; nil without files or
// when they lie under more than one root.
func commonFolder(files []systemq.ListMediaFilesOfItemsRow) *model.DeletionFolder {
	if len(files) == 0 {
		return nil
	}
	root := files[0].RootID
	common := strings.Split(path.Dir(files[0].RelPath), "/")
	for _, f := range files[1:] {
		if f.RootID != root {
			return nil
		}
		dir := strings.Split(path.Dir(f.RelPath), "/")
		n := 0
		for n < len(common) && n < len(dir) && common[n] == dir[n] {
			n++
		}
		common = common[:n]
	}
	folder := strings.Join(common, "/")
	if folder == "." {
		folder = ""
	}
	return &model.DeletionFolder{RootPath: files[0].RootPath, Path: folder}
}

// playingProfiles names the profiles that play one of the titles right now.
func (s *Service) playingProfiles(ctx context.Context, q *systemq.Queries, ids []string) ([]string, error) {
	now := s.now()
	names := []string{}
	for chunk := range slices.Chunk(ids, treeChunk) {
		found, err := q.ListPlayingProfilesOfItems(ctx, systemq.ListPlayingProfilesOfItemsParams{
			ItemIds: chunk, Now: now.UnixMilli(), ActiveAfter: now.Add(-playingWindow).UnixMilli(),
		})
		if err != nil {
			return nil, fmt.Errorf("playing profiles: %w", err)
		}
		names = append(names, found...)
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// refuseIfPlays answers *model.PlayingError while a profile plays one of the titles (O-13).
func (s *Service) refuseIfPlays(ctx context.Context, q *systemq.Queries, ids []string) error {
	names, err := s.playingProfiles(ctx, q, ids)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		return &model.PlayingError{Profiles: names}
	}
	return nil
}

// cancelJobsOfTree cancels the queued and running jobs of the titles, their files and downloads.
func (s *Service) cancelJobsOfTree(ctx context.Context, ids []string) {
	for chunk := range slices.Chunk(ids, treeChunk/2) {
		jobIDs, err := systemq.New(s.db.Read).ListActiveJobsOfItems(ctx, chunk)
		if err != nil {
			s.log.WarnContext(ctx, "list jobs of deleted titles", "err", err)
			return
		}
		s.cancelJobs(ctx, jobIDs)
	}
}

// scanRunning is the error of a delete that found the scan lock taken.
func (s *Service) scanRunning(ctx context.Context, libraryID int64) error {
	err := &model.ScanRunningError{}
	if lib, lerr := s.Get(ctx, libraryID); lerr == nil {
		err.Library = lib.Name
	}
	return err
}
