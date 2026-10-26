package visibility

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// D3 (M11): TrailerFlags answers every asked id, true only for a visible title with a stored
// trailer. AddSeries stores trailers for the films rated 6 and 18 and for the series; the film
// rated 12 has none, and P6 may not see the film rated 18.
func TestTrailerFlags(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m6, m12, m18, unknown := f.Rated(6).ID, f.Rated(12).ID, f.Rated(18).ID, "01J9000000000000000000NONE"
	asked := []string{m6, m12, m18, f.Show.Series.ID, unknown}

	got, err := r.TrailerFlags(ctx, Profile(f.P18), asked)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{m6: true, m12: false, m18: true, f.Show.Series.ID: true, unknown: false}
	if !maps.Equal(got, want) {
		t.Errorf("P18 flags = %v, want %v", got, want)
	}
	got, err = r.TrailerFlags(ctx, Profile(f.P6), asked)
	if err != nil {
		t.Fatal(err)
	}
	want = map[string]bool{m6: true, m12: false, m18: false, f.Show.Series.ID: false, unknown: false}
	if !maps.Equal(got, want) {
		t.Errorf("P6 flags = %v, want %v (titles it may not see count as without trailer)", got, want)
	}
	if got, err := r.TrailerFlags(ctx, Profile(f.P18), nil); err != nil || len(got) != 0 {
		t.Errorf("no ids: %v, %v", got, err)
	}
	if _, err := r.TrailerFlags(ctx, System("test"), asked); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: err = %v, want ErrUnscoped", err)
	}
}
