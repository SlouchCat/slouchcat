package library

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// saveLibrary (concept 24 4.3, section 11 "Stufe 3"): one transaction, the order remove, other
// content, other path, new folders, at most one scan, the typed paths, the revision, the scan
// lock, field errors per folder and nothing written on any error.

// form is what the form «Bibliothek bearbeiten» loads: the library as it is, every folder kept.
func (e *env) form(libraryID int64) model.LibrarySave {
	e.t.Helper()
	lib, err := e.svc.Get(e.ctx, libraryID)
	if err != nil {
		e.t.Fatal(err)
	}
	req := model.LibrarySave{Revision: lib.Revision, Name: lib.Name}
	for _, r := range lib.Roots {
		req.Roots = append(req.Roots, model.LibrarySaveRoot{ID: r.ID, LibraryRootRequest: model.LibraryRootRequest{
			Path: r.Path, Content: r.Content, Providers: r.Providers, Metadata: r.Metadata,
		}})
	}
	return req
}

// renameTo is the settings step of package admin, reduced to the name: it writes in the shared
// transaction, so a later failure takes it back.
func renameTo(libraryID int64, name string) func(context.Context, *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		q := appq.New(tx)
		lib, err := q.GetLibrary(ctx, libraryID)
		if err != nil {
			return err
		}
		_, err = q.UpdateLibrarySettings(ctx, appq.UpdateLibrarySettingsParams{Name: name, DefaultRating: lib.DefaultRating, Now: 1, ID: libraryID})
		return err
	}
}

// clearJobs empties the queue, so a test counts only what the save queued.
func (e *env) clearJobs() {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		e.t.Fatal(err)
	}
}

// twoFolderLibrary is a film library on two scanned folders; it returns the library and both.
func (e *env) twoFolderLibrary() (lib model.Library, first, second string) {
	e.t.Helper()
	first, second = tree(e.t, firstFolder), tree(e.t, secondFolder)
	lib = e.createVideo(root(first, model.RootMovie), root(second, model.RootMovie))
	e.scan(lib.ID)
	e.clearJobs()
	return lib, first, second
}

func (e *env) libraryName(id int64) string {
	e.t.Helper()
	return e.str(`SELECT name FROM libraries WHERE id = ?`, id)
}

func TestSaveLibraryRenameAloneScansNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, _, _ := e.twoFolderLibrary()
	req := e.form(lib.ID)

	saved, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Library.Name != "Kino" || len(saved.Library.Roots) != 2 || len(saved.MetadataChanged) != 0 {
		t.Fatalf("saved = %+v", saved)
	}
	if n := e.jobCount(JobScanLibrary); n != 0 {
		t.Fatalf("scan jobs after a rename = %d, want none", n)
	}
	if n := e.jobCount(JobMetadataFill) + e.jobCount(JobMetadataMatch); n != 0 {
		t.Fatalf("metadata jobs after a rename = %d, want none", n)
	}
	if saved.Library.Revision == req.Revision {
		t.Fatal("the revision did not change with the name")
	}
}

func TestSaveLibraryRevision(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, _ := e.twoFolderLibrary()
	req := e.form(lib.ID)

	// A scan changes no part of the form, so the loaded form stays valid.
	e.scan(lib.ID)
	if again := e.form(lib.ID); again.Revision != req.Revision {
		t.Fatal("a scan changed the revision")
	}
	// Another window changed a folder's providers meanwhile: the old form is stale and writes
	// nothing, not even the name.
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, rootIDOf(t, e, lib.ID, first), model.LibraryRootUpdate{Providers: []string{model.ProviderTMDB, model.ProviderAniList}}); err != nil {
		t.Fatal(err)
	}
	e.clearJobs()
	req.Roots = req.Roots[:1]
	req.Remove = []model.LibrarySaveRemoval{{ID: e.form(lib.ID).Roots[1].ID, ConfirmPath: e.form(lib.ID).Roots[1].Path}}
	if _, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino")); !errors.Is(err, model.ErrStale) {
		t.Fatalf("stale form: err = %v, want ErrStale", err)
	}
	if e.libraryName(lib.ID) != "Dokus" || e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, lib.ID) != 2 {
		t.Fatal("a stale form wrote something")
	}
	// The same change under the new revision goes through.
	req.Revision = e.form(lib.ID).Revision
	if _, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Save(e.ctx, lib.ID+99, req, nil); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown library: err = %v, want ErrNotFound", err)
	}
}

