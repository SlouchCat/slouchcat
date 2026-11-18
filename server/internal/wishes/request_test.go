package wishes

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Build step 2.4: creating, co-wishing, the pre-check of the limit, the season list, the outbox
// and the withdrawal (architecture 6, 19.2; DoD row 2.4 of the build plan).

// fakeLib is the library as visibility answers it: titles in the library by "media:tmdb", the
// ones a profile may not see, the playable episodes per season of a series and its catalog.
type fakeLib struct {
	mu      sync.Mutex
	items   map[string]string
	hidden  map[string]bool
	seasons map[string]map[int]int
	catalog map[string]map[int]visibility.SeasonTarget // series_catalog per series and season
	// seasonsHidden: library seasons per series the profile can play none of (HiddenSeasons)
	seasonsHidden map[string][]int
}

func newFakeLib() *fakeLib {
	return &fakeLib{items: map[string]string{}, hidden: map[string]bool{}, seasons: map[string]map[int]int{},
		catalog: map[string]map[int]visibility.SeasonTarget{}, seasonsHidden: map[string][]int{}}
}

func refKey(media string, tmdb int64) string { return fmt.Sprintf("%s:%d", media, tmdb) }

func (l *fakeLib) add(media string, tmdb int64, itemID string, seasons map[int]int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items[refKey(media, tmdb)] = itemID
	l.seasons[itemID] = seasons
}

func (l *fakeLib) MissingTitle(_ context.Context, _ visibility.Scope, right visibility.RequestRight, ref visibility.ExternalRef) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, in := l.items[refKey(ref.Media, ref.TMDBID)]; in || right == nil || !right(ref.Media) {
		return model.ErrNotFound
	}
	return nil
}

func (l *fakeLib) MatchExternalTitles(_ context.Context, _ visibility.Scope, _ visibility.RequestRight, refs []visibility.ExternalRef) ([]visibility.ExternalMatch, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]visibility.ExternalMatch, len(refs))
	for i, r := range refs {
		id, in := l.items[refKey(r.Media, r.TMDBID)]
		out[i] = visibility.ExternalMatch{Ref: r, InLibrary: in, Missing: !in}
		if in && !l.hidden[id] {
			out[i].ItemID = id
		}
	}
	return out, nil
}

func (l *fakeLib) TitleByExternalIDs(_ context.Context, ref visibility.ExternalRef) (string, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id, ok := l.items[refKey(ref.Media, ref.TMDBID)]
	return id, ok, nil
}

func (l *fakeLib) Visible(_ context.Context, _ visibility.Scope, itemID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hidden[itemID] {
		return model.ErrNotFound
	}
	return nil
}

func (l *fakeLib) VisibleSeasons(_ context.Context, _ visibility.Scope, seriesID string) (map[int]int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hidden[seriesID] {
		return nil, model.ErrNotFound
	}
	out := map[int]int{}
	for k, v := range l.seasons[seriesID] {
		out[k] = v
	}
	return out, nil
}

func (l *fakeLib) HiddenSeasons(_ context.Context, _ visibility.Scope, seriesID string) ([]int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hidden[seriesID] {
		return nil, model.ErrNotFound
	}
	return slices.Clone(l.seasonsHidden[seriesID]), nil
}

func (l *fakeLib) CatalogSeasonTarget(_ context.Context, seriesID string, season int) (visibility.SeasonTarget, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.catalog[seriesID][season]
	return t, ok, nil
}

func (l *fakeLib) Summary(_ context.Context, _ visibility.Scope, itemID string) (model.ItemSummary, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hidden[itemID] || !slices.Contains(slices.Collect(maps.Values(l.items)), itemID) {
		return model.ItemSummary{}, model.ErrNotFound
	}
	return model.ItemSummary{ID: itemID, Title: itemID}, nil
}

// fakeTitles is metadata's store of titles outside the library.
type fakeTitles map[string]metadata.ExternalTitle

func (t fakeTitles) EnsureExternalTitle(_ context.Context, media string, tmdbID int64) (metadata.ExternalTitle, error) {
	if x, ok := t[refKey(media, tmdbID)]; ok {
		return x, nil
	}
	return metadata.ExternalTitle{}, model.ErrNotFound
}

func (t fakeTitles) ExternalTitle(ctx context.Context, media string, tmdbID int64) (metadata.ExternalTitle, error) {
	return t.EnsureExternalTitle(ctx, media, tmdbID)
}

func (fakeTitles) OpenExternalTitleImage(context.Context, string, int64, string, string, bool) (model.StreamSource, error) {
	return model.StreamSource{}, model.ErrNotFound
}

// SearchExternalTitles finds the stored titles whose title in any language contains query.
func (t fakeTitles) SearchExternalTitles(_ context.Context, media, query string) ([]metadata.ExternalHit, error) {
	var out []metadata.ExternalHit
	for _, x := range t {
		if media != "" && x.Media != media {
			continue
		}
		for _, tx := range x.Texts {
			if strings.Contains(strings.ToLower(tx.Title), strings.ToLower(query)) {
				out = append(out, metadata.ExternalHit{Media: x.Media, TMDBID: x.TMDBID, Title: tx.Title, Year: x.Year})
				break
			}
		}
	}
	slices.SortFunc(out, func(a, b metadata.ExternalHit) int { return int(a.TMDBID - b.TMDBID) })
	return out, nil
}

