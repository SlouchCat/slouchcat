package offline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Trickplay of a download (concept 18, 6.8): the preparation copies the complete preview set of the
// file into <dir>/<id>/trickplay/ (manifest.json and the sheets), so the offline budget counts it
// and the maintenance of the server cache cannot take it away. The manifest of the download names
// the sheets with the download token; getDownloadTrickplaySheet serves them.

// trickplayDir is the folder of the preview set inside the directory of a download;
// trickplayWaitMarker marks, by its modification time, since when the preparation waits for it.
const (
	trickplayDir        = "trickplay"
	trickplayWaitMarker = ".trickplay-wait"
	// trickplayPatience is how long a preparation waits for a missing set (6.8); then the
	// download goes on without preview. trickplayRetry is the delay of its next look.
	trickplayPatience = 10 * time.Minute
	trickplayRetry    = 30 * time.Second
)

// WithCacheDir sets the cache directory whose trickplay sets the preparation copies (concept 18,
// 6.8); without it downloads carry no preview.
func WithCacheDir(dir string) Option { return func(s *Service) { s.cacheDir = dir } }

// queueTrickplay asks for the set of the source of a film or episode download, so it is made while
// the download encodes; a failure is logged.
func (s *Service) queueTrickplay(ctx context.Context, p planned) {
	if s.cacheDir == "" || p.fileID == "" || p.audio {
		return
	}
	if _, err := library.QueueTrickplayForDownload(context.WithoutCancel(ctx), s.db.Read, s.queue, s.cacheDir, p.fileID); err != nil {
		s.log.WarnContext(ctx, "queue trickplay of a download", "media_file_id", p.fileID, "err", err)
	}
}

// copyTrickplay copies the complete set of the source into download id (6.8) and returns its
// bytes. Without a complete set it queues one and reports wait, until trickplayPatience after the
// first wait (the marker in the download); then, and for a file that gets no preview (the setting
// off, no cache directory, an audio title), the download goes on without one. A set that goes
// while it is copied counts as missing (waitTrickplay).
func (s *Service) copyTrickplay(ctx context.Context, root *os.Root, id string, p planned) (bytes int64, wait bool, err error) {
	if s.cacheDir == "" || p.fileID == "" || p.audio {
		return 0, false, nil
	}
	m, ok := library.ReadTrickplayManifest(s.cacheDir, p.fileID, p.version)
	if !ok {
		return s.waitTrickplay(ctx, root, id, p)
	}
	tmp := id + "/" + trickplayDir + ".tmp"
	if err := root.RemoveAll(tmp); err != nil { //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
		return 0, false, err
	}
	if err := root.MkdirAll(tmp, 0o750); err != nil {
		return 0, false, fmt.Errorf("create trickplay folder of the download: %w", err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return 0, false, err
	}
	if err := root.WriteFile(tmp+"/manifest.json", raw, 0o600); err != nil {
		return 0, false, fmt.Errorf("write trickplay manifest of the download: %w", err)
	}
	bytes = int64(len(raw))
	for n := range m.Sheets() {
		written, err := s.copySheet(root, tmp, p, n)
		if errors.Is(err, fs.ErrNotExist) {
			// The set went while it was copied (the maintenance of the cache, a set made again):
			// ask for it again and wait as for a missing one.
			if rerr := root.RemoveAll(tmp); rerr != nil { //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
				return 0, false, rerr
			}
			return s.waitTrickplay(ctx, root, id, p)
		}
		if err != nil {
			return 0, false, err
		}
		bytes += written
	}
	_ = root.Remove(id + "/" + trickplayWaitMarker)                 //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
	if err := root.RemoveAll(id + "/" + trickplayDir); err != nil { //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
		return 0, false, err
	}
	if err := root.Rename(tmp, id+"/"+trickplayDir); err != nil {
		return 0, false, fmt.Errorf("store trickplay of the download: %w", err)
	}
	return bytes, false, nil
}

// waitTrickplay queues the set of the source of download id (copyTrickplay) and reports wait until
// trickplayPatience after the first wait (the marker in the download); then, and for a file that
// gets no preview, the download goes on without one.
func (s *Service) waitTrickplay(ctx context.Context, root *os.Root, id string, p planned) (bytes int64, wait bool, err error) {
	queued, err := library.QueueTrickplayForDownload(context.WithoutCancel(ctx), s.db.Read, s.queue, s.cacheDir, p.fileID)
	if err != nil {
		return 0, false, fmt.Errorf("queue trickplay of %s: %w", p.fileID, err)
	}
	marker := id + "/" + trickplayWaitMarker
	if !queued {
		_ = root.Remove(marker) //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
		return 0, false, nil
	}
	info, err := root.Stat(marker)
	if errors.Is(err, fs.ErrNotExist) {
		if err := root.WriteFile(marker, nil, 0o600); err != nil {
			return 0, false, fmt.Errorf("mark trickplay wait: %w", err)
		}
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("trickplay wait marker: %w", err)
	}
	if time.Since(info.ModTime()) < trickplayPatience {
		return 0, true, nil
	}
	s.log.InfoContext(ctx, "trickplay not ready after the wait, download without preview", "download_id", id, "media_file_id", p.fileID)
	_ = root.Remove(marker) //nolint:forbidigo // the cache root (offline packages), never a library root (ADR 0032)
	return 0, false, nil
}

// copySheet copies sheet n of the set of the source into the folder dir of the download.
func (s *Service) copySheet(root *os.Root, dir string, p planned, n int) (int64, error) {
	src, err := library.OpenTrickplaySheet(s.cacheDir, p.fileID, p.version, n)
	if err != nil {
		return 0, fmt.Errorf("open trickplay sheet %d of %s: %w", n, p.fileID, err)
	}
	defer src.Close()
	dst, err := root.OpenFile(dir+"/"+library.TrickplaySheetName(n), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("create trickplay sheet of the download: %w", err)
	}
	written, err := io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, fmt.Errorf("copy trickplay sheet %d: %w", n, err)
	}
	return written, nil
}

