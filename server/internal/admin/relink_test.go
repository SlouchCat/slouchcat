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
// its live copy and the user data left on it; a pair that is not listed is refused before
// anything moves; the relink keeps the deleted film's id, and the same pair twice is skipped.
func TestAdminRelinkItems(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	m.svc.WithLibraryMaintenance(library.New(f.DB, jobs.NewQueue(f.DB.Write, nil), transcode.Runner{}, vistest.Logger()))
	dead, live := f.Rated(16), f.Rated(18)
	m.exec(t, `UPDATE items SET tmdb_id = 424242, tvdb_id = NULL, imdb_id = NULL WHERE id IN (?, ?)`, dead.ID, live.ID)
	m.exec(t, `UPDATE items SET deleted_at = ? WHERE id = ?`, time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC).UnixMilli(), dead.ID)
	for _, table := range []string{"progress", "watchlist", "favorites", "progress_events"} {
		m.exec(t, fmt.Sprintf(`DELETE FROM %s WHERE item_id IN (?, ?)`, table), dead.ID, live.ID)
	}
	m.exec(t, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES
	           (?, ?, 1000, 7200000, 1, 10), (?, ?, 2000, 7200000, 0, 10)`, f.P18.ID, dead.ID, f.P16.ID, dead.ID)
	m.exec(t, `INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (?, ?, 1)`, f.P18.ID, dead.ID)

	find := func() (model.RelinkPair, bool) {
		t.Helper()
		list, err := m.svc.ListRelinkPairs(m.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != len(list.Pairs) {
			t.Errorf("total %d, %d pairs", list.Total, len(list.Pairs))
		}
		for _, p := range list.Pairs {
			if p.DeletedID == dead.ID {
				return p, true
			}
		}
		return model.RelinkPair{}, false
	}
	pair, ok := find()
	if !ok {
		t.Fatal("the deleted film is not listed")
	}
	if pair.LiveID != live.ID || pair.Kind != "movie" || pair.LibraryID != live.LibraryID || !pair.DeletedAt.Equal(time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("pair = %+v", pair)
	}
	if want := (model.RelinkUserData{Progress: 2, Watchlist: 1}); pair.UserData != want {
		t.Errorf("user data = %+v, want %+v", pair.UserData, want)
	}

	// The pair turned round is not listed: nothing moves.
	var verr *model.ValidationError
	_, _, err := m.svc.RelinkItems(m.ctx, []model.RelinkPairRef{{DeletedID: dead.ID, LiveID: live.ID}, {DeletedID: live.ID, LiveID: dead.ID}})
	if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0] != (model.FieldError{Field: "pairs[1]", Code: "invalid_value"}) {
		t.Fatalf("pair turned round: %v, want pairs[1] invalid_value", err)
	}
	if _, ok := find(); !ok {
		t.Fatal("the refused request merged the pair")
	}
	if _, _, err := m.svc.RelinkItems(m.ctx, nil); !errors.As(err, &verr) {
		t.Errorf("no pair: %v, want a validation error", err)
	}

	done, skipped, err := m.svc.RelinkItems(m.ctx, []model.RelinkPairRef{{DeletedID: dead.ID, LiveID: live.ID}, {DeletedID: dead.ID, LiveID: live.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || done[0].KeptID != dead.ID || done[0].RemovedID != live.ID || done[0].Merged != 1 || skipped != 1 {
		t.Errorf("relink = %+v, skipped %d", done, skipped)
	}
	var state string
	if err := f.DB.Read.QueryRow(`SELECT (deleted_at IS NULL) || '/' || (SELECT count(*) FROM items WHERE id = ?) FROM items WHERE id = ?`, live.ID, dead.ID).Scan(&state); err != nil || state != "1/0" {
		t.Errorf("kept film live / copy rows = %q, %v; want 1/0", state, err)
	}
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM media_files WHERE item_id = ?`, dead.ID).Scan(&n); err != nil || n != 2 {
		t.Errorf("files of the kept film = %d, %v; want its own and the copy's", n, err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress WHERE item_id = ?`, dead.ID).Scan(&n); err != nil || n != 2 {
		t.Errorf("progress of the kept film = %d, %v", n, err)
	}
	if _, ok := find(); ok {
		t.Error("the pair is still listed")
	}
}

// The preview is capped at 200 pairs and names the libraries; a running scan stops the relink
// after the pairs before it, which stay merged.
func TestAdminRelinkCapAndScanRunning(t *testing.T) {
	m := newMaintFixture(t)
	libs := &fakeLibraries{}
	libs.created = []model.LibraryRequest{{Name: "Filme", Kind: "video"}}
	m.svc.libraries = libs
	for i := range 201 {
		m.maint.pairs = append(m.maint.pairs, library.RelinkPair{
			LiveID: fmt.Sprintf("live%03d", i), DeletedID: fmt.Sprintf("dead%03d", i), LibraryID: 1, Kind: "series", Title: "Show",
		})
	}
	m.maint.userData = map[string]library.RelinkUserData{"dead000": {ProgressEvents: 140, PlaybackSessions: 3}}
	list, err := m.svc.ListRelinkPairs(m.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 201 || len(list.Pairs) != maxRelinkPairs {
		t.Errorf("total %d, %d pairs; want 201 and %d", list.Total, len(list.Pairs), maxRelinkPairs)
	}
	if p := list.Pairs[0]; p.LibraryName != "Filme" || p.UserData != (model.RelinkUserData{ProgressEvents: 140, PlaybackSessions: 3}) {
		t.Errorf("first pair = %+v", p)
	}
	other := int64(2)
	if list, err := m.svc.ListRelinkPairs(m.ctx, &other); err != nil || list.Total != 0 || len(list.Pairs) != 0 {
		t.Errorf("library 2 = %+v, %v", list, err)
	}

	pairs := make([]model.RelinkPairRef, 0, maxRelinkPairs+1)
	for _, p := range m.maint.pairs {
		pairs = append(pairs, model.RelinkPairRef{DeletedID: p.DeletedID, LiveID: p.LiveID})
	}
	var verr *model.ValidationError
	if _, _, err := m.svc.RelinkItems(m.ctx, pairs); !errors.As(err, &verr) || verr.Fields[0].Code != "too_long" {
		t.Errorf("201 pairs: %v, want too_long", err)
	}
	m.maint.relinkErr = []error{nil, library.ErrNotTwins, model.ErrScanRunning}
	done, skipped, err := m.svc.RelinkItems(m.ctx, pairs[:4])
	if !errors.Is(err, model.ErrScanRunning) || len(done) != 1 || skipped != 1 || len(m.maint.relinked) != 3 {
		t.Errorf("scan running = %+v, %d, %v after %v", done, skipped, err, m.maint.relinked)
	}
}
