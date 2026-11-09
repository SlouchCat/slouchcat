package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// contractTrickplayCaches maps a profile fixture (*vistest.Fixture) to the cache directory of its
// playback service, so the registry checks of getTrickplay and getTrickplaySheet can write a set.
var contractTrickplayCaches sync.Map

// checkContractTrickplay is the positive check of getTrickplay (sheet false) and getTrickplaySheet
// (sheet true) in the profile registry (concept 18, 8.2): the fixture's files carry no probe, so a
// visible film or episode first answers 404 (no preview); with a probe stored for the moment, a
// visible title answers the set (queued, no sheets) to the profile, and with a set written in the
// cache its sheet, immutable under the version of the set and revalidated under another. The probe
// and the set go again before the next operation.
func checkContractTrickplay(t *testing.T, f *vistest.Fixture, h http.Handler, token string, m vistest.Movie, sheet bool) {
	t.Helper()
	if m.Kind != "" && m.Kind != "episode" {
		return // no film or episode: no preview at all
	}
	dir, ok := contractTrickplayCaches.Load(f)
	if !ok {
		t.Fatal("no playback cache of the fixture")
	}
	raw, _ := chapterTestProbe(t)
	f.SetProbe(t, m.FileID, raw)
	defer f.SetProbe(t, m.FileID, "") // the fixture's files carry no probe
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/trickplay?mediaFileId="+m.FileID, token, "", nil)
	var tp struct {
		Version, State        string
		TileWidth, TileHeight int
		PointsMs              []int64
		Sheets                []struct {
			Index int
			URL   string `json:"url"`
		}
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &tp) != nil {
		t.Fatalf("getTrickplay of %s for %s with a probe: %d %s", m.Label, token, rec.Code, rec.Body.String())
	}
	if tp.State != library.TrickplayQueued || tp.Version == "" || tp.TileWidth != 320 || tp.TileHeight != 180 || len(tp.PointsMs) == 0 ||
		tp.Sheets == nil || len(tp.Sheets) != 0 {
		t.Errorf("getTrickplay of %s for %s: %s", m.Label, token, rec.Body.String())
	}
	if !sheet {
		return
	}
	body := writeTrickplaySet(t, dir.(string), m.FileID, tp.Version, tp.PointsMs)
	parts := strings.SplitN(tp.Version, "-", 3)
	defer func() {
		_ = os.RemoveAll(filepath.Join(dir.(string), library.TrickplayDir, m.FileID+"-"+parts[0]+"-"+parts[1]))
	}()
	path := BasePath + "/items/" + m.ID + "/trickplay/0?mediaFileId=" + m.FileID
	for _, c := range []struct {
		v, cache string
	}{
		{tp.Version, "private, max-age=31536000, immutable"},
		{"1-1-index180", "private, no-cache"},
	} {
		got := send(t, h, http.MethodGet, path+"&v="+c.v, token, "", nil)
		if got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), body) || got.Header().Get("Content-Type") != "image/webp" ||
			got.Header().Get("Cache-Control") != c.cache || got.Header().Get("ETag") != playback.TrickplaySheetETag(tp.Version, 0) {
			t.Errorf("getTrickplaySheet of %s for %s with v %s: %d %v", m.Label, token, c.v, got.Code, got.Header())
		}
	}
}