// fakeNotifier records the calls of the Notifier.
type fakeNotifier struct {
	mu      sync.Mutex
	calls   []string
	waiting []int64 // WishAvailable answers these profiles as waiting
}

func (n *fakeNotifier) add(s string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, s)
}

func (n *fakeNotifier) got() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.calls...)
}

func (n *fakeNotifier) has(s string) bool {
	for _, c := range n.got() {
		if c == s {
			return true
		}
	}
	return false
}

func (n *fakeNotifier) WishAvailable(_ context.Context, id int64, item string) (NotifyOutcome, error) {
	n.add(fmt.Sprintf("available %d %s", id, item))
	n.mu.Lock()
	defer n.mu.Unlock()
	return NotifyOutcome{Waiting: n.waiting}, nil
}

func (n *fakeNotifier) WishSeasonAvailable(_ context.Context, s SeasonNotice) (NotifyOutcome, error) {
	n.add(fmt.Sprintf("season %d %d/%d", s.WishID, s.Season, s.Episodes))
	return NotifyOutcome{}, nil
}

func (n *fakeNotifier) WishApproved(_ context.Context, id int64) error {
	n.add(fmt.Sprintf("approved %d", id))
	return nil
}

func (n *fakeNotifier) WishDeclined(_ context.Context, id int64) error {
	n.add(fmt.Sprintf("declined %d", id))
	return nil
}

func (n *fakeNotifier) WishFailed(_ context.Context, id int64, reason string) error {
	n.add(fmt.Sprintf("failed %d %s", id, reason))
	return nil
}

func (n *fakeNotifier) WishReview(_ context.Context, id int64) error {
	n.add(fmt.Sprintf("review %d", id))
	return nil
}

// The titles of the request fixture: a film, a second film, a series with seasons 0 to 3 and a
// series only TMDB knows seasons of (Seerr fails for it).
const (
	matrix   = 603
	alien    = 348
	got      = 1399
	gotTVDB  = 121361
	dark     = 70523
	darkTVDB = 334824
)

