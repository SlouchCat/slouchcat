package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const (
	fanartKey    = "fanart-project-key-123"
	fanartClient = "fanart-personal-key-456"
)

// fanartCalls records what the fake Fanart.tv API saw.
type fanartCalls struct {
	mu      sync.Mutex
	paths   []string
	headers []http.Header
	queries []string
}

func (c *fanartCalls) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.paths)
}

// serveFanart answers /fanart/v3.2/movies/550 of the fake with one German logo, one poster
// without text and one season poster (never a candidate for a film), and serves the logo itself.
func (e *env) serveFanart(t *testing.T) *fanartCalls {
	t.Helper()
	calls := &fanartCalls{}
	assets := e.tmdb.srv.URL + "/fanart-assets"
	e.tmdb.mux.HandleFunc("GET /fanart/v3.2/movies/{id}", func(w http.ResponseWriter, r *http.Request) {
		calls.mu.Lock()
		calls.paths = append(calls.paths, r.URL.Path)
		calls.headers = append(calls.headers, r.Header.Clone())
		calls.queries = append(calls.queries, r.URL.RawQuery)
		calls.mu.Unlock()
		if r.PathValue("id") != "550" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "First Film",
			"hdmovielogo": []map[string]string{
				{"id": "1", "url": assets + "/fanart/logo550.png", "lang": "de", "likes": "7", "width": "800", "height": "310"},
			},
			"movieposter": []map[string]string{
				{"id": "2", "url": assets + "/fanart/poster550.jpg", "lang": "00", "likes": "3", "width": "1000", "height": "1426"},
			},
			// A foreign host is never a candidate.
			"moviebackground": []map[string]string{
				{"id": "3", "url": "https://elsewhere.example/b.jpg", "lang": "", "likes": "9", "width": "1920", "height": "1080"},
			},
		})
	})
	e.tmdb.serveImage("/fanart-assets/fanart/logo550.png", alphaPNG(t, 800, 310), "image/png")
	return calls
}

// Fanart.tv is never asked without a key or with the switch off (concept 10 step 6, DoD).
func TestFanartNeverAskedWithoutKeyOrSwitch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, tc := range map[string]struct {
		keys testKeys
		on   bool
	}{
		"switch on, no key": {keys: testKeys{tmdb: goodToken}, on: true},
		"key, switch off":   {keys: testKeys{tmdb: goodToken, fanart: fanartKey}, on: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnvWith(t, tc.keys)
			calls := e.serveFanart(t)
			if tc.on {
				mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '1', 0)`, SettingFanartEnabled)
			}
			e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
			if err := e.svc.match(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","tvdb"]' WHERE id = ?`, idByTMDB)
			if err := e.svc.fill(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			if _, err := e.svc.ImageCandidates(ctx, idByTMDB, "logo", true); err != nil {
				t.Fatal(err)
			}
			if n := calls.count(); n != 0 {
				t.Fatalf("Fanart.tv asked %d times", n)
			}
		})
	}
}

// With key and switch the logo of Fanart.tv wins when the folder's sources have none, and lands
// in the cache as PNG and WebP (DoD of step 6); the keys travel as headers, never in the address
// or the log.
func TestFanartLogoInCache(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	ctx := context.Background()
	e := newEnvWith(t, testKeys{tmdb: goodToken, fanart: fanartKey, fanartClient: fanartClient})
	calls := e.serveFanart(t)
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '1', 0)`, SettingFanartEnabled)
	// TMDB has a German poster and no logo.
	e.tmdb.setImages("550", []map[string]any{tmdbImage("/poster550.jpg", "de", 1000, 1500, 5, 5)}, nil, nil)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	chosen := e.itemImages(t, idByTMDB)
	if chosen["logo"].Provider != ImageSourceFanart || !strings.HasSuffix(chosen["logo"].Source, "/fanart-assets/fanart/logo550.png") || chosen["logo"].Lang != "de" {
		t.Fatalf("logo = %+v, want the German Fanart.tv logo (TMDB has none)", chosen["logo"])
	}
	// The German TMDB poster beats the textless Fanart.tv one: the chain decides first, and within
	// a language the source comes last (6.2).
	if chosen["poster"].Provider != "tmdb" || chosen["poster"].Lang != "de" {
		t.Fatalf("poster = %+v, want TMDB before Fanart.tv", chosen["poster"])
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idByTMDB)
	for _, key := range []string{"logo/w400/png", "logo/w400/webp", "logo/w800/png", "logo/w800/webp"} {
		if _, ok := imgs[key]; !ok {
			t.Errorf("logo row %s missing (have %d rows)", key, len(imgs))
		}
	}
	if e.tmdb.count("/fanart-assets/fanart/logo550.png") != 1 {
		t.Errorf("Fanart.tv logo downloaded %d times, want 1", e.tmdb.count("/fanart-assets/fanart/logo550.png"))
	}
	e.checkPipelineKind(t, idByTMDB, "logo")

	calls.mu.Lock()
	if len(calls.paths) == 0 {
		calls.mu.Unlock()
		t.Fatal("Fanart.tv never asked")
	}
	for i := range calls.paths {
		if calls.headers[i].Get("api-key") != fanartKey || calls.headers[i].Get("client-key") != fanartClient {
			t.Errorf("request %d: keys not in the headers", i)
		}
		if strings.Contains(calls.queries[i], fanartKey) || strings.Contains(calls.queries[i], fanartClient) {
			t.Errorf("request %d: a key in the address: %s", i, calls.queries[i])
		}
	}
	calls.mu.Unlock()
	if strings.Contains(e.logs.String(), fanartKey) || strings.Contains(e.logs.String(), fanartClient) {
		t.Error("a Fanart.tv key in the log")
	}

	// The image choice lists the Fanart.tv candidates last, with the CDN's preview; the foreign
	// host and the season poster are left out.
	list, err := e.svc.ImageCandidates(ctx, idByTMDB, "logo", false)
	if err != nil {
		t.Fatal(err)
	}
	last := list.Candidates[len(list.Candidates)-1]
	if last.Provider != ImageSourceFanart || !strings.Contains(last.PreviewURL, "/fanart-assets/preview/") || last.Score != 7 {
		t.Fatalf("last logo candidate %+v, want Fanart.tv with its preview", last)
	}
	backdrops, err := e.svc.ImageCandidates(ctx, idByTMDB, "backdrop", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range backdrops.Candidates {
		if strings.Contains(c.Source, "elsewhere.example") {
			t.Fatalf("a foreign host became a candidate: %+v", c)
		}
	}
}
