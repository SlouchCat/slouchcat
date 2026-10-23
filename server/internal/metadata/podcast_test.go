package metadata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/feed"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// fakeFeed is a podcast feed server: n episodes (episode i published on day i), episode explicit
// flags, a channel explicit flag, an ETag that changes with the content, and a failure switch.
type fakeFeed struct {
	srv *httptest.Server

	mu         sync.Mutex
	n          int
	explicit   map[int]bool
	channelX   bool
	title      map[int]string
	status     int // != 0: answer with this status
	body       string
	requests   int
	conditions []string // If-None-Match of every request
}

func newFakeFeed(t *testing.T, n int) *fakeFeed {
	t.Helper()
	f := &fakeFeed{n: n, explicit: map[int]bool{}, title: map[int]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeFeed) set(fn func(f *fakeFeed)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeFeed) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	f.conditions = append(f.conditions, r.Header.Get("If-None-Match"))
	if r.Header.Get("User-Agent") != "Slouchcat/test" {
		http.Error(w, "user agent", http.StatusBadRequest)
		return
	}
	if f.status != 0 {
		http.Error(w, "down", f.status)
		return
	}
	body := f.body
	if body == "" {
		body = f.xml()
	}
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(body)))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/rss+xml")
	_, _ = w.Write([]byte(body))
}

func (f *fakeFeed) xml() string {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0"?><rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel>
<title>Test Podcast</title><description>&lt;p&gt;Ein Test.&lt;/p&gt;</description><itunes:author>Autorin</itunes:author>
<itunes:explicit>%t</itunes:explicit><itunes:image href="%s/cover.jpg"/>`, f.channelX, f.srv.URL)
	for i := f.n; i >= 1; i-- {
		title := f.title[i]
		if title == "" {
			title = fmt.Sprintf("Episode %d", i)
		}
		day := time.Date(2025, 1, 1, 6, 0, 0, 0, time.UTC).Add(time.Duration(i) * 24 * time.Hour)
		fmt.Fprintf(&b, `<item><title>%s</title><guid>ep-%d</guid><pubDate>%s</pubDate><itunes:duration>0:01:0%d</itunes:duration>
<itunes:explicit>%t</itunes:explicit><description>Folge %d</description><enclosure url="%s/episode-%d.mp3" length="%d" type="audio/mpeg"/></item>`,
			title, i, day.Format(time.RFC1123Z), i%10, f.explicit[i], i, f.srv.URL, i, 1000+i)
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

type podcastEnv struct {
	db    *store.DB
	svc   *Service
	queue *jobs.Queue
	feed  *fakeFeed
	clock *testClock
}

func newPodcastEnv(t *testing.T, episodes int) *podcastEnv {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := &testClock{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	q := jobs.NewQueue(db.Write, nil)
	f := newFakeFeed(t, episodes)
	// No WithFeedHTTPClient: the feed lives on the loopback address, which the address guard
	// (package netguard) allows for a feed whose own host is local.
	svc := New(db, q, staticKey(""), t.TempDir(), transcode.Runner{}, log,
		WithFeedClock(clock.now), WithFeedUserAgent("Slouchcat/test"))
	mustExec(t, db.Write, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (1, 'Filme', 'movies', NULL, 0, 0)`)
	mustExec(t, db.Write, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (3, 'Podcasts', 'podcasts', 6, 0, 0)`)
	return &podcastEnv{db: db, svc: svc, queue: q, feed: f, clock: clock}
}

type episodeRow struct {
	ID, Title, GUID, Source, Enclosure string
	Rating, Published, Duration        sql.NullInt64
	Effective                          sql.NullInt64
	Explicit, Available                int
}

func (e *podcastEnv) episodes(t *testing.T, podcastID string) map[string]episodeRow {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT id, title, guid, COALESCE(rating_source, ''), COALESCE(enclosure_url, ''), own_rating,
	       published_at, duration_ms, effective_rating, explicit, available FROM items WHERE parent_id = ? AND kind = 'podcast_episode'`, podcastID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]episodeRow{}
	for rows.Next() {
		var r episodeRow
		if err := rows.Scan(&r.ID, &r.Title, &r.GUID, &r.Source, &r.Enclosure, &r.Rating, &r.Published, &r.Duration,
			&r.Effective, &r.Explicit, &r.Available); err != nil {
			t.Fatal(err)
		}
		out[r.GUID] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *podcastEnv) podcast(t *testing.T, id string) (rating sql.NullInt64, source, feedErr sql.NullString, checked sql.NullInt64, state string) {
	t.Helper()
	err := e.db.Read.QueryRow(`SELECT own_rating, rating_source, feed_error, feed_checked_at, metadata_state FROM items WHERE id = ?`, id).
		Scan(&rating, &source, &feedErr, &checked, &state)
	if err != nil {
		t.Fatal(err)
	}
	return rating, source, feedErr, checked, state
}

