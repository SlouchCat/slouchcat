package library

import (
	"database/sql"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// filmLibrary creates a movie library with twelve films, so the loss of one stays below the
// threshold, and scans it.
func (e *env) filmLibrary() (string, model.Library) {
	e.t.Helper()
	dir := tree(e.t, twentyMovies[4:16])
	lib := e.create(dir)
	e.scan(lib.ID)
	return dir, lib
}

// addFilm writes the file rel, scans, gives its new film the TMDB id tmdb and returns the film.
func (e *env) addFilm(dir string, libID int64, rel string, tmdb int64) string {
	e.t.Helper()
	writeFile(e.t, dir, rel, "release "+rel)
	e.scan(libID)
	id := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, rel)
	e.exec(`UPDATE items SET tmdb_id = ? WHERE id = ?`, tmdb, id)
	return id
}

// dropFilm removes the file rel; the scan marks it missing and the sweep a day later soft-deletes
// its film. The clock moves on a minute after the sweep.
func (e *env) dropFilm(dir string, libID int64, rel string) {
	e.t.Helper()
	remove(e.t, dir, rel)
	e.scan(libID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		e.t.Fatalf("sweep = %d, %v; want the film", n, err)
	}
	e.clock.Advance(time.Minute)
}

// A film with a year comes back by (sort_title, year) within 30 days when no live film has the
// key; a live film with the same key wins over the deleted one.
func TestMovieRelinkByTitleAndYear(t *testing.T) {
	t.Parallel()
	for _, liveTwin := range []bool{false, true} {
		t.Run(map[bool]string{false: "deleted film", true: "live film wins"}[liveTwin], func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			dir, lib := e.filmLibrary()
			writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "first release")
			e.scan(lib.ID)
			heat := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995)/Heat (1995).mkv'`)
			e.dropFilm(dir, lib.ID, "Heat (1995)/Heat (1995).mkv")
			want := heat
			if liveTwin {
				want = e.addFilm(dir, lib.ID, "Other (1995)/Other (1995).mkv", 1)
				e.exec(`UPDATE items SET title = 'Heat', sort_title = (SELECT sort_title FROM items WHERE id = ?) WHERE id = ?`, heat, want)
			}
			e.clock.Advance(10 * 24 * time.Hour)
			writeFile(t, dir, "Heat (1995) Remux/Heat (1995).mkv", "second release")
			e.scan(lib.ID)
			if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995) Remux/Heat (1995).mkv'`); v != want {
				t.Errorf("new file joined %q, want %q", v, want)
			}
			if v := e.str(`SELECT (deleted_at IS NULL) FROM items WHERE id = ?`, heat); (v == "1") != !liveTwin {
				t.Errorf("deleted film live = %s, want %v", v, !liveTwin)
			}
		})
	}
}

// Audiobooks and podcasts come back by provider id only, never by (sort_title, year).
func TestDeletedByTitleOnlyFilmsAndSeries(t *testing.T) {
	t.Parallel()
	idx := newItemIndex()
	year := sql.NullInt64{Int64: 2001, Valid: true}
	for _, kind := range []string{"audiobook", "podcast", "movie"} {
		idx.addDeleted(systemq.Item{ID: kind, Kind: kind, SortTitle: kind, Year: year}, true)
	}
	for kind, want := range map[string]string{"audiobook": "", "podcast": "", "movie": "movie"} {
		if got := idx.find(titleInfo{sortTitle: kind, year: year}, ""); got != want {
			t.Errorf("%s by title = %q, want %q", kind, got, want)
		}
	}
}
