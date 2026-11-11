package metadata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The upload of an own image for a kind of a title (uploadItemImage, concept 15 7.5). The image is
// stored on the path "locked without choice" that image.fetch already keeps (scaleLocked): the
// kind is locked, item_images has no row for it (a chosen row would make imageSource fetch from a
// provider), the original lies as <kind>-original.<ext> in the image cache with provider manual
// and source_url upload:<sha256>, and the derived sizes are scaled from it before anything of the
// kind changes (a file ffmpeg cannot decode is refused as unsupported_format). Lifting the
// lock (ClearItemImage) removes the upload and lets the match decide the kind again; image.fetch
// then fetches the provider's image, if there is one.

// maxUploadPixels caps the pixel count of an uploaded image (50 megapixels), checked from the
// header before ffmpeg decodes anything; ffmpeg itself refuses a larger frame as well
// (transcode.MaxImagePixels), whatever the header said.
const maxUploadPixels = transcode.MaxImagePixels

// The field codes of an upload refused by its content (field file, 422).
const (
	codeImageTooLarge     = "too_large"          // more than model.MaxImageUploadBytes
	codeImageUnsupported  = "unsupported_format" // no JPEG, PNG or still WebP by content, a damaged header or body
	codeImageTooManyPixel = "too_many_pixels"    // more than maxUploadPixels
)

// sourceUploadPrefix starts the source_url of an uploaded image; the SHA-256 of the file follows.
const sourceUploadPrefix = "upload:"

// UploadItemImage stores data as the image of a kind of a title and locks the kind. The format is
// taken from the content (JPEG, PNG or WebP; the file name and the declared type do not count),
// the pixel count from the header; then the sizes are derived from a temporary copy, and only when
// ffmpeg decodes it are the lock, the rows and the files changed. Errors: model.ErrNotFound,
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
	if mediaType == "image/webp" {
		if _, animated, _ := webpConfig(data); animated {
			return v.Add("file", codeImageUnsupported) // an image is one picture
		}
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxUploadPixels {
		return v.Add("file", codeImageTooManyPixel)
	}
	if s.cacheE != nil {
		return s.cacheE
	}
	unlock, err := s.lockItemImages(ctx, item.ID)
	if err != nil {
		return err
	}
	defer unlock()

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

	// The file goes to a temporary name first and the sizes are scaled from it: a file whose
	// header passes but whose body ffmpeg cannot decode leaves the rows, the lock and the files of
	// the kind as they were.
	tmp, err := tempName(dir)
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	tmp += "." + ext // originalWidth reads the type from the extension
	if err := writeFile(root, dir, tmp, data); err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	defer s.removeFile(root, item.ID, kind, tmp) // gone after the rename
	sc, err := s.scaleDerived(ctx, root, dir, item.ID, item.Kind, kind, tmp, storedAccent(prev))
	if errors.Is(err, transcode.ErrImageExit) {
		s.log.Info("uploaded image not decodable", "item_id", item.ID, "kind", kind, "media_type", mediaType, "err", err)
		return v.Add("file", codeImageUnsupported)
	}
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	defer s.dropScaled(root, item.ID, kind, sc)

	// Lock the kind, then move the original and the sizes into place.
	now := s.now().UnixMilli()
	err = s.inTx(ctx, func(q *systemq.Queries) error {
		cur, err := q.GetItemSystem(ctx, item.ID) // the locks as they stand now
		if err != nil {
			return err
		}
		if err := q.DeleteItemImage(ctx, systemq.DeleteItemImageParams{ItemID: item.ID, Kind: kind}); err != nil {
			return err
		}
		if err := q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: item.ID,
			LockedFields: addLocked(cur.LockedFields, kind), Now: now}); err != nil {
			return err
		}
		return q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: item.ID, Field: kind, Lang: "", Provider: ImageSourceManual})
	})
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	src := imageRequest{url: sourceUploadPrefix + hash, provider: ImageSourceManual}
	rel := dir + "/" + kind + "-" + originalSize + "." + ext
	if err := root.Rename(tmp, rel); err != nil {
		return fmt.Errorf("upload %s image of %s: rename original: %w", kind, item.ID, err)
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
	produced, err := s.storeDerived(ctx, root, dir, item.ID, kind, src, sc)
	if err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	if err := s.pruneSizes(ctx, root, item.ID, kind, prev, produced); err != nil {
		return fmt.Errorf("upload %s image of %s: %w", kind, item.ID, err)
	}
	s.log.Info("image uploaded", "item_id", item.ID, "kind", kind, "media_type", mediaType, "width", cfg.Width, "height", cfg.Height)
	return nil
}

// imageLock lets one image writer of a title work at a time; refs counts the writers holding or
// waiting for it, so the last one removes it from Service.imageLocks.
type imageLock struct {
	slot chan struct{}
	refs int
}

// lockItemImages waits until no other image writer of the title is at work and returns the
// function that lets the next one in. image.fetch and the upload take it, so a fetch that
// started before an upload cannot write its download over the uploaded original: the fetch
// reads the locks of the title only once it holds the slot, the upload changes them only while
// it holds it. The error is the context's while waiting.
func (s *Service) lockItemImages(ctx context.Context, itemID string) (func(), error) {
	s.imageMu.Lock()
	if s.imageLocks == nil {
		s.imageLocks = map[string]*imageLock{}
	}
	l := s.imageLocks[itemID]
	if l == nil {
		l = &imageLock{slot: make(chan struct{}, 1)}
		s.imageLocks[itemID] = l
	}
	l.refs++
	s.imageMu.Unlock()
	release := func() {
		s.imageMu.Lock()
		if l.refs--; l.refs == 0 {
			delete(s.imageLocks, itemID)
		}
		s.imageMu.Unlock()
	}
	select {
	case l.slot <- struct{}{}:
		return func() { <-l.slot; release() }, nil
	case <-ctx.Done():
		release()
		return nil, ctx.Err()
	}
}
