package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 12 (contract 1.8.0, section 11.4): the image fields of the admin tiles over HTTP. An
// episode shows the poster and the backdrop of its series, a film its own; a title without a
// stored image has no field.

// setAdminImage stores (or replaces) the JPEG row of an image of a title with the content hash.
func setAdminImage(t *testing.T, f *vistest.Fixture, itemID, kind, size, hash string) {
	t.Helper()
	if _, err := f.DB.Write.Exec(`INSERT INTO images (item_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
	    VALUES (?, ?, ?, 'jpeg', 'https://example.org/i.jpg', ?, ?, 'image/jpeg', 200, 300, 1, 1)
	    ON CONFLICT (item_id, kind, size, format) DO UPDATE SET content_hash = excluded.content_hash`,
		itemID, kind, size, kind+"/"+hash+".jpg", hash); err != nil {
		t.Fatal(err)
	}
}

// dropAdminImages removes every stored image of a title.
func dropAdminImages(t *testing.T, f *vistest.Fixture, itemID string) {
	t.Helper()
	if _, err := f.DB.Write.Exec(`DELETE FROM images WHERE item_id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
}

// checkAdminTileFields checks the fields of contract 1.8.0 in api/openapi.yaml (part of
// TestContractOpenAPI): the image fields are optional strings, the grid of the week and its window
// and zone are required, the grid has exactly 672 values and the window at most 90 days.
func checkAdminTileFields(t *testing.T, doc map[string]any) {
	t.Helper()
	schema := func(name string) (props map[string]any, required []any) {
		s, _ := resolve(doc, "#/components/schemas/"+name).(map[string]any)
		props, _ = s["properties"].(map[string]any)
		required, _ = s["required"].([]any)
		return props, required
	}
	for name, fields := range map[string][]string{
		"AdminStream":         {"posterUrl", "backdropUrl"},
		"StatTitle":           {"posterUrl", "backdropUrl"},
		"AdminCollectionItem": {"posterUrl"},
	} {
		props, required := schema(name)
		for _, field := range fields {
			p, _ := props[field].(map[string]any)
			if p == nil || p["type"] != "string" || slices.Contains(required, any(field)) {
				t.Errorf("%s.%s = %v, want an optional string", name, field, p)
			}
		}
	}
	props, required := schema("Stats")
	for _, field := range []string{"watchTimeByQuarterHour", "timeWindowDays", "timeZone"} {
		if props[field] == nil || !slices.Contains(required, any(field)) {
			t.Errorf("Stats.%s missing or optional", field)
		}
	}
	grid, _ := props["watchTimeByQuarterHour"].(map[string]any)
	window, _ := props["timeWindowDays"].(map[string]any)
	if fmt.Sprint(grid["minItems"], grid["maxItems"]) != "672 672" || fmt.Sprint(window["maximum"]) != "90" {
		t.Errorf("grid %v, window %v; want exactly 672 values and at most 90 days", grid, window)
	}
}

func wantAdminImage(itemID, kind, size, hash string) string {
	return BasePath + "/admin/items/" + itemID + "/images/" + kind + "/" + size + "?v=" + hash
}

func TestAdminStreamBackdropURL(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	film := f.Rated(6)
	// The episode itself has images, which are not used; the series has both.
	setAdminImage(t, f, f.Show.E1.ID, "backdrop", model.AdminBackdropSize, "episodeBackdrop")
	setAdminImage(t, f, f.Show.Series.ID, "poster", model.AdminPosterSize, "seriesPoster")
	setAdminImage(t, f, f.Show.Series.ID, "backdrop", model.AdminBackdropSize, "seriesBackdrop")
	dropAdminImages(t, f, film.ID)
	setAdminImage(t, f, film.ID, "backdrop", model.AdminBackdropSize, "filmBackdrop")
	setAdminImage(t, f, film.ID, "backdrop", "w1280", "filmBackdropLarge") // another size is not used
	for _, id := range []string{f.Show.E1.ID, film.ID} {
		if _, code, p := play(t, h, "p18", id, `{}`); code != http.StatusOK {
			t.Fatalf("play %s = %d %+v", id, code, p)
		}
	}
	rec := send(t, h, http.MethodGet, BasePath+"/admin/streams", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	t.Logf("GET /admin/streams: %s", rec.Body.String())
	var list []gen.AdminStream
	decode(t, rec, &list)
	byItem := map[string]gen.AdminStream{}
	for _, st := range list {
		byItem[st.ItemId] = st
	}
	ep, mv := byItem[f.Show.E1.ID], byItem[film.ID]
	if len(list) != 2 || ep.Id == "" || mv.Id == "" {
		t.Fatalf("streams = %+v, want the episode and the film", list)
	}
	if got, want := deref(ep.BackdropUrl), wantAdminImage(f.Show.Series.ID, "backdrop", "w780", "seriesBackdrop"); got != want {
		t.Errorf("episode backdropUrl %q, want the series' %q", got, want)
	}
	if got, want := deref(ep.PosterUrl), wantAdminImage(f.Show.Series.ID, "poster", "w200", "seriesPoster"); got != want {
		t.Errorf("episode posterUrl %q, want %q", got, want)
	}
	if got, want := deref(mv.BackdropUrl), wantAdminImage(film.ID, "backdrop", "w780", "filmBackdrop"); got != want {
		t.Errorf("film backdropUrl %q, want %q", got, want)
	}
	if mv.PosterUrl != nil {
		t.Errorf("film without poster has posterUrl %q", *mv.PosterUrl)
	}
	// The dashboard maps its streams the same way.
	rec = send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
	var d gen.Dashboard
	decode(t, rec, &d)
	for _, st := range d.Streams {
		if st.ItemId == f.Show.E1.ID && deref(st.BackdropUrl) != deref(ep.BackdropUrl) {
			t.Errorf("dashboard stream backdropUrl %q, want %q", deref(st.BackdropUrl), deref(ep.BackdropUrl))
		}
	}
}

func TestStatsTopTitleImages(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	film := f.Rated(6)
	setAdminImage(t, f, f.Show.E1.ID, "poster", model.AdminPosterSize, "episodePoster")
	setAdminImage(t, f, f.Show.Series.ID, "poster", model.AdminPosterSize, "seriesPoster")
	setAdminImage(t, f, f.Show.Series.ID, "backdrop", model.AdminBackdropSize, "seriesBackdrop")
	dropAdminImages(t, f, film.ID)
	setAdminImage(t, f, film.ID, "poster", model.AdminPosterSize, "filmPoster")
	for _, m := range []vistest.Movie{f.Show.E1, f.Show.Special, film} {
		body := fmt.Sprintf(`{"mediaFileId":%q,"positionMs":10000,"durationMs":%d,"playedMs":10000,"seq":1,"state":"playing","clientTime":%q}`,
			m.FileID, vistest.DurationMs, time.Now().UTC().Format(time.RFC3339))
		wantStatus(t, send(t, h, http.MethodPost, BasePath+"/items/"+m.ID+"/progress", "p18", body, nil), http.StatusOK)
	}
	// An episode directly below the series (no season) and an orphaned episode without parents,
	// which shows its own images; both watched for 10 s, written straight into the tables.
	const direct, orphan = "01J9000000000000000000DIRE", "01J9000000000000000000ORPH"
	for id, parent := range map[string]string{direct: f.Show.Series.ID, orphan: ""} {
		now := time.Now().UnixMilli()
		f.AddBareEpisode(t, id, parent)
		if _, err := f.DB.Write.Exec(`INSERT INTO progress_events (profile_id, item_id, position_ms, duration_ms, played_ms, seq, state,
		    client_time, server_time) VALUES (?, ?, 0, 7200000, 10000, 1, 'playing', ?, ?)`, f.P18.ID, id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	setAdminImage(t, f, orphan, "poster", model.AdminPosterSize, "orphanPoster")
	rec := send(t, h, http.MethodGet, BasePath+"/admin/stats?days=30", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	t.Logf("GET /admin/stats?days=30: %s", rec.Body.String())
	var st gen.Stats
	decode(t, rec, &st)
	byItem := map[string]gen.StatTitle{}
	for _, tt := range st.TopTitles {
		byItem[tt.ItemId] = tt
	}
	ep, mv := byItem[f.Show.E1.ID], byItem[film.ID]
	if ep.Kind != gen.ItemKindEpisode || mv.ItemId == "" {
		t.Fatalf("top titles = %+v, want the episode and the film", st.TopTitles)
	}
	if got, want := deref(ep.PosterUrl), wantAdminImage(f.Show.Series.ID, "poster", "w200", "seriesPoster"); got != want {
		t.Errorf("episode posterUrl %q, want the series' %q", got, want)
	}
	if got, want := deref(ep.BackdropUrl), wantAdminImage(f.Show.Series.ID, "backdrop", "w780", "seriesBackdrop"); got != want {
		t.Errorf("episode backdropUrl %q, want the series' %q", got, want)
	}
	if got, want := deref(mv.PosterUrl), wantAdminImage(film.ID, "poster", "w200", "filmPoster"); got != want || mv.BackdropUrl != nil {
		t.Errorf("film posterUrl %q backdropUrl %v, want %q and none", got, mv.BackdropUrl, want)
	}
	// The special (season 0) and the episode without season show the series' images too; the
	// orphan its own poster and no backdrop.
	for label, id := range map[string]string{"special": f.Show.Special.ID, "episode below the series": direct} {
		tt := byItem[id]
		if deref(tt.PosterUrl) != deref(ep.PosterUrl) || deref(tt.BackdropUrl) != deref(ep.BackdropUrl) {
			t.Errorf("%s: posterUrl %q backdropUrl %q, want the series' images", label, deref(tt.PosterUrl), deref(tt.BackdropUrl))
		}
	}
	if o := byItem[orphan]; deref(o.PosterUrl) != wantAdminImage(orphan, "poster", "w200", "orphanPoster") || o.BackdropUrl != nil {
		t.Errorf("orphan: posterUrl %q backdropUrl %v, want its own poster and no backdrop", deref(o.PosterUrl), o.BackdropUrl)
	}
	// The grid of the week: 672 quarter hours holding the five events, 30 days, the zone of the server.
	var sum int64
	for _, v := range st.WatchTimeByQuarterHour {
		sum += v
	}
	if len(st.WatchTimeByQuarterHour) != model.StatsQuarterHours || sum != 50000 || st.TimeWindowDays != 30 || st.TimeZone == "" || st.TimeZone == "Local" {
		t.Errorf("grid of %d values summing to %d, window %d, zone %q; want 672, 50000, 30 and an IANA name",
			len(st.WatchTimeByQuarterHour), sum, st.TimeWindowDays, st.TimeZone)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/stats?days=365", "p18", "", nil)
	decode(t, rec, &st)
	if st.TimeWindowDays != 90 || st.Days != 365 {
		t.Errorf("days 365: window %d, want 90", st.TimeWindowDays)
	}
	if !strings.Contains(rec.Body.String(), `"watchTimeByQuarterHour":[`) {
		t.Errorf("stats lack the grid: %s", rec.Body.String())
	}
}

func TestCollectionItemPosterURL(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	with, without := f.Rated(18), f.Rated(6)
	setAdminImage(t, f, with.ID, "poster", model.AdminPosterSize, "memberPoster")
	dropAdminImages(t, f, without.ID)
	rec := send(t, h, http.MethodPost, BasePath+"/admin/collections", "p18", `{"name":"Bilder"}`, nil)
	wantStatus(t, rec, http.StatusCreated)
	var c gen.AdminCollection
	decode(t, rec, &c)
	rec = send(t, h, http.MethodPut, BasePath+"/admin/collections/"+c.Id+"/items", "p18", `{"itemIds":["`+with.ID+`","`+without.ID+`"]}`, nil)
	wantStatus(t, rec, http.StatusOK)
	rec = send(t, h, http.MethodGet, BasePath+"/admin/collections/"+c.Id, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &c)
	if len(c.Items) != 2 {
		t.Fatalf("members %+v", c.Items)
	}
	if got, want := deref(c.Items[0].PosterUrl), wantAdminImage(with.ID, "poster", "w200", "memberPoster"); got != want {
		t.Errorf("member posterUrl %q, want %q", got, want)
	}
	if c.Items[1].PosterUrl != nil {
		t.Errorf("member without poster has posterUrl %q", *c.Items[1].PosterUrl)
	}
}
