package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// «Auch die Dateien löschen» (concept 20 section 11, ADR 0032). The media folders stay read-only by
// default; the server deletes only where the operator mounted a folder writable on purpose, only
// on the administrator's explicit command, and only exactly this, in this order:
//
//  1. the media files of the title, of every title below it and of its extras;
//  2. the image and metadata files Slouchcat read for these titles: the recorded local images
//     (item_local_images), the folder cover of a book or podcast, and the fixed-name files of
//     the folder (movie.nfo, tvshow.nfo, season.nfo, metadata.json) once no media file of any
//     title is left in it;
//  3. companions next to a deleted media file whose name starts with the whole file stem
//     followed by "." or "-" ("Film (2024).de.srt", "Film (2024)-poster.jpg");
//  4. then the folder if it is empty, and empty folders above it up to below the library root.
//
// A file that is a media file of any title, or a recorded image of a title outside the tree, is
// never deleted as a companion; a media file that is also another title's file (a hard link)
// stays and is named. Nothing is deleted recursively: what Slouchcat does not know stays, and the
// result counts it. A file that is already gone counts as deleted. Every delete goes through
// paths.Remove and paths.RemoveEmptyDir (lint rule, ADR 0032).

// fixedNameFiles are the metadata files Slouchcat reads by name in a title's folder.
var fixedNameFiles = []string{"movie.nfo", "tvshow.nfo", "season.nfo", "metadata.json"}

// deleteCheck tells whether the server may delete in the folder dir below root: nil, or an error
// that wraps paths.ErrReadOnly or paths.ErrNoPermission (the test seam replaces it).
func (s *Service) deleteCheck(root paths.Root, dir string) error {
	if s.deleteCheckFn != nil {
		return s.deleteCheckFn(root, dir)
	}
	switch ro, err := paths.ReadOnlyAt(root, dir); {
	case err != nil:
		return err
	case ro:
		return paths.ErrReadOnly
	}
	return paths.MayDeleteIn(root, dir)
}

// checkFiles judges every media file of the tree without writing anything: shared with a title
// outside (stays), missing (counts as deleted), not a regular file, or in a folder the server may
// not delete in (read-only mount, missing rights). The verdicts are in the order of t.files.
func (s *Service) checkFiles(ctx context.Context, q *systemq.Queries, t itemTree) ([]model.DeletionFile, error) {
	shared := map[string]bool{}
	for chunk := range slices.Chunk(t.ids, treeChunk) {
		ids, err := q.ListSharedMediaFiles(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("shared files of %s: %w", t.item.ID, err)
		}
		for _, id := range ids {
			shared[id] = true
		}
	}
	dirs := map[string]string{} // root path + dir → reason ("" = may delete)
	out := make([]model.DeletionFile, 0, len(t.files))
	for _, f := range t.files {
		d := model.DeletionFile{RootPath: f.RootPath, Path: f.RelPath, SizeBytes: f.Size, Deletable: true}
		switch reason := s.fileReason(f, shared[f.ID], dirs); reason {
		case "", model.DeleteReasonMissing:
			d.Reason = reason
		default:
			d.Deletable, d.Reason = false, reason
		}
		out = append(out, d)
	}
	return out, nil
}

// fileCheckTimeout bounds the lstat and the access check of one file before a delete: a hanging
// share answers unreachable instead of holding the dialog (review finding 8).
const fileCheckTimeout = 3 * time.Second

