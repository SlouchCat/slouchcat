package offline

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The prepared copy on disk, the manifest and the download token (M5 design 2 and 3.3).
//
// <dir>/<id>/media.mp4 is the init segment followed by the media segments in order;
// <dir>/<id>/index.jsonl holds one JSON line for the init segment (indexHeader) and one per
// appended segment (indexEntry); <dir>/<id>/subs/<track id>.vtt are the WebVTT sidecars.

// File names inside the directory of a download.
const (
	mediaFile = "media.mp4"
	indexFile = "index.jsonl"
	subsDir   = "subs"
)

// apiBase is the prefix of the token URLs (api.BasePath).
const apiBase = "/api/v1"

// indexHeader is the first line of index.jsonl.
type indexHeader struct {
	Init   int64  `json:"init"` // length of the init segment at offset 0
	Codecs string `json:"codecs"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Plan is the fingerprint of the encode (planFingerprint); a resumed preparation continues the
	// copy only under the same plan.
	Plan string `json:"plan,omitempty"`
}

// indexEntry is one appended media segment.
type indexEntry struct {
	N          int   `json:"n"`
	StartMs    int64 `json:"startMs"`
	DurationMs int64 `json:"durationMs"`
	Offset     int64 `json:"offset"`
	Length     int64 `json:"length"`
}

// index is the readable prefix of index.jsonl: the header, the contiguous entries and the byte
// length of the lines they came from.
type index struct {
	header  indexHeader
	entries []indexEntry
	bytes   int64
}

// end is the end offset of the last entry (the init length without entries).
func (ix index) end() int64 {
	if n := len(ix.entries); n > 0 {
		return ix.entries[n-1].Offset + ix.entries[n-1].Length
	}
	return ix.header.Init
}

// parseIndex reads an index up to its first unusable line (a crash in the middle of a write): the
// header needs an init length, every entry the next number, the offset right after its
// predecessor and a positive length within fileSize.
func parseIndex(data []byte, fileSize int64) (index, bool) {
	var ix index
	first := true
	for rest := data; len(rest) > 0; {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			break // the last line lacks its newline: written partially
		}
		line := rest[:i]
		rest = rest[i+1:]
		if first {
			if json.Unmarshal(line, &ix.header) != nil || ix.header.Init <= 0 || ix.header.Init > fileSize {
				return index{}, false
			}
			first = false
			ix.bytes += int64(i + 1)
			continue
		}
		var e indexEntry
		if json.Unmarshal(line, &e) != nil || e.N != len(ix.entries) || e.Offset != ix.end() || e.Length <= 0 ||
			e.Offset+e.Length > fileSize || e.DurationMs <= 0 {
			break
		}
		ix.entries = append(ix.entries, e)
		ix.bytes += int64(i + 1)
	}
	return ix, !first
}

// root opens the downloads directory through package paths (created when missing).
func (s *Service) root() (*os.Root, error) {
	if s.dir == "" {
		return nil, errors.New("offline: no downloads directory configured")
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return nil, fmt.Errorf("create downloads directory: %w", err)
	}
	r, err := paths.NewRoot(s.dir)
	if err != nil {
		return nil, err
	}
	return paths.Open(r)
}

// errIndexUnreadable is an index without a usable header.
var errIndexUnreadable = errors.New("offline: index of the prepared download unreadable")

// readIndex reads the index of a prepared download.
func readIndex(root *os.Root, id string) (index, error) {
	info, err := root.Stat(id + "/" + mediaFile)
	if err != nil {
		return index{}, err
	}
	data, err := root.ReadFile(id + "/" + indexFile)
	if err != nil {
		return index{}, err
	}
	ix, ok := parseIndex(data, info.Size())
	if !ok {
		return index{}, fmt.Errorf("download %s: %w", id, errIndexUnreadable)
	}
	return ix, nil
}

// removeFiles deletes the directory of a download (server copy, index, subtitles).
func (s *Service) removeFiles(ctx context.Context, id string) {
	if !downloadID.MatchString(id) || s.dir == "" {
		return
	}
	root, err := s.root()
	if err != nil {
		s.log.WarnContext(ctx, "open downloads directory", "err", err)
		return
	}
	defer root.Close()
	if err := root.RemoveAll(id); err != nil {
		s.log.WarnContext(ctx, "remove download files", "download_id", id, "err", err)
	}
}

// tokenMAC is the HMAC of a download token: download, title and profile.
func tokenMAC(key []byte, downloadID, itemID string, profileID int64) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(downloadID + "\n" + itemID + "\n" + strconv.FormatInt(profileID, 10)))
	return mac.Sum(nil)
}

// SignToken returns the download token of a download for a profile:
// <download id>.<profile id>.<base64url HMAC-SHA256(key, id \n item \n profile)>, without expiry.
func SignToken(key []byte, downloadID, itemID string, profileID int64) string {
	return downloadID + "." + strconv.FormatInt(profileID, 10) + "." +
		base64.RawURLEncoding.EncodeToString(tokenMAC(key, downloadID, itemID, profileID))
}

// parseToken splits a download token; ok is false for anything malformed.
func parseToken(token string) (id string, profileID int64, mac []byte, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || !downloadID.MatchString(parts[0]) {
		return "", 0, nil, false
	}
	pid, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || pid <= 0 {
		return "", 0, nil, false
	}
	mac, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(mac) != sha256.Size {
		return "", 0, nil, false
	}
	return parts[0], pid, mac, true
}

// AuthorizeDownload resolves a download token. Every request reads the download and checks that
// the token's profile belongs to the download's account and sees the title, so a fetch goes on
// after a profile switch, but never for a profile the title is hidden from. Errors:
// model.ErrUnauthenticated (missing, malformed or forged, or naming an unknown download: without
// its row a token cannot be told from a forged one, and a different answer would tell which ids
// exist), model.ErrNotFound (profile of another account, title not visible to the profile).
func (s *Service) AuthorizeDownload(ctx context.Context, token string) (model.DownloadGrant, error) {
	id, pid, mac, ok := parseToken(token)
	if !ok || len(s.key) == 0 {
		return model.DownloadGrant{}, model.ErrUnauthenticated
	}
	row, err := s.read.GetDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DownloadGrant{}, model.ErrUnauthenticated
	}
	if err != nil {
		return model.DownloadGrant{}, fmt.Errorf("get download: %w", err)
	}
	if !hmac.Equal(mac, tokenMAC(s.key, id, row.ItemID, pid)) {
		return model.DownloadGrant{}, model.ErrUnauthenticated
	}
	account, err := s.read.GetProfileAccount(ctx, pid)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && account != row.AccountID) {
		return model.DownloadGrant{}, model.ErrNotFound
	}
	if err != nil {
		return model.DownloadGrant{}, fmt.Errorf("get profile account: %w", err)
	}
	if err := s.vis.Visible(ctx, visibility.Profile(model.Profile{ID: pid}), row.ItemID); err != nil {
		return model.DownloadGrant{}, err
	}
	return model.DownloadGrant{DownloadID: id, ItemID: row.ItemID, AccountID: row.AccountID, ProfileID: pid}, nil
}

// prepared returns the row of a grant while its server copy exists (model.ErrDownloadEvicted
// otherwise; model.ErrNotFound when the download is gone).
func (s *Service) prepared(ctx context.Context, id string) (systemq.Download, error) {
	row, err := s.read.GetDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return systemq.Download{}, model.ErrNotFound
	}
	if err != nil {
		return systemq.Download{}, fmt.Errorf("get download: %w", err)
	}
	if row.State != model.DownloadReady || row.FilesRemovedAt.Valid {
		return systemq.Download{}, model.ErrDownloadEvicted
	}
	return row, nil
}

// OpenFile opens the prepared file of a grant (getDownloadFile). model.ErrDownloadEvicted when
// the server copy is gone or not prepared. The ETag names the download and its preparation, so a
// range request with If-Range never mixes the bytes of a copy prepared again after an eviction
// (a hardware encoder is not byte-exact) with the old ones.
func (s *Service) OpenFile(ctx context.Context, g model.DownloadGrant) (model.StreamSource, error) {
	row, err := s.prepared(ctx, g.DownloadID)
	if err != nil {
		return model.StreamSource{}, err
	}
	src, err := s.open(g.DownloadID+"/"+mediaFile, "video/mp4", g.DownloadID+".mp4", model.ErrDownloadEvicted)
	if err != nil {
		return model.StreamSource{}, err
	}
	src.ETag = `"` + row.ID + "-" + strconv.FormatInt(row.ReadyAt.Int64, 36) + `"`
	return src, nil
}

// OpenSubtitle opens a WebVTT sidecar of a grant (getDownloadSubtitle); a track the download does
// not carry is model.ErrNotFound.
func (s *Service) OpenSubtitle(ctx context.Context, g model.DownloadGrant, trackID int64) (model.StreamSource, error) {
	if _, err := s.prepared(ctx, g.DownloadID); err != nil {
		return model.StreamSource{}, err
	}
	if trackID <= 0 {
		return model.StreamSource{}, model.ErrNotFound
	}
	name := strconv.FormatInt(trackID, 10) + ".vtt"
	return s.open(g.DownloadID+"/"+subsDir+"/"+name, "text/vtt; charset=utf-8", name, model.ErrNotFound)
}

// open opens a file below the downloads directory; a missing file is missing.
func (s *Service) open(rel, mediaType, name string, missing error) (model.StreamSource, error) {
	root, err := s.root()
	if err != nil {
		return model.StreamSource{}, err
	}
	defer root.Close()
	f, err := root.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return model.StreamSource{}, missing
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open %s: %w", name, err)
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		_ = f.Close()
		return model.StreamSource{}, missing
	}
	return model.StreamSource{File: f, MediaType: mediaType, Size: info.Size(), ModTime: info.ModTime(), Name: name}, nil
}

// Manifest returns what the device needs to fetch and play a prepared download of its own: the
// fragment index, codecs, size, the audio track, the WebVTT sidecars and the URLs with a download
// token of the active profile. Errors: model.ErrNotFound (unknown, another device or account, the
// title not visible to the active profile), model.ErrDownloadNotReady, model.ErrDownloadEvicted.
func (s *Service) Manifest(ctx context.Context, session model.Session, id string) (model.DownloadManifest, error) {
	if !downloadID.MatchString(id) {
		return model.DownloadManifest{}, model.ErrNotFound
	}
	row, err := s.read.GetDownload(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DownloadManifest{}, model.ErrNotFound
	}
	if err != nil {
		return model.DownloadManifest{}, fmt.Errorf("get download: %w", err)
	}
	if row.DeviceID != session.DeviceID || row.AccountID != session.Account.ID {
		return model.DownloadManifest{}, model.ErrNotFound
	}
	scope := visibility.Profile(session.Profile)
	if err := s.vis.Visible(ctx, scope, row.ItemID); err != nil {
		return model.DownloadManifest{}, err
	}
	switch {
	case row.State == model.DownloadEvicted || row.FilesRemovedAt.Valid:
		return model.DownloadManifest{}, model.ErrDownloadEvicted
	case row.State != model.DownloadReady:
		return model.DownloadManifest{}, model.ErrDownloadNotReady
	}
	root, err := s.root()
	if err != nil {
		return model.DownloadManifest{}, err
	}
	defer root.Close()
	ix, err := readIndex(root, id)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errIndexUnreadable) {
		// A ready row without its copy (removed by hand, disk replaced) or with a broken index:
		// evict it with what is left of the files, so the next sync prepares it again.
		s.log.WarnContext(ctx, "prepared download without readable copy, evicting", "download_id", id, "err", err)
		s.removeFiles(ctx, id)
		if _, err := s.write.EvictDownload(ctx, systemq.EvictDownloadParams{Now: ms(s.now()), ID: id}); err != nil {
			return model.DownloadManifest{}, fmt.Errorf("evict download: %w", err)
		}
		return model.DownloadManifest{}, model.ErrDownloadEvicted
	}
	if err != nil {
		// Anything else (an I/O error of the cache disk) may pass; the copy stays.
		return model.DownloadManifest{}, fmt.Errorf("read index of download %s: %w", id, err)
	}
	tracks, err := s.vis.Tracks(ctx, scope, row.ItemID, row.MediaFileID.String)
	if err != nil {
		return model.DownloadManifest{}, err
	}
	token := url.QueryEscape(SignToken(s.key, id, row.ItemID, session.Profile.ID))
	out := model.DownloadManifest{
		ID: id, ItemID: row.ItemID, SizeBytes: ix.end(), DurationMs: row.DurationMs.Int64, MimeType: "video/mp4",
		Codecs: ix.header.Codecs, Width: ix.header.Width, Height: ix.header.Height, InitLength: ix.header.Init,
		Segments: make([]model.DownloadSegment, 0, len(ix.entries)), FileURL: apiBase + "/downloads/" + id + "/file?dt=" + token,
		Subtitles: []model.DownloadSubtitle{},
	}
	for _, e := range ix.entries {
		out.Segments = append(out.Segments, model.DownloadSegment{StartMs: e.StartMs, DurationMs: e.DurationMs, Offset: e.Offset, Length: e.Length})
	}
	for _, t := range tracks.Audio {
		if int64(t.Index) == row.AudioStream {
			out.Audio = &t
			break
		}
	}
	for _, t := range tracks.Subtitles {
		name := strconv.FormatInt(t.ID, 10) + ".vtt"
		if info, err := root.Stat(id + "/" + subsDir + "/" + name); err != nil || info.IsDir() {
			continue
		}
		out.Subtitles = append(out.Subtitles, model.DownloadSubtitle{
			Track: t, URL: apiBase + "/downloads/" + id + "/subtitles/" + name + "?dt=" + token,
		})
	}
	return out, nil
}
