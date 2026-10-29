package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Tests of ADR 0015: a video library with film and series folders, the rules of the folder list,
// and the three effects of UpdateRoot (concept 07, step 1 DoD).

// createVideo adds a video library with the given folders.
func (e *env) createVideo(roots ...model.LibraryRootRequest) model.Library {
	e.t.Helper()
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Dokus", Kind: model.LibraryVideo, Roots: roots})
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

func root(dir, content string, providers ...string) model.LibraryRootRequest {
	if providers == nil {
		providers = model.DefaultProviders(content)
	}
	return model.LibraryRootRequest{Path: dir, Content: content, Providers: providers}
}

// TestMixedLibraryKeepsFilmAndSeriesWithTheSameTMDBIDApart is the collision case of concept 12.1:
// TMDB numbers films and series from separate id spaces, so a film with tmdb_id 1399 and a series
// with tmdb_id 1399 in one library are two titles. One index per title kind keeps them apart.
func TestMixedLibraryKeepsFilmAndSeriesWithTheSameTMDBIDApart(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	films := tree(t, []string{"Heat (1995) [tmdb-1399]/Heat (1995).mkv"})
	shows := tree(t, []string{"Thrones (2011) {tmdb-1399}/Season 01/Thrones - S01E01.mkv"})
	lib := e.createVideo(root(films, model.RootMovie), root(shows, model.RootSeries))
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v, want done with 2 files", got.Scan)
	}
	kinds := e.str(`SELECT group_concat(kind || '=' || tmdb_id, ',') FROM (SELECT kind, tmdb_id FROM items
	                WHERE library_id = ? AND tmdb_id IS NOT NULL AND deleted_at IS NULL ORDER BY kind)`, lib.ID)
	if kinds != "movie=1399,series=1399" {
		t.Fatalf("titles with tmdb_id 1399 = %q, want a film and a series", kinds)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND kind = 'episode' AND deleted_at IS NULL`, lib.ID); n != 1 {
		t.Fatalf("episodes = %d, want 1", n)
	}
	// Every title carries the list of its folder.
	if v := e.str(`SELECT providers FROM items WHERE kind = 'movie' AND library_id = ?`, lib.ID); v != `["tmdb"]` {
		t.Errorf("movie providers = %s", v)
	}
	if v := e.str(`SELECT group_concat(DISTINCT providers) FROM items WHERE kind IN ('series', 'season', 'episode') AND library_id = ?`, lib.ID); v != `["tmdb","tvdb"]` {
		t.Errorf("series tree providers = %s", v)
	}
}

// TestSeriesRootRejectsFilesWithoutMarkerTheMovieRootDoesNot: the rule "no episode marker, no
// file" is a rule of the series folder, not of the library.
func TestSeriesRootRejectsFilesWithoutMarkerTheMovieRootDoesNot(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	films := tree(t, []string{"Casablanca/Casablanca.mkv"})
	shows := tree(t, []string{"Some Show/Season 01/no marker here.mkv"})
	lib := e.createVideo(root(films, model.RootMovie), root(shows, model.RootSeries))
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 1 {
		t.Fatalf("scan = %+v, want done with 1 file", got.Scan)
	}
	if v := e.str(`SELECT group_concat(kind) FROM items WHERE library_id = ? AND deleted_at IS NULL`, lib.ID); v != "movie" {
		t.Fatalf("items = %q, want one movie", v)
	}
}

func TestCreateAppliesTheFolderRules(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Alien (1979)/Alien (1979).mkv"})
	other := tree(t, []string{"Aliens (1986)/Aliens (1986).mkv"})
	cases := []struct {
		name        string
		kind        string
		roots       []model.LibraryRootRequest
		field, code string
	}{
		{"audiobook folder in a video library", model.LibraryVideo, []model.LibraryRootRequest{root(dir, model.RootAudiobook)}, "roots[0].content", model.CodeContentNotAllowed},
		{"movie folder in a podcast library", model.LibraryPodcasts, []model.LibraryRootRequest{root(dir, model.RootMovie)}, "roots[0].content", model.CodeContentNotAllowed},
		{"anilist alone", model.LibraryVideo, []model.LibraryRootRequest{root(dir, model.RootMovie, "anilist")}, "roots[0].providers", model.CodeProvidersNotAllowed},
		{"tvdb on a movie folder", model.LibraryVideo, []model.LibraryRootRequest{root(dir, model.RootMovie, "tvdb")}, "roots[0].providers", model.CodeProvidersNotAllowed},
		{"duplicates", model.LibraryVideo, []model.LibraryRootRequest{root(dir, model.RootSeries, "tmdb", "tmdb")}, "roots[0].providers", model.CodeProvidersNotAllowed},
		{"empty list", model.LibraryVideo, []model.LibraryRootRequest{{Path: dir, Content: model.RootMovie, Providers: []string{}}}, "roots[0].providers", model.CodeProvidersNotAllowed},
		{"no folder at all", model.LibraryVideo, nil, "roots", model.CodeRootsRequired},
		{"no folder in an audiobook library", model.LibraryAudiobooks, nil, "roots", model.CodeRootsRequired},
		{"two folders of the request overlap", model.LibraryVideo, []model.LibraryRootRequest{root(dir, model.RootMovie), root(filepath.Join(dir, "Alien (1979)"), model.RootMovie)}, "roots[1].path", model.CodeRootOverlap},
		{"a folder that does not exist", model.LibraryVideo, []model.LibraryRootRequest{root(other, model.RootMovie), root(filepath.Join(other, "missing"), model.RootSeries)}, "roots[1].path", "library_path_invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "X", Kind: c.kind, Roots: c.roots})
			if !hasFieldError(err, c.field, c.code) {
				t.Fatalf("err = %v, want %s %s", err, c.field, c.code)
			}
		})
	}
	// All or nothing: the failed requests left no library and no root behind.
	if n := e.count(`SELECT count(*) FROM libraries`) + e.count(`SELECT count(*) FROM library_roots`); n != 0 {
		t.Fatalf("rows left behind by refused requests = %d", n)
	}
	// What the rules allow: TVDB first with two fillers on a series folder, a podcast library
	// without folder, and a library with a film and a series folder in one go.
	if _, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Anime", Kind: model.LibraryVideo,
		Roots: []model.LibraryRootRequest{root(dir, model.RootSeries, "tvdb", "tmdb", "anilist")}}); err != nil {
		t.Fatalf("tvdb, tmdb, anilist on a series folder: %v", err)
	}
	if _, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Podcasts", Kind: model.LibraryPodcasts}); err != nil {
		t.Fatalf("podcast library without folder: %v", err)
	}
	mixed, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Dokus", Kind: model.LibraryVideo,
		Roots: []model.LibraryRootRequest{root(other, model.RootMovie), root(tree(t, nil), model.RootSeries)}})
	if err != nil || len(mixed.Roots) != 2 || mixed.Roots[0].Content != model.RootMovie || mixed.Roots[1].Content != model.RootSeries {
		t.Fatalf("mixed library = %+v, %v", mixed.Roots, err)
	}
}

// TestAddRootsIsAllOrNothing: three folders whose third one overlaps leave none behind (F-13).
func TestAddRootsIsAllOrNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(tree(t, []string{"Alien (1979)/Alien (1979).mkv"}))
	one, two := tree(t, nil), tree(t, nil)
	if err := os.MkdirAll(filepath.Join(one, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.AddRoots(e.ctx, lib.ID, []model.LibraryRootRequest{
		root(one, model.RootMovie), root(two, model.RootSeries), root(filepath.Join(one, "sub"), model.RootMovie),
	})
	if !hasFieldError(err, "roots[2].path", model.CodeRootOverlap) {
		t.Fatalf("err = %v, want roots[2].path root_overlap", err)
	}
	if n := e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, lib.ID); n != 1 {
		t.Fatalf("roots after the refused request = %d, want 1", n)
	}
	after, err := e.svc.AddRoots(e.ctx, lib.ID, []model.LibraryRootRequest{root(one, model.RootMovie), root(two, model.RootSeries)})
	if err != nil || len(after.Roots) != 3 {
		t.Fatalf("two folders in one go: %+v, %v", after.Roots, err)
	}
	// One scan for the library, not one per folder.
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Fatalf("scan jobs = %d, want 1", n)
	}
}

// TestUpdateRootProvidersQueuesOneJobPerTitleAndDeletesNothing covers the first two effects of
// concept 6.3: a changed list lands on every title of the folder, each title gets one metadata
// job, and nothing is deleted or scanned.
func TestUpdateRootProvidersQueuesOneJobPerTitleAndDeletesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Alien (1979)/Alien (1979).mkv", "Aliens (1986)/Aliens (1986).mkv", "Aliens (1986)/Aliens (1986)-trailer.mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	after, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Providers: []string{"tmdb", "anilist"}})
	if err != nil {
		t.Fatalf("update root: %v", err)
	}
	if got := after.Roots[0].Providers; len(got) != 2 || got[1] != "anilist" {
		t.Fatalf("root providers = %v", got)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND providers = '["tmdb","anilist"]'`, lib.ID); n != 3 {
		t.Fatalf("items with the new list = %d, want 3 (two films and the extra)", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key LIKE 'match:%'`, JobMetadataMatch); n != 2 {
		t.Fatalf("match jobs = %d, want one per film", n)
	}
	if n := e.jobCount(JobScanLibrary); n != 0 {
		t.Fatalf("scan jobs = %d, want none", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND deleted_at IS NOT NULL`, lib.ID) +
		e.count(`SELECT count(*) FROM media_files WHERE root_id = ?`, rootID); n != 3 {
		t.Fatalf("deleted titles + files = %d, want 0 + 3", n)
	}
	// The same list again changes nothing and queues nothing.
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Providers: []string{"tmdb", "anilist"}}); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobMetadataMatch); n != 0 {
		t.Fatalf("match jobs after an unchanged list = %d", n)
	}
	// A list the rules refuse.
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Providers: []string{"tvdb"}}); !hasFieldError(err, "providers", model.CodeProvidersNotAllowed) {
		t.Fatalf("tvdb on a movie folder: %v", err)
	}
}