// TestSaveLibraryUnlistedFolderStays: a folder the request forgets is an error, never a removal.
func TestSaveLibraryUnlistedFolderStays(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, _, _ := e.twoFolderLibrary()
	req := e.form(lib.ID)
	req.Roots = req.Roots[:1]

	if _, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino")); !errors.Is(err, model.ErrRootUnlisted) {
		t.Fatalf("err = %v, want ErrRootUnlisted", err)
	}
	if e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, lib.ID) != 2 || e.libraryName(lib.ID) != "Dokus" {
		t.Fatal("an unlisted folder was removed or the name written")
	}
}

func TestSaveLibraryTypedPaths(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, second := e.twoFolderLibrary()
	base := e.form(lib.ID)
	firstID, secondID := base.Roots[0].ID, base.Roots[1].ID
	titles := e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL`)

	for _, c := range []struct {
		confirm, code string
	}{{"", "required"}, {"  ", "required"}, {first + "x", "confirm_mismatch"}, {filepath.Dir(first), "confirm_mismatch"}} {
		// Removing the first folder and reading the second as series, both with the same
		// confirmation.
		req := e.form(lib.ID)
		req.Roots = req.Roots[1:]
		req.Roots[0].Content, req.Roots[0].Providers, req.Roots[0].ConfirmPath = model.RootSeries, model.DefaultProviders(model.RootSeries), c.confirm
		req.Remove = []model.LibrarySaveRemoval{{ID: firstID, ConfirmPath: c.confirm}}
		_, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino"))
		if !hasFieldError(err, "removeRoots[0].confirmPath", c.code) || !hasFieldError(err, "roots[0].confirmPath", c.code) {
			t.Errorf("confirmation %q: %v, want removeRoots[0].confirmPath and roots[0].confirmPath %s", c.confirm, err, c.code)
		}
	}
	if e.count(`SELECT count(*) FROM library_roots WHERE library_id = ? AND content = 'movie'`, lib.ID) != 2 ||
		e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL`) != titles || e.libraryName(lib.ID) != "Dokus" {
		t.Fatal("a refused confirmation wrote something")
	}
	// A new path needs no typed path; the typed path of a content change is the stored one, with
	// surrounding spaces forgiven.
	req := e.form(lib.ID)
	req.Roots[1].Content, req.Roots[1].Providers, req.Roots[1].ConfirmPath = model.RootSeries, model.DefaultProviders(model.RootSeries), " "+second+"\n"
	req.Roots = req.Roots[1:]
	req.Remove = []model.LibrarySaveRemoval{{ID: firstID, ConfirmPath: first}}
	saved, err := e.svc.Save(e.ctx, lib.ID, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Library.Roots) != 1 || saved.Library.Roots[0].ID != secondID || saved.Library.Roots[0].Content != model.RootSeries {
		t.Fatalf("roots = %+v", saved.Library.Roots)
	}
}

func TestSaveLibraryPathAndContentOfOneFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	media := t.TempDir()
	dir := filepath.Join(media, "Filme")
	for i, f := range firstFolder {
		writeFile(t, dir, f, "video "+string(rune('a'+i)))
	}
	writeFile(t, media, "Neu/Alien (1979)/Alien (1979).mkv", "video a")
	WithBrowseRoots([]string{media})(e.svc)
	lib := e.create(dir)
	e.scan(lib.ID)
	e.clearJobs()

	req := e.form(lib.ID)
	req.Roots[0].Path = filepath.Join(media, "Neu")
	req.Roots[0].Content, req.Roots[0].Providers, req.Roots[0].ConfirmPath = model.RootSeries, model.DefaultProviders(model.RootSeries), dir
	if _, err := e.svc.Save(e.ctx, lib.ID, req, nil); !hasFieldError(err, "roots[0].path", model.CodePathWithContent) {
		t.Fatalf("path and content together: %v, want roots[0].path path_with_content", err)
	}
	if e.str(`SELECT path FROM library_roots WHERE library_id = ?`, lib.ID) != dir || e.jobCount(JobScanLibrary) != 0 {
		t.Fatal("a refused path and content change wrote something")
	}
	// The path alone goes through, without a typed path, and queues the one scan.
	req = e.form(lib.ID)
	req.Roots[0].Path = filepath.Join(media, "Neu")
	if _, err := e.svc.Save(e.ctx, lib.ID, req, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.str(`SELECT path FROM library_roots WHERE library_id = ?`, lib.ID); got != filepath.Join(media, "Neu") {
		t.Fatalf("path = %s", got)
	}
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Fatalf("scan jobs after the path change = %d, want 1", n)
	}
}

