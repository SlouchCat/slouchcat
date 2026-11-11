package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An episode still is stored in w780 and w1280 only, without its original (decision of 11 November
// 2026), and a narrow still is never scaled up: a 640 px still is w780 at 640 px and nothing else.
// The stored kind counts as complete for the check after a scan.
func TestEpisodeStillSizes(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.tmdb.serveImage("/t/p/original/still1-2.jpg", jpegBytes(t, 640, 360), "image/jpeg")
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]map[string]int64{
		idE0101: {"backdrop/w780/jpeg": 780, "backdrop/w780/webp": 780, "backdrop/w1280/jpeg": 1280, "backdrop/w1280/webp": 1280},
		idE0102: {"backdrop/w780/jpeg": 640, "backdrop/w780/webp": 640},
	} {
		if err := e.svc.fetchImages(ctx, id); err != nil {
			t.Fatal(err)
		}
		imgs := e.images(t, id)
		if len(imgs) != len(want) {
			t.Errorf("%s: rows %v, want %v", id, keys(imgs), want)
		}
		for key, width := range want {
			if r := imgs[key]; r.Width != width {
				t.Errorf("%s %s: %+v, want %d px wide", id, key, r, width)
			}
		}
		entries, err := os.ReadDir(filepath.Join(e.cache, imagesDir, id))
		if err != nil || len(entries) != len(want) {
			t.Errorf("%s: %d files (%v), want %d (no original)", id, len(entries), err, len(want))
		}
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil {
		t.Fatal(err)
	} else if got := e.imageJobs(t); strings.Contains(strings.Join(got, " "), idE0101) || strings.Contains(strings.Join(got, " "), idE0102) {
		t.Errorf("complete stills queued again (%d): %v", n, got)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// storeRow writes one image row and its file the way the pipeline before 1.28.1 did.
func (e *env) storeRow(t *testing.T, id, kind, size, format string, width int) string {
	t.Helper()
	ext := map[string]string{"jpeg": "jpg", "webp": "webp", "png": "png"}[format]
	rel := imagesDir + "/" + id + "/" + kind + "-" + size + "." + ext
	path := filepath.Join(e.cache, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rel), 0o600); err != nil {
		t.Fatal(err)
	}
	var accent any // derived poster rows carry the accent, else the check after a scan retries it
	if kind == "poster" && size != originalSize {
		accent = "#aa3322"
	}
	mustExec(t, e.db.Write, `INSERT INTO images (item_id, kind, size, format, lang, provider, source_url, rel_path, content_hash,
	                         media_type, width, height, bytes, accent, created_at)
	                         VALUES (?, ?, ?, ?, '', 'tmdb', 'https://image.tmdb.org/t/p/original/x.jpg', ?, 'h', ?, ?, ?, 1, ?, 0)`,
		id, kind, size, format, rel, "image/"+format, width, width*9/16, accent)
	return path
}

// storeOldKind stores a kind with its original of origWidth and every size the pipeline made
// before 1.28.1 (scaled up from a narrow original), and returns the files by "<size>/<format>".
func (e *env) storeOldKind(t *testing.T, id, kind string, origWidth int, sizes []int) map[string]string {
	t.Helper()
	files := map[string]string{originalSize + "/jpeg": e.storeRow(t, id, kind, originalSize, "jpeg", origWidth)}
	for _, w := range sizes {
		for _, f := range []string{"jpeg", "webp"} {
			files[sizeName(w)+"/"+f] = e.storeRow(t, id, kind, sizeName(w), f, w)
		}
	}
	return files
}

// The one-time trim after 1.28.1 removes, without a download or an ffmpeg run, the sizes the
// pipeline no longer makes and the original of every kind that holds its sizes; a locked kind keeps
// its original, an incomplete kind stays for image.fetch, and the run happens once.
func TestTrimImageSizesOnce(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, tvdb: tvdbKey, pin: tvdbPin})
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550, locked: `["poster"]`})
	e.addItem(t, itemSeed{id: idBySearch, title: "Second", tmdbID: 551})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched'`)
	old := []int{780, 1280, 1920}
	still := e.storeOldKind(t, idE0101, "backdrop", 1920, old)
	still4K := e.storeOldKind(t, idE0102, "backdrop", 3840, append(old, 3840))
	narrow := e.storeOldKind(t, idByTMDB, "backdrop", 1000, old)
	locked := e.storeOldKind(t, idByTMDB, "poster", 2000, []int{200, 400, 800})
	broken := e.storeOldKind(t, idBySearch, "backdrop", 1920, old)
	// An upload whose lock is gone keeps its original, the only copy.
	upload := e.storeOldKind(t, idBySearch, "poster", 2000, []int{200, 400, 800})
	mustExec(t, e.db.Write, `UPDATE images SET provider = 'manual' WHERE item_id = ? AND kind = 'poster' AND size = 'original'`, idBySearch)
	if err := os.Remove(broken["w1280/webp"]); err != nil {
		t.Fatal(err)
	}

	n, err := e.svc.TrimImageSizesOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gone := func(files map[string]string, keys ...string) {
		t.Helper()
		for key, path := range files {
			_, err := os.Stat(path)
			want := !strings.Contains(" "+strings.Join(keys, " ")+" ", " "+key+" ")
			if (err == nil) != want {
				t.Errorf("%s: file there %v, want %v", path, err == nil, want)
			}
		}
	}
	gone(still, "original/jpeg", "w1920/jpeg", "w1920/webp")
	gone(still4K, "original/jpeg", "w1920/jpeg", "w1920/webp", "w3840/jpeg", "w3840/webp")
	gone(narrow) // w1280 was scaled up from 1000 px: image.fetch derives it again before the original goes
	gone(locked)
	gone(broken, "w1280/webp")
	gone(upload)
	if n != 3+5 {
		t.Errorf("trimmed %d rows, want 8", n)
	}
	rows := func(id, kind string) int {
		c := 0
		for key := range e.images(t, id) {
			if strings.HasPrefix(key, kind+"/") {
				c++
			}
		}
		return c
	}
	for _, tc := range []struct {
		id, kind string
		want     int
	}{{idE0101, "backdrop", 4}, {idE0102, "backdrop", 4}, {idByTMDB, "backdrop", 7}, {idByTMDB, "poster", 7}, {idBySearch, "backdrop", 7}} {
		if got := rows(tc.id, tc.kind); got != tc.want {
			t.Errorf("%s %s: %d rows, want %d", tc.id, tc.kind, got, tc.want)
		}
	}
	// The trimmed kinds are complete for the check after a scan; the scaled-up one is queued.
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if _, err := e.svc.QueueIncompleteImages(ctx, 1); err != nil {
		t.Fatal(err)
	}
	jobs := strings.Join(e.imageJobs(t), " ")
	if strings.Contains(jobs, idE0101) || strings.Contains(jobs, idE0102) || !strings.Contains(jobs, idByTMDB) {
		t.Errorf("image jobs %s, want only the scaled-up %s", jobs, idByTMDB)
	}

	// Once: a second start leaves new extra rows alone.
	e.storeRow(t, idE0101, "backdrop", "w1920", "jpeg", 1920)
	if n, err := e.svc.TrimImageSizesOnce(ctx); err != nil || n != 0 {
		t.Errorf("second trim = %d, %v", n, err)
	}
	var v string
	if err := e.db.Read.QueryRow(`SELECT value FROM settings WHERE key = ?`, SettingImageSizesTrimmed).Scan(&v); err != nil || v != "1" {
		t.Errorf("setting %s = %q, %v", SettingImageSizesTrimmed, v, err)
	}
}

