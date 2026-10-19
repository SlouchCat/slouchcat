package store

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
)

// The derived columns effective_rating, available and min_child_rating are maintained only by
// triggers (plan 3.2). These tests pin their semantics on a series → season → episode tree and on
// the view visible_items.

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), Options{ConfigDir: t.TempDir(), Version: "test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func queryInt(t *testing.T, db *sql.DB, query string, args ...any) sql.NullInt64 {
	t.Helper()
	var v sql.NullInt64
	if err := db.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func val(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: true} }

var null = sql.NullInt64{}

const (
	idSeries  = "01SERIES00000000000000000A"
	idSeason  = "01SEASON00000000000000000A"
	idEp1     = "01EPISODE000000000000000A1"
	idEp2     = "01EPISODE000000000000000A2"
	idMovie   = "01MOVIE000000000000000000A"
	idRootDir = 1
)

func seedTree(t *testing.T, db *sql.DB) {
	t.Helper()
	mustExec(t, db, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'TV', 'series', 0, 0)`)
	mustExec(t, db, `INSERT INTO library_roots (id, library_id, path, created_at) VALUES (1, 1, '/media/tv', 0)`)
	mustExec(t, db, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`)
	mustExec(t, db, `INSERT INTO profiles (id, account_id, name, max_rating, created_at, updated_at) VALUES (18, 1, 'P18', 18, 0, 0)`)
	mustExec(t, db, `INSERT INTO profiles (id, account_id, name, max_rating, created_at, updated_at) VALUES (6, 1, 'P6', 6, 0, 0)`)
	mustExec(t, db, `INSERT INTO profiles (id, account_id, name, max_rating, created_at, updated_at) VALUES (12, 1, 'P12', 12, 0, 0)`)
	item := `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, own_rating, added_at, created_at, updated_at)
	         VALUES (?, ?, ?, 1, ?, ?, ?, 0, 0, 0)`
	mustExec(t, db, item, idSeries, "series", nil, "Show", "show", 12)
	mustExec(t, db, item, idSeason, "season", idSeries, "Season 1", "season 1", nil)
	mustExec(t, db, item, idEp1, "episode", idSeason, "E1", "e1", nil)
	mustExec(t, db, item, idEp2, "episode", idSeason, "E2", "e2", 16)
	mustExec(t, db, item, idMovie, "movie", nil, "Film", "film", 6)
}

