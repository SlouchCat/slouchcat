package library

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// podcastRoot returns the podcast folder of a podcast library.
func (e *env) podcastRoot(libraryID int64) appq.LibraryRoot {
	e.t.Helper()
	roots, err := appq.New(e.db.Read).ListLibraryRoots(e.ctx, libraryID)
	if err != nil || len(roots) == 0 {
		e.t.Fatalf("roots of library %d: %v %v", libraryID, roots, err)
	}
	return roots[0]
}

// dirNames lists the names in dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Concept 19 P-6 (Go part): a read-only folder, a folder whose file system is not the one it was
// added with or whose known podcast folders are gone (the NAS is not mounted), and a file system
// without hard links fail the folder check with their reason; after the fix the check passes. The
// check leaves nothing behind, and nothing is written into a folder that is not mounted.
func TestPodcastFolderCheck(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Bekannt/Folge 1.mp3", "audio")
	lib := e.createPodcasts(dir)
	e.scan(lib.ID)
	root := e.podcastRoot(lib.ID)
	check := func(want string) model.PodcastFolder {
		t.Helper()
		got := e.svc.CheckPodcastFolder(e.ctx, root)
		if got.Problem != want || got.Writable != (want == "") || got.RootID != root.ID || got.Path != dir {
			t.Errorf("folder check = %+v, want problem %q", got, want)
		}
		return got
	}
	if got := check(""); got.FreeBytes <= 0 {
		t.Errorf("free bytes of a writable folder = %d", got.FreeBytes)
	}
	if names := dirNames(t, dir); len(names) != 1 || names[0] != "Bekannt" {
		t.Errorf("the check left %v", names)
	}

	// Read-only.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	check(model.PodcastFolderReadOnly)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	check("")

	// Not mounted: the folder was added on another file system ...
	other := root
	other.FsType = string(model.RootPodcast) + "-elsewhere"
	if got := e.svc.CheckPodcastFolder(e.ctx, other); got.Problem != model.PodcastFolderNotMounted || got.Writable {
		t.Errorf("another file system = %+v", got)
	}
	// ... or none of its known folders is there (an empty mount point).
	if err := os.Rename(filepath.Join(dir, "Bekannt"), filepath.Join(t.TempDir(), "Bekannt")); err != nil {
		t.Fatal(err)
	}
	check(model.PodcastFolderNotMounted)
	if names := dirNames(t, dir); len(names) != 0 {
		t.Errorf("a folder that is not mounted got %v", names)
	}
	writeFile(t, dir, "Bekannt/Folge 1.mp3", "audio")
	check("")

	// No hard links.
	e.svc.podcastLink = func(*os.Root, string, string) error { return &os.LinkError{Op: "link", Err: syscall.EPERM} }
	check(model.PodcastFolderNoHardLinks)
	e.svc.podcastLink = nil
	check("")
	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("the checks left %v", names)
	}

	// A folder of another content is never written; the folder of a library comes from its setting.
	other = root
	other.Content = model.RootAudiobook
	if got := e.svc.CheckPodcastFolder(e.ctx, other); got.Writable || got.Problem != model.PodcastFolderReadOnly {
		t.Errorf("audiobook folder = %+v", got)
	}
	if got, err := e.svc.PodcastFolderOf(e.ctx, lib.ID); err != nil || !got.Writable || got.RootID != root.ID {
		t.Errorf("folder of the library = %+v %v", got, err)
	}
	if _, err := e.svc.PodcastFolderOf(e.ctx, 999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
}

// podcastFixture is a podcast library on a temp folder with one subscription; ep adds feed episodes,
// file adds media files.
type podcastFixture struct {
	e       *env
	lib     model.Library
	root    appq.LibraryRoot
	dir     string
	podcast string
}

func newPodcastFixture(t *testing.T) *podcastFixture {
	t.Helper()
	e := newEnv(t)
	dir := t.TempDir()
	lib := e.createPodcasts(dir)
	f := &podcastFixture{e: e, lib: lib, root: e.podcastRoot(lib.ID), dir: dir, podcast: e.svc.newID()}
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast', ?, 'Testpodcast', 'testpodcast', 'https://feeds.example.test/feed.xml', 'admin', '["local"]', 'matched', 1, 1, 1)`, f.podcast, lib.ID)
	return f
}

func (f *podcastFixture) exec(query string, args ...any) {
	f.e.t.Helper()
	if _, err := f.e.db.Write.ExecContext(f.e.ctx, query, args...); err != nil {
		f.e.t.Fatalf("%s: %v", query, err)
	}
}

