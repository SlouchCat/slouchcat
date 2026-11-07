package radio

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// pngOf returns a w×h PNG in one colour.
func pngOf(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// icoOf returns an ICO whose entries have the given widths (0 = 256), each a PNG of its size.
func icoOf(t testing.TB, widths ...int) []byte {
	t.Helper()
	head := []byte{0, 0, 1, 0, byte(len(widths)), 0}
	var dir, body []byte
	off := 6 + 16*len(widths)
	for _, w := range widths {
		px := w
		if px == 0 {
			px = 256
		}
		img := pngOf(t, px, px)
		e := make([]byte, 16)
		e[0], e[1] = byte(w), byte(w)
		binary.LittleEndian.PutUint16(e[4:], 1)
		binary.LittleEndian.PutUint16(e[6:], 32)
		binary.LittleEndian.PutUint32(e[8:], uint32(len(img)))
		binary.LittleEndian.PutUint32(e[12:], uint32(off+len(body)))
		dir = append(dir, e...)
		body = append(body, img...)
	}
	return append(append(head, dir...), body...)
}

func serve(st *radiotest.Station, path, contentType string, body []byte) string {
	st.Set(path, radiotest.Mount{Kind: radiotest.HTML, ContentType: contentType, Body: body})
	return st.URL(path)
}

func newLogoEnv(t *testing.T) *relayEnv {
	t.Helper()
	e := newRelayEnv(t)
	e.s.o.cacheDir = t.TempDir()
	e.s.o.queue = jobs.NewQueue(e.f.DB.Write, e.s.now)
	return e
}

func (e *relayEnv) runLogoJob(id int64) {
	e.t.Helper()
	payload, _ := json.Marshal(logoPayload{StationID: id})
	if err := e.s.handleLogo(context.Background(), &jobs.Job{Type: JobLogoFetch, Payload: payload}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *relayEnv) checkLogoFiles(id int64) {
	e.t.Helper()
	for _, w := range logoWidths {
		for _, ext := range []string{"webp", "png"} {
			st, err := e.s.Image(context.Background(), id, "logo", "w"+strconv.Itoa(w), ext == "webp", false)
			if err != nil {
				e.t.Fatalf("logo w%d %s: %v", w, ext, err)
			}
			b, _ := io.ReadAll(st.File)
			_ = st.File.Close()
			if ext == "png" {
				img, err := png.Decode(bytes.NewReader(b))
				if err != nil || img.Bounds().Dx() != w || img.Bounds().Dy() != w {
					e.t.Fatalf("logo w%d is not a %d-px square: %v", w, w, err)
				}
			}
		}
	}
}

func TestLogoChainFavicon(t *testing.T) {
	e := newLogoEnv(t)
	id := e.addStation("Logo FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ? WHERE id = ?`, serve(e.st, "/fav.png", "image/png", pngOf(t, 200, 120)), id)
	e.s.queueLogos(context.Background(), []int64{id})
	if st := e.stationRow(id); st.LogoState != "queued" {
		t.Fatalf("state %q after queueing", st.LogoState)
	}
	e.s.queueLogos(context.Background(), []int64{id}) // queued once
	e.runLogoJob(id)
	st := e.stationRow(id)
	if st.LogoState != logoStateCatalog || len(st.LogoHash.String) != 64 || !st.LogoTriedAt.Valid {
		t.Fatalf("station %+v", st)
	}
	e.checkLogoFiles(id)
	if _, err := e.s.Image(context.Background(), id, "logo", "w128", false, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown size: %v", err)
	}
	e.exec(`UPDATE radio_stations SET hidden = 1 WHERE id = ?`, id)
	e.s.reloadStation(context.Background(), id) // the index view decides visibility (M-5)
	if _, err := e.s.Image(context.Background(), id, "logo", "w96", true, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("logo of a hidden station for a viewer: %v", err)
	}
	if src, err := e.s.Image(context.Background(), id, "logo", "w96", true, true); err != nil {
		t.Fatalf("logo of a hidden station for the administration: %v", err)
	} else {
		_ = src.File.Close()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.changed) != 2 { // queued, catalog
		t.Fatalf("onStationChanged calls %v", e.changed)
	}
}

func TestLogoChainWebsiteIcons(t *testing.T) {
	e := newLogoEnv(t)
	icon := serve(e.st, "/apple.png", "image/png", pngOf(t, 180, 180))
	serve(e.st, "/small.png", "image/png", pngOf(t, 32, 32))
	html := `<!doctype html><html><head><link rel="icon" sizes="32x32" href="/small.png">
		<LINK REL='apple-touch-icon' href='` + icon + `'><meta property="og:image" content="/og.png"></head></html>`
	id := e.addStation("Web FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ?, homepage = ? WHERE id = ?`, e.st.URL("/missing.ico"),
		serve(e.st, "/", "text/html", []byte(html)), id)
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState != logoStateWebsite {
		t.Fatalf("state %q, want website", st.LogoState)
	}
	e.checkLogoFiles(id)

	// The manifest's icons when the page links nothing better.
	manifest := `{"icons":[{"src":"/vector.svg","type":"image/svg+xml","sizes":"any"},{"src":"/m512.png","sizes":"512x512"}]}`
	serve(e.st, "/m512.png", "image/png", pngOf(t, 512, 512))
	serve(e.st, "/app.webmanifest", "application/manifest+json", []byte(manifest))
	id2 := e.addStation("Manifest FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET homepage = ? WHERE id = ?`,
		serve(e.st, "/m.html", "text/html", []byte(`<link rel="manifest" href="/app.webmanifest">`)), id2)
	e.runLogoJob(id2)
	if st := e.stationRow(id2); st.LogoState != logoStateWebsite {
		t.Fatalf("manifest: state %q, want website", st.LogoState)
	}
}

func TestLogoRejectsSVGAndSmall(t *testing.T) {
	e := newLogoEnv(t)
	svg := serve(e.st, "/logo.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512"/>`))
	html := `<link rel="icon" sizes="32x32" href="/small.png"><meta property="og:image" content="/logo.svg">`
	serve(e.st, "/small.png", "image/png", pngOf(t, 32, 32))
	id := e.addStation("Small FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ?, homepage = ? WHERE id = ?`, svg, serve(e.st, "/", "text/html", []byte(html)), id)
	e.runLogoJob(id)
	st := e.stationRow(id)
	if st.LogoState != logoStateFailed || st.LogoHash.Valid {
		t.Fatalf("state %q hash %v, want failed without a logo", st.LogoState, st.LogoHash.Valid)
	}
	if _, err := e.s.Image(context.Background(), id, "logo", "w96", true, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("image without a logo: %v", err)
	}
	// A failed chain is retried after 30 days only.
	e.s.queueLogos(context.Background(), []int64{id})
	if e.stationRow(id).LogoState != logoStateFailed {
		t.Fatal("a failed chain was queued again at once")
	}
	e.exec(`UPDATE radio_stations SET logo_tried_at = 1 WHERE id = ?`, id)
	e.s.queueLogos(context.Background(), []int64{id})
	if e.stationRow(id).LogoState != "queued" {
		t.Fatal("a chain that failed 30 days ago was not queued again")
	}
}

func TestLogoICOLargestEntry(t *testing.T) {
	if w, h, ok := logoSize(icoOf(t, 16, 0, 48)); !ok || w != 256 || h != 256 {
		t.Fatalf("ICO size %d×%d %v, want 256×256 (width 0)", w, h, ok)
	}
	if w, h, ok := logoSize(icoOf(t, 16, 32)); !ok || w != 32 || h != 32 {
		t.Fatalf("ICO size %d×%d", w, h)
	}
	vp8l := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8L\x00\x00\x00\x00\x2f"), 0, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(vp8l[21:], uint32(199)|uint32(99)<<14)
	if w, h, ok := logoSize(vp8l); !ok || w != 200 || h != 100 {
		t.Fatalf("VP8L size %d×%d %v", w, h, ok)
	}
	vp8x := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x00\x00\x00\x00"), 0xFF, 0x01, 0, 0x7F, 0, 0)
	if w, h, ok := logoSize(vp8x); !ok || w != 512 || h != 128 {
		t.Fatalf("VP8X size %d×%d %v", w, h, ok)
	}
	if _, _, ok := logoSize([]byte(`<svg width="512" height="512"/>`)); ok {
		t.Fatal("SVG accepted")
	}
	// An ICO logo goes through the chain.
	e := newLogoEnv(t)
	id := e.addStation("Ico FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ? WHERE id = ?`, serve(e.st, "/favicon.ico", "image/x-icon", icoOf(t, 16, 0)), id)
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState != logoStateCatalog {
		t.Fatalf("ICO: state %q", st.LogoState)
	}
}

func TestLogoNeverLocal(t *testing.T) {
	e := newLogoEnv(t)
	e.s.rel = newRelayState() // the production dial control
	e.s.rel.control = netguard.RefuseLocal
	id := e.addStation("Local FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ? WHERE id = ?`, serve(e.st, "/fav.png", "image/png", pngOf(t, 200, 200)), id)
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState != logoStateFailed {
		t.Fatalf("state %q, want failed", st.LogoState)
	}
	if n := e.st.Connections("/fav.png"); n != 0 {
		t.Fatalf("the favicon on 127.0.0.1 was fetched %d times", n)
	}
}

func TestOwnLogoWins(t *testing.T) {
	e := newLogoEnv(t)
	ctx := context.Background()
	id := e.addStation("Own FM", "CH", "MP3", 128)
	e.exec(`UPDATE radio_stations SET favicon_url = ? WHERE id = ?`, serve(e.st, "/fav.png", "image/png", pngOf(t, 200, 200)), id)
	if err := e.s.setOwnLogo(ctx, id, bytes.NewReader(pngOf(t, 300, 150))); err != nil {
		t.Fatal(err)
	}
	own := e.stationRow(id)
	if own.LogoState != logoStateOwn || !own.LogoHash.Valid {
		t.Fatalf("after the upload %+v", own)
	}
	fields, err := appq.New(e.f.DB.Read).ListRadioStationLocks(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0] != "logo" {
		t.Fatalf("locks %v, want logo", fields)
	}
	e.s.queueLogos(ctx, []int64{id})
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState != logoStateOwn || st.LogoHash != own.LogoHash {
		t.Fatalf("the job replaced the own logo: %+v", st)
	}
	e.checkLogoFiles(id)
	var ve *model.ValidationError
	if err := e.s.setOwnLogo(ctx, id, bytes.NewReader([]byte("<svg/>"))); !errors.As(err, &ve) || ve.Fields[0].Code != "unsupported_format" {
		t.Fatalf("SVG upload: %v", err)
	}
	if err := e.s.setOwnLogo(ctx, 9999, bytes.NewReader(pngOf(t, 100, 100))); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown station: %v", err)
	}
	if err := e.s.deleteOwnLogo(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st := e.stationRow(id); st.LogoState != "queued" || st.LogoHash.Valid {
		t.Fatalf("after the delete %+v, want queued for the chain", st)
	}
	if _, err := os.Stat(filepath.Join(e.s.o.cacheDir, logoDir(id))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("logo files left: %v", err)
	}
	e.runLogoJob(id)
	if st := e.stationRow(id); st.LogoState != logoStateCatalog {
		t.Fatalf("chain after the delete: %q", st.LogoState)
	}
}

func TestLogoJobTypeAndSyncPrefetch(t *testing.T) {
	e := newLogoEnv(t)
	jt := e.s.logoJobType()
	if jt.Name != JobLogoFetch || jt.Group != jobs.GroupMetadata || jt.Pool != jobs.PoolGeneral ||
		jt.Trigger != jobs.TriggerEvent || jt.Priority != 0 || jt.Handler == nil {
		t.Fatalf("job type %+v", jt)
	}
	jobs.NewRegistry(e.s.o.queue, testLogger()).Register(jt)
	fav := e.addStation("Fav FM", "CH", "MP3", 128)
	rec := e.addStation("Rec FM", "CH", "MP3", 128)
	other := e.addStation("Other FM", "CH", "MP3", 128)
	e.exec(`INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (?, ?, 1, 1)`, e.f.P6.ID, fav)
	e.exec(`UPDATE radio_stations SET recommended = 1 WHERE id = ?`, rec)
	e.s.queueSyncLogos(context.Background())
	for id, want := range map[int64]string{fav: "queued", rec: "queued", other: logoStateNone} {
		if st := e.stationRow(id); st.LogoState != want {
			t.Errorf("station %d: logo state %q, want %q", id, st.LogoState, want)
		}
	}
	var n int
	if err := e.f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key LIKE 'radio.logo:%'`, JobLogoFetch).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d logo jobs queued (%v), want 2", n, err)
	}
}