func addFile(t *testing.T, db *sql.DB, id, itemID, rel string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	                 VALUES (?, ?, ?, ?, 1, 0, 'mkv', 0, 0)`, id, itemID, idRootDir, rel)
}

func effective(t *testing.T, db *sql.DB, id string) sql.NullInt64 {
	t.Helper()
	return queryInt(t, db, `SELECT effective_rating FROM items WHERE id = ?`, id)
}

func available(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	return queryInt(t, db, `SELECT available FROM items WHERE id = ?`, id).Int64
}

func minChild(t *testing.T, db *sql.DB, id string) sql.NullInt64 {
	t.Helper()
	return queryInt(t, db, `SELECT min_child_rating FROM items WHERE id = ?`, id)
}

func visible(t *testing.T, db *sql.DB, profile int64, id string) bool {
	t.Helper()
	return queryInt(t, db, `SELECT count(*) FROM visible_items WHERE profile_id = ? AND item_id = ?`, profile, id).Int64 == 1
}

func TestTriggersInheritRatings(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)

	// Season and episode 1 inherit 12 from the series; episode 2 keeps its stricter 16 (AN-9).
	if got := effective(t, db.Write, idSeason); got != val(12) {
		t.Errorf("season effective = %v, want 12", got)
	}
	if got := effective(t, db.Write, idEp1); got != val(12) {
		t.Errorf("episode 1 effective = %v, want 12", got)
	}
	if got := effective(t, db.Write, idEp2); got != val(16) {
		t.Errorf("episode 2 effective = %v, want 16", got)
	}

	// Raising the series to 16 reaches every descendant in the same statement.
	mustExec(t, db.Write, `UPDATE items SET own_rating = 16 WHERE id = ?`, idSeries)
	if got := effective(t, db.Write, idEp1); got != val(16) {
		t.Errorf("after raise: episode 1 effective = %v, want 16", got)
	}

	// An unknown series rating makes every descendant unknown, even with an own rating.
	mustExec(t, db.Write, `UPDATE items SET own_rating = NULL WHERE id = ?`, idSeries)
	if got := effective(t, db.Write, idEp2); got != null {
		t.Errorf("unknown parent: episode 2 effective = %v, want NULL", got)
	}
}

func TestTriggersAvailabilityAndVisibility(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)

	// Without files nothing is available or visible, not even for P18.
	if available(t, db.Write, idSeries) != 0 || visible(t, db.Write, 18, idSeries) {
		t.Fatal("series must be unavailable without files")
	}

	addFile(t, db.Write, "01FILE0000000000000000000A", idEp1, "Show/Season 01/E1.mkv")
	if available(t, db.Write, idEp1) != 1 {
		t.Fatal("episode 1 must be available after its file was added")
	}
	if available(t, db.Write, idSeason) != 1 || available(t, db.Write, idSeries) != 1 {
		t.Fatal("season and series must become available through the chain")
	}
	if got := minChild(t, db.Write, idSeries); got != val(12) {
		t.Errorf("series min_child_rating = %v, want 12", got)
	}

	// P18 and P12 see the series (12), P6 does not; nobody sees episode 2 (no file).
	if !visible(t, db.Write, 18, idSeries) || !visible(t, db.Write, 12, idSeries) || visible(t, db.Write, 6, idSeries) {
		t.Error("series visibility by rating is wrong")
	}
	if !visible(t, db.Write, 12, idEp1) {
		t.Error("episode 1 (12) must be visible for P12: off-by-one")
	}
	if visible(t, db.Write, 18, idEp2) {
		t.Error("episode 2 without file must not be visible")
	}

	// Raising the series to 16 hides series, season and episode for P12 at once (10.2).
	mustExec(t, db.Write, `UPDATE items SET own_rating = 16 WHERE id = ?`, idSeries)
	for _, id := range []string{idSeries, idSeason, idEp1} {
		if visible(t, db.Write, 12, id) {
			t.Errorf("%s must be hidden for P12 after the series went to 16", id)
		}
		if !visible(t, db.Write, 18, id) {
			t.Errorf("%s must stay visible for P18", id)
		}
	}
	mustExec(t, db.Write, `UPDATE items SET own_rating = 12 WHERE id = ?`, idSeries)
	if !visible(t, db.Write, 12, idEp1) {
		t.Error("episode 1 must be visible again for P12 after the series went back to 12")
	}

	// A missing file takes the leaf and, as the only child, the whole chain out of visibility.
	mustExec(t, db.Write, `UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, idEp1)
	if available(t, db.Write, idSeries) != 0 || visible(t, db.Write, 18, idSeries) {
		t.Error("series must be unavailable once its only file is missing")
	}
	mustExec(t, db.Write, `UPDATE media_files SET missing_since = NULL WHERE item_id = ?`, idEp1)
	if available(t, db.Write, idSeries) != 1 {
		t.Error("series must return when the file reappears")
	}

	// Soft delete of the episode also empties the container.
	mustExec(t, db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, idEp1)
	if available(t, db.Write, idSeason) != 0 || visible(t, db.Write, 18, idSeason) {
		t.Error("season must be unavailable after its only episode was soft-deleted")
	}
}

func TestTriggersMovieAndLibraryRights(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)
	addFile(t, db.Write, "01FILE0000000000000000000B", idMovie, "Film/Film.mkv")

	if !visible(t, db.Write, 6, idMovie) || !visible(t, db.Write, 18, idMovie) {
		t.Fatal("movie rated 6 must be visible for P6 and P18")
	}
	// Unknown rating hides the movie from everyone under 18 (AN-1).
	mustExec(t, db.Write, `UPDATE items SET own_rating = NULL WHERE id = ?`, idMovie)
	if visible(t, db.Write, 6, idMovie) || visible(t, db.Write, 12, idMovie) || !visible(t, db.Write, 18, idMovie) {
		t.Error("unknown rating must be visible for P18 only")
	}
	// A profile restricted to library rights sees nothing from a library it has no right to.
	mustExec(t, db.Write, `UPDATE profiles SET all_libraries = 0 WHERE id = 18`)
	if visible(t, db.Write, 18, idMovie) {
		t.Error("profile without library right must not see the movie")
	}
	mustExec(t, db.Write, `INSERT INTO profile_libraries (profile_id, library_id) VALUES (18, 1)`)
	if !visible(t, db.Write, 18, idMovie) {
		t.Error("profile with library right must see the movie")
	}
}

