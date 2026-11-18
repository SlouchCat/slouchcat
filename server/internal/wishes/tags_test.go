package wishes

import (
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr"
	"github.com/slouchcat/slouchcat/server/internal/arr/arrtest"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// The fixture of stage 4: the fixture of the wishes plus a fake Radarr and a fake Sonarr that Seerr
// lists as its default servers without 4K (beside a 4K Radarr that must never be used), a fake
// clock, and both hosts confirmed by «Testen».

type arrFixture struct {
	*fixture
	radarr, sonarr *arrtest.Server

	mu  sync.Mutex
	now time.Time
}

func newArrFixture(t *testing.T) *arrFixture {
	t.Helper()
	f := &arrFixture{fixture: newFixture(t), radarr: arrtest.NewRadarr(t), sonarr: arrtest.NewSonarr(t),
		now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	f.fake.Configure(func(c *seerrtest.Config) {
		c.Radarr = []seerrtest.DVR{
			{ID: 1, Name: "Radarr 4K", Hostname: "192.0.2.44", Port: 7878, APIKey: "radarr-4k-FAKE-key", Is4K: true, IsDefault: true},
			dvrOf(t, 0, f.radarr.URL), // Seerr's server id can be 0 (N-113)
		}
		c.Sonarr = []seerrtest.DVR{dvrOf(t, 0, f.sonarr.URL)}
	})
	f.rebuild(WithClock(f.clock), WithArrOptions(arr.WithLimiter(ratelimit.New("arr-test", 1000, 100)), arr.WithTimeout(time.Second)))
	for _, k := range []arr.Kind{arr.Radarr, arr.Sonarr} {
		res, err := f.svc.TestArrConnection(f.ctx, k)
		if err != nil || !res.Reachable || !res.KeyValid {
			t.Fatalf("test %s: %+v, %v", k, res, err)
		}
	}
	f.radarr.ResetCalls()
	f.sonarr.ResetCalls()
	return f
}

func dvrOf(t *testing.T, id int, raw string) seerrtest.DVR {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(port)
	return seerrtest.DVR{ID: id, Name: "default", Hostname: host, Port: p, APIKey: arrtest.APIKey, IsDefault: true}
}

func (f *arrFixture) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *arrFixture) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// tagRun runs one wishes.tag job.
func (f *arrFixture) tagRun() {
	f.t.Helper()
	if err := f.svc.handleTags(f.ctx, &jobs.Job{Type: JobTag}); err != nil {
		f.t.Fatalf("wishes.tag: %v", err)
	}
}

// wish adds a wish in state with its owner profile; a show gets the TVDB id 121361 and season 1.
func (f *arrFixture) wish(id int, media string, tmdb int, state, origin string, owner int) {
	f.t.Helper()
	if media == "movie" {
		f.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (?, 'movie', ?, 'Film', ?, ?, 1, 1)`, id, tmdb, origin, state)
	} else {
		f.exec(`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (?, 'show', ?, 121361, 'Serie', ?, ?, 1, 1)`, id, tmdb, origin, state)
		f.exec(`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (?, 1, ?, 'processing')`, id, tmdb)
	}
	f.join(id, owner, "owner")
}

func (f *arrFixture) join(wish, profile int, role string) {
	f.t.Helper()
	f.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at)
	        SELECT ?, id, account_id, ?, 1 FROM profiles WHERE id = ?`, wish, role, profile)
}

func (f *arrFixture) tagState(wish, profile int) (state, name, lastErr string) {
	f.t.Helper()
	err := f.db.Read.QueryRowContext(f.ctx, `SELECT state, tag_name, COALESCE(last_error, '') FROM wish_tags WHERE wish_id = ? AND profile_id = ?`,
		wish, profile).Scan(&state, &name, &lastErr)
	if err != nil {
		f.t.Fatalf("tag row %d/%d: %v", wish, profile, err)
	}
	return state, name, lastErr
}

// allowedArrRoutes are the only calls rule 11 allows (data model 6).
var allowedArrRoutes = map[string]bool{
	"GET /api/v3/tag": true, "POST /api/v3/tag": true, "GET /api/v3/movie": true, "PUT /api/v3/movie/editor": true,
	"GET /api/v3/series": true, "PUT /api/v3/series/editor": true, "GET /api/v3/queue": true,
}

// onlyAllowedCalls fails on any other route and on an editor call with another mode than "add".
func onlyAllowedCalls(t *testing.T, s *arrtest.Server) {
	t.Helper()
	for _, c := range s.CallLog() {
		if !allowedArrRoutes[c.Route] {
			t.Errorf("%s: call outside the allowed endpoints: %s", s.Kind, c.Route)
		}
		if strings.HasSuffix(c.Route, "/editor") {
			var body struct {
				ApplyTags string `json:"applyTags"`
			}
			if err := json.Unmarshal(c.Body, &body); err != nil || body.ApplyTags != "add" {
				t.Errorf("%s: editor sent %s, want applyTags add", s.Kind, c.Body)
			}
		}
		if strings.Contains(string(c.Body), "remove") || strings.Contains(string(c.Body), "replace") {
			t.Errorf("%s: a call carried remove or replace: %s", s.Kind, c.Body)
		}
	}
}

func tagID(s *arrtest.Server, label string) int {
	for id, l := range s.Tags() {
		if l == label {
			return id
		}
	}
	return 0
}

// The tag of the profile is created once and hangs on the film and on the series; a second run
// calls nothing; the wishes stay as they were.
func TestTagSetOnceOnFilmAndSeries(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603, Title: "The Matrix"})
	f.sonarr.AddSeries(arrtest.Series{ID: 21, TVDBID: 121361, Title: "Game of Thrones", Tags: []int{}})
	f.sonarr.AddTag("1-marco") // Seerr's own tag of the user stays (N-113)
	f.wish(1, "movie", 603, "processing", "slouchcat", 2)
	f.wish(2, "show", 1399, "processing", "slouchcat", 2)
	f.tagRun()

	for _, s := range []*arrtest.Server{f.radarr, f.sonarr} {
		if s.Calls("POST /api/v3/tag") != 1 {
			t.Errorf("%s: tag created %d times, want 1", s.Kind, s.Calls("POST /api/v3/tag"))
		}
		if tagID(s, "sc-noah") == 0 {
			t.Errorf("%s: tag sc-noah missing: %v", s.Kind, s.Tags())
		}
	}
	if m, _ := f.radarr.Movie(11); !containsInt(m.Tags, tagID(f.radarr, "sc-noah")) {
		t.Errorf("movie tags = %v, want sc-noah", m.Tags)
	}
	if se, _ := f.sonarr.Series(21); !containsInt(se.Tags, tagID(f.sonarr, "sc-noah")) {
		t.Errorf("series tags = %v, want sc-noah", se.Tags)
	}
	if len(f.sonarr.Tags()) != 2 {
		t.Errorf("sonarr tags = %v, want Seerr's tag and sc-noah", f.sonarr.Tags())
	}
	for _, w := range []int{1, 2} {
		if st, name, _ := f.tagState(w, 2); st != tagSet || name != "sc-noah" {
			t.Errorf("wish %d: %s %s, want set sc-noah", w, st, name)
		}
	}
	f.radarr.ResetCalls()
	f.sonarr.ResetCalls()
	f.tagRun()
	if n := len(f.radarr.CallLog()) + len(f.sonarr.CallLog()); n != 0 {
		t.Errorf("second run made %d calls, want 0", n)
	}
	if got := f.count(`SELECT count(*) FROM wishes WHERE state = 'processing' AND updated_at = 1`); got != 2 {
		t.Errorf("wishes changed: %d unchanged, want 2", got)
	}
	onlyAllowedCalls(t, f.radarr)
	onlyAllowedCalls(t, f.sonarr)
}

