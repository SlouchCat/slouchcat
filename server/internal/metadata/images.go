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
	_ "image/jpeg" // registers JPEG for image.DecodeConfig (TMDB originals, derived JPEGs)
	_ "image/png"  // registers PNG for image.DecodeConfig
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// imagesDir is the directory below the cache root that holds one folder per item.
const imagesDir = "images"

// Image pipeline (plan 3.1, M1b). image.fetch downloads the TMDB original of a kind once, keeps
// it as images/<item>/<kind>-original.<ext> (row size 'original', format 'jpeg' as its single
// slot) and derives every delivered width as WebP and JPEG with ffmpeg:
// images/<item>/<kind>-w<width>.webp and .jpg. Derived rows carry the original's URL as
// source_url, so a changed TMDB image is detected. The accent colour comes from palettegen on the
// w200 poster JPEG and is stored on every derived poster row.
const (
	originalSize      = "original"
	originalFormat    = "jpeg"
	accentSourceWidth = 200
	accentColors      = 8
)

// imageKinds lists the stored kinds in processing order.
var imageKinds = []string{"poster", "backdrop"}

// imageWidths lists the delivered widths per kind.
var imageWidths = map[string][]int{
	"poster":   {200, 400, 800},
	"backdrop": {780, 1280, 1920},
}

// imageFormats lists the delivered formats with their file extension and media type. JPEG comes
// first: its header gives the dimensions of both formats of a width.
var imageFormats = []struct {
	format    transcode.ImageFormat
	ext       string
	mediaType string
}{
	{transcode.ImageJPEG, "jpg", "image/jpeg"},
	{transcode.ImageWebP, "webp", "image/webp"},
}

// imageExt lists the accepted media types of downloaded originals and their file extensions.
var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png"}

// sizeName is the size column of a derived width ("w400").
func sizeName(width int) string { return "w" + strconv.Itoa(width) }

func (s *Service) handleImageFetch(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.fetchImages(ctx, itemID)
}

// fetchImages runs the image pipeline for a matched item. It re-reads the English movie detail for
// the image paths and, per kind, skips the work when every derived row stems from the current
// TMDB original and its file exists (so a run after a deleted file or a new size only fills the
// gap). The original is downloaded only when its row is missing, stale or its file is gone. A kind
// without a TMDB path keeps its rows. A kind locked by the admin is never fetched from TMDB, but
// its missing sizes are scaled from the stored image (scaleLocked).
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
	root, err := paths.Open(s.cache)
	if err != nil {
		return fmt.Errorf("image.fetch: %w", err)
	}
	defer root.Close()
	rows, err := s.read.ListImagesByItem(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("image.fetch: list images of %s: %w", item.ID, err)
	}
	byKind := map[string][]systemq.Image{}
	for _, r := range rows {
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}
	locked := lockedFields(item.LockedFields)
	unlocked := 0
	for _, kind := range imageKinds {
		if !locked[kind] {
			unlocked++
			continue
		}
		if err := s.scaleLocked(ctx, root, item.ID, kind, byKind[kind]); err != nil {
			return fmt.Errorf("image.fetch %s %s: %w", item.ID, kind, err)
		}
	}
	if unlocked == 0 {
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
	tmdbPath := map[string]string{"poster": m.PosterPath, "backdrop": m.BackdropPath}
	for _, kind := range imageKinds {
		path := tmdbPath[kind]
		if path == "" || locked[kind] {
			continue
		}
		if err := s.processKind(ctx, c, root, item.ID, kind, path, byKind[kind]); err != nil {
			return fmt.Errorf("image.fetch %s %s: %w", item.ID, kind, err)
		}
	}
	return nil
}

// storedImage is the part of an image row the completeness check needs.
type storedImage struct {
	size, format, sourceURL, relPath string
}

