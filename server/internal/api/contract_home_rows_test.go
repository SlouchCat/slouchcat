package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/discover"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// homeRowsCheck matches the key list of the CHECK constraint of the rebuilt home_rows table.
var homeRowsCheck = regexp.MustCompile(`(?s)CREATE TABLE home_rows_new \(.*?\bkey\s+TEXT\s+NOT NULL CHECK \(key IN \(([^)]*)\)\)`)

// E-9, ADR 0011, concept 15 P-7: the start page row keys live in three places, the HomeRowKey enum
// of the contract, model.HomeRowKeys and the CHECK of home_rows in migration 00035. All three name
// the same set, so a key cannot be accepted by one layer and refused by another.
func TestHomeRowKeysAgree(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	schema, _ := resolve(parseYAML(t, data), "#/components/schemas/HomeRowKey").(map[string]any)
	values, _ := schema["enum"].([]any)
	var contract []string
	for _, v := range values {
		contract = append(contract, fmt.Sprint(v))
	}

	migration, err := os.ReadFile("../../migrations/00035_home_rows_libraries.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := homeRowsCheck.FindSubmatch(migration)
	if m == nil {
		t.Fatal("migration 00035 has no CHECK (key IN (...)) on home_rows_new")
	}
	var check []string
	for _, k := range strings.Split(string(m[1]), ",") {
		check = append(check, strings.Trim(strings.TrimSpace(k), "'"))
	}

	want := slices.Sorted(slices.Values(model.HomeRowKeys))
	if len(slices.Compact(slices.Clone(want))) != 10 {
		t.Fatalf("model.HomeRowKeys = %v, want ten distinct keys", model.HomeRowKeys)
	}
	for place, got := range map[string][]string{"contract HomeRowKey": contract, "migration 00035 CHECK": check} {
		if got := slices.Sorted(slices.Values(got)); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want the keys of model.HomeRowKeys %v", place, got, want)
		}
	}
}

// Concept 15 P-1 through GET /home and GET /: the row continue holds one entry per series, an
// episode carries ItemRef.seriesRef (contract 1.14.0) with the series' id, title and images, a film
// none; with continue on top the shell preloads and paints the series poster for the episode.
func TestHomeContinueSeriesOverHTTP(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	log := vistest.Logger()
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	now := time.Now().UnixMilli()
	// Two started episodes of the series (E3 newer) and the seeded film rated 12 (started long ago).
	exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, ?, 0, ?), (?, ?, 1000, ?, 0, ?)`,
		f.P18.ID, f.Show.E1.ID, vistest.DurationMs, now-2000, f.P18.ID, f.Show.E3.ID, vistest.DurationMs, now-1000)
	exec(`UPDATE home_rows SET hidden = 1 WHERE key IN ('spotlight', 'picks')`)
	session := model.Session{ID: 1, Account: f.Account, Profile: f.P18, ExpiresAt: time.Now().Add(time.Hour)}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
		Auth: preloadAuth{sessions: map[string]model.Session{"p18": session}}, Catalog: discover.New(f.DB, f.CacheDir, log), Playback: playback.New(f.DB, log),
	})

	type images struct {
		Poster      *struct{ URL string } `json:"poster"`
		PosterSmall *struct{ URL string } `json:"posterSmall"`
		Backdrop    *struct{ URL string } `json:"backdrop"`
	}
	var page struct {
		Rows []struct {
			Key   string `json:"key"`
			Items []struct {
				ID            string `json:"id"`
				Kind          string `json:"kind"`
				SeasonNumber  *int   `json:"seasonNumber"`
				EpisodeNumber *int   `json:"episodeNumber"`
				SeriesTitle   string `json:"seriesTitle"`
				Progress      *struct {
					PositionMs int64 `json:"positionMs"`
					DurationMs int64 `json:"durationMs"`
				} `json:"progress"`
				SeriesRef *struct {
					ID     string `json:"id"`
					Title  string `json:"title"`
					Images images `json:"images"`
				} `json:"seriesRef"`
			} `json:"items"`
		} `json:"rows"`
	}
	rec := getWithToken(h, BasePath+"/home", "p18")
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("home: status %d, err %v, body %s", rec.Code, err, rec.Body.String())
	}
	if len(page.Rows) == 0 || page.Rows[0].Key != "continue" {
		t.Fatalf("rows %+v, want continue first", page.Rows)
	}
	items := page.Rows[0].Items
	if len(items) != 2 || items[0].ID != f.Show.E3.ID || items[1].ID != f.Rated(12).ID {
		t.Fatalf("continue = %+v, want E3 and the film rated 12", items)
	}
	ep, film := items[0], items[1]
	series := "/api/v1/items/" + f.Show.Series.ID + "/images/"
	if s := ep.SeriesRef; s == nil || s.ID != f.Show.Series.ID || s.Title != ep.SeriesTitle || s.Title == "" ||
		s.Images.Poster == nil || !strings.HasPrefix(s.Images.Poster.URL, series+"poster/w400?") ||
		s.Images.PosterSmall == nil || !strings.HasPrefix(s.Images.PosterSmall.URL, series+"poster/w200?") ||
		s.Images.Backdrop == nil || !strings.HasPrefix(s.Images.Backdrop.URL, series+"backdrop/w1280?") {
		t.Errorf("episode seriesRef = %+v, want the series with its images", s)
	}
	if ep.SeasonNumber == nil || *ep.SeasonNumber != 1 || ep.EpisodeNumber == nil || *ep.EpisodeNumber != 3 ||
		ep.Progress == nil || ep.Progress.PositionMs != 1000 || ep.Progress.DurationMs != vistest.DurationMs {
		t.Errorf("episode = %+v, want S1 E3 with its progress", ep)
	}
	if film.SeriesRef != nil {
		t.Errorf("film seriesRef = %+v, want absent", film.SeriesRef)
	}
	if !strings.Contains(rec.Body.String(), `"seriesTitle"`) || strings.Count(rec.Body.String(), `"seriesRef":`) != 1 {
		t.Errorf("body %s: want seriesRef on the episode only", rec.Body.String())
	}

	// The shell paints the series poster as the first card and preloads it.
	shell := getWithToken(h, "/", "p18")
	esc := func(u string) string { return strings.ReplaceAll(u, "&", "&amp;") }
	small, large := ep.SeriesRef.Images.PosterSmall.URL, ep.SeriesRef.Images.Poster.URL
	if body := shell.Body.String(); !strings.Contains(body, `<div id="poster-prepaint" aria-hidden="true"><img src="`+esc(small)+`" srcset="`+esc(small)+" 200w, "+esc(large)+" 400w") {
		t.Errorf("shell %q, want the series poster painted first", body)
	}
	if links := shell.Header().Values("Link"); !slices.ContainsFunc(links, func(l string) bool { return strings.HasPrefix(l, "<"+small+">; rel=preload; as=image") }) {
		t.Errorf("Link = %q, want the series poster preloaded", links)
	}
}

