package playback

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 18 part A: a start of a film reports PlaybackInfo.trickplay (queued, no sheets, the grid
// of the file) and queues trickplay.generate at priority 9 from the start position; getTrickplay and
// getTrickplaySheet report and serve the sheets of the file's version for a profile that sees the
// title and answer 404 otherwise; the setting off reports nothing.
func TestStartTrickplay(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddTracks(t)
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	f.SetProbedAt(t, m.FileID, 4242)
	cache := t.TempDir()
	s := New(f.DB, vistest.Logger(), WithTranscoder(newFakeTranscoder()), WithTokenKey(testKey), WithCacheDir(cache))
	ctx := context.Background()
	session := browserSession(t, f, f.P6)

	resume := int64(1_250_000)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &capsChromeLinux, PositionMs: &resume})
	if err != nil {
		t.Fatal(err)
	}
	tp := info.Trickplay
	if tp == nil || tp.State != library.TrickplayQueued || len(tp.Sheets) != 0 || tp.TileWidth != 320 || tp.TileHeight != 180 ||
		tp.Columns != 10 || tp.Rows != 10 || tp.IntervalMs != 10_000 || len(tp.PointsMs) == 0 {
		t.Fatalf("trickplay of the first start %+v", tp)
	}
	var prio int
	var payload string
	if err := f.DB.Read.QueryRow(`SELECT priority, payload FROM jobs WHERE type = ? AND idempotency_key = ?`,
		library.JobTrickplay, "trickplay:"+m.FileID).Scan(&prio, &payload); err != nil {
		t.Fatal(err)
	}
	if prio != 9 || !strings.Contains(payload, `"startMs":1250000`) || !strings.Contains(payload, `"origin":"start"`) {
		t.Errorf("start job %d %s", prio, payload)
	}

	// The job wrote the manifest and sheet 0 of the current version.
	set := filepath.Join(cache, library.TrickplayDir, trickplayFolder(m.FileID, tp.Version))
	if err := os.MkdirAll(set, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := library.TrickplayManifest{Version: tp.Version, IntervalMs: tp.IntervalMs, TileWidth: 320, TileHeight: 180, Columns: 10, Rows: 10,
		PointsMs: tp.PointsMs, SeekMs: tp.PointsMs, Source: library.TrickplaySourceIndex}
	raw, _ := json.Marshal(manifest)
	for name, data := range map[string]string{"manifest.json": string(raw), "0.webp": "RIFF-sheet-0"} {
		if err := os.WriteFile(filepath.Join(set, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Trickplay(ctx, f.P6, m.ID, m.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sheets) != 1 || got.Sheets[0].Index != 0 || got.Sheets[0].URL != TrickplaySheetURL(m.ID, m.FileID, 0, tp.Version) || got.State != library.TrickplayQueued {
		t.Errorf("getTrickplay %+v", got)
	}
	src, version, err := s.TrickplaySheet(ctx, f.P6, m.ID, m.FileID, 0)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(src.File)
	_ = src.File.Close()
	if string(body) != "RIFF-sheet-0" || src.MediaType != "image/webp" || version != tp.Version || src.ETag != TrickplaySheetETag(tp.Version, 0) {
		t.Errorf("sheet %q %s %s %s", body, src.MediaType, version, src.ETag)
	}
	for _, c := range []struct {
		name    string
		profile model.Profile
		item    string
		file    string
		sheet   int
	}{
		{"missing sheet", f.P6, m.ID, m.FileID, 1},
		{"not a ULID", f.P6, m.ID, "x", 0},
		{"title above the rating", f.P6, f.Rated(18).ID, f.Rated(18).FileID, 0},
		{"file of another title", f.P6, m.ID, f.Rated(0).FileID, 0},
	} {
		if _, _, err := s.TrickplaySheet(ctx, c.profile, c.item, c.file, c.sheet); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("sheet, %s: %v", c.name, err)
		}
		if c.sheet == 0 {
			if _, err := s.Trickplay(ctx, c.profile, c.item, c.file); !errors.Is(err, model.ErrNotFound) {
				t.Errorf("getTrickplay, %s: %v", c.name, err)
			}
		}
	}

	// The marker: complete.
	if err := os.WriteFile(filepath.Join(set, "complete"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Trickplay(ctx, f.P6, m.ID, m.FileID); err != nil || got.State != library.TrickplayComplete {
		t.Errorf("complete set: %+v, %v", got, err)
	}

	// The setting off: no trickplay at a start, getTrickplay and the sheets 404.
	if _, err := f.DB.Write.Exec(`UPDATE libraries SET trickplay_movies = 'off' WHERE id = ?`, f.LibraryID()); err != nil {
		t.Fatal(err)
	}
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &capsChromeLinux})
	if err != nil {
		t.Fatal(err)
	}
	if info.Trickplay != nil {
		t.Errorf("setting off reports %+v", info.Trickplay)
	}
	if _, err := s.Trickplay(ctx, f.P6, m.ID, m.FileID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("getTrickplay at off: %v", err)
	}
	if _, _, err := s.TrickplaySheet(ctx, f.P6, m.ID, m.FileID, 0); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("sheet at off: %v", err)
	}
}

// trickplayFolder returns the folder name of the set of a file from the version of the set
// (library.TrickplaySetVersion "<size>-<mtime>-<source><height>": the folder carries size and mtime).
func trickplayFolder(fileID, version string) string {
	parts := strings.SplitN(version, "-", 3)
	return fileID + "-" + parts[0] + "-" + parts[1]
}
