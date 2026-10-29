package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// The mapping list of Fribb (github.com/Fribb/anime-lists, file anime-list-full.json) links
// the ids of TMDB, TVDB and IMDb to AniList ids (ADR 0015, concept 4.4). The repository
// carries no licence file; the list is fetched at run time and cached, never copied into this
// repository (decision F-14). Checked against the file on 29 October 2026: a JSON list of
// objects with "type" (TV, MOVIE, OVA, ONA, SPECIAL, UNKNOWN, or absent), "anilist_id",
// "tvdb_id", "themoviedb_id" as an object {"tv": N} or {"movie": [N, …]}, "imdb_id" as a list
// of strings, "season" as {"tvdb": n, "tmdb": n}; 7.5 MB, with an ETag.
//
// The list is loaded once, kept under <cache>/anime-lists/ with its ETag, renewed weekly with a
// conditional GET, read with a 64 MB brake and streamed into three maps, kept apart by kind
// because TMDB numbers films and series in separate spaces: (movie, tmdb) → anilist,
// imdb → anilist (films only; decision F-15, because only 15 % of the films carry a TMDB id)
// and tvdb → the AniList entries of the series, one per season. The title level of a series
// takes season 1.
const (
	DefaultAnimeListsURL = "https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json"
	animeListsDir        = "anime-lists"
	animeListsFile       = "anime-list-full.json"
	animeListsETagFile   = "anime-list-full.etag"
	maxAnimeListBytes    = 64 << 20
	animeListsTTL        = 7 * 24 * time.Hour
	// animeListsRetryAfter is the pause after a failed download without a cached list.
	animeListsRetryAfter = 5 * time.Minute
)

// errAnimeListTooLarge ends a download that exceeds maxAnimeListBytes.
var errAnimeListTooLarge = fmt.Errorf("anime list larger than %d bytes", maxAnimeListBytes)

// animeSeason is one AniList entry of a TVDB series.
type animeSeason struct {
	AniListID int64
	Season    int // season.tvdb; -1 when the entry has none
}

// animeLists holds the parsed mapping list and renews it.
type animeLists struct {
	url   string
	http  *http.Client
	cache paths.Root
	now   func() time.Time
	log   *slog.Logger

	loadMu       sync.Mutex // one load at a time: workers that arrive together share it
	mu           sync.Mutex
	loadedAt     time.Time
	failedAt     time.Time // last download that failed without a cached list (cooldown)
	failure      error
	movieByTMDB  map[int64]int64
	movieByIMDB  map[string]int64
	seriesByTVDB map[int64][]animeSeason
}

func newAnimeLists(url string, hc *http.Client, cache paths.Root, now func() time.Time, log *slog.Logger) *animeLists {
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	return &animeLists{url: url, http: hc, cache: cache, now: now, log: log}
}

// animeEntry is one object of the list, with the fields this package reads. In the file of
// 29 October 2026 "themoviedb_id" is {"tv": N} for series and {"movie": [N, …]} for films (a
// list, one film entry may name several TMDB films); both are read as a number or a list.
type animeEntry struct {
	Type      string     `json:"type"`
	AniListID int64      `json:"anilist_id"`
	TVDBID    int64      `json:"tvdb_id"`
	IMDB      stringList `json:"imdb_id"`
	TMDB      struct {
		TV    intList `json:"tv"`
		Movie intList `json:"movie"`
	} `json:"themoviedb_id"`
	Season struct {
		TVDB *int `json:"tvdb"`
	} `json:"season"`
}

// intList reads an id field that is a number or a list of numbers; any other shape is empty.
type intList []int64

func (l *intList) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*l = intList{n}
		return nil
	}
	var out []int64
	if err := json.Unmarshal(b, &out); err != nil {
		*l = nil
		return nil
	}
	*l = out
	return nil
}

// stringList reads "imdb_id", which is a list of strings in the file; a single string is
// accepted as well.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = stringList{s}
		return nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		*l = nil
		return nil // another shape: no IMDb id
	}
	*l = out
	return nil
}

// MovieAniListID returns the AniList id of a film by its TMDB id, else by its IMDb id; 0 when
// the list knows neither.
func (a *animeLists) MovieAniListID(ctx context.Context, tmdbID int64, imdbID string) (int64, error) {
	if err := a.ensure(ctx); err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if id := a.movieByTMDB[tmdbID]; id > 0 {
		return id, nil
	}
	if imdbID != "" {
		return a.movieByIMDB[imdbID], nil
	}
	return 0, nil
}

