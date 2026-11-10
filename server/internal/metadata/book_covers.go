package metadata

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The cover choice of an audiobook (concept 19 D5.3, contract 1.24.0). A book has no provider; its
// candidates are every image in its folder (provider local, source file:<rel>?<size>-<mtime> like
// coverSource) and the picture embedded in the first part that has one (provider embedded, source
// embedded:<fileId>?<mtime>), plus an upload. The folder counts only while no other title's media
// file lies beside the parts (a shared folder of .m4b files holds the covers of other books). A
// chosen candidate is an item_images row with chosen = 1, which image.fetch fetches instead of the
// cover the scan found (coverSource), so a scan keeps it.

// ImageSourceEmbedded is the provider of the picture embedded in a media file.
const ImageSourceEmbedded = "embedded"

// maxCoverHeader bounds the bytes read for the dimensions of a folder image.
const maxCoverHeader = 256 << 10

// bookImageExts are the extensions of a folder image (the formats of imageExt).
var bookImageExts = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}

// bookImage is an image in the folder of a book.
type bookImage struct {
	rel           string
	size, mtime   int64
	width, height int
}

// source is the image's cover source (the form coverSource gives a folder cover).
func (b bookImage) source() string {
	return coverFilePrefix + b.rel + "?" + strconv.FormatInt(b.size, 10) + "-" + strconv.FormatInt(b.mtime, 10)
}

// bookCoverCandidates lists the cover candidates of a book: the folder images (the cover the scan
// chose first, then by name), then the embedded picture.
func (s *Service) bookCoverCandidates(ctx context.Context, item systemq.Item) (model.ImageCandidateList, error) {
	out := model.ImageCandidateList{Candidates: []model.ImageCandidate{}, Unavailable: []string{}}
	parts, err := s.read.ListAudiobookParts(ctx, item.ID)
	if err != nil {
		return out, fmt.Errorf("cover candidates of %s: parts: %w", item.ID, err)
	}
	images, root, err := s.bookFolderImages(ctx, item, parts, true)
	if err != nil {
		return out, err
	}
	if root != nil {
		_ = root.Close()
	}
	for _, img := range images {
		out.Candidates = append(out.Candidates, model.ImageCandidate{Provider: ImageSourceLocal, Width: img.width, Height: img.height,
			Source: img.source()})
	}
	if mf, w, h, ok := embeddedCoverOf(parts); ok {
		out.Candidates = append(out.Candidates, model.ImageCandidate{Provider: ImageSourceEmbedded, Width: w, Height: h,
			Source: coverEmbeddedPrefix + mf.ID + "?" + strconv.FormatInt(mf.Mtime, 10)})
	}
	// The cover of the book source the book is matched to (concept 19 D6.3; its size is unknown).
	if r, ok, err := s.bookProviderCover(ctx, item.ID); err != nil {
		return out, err
	} else if ok {
		out.Candidates = append(out.Candidates, model.ImageCandidate{Provider: r.Provider, Source: r.Source, PreviewURL: r.Source})
	}
	return out, nil
}