// kindComplete reports whether the rows of one kind hold every derived width in both formats with
// an existing file and nothing else apart from the original. A non-empty sourceURL must match the
// source_url of every derived row (image.fetch knows the current TMDB original; the check after a
// scan does not).
func kindComplete(kind string, rows []storedImage, sourceURL string, exists func(rel string) bool) bool {
	want := map[string]bool{}
	for _, w := range imageWidths[kind] {
		for _, f := range imageFormats {
			want[sizeName(w)+"/"+string(f.format)] = true
		}
	}
	have := 0
	for _, r := range rows {
		if r.size == originalSize {
			continue
		}
		if !want[r.size+"/"+r.format] {
			return false // a size the pipeline no longer produces (M1a's w500 poster)
		}
		if (sourceURL != "" && r.sourceURL != sourceURL) || !exists(r.relPath) {
			return false
		}
		have++
	}
	return have == len(want)
}

// processKind brings the rows and files of one kind up to date with the TMDB original at
// tmdbPath.
func (s *Service) processKind(ctx context.Context, c *Client, root *os.Root, itemID, kind, tmdbPath string, prev []systemq.Image) error {
	sourceURL := c.imageURL(originalSize, tmdbPath)
	exists := rootExists(root)
	var orig *systemq.Image
	for i, r := range prev {
		if r.Size == originalSize {
			orig = &prev[i]
		}
	}
	if kindComplete(kind, storedImages(prev), sourceURL, exists) {
		return s.fillAccent(ctx, itemID, kind, prev)
	}

	dir := imagesDir + "/" + itemID
	if err := root.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}
	origRel := ""
	if orig != nil && orig.SourceUrl == sourceURL && exists(orig.RelPath) {
		origRel = orig.RelPath
	} else {
		rel, err := s.storeOriginal(ctx, c, root, dir, itemID, kind, tmdbPath, sourceURL, orig)
		if err != nil {
			return err
		}
		origRel = rel
	}
	return s.deriveAndPrune(ctx, root, dir, itemID, kind, origRel, sourceURL, prev)
}

// deriveAndPrune derives every size of kind from the original at origRel and removes the rows
// (and files) of sizes the pipeline no longer produces (M1a's w500 poster). A palette failure
// keeps the accent the poster rows had.
func (s *Service) deriveAndPrune(ctx context.Context, root *os.Root, dir, itemID, kind, origRel, sourceURL string, prev []systemq.Image) error {
	produced, err := s.deriveImages(ctx, root, dir, itemID, kind, origRel, sourceURL, storedAccent(prev))
	if err != nil {
		return err
	}
	for _, r := range prev {
		if r.Size == originalSize || produced[r.Size+"/"+r.Format] {
			continue
		}
		if err := s.write.DeleteImage(ctx, r.ID); err != nil {
			return fmt.Errorf("delete image row: %w", err)
		}
		s.removeFile(root, itemID, kind, r.RelPath)
	}
	return nil
}

// scaleLocked brings a kind the admin locked up to the current sizes without asking TMDB: locked
// means "keep this image", not "keep these sizes". The derived sizes are scaled from the stored
// original; without one (M1a stored no original) the widest stored JPEG becomes the original, so
// the M1a w500 poster turns into w200, w400 and w800 (w800 slightly upscaled). A kind without any
// usable file is left alone.
func (s *Service) scaleLocked(ctx context.Context, root *os.Root, itemID, kind string, prev []systemq.Image) error {
	exists := rootExists(root)
	if len(prev) == 0 {
		return nil
	}
	if kindComplete(kind, storedImages(prev), "", exists) {
		return s.fillAccent(ctx, itemID, kind, prev)
	}
	src := lockedSource(prev, exists)
	if src == nil {
		return nil
	}
	dir := imagesDir + "/" + itemID
	if src.Size == originalSize {
		return s.deriveAndPrune(ctx, root, dir, itemID, kind, src.RelPath, src.SourceUrl, prev)
	}
	data, err := root.ReadFile(src.RelPath)
	if err != nil {
		return fmt.Errorf("read stored %s: %w", kind, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode stored %s header: %w", kind, err)
	}
	rel := dir + "/" + kind + "-" + originalSize + ".jpg"
	if err := writeFile(root, dir, rel, data); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
		ItemID: itemID, Kind: kind, Size: originalSize, Format: originalFormat, SourceUrl: src.SourceUrl,
		RelPath: rel, ContentHash: hex.EncodeToString(sum[:]), MediaType: "image/jpeg",
		Width: int64(cfg.Width), Height: int64(cfg.Height), Bytes: int64(len(data)), Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("upsert original row: %w", err)
	}
	return s.deriveAndPrune(ctx, root, dir, itemID, kind, rel, src.SourceUrl, prev)
}

