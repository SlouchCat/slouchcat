package offline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// AN-20: the server copy goes 24 h after the device fetched it and 7 days after it was ready
// without a fetch; orphaned directories, the partial files of failed preparations and tombstones
// older than 400 days go as well.
func TestCleanup(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	fetched := e.create(f.P18, e.devA, f.Rated(0).ID, model.DownloadLow)
	unfetched := e.create(f.P18, e.devA, f.Rated(6).ID, model.DownloadLow)
	fresh := e.create(f.P18, e.devA, f.Rated(12).ID, model.DownloadLow)
	e.runJobs()
	failed := e.create(f.P18, e.devA, f.Rated(16).ID, model.DownloadLow)
	e.enc.failAt, e.enc.failCalls = 2, 100
	e.exec(`UPDATE jobs SET max_attempts = 1 WHERE idempotency_key = ?`, primaryKey(failed.ID))
	e.runJobs()
	if r := e.row(failed.ID); r.state != model.DownloadFailed || !exists(t, e, failed.ID) {
		t.Fatalf("failed = %+v", r)
	}
	orphan := "01J9XRPHAN0000000000000000"
	if err := os.MkdirAll(filepath.Join(e.dir, orphan), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(e.dir, "not-a-download"), 0o750); err != nil {
		t.Fatal(err)
	}
	// The device reports the first copy complete.
	if _, err := e.svc.Sync(ctx, e.session(f.P18, e.devA), model.OfflineSyncRequest{Held: []model.HeldDownload{{ID: fetched.ID, Complete: true}}}); err != nil {
		t.Fatal(err)
	}
	if r := e.row(fetched.ID); !r.fetched.Valid {
		t.Fatal("fetch not recorded")
	}
	e.exec(`INSERT INTO download_tombstones (id, device_id, removed_at) VALUES ('01J9OLDTOMB000000000000000', ?, ?), ('01J9NEWTOMB000000000000000', ?, ?)`,
		e.devA, e.clock.Now().Add(-401*24*time.Hour).UnixMilli(), e.devA, e.clock.Now().Add(-399*24*time.Hour).UnixMilli())

	// After 23 h nothing but the orphan, the failed files and the old tombstone goes.
	e.clock.Advance(23 * time.Hour)
	if err := e.svc.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if r := e.row(fetched.ID); r.state != model.DownloadReady || !exists(t, e, fetched.ID) {
		t.Errorf("fetched copy after 23 h = %+v", r)
	}
	if exists(t, e, orphan) || !exists(t, e, "not-a-download") {
		t.Error("orphan cleanup")
	}
	if r := e.row(failed.ID); !r.removed.Valid || exists(t, e, failed.ID) {
		t.Errorf("failed copy = %+v", r)
	}
	if got := e.tombstoneIDs(); strings.Join(got, ",") != "01J9NEWTOMB000000000000000" {
		t.Errorf("tombstones = %v", got)
	}
	// 25 h after the fetch the fetched copy goes; the unfetched ones stay until 7 days.
	e.clock.Advance(2 * time.Hour)
	if err := e.svc.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if r := e.row(fetched.ID); r.state != model.DownloadEvicted || !r.removed.Valid || exists(t, e, fetched.ID) {
		t.Errorf("fetched copy after 25 h = %+v", r)
	}
	if r := e.row(unfetched.ID); r.state != model.DownloadReady {
		t.Errorf("unfetched after 25 h = %+v", r)
	}
	e.clock.Advance(7*24*time.Hour - 25*time.Hour + time.Minute)
	if err := e.svc.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{unfetched.ID, fresh.ID} {
		if r := e.row(id); r.state != model.DownloadEvicted || exists(t, e, id) {
			t.Errorf("unfetched %s after 7 days = %+v", id, r)
		}
	}
	// Run stops with its context.
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { e.svc.Run(runCtx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func (e *env) tombstoneIDs() []string {
	e.t.Helper()
	rows, err := e.f.DB.Read.Query(`SELECT id FROM download_tombstones WHERE id LIKE '01J9%TOMB%' ORDER BY id`)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}
