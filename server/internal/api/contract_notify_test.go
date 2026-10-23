package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// dispatchRecorder is a notify.Dispatcher that records the episode ids per profile.
type dispatchRecorder struct {
	mu    sync.Mutex
	calls map[int64][]string
}

func (r *dispatchRecorder) Dispatch(_ context.Context, n model.Notification, p model.Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[p.ID] = append(r.calls[p.ID], n.Item.ID)
	return nil
}

func (r *dispatchRecorder) of(p model.Profile) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls[p.ID])
}

// E-8, plan 5.5 and 10.2: a new episode of a favourite series reaches only the inboxes of the
// profiles that may see it, and the dispatcher is called for nobody else. The inbox answers only
// visible ids; another profile's entry answers 404.
//
// The fixture episodes stand in for new ones (the hook of library and metadata hands over the ids
// of new episode items; package api may not write title tables): Show.E2 has its own rating 16,
// Show.E1 the series' rating 12. The fixture series is rated 12, so P6 can never see its episodes:
// P6 holds a favourite (set while the series was rated lower) to prove the negative, P12 and P18
// receive the series-level episode.
func TestContractNotifications(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	rec := &dispatchRecorder{calls: map[int64][]string{}}
	svc := notify.New(f.DB, visibility.New(f.DB, vistest.Logger()), vistest.Logger(), notify.WithDispatcher(rec))
	o := pf.opts
	o.Notify = svc
	h := NewHandler(o)
	ctx := context.Background()
	for _, p := range []model.Profile{f.P6, f.P12, f.P18} {
		if _, err := f.DB.Write.Exec(`INSERT OR IGNORE INTO favorites (profile_id, item_id, created_at) VALUES (?, ?, 1)`, p.ID, f.Show.Series.ID); err != nil {
			t.Fatal(err)
		}
	}
	inbox := func(token string) gen.NotificationPage {
		t.Helper()
		r := send(t, h, http.MethodGet, BasePath+"/notifications", token, "", nil)
		if r.Code != http.StatusOK {
			t.Fatalf("%s: GET /notifications = %d %s", token, r.Code, r.Body.String())
		}
		p := pf.tokens[token]
		movies := slices.Concat(f.All(), f.Show.All())
		assertOnlyVisible(t, f, "listNotifications", p, movies, itemIDsIn(t, r, movies))
		var page gen.NotificationPage
		decode(t, r, &page)
		return page
	}
	items := func(page gen.NotificationPage) []string {
		var out []string
		for _, n := range page.Notifications {
			out = append(out, n.Item.Id)
		}
		return out
	}
	// An episode with its own rating 16: only P18.
	e16 := f.Show.E2.ID
	if err := svc.NewEpisodes(ctx, []string{e16}); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"p6", "p12"} {
		if page := inbox(token); len(page.Notifications) != 0 || page.Total != 0 || page.Unread != 0 {
			t.Errorf("%s inbox after the FSK 16 episode = %+v", token, page)
		}
	}
	page := inbox("p18")
	if got := items(page); !slices.Equal(got, []string{e16}) || page.Total != 1 || page.Unread != 1 {
		t.Fatalf("p18 inbox = %v total %d unread %d", got, page.Total, page.Unread)
	}
	if n := page.Notifications[0]; n.Kind != gen.NewEpisode || n.Parent == nil || n.Parent.Id != f.Show.Series.ID || n.ReadAt != nil {
		t.Errorf("p18 entry = %+v", n)
	}
	if got := rec.of(f.P6); len(got) != 0 {
		t.Errorf("dispatcher called for P6: %v", got)
	}
	if got := rec.of(f.P12); len(got) != 0 {
		t.Errorf("dispatcher called for P12: %v", got)
	}

	// An episode on the series level: P12 and P18, still not P6.
	e12 := f.Show.E1.ID
	if err := svc.NewEpisodes(ctx, []string{e12}); err != nil {
		t.Fatal(err)
	}
	if page := inbox("p6"); len(page.Notifications) != 0 {
		t.Errorf("p6 inbox = %v", items(page))
	}
	p12 := inbox("p12")
	if got := items(p12); !slices.Equal(got, []string{e12}) {
		t.Errorf("p12 inbox = %v", got)
	}
	if got := items(inbox("p18")); len(got) != 2 || !slices.Contains(got, e12) || !slices.Contains(got, e16) {
		t.Errorf("p18 inbox = %v", got)
	}
	if got := rec.of(f.P6); len(got) != 0 {
		t.Errorf("dispatcher called for P6: %v", got)
	}
	if got := rec.of(f.P12); !slices.Equal(got, []string{e12}) {
		t.Errorf("dispatcher for P12: %v", got)
	}

	// Another profile's entry answers 404, the own one 204.
	foreign := p12.Notifications[0].Id
	path := func(id int64) string { return fmt.Sprintf("%s/notifications/%d/read", BasePath, id) }
	for _, token := range []string{"p18", "p6", "pg"} {
		if r := send(t, h, http.MethodPut, path(foreign), token, "", nil); r.Code != http.StatusNotFound || problemCode(t, r) != "not_found" {
			t.Errorf("%s marks P12's entry: %d %s", token, r.Code, r.Body.String())
		}
	}
	if r := send(t, h, http.MethodPut, path(foreign), "p12", "", nil); r.Code != http.StatusNoContent {
		t.Errorf("p12 marks its entry: %d %s", r.Code, r.Body.String())
	}
	if page := inbox("p12"); page.Unread != 0 || page.Notifications[0].ReadAt == nil {
		t.Errorf("p12 after read = %+v", page)
	}
	if page := inbox("p18"); page.Unread != 2 {
		t.Errorf("p18 unread = %d, want its entries untouched", page.Unread)
	}
	if r := send(t, h, http.MethodPost, BasePath+"/notifications/read-all", "p18", "", nil); r.Code != http.StatusNoContent {
		t.Errorf("read-all: %d %s", r.Code, r.Body.String())
	}
	if page := inbox("p18"); page.Unread != 0 || page.Total != 2 {
		t.Errorf("p18 after read-all = %+v", page)
	}

	// Paging: newest first, limit and offset echoed.
	r := send(t, h, http.MethodGet, BasePath+"/notifications?offset=1&limit=1", "p18", "", nil)
	var second gen.NotificationPage
	decode(t, r, &second)
	if r.Code != http.StatusOK || second.Offset != 1 || second.Limit != 1 || second.Total != 2 || len(second.Notifications) != 1 ||
		second.Notifications[0].Item.Id != e16 {
		t.Errorf("second page = %d %+v", r.Code, second)
	}
}