// lockedSource returns the stored image a locked kind is scaled from: the original when its file
// exists, else the widest derived JPEG with an existing file, or nil.
func lockedSource(prev []systemq.Image, exists func(rel string) bool) *systemq.Image {
	var best *systemq.Image
	for i := range prev {
		r := &prev[i]
		if !exists(r.RelPath) {
			continue
		}
		if r.Size == originalSize {
			return r
		}
		if r.Format == string(transcode.ImageJPEG) && r.MediaType == "image/jpeg" && (best == nil || r.Width > best.Width) {
			best = r
		}
	}
	return best
}

// fillAccent computes the accent of a complete poster whose derived rows carry none (the palette
// failed on an earlier run) from its w200 JPEG and stores it on every derived row. A failure again
// leaves the rows as they are; QueueIncompleteImages bounds the retries (maxAccentRetries).
func (s *Service) fillAccent(ctx context.Context, itemID, kind string, prev []systemq.Image) error {
	if kind != "poster" || storedAccent(prev).Valid {
		return nil
	}
	var src *systemq.Image
	for i := range prev {
		if prev[i].Size == sizeName(accentSourceWidth) && prev[i].Format == string(transcode.ImageJPEG) {
			src = &prev[i]
		}
	}
	if src == nil {
		return nil
	}
	abs, err := paths.Resolve(s.cache, src.RelPath)
	if err != nil {
		return fmt.Errorf("resolve accent source: %w", err)
	}
	accent := s.accent(ctx, itemID, abs)
	if !accent.Valid {
		return nil
	}
	for _, r := range prev {
		if r.Size == originalSize {
			continue
		}
		if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
			ItemID: r.ItemID, Kind: r.Kind, Size: r.Size, Format: r.Format, Lang: r.Lang, SourceUrl: r.SourceUrl,
			RelPath: r.RelPath, ContentHash: r.ContentHash, MediaType: r.MediaType, Width: r.Width, Height: r.Height,
			Bytes: r.Bytes, Accent: accent, Now: r.CreatedAt,
		}); err != nil {
			return fmt.Errorf("store accent: %w", err)
		}
	}
	return nil
}

// storedAccent returns the accent of the derived poster rows, or an invalid value.
func storedAccent(prev []systemq.Image) sql.NullString {
	for _, r := range prev {
		if r.Kind == "poster" && r.Size != originalSize && r.Accent.Valid {
			return r.Accent
		}
	}
	return sql.NullString{}
}

func storedImages(prev []systemq.Image) []storedImage {
	out := make([]storedImage, 0, len(prev))
	for _, r := range prev {
		out = append(out, storedImage{size: r.Size, format: r.Format, sourceURL: r.SourceUrl, relPath: r.RelPath})
	}
	return out
}

func rootExists(root *os.Root) func(rel string) bool {
	return func(rel string) bool {
		_, err := root.Stat(rel)
		return err == nil
	}
}

// storeOriginal downloads the TMDB original to <dir>/<kind>-original.<ext> (temporary name, then
// rename), upserts its row and returns its rel_path. A replaced original with another extension
// is removed.
func (s *Service) storeOriginal(ctx context.Context, c *Client, root *os.Root, dir, itemID, kind, tmdbPath, sourceURL string, prev *systemq.Image) (string, error) {
	data, mediaType, err := download(ctx, c, originalSize, tmdbPath)
	if err != nil {
		return "", err
	}
	ext, ok := imageExt[mediaType]
	if !ok {
		return "", fmt.Errorf("unsupported image media type %q", mediaType)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image header: %w", err)
	}
	rel := dir + "/" + kind + "-" + originalSize + "." + ext
	if err := writeFile(root, dir, rel, data); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
		ItemID:      itemID,
		Kind:        kind,
		Size:        originalSize,
		Format:      originalFormat,
		SourceUrl:   sourceURL,
		RelPath:     rel,
		ContentHash: hex.EncodeToString(sum[:]),
		MediaType:   mediaType,
		Width:       int64(cfg.Width),
		Height:      int64(cfg.Height),
		Bytes:       int64(len(data)),
		Now:         s.now().UnixMilli(),
	}); err != nil {
		return "", fmt.Errorf("upsert original row: %w", err)
	}
	if prev != nil && prev.RelPath != rel {
		s.removeFile(root, itemID, kind, prev.RelPath)
	}
	return rel, nil
}

