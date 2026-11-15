package visibility

import (
	"context"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Minimum ages (concept 22 stage 3, F-17): a PG-13 film is 13, an R film 17. A profile «ab 12»
// no longer sees the PG-13 film (it saw it as FSK 12 before), «ab 13» and «ab 16» do; the R film
// needs «ab 17». The same film in a French household carries the French «-12» and is 12 again, so
// a French «ab 12» sees it (the visibility layer compares ages, the rating country decides the
// value). Facets list the stored ages and the age filter means age ≤ value.
func TestMinimumAgesInTheLibrary(t *testing.T) {
	t.Parallel()
	f := libraryFixture(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	pg13, rated := f.Rated(12), f.Rated(16)
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE items SET own_rating = 13, rating_source = 'tmdb:US' WHERE id = ?`, pg13.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE items SET own_rating = 17, rating_source = 'tmdb:US' WHERE id = ?`, rated.ID); err != nil {
		t.Fatal(err)
	}
	add := func(name string, age int) model.Profile {
		p, err := appq.New(f.DB.Write).InsertProfile(ctx, appq.InsertProfileParams{AccountID: f.Account.ID, Name: name, MaxRating: int64(age),
			Language: "de", AllLibraries: 1, AudioLanguages: "[]", SubtitleMode: "forced", SubtitleLanguages: "[]", Now: 1})
		if err != nil {
			t.Fatal(err)
		}
		return model.Profile{ID: p.ID, AccountID: f.Account.ID, Name: name, MaxRating: age, Language: "de", AllLibraries: true, LibraryIDs: []int64{}}
	}
	p13, p17 := add("P13", 13), add("P17", 17)
	sees := func(p model.Profile, id string) bool {
		return slices.Contains(ids(libraryItems(t, r, p, f.LibraryID(), model.LibraryQuery{}).Items), id)
	}
	for _, tc := range []struct {
		p    model.Profile
		id   string
		want bool
	}{
		{f.P12, pg13.ID, false}, {p13, pg13.ID, true}, {f.P16, pg13.ID, true},
		{f.P16, rated.ID, false}, {p17, rated.ID, true}, {f.P18, rated.ID, true},
	} {
		if got := sees(tc.p, tc.id); got != tc.want {
			t.Errorf("%s sees the film %s = %v, want %v", tc.p.Name, tc.id, got, tc.want)
		}
	}
	if got := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{}).Facets.Ratings; !slices.Equal(got, []int{0, 6, 12, 13, 17, 18}) {
		t.Errorf("P18 ratings facet = %v, want 0 6 12 (the series) 13 17 18", got)
	}
	if got := libraryItems(t, r, p13, f.LibraryID(), model.LibraryQuery{}).Facets.Ratings; !slices.Equal(got, []int{0, 6, 12, 13}) {
		t.Errorf("P13 ratings facet = %v, want 0 6 12 13", got)
	}
	// The age filter «bis 13» is age ≤ 13: the PG-13 film yes, the R film no.
	page := libraryItems(t, r, f.P18, f.LibraryID(), model.LibraryQuery{MaxRating: 13})
	if got := ids(page.Items); !slices.Contains(got, pg13.ID) || slices.Contains(got, rated.ID) {
		t.Errorf("P18 maxRating 13 = %v, want the PG-13 film and not the R film", got)
	}
	// The French household: the film's own certificate «-12» makes it 12, «ab 12» sees it.
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE items SET own_rating = 12, rating_source = 'tmdb:FR' WHERE id = ?`, pg13.ID); err != nil {
		t.Fatal(err)
	}
	if !sees(f.P12, pg13.ID) {
		t.Error("a profile «ab 12» in France does not see the film rated -12")
	}
}
