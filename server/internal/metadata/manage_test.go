package metadata

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func fieldCodes(t *testing.T, err error) []string {
	t.Helper()
	var v *model.ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("error %v, want a validation error", err)
	}
	var out []string
	for _, f := range v.Fields {
		out = append(out, f.Field+":"+f.Code)
	}
	return out
}

func intp(n int) *int { return &n }

func TestSearch(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()

	_, err := e.svc.Search(ctx, "imdb", "episode", "  ", intp(1700))
	if got := fieldCodes(t, err); !slices.Equal(got, []string{"provider:provider_not_searchable", "kind:invalid_value", "query:required", "year:invalid_value"}) {
		t.Fatalf("validation %q", got)
	}
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := e.svc.Search(ctx, "tmdb", "movie", string(long), nil); !slices.Equal(fieldCodes(t, err), []string{"query:too_long"}) {
		t.Fatalf("long query: %v", err)
	}

	movies, err := e.svc.Search(ctx, "tmdb", "movie", " Searched Film ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 2 || movies[1].ProviderID != "601" || movies[1].Provider != "tmdb" || *movies[1].Year != 2002 {
		t.Fatalf("movie candidates %+v", movies)
	}
	series, err := e.svc.Search(ctx, "tmdb", "series", "Test Show", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := model.MetadataCandidate{Provider: "tmdb", ProviderID: "900", Title: "Test Show", OriginalTitle: "Test Show",
		Year: series[0].Year, Overview: "English series overview", PosterURL: e.tmdb.srv.URL + "/t/p/w200/tvposter900.jpg"}
	if len(series) != 1 || series[0] != want || *series[0].Year != 2010 {
		t.Fatalf("series candidates %+v", series)
	}
	tv, err := e.svc.Search(ctx, "tvdb", "series", "Only On TVDB", intp(2015))
	if err != nil || len(tv) != 1 || tv[0].ProviderID != "424242" || tv[0].Provider != "tvdb" {
		t.Fatalf("tvdb candidates %+v, %v", tv, err)
	}
	if tvm, err := e.svc.Search(ctx, "tvdb", "movie", "Only On TVDB", nil); err != nil || len(tvm) != 0 {
		t.Fatalf("tvdb movies %+v, %v", tvm, err)
	}

	none := newEnv(t, "")
	if _, err := none.svc.Search(ctx, "tmdb", "movie", "x", nil); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Fatalf("tmdb without key: %v", err)
	}
	if _, err := none.svc.Search(ctx, "tvdb", "series", "x", nil); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Fatalf("tvdb without key: %v", err)
	}
	bad := newEnvWith(t, testKeys{tmdb: "revoked", tvdb: "wrong"})
	if _, err := bad.svc.Search(ctx, "tmdb", "movie", "x", nil); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Fatalf("rejected tmdb key: %v", err)
	}
	if _, err := bad.svc.Search(ctx, "tvdb", "series", "x", nil); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Fatalf("rejected tvdb key: %v", err)
	}
}

// addFile adds a media file to item (root 1 of library 1).
func (e *env) addFile(t *testing.T, fileID, itemID string) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT OR IGNORE INTO library_roots (id, library_id, path, created_at) VALUES (1, 1, '/media/movies', 0)`)
	mustExec(t, e.db.Write, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	                         VALUES (?, ?, 1, ?, 1, 1, 'mkv', 0, 0)`, fileID, itemID, fileID+".mkv")
}

