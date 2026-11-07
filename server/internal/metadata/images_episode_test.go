package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The episode still («Folgenbild», concept 15 7.5) is chosen like a poster: the TMDB stills of the
// episode are its candidates, a choice locks it, lifting the lock brings the provider's still back
// through a match of the episode. With the setting "episode stills" off the kind is not choosable;
// lifting a lock still works.
func TestEpisodeStillChoice(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.mux.HandleFunc("GET /3/tv/900/season/1/episode/1/images", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("language") || r.URL.Query().Has("include_image_language") {
			t.Errorf("stills asked with a language filter: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"stills": []map[string]any{
			tmdbImage("/still-a.jpg", nil, 1920, 1080, 5.5, 4),
			tmdbImage("/still-b.jpg", "de", 1280, 720, 4.0, 2),
		}})
	})
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	auto := e.itemImages(t, idE0101)["backdrop"]
	if auto.Source == "" || auto.Chosen != 0 {
		t.Fatalf("automatic still %+v, want the provider's", auto)
	}

	kinds, err := e.svc.ChoosableImageKinds(ctx, idE0101)
	if err != nil || !slices.Equal(kinds, []string{"backdrop"}) {
		t.Fatalf("choosable kinds of an episode %v, %v; want [backdrop]", kinds, err)
	}
	list, err := e.svc.ImageCandidates(ctx, idE0101, "backdrop", false)
	if err != nil {
		t.Fatal(err)
	}
	var srcs []string
	for _, c := range list.Candidates {
		if c.Provider != model.ProviderTMDB || !strings.Contains(c.PreviewURL, "/w300/") {
			t.Errorf("candidate %+v: want TMDB with a w300 preview", c)
		}
		srcs = append(srcs, c.Source)
	}
	if !slices.Contains(srcs, "/still-a.jpg") || !slices.Contains(srcs, "/still-b.jpg") {
		t.Fatalf("still candidates %v", srcs)
	}
	var verr *model.ValidationError
	if _, err := e.svc.ImageCandidates(ctx, idE0101, "poster", false); !errors.As(err, &verr) || verr.Fields[0].Field != "kind" {
		t.Fatalf("poster of an episode: %v, want 422 kind", err)
	}

	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.SetItemImage(ctx, idE0101, "backdrop", model.ProviderTMDB, "/still-b.jpg"); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idE0101)["backdrop"]; got.Source != "/still-b.jpg" || got.Chosen != 1 {
		t.Fatalf("after the choice %+v", got)
	}
	if r := e.item(t, idE0101); !strings.Contains(r.Locked, `"backdrop"`) {
		t.Fatalf("locked fields %s, want backdrop", r.Locked)
	}
	if got := e.imageJobs(t); len(got) != 1 || !strings.HasPrefix(got[0], "image:"+idE0101+" p7") {
		t.Fatalf("image jobs %v, want one at the manual priority", got)
	}

	// Lifting the lock: the episode turns pending, its match decides the provider's still again.
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.ClearItemImage(ctx, idE0101, "backdrop"); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idE0101); r.State != statePending || strings.Contains(r.Locked, `"backdrop"`) {
		t.Fatalf("after clearing: state %s, locked %s", r.State, r.Locked)
	}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, []string{"match:" + idE0101 + " p7"}) {
		t.Fatalf("match jobs %v after clearing", got)
	}
	if err := e.svc.match(ctx, idE0101); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idE0101)["backdrop"]; got.Source != auto.Source || got.Chosen != 0 {
		t.Fatalf("after the match %+v, want the automatic %q", got, auto.Source)
	}

	// The setting "episode stills" off: no kind to choose, candidates and choice are 422, the
	// lock is lifted anyway.
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '["logo","season_poster"]', 0)`, SettingImageKinds)
	if kinds, err := e.svc.ChoosableImageKinds(ctx, idE0101); err != nil || len(kinds) != 0 {
		t.Fatalf("choosable kinds with the setting off %v, %v; want none", kinds, err)
	}
	if kinds, err := e.svc.ChoosableImageKinds(ctx, idSeason1); err != nil || !slices.Equal(kinds, []string{"poster"}) {
		t.Fatalf("choosable kinds of a season %v, %v", kinds, err)
	}
	if _, err := e.svc.ImageCandidates(ctx, idE0101, "backdrop", false); !errors.As(err, &verr) {
		t.Fatalf("candidates with the setting off: %v, want 422", err)
	}
	if err := e.svc.SetItemImage(ctx, idE0101, "backdrop", model.ProviderTMDB, "/still-a.jpg"); !errors.As(err, &verr) {
		t.Fatalf("choice with the setting off: %v, want 422", err)
	}
	if err := e.svc.ClearItemImage(ctx, idE0101, "backdrop"); err != nil {
		t.Fatalf("clearing with the setting off: %v", err)
	}
	if _, err := e.svc.ChoosableImageKinds(ctx, "01ZZZZZZZZZZZZZZZZZZZZZZZZ"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown title: %v", err)
	}
}