// TestSaveLibraryOrderAndOneScan: remove, other content and a new folder on the path of the
// removed one, in one request: the order lets the new folder take the path, and one scan follows.
func TestSaveLibraryOrderAndOneScan(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, second := e.twoFolderLibrary()
	req := e.form(lib.ID)
	firstID, secondID := req.Roots[0].ID, req.Roots[1].ID
	req.Remove = []model.LibrarySaveRemoval{{ID: firstID, ConfirmPath: first}}
	req.Roots = []model.LibrarySaveRoot{
		{ID: secondID, ConfirmPath: second, LibraryRootRequest: root(second, model.RootSeries)},
		{LibraryRootRequest: root(first, model.RootSeries)},
	}
	saved, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino"))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Library.Roots) != 2 || saved.Library.Name != "Kino" {
		t.Fatalf("saved = %+v", saved.Library)
	}
	newID := rootIDOf(t, e, lib.ID, first)
	if newID == firstID || newID == secondID {
		t.Fatalf("the new folder on %s has the id %d of a stored one", first, newID)
	}
	if got := e.str(`SELECT content FROM library_roots WHERE id = ?`, newID); got != model.RootSeries {
		t.Fatalf("content of the new folder = %s", got)
	}
	if got := e.count(`SELECT count(*) FROM media_files WHERE root_id IN (?, ?)`, firstID, secondID); got != 0 {
		t.Fatalf("media files of the removed and re-read folders = %d, want 0", got)
	}
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Fatalf("scan jobs = %d, want exactly 1", n)
	}
}

// TestSaveLibraryRollsBackEverything: the last step fails (another library took the new
// folder's path inside the same transaction), and neither the removal, nor the content change,
// nor the name is written.
func TestSaveLibraryRollsBackEverything(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, second := e.twoFolderLibrary()
	other, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Andere", Kind: model.LibraryPodcasts})
	if err != nil {
		t.Fatal(err)
	}
	fresh := tree(t, []string{"Up (2009)/Up (2009).mkv"})
	titles := e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL`)
	req := e.form(lib.ID)
	firstID, secondID := req.Roots[0].ID, req.Roots[1].ID
	req.Remove = []model.LibrarySaveRemoval{{ID: firstID, ConfirmPath: first}}
	req.Roots = []model.LibrarySaveRoot{
		{ID: secondID, ConfirmPath: second, LibraryRootRequest: root(second, model.RootSeries)},
		{LibraryRootRequest: root(fresh, model.RootMovie)},
	}
	settings := func(ctx context.Context, tx *sql.Tx) error {
		if err := renameTo(lib.ID, "Kino")(ctx, tx); err != nil {
			return err
		}
		_, err := insertRoot(ctx, appq.New(tx), other.ID, preparedRoot{path: fresh, content: model.RootPodcast, providers: []string{model.ProviderLocal}, fsType: "ext4", watch: "inotify"}, 1)
		return err
	}
	if _, err := e.svc.Save(e.ctx, lib.ID, req, settings); !hasFieldError(err, "roots[1].path", model.CodeRootOverlap) {
		t.Fatalf("err = %v, want roots[1].path root_overlap", err)
	}
	if e.libraryName(lib.ID) != "Dokus" || e.count(`SELECT count(*) FROM library_roots WHERE library_id = ? AND content = 'movie'`, lib.ID) != 2 ||
		e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, other.ID) != 0 ||
		e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL`) != titles || e.jobCount(JobScanLibrary) != 0 {
		t.Fatal("a failed save left something behind")
	}
	// The settings step failing writes nothing either.
	boom := errors.New("boom")
	if _, err := e.svc.Save(e.ctx, lib.ID, req, func(context.Context, *sql.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, lib.ID) != 2 {
		t.Fatal("a failed settings step left the removal behind")
	}
}

