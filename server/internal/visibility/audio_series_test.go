package visibility

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func seriesFixture(t *testing.T) (*vistest.Fixture, *Reader) {
	t.Helper()
	f := vistest.New(t)
	f.AddAudio(t)
	f.AddAudioSeries(t)
	return f, New(f.DB, vistest.Logger())
}

func run(t *testing.T, f *vistest.Fixture, q string, args ...any) {
	t.Helper()
	if _, err := f.DB.Write.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func groupByID(l model.AudioGroupList, id string) *model.AudioGroup {
	for i := range l.Groups {
		if l.Groups[i].ID == id {
			return &l.Groups[i]
		}
	}
	return nil
}

// PD-4: a series of volumes rated 0 (volume 1), 12 (volume 2) and 18 (volume 0.5). P12 sees two
// volumes and the cover of a visible one; P6 sees no series in the row, the page is
// model.ErrNotFound and the detail of its visible volume has no seriesId; P18 sees all three. The
// name is the most common spelling without suffix («Reihe - Band» counts as «Reihe»).
func TestAudioSeriesVisibility(t *testing.T) {
	t.Parallel()
	f, r := seriesFixture(t)
	ctx := context.Background()
	lib, books := f.Audio.AudiobookLibraryID, f.Audio.Books
	noFilter := model.LibraryQuery{MaxRating: -1}

	for _, c := range []struct {
		p      model.Profile
		count  int
		cover  string
		detail string
	}{{f.P18, 3, books[4].ID, vistest.AudioSeriesID}, {f.P12, 2, books[0].ID, vistest.AudioSeriesID}, {f.P6, 0, "", ""}} {
		l, err := r.AudioGroups(ctx, Profile(c.p), lib, noFilter)
		if err != nil {
			t.Fatal(err)
		}
		g := groupByID(l, vistest.AudioSeriesID)
		switch {
		case c.count == 0 && g != nil:
			t.Errorf("%s sees the series with one visible volume: %+v", c.p.Name, g)
		case c.count > 0 && (g == nil || g.Count != c.count || g.Matching != c.count || g.Name != vistest.AudioSeriesName ||
			g.Kind != model.AudioGroupSeries || g.Cover == nil || g.Cover.ID != c.cover || g.Author != "Autorin"):
			t.Errorf("%s: series tile %+v, want %d volumes and the cover %s", c.p.Name, g, c.count, c.cover)
		}
		page, err := r.AudioSeries(ctx, Profile(c.p), lib, vistest.AudioSeriesID)
		switch {
		case c.count == 0 && !errors.Is(err, model.ErrNotFound):
			t.Errorf("%s: series page %v, want not found", c.p.Name, err)
		case c.count > 0 && (err != nil || len(page.Volumes) != c.count || page.Cover == nil || page.Cover.ID != c.cover):
			t.Errorf("%s: series page %+v (%v)", c.p.Name, page, err)
		}
		d, err := r.Item(ctx, Profile(c.p), books[0].ID)
		if err != nil || d.SeriesID != c.detail || d.LibraryID != lib {
			t.Errorf("%s: detail seriesId %q libraryId %d (%v), want %q and %d", c.p.Name, d.SeriesID, d.LibraryID, err, c.detail, lib)
		}
	}
	// Of another library, unknown: not found.
	if _, err := r.AudioSeries(ctx, Profile(f.P18), f.Audio.PodcastLibraryID, vistest.AudioSeriesID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("series of another library: %v", err)
	}
	// A library the profile does not see is not found; one without audiobooks has no groups.
	if _, err := r.AudioGroups(ctx, Profile(f.P18), 999, noFilter); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
	if l, err := r.AudioGroups(ctx, Profile(f.P18), f.Audio.PodcastLibraryID, noFilter); err != nil || len(l.Groups) != 0 {
		t.Errorf("podcast library groups %+v (%v)", l, err)
	}
}

// The page orders the volumes by number (0.5 before 1), books without number last by title, and
// keeps two books with the same number; «Weiterhören» is the first volume not completed, else the
// first; the narrator only when all volumes share one; spellings with counts, the most common first.
func TestAudioSeriesPageOrder(t *testing.T) {
	t.Parallel()
	f, r := seriesFixture(t)
	ctx := context.Background()
	lib, b := f.Audio.AudiobookLibraryID, f.Audio.Books
	// Volumes for P18: AJ 0.5, A0 1, AC 2; A6 joins with number 1 (a radio play), AG and AP without
	// number (titles «… AG» and «… AP» order them).
	run(t, f, `UPDATE items SET audio_series_id = ?, series_name = 'Reihe', series_index = 1, narrator = 'Ensemble' WHERE id = ?`, vistest.AudioSeriesID, b[1].ID)
	run(t, f, `UPDATE items SET audio_series_id = ?, series_name = 'Reihe', series_index = NULL WHERE id IN (?, ?)`, vistest.AudioSeriesID, b[3].ID, f.Audio.Parts.ID)
	page, err := r.AudioSeries(ctx, Profile(f.P18), lib, vistest.AudioSeriesID)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, v := range page.Volumes {
		order = append(order, v.Item.ID)
	}
	first := []string{b[4].ID, b[0].ID, b[1].ID}
	if b[1].ID < b[0].ID {
		first = []string{b[4].ID, b[1].ID, b[0].ID}
	}
	want := append(first, b[2].ID, b[3].ID, f.Audio.Parts.ID)
	if !slices.Equal(order, want) {
		t.Errorf("volume order %v, want %v", order, want)
	}
	if page.Volumes[0].SeriesIndex == nil || *page.Volumes[0].SeriesIndex != 0.5 || page.Volumes[5].SeriesIndex != nil {
		t.Errorf("volume numbers %v … %v", page.Volumes[0].SeriesIndex, page.Volumes[5].SeriesIndex)
	}
	if page.Next == nil || page.Next.ID != b[4].ID || page.Narrator != "" || page.Name != "Reihe" {
		t.Errorf("next %v, narrator %q, name %q", page.Next, page.Narrator, page.Name)
	}
	if len(page.Spellings) != 2 || page.Spellings[0] != (model.AudioSeriesSpelling{Name: "Reihe", Count: 5}) ||
		page.Spellings[1] != (model.AudioSeriesSpelling{Name: "Reihe - Band", Count: 1}) {
		t.Errorf("spellings %+v", page.Spellings)
	}
	if page.DurationMs != 5*vistest.DurationMs+3*vistest.DurationMs {
		t.Errorf("duration %d", page.DurationMs)
	}
	// The first two completed: next is the third; all completed: the first.
	run(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (?, ?, 0, 1, 1, 1, 1), (?, ?, 0, 1, 1, 1, 1)`,
		f.P18.ID, order[0], f.P18.ID, order[1])
	if page, err = r.AudioSeries(ctx, Profile(f.P18), lib, vistest.AudioSeriesID); err != nil || page.Next.ID != order[2] {
		t.Errorf("next after two completed: %v (%v), want %s", page.Next, err, order[2])
	}
	l, err := r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{MaxRating: -1})
	if g := groupByID(l, vistest.AudioSeriesID); err != nil || g == nil || g.Completed != 2 || g.Count != 6 {
		t.Errorf("tile %+v (%v), want 2 of 6 completed", g, err)
	}
	for _, id := range order[2:] {
		run(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (?, ?, 0, 1, 1, 1, 1)`, f.P18.ID, id)
	}
	if page, err = r.AudioSeries(ctx, Profile(f.P18), lib, vistest.AudioSeriesID); err != nil || page.Next.ID != order[0] {
		t.Errorf("next with all completed: %v (%v), want the first", page.Next, err)
	}
	// A locked name wins over the spellings; one narrator for all volumes is named.
	run(t, f, `UPDATE audio_series SET name = 'Fest', name_locked = 1 WHERE id = ?`, vistest.AudioSeriesID)
	run(t, f, `UPDATE items SET narrator = 'Eine' WHERE audio_series_id = ?`, vistest.AudioSeriesID)
	if page, err = r.AudioSeries(ctx, Profile(f.P18), lib, vistest.AudioSeriesID); err != nil || page.Name != "Fest" || !page.NameLocked || page.Narrator != "Eine" {
		t.Errorf("locked name %q %v, narrator %q (%v)", page.Name, page.NameLocked, page.Narrator, err)
	}
}