func (e *podcastEnv) jobs(t *testing.T, typ string) []string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT idempotency_key FROM jobs WHERE type = ? AND state = 'queued' ORDER BY id`, typ)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSubscribePodcast(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Test Podcast" || p.Author != "Autorin" || p.EpisodeCount != 2 || p.LibraryID != 3 || p.Explicit ||
		p.LastCheckedAt == nil || !p.LastCheckedAt.Equal(e.clock.now()) || p.LastError != "" || p.FeedURL != e.feed.srv.URL+"/feed.xml" {
		t.Fatalf("subscribed = %+v", p)
	}
	rating, source, _, _, state := e.podcast(t, p.ID)
	if rating.Int64 != 6 || source.String != "library" || state != "matched" {
		t.Errorf("podcast rating %v %v, state %s", rating, source, state)
	}
	var cover, overview string
	if err := e.db.Read.QueryRow(`SELECT i.cover_url, t.overview FROM items AS i JOIN item_texts AS t ON t.item_id = i.id AND t.lang = 'en' WHERE i.id = ?`, p.ID).
		Scan(&cover, &overview); err != nil || cover != e.feed.srv.URL+"/cover.jpg" || overview != "Ein Test." {
		t.Errorf("cover %q, overview %q, err %v", cover, overview, err)
	}
	eps := e.episodes(t, p.ID)
	ep1 := eps["ep-1"]
	if len(eps) != 2 || ep1.Title != "Episode 1" || ep1.Enclosure != e.feed.srv.URL+"/episode-1.mp3" || ep1.Duration.Int64 != 61_000 ||
		ep1.Published.Int64 != time.Date(2025, 1, 2, 6, 0, 0, 0, time.UTC).UnixMilli() || ep1.Rating.Valid || ep1.Effective.Int64 != 6 || ep1.Available != 1 {
		t.Errorf("episodes = %+v", eps)
	}
	if got := e.jobs(t, JobImageFetch); len(got) != 1 || got[0] != "image:"+p.ID {
		t.Errorf("image jobs = %v", got)
	}
	// The same feed again in the library: feed_exists.
	if _, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml"); !errors.Is(err, model.ErrFeedExists) {
		t.Errorf("second subscription: %v", err)
	}
	list, err := e.svc.ListPodcasts(ctx)
	if err != nil || len(list) != 1 || list[0].ID != p.ID {
		t.Errorf("list = %+v %v", list, err)
	}
}

func TestSubscribePodcastErrors(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 1)
	ctx := context.Background()
	for _, tc := range []struct {
		library int64
		url     string
		field   string
		code    string
	}{
		{3, "ftp://example.test/feed.xml", "feedUrl", "invalid_url"},
		{3, "https://user:secret@example.test/feed.xml", "feedUrl", "invalid_url"},
		{3, "/feed.xml", "feedUrl", "invalid_url"},
		{3, "https://example.test/" + strings.Repeat("a", 2100), "feedUrl", "invalid_url"},
		{99, e.feed.srv.URL + "/feed.xml", "libraryId", "unknown_library"},
		{1, e.feed.srv.URL + "/feed.xml", "libraryId", "library_kind"},
	} {
		_, err := e.svc.Subscribe(ctx, tc.library, tc.url)
		var verr *model.ValidationError
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != tc.field || verr.Fields[0].Code != tc.code {
			t.Errorf("%d %q: %v, want %s %s", tc.library, tc.url, err, tc.field, tc.code)
		}
	}
	e.feed.set(func(f *fakeFeed) { f.status = http.StatusNotFound })
	if _, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml"); !errors.Is(err, model.ErrFeedUnreachable) {
		t.Errorf("404: %v", err)
	}
	e.feed.set(func(f *fakeFeed) {
		f.status, f.body = 0, `<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title></feed>`
	})
	if _, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml"); !errors.Is(err, model.ErrFeedInvalid) {
		t.Errorf("atom: %v", err)
	}
	e.feed.set(func(f *fakeFeed) { f.body = `<rss><channel><title>broken` })
	if _, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml"); !errors.Is(err, model.ErrFeedInvalid) {
		t.Errorf("broken XML: %v", err)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	if _, err := e.svc.Subscribe(ctx, 3, closed.URL+"/feed.xml"); !errors.Is(err, model.ErrFeedUnreachable) {
		t.Errorf("closed server: %v", err)
	}
	if list, _ := e.svc.ListPodcasts(ctx); len(list) != 0 {
		t.Errorf("failed subscriptions stored: %+v", list)
	}
}

// H-2: a new episode appears with the next fetch; 304 keeps the state; explicit → 18; a failure is
// recorded and cleared by the next success; a title change updates the episode by guid.
func TestFetchPodcast(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	// Unchanged feed: the server answers 304 to the stored ETag; only the check time moves.
	e.clock.set(e.clock.now().Add(time.Hour))
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	_, _, _, checked, _ := e.podcast(t, p.ID)
	e.feed.mu.Lock()
	cond := e.feed.conditions[len(e.feed.conditions)-1]
	e.feed.mu.Unlock()
	if cond == "" || checked.Int64 != e.clock.now().UnixMilli() || len(e.episodes(t, p.ID)) != 2 {
		t.Errorf("304: condition %q, checked %v", cond, checked)
	}
	// Episode 3 is published and explicit; episode 1 gets a new title.
	e.feed.set(func(f *fakeFeed) { f.n, f.explicit[3], f.title[1] = 3, true, "Episode 1 (neu)" })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	eps := e.episodes(t, p.ID)
	if len(eps) != 3 || eps["ep-3"].Rating.Int64 != 18 || eps["ep-3"].Source != "feed" || eps["ep-3"].Explicit != 1 ||
		eps["ep-3"].Effective.Int64 != 18 || eps["ep-1"].Title != "Episode 1 (neu)" || eps["ep-2"].Effective.Int64 != 6 {
		t.Errorf("after the update: %+v", eps)
	}
	// A failure is recorded and returned (the job retries); the next success clears it.
	e.feed.set(func(f *fakeFeed) { f.status = http.StatusInternalServerError })
	if err := e.svc.FetchPodcast(ctx, p.ID); err == nil {
		t.Error("failed fetch returned no error")
	}
	_, _, feedErr, _, _ := e.podcast(t, p.ID)
	if !strings.HasPrefix(feedErr.String, "feed_unreachable: feed answered HTTP 500") {
		t.Errorf("feed_error = %q", feedErr.String)
	}
	list, _ := e.svc.ListPodcasts(ctx)
	if len(list) != 1 || list[0].LastError != feedErr.String || list[0].EpisodeCount != 3 {
		t.Errorf("list after failure = %+v", list)
	}
	// The channel turns explicit: the podcast is 18 (feed), the episodes follow.
	e.feed.set(func(f *fakeFeed) { f.status, f.channelX, f.explicit[3] = 0, true, false })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	rating, source, feedErr, _, _ := e.podcast(t, p.ID)
	eps = e.episodes(t, p.ID)
	if rating.Int64 != 18 || source.String != "feed" || feedErr.Valid || eps["ep-1"].Effective.Int64 != 18 || eps["ep-3"].Rating.Valid {
		t.Errorf("explicit channel: %v %v %v, episodes %+v", rating, source, feedErr, eps)
	}
	// A manual rating of an episode stays.
	mustExec(t, e.db.Write, `UPDATE items SET own_rating = 16, rating_source = 'manual' WHERE id = ?`, eps["ep-2"].ID)
	e.feed.set(func(f *fakeFeed) { f.channelX, f.explicit[2] = false, true })
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if ep := e.episodes(t, p.ID)["ep-2"]; ep.Rating.Int64 != 16 || ep.Source != "manual" {
		t.Errorf("manual episode rating = %+v", ep)
	}
	// Unsubscribed: the fetch does nothing.
	if err := e.svc.Unsubscribe(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Errorf("fetch of a deleted podcast: %v", err)
	}
}

// At most the newest episodes of a feed are stored, undated ones last.
func TestNewestEpisodes(t *testing.T) {
	t.Parallel()
	day := func(d int) time.Time { return time.Date(2025, 1, d, 0, 0, 0, 0, time.UTC) }
	eps := []feed.Episode{{GUID: "a", PublishedAt: day(2)}, {GUID: "undated"}, {GUID: "b", PublishedAt: day(5)}, {GUID: "c", PublishedAt: day(3)}}
	var got []string
	for _, e := range newestEpisodes(eps, 3) {
		got = append(got, e.GUID)
	}
	if strings.Join(got, ",") != "b,c,a" {
		t.Errorf("newest = %v", got)
	}
	if n := len(newestEpisodes(eps, maxFeedEpisodes)); n != 4 || maxFeedEpisodes != 2000 {
		t.Errorf("all = %d, cap %d", n, maxFeedEpisodes)
	}
}

// The poller queues a subscription once its last fetch is FeedInterval old (H-2: < 65 min with the
// 5-minute tick and the wait of the job).
func TestQueueDuePodcasts(t *testing.T) {
	t.Parallel()
	if FeedInterval+feedPollInterval > 60*time.Minute {
		t.Errorf("FeedInterval %v plus the poller tick %v leaves no reserve below 65 minutes", FeedInterval, feedPollInterval)
	}
	e := newPodcastEnv(t, 1)
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		advance time.Duration
		want    int
	}{{0, 0}, {FeedInterval - time.Minute, 0}, {time.Minute, 1}} {
		e.clock.set(e.clock.now().Add(step.advance))
		n, err := e.svc.QueueDuePodcasts(ctx)
		if err != nil || n != step.want {
			t.Fatalf("after %v: queued %d, %v; want %d", step.advance, n, err, step.want)
		}
	}
	if got := e.jobs(t, JobPodcastFetch); len(got) != 1 || got[0] != "podcast:"+p.ID {
		t.Errorf("fetch jobs = %v", got)
	}
	// The job runs through the worker handler.
	j, err := e.queue.Claim(ctx, "w", []string{JobPodcastFetch})
	if err != nil || j == nil {
		t.Fatalf("claim %v %v", j, err)
	}
	if err := e.svc.handlePodcastFetch(ctx, j); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.svc.QueueDuePodcasts(ctx); n != 0 {
		t.Errorf("fetched podcast still due: %d", n)
	}
	// Refresh queues at once; unknown ids are not found.
	if _, err := e.svc.RefreshPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RefreshPodcast(ctx, "01J0000000000000000000XXXX"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("refresh unknown: %v", err)
	}
	if n, err := e.svc.RefreshLibrary(ctx, 3); err != nil || n != 1 {
		t.Errorf("refresh library: %d %v", n, err)
	}
	if err := e.svc.Unsubscribe(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Unsubscribe(ctx, p.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second unsubscribe: %v", err)
	}
	if list, _ := e.svc.ListPodcasts(ctx); len(list) != 0 {
		t.Errorf("list after unsubscribe = %+v", list)
	}
	var live int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM items WHERE deleted_at IS NULL AND kind IN ('podcast', 'podcast_episode')`).Scan(&live); err != nil || live != 0 {
		t.Errorf("live podcast items after unsubscribe: %d %v", live, err)
	}
}