func TestSaveLibraryWhileAScanRuns(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, second := e.twoFolderLibrary()
	unlock, err := e.svc.lockScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	// Removal, other content, other path: the whole save is refused and writes nothing.
	removal := e.form(lib.ID)
	removal.Remove = []model.LibrarySaveRemoval{{ID: removal.Roots[0].ID, ConfirmPath: first}}
	removal.Roots = removal.Roots[1:]
	content := e.form(lib.ID)
	content.Roots[1].Content, content.Roots[1].Providers, content.Roots[1].ConfirmPath = model.RootSeries, model.DefaultProviders(model.RootSeries), second
	for name, req := range map[string]model.LibrarySave{"removal": removal, "content": content} {
		if _, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino")); !errors.Is(err, model.ErrScanRunning) {
			t.Errorf("%s under a scan: err = %v, want ErrScanRunning", name, err)
		}
	}
	if e.libraryName(lib.ID) != "Dokus" || e.count(`SELECT count(*) FROM library_roots WHERE library_id = ? AND content = 'movie'`, lib.ID) != 2 {
		t.Fatal("a refused save under a scan wrote something")
	}
	// Name, providers, languages, images and a new folder go through during the scan.
	langs := []string{"en"}
	light := tree(t, []string{"Up (2009)/Up (2009).mkv"})
	req := e.form(lib.ID)
	req.Roots[0].Providers = []string{model.ProviderTMDB, model.ProviderAniList}
	req.Roots[1].Metadata = model.RootMetadata{TextLangs: &langs, Images: &model.RootImages{}}
	req.Roots = append(req.Roots, model.LibrarySaveRoot{LibraryRootRequest: root(light, model.RootMovie)})
	saved, err := e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino"))
	if err != nil {
		t.Fatalf("light save under a scan: %v", err)
	}
	if saved.Library.Name != "Kino" || len(saved.Library.Roots) != 3 {
		t.Fatalf("saved = %+v", saved.Library)
	}
}

// TestSaveLibraryProvidersAndMetadata: a new provider list queues matching or filling, no scan;
// new languages or images are stored and answered with the titles of the folder.
func TestSaveLibraryProvidersAndMetadata(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, _, _ := e.twoFolderLibrary()
	req := e.form(lib.ID)
	firstID, secondID := req.Roots[0].ID, req.Roots[1].ID
	langs := []string{"ja", "en"}
	req.Roots[0].Providers = []string{model.ProviderTMDB, model.ProviderAniList}
	req.Roots[1].Metadata = model.RootMetadata{TextLangs: &langs}

	saved, err := e.svc.Save(e.ctx, lib.ID, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobScanLibrary); n != 0 {
		t.Fatalf("scan jobs = %d, want none", n)
	}
	if n := e.jobCount(JobMetadataFill); n != len(firstFolder) {
		t.Fatalf("fill jobs = %d, want one per title of the first folder (%d)", n, len(firstFolder))
	}
	if got := e.str(`SELECT providers FROM library_roots WHERE id = ?`, firstID); got != `["tmdb","anilist"]` {
		t.Fatalf("providers = %s", got)
	}
	want := []model.RootTitles{{RootID: secondID, HomeTitleCount: len(secondFolder)}}
	if !slices.Equal(saved.MetadataChanged, want) {
		t.Fatalf("metadataChanged = %+v, want %+v", saved.MetadataChanged, want)
	}
	if m := saved.Library.Roots[1].Metadata; m.TextLangs == nil || !slices.Equal(*m.TextLangs, langs) {
		t.Fatalf("languages = %+v", m)
	}
	// The same form again changes nothing and names no folder; back to the default does.
	again := e.form(lib.ID)
	if saved, err = e.svc.Save(e.ctx, lib.ID, again, nil); err != nil || len(saved.MetadataChanged) != 0 {
		t.Fatalf("unchanged form: %+v, %v", saved.MetadataChanged, err)
	}
	again = e.form(lib.ID)
	again.Roots[1].Metadata = model.RootMetadata{}
	if saved, err = e.svc.Save(e.ctx, lib.ID, again, nil); err != nil || len(saved.MetadataChanged) != 1 {
		t.Fatalf("back to the default: %+v, %v", saved.MetadataChanged, err)
	}
	if e.count(`SELECT count(*) FROM library_roots WHERE id = ? AND text_langs IS NULL`, secondID) != 1 {
		t.Fatal("the languages did not go back to the default")
	}
}

