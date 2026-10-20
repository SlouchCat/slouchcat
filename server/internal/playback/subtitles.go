package playback

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Text subtitles as WebVTT (W-4, M4 design 2.2 getPlaybackSubtitle): the selected text track of a
// playback (an embedded SubRip/ASS/MOV-text stream or a sidecar file) is converted to WebVTT by
// the injected converter (transcode.Runner through WithSubtitleConverter) once per playback
// session and cached under <cache>/subtitles/<playback id>/<track>.vtt until the session ends.
// Without a converter text subtitles are not available (404).

// SubtitleConverter converts subtitles to WebVTT: embedded converts the subtitle stream with the
// index of a media file, sidecar a subtitle file; both take absolute paths resolved through
// package paths.
type SubtitleConverter struct {
	Embedded func(ctx context.Context, path string, streamIndex int) ([]byte, error)
	Sidecar  func(ctx context.Context, path string) ([]byte, error)
}

// WithSubtitleConverter enables the WebVTT endpoint with the converters of package transcode
// (transcode.Runner.Subtitles and SubtitlesFile).
func WithSubtitleConverter(embedded func(ctx context.Context, path string, streamIndex int) ([]byte, error),
	sidecar func(ctx context.Context, path string) ([]byte, error),
) Option {
	return func(s *Service) { s.subs = SubtitleConverter{Embedded: embedded, Sidecar: sidecar} }
}

// WithCacheDir sets the cache directory (SLOUCHCAT cache); converted subtitles are kept below
// <dir>/subtitles. Without it every request converts again.
func WithCacheDir(dir string) Option {
	return func(s *Service) { s.cacheDir = dir }
}

// playbackID is the form of a playback session id (ULID), checked before it names a directory.
var playbackID = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// Subtitle returns the WebVTT file of a text subtitle track of a grant. Only the track the
// playback selected for text delivery is served (PlaybackGrant.SubtitleTrack); any other track, a
// bitmap track, a title that is no longer visible and a server without converter return
// model.ErrNotFound.
func (s *Service) Subtitle(ctx context.Context, g model.PlaybackGrant, trackID int64) (model.StreamSource, error) {
	if trackID <= 0 || trackID != g.SubtitleTrack || !playbackID.MatchString(g.ID) {
		return model.StreamSource{}, model.ErrNotFound
	}
	scope := visibility.Profile(g.Session.Profile)
	loc, err := s.vis.MediaFile(ctx, scope, g.ItemID, g.MediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	tracks, err := s.vis.Tracks(ctx, scope, g.ItemID, g.MediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	var track *model.Track
	for i := range tracks.Subtitles {
		if tracks.Subtitles[i].ID == trackID && tracks.Subtitles[i].Text {
			track = &tracks.Subtitles[i]
		}
	}
	if track == nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	cached := ""
	if s.cacheDir != "" {
		cached = filepath.Join(s.cacheDir, "subtitles", g.ID, strconv.FormatInt(trackID, 10)+".vtt")
		if data, err := os.ReadFile(cached); err == nil { //nolint:gosec // G304: below the cache dir, playback id and track id checked
			return vttSource(data, trackID), nil
		}
	}
	data, err := s.convert(ctx, loc, *track)
	if err != nil {
		return model.StreamSource{}, err
	}
	if cached != "" {
		if err := writeAtomic(cached, data); err != nil {
			s.log.WarnContext(ctx, "cache subtitles", "playback_id", g.ID, "track_id", trackID, "err", err)
		}
	}
	return vttSource(data, trackID), nil
}

// Tracks returns the audio and subtitle tracks of a media file of a visible title (getItemTracks,
// I-5); an empty mediaFileID selects the primary file. Sidecar paths stay on the server.
func (s *Service) Tracks(ctx context.Context, profile model.Profile, itemID, mediaFileID string) (model.TrackList, error) {
	if len(mediaFileID) > maxClientIDLength {
		return model.TrackList{}, model.ErrNotFound
	}
	scope := visibility.Profile(profile)
	loc, err := s.vis.MediaFile(ctx, scope, itemID, mediaFileID)
	if err != nil {
		return model.TrackList{}, err
	}
	return s.vis.Tracks(ctx, scope, loc.ItemID, loc.File.ID)
}

// convert runs the converter of the track's source.
func (s *Service) convert(ctx context.Context, loc visibility.MediaFileLocation, t model.Track) ([]byte, error) {
	if t.Source == "sidecar" {
		if s.subs.Sidecar == nil {
			return nil, model.ErrNotFound
		}
		abs, err := rootPath(loc.RootPath, t.RelPath, "sidecar subtitle")
		if err != nil {
			return nil, notFoundIfMissing(err)
		}
		data, err := s.subs.Sidecar(ctx, abs)
		if err != nil {
			return nil, fmt.Errorf("convert sidecar subtitle %d: %w", t.ID, err)
		}
		return data, nil
	}
	if s.subs.Embedded == nil {
		return nil, model.ErrNotFound
	}
	abs, err := sourcePath(loc)
	if err != nil {
		return nil, err
	}
	data, err := s.subs.Embedded(ctx, abs, t.Index)
	if err != nil {
		return nil, fmt.Errorf("convert subtitle stream %d of %s: %w", t.Index, loc.File.ID, err)
	}
	return data, nil
}

type vttReader struct{ *bytes.Reader }

func (vttReader) Close() error { return nil }

func vttSource(data []byte, trackID int64) model.StreamSource {
	return model.StreamSource{
		File: vttReader{bytes.NewReader(data)}, MediaType: "text/vtt; charset=utf-8", Size: int64(len(data)),
		Name: strconv.FormatInt(trackID, 10) + ".vtt",
	}
}

// writeAtomic writes data to path through a temporary file in the same directory.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".vtt-*")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}

// removeSubtitles deletes the cached subtitles of an ended playback session.
func (s *Service) removeSubtitles(id string) {
	if s.cacheDir == "" || !playbackID.MatchString(id) {
		return
	}
	dir := filepath.Join(s.cacheDir, "subtitles", id)
	if err := os.RemoveAll(dir); err != nil {
		s.log.Warn("remove cached subtitles", "playback_id", id, "err", err)
	}
}

// subtitleCacheAge is how long the sweep keeps a subtitle directory whose playback session row is
// gone (ended more than a day ago, or lost with a database restore).
const subtitleCacheAge = 2 * endedRetention

// sweepSubtitles removes subtitle directories older than subtitleCacheAge.
func (s *Service) sweepSubtitles(now time.Time) {
	if s.cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(filepath.Join(s.cacheDir, "subtitles"))
	if err != nil {
		return // nothing cached yet
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !e.IsDir() || !playbackID.MatchString(e.Name()) || now.Sub(info.ModTime()) < subtitleCacheAge {
			continue
		}
		s.removeSubtitles(e.Name())
	}
}
