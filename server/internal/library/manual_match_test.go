package library

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// noKeys is a metadata key source without any provider key: the manual assignment itself never
// talks to a provider, and the match job the refresh queues is not run here.
type noKeys struct{}

func (noKeys) TMDBKey(context.Context) (string, error)              { return "", nil }
func (noKeys) TVDBKey(context.Context) (key, pin string, err error) { return "", "", nil }
func (noKeys) YouTubeKey(context.Context) (string, error)           { return "", nil }
func (noKeys) TraktClientID(context.Context) (string, error)        { return "", nil }

// B-6: a manual assignment survives three full scans and a metadata refresh. The scanner binds
// files by path, inode and hash, never by title, and the refresh keeps the locked provider ids
// (the metadata package proves in TestAssignMatchAndRefresh that the match job honours the locks).
func TestManualMatchSurvivesScansAndRefresh(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Wrong Title (2001)/Wrong Title (2001).mkv"})
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.ItemCount != 1 || got.Scan.FileCount != 1 {
		t.Fatalf("first scan = %+v", got.Scan)
	}
	itemID := e.str(`SELECT id FROM items WHERE kind = 'movie'`)
	meta := metadata.New(e.db, e.queue, noKeys{}, t.TempDir(), transcode.Runner{}, discard)
	if err := meta.AssignMatch(e.ctx, itemID, "tmdb", "601"); err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		// A second version arrives before the last scan: it must join the assigned title.
		if i == 2 {
			writeFile(t, dir, "Wrong Title (2001)/Wrong Title (2001) - 2160p.mkv", "second version")
		}
		got := e.scan(lib.ID)
		wantFiles := 1
		if i == 2 {
			wantFiles = 2
		}
		if got.Scan.State != "done" || got.Scan.ItemCount != 1 || got.Scan.FileCount != wantFiles {
			t.Fatalf("scan %d = %+v", i+1, got.Scan)
		}
		if id := e.str(`SELECT id FROM items WHERE kind = 'movie' AND deleted_at IS NULL`); id != itemID {
			t.Fatalf("scan %d replaced the title: %s → %s", i+1, itemID, id)
		}
		if tmdb := e.str(`SELECT tmdb_id FROM items WHERE id = ?`, itemID); tmdb != "601" {
			t.Fatalf("scan %d lost the assignment: tmdb_id %q", i+1, tmdb)
		}
	}
	if err := meta.Refresh(e.ctx, itemID); err != nil {
		t.Fatal(err)
	}
	if tmdb := e.str(`SELECT tmdb_id FROM items WHERE id = ?`, itemID); tmdb != "601" {
		t.Fatalf("refresh lost the assignment: tmdb_id %q", tmdb)
	}
	if locked := e.str(`SELECT locked_fields FROM items WHERE id = ?`, itemID); locked != `["tmdb_id","tvdb_id","imdb_id"]` {
		t.Fatalf("locked fields after refresh = %s", locked)
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, itemID); n != 2 {
		t.Fatalf("media files of the assigned title = %d, want 2", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued' AND priority = 7`, JobMetadataMatch); n != 1 {
		t.Fatalf("queued refresh match jobs = %d, want 1 at priority 7", n)
	}
}