// fileReason is the verdict of one file (see checkFiles); dirs caches the folders. The checks run
// with a deadline; a root whose check hangs answers unreachable at once until that check returns.
func (s *Service) fileReason(f systemq.ListMediaFilesOfItemsRow, shared bool, dirs map[string]string) string {
	if shared {
		return model.DeleteReasonShared
	}
	if _, hanging := s.hangingChecks.Load(f.RootPath); hanging {
		return model.DeleteReasonUnreachable
	}
	timeout := s.fileCheckTimeout
	if timeout == 0 {
		timeout = fileCheckTimeout
	}
	key := f.RootPath + "\x00" + path.Dir(f.RelPath)
	cached, known := dirs[key]
	type verdict struct{ reason, dir string }
	answer := make(chan verdict, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		reason, dir := s.fileReasonNow(f, cached, known)
		answer <- verdict{reason, dir}
	}()
	select {
	case v := <-answer:
		if v.dir != "-" {
			dirs[key] = v.dir
		}
		return v.reason
	case <-time.After(timeout):
		s.hangingChecks.Store(f.RootPath, struct{}{})
		go func() {
			<-done
			s.hangingChecks.Delete(f.RootPath)
		}()
		return model.DeleteReasonUnreachable
	}
}

// fileReasonNow is fileReason without the deadline. cached is the verdict of the file's folder
// when known; it returns the file's reason and the folder's verdict to remember ("-" when the
// folder was not checked). It touches no shared state, so a check that hangs past its deadline
// harms nothing.
func (s *Service) fileReasonNow(f systemq.ListMediaFilesOfItemsRow, cached string, known bool) (string, string) {
	root, err := paths.NewRoot(f.RootPath)
	if err != nil {
		return model.DeleteReasonUnreachable, "-"
	}
	r, err := paths.Open(root)
	if err != nil {
		return model.DeleteReasonUnreachable, "-"
	}
	fi, err := r.Lstat(f.RelPath)
	_ = r.Close()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return model.DeleteReasonMissing, "-"
	case err != nil:
		return model.DeleteReasonUnreachable, "-"
	case !fi.Mode().IsRegular():
		return model.DeleteReasonNotRegular, "-"
	}
	if known {
		return cached, "-"
	}
	dir := path.Dir(f.RelPath)
	reason := ""
	switch err := s.deleteCheck(root, dir); {
	case err == nil:
	case errors.Is(err, paths.ErrReadOnly):
		reason = model.DeleteReasonReadOnly
	case errors.Is(err, paths.ErrNoPermission):
		reason = model.DeleteReasonNoPermission
	default:
		reason = model.DeleteReasonUnreachable
	}
	return reason, reason
}

// filesVerdict is whether «Auch die Dateien löschen» is possible, and why not.
func filesVerdict(files []model.DeletionFile) (bool, string) {
	if len(files) == 0 {
		return false, "no_files"
	}
	some := false
	for _, f := range files {
		switch {
		case f.Deletable:
			some = true
		case f.Reason != model.DeleteReasonShared:
			return false, f.Reason
		}
	}
	if !some {
		return false, model.DeleteReasonShared
	}
	return true, ""
}

// fileTarget is a file below a root that the delete takes.
type fileTarget struct {
	rootID int64
	root   paths.Root
	rel    string
}

