package library

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The scanner lists the image files next to the media (concept 10 4.6, ADR 0018): poster,
// fanart and logo of a film alone in its folder, only <stem>-* files of films sharing a folder,
// the series folder's files for the series, seasonNN-poster for a season, <stem>-thumb for an
// episode; landscape.jpg is not read; a file outside the root is never reached (rule 4); a
// replaced file (size/mtime) is a changed row and a removed file leaves the table.

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.NRGBA{R: 200, G: 30, B: 40, A: uint8(255 * x / max(w, 1))})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeBytes(t *testing.T, dir, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

type localRow struct {
	kind, rel     string
	width, height int64
}

func (e *env) localImages(itemID string) []localRow {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT kind, rel_path, width, height FROM item_local_images WHERE item_id = ? ORDER BY kind`, itemID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []localRow
	for rows.Next() {
		var r localRow
		if err := rows.Scan(&r.kind, &r.rel, &r.width, &r.height); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestScanListsLocalImagesOfMovies(t *testing.T) {
	e := newEnv(t)
	// Alone has a folder of its own (a second file there is a version of the same film); Shared
	// and Other Cut lie directly in the root, so only their <stem>-* files count.
	dir := tree(t, []string{
		"Alone (2001)/Alone (2001).mkv",
		"Alone (2001)/Alone (2001) - 2160p.mkv",
		"Shared (2002).mkv",
		"Other Cut (2003).mkv",
	})
	poster := pngBytes(t, 600, 900)
	writeBytes(t, dir, "Alone (2001)/poster.png", poster)
	writeBytes(t, dir, "Alone (2001)/folder.jpg", pngBytes(t, 10, 15)) // poster wins over folder
	writeBytes(t, dir, "Alone (2001)/fanart.png", pngBytes(t, 1920, 1080))
	writeBytes(t, dir, "Alone (2001)/clearlogo.png", pngBytes(t, 800, 300))
	writeBytes(t, dir, "Alone (2001)/landscape.png", pngBytes(t, 1000, 562)) // Emby's thumb: not read
	writeBytes(t, dir, "poster.png", pngBytes(t, 600, 900))                  // a generic name in the root: nobody's
	writeBytes(t, dir, "Shared (2002)-poster.png", pngBytes(t, 500, 750))
	writeBytes(t, dir, "Other Cut (2003)-logo.png", pngBytes(t, 400, 150))
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan %+v", got.Scan)
	}
	alone := e.str(`SELECT id FROM items WHERE title = 'Alone' AND kind = 'movie'`)
	shared := e.str(`SELECT id FROM items WHERE title = 'Shared' AND kind = 'movie'`)
	other := e.str(`SELECT id FROM items WHERE title = 'Other Cut' AND kind = 'movie'`)
	if alone == "" || shared == "" || other == "" {
		t.Fatalf("films missing: %q %q %q", alone, shared, other)
	}
	got := e.localImages(alone)
	want := []localRow{{"backdrop", "Alone (2001)/fanart.png", 1920, 1080}, {"logo", "Alone (2001)/clearlogo.png", 800, 300}, {"poster", "Alone (2001)/poster.png", 600, 900}}
	if len(got) != len(want) {
		t.Fatalf("alone rows = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alone row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := e.localImages(shared); len(got) != 1 || got[0].rel != "Shared (2002)-poster.png" || got[0].width != 500 {
		t.Errorf("shared rows = %+v, want the stem poster only", got)
	}
	if got := e.localImages(other); len(got) != 1 || got[0].kind != "logo" || got[0].rel != "Other Cut (2003)-logo.png" {
		t.Errorf("other rows = %+v, want the stem logo only", got)
	}
	if n := e.count(`SELECT count(*) FROM item_local_images WHERE rel_path LIKE '%landscape%'`); n != 0 {
		t.Errorf("landscape.png listed %d times, want 0", n)
	}

	// A replaced poster is a new row (size changes), a removed logo leaves the table.
	writeBytes(t, dir, "Alone (2001)/poster.png", pngBytes(t, 1000, 1500))
	remove(t, dir, "Alone (2001)/clearlogo.png")
	e.clock.Advance(1)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("rescan %+v", got.Scan)
	}
	got = e.localImages(alone)
	if len(got) != 2 || got[1].kind != "poster" || got[1].width != 1000 {
		t.Errorf("after the change: %+v, want fanart and the new poster", got)
	}
}

func TestScanListsLocalImagesOfSeries(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	writeBytes(t, dir, showDir+"/poster.png", pngBytes(t, 600, 900))
	writeBytes(t, dir, showDir+"/logo.png", pngBytes(t, 800, 300))
	writeBytes(t, dir, showDir+"/season01-poster.png", pngBytes(t, 500, 750))
	writeBytes(t, dir, showDir+"/Season 01/Show - S01E02-thumb.png", pngBytes(t, 1280, 720))
	lib := e.createSeries(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan %+v", got.Scan)
	}
	series := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	season := e.str(`SELECT id FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 1`, series)
	episode := e.episode("Show", 1, 2)
	if series == "" || season == "" || episode == "" {
		t.Fatalf("tree missing: %q %q %q", series, season, episode)
	}
	if got := e.localImages(series); len(got) != 2 || got[0].kind != "logo" || got[1].rel != showDir+"/poster.png" {
		t.Errorf("series rows = %+v", got)
	}
	if got := e.localImages(season); len(got) != 1 || got[0].rel != showDir+"/season01-poster.png" {
		t.Errorf("season rows = %+v", got)
	}
	if got := e.localImages(episode); len(got) != 1 || got[0].kind != "backdrop" || got[0].width != 1280 {
		t.Errorf("episode rows = %+v", got)
	}
}

// A symlink out of the root is never followed (rule 4): the header read fails inside the root and
// the file is not listed.
func TestLocalImageOutsideTheRootIsNeverRead(t *testing.T) {
	e := newEnv(t)
	dir := tree(t, []string{"Alone (2001)/Alone (2001).mkv"})
	outside := t.TempDir()
	writeBytes(t, outside, "secret.png", pngBytes(t, 600, 900))
	if err := os.Symlink(filepath.Join(outside, "secret.png"), filepath.Join(dir, "Alone (2001)", "poster.png")); err != nil {
		t.Skip("symlinks not supported here")
	}
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan %+v", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM item_local_images`); n != 0 {
		t.Fatalf("a poster behind a symlink out of the root was listed (%d rows)", n)
	}
}
