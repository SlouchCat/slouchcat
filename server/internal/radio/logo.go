package radio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // DecodeConfig of GIF logos
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Station logos (concept 16 section 6, build plan S7). The job radio.logo.fetch walks the chain,
// first hit wins: an own upload (never replaced by the job), the catalog favicon, the icons the
// homepage names (apple-touch-icon, icon with sizes, the web manifest's icons, og:image), else
// none (the app shows initials). Every fetch goes through the guarded client. A logo is scaled to
// fit a square with a transparent background, 96, 256 and 512 px wide, in WebP and PNG, below
// <cacheDir>/images/radio/<station>/; logo_hash (sha256 of the source) versions its URL.

const (
	// JobLogoFetch is the job type of the logo chain.
	JobLogoFetch     = "radio.logo.fetch"
	logoPriority     = 0 // the lowest: logos wait for everything else
	logoRetryAfter   = 30 * 24 * time.Hour
	maxLogoBytes     = 10 << 20
	maxLogoHTML      = 1 << 20
	minLogoSize      = 96
	logoStateNone    = "none"
	logoStateCatalog = "catalog"
	logoStateWebsite = "website"
	logoStateOwn     = "own"
	logoStateFailed  = "failed"
)

// logoWidths are the delivered widths; the size names of getRadioImage are w96, w256, w512.
var logoWidths = []int{96, 256, 512}

// logoDir is the cache directory of a station's logo files, relative to the cache directory.
func logoDir(stationID int64) string {
	return filepath.Join("images", "radio", strconv.FormatInt(stationID, 10))
}

func logoFile(width int, f transcode.ImageFormat) string {
	return "logo-w" + strconv.Itoa(width) + "." + string(f)
}

// logoJobType is the job type the wiring registers (RegisterTypes).
func (s *Service) logoJobType() jobs.Type {
	return jobs.Type{Name: JobLogoFetch, Group: jobs.GroupMetadata, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: logoPriority, Handler: s.handleLogo}
}

type logoPayload struct {
	StationID int64 `json:"stationId"`
}

// queueLogos queues, in the order given, the logo job of the stations without a logo (and of those
// whose chain failed more than 30 days ago) and returns how many it queued; it serves the onListed
// hook of lists, favourites and the start page, the sync, the apply of an offer and the own
// stations. The administration's list and image route never call it (B7.2).
func (s *Service) queueLogos(ctx context.Context, stationIDs []int64) int {
	if s.o.queue == nil {
		return 0
	}
	retry := sql.NullInt64{Int64: s.now().Add(-logoRetryAfter).UnixMilli(), Valid: true}
	q := appq.New(s.db.Write)
	var queued []int64
	defer func() { s.onStationsChanged(ctx, queued) }() // one index copy for the whole list
	for _, id := range stationIDs {
		n, err := q.MarkRadioLogoQueued(ctx, appq.MarkRadioLogoQueuedParams{ID: id, RetryBefore: retry})
		if err != nil {
			s.log.WarnContext(ctx, "queue radio logo", "station_id", id, "err", err)
			continue
		}
		if n == 0 {
			continue
		}
		payload, _ := json.Marshal(logoPayload{StationID: id})
		if _, err := s.o.queue.Enqueue(ctx, jobs.NewJob{Type: JobLogoFetch, Priority: logoPriority,
			IdempotencyKey: "radio.logo:" + strconv.FormatInt(id, 10), Payload: payload}); err != nil {
			s.log.WarnContext(ctx, "enqueue radio logo", "station_id", id, "err", err)
			// Without the job the station would stay «queued» for good: the next trigger queues it again.
			if err := q.ResetRadioLogoQueued(context.WithoutCancel(ctx), id); err != nil {
				s.log.WarnContext(ctx, "reset radio logo state", "station_id", id, "err", err)
			}
			continue
		}
		queued = append(queued, id)
	}
	return len(queued)
}