// deleteWithFiles is DeleteItem with mode files, under the scan lock: the files go first, then the
// database. The rows of the deleted files go at once (no player offers a deleted version); a
// title below that kept no file is deleted for good, and when no file of the whole tree is left,
// the title itself goes for good with everything on it (O-14). If a file could not be deleted,
// the title stays with the remaining files and the result names them.
func (s *Service) deleteWithFiles(ctx context.Context, q *systemq.Queries, t itemTree, by model.Actor) (model.ItemDeletionResult, error) {
	checks, err := s.checkFiles(ctx, q, t)
	if err != nil {
		return model.ItemDeletionResult{}, err
	}
	if ok, reason := filesVerdict(checks); !ok {
		return model.ItemDeletionResult{}, &model.FilesNotDeletableError{Reason: reason}
	}
	s.cancelJobsOfTree(ctx, t.ids)
	out := model.ItemDeletionResult{Leftover: []model.DeletionLeftover{}}
	var gone []string               // ids of the media files that are gone
	dirs := map[string]fileTarget{} // the folders the files left, by root and path
	for i, f := range t.files {
		root, err := paths.NewRoot(f.RootPath)
		if err != nil {
			out.Leftover = append(out.Leftover, model.DeletionLeftover{RootPath: f.RootPath, Path: f.RelPath, Reason: model.DeleteReasonUnreachable})
			continue
		}
		if checks[i].Reason == model.DeleteReasonShared {
			out.Leftover = append(out.Leftover, model.DeletionLeftover{RootPath: f.RootPath, Path: f.RelPath, Reason: model.DeleteReasonShared})
			continue
		}
		// The file must still be the one the scan recorded (device and inode), else it stays.
		switch err := s.removeFile(root, f.RelPath, recordedID(f), true); {
		case err == nil:
			out.DeletedBytes += f.Size
		case errors.Is(err, fs.ErrNotExist):
			// Already gone: counts as deleted.
		default:
			s.log.WarnContext(ctx, "media file not deleted", "item_id", t.item.ID, "root", f.RootPath, "path", f.RelPath, "err", err)
			out.Leftover = append(out.Leftover, model.DeletionLeftover{RootPath: f.RootPath, Path: f.RelPath, Reason: removeReason(err)})
			continue
		}
		out.DeletedFiles++
		gone = append(gone, f.ID)
		dirs[f.RootPath+"\x00"+path.Dir(f.RelPath)] = fileTarget{rootID: f.RootID, root: root, rel: path.Dir(f.RelPath)}
	}
	extra, err := s.recordedAndCompanions(ctx, q, t, gone, dirs)
	if err != nil {
		return out, err
	}
	for _, x := range extra {
		switch err := s.removeFile(x.root, x.rel, paths.FileID{}, false); {
		case err == nil, errors.Is(err, fs.ErrNotExist):
		default:
			s.log.WarnContext(ctx, "file of a deleted title not deleted", "item_id", t.item.ID, "root", x.root.Dir(), "path", x.rel, "err", err)
			out.Leftover = append(out.Leftover, model.DeletionLeftover{RootPath: x.root.Dir(), Path: x.rel, Reason: removeReason(err)})
		}
	}
	out.LeftInFolder = removeEmptyFolders(dirs)
	removed, deleted, err := s.dropDeletedFiles(ctx, t, gone, extra)
	if err != nil {
		return out, err
	}
	out.RemovedItems, out.TitleDeleted = removed, slices.Contains(deleted, t.item.ID)
	s.removeItemImages(ctx, deleted)
	s.log.WarnContext(ctx, "title deleted with its files", "item_id", t.item.ID, "kind", t.item.Kind,
		"files_deleted", out.DeletedFiles, "bytes", out.DeletedBytes, "files_left", len(out.Leftover),
		"left_in_folder", out.LeftInFolder, "title_deleted", out.TitleDeleted)
	s.recordActivity(ctx, model.ActivityEvent{
		Kind: model.ActivityItemDeleted, AccountID: by.AccountID, ProfileID: by.ProfileID, ItemID: t.item.ID,
		LibraryID: t.item.LibraryID, Subject: t.item.DisplayTitle,
		Detail: model.ActivityDetail{Mode: model.DeleteModeFiles, Count: out.DeletedFiles, SizeBytes: out.DeletedBytes},
	})
	return out, nil
}

// removeFile is paths.RemoveExpected for a media file (its recorded identity must match; none
// recorded leaves it as changed, review N3) and paths.Remove for the others, or the test seam.
func (s *Service) removeFile(root paths.Root, rel string, want paths.FileID, media bool) error {
	if s.removeFileFn != nil {
		return s.removeFileFn(root, rel)
	}
	if media {
		return paths.RemoveExpected(root, rel, want)
	}
	return paths.Remove(root, rel)
}