// feedEpisode adds a feed episode (with guid and enclosure) published at at.
func (f *podcastFixture) feedEpisode(title string, at time.Time, durationMs int64) string {
	id := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, published_at, duration_ms, enclosure_url, enclosure_type,
		enclosure_length, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast_episode', ?, ?, ?, ?, ?, ?, ?, ?, 'audio/mpeg', 1000, '["local"]', 'matched', 1, 1, 1)`,
		id, f.podcast, f.lib.ID, title, SortTitle(title), "guid-"+id, at.UnixMilli(), sql.NullInt64{Int64: durationMs, Valid: durationMs > 0},
		"https://cdn.example.test/"+id+".mp3")
	return id
}

// fileEpisode adds a file episode (no guid) of podcast with a media file at rel, published at at.
func (f *podcastFixture) fileEpisode(podcast, rel string, at time.Time) string {
	id := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, published_at, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast_episode', ?, ?, ?, ?, ?, '["local"]', 'matched', 1, 1, 1)`,
		id, podcast, f.lib.ID, filepath.Base(rel), SortTitle(filepath.Base(rel)), at.UnixMilli())
	f.file(id, rel)
	return id
}

// file adds a media file of item at rel.
func (f *podcastFixture) file(itemID, rel string) string {
	id := f.e.svc.newID()
	f.exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 1, 'mp3', 1, 1)`,
		id, itemID, f.root.ID, rel)
	return id
}

func (f *podcastFixture) settings() systemq.PodcastEffectiveSettingsRow {
	f.e.t.Helper()
	rows, err := systemq.New(f.e.db.Read).PodcastEffectiveSettings(f.e.ctx, systemq.PodcastEffectiveSettingsParams{PodcastID: f.podcast})
	if err != nil || len(rows) != 1 {
		f.e.t.Fatalf("effective settings: %v %v", rows, err)
	}
	return rows[0]
}

func (f *podcastFixture) withoutFile() []systemq.PodcastFeedEpisodesRow {
	f.e.t.Helper()
	rows, err := systemq.New(f.e.db.Read).PodcastFeedEpisodes(f.e.ctx, sql.NullString{String: f.podcast, Valid: true})
	if err != nil {
		f.e.t.Fatal(err)
	}
	var out []systemq.PodcastFeedEpisodesRow
	for _, r := range rows {
		if r.Files == 0 {
			out = append(out, r)
		}
	}
	return out
}

// Concept 19 P-8 (section 8, ADR 0029 decision 6): a file whose name matches the title of a feed
// episode by key is paired instead of loaded (also after Audiobookshelf's colon rule and its uuid
// suffix); two files by key, a key hit on a file of another item (a deleted episode of an earlier
// subscription), two candidates of today's rule and an unpaired file within 24 hours make the
// episode unclear; an episode without any of these is loaded.
func TestPodcastRiegel(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	// Riegel 1, one hit at a file episode without guid: paired.
	pairTitle := f.feedEpisode("Folge 1: Der Anfang", day, 0)
	pairFile := f.fileEpisode(f.podcast, "Testpodcast/Folge 1 - Der Anfang (0f8fad5b-d9cb-469f-a165-70867728950e).mp3", day.Add(-30*24*time.Hour))
	// Riegel 1, two hits: unclear.
	twoHits := f.feedEpisode("Folge 2", day.Add(24*time.Hour), 0)
	f.fileEpisode(f.podcast, "Testpodcast/Folge 2.mp3", day.Add(-40*24*time.Hour))
	f.fileEpisode(f.podcast, "Testpodcast/Folge 2.m4a", day.Add(-41*24*time.Hour))
	// Riegel 1, a hit on a deleted episode of an earlier subscription of the same feed: unclear.
	earlier := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, providers, metadata_state, added_at, created_at, updated_at, deleted_at)
		VALUES (?, 'podcast', ?, 'Testpodcast', 'testpodcast', 'https://feeds.example.test/feed.xml', '["local"]', 'matched', 1, 1, 1, 2)`, earlier, f.lib.ID)
	oldEp := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, providers, metadata_state, added_at, created_at, updated_at, deleted_at)
		VALUES (?, 'podcast_episode', ?, ?, 'Folge 3', 'folge 3', 'old-3', '["local"]', 'matched', 1, 1, 1, 2)`, oldEp, earlier, f.lib.ID)
	f.file(oldEp, "Alter Ordner/Folge 3.mp3")
	elsewhere := f.feedEpisode("Folge 3", day.Add(2*24*time.Hour), 0)
	// Riegel 2, two candidates of today's rule (same minute, no key hit): unclear.
	sameMinute := f.feedEpisode("Interview", day.Add(10*24*time.Hour), 0)
	f.fileEpisode(f.podcast, "Testpodcast/a.mp3", day.Add(10*24*time.Hour+10*time.Second))
	f.fileEpisode(f.podcast, "Testpodcast/b.mp3", day.Add(10*24*time.Hour+20*time.Second))
	// Riegel 3, an unpaired file within 24 hours: unclear.
	near := f.feedEpisode("Ganz anders", day.Add(20*24*time.Hour), 0)
	f.fileEpisode(f.podcast, "Testpodcast/xyz.mp3", day.Add(20*24*time.Hour+23*time.Hour))
	// Nothing there: loaded.
	load := f.feedEpisode("Folge 99", day.Add(60*24*time.Hour), 0)

	got, err := f.e.svc.judgePodcastEpisodes(f.e.ctx, f.settings(), f.withoutFile())
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]episodeVerdict{
		pairTitle: {verdict: verdictPair, pairWith: pairFile}, twoHits: {verdict: verdictUnclear}, elsewhere: {verdict: verdictUnclear},
		sameMinute: {verdict: verdictUnclear}, near: {verdict: verdictUnclear}, load: {verdict: verdictLoad},
	} {
		if got[id] != want {
			t.Errorf("verdict of %s = %+v, want %+v", id, got[id], want)
		}
	}
	// The pair: the feed episode takes the file, the file episode leaves; a second pairing is refused.
	if ok, err := f.e.svc.pairPodcastFile(f.e.ctx, pairTitle, pairFile); err != nil || !ok {
		t.Fatalf("pair = %v %v", ok, err)
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, pairTitle); n != 1 {
		t.Errorf("files of the paired feed episode = %d", n)
	}
	if n := f.e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NOT NULL`, pairFile); n != 1 {
		t.Errorf("file episode kept after the pair")
	}
	if ok, err := f.e.svc.pairPodcastFile(f.e.ctx, pairTitle, pairFile); err != nil || ok {
		t.Errorf("second pair = %v %v", ok, err)
	}
}

