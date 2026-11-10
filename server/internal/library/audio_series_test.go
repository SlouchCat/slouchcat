package library

import (
	"fmt"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// book inserts an audiobook of lib with series name and author (empty = NULL) and returns its id.
func (e *env) book(lib int64, n int, series, author string, index float64) string {
	e.t.Helper()
	id := fmt.Sprintf("01J9B00K%018d", n)
	e.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at, series_name, author, series_index)
	        VALUES (?, 'audiobook', ?, ?, ?, 1, 1, 1, NULLIF(?, ''), NULLIF(?, ''), ?)`,
		id, lib, fmt.Sprintf("Buch %d", n), fmt.Sprintf("buch %d", n), series, author, index)
	return id
}

// relink runs relinkAudioSeries with names in its own transaction.
func (e *env) relink(names audioseries.Names, ids ...string) {
	e.t.Helper()
	tx, err := e.db.Write.BeginTx(e.ctx, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := relinkAudioSeries(e.ctx, systemq.New(tx), names, ids, e.clock.Now()); err != nil {
		e.t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) seriesOf(id string) string {
	e.t.Helper()
	return e.str(`SELECT audio_series_id FROM items WHERE id = ?`, id)
}

// PD-3 with Marco's spellings (concept 19 D3): Chris Bradford's books in «Bodyguard» and «Bodyguard
// - Band» with numbers 1 to 6 are one series of six volumes named «Bodyguard»; a series of the same
// name by another author stays apart; a book without series name has none; another library holds
// its own series. A new list of suffixes changes keys, never an id (D4.4).
func TestRelinkAudioSeriesMarcosSpellings(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	other := e.createAudiobooks(t.TempDir()).ID
	var bodyguard []string
	for n := 1; n <= 6; n++ {
		series, author := "Bodyguard", "Chris Bradford"
		if n%2 == 0 {
			series, author = "Bodyguard - Band", "Chris  Bradford"
		}
		bodyguard = append(bodyguard, e.book(lib, n, series, author, float64(n)))
	}
	stranger := e.book(lib, 7, "Bodyguard", "Andere Autorin", 1)
	tolkien := []string{e.book(lib, 8, "Herr der Ringe", "J.R.R. Tolkien", 1), e.book(lib, 9, "Der Herr Der Ringe - Band", "J R R Tolkien", 2),
		e.book(lib, 10, "Herr der Ringe", "J. R. R. Tolkien, Wolfgang Krege", 3)}
	kling := []string{e.book(lib, 11, "Känguru", "Marc-Uwe Kling", 1), e.book(lib, 12, "Känguru", "Marc Uwe Kling", 2)}
	none := e.book(lib, 13, "", "Chris Bradford", 0)
	elsewhere := e.book(other, 14, "Bodyguard", "Chris Bradford", 1)
	all := append(append(append(append([]string{}, bodyguard...), tolkien...), kling...), stranger, none, elsewhere)
	e.relink(audioseries.Default(), all...)

	series := e.seriesOf(bodyguard[0])
	for _, id := range bodyguard {
		if got := e.seriesOf(id); got == "" || got != series {
			t.Errorf("Bodyguard volume %s in series %q, want %q", id, got, series)
		}
	}
	if n := e.count(`SELECT count(*) FROM items WHERE audio_series_id = ?`, series); n != 6 {
		t.Errorf("Bodyguard series has %d volumes, want 6", n)
	}
	if name := e.str(`SELECT name FROM audio_series WHERE id = ?`, series); name != "Bodyguard" {
		t.Errorf("series name %q, want Bodyguard", name)
	}
	for what, id := range map[string]string{"other author": stranger, "other library": elsewhere} {
		if got := e.seriesOf(id); got == "" || got == series {
			t.Errorf("%s: series %q, want its own", what, got)
		}
	}
	if got := e.seriesOf(none); got != "" {
		t.Errorf("book without series name in series %q", got)
	}
	for _, group := range [][]string{tolkien, kling} {
		for _, id := range group[1:] {
			if a, b := e.seriesOf(group[0]), e.seriesOf(id); a == "" || a != b {
				t.Errorf("%s in series %q, want %q", id, b, a)
			}
		}
	}
	if n := e.count(`SELECT count(*) FROM audio_series`); n != 5 {
		t.Errorf("%d series, want 5 (Bodyguard twice, the stranger, Tolkien, Kling)", n)
	}

	// A list without «band» splits the key of «Bodyguard - Band» off: those volumes go to a new
	// series, the others keep the id. A list that strips «ringe» as well renames the key of the
	// whole Tolkien series: it keeps its id, so the address of its page stays.
	ringe := e.seriesOf(tolkien[0])
	e.relink(audioseries.Parse("reihe\nringe\nband\n"), all...)
	if got := e.seriesOf(tolkien[0]); got != ringe {
		t.Errorf("Tolkien series %q after the new list, want the id %q", got, ringe)
	}
	if key := e.str(`SELECT series_key FROM audio_series WHERE id = ?`, ringe); key != "herr der"+"\x1f"+"j r r tolkien" {
		t.Errorf("Tolkien key %q after the new list", key)
	}
	if got := e.seriesOf(bodyguard[0]); got != series {
		t.Errorf("Bodyguard series %q after the new list, want the id %q", got, series)
	}
	e.relink(audioseries.Parse("reihe\n"), all...)
	if got := e.seriesOf(bodyguard[0]); got != series {
		t.Errorf("Bodyguard 1 in %q after dropping «band», want %q", got, series)
	}
	split := e.seriesOf(bodyguard[1])
	if split == series || split == "" {
		t.Errorf("Bodyguard - Band 2 in %q, want a new series", split)
	}
	// Back to the embedded list: they join the series that holds the key; the emptied one stays
	// for the sweep.
	e.relink(audioseries.Default(), all...)
	for _, id := range bodyguard {
		if got := e.seriesOf(id); got != series {
			t.Errorf("Bodyguard volume %s back in %q, want %q", id, got, series)
		}
	}
	if n := e.count(`SELECT count(*) FROM audio_series WHERE id = ?`, split); n != 1 {
		t.Errorf("emptied series deleted before the sweep")
	}
}

// The sweep deletes a series without any row in items and without locked name (D4.4); a series
// whose only book is soft-deleted and a locked series stay.
func TestSweepDeletesOrphanAudioSeries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	gone := e.book(lib, 1, "Leer", "A", 1)
	deleted := e.book(lib, 2, "Weich", "A", 1)
	locked := e.book(lib, 3, "Gesperrt", "A", 1)
	e.relink(audioseries.Default(), gone, deleted, locked)
	empty, soft, keep := e.seriesOf(gone), e.seriesOf(deleted), e.seriesOf(locked)
	e.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, deleted)
	e.exec(`UPDATE audio_series SET name_locked = 1 WHERE id = ?`, keep)
	e.exec(`UPDATE items SET series_name = NULL WHERE id IN (?, ?)`, gone, locked)
	e.relink(audioseries.Default(), gone, locked)
	if _, err := e.svc.Sweep(e.ctx); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{empty: 0, soft: 1, keep: 1} {
		if n := e.count(`SELECT count(*) FROM audio_series WHERE id = ?`, id); n != want {
			t.Errorf("series %s after the sweep: %d rows, want %d", id, n, want)
		}
	}
}

// The first start after migration 00045 links every audiobook once (soft-deleted ones too) and
// remembers it; a second start changes nothing.
func TestRelinkAudioSeriesOnce(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	a, b := e.book(lib, 1, "Bodyguard", "Chris Bradford", 1), e.book(lib, 2, "Bodyguard - Band", "Chris Bradford", 2)
	e.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, b)
	done, err := e.svc.RelinkAudioSeriesOnce(e.ctx)
	if err != nil || !done {
		t.Fatalf("first start: %v %v", done, err)
	}
	if s := e.seriesOf(a); s == "" || s != e.seriesOf(b) {
		t.Errorf("books in %q and %q, want one series", s, e.seriesOf(b))
	}
	e.exec(`UPDATE items SET audio_series_id = NULL`)
	if done, err := e.svc.RelinkAudioSeriesOnce(e.ctx); err != nil || done {
		t.Fatalf("second start: %v %v, want nothing to do", done, err)
	}
	if s := e.seriesOf(a); s != "" {
		t.Errorf("second start linked again: %q", s)
	}
}

// The scan links a book when it completes it (finishAudiobook), in the same transaction.
func TestFinishAudiobookLinksSeries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	a, b := e.book(lib, 1, "Reihe X", "A", 1), e.book(lib, 2, "Reihe X - Band", "A", 2)
	for i, id := range []string{a, b} {
		e.exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
		        VALUES (?, ?, (SELECT id FROM library_roots WHERE library_id = ?), ?, 1, 1, 'm4b', 1, 1)`,
			fmt.Sprintf("01J9F11E%018d", i), id, lib, fmt.Sprintf("x/%d.m4b", i))
		if err := e.svc.finishAudiobook(e.ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if s := e.seriesOf(a); s == "" || s != e.seriesOf(b) {
		t.Errorf("finished books in %q and %q, want one series", s, e.seriesOf(b))
	}
}
