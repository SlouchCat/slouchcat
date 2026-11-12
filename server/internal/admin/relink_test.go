package admin

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// ADR 0023 point 3 with the relink of package library: the preview lists the deleted film with
// both its live copies, a fresh one and one that lived beside it for a month (the maintenance
// action knows no freshness rule, decision 21), with both titles and the user data left on the
// deleted film. An id that is no ULID refuses the request before anything moves. The relink keeps
// the deleted film's id; the same pair twice counts once, and the second copy, the pair turned
// round and a pair with an unknown id (a copy merged meanwhile) are skipped.
func TestAdminRelinkItems(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	m.svc.WithLibraryMaintenance(library.New(f.DB, jobs.NewQueue(f.DB.Write, nil), transcode.Runner{}, vistest.Logger()))
	dead, live, old := f.Rated(16), f.Rated(18), f.Rated(12)
	deletedAt := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	m.exec(t, `UPDATE items SET tmdb_id = 424242, tvdb_id = NULL, imdb_id = NULL WHERE id IN (?, ?, ?)`, dead.ID, live.ID, old.ID)
	m.exec(t, `UPDATE items SET title = 'Heat', year = 1995 WHERE id = ?`, dead.ID)
	m.exec(t, `UPDATE items SET title = 'Heat (Remaster)', year = 1995 WHERE id = ?`, live.ID)
	m.exec(t, `UPDATE items SET title = 'Heat Copy', year = NULL WHERE id = ?`, old.ID)
	m.exec(t, `UPDATE items SET deleted_at = ? WHERE id = ?`, deletedAt.UnixMilli(), dead.ID)
	m.exec(t, `UPDATE items SET created_at = ? WHERE id = ?`, deletedAt.Add(24*time.Hour).UnixMilli(), live.ID)
	m.exec(t, `UPDATE items SET created_at = ? WHERE id = ?`, deletedAt.Add(-30*24*time.Hour).UnixMilli(), old.ID)
	for _, table := range []string{"progress", "watchlist", "progress_events"} {
		m.exec(t, fmt.Sprintf(`DELETE FROM %s WHERE item_id IN (?, ?, ?)`, table), dead.ID, live.ID, old.ID)
	}
	m.exec(t, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES
	           (?, ?, 1000, 7200000, 1, 10), (?, ?, 2000, 7200000, 0, 10)`, f.P18.ID, dead.ID, f.P16.ID, dead.ID)
	m.exec(t, `INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (?, ?, 1, 1)`, f.P18.ID, dead.ID)

	listed := func() map[string]model.RelinkPair {
		t.Helper()
		list, err := m.svc.ListRelinkPairs(m.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != len(list.Pairs) {
			t.Errorf("total %d, %d pairs", list.Total, len(list.Pairs))
		}
		out := map[string]model.RelinkPair{}
		for _, p := range list.Pairs {
			if p.DeletedID == dead.ID {
				out[p.LiveID] = p
			}
		}
		return out
	}
	pairs := listed()
	pair, ok := pairs[live.ID]
	if !ok || len(pairs) != 2 {
		t.Fatalf("pairs of the deleted film = %+v, want the fresh and the old copy", pairs)
	}
	if pair.Kind != "movie" || pair.LibraryID != live.LibraryID || !pair.DeletedAt.Equal(deletedAt) ||
		pair.Title != "Heat (Remaster)" || pair.Year == nil || *pair.Year != 1995 ||
		pair.DeletedTitle != "Heat" || pair.DeletedYear == nil || *pair.DeletedYear != 1995 {
		t.Errorf("pair = %+v", pair)
	}
	if want := (model.RelinkUserData{Progress: 2, Watchlist: 1}); pair.UserData != want {
		t.Errorf("user data = %+v, want %+v", pair.UserData, want)
	}
	if p := pairs[old.ID]; p.Title != "Heat Copy" || p.Year != nil || p.UserData != pair.UserData {
		t.Errorf("pair of the old copy = %+v", p)
	}

	// An id that is no ULID: nothing moves.
	var verr *model.ValidationError
	_, err := m.svc.RelinkItems(m.ctx, []model.RelinkPairRef{{DeletedID: dead.ID, LiveID: live.ID}, {DeletedID: dead.ID, LiveID: "not-an-id"}})
	if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0] != (model.FieldError{Field: "pairs[1]", Code: "invalid_value"}) {
		t.Fatalf("malformed id: %v, want pairs[1] invalid_value", err)
	}
	if len(listed()) != 2 {
		t.Fatal("the refused request merged a pair")
	}
	if _, err := m.svc.RelinkItems(m.ctx, nil); !errors.As(err, &verr) {
		t.Errorf("no pair: %v, want a validation error", err)
	}

	out, err := m.svc.RelinkItems(m.ctx, []model.RelinkPairRef{
		{DeletedID: dead.ID, LiveID: live.ID}, {DeletedID: dead.ID, LiveID: live.ID}, // the same pair twice
		{DeletedID: dead.ID, LiveID: old.ID},                       // the second copy: its deleted title is back by then
		{DeletedID: live.ID, LiveID: dead.ID},                      // turned round: no pair
		{DeletedID: dead.ID, LiveID: "01ZZZZZZZZZZZZZZZZZZZZZZZZ"}, // unknown: merged meanwhile
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Relinked) != 1 || out.Relinked[0].KeptID != dead.ID || out.Relinked[0].RemovedID != live.ID ||
		out.Relinked[0].Merged != 1 || out.Skipped != 3 || out.StoppedBy != "" {
		t.Errorf("relink = %+v", out)
	}
	var state string
	if err := f.DB.Read.QueryRow(`SELECT (deleted_at IS NULL) || '/' || (SELECT count(*) FROM items WHERE id = ?) || '/' ||
	                              (SELECT deleted_at IS NULL FROM items WHERE id = ?) FROM items WHERE id = ?`,
		live.ID, old.ID, dead.ID).Scan(&state); err != nil || state != "1/0/1" {
		t.Errorf("kept film live / copy rows / old copy live = %q, %v; want 1/0/1", state, err)
	}
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM media_files WHERE item_id = ?`, dead.ID).Scan(&n); err != nil || n != 2 {
		t.Errorf("files of the kept film = %d, %v; want its own and the copy's", n, err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress WHERE item_id = ?`, dead.ID).Scan(&n); err != nil || n != 2 {
		t.Errorf("progress of the kept film = %d, %v", n, err)
	}
	if pairs := listed(); len(pairs) != 0 {
		t.Errorf("pairs still listed: %+v", pairs)
	}
}

// The preview is capped at 200 pairs, ordered by library name and title, names the libraries and
// counts the user data of the shown pairs in one call. A running scan stops the relink after the
// pairs before it, which stay merged; with nothing merged it names the library.
func TestAdminRelinkCapAndScanRunning(t *testing.T) {
	m := newMaintFixture(t)
	libs := &fakeLibraries{}
	libs.created = []model.LibraryRequest{{Name: "Serien", Kind: "video"}, {Name: "Filme", Kind: "video"}}
	m.svc.libraries = libs
	// ULID-shaped ids: a live copy starts with 1, its deleted title with 2.
	liveID := func(i int) string { return fmt.Sprintf("1%025d", i) }
	deadID := func(i int) string { return fmt.Sprintf("2%025d", i) }
	for i := range 201 {
		m.maint.pairs = append(m.maint.pairs, library.RelinkPair{
			LiveID: liveID(i), DeletedID: deadID(i), LibraryID: 1, Kind: "series", Title: fmt.Sprintf("Show %03d", 200-i),
		})
	}
	m.maint.pairs = append(m.maint.pairs, library.RelinkPair{LiveID: liveID(999), DeletedID: deadID(999), LibraryID: 2, Kind: "movie", Title: "Zulu"})
	m.maint.userData = map[string]library.RelinkUserData{deadID(999): {ProgressEvents: 140, PlaybackSessions: 3}}
	list, err := m.svc.ListRelinkPairs(m.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 202 || len(list.Pairs) != maxRelinkPairs {
		t.Errorf("total %d, %d pairs; want 202 and %d", list.Total, len(list.Pairs), maxRelinkPairs)
	}
	// «Filme» before «Serien», then by title.
	if p := list.Pairs[0]; p.LibraryName != "Filme" || p.UserData != (model.RelinkUserData{ProgressEvents: 140, PlaybackSessions: 3}) {
		t.Errorf("first pair = %+v", p)
	}
	if p := list.Pairs[1]; p.LibraryName != "Serien" || p.Title != "Show 000" || p.UserData != (model.RelinkUserData{}) {
		t.Errorf("second pair = %+v", p)
	}
	if len(m.maint.counted) != 1 || len(m.maint.counted[0]) != maxRelinkPairs {
		t.Errorf("user data counted in %d calls, want one with %d titles", len(m.maint.counted), maxRelinkPairs)
	}
	other := int64(3)
	if list, err := m.svc.ListRelinkPairs(m.ctx, &other); err != nil || list.Total != 0 || len(list.Pairs) != 0 {
		t.Errorf("library 3 = %+v, %v", list, err)
	}

	pairs := make([]model.RelinkPairRef, 0, maxRelinkPairs+1)
	for _, p := range m.maint.pairs[:201] {
		pairs = append(pairs, model.RelinkPairRef{DeletedID: p.DeletedID, LiveID: p.LiveID})
	}
	var verr *model.ValidationError
	if _, err := m.svc.RelinkItems(m.ctx, pairs); !errors.As(err, &verr) || verr.Fields[0].Code != "too_long" {
		t.Errorf("201 pairs: %v, want too_long", err)
	}
	// Merged, gone meanwhile, stopped by a scan: the first stays merged.
	m.maint.relinkErr = []error{nil, library.ErrNotTwins, model.ErrScanRunning}
	out, err := m.svc.RelinkItems(m.ctx, pairs[:4])
	if err != nil || len(out.Relinked) != 1 || out.Skipped != 1 || out.StoppedBy != "scan_running" || len(m.maint.relinked) != 3 {
		t.Errorf("scan running = %+v, %v after %v", out, err, m.maint.relinked)
	}
	// Nothing merged: the error names the library.
	m.maint.relinked, m.maint.relinkErr = nil, []error{model.ErrScanRunning}
	var sr *model.ScanRunningError
	if _, err := m.svc.RelinkItems(m.ctx, []model.RelinkPairRef{{DeletedID: deadID(999), LiveID: liveID(999)}}); !errors.As(err, &sr) ||
		sr.Library != "Filme" || !errors.Is(err, model.ErrScanRunning) {
		t.Errorf("scan running before any merge = %v, want the library «Filme»", err)
	}
}
