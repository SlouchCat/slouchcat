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
	_ "image/jpeg" // registers JPEG for image.DecodeConfig (TMDB originals, derived JPEGs)
	_ "image/png"  // registers PNG for image.DecodeConfig
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
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

// imageWidths lists the delivered widths per kind (logos: ADR 0018, concept 10 6.3).
var imageWidths = map[string][]int{
	"poster":   {200, 400, 800},
	"backdrop": {780, 1280, 1920},
	"logo":     {400, 800},
}

// The backdrop size for 4K and high-density screens (contract 1.6.0): made only from an original of
// at least uhdMinOriginal pixels and never scaled up, so an original between 2560 and 3839 px keeps
// its width under the name w3840 (the row and ImageRef carry the real width).
const (
	uhdSize        = "w3840"
	uhdWidth       = 3840
	uhdMinOriginal = 2560
)

// uhdKinds are the kinds with the extra size.
var uhdKinds = map[string]bool{"backdrop": true}

// sizeSpec is one derived size: its name (w780 …) and the width it is scaled to.
type sizeSpec struct {
	name  string
	width int
}

// sizesFor returns the derived sizes of a kind for an original of origWidth pixels (0 = unknown,
// then without the 4K size).
func sizesFor(kind string, origWidth int) []sizeSpec {
	out := make([]sizeSpec, 0, len(imageWidths[kind])+1)
	for _, w := range imageWidths[kind] {
		out = append(out, sizeSpec{name: sizeName(w), width: w})
	}
	if uhdKinds[kind] && origWidth >= uhdMinOriginal {
		out = append(out, sizeSpec{name: uhdSize, width: min(origWidth, uhdWidth)})
	}
	return out
}

// imageFormat is a delivered format with its file extension and media type.
type imageFormat struct {
	format    transcode.ImageFormat
	ext       string
	mediaType string
}

// imageFormats lists the delivered formats of posters and backdrops. JPEG comes first: its header
// gives the dimensions of both formats of a width.
var imageFormats = []imageFormat{
	{transcode.ImageJPEG, "jpg", "image/jpeg"},
	{transcode.ImageWebP, "webp", "image/webp"},
}

// logoFormats are the formats of a logo: PNG (transparency) first for the dimensions, then WebP;
// never JPEG (6.3).
var logoFormats = []imageFormat{
	{transcode.ImagePNG, "png", "image/png"},
	{transcode.ImageWebP, "webp", "image/webp"},
}

// formatsOf returns the delivered formats of a kind.
func formatsOf(kind string) []imageFormat {
	if kind == "logo" {
		return logoFormats
	}
	return imageFormats
}

// FallbackFormat is the format a client without WebP gets for a kind: jpeg, png for a logo.
func FallbackFormat(kind string) string {
	if kind == "logo" {
		return "png"
	}
	return "jpeg"
}

// errImageSkipped marks a source the pipeline cannot use (an SVG logo without PNG twin, an
// answer that is no image): the kind is left as it is and the job goes on.
var errImageSkipped = errors.New("image source skipped")

// imageExt lists the accepted media types of originals and their file extensions.
var imageExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// sizeName is the size column of a derived width ("w400").
func sizeName(width int) string { return "w" + strconv.Itoa(width) }

func (s *Service) handleImageFetch(ctx context.Context, j *jobs.Job) error {
	itemID, err := parsePayload(j)
	if err != nil {
		return err
	}
	return s.fetchImages(ctx, itemID)
}

// itemImageKinds lists the image kinds per item kind: posters for movies, series and seasons,
// backdrops for movies and series and the still of an episode; the (square) cover of audiobooks,
// podcasts and podcast episodes is their poster (M6).
var itemImageKinds = map[string][]string{
	"movie":           {"poster", "backdrop", "logo"},
	"series":          {"poster", "backdrop", "logo"},
	"season":          {"poster"},
	"episode":         {"backdrop"},
	"audiobook":       {"poster"},
	"podcast":         {"poster"},
	"podcast_episode": {"poster"},
}

// audioKinds are the item kinds whose poster is a cover (coverSource) instead of a provider image.
var audioKinds = map[string]bool{"audiobook": true, "podcast": true, "podcast_episode": true}