// SeriesAniListID returns the AniList id of the title level of a series by its TVDB id: the
// entry with season 1, else the lowest numbered season, else the first entry; 0 when the list
// does not know the series.
func (a *animeLists) SeriesAniListID(ctx context.Context, tvdbID int64) (int64, error) {
	if err := a.ensure(ctx); err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entries := a.seriesByTVDB[tvdbID]
	if len(entries) == 0 {
		return 0, nil
	}
	best := entries[0]
	for _, e := range entries[1:] {
		switch {
		case e.Season == 1 && best.Season != 1:
			best = e
		case best.Season == 1:
		case e.Season > 0 && (best.Season <= 0 || e.Season < best.Season):
			best = e
		}
	}
	return best.AniListID, nil
}

// ensure loads the maps when they are missing or older than animeListsTTL: from the cached
// file when it is fresh, else from the host with the stored ETag (304 keeps the file). A
// download failure with a cached file keeps the stale list and logs; without one it is an
// error the job retries.
func (a *animeLists) ensure(ctx context.Context) error {
	if a.fresh() {
		return nil
	}
	// Several fill jobs ask at once after a scan: the first loads, the others wait and find
	// the list fresh (the measurement of 29 October 2026 fetched it twice without this).
	a.loadMu.Lock()
	defer a.loadMu.Unlock()
	if a.fresh() {
		return nil
	}
	// A failed download without a cached list is not tried again at once by every waiting
	// job: the fill jobs behind it get the same answer for a while and the host some rest.
	if a.failure != nil && a.now().Sub(a.failedAt) < animeListsRetryAfter {
		return a.failure
	}
	root, err := paths.Open(a.cache)
	if err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	defer root.Close()
	if err := root.MkdirAll(animeListsDir, 0o755); err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	rel := animeListsDir + "/" + animeListsFile
	info, statErr := root.Stat(rel)
	cached := statErr == nil
	if cached && a.now().Sub(info.ModTime()) < animeListsTTL {
		if err := a.parseFile(root, rel); err == nil {
			return nil
		} else if !errors.Is(err, errAnimeListTooLarge) {
			// A cached file that does not parse (cut short, an HTML answer) would block
			// AniList for a week: it goes, the list is fetched again.
			a.log.Warn("cached anime list unreadable, fetching it again", "err", err)
		}
		_ = root.Remove(rel)
		_ = root.Remove(animeListsDir + "/" + animeListsETagFile)
		cached = false
	}
	etag := ""
	if cached {
		if b, err := root.ReadFile(animeListsDir + "/" + animeListsETagFile); err == nil {
			etag = strings.TrimSpace(string(b))
		}
	}
	err = a.download(ctx, root, rel, etag)
	switch {
	case err == nil:
	case cached && !errors.Is(err, context.Canceled):
		a.log.Warn("anime list not renewed, using the cached one", "err", err)
	default:
		if !errors.Is(err, context.Canceled) {
			a.failedAt, a.failure = a.now(), err
		}
		return err
	}
	if err := a.parseFile(root, rel); err != nil {
		a.failedAt, a.failure = a.now(), err
		return err
	}
	a.failure = nil
	return nil
}

// fresh reports whether the maps were loaded within animeListsTTL.
func (a *animeLists) fresh() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.loadedAt.IsZero() && a.now().Sub(a.loadedAt) < animeListsTTL
}

// download fetches the list with If-None-Match. 304 touches the cached file; 200 writes the
// new file and ETag through a temporary name. More than maxAnimeListBytes is an error.
func (a *animeLists) download(ctx context.Context, root *os.Root, rel, etag string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url, nil)
	if err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	logRequest(a.log, "anime-lists", req)
	resp, err := a.http.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: anime lists: %w", model.ErrTMDBUnreachable, err)
	}
	defer resp.Body.Close()
	now := a.now()
	switch {
	case resp.StatusCode == http.StatusNotModified:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		if err := root.Chtimes(rel, now, now); err != nil {
			return fmt.Errorf("anime lists: touch: %w", err)
		}
		return nil
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("%w: anime lists: status %d", model.ErrTMDBUnreachable, resp.StatusCode)
	}
	tmp, err := tempName(animeListsDir)
	if err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxAnimeListBytes+1))
	closeErr := f.Close()
	switch {
	case err != nil:
		_ = root.Remove(tmp)
		return fmt.Errorf("%w: anime lists: read: %w", model.ErrTMDBUnreachable, err)
	case closeErr != nil:
		_ = root.Remove(tmp)
		return fmt.Errorf("anime lists: %w", closeErr)
	case n > maxAnimeListBytes:
		_ = root.Remove(tmp)
		return errAnimeListTooLarge
	}
	if err := root.Rename(tmp, rel); err != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("anime lists: %w", err)
	}
	etagRel := animeListsDir + "/" + animeListsETagFile
	if tag := resp.Header.Get("ETag"); tag != "" {
		if err := root.WriteFile(etagRel, []byte(tag), 0o644); err != nil {
			return fmt.Errorf("anime lists: %w", err)
		}
	} else if err := root.Remove(etagRel); err != nil && !errors.Is(err, os.ErrNotExist) {
		// An old tag beside a new body would ask the host about the wrong version.
		return fmt.Errorf("anime lists: %w", err)
	}
	return nil
}