const (
	idSeasonExtra = "01EXTRA0000000000000000SEA"
	idSeriesExtra = "01EXTRA0000000000000000SER"
)

// Migration 00008: extras never make a container available and never lower its smallest child
// rating, but they inherit the ratings of their parent chain like every other child.
func TestTriggersExtrasAreNoContent(t *testing.T) {
	db := openTestDB(t)
	seedTree(t, db.Write)
	item := `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, own_rating, extra_kind, added_at, created_at, updated_at)
	         VALUES (?, 'extra', ?, 1, ?, ?, ?, 'featurette', 0, 0, 0)`
	mustExec(t, db.Write, item, idSeasonExtra, idSeason, "Making of", "making of", 0) // own rating 0, below the series
	mustExec(t, db.Write, item, idSeriesExtra, idSeries, "Trailer", "trailer", nil)

	// An extra with a file alone: the season and the series stay unavailable and invisible.
	addFile(t, db.Write, "01FILE000000000000000000X1", idSeasonExtra, "Show/Season 01/Featurettes/Making of.mkv")
	addFile(t, db.Write, "01FILE000000000000000000X2", idSeriesExtra, "Show/Trailers/Trailer.mkv")
	if available(t, db.Write, idSeasonExtra) != 1 {
		t.Fatal("the extra with a file must be available itself")
	}
	if available(t, db.Write, idSeason) != 0 || available(t, db.Write, idSeries) != 0 || visible(t, db.Write, 18, idSeries) {
		t.Error("an extra alone must not make season or series available")
	}
	if minChild(t, db.Write, idSeason) != null || minChild(t, db.Write, idSeries) != null {
		t.Errorf("min_child_rating with extras only: season %v, series %v, want NULL", minChild(t, db.Write, idSeason), minChild(t, db.Write, idSeries))
	}

	// With an episode the season counts that episode only: the extra (effective 12) does not
	// lower the minimum below the episode's 16.
	addFile(t, db.Write, "01FILE000000000000000000E2", idEp2, "Show/Season 01/E2.mkv")
	if available(t, db.Write, idSeason) != 1 || minChild(t, db.Write, idSeason) != val(16) {
		t.Errorf("season with E2: available %d, min_child_rating %v, want 1 and 16", available(t, db.Write, idSeason), minChild(t, db.Write, idSeason))
	}
	if visible(t, db.Write, 12, idSeason) {
		t.Error("the season whose only episode is rated 16 must stay hidden for P12 despite its extra")
	}

	// Inheritance: the extras follow the series' rating (the season extra keeps max(own, parent)).
	if effective(t, db.Write, idSeasonExtra) != val(12) || effective(t, db.Write, idSeriesExtra) != val(12) {
		t.Errorf("extra ratings = %v, %v, want 12", effective(t, db.Write, idSeasonExtra), effective(t, db.Write, idSeriesExtra))
	}
	mustExec(t, db.Write, `UPDATE items SET own_rating = 16 WHERE id = ?`, idSeries)
	if effective(t, db.Write, idSeasonExtra) != val(16) || effective(t, db.Write, idSeriesExtra) != val(16) {
		t.Errorf("after raising the series: extras %v, %v, want 16", effective(t, db.Write, idSeasonExtra), effective(t, db.Write, idSeriesExtra))
	}
	if visible(t, db.Write, 12, idSeriesExtra) || !visible(t, db.Write, 18, idSeriesExtra) {
		t.Error("the series extra must follow the raise: hidden for P12, visible for P18")
	}
}

