package offline

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 18, 6.8: a film download waits for the trickplay set of its file (and queues it), copies
// it into the download once it is complete, counts it in the size of the copy and names its sheets
// in the manifest with the download token; after 10 minutes without a set, or at the setting off,
// the download is ready without preview.
func TestDownloadCarriesTrickplay(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	e := newEnv(t, WithCacheDir(cache))
	ctx := context.Background()
	f := e.f
	m := f.Rated(6)
	e.exec(`UPDATE media_files SET probed_at = 1, duration_ms = 20000 WHERE id = ?`, m.FileID)
	st := e.create(f.P12, e.devA, m.ID, model.DownloadLow)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadPreparing {
		t.Fatalf("state while the set is missing: %s", r.state)
	}
	var prio int
	if err := f.DB.Read.QueryRow(`SELECT priority FROM jobs WHERE type = ? AND idempotency_key = ?`, library.JobTrickplay, "trickplay:"+m.FileID).Scan(&prio); err != nil || prio != 5 {
		t.Fatalf("download trickplay job priority %d (%v)", prio, err)
	}
	// The set is complete now.
	var size, mtime int64
	if err := f.DB.Read.QueryRow(`SELECT size, mtime FROM media_files WHERE id = ?`, m.FileID).Scan(&size, &mtime); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(cache, library.TrickplayDir, library.TrickplaySetName(m.FileID, library.FileVersion{Size: size, Mtime: mtime}))
	if err := os.MkdirAll(set, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(library.TrickplayManifest{Version: "v", IntervalMs: 10_000, TileWidth: 320, TileHeight: 180, Columns: 10, Rows: 10,
		PointsMs: []int64{0, 10_000}, SeekMs: []int64{1, 10_001}, Source: library.TrickplaySourceIndex})
	for name, data := range map[string]string{"manifest.json": string(manifest), "0.webp": "RIFF-sheet", "complete": ""} {
		if err := os.WriteFile(filepath.Join(set, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.clock.Advance(time.Minute)
	e.runJobs()
	r := e.row(st.ID)
	if r.state != model.DownloadReady {
		t.Fatalf("state after the set: %s %s", r.state, r.err)
	}
	if want := int64(len(mintedMediaOf(t, e, st.ID))) + int64(len("RIFF-sheet")); r.size < want {
		t.Errorf("size %d does not count the previews (want at least %d)", r.size, want)
	}
	man, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st.ID)
	if err != nil {
		t.Fatal(err)
	}
	tp := man.Trickplay
	if tp == nil || tp.State != library.TrickplayComplete || len(tp.Sheets) != 1 || len(tp.PointsMs) != 2 ||
		!strings.HasPrefix(tp.Sheets[0].URL, "/api/v1/downloads/"+st.ID+"/trickplay/0?dt=") {
		t.Fatalf("manifest trickplay %+v", tp)
	}
	g := model.DownloadGrant{DownloadID: st.ID, ItemID: m.ID, AccountID: f.Account.ID, ProfileID: f.P12.ID}
	src, err := e.svc.OpenTrickplaySheet(ctx, g, 0)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(src.File)
	_ = src.File.Close()
	if string(body) != "RIFF-sheet" || src.MediaType != "image/webp" {
		t.Errorf("sheet %q %s", body, src.MediaType)
	}
	if _, err := e.svc.OpenTrickplaySheet(ctx, g, 1); err == nil {
		t.Error("a sheet the download does not carry opened")
	}

	// Another film: no set within 10 minutes, the download goes on without preview.
	other := f.Rated(0)
	e.exec(`UPDATE media_files SET probed_at = 1, duration_ms = 20000 WHERE id = ?`, other.FileID)
	st2 := e.create(f.P12, e.devA, other.ID, model.DownloadLow)
	e.runJobs()
	marker := filepath.Join(e.dir, st2.ID, trickplayWaitMarker)
	old := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute)
	e.runJobs()
	if r := e.row(st2.ID); r.state != model.DownloadReady {
		t.Fatalf("state after the wait: %s", r.state)
	}
	if man, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st2.ID); err != nil || man.Trickplay != nil {
		t.Errorf("download without set carries %+v (%v)", man.Trickplay, err)
	}

	// The setting off: ready at once, without preview.
	e.exec(`UPDATE libraries SET trickplay_movies = 'off'`)
	third := f.Rated(12)
	e.exec(`UPDATE media_files SET probed_at = 1, duration_ms = 20000 WHERE id = ?`, third.FileID)
	st3 := e.create(f.P12, e.devA, third.ID, model.DownloadLow)
	e.runJobs()
	if r := e.row(st3.ID); r.state != model.DownloadReady {
		t.Errorf("state at off: %s", r.state)
	}
}

// mintedMediaOf returns the prepared file of a download.
func mintedMediaOf(t *testing.T, e *env, id string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, id, mediaFile))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
