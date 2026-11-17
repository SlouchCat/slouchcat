package wishes

import (
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr"
	"github.com/slouchcat/slouchcat/server/internal/arr/arrtest"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// progressRun runs one wishes.progress job a minute after the last one.
func (f *arrFixture) progressRun() {
	f.t.Helper()
	f.advance(time.Minute)
	if err := f.svc.handleProgress(f.ctx, &jobs.Job{Type: JobProgress}); err != nil {
		f.t.Fatalf("wishes.progress: %v", err)
	}
}

// movieWish adds a film wish in procurement.
func (f *arrFixture) movieWish(id, tmdb int) {
	f.t.Helper()
	f.wish(id, "movie", tmdb, "processing", "slouchcat", 1)
}

// showWish adds a series wish in procurement with its seasons and their states.
func (f *arrFixture) showWish(id, tmdb, tvdb int, seasons map[int]string) {
	f.t.Helper()
	f.exec(`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (?, 'show', ?, ?, 'Serie', 'slouchcat', 'processing', 1, 1)`, id, tmdb, tvdb)
	for n, st := range seasons {
		f.exec(`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (?, ?, ?, ?)`, id, n, tmdb, st)
	}
	f.join(id, 1, "owner")
}

func (f *arrFixture) progress(id int64) (WishProgress, bool, AdminWishProgress, bool) {
	f.t.Helper()
	v, err := f.svc.ProgressOf(f.ctx, []int64{id})
	if err != nil {
		f.t.Fatal(err)
	}
	a, err := f.svc.AdminProgressOf(f.ctx, []int64{id})
	if err != nil {
		f.t.Fatal(err)
	}
	vp, vok := v[id]
	ap, aok := a[id]
	return vp, vok, ap, aok
}

// The four displays from the queue of Radarr: download with percent and time left, search, import
// and stalled with the message only for the administration and without a path; an entry without a
// wish is not kept; the queue is read once per run over both pages; the readers never call Radarr.
func TestProgressRadarrStages(t *testing.T) {
	f := newArrFixture(t)
	q := arrtest.RadarrQueue()
	q[1]["errorMessage"] = "Import failed, path does not exist: /data/media/movies/Film (2024)/film.mkv"
	f.radarr.SetQueue(q...)
	f.radarr.SetMaxPageSize(2) // four entries on two pages
	f.radarr.AddMovie(arrtest.Movie{ID: 1, TMDBID: 601})
	f.radarr.AddMovie(arrtest.Movie{ID: 2, TMDBID: 602})
	f.radarr.AddMovie(arrtest.Movie{ID: 3, TMDBID: 603})
	f.radarr.AddMovie(arrtest.Movie{ID: 4, TMDBID: 604})
	f.radarr.AddMovie(arrtest.Movie{ID: 5, TMDBID: 605, HasFile: true})
	f.radarr.AddMovie(arrtest.Movie{ID: 9, TMDBID: 609}) // in the queue? no, and without a wish
	for id, tmdb := range map[int]int{1: 601, 2: 602, 3: 603, 4: 604, 5: 605} {
		f.movieWish(id, tmdb)
	}
	f.progressRun()

	if n := f.radarr.Calls("GET /api/v3/queue"); n != 2 {
		t.Errorf("queue pages read %d, want 2 (one read over two pages)", n)
	}
	v, ok, a, _ := f.progress(1)
	if !ok || v.Stage != StageDownload || v.Percent == nil || *v.Percent != 45 || v.ETASeconds == nil || *v.ETASeconds != 720 {
		t.Errorf("wish 1: %+v %v, want download 45 %% 720 s", v, ok)
	}
	if a.Service != "radarr" || a.Message != "(SABnzbd)" {
		t.Errorf("wish 1 admin: %+v", a)
	}
	v, ok, a, _ = f.progress(2)
	if !ok || v.Stage != StageStalled || v.Percent != nil {
		t.Errorf("wish 2: %+v, want stalled", v)
	}
	if !strings.Contains(a.Message, "No files found are eligible for import") || strings.Contains(a.Message, "/data") ||
		strings.Contains(a.Message, "film.mkv") || strings.Contains(a.Message, "/") {
		t.Errorf("wish 2 admin message %q: want the message without a path", a.Message)
	}
	if v, ok, _, _ := f.progress(3); !ok || v.Stage != StageDownload || v.Percent != nil {
		t.Errorf("wish 3 (entry on page 2 without sizes): %+v %v, want download without percent", v, ok)
	}
	if v, ok, _, _ := f.progress(4); !ok || v.Stage != StageSearch {
		t.Errorf("wish 4: %+v %v, want search", v, ok)
	}
	if v, ok, _, _ := f.progress(5); !ok || v.Stage != StageImport {
		t.Errorf("wish 5: %+v %v, want import", v, ok)
	}
	if got := f.count(`SELECT count(*) FROM wish_progress`); got != 5 {
		t.Errorf("%d rows, want 5 (one per wish, none for entries without a wish)", got)
	}
	if got := f.count(`SELECT count(*) FROM wish_progress WHERE COALESCE(admin_message, '') LIKE '%/%'`); got != 0 {
		t.Errorf("%d rows with a path in admin_message", got)
	}
	f.radarr.ResetCalls()
	f.progress(1)
	if _, err := f.svc.ArrStatus(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(f.radarr.CallLog()); n != 0 {
		t.Errorf("the readers made %d calls to Radarr, want 0", n)
	}
	// The title ids are kept: the next run reads only the queue.
	f.progressRun()
	if n, m := f.radarr.Calls("GET /api/v3/queue"), f.radarr.Calls("GET /api/v3/movie"); n != 2 || m != 0 {
		t.Errorf("second run: %d queue pages and %d lookups, want 2 and 0", n, m)
	}
	onlyAllowedCalls(t, f.radarr)
}

// Sonarr: entries by seriesId and seasonNumber of the wished seasons, the sum over the entries, an
// entry without seriesId passed over, «x von y Folgen da» from the season statistics.
func TestProgressSonarrSeasons(t *testing.T) {
	f := newArrFixture(t)
	f.sonarr.SetQueue(arrtest.SonarrQueue()...)
	f.sonarr.AddSeries(arrtest.Series{ID: 7, TVDBID: 700, Seasons: []arrtest.Season{
		{Number: 1, Monitored: true, EpisodeFileCount: 6, EpisodeCount: 10, TotalEpisodeCount: 10},
		{Number: 2, Monitored: true, EpisodeFileCount: 0, EpisodeCount: 8, TotalEpisodeCount: 8},
	}})
	f.sonarr.AddSeries(arrtest.Series{ID: 8, TVDBID: 800, Seasons: []arrtest.Season{
		{Number: 1, Monitored: true, EpisodeFileCount: 5, EpisodeCount: 5, TotalEpisodeCount: 5},
	}})
	f.showWish(1, 1399, 700, map[int]string{1: "processing"})
	f.showWish(2, 1400, 700, map[int]string{2: "processing"}) // season 2 has only the entry without seriesId
	f.showWish(3, 1401, 800, map[int]string{1: "processing"})
	f.progressRun()

	v, ok, _, _ := f.progress(1)
	if !ok || v.Stage != StageDownload || v.Percent == nil || *v.Percent != 37 || v.ETASeconds == nil || *v.ETASeconds != 93784 {
		t.Errorf("wish 1: %+v, want download 37 %% 93784 s", v)
	}
	if v.Episodes == nil || v.Episodes.Done != 6 || v.Episodes.Total != 10 {
		t.Errorf("wish 1 episodes %+v, want 6 of 10", v.Episodes)
	}
	if v, ok, _, _ := f.progress(2); !ok || v.Stage != StageSearch || v.Episodes == nil || v.Episodes.Total != 8 {
		t.Errorf("wish 2: %+v %v, want search 0 of 8", v, ok)
	}
	if v, ok, _, _ := f.progress(3); !ok || v.Stage != StageImport {
		t.Errorf("wish 3: %+v %v, want import", v, ok)
	}
	onlyAllowedCalls(t, f.sonarr)
}

// Without wishes in procurement nothing is called; the switch off calls nothing and empties the
// table.
func TestProgressNoWishesAndSwitchOff(t *testing.T) {
	f := newArrFixture(t)
	f.wish(1, "movie", 601, "pending", "slouchcat", 1)
	f.progressRun()
	if n := len(f.radarr.CallLog()) + len(f.sonarr.CallLog()); n != 0 {
		t.Errorf("%d calls without wishes in procurement, want 0", n)
	}
	q := &fakeQueue{}
	f.svc.queue = q
	if ids, err := f.svc.runProgress(f.ctx, nil, progressPriority); err != nil || len(ids) != 0 || q.count(JobProgress) != 0 {
		t.Errorf("schedule without wishes: %v %v, %d jobs", ids, err, q.count(JobProgress))
	}
	f.radarr.AddMovie(arrtest.Movie{ID: 2, TMDBID: 602})
	f.movieWish(2, 602)
	f.progressRun()
	if got := f.count(`SELECT count(*) FROM wish_progress`); got != 1 {
		t.Fatalf("%d rows, want 1", got)
	}
	if q.count(JobProgress) != 1 {
		t.Errorf("next run queued %d times, want 1", q.count(JobProgress))
	}
	f.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.show_progress', '0', 1)`)
	f.radarr.ResetCalls()
	f.progressRun()
	if got := f.count(`SELECT count(*) FROM wish_progress`); got != 0 || len(f.radarr.CallLog()) != 0 {
		t.Errorf("switch off: %d rows and %d calls, want 0 and 0", got, len(f.radarr.CallLog()))
	}
	if ids, err := f.svc.runProgress(f.ctx, nil, progressPriority); err != nil || len(ids) != 0 {
		t.Errorf("schedule with the switch off: %v %v", ids, err)
	}
	if v, err := f.svc.AdminProgressOf(f.ctx, []int64{2}); err != nil || len(v) != 0 {
		t.Errorf("admin read with the switch off: %v %v", v, err)
	}
}

// The keys come from Seerr at most every 10 minutes and at once after a 401.
func TestProgressKeysFromSeerr(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 2, TMDBID: 602})
	f.movieWish(2, 602)
	seerrReads := func() int { return f.fake.Calls("GET /settings/radarr") }
	base := seerrReads() // the test of the fixture
	for range 8 {
		f.progressRun()
	}
	if got := seerrReads() - base; got != 0 {
		t.Errorf("Seerr read %d times within 10 minutes, want 0", got)
	}
	f.progressRun()
	f.progressRun() // 10 minutes and more after the test
	if got := seerrReads() - base; got != 1 {
		t.Errorf("Seerr read %d times after 10 minutes, want 1", got)
	}
	f.radarr.SetFailure("GET /api/v3/queue", arrtest.Failure{Status: 401})
	f.progressRun()
	f.radarr.SetFailure("GET /api/v3/queue", arrtest.Failure{})
	f.progressRun()
	if got := seerrReads() - base; got != 2 {
		t.Errorf("Seerr read %d times after a 401, want 2", got)
	}
	if v, ok, _, _ := f.progress(2); !ok || v.Stage != StageSearch {
		t.Errorf("after the new key: %+v %v, want search", v, ok)
	}
}

