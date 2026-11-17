package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// externalRow reads the stored columns of an external title the tests look at.
type externalRow struct {
	minAge                   *int64
	poster, backdrop, logo   *string
	certs, genres, trailers  string
	seasons                  string
	lastEpisode, nextEpisode *string
	status                   *string
	fetchedAt                int64
}

func (e *env) externalRow(t *testing.T, media string, id int64) (externalRow, bool) {
	t.Helper()
	var r externalRow
	err := e.db.Read.QueryRow(`SELECT min_age, poster_path, backdrop_path, logo_path, certifications, genres, trailers,
		seasons, last_episode, next_episode, status, fetched_at FROM external_titles WHERE media = ? AND tmdb_id = ?`, media, id).
		Scan(&r.minAge, &r.poster, &r.backdrop, &r.logo, &r.certs, &r.genres, &r.trailers, &r.seasons, &r.lastEpisode,
			&r.nextEpisode, &r.status, &r.fetchedAt)
	if err != nil {
		return externalRow{}, false
	}
	return r, true
}

// A film outside the library: texts in both text languages, genre names per language, the IMDb id,
// the trailers of the TMDB step (one per language and one without), the age of the first country
// with a known certificate. A second Ensure reads the stored row without asking TMDB.
func TestExternalTitleMovie(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	got, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, 550)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinAge == nil || *got.MinAge != 12 {
		t.Errorf("min age = %v, want 12 (DE before US)", deref(got.MinAge))
	}
	if got.Texts["de"].Title != "Erster Film" || got.Texts["de"].Overview != "Deutsche Beschreibung" || got.Texts["en"].Title != "First Film" ||
		got.Texts["en"].Tagline != "English tagline" {
		t.Errorf("texts = %+v", got.Texts)
	}
	if got.IMDBID != "tt0000550" || got.Year != 2001 || got.ReleaseDate != "2001-03-04" || got.RuntimeMinutes != 117 || got.OriginalTitle != "First Film (Original)" {
		t.Errorf("facts = %+v", got)
	}
	if len(got.Genres) != 2 || got.Genres[1].Names["de"] != "Komödie" || got.Genres[1].Names["en"] != "Comedy" {
		t.Errorf("genres = %+v", got.Genres)
	}
	if got.Certifications["DE"] != "12" || got.Certifications["US"] != "R" {
		t.Errorf("certifications = %v", got.Certifications)
	}
	keys := map[string]string{}
	for _, tr := range got.Trailers {
		keys[tr.Lang] = tr.Key
	}
	if keys["de"] != "deOfficial1" || keys["en"] != "enOfficial1" || keys[""] != "noLang00001" || len(keys) != 3 {
		t.Errorf("trailers = %+v", got.Trailers)
	}
	if !slices.Equal(got.ImageKinds, []string{"backdrop", "poster"}) {
		t.Errorf("image kinds = %v, want backdrop and poster", got.ImageKinds)
	}
	if len(got.Seasons) != 0 || got.LastEpisode != nil {
		t.Errorf("a film with seasons %+v or a last episode %+v", got.Seasons, got.LastEpisode)
	}
	requests := e.tmdb.count("/3/movie/550")
	if requests != 2 {
		t.Errorf("detail requests = %d, want 2 (de-DE and en-US)", requests)
	}
	again, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, 550)
	if err != nil || again.Texts["de"].Title != "Erster Film" {
		t.Fatalf("second ensure = %+v, %v", again, err)
	}
	if n := e.tmdb.count("/3/movie/550"); n != requests {
		t.Errorf("second ensure asked TMDB again: %d requests, want %d", n, requests)
	}
	if _, err := e.svc.ExternalTitle(ctx, MediaMovie, 601); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("title never fetched = %v, want ErrNotFound", err)
	}
}