// queueSyncLogos queues the logos the catalog sync prefetches (B7.2): of the stations this run
// founded (created) that are in the offer (new stations of a country with «all»), of the
// favourites of every profile and of the recommended stations, as far as they are in the offer;
// favourites first, then by popularity. Nothing outside the offer is fetched (RF-8).
func (s *Service) queueSyncLogos(ctx context.Context, created []int64) {
	q := appq.New(s.db.Read)
	favs, err := q.ListFavoriteStationIDsOfAllProfiles(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "list favourite radio stations", "err", err)
	}
	rec, err := q.ListRecommendedRadioStationIDs(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "list recommended radio stations", "err", err)
	}
	idx, err := s.index(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "radio index for the logo prefetch", "err", err)
		return
	}
	o, err := s.offer(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "radio offer for the logo prefetch", "err", err)
		return
	}
	var list []*indexEntry
	seen := map[int64]bool{}
	for _, id := range slices.Concat(created, favs, rec) {
		if e := idx.byID[id]; e != nil && !seen[id] && o.offered(e) {
			seen[id] = true
			list = append(list, e)
		}
	}
	logoOrder(list, idSet(favs))
	ids := make([]int64, 0, len(list))
	for _, e := range list {
		ids = append(ids, e.st.ID)
	}
	s.queueLogos(ctx, ids)
}

// removeLogoFiles removes the logo files of stations that are gone (the sync's deletes, a deleted
// own station; B8), through the cache root (rule 4).
func (s *Service) removeLogoFiles(ctx context.Context, stationIDs []int64) {
	if len(stationIDs) == 0 || s.o.cacheDir == "" {
		return
	}
	root, err := paths.NewRoot(s.o.cacheDir)
	if err != nil {
		s.log.WarnContext(ctx, "radio logo cache root", "err", err)
		return
	}
	dir, err := paths.Open(root)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.log.WarnContext(ctx, "open the radio logo cache", "err", err)
		}
		return
	}
	defer dir.Close()
	for _, id := range stationIDs {
		if err := dir.RemoveAll(logoDir(id)); err != nil {
			s.log.WarnContext(ctx, "remove radio logo files", "station_id", id, "err", err)
		}
	}
}

// handleLogo runs the logo chain of one station.
func (s *Service) handleLogo(ctx context.Context, j *jobs.Job) error {
	var p logoPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.StationID <= 0 {
		return fmt.Errorf("radio logo job: bad payload")
	}
	st, err := appq.New(s.db.Read).GetRadioStation(ctx, p.StationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // removed from the catalog meanwhile
	}
	if err != nil {
		return fmt.Errorf("get radio station: %w", err)
	}
	if st.LogoState == logoStateOwn {
		return nil
	}
	// One job per host at a time (B7.2): a taken host sends the job back without an attempt.
	hosts := logoHosts(st.FaviconUrl.String, st.Homepage.String)
	if !s.logo.lockHosts(hosts) {
		return jobs.RetryLater(s.logo.retryDelay(), "another radio logo job fetches from the same host")
	}
	defer s.logo.unlockHosts(hosts)
	src, state := s.fetchLogo(ctx, st)
	hash := sql.NullString{}
	if src != nil {
		h, err := s.renderLogo(ctx, st.ID, src)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.WarnContext(ctx, "render radio logo", "station_id", st.ID, "err", err)
			state = logoStateFailed
		} else {
			hash = sql.NullString{String: h, Valid: true}
		}
	}
	n, err := appq.New(s.db.Write).SetRadioLogoFetched(ctx, appq.SetRadioLogoFetchedParams{
		LogoState: state, LogoHash: hash, Now: s.nowMs(), ID: st.ID})
	if err != nil {
		return fmt.Errorf("set radio logo: %w", err)
	}
	if n > 0 {
		s.onStationChanged(ctx, st.ID)
		return nil
	}
	// The sync may have deleted the station while the chain ran: the files just written would
	// stay behind for good (B8).
	if _, err := appq.New(s.db.Read).GetRadioStation(ctx, st.ID); errors.Is(err, sql.ErrNoRows) {
		s.removeLogoFiles(ctx, []int64{st.ID})
	}
	return nil
}

// fetchLogo walks the chain and returns the first usable source image with its state; nil and
// logoStateFailed when none is usable.
func (s *Service) fetchLogo(ctx context.Context, st appq.RadioStation) ([]byte, string) {
	if st.FaviconUrl.Valid {
		if b := s.logoCandidate(ctx, st.FaviconUrl.String); b != nil {
			return b, logoStateCatalog
		}
	}
	if st.Homepage.Valid {
		for _, u := range s.homepageIcons(ctx, st.Homepage.String) {
			if b := s.logoCandidate(ctx, u); b != nil {
				return b, logoStateWebsite
			}
		}
	}
	return nil, logoStateFailed
}