// bookFolderImages reads the images in the folder of a book's parts through its library root (rule
// 4) and returns them with the open root (the caller closes it); dims reads the dimensions from the
// head of each file. None for a book directly in the root, or for an .m4b book in a folder with
// another title's file (the scan's rule, library.bookFiles: parts of other formats own their folder).
func (s *Service) bookFolderImages(ctx context.Context, item systemq.Item, parts []systemq.MediaFile, dims bool) ([]bookImage, *os.Root, error) {
	if len(parts) == 0 {
		return nil, nil, nil
	}
	dir := path.Dir(parts[0].RelPath)
	if dir == "." {
		return nil, nil, nil
	}
	if strings.EqualFold(path.Ext(parts[0].RelPath), ".m4b") {
		foreign, err := s.read.CountForeignMediaFilesInDir(ctx, systemq.CountForeignMediaFilesInDirParams{
			RootID: parts[0].RootID, ItemID: item.ID, Dir: dir})
		if err != nil {
			return nil, nil, fmt.Errorf("cover candidates of %s: folder: %w", item.ID, err)
		}
		if foreign > 0 {
			return nil, nil, nil
		}
	}
	rootPath, err := s.read.GetLibraryRootPath(ctx, parts[0].RootID)
	if err != nil {
		return nil, nil, fmt.Errorf("cover candidates of %s: root: %w", item.ID, err)
	}
	pr, err := paths.NewRoot(rootPath)
	if err != nil {
		return nil, nil, err
	}
	root, err := paths.Open(pr)
	if err != nil {
		return nil, nil, err
	}
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		s.log.Warn("cover candidates: folder not readable", "item_id", item.ID, "err", err)
		return nil, root, nil
	}
	var out []bookImage
	for _, e := range entries {
		mediaType := bookImageExts[strings.ToLower(path.Ext(e.Name()))]
		if mediaType == "" || strings.HasPrefix(e.Name(), ".") || !e.Type().IsRegular() {
			continue
		}
		rel := path.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil || fi.Size() > maxLocalPreviewBytes {
			continue
		}
		img := bookImage{rel: rel, size: fi.Size(), mtime: fi.ModTime().UnixMilli()}
		if dims {
			if f, err := root.Open(rel); err == nil {
				head, _ := io.ReadAll(io.LimitReader(f, maxCoverHeader))
				_ = f.Close()
				if cfg, err := imageConfig(head, mediaType); err == nil {
					img.width, img.height = cfg.Width, cfg.Height
				}
			}
		}
		out = append(out, img)
	}
	cover := item.CoverPath.String
	slices.SortFunc(out, func(a, b bookImage) int {
		if (a.rel == cover) != (b.rel == cover) {
			if a.rel == cover {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.rel), strings.ToLower(b.rel))
	})
	return out, root, nil
}

// embeddedCoverOf returns the first part with an attached picture and the picture's size.
func embeddedCoverOf(parts []systemq.MediaFile) (systemq.MediaFile, int, int, bool) {
	for _, p := range parts {
		if !p.Probe.Valid {
			continue
		}
		var pr transcode.Probe
		if json.Unmarshal([]byte(p.Probe.String), &pr) != nil {
			continue
		}
		for _, st := range pr.Streams {
			if st.AttachedPic {
				return p, st.Width, st.Height, true
			}
		}
	}
	return systemq.MediaFile{}, 0, 0, false
}

// bookCoverPreview serves a cover candidate of a book for the preview handler: a folder image or
// the embedded picture the candidates name now, matched without the version (a chosen cover whose
// file changed since still shows); anything else is model.ErrNotFound.
func (s *Service) bookCoverPreview(ctx context.Context, item systemq.Item, source string) (model.StreamSource, error) {
	parts, err := s.read.ListAudiobookParts(ctx, item.ID)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("cover preview of %s: %w", item.ID, err)
	}
	want := unversioned(source)
	switch {
	case strings.HasPrefix(source, coverEmbeddedPrefix):
		mf, _, _, ok := embeddedCoverOf(parts)
		if !ok || coverEmbeddedPrefix+mf.ID != want {
			return model.StreamSource{}, model.ErrNotFound
		}
		abs, err := s.mediaFilePath(ctx, mf.ID)
		if err != nil {
			return model.StreamSource{}, err
		}
		data, err := s.runner.ExtractCover(ctx, abs)
		if err != nil {
			s.log.Warn("cover preview: embedded picture not readable", "item_id", item.ID, "err", err)
			return model.StreamSource{}, model.ErrNotFound
		}
		mediaType := http.DetectContentType(data)
		if _, ok := imageExt[mediaType]; !ok {
			return model.StreamSource{}, model.ErrNotFound
		}
		return model.StreamSource{File: nopSeekCloser{bytes.NewReader(data)}, MediaType: mediaType, Size: int64(len(data)),
			ModTime: time.UnixMilli(mf.Mtime), Name: "embedded cover preview"}, nil
	case strings.HasPrefix(source, coverFilePrefix):
		images, root, err := s.bookFolderImages(ctx, item, parts, false)
		if err != nil {
			return model.StreamSource{}, err
		}
		if root == nil {
			return model.StreamSource{}, model.ErrNotFound
		}
		i := slices.IndexFunc(images, func(b bookImage) bool { return coverFilePrefix+b.rel == want })
		if i < 0 {
			_ = root.Close()
			return model.StreamSource{}, model.ErrNotFound
		}
		f, err := root.Open(images[i].rel)
		_ = root.Close() // an open file outlives its root
		if errors.Is(err, fs.ErrNotExist) {
			return model.StreamSource{}, model.ErrNotFound
		}
		if err != nil {
			return model.StreamSource{}, fmt.Errorf("cover preview of %s: %w", item.ID, err)
		}
		fi, err := f.Stat()
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxLocalPreviewBytes {
			_ = f.Close()
			return model.StreamSource{}, model.ErrNotFound
		}
		return model.StreamSource{File: f, MediaType: bookImageExts[strings.ToLower(path.Ext(images[i].rel))], Size: fi.Size(),
			ModTime: fi.ModTime(), Name: "folder cover preview"}, nil
	}
	return model.StreamSource{}, model.ErrNotFound
}

