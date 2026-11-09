package playback

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Trickplay (concept 18 part A, ADR 0027): the job trickplay.generate of package library writes the
// preview sets below the cache directory; playback reports the set of the file a playback plays
// (PlaybackInfo.trickplay, getTrickplay) and serves its sheets (getTrickplaySheet). The library
// setting, the visibility and the version are decided here, the client only shows (rule 12).
// Nothing here runs ffmpeg.

// maxTrickplaySheet bounds the sheet number of getTrickplaySheet (a film of 10 hours has 36).
const maxTrickplaySheet = 9999

// trickplayFor returns the preview set of the file loc of a title for the app, nil when the title
// gets none: no film or episode, the library setting off, no probe or no video stream.
func (s *Service) trickplayFor(ctx context.Context, item model.ItemSummary, loc visibility.MediaFileLocation, probe *transcode.Probe, kf keyframeIndex) (*model.Trickplay, error) {
	setting := loc.Trickplay.For(item.Kind)
	if setting.Mode == "" || setting.Mode == model.TrickplayOff {
		return nil, nil
	}
	video := mainVideo(probe)
	if video == nil {
		return nil, nil
	}
	var keyframes []time.Duration
	switch kf.source {
	case library.KeyframesCues, library.KeyframesMP4, library.KeyframesScan:
		if decoded, err := library.DecodeKeyframes(kf.blob); err == nil {
			keyframes = decoded
		}
	}
	version := library.FileVersion{Size: loc.File.SizeBytes, Mtime: loc.Mtime}
	m, sheets, state, ok, err := library.TrickplayStatus(ctx, s.db.Read, s.cacheDir, library.TrickplayFile{
		ID: loc.File.ID, Version: version, DurationMs: loc.File.DurationMs, Video: video, Keyframes: keyframes,
	})
	if err != nil || !ok {
		return nil, err
	}
	out := &model.Trickplay{
		Version: m.Version, State: state, IntervalMs: m.IntervalMs, TileWidth: m.TileWidth, TileHeight: m.TileHeight,
		Columns: m.Columns, Rows: m.Rows, PointsMs: m.PointsMs, Sheets: make([]model.TrickplaySheet, 0, len(sheets)),
	}
	for _, n := range sheets {
		out.Sheets = append(out.Sheets, model.TrickplaySheet{Index: n, URL: TrickplaySheetURL(item.ID, loc.File.ID, n, m.Version)})
	}
	return out, nil
}

// queueTrickplay is the start trigger of the preview set (library.QueueTrickplayForPlayback): the
// file from the start position, and for an episode at «Beim Start alle Folgen» the look-ahead of
// its series. A failure is logged; the playback starts all the same.
func (s *Service) queueTrickplay(ctx context.Context, item model.ItemSummary, loc visibility.MediaFileLocation, startMs int64, seriesID string) {
	if s.queue == nil || s.cacheDir == "" {
		return
	}
	err := library.QueueTrickplayForPlayback(context.WithoutCancel(ctx), s.queue, s.cacheDir, library.TrickplayStart{
		MediaFileID: loc.File.ID, Version: library.FileVersion{Size: loc.File.SizeBytes, Mtime: loc.Mtime}, Kind: item.Kind,
		StartMs: startMs, Setting: loc.Trickplay.For(item.Kind), SeriesID: seriesID, EpisodeID: item.ID,
	})
	if err != nil {
		s.log.WarnContext(ctx, "queue trickplay", "media_file_id", loc.File.ID, "err", err)
	}
}

// sweepTrickplay removes the temporary preview sets of files without an open playback for 10
// minutes (library.SweepTrickplaySessions, concept 18 6.5); a failure is logged.
func (s *Service) sweepTrickplay(ctx context.Context, now time.Time) {
	if s.cacheDir == "" {
		return
	}
	if err := library.SweepTrickplaySessions(ctx, s.db.Read, s.cacheDir, now); err != nil {
		s.log.WarnContext(ctx, "sweep temporary trickplay sets", "err", err)
	}
}

