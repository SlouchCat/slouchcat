package library

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The home folder of a title (concept 24 section 3.2, ADR 0036) on every path that creates a
// title, brings it back or takes its folder away, each with the provider list after a move.

const alienFilm = "Alien (1979)/Alien (1979).mkv"

// twoMovieFolders creates a film library on a first folder holding Alien and scans it, then adds a
// second folder with the provider list second (empty: the default) holding a copy of Alien with
// other content (a second file of the same title, not a move) and scans again. It returns the
// library, both roots and the film.
func (e *env) twoMovieFolders(second []string) (lib model.Library, a, b int64, film, dirA, dirB string) {
	e.t.Helper()
	// Twelve other films keep the loss of one file below the scan's loss protection.
	dirA, dirB = tree(e.t, append([]string{alienFilm}, twentyMovies[4:16]...)), tree(e.t, []string{alienFilm})
	writeFile(e.t, dirB, alienFilm, "a second copy of the same film")
	lib = e.create(dirA)
	e.scan(lib.ID)
	if second == nil {
		second = model.DefaultProviders(model.RootMovie)
	}
	if _, err := e.svc.AddRoots(e.ctx, lib.ID, []model.LibraryRootRequest{{Path: dirB, Content: model.RootMovie, Providers: second}}); err != nil {
		e.t.Fatal(err)
	}
	e.scan(lib.ID)
	a, b = rootIDOf(e.t, e, lib.ID, dirA), rootIDOf(e.t, e, lib.ID, dirB)
	film = e.str(`SELECT id FROM items WHERE title = 'Alien'`)
	if n := e.count(`SELECT count(DISTINCT root_id) FROM media_files WHERE item_id = ?`, film); n != 2 {
		e.t.Fatalf("the film should have a file in each folder, has %d folders", n)
	}
	return lib, a, b, film, dirA, dirB
}

// home returns root_id and providers of an item as "root/providers".
func (e *env) home(id string) string {
	e.t.Helper()
	return e.str(`SELECT coalesce(root_id, 0) || '/' || providers FROM items WHERE id = ?`, id)
}

// settleJobs marks every job done, so a later queued match is the one the step queued.
func (e *env) settleJobs() {
	e.t.Helper()
	e.exec(`UPDATE jobs SET state = 'done' WHERE state = 'queued'`)
}

func (e *env) queuedMatch(id string) bool {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = 'match:' || ? AND state = 'queued'`, id) == 1
}

func rootText(n int64) string { return strconv.FormatInt(n, 10) }

func TestHomeFolderOnCreateAndForChildren(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	// A film: its folder and that folder's list.
	_, a, _, film, _, _ := e.twoMovieFolders([]string{model.ProviderTMDB, model.ProviderAniList})
	if got := e.home(film); got != rootText(a)+`/["tmdb"]` {
		t.Errorf("film = %s, want the first folder with its list (the second folder's copy is newer)", got)
	}

	// A series begun in one folder: the season and episode a second folder brings take the series'
	// home folder and list, not the folder being read.
	dirA := tree(t, []string{"Show (2020)/Season 01/Show (2020) S01E01.mkv"})
	dirB := tree(t, []string{"Show (2020)/Season 02/Show (2020) S02E01.mkv"})
	lib := e.createSeries(dirA)
	e.scan(lib.ID)
	if _, err := e.svc.AddRoots(e.ctx, lib.ID, []model.LibraryRootRequest{{Path: dirB, Content: model.RootSeries, Providers: []string{model.ProviderTVDB, model.ProviderTMDB}}}); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	sa := rootIDOf(t, e, lib.ID, dirA)
	series := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series'`); n != 1 {
		t.Fatalf("series rows = %d, want one series in two folders", n)
	}
	want := rootText(sa) + `/["tmdb","tvdb"]`
	if got := e.home(series); got != want {
		t.Errorf("series = %s, want %s", got, want)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind IN ('season', 'episode') AND (root_id IS NOT ? OR providers <> '["tmdb","tvdb"]')`, sa); n != 0 {
		t.Errorf("%d seasons or episodes do not carry the series' home folder and list", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'episode'`); n != 2 {
		t.Errorf("episodes = %d, want 2", n)
	}
}

