package library

import (
	"testing"
)

// The hourly library.sweep deletes user picks past the retention of picks_config (concept 18 part
// B, 14.5) and keeps younger ones; a shorter retention reaches the next sweep.
func TestSweepDeletesExpiredUserPicks(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:2])
	lib := e.create(dir)
	e.scan(lib.ID)
	e.profiles(1)
	first := e.str(`SELECT id FROM items ORDER BY id LIMIT 1`)
	second := e.str(`SELECT id FROM items WHERE id <> ? ORDER BY id LIMIT 1`, first)
	const day = int64(24 * 60 * 60 * 1000)
	now := e.clock.Now().UnixMilli()
	e.exec(`INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (?, 1, ?), (?, 1, ?)`, first, now-60*day, second, now-59*day)
	if err := e.svc.handleSweep(e.ctx, nil); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT group_concat(item_id) FROM user_picks`); v != second {
		t.Errorf("user picks after the sweep = %q, want only %q (59 days old)", v, second)
	}
	e.exec(`UPDATE picks_config SET retention_days = 30`)
	if err := e.svc.handleSweep(e.ctx, nil); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM user_picks`); n != 0 {
		t.Errorf("%d user picks after the sweep with 30 days, want none", n)
	}
}
