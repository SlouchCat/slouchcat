package playback

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strconv"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Chapter images (concept 13 stage 3): the job chapters.images of package library writes them below
// the cache directory per probe of a media file; playback names the ones that exist and serves them
// (getChapterImage). Nothing here runs ffmpeg.

// withChapterImages sets Chapter.ImageURL on the chapters of a film or an episode whose image
// exists for the file's current probe. One directory read, no ffmpeg in the request path; without
// a cache directory, a probe or the folder the chapters stay as they are.
func (s *Service) withChapterImages(ctx context.Context, item model.ItemSummary, loc visibility.MediaFileLocation, chapters []model.Chapter) {
	if s.cacheDir == "" || loc.ProbedAt == 0 || len(chapters) == 0 || (item.Kind != "movie" && item.Kind != "episode") {
		return
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return
	}
	names, err := paths.ReadDirNames(cache, library.ChapterImagesRel(loc.File.ID, loc.ProbedAt), 0)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.log.WarnContext(ctx, "read chapter images", "media_file_id", loc.File.ID, "err", err)
		}
		return
	}
	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}
	for i := range chapters {
		if have[library.ChapterImageName(i)] {
			chapters[i].ImageURL = ChapterImageURL(item.ID, loc.File.ID, i, loc.ProbedAt)
		}
	}
}

// ChapterImageURL is the relative URL of getChapterImage; v (the probe time) only busts caches, so
// the immutable image of an older probe is never shown for a new one.
func ChapterImageURL(itemID, mediaFileID string, idx int, probedAt int64) string {
	return "/api/v1/items/" + url.PathEscape(itemID) + "/chapters/" + strconv.Itoa(idx) + "/image?mediaFileId=" +
		url.QueryEscape(mediaFileID) + "&v=" + strconv.FormatInt(probedAt, 10)
}

// ChapterImageETag is the strong validator of a chapter image: the probe time of its file, the v
// of ChapterImageURL in quotes. The image of one probe never changes.
func ChapterImageETag(probedAt int64) string {
	return strconv.Quote(strconv.FormatInt(probedAt, 10))
}

// maxChapterIndex bounds the chapter index of getChapterImage (a film with replacement chapters
// every 10 minutes has a few dozen).
const maxChapterIndex = 9999

// ChapterImage opens the image of chapter idx of the media file mediaFileID of a visible title
// (getChapterImage): mediaFileID must be a ULID of a file of that title, idx a chapter whose image
// exists for the file's current probe. Anything else, also an invisible title, a file of another
// title and a title above the profile's age rating, is model.ErrNotFound (never 403). The file is
// opened through package paths below the cache directory (CLAUDE.md rule 4); its ETag is
// ChapterImageETag of the current probe.
func (s *Service) ChapterImage(ctx context.Context, profile model.Profile, itemID, mediaFileID string, idx int) (model.StreamSource, error) {
	if s.cacheDir == "" || idx < 0 || idx > maxChapterIndex {
		return model.StreamSource{}, model.ErrNotFound
	}
	if _, err := ulid.ParseStrict(mediaFileID); err != nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	loc, err := s.vis.MediaFile(ctx, visibility.Profile(profile), itemID, mediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	if loc.ProbedAt == 0 {
		return model.StreamSource{}, model.ErrNotFound
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return model.StreamSource{}, err
	}
	root, err := paths.Open(cache)
	if err != nil {
		return model.StreamSource{}, err
	}
	defer root.Close() // a file opened through the root stays usable after it is closed
	f, err := root.Open(library.ChapterImagesRel(loc.File.ID, loc.ProbedAt) + "/" + library.ChapterImageName(idx))
	if errors.Is(err, fs.ErrNotExist) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open chapter image: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat chapter image: %w", err), f.Close())
	}
	return model.StreamSource{
		File: f, MediaType: "image/webp", Size: info.Size(), ModTime: info.ModTime().UTC(),
		Name: itemID + "/chapters/" + strconv.Itoa(idx), ETag: ChapterImageETag(loc.ProbedAt),
	}, nil
}
