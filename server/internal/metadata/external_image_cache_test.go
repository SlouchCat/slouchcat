package metadata

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// A poster narrower than the smallest size of the ladder (w200) keeps one size, at its own width,
// so the title has a poster after its first scaling; every size it is asked for serves that file.
func TestExternalTitleTinyPoster(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.mu.Lock()
	e.tmdb.movies["602"].de["poster_path"], e.tmdb.movies["602"].en["poster_path"] = "/tiny602.jpg", "/tiny602.jpg"
	e.tmdb.mu.Unlock()
	e.tmdb.serveImage("/t/p/original/tiny602.jpg", jpegBytes(t, 100, 150), "image/jpeg")
	if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, 602); err != nil {
		t.Fatal(err)
	}
	for _, size := range []string{"w200", "w400", "w800"} {
		src, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 602, "poster", size, false)
		if err != nil {
			t.Fatalf("poster %s of a 100 px original: %v", size, err)
		}
		data, _ := io.ReadAll(src.File)
		_ = src.File.Close()
		if len(data) == 0 {
			t.Errorf("poster %s is empty", size)
		}
	}
	var size string
	var width int
	if err := e.db.Read.QueryRow(`SELECT size, width FROM external_title_images WHERE tmdb_id = 602 AND kind = 'poster' AND format = 'jpeg'`).Scan(&size, &width); err != nil {
		t.Fatal(err)
	}
	if size != "w200" || width != 100 {
		t.Errorf("stored poster = %s at %d px, want w200 at 100 px", size, width)
	}
	title, err := e.svc.ExternalTitle(ctx, MediaMovie, 602)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, im := range title.Images {
		found = found || (im.Kind == "poster" && im.Size == "w200" && im.Width == 100)
	}
	if !found {
		t.Errorf("image refs after scaling = %+v, want the poster w200 at 100 px", title.Images)
	}
}

// The image cache stays below its cap: a new fetch drops the images of the least recently used
// title (row and files), never those of the title just fetched; the dropped title keeps its row
// and fetches its images again on the next request.
func TestExternalImageCacheBound(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	for _, id := range []int64{550, 601} {
		if _, err := e.svc.EnsureExternalTitle(ctx, MediaMovie, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 550, "poster", "w200", false); err != nil {
		t.Fatal(err)
	}
	var first int64
	var rel string
	if err := e.db.Read.QueryRow(`SELECT sum(bytes), min(rel_path) FROM external_title_images WHERE tmdb_id = 550`).Scan(&first, &rel); err != nil {
		t.Fatal(err)
	}
	// 550 was used long ago, 601 now; the cap holds one title's posters.
	if _, err := e.db.Write.Exec(`UPDATE external_titles SET used_at = CASE tmdb_id WHEN 550 THEN 1 ELSE 9e15 END`); err != nil {
		t.Fatal(err)
	}
	e.svc.externalImageCap = first + first/2
	if _, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 601, "poster", "w200", false); err != nil {
		t.Fatal(err)
	}
	count := func(id int64) int {
		t.Helper()
		var n int
		if err := e.db.Read.QueryRow(`SELECT count(*) FROM external_title_images WHERE tmdb_id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(550) != 0 || count(601) == 0 {
		t.Errorf("rows after the bound: 550 %d, 601 %d", count(550), count(601))
	}
	if _, err := os.Stat(filepath.Join(e.cache, rel)); !os.IsNotExist(err) {
		t.Errorf("file of the dropped title: %v", err)
	}
	var total int64
	if err := e.db.Read.QueryRow(`SELECT sum(bytes) FROM external_title_images`).Scan(&total); err != nil || total > e.svc.externalImageCap {
		t.Errorf("total %d over the cap %d (%v)", total, e.svc.externalImageCap, err)
	}
	// The dropped title still exists and fetches again.
	if _, err := e.svc.OpenExternalTitleImage(ctx, MediaMovie, 550, "poster", "w200", false); err != nil {
		t.Errorf("refetch of the dropped title: %v", err)
	}
	if n := e.tmdb.count("/t/p/original/poster550.jpg"); n != 2 {
		t.Errorf("downloads of 550 = %d, want 2", n)
	}
}