// B-6: a manual match survives refreshes and keeps the files of the title.
func TestAssignMatchAndRefresh(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, imdbID: "tt0000550", locked: `["title"]`})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	file := tid("FIL", 1)
	e.addFile(t, file, idByTMDB)
	mustExec(t, e.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, source, created_at)
	                         VALUES (?, 'en', 'autoTrail01', 'tmdb', 0), (?, 'de', 'manualPick1', 'manual', 0)`, idByTMDB, idByTMDB)
	mustExec(t, e.db.Write, `DELETE FROM jobs`)

	if err := e.svc.AssignMatch(ctx, idByTMDB, "tmdb", "601"); err != nil {
		t.Fatal(err)
	}
	r := e.item(t, idByTMDB)
	if r.TMDB.Int64 != 601 || r.IMDB.Valid || r.State != "pending" || r.Locked != `["title","tmdb_id","tvdb_id","imdb_id"]` {
		t.Fatalf("after assign %+v", r)
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "pending" {
		t.Fatalf("trailer state %s", state)
	}
	if got := e.trailers(t, idByTMDB); len(got) != 1 || got[0].Source != "manual" {
		t.Fatalf("trailers after assign %+v", got)
	}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("match jobs %q", got)
	}
	for range 3 {
		if err := e.svc.match(ctx, idByTMDB); err != nil {
			t.Fatal(err)
		}
		mustExec(t, e.db.Write, `DELETE FROM jobs`)
		if err := e.svc.Refresh(ctx, idByTMDB); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("refresh match jobs %q", got)
	}
	if got := e.jobList(t, JobProbeFile); !slices.Equal(got, []string{"probe:" + file + " p3"}) {
		t.Fatalf("refresh probe jobs %q", got)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	r = e.item(t, idByTMDB)
	if r.State != "matched" || r.TMDB.Int64 != 601 || r.IMDB.Valid || r.Rating.Int64 != 12 || r.Source.String != "tmdb:US" {
		t.Fatalf("after refresh %+v", r)
	}
	if tx := e.texts(t, idByTMDB); tx["en"].Overview.String != "English overview of Searched Film" || tx["en"].Title != "First" {
		t.Fatalf("texts of the assigned title %+v", tx)
	}
	var files int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM media_files WHERE item_id = ?`, idByTMDB).Scan(&files); err != nil || files != 1 {
		t.Fatalf("files %d, %v", files, err)
	}

	// Validation and unknown items.
	if err := e.svc.AssignMatch(ctx, idByTMDB, "tvdb", "1"); !slices.Equal(fieldCodes(t, err), []string{"provider:invalid_value"}) {
		t.Fatalf("tvdb for a movie: %v", err)
	}
	if err := e.svc.AssignMatch(ctx, idByTMDB, "omdb", "abc"); !slices.Equal(fieldCodes(t, err), []string{"provider:provider_not_searchable", "providerId:invalid_value"}) {
		t.Fatalf("bad input: %v", err)
	}
	if err := e.svc.AssignMatch(ctx, idNoMatch, "tmdb", "1"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown item: %v", err)
	}
	if err := e.svc.Refresh(ctx, "not-an-id"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("refresh of an unknown item: %v", err)
	}
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.AssignMatch(ctx, idE0101, "tmdb", "1"); !slices.Equal(fieldCodes(t, err), []string{"provider:invalid_value"}) {
		t.Fatalf("episode: %v", err)
	}
	if err := e.svc.AssignMatch(ctx, idSeries, "tvdb", "424242"); err != nil {
		t.Fatal(err)
	}
	if s := e.child(t, idSeries); s.TVDB.Int64 != 424242 || s.TMDB.Valid || s.State != "pending" {
		t.Fatalf("series after tvdb assign %+v", s)
	}
}

func TestRefreshEpisodeAndSeason(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.Refresh(ctx, idE0101); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idE0101); r.State != "pending" {
		t.Fatalf("refreshed episode %s, want pending", r.State)
	}
	if err := e.svc.Refresh(ctx, idSeason1); err != nil {
		t.Fatal(err)
	}
	want := []string{"match:" + idE0101 + " p7", "match:" + idSeries + " p7"}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, want) {
		t.Fatalf("match jobs %q, want %q", got, want)
	}
	if err := e.svc.match(ctx, idE0101); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idE0101); r.State != "matched" {
		t.Fatalf("episode after refresh %s", r.State)
	}
}

func TestManualTrailers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if _, err := e.svc.AddManualTrailer(ctx, idByTMDB, "DE", "bad id!"); !slices.Equal(fieldCodes(t, err), []string{"lang:invalid_value", "youtubeId:invalid_value"}) {
		t.Fatalf("validation: %v", err)
	}
	if _, err := e.svc.AddManualTrailer(ctx, idNoMatch, "de", "dQw4w9WgXcQ"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown item: %v", err)
	}
	mustExec(t, e.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at)
	                         VALUES (?, 'en', 'enOfficial1', 'Official Trailer', 'tmdb', 1, 0)`, idByTMDB)
	added, err := e.svc.AddManualTrailer(ctx, idByTMDB, "de", "dQw4w9WgXcQ")
	if err != nil {
		t.Fatal(err)
	}
	if added.Lang != "de" || added.YouTubeID != "dQw4w9WgXcQ" || added.Source != "manual" || added.ID == 0 ||
		added.EmbedURL() != "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ" {
		t.Fatalf("added %+v", added)
	}
	same, err := e.svc.AddManualTrailer(ctx, idByTMDB, "en", "enOfficial1")
	if err != nil {
		t.Fatal(err)
	}
	if same.Source != "manual" || same.Title != "Official Trailer" {
		t.Fatalf("automatic row not turned manual: %+v", same)
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "found" {
		t.Fatalf("state %s", state)
	}
	list, err := e.svc.ListTrailersSystem(ctx, idByTMDB)
	if err != nil || len(list) != 2 || list[0].Lang != "de" || list[1].ID != same.ID {
		t.Fatalf("list %+v, %v", list, err)
	}
	if _, err := e.svc.ListTrailersSystem(ctx, idNoMatch); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("list of an unknown item: %v", err)
	}
	if err := e.svc.DeleteTrailer(ctx, idByTMDB, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown trailer: %v", err)
	}
	if err := e.svc.DeleteTrailer(ctx, idNoMatch, added.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("trailer of another item: %v", err)
	}
	for _, id := range []int64{added.ID, same.ID} {
		if err := e.svc.DeleteTrailer(ctx, idByTMDB, id); err != nil {
			t.Fatal(err)
		}
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "none" {
		t.Fatalf("state %s after deleting every trailer", state)
	}
}