// fetchImages runs the image pipeline for a matched item. It reads the image paths the match
// stored on the item (no provider request) and, per kind, skips the work when every derived row
// stems from the current original and its file exists (so a run after a deleted file or a new
// size only fills the gap). The original is downloaded only when its row is missing, stale or its
// file is gone. A kind without a path keeps its rows. A kind locked by the admin is never
// fetched, but its missing sizes are scaled from the stored image (scaleLocked).
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
	kinds := itemImageKinds[item.Kind]
	if item.DeletedAt.Valid || item.MetadataState != stateMatched || len(kinds) == 0 {
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
	chosen, err := s.read.ListItemImagesByItem(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("image.fetch: list image choice of %s: %w", item.ID, err)
	}
	decided := map[string]systemq.ItemImage{}
	for _, c := range chosen {
		decided[c.Kind] = c
	}
	settings, err := s.Effective(ctx, item.LibraryID)
	if err != nil {
		return fmt.Errorf("image.fetch: %w", err)
	}
	wanted := imageKindsOf(item.Kind, settings)
	locked := lockedFields(item.LockedFields)
	for _, kind := range kinds {
		if !audioKinds[item.Kind] && !slices.Contains(wanted, kind) {
			// The kind is switched off (metadata.image_kinds): its files go, unless the
			// administrator locked the kind (6.3, 8.1).
			if !locked[kind] {
				s.dropKind(ctx, root, item.ID, kind, byKind[kind])
			}
			continue
		}
		row, hasRow := decided[kind]
		if locked[kind] && (!hasRow || row.Chosen != 1) {
			// Locked without a choice (an upload, the old lock): keep the image, scale the sizes.
			if err := s.scaleLocked(ctx, root, item.ID, kind, byKind[kind]); err != nil {
				return fmt.Errorf("image.fetch %s %s: %w", item.ID, kind, err)
			}
			continue
		}
		src := imageRequest{provider: ImageSourceCover}
		if audioKinds[item.Kind] {
			if src.url, err = s.coverSource(ctx, item); err != nil {
				return fmt.Errorf("image.fetch %s cover: %w", item.ID, err)
			}
		} else if hasRow {
			src = s.imageSource(item, row)
		}
		if src.url == "" {
			continue
		}
		err := s.processKind(ctx, root, item.ID, kind, src, byKind[kind])
		if errors.Is(err, errImageSkipped) {
			s.log.Warn("image source skipped", "item_id", item.ID, "kind", kind, "provider", src.provider, "err", err)
			continue
		}
		if err != nil {
			return fmt.Errorf("image.fetch %s %s: %w", item.ID, kind, err)
		}
	}
	return nil
}

// imageRequest is the original to fetch for one kind: its address, the source it came from and
// the language of the image (both copied onto every row, 4.4.1).
type imageRequest struct {
	url      string
	provider string
	lang     string
}

// dropKind removes the rows and files of a kind the administrator switched off.
func (s *Service) dropKind(ctx context.Context, root *os.Root, itemID, kind string, prev []systemq.Image) {
	for _, r := range prev {
		if err := s.write.DeleteImage(ctx, r.ID); err != nil {
			s.log.Warn("delete image row of a switched-off kind", "item_id", itemID, "kind", kind, "err", err)
			continue
		}
		s.removeFile(root, itemID, kind, r.RelPath)
	}
}

// imageSource turns the decided row of a kind (item_images, ADR 0018) into the original to
// fetch: a TMDB path (/abc.jpg) becomes the TMDB original URL (an .svg logo is asked for as
// .png, the twin TMDB serves for every logo, O-3), a TVDB artwork, AniList or Fanart.tv URL is
// taken as it is, a file:<rel>?… source is read through package paths (local files). An address
// of an unknown host is ignored and logged. url "" means there is nothing to fetch.
func (s *Service) imageSource(item systemq.Item, row systemq.ItemImage) imageRequest {
	out := imageRequest{provider: row.Provider, lang: row.Lang}
	src := row.Source
	switch {
	case strings.HasPrefix(src, "/"):
		if strings.HasSuffix(strings.ToLower(src), ".svg") {
			src = src[:len(src)-4] + ".png"
		}
		out.url = s.client.imageURL(originalSize, src)
	case strings.HasPrefix(src, coverFilePrefix):
		out.url = src
	case strings.HasPrefix(src, s.tvdb.artBase+"/"), s.anilist.imageURL(src), s.fanart.assetURL(src):
		out.url = src
	default:
		s.log.Warn("image path of an unknown host ignored", "item_id", item.ID, "kind", row.Kind)
	}
	return out
}

