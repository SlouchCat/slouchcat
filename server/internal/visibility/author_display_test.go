package visibility

import (
	"context"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// PD-13 (concept 19 D6.4), the reading side: with items.author_display as
// library.RecomputeAuthorDisplay writes it, the titles show, sort by author and are found as
// «J. R. R. Tolkien», while items.author keeps the files' spellings; the series tile and page
// show the unified author too.
func TestAuthorDisplayShownSortedSearched(t *testing.T) {
	t.Parallel()
	f, r := seriesFixture(t)
	ctx := context.Background()
	books, lib := f.Audio.Books, f.Audio.AudiobookLibraryID
	const unified = "J. R. R. Tolkien"
	// The raw spellings sort apart («J R R …» < «J Rz» < «J.R.R. …»); the unified ones together after «J Rz».
	for _, b := range []struct {
		book            vistest.Movie
		author, display string
	}{
		{books[0], "J R R Tolkien", unified}, {books[1], "J Rz", ""}, {books[2], "J.R.R. Tolkien", unified},
		{books[3], "Zora", ""}, {books[4], "J. R. R. Tolkien", ""}, {books[5], "Anna", ""},
	} {
		run(t, f, `UPDATE items SET author = ?, author_display = NULLIF(?, '') WHERE id = ?`, b.author, b.display, b.book.ID)
	}
	run(t, f, `UPDATE items SET author = NULL WHERE id = ?`, f.Audio.Parts.ID)

	page := libraryItems(t, r, f.P18, lib, model.LibraryQuery{Sort: model.LibrarySortAuthor})
	var authors []string
	for _, it := range page.Items {
		if it.Author != "" {
			authors = append(authors, it.Author)
		}
	}
	if want := []string{"Anna", "J Rz", unified, unified, unified, "Zora"}; !slices.Equal(authors, want) {
		t.Errorf("authors sorted by author = %q, want %q", authors, want)
	}
	var raw string
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT author FROM items WHERE id = ?`, books[2].ID).Scan(&raw); err != nil || raw != "J.R.R. Tolkien" {
		t.Errorf("items.author = %q (%v), want the files' spelling", raw, err)
	}

	res := search(t, r, f, f.P18, model.SearchQuery{Text: unified, MaxRating: -1})
	var found []string
	for _, it := range res.Items {
		if it.Author == unified {
			found = append(found, it.ID)
		}
	}
	slices.Sort(found)
	want := []string{books[0].ID, books[2].ID, books[4].ID}
	slices.Sort(want)
	if !slices.Equal(found, want) {
		t.Errorf("search %q found %v as %q, want %v", unified, found, unified, want)
	}
	d, err := r.Item(ctx, Profile(f.P18), books[2].ID)
	if err != nil || d.Author != unified {
		t.Errorf("detail author %q (%v), want %q", d.Author, err, unified)
	}
	groups, err := r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{MaxRating: -1})
	if g := groupByID(groups, vistest.AudioSeriesID); err != nil || g == nil || g.Author != unified {
		t.Errorf("series tile %+v (%v), want the author %q", g, err, unified)
	}
}

// D6.4: with the switch metadata.books.normalize_series off the series shows the most common
// spelling of its visible volumes with the suffix; on (and absent) without.
func TestAudioSeriesNameSwitch(t *testing.T) {
	t.Parallel()
	f, r := seriesFixture(t)
	ctx := context.Background()
	run(t, f, `UPDATE items SET series_name = 'Reihe - Band' WHERE id = ?`, f.Audio.Books[0].ID)
	for _, c := range []struct{ value, want string }{{"", vistest.AudioSeriesName}, {"0", "Reihe - Band"}, {"1", vistest.AudioSeriesName}} {
		if c.value != "" {
			run(t, f, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, 1)
			           ON CONFLICT (key) DO UPDATE SET value = excluded.value`, audioseries.SettingNormalizeSeries, c.value)
		}
		page, err := r.AudioSeries(ctx, Profile(f.P18), f.Audio.AudiobookLibraryID, vistest.AudioSeriesID)
		if err != nil || page.Name != c.want {
			t.Errorf("switch %q: series name %q (%v), want %q", c.value, page.Name, err, c.want)
		}
		groups, err := r.AudioGroups(ctx, Profile(f.P18), f.Audio.AudiobookLibraryID, model.LibraryQuery{MaxRating: -1})
		if g := groupByID(groups, vistest.AudioSeriesID); err != nil || g == nil || g.Name != c.want {
			t.Errorf("switch %q: series tile %+v (%v), want %q", c.value, g, err, c.want)
		}
	}
}
