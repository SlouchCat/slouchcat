package discover

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// fakeRadio gives the row radio the stations of a profile (by profile id) or an error.
type fakeRadio struct {
	mu       sync.Mutex
	stations map[int64][]model.RadioHomeStation
	err      error
	calls    int
}

func (r *fakeRadio) HomeStations(_ context.Context, p model.Profile) ([]model.RadioHomeStation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.stations[p.ID], r.err
}

// radioFixture is the fixture with the row radio shown first and a fake radio with one favourite
// of P6.
func radioFixture(t *testing.T) (*Service, *fakeRadio, *vistest.Fixture) {
	t.Helper()
	f := discoverFixture(t)
	exec(t, f, `UPDATE home_rows SET hidden = 0, position = -1 WHERE id = 'radio'`)
	r := &fakeRadio{stations: map[int64][]model.RadioHomeStation{}}
	r.stations[f.P6.ID] = []model.RadioHomeStation{{Station: model.RadioStation{ID: 9002, Name: "Fixture Radio DE", LogoHash: "abc"},
		Now: &model.RadioNowEntry{Text: "Band - Song", Artist: "Band", Title: "Song"}}}
	return New(f.DB, f.CacheDir, vistest.Logger(), WithRadio(r)), r, f
}

// The row radio («Meine Sender», concept 16 section 10.2) stands at its stored place with the
// favourites of the profile; without favourites, hidden, without a radio service or on an error
// of the radio it is left out, and the rest of the page stays (critic m-8).
func TestHomeRadioRow(t *testing.T) {
	t.Parallel()
	s, r, f := radioFixture(t)
	ctx := context.Background()
	page, err := s.Home(ctx, f.P6, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) < 2 || page.Rows[0].ID != RowRadio || len(page.Rows[0].Stations) != 1 || page.Rows[0].Stations[0].Station.ID != 9002 {
		t.Fatalf("rows %v, first %+v; want radio first with the favourite", rowIDs(page.Rows), page.Rows[0])
	}
	others := rowIDs(page.Rows[1:])

	if page, err := s.Home(ctx, f.P0, nil); err != nil || slices.Contains(rowIDs(page.Rows), RowRadio) {
		t.Errorf("P0 without favourites: rows %v, %v", rowIDs(page.Rows), err)
	}
	r.mu.Lock()
	r.err = errors.New("radio index broken")
	r.mu.Unlock()
	page, err = s.Home(ctx, f.P6, nil)
	if err != nil || !slices.Equal(rowIDs(page.Rows), others) {
		t.Errorf("radio error: rows %v, %v; want %v", rowIDs(page.Rows), err, others)
	}
	if page, err := New(f.DB, f.CacheDir, vistest.Logger()).Home(ctx, f.P6, nil); err != nil || !slices.Equal(rowIDs(page.Rows), others) {
		t.Errorf("without a radio service: rows %v, %v", rowIDs(page.Rows), err)
	}
	r.mu.Lock()
	r.err, r.calls = nil, 0
	r.mu.Unlock()
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE id = 'radio'`)
	if page, err := s.Home(ctx, f.P6, nil); err != nil || slices.Contains(rowIDs(page.Rows), RowRadio) || r.calls != 0 {
		t.Errorf("hidden row: rows %v, %d calls, %v", rowIDs(page.Rows), r.calls, err)
	}
}

// The row radio counts towards the 15 rows: behind 15 filled rows it is cut, ahead of them it
// takes a place.
func TestHomeRadioRowCountsTowardsTheCap(t *testing.T) {
	t.Parallel()
	r := &fakeRadio{stations: map[int64][]model.RadioHomeStation{}}
	var s *Service
	s, _, f := rowsFixture(t, func(ctx context.Context, _ int, setting model.HomeRowSetting) (model.HomeRow, error) {
		if setting.Key == RowRadio {
			return s.row(ctx, model.Profile{}, setting)
		}
		return model.HomeRow{ID: setting.ID, Key: setting.Key, Items: []model.ItemSummary{{ID: "x"}}}, nil
	})
	s.radio = r
	r.stations[0] = []model.RadioHomeStation{{Station: model.RadioStation{ID: 1}}}
	ctx := context.Background()
	exec(t, f, `UPDATE home_rows SET position = 100000 WHERE id = 'radio'`)
	page, err := s.page(ctx, model.Profile{}, false)
	if err != nil || len(page.Rows) != MaxRows || slices.Contains(rowIDs(page.Rows), RowRadio) {
		t.Errorf("radio behind the cut: %d rows %v, %v", len(page.Rows), rowIDs(page.Rows), err)
	}
	exec(t, f, `UPDATE home_rows SET position = -1 WHERE id = 'radio'`)
	page, err = s.page(ctx, model.Profile{}, false)
	if err != nil || len(page.Rows) != MaxRows || page.Rows[0].ID != RowRadio {
		t.Errorf("radio first: %d rows %v, %v", len(page.Rows), rowIDs(page.Rows), err)
	}
}

// FirstRow hands the row radio to the prepaint when it comes first and the page has no stage.
func TestFirstRowRadio(t *testing.T) {
	t.Parallel()
	s, _, f := radioFixture(t)
	exec(t, f, `UPDATE home_rows SET hidden = 1 WHERE id = 'spotlight'`)
	row, err := s.FirstRow(context.Background(), f.P6)
	if err != nil || row.Key != RowRadio || len(row.Stations) != 1 {
		t.Fatalf("first row %+v, %v; want radio", row, err)
	}
	if row, _ := s.FirstRow(context.Background(), f.P0); row.Key == RowRadio {
		t.Error("P0 without favourites gets the row radio first")
	}
}