// M6 review m-1, m-6, m-11: locked fields of the channel and of an episode stay; an episode that
// turns explicit ends open streams; an episode keeps an image only when it differs from the
// channel's, and a new episode with its own image is queued for image.fetch.
func TestFetchPodcastLocksAndEpisodeCovers(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 2)
	rechecks := 0
	e.svc.recheck = func(context.Context) error { rechecks++; return nil }
	ctx := context.Background()
	p, err := e.svc.Subscribe(ctx, 3, e.feed.srv.URL+"/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	ep1 := e.episodes(t, p.ID)["ep-1"].ID
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["author","overview"]' WHERE id = ?`, p.ID)
	mustExec(t, e.db.Write, `UPDATE items SET locked_fields = '["title","poster","overview"]', cover_url = 'https://img.test/kept.jpg' WHERE id = ?`, ep1)
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	e.feed.set(func(f *fakeFeed) {
		f.n, f.title[1], f.explicit[2] = 3, "Episode 1 (neu)", true
		body := strings.NewReplacer(
			"<itunes:author>Autorin</itunes:author>", "<itunes:author>Neue Autorin</itunes:author>",
			"<description>&lt;p&gt;Ein Test.&lt;/p&gt;</description>", "<description>Neu.</description>",
			"<description>Folge 1</description>", "<description>Folge eins, neu</description>",
			"<guid>ep-1</guid>", `<guid>ep-1</guid><itunes:image href="https://img.test/ep1-new.jpg"/>`,
			"<guid>ep-2</guid>", `<guid>ep-2</guid><itunes:image href="`+f.srv.URL+`/cover.jpg"/>`,
			"<guid>ep-3</guid>", `<guid>ep-3</guid><itunes:image href="https://img.test/ep3.jpg"/>`,
		).Replace(f.xml())
		f.body = body
	})
	if err := e.svc.FetchPodcast(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	var author, overview string
	if err := e.db.Read.QueryRow(`SELECT i.author, t.overview FROM items AS i JOIN item_texts AS t ON t.item_id = i.id AND t.lang = 'de' WHERE i.id = ?`, p.ID).
		Scan(&author, &overview); err != nil || author != "Autorin" || overview != "Ein Test." {
		t.Errorf("locked channel: author %q, overview %q, %v", author, overview, err)
	}
	type row struct{ title, text, overview, cover string }
	get := func(id string) row {
		var r row
		if err := e.db.Read.QueryRow(`SELECT i.title, t.title, COALESCE(t.overview, ''), COALESCE(i.cover_url, '') FROM items AS i
		    JOIN item_texts AS t ON t.item_id = i.id AND t.lang = 'en' WHERE i.id = ?`, id).Scan(&r.title, &r.text, &r.overview, &r.cover); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get(ep1); r != (row{"Episode 1", "Episode 1", "Folge 1", "https://img.test/kept.jpg"}) {
		t.Errorf("locked episode = %+v", r)
	}
	eps := e.episodes(t, p.ID)
	if r := get(eps["ep-2"].ID); r.cover != "" || eps["ep-2"].Rating.Int64 != 18 {
		t.Errorf("episode with the channel's image = %+v, rating %v", r, eps["ep-2"].Rating)
	}
	if r := get(eps["ep-3"].ID); r.cover != "https://img.test/ep3.jpg" {
		t.Errorf("new episode with its own image = %+v", r)
	}
	if got := e.jobs(t, JobImageFetch); len(got) != 1 || got[0] != "image:"+eps["ep-3"].ID {
		t.Errorf("image jobs = %v", got)
	}
	if rechecks != 1 {
		t.Errorf("open streams rechecked %d times after an episode turned explicit, want 1", rechecks)
	}

	// M6 review M-2: the credentials of a private feed never reach feed_error or the error.
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	mustExec(t, e.db.Write, `UPDATE items SET feed_url = ? WHERE id = ?`, closed.URL+"/private/s3cr3t-path.xml?auth=s3cr3t-token", p.ID)
	err = e.svc.FetchPodcast(ctx, p.ID)
	_, _, feedErr, _, _ := e.podcast(t, p.ID)
	if err == nil || strings.Contains(err.Error(), "s3cr3t") || strings.Contains(feedErr.String, "s3cr3t") ||
		!strings.Contains(feedErr.String, closed.URL) {
		t.Errorf("fetch error %v, feed_error %q", err, feedErr.String)
	}
}

// M6 review m-2: two subscriptions of one feed at once store one podcast (unique index
// items_podcast_feed); the other request answers feed_exists.
func TestSubscribeConcurrently(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 1)
	body := e.feed.xml()
	var arrived sync.WaitGroup
	arrived.Add(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		arrived.Done()
		arrived.Wait() // both requests passed the existence check before either stores
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := e.svc.Subscribe(context.Background(), 3, srv.URL+"/feed.xml")
			errs <- err
		}()
	}
	var ok, exists int
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			ok++
		case errors.Is(err, model.ErrFeedExists):
			exists++
		default:
			t.Errorf("subscribe: %v", err)
		}
	}
	if list, _ := e.svc.ListPodcasts(context.Background()); ok != 1 || exists != 1 || len(list) != 1 {
		t.Errorf("%d subscribed, %d feed_exists, %d stored", ok, exists, len(list))
	}
}

// M6 review m-12: at most 5 redirects, only to http and https, and a feed of at most 16 MiB.
func TestFeedFetchLimits(t *testing.T) {
	t.Parallel()
	e := newPodcastEnv(t, 1)
	body := e.feed.xml()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/r/") && r.URL.Path != "/r/0":
			n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/r/"))
			http.Redirect(w, r, "/r/"+strconv.Itoa(n-1), http.StatusFound)
		case r.URL.Path == "/ftp":
			http.Redirect(w, r, "ftp://example.test/feed.xml", http.StatusFound)
		case r.URL.Path == "/big":
			_, _ = w.Write([]byte(body[:200]))
			_, _ = w.Write(make([]byte, maxFeedBytes))
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	if _, err := e.svc.Subscribe(ctx, 3, srv.URL+"/r/5"); err != nil {
		t.Errorf("5 redirects: %v", err)
	}
	for path, want := range map[string]error{"/r/6": model.ErrFeedUnreachable, "/ftp": model.ErrFeedUnreachable, "/big": model.ErrFeedInvalid} {
		if _, err := e.svc.Subscribe(ctx, 3, srv.URL+path); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", path, err, want)
		}
	}
}
