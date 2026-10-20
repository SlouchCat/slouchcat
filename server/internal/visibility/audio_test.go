package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// addBook inserts an audiobook with the given rating into library 1 with three parts (the second
// one missing) inserted out of order, and three book-wide chapters.
func addBook(t *testing.T, db *store.DB, id string, rating int, rootID int64) (parts []string) {
	t.Helper()
	ctx := context.Background()
	run := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Write.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	run(`INSERT INTO items (id, kind, library_id, title, sort_title, own_rating, rating_source, author, narrator, series_name,
	     series_index, duration_ms, metadata_state, added_at, created_at, updated_at)
	     VALUES (?, 'audiobook', 1, 'Buch', 'buch', ?, 'library', 'Autorin', 'Sprecher', 'Reihe', 2.5, 30000, 'matched', 1, 1, 1)`, id, rating)
	files := []struct {
		id, rel string
		index   int
		missing bool
	}{
		{id[:24] + "F3", id + "/03.mp3", 3, false},
		{id[:24] + "F1", id + "/01.mp3", 1, false},
		{id[:24] + "F2", id + "/02.mp3", 2, true},
	}
	for _, f := range files {
		run(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, audio_codec, duration_ms, part_index, created_at, updated_at)
		     VALUES (?, ?, ?, ?, 10, 1, 'mp3', 'mp3', 10000, ?, 1, 1)`, f.id, id, rootID, f.rel, f.index)
		if f.missing {
			run(`UPDATE media_files SET missing_since = 2 WHERE id = ?`, f.id)
		}
	}
	run(`INSERT INTO chapters (item_id, idx, title, start_ms, end_ms, media_file_id, file_start_ms) VALUES
	     (?, 1, 'Zwei', 10000, 20000, ?, 0), (?, 0, 'Eins', 0, 10000, ?, 0), (?, 2, 'Drei', 20000, 30000, ?, 0)`,
		id, files[2].id, id, files[1].id, id, files[0].id)
	return []string{files[1].id, files[0].id}
}

func TestAudiobookChaptersAndParts(t *testing.T) {
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	const book, adult = "01J9000000000000000000BOOK", "01J9000000000000000000AD18"
	wantParts := addBook(t, f.DB, book, 6, f.RootID())
	addBook(t, f.DB, adult, 18, f.RootID())

	chapters, err := r.Chapters(ctx, Profile(f.P6), book)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.Chapter{{StartMs: 0, EndMs: 10000, Title: "Eins"}, {StartMs: 10000, EndMs: 20000, Title: "Zwei"}, {StartMs: 20000, EndMs: 30000, Title: "Drei"}}
	if !slices.Equal(chapters, want) {
		t.Errorf("chapters %+v, want %+v", chapters, want)
	}
	parts, err := r.Parts(ctx, Profile(f.P6), book)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range parts {
		ids = append(ids, p.File.ID)
		if p.RootPath != f.MediaDir || p.ItemID != book || !p.File.Available || p.File.DurationMs != 10000 {
			t.Errorf("part %+v", p)
		}
	}
	if !slices.Equal(ids, wantParts) {
		t.Errorf("parts %v, want %v (part order, missing part left out)", ids, wantParts)
	}

	d, err := r.Item(ctx, Profile(f.P6), book)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Chapters, want) || d.Author != "Autorin" || d.Narrator != "Sprecher" || d.SeriesName != "Reihe" ||
		d.SeriesIndex == nil || *d.SeriesIndex != 2.5 || d.DurationMs == nil || *d.DurationMs != 30000 {
		t.Errorf("detail %+v", d)
	}

	// A book the profile may not see: 404 for chapters and parts alike, and nothing for the system scope.
	for _, p := range []model.Profile{f.P0, f.P16} {
		if _, err := r.Chapters(ctx, Profile(p), adult); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s chapters of an 18 book: %v", p.Name, err)
		}
		if _, err := r.Parts(ctx, Profile(p), adult); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s parts of an 18 book: %v", p.Name, err)
		}
	}
	if _, err := r.Chapters(ctx, Profile(f.P0), book); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P0 chapters of a 6 book: %v", err)
	}
	if _, err := r.Parts(ctx, System("test"), book); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v", err)
	}
	// A visible title without chapters: an empty list.
	if c, err := r.Chapters(ctx, Profile(f.P18), f.Rated(0).ID); err != nil || len(c) != 0 {
		t.Errorf("film chapters %v %v", c, err)
	}
}
