package library

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

// M7 E-8: the new-episode hook hears about every episode item a scan creates, after its batch
// committed (the items are readable then), for full and path scans; files of known episodes,
// extras and rescans announce nothing.
func TestNewEpisodesHook(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	var heard []string
	WithNewEpisodes(func(ctx context.Context, ids []string) error {
		mu.Lock()
		defer mu.Unlock()
		for _, id := range ids {
			if n := e.count(`SELECT count(*) FROM items WHERE id = ? AND kind = 'episode'`, id); n != 1 {
				t.Errorf("announced %s is no committed episode", id)
			}
		}
		heard = append(heard, ids...)
		return ctx.Err()
	})(e.svc)
	take := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := heard
		heard = nil
		slices.Sort(out)
		return out
	}
	episodes := func(since time.Time) []string {
		rows, err := e.db.Read.QueryContext(e.ctx, `SELECT id FROM items WHERE kind = 'episode' AND created_at >= ? ORDER BY id`, since.UnixMilli())
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}

	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	start := e.clock.Now()
	e.scan(lib.ID)
	all := episodes(start)
	if got := take(); len(all) == 0 || !slices.Equal(got, all) {
		t.Fatalf("full scan announced %v, want the %d episodes %v", got, len(all), all)
	}

	e.clock.Advance(time.Hour)
	since := e.clock.Now()
	writeFile(t, dir, showDir+"/Season 02/Show - S02E01 - Neu.mkv", "s2e1")
	writeFile(t, dir, showDir+"/Season 02/Show - S02E02.mkv", "s2e2")
	writeFile(t, dir, showDir+"/Season 02/Trailers/Season 2 Trailer.mkv", "s2 trailer")
	writeFile(t, dir, showDir+"/Season 01/Show - S01E02 - 1080p.mkv", "second version")
	e.pathScan(lib, showDir, showDir+"/Season 01", showDir+"/Season 02", showDir+"/Season 02/Trailers")
	added := episodes(since)
	if got := take(); len(added) != 2 || !slices.Equal(got, added) {
		t.Errorf("path scan announced %v, want the two new episodes %v", got, added)
	}

	e.clock.Advance(time.Hour)
	e.scan(lib.ID)
	if got := take(); len(got) != 0 {
		t.Errorf("rescan announced %v", got)
	}
}
