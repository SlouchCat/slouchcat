package library

import (
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Step 0 of concept 20 stage 5 (section 10.2): what a scan does with a file under a title that
// was taken out of Slouchcat while its files stay on the disk. Three changes in the folder, each
// for a film and a series: a new file (a second version of the film, a new episode of the
// series), the file renamed, and an upgrade (the file replaced by one of another resolution).
//
// takenOutMode says how the title left. "soft-deleted" is what the database knew before stage 5:
// deleted_at on the title and every title below it, as the sweep, the cleanup or a removed
// folder leave it, with the media_files rows kept. Its outcome is the behaviour of today: a
// renamed file follows its row and the title stays deleted; a new file brings the title back with
// its user data when the deletion is less than titleRelinkWindow old (by name and year, ADR 0023,
// as here: the folders carry no provider id), and after that it makes a second title of the same
// name beside the deleted one.
type takenOutCase struct {
	name string
	// change acts on the folder; the scan runs after it.
	change func(t *testing.T, dir string)
	// file is the rel path of the file whose title the test follows after the scan.
	file string
}

// takenOutMode is one way a title leaves Slouchcat with its files kept.
type takenOutMode struct {
	name string
	mark func(e *env, titleID string)
	// outcome says per case what becomes of the title.
	outcome map[string]takenOutcome
}

// takenOutcome is what a scan makes of a title that was taken out.
type takenOutcome int

const (
	// stays: the file belongs to the title (or a title below it), which stays taken out.
	stays takenOutcome = iota
	// revives: the file belongs to the title, which is live again.
	revives
	// doubles: the file belongs to a new live title of the same name; the old one stays out.
	doubles
)

// softDeleteTree soft-deletes a title and every title below it at the given age, files kept.
func softDeleteTree(age time.Duration) func(e *env, titleID string) {
	return func(e *env, titleID string) {
		at := e.clock.Now().Add(-age).UnixMilli()
		e.exec(`UPDATE items SET deleted_at = ?, updated_at = ?
		        WHERE id = ? OR parent_id = ? OR parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = ?)`,
			at, at, titleID, titleID, titleID)
	}
}

// removeTitle takes the title out of Slouchcat as the administrator does («Nur aus Slouchcat
// entfernen», concept 20 section 10.2): the rule of stage 5 is that the title stays out whatever
// happens in its folder, and a new file there joins it, taken out as well.
func removeTitle(e *env, titleID string) {
	e.t.Helper()
	title := e.str(`SELECT title FROM items WHERE id = ?`, titleID)
	if _, err := e.deleteItem(titleID, "de", model.DeleteModeLibrary, title, model.Actor{}); err != nil {
		e.t.Fatalf("remove %s: %v", titleID, err)
	}
}

var takenOutModes = []takenOutMode{
	{name: "removed", mark: removeTitle, outcome: map[string]takenOutcome{
		"film new version": stays, "film renamed": stays, "film upgrade": stays,
		"series new episode": stays, "series renamed": stays, "series upgrade": stays,
	}},
	{name: "soft-deleted now", mark: softDeleteTree(0), outcome: map[string]takenOutcome{
		"film new version": revives, "film renamed": stays, "film upgrade": revives,
		"series new episode": revives, "series renamed": stays, "series upgrade": revives,
	}},
	{name: "soft-deleted 31 days ago", mark: softDeleteTree(31 * 24 * time.Hour), outcome: map[string]takenOutcome{
		"film new version": doubles, "film renamed": stays, "film upgrade": doubles,
		"series new episode": doubles, "series renamed": stays, "series upgrade": doubles,
	}},
}

const (
	takenFilm   = "Collateral (2004)/Collateral (2004) - 1080p.mkv"
	takenFolder = "Collateral (2004)"
	takenShow   = "Night Shift (2019)"
	takenE01    = takenShow + "/Season 01/Night Shift - S01E01 - 1080p.mkv"
)

func TestScanUnderTakenOutTitle(t *testing.T) {
	t.Parallel()
	films := []takenOutCase{
		{name: "film new version", file: takenFolder + "/Collateral (2004) - 2160p.mkv", change: func(t *testing.T, dir string) {
			writeFile(t, dir, takenFolder+"/Collateral (2004) - 2160p.mkv", "second version")
		}},
		{name: "film renamed", file: takenFolder + "/Collateral (2004) - Bluray-1080p.mkv", change: func(t *testing.T, dir string) {
			rename(t, dir, takenFilm, takenFolder+"/Collateral (2004) - Bluray-1080p.mkv")
		}},
		{name: "film upgrade", file: takenFolder + "/Collateral (2004) - 2160p.mkv", change: func(t *testing.T, dir string) {
			remove(t, dir, takenFilm)
			writeFile(t, dir, takenFolder+"/Collateral (2004) - 2160p.mkv", "upgrade with another size")
		}},
	}
	series := []takenOutCase{
		{name: "series new episode", file: takenShow + "/Season 01/Night Shift - S01E03.mkv", change: func(t *testing.T, dir string) {
			writeFile(t, dir, takenShow+"/Season 01/Night Shift - S01E03.mkv", "new episode")
		}},
		{name: "series renamed", file: takenShow + "/Season 01/Night Shift - S01E01 - Pilot.mkv", change: func(t *testing.T, dir string) {
			rename(t, dir, takenE01, takenShow+"/Season 01/Night Shift - S01E01 - Pilot.mkv")
		}},
		{name: "series upgrade", file: takenShow + "/Season 01/Night Shift - S01E01 - 2160p.mkv", change: func(t *testing.T, dir string) {
			remove(t, dir, takenE01)
			writeFile(t, dir, takenShow+"/Season 01/Night Shift - S01E01 - 2160p.mkv", "upgrade with another size")
		}},
	}
	for _, mode := range takenOutModes {
		for _, tc := range films {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				dir := tree(t, append(append([]string{}, twentyMovies[4:16]...), takenFilm))
				lib := e.create(dir)
				e.scan(lib.ID)
				film := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, takenFilm)
				mode.mark(e, film)
				tc.change(t, dir)
				e.scan(lib.ID)
				e.checkTakenOut(film, tc.file, mode.outcome[tc.name])
			})
		}
		for _, tc := range series {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				files := []string{takenE01, takenShow + "/Season 01/Night Shift - S01E02.mkv"}
				for _, show := range []string{"Keep One (2001)", "Keep Two (2002)", "Keep Three (2003)"} {
					for _, ep := range []string{"S01E01", "S01E02", "S01E03"} {
						files = append(files, show+"/Season 01/Keep - "+ep+".mkv")
					}
				}
				dir := tree(t, files)
				lib := e.createSeries(dir)
				e.scan(lib.ID)
				show := e.str(`SELECT s.parent_id FROM media_files AS m JOIN items AS ep ON ep.id = m.item_id
				               JOIN items AS s ON s.id = ep.parent_id WHERE m.rel_path = ?`, takenE01)
				mode.mark(e, show)
				tc.change(t, dir)
				e.scan(lib.ID)
				e.checkTakenOut(show, tc.file, mode.outcome[tc.name])
			})
		}
	}
}