// recordedID is the device and inode the scan recorded for a media file (zero: none recorded).
func recordedID(f systemq.ListMediaFilesOfItemsRow) paths.FileID {
	if f.Dev == 0 || f.Inode == 0 {
		return paths.FileID{}
	}
	return paths.FileID{Dev: uint64(f.Dev), Inode: uint64(f.Inode)} //nolint:gosec // G115: stored from a uint64 by toInt64
}

// removeReason maps a failed paths.Remove to the fixed code of the result.
func removeReason(err error) string {
	switch {
	case errors.Is(err, paths.ErrChanged):
		return model.DeleteReasonChanged
	case errors.Is(err, paths.ErrNotRegular):
		return model.DeleteReasonNotRegular
	case errors.Is(err, fs.ErrPermission):
		return model.DeleteReasonNoPermission
	}
	return model.DeleteReasonFailed
}

// recordedAndCompanions lists steps 2 and 3: the files Slouchcat recorded or reads for the titles,
// and the companions of the deleted media files. None of them is a media file of any title or a
// recorded image of a title outside the tree.
func (s *Service) recordedAndCompanions(ctx context.Context, q *systemq.Queries, t itemTree, gone []string, dirs map[string]fileTarget) ([]fileTarget, error) {
	if len(gone) == 0 {
		return nil, nil
	}
	roots := map[int64]paths.Root{}
	rootPaths := map[int64]string{}
	for _, f := range t.files {
		rootPaths[f.RootID] = f.RootPath
	}
	rootOf := func(id int64) (paths.Root, bool) {
		if r, ok := roots[id]; ok {
			return r, true
		}
		p, ok := rootPaths[id]
		if !ok {
			return paths.Root{}, false
		}
		r, err := paths.NewRoot(p)
		if err != nil {
			return paths.Root{}, false
		}
		roots[id] = r
		return r, true
	}
	var out []fileTarget
	seen := map[string]bool{}
	add := func(rootID int64, rel string) error {
		root, ok := rootOf(rootID)
		key := fmt.Sprint(rootID) + "\x00" + rel
		if !ok || seen[key] {
			return nil
		}
		if _, err := paths.Resolve(root, rel); err != nil {
			return nil //nolint:nilerr // a recorded path that could leave its root is skipped, never deleted
		}
		n, err := q.CountTitleFilesAtPath(ctx, systemq.CountTitleFilesAtPathParams{RootID: rootID, RelPath: rel, ItemIds: t.ids})
		if err != nil {
			return fmt.Errorf("owner of %s: %w", rel, err)
		}
		sidecars, err := q.CountSidecarsOfOthersAtPath(ctx, systemq.CountSidecarsOfOthersAtPathParams{RootID: rootID, RelPath: sql.NullString{String: rel, Valid: true}, ItemIds: t.ids})
		if err != nil {
			return fmt.Errorf("sidecar owner of %s: %w", rel, err)
		}
		if n+sidecars > 0 {
			return nil
		}
		seen[key] = true
		out = append(out, fileTarget{rootID: rootID, root: root, rel: rel})
		return nil
	}
	// (2) Recorded images and folder covers, only of the titles that go (review finding 6): a
	// series that keeps a file keeps its poster.
	for chunk := range slices.Chunk(endingTitles(t, gone), treeChunk) {
		images, err := q.ListLocalImagesOfItems(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("recorded images of %s: %w", t.item.ID, err)
		}
		for _, img := range images {
			if err := add(img.RootID, img.RelPath); err != nil {
				return nil, err
			}
		}
		covers, err := q.ListCoverPathsOfItems(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("covers of %s: %w", t.item.ID, err)
		}
		for _, c := range covers {
			if strings.HasPrefix(c.CoverPath, embeddedCover) {
				continue
			}
			for _, f := range t.files {
				if f.ItemID == c.ID {
					if err := add(f.RootID, c.CoverPath); err != nil {
						return nil, err
					}
					break
				}
			}
		}
	}
	// Fixed-name metadata files in each folder a file left, and in the folders above it below
	// the root, once no media file of any title is left at or below that folder.
	folders := map[string]fileTarget{}
	for _, d := range dirs {
		for dir := d.rel; dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
			folders[fmt.Sprint(d.rootID)+"\x00"+dir] = fileTarget{rootID: d.rootID, root: d.root, rel: dir}
		}
	}
	for _, d := range folders {
		n, err := q.CountMediaFilesBelow(ctx, systemq.CountMediaFilesBelowParams{RootID: d.rootID, Prefix: d.rel + "/", FileIds: gone})
		if err != nil {
			return nil, fmt.Errorf("files below %s: %w", d.rel, err)
		}
		if n > 0 {
			continue
		}
		for _, name := range fixedNameFiles {
			if err := add(d.rootID, d.rel+"/"+name); err != nil {
				return nil, err
			}
		}
	}
	// (3) Companions: the whole stem of a deleted file, then "." or "-".
	goneSet := map[string]bool{}
	for _, id := range gone {
		goneSet[id] = true
	}
	for _, f := range t.files {
		if !goneSet[f.ID] {
			continue
		}
		root, ok := rootOf(f.RootID)
		if !ok {
			continue
		}
		dir, base := path.Dir(f.RelPath), path.Base(f.RelPath)
		stem := strings.TrimSuffix(base, path.Ext(base))
		names, err := paths.ReadDirNames(root, dir, 0)
		if err != nil {
			continue // the folder is gone or unreadable: no companion to find
		}
		// The stems of the other media files of the folder, on the disk and in the database: a
		// candidate that starts with a longer one belongs to that file («Alien-Resurrection.de.srt»
		// next to «Alien.mkv» and «Alien-Resurrection.mkv»; review N1).
		known, err := q.ListMediaFileNamesInFolder(ctx, systemq.ListMediaFileNamesInFolderParams{RootID: f.RootID, Prefix: folderPrefix(dir)})
		if err != nil {
			return nil, fmt.Errorf("media files in %s: %w", dir, err)
		}
		longer := longerStems(stem, base, names, known)
		var candidates []string
		for _, n := range names {
			if n != base && hasStemPrefix(n, stem) && !slices.ContainsFunc(longer, func(l string) bool { return hasStemPrefix(n, l) }) {
				candidates = append(candidates, n)
			}
		}
		entries, err := paths.LstatAll(root, dir, candidates)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue // a folder or a symlink of that name stays
			}
			// A file with a media extension is never a companion: it may be the next film or
			// episode of the folder that no scan has seen yet ("Alien-Resurrection.mkv" next to
			// "Alien.mkv"). Nor is a file that is a media file of any title under another name.
			if isMediaName(e.Name()) {
				continue
			}
			if ok, err := s.knownIdentity(ctx, q, e); err != nil {
				return nil, err
			} else if ok {
				continue
			}
			if err := add(f.RootID, path.Join(dir, e.Name())); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// endingTitles are the titles of the tree that keep no media file after the delete: those whose