// storedImage is the part of an image row the completeness check needs.
type storedImage struct {
	size, format, sourceURL, relPath string
	width                            int
}

// kindComplete reports whether the rows of one kind hold every derived width in both formats with
// an existing file and nothing else apart from the original. A non-empty sourceURL must match the
// source_url of every derived row (image.fetch knows the current TMDB original; the check after a
// scan does not).
func kindComplete(kind string, rows []storedImage, sourceURL string, exists func(rel string) bool) bool {
	origWidth := 0
	for _, r := range rows {
		if r.size == originalSize {
			origWidth = r.width
		}
	}
	want := map[string]bool{}
	for _, sz := range sizesFor(kind, origWidth) {
		for _, f := range formatsOf(kind) {
			want[sz.name+"/"+string(f.format)] = true
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

// processKind brings the rows and files of one kind up to date with the original of src.
func (s *Service) processKind(ctx context.Context, root *os.Root, itemID, kind string, src imageRequest, prev []systemq.Image) error {
	exists := rootExists(root)
	var orig *systemq.Image
	for i, r := range prev {
		if r.Size == originalSize {
			orig = &prev[i]
		}
	}
	if kindComplete(kind, storedImages(prev), src.url, exists) {
		return s.fillAccent(ctx, itemID, kind, prev)
	}

	dir := imagesDir + "/" + itemID
	if err := root.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}
	origRel := ""
	if orig != nil && orig.SourceUrl == src.url && exists(orig.RelPath) {
		origRel = orig.RelPath
	} else {
		rel, err := s.storeOriginal(ctx, root, dir, itemID, kind, src, orig)
		if err != nil {
			return err
		}
		origRel = rel
	}
	return s.deriveAndPrune(ctx, root, dir, itemID, kind, origRel, src, prev)
}

// deriveAndPrune derives every size of kind from the original at origRel and removes the rows
// (and files) of sizes the pipeline no longer produces (M1a's w500 poster). A palette failure
// keeps the accent the poster rows had.
func (s *Service) deriveAndPrune(ctx context.Context, root *os.Root, dir, itemID, kind, origRel string, src imageRequest, prev []systemq.Image) error {
	sc, err := s.scaleDerived(ctx, root, dir, itemID, kind, origRel, storedAccent(prev))
	if err != nil {
		return err
	}
	defer s.dropScaled(root, itemID, kind, sc)
	produced, err := s.storeDerived(ctx, root, dir, itemID, kind, src, sc)
	if err != nil {
		return err
	}
	return s.pruneSizes(ctx, root, itemID, kind, prev, produced)
}

// pruneSizes removes the rows (and files) of kind in prev whose size and format are not among
// produced, apart from the original.
func (s *Service) pruneSizes(ctx context.Context, root *os.Root, itemID, kind string, prev []systemq.Image, produced map[string]bool) error {
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
	req := imageRequest{url: src.SourceUrl, provider: src.Provider, lang: src.Lang.String}
	if src.Size == originalSize {
		return s.deriveAndPrune(ctx, root, dir, itemID, kind, src.RelPath, req, prev)
	}
	data, err := root.ReadFile(src.RelPath)
	if err != nil {
		return fmt.Errorf("read stored %s: %w", kind, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode stored %s header: %w", kind, err)
	}
	// The original keeps the media type of the stored file (a PNG logo stays PNG, 6.3).
	ext, ok := imageExt[src.MediaType]
	if !ok {
		ext = "jpg"
	}
	rel := dir + "/" + kind + "-" + originalSize + "." + ext
	if err := writeFile(root, dir, rel, data); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if err := s.write.UpsertImage(ctx, systemq.UpsertImageParams{
		ItemID: itemID, Kind: kind, Size: originalSize, Format: originalFormat, SourceUrl: src.SourceUrl,
		Lang: src.Lang, Provider: src.Provider,
		RelPath: rel, ContentHash: hex.EncodeToString(sum[:]), MediaType: src.MediaType,
		Width: int64(cfg.Width), Height: int64(cfg.Height), Bytes: int64(len(data)), Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("upsert original row: %w", err)
	}
	return s.deriveAndPrune(ctx, root, dir, itemID, kind, rel, req, prev)
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
		if r.Format != string(transcode.ImageWebP) && (best == nil || r.Width > best.Width) {
			best = r // the widest JPEG, or PNG of a logo (6.3)
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
		out = append(out, storedImage{size: r.Size, format: r.Format, sourceURL: r.SourceUrl, relPath: r.RelPath, width: int(r.Width)})
	}
	return out
}

func rootExists(root *os.Root) func(rel string) bool {
	return func(rel string) bool {
		_, err := root.Stat(rel)
		return err == nil
	}
}

// storeOriginal downloads the original at sourceURL to <dir>/<kind>-original.<ext> (temporary name, then
// rename), upserts its row and returns its rel_path. A replaced original with another extension
// is removed.
func (s *Service) storeOriginal(ctx context.Context, root *os.Root, dir, itemID, kind string, src imageRequest, prev *systemq.Image) (string, error) {
	sourceURL := src.url
	data, mediaType, err := s.fetchOriginal(ctx, itemID, sourceURL)
	if err != nil {
		return "", err
	}
	ext, ok := imageExt[mediaType]
	if !ok {
		// An SVG logo without PNG twin, or an answer that is no image: the kind is skipped (9.4).
		return "", fmt.Errorf("%w: unsupported image media type %q", errImageSkipped, mediaType)
	}
	cfg, err := imageConfig(data, mediaType)
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
		Lang:        sql.NullString{String: src.lang, Valid: true},
		Provider:    src.provider,
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

// derived is one scaled output of scaleDerived before storeDerived renames it into place.
type derived struct {
	size      string // w780 … w3840
	width     int
	format    transcode.ImageFormat
	ext       string
	mediaType string
	tmp       string // rel_path of the temporary file
}

// scaled holds the outputs of one ffmpeg run in their temporary files, until storeDerived renames
// them into place; dropScaled removes what is left.
type scaled struct {
	outs   []derived
	accent sql.NullString // the accent of a poster; invalid for the other kinds
}

// scaleDerived scales the original at origRel into every width and format of kind in one ffmpeg
// run, to temporary files, and computes the accent colour for posters (prevAccent stays when the
// palette fails). Nothing of the kind's rows or final files changes; on an error the temporary
// files are gone already.
func (s *Service) scaleDerived(ctx context.Context, root *os.Root, dir, itemID, kind, origRel string, prevAccent sql.NullString) (*scaled, error) {
	in, err := paths.Resolve(s.cache, origRel)
	if err != nil {
		return nil, fmt.Errorf("resolve original: %w", err)
	}
	sc := &scaled{}
	ok := false
	defer func() {
		if !ok {
			s.dropScaled(root, itemID, kind, sc)
		}
	}()
	var args []transcode.ImageOutput
	for _, sz := range sizesFor(kind, s.originalWidth(root, origRel)) {
		for _, f := range formatsOf(kind) {
			tmp, err := tempName(dir)
			if err != nil {
				return nil, err
			}
			abs, err := paths.Resolve(s.cache, tmp)
			if err != nil {
				return nil, fmt.Errorf("resolve temporary image: %w", err)
			}
			sc.outs = append(sc.outs, derived{size: sz.name, width: sz.width, format: f.format, ext: f.ext, mediaType: f.mediaType, tmp: tmp})
			args = append(args, transcode.ImageOutput{Path: abs, Width: sz.width, Format: f.format})
		}
	}
	if err := s.runner.ScaleImage(ctx, in, args); err != nil {
		return nil, fmt.Errorf("scale %s: %w", origRel, err)
	}
	if kind == "poster" {
		for i, o := range sc.outs {
			if o.width == accentSourceWidth && o.format == transcode.ImageJPEG {
				sc.accent = s.accent(ctx, itemID, args[i].Path)
			}
		}
		if !sc.accent.Valid {
			sc.accent = prevAccent
		}
	}
	ok = true
	return sc, nil
}

// dropScaled removes the temporary files of sc that were not renamed into place.
func (s *Service) dropScaled(root *os.Root, itemID, kind string, sc *scaled) {
	for _, o := range sc.outs {
		if err := root.Remove(o.tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("remove temporary image", "item_id", itemID, "kind", kind, "err", err)
		}
	}
}

// storeDerived renames the scaled files of sc into place and upserts their rows with the source
// src. It returns the produced "<size>/<format>" keys.
func (s *Service) storeDerived(ctx context.Context, root *os.Root, dir, itemID, kind string, src imageRequest, sc *scaled) (map[string]bool, error) {
	sourceURL := src.url
	accent := sc.accent
	dims := map[int]image.Config{} // per width, from the first format of the kind (JPEG, or PNG for a logo)
	produced := map[string]bool{}
	first := formatsOf(kind)[0].format
	for _, o := range sc.outs {
		data, err := root.ReadFile(o.tmp)
		if err != nil {
			return nil, fmt.Errorf("read scaled image: %w", err)
		}
		if o.format == first {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("decode scaled image header: %w", err)
			}
			dims[o.width] = cfg
		}
		size := o.size
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
			Lang:        sql.NullString{String: src.lang, Valid: true},
			Provider:    src.provider,
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

// originalWidth reads the width of the original at rel from its header; 0 when it cannot be read
// (then the 4K size is left out, the other sizes do not need it).
func (s *Service) originalWidth(root *os.Root, rel string) int {
	f, err := root.Open(rel)
	if err != nil {
		return 0
	}
	defer f.Close()
	head := make([]byte, 1<<20) // a JPEG may carry large EXIF or ICC segments before its frame header
	n, _ := io.ReadFull(f, head)
	mediaType := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}[strings.ToLower(path.Ext(rel))]
	cfg, err := imageConfig(head[:n], mediaType)
	if err != nil {
		return 0
	}
	return cfg.Width
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
// stored file can be scaled (image.fetch scales them without TMDB). A title of any kind with a
// stored image path but no row of that kind yet (its first image.fetch failed) is queued as well
// (episode stills at priority 1); a title without path and rows has no artwork and costs nothing
// per scan. The library scan calls it after every successful run (the server schedules a scan of
// every library at start, so titles from M1a are queued right after the upgrade); it returns the
// number of titles queued.
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
		t.kinds[r.Kind] = append(t.kinds[r.Kind], storedImage{size: r.Size, format: r.Format, sourceURL: r.SourceUrl, relPath: r.RelPath, width: int(r.Width)})
		if r.Kind == "poster" && r.Size != originalSize && r.Accent.Valid {
			t.accent = true
		}
	}
	missing, err := s.read.ListItemsMissingImages(ctx, libraryID)
	if err != nil {
		return 0, fmt.Errorf("list titles without images: %w", err)
	}
	queued := 0
	done := map[string]bool{}
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
		if err := s.enqueueImages(ctx, id, imageFetchPriority); err != nil {
			return queued, err
		}
		done[id] = true
		queued++
	}
	for _, m := range missing {
		locked := lockedFields(m.LockedFields)
		if done[m.ID] || ((m.PosterMissing != 1 || locked["poster"]) && (m.BackdropMissing != 1 || locked["backdrop"])) {
			continue
		}
		prio := imageFetchPriority
		if m.Kind == "episode" {
			prio = stillPriority
		}
		if err := s.enqueueImages(ctx, m.ID, prio); err != nil {
			return queued, err
		}
		done[m.ID] = true
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
func download(ctx context.Context, c *Client, sourceURL string) ([]byte, string, error) {
	body, mediaType, err := c.DownloadURL(ctx, sourceURL)
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

// Covers of audiobooks, podcasts and episodes (M6 design 3.1). The source of a cover is a string
// stored as source_url, so a changed cover is fetched again:
//
//	coverFilePrefix + cover_path + "?" + size + "-" + mtime  a folder image below the library root
//	coverEmbeddedPrefix + media file id + "?" + mtime        the picture embedded in a media file
//	http(s) URL                                               cover_url (a feed image, any host)
//
// The part after the last "?" is the version; a folder name may contain "?" itself.
const (
	coverFilePrefix     = "file:"
	coverEmbeddedPrefix = "embedded:"
	// maxCoverBytes and coverTimeout bound a cover read from a file or fetched from a feed host.
	maxCoverBytes = 10 << 20
	coverTimeout  = 15 * time.Second
)

// coverClients fetch feed images: no address in the local network unless the podcast's feed is
// there itself (package netguard); coverTimeout bounds each fetch.
var coverClients = netguard.NewClients(http.Client{}, 0)

// coverSource returns the source of the cover of an audio item: the folder image or embedded
// picture of cover_path (an audiobook), else cover_url when it is an http or https URL; "" when
// there is none or the file is gone.
func (s *Service) coverSource(ctx context.Context, item systemq.Item) (string, error) {
	cover := item.CoverPath.String
	switch {
	case strings.HasPrefix(cover, coverEmbeddedPrefix):
		mf, err := s.read.GetMediaFileSystem(ctx, strings.TrimPrefix(cover, coverEmbeddedPrefix))
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		if err != nil {
			return "", fmt.Errorf("load media file of the cover: %w", err)
		}
		return coverEmbeddedPrefix + mf.ID + "?" + strconv.FormatInt(mf.Mtime, 10), nil
	case cover != "":
		root, err := s.coverRoot(ctx, item.ID)
		if err != nil || root == nil {
			return "", err
		}
		defer root.Close()
		fi, err := root.Stat(cover)
		if err != nil || !fi.Mode().IsRegular() {
			s.log.Warn("cover file not readable", "item_id", item.ID, "err", err)
			return "", nil
		}
		return coverFilePrefix + cover + "?" + strconv.FormatInt(fi.Size(), 10) + "-" + strconv.FormatInt(fi.ModTime().UnixMilli(), 10), nil
	}
	if u, err := url.Parse(item.CoverUrl.String); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return u.String(), nil
	}
	return "", nil
}

// coverRoot opens the library root of an item's first media file, nil without one.
func (s *Service) coverRoot(ctx context.Context, itemID string) (*os.Root, error) {
	dir, err := s.read.GetItemCoverRoot(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load cover root: %w", err)
	}
	pr, err := paths.NewRoot(dir)
	if err != nil {
		return nil, err
	}
	return paths.Open(pr)
}

// fetchOriginal reads the original image of a source: a folder image through package paths, an
// embedded picture through ffmpeg (transcode.Runner.ExtractCover), a cover URL of any host over
// plain HTTP, else a provider image through the TMDB client.
func (s *Service) fetchOriginal(ctx context.Context, itemID, source string) ([]byte, string, error) {
	switch {
	case strings.HasPrefix(source, coverFilePrefix):
		rel := unversioned(strings.TrimPrefix(source, coverFilePrefix))
		root, err := s.localImageRoot(ctx, itemID, rel)
		if err != nil {
			return nil, "", err
		}
		if root == nil {
			if root, err = s.coverRoot(ctx, itemID); err != nil {
				return nil, "", err
			}
		}
		if root == nil {
			return nil, "", fmt.Errorf("%w: root of the local file gone", errImageSkipped)
		}
		defer root.Close()
		f, err := root.Open(rel)
		if err != nil {
			return nil, "", fmt.Errorf("open cover: %w", err)
		}
		defer f.Close()
		return sniffCover(io.LimitReader(f, maxCoverBytes+1))
	case strings.HasPrefix(source, coverEmbeddedPrefix):
		id := unversioned(strings.TrimPrefix(source, coverEmbeddedPrefix))
		abs, err := s.mediaFilePath(ctx, id)
		if err != nil {
			return nil, "", err
		}
		data, err := s.runner.ExtractCover(ctx, abs)
		if err != nil {
			return nil, "", fmt.Errorf("extract cover: %w", err)
		}
		return sniffCover(bytes.NewReader(data))
	case s.providerImage(source):
		return download(ctx, s.client, source)
	default:
		feedURL, origin := s.feedOf(ctx, itemID)
		return s.downloadCover(ctx, feedURL, origin, source)
	}
}

// localImageRoot opens the library root of a local image file the scanner listed for the title
// (item_local_images, ADR 0018; the row names the root, so a series without media file of its own
// is served too); nil when no row names rel (then it is an audio cover, coverRoot).
func (s *Service) localImageRoot(ctx context.Context, itemID, rel string) (*os.Root, error) {
	rows, err := s.read.ListItemLocalImages(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("load local images: %w", err)
	}
	for _, r := range rows {
		if r.RelPath != rel {
			continue
		}
		dir, err := s.read.GetLibraryRootPath(ctx, r.RootID)
		if err != nil {
			return nil, fmt.Errorf("load root of a local image: %w", err)
		}
		pr, err := paths.NewRoot(dir)
		if err != nil {
			return nil, err
		}
		return paths.Open(pr)
	}
	return nil, nil
}

// unversioned drops the version after the last "?" of a cover source.
func unversioned(source string) string {
	if i := strings.LastIndexByte(source, '?'); i >= 0 {
		return source[:i]
	}
	return source
}

// feedOf returns the feed URL of the podcast of an item (a podcast's own, an episode's parent's);
// "" for any other item.
func (s *Service) feedOf(ctx context.Context, itemID string) (feedURL, origin string) {
	item, err := s.read.GetItemSystem(ctx, itemID)
	if err != nil {
		return "", ""
	}
	if item.Kind == "podcast_episode" && item.ParentID.Valid {
		if p, err := s.read.PodcastGet(ctx, item.ParentID.String); err == nil {
			return p.FeedUrl.String, p.FeedOrigin.String
		}
		return "", ""
	}
	return item.FeedUrl.String, item.FeedOrigin.String
}

// providerImage reports whether a source is a TMDB, TVDB or AniList image (fetched through the
// provider client, not the guarded cover client: the hosts are constants).
func (s *Service) providerImage(source string) bool {
	return strings.HasPrefix(source, s.client.imageURL(originalSize, "")) || strings.HasPrefix(source, s.tvdb.artBase+"/") ||
		s.anilist.imageURL(source) || s.fanart.assetURL(source)
}

// mediaFilePath resolves the absolute path of a media file through package paths.
func (s *Service) mediaFilePath(ctx context.Context, id string) (string, error) {
	mf, err := s.read.GetMediaFileSystem(ctx, id)
	if err != nil {
		return "", fmt.Errorf("load media file of the cover: %w", err)
	}
	root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, mf.RootID)
	if err != nil {
		return "", fmt.Errorf("load library root of the cover: %w", err)
	}
	pr, err := paths.NewRoot(root.Path)
	if err != nil {
		return "", err
	}
	return paths.Resolve(pr, mf.RelPath)
}

// downloadCover fetches a cover URL of the feed at feedURL (a feed image, any http or https host
// outside the local network unless the feed is local, package netguard): at most maxCoverBytes
// within coverTimeout, JPEG, PNG or WebP only (by content: feed hosts often send image/jpg or no
// type at all). No credentials or provider keys are sent; errors name the host only.
func (s *Service) downloadCover(ctx context.Context, feedURL, feedOrigin, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, "", fmt.Errorf("cover url not usable")
	}
	ctx, cancel := context.WithTimeout(ctx, coverTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("cover request: %w", netguard.Clean(err))
	}
	req.Header.Set("Accept", "image/jpeg, image/png, image/webp")
	resp, err := coverClients.For(ctx, feedURL, feedOrigin).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch cover from %s: %w", u.Host, netguard.Clean(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch cover from %s: status %d", u.Host, resp.StatusCode)
	}
	data, mediaType, err := sniffCover(io.LimitReader(resp.Body, maxCoverBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("cover from %s: %w", u.Host, err)
	}
	return data, mediaType, nil
}

// sniffCover reads a cover of at most maxCoverBytes and determines its media type from the content;
// anything but JPEG, PNG and WebP is refused.
func sniffCover(r io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("read cover: %w", err)
	}
	if len(data) > maxCoverBytes {
		return nil, "", fmt.Errorf("cover larger than %d bytes", maxCoverBytes)
	}
	mediaType := http.DetectContentType(data)
	if _, ok := imageExt[mediaType]; !ok {
		return nil, "", fmt.Errorf("unsupported cover media type %q", mediaType)
	}
	return data, mediaType, nil
}

// imageConfig returns the dimensions of an original: JPEG and PNG through package image, WebP
// from its header (the standard library has no WebP decoder).
func imageConfig(data []byte, mediaType string) (image.Config, error) {
	if mediaType != "image/webp" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		return cfg, err
	}
	if len(data) < 30 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return image.Config{}, errors.New("not a webp image")
	}
	le16 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 }
	le24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
	switch string(data[12:16]) {
	case "VP8 ":
		return image.Config{Width: le16(data[26:]) & 0x3fff, Height: le16(data[28:]) & 0x3fff}, nil
	case "VP8L":
		b := uint32(data[21]) | uint32(data[22])<<8 | uint32(data[23])<<16 | uint32(data[24])<<24
		return image.Config{Width: int(b&0x3fff) + 1, Height: int((b>>14)&0x3fff) + 1}, nil
	case "VP8X":
		return image.Config{Width: le24(data[24:]) + 1, Height: le24(data[27:]) + 1}, nil
	}
	return image.Config{}, errors.New("unknown webp chunk")
}