// checkTakenOut checks where the file went after the scan and whether the title is live.
func (e *env) checkTakenOut(titleID, file string, want takenOutcome) {
	e.t.Helper()
	owner := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, file)
	if owner == "" {
		e.t.Fatalf("file %s has no row", file)
	}
	under := e.count(`SELECT count(*) FROM items AS i
	                  WHERE i.id = ? AND (i.id = ? OR i.parent_id = ? OR i.parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = ?))`,
		owner, titleID, titleID, titleID) == 1
	kind := e.str(`SELECT kind FROM items WHERE id = ?`, titleID)
	titles := e.count(`SELECT count(*) FROM items WHERE kind = ? AND title = (SELECT title FROM items WHERE id = ?)`, kind, titleID)
	live := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, titleID) == 1
	ownerLive := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL`, owner) == 1
	var got takenOutcome
	switch {
	case under && titles == 1 && live && ownerLive:
		got = revives
	case under && titles == 1 && !live && !ownerLive:
		got = stays
		// Taken out (stage 5): the file's title carries the stamp of the title, so it comes back
		// with it; a soft-deleted title has none.
		if e.count(`SELECT count(*) FROM items WHERE id = ? AND removed_at IS NOT NULL`, titleID) == 1 &&
			e.count(`SELECT count(*) FROM items AS o JOIN items AS t ON t.id = ? WHERE o.id = ? AND o.removed_at = t.removed_at`, titleID, owner) != 1 {
			e.t.Errorf("the title of %s is not taken out with the title", file)
		}
	case !under && titles == 2 && !live && ownerLive:
		got = doubles
	default:
		e.t.Fatalf("file %s: under the title %v, %d titles of the name, title live %v, its file's title live %v",
			file, under, titles, live, ownerLive)
	}
	if got != want {
		e.t.Errorf("outcome %v, want %v", got, want)
	}
}

// Without a year in the folder name the name does not find a title (only the folder does): a new
// file in the folder of a removed film or series still joins it, through the folder rule of find.
func TestRemovedTitleKeepsItsFolderWithoutYear(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kept, added string
		series            bool
	}{
		{name: "film", kept: "Heat/Heat - 1080p.mkv", added: "Heat/Heat - 2160p.mkv"},
		{name: "series", kept: "Nightly/Season 01/Nightly - S01E01.mkv", added: "Nightly/Season 02/Nightly - S02E01.mkv", series: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			var lib model.Library
			var title string
			if tc.series {
				files := []string{tc.kept}
				for _, ep := range []string{"S01E01", "S01E02", "S01E03", "S01E04", "S01E05"} {
					files = append(files, "Keep (2001)/Season 01/Keep - "+ep+".mkv")
				}
				dir := tree(t, files)
				lib = e.createSeries(dir)
				e.scan(lib.ID)
				title = e.str(`SELECT s.parent_id FROM media_files AS m JOIN items AS ep ON ep.id = m.item_id
				               JOIN items AS s ON s.id = ep.parent_id WHERE m.rel_path = ?`, tc.kept)
				removeTitle(e, title)
				writeFile(t, dir, tc.added, "new season")
			} else {
				dir := tree(t, append(append([]string{}, twentyMovies[4:16]...), tc.kept))
				lib = e.create(dir)
				e.scan(lib.ID)
				title = e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, tc.kept)
				removeTitle(e, title)
				writeFile(t, dir, tc.added, "another version")
			}
			e.scan(lib.ID)
			e.checkTakenOut(title, tc.added, stays)
		})
	}
}