// files all went, and the containers above whose children all end. With every file gone it is the
// whole tree.
func endingTitles(t itemTree, gone []string) []string {
	goneSet := make(map[string]bool, len(gone))
	for _, id := range gone {
		goneSet[id] = true
	}
	keeps := map[string]bool{} // titles with a file left, and every title above them
	for _, f := range t.files {
		if goneSet[f.ID] {
			continue
		}
		for id, ok := f.ItemID, true; ok; id, ok = t.parent[id] {
			keeps[id] = true
		}
	}
	out := make([]string, 0, len(t.ids))
	for _, id := range t.ids {
		if !keeps[id] {
			out = append(out, id)
		}
	}
	return out
}

// hasStemPrefix reports whether name starts with the whole stem followed by "." or "-".
func hasStemPrefix(name, stem string) bool {
	return strings.HasPrefix(name, stem+".") || strings.HasPrefix(name, stem+"-")
}

// folderPrefix is the rel_path prefix of the files directly in dir ("" for the root itself).
func folderPrefix(dir string) string {
	if dir == "." || dir == "" {
		return ""
	}
	return dir + "/"
}

// longerStems are the stems of the media files of the folder other than base (names on the disk
// with a media extension, known rel_paths of the database) that are longer than stem and start with
// it: their companions are not those of stem.
func longerStems(stem, base string, names, known []string) []string {
	var out []string
	add := func(name string) {
		if name == base {
			return
		}
		if l := strings.TrimSuffix(name, path.Ext(name)); len(l) > len(stem) && strings.HasPrefix(l, stem) && !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	for _, n := range names {
		if isMediaName(n) {
			add(n)
		}
	}
	for _, rel := range known {
		add(path.Base(rel))
	}
	return out
}

// isMediaName reports whether the name has the extension of a file the scanner reads as media
// (video, or audio of books and podcasts).
func isMediaName(name string) bool {
	return IsVideo(name) || IsAudio(name)
}

// knownIdentity reports whether the entry is, by device and inode, the file of a media_files row.
func (s *Service) knownIdentity(ctx context.Context, q *systemq.Queries, e fs.DirEntry) (bool, error) {
	info, err := e.Info()
	if err != nil {
		return true, nil //nolint:nilerr // an entry that cannot be read is no companion to delete
	}
	id := sysFileID(info.Sys())
	if id.dev == 0 || id.inode == 0 {
		return false, nil
	}
	n, err := q.CountMediaFilesWithIdentity(ctx, systemq.CountMediaFilesWithIdentityParams{Dev: toInt64(id.dev), Inode: toInt64(id.inode)})
	if err != nil {
		return false, fmt.Errorf("identity of %s: %w", e.Name(), err)
	}
	return n > 0, nil
}

// removeEmptyFolders is step 4: each folder a file left goes when it is empty, then each empty
// folder above it up to below the library root. It returns how many entries stay in the folders
// that stayed (what Slouchcat does not know; nothing is deleted recursively).
func removeEmptyFolders(dirs map[string]fileTarget) int {
	left := map[string]int{}
	keys := slices.Sorted(func(yield func(string) bool) {
		for k := range dirs {
			if !yield(k) {
				return
			}
		}
	})
	// The deepest first, so a parent is tried after its children.
	slices.SortStableFunc(keys, func(a, b string) int {
		return strings.Count(dirs[b].rel, "/") - strings.Count(dirs[a].rel, "/")
	})
	for _, k := range keys {
		d := dirs[k]
		for dir := d.rel; dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
			if err := paths.RemoveEmptyDir(d.root, dir); err != nil {
				if names, rerr := paths.ReadDirNames(d.root, dir, 0); rerr == nil {
					left[d.root.Dir()+"\x00"+dir] = len(names)
				}
				break
			}
			delete(left, d.root.Dir()+"\x00"+dir)
		}
	}
	n := 0
	for _, c := range left {
		n += c
	}
	return n
}

