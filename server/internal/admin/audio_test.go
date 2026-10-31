package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// dbLibraries answers Get from the libraries table (the vistest libraries).
type dbLibraries struct {
	fakeLibraries
	q *appq.Queries
}

func (d *dbLibraries) Get(ctx context.Context, id int64) (model.Library, error) {
	l, err := d.q.GetLibrary(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, err
	}
	out := model.Library{ID: l.ID, Name: l.Name, Kind: l.Kind}
	if l.DefaultRating.Valid {
		r := int(l.DefaultRating.Int64)
		out.DefaultRating = &r
	}
	return out, nil
}

// fakePodcasts records the refreshed libraries.
type fakePodcasts struct{ refreshed []int64 }

func (p *fakePodcasts) Subscribe(context.Context, int64, string) (model.AdminPodcast, error) {
	return model.AdminPodcast{}, model.ErrFeedUnreachable
}
func (p *fakePodcasts) Unsubscribe(context.Context, string) error { return model.ErrNotFound }
func (p *fakePodcasts) ListPodcasts(context.Context) ([]model.AdminPodcast, error) {
	return []model.AdminPodcast{}, nil
}
func (p *fakePodcasts) RefreshPodcast(context.Context, string) (model.AdminPodcast, error) {
	return model.AdminPodcast{}, model.ErrNotFound
}
func (p *fakePodcasts) QueueAllPodcasts(context.Context, int) ([]int64, error) { return nil, nil }
func (p *fakePodcasts) RefreshLibrary(_ context.Context, id int64) (int, error) {
	p.refreshed = append(p.refreshed, id)
	return 1, nil
}

type audioFixture struct {
	f    *vistest.Fixture
	svc  *Service
	pods *fakePodcasts
	ctx  context.Context
}

func newAudioFixture(t *testing.T) audioFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddAudio(t)
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	pods := &fakePodcasts{}
	svc := New(f.DB, auth.New(f.DB, log), &dbLibraries{q: appq.New(f.DB.Read)}, ring, &fakeTMDB{}, func(string) string { return "" }, log).
		WithPodcasts(pods).WithQueue(jobs.NewQueue(f.DB.Write, nil))
	return audioFixture{f: f, svc: svc, pods: pods, ctx: context.Background()}
}

func (a audioFixture) rating(t *testing.T, id string) (own, effective sql.NullInt64, source sql.NullString) {
	t.Helper()
	if err := a.f.DB.Read.QueryRow(`SELECT own_rating, effective_rating, rating_source FROM items WHERE id = ?`, id).Scan(&own, &effective, &source); err != nil {
		t.Fatal(err)
	}
	return own, effective, source
}

// AN-2: a new default rating reaches the titles rated by the library; manual and feed ratings stay.
// The manual 6 of Books[1] lies below the new default 12: it is raised to 12 (M6 review m-8, AN-2
// "only stricter"); the manual 16 stays.
func TestUpdateLibraryDefaultRating(t *testing.T) {
	a := newAudioFixture(t)
	au := a.f.Audio
	twelve, name := 12, "Hörspiele"
	lib, err := a.svc.UpdateLibrary(a.ctx, au.AudiobookLibraryID, model.LibraryUpdate{Name: &name, DefaultRating: &twelve})
	if err != nil || lib.Name != name || lib.DefaultRating == nil || *lib.DefaultRating != 12 {
		t.Fatalf("update = %+v %v", lib, err)
	}
	for _, tc := range []struct {
		m      vistest.Movie
		own    int64
		source string
	}{{au.Books[0], 12, "library"}, {au.Parts, 12, "library"}, {au.Books[1], 12, "manual"}, {au.Books[3], 16, "manual"}} {
		own, eff, src := a.rating(t, tc.m.ID)
		if own.Int64 != tc.own || eff.Int64 != tc.own || src.String != tc.source {
			t.Errorf("%s: own %v effective %v source %v, want %d %s", tc.m.Label, own, eff, src, tc.own, tc.source)
		}
	}
	// The podcast library: the podcast follows, its episodes inherit, the explicit one stays 18.
	if _, err := a.svc.UpdateLibrary(a.ctx, au.PodcastLibraryID, model.LibraryUpdate{DefaultRating: &twelve}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		m         vistest.Movie
		effective int64
	}{{au.Podcast, 12}, {au.EnclosureOnly, 12}, {au.Episodes[0], 12}, {au.Episodes[3], 16}, {au.Explicit, 18}} {
		if _, eff, _ := a.rating(t, tc.m.ID); eff.Int64 != tc.effective {
			t.Errorf("%s: effective %v, want %d", tc.m.Label, eff, tc.effective)
		}
	}
	// Video libraries have no default rating; unknown libraries are not found; a name is required.
	var verr *model.ValidationError
	if _, err := a.svc.UpdateLibrary(a.ctx, a.f.LibraryID(), model.LibraryUpdate{DefaultRating: &twelve}); !errors.As(err, &verr) ||
		verr.Fields[0].Field != "defaultRating" || verr.Fields[0].Code != "not_allowed" {
		t.Errorf("video library: %v", err)
	}
	empty, seven := " ", 7
	if _, err := a.svc.UpdateLibrary(a.ctx, au.AudiobookLibraryID, model.LibraryUpdate{Name: &empty, DefaultRating: &seven}); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Errorf("invalid update: %v", err)
	}
	if _, err := a.svc.UpdateLibrary(a.ctx, 999, model.LibraryUpdate{Name: &name}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
}

