package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// getTrickplay and getTrickplaySheet (concept 18, 8.2): the set of a visible title's file with its
// sheet URLs, the sheet immutable under the version of the file and revalidated without it; 404
// for an invisible title, a file of another title, a missing sheet and a sheet that is no number.
func TestTrickplayEndpoints(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	dir := t.TempDir()
	pb := playback.New(f.DB, vistest.Logger(), playback.WithTokenKey(testTokenKey), playback.WithCacheDir(dir))
	h := pf.handlerWithPlayback(pb)
	m12, m18 := f.Rated(12), f.Rated(18)
	raw, _ := chapterTestProbe(t)
	for _, m := range []vistest.Movie{m12, m18} {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", raw, nil)
	}
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/trickplay?mediaFileId="+m12.FileID, "p12", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("getTrickplay: %d %s", rec.Code, rec.Body.String())
	}
	var tp struct {
		Version, State                 string
		TileWidth, TileHeight, Columns int
		PointsMs                       []int64
		Sheets                         []struct {
			Index int
			URL   string `json:"url"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tp); err != nil {
		t.Fatal(err)
	}
	if tp.State != "queued" || tp.TileWidth != 320 || tp.TileHeight != 180 || tp.Columns != 10 || len(tp.PointsMs) == 0 || tp.Sheets == nil || len(tp.Sheets) != 0 {
		t.Fatalf("getTrickplay %s", rec.Body.String())
	}
	set := filepath.Join(dir, library.TrickplayDir, m12.FileID+"-"+tp.Version)
	if err := os.MkdirAll(set, 0o750); err != nil {
		t.Fatal(err)
	}
	sheet := []byte("RIFF\x10\x00\x00\x00WEBPVP8 sheet")
	if err := os.WriteFile(filepath.Join(set, "0.webp"), sheet, 0o600); err != nil {
		t.Fatal(err)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/trickplay?mediaFileId="+m12.FileID, "p12", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &tp); err != nil || len(tp.Sheets) != 1 {
		t.Fatalf("getTrickplay with a sheet: %s", rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, tp.Sheets[0].URL, "p12", "", nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), sheet) || rec.Header().Get("Content-Type") != "image/webp" ||
		rec.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" || rec.Header().Get("ETag") != playback.TrickplaySheetETag(tp.Version, 0) {
		t.Errorf("sheet: %d %v", rec.Code, rec.Header())
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/trickplay/0?mediaFileId="+m12.FileID+"&v=1-1", "p12", "", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-cache" {
		t.Errorf("sheet with another v: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	for _, c := range []struct{ name, token, path string }{
		{"age rule", "p6", tp.Sheets[0].URL},
		{"invisible title", "p12", BasePath + "/items/" + m18.ID + "/trickplay/0?mediaFileId=" + m18.FileID},
		{"file of another title", "p18", BasePath + "/items/" + m12.ID + "/trickplay/0?mediaFileId=" + m18.FileID},
		{"missing sheet", "p12", BasePath + "/items/" + m12.ID + "/trickplay/1?mediaFileId=" + m12.FileID},
		{"sheet with a leading zero", "p12", BasePath + "/items/" + m12.ID + "/trickplay/00?mediaFileId=" + m12.FileID},
		{"sheet no number", "p12", BasePath + "/items/" + m12.ID + "/trickplay/x?mediaFileId=" + m12.FileID},
		{"getTrickplay age rule", "p6", BasePath + "/items/" + m12.ID + "/trickplay?mediaFileId=" + m12.FileID},
		{"getTrickplay file id no ULID", "p12", BasePath + "/items/" + m12.ID + "/trickplay?mediaFileId=x"},
	} {
		rec := send(t, h, http.MethodGet, c.path, c.token, "", nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s: status %d (%s), want 404 not_found", c.name, rec.Code, rec.Body.String())
		}
	}
}