// TestUpdateRootContentChangeRereadsTheFolder covers the third effect: a film folder read as a
// series folder. The old titles leave like with a removal (download tombstone, no title without
// files, ignored files gone), the scanner's state of the folder is cleared, the watcher stays,
// and the queued scan creates the episodes.
func TestUpdateRootContentChangeRereadsTheFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Show (2020)/Season 01/Show - S01E01.mkv", "Other (2021)/Season 01/Other - S01E01.mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, dir)
	if n := e.count(`SELECT count(*) FROM items WHERE library_id = ? AND kind = 'movie' AND deleted_at IS NULL`, lib.ID); n != 2 {
		t.Fatalf("read as films: %d movies, want 2", n)
	}
	itemID := e.str(`SELECT item_id FROM media_files WHERE root_id = ? ORDER BY rel_path LIMIT 1`, rootID)
	fileID := e.str(`SELECT id FROM media_files WHERE item_id = ?`, itemID)
	downloadID := e.addDownload(itemID, fileID)
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO ignored_files (root_id, rel_path, reason, created_at) VALUES (?, 'x.mkv', 'test', 0)`, rootID); err != nil {
		t.Fatal(err)
	}
	e.svc.orphanMu.Lock()
	e.svc.orphans[rootID] = map[string]fileStamp{"old.mkv": {1, 1}}
	e.svc.orphanMu.Unlock()
	e.svc.pathMu.Lock()
	e.svc.pathPending[lib.ID] = map[int64]map[string]bool{rootID: {"Show (2020)": true}}
	e.svc.pathMu.Unlock()
	watchersBefore := len(e.svc.watcherStop)

	series := model.RootSeries
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Content: &series}); !errors.Is(err, model.ErrConfirmMismatch) {
		t.Fatalf("content change without the typed path: %v", err)
	}
	unlock, ok := e.svc.tryLockScan(lib.ID)
	if !ok {
		t.Fatal("scan lock busy")
	}
	_, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Content: &series, ConfirmPath: dir})
	unlock()
	if !errors.Is(err, model.ErrScanRunning) {
		t.Fatalf("content change under a running scan: %v", err)
	}

	after, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Content: &series, ConfirmPath: dir})
	if err != nil {
		t.Fatalf("content change: %v", err)
	}
	// The list stays as it was when the new content allows it; the form sends a new one otherwise.
	if r := after.Roots[0]; r.Content != model.RootSeries || len(r.Providers) != 1 || r.Providers[0] != "tmdb" {
		t.Fatalf("root after the change = %+v", r)
	}
	if n := e.count(`SELECT count(*) FROM items AS i WHERE i.library_id = ? AND i.deleted_at IS NULL
	    AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)`, lib.ID); n != 0 {
		t.Fatalf("undeleted titles without a file = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM download_tombstones WHERE id = ?`, downloadID); n != 1 {
		t.Fatal("the download of the leaving film left no tombstone")
	}
	if n := e.count(`SELECT count(*) FROM ignored_files WHERE root_id = ?`, rootID); n != 0 {
		t.Fatal("ignored files of the old reading survived")
	}
	e.svc.orphanMu.Lock()
	orphans := len(e.svc.orphans[rootID])
	e.svc.orphanMu.Unlock()
	e.svc.pathMu.Lock()
	pending := len(e.svc.pathPending[lib.ID][rootID])
	e.svc.pathMu.Unlock()
	if orphans != 0 || pending != 0 {
		t.Fatalf("scanner state of the folder: %d orphans, %d pending dirs", orphans, pending)
	}
	if len(e.svc.watcherStop) != watchersBefore {
		t.Fatal("the watcher of the folder was stopped")
	}
	if n := e.jobCount(JobScanLibrary); n == 0 {
		t.Fatal("no scan queued after the change")
	}
	got := e.scan(lib.ID)
	if got.Scan.State != "done" {
		t.Fatalf("scan after the change = %+v", got.Scan)
	}
	if v := e.str(`SELECT group_concat(kind || '=' || n, ',') FROM (SELECT kind, count(*) AS n FROM items
	                WHERE library_id = ? AND deleted_at IS NULL GROUP BY kind ORDER BY kind)`, lib.ID); v != "episode=2,season=2,series=2" {
		t.Fatalf("items after the re-read = %s", v)
	}
	// A content the family refuses.
	book := model.RootAudiobook
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootID, model.LibraryRootUpdate{Content: &book, ConfirmPath: dir}); !hasFieldError(err, "content", model.CodeContentNotAllowed) {
		t.Fatalf("audiobook content in a video library: %v", err)
	}
}

// TestPodcastLibraryTakesPodcastFolders: a podcast library accepts a folder of its content and
// refuses any other (F-1, F-3).
func TestPodcastLibraryTakesPodcastFolders(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Podcasts", Kind: model.LibraryPodcasts})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Show"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddRoots(e.ctx, lib.ID, rootReq(dir, model.RootAudiobook)); !hasFieldError(err, "roots[0].content", model.CodeContentNotAllowed) {
		t.Fatalf("audiobook folder in a podcast library: %v", err)
	}
	after, err := e.svc.AddRoots(context.Background(), lib.ID, rootReq(dir, model.RootPodcast))
	if err != nil || len(after.Roots) != 1 || after.Roots[0].Content != model.RootPodcast || after.Roots[0].Providers[0] != "local" {
		t.Fatalf("podcast folder: %+v, %v", after.Roots, err)
	}
}
