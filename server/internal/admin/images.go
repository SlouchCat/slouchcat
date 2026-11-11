package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Images of the administration tables (M11 step 5b): system-wide, without a profile check, so a
// pick for a child profile shows its poster although the main profile may not watch the title.

// derivedImageSizes are the sizes of the image pipeline per kind; a missing one is answered with
// the widest stored size of the kind (decision of 11 November 2026), any other size is 404.
var derivedImageSizes = map[string]map[string]bool{
	"poster":   {"w200": true, "w400": true, "w800": true},
	"backdrop": {"w780": true, "w1280": true, "w1920": true, "w3840": true},
	"logo":     {"w400": true, "w800": true},
}

// ItemImage opens a stored image of a title that is not deleted: the WebP row when preferWebP and
// stored, else the JPEG row. The file is opened through package paths below the cache directory of
// WithSystem (CLAUDE.md rule 4). A missing size of the pipeline is answered with the widest stored
// one. model.ErrNotFound for an unknown title, a missing row or a row whose file is gone;
// ErrNotConfigured without a cache directory.
func (s *Service) ItemImage(ctx context.Context, itemID, kind, size string, preferWebP bool) (model.StreamSource, error) {
	if s.ops.system.CacheDir == "" {
		return model.StreamSource{}, ErrNotConfigured
	}
	fallback := "jpeg"
	if kind == "logo" {
		fallback = "png" // a logo keeps its transparency (ADR 0018)
	}
	format := fallback
	if preferWebP {
		format = "webp"
	}
	im, err := s.sys.AdminGetItemImage(ctx, systemq.AdminGetItemImageParams{ItemID: itemID, Kind: kind, Size: size, Format: format, Fallback: fallback})
	if errors.Is(err, sql.ErrNoRows) && derivedImageSizes[kind][size] {
		// A size the pipeline did not derive (never wider than the original): the widest stored one.
		var w systemq.AdminGetWidestItemImageRow
		w, err = s.sys.AdminGetWidestItemImage(ctx, systemq.AdminGetWidestItemImageParams{ItemID: itemID, Kind: kind, Format: format, Fallback: fallback})
		im = systemq.AdminGetItemImageRow(w)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("get item image: %w", err)
	}
	root, err := paths.NewRoot(s.ops.system.CacheDir)
	if err != nil {
		return model.StreamSource{}, err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return model.StreamSource{}, err
	}
	defer dir.Close() // files opened through the root stay usable after it is closed
	f, err := dir.Open(im.RelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.log.WarnContext(ctx, "image row without file", "item_id", itemID, "kind", kind, "size", size)
			return model.StreamSource{}, model.ErrNotFound
		}
		return model.StreamSource{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat image: %w", err), f.Close())
	}
	return model.StreamSource{
		File:      f,
		MediaType: im.MediaType,
		Size:      info.Size(),
		ModTime:   time.UnixMilli(im.CreatedAt).UTC(),
		Name:      itemID + "/" + kind + "/" + size,
	}, nil
}

// posterHashes returns item id → content hash of the model.AdminPosterSize poster for the titles
// of ids that have one, in one query.
func posterHashes(ctx context.Context, q *systemq.Queries, ids []string) (map[string]string, error) {
	return imageHashes(ctx, q, "poster", model.AdminPosterSize, ids)
}

// backdropHashes returns item id → content hash of the model.AdminBackdropSize backdrop for the
// titles of ids that have one, in one query (concept 12).
func backdropHashes(ctx context.Context, q *systemq.Queries, ids []string) (map[string]string, error) {
	return imageHashes(ctx, q, "backdrop", model.AdminBackdropSize, ids)
}

// imageHashes returns item id → content hash of the image of kind in size for the titles of ids
// that have one, in one query.
func imageHashes(ctx context.Context, q *systemq.Queries, kind, size string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.AdminListImages(ctx, systemq.AdminListImagesParams{ItemIds: ids, Kind: kind, Size: size})
	if err != nil {
		return nil, fmt.Errorf("list %s images: %w", kind, err)
	}
	for _, r := range rows {
		out[r.ItemID] = r.ContentHash
	}
	return out, nil
}

// withPosters sets the poster hash of each row of list; poster returns the item id of a row and
// where its hash goes.
func withPosters[T any](ctx context.Context, q *systemq.Queries, list []T, poster func(*T) (itemID string, hash *string)) error {
	ids := make([]string, 0, len(list))
	for i := range list {
		id, _ := poster(&list[i])
		ids = append(ids, id)
	}
	hashes, err := posterHashes(ctx, q, ids)
	if err != nil {
		return err
	}
	for i := range list {
		id, hash := poster(&list[i])
		*hash = hashes[id]
	}
	return nil
}
