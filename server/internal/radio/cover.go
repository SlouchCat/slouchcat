package radio

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Covers (concept 16 section 8, F-6, plan A2, D-5, D-6, D-10). A title change of a running relay
// queues a lookup with the in-process worker of the service (not the job queue: up to ten relays
// change titles every few minutes). The chain, first hit wins: the image the stream announced in
// StreamUrl (always, whatever the settings), MusicBrainz with the Cover Art Archive (setting
// cover_musicbrainz), the iTunes search (setting cover_itunes). The result is cached in
// radio_tracks per normalised "artist|title" (or "url:" and the SHA-256 of a stream image), a hit
// 30 days and a miss 24 hours; a network failure or a rate limit writes nothing, so the next title
// change tries again. Images are checked like logos (no SVG, at least 96 px) and scaled to squares
// of 96, 256 and 512 px in WebP and PNG below <cacheDir>/radio/covers/<id % 256>/<id>/. Every
// request goes through the guarded client; logs name hosts only (rule 10) and nothing goes to
// Radio Browser (F-14).

const (
	coverQueueSize = 64
	coverHit       = 30 * 24 * time.Hour
	coverMiss      = 24 * time.Hour
	coverTimeout   = 10 * time.Second
	coverSweep     = 24 * time.Hour
	maxCoverJSON   = 1 << 20
	maxCoverImage  = 10 << 20
	coverScoreMin  = 90 // MusicBrainz search score of an accepted recording
	coverContact   = "(+https://github.com/SlouchCat/slouchcat)"

	trackFound          = "found"
	trackNone           = "none"
	coverSourceStream   = "stream"
	coverSourceMB       = "musicbrainz"
	coverSourceITunes   = "itunes"
	coverRateLimitPause = time.Minute
	// coverMinGap is the least time between two cover lookups of one relay: a station that
	// changes its title every metadata block cannot make the server fetch for it (review M-1).
	coverMinGap = 20 * time.Second
	// coverRender bounds one ffmpeg run that scales a cover.
	coverRender = 30 * time.Second
	// coverMaxFound is the budget of the cache: the found covers over it, oldest first, go in the
	// sweep (about 300 KB of files each).
	coverMaxFound = 5000
	// maxTrackKey is the longest key kept as text; a longer one is "h:" and its SHA-256 (the
	// column allows 600 characters, folding can make a title longer: «ß» becomes «ss»).
	maxTrackKey = 512
)

// coverState is the cover part of the service (lane A).
type coverState struct {
	queue  chan coverJob
	mu     sync.Mutex
	queued map[string]bool // keys in the queue or being looked up
	swept  time.Time       // last sweep of the expired rows

	mb     *ratelimit.Limiter // MusicBrainz and the Cover Art Archive: one request a second
	itunes *ratelimit.Limiter // the iTunes search: 20 requests a minute
	stream *ratelimit.Limiter // the images stations name in StreamUrl: one every 5 s for all
	// minGap is coverMinGap, render coverRender and maxFound coverMaxFound; tests change them.
	minGap   time.Duration
	render   time.Duration
	maxFound int64

	// The services' base URLs; tests point them at httptest servers.
	mbBase, caaBase, itunesBase string
}

func newCoverState() *coverState {
	return &coverState{queue: make(chan coverJob, coverQueueSize), queued: map[string]bool{},
		mb: ratelimit.New("radio-musicbrainz", 1, 1), itunes: ratelimit.New("radio-itunes", 20.0/60, 1),
		stream: ratelimit.New("radio-stream-cover", 0.2, 2), minGap: coverMinGap, render: coverRender, maxFound: coverMaxFound,
		mbBase: "https://musicbrainz.org", caaBase: "https://coverartarchive.org", itunesBase: "https://itunes.apple.com"}
}

// coverJob is one lookup: key is the key of the timeline entries it serves.
type coverJob struct {
	key           string
	image         string // the StreamUrl image address, "" without one
	artist, title string // search terms (noise removed), "" without a split title
}

// coverResult is the outcome of one step of the chain: an image, none, or a transient failure.
type coverResult struct {
	img       []byte
	source    string
	transient bool
}

