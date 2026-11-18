package metadata

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/store/externalq"
)

// The bound of the image cache of titles outside the library (concept 25, security finding of
// stages 2 and 5): any profile with the request right can name TMDB ids, and every title it opens
// stores its images. The cache keeps at most ExternalImageCacheBytes of them. After every new
// fetch the titles least recently used (external_titles.used_at, the day a page last read the
// title, then the oldest fetch) lose their stored images until the total fits again; the title
// just fetched is never dropped. A dropped title keeps its row: its next page offers the nominal
// sizes again and the next image request fetches anew. The genre backdrops of «Entdecken» are a
// handful of files and are bounded by number (maxGenreImages, the oldest go first).
const (
	// ExternalImageCacheBytes is 2 GiB: about a thousand titles with poster, backdrop and logo in
	// every size and both formats (a view of «Entdecken» shows up to 140 titles).
	ExternalImageCacheBytes int64 = 2 << 30
	// externalEvictBatch is how many titles one pass looks at.
	externalEvictBatch = 200
	// maxGenreImages bounds the genre backdrops (TMDB has about 35 genres of films and series).
	maxGenreImages = 200
)

// boundExternalImages drops the images of the least recently used titles while the cache holds
// more than its cap; media and tmdbID name the title just fetched, which stays.
func (s *Service) boundExternalImages(ctx context.Context, media string, tmdbID int64) error {
	if s.externalImageCap <= 0 {
		return nil
	}
	q := externalq.New(s.db.Write)
	total, err := q.SumExternalTitleImageBytes(ctx)
	if err != nil {
		return fmt.Errorf("size of external title images: %w", err)
	}
	if total <= s.externalImageCap {
		return nil
	}
	titles, err := q.ListExternalImageTitlesByUse(ctx, externalEvictBatch)
	if err != nil {
		return fmt.Errorf("external title images by use: %w", err)
	}
	var files []string
	for _, t := range titles {
		if total <= s.externalImageCap {
			break
		}
		if t.Media == media && t.TmdbID == tmdbID {
			continue
		}
		rels, err := q.DeleteExternalTitleImageRows(ctx, externalq.DeleteExternalTitleImageRowsParams{Media: t.Media, TmdbID: t.TmdbID})
		if err != nil {
			return fmt.Errorf("drop images of %s: %w", externalKey(t.Media, t.TmdbID), err)
		}
		files = append(files, rels...)
		total -= t.Bytes
	}
	if len(files) > 0 {
		s.log.InfoContext(ctx, "external title image cache bounded", "files", len(files), "bytes", total)
	}
	s.removeExternalFiles(files)
	return nil
}

// boundGenreImages removes the oldest genre backdrops beyond maxGenreImages; keep was just written.
func (s *Service) boundGenreImages(root *os.Root, keep string) {
	dir, err := root.Open(genreImagesDir)
	if err != nil {
		return
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil || len(entries) <= maxGenreImages {
		return
	}
	type aged struct {
		rel string
		at  int64
	}
	var files []aged
	for _, e := range entries {
		rel := genreImagesDir + "/" + e.Name()
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || rel == keep {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, aged{rel: rel, at: info.ModTime().UnixNano()})
	}
	slices.SortFunc(files, func(a, b aged) int { return cmp.Compare(a.at, b.at) })
	var drop []string
	for i := 0; i < len(files) && len(files)-i >= maxGenreImages; i++ {
		drop = append(drop, files[i].rel)
	}
	s.removeExternalFiles(drop)
}