// min_age follows the country order of ADR 0034: the rating country first, then DE, AT, CH, GB,
// US; a title without any known certificate stays unknown (NULL). A change of the rating country
// computes the stored ages again without asking TMDB.
func TestExternalTitleMinAgeFollowsCountryOrder(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	for _, id := range []int64{550, 601, 602} {
		if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, id); err != nil {
			t.Fatal(err)
		}
	}
	age := func(id int64) *int64 {
		r, ok := e.externalRow(t, MediaMovie, id)
		if !ok {
			t.Fatalf("no row for %d", id)
		}
		return r.minAge
	}
	want := func(id int64, w *int64) {
		t.Helper()
		got := age(id)
		if (got == nil) != (w == nil) || (got != nil && *got != *w) {
			t.Errorf("min_age of %d = %v, want %v", id, nullAge(got), nullAge(w))
		}
	}
	i64 := func(v int64) *int64 { return &v }
	want(550, i64(12)) // DE 12 before US R
	want(601, i64(13)) // only US PG-13: the last country of the order
	want(602, nil)     // no certificate: unknown
	requests := e.tmdb.total()
	e.setRatingCountry(t, "US") // through UpdateMetadataSettings, as the administration saves it
	want(550, i64(17))          // the rating country US first: R
	want(601, i64(13))
	want(602, nil)
	if e.tmdb.total() != requests {
		t.Errorf("recomputing the ages asked TMDB")
	}
	// A title fetched while the rating country is US takes the US age at once.
	mustExec(t, e.db.Write, `UPDATE external_titles SET min_age = NULL WHERE tmdb_id = 550`)
	got, err := e.svc.RefreshExternalTitle(ctx, MediaMovie, 550)
	if err != nil || got.MinAge == nil || *got.MinAge != 17 {
		t.Errorf("refresh with rating country US = %v, %v, want 17", deref(got.MinAge), err)
	}
}

func nullAge(p *int64) any {
	if p == nil {
		return "NULL"
	}
	return *p
}

// A series: the content ratings give the age, the external ids the TVDB and IMDb id, the seasons
// keep their episode counts and air dates, the last and the next episode and the status are stored.
func TestExternalTitleShow(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.mux.HandleFunc("GET /3/tv/901", func(w http.ResponseWriter, r *http.Request) {
		name := "Fehlende Serie"
		if r.URL.Query().Get("language") == langEN {
			name = "Missing Show"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 901, "name": name, "original_name": "Missing Show", "overview": "x", "first_air_date": "2020-02-03",
			"episode_run_time": []int{50}, "status": "Returning Series", "poster_path": "/tvposter901.jpg",
			"genres": []any{map[string]any{"id": 10765, "name": "Sci-Fi & Fantasy"}},
			"seasons": []any{
				map[string]any{"season_number": 0, "episode_count": 2, "air_date": "2019-12-01"},
				map[string]any{"season_number": 1, "episode_count": 8, "air_date": "2020-02-03"},
				map[string]any{"season_number": 2, "episode_count": 10, "air_date": "not a date"},
			},
			"last_episode_to_air": map[string]any{"season_number": 2, "episode_number": 4, "air_date": "2026-11-10"},
			"next_episode_to_air": map[string]any{"season_number": 2, "episode_number": 5, "air_date": "2026-11-24"},
			"content_ratings":     map[string]any{"results": []any{map[string]any{"iso_3166_1": "GB", "rating": "15"}, map[string]any{"iso_3166_1": "US", "rating": "TV-MA"}}},
			"external_ids":        map[string]any{"imdb_id": "tt0000901", "tvdb_id": 424243},
		})
	})
	got, err := e.svc.EnsureExternalTitle(ctx, MediaShow, 901)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinAge == nil || *got.MinAge != 15 {
		t.Errorf("min age = %v, want 15 (GB before US)", deref(got.MinAge))
	}
	if got.TVDBID != 424243 || got.IMDBID != "tt0000901" || got.Status != "Returning Series" || got.RuntimeMinutes != 50 || got.Year != 2020 {
		t.Errorf("facts = %+v", got)
	}
	if got.Texts["de"].Title != "Fehlende Serie" || got.Texts["en"].Title != "Missing Show" {
		t.Errorf("texts = %+v", got.Texts)
	}
	wantSeasons := []ExternalSeason{{0, 2, "2019-12-01"}, {1, 8, "2020-02-03"}, {2, 10, ""}}
	if !slices.Equal(got.Seasons, wantSeasons) {
		t.Errorf("seasons = %+v, want %+v", got.Seasons, wantSeasons)
	}
	if got.LastEpisode == nil || *got.LastEpisode != (ExternalEpisode{2, 4, "2026-11-10"}) ||
		got.NextEpisode == nil || *got.NextEpisode != (ExternalEpisode{2, 5, "2026-11-24"}) {
		t.Errorf("last %+v, next %+v", got.LastEpisode, got.NextEpisode)
	}
	if !slices.Equal(got.ImageKinds, []string{"poster"}) {
		t.Errorf("image kinds = %v", got.ImageKinds)
	}
	// The same TMDB id as a film is another title (TMDB numbers films and series apart).
	if _, err := e.svc.ExternalTitle(ctx, MediaMovie, 901); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("film 901 = %v, want ErrNotFound", err)
	}
}