// TrickplaySheetURL is the relative URL of getTrickplaySheet; v is the version of the set
// (library.TrickplaySetVersion), so the immutable sheet of another file version or of a set made
// again is never shown.
func TrickplaySheetURL(itemID, mediaFileID string, sheet int, version string) string {
	return "/api/v1/items/" + url.PathEscape(itemID) + "/trickplay/" + strconv.Itoa(sheet) + "?mediaFileId=" +
		url.QueryEscape(mediaFileID) + "&v=" + url.QueryEscape(version)
}

// TrickplaySheetETag is the strong validator of a sheet: version of the set and sheet in quotes. A
// sheet of one set version never changes.
func TrickplaySheetETag(version string, sheet int) string {
	return strconv.Quote(version + "/" + strconv.Itoa(sheet))
}

// Trickplay returns the preview set of the media file mediaFileID of a visible title (getTrickplay):
// mediaFileID must be a ULID of a file of that title. An invisible or unknown title, a file of
// another title and a title without preview (no film or episode, no video, the setting off) are
// model.ErrNotFound (never 403).
func (s *Service) Trickplay(ctx context.Context, profile model.Profile, itemID, mediaFileID string) (model.Trickplay, error) {
	if _, err := ulid.ParseStrict(mediaFileID); err != nil {
		return model.Trickplay{}, model.ErrNotFound
	}
	scope := visibility.Profile(profile)
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return model.Trickplay{}, err
	}
	loc, err := s.vis.MediaFile(ctx, scope, itemID, mediaFileID)
	if err != nil {
		return model.Trickplay{}, err
	}
	probe, kf, err := s.storedProbe(ctx, scope, loc)
	if err != nil {
		return model.Trickplay{}, err
	}
	tp, err := s.trickplayFor(ctx, item, loc, probe, kf)
	if err != nil {
		return model.Trickplay{}, err
	}
	if tp == nil {
		return model.Trickplay{}, model.ErrNotFound
	}
	return *tp, nil
}

// TrickplaySheet opens sheet n of the preview set of the media file mediaFileID of a visible title
// (getTrickplaySheet) at the file's current version. Anything else, also an invisible title, a
// file of another title, a title whose library setting is off and a sheet that does not exist, is
// model.ErrNotFound (never 403). The ETag is TrickplaySheetETag of the version of the set that
// serves the sheet; the caller caches it for good only when v names that version.
func (s *Service) TrickplaySheet(ctx context.Context, profile model.Profile, itemID, mediaFileID string, n int) (model.StreamSource, string, error) {
	if s.cacheDir == "" || n < 0 || n > maxTrickplaySheet {
		return model.StreamSource{}, "", model.ErrNotFound
	}
	if _, err := ulid.ParseStrict(mediaFileID); err != nil {
		return model.StreamSource{}, "", model.ErrNotFound
	}
	scope := visibility.Profile(profile)
	item, err := s.vis.Summary(ctx, scope, itemID)
	if err != nil {
		return model.StreamSource{}, "", err
	}
	loc, err := s.vis.MediaFile(ctx, scope, itemID, mediaFileID)
	if err != nil {
		return model.StreamSource{}, "", err
	}
	if mode := loc.Trickplay.For(item.Kind).Mode; mode == "" || mode == model.TrickplayOff {
		return model.StreamSource{}, "", model.ErrNotFound
	}
	v := library.FileVersion{Size: loc.File.SizeBytes, Mtime: loc.Mtime}
	f, err := library.OpenTrickplaySheet(s.cacheDir, loc.File.ID, v, n)
	if errors.Is(err, fs.ErrNotExist) {
		return model.StreamSource{}, "", model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, "", fmt.Errorf("open trickplay sheet: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, "", errors.Join(fmt.Errorf("stat trickplay sheet: %w", err), f.Close())
	}
	// The version of the set that serves the sheet (its manifest): a set made again gets new URLs.
	version, _ := library.TrickplaySetVersionOf(s.cacheDir, loc.File.ID, v)
	return model.StreamSource{
		File: f, MediaType: "image/webp", Size: info.Size(), ModTime: info.ModTime().UTC(),
		Name: itemID + "/trickplay/" + strconv.Itoa(n), ETag: TrickplaySheetETag(version, n),
	}, version, nil
}