// AN-2 "only stricter": a manual rating below the library default is refused; clearing an
// override restores the library default (or 18 for an explicit title) without a metadata match.
func TestAudioRatingFloor(t *testing.T) {
	a := newAudioFixture(t)
	au := a.f.Audio
	twelve, six, sixteen := 12, 6, 16
	if _, err := a.svc.UpdateLibrary(a.ctx, au.AudiobookLibraryID, model.LibraryUpdate{DefaultRating: &twelve}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.svc.SetItemRating(a.ctx, au.Books[3].ID, &six); !errors.Is(err, model.ErrRatingBelowDefault) {
		t.Errorf("manual 6 below 12: %v", err)
	}
	if own, _, _ := a.rating(t, au.Books[3].ID); own.Int64 != 16 {
		t.Errorf("refused rating changed the title: %v", own)
	}
	r, err := a.svc.SetItemRating(a.ctx, au.Books[0].ID, &sixteen)
	if err != nil || r.OwnRating == nil || *r.OwnRating != 16 || r.RatingSource != "manual" {
		t.Errorf("manual 16 = %+v %v", r, err)
	}
	// A mass rating with one audio title below its default changes nothing.
	movie := a.f.Rated(0)
	if _, err := a.svc.SetItemsRating(a.ctx, []string{movie.ID, au.Parts.ID}, &six); !errors.Is(err, model.ErrRatingBelowDefault) {
		t.Errorf("mass rating: %v", err)
	}
	if own, _, _ := a.rating(t, movie.ID); own.Int64 != 0 {
		t.Errorf("mass rating changed the movie: %v", own)
	}
	// Clearing restores the library default; an explicit episode goes back to 18 from the feed.
	r, err = a.svc.SetItemRating(a.ctx, au.Books[0].ID, nil)
	if err != nil || r.OwnRating == nil || *r.OwnRating != 12 || r.RatingSource != "library" {
		t.Errorf("cleared = %+v %v", r, err)
	}
	if _, err := a.svc.SetItemRating(a.ctx, au.Explicit.ID, &sixteen); err != nil {
		t.Fatal(err)
	}
	if r, err := a.svc.SetItemRating(a.ctx, au.Explicit.ID, nil); err != nil || *r.OwnRating != 18 || r.RatingSource != "feed" {
		t.Errorf("cleared explicit episode = %+v %v", r, err)
	}
	var jobsQueued int
	if err := a.f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = 'metadata.match'`).Scan(&jobsQueued); err != nil || jobsQueued != 0 {
		t.Errorf("metadata matches queued for audio titles: %d %v", jobsQueued, err)
	}
}

// scanLibrary of a podcast library refreshes its subscriptions instead of scanning.
func TestScanPodcastLibrary(t *testing.T) {
	a := newAudioFixture(t)
	lib, err := a.svc.ScanLibrary(a.ctx, a.f.Audio.PodcastLibraryID)
	if err != nil || lib.ID != a.f.Audio.PodcastLibraryID || len(a.pods.refreshed) != 1 || a.pods.refreshed[0] != lib.ID {
		t.Errorf("scan = %+v %v, refreshed %v", lib, err, a.pods.refreshed)
	}
}