func TestHomeFolderMovesWhenTheFilesMove(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, _, b, film, dirA, _ := e.twoMovieFolders([]string{model.ProviderTMDB, model.ProviderAniList})
	e.settleJobs()
	// The file in the home folder goes: the title moves to the other folder with its list and its
	// match is queued.
	remove(t, dirA, alienFilm)
	e.scan(lib.ID)
	if got := e.home(film); got != rootText(b)+`/["tmdb","anilist"]` {
		t.Errorf("after the move = %s, want the second folder with its list", got)
	}
	if !e.queuedMatch(film) {
		t.Error("the move queued no match")
	}
	// The file comes back in the first folder: the title stays where it has a present file.
	e.settleJobs()
	writeFile(t, dirA, alienFilm, "video 0 "+alienFilm)
	e.scan(lib.ID)
	if got := e.home(film); got != rootText(b)+`/["tmdb","anilist"]` {
		t.Errorf("after the return to the first folder = %s, want it to stay in the second", got)
	}
	if e.queuedMatch(film) {
		t.Error("a title that did not move queued a match")
	}
}

func TestHomeFolderOfASoftDeletedTitleThatReturns(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, append([]string{alienFilm}, twentyMovies[4:16]...))
	lib := e.create(dir)
	e.scan(lib.ID)
	a := rootIDOf(t, e, lib.ID, dir)
	film := e.str(`SELECT id FROM items WHERE title = 'Alien'`)
	// The file goes, the sweep takes the title out; a home folder that lost its reference (a
	// removed folder sets it to NULL) is set again when the file reappears and brings it back.
	remove(t, dir, alienFilm)
	e.scan(lib.ID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v", n, err)
	}
	e.exec(`UPDATE items SET root_id = NULL WHERE id = ?`, film)
	writeFile(t, dir, alienFilm, "video 0 "+alienFilm)
	e.scan(lib.ID)
	if got := e.str(`SELECT (deleted_at IS NULL) || '/' || coalesce(root_id, 0) FROM items WHERE id = ?`, film); got != "1/"+rootText(a) {
		t.Errorf("returned film = %s, want live in its folder", got)
	}
}

func TestHomeFolderAfterTheFolderIsRemoved(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, a, b, film, dirA, _ := e.twoMovieFolders([]string{model.ProviderTMDB, model.ProviderAniList})
	e.settleJobs()
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, a, dirA); err != nil {
		t.Fatal(err)
	}
	if got := e.home(film); got != rootText(b)+`/["tmdb","anilist"]` {
		t.Errorf("after the removal = %s, want the second folder with its list", got)
	}
	if !e.queuedMatch(film) {
		t.Error("the move queued no match")
	}
}

func TestHomeFolderAfterAContentChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, a, b, film, dirA, _ := e.twoMovieFolders([]string{model.ProviderTMDB, model.ProviderAniList})
	ja := []string{"ja"}
	tl, _ := model.RootMetadataColumns(model.RootMetadata{TextLangs: &ja})
	e.exec(`UPDATE library_roots SET text_langs = ? WHERE id IN (?, ?)`, tl, a, b)
	e.settleJobs()
	series := model.RootSeries
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, a, model.LibraryRootUpdate{Content: &series, Providers: model.DefaultProviders(series), ConfirmPath: dirA}); err != nil {
		t.Fatal(err)
	}
	if got := e.home(film); got != rootText(b)+`/["tmdb","anilist"]` {
		t.Errorf("after the content change = %s, want the second folder with its list", got)
	}
	if !e.queuedMatch(film) {
		t.Error("the move queued no match")
	}
	// The folder keeps its own languages with the other video content.
	if n := e.count(`SELECT count(*) FROM library_roots WHERE id = ? AND text_langs IS NOT NULL`, a); n != 1 {
		t.Error("a content change dropped the own languages")
	}
}

func TestHomeFolderKeepsAcrossAPathChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{alienFilm})
	lib := e.create(dir)
	e.scan(lib.ID)
	a := rootIDOf(t, e, lib.ID, dir)
	film := e.str(`SELECT id FROM items WHERE kind = 'movie'`)
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, a, model.LibraryRootUpdate{Path: &moved}); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	if got := e.home(film); got != rootText(a)+`/["tmdb"]` {
		t.Errorf("after the path change = %s, want the same folder", got)
	}
}

func TestProvidersFollowTheHomeFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, a, b, film, _, _ := e.twoMovieFolders(nil)
	// A new list on the second folder leaves the film, whose home is the first.
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, b, model.LibraryRootUpdate{Providers: []string{model.ProviderTMDB, model.ProviderAniList}}); err != nil {
		t.Fatal(err)
	}
	if got := e.home(film); got != rootText(a)+`/["tmdb"]` {
		t.Errorf("after the second folder's change = %s, want the first folder's list", got)
	}
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, a, model.LibraryRootUpdate{Providers: []string{model.ProviderTMDB, model.ProviderAniList}}); err != nil {
		t.Fatal(err)
	}
	if got := e.home(film); got != rootText(a)+`/["tmdb","anilist"]` {
		t.Errorf("after the home folder's change = %s, want its new list", got)
	}
}