// A title not yet in Radarr stays pending and gets its tag in a later run.
func TestTagPendingUntilTitleInRadarr(t *testing.T) {
	f := newArrFixture(t)
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.tagRun()
	if st, _, e := f.tagState(1, 1); st != tagPending || e != tagErrNotInArr {
		t.Fatalf("state %s %s, want pending not_in_arr", st, e)
	}
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.tagRun()
	if st, name, _ := f.tagState(1, 1); st != tagSet || name != "sc-marco" {
		t.Fatalf("state %s %s, want set sc-marco", st, name)
	}
	if f.radarr.Calls("POST /api/v3/tag") != 1 {
		t.Errorf("tag created %d times, want 1", f.radarr.Calls("POST /api/v3/tag"))
	}
	onlyAllowedCalls(t, f.radarr)
}

// Radarr unreachable: the wish stays as it is, the row is failed, the next run sets the tag.
func TestTagRadarrUnreachableThenSet(t *testing.T) {
	f := newArrFixture(t)
	f.rebuild(WithClock(f.clock), WithArrOptions(arr.WithLimiter(ratelimit.New("arr-test", 1000, 100)), arr.WithTimeout(200*time.Millisecond)))
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.wish(2, "movie", 604, "processing", "slouchcat", 2)
	f.radarr.SetFailure("*", arrtest.Failure{Hang: true})
	f.tagRun()
	for _, w := range [][2]int{{1, 1}, {2, 2}} {
		if st, _, e := f.tagState(w[0], w[1]); st != tagFailed || (e != "timeout" && e != "unreachable") {
			t.Errorf("wish %d: %s %s, want failed timeout", w[0], st, e)
		}
	}
	if f.radarr.Calls("GET /api/v3/tag") != 1 {
		t.Errorf("tag list read %d times while Radarr hangs, want 1 (the run stops)", f.radarr.Calls("GET /api/v3/tag"))
	}
	if got := f.count(`SELECT count(*) FROM wishes WHERE state = 'processing' AND updated_at = 1 AND fail_reason IS NULL`); got != 2 {
		t.Errorf("wishes changed: %d unchanged, want 2", got)
	}
	st, err := f.svc.ArrStatus(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Radarr.Reachable == nil || *st.Radarr.Reachable || st.Radarr.Counts.Failed != 2 {
		t.Errorf("status %+v, want unreachable with 2 failed", st.Radarr)
	}
	f.radarr.SetFailure("*", arrtest.Failure{})
	f.tagRun()
	if st, _, _ := f.tagState(1, 1); st != tagSet {
		t.Errorf("after the service came back: %s, want set", st)
	}
	if st, _, e := f.tagState(2, 2); st != tagPending || e != tagErrNotInArr {
		t.Errorf("wish 2 without movie: %s %s, want pending not_in_arr", st, e)
	}
}

// A co-wisher gets its tag afterwards; the owner's row is not touched again.
func TestTagCoWisherLater(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.tagRun()
	f.join(1, 3, "joined")
	f.radarr.ResetCalls()
	f.tagRun()
	if st, name, _ := f.tagState(1, 3); st != tagSet || name != "sc-gast" {
		t.Fatalf("co-wisher: %s %s, want set sc-gast", st, name)
	}
	m, _ := f.radarr.Movie(11)
	if !containsInt(m.Tags, tagID(f.radarr, "sc-marco")) || !containsInt(m.Tags, tagID(f.radarr, "sc-gast")) {
		t.Errorf("movie tags = %v (%v), want both", m.Tags, f.radarr.Tags())
	}
	if n := f.radarr.Calls("PUT /api/v3/movie/editor"); n != 1 {
		t.Errorf("editor called %d times for the co-wisher, want 1", n)
	}
	onlyAllowedCalls(t, f.radarr)
}

// Requests placed directly in Seerr get no tag, other states neither; nothing calls Radarr.
func TestTagNothingForSeerrRequests(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "seerr", 1)
	f.wish(2, "movie", 604, "pending", "slouchcat", 1)
	f.wish(3, "movie", 605, "available", "slouchcat", 1)
	f.tagRun()
	if got := f.count(`SELECT count(*) FROM wish_tags`); got != 0 {
		t.Errorf("%d tag rows, want 0", got)
	}
	if n := len(f.radarr.CallLog()); n != 0 {
		t.Errorf("%d calls to Radarr, want 0", n)
	}
	if got := f.fake.Calls("GET /settings/radarr"); got != 1 { // the test of the fixture
		t.Errorf("Seerr asked %d times, want 1", got)
	}
}

