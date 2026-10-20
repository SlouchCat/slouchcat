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
	"github.com/slouchcat/slouchcat/server/internal/paths"
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
// bitmap track, a title that is no longer visible, a missing source file and a server without
// converter return model.ErrNotFound. Concurrent requests of the same playback and track share one
// conversion (convertShared).
func (s *Service) Subtitle(ctx context.Context, g model.PlaybackGrant, trackID int64) (model.StreamSource, error) {
	if trackID <= 0 || trackID != g.SubtitleTrack || !playbackID.MatchString(g.ID) {
		return model.StreamSource{}, model.ErrNotFound
	}
	scope := visibility.Profile(g.Session.Profile)
	loc, err := s.vis.MediaFile(ctx, scope, g.ItemID, g.MediaFileID)
	if err != nil {
		return model.StreamSource{}, err
	}
	if !loc.File.Available {
		return model.StreamSource{}, model.ErrNotFound
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
	data, err := s.convertShared(ctx, g.ID+"/"+strconv.FormatInt(trackID, 10), loc, *track, cached)
	if err != nil {
		return model.StreamSource{}, err
	}
	return vttSource(data, trackID), nil
}

const (
	// maxSubtitleConversions bounds the WebVTT conversions (ffmpeg processes) that run at once;
	// they are not part of the transcode budget.
	maxSubtitleConversions = 4
	// subtitleConvertTimeout bounds one shared conversion including its wait for a slot (the
	// converter has its own 60-s bound).
	subtitleConvertTimeout = 2 * time.Minute
)

// subtitleFlight is one running conversion; done is closed when data and err are set.
type subtitleFlight struct {
	done chan struct{}
	data []byte
	err  error
}

// convertShared converts track once for all concurrent requests of the same playback session and
// track (key): the first request starts the conversion, the others wait for its result. The
// conversion runs detached from the requests (a client that goes away does not cancel it for the
// others), waits for one of maxSubtitleConversions slots and writes the cache file (cached, empty =
// uncached) before the waiters get the result.
func (s *Service) convertShared(ctx context.Context, key string, loc visibility.MediaFileLocation, track model.Track, cached string) ([]byte, error) {
	s.subMu.Lock()
	f, running := s.subFlights[key]
	if !running {
		f = &subtitleFlight{done: make(chan struct{})}
		s.subFlights[key] = f
		go s.runConversion(context.WithoutCancel(ctx), key, f, loc, track, cached)
	}
	s.subMu.Unlock()
	select {
	case <-f.done:
		return f.data, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) runConversion(ctx context.Context, key string, f *subtitleFlight, loc visibility.MediaFileLocation, track model.Track, cached string) {
	ctx, cancel := context.WithTimeout(ctx, subtitleConvertTimeout)
	defer cancel()
	defer func() {
		s.subMu.Lock()
		delete(s.subFlights, key)
		s.subMu.Unlock()
		close(f.done)
	}()
	select {
	case s.subSlots <- struct{}{}:
		defer func() { <-s.subSlots }()
	case <-ctx.Done():
		f.err = fmt.Errorf("wait for a subtitle conversion slot: %w", ctx.Err())
		return
	}
	f.data, f.err = s.convert(ctx, loc, track)
	if f.err == nil && cached != "" {
		if err := writeAtomic(cached, f.data); err != nil {
			s.log.WarnContext(ctx, "cache subtitles", "key", key, "err", err)
		}
	}
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

// convert runs the converter of the track's source. A source file that no longer exists is
// model.ErrNotFound.
func (s *Service) convert(ctx context.Context, loc visibility.MediaFileLocation, t model.Track) ([]byte, error) {
	if t.Source == "sidecar" {
		if s.subs.Sidecar == nil {
			return nil, model.ErrNotFound
		}
		if err := fileExists(loc.RootPath, t.RelPath); err != nil {
			return nil, err
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
	if err := fileExists(loc.RootPath, loc.RelPath); err != nil {
		return nil, err
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

// fileExists checks through package paths that rel is a file below the library root rootDir; a
// missing file (or a directory) is model.ErrNotFound.
func fileExists(rootDir, rel string) error {
	root, err := paths.NewRoot(rootDir)
	if err != nil {
		return err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return notFoundIfMissing(err)
	}
	defer dir.Close()
	info, err := dir.Stat(rel)
	if err != nil {
		return notFoundIfMissing(err)
	}
	if info.IsDir() {
		return model.ErrNotFound
	}
	return nil
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