// The images are fetched on their first request and stored below the cache root in every size up
// to the original's width, as JPEG and WebP; the next request reads the stored file. An unknown
// size, a kind without TMDB image and a title without row are not found.
func TestExternalTitleImagesStoredLocally(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, 550); err != nil {
		t.Fatal(err)
	}
	open := func(kind, size string, webp bool) (model.StreamSource, []byte, error) {
		t.Helper()
		src, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 550, kind, size, webp)
		if err != nil {
			return src, nil, err
		}
		defer src.File.Close()
		data, err := io.ReadAll(src.File)
		return src, data, err
	}
	src, data, err := open("poster", "w400", true)
	if err != nil {
		t.Fatal(err)
	}
	if src.MediaType != "image/webp" || len(data) == 0 || src.Size != int64(len(data)) || src.ETag == "" {
		t.Errorf("poster w400 webp = %s, %d bytes (size %d), etag %q", src.MediaType, len(data), src.Size, src.ETag)
	}
	if _, err := os.Stat(filepath.Join(e.cache, "titles", "movie", "550", "poster-w400.webp")); err != nil {
		t.Errorf("stored file: %v", err)
	}
	var rows int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM external_title_images WHERE media = 'movie' AND tmdb_id = 550 AND kind = 'poster'
		AND size IN ('w200', 'w400', 'w800') AND format IN ('jpeg', 'webp') AND source_url LIKE '%/original/poster550.jpg'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 6 {
		t.Errorf("poster rows = %d, want 6 (three sizes, two formats)", rows)
	}
	if src, _, err := open("poster", "w800", false); err != nil || src.MediaType != "image/jpeg" {
		t.Errorf("poster w800 jpeg = %s, %v", src.MediaType, err)
	}
	if n := e.tmdb.count("/t/p/original/poster550.jpg"); n != 1 {
		t.Errorf("poster downloads = %d, want 1", n)
	}
	for _, c := range []struct{ kind, size string }{{"poster", "w500"}, {"still", "w400"}, {"logo", "w400"}} {
		if _, _, err := open(c.kind, c.size, true); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s %s = %v, want ErrNotFound", c.kind, c.size, err)
		}
	}
	if _, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 601, "poster", "w400", true); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("image of a title without row = %v, want ErrNotFound", err)
	}
	// A refresh that finds another poster drops the stored sizes; the next request fetches the new one.
	e.tmdb.mu.Lock()
	e.tmdb.movies["550"].de["poster_path"], e.tmdb.movies["550"].en["poster_path"] = "/poster601.jpg", "/poster601.jpg"
	e.tmdb.mu.Unlock()
	if _, err := e.svc.RefreshExternalTitle(ctx, MediaMovie, 550); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.cache, "titles", "movie", "550", "poster-w400.webp")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file of the old poster after the refresh: %v, want gone", err)
	}
	if _, _, err := open("poster", "w400", true); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/t/p/original/poster601.jpg"); n != 1 {
		t.Errorf("downloads of the new poster = %d, want 1", n)
	}
}

// A title TMDB does not know is not stored, and the next Ensure within the hour does not ask again.
func TestExternalTitleUnknownAtTMDB(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	for range 3 {
		if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, 4040); !errors.Is(err, model.ErrNotFound) {
			t.Fatalf("ensure of an unknown title = %v, want ErrNotFound", err)
		}
	}
	if n := e.tmdb.count("/3/movie/4040"); n != 1 {
		t.Errorf("requests for the unknown title = %d, want 1", n)
	}
	if _, ok := e.externalRow(t, MediaMovie, 4040); ok {
		t.Error("an unknown title was stored")
	}
	if _, err := e.svc.EnsureExternalTitle(ctx, "series", 550); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown media = %v, want ErrNotFound", err)
	}
	if _, err := newEnv(t, "").svc.EnsureExternalTitle(ctx, MediaMovie, 550); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Errorf("ensure without TMDB key = %v, want ErrProviderNotConfigured", err)
	}
}

