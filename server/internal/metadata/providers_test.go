package metadata

import (
	"context"
	"testing"
)

// Tests of the provider list of a title (ADR 0015): the first entry identifies the series and
// numbers its seasons, TVDB first fetches the tmdb_id on the side, and the default list behaves
// as the code did before.

// setProviders writes the provider list of the test series and its tree.
func (e *env) setProviders(t *testing.T, list string) {
	t.Helper()
	mustExec(t, e.db.Write, `UPDATE items SET providers = ? WHERE library_id = 2`, list)
}

// TestMatchSeriesTVDBFirst: with ["tvdb","tmdb"] TVDB identifies the series without a TMDB
// search, the seasons and episodes follow TVDB's numbering and texts, and the tmdb_id arrives
// through TMDB's external-id lookup so credits, trailers and images keep their key.
func TestMatchSeriesTVDBFirst(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	e.addNode(t, node{id: idSeason1, kind: "season", parent: idSeries, title: "Staffel 1", season: 1})
	e.addNode(t, node{id: idE0101, kind: "episode", parent: idSeason1, title: "Folge 1", season: 1, episode: 1})
	e.setProviders(t, `["tvdb","tmdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/search/tv"); n != 0 {
		t.Fatalf("%d TMDB searches, want none with TVDB first", n)
	}
	if n := e.tmdb.count("/3/find/424242"); n != 1 {
		t.Fatalf("%d lookups of the tmdb_id by tvdb_id, want 1", n)
	}
	s := e.child(t, idSeries)
	if s.State != "matched" || s.TVDB.Int64 != 424242 || s.TMDB.Int64 != 900 {
		t.Fatalf("series %+v, want tvdb 424242 and tmdb 900", s)
	}
	if tx := e.texts(t, idSeries); tx["de"].Title != "Nur bei TVDB" {
		t.Fatalf("texts %+v, want the TVDB title", tx)
	}
	art := e.tvdb.srv.URL + "/art/banners/"
	if r := e.child(t, idE0101); r.State != "matched" || r.Date.String != "2015-02-03" || r.Backdrop.String != art+"episodes/424242/1.jpg" {
		t.Fatalf("episode 1 %+v, want TVDB's episode", r)
	}
	// seriesProvider follows the list: a later episode is described by TVDB, not by TMDB,
	// although the series carries both ids now.
	e.addNode(t, node{id: idE0102, kind: "episode", parent: idSeason1, title: "Folge 2", season: 1, episode: 2})
	e.setProviders(t, `["tvdb","tmdb"]`)
	if err := e.svc.match(ctx, idE0102); err != nil {
		t.Fatal(err)
	}
	if tx := e.texts(t, idE0102); tx["en"].Title != "Episode two" {
		t.Fatalf("episode 2 texts %+v, want TVDB's", tx)
	}
	if n := e.tmdb.count("/3/tv/900/season/1"); n != 0 {
		t.Fatalf("%d TMDB season requests with TVDB first", n)
	}
}

// TestMatchSeriesDefaultListIsTMDBFirst: ["tmdb","tvdb"] behaves as before ADR 0015 -- TMDB is
// searched first and TVDB only steps in when TMDB knows nothing.
func TestMatchSeriesDefaultListIsTMDBFirst(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addNode(t, node{id: idSeries, kind: "series", title: "Only On TVDB", year: 2015})
	e.setProviders(t, `["tmdb","tvdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/search/tv"); n != 2 {
		t.Fatalf("%d TMDB searches, want 2 before the fallback", n)
	}
	if s := e.child(t, idSeries); s.State != "matched" || s.TVDB.Int64 != 424242 {
		t.Fatalf("series %+v", s)
	}
}

// TestMatchSeriesTVDBFirstWithoutKeyIsSkipped: a list that names TVDB first needs the key.
func TestMatchSeriesTVDBFirstWithoutKeyIsSkipped(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.addNode(t, node{id: idSeries, kind: "series", title: "Test Show", year: 2011})
	e.setProviders(t, `["tvdb","tmdb"]`)
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if s := e.child(t, idSeries); s.State != "skipped" || s.Err.String != "no_tvdb_key" {
		t.Fatalf("series %+v, want skipped no_tvdb_key", s)
	}
}