// PD-6: a manual collection whose visible members are all audiobooks appears in the row of its
// library and not in the start page row «Sammlungen», but still on the detail page of its members;
// a collection with a film as well stays on the start page and is no audiobook collection. The
// filters work as on the library page: a group appears when one of its volumes passes, Matching
// counts them; the groups are sorted by name.
func TestAudioCollectionsAndFilters(t *testing.T) {
	t.Parallel()
	f, r := seriesFixture(t)
	ctx := context.Background()
	lib, b := f.Audio.AudiobookLibraryID, f.Audio.Books
	const mixed = "01J9000000000000000000CMMX"
	run(t, f, `INSERT INTO collections (id, source, name, created_at, updated_at) VALUES (?, 'manual', 'Aaa Gemischt', 1, 1)`, mixed)
	run(t, f, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 0, 1), (?, ?, 1, 1)`,
		mixed, b[0].ID, mixed, f.Rated(0).ID)

	l, err := r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{MaxRating: -1})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range l.Groups {
		names = append(names, g.Name)
	}
	if !slices.Equal(names, []string{vistest.AudioCollectionName, vistest.AudioSeriesName}) || l.SeriesCount != 1 || l.CollectionCount != 1 {
		t.Errorf("groups %v (%d series, %d collections)", names, l.SeriesCount, l.CollectionCount)
	}
	if c := groupByID(l, vistest.AudioCollectionID); c == nil || c.Kind != model.AudioGroupCollection || c.Count != 2 || c.Cover == nil || c.Cover.ID != b[0].ID {
		t.Errorf("audio collection %+v", c)
	}
	home, err := r.Collections(ctx, Profile(f.P18), MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, c := range home {
		ids = append(ids, c.ID)
	}
	if slices.Contains(ids, vistest.AudioCollectionID) || !slices.Contains(ids, mixed) {
		t.Errorf("home collections %v: want the mixed one and not the audiobook one", ids)
	}
	d, err := r.Item(ctx, Profile(f.P18), b[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var shelf []string
	for _, c := range d.Collections {
		shelf = append(shelf, c.ID)
	}
	if !slices.Contains(shelf, vistest.AudioCollectionID) || !slices.Contains(shelf, mixed) {
		t.Errorf("detail collections %v, want both", shelf)
	}
	// The audiobook collection holds A0 and the book of parts, both rated 0: P0 sees it too.
	if l, err := r.AudioGroups(ctx, Profile(f.P0), lib, model.LibraryQuery{MaxRating: -1}); err != nil || groupByID(l, vistest.AudioCollectionID) == nil {
		t.Errorf("P0 groups %+v (%v)", l, err)
	}

	// Filters: a year only volume 2 (rated 12) has; the series tile says 1 of 3, the collection
	// has no passing volume and is left out.
	run(t, f, `UPDATE items SET year = 1999 WHERE id = ?`, b[2].ID)
	l, err = r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{YearFrom: 1999, YearTo: 1999, MaxRating: -1})
	if err != nil {
		t.Fatal(err)
	}
	if g := groupByID(l, vistest.AudioSeriesID); len(l.Groups) != 1 || g == nil || g.Matching != 1 || g.Count != 3 || l.CollectionCount != 0 {
		t.Errorf("year filter: %+v", l)
	}
	// maxRating 6 hides volume 2: nothing passes; status started passes the collection's started book.
	if l, err = r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{YearFrom: 1999, YearTo: 1999, MaxRating: 6}); err != nil || len(l.Groups) != 0 {
		t.Errorf("maxRating filter: %+v (%v)", l, err)
	}
	run(t, f, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 5, 100, 0, 1)`, f.P18.ID, f.Audio.Parts.ID)
	l, err = r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{MaxRating: -1, Status: []string{model.LibraryStatusStarted}})
	if c := groupByID(l, vistest.AudioCollectionID); err != nil || len(l.Groups) != 1 || c == nil || c.Matching != 1 || c.Count != 2 {
		t.Errorf("status filter: %+v (%v)", l, err)
	}
	var verr *model.ValidationError
	if _, err := r.AudioGroups(ctx, Profile(f.P18), lib, model.LibraryQuery{MaxRating: -1, Status: []string{"x"}}); !errors.As(err, &verr) {
		t.Errorf("invalid status: %v", err)
	}

	// A collection with books in two libraries: with a filter Matching counts the passing members
	// of both, like Count, and the tile appears only where a passing member is.
	other, err := appq.New(f.DB.Write).InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Hörbücher 2", Kind: model.LibraryAudiobooks,
		DefaultRating: sql.NullInt64{Valid: true}, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	run(t, f, `UPDATE items SET library_id = ? WHERE id = ?`, other.ID, f.Audio.Parts.ID)
	run(t, f, `UPDATE items SET year = 1888 WHERE id IN (?, ?)`, b[0].ID, f.Audio.Parts.ID)
	year := model.LibraryQuery{YearFrom: 1888, YearTo: 1888, MaxRating: -1}
	for _, l := range []int64{lib, other.ID} {
		groups, err := r.AudioGroups(ctx, Profile(f.P18), l, year)
		if c := groupByID(groups, vistest.AudioCollectionID); err != nil || c == nil || c.Matching != 2 || c.Count != 2 {
			t.Errorf("library %d: collection %+v (%v), want 2 of 2", l, c, err)
		}
	}
	run(t, f, `UPDATE items SET year = 1999 WHERE id = ?`, b[0].ID)
	if groups, err := r.AudioGroups(ctx, Profile(f.P18), lib, year); err != nil || groupByID(groups, vistest.AudioCollectionID) != nil {
		t.Errorf("collection without a passing member here: %+v (%v)", groups, err)
	}
	groups, err := r.AudioGroups(ctx, Profile(f.P18), other.ID, year)
	if c := groupByID(groups, vistest.AudioCollectionID); err != nil || c == nil || c.Matching != 1 || c.Count != 2 {
		t.Errorf("other library: collection %+v (%v), want 1 of 2", c, err)
	}
}
