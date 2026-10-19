package metadata

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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

// Image sizes stored in M1a, unchanged as TMDB delivers them (the pipeline follows in M1b). The
// small backdrop serves the hero on phones (P-10).
const (
	posterSize        = "w500"
	backdropSize      = "w1280"
	backdropSmallSize = "w780"
)

// storedImages lists every image image.fetch stores for a title: kind and TMDB size.
var storedImages = []struct{ kind, size string }{
	{"poster", posterSize},
	{"backdrop", backdropSize},
	{"backdrop", backdropSmallSize},
}

// imageExt lists the accepted media types and their file extensions.
var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png"}

func (s *Service) handleImageFetch(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.fetchImages(ctx, itemID)
}

// fetchImages stores the poster and both backdrop sizes of a matched item. It re-reads the English
// movie detail for the image paths, skips images whose source is unchanged and whose file exists
// (so a run after a new size was added fetches only that size), and stores nothing for a missing
// path.
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
	tmdbPath := map[string]string{"poster": m.PosterPath, "backdrop": m.BackdropPath}
	for _, img := range storedImages {
		path := tmdbPath[img.kind]
		if path == "" || locked[img.kind] {
			continue
		}
		if err := s.storeImage(ctx, c, root, item.ID, img.kind, img.size, path); err != nil {
			return fmt.Errorf("image.fetch %s %s %s: %w", item.ID, img.kind, img.size, err)
		}
	}
	return nil
}

// QueueIncompleteImages enqueues image.fetch (key image:<itemId>) for every matched title of the
// library whose images are incomplete: a stored kind lacks one of its sizes (for example w780 next
// to an older w1280 backdrop) or the file of a stored size is gone from the cache. Locked kinds are
// left alone, and titles without any image row are not checked (TMDB has none, or their first
// image.fetch is pending), so a title without artwork costs no TMDB request per scan. The library
// scan calls it after every successful run; it returns the number of titles queued.
func (s *Service) QueueIncompleteImages(ctx context.Context, libraryID int64) (int, error) {
	if s.cacheE != nil {
		return 0, s.cacheE
	}
	rows, err := s.read.ListMatchedItemImages(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list matched images: %w", err)
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return 0, fmt.Errorf("image check: %w", err)
	}
	defer root.Close()

	type title struct {
		locked map[string]bool
		kinds  map[string]bool // stored kinds
		files  map[string]bool // "<kind>/<size>" whose file exists
	}
	var order []string
	titles := map[string]*title{}
	for _, r := range rows {
		t := titles[r.ItemID]
		if t == nil {
			t = &title{locked: lockedFields(r.LockedFields), kinds: map[string]bool{}, files: map[string]bool{}}
			titles[r.ItemID] = t
			order = append(order, r.ItemID)
		}
		if t.locked[r.Kind] {
			continue
		}
		t.kinds[r.Kind] = true
		if _, err := root.Stat(r.RelPath); err == nil {
			t.files[r.Kind+"/"+r.Size] = true
		}
	}
	queued := 0
	for _, id := range order {
		t := titles[id]
		complete := true
		for _, img := range storedImages {
			if t.kinds[img.kind] && !t.files[img.kind+"/"+img.size] {
				complete = false
			}
		}
		if complete {
			continue
		}
		payload, err := json.Marshal(itemPayload{ItemID: id})
		if err != nil {
			return queued, fmt.Errorf("image payload: %w", err)
		}
		if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
			Type:           JobImageFetch,
			Priority:       imageFetchPriority,
			IdempotencyKey: "image:" + id,
			Payload:        payload,
		}); err != nil {
			return queued, fmt.Errorf("enqueue image.fetch %s: %w", id, err)
		}
		queued++
	}
	return queued, nil
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