// Radarr gone: the row says unreachable, viewers see nothing, the administration the error and
// the time of the last read; a stale row shows viewers nothing; a closed wish keeps no row.
func TestProgressRadarrGoneAndStale(t *testing.T) {
	f := newArrFixture(t)
	f.rebuild(WithClock(f.clock), WithArrOptions(arr.WithLimiter(ratelimit.New("arr-test", 1000, 100)), arr.WithTimeout(200*time.Millisecond)))
	f.radarr.SetQueue(arrtest.RadarrQueue()...)
	f.radarr.AddMovie(arrtest.Movie{ID: 1, TMDBID: 601})
	f.movieWish(1, 601)
	f.progressRun()
	if _, ok, _, _ := f.progress(1); !ok {
		t.Fatal("no progress after the first run")
	}
	readAt := f.clock()
	f.radarr.SetFailure("*", arrtest.Failure{Hang: true})
	f.progressRun()
	if v, ok, a, aok := f.progress(1); ok || !aok || a.Error != ProgressUnreachable || !a.ReadAt.Equal(readAt) {
		t.Errorf("Radarr gone: viewer %+v %v, admin %+v, want nothing for viewers, unreachable with the last read", v, ok, a)
	}
	st, err := f.svc.ArrStatus(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Radarr.Progress.Error != ProgressUnreachable || !st.Radarr.Progress.Enabled {
		t.Errorf("status %+v, want unreachable", st.Radarr.Progress)
	}
	if got := f.count(`SELECT count(*) FROM wishes WHERE id = 1 AND state = 'processing' AND updated_at = 1`); got != 1 {
		t.Error("the wish changed")
	}
	f.radarr.SetFailure("*", arrtest.Failure{})
	f.progressRun()
	if _, ok, _, _ := f.progress(1); !ok {
		t.Error("no progress after Radarr came back")
	}
	f.advance(6 * time.Minute)
	if v, ok, a, _ := f.progress(1); ok || !a.Stale {
		t.Errorf("stale: viewer %+v %v, admin %+v, want nothing for viewers, stale for the administration", v, ok, a)
	}
	f.exec(`UPDATE wishes SET state = 'available', updated_at = 2 WHERE id = 1`)
	if got := f.count(`SELECT count(*) FROM wish_progress`); got != 0 {
		t.Errorf("%d rows after the wish was fulfilled, want 0", got)
	}
}

// A title not yet in Radarr: nothing for viewers, not_found for the administration; an
// unconfirmed host is not called.
func TestProgressNotFoundAndUnconfirmed(t *testing.T) {
	f := newArrFixture(t)
	f.movieWish(1, 601)
	f.progressRun()
	if v, ok, a, _ := f.progress(1); ok || a.Error != ProgressNotFound {
		t.Errorf("not in Radarr: viewer %+v %v, admin %+v", v, ok, a)
	}
	f.fake.Configure(func(c *seerrtest.Config) {
		c.Radarr = []seerrtest.DVR{dvrOf(t, 0, strings.Replace(f.radarr.URL, "127.0.0.1", "localhost", 1))}
	})
	f.advance(11 * time.Minute)
	f.radarr.ResetCalls()
	f.progressRun()
	if n := len(f.radarr.CallLog()); n != 0 {
		t.Errorf("%d calls to an unconfirmed host, want 0", n)
	}
	if _, _, a, _ := f.progress(1); a.Error != ProgressUnreachable {
		t.Errorf("unconfirmed host: %+v, want unreachable", a)
	}
}