type rfix struct {
	*fixture
	lib    *fakeLib
	titles fakeTitles
	note   *fakeNotifier
	clock  *testClock
	marco  model.Profile // account 1, sends as kinder (3)
	noah   model.Profile // account 1, second profile
	gast   model.Profile // account 2, no Seerr user: sends as the shared user (5)
}

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newRequestFixture(t *testing.T) *rfix {
	t.Helper()
	f := newFixture(t)
	f.refresh()
	if _, err := f.svc.SetAccountMapping(f.ctx, 1, ip(3), 1); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MarkShared(f.ctx, 5); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE profiles SET can_request = 1 WHERE id = 2`)
	f.cfg.set(func(c *fakeConfig) { c.wish.AllowWithdraw, c.wish.FromSearch, c.wish.FromDetail = true, true, true })
	f.queue.mu.Lock()
	f.queue.jobs = nil
	f.queue.mu.Unlock()
	r := &rfix{fixture: f, lib: newFakeLib(), note: &fakeNotifier{}, clock: &testClock{t: time.Now()},
		marco: model.Profile{ID: 1, AccountID: 1, Language: "de"}, noah: model.Profile{ID: 2, AccountID: 1, Language: "de"},
		gast: model.Profile{ID: 3, AccountID: 2, Language: "en"}}
	r.titles = fakeTitles{
		refKey("movie", matrix): {Media: "movie", TMDBID: matrix, IMDBID: "tt0133093", Year: 1999,
			Texts: map[string]metadata.ExternalText{"de": {Title: "Matrix"}, "en": {Title: "The Matrix"}}},
		refKey("movie", alien): {Media: "movie", TMDBID: alien, Year: 1979, Texts: map[string]metadata.ExternalText{"en": {Title: "Alien"}}},
		refKey("show", got): {Media: "show", TMDBID: got, TVDBID: gotTVDB, Texts: map[string]metadata.ExternalText{"de": {Title: "Game of Thrones"}},
			Seasons: []metadata.ExternalSeason{{Number: 0, EpisodeCount: 5}, {Number: 1, EpisodeCount: 10}, {Number: 2, EpisodeCount: 10}, {Number: 3, EpisodeCount: 10}}},
		refKey("show", dark): {Media: "show", TMDBID: dark, TVDBID: darkTVDB, Texts: map[string]metadata.ExternalText{"de": {Title: "Dark"}},
			Seasons: []metadata.ExternalSeason{{Number: 0, EpisodeCount: 1}, {Number: 1, EpisodeCount: 10}, {Number: 2, EpisodeCount: 8}}},
	}
	f.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: matrix, IMDBID: "tt0133093", Name: "The Matrix", Year: 1999})
	f.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: alien, Name: "Alien", Year: 1979})
	f.fake.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: got, TVDBID: gotTVDB, Name: "Game of Thrones",
		Seasons: []seerrtest.Season{{Number: 0, Episodes: 5}, {Number: 1, Episodes: 10}, {Number: 2, Episodes: 10}, {Number: 3, Episodes: 10}}})
	r.rebuild()
	return r
}

func (r *rfix) rebuild() {
	r.fixture.rebuild(WithLibrary(r.lib), WithTitles(r.titles), WithNotifier(r.note), WithClock(r.clock.now))
}

func (r *rfix) create(p model.Profile, in model.WishCreate) model.WishCreated {
	r.t.Helper()
	res, err := r.svc.CreateWish(r.ctx, p, in)
	if err != nil {
		r.t.Fatalf("create %+v: %v", in, err)
	}
	return res
}

func film(tmdb int64) model.WishCreate { return model.WishCreate{Media: "movie", TMDBID: tmdb} }

func show(tmdb int64, seasons ...int) model.WishCreate {
	return model.WishCreate{Media: "show", TMDBID: tmdb, Seasons: seasons}
}

// runJobs runs the queued jobs of typ and forgets them; a RetryLater error is returned per job.
func (r *rfix) runJobs(typ string) []error {
	r.t.Helper()
	r.queue.mu.Lock()
	var run []jobs.NewJob
	keep := r.queue.jobs[:0]
	for _, j := range r.queue.jobs {
		if j.Type == typ {
			run = append(run, j)
		} else {
			keep = append(keep, j)
		}
	}
	r.queue.jobs = keep
	r.queue.mu.Unlock()
	var out []error
	for _, j := range run {
		job := &jobs.Job{Type: j.Type, Payload: j.Payload}
		var err error
		switch typ {
		case JobSubmit:
			err = r.svc.handleSubmit(r.ctx, job)
		case JobWithdraw:
			err = r.svc.handleWithdraw(r.ctx, job)
		case JobSync:
			err = r.svc.handleSync(r.ctx, job)
		case JobReconcile:
			err = r.svc.handleReconcile(r.ctx, job)
		}
		var later *jobs.RetryLaterError
		if err != nil && !errors.As(err, &later) {
			r.t.Fatalf("%s: %v", typ, err)
		}
		out = append(out, err)
	}
	return out
}

func (r *rfix) submitJob(id int64) error {
	return r.svc.handleSubmit(r.ctx, &jobs.Job{Type: JobSubmit, Payload: []byte(fmt.Sprintf(`{"wishId":%d}`, id))})
}

func (r *rfix) wish(id int64) model.AdminWish {
	r.t.Helper()
	all, _, _, err := r.svc.AdminWishes(r.ctx, model.WishFilterAll, false, 0, 0)
	if err != nil {
		r.t.Fatal(err)
	}
	for _, w := range all {
		if w.ID == id {
			return w
		}
	}
	r.t.Fatalf("wish %d not found", id)
	return model.AdminWish{}
}

// directRequest makes a request in Seerr as a user, past Slouchcat.
func (r *rfix) directRequest(user int, req seerr.NewRequest) seerr.Request {
	r.t.Helper()
	c, err := seerr.New(r.fake.URL, seerrtest.APIKey)
	if err != nil {
		r.t.Fatal(err)
	}
	out, err := c.AsUser(user).CreateRequest(r.ctx, req)
	if err != nil {
		r.t.Fatalf("direct request: %v", err)
	}
	return out
}

func (r *rfix) setQuota(seerrID, movieUsed, tvUsed int, nextFree time.Time) {
	r.exec(`UPDATE seerr_users SET movie_quota_used = ?, tv_quota_used = ?, movie_next_free_at = ?, tv_next_free_at = ?, quota_at = 1
	        WHERE seerr_id = ?`, movieUsed, tvUsed, nextFree.UnixMilli(), nextFree.UnixMilli(), seerrID)
}

// A film goes to Seerr at once as the account's user, without pre-check: one POST, nothing read
// before it; the user needs an approval, so the wish is pending and a review is announced.
func TestCreateMovieSubmitsAtOnce(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.ResetCalls()
	res := r.create(r.marco, film(matrix))
	if !res.Created || res.Wish == nil || res.Wish.State != model.WishPending || res.Wish.Role != model.WishRoleOwner || res.Wish.Title != "Matrix" {
		t.Fatalf("created = %+v", res)
	}
	log := r.fake.CallLog()
	if len(log) == 0 || log[0].Route != "POST /request" || log[0].APIUser != "3" || r.fake.Calls("GET /movie/{id}") != 0 {
		t.Fatalf("calls = %+v", log)
	}
	reqs := r.fake.Requests()
	if len(reqs) != 1 || reqs[0].UserID != 3 || reqs[0].Status != seerrtest.RequestPending {
		t.Fatalf("seerr requests = %+v", reqs)
	}
	if w := r.wish(res.Wish.ID); w.SeerrRequestID != reqs[0].ID || w.SeerrUserID != 3 {
		t.Errorf("admin wish = %+v", w)
	}
	if !r.note.has(fmt.Sprintf("review %d", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
	if r.queue.count(JobSubmit) != 0 {
		t.Error("a sent wish was queued")
	}
	// The same profile asks again: the same wish, no second request.
	again := r.create(r.marco, film(matrix))
	if again.Created || again.Wish == nil || again.Wish.ID != res.Wish.ID || r.fake.Calls("POST /request") != 1 {
		t.Errorf("again = %+v, posts %d", again, r.fake.Calls("POST /request"))
	}
}

// A second profile joins the open wish without any call to Seerr and outside the limit.
func TestCreateCoWishWithoutSeerr(t *testing.T) {
	r := newRequestFixture(t)
	first := r.create(r.gast, film(matrix))
	r.setQuota(3, 10, 0, time.Now().Add(48*time.Hour)) // kinder's limit is used up
	r.fake.ResetCalls()
	res := r.create(r.marco, film(matrix))
	if res.Created || res.Wish == nil || res.Wish.ID != first.Wish.ID || res.Wish.Role != model.WishRoleJoined || len(res.Joined) != 1 {
		t.Fatalf("co-wish = %+v", res)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("co-wish called seerr %d times: %+v", n, r.fake.CallLog())
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE wish_id = ?`, first.Wish.ID); n != 2 {
		t.Errorf("requesters = %d", n)
	}
	// «Nur Zustand zeigen»: no co-wish.
	r.cfg.set(func(c *fakeConfig) { c.wish.AlreadyRequested = model.WishesAlreadyStatus })
	var na *model.WishNotAllowedError
	if _, err := r.svc.CreateWish(r.ctx, r.noah, film(matrix)); !errors.As(err, &na) || na.Reason != model.WishReasonStatusOnly {
		t.Errorf("status only = %v", err)
	}
}