// The switch off: no row, no call.
func TestTagSwitchOff(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.profile_tags', '0', 1)`)
	f.tagRun()
	if got := f.count(`SELECT count(*) FROM wish_tags`); got != 0 || len(f.radarr.CallLog()) != 0 {
		t.Errorf("%d rows and %d calls with the switch off, want 0", got, len(f.radarr.CallLog()))
	}
}

// The key of Radarr and Sonarr never reaches a log, the database or a returned value.
func TestTagKeyNeverStored(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.wish(2, "show", 1399, "processing", "slouchcat", 1)
	f.tagRun()
	st, err := f.svc.ArrStatus(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.TestArrConnection(f.ctx, arr.Radarr)
	if err != nil {
		t.Fatal(err)
	}
	checks := f.svc.ArrTagChecks(f.ctx)
	own := "http://192.0.2.9:7878"
	st2, err := f.svc.UpdateArrSettings(f.ctx, ArrSettingsUpdate{SonarrURL: &own})
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"status": st, "test": res, "checks": checks, "status after update": st2} {
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), arrtest.APIKey) || strings.Contains(string(b), "radarr-4k-FAKE-key") {
			t.Errorf("%s carries the key: %s", name, b)
		}
	}
	if strings.Contains(f.logs.String(), arrtest.APIKey) {
		t.Error("the key reached the log")
	}
	for _, q := range []string{
		`SELECT count(*) FROM settings WHERE value LIKE ?`,
		`SELECT count(*) FROM wish_tags WHERE COALESCE(last_error, '') || tag_name LIKE ?`,
		`SELECT count(*) FROM seerr_log WHERE COALESCE(error, '') || path LIKE ?`,
	} {
		if got := f.count(q, "%"+arrtest.APIKey+"%"); got != 0 {
			t.Errorf("%s: %d rows with the key", q, got)
		}
	}
	if st.Radarr.Source != ArrSourceSeerr || !st.Radarr.Confirmed || st2.Sonarr.Source != ArrSourceOwn {
		t.Errorf("status radarr %+v sonarr %+v", st.Radarr, st2.Sonarr)
	}
	if len(checks) != 2 || checks[0].Key != CheckRadarrTags || checks[0].Level != model.SeerrLevelOK {
		t.Errorf("checks = %+v", checks)
	}
}