// dropDeletedFiles is the database half, in one transaction: the rows of the deleted files and of
// the recorded images that went; the titles of the tree that kept no file (and seasons left
// without a child) are deleted for good, and so is the title itself when no file of the tree is
// left (O-14). It returns how many titles left and the ids deleted for good.
func (s *Service) dropDeletedFiles(ctx context.Context, t itemTree, gone []string, extra []fileTarget) (int, []string, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for chunk := range slices.Chunk(gone, treeChunk) {
		if _, err := q.DeleteMediaFilesByID(ctx, chunk); err != nil {
			return 0, nil, fmt.Errorf("drop rows of deleted files: %w", err)
		}
	}
	for _, x := range extra {
		if err := q.DeleteLocalImageRows(ctx, systemq.DeleteLocalImageRowsParams{RootID: x.rootID, RelPath: x.rel}); err != nil {
			return 0, nil, fmt.Errorf("drop recorded image %s: %w", x.rel, err)
		}
	}
	var left int64
	for chunk := range slices.Chunk(t.ids, treeChunk) {
		n, err := q.CountMediaFilesOfItems(ctx, chunk)
		if err != nil {
			return 0, nil, fmt.Errorf("files left of %s: %w", t.item.ID, err)
		}
		left += n
	}
	var deleted []string
	if left == 0 && (t.item.Kind != "podcast_episode" || !t.item.EnclosureUrl.Valid) {
		deleted = append([]string{}, t.ids...)
	} else {
		owners := map[string]bool{}
		for _, f := range t.files {
			if slices.Contains(gone, f.ID) {
				owners[f.ItemID] = true
			}
		}
		candidates := slices.Sorted(func(yield func(string) bool) {
			for id := range owners {
				if !yield(id) {
					return
				}
			}
		})
		for chunk := range slices.Chunk(candidates, treeChunk) {
			ids, err := q.ListItemsWithoutFiles(ctx, chunk)
			if err != nil {
				return 0, nil, fmt.Errorf("titles without files: %w", err)
			}
			deleted = append(deleted, ids...)
		}
	}
	// The foreign keys take what hangs below a deleted title, so the rows a statement reports
	// undercount; the titles asked for are the count.
	for chunk := range slices.Chunk(deleted, treeChunk) {
		if _, err := q.HardDeleteItems(ctx, chunk); err != nil {
			return 0, nil, fmt.Errorf("delete titles for good: %w", err)
		}
	}
	// Containers left without a child go as well (review finding 7): seasons of the tree, then
	// the season and the series above the title (the last episode of a season, the last season of
	// a series), each after the one below it.
	var groups [][]string
	if left > 0 {
		groups = append(groups, t.ids)
	}
	for id, n := t.item.ParentID, 0; id.Valid && n < 2; n++ {
		groups = append(groups, []string{id.String})
		up, err := q.GetDeletionItem(ctx, systemq.GetDeletionItemParams{ID: id.String})
		if err != nil {
			return 0, nil, fmt.Errorf("load parent %s: %w", id.String, err)
		}
		id = up.ParentID
	}
	for _, group := range groups {
		for chunk := range slices.Chunk(group, treeChunk) {
			ids, err := q.ListEmptyContainers(ctx, chunk)
			if err != nil {
				return 0, nil, fmt.Errorf("empty containers: %w", err)
			}
			if len(ids) == 0 {
				continue
			}
			if _, err := q.HardDeleteItems(ctx, ids); err != nil {
				return 0, nil, fmt.Errorf("delete empty containers: %w", err)
			}
			deleted = append(deleted, ids...)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, fmt.Errorf("delete titles for good: %w", err)
	}
	return len(deleted), deleted, nil
}

// mountCheckTimeout bounds the read-only check of a library folder in the list: a hanging share
// must not hold the page.
const mountCheckTimeout = 2 * time.Second

// rootReadOnly tells whether the file system of a library folder is mounted read-only («Ordner
// von …»: «schreibbar» or «nur lesbar», ADR 0032), checked without writing; nil when the check
// did not answer in time or failed. A check that hangs keeps its place in mountChecks, so later
// lists answer nil at once instead of starting one more.
func (s *Service) rootReadOnly(dir string) *bool {
	if _, busy := s.mountChecks.LoadOrStore(dir, struct{}{}); busy {
		return nil
	}
	answer := make(chan *bool, 1)
	go func() {
		defer s.mountChecks.Delete(dir)
		root, err := paths.NewRoot(dir)
		if err != nil {
			answer <- nil
			return
		}
		ro, err := paths.ReadOnlyAt(root, ".")
		if err != nil {
			answer <- nil
			return
		}
		answer <- &ro
	}()
	select {
	case v := <-answer:
		return v
	case <-time.After(mountCheckTimeout):
		return nil
	}
}