// Migration 00010: a series counts its seasons with the stricter of their own rating and their
// smallest episode rating, so a series whose episodes are all above a level is above it too, and
// an extra is visible only below a visible parent.
func TestTriggersAllEpisodesAboveTheLevel(t *testing.T) {
	t.Run("all episodes above the level hide the series and the season", func(t *testing.T) {
		db := openTestDB(t)
		seedTree(t, db.Write)
		mustExec(t, db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, extra_kind, added_at, created_at, updated_at)
		                       VALUES (?, 'extra', ?, 1, 'Making of', 'making of', 'featurette', 0, 0, 0)`, idSeriesExtra, idSeries)
		addFile(t, db.Write, "01FILE000000000000000000E1", idEp1, "Show/Season 01/E1.mkv")
		addFile(t, db.Write, "01FILE000000000000000000E2", idEp2, "Show/Season 01/E2.mkv")
		addFile(t, db.Write, "01FILE000000000000000000X2", idSeriesExtra, "Show/Trailers/Making of.mkv")
		for _, id := range []string{idSeries, idSeason, idEp1, idSeriesExtra} {
			if !visible(t, db.Write, 12, id) {
				t.Fatalf("%s must be visible for P12 while episode 1 is rated 12", id)
			}
		}

		// Every episode at 16 (mass rating): season and series are above 12, and so is the extra.
		mustExec(t, db.Write, `UPDATE items SET own_rating = 16 WHERE kind = 'episode'`)
		if minChild(t, db.Write, idSeason) != val(16) || minChild(t, db.Write, idSeries) != val(16) {
			t.Errorf("min_child_rating season %v, series %v, want 16 and 16", minChild(t, db.Write, idSeason), minChild(t, db.Write, idSeries))
		}
		for _, id := range []string{idSeries, idSeason, idEp1, idEp2, idSeriesExtra} {
			if visible(t, db.Write, 12, id) {
				t.Errorf("%s must be hidden for P12 when every episode is rated 16", id)
			}
			if !visible(t, db.Write, 18, id) {
				t.Errorf("%s must stay visible for P18", id)
			}
		}

		// One episode back at 12: the whole chain returns.
		mustExec(t, db.Write, `UPDATE items SET own_rating = NULL WHERE id = ?`, idEp1)
		if minChild(t, db.Write, idSeries) != val(12) {
			t.Errorf("series min_child_rating = %v, want 12", minChild(t, db.Write, idSeries))
		}
		for _, id := range []string{idSeries, idSeason, idEp1, idSeriesExtra} {
			if !visible(t, db.Write, 12, id) {
				t.Errorf("%s must be visible for P12 again", id)
			}
		}

		// A hard-deleted episode recomputes season and series through the chain.
		mustExec(t, db.Write, `DELETE FROM items WHERE id = ?`, idEp1)
		if minChild(t, db.Write, idSeason) != val(16) || minChild(t, db.Write, idSeries) != val(16) {
			t.Errorf("after delete: min_child_rating season %v, series %v, want 16 and 16", minChild(t, db.Write, idSeason), minChild(t, db.Write, idSeries))
		}
	})

	t.Run("extras of a hidden, unavailable or deleted parent are hidden", func(t *testing.T) {
		db := openTestDB(t)
		seedTree(t, db.Write)
		const idMovieExtra = "01EXTRA0000000000000000MOV"
		item := `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, extra_kind, added_at, created_at, updated_at)
		         VALUES (?, 'extra', ?, 1, ?, ?, 'trailer', 0, 0, 0)`
		mustExec(t, db.Write, item, idMovieExtra, idMovie, "Teaser", "teaser")
		mustExec(t, db.Write, item, idSeriesExtra, idSeries, "Trailer", "trailer")
		addFile(t, db.Write, "01FILE000000000000000000X1", idMovieExtra, "Film/Trailers/Teaser.mkv")
		addFile(t, db.Write, "01FILE000000000000000000X2", idSeriesExtra, "Show/Trailers/Trailer.mkv")

		// The film has no file yet and the series no episode: both extras are hidden.
		if visible(t, db.Write, 18, idMovieExtra) || visible(t, db.Write, 18, idSeriesExtra) {
			t.Error("extras of unavailable parents must be hidden")
		}
		addFile(t, db.Write, "01FILE0000000000000000000M", idMovie, "Film/Film.mkv")
		addFile(t, db.Write, "01FILE000000000000000000E1", idEp1, "Show/Season 01/E1.mkv")
		if !visible(t, db.Write, 6, idMovieExtra) || !visible(t, db.Write, 12, idSeriesExtra) {
			t.Error("extras must be visible below their visible parents")
		}
		// A series whose only episode is above 12 hides its extra for P12.
		mustExec(t, db.Write, `UPDATE items SET own_rating = 16 WHERE id = ?`, idEp1)
		if visible(t, db.Write, 12, idSeriesExtra) || !visible(t, db.Write, 18, idSeriesExtra) {
			t.Error("the extra of a series hidden by its episodes must be hidden for P12 only")
		}
		mustExec(t, db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, idMovie)
		if visible(t, db.Write, 18, idMovieExtra) {
			t.Error("the extra of a soft-deleted film must be hidden")
		}
		mustExec(t, db.Write, `UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, idEp1)
		if visible(t, db.Write, 18, idSeriesExtra) {
			t.Error("the extra of an unavailable series must be hidden")
		}
	})
}
