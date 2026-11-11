package metadata

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Concept 19 part A, 5.2 and P-14: the planner hook hears a new subscription without episode ids
// (its first fetch never counts as new episodes), a fetch with the ids of the episodes it reports
// as new, and nothing of a fetch without new episodes, so 94 podcasts polled 26 times a day queue
// no empty runs.
func TestPodcastFetchedHook(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	type call struct {
		podcast string
		ids     []string
	}
	var mu sync.Mutex
	var calls []call
	hook := func(ctx context.Context, podcastID string, ids []string) error {
		if d, ok := ctx.Deadline(); !ok || time.Until(d) > newEpisodesTimeout {
			t.Errorf("hook context deadline %v (set %v), want at most %v ahead", d, ok, newEpisodesTimeout)
		}
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, call{podcastID, slices.Sorted(slices.Values(ids))})
		return nil
	}
	e.svc = New(e.db, e.queue, staticKey(""), t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler),
		WithFeedClock(e.clock.now), WithFeedUserAgent("Slouchcat/test"), WithPodcastFetched(hook))
	take := func() []call {
		mu.Lock()
		defer mu.Unlock()
		out := calls
		calls = nil
		return out
	}
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 1 || got[0].podcast != p.ID || len(got[0].ids) != 0 {
		t.Fatalf("subscribe told the planner %+v, want the podcast without episode ids", got)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	e.feed.set(func(f *fakeFeed) { f.n = 3 })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got, want := take(), []string{e.episodes(t, p.ID)["ep-3"].ID}; len(got) != 1 || got[0].podcast != p.ID || !slices.Equal(got[0].ids, want) {
		t.Errorf("fetch told the planner %+v, want %s with %v", got, p.ID, want)
	}
	e.clock.set(e.clock.now().Add(time.Hour))
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if got := take(); len(got) != 0 {
		t.Errorf("a fetch without new episodes told the planner %+v", got)
	}
}

// Concept 19 part A, 5.1 and 5.3: updatePodcast stores the own setting of a podcast (inherit
// returns it to the library), asks the folder check before a setting that stores and runs the
// planner after; the list shows own and effective setting.
func TestUpdatePodcastDownloads(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	var mu sync.Mutex
	var changed []string
	savable := errors.New("root not writable")
	e.svc = New(e.db, e.queue, staticKey(""), t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler),
		WithFeedClock(e.clock.now), WithFeedUserAgent("Slouchcat/test"), WithPodcastSettingHooks(
			func(context.Context, string) error { return savable },
			func(_ context.Context, id string, _ bool) error {
				mu.Lock()
				defer mu.Unlock()
				changed = append(changed, id)
				return nil
			}))
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != model.PodcastInherit || p.Scope != model.PodcastInherit || p.EffectiveMode != model.PodcastLink ||
		p.EffectiveScope != model.PodcastScopeNew || p.EffectiveLastN != 10 || p.LastN != nil || p.LatestEpisodeAt == nil {
		t.Errorf("a new subscription = %+v, want the library's setting", p)
	}
	str := func(v string) *string { return &v }
	n := 3
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Mode: str(model.PodcastSave)}); !errors.Is(err, savable) {
		t.Errorf("«Speichern» on a folder that fails the check: %v", err)
	}
	var verr *model.ValidationError
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Mode: str("keep"), Scope: str("some"), LastN: new(0)}); !errors.As(err, &verr) || len(verr.Fields) != 3 {
		t.Errorf("invalid setting: %v", err)
	}
	got, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Mode: str(model.PodcastLink), Scope: str(model.PodcastScopeLastN), LastN: &n})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != model.PodcastLink || got.Scope != model.PodcastScopeLastN || got.LastN == nil || *got.LastN != 3 ||
		got.EffectiveScope != model.PodcastScopeLastN || got.EffectiveLastN != 3 {
		t.Errorf("own setting = %+v", got)
	}
	got, err = e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Scope: str(model.PodcastInherit)})
	if err != nil || got.Scope != model.PodcastInherit || got.LastN != nil || got.EffectiveLastN != 10 || got.Mode != model.PodcastLink {
		t.Errorf("scope back to the library = %+v %v", got, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(changed) != 2 || changed[0] != p.ID {
		t.Errorf("planner hook heard %v, want the podcast twice", changed)
	}
	if _, err := e.svc.UpdatePodcastDownloads(ctx, "01J9ZZZZZZZZZZZZZZZZZZZZZZ", model.PodcastDownloadsUpdate{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown podcast: %v", err)
	}
}

// Concept 19 part A, 5.3 and 5.4: the folder check is asked only when a podcast changes to
// «Speichern»; while it stores, a change of the scope passes though the check fails. The planner
// hook hears whether the change ends «Speichern» (then it cancels the downloads of the buttons too).
func TestUpdatePodcastDownloadsChecksOnlyOnSave(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	type call struct {
		id   string
		stop bool
	}
	var mu sync.Mutex
	var calls []call
	checks := 0
	e.svc = New(e.db, e.queue, staticKey(""), t.TempDir(), transcode.Runner{}, slog.New(slog.DiscardHandler),
		WithFeedClock(e.clock.now), WithFeedUserAgent("Slouchcat/test"), WithPodcastSettingHooks(
			func(context.Context, string) error {
				mu.Lock()
				defer mu.Unlock()
				checks++
				return errors.New("root not writable")
			},
			func(_ context.Context, id string, stop bool) error {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, call{id, stop})
				return nil
			}))
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE libraries SET podcast_mode = 'save' WHERE id = 3`)
	str := func(v string) *string { return &v }
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Scope: str(model.PodcastScopeAll)}); err != nil {
		t.Errorf("a scope while the podcast stores: %v", err)
	}
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Mode: str(model.PodcastLink)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Scope: str(model.PodcastScopeNew)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdatePodcastDownloads(ctx, p.ID, model.PodcastDownloadsUpdate{Mode: str(model.PodcastInherit)}); err == nil {
		t.Errorf("«Nur Link» → «Wie Bibliothek» (Speichern) passed a failing folder check")
	}
	mu.Lock()
	defer mu.Unlock()
	if checks != 1 {
		t.Errorf("folder checks = %d, want only the one of the change to «Speichern»", checks)
	}
	if want := []call{{p.ID, false}, {p.ID, true}, {p.ID, false}}; !slices.Equal(calls, want) {
		t.Errorf("planner hook heard %+v, want %+v", calls, want)
	}
}