// logoGet fetches one URL of the chain through the guarded client, at most four requests a
// second for all logo jobs together.
func (s *Service) logoGet(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if u, err := url.Parse(rawURL); err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("no http(s) URL")
	}
	if err := s.logo.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	// A whole request has a deadline: a homepage that never ends (an event stream such as
	// api.zeno.fm/mounts/metadata/subscribe/…) would otherwise hold the worker and the host lock
	// of the job for good, and every other job of that host would wait behind it.
	ctx, cancel := context.WithTimeout(ctx, s.logo.fetchTimeout)
	defer cancel()
	b, _, err := s.netClient().get(ctx, rawURL, limit)
	return b, err
}

// logoCandidate fetches an image and returns it when it is a usable logo.
func (s *Service) logoCandidate(ctx context.Context, rawURL string) []byte {
	b, err := s.logoGet(ctx, rawURL, maxLogoBytes)
	if err != nil {
		s.log.DebugContext(ctx, "radio logo candidate", "site", netguard.Site(rawURL), "err", err)
		return nil
	}
	if w, h, ok := logoSize(b); !ok || w < minLogoSize || h < minLogoSize {
		return nil
	}
	return b
}

// maxManifestIcons bounds the icons read from a web manifest (each one may become a fetch).
const maxManifestIcons = 8

var (
	tagRe  = regexp.MustCompile(`(?is)<(link|meta)\b[^>]*>`)
	attrRe = regexp.MustCompile(`(?is)([a-z][a-z0-9_:-]*)\s*=\s*("[^"]*"|'[^']*'|[^\s"'>]+)`)
)

// homepageIcons reads the homepage and returns its icon URLs, best first: apple-touch-icon, icons
// with sizes (largest first), the icons of the web manifest (largest first), og:image.
func (s *Service) homepageIcons(ctx context.Context, homepage string) []string {
	page, err := s.logoGet(ctx, homepage, maxLogoHTML)
	if err != nil {
		return nil
	}
	base, err := url.Parse(homepage)
	if err != nil {
		return nil
	}
	type icon struct {
		url  string
		rank int // lower first
		size int
	}
	var icons []icon
	add := func(ref string, rank, size int) {
		if u, err := base.Parse(strings.TrimSpace(ref)); err == nil && ref != "" {
			icons = append(icons, icon{u.String(), rank, size})
		}
	}
	var manifest string
	for _, tag := range tagRe.FindAllString(string(page), 200) {
		a := map[string]string{}
		for _, m := range attrRe.FindAllStringSubmatch(tag, 20) {
			a[strings.ToLower(m[1])] = strings.Trim(m[2], `"'`)
		}
		rel := strings.ToLower(a["rel"])
		switch {
		case strings.HasPrefix(strings.ToLower(tag), "<meta"):
			if p := strings.ToLower(a["property"]); p == "og:image" {
				add(a["content"], 3, 0)
			}
		case strings.Contains(rel, "apple-touch-icon"):
			add(a["href"], 0, iconSize(a["sizes"]))
		case strings.Contains(rel, "manifest"):
			manifest = a["href"]
		case strings.Contains(rel, "icon") && a["sizes"] != "":
			add(a["href"], 1, iconSize(a["sizes"]))
		}
	}
	if manifest != "" {
		if u, err := base.Parse(manifest); err == nil {
			if b, err := s.logoGet(ctx, u.String(), maxLogoHTML); err == nil {
				var m struct {
					Icons []struct{ Src, Sizes, Type string } `json:"icons"`
				}
				if json.Unmarshal(b, &m) == nil {
					for _, ic := range m.Icons[:min(len(m.Icons), maxManifestIcons)] {
						if !strings.Contains(ic.Type, "svg") {
							if iu, err := u.Parse(ic.Src); err == nil {
								icons = append(icons, icon{iu.String(), 2, iconSize(ic.Sizes)})
							}
						}
					}
				}
			}
		}
	}
	slices.SortStableFunc(icons, func(a, b icon) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		return b.size - a.size
	})
	out := make([]string, 0, len(icons))
	for _, ic := range icons {
		if !slices.Contains(out, ic.url) {
			out = append(out, ic.url)
		}
	}
	return out
}

// iconSize returns the largest width of a sizes attribute ("32x32 192x192"; "any" counts large).
func iconSize(sizes string) int {
	best := 0
	for f := range strings.FieldsSeq(strings.ToLower(sizes)) {
		if f == "any" {
			return 1 << 16
		}
		w, _, _ := strings.Cut(f, "x")
		if n, err := strconv.Atoi(w); err == nil && n > best {
			best = n
		}
	}
	return best
}