// Another host from Seerr halts the tags until the administrator tests again; the 4K server is
// never used.
func TestTagHostChangeHaltsUntilConfirmed(t *testing.T) {
	f := newArrFixture(t)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	moved := dvrOf(t, 0, strings.Replace(f.radarr.URL, "127.0.0.1", "localhost", 1)) // the same fake under another host
	f.fake.Configure(func(c *seerrtest.Config) { c.Radarr = []seerrtest.DVR{moved} })
	f.advance(11 * time.Minute) // the key cache reads Seerr again
	f.tagRun()
	if n := len(f.radarr.CallLog()); n != 0 {
		t.Fatalf("%d calls to an unconfirmed host, want 0", n)
	}
	if st, _, _ := f.tagState(1, 1); st != tagPending {
		t.Errorf("row %s while halted, want pending", st)
	}
	st, err := f.svc.ArrStatus(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Radarr.Confirmed || st.Radarr.LastError != "not_confirmed" {
		t.Errorf("status %+v, want unconfirmed", st.Radarr)
	}
	if res, err := f.svc.TestArrConnection(f.ctx, arr.Radarr); err != nil || !res.KeyValid || !strings.Contains(res.Host, "localhost") {
		t.Fatalf("test: %+v, %v", res, err)
	}
	f.tagRun()
	if st, _, _ := f.tagState(1, 1); st != tagSet {
		t.Errorf("row %s after the confirmation, want set", st)
	}
	if got := f.count(`SELECT count(*) FROM settings WHERE key = 'arr.radarr_host' AND value LIKE 'http://localhost:%'`); got != 1 {
		t.Errorf("confirmed host not stored")
	}
}

// Two profiles with the same name: the earlier keeps the plain name, the later gets the account's
// name; the names never change afterwards, also not after a rename.
func TestTagCollisionRule(t *testing.T) {
	f := newArrFixture(t)
	f.exec(`INSERT INTO profiles (id, account_id, name, is_default, created_at, updated_at, can_request) VALUES (5, 2, 'Noah', 0, 1, 1, 1)`)
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.radarr.AddMovie(arrtest.Movie{ID: 12, TMDBID: 604})
	f.wish(1, "movie", 603, "processing", "slouchcat", 5) // the later profile first
	f.tagRun()
	if _, name, _ := f.tagState(1, 5); name != "sc-noah-gast" {
		t.Errorf("later profile: %s, want sc-noah-gast", name)
	}
	f.join(1, 2, "joined")
	f.tagRun()
	if _, name, _ := f.tagState(1, 2); name != "sc-noah" {
		t.Errorf("earlier profile: %s, want sc-noah", name)
	}
	f.exec(`UPDATE profiles SET name = 'Lena' WHERE id = 5`)
	f.wish(2, "movie", 604, "processing", "slouchcat", 5)
	f.tagRun()
	if _, name, _ := f.tagState(2, 5); name != "sc-noah-gast" {
		t.Errorf("after the rename: %s, want the stored sc-noah-gast", name)
	}
}

// The label rule N-113 for names with umlauts, blanks and emoji, the prefix setting and the name
// set by hand.
func TestTagLabelRule(t *testing.T) {
	f := newArrFixture(t)
	f.exec(`INSERT INTO profiles (id, account_id, name, is_default, created_at, updated_at, can_request)
	        VALUES (6, 2, 'Jürg Müller', 0, 1, 1, 1), (7, 2, '🎬🍿', 0, 1, 1, 1), (8, 2, '  Ömer   Straße! ', 0, 1, 1, 1)`)
	for id, want := range map[int64]string{6: "sc-juerg-mueller", 7: "sc-profil-7", 8: "sc-oemer-strasse"} {
		got, err := f.svc.profileTagName(f.ctx, id)
		if err != nil || got != want {
			t.Errorf("profile %d: %q, %v; want %q", id, got, err, want)
		}
	}
	f.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('wishes.tag_prefix', 'kids-', 1)`)
	if tag, err := f.svc.ProfileTagOf(f.ctx, 2); err != nil || tag.Name != "kids-noah" || tag.Custom {
		t.Errorf("with the prefix kids-: %+v, %v", tag, err)
	}
	bad := "Noah Kids"
	var verr *model.ValidationError
	if err := f.svc.SetProfileTagName(f.ctx, 2, &bad); !errors.As(err, &verr) {
		t.Errorf("name with blank: %v, want a validation error", err)
	}
	taken := "sc-juerg-mueller"
	if err := f.svc.SetProfileTagName(f.ctx, 2, &taken); !errors.Is(err, ErrTagNameTaken) {
		t.Errorf("taken name: %v, want ErrTagNameTaken", err)
	}
	if err := f.svc.SetProfileTagName(f.ctx, 99, &bad); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown profile: %v, want ErrNotFound", err)
	}
	custom := "noah-x"
	if err := f.svc.SetProfileTagName(f.ctx, 2, &custom); err != nil {
		t.Fatal(err)
	}
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 2)
	f.tagRun()
	if st, name, _ := f.tagState(1, 2); st != tagSet || name != "noah-x" || tagID(f.radarr, "noah-x") == 0 {
		t.Errorf("custom name: %s %s, tags %v", st, name, f.radarr.Tags())
	}
	if tag, _ := f.svc.ProfileTagOf(f.ctx, 2); !tag.Custom {
		t.Errorf("custom flag lost: %+v", tag)
	}
	tags, err := f.svc.WishTagsOf(f.ctx, []int64{1})
	if err != nil || len(tags[1]) != 1 || tags[1][0].State != tagSet || tags[1][0].Service != "radarr" {
		t.Errorf("WishTagsOf = %+v, %v", tags, err)
	}
}

// A series without TVDB id stays pending with its reason.
func TestTagSeriesWithoutTVDB(t *testing.T) {
	f := newArrFixture(t)
	f.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (1, 'show', 1399, 'Serie', 'slouchcat', 'processing', 1, 1)`)
	f.join(1, 1, "owner")
	f.tagRun()
	if st, _, e := f.tagState(1, 1); st != tagPending || e != tagErrNoTVDB {
		t.Errorf("%s %s, want pending tvdb_id_missing", st, e)
	}
	onlyAllowedCalls(t, f.sonarr)
}