// titles.refresh fetches only the rows older than seven days; a title that vanished at TMDB keeps
// its row, the run succeeds and the next run does not ask for it again (no retry storm).
func TestExternalTitleRefreshJob(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	for _, id := range []int64{550, 601} {
		if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, id); err != nil {
			t.Fatal(err)
		}
	}
	stale := testNow.Add(-ExternalTitleMaxAge - time.Hour).UnixMilli()
	fresh := testNow.Add(-ExternalTitleMaxAge + time.Hour).UnixMilli()
	mustExec(t, e.db.Write, `UPDATE external_titles SET fetched_at = ? WHERE tmdb_id = 550`, stale)
	mustExec(t, e.db.Write, `UPDATE external_titles SET fetched_at = ? WHERE tmdb_id = 601`, fresh)
	// Film 777 was stored once and has vanished at TMDB since.
	mustExec(t, e.db.Write, `INSERT INTO external_titles (media, tmdb_id, min_age, fetched_at) VALUES ('movie', 777, 6, ?)`, stale)
	mustExec(t, e.db.Write, `INSERT INTO external_title_texts (media, tmdb_id, language, title) VALUES ('movie', 777, 'de', 'Verschwunden')`)
	before550, before601 := e.tmdb.count("/3/movie/550"), e.tmdb.count("/3/movie/601")
	run := func() {
		t.Helper()
		if err := e.svc.handleTitlesRefresh(ctx, &jobs.Job{Type: JobTitlesRefresh}); err != nil {
			t.Fatalf("titles.refresh: %v", err)
		}
	}
	run()
	if n := e.tmdb.count("/3/movie/550") - before550; n != 2 {
		t.Errorf("requests for the stale film = %d, want 2", n)
	}
	if n := e.tmdb.count("/3/movie/601") - before601; n != 0 {
		t.Errorf("requests for the fresh film = %d, want 0", n)
	}
	gone, ok := e.externalRow(t, MediaMovie, 777)
	if !ok || gone.fetchedAt != testNow.UnixMilli() || gone.minAge == nil || *gone.minAge != 6 {
		t.Errorf("vanished film = %+v, %v; want its row kept with fetched_at moved to now", gone, ok)
	}
	if r, _ := e.externalRow(t, MediaMovie, 550); r.fetchedAt != testNow.UnixMilli() {
		t.Errorf("fetched_at of the refreshed film = %d, want %d", r.fetchedAt, testNow.UnixMilli())
	}
	vanished := e.tmdb.count("/3/movie/777")
	if vanished != 1 {
		t.Errorf("requests for the vanished film = %d, want 1", vanished)
	}
	run()
	if n := e.tmdb.count("/3/movie/777"); n != vanished {
		t.Errorf("second run asked for the vanished film again: %d requests", n)
	}
	// The schedule queues one job under its key at the priority it is given.
	ids, err := e.svc.runTitlesRefresh(ctx, e.queue, titlesRefreshPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run = %v, %v", ids, err)
	}
	var prio int
	var key string
	if err := e.db.Read.QueryRow(`SELECT priority, idempotency_key FROM jobs WHERE id = ?`, ids[0]).Scan(&prio, &key); err != nil {
		t.Fatal(err)
	}
	if prio != 7 || key != titlesRefreshKey {
		t.Errorf("queued job priority %d key %q, want 7 and %q", prio, key, titlesRefreshKey)
	}
}

// titles.refresh without a TMDB key does nothing and does not fail; with a key TMDB rejects it
// fails, so the queue retries after its backoff.
func TestExternalTitleRefreshJobWithoutKey(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	mustExec(t, e.db.Write, `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 550, 0)`)
	if err := e.svc.handleTitlesRefresh(ctx, &jobs.Job{Type: JobTitlesRefresh}); err != nil {
		t.Errorf("titles.refresh without key: %v", err)
	}
	if n := e.tmdb.total(); n != 0 {
		t.Errorf("requests without key = %d", n)
	}
	bad := newEnv(t, "rejected-token")
	mustExec(t, bad.db.Write, `INSERT INTO external_titles (media, tmdb_id, fetched_at) VALUES ('movie', 550, 0)`)
	if err := bad.svc.handleTitlesRefresh(ctx, &jobs.Job{Type: JobTitlesRefresh}); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Errorf("titles.refresh with a rejected key = %v, want ErrTMDBKeyInvalid", err)
	}
}
