package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// recorder is a dispatcher that records every call.
type recorder struct {
	mu    sync.Mutex
	calls map[int64][]string // profile id -> episode ids
}

func (r *recorder) Dispatch(_ context.Context, n model.Notification, p model.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n.ProfileID != p.ID || n.Kind != KindNewEpisode || n.ID == 0 {
		return errors.New("unexpected notification")
	}
	r.calls[p.ID] = append(r.calls[p.ID], n.Item.ID)
	return nil
}

func (r *recorder) of(p model.Profile) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls[p.ID])
}

type env struct {
	f   *vistest.Fixture
	svc *Service
	rec *recorder
	now time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	rec := &recorder{calls: map[int64][]string{}}
	e := &env{f: f, rec: rec, now: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)}
	e.svc = New(f.DB, visibility.New(f.DB, vistest.Logger()), vistest.Logger(), WithDispatcher(rec),
		WithClock(func() time.Time { return e.now }))
	return e
}

func (e *env) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.f.DB.Write.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (e *env) favorite(t *testing.T, itemID string, profiles ...model.Profile) {
	t.Helper()
	for _, p := range profiles {
		e.exec(t, `INSERT INTO favorites (profile_id, item_id, created_at) VALUES (?, ?, 1)`, p.ID, itemID)
	}
}

// addEpisode inserts a new episode with a file into season 1 of the fixture series (own rating
// rating, nil = the series' rating) and returns its id.
func (e *env) addEpisode(t *testing.T, label string, rating *int) string {
	t.Helper()
	id := "01J9000000000000000000N0" + label
	var own sql.NullInt64
	if rating != nil {
		own = sql.NullInt64{Int64: int64(*rating), Valid: true}
	}
	e.exec(t, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, season_number, episode_number, own_rating,
	           added_at, created_at, updated_at) VALUES (?, 'episode', ?, ?, ?, ?, 1, 9, ?, 2, 2, 2)`,
		id, e.f.Show.Season.ID, e.f.LibraryID(), "Episode "+label, "0001 0009", own)
	e.exec(t, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	           SELECT ?, ?, root_id, ?, size, 1, 'mp4', 2, 2 FROM media_files WHERE item_id = ?`,
		"01J9000000000000000000G0"+label, id, "Show/Season 01/Show - S01E09 "+label+".mp4", e.f.Show.E1.ID)
	return id
}

func (e *env) inbox(t *testing.T, p model.Profile) ([]string, int, int) {
	t.Helper()
	list, total, unread, err := e.svc.Notifications(context.Background(), p, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range list {
		ids = append(ids, n.Item.ID)
	}
	return ids, total, unread
}

func ptr(v int) *int { return &v }

// 5.5, E-8: a new episode reaches the inbox of a profile with the series as favourite only when the
// profile may see the episode; the dispatcher is never called for the others.
func TestNewEpisodesVisibility(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	e.favorite(t, f.Show.Series.ID, f.P6, f.P12, f.P18)

	e16 := e.addEpisode(t, "16", ptr(16))
	if err := e.svc.NewEpisodes(ctx, []string{e16}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []model.Profile{f.P6, f.P12} {
		if ids, total, _ := e.inbox(t, p); len(ids) != 0 || total != 0 || len(e.rec.of(p)) != 0 {
			t.Errorf("profile %s: inbox %v (total %d), dispatched %v for an FSK 16 episode", p.Name, ids, total, e.rec.of(p))
		}
	}
	if ids, total, unread := e.inbox(t, f.P18); !slices.Equal(ids, []string{e16}) || total != 1 || unread != 1 {
		t.Errorf("P18 inbox %v total %d unread %d", ids, total, unread)
	}

	e.now = e.now.Add(time.Minute)
	e12 := e.addEpisode(t, "12", nil)
	if err := e.svc.NewEpisodes(ctx, []string{e12, f.Show.Extra.ID, f.Rated(6).ID, "01J9000000000000000000ZZZZ"}); err != nil {
		t.Fatal(err)
	}
	if ids, _, _ := e.inbox(t, f.P12); !slices.Equal(ids, []string{e12}) {
		t.Errorf("P12 inbox %v, want the series-level episode", ids)
	}
	if ids, total, unread := e.inbox(t, f.P18); !slices.Equal(ids, []string{e12, e16}) || total != 2 || unread != 2 {
		t.Errorf("P18 inbox %v total %d unread %d, want newest first", ids, total, unread)
	}
	if got := e.rec.of(f.P6); len(got) != 0 {
		t.Errorf("P6 dispatched %v", got)
	}
	// A second event for the same episode adds nothing.
	if err := e.svc.NewEpisodes(ctx, []string{e12}); err != nil {
		t.Fatal(err)
	}
	if got := e.rec.of(f.P18); !slices.Equal(got, []string{e16, e12}) {
		t.Errorf("P18 dispatched %v", got)
	}

	// The entry carries the parent when it is visible.
	list, _, _, err := e.svc.Notifications(ctx, f.P18, 0, 1)
	if err != nil || len(list) != 1 || list[0].Parent == nil || list[0].Parent.ID != f.Show.Series.ID || list[0].ReadAt != nil {
		t.Fatalf("P18 first entry = %+v, %v", list, err)
	}
	// An episode that is gone leaves the inbox and its counts.
	e.exec(t, `UPDATE items SET deleted_at = 3 WHERE id = ?`, e16)
	if ids, total, unread := e.inbox(t, f.P18); !slices.Equal(ids, []string{e12}) || total != 1 || unread != 1 {
		t.Errorf("P18 inbox after the deletion %v total %d unread %d", ids, total, unread)
	}
}

func TestPodcastEpisodes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	a := e.f.Audio
	e.favorite(t, a.Podcast.ID, e.f.P16, e.f.P18)
	if err := e.svc.NewEpisodes(ctx, []string{a.Episodes[3].ID, a.Explicit.ID}); err != nil {
		t.Fatal(err)
	}
	if ids, _, _ := e.inbox(t, e.f.P16); !slices.Equal(ids, []string{a.Episodes[3].ID}) {
		t.Errorf("P16 inbox %v, want only the FSK 16 episode", ids)
	}
	if ids, _, _ := e.inbox(t, e.f.P18); len(ids) != 2 {
		t.Errorf("P18 inbox %v, want both", ids)
	}
}

func TestMarkRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	e.favorite(t, f.Show.Series.ID, f.P12, f.P18)
	ep := e.addEpisode(t, "12", nil)
	if err := e.svc.NewEpisodes(ctx, []string{ep}); err != nil {
		t.Fatal(err)
	}
	list, _, _, err := e.svc.Notifications(ctx, f.P18, 0, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("P18 inbox %v %v", list, err)
	}
	id := list[0].ID
	if err := e.svc.MarkRead(ctx, f.P12, id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("foreign entry: %v", err)
	}
	if err := e.svc.MarkRead(ctx, f.P18, id+1000); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown entry: %v", err)
	}
	if err := e.svc.MarkRead(ctx, f.P18, id); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.MarkRead(ctx, f.P18, id); err != nil {
		t.Errorf("second mark: %v", err)
	}
	if _, total, unread := e.inbox(t, f.P18); total != 1 || unread != 0 {
		t.Errorf("P18 after read: total %d unread %d", total, unread)
	}
	if _, _, unread := e.inbox(t, f.P12); unread != 1 {
		t.Errorf("P12 unread %d, want its own entry untouched", unread)
	}
	if err := e.svc.MarkAllRead(ctx, f.P12); err != nil {
		t.Fatal(err)
	}
	if _, total, unread := e.inbox(t, f.P12); total != 1 || unread != 0 {
		t.Errorf("P12 after read-all: total %d unread %d", total, unread)
	}
	// An entry whose episode became invisible cannot be marked.
	e.exec(t, `UPDATE items SET own_rating = 16 WHERE id = ?`, ep)
	p12, _, _, err := e.svc.Notifications(ctx, f.P12, 0, 10)
	if err != nil || len(p12) != 0 {
		t.Fatalf("P12 inbox after the rating change: %v %v", p12, err)
	}
	var p12ID int64
	if err := e.f.DB.Read.QueryRow(`SELECT id FROM notifications WHERE profile_id = ?`, f.P12.ID).Scan(&p12ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.MarkRead(ctx, f.P12, p12ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible entry: %v", err)
	}
}