var (
	// coverNoise is what the search leaves out of a title (concept 16 section 8.1): bracketed
	// additions such as «(Radio Edit)» or «[Live]», and a featured artist.
	coverNoise = regexp.MustCompile(`\s*[(\[][^)\]]*[)\]]`)
	coverFeat  = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.|featuring)\s.*$`)
	coverExt   = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
)

// searchTerm returns the text of an artist or title for the search.
func searchTerm(s string) string {
	return strings.TrimSpace(coverFeat.ReplaceAllString(coverNoise.ReplaceAllString(s, ""), ""))
}

// trackKey is the cache key of a title: the folded "artist|title", or "h:" and its SHA-256 when
// that is longer than maxTrackKey bytes.
func trackKey(artist, title string) string {
	k := rules.Fold(artist) + "|" + rules.Fold(title)
	if len(k) > maxTrackKey {
		sum := sha256.Sum256([]byte(k))
		return "h:" + hex.EncodeToString(sum[:])
	}
	return k
}

// imageURL reports whether a StreamUrl names an http(s) image (by the extension of its path).
func imageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return slices.Contains(coverExt, strings.ToLower(path.Ext(u.Path)))
}

// coverJobFor returns the lookup of a timeline entry; ok is false when it names nothing to look up.
func coverJobFor(e titleEntry) (coverJob, bool) {
	j := coverJob{artist: searchTerm(e.Artist), title: searchTerm(e.Title)}
	if j.artist == "" || j.title == "" {
		j.artist, j.title = "", ""
	}
	switch {
	case imageURL(e.streamURL):
		sum := sha256.Sum256([]byte(e.streamURL))
		j.image, j.key = e.streamURL, "url:"+hex.EncodeToString(sum[:])
	case j.artist != "":
		j.key = trackKey(j.artist, j.title)
	default:
		return coverJob{}, false
	}
	return j, true
}

// queueCover hands a lookup to the worker; a key already queued is not queued twice, and a full
// queue drops the lookup (the next title change of the station asks again).
func (s *Service) queueCover(j coverJob) {
	s.cov.mu.Lock()
	defer s.cov.mu.Unlock()
	if s.cov.queued[j.key] {
		return
	}
	select {
	case s.cov.queue <- j:
		s.cov.queued[j.key] = true
	default:
		s.log.Debug("radio cover queue full, lookup dropped")
	}
}

// runCovers works off the cover queue until ctx ends (started by Run).
func (s *Service) runCovers(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-s.cov.queue:
			s.lookupCover(ctx, j)
		}
	}
}

// lookupCover runs the chain of one job and puts the cover on the timeline entries of its key.
// The key leaves the queued set before the entries are updated: an entry added after that queues
// its own lookup (a cache hit), one added before it is updated here (review: dedupe race).
func (s *Service) lookupCover(ctx context.Context, j coverJob) {
	var id int64
	var hash string
	var err error
	if s.o.cacheDir != "" {
		id, hash, err = s.coverFor(ctx, j)
	}
	s.cov.mu.Lock()
	delete(s.cov.queued, j.key)
	s.cov.mu.Unlock()
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "radio cover lookup", "err", err)
		}
		return
	}
	if id != 0 {
		s.applyCover(j.key, id, hash)
	}
}

// coverFor returns the track id and image hash of the job's cover; id 0 means none (now).
func (s *Service) coverFor(ctx context.Context, j coverJob) (int64, string, error) {
	if j.image != "" {
		id, hash, err := s.cachedCover(ctx, j.key, j.artist, j.title, func() coverResult {
			return s.fetchCoverImage(ctx, s.cov.stream, j.image, coverSourceStream)
		})
		if err != nil || id != 0 || j.artist == "" {
			return id, hash, err
		}
	}
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("get radio config: %w", err)
	}
	mb, it := cfg.CoverMusicbrainz == 1, cfg.CoverItunes == 1
	if !mb && !it {
		return 0, "", nil
	}
	return s.cachedCover(ctx, trackKey(j.artist, j.title), j.artist, j.title, func() coverResult {
		var res coverResult
		if mb {
			if res = s.musicBrainzCover(ctx, j.artist, j.title); res.img != nil {
				return res
			}
		}
		if it {
			r := s.iTunesCover(ctx, j.artist, j.title)
			r.transient = r.transient || res.transient
			return r
		}
		return res
	})
}

// cachedCover answers key from radio_tracks while its row is valid, else runs lookup and stores
// the outcome (nothing on a transient failure).
func (s *Service) cachedCover(ctx context.Context, key, artist, title string, lookup func() coverResult) (int64, string, error) {
	now := s.now()
	row, err := appq.New(s.db.Read).GetRadioTrackByKey(ctx, key)
	switch {
	case err == nil && row.ExpiresAt > now.UnixMilli():
		if row.State == trackFound {
			return row.ID, row.ImageHash.String, nil
		}
		return 0, "", nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return 0, "", fmt.Errorf("get radio track: %w", err)
	}
	had := err == nil && row.State == trackFound
	res := lookup()
	if res.transient || ctx.Err() != nil {
		return 0, "", ctx.Err()
	}
	p := appq.UpsertRadioTrackParams{Key: key, Artist: nullString(artist), Title: nullString(title), State: trackNone,
		Now: now.UnixMilli(), ExpiresAt: now.Add(coverMiss).UnixMilli()}
	var tmp string
	if res.img != nil {
		tmp, err = s.renderCover(ctx, res.img)
		if err != nil {
			s.log.WarnContext(ctx, "render radio cover", "source", res.source, "err", err)
		} else {
			sum := sha256.Sum256(res.img)
			p.State, p.Source = trackFound, sql.NullString{String: res.source, Valid: true}
			p.ImageHash, p.ExpiresAt = sql.NullString{String: hex.EncodeToString(sum[:]), Valid: true}, now.Add(coverHit).UnixMilli()
		}
	}
	id, err := appq.New(s.db.Write).UpsertRadioTrack(ctx, p)
	if err != nil {
		if tmp != "" {
			_ = os.RemoveAll(tmp)
		}
		return 0, "", fmt.Errorf("upsert radio track: %w", err)
	}
	dir := filepath.Join(s.o.cacheDir, coverDir(id))
	if tmp == "" {
		if had {
			_ = os.RemoveAll(dir) // found before, none now
		}
		return 0, "", nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return 0, "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return 0, "", errors.Join(err, os.RemoveAll(tmp))
	}
	return id, p.ImageHash.String, nil
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// applyCover puts a cover on every timeline entry of key of the running relays, also those
// behind live.
func (s *Service) applyCover(key string, id int64, hash string) {
	s.rel.mu.Lock()
	relays := make([]*relay, 0, len(s.rel.relays))
	for _, r := range s.rel.relays {
		relays = append(relays, r)
	}
	s.rel.mu.Unlock()
	for _, r := range relays {
		r.mu.Lock()
		for i := range r.titles {
			if r.titles[i].coverKey == key {
				r.titles[i].CoverTrackID, r.titles[i].CoverHash = id, hash
			}
		}
		r.mu.Unlock()
	}
}

// errCoverTooLarge is the error of an answer over its size cap (not retried).
var errCoverTooLarge = errors.New("answer too large")

// coverAgent is the User-Agent of the cover requests: MusicBrainz asks for a contact.
func (s *Service) coverAgent() string {
	agent := s.o.userAgent
	if agent == "" {
		agent = "Slouchcat"
	}
	return agent + " " + coverContact
}

// coverGet fetches rawURL through the guarded client (redirects re-checked by its dialer) within
// coverTimeout, after a token of l (nil: no limiter). It returns the body of a 2xx answer, else the
// status. A 429 or 503 pauses l. Errors never carry the URL.
func (s *Service) coverGet(ctx context.Context, l *ratelimit.Limiter, rawURL string, limit int64) ([]byte, int, error) {
	if l != nil {
		if err := l.Wait(ctx); err != nil {
			return nil, 0, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, coverTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("unusable address of %s", netguard.Site(rawURL))
	}
	req.Header.Set("User-Agent", s.coverAgent())
	resp, err := s.netClient().http.Do(req)
	if err != nil {
		return nil, 0, netguard.Clean(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		if l != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) {
			l.Backoff(time.Now().Add(coverRateLimitPause))
		}
		return nil, resp.StatusCode, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.StatusCode, netguard.Clean(err)
	}
	if int64(len(b)) > limit {
		return nil, resp.StatusCode, errCoverTooLarge
	}
	return b, resp.StatusCode, nil
}

// failed returns the result of a request that brought nothing: transient for a network failure,
// a rate limit or a server error, and for a search service the guard refused (nothing was learnt
// about the title); none for an image at a refused (local) address, an answer over its cap or a
// client error such as 404.
func (s *Service) failed(ctx context.Context, rawURL string, status int, err error, search bool) coverResult {
	site := netguard.Site(rawURL)
	switch {
	case errors.Is(err, netguard.ErrRefused) && search:
		s.log.InfoContext(ctx, "radio cover search refused", "site", site, "err", err)
		return coverResult{transient: true}
	case errors.Is(err, netguard.ErrRefused), errors.Is(err, errCoverTooLarge):
		s.log.InfoContext(ctx, "radio cover refused", "site", site, "err", err)
		return coverResult{}
	case err != nil:
		s.log.InfoContext(ctx, "radio cover request failed", "site", site, "err", err)
		return coverResult{transient: true}
	case status == http.StatusTooManyRequests || status >= 500:
		s.log.InfoContext(ctx, "radio cover service busy", "site", site, "status", status)
		return coverResult{transient: true}
	}
	return coverResult{}
}

// fetchCoverImage fetches an image of the chain and returns it when it is usable (logoSize: PNG,
// JPEG, GIF, ICO or WebP, at least 96 px each way, at most transcode.MaxImagePixels).
func (s *Service) fetchCoverImage(ctx context.Context, l *ratelimit.Limiter, rawURL, source string) coverResult {
	if u, err := url.Parse(rawURL); err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return coverResult{}
	}
	b, status, err := s.coverGet(ctx, l, rawURL, maxCoverImage)
	if b == nil {
		return s.failed(ctx, rawURL, status, err, false)
	}
	if w, h, ok := logoSize(b); !ok || w < minLogoSize || h < minLogoSize || w*h > transcode.MaxImagePixels || logoDemuxer(b) == "" {
		s.log.InfoContext(ctx, "radio cover image not usable", "site", netguard.Site(rawURL), "source", source)
		return coverResult{}
	}
	return coverResult{img: b, source: source}
}

// luceneQuoted quotes s as a Lucene phrase.
func luceneQuoted(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// mbRelease is a release of a MusicBrainz recording search answer.
type mbRelease struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ReleaseGroup struct {
		ID          string `json:"id"`
		PrimaryType string `json:"primary-type"`
	} `json:"release-group"`
}

// rank orders the releases of a recording: official albums and singles first, then other
// official releases, then the rest.
func (r mbRelease) rank() int {
	switch {
	case r.Status == "Official" && (r.ReleaseGroup.PrimaryType == "Album" || r.ReleaseGroup.PrimaryType == "Single"):
		return 0
	case r.Status == "Official":
		return 1
	}
	return 2
}

// mbRecordings is the part of a MusicBrainz recording search answer the chain reads.
type mbRecordings struct {
	Recordings []struct {
		Score    int         `json:"score"`
		Releases []mbRelease `json:"releases"`
	} `json:"recordings"`
}

// musicBrainzCover searches the recording (score at least 90), takes its best release (mbRelease
// rank) and its front cover of 500 px from the Cover Art Archive, else the one of its release
// group.
func (s *Service) musicBrainzCover(ctx context.Context, artist, title string) coverResult {
	q := url.Values{"query": {"artist:" + luceneQuoted(artist) + " AND recording:" + luceneQuoted(title)},
		"fmt": {"json"}, "limit": {"5"}}
	u := s.cov.mbBase + "/ws/2/recording?" + q.Encode()
	b, status, err := s.coverGet(ctx, s.cov.mb, u, maxCoverJSON)
	if b == nil {
		return s.failed(ctx, u, status, err, true)
	}
	var res mbRecordings
	if err := json.Unmarshal(b, &res); err != nil {
		s.log.InfoContext(ctx, "radio cover search answer unreadable", "site", netguard.Site(u), "err", err)
		return coverResult{}
	}
	var best *mbRelease
	for _, rec := range res.Recordings {
		if rec.Score < coverScoreMin || len(rec.Releases) == 0 {
			continue
		}
		rel := slices.MinFunc(rec.Releases, func(a, b mbRelease) int { return a.rank() - b.rank() })
		best = &rel
		break
	}
	if best == nil {
		return coverResult{}
	}
	var refs []string
	if best.ID != "" {
		refs = append(refs, "release/"+url.PathEscape(best.ID))
	}
	if best.ReleaseGroup.ID != "" {
		refs = append(refs, "release-group/"+url.PathEscape(best.ReleaseGroup.ID))
	}
	transient := false
	for _, ref := range refs {
		r := s.fetchCoverImage(ctx, s.cov.mb, s.cov.caaBase+"/"+ref+"/front-500", coverSourceMB)
		if r.img != nil {
			return r
		}
		transient = transient || r.transient
	}
	return coverResult{transient: transient}
}

// iTunesResults is the part of an iTunes search answer the chain reads.
type iTunesResults struct {
	Results []struct {
		ArtistName    string `json:"artistName"`
		ArtworkURL100 string `json:"artworkUrl100"`
	} `json:"results"`
}

// iTunesCover searches the song in the store of the server's country and takes the artwork of the
// first result by the same artist, at 600 px.
func (s *Service) iTunesCover(ctx context.Context, artist, title string) coverResult {
	q := url.Values{"term": {artist + " " + title}, "media": {"music"}, "entity": {"song"}, "limit": {"5"}}
	if c, err := s.defaultCountry(ctx); err == nil && c != "" {
		q.Set("country", strings.ToLower(c))
	}
	u := s.cov.itunesBase + "/search?" + q.Encode()
	b, status, err := s.coverGet(ctx, s.cov.itunes, u, maxCoverJSON)
	if b == nil {
		return s.failed(ctx, u, status, err, true)
	}
	var res iTunesResults
	if err := json.Unmarshal(b, &res); err != nil {
		s.log.InfoContext(ctx, "radio cover search answer unreadable", "site", netguard.Site(u), "err", err)
		return coverResult{}
	}
	want := rules.Fold(artist)
	for _, r := range res.Results {
		if r.ArtworkURL100 != "" && rules.Fold(searchTerm(r.ArtistName)) == want {
			return s.fetchCoverImage(ctx, nil, strings.Replace(r.ArtworkURL100, "100x100bb", "600x600bb", 1), coverSourceITunes)
		}
	}
	return coverResult{}
}

// coverDir is the directory of a track's cover files, relative to the cache directory.
func coverDir(id int64) string {
	return filepath.Join("radio", "covers", strconv.FormatInt(id%256, 10), strconv.FormatInt(id, 10))
}

func coverFile(width int, f transcode.ImageFormat) string {
	return "cover-w" + strconv.Itoa(width) + "." + string(f)
}

// renderCover writes the square cover files of src into a new directory below the covers and
// returns it; the caller moves it to the track's directory.
func (s *Service) renderCover(ctx context.Context, src []byte) (string, error) {
	format := logoDemuxer(src)
	if format == "" {
		return "", errors.New("not a usable image")
	}
	dir := filepath.Join(s.o.cacheDir, "radio", "covers", "new-"+ulid.Make().String())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	in := filepath.Join(dir, "source.tmp")
	if err := os.WriteFile(in, src, 0o600); err != nil {
		return "", errors.Join(err, os.RemoveAll(dir))
	}
	var outs []transcode.ImageOutput
	for _, w := range logoWidths {
		for _, f := range []transcode.ImageFormat{transcode.ImageWebP, transcode.ImagePNG} {
			outs = append(outs, transcode.ImageOutput{Path: filepath.Join(dir, coverFile(w, f)), Width: w, Format: f})
		}
	}
	// The square scaler names the demuxer (no probing) and reads only the local file, like logos.
	rctx, cancel := context.WithTimeout(ctx, s.cov.render)
	defer cancel()
	if err := s.o.runner.SquareImage(rctx, in, format, outs); err != nil {
		return "", errors.Join(err, os.RemoveAll(dir))
	}
	if err := os.Remove(in); err != nil {
		return "", errors.Join(err, os.RemoveAll(dir))
	}
	return dir, nil
}

// sweepCovers deletes the expired rows of radio_tracks and the files of their covers, once a day
// (sweep).
func (s *Service) sweepCovers(ctx context.Context, now time.Time) {
	s.cov.mu.Lock()
	due := now.Sub(s.cov.swept) >= coverSweep
	if due {
		s.cov.swept = now
	}
	s.cov.mu.Unlock()
	if !due {
		return
	}
	rows, err := appq.New(s.db.Write).DeleteExpiredRadioTracks(ctx, now.UnixMilli())
	if err != nil {
		s.log.WarnContext(ctx, "delete expired radio tracks", "err", err)
		return
	}
	if s.o.cacheDir == "" {
		return
	}
	for _, r := range rows {
		if r.ImageHash.Valid {
			_ = os.RemoveAll(filepath.Join(s.o.cacheDir, coverDir(r.ID)))
		}
	}
	s.trimCovers(ctx)
	// Renders a stopped process left behind (the one worker finishes its render within minutes).
	news, _ := filepath.Glob(filepath.Join(s.o.cacheDir, "radio", "covers", "new-*"))
	for _, d := range news {
		if info, err := os.Stat(d); err == nil && now.Sub(info.ModTime()) > time.Hour {
			_ = os.RemoveAll(d)
		}
	}
}

// trimCovers keeps the cache within its budget: the found rows over maxFound go, the oldest
// checks first, with their files (review M-1).
func (s *Service) trimCovers(ctx context.Context) {
	q := appq.New(s.db.Write)
	n, err := q.CountFoundRadioTracks(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "count radio covers", "err", err)
		return
	}
	if n <= s.cov.maxFound {
		return
	}
	ids, err := q.DeleteOldestFoundRadioTracks(ctx, n-s.cov.maxFound)
	if err != nil {
		s.log.WarnContext(ctx, "trim radio covers", "err", err)
		return
	}
	for _, id := range ids {
		_ = os.RemoveAll(filepath.Join(s.o.cacheDir, coverDir(id)))
	}
	s.log.InfoContext(ctx, "radio covers over the budget removed", "count", len(ids))
}

// Cover opens the cover of a track (size w96, w256 or w512) as WebP or PNG; hash is the v of the
// URL and must be the stored image hash. An unknown or expired track, one without a cover or
// another hash is model.ErrNotFound. The caller closes the file.
func (s *Service) Cover(ctx context.Context, trackID int64, size, hash string, webp bool) (model.StreamSource, error) {
	width, err := strconv.Atoi(strings.TrimPrefix(size, "w"))
	if err != nil || !strings.HasPrefix(size, "w") || !slices.Contains(logoWidths, width) {
		return model.StreamSource{}, model.ErrNotFound
	}
	row, err := appq.New(s.db.Read).GetRadioTrack(ctx, trackID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("get radio track: %w", err)
	}
	if row.State != trackFound || row.ExpiresAt <= s.now().UnixMilli() || s.o.cacheDir == "" || hash != row.ImageHash.String {
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
	f, err := dir.Open(filepath.Join(coverDir(trackID), coverFile(width, format)))
	if errors.Is(err, fs.ErrNotExist) {
		s.log.WarnContext(ctx, "radio cover without file", "track_id", trackID, "size", size)
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat radio cover: %w", err), f.Close())
	}
	return model.StreamSource{File: f, MediaType: mediaType, Size: info.Size(), ModTime: info.ModTime().UTC(),
		Name: "radio/covers/" + strconv.FormatInt(trackID, 10) + "/" + size,
		ETag: `"` + row.ImageHash.String + "-" + size + "-" + string(format) + `"`}, nil
}