// The limit of new wishes per profile and hour: createPerHour by default (0 keeps it), the e2e
// suite's WithCreatePerHour otherwise; the wish past it answers ErrTooManyAttempts (429).
func TestCreatePerHourOption(t *testing.T) {
	r := newRequestFixture(t)
	if r.svc.createLimit != createPerHour {
		t.Fatalf("default limit = %d, want %d", r.svc.createLimit, createPerHour)
	}
	r.fixture.rebuild(WithLibrary(r.lib), WithTitles(r.titles), WithNotifier(r.note), WithClock(r.clock.now), WithCreatePerHour(0))
	if r.svc.createLimit != createPerHour {
		t.Fatalf("limit with 0 = %d, want %d", r.svc.createLimit, createPerHour)
	}
	r.fixture.rebuild(WithLibrary(r.lib), WithTitles(r.titles), WithNotifier(r.note), WithClock(r.clock.now), WithCreatePerHour(2))
	r.create(r.marco, film(matrix))
	r.create(r.marco, film(alien))
	if _, err := r.svc.CreateWish(r.ctx, r.marco, show(got, 1)); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("third wish within the hour: %v, want ErrTooManyAttempts", err)
	}
	// Another profile has its own count.
	r.create(r.noah, show(got, 1))
}

// The hourly limit counts what a profile asked for itself, a wish or a co-wish, and nothing the
// server wrote for it (patch 1.41.1): 61 requests taken over from Seerr gave the main profile 61
// requester rows within minutes, and the limit of 20 then refused every wish for an hour.
func TestCreatePerHourCountsOwnWishesOnly(t *testing.T) {
	r := newRequestFixture(t)
	r.fixture.rebuild(WithLibrary(r.lib), WithTitles(r.titles), WithNotifier(r.note), WithClock(r.clock.now), WithCreatePerHour(2))
	now := r.clock.now().UnixMilli()
	// 25 requests taken over from Seerr just now: the take-over writes rows of the main profile
	// with the time of the take-over and history entries of its own kind.
	for i := 1; i <= 25; i++ {
		r.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, seerr_request_id, state, created_at, updated_at)
			VALUES (?, 'movie', ?, 'Taken over', 'seerr', ?, 'processing', ?, ?)`, 900+i, 9000+i, 9000+i, now, now)
		r.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (?, 1, 1, 'owner', ?)`, 900+i, now)
		r.exec(`INSERT INTO wish_events (wish_id, at, kind, detail) VALUES (?, ?, 'imported', ?)`,
			900+i, now, fmt.Sprintf(`{"requestId":%d,"profileId":1}`, 9000+i))
	}
	// The profile's own: a co-wish of the film another profile wished, then a new wish.
	r.create(r.noah, film(matrix))
	if res, err := r.svc.CreateWish(r.ctx, r.marco, film(matrix)); err != nil || res.Wish == nil || res.Wish.Role != model.WishRoleJoined {
		t.Fatalf("co-wish after the take-over: %+v, %v", res, err)
	}
	r.create(r.marco, film(alien))
	// Two of its own within the hour: the third is refused.
	if _, err := r.svc.CreateWish(r.ctx, r.marco, show(got, 1)); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("third own wish within the hour: %v, want ErrTooManyAttempts", err)
	}
	// An hour later the own ones no longer count either.
	r.clock.advance(time.Hour + time.Second)
	r.create(r.marco, show(got, 1))
}