// parseFile streams the cached file into the maps.
func (a *animeLists) parseFile(root *os.Root, rel string) error {
	f, err := root.Open(rel)
	if err != nil {
		return fmt.Errorf("anime lists: %w", err)
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > maxAnimeListBytes {
		return errAnimeListTooLarge
	}
	movieByTMDB, movieByIMDB, seriesByTVDB, err := parseAnimeList(io.LimitReader(f, maxAnimeListBytes+1))
	var skipped errAnimeListEntriesSkipped
	switch {
	case errors.As(err, &skipped):
		a.log.Warn("anime list: entries of an unexpected shape skipped", "skipped", int(skipped))
	case err != nil:
		return err
	}
	a.mu.Lock()
	a.movieByTMDB, a.movieByIMDB, a.seriesByTVDB, a.loadedAt = movieByTMDB, movieByIMDB, seriesByTVDB, a.now()
	a.mu.Unlock()
	a.log.Info("anime list loaded", "movies_by_tmdb", len(movieByTMDB), "movies_by_imdb", len(movieByIMDB),
		"series_by_tvdb", len(seriesByTVDB))
	return nil
}

// parseAnimeList reads the JSON list entry by entry, never the whole file at once. A film is an
// entry with themoviedb_id.movie or, for the IMDb map, with type MOVIE; a series is an entry with
// tvdb_id. Entries without anilist_id carry nothing for this package.
func parseAnimeList(r io.Reader) (movieByTMDB map[int64]int64, movieByIMDB map[string]int64, seriesByTVDB map[int64][]animeSeason, err error) {
	movieByTMDB, movieByIMDB, seriesByTVDB = map[int64]int64{}, map[string]int64{}, map[int64][]animeSeason{}
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("anime lists: parse: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return nil, nil, nil, errors.New("anime lists: parse: not a JSON list")
	}
	skipped := 0
	for dec.More() {
		var e animeEntry
		if err := dec.Decode(&e); err != nil {
			// A field of another shape than expected (the file changes over time): the decoder
			// has consumed the entry, the entry contributes nothing, the list goes on.
			var typeErr *json.UnmarshalTypeError
			if errors.As(err, &typeErr) {
				skipped++
				continue
			}
			return nil, nil, nil, fmt.Errorf("anime lists: parse: %w", err)
		}
		if e.AniListID <= 0 {
			continue
		}
		for _, id := range e.TMDB.Movie {
			if _, dup := movieByTMDB[id]; id > 0 && !dup {
				movieByTMDB[id] = e.AniListID
			}
		}
		if e.Type == "MOVIE" {
			for _, id := range e.IMDB {
				if id != "" {
					if _, dup := movieByIMDB[id]; !dup {
						movieByIMDB[id] = e.AniListID
					}
				}
			}
		}
		if e.TVDBID > 0 && len(e.TMDB.Movie) == 0 {
			season := -1
			if e.Season.TVDB != nil {
				season = *e.Season.TVDB
			}
			seriesByTVDB[e.TVDBID] = append(seriesByTVDB[e.TVDBID], animeSeason{AniListID: e.AniListID, Season: season})
		}
	}
	if _, err := dec.Token(); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, nil, nil, fmt.Errorf("anime lists: parse: %w", errAnimeListTooLarge)
		}
		return nil, nil, nil, fmt.Errorf("anime lists: parse: %w", err)
	}
	if skipped > 0 {
		return movieByTMDB, movieByIMDB, seriesByTVDB, errAnimeListEntriesSkipped(skipped)
	}
	return movieByTMDB, movieByIMDB, seriesByTVDB, nil
}

// errAnimeListEntriesSkipped tells parseFile how many entries had an unexpected shape; the maps
// are complete apart from those, so the caller logs and goes on.
type errAnimeListEntriesSkipped int

func (e errAnimeListEntriesSkipped) Error() string {
	return fmt.Sprintf("anime lists: %d entries of an unexpected shape skipped", int(e))
}
