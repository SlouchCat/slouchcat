package metadata

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // registers JPEG for image.DecodeConfig (TMDB serves JPEG)
	_ "image/png"  // registers PNG for image.DecodeConfig
	"io"
	"net/http"
	"os"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// imagesDir is the directory below the cache root that holds one folder per item.
const imagesDir = "images"

// Image sizes stored in M1a, unchanged as TMDB delivers them (the pipeline follows in M1b).
const (
	posterSize   = "w500"
	backdropSize = "w1280"
)

// imageExt lists the accepted media types and their file extensions.
var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png"}

func (s *Service) handleImageFetch(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.fetchImages(ctx, itemID)
}

// fetchImages stores poster and backdrop of a matched item. It re-reads the English movie detail
// for the image paths, skips images whose source is unchanged and whose file exists, and stores
// nothing for a missing path.
func (s *Service) fetchImages(ctx context.Context, itemID string) error {
	if s.cacheE != nil {
		return s.cacheE
	}
	item, err := s.read.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("image.fetch: load item %s: %w", itemID, err)
	}
	if item.DeletedAt.Valid || item.MetadataState != stateMatched || !item.TmdbID.Valid {
		return nil
	}
	key, err := s.keys.TMDBKey(ctx)
	if err != nil {
		return fmt.Errorf("image.fetch: read tmdb key: %w", err)
	}
	if key == "" {
		return nil
	}
	c := s.client.WithKey(key)
	m, err := c.Movie(ctx, item.TmdbID.Int64, langEN)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		s.log.Warn("tmdb key rejected", "item_id", item.ID)
		return nil
	case err != nil:
		return fmt.Errorf("image.fetch %s: %w", item.ID, err)
	}

	root, err := paths.Open(s.cache)
	if err != nil {
		return fmt.Errorf("image.fetch: %w", err)
	}
	defer root.Close()

	locked := lockedFields(item.LockedFields)
	for _, img := range []struct{ kind, size, path string }{
		{"poster", posterSize, m.PosterPath},
		{"backdrop", backdropSize, m.BackdropPath},
	} {
		if img.path == "" || locked[img.kind] {
			continue
		}
		if err := s.storeImage(ctx, c, root, item.ID, img.kind, img.size, img.path); err != nil {
			return fmt.Errorf("image.fetch %s %s: %w", item.ID, img.kind, err)
		}
	}
	return nil
}

// storeImage downloads one image to images/<itemID>/<kind>-<size>.<ext> (temporary name, then
// rename) and upserts its row.
func (s *Service) storeImage(ctx context.Context, c *Client, root *os.Root, itemID, kind, size, tmdbPath string) error {
	sourceURL := c.imageURL(size, tmdbPath)
	prev, err := s.read.GetImageByItemKindSize(ctx, systemq.GetImageByItemKindSizeParams{ItemID: itemID, Kind: kind, Size: size})
	hasPrev := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load image row: %w", err)
	}
	if hasPrev && prev.SourceUrl == sourceURL {
		if _, err := root.Stat(prev.RelPath); err == nil {
			return nil
		}
	}

	data, mediaType, err := download(ctx, c, size, tmdbPath)
	if err != nil {
		return err
	}
	ext, ok := imageExt[mediaType]
	if !ok {
		return fmt.Errorf("unsupported image media type %q", mediaType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode image header: %w", err)
	}
	sum := sha256.Sum256(data)

	dir := imagesDir + "/" + itemID
	rel := dir + "/" + kind + "-" + size + "." + ext
	if err := root.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}
	if err := writeFile(root, dir, rel, data); err != nil {
		return err
	}
	if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
		ItemID:      itemID,
		Kind:        kind,
		Size:        size,
		SourceUrl:   sourceURL,
		RelPath:     rel,
		ContentHash: hex.EncodeToString(sum[:]),
		MediaType:   mediaType,
		Width:       int64(cfg.Width),
		Height:      int64(cfg.Height),
		Bytes:       int64(len(data)),
		Now:         s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("upsert image row: %w", err)
	}
	if hasPrev && prev.RelPath != rel {
		if err := root.Remove(prev.RelPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("remove replaced image", "item_id", itemID, "kind", kind, "err", err)
		}
	}
	return nil
}

// download reads the whole image (posters are a few hundred KiB) and determines its media type,
// sniffing the content when the answer has no usable Content-Type.
func download(ctx context.Context, c *Client, size, tmdbPath string) ([]byte, string, error) {
	body, mediaType, err := c.DownloadImage(ctx, size, tmdbPath)
	if err != nil {
		return nil, "", err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("%w: read image: %w", model.ErrTMDBUnreachable, err)
	}
	if len(data) > maxImageBytes {
		return nil, "", fmt.Errorf("image larger than %d bytes", maxImageBytes)
	}
	if _, ok := imageExt[mediaType]; !ok {
		mediaType = http.DetectContentType(data)
	}
	return data, mediaType, nil
}

// writeFile writes data to a temporary file in dir and renames it to rel, so readers never see a
// partial image.
func writeFile(root *os.Root, dir, rel string, data []byte) error {
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("temp name: %w", err)
	}
	tmp := dir + "/.tmp-" + hex.EncodeToString(rnd[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = root.Rename(tmp, rel)
	}
	if werr != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("write image file: %w", werr)
	}
	return nil
}
