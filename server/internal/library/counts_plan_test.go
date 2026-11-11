package library

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// CountLibraryMediaFiles counts the present files of one library without reading the table row of
// every present file of the server. The join with missing_since IS NULL it replaced let SQLite walk
// media_files_missing for the IS NULL: 1.35 s of the 1.8 s of GET /admin/libraries on CT 190
// (queries/system/items.sql). The test fills two libraries, one with a second root and missing
// files, checks the numbers and that the plan reaches media_files only through root_id and the
// missing rows, before and after ANALYZE.
func TestCountLibraryMediaFilesCountsWithoutATableWalk(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'A', 'video', 0, 0), (2, 'B', 'video', 0, 0)`)
	e.exec(`INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at) VALUES
		(1, 1, '/m/a1', 'ext4', 'inotify', 0), (2, 1, '/m/a2', 'ext4', 'inotify', 0), (3, 2, '/m/b', 'ext4', 'inotify', 0)`)
	// root -> present and missing files
	files := map[int64][2]int{1: {300, 7}, 2: {40, 3}, 3: {900, 0}}
	n := 0
	for root, c := range files {
		lib := int64(1)
		if root == 3 {
			lib = 2
		}
		for i := range c[0] + c[1] {
			n++
			item, file := fmt.Sprintf("01KW%022d", n), fmt.Sprintf("01KF%022d", n)
			var missing any
			if i >= c[0] {
				missing = int64(5)
			}
			e.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at)
				VALUES (?, 'movie', ?, ?, ?, 1, 1, 1)`, item, lib, item, item)
			e.exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, missing_since,
				created_at, updated_at) VALUES (?, ?, ?, ?, 1, 1, 'mkv', ?, 1, 1)`, file, item, root, file+".mkv", missing)
		}
	}

	rec := &recordingDB{DBTX: e.db.Read}
	q := systemq.New(rec)
	want := map[int64]int64{1: 340, 2: 900, 3: 0}
	for _, analyzed := range []bool{false, true} {
		if analyzed {
			e.exec(`ANALYZE`)
		}
		for lib, w := range want {
			got, err := q.CountLibraryMediaFiles(e.ctx, lib)
			if err != nil {
				t.Fatal(err)
			}
			if got != w {
				t.Errorf("library %d (analyzed %v): %d files, want %d", lib, analyzed, got, w)
			}
			plan := e.plan(rec.query, rec.args)
			for _, line := range plan {
				onFiles := strings.Contains(line, " media_files ") || strings.HasSuffix(line, " media_files") ||
					strings.Contains(line, " m ") || strings.HasSuffix(line, " m")
				if !onFiles {
					continue
				}
				// Allowed: a covering index on root_id, or the missing rows of media_files_missing.
				if !strings.Contains(line, "COVERING INDEX") && !strings.Contains(line, "media_files_missing (missing_since>?)") {
					t.Errorf("library %d (analyzed %v) reads media_files rows: %q\n%s", lib, analyzed, line, strings.Join(plan, "\n"))
				}
			}
		}
	}
}

// ListRoots carries the libraries with their roots, as List does, without the counts.
func TestListRootsMatchesList(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(tree(t, nil))
	e.scan(lib.ID)
	full, err := e.svc.List(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	brief, err := e.svc.ListRoots(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != 1 || len(brief) != 1 {
		t.Fatalf("List = %d, ListRoots = %d libraries, want 1 each", len(full), len(brief))
	}
	f, b := full[0], brief[0]
	if b.ID != f.ID || b.Name != f.Name || b.Kind != f.Kind || len(b.Roots) != len(f.Roots) || len(b.Roots) != 1 {
		t.Fatalf("ListRoots = %+v, want the library and roots of List %+v", b, f)
	}
	fr, br := f.Roots[0], b.Roots[0]
	if br.ID != fr.ID || br.Path != fr.Path || br.FSType != fr.FSType || br.Watch != fr.Watch ||
		br.Warning != fr.Warning || br.Content != fr.Content {
		t.Errorf("ListRoots root = %+v, want %+v", br, fr)
	}
	if b.Scan.ItemCount != 0 || br.FileCount != 0 {
		t.Errorf("ListRoots carries counts: %+v", b)
	}
}