// Two profiles create the same film at the same moment: one wish, one co-wish, one request.
func TestCreateAtTheSameMoment(t *testing.T) {
	r := newRequestFixture(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, p := range []model.Profile{r.marco, r.gast} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.svc.CreateWish(r.ctx, p, film(matrix))
		}()
	}
	wg.Wait()
	if errors.Join(errs...) != nil {
		t.Fatal(errs)
	}
	if n := r.count(`SELECT count(*) FROM wishes`); n != 1 {
		t.Errorf("wishes = %d", n)
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE role = 'owner'`); n != 1 {
		t.Errorf("owners = %d", n)
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE role = 'joined'`); n != 1 {
		t.Errorf("joined = %d", n)
	}
	if n := r.fake.Calls("POST /request"); n != 1 {
		t.Errorf("posts = %d", n)
	}
}

// The limit is used up: quota_exceeded with nextFreeAt, no wish, no call. Waiting wishes count.
func TestQuotaCheckRefuses(t *testing.T) {
	r := newRequestFixture(t)
	free := time.Now().Add(72 * time.Hour).Truncate(time.Millisecond)
	r.setQuota(3, 10, 0, free)
	r.fake.ResetCalls()
	var qe *model.WishQuotaExceededError
	if _, err := r.svc.CreateWish(r.ctx, r.marco, film(matrix)); !errors.As(err, &qe) {
		t.Fatalf("err = %v", err)
	}
	if qe.Needed != 1 || qe.Quota.Movie == nil || qe.Quota.Movie.Remaining != 0 || qe.Quota.Movie.NextFreeAt == nil || !qe.Quota.Movie.NextFreeAt.Equal(free) {
		t.Errorf("quota = %+v %+v", qe, qe.Quota.Movie)
	}
	if n := r.count(`SELECT count(*) FROM wishes`); n != 0 || len(r.fake.CallLog()) != 0 {
		t.Errorf("wishes %d, calls %d", n, len(r.fake.CallLog()))
	}
	// 9 used and one wish waiting in the outbox: full as well.
	r.setQuota(3, 9, 0, free)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	r.create(r.marco, film(alien)) // waits in the outbox
	q, err := r.svc.WishQuota(r.ctx, r.marco)
	if err != nil || q.Movie == nil || q.Movie.Queued != 1 || q.Movie.Used != 9 || q.Movie.Remaining != 0 {
		t.Fatalf("quota = %+v %v", q.Movie, err)
	}
	if _, err := r.svc.CreateWish(r.ctx, r.marco, film(matrix)); !errors.As(err, &qe) {
		t.Errorf("with a queued wish: %v", err)
	}
}

// 3 seasons with 2 free places: refused; a user with MANAGE_USERS has no pre-check.
func TestQuotaCheckSeasonsAndManageUsers(t *testing.T) {
	r := newRequestFixture(t)
	r.setQuota(3, 0, 3, time.Now().Add(time.Hour)) // 5 seasons per 7 days, 3 used
	var qe *model.WishQuotaExceededError
	if _, err := r.svc.CreateWish(r.ctx, r.marco, show(got, 1, 2, 3)); !errors.As(err, &qe) || qe.Needed != 3 || qe.Quota.TV.Remaining != 2 {
		t.Fatalf("err = %v %+v", err, qe)
	}
	if res := r.create(r.marco, show(got, 1, 2)); !res.Created {
		t.Errorf("two seasons = %+v", res)
	}
	// An administrator of Seerr: no limit, no pre-check, even with a full state.
	r.exec(`UPDATE seerr_users SET is_admin = 1 WHERE seerr_id = 3`)
	r.fake.UpdateUser(3, func(u *seerrtest.User) { u.Permissions |= seerrtest.PermManageUsers })
	r.setQuota(3, 10, 5, time.Now().Add(time.Hour))
	if res := r.create(r.marco, film(matrix)); !res.Created {
		t.Errorf("manage users = %+v", res)
	}
	if q, err := r.svc.WishQuota(r.ctx, r.marco); err != nil || !q.Unlimited || q.Reason != model.WishQuotaReasonAdmin {
		t.Errorf("quota = %+v %v", q, err)
	}
}