// The image folder of a title deleted for good goes (RemoveItemImages after a merge or a relink,
// SweepOrphanImages every hour); the folders of titles that exist (soft-deleted ones too), a
// folder another title's rows still name (a relink moved them) and images/radio stay.
func TestSweepOrphanImages(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	const (
		orphan  = "01JTESTGONE000000000000001"
		orphan2 = "01JTESTGONE000000000000002"
		moved   = "01JTESTGONE000000000000003"
	)
	e.addItem(t, itemSeed{id: idByTMDB, title: "Live", tmdbID: 550})
	e.addItem(t, itemSeed{id: idBySearch, title: "Soft-deleted", tmdbID: 551})
	mustExec(t, e.db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, idBySearch)
	dir := func(name string) string { return filepath.Join(e.cache, imagesDir, name) }
	write := func(name string) {
		t.Helper()
		if err := os.MkdirAll(dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir(name), "backdrop-w780.jpg"), []byte("jpeg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	there := func(name string) bool {
		_, err := os.Stat(dir(name))
		return err == nil
	}
	for _, name := range []string{idByTMDB, idBySearch, orphan, orphan2, moved, "radio"} {
		write(name)
	}
	// The live title's row names a file in the folder of a deleted copy (RelinkMoveCachedImages).
	mustExec(t, e.db.Write, `INSERT INTO images (item_id, kind, size, format, lang, provider, source_url, rel_path, content_hash,
	                         media_type, width, height, bytes, created_at)
	                         VALUES (?, 'poster', 'w400', 'jpeg', '', 'tmdb', 'x', ?, 'h', 'image/jpeg', 400, 600, 4, 0)`,
		idByTMDB, imagesDir+"/"+moved+"/poster-w400.jpg")

	// At once: only the given title that no longer exists.
	if err := e.svc.RemoveItemImages(ctx, []string{orphan, idByTMDB, idBySearch, moved, "../etc"}); err != nil {
		t.Fatal(err)
	}
	if there(orphan) || !there(orphan2) || !there(idByTMDB) || !there(idBySearch) || !there(moved) {
		t.Fatalf("after RemoveItemImages: orphan %v, orphan2 %v, live %v, soft-deleted %v, moved %v",
			there(orphan), there(orphan2), there(idByTMDB), there(idBySearch), there(moved))
	}

	// The sweep takes what is left over.
	n, err := e.svc.SweepOrphanImages(ctx)
	if err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want 1", n, err)
	}
	if there(orphan2) || !there(idByTMDB) || !there(idBySearch) || !there(moved) || !there("radio") {
		t.Errorf("after the sweep: orphan2 %v, live %v, soft-deleted %v, moved %v, radio %v",
			there(orphan2), there(idByTMDB), there(idBySearch), there(moved), there("radio"))
	}
	// Once the row is gone, the copy's folder goes too.
	mustExec(t, e.db.Write, `DELETE FROM images WHERE item_id = ?`, idByTMDB)
	if n, err := e.svc.SweepOrphanImages(ctx); err != nil || n != 1 || there(moved) {
		t.Errorf("second sweep = %d, %v, moved folder there %v", n, err, there(moved))
	}
}

// A cover narrower than 200 px is w200 at its own width (never scaled up) and still carries the
// accent from the first run, so the check after a scan does not queue it again.
func TestNarrowCoverKeepsItsAccent(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	cover := jpegBytes(t, 150, 150)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(cover) }))
	t.Cleanup(srv.Close)
	e := audioEnv(t, t.TempDir())
	ctx := context.Background()
	e.addAudio(t, idPodcast, "podcast", 4, nil, nil, srv.URL+"/cover.jpg")
	mustExec(t, e.db.Write, `UPDATE items SET feed_url = ? WHERE id = ?`, srv.URL+"/feed.xml", idPodcast)
	if err := e.svc.fetchImages(ctx, idPodcast); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idPodcast)
	if len(imgs) != 2 || imgs["poster/w200/jpeg"].Width != 150 || !imgs["poster/w200/jpeg"].Accent.Valid || !imgs["poster/w200/webp"].Accent.Valid {
		t.Fatalf("rows %+v, want w200 at 150 px with an accent", imgs)
	}
}