// TestSaveLibraryFieldErrorsPerFolder: every folder is named by its index in the request, and
// nothing is written.
func TestSaveLibraryFieldErrorsPerFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib, first, _ := e.twoFolderLibrary()
	base := e.form(lib.ID)

	// Unknown and repeated ids come first.
	req := e.form(lib.ID)
	req.Roots = append(req.Roots, model.LibrarySaveRoot{ID: 999, LibraryRootRequest: root(t.TempDir(), model.RootMovie)})
	req.Remove = []model.LibrarySaveRemoval{{ID: base.Roots[0].ID, ConfirmPath: first}, {ID: 998, ConfirmPath: "/x"}}
	_, err := e.svc.Save(e.ctx, lib.ID, req, nil)
	for _, f := range [][2]string{{"roots[2].id", "unknown_root"}, {"roots[0].id", "duplicate"}, {"removeRoots[1].id", "unknown_root"}} {
		if !hasFieldError(err, f[0], f[1]) {
			t.Errorf("ids: %v, want %s %s", err, f[0], f[1])
		}
	}
	// Then the rules of every folder, kept and new.
	req = e.form(lib.ID)
	req.Roots[1].Providers = []string{model.ProviderAniList}
	req.Roots = append(req.Roots,
		model.LibrarySaveRoot{LibraryRootRequest: root("relativ", model.RootMovie)},
		model.LibrarySaveRoot{LibraryRootRequest: root(filepath.Join(first, "Alien (1979)"), model.RootMovie)},
		model.LibrarySaveRoot{LibraryRootRequest: root(t.TempDir(), model.RootAudiobook)},
	)
	_, err = e.svc.Save(e.ctx, lib.ID, req, renameTo(lib.ID, "Kino"))
	for _, f := range [][2]string{
		{"roots[1].providers", model.CodeProvidersNotAllowed},
		{"roots[2].path", "library_path_invalid"},
		{"roots[3].path", model.CodeRootOverlap},
		{"roots[4].content", model.CodeContentNotAllowed},
	} {
		if !hasFieldError(err, f[0], f[1]) {
			t.Errorf("rules: %v, want %s %s", err, f[0], f[1])
		}
	}
	if e.count(`SELECT count(*) FROM library_roots WHERE library_id = ?`, lib.ID) != 2 || e.libraryName(lib.ID) != "Dokus" || e.jobCount(JobScanLibrary) != 0 {
		t.Fatal("a refused save wrote something")
	}
}

// TestSaveLibraryRemovesTheLastFolder: editing may leave the library without a folder, as the
// folder removal does (E13); no scan is queued for it.
func TestSaveLibraryRemovesTheLastFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, firstFolder)
	lib := e.create(dir)
	e.scan(lib.ID)
	e.clearJobs()
	req := e.form(lib.ID)
	req.Remove = []model.LibrarySaveRemoval{{ID: req.Roots[0].ID, ConfirmPath: dir}}
	req.Roots = nil

	saved, err := e.svc.Save(e.ctx, lib.ID, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Library.Roots) != 0 || e.jobCount(JobScanLibrary) != 0 {
		t.Fatalf("roots = %+v, scan jobs = %d", saved.Library.Roots, e.jobCount(JobScanLibrary))
	}
	if n := e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL AND kind = 'movie'`); n != 0 {
		t.Fatalf("films left = %d, want 0", n)
	}
}