// Seerr refuses with 403 quota although the pre-check let it pass (someone requested in Seerr
// directly): failed, quota.
func TestSubmitQuotaRefusedBySeerr(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.UpdateUser(3, func(u *seerrtest.User) { u.MovieQuotaLimit, u.MovieQuotaDays = ip(1), ip(7) })
	r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	res := r.create(r.marco, film(matrix))
	if !res.Created || res.Wish.State != model.WishFailed || res.Wish.FailReason != model.WishFailQuota {
		t.Fatalf("wish = %+v", res.Wish)
	}
	if !r.note.has(fmt.Sprintf("failed %d quota", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
	if n := r.count(`SELECT count(*) FROM seerr_users WHERE seerr_id = 3 AND quota_at IS NOT NULL`); n != 1 {
		t.Error("quota state not read after the refusal")
	}
}

// The dialog lists the seasons of GET /tv/{tmdbId} without season 0, cached for an hour; without
// Seerr the seasons of TMDB (tmdb_fallback).
func TestSeasonsFromSeerrAndFallback(t *testing.T) {
	r := newRequestFixture(t)
	got1, err := r.svc.WishSeasons(r.ctx, r.marco, got)
	if err != nil || got1.Source != model.WishSeasonsFromSeerr || len(got1.Seasons) != 3 || got1.Seasons[0].Number != 1 || got1.Seasons[0].EpisodeCount != 10 {
		t.Fatalf("seasons = %+v %v", got1, err)
	}
	if _, err := r.svc.WishSeasons(r.ctx, r.marco, got); err != nil || r.fake.Calls("GET /tv/{id}") != 1 {
		t.Errorf("cached: %v, calls %d", err, r.fake.Calls("GET /tv/{id}"))
	}
	// Dark is unknown to the fake Seerr: TMDB's seasons.
	fb, err := r.svc.WishSeasons(r.ctx, r.marco, dark)
	if err != nil || fb.Source != model.WishSeasonsTMDBFallback || len(fb.Seasons) != 2 || fb.Seasons[1].EpisodeCount != 8 {
		t.Fatalf("fallback = %+v %v", fb, err)
	}
	// A season another wish holds is «requested»; a season the profile plays is «available».
	r.create(r.gast2(), show(got, 1))
	s2, err := r.svc.WishSeasons(r.ctx, r.marco, got)
	if err != nil || s2.Seasons[0].State != model.WishSeasonOfferRequested || s2.Seasons[1].State != model.WishSeasonOfferMissing {
		t.Errorf("states = %+v %v", s2, err)
	}
	// A series of the library with season 2 playable: offered with its state.
	r.lib.add("show", got, "series-got", map[int]int{2: 10})
	s3, err := r.svc.WishSeasons(r.ctx, r.marco, got)
	if err != nil || s3.Seasons[1].State != model.WishSeasonOfferAvailable || s3.Seasons[2].State != model.WishSeasonOfferMissing {
		t.Errorf("library series = %+v %v", s3, err)
	}
}

// gast2 is a profile of account 2 with the right for series: account 2 sends as user 4 here.
func (r *rfix) gast2() model.Profile {
	r.t.Helper()
	if _, err := r.svc.SetAccountMapping(r.ctx, 2, ip(4), 1); err != nil {
		r.t.Fatal(err)
	}
	return r.gast
}

// The immediate attempt times out: the wish waits as queued and goes to wishes.submit.
func TestSubmitTimeoutQueued(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Hang: true})
	res := r.create(r.marco, film(matrix))
	if res.Wish.State != model.WishQueued {
		t.Fatalf("state = %s", res.Wish.State)
	}
	if w := r.wish(res.Wish.ID); w.Attempts != 1 {
		t.Errorf("attempts = %d", w.Attempts)
	}
	if r.queue.count(JobSubmit) != 1 {
		t.Errorf("submit jobs = %d", r.queue.count(JobSubmit))
	}
	// The next attempt (Seerr back) pre-checks, finds nothing and sends.
	r.fake.SetFailure("POST /request", seerrtest.Failure{})
	r.fake.ResetCalls()
	r.runJobs(JobSubmit)
	if w := r.wish(res.Wish.ID); w.State != model.WishPending || r.fake.Calls("GET /movie/{id}") != 1 || r.fake.Calls("POST /request") != 1 {
		t.Errorf("after the job: %s, calls %+v", w.State, r.fake.CallLog())
	}
}

// The first POST reached Seerr, its answer was lost, the server restarts: the next attempt finds
// the request with the pre-check and sends no second POST.
func TestSubmitLostAnswerNoSecondPost(t *testing.T) {
	r := newRequestFixture(t)
	arrived := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	r.fake.SetFailure("POST /request", seerrtest.Failure{Hang: true})
	res := r.create(r.marco, film(matrix))
	if res.Wish.State != model.WishQueued {
		t.Fatalf("state = %s", res.Wish.State)
	}
	r.rebuild() // restart
	posts := r.fake.Calls("POST /request")
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if n := r.fake.Calls("POST /request"); n != posts {
		t.Errorf("second post: %d -> %d", posts, n)
	}
	if w := r.wish(res.Wish.ID); w.SeerrRequestID != arrived.ID || w.State != model.WishPending {
		t.Errorf("wish = %+v", w)
	}
	if n := len(r.fake.Requests()); n != 1 {
		t.Errorf("requests in seerr = %d", n)
	}
}