// nopSeekCloser gives a bytes.Reader the Close of a file.
type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

// bookCurrentCover is the cover image.fetch takes for a book without choice as an image of the title
// page: the cover of its files (coverSource, provider local or embedded), else the cover of its
// book source; ok false without one.
func (s *Service) bookCurrentCover(ctx context.Context, item systemq.Item) (model.AdminItemImage, bool, error) {
	src, err := s.coverSource(ctx, item)
	if err != nil {
		return model.AdminItemImage{}, false, err
	}
	if src == "" {
		// Without a cover of its own the book shows the cover of its book source (image.fetch).
		r, ok, err := s.bookProviderCover(ctx, item.ID)
		if err != nil || !ok {
			return model.AdminItemImage{}, false, err
		}
		return model.AdminItemImage{Kind: "poster", Provider: r.Provider, Source: r.Source, PreviewURL: r.Source}, true, nil
	}
	provider := ImageSourceLocal
	switch {
	case strings.HasPrefix(src, coverEmbeddedPrefix):
		provider = ImageSourceEmbedded
	case !strings.HasPrefix(src, coverFilePrefix):
		return model.AdminItemImage{}, false, nil
	}
	return model.AdminItemImage{Kind: "poster", Provider: provider, Source: src}, true, nil
}

// isBookCover reports whether an item_images row names the cover of a book source: one of the
// five sources with an http or https address.
func isBookCover(provider, source string) bool {
	if !slices.Contains(BookProviderNames, provider) {
		return false
	}
	u, err := url.Parse(source)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}

// bookCoverOrigin is the address on whose behalf the cover of a book source is fetched (feedOf):
// the base address of the source of the book's poster row, so the guarded cover client allows the
// local network only for a source that is itself there (a mock, a self-hosted Audnexus).
func (s *Service) bookCoverOrigin(ctx context.Context, itemID string) (feedURL, origin string) {
	rows, err := s.read.ListItemImagesByItem(ctx, itemID)
	if err != nil {
		return "", ""
	}
	for _, r := range rows {
		if r.Kind != "poster" || !isBookCover(r.Provider, r.Source) {
			continue
		}
		switch r.Provider {
		case ProviderAudible:
			a := s.books.audible
			if netguard.Site(r.Source) == netguard.Site(a.audnexus(ctx)) {
				return a.audnexus(ctx), ""
			}
			return a.base(a.region(ctx)), ""
		case ProviderDNB:
			return s.books.dnb.base, ""
		case ProviderAppleBooks:
			return s.books.apple.base, ""
		case ProviderOpenLibrary:
			return s.books.openLibrary.covers, ""
		case ProviderGoogleBooks:
			return s.books.google.base, ""
		}
	}
	return "", ""
}

// bookProviderCover is the poster row of a book that names a source's cover and was not chosen
// (the candidate a match stored); ok is false without one.
func (s *Service) bookProviderCover(ctx context.Context, itemID string) (systemq.ItemImage, bool, error) {
	rows, err := s.read.ListItemImagesByItem(ctx, itemID)
	if err != nil {
		return systemq.ItemImage{}, false, fmt.Errorf("images of %s: %w", itemID, err)
	}
	for _, r := range rows {
		if r.Kind == "poster" && isBookCover(r.Provider, r.Source) {
			return r, true, nil
		}
	}
	return systemq.ItemImage{}, false, nil
}