// k3, k4 (M7 review): without any favourite a batch stores nothing; one call notifies at most
// MaxPerParent episodes of one series, in the order of the ids, and a later call notifies the rest.
func TestNewEpisodesBatchAndCap(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	var ids []string
	for i := range MaxPerParent + 5 {
		ids = append(ids, e.addEpisode(t, fmt.Sprintf("%02d", i), nil))
	}
	if err := e.svc.NewEpisodes(ctx, ids); err != nil {
		t.Fatal(err)
	}
	var stored int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM notifications`).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("notifications without a favourite = %d, %v", stored, err)
	}

	e.favorite(t, f.Show.Series.ID, f.P12, f.P18)
	e.favorite(t, f.Audio.Podcast.ID, f.P18)
	// The podcast episode counts for its own parent, not for the series.
	batch := append(slices.Clone(ids), f.Audio.Episodes[0].ID)
	if err := e.svc.NewEpisodes(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if got := e.rec.of(f.P12); !slices.Equal(got, ids[:MaxPerParent]) {
		t.Errorf("P12 dispatched %d episodes, want the first %d in order", len(got), MaxPerParent)
	}
	if _, total, _ := e.inbox(t, f.P18); total != MaxPerParent+1 {
		t.Errorf("P18 inbox total %d, want %d series episodes and the podcast episode", total, MaxPerParent+1)
	}
	if err := e.svc.NewEpisodes(ctx, ids[MaxPerParent:]); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := e.inbox(t, f.P12); total != MaxPerParent+5 {
		t.Errorf("P12 inbox total %d after the second call, want %d", total, MaxPerParent+5)
	}
}

// n2 (M7 review): "mark all read" leaves the entries of episodes the profile may not see untouched.
func TestMarkAllReadOnlyVisible(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	e.favorite(t, f.Show.Series.ID, f.P12)
	a := e.addEpisode(t, "0A", nil)
	b := e.addEpisode(t, "0B", nil)
	if err := e.svc.NewEpisodes(ctx, []string{a, b}); err != nil {
		t.Fatal(err)
	}
	e.exec(t, `UPDATE items SET own_rating = 16 WHERE id = ?`, b)
	if err := e.svc.MarkAllRead(ctx, f.P12); err != nil {
		t.Fatal(err)
	}
	e.exec(t, `UPDATE items SET own_rating = NULL WHERE id = ?`, b)
	if ids, total, unread := e.inbox(t, f.P12); total != 2 || unread != 1 {
		t.Errorf("P12 inbox %v total %d unread %d, want the hidden entry still unread", ids, total, unread)
	}
}