// 409 and 202 attach to the request that exists; a series whose season 1 someone else requested
// carries two request ids.
func TestSubmitDuplicateAndForeignSeasons(t *testing.T) {
	r := newRequestFixture(t)
	foreign := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	res := r.create(r.marco, film(matrix))
	if w := r.wish(res.Wish.ID); w.SeerrRequestID != foreign.ID || w.State != model.WishProcessing {
		t.Errorf("409: %+v", w)
	}
	s1 := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{1}})
	sr := r.create(r.marco, show(got, 1, 2))
	w := r.wish(sr.Wish.ID)
	if len(w.Seasons) != 2 || w.Seasons[0].SeerrRequestID != s1.ID || w.Seasons[1].SeerrRequestID == 0 || w.Seasons[1].SeerrRequestID == s1.ID {
		t.Fatalf("seasons = %+v", w.Seasons)
	}
	if w.State != model.WishPending { // season 2 waits for its approval, season 1 is approved
		t.Errorf("state = %s", w.State)
	}
	// 202: every season is in Seerr already.
	r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{3}})
	r.exec(`UPDATE wish_seasons SET state = 'failed' WHERE wish_id = ?`, sr.Wish.ID) // free the seasons for a new wish
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'quota' WHERE id = ?`, sr.Wish.ID)
	all := r.create(r.noah, show(got, 3))
	if aw := r.wish(all.Wish.ID); aw.State != model.WishProcessing || aw.Seasons[0].SeerrRequestID == 0 {
		t.Errorf("202: %+v", aw)
	}
}

// 403 for an unknown user: seerr_user_gone; for a wrong key: the wish stays queued.
func TestSubmitUserGoneOrKeyInvalid(t *testing.T) {
	r := newRequestFixture(t)
	r.cfg.set(func(c *fakeConfig) { c.seer.APIKey = "wrong-key-of-the-test" })
	res := r.create(r.marco, film(matrix))
	if res.Wish.State != model.WishQueued {
		t.Fatalf("wrong key: %s", res.Wish.State)
	}
	if n := r.count(`SELECT count(*) FROM settings WHERE key = 'seerr.last_error' AND value = 'key_invalid'`); n != 1 {
		t.Error("key state not recorded")
	}
	r.cfg.set(func(c *fakeConfig) { c.seer.APIKey = seerrtest.APIKey })
	r.fake.RemoveUser(3)
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishFailed || w.FailReason != model.WishFailSeerrUserGone {
		t.Errorf("user gone: %+v", w.Wish)
	}
}

// 24 h without Seerr: failed with an event; the time Seerr is switched off does not count (T-2).
func TestSubmitDeadline(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	res := r.create(r.marco, film(matrix))
	r.clock.advance(time.Hour)
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	if err := r.submitJob(res.Wish.ID); err != nil { // rests
		t.Fatal(err)
	}
	r.clock.advance(20 * time.Hour)
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = true })
	var later *jobs.RetryLaterError
	if err := r.submitJob(res.Wish.ID); !errors.As(err, &later) { // the first attempt after the rest ends it
		t.Fatalf("seerr back: %v", err)
	}
	r.clock.advance(4 * time.Hour) // 25 h since creation, 20 h of them at rest
	if err := r.submitJob(res.Wish.ID); !errors.As(err, &later) {
		t.Fatalf("after the rest: %v", err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishQueued {
		t.Fatalf("failed during the rest: %+v", w.Wish)
	}
	r.clock.advance(20 * time.Hour)
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	w := r.wish(res.Wish.ID)
	if w.State != model.WishFailed || w.FailReason != model.WishFailUnreachable24h || w.Events[0].Kind != "state_changed" {
		t.Errorf("deadline: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("failed %d unreachable_24h", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
}

// The immediate attempt timed out, but the request arrived; the profile withdraws: the job finds
// the own pending request in Seerr and deletes it.
func TestWithdrawAfterTimeoutLeavesNothing(t *testing.T) {
	r := newRequestFixture(t)
	r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix}) // the lost first POST
	r.fake.SetFailure("POST /request", seerrtest.Failure{Hang: true, Times: 1})
	res := r.create(r.marco, film(matrix))
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishWithdrawn {
		t.Fatalf("state = %s", w.State)
	}
	r.runJobs(JobWithdraw)
	if reqs := r.fake.Requests(); len(reqs) != 0 {
		t.Errorf("requests left in seerr: %+v", reqs)
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE withdraw_pending = 1`); n != 0 {
		t.Error("mark not cleared")
	}
	// A wish that never left: withdrawn without a call.
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503})
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	r.exec(`INSERT INTO wishes (media, tmdb_id, title, origin, seerr_user_id, state, created_at, updated_at)
	        VALUES ('movie', 348, 'Alien', 'slouchcat', 3, 'queued', 1, 1)`)
	r.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (last_insert_rowid(), 1, 1, 'owner', 1)`)
	id := int64(r.count(`SELECT id FROM wishes WHERE tmdb_id = 348`))
	if err := r.svc.WithdrawWish(r.ctx, r.marco, id); err != nil || r.queue.count(JobWithdraw) != 0 {
		t.Errorf("never sent: %v, jobs %d", err, r.queue.count(JobWithdraw))
	}
}

// The request was approved before the withdrawal reached Seerr: it stays, the wish opens again
// as processing with the hint.
func TestWithdrawAlreadyApproved(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	r.fake.SetRequestStatus(r.fake.Requests()[0].ID, seerrtest.RequestApproved)
	r.runJobs(JobWithdraw)
	if n := len(r.fake.Requests()); n != 1 {
		t.Errorf("requests = %d", n)
	}
	page, err := r.svc.ProfileWishes(r.ctx, r.marco, model.WishFilterOpen, 0, 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page = %+v %v", page, err)
	}
	if w := page.Items[0]; w.State != model.WishProcessing || w.Hint != model.WishHintAlreadyApproved || w.CanWithdraw {
		t.Errorf("reopened = %+v", w)
	}
	// Past the approval the owner cannot withdraw any more.
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); !errors.Is(err, model.ErrWishNotWithdrawable) {
		t.Errorf("processing: %v", err)
	}
}

// A co-wisher withdraws itself; an owner with co-wishers hands the wish over; the switch off
// refuses; a foreign wish is not found.
func TestWithdrawJoinedAndHandOver(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	r.create(r.noah, film(matrix))
	r.create(r.gast, film(matrix))
	if err := r.svc.WithdrawWish(r.ctx, r.noah, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE wish_id = ? AND profile_id = 3 AND role = 'owner'`, res.Wish.ID); n != 1 {
		t.Error("owner not handed over")
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPending || len(r.fake.Requests()) != 1 {
		t.Errorf("wish = %s", w.State)
	}
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("gone requester: %v", err)
	}
	r.cfg.set(func(c *fakeConfig) { c.wish.AllowWithdraw = false })
	var na *model.WishNotAllowedError
	if err := r.svc.WithdrawWish(r.ctx, r.gast, res.Wish.ID); !errors.As(err, &na) || na.Reason != model.WishReasonWithdrawDisabled {
		t.Errorf("switch off: %v", err)
	}
}