func TestHomeFolderOfAudiobooksAndFolderPodcasts(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required (install ffmpeg)")
	}
	e := newEnv(t)
	books := e.createAudiobooks(audiobookMedia(t))
	e.scan(books.ID)
	br := books.Roots[0].ID
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'audiobook' AND library_id = ? AND root_id IS NOT ?`, books.ID, br); n != 0 {
		t.Errorf("%d audiobooks without their folder as home folder", n)
	}
	pods := e.createPodcasts(podcastFolderMedia(t))
	e.scan(pods.ID)
	pr := pods.Roots[0].ID
	if n := e.count(`SELECT count(*) FROM items WHERE kind IN ('podcast', 'podcast_episode') AND library_id = ? AND root_id = ?`, pods.ID, pr); n != 3 {
		t.Errorf("podcast and episodes in the folder = %d, want 3", n)
	}
}

// A subscription made by hand has no home folder; a folder that brings its feed adopts it, and the
// podcast with every episode takes that folder.
func TestHomeFolderOfAnAdoptedSubscription(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	root := t.TempDir()
	media := filepath.Join(filepath.Dir(testMedia(t)), "Podcasts")
	podcastDir(t, root, "Abonniert", map[string]any{"title": "Abonniert", "feedURL": "https://feeds.example.test/a.xml"},
		map[string]string{"a1.mp3": filepath.Join(media, "episode-1.mp3")})
	lib := e.createPodcasts(root)
	e.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers,
	    metadata_state, added_at, created_at, updated_at) VALUES ('01ABONNIERT00000000000000A', 'podcast', ?, 'Abo von Hand', 'abo von hand',
	    'https://feeds.example.test/a.xml', 'admin', '["local"]', 'matched', 0, 0, 0)`, lib.ID)
	e.exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, enclosure_url, providers,
	    metadata_state, added_at, created_at, updated_at) VALUES ('01ABONNIERTFEEDEP000000001', 'podcast_episode', '01ABONNIERT00000000000000A', ?,
	    'Feed', 'feed', 'g1', 'https://feeds.example.test/1.mp3', '["local"]', 'matched', 0, 0, 0)`, lib.ID)
	e.scan(lib.ID)
	r := lib.Roots[0].ID
	if n := e.count(`SELECT count(*) FROM items WHERE (id = '01ABONNIERT00000000000000A' OR parent_id = '01ABONNIERT00000000000000A') AND root_id = ?`, r); n != 3 {
		t.Errorf("adopted podcast with its feed and file episode in the folder = %d rows, want 3", n)
	}
}

// The relink brings a deleted title back with the files of its live copy: it takes the copy's
// folder as its home folder, with that folder's list.
func TestHomeFolderOfARelinkedTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dirA := tree(t, append([]string{"Heat/Heat.mkv"}, twentyMovies[4:16]...))
	lib := e.create(dirA)
	e.scan(lib.ID)
	heat := e.str(`SELECT id FROM items WHERE title = 'Heat'`)
	e.exec(`UPDATE items SET tmdb_id = 949 WHERE id = ?`, heat)
	remove(t, dirA, "Heat/Heat.mkv")
	e.scan(lib.ID)
	e.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, heat)
	dirB := tree(t, []string{"Heat Copy/Heat.mkv"})
	writeFile(t, dirB, "Heat Copy/Heat.mkv", "another release")
	if _, err := e.svc.AddRoots(e.ctx, lib.ID, []model.LibraryRootRequest{{Path: dirB, Content: model.RootMovie, Providers: []string{model.ProviderTMDB, model.ProviderAniList}}}); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	b := rootIDOf(t, e, lib.ID, dirB)
	copyID := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat Copy/Heat.mkv'`)
	e.exec(`UPDATE items SET tmdb_id = 949 WHERE id = ?`, copyID)
	if _, err := e.svc.Relink(e.ctx, heat, copyID); err != nil {
		t.Fatal(err)
	}
	if got := e.home(heat); got != rootText(b)+`/["tmdb","anilist"]` {
		t.Errorf("relinked film = %s, want the copy's folder with its list", got)
	}
}
