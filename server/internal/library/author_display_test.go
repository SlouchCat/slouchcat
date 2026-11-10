package library

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// recompute runs RecomputeAuthorDisplay in its own transaction.
func (e *env) recompute() {
	e.t.Helper()
	tx, err := e.db.Write.BeginTx(e.ctx, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := RecomputeAuthorDisplay(e.ctx, systemq.New(tx)); err != nil {
		e.t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		e.t.Fatal(err)
	}
}

// display returns items.author_display of a book, "NULL" for none.
func (e *env) display(id string) string {
	e.t.Helper()
	return e.str(`SELECT COALESCE(author_display, 'NULL') FROM items WHERE id = ?`, id)
}

// keyOf returns the series key of a book, "" without series.
func (e *env) keyOf(id string) string {
	e.t.Helper()
	return e.str(`SELECT COALESCE((SELECT series_key FROM audio_series WHERE id = i.audio_series_id), '') FROM items AS i WHERE i.id = ?`, id)
}

func (e *env) setSwitch(key, value string) {
	e.t.Helper()
	e.exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, 1)
	        ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
}

// PD-13 and D6.4 with Marco's spellings (concept 19 D3): «J R R Tolkien» and «J.R.R. Tolkien» get
// «J. R. R. Tolkien», a field that is unified already keeps NULL, and items.author stays the files'
// value. The spelling a source wrote wins over the most common one («Marc-Uwe Kling» from Audible
// over two «Marc Uwe Kling»), and so does one the administrator locked; without either the most
// common spelling wins. Only changed rows are written; series keys and ids stay (PD-3). With the
// switch off every author_display is NULL.
func TestRecomputeAuthorDisplay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	t1, t2, t3 := e.book(lib, 1, "Herr der Ringe", "J R R Tolkien", 1), e.book(lib, 2, "Der Herr Der Ringe - Band", "J.R.R. Tolkien", 2),
		e.book(lib, 3, "Herr der Ringe", "J. R. R. Tolkien, Wolfgang Krege", 3)
	k1, k2, k3 := e.book(lib, 4, "Känguru", "Marc Uwe Kling", 1), e.book(lib, 5, "Känguru", "Marc Uwe Kling", 2),
		e.book(lib, 6, "Känguru", "Marc-Uwe Kling", 3)
	plain := e.book(lib, 7, "", "Chris Bradford", 0)
	gone := e.book(lib, 8, "", "J.R.R. Tolkien", 0)
	e.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, gone)
	e.exec(`INSERT INTO item_field_sources (item_id, field, lang, provider) VALUES (?, 'author', '', 'audible'), (?, 'author', '', 'local'), (?, 'author', '', 'local')`, k3, k1, k2)
	all := []string{t1, t2, t3, k1, k2, k3, plain, gone}
	e.relink(audioseries.Default(), all...)
	series := map[string]string{}
	keys := map[string]string{}
	for _, id := range all {
		series[id] = e.seriesOf(id)
		keys[id] = e.keyOf(id)
	}
	e.exec(`UPDATE items SET updated_at = 1`)

	e.recompute()
	for id, want := range map[string]string{
		t1: "J. R. R. Tolkien", t2: "J. R. R. Tolkien", t3: "NULL",
		k1: "Marc-Uwe Kling", k2: "Marc-Uwe Kling", k3: "NULL", plain: "NULL", gone: "NULL",
	} {
		if got := e.display(id); got != want {
			t.Errorf("author_display of %s = %q, want %q", id, got, want)
		}
	}
	if raw := e.str(`SELECT author FROM items WHERE id = ?`, t2); raw != "J.R.R. Tolkien" {
		t.Errorf("items.author = %q, want the files' value", raw)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE updated_at <> 1`); n != 4 {
		t.Errorf("%d rows written, want 4 (two Tolkien, two Kling)", n)
	}
	for _, id := range all {
		key := e.keyOf(id)
		if e.seriesOf(id) != series[id] || key != keys[id] {
			t.Errorf("book %s: series %q key %q after the recompute, want %q %q", id, e.seriesOf(id), key, series[id], keys[id])
		}
	}
	e.exec(`UPDATE items SET updated_at = 2`)
	e.recompute()
	if n := e.count(`SELECT count(*) FROM items WHERE updated_at <> 2`); n != 0 {
		t.Errorf("second run wrote %d rows, want none", n)
	}

	// Without the source the most common spelling wins; a lock of the administrator counts like a source.
	e.exec(`DELETE FROM item_field_sources WHERE item_id = ?`, k3)
	e.recompute()
	for id, want := range map[string]string{k1: "NULL", k2: "NULL", k3: "Marc Uwe Kling"} {
		if got := e.display(id); got != want {
			t.Errorf("most common: author_display of %s = %q, want %q", id, got, want)
		}
	}
	e.exec(`UPDATE items SET locked_fields = '["author"]' WHERE id = ?`, k3)
	e.recompute()
	if got := e.display(k1); got != "Marc-Uwe Kling" {
		t.Errorf("locked spelling: author_display = %q, want Marc-Uwe Kling", got)
	}

	e.setSwitch(audioseries.SettingNormalizeAuthors, "0")
	e.recompute()
	if n := e.count(`SELECT count(*) FROM items WHERE author_display IS NOT NULL`); n != 0 {
		t.Errorf("switch off: %d books keep author_display", n)
	}
}

// D6.4: with the switch metadata.books.normalize_series off the recompute names a series after its
// most common spelling with the suffix, a scan keeps that name, a locked name stays, and the key
// and id never change; on again, the name loses the suffix.
func TestRecomputeSeriesNameSwitch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.createAudiobooks(t.TempDir()).ID
	ids := []string{e.book(lib, 1, "Bodyguard", "Chris Bradford", 1), e.book(lib, 2, "Bodyguard - Band", "Chris Bradford", 2),
		e.book(lib, 3, "Bodyguard - Band", "Chris Bradford", 3)}
	locked := e.book(lib, 4, "Reihe - Band", "A", 1)
	e.relink(audioseries.Default(), append(ids, locked)...)
	id, lockedID := e.seriesOf(ids[0]), e.seriesOf(locked)
	key := e.str(`SELECT series_key FROM audio_series WHERE id = ?`, id)
	e.exec(`UPDATE audio_series SET name = 'Von Hand', name_locked = 1 WHERE id = ?`, lockedID)
	name := func() string { return e.str(`SELECT name FROM audio_series WHERE id = ?`, id) }
	if got := name(); got != "Bodyguard" {
		t.Fatalf("name %q, want Bodyguard", got)
	}

	e.setSwitch(audioseries.SettingNormalizeSeries, "0")
	e.recompute()
	if got := name(); got != "Bodyguard - Band" {
		t.Errorf("switch off: name %q, want Bodyguard - Band", got)
	}
	e.relink(audioseries.Default(), ids...) // the scan
	if got := name(); got != "Bodyguard - Band" {
		t.Errorf("switch off after a scan: name %q, want Bodyguard - Band", got)
	}
	for _, b := range ids {
		if e.seriesOf(b) != id {
			t.Errorf("book %s left series %s", b, id)
		}
	}
	if got := e.str(`SELECT series_key FROM audio_series WHERE id = ?`, id); got != key {
		t.Errorf("key %q, want %q", got, key)
	}
	if got := e.str(`SELECT name FROM audio_series WHERE id = ?`, lockedID); got != "Von Hand" {
		t.Errorf("locked name %q, want Von Hand", got)
	}
	e.setSwitch(audioseries.SettingNormalizeSeries, "1")
	e.recompute()
	if got := name(); got != "Bodyguard" {
		t.Errorf("switch on again: name %q, want Bodyguard", got)
	}
}

// QueueAuthorDisplay queues one job under its key; while that one runs a request queues one more
// behind it and further requests add nothing. The first start queues the job once
// (SettingAuthorDisplayQueued), a scan that touched audiobooks queues it, and the handler writes the
// names.
func TestQueueAuthorDisplay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	active := func() int {
		return e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state IN ('queued', 'running')`, JobAuthorDisplay)
	}
	if _, err := e.svc.RelinkAudioSeriesOnNewKeys(e.ctx); err != nil {
		t.Fatal(err)
	}
	if n := active(); n != 1 {
		t.Fatalf("first start queued %d jobs, want 1", n)
	}
	e.exec(`UPDATE jobs SET state = 'done' WHERE type = ?`, JobAuthorDisplay)
	if _, err := e.svc.RelinkAudioSeriesOnNewKeys(e.ctx); err != nil {
		t.Fatal(err)
	}
	if n := active(); n != 0 {
		t.Fatalf("second start queued %d jobs, want none", n)
	}

	for range 2 {
		if err := e.svc.QueueAuthorDisplay(e.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := active(); n != 1 {
		t.Errorf("two requests: %d active jobs, want 1", n)
	}
	e.exec(`UPDATE jobs SET state = 'running' WHERE type = ? AND state = 'queued'`, JobAuthorDisplay)
	for range 3 {
		if err := e.svc.QueueAuthorDisplay(e.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND idempotency_key = ?`, JobAuthorDisplay, authorDisplayNextKey); n != 1 || active() != 2 {
		t.Errorf("requests while running: %d queued behind, %d active, want 1 and 2", n, active())
	}
	e.exec(`UPDATE jobs SET state = 'done' WHERE type = ?`, JobAuthorDisplay)

	lib := e.createAudiobooks(t.TempDir()).ID
	a := e.book(lib, 1, "", "J R R Tolkien", 0)
	e.svc.finishAudiobooks(e.ctx, map[string]bool{a: true})
	if n := active(); n != 1 {
		t.Errorf("scan with audiobooks: %d active jobs, want 1", n)
	}
	if err := e.svc.handleAuthorDisplay(e.ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.display(a); got != "J. R. R. Tolkien" {
		t.Errorf("handler: author_display %q", got)
	}
}

// A job queued at priority 1 by an older build is raised to prioAuthorDisplay, at the next start
// and at the next request (CT 190, 10 Nov 2026: it waited behind the image backlog).
func TestAuthorDisplayJobRaisedFromOlderBuild(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	prio := func() int {
		return e.count(`SELECT priority FROM jobs WHERE type = ? AND idempotency_key = ? AND state = 'queued'`, JobAuthorDisplay, authorDisplayKey)
	}
	e.exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, '1', 1)`, SettingAuthorDisplayQueued)
	e.exec(`INSERT INTO jobs (type, priority, idempotency_key, payload, state, run_after, max_attempts, created_at, updated_at)
	        VALUES (?, 1, ?, '{}', 'queued', 1, 3, 1, 1)`, JobAuthorDisplay, authorDisplayKey)
	if err := e.svc.queueAuthorDisplayOnce(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := prio(); got != prioAuthorDisplay {
		t.Errorf("after the start the job waits at %d, want %d", got, prioAuthorDisplay)
	}
	e.exec(`UPDATE jobs SET priority = 1 WHERE type = ?`, JobAuthorDisplay)
	if err := e.svc.QueueAuthorDisplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := prio(); got != prioAuthorDisplay {
		t.Errorf("after a request the job waits at %d, want %d", got, prioAuthorDisplay)
	}
}