// Two feed episodes with the same title would pair with one file: neither is paired, both are unclear.
func TestPodcastRiegelOneFileTwoEpisodes(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	a := f.feedEpisode("Wiederholung", day, 0)
	b := f.feedEpisode("Wiederholung", day.Add(7*24*time.Hour), 0)
	f.fileEpisode(f.podcast, "Testpodcast/Wiederholung.mp3", day.Add(-30*24*time.Hour))
	got, err := f.e.svc.judgePodcastEpisodes(f.e.ctx, f.settings(), f.withoutFile())
	if err != nil {
		t.Fatal(err)
	}
	if got[a].verdict != verdictUnclear || got[b].verdict != verdictUnclear {
		t.Errorf("verdicts = %+v %+v, want both unclear", got[a], got[b])
	}
}

// The state of the list of the administration (section 11): the unclear episodes of the riegel
// and «Speichern» possible, the same from the queries over every podcast as from those of one
// podcast; a podcast without feed is missing, a folder that fails the check is not savable.
func TestPodcastStates(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	f.feedEpisode("Folge 1: Der Anfang", day, 0) // paired by key: not unclear
	f.fileEpisode(f.podcast, "Testpodcast/Folge 1 - Der Anfang.mp3", day.Add(-30*24*time.Hour))
	f.feedEpisode("Folge 2", day.Add(24*time.Hour), 0) // two hits
	f.fileEpisode(f.podcast, "Testpodcast/Folge 2.mp3", day.Add(-40*24*time.Hour))
	f.fileEpisode(f.podcast, "Testpodcast/Folge 2.m4a", day.Add(-41*24*time.Hour))
	f.feedEpisode("Ganz anders", day.Add(20*24*time.Hour), 0) // a file within 24 hours
	f.fileEpisode(f.podcast, "Testpodcast/xyz.mp3", day.Add(20*24*time.Hour+23*time.Hour))
	f.feedEpisode("Folge 99", day.Add(60*24*time.Hour), 0) // nothing there
	withFile := f.feedEpisode("Folge 100", day.Add(70*24*time.Hour), 0)
	f.file(withFile, "Testpodcast/Folge 100.mp3")
	// A second subscription without files and a folder podcast without feed.
	other := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast', ?, 'Zweiter', 'zweiter', 'https://feeds.example.test/other.xml', 'admin', '["local"]', 'matched', 1, 1, 1)`, other, f.lib.ID)
	folderOnly := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast', ?, 'Ordner', 'ordner', '["local"]', 'matched', 1, 1, 1)`, folderOnly, f.lib.ID)

	if err := os.Mkdir(filepath.Join(f.dir, "Testpodcast"), 0o755); err != nil { // a known folder is there
		t.Fatal(err)
	}
	want := map[string]model.PodcastState{f.podcast: {Savable: true, UnclearEpisodes: 2}, other: {Savable: true}}
	all, err := f.e.svc.PodcastStates(f.e.ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(want) || all[f.podcast] != want[f.podcast] || all[other] != want[other] {
		t.Errorf("states of all = %+v, want %+v", all, want)
	}
	for _, id := range []string{f.podcast, other} {
		one, err := f.e.svc.PodcastStates(f.e.ctx, id)
		if err != nil || len(one) != 1 || one[id] != want[id] {
			t.Errorf("state of %s = %+v %v, want %+v", id, one, err, want[id])
		}
	}
	if one, err := f.e.svc.PodcastStates(f.e.ctx, folderOnly); err != nil || len(one) != 0 {
		t.Errorf("state of a podcast without feed = %+v %v", one, err)
	}

	// Another file system than when the folder was added: nothing is savable any more.
	f.exec(`UPDATE library_roots SET fs_type = 'nfs4' WHERE id = ?`, f.root.ID)
	all, err = f.e.svc.PodcastStates(f.e.ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	one, err := f.e.svc.PodcastStates(f.e.ctx, f.podcast)
	if err != nil || all[f.podcast].Savable || all[other].Savable || one[f.podcast].Savable {
		t.Errorf("states on an unmounted folder = %+v, %+v %v", all, one, err)
	}
}

// Concept 19, 5.4: folder checks that run at once (the list, a setting and five downloads) do not
// disturb each other: each writes its own test file, and none removes a test file of another.
func TestPodcastFolderCheckInParallel(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Bekannt/Folge 1.mp3", "audio")
	lib := e.createPodcasts(dir)
	e.scan(lib.ID)
	root := e.podcastRoot(lib.ID)
	writeFile(t, dir, podcastOwnPrefix+"schreibtest-other", "x") // a check of another process
	var wg sync.WaitGroup
	problems := make(chan string, 40)
	for range 40 {
		wg.Go(func() {
			if got := e.svc.CheckPodcastFolder(e.ctx, root); !got.Writable {
				problems <- got.Problem
			}
		})
	}
	wg.Wait()
	close(problems)
	for p := range problems {
		t.Errorf("a parallel check failed: %s", p)
	}
	if got := strings.Join(dirNames(t, dir), ","); got != podcastOwnPrefix+"schreibtest-other,Bekannt" {
		t.Errorf("folder after the checks = %s", got)
	}
}

// Concept 19, 5.4 (not_mounted): the known folders the check looks for are those with a file that
// is there first; folders whose files are all missing do not make a mounted folder look unmounted.
// A root on a file system of unknown type proves nothing by its type: without a known folder it
// counts as not mounted, with one the folder must be there.
func TestPodcastFolderCheckKnownFolders(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	ep := f.feedEpisode("Folge", time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC), 0)
	for i := range 60 {
		id := f.file(ep, "A"+strconv.Itoa(100+i)+"/gone.mp3")
		f.exec(`UPDATE media_files SET missing_since = 1 WHERE id = ?`, id)
	}
	f.file(ep, "Z/here.mp3")
	writeFile(t, f.dir, "Z/here.mp3", "audio")
	if got := f.e.svc.CheckPodcastFolder(f.e.ctx, f.root); !got.Writable {
		t.Errorf("check with 60 missing folders and one there = %+v", got)
	}

	g := newPodcastFixture(t)
	g.e.svc.podcastFSType = func(paths.Root) (paths.FSType, error) { return paths.FSUnknown, nil }
	unknown := g.root
	unknown.FsType = string(paths.FSUnknown)
	if got := g.e.svc.CheckPodcastFolder(g.e.ctx, unknown); got.Writable || got.Problem != model.PodcastFolderNotMounted {
		t.Errorf("unknown file system without a known folder = %+v, want not_mounted", got)
	}
	gep := g.feedEpisode("Folge", time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC), 0)
	g.file(gep, "Bekannt/Folge.mp3")
	writeFile(t, g.dir, "Bekannt/Folge.mp3", "audio")
	if got := g.e.svc.CheckPodcastFolder(g.e.ctx, unknown); !got.Writable {
		t.Errorf("unknown file system with its known folder there = %+v", got)
	}
}