// logoSize reads the dimensions of a logo source by its header: PNG, JPEG and GIF, ICO (the
// largest entry; width 0 means 256), WebP (VP8, VP8L, VP8X). SVG and anything else is not usable.
func logoSize(b []byte) (w, h int, ok bool) {
	switch {
	case len(b) >= 6 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0: // ICO
		n := int(binary.LittleEndian.Uint16(b[4:]))
		for i := range n {
			off := 6 + 16*i
			if off+16 > len(b) {
				break
			}
			ew, eh := int(b[off]), int(b[off+1])
			if ew == 0 {
				ew = 256
			}
			if eh == 0 {
				eh = 256
			}
			if ew*eh > w*h {
				w, h = ew, eh
			}
		}
		return w, h, w > 0
	case len(b) >= 30 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		switch string(b[12:16]) {
		case "VP8 ":
			if b[23] == 0x9d && b[24] == 0x01 && b[25] == 0x2a {
				return int(binary.LittleEndian.Uint16(b[26:]) & 0x3FFF), int(binary.LittleEndian.Uint16(b[28:]) & 0x3FFF), true
			}
		case "VP8L":
			if b[20] == 0x2f {
				v := binary.LittleEndian.Uint32(b[21:])
				return int(v&0x3FFF) + 1, int(v>>14&0x3FFF) + 1, true
			}
		case "VP8X":
			return (int(b[24]) | int(b[25])<<8 | int(b[26])<<16) + 1, (int(b[27]) | int(b[28])<<8 | int(b[29])<<16) + 1, true
		}
		return 0, 0, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || format != "png" && format != "jpeg" && format != "gif" {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// logoDemuxer returns the ffmpeg demuxer of a logo source by its signature (the formats logoSize
// reads), "" for anything else: ffmpeg is told the format instead of probing the file.
func logoDemuxer(b []byte) string {
	switch {
	case len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0:
		return "ico"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp_pipe"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "png_pipe"
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "jpeg_pipe"
	case bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")):
		return "gif"
	}
	return ""
}

// renderLogo writes the square logo files of a station from src and returns the logo hash.
func (s *Service) renderLogo(ctx context.Context, stationID int64, src []byte) (string, error) {
	if s.o.cacheDir == "" {
		return "", errors.New("no cache directory")
	}
	format := logoDemuxer(src)
	if w, h, ok := logoSize(src); !ok || w*h > transcode.MaxImagePixels || format == "" {
		return "", errors.New("not a usable image")
	}
	dir := filepath.Join(s.o.cacheDir, logoDir(stationID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	in := filepath.Join(dir, "source.tmp")
	if err := os.WriteFile(in, src, 0o600); err != nil {
		return "", err
	}
	defer os.Remove(in)
	var outs []transcode.ImageOutput
	for _, w := range logoWidths {
		for _, f := range []transcode.ImageFormat{transcode.ImageWebP, transcode.ImagePNG} {
			outs = append(outs, transcode.ImageOutput{Path: filepath.Join(dir, logoFile(w, f)+".tmp"), Width: w, Format: f})
		}
	}
	if err := s.o.runner.SquareImage(ctx, in, format, outs); err != nil {
		for _, o := range outs {
			_ = os.Remove(o.Path)
		}
		return "", err
	}
	for _, o := range outs {
		if err := os.Rename(o.Path, strings.TrimSuffix(o.Path, ".tmp")); err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:]), nil
}

// setOwnLogo stores an uploaded logo of a station and locks the field logo, so the job never
// replaces it (the admin SetLogo answers the station view after it). Errors: model.ErrNotFound,
// *model.ValidationError on field file (unsupported_format, too_many_pixels).
func (s *Service) setOwnLogo(ctx context.Context, stationID int64, r io.Reader) error {
	if _, err := appq.New(s.db.Read).GetRadioStation(ctx, stationID); errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("get radio station: %w", err)
	}
	src, err := io.ReadAll(io.LimitReader(r, model.MaxImageUploadBytes))
	if err != nil {
		return fmt.Errorf("read logo upload: %w", err)
	}
	w, h, ok := logoSize(src)
	switch {
	case !ok:
		return (&model.ValidationError{}).Add("file", "unsupported_format")
	case w*h > transcode.MaxImagePixels:
		return (&model.ValidationError{}).Add("file", "too_many_pixels")
	}
	hash, err := s.renderLogo(ctx, stationID, src)
	if errors.Is(err, transcode.ErrImageExit) {
		return (&model.ValidationError{}).Add("file", "unsupported_format")
	}
	if err != nil {
		return fmt.Errorf("render logo: %w", err)
	}
	if err := s.writeOwnLogo(ctx, stationID, logoStateOwn, sql.NullString{String: hash, Valid: true}, true); err != nil {
		return err
	}
	s.onStationChanged(ctx, stationID)
	return nil
}

// deleteOwnLogo removes an uploaded logo and lifts the lock; the chain runs again.
func (s *Service) deleteOwnLogo(ctx context.Context, stationID int64) error {
	st, err := appq.New(s.db.Read).GetRadioStation(ctx, stationID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get radio station: %w", err)
	}
	if st.LogoState != logoStateOwn {
		return nil
	}
	if s.o.cacheDir != "" {
		_ = os.RemoveAll(filepath.Join(s.o.cacheDir, logoDir(stationID)))
	}
	if err := s.writeOwnLogo(ctx, stationID, logoStateNone, sql.NullString{}, false); err != nil {
		return err
	}
	s.onStationChanged(ctx, stationID)
	s.queueLogos(ctx, []int64{stationID})
	return nil
}

// writeOwnLogo sets the logo state and the lock logo in one transaction.
func (s *Service) writeOwnLogo(ctx context.Context, stationID int64, state string, hash sql.NullString, lock bool) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	if err := q.SetRadioLogoOwn(ctx, appq.SetRadioLogoOwnParams{LogoState: state, LogoHash: hash, ID: stationID}); err != nil {
		return fmt.Errorf("set radio logo: %w", err)
	}
	if lock {
		err = q.LockRadioStationLogo(ctx, stationID)
	} else {
		err = q.UnlockRadioStationLogo(ctx, stationID)
	}
	if err != nil {
		return fmt.Errorf("radio logo lock: %w", err)
	}
	return tx.Commit()
}

// Image opens a station logo (kind logo; size w96, w256 or w512) as WebP or PNG. Without admin the
// viewer rules apply (stations outside the offer are model.ErrNotFound); no logo is
// model.ErrNotFound as well. It only reads the cache and queues nothing. The caller closes the
// file.
func (s *Service) Image(ctx context.Context, stationID int64, kind, size string, webp, admin bool) (model.StreamSource, error) {
	width, err := strconv.Atoi(strings.TrimPrefix(size, "w"))
	if kind != "logo" || err != nil || !strings.HasPrefix(size, "w") || !slices.Contains(logoWidths, width) {
		return model.StreamSource{}, model.ErrNotFound
	}
	var st appq.RadioStation
	if admin {
		st, err = appq.New(s.db.Read).GetRadioStation(ctx, stationID)
		if errors.Is(err, sql.ErrNoRows) {
			return model.StreamSource{}, model.ErrNotFound
		}
	} else {
		st, _, err = s.listenStation(ctx, stationID)
	}
	if err != nil {
		return model.StreamSource{}, err
	}
	if !st.LogoHash.Valid || s.o.cacheDir == "" {
		return model.StreamSource{}, model.ErrNotFound
	}
	format, mediaType := transcode.ImagePNG, "image/png"
	if webp {
		format, mediaType = transcode.ImageWebP, "image/webp"
	}
	root, err := paths.NewRoot(s.o.cacheDir)
	if err != nil {
		return model.StreamSource{}, err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return model.StreamSource{}, err
	}
	defer dir.Close() // files opened through the root stay usable after it is closed
	f, err := dir.Open(filepath.Join(logoDir(stationID), logoFile(width, format)))
	if errors.Is(err, fs.ErrNotExist) {
		s.log.WarnContext(ctx, "radio logo without file", "station_id", stationID, "size", size)
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat radio logo: %w", err), f.Close())
	}
	return model.StreamSource{File: f, MediaType: mediaType, Size: info.Size(), ModTime: info.ModTime().UTC(),
		Name: "radio/" + strconv.FormatInt(stationID, 10) + "/logo/" + size, ETag: `"` + st.LogoHash.String + "-" + size + "-" + string(format) + `"`}, nil
}