// OpenTrickplaySheet opens sheet n of the preview set of a grant (getDownloadTrickplaySheet); a
// sheet the download does not carry is model.ErrNotFound.
func (s *Service) OpenTrickplaySheet(ctx context.Context, g model.DownloadGrant, n int) (model.StreamSource, error) {
	if _, err := s.prepared(ctx, g.DownloadID); err != nil {
		return model.StreamSource{}, err
	}
	if n < 0 {
		return model.StreamSource{}, model.ErrNotFound
	}
	name := library.TrickplaySheetName(n)
	return s.open(g.DownloadID+"/"+trickplayDir+"/"+name, "image/webp", name, model.ErrNotFound)
}

// downloadTrickplay reads the preview set copied into download id (nil without one, or with an
// unreadable manifest) with the sheet URLs of the download token.
func downloadTrickplay(root *os.Root, id, token string) *model.Trickplay {
	b, err := root.ReadFile(id + "/" + trickplayDir + "/manifest.json")
	if err != nil {
		return nil
	}
	var m library.TrickplayManifest
	if json.Unmarshal(b, &m) != nil || m.Columns <= 0 || m.Rows <= 0 {
		return nil
	}
	f, err := root.Open(id + "/" + trickplayDir)
	if err != nil {
		return nil
	}
	names, err := f.Readdirnames(0)
	_ = f.Close()
	if err != nil {
		return nil
	}
	var sheets []int
	for _, name := range names {
		base, ok := strings.CutSuffix(name, ".webp")
		if n, err := strconv.Atoi(base); ok && err == nil && n >= 0 && n < m.Sheets() && strconv.Itoa(n) == base {
			sheets = append(sheets, n)
		}
	}
	slices.Sort(sheets)
	out := &model.Trickplay{
		Version: m.Version, State: library.TrickplayComplete, IntervalMs: m.IntervalMs, TileWidth: m.TileWidth,
		TileHeight: m.TileHeight, Columns: m.Columns, Rows: m.Rows, PointsMs: m.PointsMs,
		Sheets: make([]model.TrickplaySheet, 0, len(sheets)),
	}
	for _, n := range sheets {
		out.Sheets = append(out.Sheets, model.TrickplaySheet{
			Index: n, URL: apiBase + "/downloads/" + id + "/trickplay/" + strconv.Itoa(n) + "?dt=" + token,
		})
	}
	return out
}
