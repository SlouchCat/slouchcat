package metadata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The upload of an own image for a kind of a title (uploadItemImage, concept 15 7.5). The image is
// stored on the path "locked without choice" that image.fetch already keeps (scaleLocked): the
// kind is locked, item_images has no row for it (a chosen row would make imageSource fetch from a
// provider), the original lies as <kind>-original.<ext> in the image cache with provider manual
// and source_url upload:<sha256>, and the derived sizes are scaled from it right away. Lifting the
// lock (ClearItemImage) lets the match decide the kind again; image.fetch then replaces the upload,
// whose source no longer matches.

// maxUploadPixels caps the pixel count of an uploaded image (50 megapixels), checked from the
// header before ffmpeg decodes anything.
const maxUploadPixels = 50_000_000

// The field codes of an upload refused by its content (field file, 422).
const (
	codeImageTooLarge     = "too_large"          // more than model.MaxImageUploadBytes
	codeImageUnsupported  = "unsupported_format" // no JPEG, PNG or WebP by content, or a damaged header
	codeImageTooManyPixel = "too_many_pixels"    // more than maxUploadPixels
)

// sourceUploadPrefix starts the source_url of an uploaded image; the SHA-256 of the file follows.
const sourceUploadPrefix = "upload:"

// UploadItemImage stores data as the image of a kind of a title and locks the kind. The format is
// taken from the content (JPEG, PNG or WebP; the file name and the declared type do not count),
// the pixel count from the header; only then are the sizes derived. Errors: model.ErrNotFound,
// *model.ValidationError (kind invalid_value; file too_large, unsupported_format, too_many_pixels).
func (s *Service) UploadItemImage(ctx context.Context, itemID, kind string, data []byte) error {
	item, err := s.choosableNow(ctx, itemID, kind)
	if err != nil {
		return err
	}
	var v *model.ValidationError
	if len(data) > model.MaxImageUploadBytes {
		return v.Add("file", codeImageTooLarge)
	}
	mediaType := http.DetectContentType(data)
	ext, ok := imageExt[mediaType]
	if !ok {
		return v.Add("file", codeImageUnsupported)
	}
	cfg, err := imageConfig(data, mediaType)
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return v.Add("file", codeImageUnsupported)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxUploadPixels {
		return v.Add("file", codeImageTooManyPixel)
	}
	if s.cacheE != nil {
		return s.cacheE
	}

	// Lock first, so an image.fetch that starts meanwhile keeps the kind (scaleLocked).
	now := s.now().UnixMilli()
	err = s.inTx(ctx, func(q *systemq.Queries) error {
		if err := q.DeleteItemImage(ctx, systemq.DeleteItemImageParams{ItemID: item.ID, Kind: kind}); err != nil {
			return err
		}
		if err := q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: item.ID,
			LockedFields: addLocked(item.LockedFields, kind), Now: now}); err != nil {
			return err
		}
		return q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: item.ID, Field: kind, Lang: "", Provider: ImageSourceManual})
	})
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}

	root, err := paths.Open(s.cache)
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	defer root.Close()
	rows, err := s.read.ListImagesByItem(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("upload %s image of %s: list images: %w", kind, item.ID, err)
	}
	var prev []systemq.Image
	prevOrig := ""
	for _, r := range rows {
		if r.Kind != kind {
			continue
		}
		prev = append(prev, r)
		if r.Size == originalSize {
			prevOrig = r.RelPath
		}
	}
	dir := imagesDir + "/" + item.ID
	if err := root.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("upload %s image of %s: create image dir: %w", kind, item.ID, err)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	src := imageRequest{url: sourceUploadPrefix + hash, provider: ImageSourceManual}
	rel := dir + "/" + kind + "-" + originalSize + "." + ext
	if err := writeFile(root, dir, rel, data); err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
		ItemID: item.ID, Kind: kind, Size: originalSize, Format: originalFormat,
		Lang: sql.NullString{String: "", Valid: true}, Provider: ImageSourceManual, SourceUrl: src.url,
		RelPath: rel, ContentHash: hash, MediaType: mediaType,
		Width: int64(cfg.Width), Height: int64(cfg.Height), Bytes: int64(len(data)), Now: now,
	}); err != nil {
		return fmt.Errorf("upload %s image of %s: upsert original row: %w", kind, item.ID, err)
	}
	if prevOrig != "" && prevOrig != rel {
		s.removeFile(root, item.ID, kind, prevOrig) // the former original had another extension
	}
	if err := s.deriveAndPrune(ctx, root, dir, item.ID, kind, rel, src, prev); err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	s.log.Info("image uploaded", "item_id", item.ID, "kind", kind, "media_type", mediaType, "width", cfg.Width, "height", cfg.Height)
	return nil
}
