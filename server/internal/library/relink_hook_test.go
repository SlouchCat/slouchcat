package library

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// tmdbKey is a KeySource with a TMDB token only.
type tmdbKey struct{}

func (tmdbKey) TMDBKey(context.Context) (string, error)            { return "token", nil }
func (tmdbKey) TVDBKey(context.Context) (string, string, error)    { return "", "", nil }
func (tmdbKey) YouTubeKey(context.Context) (string, error)         { return "", nil }
func (tmdbKey) TraktClientID(context.Context) (string, error)      { return "", nil }
func (tmdbKey) FanartKeys(context.Context) (string, string, error) { return "", "", nil }
func (tmdbKey) GoogleBooksKey(context.Context) (string, error)     { return "", nil }

// fakeTMDBHeat answers a title search with Heat (TMDB 949) and its details; anything else is
// unknown.
func fakeTMDBHeat(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("GET /3/search/movie", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"results": []any{map[string]any{"id": 949, "title": "Heat", "release_date": "1995-12-15"}}})
	})
	mux.HandleFunc("GET /3/movie/949", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"id": 949, "title": "Heat", "imdb_id": "tt0113277", "release_date": "1995-12-15", "runtime": 170})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"status_code":34}`, http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The real hook path (ADR 0023): metadata.match finds the TMDB id of a copy whose deleted twin
// carries it, the hook (QueueRelink) queues library.relink and the copy's match stops after the
// title; the job merges the copy into its twin and queues the twin's match, which finds no twin
// and goes on with images and trailer.
func TestRelinkThroughMatchHook(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, lib := e.filmLibrary()
	heat := e.addFilm(dir, lib.ID, "Heat/Heat.mkv", 949)
	e.profiles(1)
	e.progress(1, heat, 700, false)
	e.dropFilm(dir, lib.ID, "Heat/Heat.mkv")
	writeFile(t, dir, "Heat Copy/Heat.mkv", "another release")
	e.scan(lib.ID)
	copyID := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat Copy/Heat.mkv'`)

	srv := fakeTMDBHeat(t)
	meta := metadata.New(e.db, e.queue, tmdbKey{}, t.TempDir(), transcode.Runner{}, discard,
		metadata.WithBaseURL(srv.URL+"/3", srv.URL+"/t/p"), metadata.WithHTTPClient(srv.Client()),
		metadata.WithClock(e.clock.Now), metadata.WithRelink(e.svc.QueueRelink))
	registry := jobs.NewRegistry(e.queue, discard)
	meta.RegisterTypes(registry)
	e.svc.RegisterTypes(registry)
	handlers := map[string]jobs.Handler{}
	for _, typ := range registry.Types() {
		handlers[typ.Name] = typ.Handler
	}
	// run runs the active job of type typ under key with its handler and completes it.
	run := func(typ, key string) {
		t.Helper()
		active, err := e.queue.ListActive(e.ctx, typ)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range active {
			if j.IdempotencyKey != key {
				continue
			}
			if err := handlers[typ](e.ctx, &j); err != nil {
				t.Fatalf("%s %s: %v", typ, key, err)
			}
			e.exec(`DELETE FROM jobs WHERE id = ?`, j.ID)
			return
		}
		t.Fatalf("no active %s job %s", typ, key)
	}

	run(JobMetadataMatch, "match:"+copyID)
	if v := e.str(`SELECT tmdb_id || '/' || metadata_state FROM items WHERE id = ?`, copyID); v != "949/matched" {
		t.Fatalf("copy after the match = %s", v)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobImageFetch, "image:"+copyID); n != 0 {
		t.Error("the copy's match went on to its images")
	}
	run(JobRelink, relinkKey(copyID))
	if n := e.count(`SELECT count(*) FROM items WHERE id = ?`, copyID); n != 0 {
		t.Fatal("copy not merged")
	}
	if v := e.str(`SELECT (deleted_at IS NULL) || '/' || available FROM items WHERE id = ?`, heat); v != "1/1" {
		t.Errorf("film live/available = %s, want 1/1", v)
	}
	if n := e.count(`SELECT count(*) FROM progress WHERE item_id = ? AND position_ms = 700`, heat); n != 1 {
		t.Error("progress of the film lost")
	}
	run(JobMetadataMatch, "match:"+heat)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobImageFetch, "image:"+heat); n != 1 {
		t.Error("the survivor's match did not go on to its images")
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ?`, JobRelink); n != 0 {
		t.Errorf("relink jobs after the survivor's match = %d, want none", n)
	}
}