// derived is one scaled output of deriveImages before it is renamed into place.
type derived struct {
	width     int
	format    transcode.ImageFormat
	ext       string
	mediaType string
	tmp       string // rel_path of the temporary file
}

// deriveImages scales the original at origRel into every width and format of kind in one ffmpeg
// run, computes the accent colour for posters (prevAccent stays when the palette fails), renames
// the files into place and upserts their rows. It returns the produced "<size>/<format>" keys.
func (s *Service) deriveImages(ctx context.Context, root *os.Root, dir, itemID, kind, origRel, sourceURL string, prevAccent sql.NullString) (map[string]bool, error) {
	in, err := paths.Resolve(s.cache, origRel)
	if err != nil {
		return nil, fmt.Errorf("resolve original: %w", err)
	}
	var outs []derived
	var args []transcode.ImageOutput
	defer func() {
		for _, o := range outs {
			if err := root.Remove(o.tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
				s.log.Warn("remove temporary image", "item_id", itemID, "kind", kind, "err", err)
			}
		}
	}()
	for _, w := range imageWidths[kind] {
		for _, f := range imageFormats {
			tmp, err := tempName(dir)
			if err != nil {
				return nil, err
			}
			abs, err := paths.Resolve(s.cache, tmp)
			if err != nil {
				return nil, fmt.Errorf("resolve temporary image: %w", err)
			}
			outs = append(outs, derived{width: w, format: f.format, ext: f.ext, mediaType: f.mediaType, tmp: tmp})
			args = append(args, transcode.ImageOutput{Path: abs, Width: w, Format: f.format})
		}
	}
	if err := s.runner.ScaleImage(ctx, in, args); err != nil {
		return nil, fmt.Errorf("scale %s: %w", origRel, err)
	}

	var accent sql.NullString
	if kind == "poster" {
		for i, o := range outs {
			if o.width == accentSourceWidth && o.format == transcode.ImageJPEG {
				accent = s.accent(ctx, itemID, args[i].Path)
			}
		}
		if !accent.Valid {
			accent = prevAccent
		}
	}

	dims := map[int]image.Config{} // per width, from the JPEG (first in imageFormats)
	produced := map[string]bool{}
	for _, o := range outs {
		data, err := root.ReadFile(o.tmp)
		if err != nil {
			return nil, fmt.Errorf("read scaled image: %w", err)
		}
		if o.format == transcode.ImageJPEG {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("decode scaled image header: %w", err)
			}
			dims[o.width] = cfg
		}
		size := sizeName(o.width)
		rel := dir + "/" + kind + "-" + size + "." + o.ext
		if err := root.Rename(o.tmp, rel); err != nil {
			return nil, fmt.Errorf("rename scaled image: %w", err)
		}
		sum := sha256.Sum256(data)
		var rowAccent sql.NullString
		if kind == "poster" {
			rowAccent = accent
		}
		if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
			ItemID:      itemID,
			Kind:        kind,
			Size:        size,
			Format:      string(o.format),
			SourceUrl:   sourceURL,
			RelPath:     rel,
			ContentHash: hex.EncodeToString(sum[:]),
			MediaType:   o.mediaType,
			Width:       int64(dims[o.width].Width),
			Height:      int64(dims[o.width].Height),
			Bytes:       int64(len(data)),
			Accent:      rowAccent,
			Now:         s.now().UnixMilli(),
		}); err != nil {
			return nil, fmt.Errorf("upsert image row: %w", err)
		}
		produced[size+"/"+string(o.format)] = true
	}
	return produced, nil
}