// What createWish refuses before anything is stored.
func TestCreateRefusals(t *testing.T) {
	r := newRequestFixture(t)
	oma := model.Profile{ID: 4, AccountID: 3}
	var na *model.WishNotAllowedError
	if _, err := r.svc.CreateWish(r.ctx, oma, film(matrix)); !errors.As(err, &na) || na.Reason != model.WishReasonAccountNotAllowed {
		t.Errorf("no right: %v", err)
	}
	if _, err := r.svc.CreateWish(r.ctx, r.gast, show(got, 1)); !errors.As(err, &na) || na.Reason != model.WishReasonShowsNotAllowed {
		t.Errorf("shared user without series: %v", err)
	}
	if _, err := r.svc.CreateWish(r.ctx, r.marco, film(999)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}
	r.lib.add("movie", matrix, "item-matrix", nil)
	var aa *model.WishAlreadyAvailableError
	if _, err := r.svc.CreateWish(r.ctx, r.marco, film(matrix)); !errors.As(err, &aa) || aa.ItemID != "item-matrix" {
		t.Errorf("in the library: %v", err)
	}
	r.lib.hidden["item-matrix"] = true
	if _, err := r.svc.CreateWish(r.ctx, r.marco, film(matrix)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("in the library, not visible: %v", err)
	}
	var verr *model.ValidationError
	for _, in := range []model.WishCreate{{Media: "book", TMDBID: 1}, {Media: "movie", TMDBID: matrix, Seasons: []int{1}},
		show(got, 0), show(got, 9), {Media: "show", TMDBID: got}} {
		if _, err := r.svc.CreateWish(r.ctx, r.marco, in); !errors.As(err, &verr) {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if n := r.count(`SELECT count(*) FROM wishes`); n != 0 {
		t.Errorf("wishes = %d", n)
	}
}

// A library season the profile can play none of (rated above it) is neither offered nor
// requestable: named, it answers season_not_offered like a season that does not exist; «all»
// leaves it out (review of 18 Nov 2026, open question «hidden season reads as missing»).
func TestHiddenLibrarySeasonNotOffered(t *testing.T) {
	r := newRequestFixture(t)
	r.lib.add("show", got, "series-got", map[int]int{1: 10})
	r.lib.seasonsHidden["series-got"] = []int{2}
	ws, err := r.svc.WishSeasons(r.ctx, r.marco, got)
	if err != nil {
		t.Fatal(err)
	}
	var numbers []int
	for _, se := range ws.Seasons {
		numbers = append(numbers, se.Number)
	}
	if !slices.Equal(numbers, []int{1, 3}) {
		t.Errorf("offered = %v, want [1 3]", numbers)
	}
	for _, n := range []int{2, 4} { // the hidden season and one that does not exist answer alike
		_, err := r.svc.CreateWish(r.ctx, r.marco, show(got, n))
		var verr *model.ValidationError
		if !errors.As(err, &verr) || !strings.Contains(fmt.Sprint(verr.Fields), model.WishCodeSeasonNotOffered) {
			t.Errorf("season %d: %v, want season_not_offered", n, err)
		}
	}
	res, err := r.svc.CreateWish(r.ctx, r.marco, model.WishCreate{Media: model.WishMediaShow, TMDBID: got, AllSeasons: true})
	if err != nil {
		t.Fatal(err)
	}
	var wished []int
	for _, se := range r.wish(res.Wish.ID).Seasons {
		wished = append(wished, se.Number)
	}
	if !slices.Equal(wished, []int{3}) {
		t.Errorf("all seasons = %v, want [3]", wished)
	}
}
