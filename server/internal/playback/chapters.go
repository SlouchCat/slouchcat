package playback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Chapter images (concept 13 stage 3): the job chapters.images of package library writes them below
// the cache directory per version of a media file (library.ChapterImagesVersion: probe time and
// chapter fingerprint); playback names the ones that exist and serves them (getChapterImage).
// Nothing here runs ffmpeg.

// withChapterImages sets Chapter.ImageURL on the chapters of a film or an episode whose image
// exists for the file's current version. One directory read, no ffmpeg in the request path; without
// a cache directory, a probe or the folder the chapters stay as they are.
func (s *Service) withChapterImages(ctx context.Context, item model.ItemSummary, loc visibility.MediaFileLocation, chapters []model.Chapter) {
	if s.cacheDir == "" || loc.ProbedAt == 0 || len(chapters) == 0 || (item.Kind != "movie" && item.Kind != "episode") {
		return
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return
	}
	version := library.ChapterImagesVersion(loc.ProbedAt, chapters)
	names, err := paths.ReadDirNames(cache, library.ChapterImagesRel(loc.File.ID, version), 0)
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
			chapters[i].ImageURL = ChapterImageURL(item.ID, loc.File.ID, i, version)
		}
	}
}

// ChapterImageURL is the relative URL of getChapterImage; v is the version of the images
// (library.ChapterImagesVersion), so the immutable image of another probe or chapter list is never
// shown for this one.
func ChapterImageURL(itemID, mediaFileID string, idx int, version string) string {
	return "/api/v1/items/" + url.PathEscape(itemID) + "/chapters/" + strconv.Itoa(idx) + "/image?mediaFileId=" +
		url.QueryEscape(mediaFileID) + "&v=" + url.QueryEscape(version)
}

// ChapterImageETag is the strong validator of a chapter image: its version in quotes. The image of
// one version never changes.
func ChapterImageETag(version string) string { return strconv.Quote(version) }

// maxChapterIndex bounds the chapter index of getChapterImage (a film with replacement chapters
// every 10 minutes has a few dozen).
const maxChapterIndex = 9999

// chapterVersion is the form of library.ChapterImagesVersion.
var chapterVersion = regexp.MustCompile(`^(\d+)-[0-9a-f]{8}$`)

// ChapterImage opens the image of chapter idx of the media file mediaFileID of a visible title
// (getChapterImage): mediaFileID must be a ULID of a file of that title, idx a chapter whose image
// exists. Anything else, also an invisible title, a file of another title and a title above the
// profile's age rating, is model.ErrNotFound (never 403). A version of the file's current probe
// (the v of ChapterImageURL) picks its folder; without one, or with one of another probe, the
// current chapters of the file pick it. The file is opened through package paths below the cache
// directory (CLAUDE.md rule 4); its ETag is ChapterImageETag of the version it belongs to.
func (s *Service) ChapterImage(ctx context.Context, profile model.Profile, itemID, mediaFileID string, idx int, version string) (model.StreamSource, error) {
	if s.cacheDir == "" || idx < 0 || idx > maxChapterIndex {
		return model.StreamSource{}, model.ErrNotFound
	}
	if _, err := ulid.ParseStrict(mediaFileID); err != nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	scope := visibility.Profile(profile)
	loc, err := s.vis.MediaFile(ctx, scope, itemID, mediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	if loc.ProbedAt == 0 {
		return model.StreamSource{}, model.ErrNotFound
	}
	if m := chapterVersion.FindStringSubmatch(version); m == nil || m[1] != strconv.FormatInt(loc.ProbedAt, 10) {
		if version, err = s.currentChapterVersion(ctx, scope, itemID, loc); err != nil {
			return model.StreamSource{}, err
		}
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
	f, err := root.Open(library.ChapterImagesRel(loc.File.ID, version) + "/" + library.ChapterImageName(idx))
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
		Name: itemID + "/chapters/" + strconv.Itoa(idx), ETag: ChapterImageETag(version),
	}, nil
}

// currentChapterVersion computes the version of the chapter images of a file from its stored probe
// and the kind of its title, as Start does (the request without a current v).
func (s *Service) currentChapterVersion(ctx context.Context, scope visibility.Scope, itemID string, loc visibility.MediaFileLocation) (string, error) {
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return "", err
	}
	probe, _, err := s.storedProbe(ctx, scope, loc)
	if err != nil {
		return "", err
	}
	if probe == nil {
		return "", model.ErrNotFound
	}
	return library.ChapterImagesVersion(loc.ProbedAt, transcode.Chapters(probe, item.Kind, loc.File.DurationMs)), nil
}

// sweepChapterImages hands the media files of the playbacks that ended since the last sweep to
// library.QueueChapterImagesAfterPlayback (concept 13 11.6: after the playback, never during it).
// A stop of the client, the administration, a logout, a replacement and an expiry (a closed or
// killed app) all end a row; the job waits 10 minutes and longer while the file plays again. The
// first sweep after a start looks back endedSweepWindow, so a restart loses nothing.
func (s *Service) sweepChapterImages(ctx context.Context, now time.Time) {
	if s.cacheDir == "" || s.queue == nil {
		return
	}
	since := s.chapterSince.Load()
	if since == 0 {
		since = now.Add(-endedSweepWindow).UnixMilli()
	}
	files, err := s.read.ListEndedPlaybackFiles(ctx, appq.ListEndedPlaybackFilesParams{Since: since, Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}})
	if err != nil {
		s.log.WarnContext(ctx, "list ended playback files", "err", err)
		return
	}
	if err := library.QueueChapterImagesAfterPlayback(ctx, s.db.Read, s.queue, s.cacheDir, files); err != nil {
		s.log.WarnContext(ctx, "queue chapter images after playback", "files", len(files), "err", err)
		return
	}
	s.chapterSince.Store(now.UnixMilli())
}
