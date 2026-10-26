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

// ItemImage opens a stored image of a title that is not deleted: the WebP row when preferWebP and
// stored, else the JPEG row. The file is opened through package paths below the cache directory of
// WithSystem (CLAUDE.md rule 4). model.ErrNotFound for an unknown title, a missing row or a row
// whose file is gone; ErrNotConfigured without a cache directory.
func (s *Service) ItemImage(ctx context.Context, itemID, kind, size string, preferWebP bool) (model.StreamSource, error) {
	if s.ops.system.CacheDir == "" {
		return model.StreamSource{}, ErrNotConfigured
	}
	format := "jpeg"
	if preferWebP {
		format = "webp"
	}
	im, err := s.sys.AdminGetItemImage(ctx, systemq.AdminGetItemImageParams{ItemID: itemID, Kind: kind, Size: size, Format: format})
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
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.AdminListPosters(ctx, systemq.AdminListPostersParams{ItemIds: ids, Size: model.AdminPosterSize})
	if err != nil {
		return nil, fmt.Errorf("list posters: %w", err)
	}
	for _, r := range rows {
		out[r.ItemID] = r.ContentHash
	}
	return out, nil
}

// withPosters sets the poster hash of each pick.
func withPosters(ctx context.Context, q *systemq.Queries, picks []model.AdminPick) error {
	ids := make([]string, 0, len(picks))
	for _, p := range picks {
		ids = append(ids, p.ItemID)
	}
	hashes, err := posterHashes(ctx, q, ids)
	if err != nil {
		return err
	}
	for i := range picks {
		picks[i].PosterHash = hashes[picks[i].ItemID]
	}
	return nil
}
