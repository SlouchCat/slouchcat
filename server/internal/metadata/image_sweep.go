package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The image cache holds one folder per title, images/<item id>/ (images.go). A hard-deleted title
// (a merge, a relink) takes its rows with it (ON DELETE CASCADE), its folder goes here: at once
// through RemoveItemImages, and in the hourly library.sweep through SweepOrphanImages for every
// folder the first missed (a cascaded child, a stopped server). A folder stays while an image row
// of another title names it: a relink moves the copy's cached images to the survivor with their
// files (RelinkMoveCachedImages). Only folders named like an item ID are touched, so
// images/radio/ (package radio) stays as it is.

// imageFolderOf returns the item folder ("images/<id>") of a rel_path in it, or "".
func imageFolderOf(rel string) string {
	rest, ok := strings.CutPrefix(rel, imagesDir+"/")
	if !ok {
		return ""
	}
	name, _, ok := strings.Cut(rest, "/")
	if !ok || !isULID(name) {
		return ""
	}
	return imagesDir + "/" + name
}

// foreignFolders returns the item folders named by image rows of other titles.
func (s *Service) foreignFolders(ctx context.Context) (map[string]bool, error) {
	rels, err := s.read.ListForeignImagePaths(ctx)
	if err != nil {
		return nil, fmt.Errorf("list image rows in other folders: %w", err)
	}
	out := map[string]bool{}
	for _, rel := range rels {
		if dir := imageFolderOf(rel); dir != "" {
			out[dir] = true
		}
	}
	return out, nil
}

// RemoveItemImages removes the image folders of hard-deleted titles: an ID whose title still
// exists (soft-deleted ones too, they may come back) or whose folder another title's row names is
// left alone. A failure is returned after the other folders were tried; the sweep tries again.
func (s *Service) RemoveItemImages(ctx context.Context, ids []string) error {
	if s.cacheE != nil || len(ids) == 0 {
		return s.cacheE
	}
	foreign, err := s.foreignFolders(ctx)
	if err != nil {
		return err
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return fmt.Errorf("remove item images: %w", err)
	}
	defer root.Close()
	var errs []error
	for _, id := range ids {
		if !isULID(id) || foreign[imagesDir+"/"+id] {
			continue
		}
		if _, err := s.read.GetItemSystem(ctx, id); err == nil {
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			errs = append(errs, fmt.Errorf("load item %s: %w", id, err))
			continue
		}
		if _, err := s.removeImageFolder(ctx, root, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// removeImageFolder removes images/<id>/ under the image lock of the title (no image.fetch writes
// into it meanwhile); it reports whether there was a folder.
func (s *Service) removeImageFolder(ctx context.Context, root *os.Root, id string) (bool, error) {
	dir := imagesDir + "/" + id
	if _, err := root.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	unlock, err := s.lockItemImages(ctx, id)
	if err != nil {
		return false, err
	}
	defer unlock()
	if err := root.RemoveAll(dir); err != nil { //nolint:forbidigo // the cache root (images), never a library root (ADR 0032)
		return false, fmt.Errorf("remove image folder of %s: %w", id, err)
	}
	return true, nil
}

// SweepOrphanImages removes every item folder below images/ whose title no longer exists and
// whose files no other title's row names, and returns how many went. The folders are listed before
// the titles are read, so a title created meanwhile (its folder comes after its row) is never
// taken for an orphan. The hourly library.sweep runs it.
func (s *Service) SweepOrphanImages(ctx context.Context) (int, error) {
	if s.cacheE != nil {
		return 0, s.cacheE
	}
	names, err := paths.ReadDirNames(s.cache, imagesDir, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("list image folders: %w", err)
	}
	ids, err := s.read.ListAllItemIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list titles: %w", err)
	}
	live := make(map[string]bool, len(ids))
	for _, id := range ids {
		live[id] = true
	}
	foreign, err := s.foreignFolders(ctx)
	if err != nil {
		return 0, err
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return 0, fmt.Errorf("image sweep: %w", err)
	}
	defer root.Close()
	removed := 0
	var errs []error
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		if !isULID(name) || live[name] || foreign[imagesDir+"/"+name] {
			continue // images/radio and the folders of titles that exist
		}
		ok, err := s.removeImageFolder(ctx, root, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			removed++
		}
	}
	if removed > 0 {
		s.log.InfoContext(ctx, "image folders of deleted titles removed", "folders", removed)
	}
	return removed, errors.Join(errs...)
}

// SettingImageSizesTrimmed marks that the stored images were trimmed to the sizes of 1.28.1 once.
const SettingImageSizesTrimmed = "metadata.image_sizes_trimmed"

// TrimImageSizesOnce brings the stored images of every title to the sizes of 1.28.1 once, without
// a download or an ffmpeg run (decision of 11 November 2026): a kind that holds every size it
// needs loses the sizes the pipeline no longer makes (the w1920 and w3840 stills of episodes, the
// sizes scaled up from a narrow original) and, unless the administrator locked it, its original
// (keepOriginal). A kind that misses a size stays as it is for image.fetch. The setting
// An uploaded original (provider manual) stays as well, even when its lock is gone: it is the only
// copy. SettingImageSizesTrimmed remembers a complete run; a failed or stopped run starts again at the
// next start. It returns the number of rows removed.
func (s *Service) TrimImageSizesOnce(ctx context.Context) (int, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingImageSizesTrimmed); err == nil {
		return 0, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read %s: %w", SettingImageSizesTrimmed, err)
	}
	if s.cacheE != nil {
		return 0, s.cacheE
	}
	ids, err := s.read.ListImageItemIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list titles with images: %w", err)
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return 0, fmt.Errorf("trim images: %w", err)
	}
	defer root.Close()
	removed, failed := 0, 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		n, err := s.trimItemImages(ctx, root, id)
		removed += n
		if err != nil {
			s.log.WarnContext(ctx, "trim the images of a title", "item_id", id, "err", err)
			failed++
		}
	}
	if failed > 0 {
		return removed, fmt.Errorf("images of %d titles not trimmed", failed)
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingImageSizesTrimmed, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return removed, fmt.Errorf("write %s: %w", SettingImageSizesTrimmed, err)
	}
	s.log.InfoContext(ctx, "stored images trimmed to the delivered sizes", "titles", len(ids), "rows", removed)
	return removed, nil
}

// trimItemImages trims the kinds of one title under its image lock and returns the rows removed.
func (s *Service) trimItemImages(ctx context.Context, root *os.Root, id string) (int, error) {
	unlock, err := s.lockItemImages(ctx, id)
	if err != nil {
		return 0, err
	}
	defer unlock()
	item, err := s.read.GetItemSystem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil // deleted meanwhile, the sweep takes its folder
	}
	if err != nil {
		return 0, fmt.Errorf("load item: %w", err)
	}
	rows, err := s.read.ListImagesByItem(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("list images: %w", err)
	}
	byKind := map[string][]systemq.Image{}
	for _, r := range rows {
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}
	locked := lockedFields(item.LockedFields)
	exists := rootExists(root)
	removed := 0
	for kind, prev := range byKind {
		if covered, _ := kindCovered(item.Kind, kind, storedImages(prev), "", exists); !covered {
			continue
		}
		uploaded := slices.ContainsFunc(prev, func(r systemq.Image) bool { return r.Size == originalSize && r.Provider == ImageSourceManual })
		kept, err := s.trimKind(ctx, root, id, item.Kind, kind, prev, !keepOriginal && !locked[kind] && !uploaded)
		if err != nil {
			return removed, err
		}
		removed += len(prev) - len(kept)
	}
	return removed, nil
}