// Concept 15 P-7 through GET /home and GET /: «Meine Medien» carries its tiles (id, name, kind,
// itemCount, the four newest titles with their images) and no items, «Neu in …» its library id and
// name; with «Meine Medien» on top the shell preloads the first tile's first poster and paints the
// collages of the first tiles ahead.
func TestHomeLibraryRowsOverHTTP(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	log := vistest.Logger()
	ctx := context.Background()
	for _, q := range []string{
		`UPDATE home_rows SET hidden = 1`,
		`UPDATE home_rows SET hidden = 0 WHERE key IN ('libraries', 'library')`,
	} {
		if _, err := f.DB.Write.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	session := model.Session{ID: 1, Account: f.Account, Profile: f.P18, ExpiresAt: time.Now().Add(time.Hour)}
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: shellHandler(),
		Auth: preloadAuth{sessions: map[string]model.Session{"p18": session}}, Catalog: discover.New(f.DB, f.CacheDir, log), Playback: playback.New(f.DB, log),
	})

	type item struct {
		ID     string `json:"id"`
		Images struct {
			Poster      *struct{ URL string } `json:"poster"`
			PosterSmall *struct{ URL string } `json:"posterSmall"`
		} `json:"images"`
	}
	var page struct {
		Rows []struct {
			ID          string  `json:"id"`
			Key         string  `json:"key"`
			LibraryID   *int64  `json:"libraryId"`
			LibraryName *string `json:"libraryName"`
			Items       []item  `json:"items"`
			Libraries   []struct {
				ID        int64  `json:"id"`
				Name      string `json:"name"`
				Kind      string `json:"kind"`
				ItemCount int    `json:"itemCount"`
				Items     []item `json:"items"`
			} `json:"libraries"`
		} `json:"rows"`
	}
	rec := getWithToken(h, BasePath+"/home", "p18")
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("home: status %d, err %v, body %s", rec.Code, err, rec.Body.String())
	}
	lib := f.LibraryID()
	if len(page.Rows) != 2 || page.Rows[0].ID != "libraries" || page.Rows[1].ID != fmt.Sprintf("library:%d", lib) {
		t.Fatalf("rows %+v, want «Meine Medien» and «Neu in Filme»", page.Rows)
	}
	tiles, row := page.Rows[0], page.Rows[1]
	if len(tiles.Items) != 0 || len(tiles.Libraries) != 1 || tiles.LibraryID != nil {
		t.Fatalf("«Meine Medien» %+v, want one tile and no items", tiles)
	}
	tile := tiles.Libraries[0]
	if tile.ID != lib || tile.Name != "Filme" || tile.Kind != "video" || tile.ItemCount < len(row.Items) || len(tile.Items) != 4 ||
		tile.Items[0].ID != row.Items[0].ID || tile.Items[0].Images.PosterSmall == nil {
		t.Errorf("tile %+v, want library 1 with its four newest titles", tile)
	}
	if row.Key != "library" || row.LibraryID == nil || *row.LibraryID != lib || row.LibraryName == nil || *row.LibraryName != "Filme" || len(row.Items) == 0 {
		t.Errorf("library row %+v, want library 1 with its name and titles", row)
	}

	shell := getWithToken(h, "/", "p18")
	esc := func(u string) string { return strings.ReplaceAll(u, "&", "&amp;") }
	small := tile.Items[0].Images.PosterSmall.URL
	body := shell.Body.String()
	if !strings.Contains(body, `<div id="poster-prepaint" aria-hidden="true"><span class="tile"><img src="`+esc(small)+`"`) ||
		strings.Count(body, "<img ") != 4 || strings.Count(body, `fetchpriority="high"`) != 1 {
		t.Errorf("shell %q, want the collage of the one tile painted ahead", body)
	}
	if links := shell.Header().Values("Link"); !slices.ContainsFunc(links, func(l string) bool { return strings.HasPrefix(l, "<"+small+">; rel=preload; as=image") }) {
		t.Errorf("Link = %q, want the first tile's first poster preloaded", links)
	}
}