// accent computes the accent colour of the poster at the absolute path. A failure only costs the
// tint, so it is logged and an invalid value returned (the caller keeps a previous accent).
func (s *Service) accent(ctx context.Context, itemID, path string) sql.NullString {
	colors, err := s.runner.Palette(ctx, path, accentColors)
	if err != nil {
		s.log.Warn("poster palette failed", "item_id", itemID, "err", err)
		return sql.NullString{}
	}
	value, ok := transcode.AccentColor(colors)
	return sql.NullString{String: value, Valid: ok}
}

// removeFile deletes a replaced image file; a missing file is fine.
func (s *Service) removeFile(root *os.Root, itemID, kind, rel string) {
	if err := root.Remove(rel); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("remove replaced image", "item_id", itemID, "kind", kind, "err", err)
	}
}

// maxAccentRetries bounds how often QueueIncompleteImages queues a complete poster only because
// its rows carry no accent (the palette failed, or the poster has no saturated colour at all). The
// count lives in memory per title, so every server start allows that many retries again; with the
// scan at start and after library changes that is a handful of palette runs per title and run.
const maxAccentRetries = 3

// QueueIncompleteImages enqueues image.fetch (key image:<itemId>) for every matched title of the
// library whose images are incomplete: a stored kind lacks one of its derived sizes or formats
// (for example the WebP files next to the JPEG rows kept from M1a), still has a size the pipeline
// no longer produces (M1a's w500 poster), the file of a derived row is gone from the cache, or the
// poster rows carry no accent (at most maxAccentRetries times). Locked kinds count as well when a
// stored file can be scaled (image.fetch scales them without TMDB). Titles without any image row
// are not checked (TMDB has none, or their first image.fetch is pending), so a title without
// artwork costs no TMDB request per scan. The library scan calls it after every successful run
// (the server schedules a scan of every library at start, so titles from M1a are queued right
// after the upgrade); it returns the number of titles queued.
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
	exists := rootExists(root)

	type title struct {
		locked map[string]bool
		kinds  map[string][]storedImage // stored kinds
		accent bool                     // a derived poster row carries an accent
	}
	var order []string
	titles := map[string]*title{}
	for _, r := range rows {
		t := titles[r.ItemID]
		if t == nil {
			t = &title{locked: lockedFields(r.LockedFields), kinds: map[string][]storedImage{}}
			titles[r.ItemID] = t
			order = append(order, r.ItemID)
		}
		t.kinds[r.Kind] = append(t.kinds[r.Kind], storedImage{size: r.Size, format: r.Format, sourceURL: r.SourceUrl, relPath: r.RelPath})
		if r.Kind == "poster" && r.Size != originalSize && r.Accent.Valid {
			t.accent = true
		}
	}
	queued := 0
	for _, id := range order {
		t := titles[id]
		complete := true
		for kind, stored := range t.kinds {
			if kindComplete(kind, stored, "", exists) {
				continue
			}
			if t.locked[kind] && !slices.ContainsFunc(stored, func(si storedImage) bool { return exists(si.relPath) }) {
				continue // nothing left to scale from
			}
			complete = false
		}
		if complete && t.kinds["poster"] != nil && !t.accent && s.accentRetry(id) {
			complete = false
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

// accentRetry reports whether a poster without accent may be queued once more and counts it.
func (s *Service) accentRetry(itemID string) bool {
	s.accentMu.Lock()
	defer s.accentMu.Unlock()
	if s.accentRetries[itemID] >= maxAccentRetries {
		return false
	}
	s.accentRetries[itemID]++
	return true
}

// download reads the whole original (TMDB originals are a few MiB at most) and determines its
// media type, sniffing the content when the answer has no usable Content-Type.
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

// tempName returns a fresh temporary rel_path in dir; readers never see a partial image because
// files are renamed into place.
func tempName(dir string) (string, error) {
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", fmt.Errorf("temp name: %w", err)
	}
	return dir + "/.tmp-" + hex.EncodeToString(rnd[:]), nil
}

// writeFile writes data to a temporary file in dir and renames it to rel, so readers never see a
// partial image.
func writeFile(root *os.Root, dir, rel string, data []byte) error {
	tmp, err := tempName(dir)
	if err != nil {
		return err
	}
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