// The own address replaces the address from Seerr and needs no confirmation; a bad one is refused.
func TestTagOwnAddress(t *testing.T) {
	f := newArrFixture(t)
	bad := "ftp://radarr.example"
	var verr *model.ValidationError
	if _, err := f.svc.UpdateArrSettings(f.ctx, ArrSettingsUpdate{RadarrURL: &bad}); !errors.As(err, &verr) {
		t.Fatalf("bad address: %v, want a validation error", err)
	}
	own := strings.Replace(f.radarr.URL, "127.0.0.1", "localhost", 1)
	if _, err := f.svc.UpdateArrSettings(f.ctx, ArrSettingsUpdate{RadarrURL: &own}); err != nil {
		t.Fatal(err)
	}
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.tagRun()
	if st, _, _ := f.tagState(1, 1); st != tagSet {
		t.Errorf("with the own address: %s, want set", st)
	}
	empty := ""
	st, err := f.svc.UpdateArrSettings(f.ctx, ArrSettingsUpdate{RadarrURL: &empty})
	if err != nil || st.Radarr.Source != ArrSourceSeerr {
		t.Errorf("back to Seerr: %+v, %v", st.Radarr, err)
	}
}

// A pending row takes the tag name set meanwhile (review Minor 2, 9).
func TestTagNameChangedWhilePending(t *testing.T) {
	f := newArrFixture(t)
	f.wish(1, "movie", 603, "processing", "slouchcat", 1)
	f.tagRun()
	if st, name, _ := f.tagState(1, 1); st != tagPending || name != "sc-marco" {
		t.Fatalf("state %s %s, want pending sc-marco", st, name)
	}
	custom := "papa"
	if err := f.svc.SetProfileTagName(f.ctx, 1, &custom); err != nil {
		t.Fatal(err)
	}
	f.radarr.AddMovie(arrtest.Movie{ID: 11, TMDBID: 603})
	f.tagRun()
	if st, name, _ := f.tagState(1, 1); st != tagSet || name != "papa" {
		t.Fatalf("state %s %s, want set papa", st, name)
	}
	// The old tag was created by the first run (it ensures the tag before it looks for the title)
	// but never reaches the title.
	if m, _ := f.radarr.Movie(11); !containsInt(m.Tags, tagID(f.radarr, "papa")) || containsInt(m.Tags, tagID(f.radarr, "sc-marco")) {
		t.Errorf("movie tags = %v, radarr tags %v", m.Tags, f.radarr.Tags())
	}
}
