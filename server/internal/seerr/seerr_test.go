package seerr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

func fast() *ratelimit.Limiter { return ratelimit.New("seerr-test", 1000, 100) }

func newClient(t *testing.T, f *seerrtest.Server, opts ...Option) *Client {
	t.Helper()
	c, err := New(f.URL, seerrtest.APIKey, append([]Option{WithLimiter(fast())}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ptr[T any](v T) *T { return &v }

// localUser adds a local user with the given permissions and returns its id.
func localUser(f *seerrtest.Server, email string, perms uint64) int {
	return f.AddUser(seerrtest.User{Email: email, Username: strings.Split(email, "@")[0], UserType: seerrtest.UserLocal, Permissions: perms, Password: "old-password-1"})
}

func TestNewRefusesBadAddresses(t *testing.T) {
	for _, raw := range []string{"", "ftp://seerr.lan", "http://", "http://user:secret-pw@seerr.lan", "http://seerr.lan/?k=v", "http://seerr.lan/#x", "::"} {
		_, err := New(raw, "key")
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("New(%q) = %v, want ErrInvalid", raw, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret-pw") {
			t.Errorf("error quotes the address: %v", err)
		}
	}
	if _, err := New("http://seerr.lan", " "); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty key: %v", err)
	}
	for raw, want := range map[string]string{
		"http://192.168.1.20:5055/":          "http://192.168.1.20:5055",
		"https://seerr.example.org/seerr/":   "https://seerr.example.org/seerr",
		"http://seerr.lan:5055/api/v1":       "http://seerr.lan:5055",
		" https://seerr.example.org/api/v1/": "https://seerr.example.org",
	} {
		c, err := New(raw, "key")
		if err != nil || c.base != want {
			t.Errorf("New(%q) base = %v, %v; want %s", raw, c, err, want)
		}
	}
}

func TestDefaults(t *testing.T) {
	c, err := New("http://seerr.lan", "key")
	if err != nil {
		t.Fatal(err)
	}
	if c.http.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", c.http.Timeout)
	}
	tr := c.http.Transport.(*http.Transport)
	if tr.Proxy != nil {
		t.Error("transport uses a proxy from the environment")
	}
	if Rate != 5 || Burst != 5 {
		t.Errorf("rate %d burst %d, want 5 and 5", Rate, Burst)
	}
}

func TestLANAddressReachable(t *testing.T) {
	f := seerrtest.New(t)
	u, _ := url.Parse(f.URL)
	ip := netip.MustParseAddr(u.Hostname())
	if !netguard.Local(ip) {
		t.Fatalf("%s is not a local address; the test proves nothing", ip)
	}
	st, err := newClient(t, f).Status(context.Background())
	if err != nil || st.Version != "3.5.2" {
		t.Fatalf("Status on %s = %+v, %v", ip, st, err)
	}
}

func TestRedirectRefused(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer target.Close()
	f := seerrtest.New(t)
	for _, status := range []int{http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		f.SetFailure("POST /user/{id}/settings/password", seerrtest.Failure{Status: status, Location: target.URL + "/api/v1/user/2/settings/password"})
		err := newClient(t, f).AsAdmin().SetPassword(context.Background(), 2, "new-password-1")
		if !errors.Is(err, ErrRedirect) || !errors.Is(err, ErrUnreachable) {
			t.Errorf("status %d: err = %v, want ErrRedirect and ErrUnreachable", status, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("redirect target got %d requests", n)
	}
}

func TestTimeout(t *testing.T) {
	f := seerrtest.New(t)
	f.SetFailure("GET /auth/me", seerrtest.Failure{Hang: true})
	c := newClient(t, f, WithTimeout(300*time.Millisecond))
	start := time.Now()
	_, err := c.AsAdmin().Me(context.Background())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
	// The fake never answers, so the error alone shows that a time-out ended the call; the bound
	// tells the 300 ms of the option from the default Timeout (10 s) and is safe on a busy machine.
	if d := time.Since(start); d >= Timeout {
		t.Errorf("call took %v, the option did not replace the default %v", d, Timeout)
	}

	// A deadline of the caller (the 3 s immediate attempt) is a time-out as well.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = newClient(t, f).AsAdmin().Me(ctx)
	if !errors.Is(err, ErrUnreachable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("caller deadline: err = %v", err)
	}

	// A cancelled context is not "unreachable".
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = newClient(t, f).AsAdmin().Me(ctx)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnreachable) {
		t.Errorf("cancelled: err = %v", err)
	}
}

func TestUnreachable(t *testing.T) {
	f := seerrtest.New(t)
	c := newClient(t, f)
	f.Close()
	_, err := c.Status(context.Background())
	if !errors.Is(err, ErrUnreachable) || strings.Contains(err.Error(), seerrtest.APIKey) {
		t.Errorf("err = %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	f := seerrtest.New(t)
	c, err := New(f.URL, seerrtest.APIKey) // the default limiter
	if err != nil {
		t.Fatal(err)
	}
	const n = 15 // 5 in the burst, 10 more at 5 per second: at least 2 s
	start := time.Now()
	var mu sync.Mutex
	var done []time.Duration
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			if _, err := c.Status(context.Background()); err != nil {
				t.Error(err)
			}
			mu.Lock()
			done = append(done, time.Since(start))
			mu.Unlock()
		})
	}
	wg.Wait()
	if d := time.Since(start); d < 1800*time.Millisecond {
		t.Errorf("%d calls took %v, want at least 1.8 s at 5 per second", n, d)
	}
	early := 0
	for _, d := range done {
		if d < 900*time.Millisecond {
			early++
		}
	}
	if early > 10 { // burst 5 plus 5 per second
		t.Errorf("%d calls within 0.9 s", early)
	}
	if got := f.Calls("GET /status"); got != n {
		t.Errorf("fake saw %d calls", got)
	}
}

func TestErrorClasses(t *testing.T) {
	f := seerrtest.New(t)
	c := newClient(t, f)
	all := []error{ErrUnreachable, ErrRedirect, ErrKeyInvalid, ErrAccessDenied, ErrForbidden, ErrNoPermission, ErrQuota,
		ErrBlocklisted, ErrCSRF, ErrNotFound, ErrConflict, ErrDuplicate, ErrUserExists, ErrBadRequest, ErrServer,
		ErrInvalidEmail, ErrUserGone}
	for _, tc := range []struct {
		status int
		msg    string
		asUser bool
		want   []error
	}{
		{403, seerrtest.TextAccessEndpoint, false, []error{ErrKeyInvalid}},
		{403, seerrtest.TextAccessEndpoint, true, []error{ErrAccessDenied, ErrForbidden}},
		{401, "", false, []error{ErrKeyInvalid}},
		{401, seerrtest.TextNoSession, false, []error{ErrKeyInvalid}},
		{400, seerrtest.TextInvalidEmail, false, []error{ErrInvalidEmail, ErrBadRequest}},
		{400, "Password must be at least 8 characters.", false, []error{ErrBadRequest}},
		{403, seerrtest.TextMovieQuota, true, []error{ErrQuota, ErrForbidden}},
		{403, seerrtest.TextSeriesQuota, true, []error{ErrQuota, ErrForbidden}},
		{403, seerrtest.TextBlocklisted, true, []error{ErrBlocklisted, ErrForbidden}},
		{403, "This media is blacklisted.", true, []error{ErrBlocklisted, ErrForbidden}},
		{403, seerrtest.TextNoMovieRight, true, []error{ErrNoPermission, ErrForbidden}},
		{403, seerrtest.TextNoSeriesRight, true, []error{ErrNoPermission, ErrForbidden}},
		{403, "You do not have permission to make 4K movie requests.", true, []error{ErrNoPermission, ErrForbidden}},
		{403, seerrtest.TextModifyUser, false, []error{ErrNoPermission, ErrForbidden}},
		{403, "You do not have permission to bypass user quota limits.", false, []error{ErrNoPermission, ErrForbidden}},
		{401, seerrtest.TextDeleteRequest, true, []error{ErrNoPermission, ErrForbidden}},
		{403, seerrtest.TextCSRF, false, []error{ErrCSRF}},
		{403, "something else", false, []error{ErrForbidden}},
		{404, "", false, []error{ErrNotFound}},
		{409, seerrtest.TextDuplicate, true, []error{ErrDuplicate, ErrConflict}},
		{409, "Auto-request for this media and user already exists.", true, []error{ErrDuplicate, ErrConflict}},
		{409, seerrtest.TextUserExists, false, []error{ErrUserExists, ErrConflict}},
		{409, seerrtest.TextOnlyPending, false, []error{ErrConflict}},
		{400, "request/body must have required property 'mediaType'", false, []error{ErrBadRequest}},
		{500, "Unable to retrieve media from TMDB.", false, []error{ErrServer}},
		{503, "", false, []error{ErrServer}},
	} {
		f.SetFailure("GET /auth/me", seerrtest.Failure{Status: tc.status, Message: tc.msg})
		s := c.AsAdmin()
		if tc.asUser {
			s = c.AsUser(1)
		}
		_, err := s.Me(context.Background())
		var se *Error
		if !errors.As(err, &se) || se.Status != tc.status || se.Message != tc.msg {
			t.Errorf("%d %q: err = %#v", tc.status, tc.msg, err)
			continue
		}
		for _, class := range all {
			want := slices.ContainsFunc(tc.want, func(w error) bool { return errors.Is(w, class) })
			if errors.Is(err, class) != want {
				t.Errorf("%d %q (user %v): errors.Is(%v) = %v", tc.status, tc.msg, tc.asUser, class, !want)
			}
		}
	}
}

func TestAPIUserHeaderExactlyWhenAsked(t *testing.T) {
	f := seerrtest.New(t)
	uid := localUser(f, "anna@example.org", seerrtest.PermRequest)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, Name: "The Matrix", Year: 1999})
	c := newClient(t, f)
	ctx := context.Background()

	admin := c.AsAdmin()
	if _, ok := admin.UserID(); ok {
		t.Fatal("admin session reports a user")
	}
	_, _ = c.Status(ctx)
	_, _ = admin.Me(ctx)
	_, _ = admin.Users(ctx, UserQuery{Q: "anna"})
	_, _ = admin.Quota(ctx, uid)
	_, _ = admin.UserSettings(ctx, uid)
	_ = admin.SetPermissions(ctx, uid, PermRequest)
	_, _ = admin.PublicSettings(ctx)
	_, _ = admin.Requests(ctx, RequestQuery{Take: 50, Sort: "modified"})
	_, _ = admin.Movie(ctx, 603)
	for _, call := range f.CallLog() {
		if call.HasAPIUser || !call.HasKey {
			t.Errorf("admin call %s: X-API-User %v (%q), key %v", call.Route, call.HasAPIUser, call.APIUser, call.HasKey)
		}
	}

	f.ResetCalls()
	user := c.AsUser(uid)
	if id, ok := user.UserID(); !ok || id != uid {
		t.Fatalf("UserID = %d, %v", id, ok)
	}
	me, err := user.Me(ctx)
	if err != nil || me.ID != uid {
		t.Fatalf("Me as user = %+v, %v", me, err)
	}
	_, _ = user.CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603})
	_, _ = user.Movie(ctx, 603)
	_, _ = user.Trending(ctx, TrendingQuery{})
	_, _ = user.Search(ctx, SearchQuery{Query: "matrix"})
	log := f.CallLog()
	if len(log) != 5 {
		t.Fatalf("calls = %+v", log)
	}
	for _, call := range log {
		if !call.HasAPIUser || call.APIUser != fmt.Sprint(uid) || !call.HasKey {
			t.Errorf("user call %s: X-API-User %v %q, key %v", call.Route, call.HasAPIUser, call.APIUser, call.HasKey)
		}
	}

	// An invalid id sends nothing.
	f.ResetCalls()
	if _, err := c.AsUser(0).Me(ctx); !errors.Is(err, ErrInvalid) {
		t.Errorf("AsUser(0): %v", err)
	}
	if _, err := (Session{}).Me(ctx); !errors.Is(err, ErrInvalid) {
		t.Errorf("zero session: %v", err)
	}
	if n := len(f.CallLog()); n != 0 {
		t.Errorf("invalid sessions made %d calls", n)
	}
}

func TestKeyNeverInErrorText(t *testing.T) {
	f := seerrtest.New(t)
	wrong, err := New(f.URL, "wrong-key-5a3f9c1d", WithLimiter(fast()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = wrong.AsAdmin().Me(context.Background())
	if !errors.Is(err, ErrKeyInvalid) || strings.Contains(err.Error(), "wrong-key-5a3f9c1d") {
		t.Errorf("wrong key: %v", err)
	}

	// Seerr echoing the key in its message: the error shows "[redacted]".
	c := newClient(t, f)
	f.SetFailure("GET /user", seerrtest.Failure{Status: 500, Message: "invalid header X-Api-Key=" + seerrtest.APIKey})
	_, err = c.AsAdmin().Users(context.Background(), UserQuery{})
	if !errors.Is(err, ErrServer) || strings.Contains(err.Error(), seerrtest.APIKey) || !strings.Contains(err.Error(), redacted) {
		t.Errorf("echoed key: %v", err)
	}

	for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprintf("%+v", c.AsUser(2))} {
		if strings.Contains(s, seerrtest.APIKey) {
			t.Errorf("printed client shows the key: %s", s)
		}
	}
}

func TestPasswordNeverInLogOrError(t *testing.T) {
	const password = "Pa55word-unique-81723"
	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	f := seerrtest.New(t)
	c := newClient(t, f, WithLogger(logger))
	ctx := context.Background()
	admin := c.AsAdmin()

	u, err := admin.CreateUser(ctx, NewUser{Email: "ben@example.org", Username: "ben", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.SetPassword(ctx, u.ID, password+"-2"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.User(u.ID); got.Password != password+"-2" {
		t.Fatalf("password in the fake = %q", got.Password)
	}
	// Failures that echo the body.
	f.SetFailure("POST /user", seerrtest.Failure{Status: 500, Message: `body {"password":"` + password + `"} refused`})
	_, err1 := admin.CreateUser(ctx, NewUser{Email: "carla@example.org", Password: password})
	f.SetFailure("POST /user/{id}/settings/password", seerrtest.Failure{Status: 400, Message: "newPassword " + password + " too weak"})
	err2 := admin.SetPassword(ctx, u.ID, password)
	for _, err := range []error{err1, err2} {
		if err == nil || strings.Contains(err.Error(), password) {
			t.Errorf("error text: %v", err)
		}
	}
	// A refused user 1 password (Seerr wants the current one).
	f.SetFailure("POST /user/{id}/settings/password", seerrtest.Failure{})
	if err := admin.SetPassword(ctx, 1, password); !errors.Is(err, ErrForbidden) || strings.Contains(err.Error(), password) {
		t.Errorf("user 1: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, password) || strings.Contains(out, seerrtest.APIKey) {
		t.Errorf("log shows a secret:\n%s", out)
	}
	if !strings.Contains(out, `"path":"/user/`+fmt.Sprint(u.ID)+`/settings/password"`) || !strings.Contains(out, `"as":"admin"`) {
		t.Errorf("log lacks the call line:\n%s", out)
	}
}

func TestUserSettingsRoundTripKeepsFields(t *testing.T) {
	f := seerrtest.New(t)
	uid := f.AddUser(seerrtest.User{
		Email: "dora@example.org", Username: "dora", Permissions: seerrtest.PermRequest, Locale: "de",
		DiscoverRegion: "CH", StreamingRegion: "CH", OriginalLanguage: "de", DiscordID: "4711",
	})
	ctx := context.Background()
	admin := newClient(t, f).AsAdmin()
	set, err := admin.UserSettings(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if set.Username == nil || *set.Username != "dora" || set.MovieQuotaLimit != nil {
		t.Fatalf("read = %+v", set)
	}
	if raw, ok := set.Field("locale"); !ok || string(raw) != `"de"` {
		t.Fatalf("locale field = %s, %v", raw, ok)
	}
	set.MovieQuotaLimit, set.MovieQuotaDays = ptr(10), ptr(7)
	set.TVQuotaLimit, set.TVQuotaDays = ptr(5), ptr(7)
	if err := admin.SetUserSettings(ctx, uid, set); err != nil {
		t.Fatal(err)
	}
	got, _ := f.User(uid)
	if got.Locale != "de" || got.DiscoverRegion != "CH" || got.StreamingRegion != "CH" || got.OriginalLanguage != "de" ||
		got.DiscordID != "4711" || got.Username != "dora" || got.Email != "dora@example.org" {
		t.Errorf("unrelated fields changed: %+v", got)
	}
	if *got.MovieQuotaLimit != 10 || *got.MovieQuotaDays != 7 || *got.TVQuotaLimit != 5 || *got.TVQuotaDays != 7 {
		t.Errorf("quota = %v %v %v %v", *got.MovieQuotaLimit, *got.MovieQuotaDays, *got.TVQuotaLimit, *got.TVQuotaDays)
	}

	// Without a read the client refuses, because the fake (like Seerr) clears what is missing.
	f.ResetCalls()
	if err := admin.SetUserSettings(ctx, uid, UserSettings{MovieQuotaLimit: ptr(1)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unread write: %v", err)
	}
	if n := f.Calls("POST /user/{id}/settings/main"); n != 0 {
		t.Errorf("unread write sent %d calls", n)
	}
	req, _ := http.NewRequest(http.MethodPost, f.URL+"/api/v1/user/"+fmt.Sprint(uid)+"/settings/main", strings.NewReader(`{"movieQuotaLimit":3}`))
	req.Header.Set("X-Api-Key", seerrtest.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got, _ := f.User(uid); got.Locale != "" || got.Username != "" {
		t.Errorf("the fake kept fields that were not sent: %+v", got)
	}

	// Quota fields of a user with MANAGE_USERS are not taken.
	mgr := localUser(f, "max@example.org", seerrtest.PermManageUsers)
	set, _ = admin.UserSettings(ctx, mgr)
	set.MovieQuotaLimit = ptr(2)
	if err := admin.SetUserSettings(ctx, mgr, set); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.User(mgr); got.MovieQuotaLimit != nil {
		t.Errorf("quota written for MANAGE_USERS: %v", *got.MovieQuotaLimit)
	}
}

func TestDVRServersKeepTheKeySecret(t *testing.T) {
	const arrKey = "radarr-key-0b9e77c2"
	f := seerrtest.New(t)
	f.Configure(func(c *seerrtest.Config) {
		c.Radarr = []seerrtest.DVR{{ID: 0, Name: "Radarr", Hostname: "radarr", Port: 7878, APIKey: arrKey, IsDefault: true, TagRequests: true}}
		c.Sonarr = []seerrtest.DVR{{ID: 0, Name: "Sonarr", Hostname: "sonarr", Port: 8989, APIKey: "sonarr-key-x", IsDefault: true}}
	})
	var buf bytes.Buffer
	logger, _ := logging.New(&buf, logging.FormatText, slog.LevelDebug)
	admin := newClient(t, f, WithLogger(logger)).AsAdmin()
	servers, err := admin.RadarrServers(context.Background())
	if err != nil || len(servers) != 1 {
		t.Fatalf("radarr = %v, %v", servers, err)
	}
	r := servers[0]
	if r.APIKey.Reveal() != arrKey || r.Hostname != "radarr" || r.Port != 7878 || !r.TagRequests || !r.IsDefault {
		t.Errorf("server = %+v", r)
	}
	js, _ := json.Marshal(r)
	logger.Info("server", "server", r, "key", r.APIKey)
	for _, s := range []string{fmt.Sprint(r), fmt.Sprintf("%+v", r), fmt.Sprintf("%#v", r), fmt.Sprintf("%s|%q|%x", r.APIKey, r.APIKey, r.APIKey),
		r.APIKey.String(), string(js), buf.String()} {
		if strings.Contains(s, arrKey) {
			t.Errorf("key shows: %s", s)
		}
	}
	if sonarr, err := admin.SonarrServers(context.Background()); err != nil || len(sonarr) != 1 || sonarr[0].APIKey.Reveal() != "sonarr-key-x" {
		t.Errorf("sonarr = %v, %v", sonarr, err)
	}
	// The settings routes need ADMIN.
	uid := localUser(f, "eva@example.org", seerrtest.PermRequest)
	if _, err := newClient(t, f).AsUser(uid).RadarrServers(context.Background()); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("as user: %v", err)
	}
}

func TestConnectionCalls(t *testing.T) {
	f := seerrtest.New(t)
	f.Configure(func(c *seerrtest.Config) { c.LocalLogin = false; c.MetadataTV = "tvdb"; c.Version = "2.7.3" })
	ctx := context.Background()
	c := newClient(t, f)
	admin := c.AsAdmin()
	st, err := c.Status(ctx)
	if err != nil || st.Version != "2.7.3" || st.CommitTag != "v2.7.3" {
		t.Errorf("status = %+v, %v", st, err)
	}
	me, err := admin.Me(ctx)
	if err != nil || me.ID != 1 || me.Email != "admin@example.org" || !me.Permissions.Has(PermManageUsers) || me.UserType != UserJellyfin || me.CreatedAt.IsZero() {
		t.Errorf("me = %+v, %v", me, err)
	}
	pub, err := admin.PublicSettings(ctx)
	if err != nil || pub.LocalLogin || !pub.PartialRequestsEnabled {
		t.Errorf("public = %+v, %v", pub, err)
	}
	md, err := admin.MetadataSettings(ctx)
	if err != nil || md.TV != "tvdb" || md.Anime != "tmdb" {
		t.Errorf("metadatas = %+v, %v", md, err)
	}
	// An unknown X-API-User (deleted user) is refused, the administrator still works.
	if _, err := c.AsUser(99).Me(ctx); !errors.Is(err, ErrAccessDenied) || errors.Is(err, ErrKeyInvalid) {
		t.Errorf("unknown user: %v", err)
	}
	// CSRF probe: off, then on.
	if on, err := admin.CSRFProtection(ctx); err != nil || on {
		t.Errorf("csrf off: %v, %v", on, err)
	}
	f.SetCSRF(true)
	if on, err := admin.CSRFProtection(ctx); err != nil || !on {
		t.Errorf("csrf on: %v, %v", on, err)
	}
	if err := admin.SetPermissions(ctx, 1, PermAdmin); !errors.Is(err, ErrCSRF) {
		t.Errorf("write with csrf on: %v", err)
	}
	if len(f.Requests()) != 0 {
		t.Error("the csrf probe created a request")
	}
}

func TestUsers(t *testing.T) {
	f := seerrtest.New(t)
	f.AddUser(seerrtest.User{Email: "jelly@example.org", JellyfinUsername: "jelly", UserType: seerrtest.UserJellyfin, Permissions: seerrtest.PermRequest})
	ctx := context.Background()
	admin := newClient(t, f).AsAdmin()

	u, err := admin.CreateUser(ctx, NewUser{Email: "Finn@Example.org", Username: "finn", Password: "long-enough-1"})
	if err != nil || u.ID == 0 || u.UserType != UserLocal || u.Permissions != PermRequest {
		t.Fatalf("create = %+v, %v", u, err)
	}
	if _, err := admin.CreateUser(ctx, NewUser{Email: "finn@example.org", Password: "long-enough-1"}); !errors.Is(err, ErrUserExists) || !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate email: %v", err)
	}
	if _, err := admin.CreateUser(ctx, NewUser{Email: "x@example.org"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no password: %v", err)
	}
	page, err := admin.Users(ctx, UserQuery{Q: "finn@example.org", Take: 100})
	if err != nil || page.PageInfo.Results != 1 || len(page.Results) != 1 || page.Results[0].ID != u.ID {
		t.Errorf("search = %+v, %v", page, err)
	}
	if call := f.CallLog()[len(f.CallLog())-1]; call.Query.Get("q") != "finn@example.org" || call.Query.Get("take") != "100" || call.Query.Has("skip") {
		t.Errorf("query = %v", call.Query)
	}
	all, err := admin.Users(ctx, UserQuery{Take: 2, Skip: 1})
	if err != nil || all.PageInfo.Results != 3 || len(all.Results) != 2 || all.Results[0].JellyfinUsername != "jelly" {
		t.Errorf("page = %+v, %v", all, err)
	}
	got, err := admin.User(ctx, u.ID)
	if err != nil || got.Email != "finn@example.org" {
		t.Errorf("get = %+v, %v", got, err)
	}
	if _, err := admin.User(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: %v", err)
	}

	// Passwords: the administrator sets another user's, not user 1's, not another administrator's
	// unless it is user 1 itself (the api key acts as user 1).
	if err := admin.SetPassword(ctx, u.ID, "new-password-2"); err != nil {
		t.Errorf("set password: %v", err)
	}
	if err := admin.SetPassword(ctx, 1, "new-password-2"); !errors.Is(err, ErrForbidden) {
		t.Errorf("user 1 password: %v", err)
	}
	if err := admin.SetPassword(ctx, u.ID, "short"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("short password: %v", err)
	}
	other := f.AddUser(seerrtest.User{Email: "boss@example.org", Permissions: seerrtest.PermAdmin, Password: "boss-password"})
	if err := admin.SetPassword(ctx, other, "new-password-3"); err != nil {
		t.Errorf("user 1 sets an administrator's password: %v", err)
	}
	mgr := localUser(f, "mgr@example.org", seerrtest.PermManageUsers)
	if err := newClient(t, f).AsUser(mgr).SetPassword(ctx, other, "new-password-4"); !errors.Is(err, ErrForbidden) {
		t.Errorf("manager sets an administrator's password: %v", err)
	}

	if err := admin.SetPermissions(ctx, u.ID, PermRequest|PermAutoApprove); err != nil {
		t.Error(err)
	}
	if fu, _ := f.User(u.ID); fu.Permissions != uint64(PermRequest|PermAutoApprove) {
		t.Errorf("permissions = %d", fu.Permissions)
	}
	if err := admin.SetPermissions(ctx, u.ID, PermNone); err != nil {
		t.Error(err)
	}

	if err := admin.DeleteUser(ctx, 1); err == nil {
		t.Error("user 1 deleted")
	}
	if err := admin.DeleteUser(ctx, u.ID); err != nil {
		t.Error(err)
	}
	if _, ok := f.User(u.ID); ok {
		t.Error("user still there")
	}
}

func TestQuotaRollingWindow(t *testing.T) {
	f := seerrtest.New(t)
	for i := range 4 {
		f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 100 + i, Name: fmt.Sprint("Film ", i)})
	}
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 200, TVDBID: 2000, Name: "Serie", Seasons: []seerrtest.Season{{Number: 0, Episodes: 3}, {Number: 1, Episodes: 10}, {Number: 2, Episodes: 10}, {Number: 3, Episodes: 8}}})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 201, TVDBID: 2001, Name: "Serie 2", Seasons: []seerrtest.Season{{Number: 1, Episodes: 6}}})
	uid := f.AddUser(seerrtest.User{Email: "gina@example.org", Permissions: seerrtest.PermRequest,
		MovieQuotaLimit: ptr(2), MovieQuotaDays: ptr(7), TVQuotaLimit: ptr(3), TVQuotaDays: ptr(7)})
	ctx := context.Background()
	c := newClient(t, f)
	admin, user := c.AsAdmin(), c.AsUser(uid)

	movie := func(id int) error {
		_, err := user.CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: id})
		return err
	}
	if err := movie(100); err != nil {
		t.Fatal(err)
	}
	if err := movie(101); err != nil {
		t.Fatal(err)
	}
	if err := movie(102); !errors.Is(err, ErrQuota) {
		t.Fatalf("third movie: %v", err)
	}
	q, err := admin.Quota(ctx, uid)
	if err != nil || q.Movie.Used != 2 || *q.Movie.Limit != 2 || *q.Movie.Remaining != 0 || !q.Movie.Restricted {
		t.Fatalf("quota = %+v, %v", q, err)
	}
	// A declined request does not count.
	if _, err := admin.DeclineRequest(ctx, f.Requests()[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := movie(102); err != nil {
		t.Fatalf("after decline: %v", err)
	}
	// Series count seasons; season 0 is dropped without special episodes.
	if _, err := user.CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 200, TVDBID: 2000, Seasons: []int{0, 1, 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := user.CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 201, TVDBID: 2001, Seasons: []int{1}}); err != nil {
		t.Fatal(err)
	}
	q, _ = admin.Quota(ctx, uid)
	if q.TV.Used != 3 || !q.TV.Restricted {
		t.Fatalf("tv quota = %+v", q.TV)
	}
	if _, err := user.CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 200, TVDBID: 2000, Seasons: []int{3}}); !errors.Is(err, ErrQuota) {
		t.Errorf("fourth season: %v", err)
	}
	// The window rolls.
	f.Advance(8 * 24 * time.Hour)
	q, _ = admin.Quota(ctx, uid)
	if q.Movie.Used != 0 || q.TV.Used != 0 || q.Movie.Restricted {
		t.Errorf("after 8 days = %+v", q)
	}
	if err := movie(103); err != nil {
		t.Errorf("after 8 days: %v", err)
	}
	// MANAGE_USERS has no limit.
	mgr := f.AddUser(seerrtest.User{Email: "hans@example.org", Permissions: seerrtest.PermRequest | seerrtest.PermManageUsers, MovieQuotaLimit: ptr(1), MovieQuotaDays: ptr(7)})
	q, _ = admin.Quota(ctx, mgr)
	if (q.Movie.Limit != nil && *q.Movie.Limit != 0) || q.Movie.Restricted {
		t.Errorf("manager quota = %+v", q.Movie)
	}
	page, err := admin.UserRequests(ctx, uid, 10, 0)
	if err != nil || page.PageInfo.Results != 6 || page.Results[0].ID < page.Results[1].ID {
		t.Errorf("user requests = %+v, %v", page.PageInfo, err)
	}
}

func TestRequests(t *testing.T) {
	f := seerrtest.New(t)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, IMDBID: "tt0133093", Name: "The Matrix", Year: 1999})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 604, Name: "Reloaded", Year: 2003})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 605, Name: "Blocked"})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 1399, TVDBID: 121361, Name: "Thrones", Seasons: []seerrtest.Season{{Number: 1, Episodes: 10}, {Number: 2, Episodes: 10}, {Number: 3, Episodes: 10}}})
	f.SetMediaStatus("movie", 605, seerrtest.MediaBlocklisted, nil)
	kid := localUser(f, "kid@example.org", seerrtest.PermRequest)
	parent := localUser(f, "parent@example.org", seerrtest.PermRequest|seerrtest.PermAutoApprove)
	none := localUser(f, "none@example.org", 0)
	ctx := context.Background()
	c := newClient(t, f)
	admin := c.AsAdmin()

	// The api key alone never makes a request.
	if _, err := admin.CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603}); !errors.Is(err, ErrInvalid) {
		t.Errorf("admin request: %v", err)
	}
	for _, bad := range []NewRequest{{MediaType: MediaTypeTV, TMDBID: 1399, Seasons: []int{1}}, {MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 1}, {MediaType: "music", TMDBID: 1}, {MediaType: MediaTypeMovie}} {
		if _, err := c.AsUser(kid).CreateRequest(ctx, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	if n := f.Calls("POST /request"); n != 0 {
		t.Fatalf("refused requests sent %d calls", n)
	}

	// Auto-approval follows the user of X-API-User.
	r1, err := c.AsUser(kid).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603})
	if err != nil || r1.Status != RequestPending || r1.Media == nil || r1.Media.Status != MediaPending || r1.RequestedBy.ID != kid {
		t.Fatalf("kid request = %+v, %v", r1, err)
	}
	r2, err := c.AsUser(parent).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 604})
	if err != nil || r2.Status != RequestApproved || r2.Media.Status != MediaProcessing {
		t.Fatalf("parent request = %+v, %v", r2, err)
	}
	for _, call := range f.CallLog() {
		if call.Route == "POST /request" && strings.Contains(strings.Join(call.BodyKeys, ","), "userId") {
			t.Errorf("body carries userId: %v", call.BodyKeys)
		}
	}
	if _, err := c.AsUser(parent).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603}); !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := c.AsUser(kid).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 605}); !errors.Is(err, ErrBlocklisted) {
		t.Errorf("blocklisted: %v", err)
	}
	if _, err := c.AsUser(none).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603}); !errors.Is(err, ErrNoPermission) {
		t.Errorf("no right: %v", err)
	}
	if _, err := c.AsUser(none).CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 121361, Seasons: []int{1}}); !errors.Is(err, ErrNoPermission) {
		t.Errorf("no series right: %v", err)
	}

	// Series: tvdbId goes along; seasons already requested drop out; nothing left is 202.
	r3, err := c.AsUser(kid).CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 121361, Seasons: []int{1, 2}})
	if err != nil || len(r3.Seasons) != 2 || r3.Media.TVDBID == nil || *r3.Media.TVDBID != 121361 {
		t.Fatalf("series = %+v, %v", r3, err)
	}
	if _, err := c.AsUser(parent).CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 121361, Seasons: []int{1, 2}}); !errors.Is(err, ErrNoSeasons) {
		t.Errorf("no new seasons: %v", err)
	}
	r4, err := c.AsUser(parent).CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 121361, Seasons: []int{2, 3}})
	if err != nil || len(r4.Seasons) != 1 || r4.Seasons[0].SeasonNumber != 3 {
		t.Errorf("further season = %+v, %v", r4, err)
	}

	// Read, list, approve, decline, delete.
	got, err := admin.Request(ctx, r1.ID)
	if err != nil || got.ID != r1.ID || got.Media.TMDBID != 603 {
		t.Errorf("get = %+v, %v", got, err)
	}
	if _, err := admin.Request(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing request: %v", err)
	}
	f.Advance(time.Minute)
	if _, err := admin.ApproveRequest(ctx, r1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ApproveRequest(ctx, r1.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("approve twice: %v", err)
	}
	if _, err := c.AsUser(kid).DeclineRequest(ctx, r3.ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("kid declines: %v", err)
	}
	dec, err := admin.DeclineRequest(ctx, r3.ID)
	if err != nil || dec.Status != RequestDeclined {
		t.Errorf("decline = %+v, %v", dec, err)
	}
	page, err := admin.Requests(ctx, RequestQuery{Take: 50, Sort: "modified", SortDirection: "desc"})
	if err != nil || page.PageInfo.Results != 4 || page.Results[0].ID != r3.ID {
		t.Errorf("list = %+v, %v", page, err)
	}
	if call := f.CallLog()[len(f.CallLog())-1]; call.Query.Get("sort") != "modified" || call.Query.Get("take") != "50" {
		t.Errorf("list query = %v", call.Query)
	}
	if err := c.AsUser(kid).DeleteRequest(ctx, r1.ID); !errors.Is(err, ErrNoPermission) { // approved: 401
		t.Errorf("delete approved: %v", err)
	}
	r5, _ := c.AsUser(kid).CreateRequest(ctx, NewRequest{MediaType: MediaTypeTV, TMDBID: 1399, TVDBID: 121361, Seasons: []int{1}})
	if err := c.AsUser(kid).DeleteRequest(ctx, r5.ID); err != nil {
		t.Errorf("delete own pending: %v", err)
	}
	if err := c.AsUser(kid).DeleteRequest(ctx, r5.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}

	// Titles.
	m, err := c.AsUser(kid).Movie(ctx, 603)
	if err != nil || m.Title != "The Matrix" || m.MediaInfo == nil || len(m.MediaInfo.Requests) != 1 || *m.ExternalIDs.IMDBID != "tt0133093" {
		t.Errorf("movie = %+v, %v", m, err)
	}
	tv, err := admin.TV(ctx, 1399)
	if err != nil || len(tv.Seasons) != 3 || tv.Seasons[2].EpisodeCount != 10 || *tv.ExternalIDs.TVDBID != 121361 || tv.MediaInfo == nil {
		t.Errorf("tv = %+v, %v", tv, err)
	}
	if _, err := admin.Movie(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown movie: %v", err)
	}
}

func TestDiscoverAndSearch(t *testing.T) {
	f := seerrtest.New(t)
	for i := range 25 {
		f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 1000 + i, Name: fmt.Sprint("Movie ", i), Popularity: float64(i), GenreIDs: []int{28}, Certification: "12", Year: 2020})
	}
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 2000, Name: "Comedy Film", GenreIDs: []int{35}, Certification: "16", Upcoming: true, Year: 2027})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 3000, TVDBID: 30000, IMDBID: "tt3000", Name: "Space Show", GenreIDs: []int{10765}, Upcoming: true})
	f.AddTitle(seerrtest.Title{MediaType: "person", TMDBID: 4000, Name: "Space Person"})
	uid := localUser(f, "ida@example.org", seerrtest.PermRequest)
	ctx := context.Background()
	user := newClient(t, f).AsUser(uid)

	tr, err := user.Trending(ctx, TrendingQuery{PageQuery: PageQuery{Language: "de"}, MediaType: "all", TimeWindow: "week"})
	if err != nil || tr.TotalResults != 27 || tr.TotalPages != 2 || len(tr.Results) != 20 {
		t.Errorf("trending = %d %d %d, %v", tr.TotalResults, tr.TotalPages, len(tr.Results), err)
	}
	p2, err := user.DiscoverMovies(ctx, DiscoverQuery{Page: 2, SortBy: "popularity.desc", Genre: "28", CertificationLte: "12", CertificationCountry: "CH"})
	if err != nil || p2.Page != 2 || p2.TotalResults != 25 || len(p2.Results) != 5 || p2.Results[0].MediaType != "movie" {
		t.Errorf("movies page 2 = %+v, %v", p2, err)
	}
	if lte, _ := user.DiscoverMovies(ctx, DiscoverQuery{CertificationLte: "12", CertificationCountry: "CH"}); lte.TotalResults != 25 {
		t.Errorf("certificationLte = %d", lte.TotalResults)
	}
	if up, err := user.UpcomingMovies(ctx, PageQuery{}); err != nil || up.TotalResults != 1 || up.Results[0].Title != "Comedy Film" {
		t.Errorf("upcoming = %+v, %v", up, err)
	}
	if tv, err := user.DiscoverTV(ctx, DiscoverQuery{Genre: "10765"}); err != nil || tv.TotalResults != 1 || tv.Results[0].Name != "Space Show" {
		t.Errorf("tv = %+v, %v", tv, err)
	}
	if up, err := user.UpcomingTV(ctx, PageQuery{Language: "en"}); err != nil || up.TotalResults != 1 {
		t.Errorf("upcoming tv = %+v, %v", up, err)
	}
	if g, err := user.MoviesByGenre(ctx, 35, PageQuery{Page: 1}); err != nil || g.TotalResults != 1 {
		t.Errorf("by genre = %+v, %v", g, err)
	}
	if g, err := user.TVByGenre(ctx, 10765, PageQuery{}); err != nil || g.TotalResults != 1 {
		t.Errorf("tv by genre = %+v, %v", g, err)
	}
	if gs, err := user.MovieGenres(ctx, "de"); err != nil || len(gs) != 2 || gs[0].ID != 28 || gs[0].Name != "Action" || len(gs[0].Backdrops) != 4 {
		t.Errorf("movie genres = %+v, %v", gs, err)
	}
	if gs, err := user.TVGenres(ctx, ""); err != nil || len(gs) != 1 {
		t.Errorf("tv genres = %+v, %v", gs, err)
	}
	for q, want := range map[string]int{"space": 2, "tvdb:30000": 3000, "tmdb:2000": 2000, "imdb:tt3000": 3000, "year:2027 comedy": 2000, "Movie 1": 11} {
		res, err := user.Search(ctx, SearchQuery{Query: q})
		switch {
		case err != nil:
			t.Errorf("search %q: %v", q, err)
		case want >= 1000 && (res.TotalResults != 1 || res.Results[0].ID != want):
			t.Errorf("search %q = %+v", q, res.Results)
		case want < 1000 && res.TotalResults != want:
			t.Errorf("search %q: %d results, want %d", q, res.TotalResults, want)
		}
	}
	if _, err := user.Search(ctx, SearchQuery{Query: " "}); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty search: %v", err)
	}
	// Every call above passed the validator of the fake (unknown parameters answer 400), and the
	// search text went as typed, space as %20.
	for _, call := range f.CallLog() {
		if call.Route == "GET /search" && call.Query.Get("query") == "year:2027 comedy" {
			return
		}
	}
	t.Error("search text did not arrive as typed")
}

func TestOversizedAnswer(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte(" "), MaxResponseBytes+10))
	}))
	defer big.Close()
	c, err := New(big.URL, "key", WithLimiter(fast()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Status(context.Background()); !errors.Is(err, ErrServer) {
		t.Errorf("err = %v", err)
	}
}

// The observer sees every finished call with method, path without query, status, duration and
// error; a password of the body never reaches it.
func TestObserverSeesEveryCall(t *testing.T) {
	f := seerrtest.New(t)
	var mu sync.Mutex
	var calls []Call
	c := newClient(t, f, WithObserver(func(_ context.Context, call Call) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, call)
	}))
	ctx := context.Background()
	if _, err := c.AsAdmin().Users(ctx, UserQuery{Q: "secret-query", Take: 5}); err != nil {
		t.Fatal(err)
	}
	f.SetFailure("POST /user/{id}/settings/password", seerrtest.Failure{Status: 500, Message: "boom pw-FAKE-0815"})
	err := c.AsAdmin().SetPassword(ctx, 1, "pw-FAKE-0815")
	if err == nil {
		t.Fatal("the failure was not reported")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("calls = %+v", calls)
	}
	if g := calls[0]; g.Method != http.MethodGet || g.Path != "/user" || g.Status != http.StatusOK || g.Err != nil || g.Duration < 0 {
		t.Errorf("first call = %+v", g)
	}
	if g := calls[1]; g.Path != "/user/1/settings/password" || g.Status != 500 || !errors.Is(g.Err, ErrServer) ||
		strings.Contains(g.Err.Error(), "pw-FAKE-0815") {
		t.Errorf("second call = %+v", g)
	}
}

// The fake and the client answer as the real Seerr 3.5.1 did (appendix I).
func TestAnswersOfSeerr351(t *testing.T) {
	f := seerrtest.New(t)
	f.Configure(func(c *seerrtest.Config) {
		c.DefaultPermissions = seerrtest.OwnerDefaultPermissions
		c.DefaultMovieQuotaLimit, c.DefaultMovieQuotaDays = 3, 14
		c.MediaServerType = 3
	})
	ctx := context.Background()
	c := newClient(t, f)
	admin := c.AsAdmin()

	// I-12, I-14: 201 without the email, permissions from defaultPermissions; 409 in any spelling.
	u, err := admin.CreateUser(ctx, NewUser{Email: "nora@example.org", Username: "nora", Password: "long-enough-1"})
	if err != nil || u.ID == 0 || u.Email != "" || u.Permissions != Permission(seerrtest.OwnerDefaultPermissions) {
		t.Fatalf("create = %+v, %v", u, err)
	}
	if _, err := admin.CreateUser(ctx, NewUser{Email: "NORA@Example.ORG", Password: "long-enough-1"}); !errors.Is(err, ErrUserExists) {
		t.Errorf("duplicate in another spelling: %v", err)
	}
	// I-7: the defaults of a new user.
	ms, err := admin.MainSettings(ctx)
	if err != nil || ms.DefaultPermissions != Permission(seerrtest.OwnerDefaultPermissions) ||
		ms.DefaultQuotas.Movie.QuotaLimit == nil || *ms.DefaultQuotas.Movie.QuotaLimit != 3 || *ms.DefaultQuotas.Movie.QuotaDays != 14 ||
		ms.DefaultQuotas.TV.QuotaLimit == nil || *ms.DefaultQuotas.TV.QuotaLimit != 0 {
		t.Errorf("main settings = %+v, %v", ms, err)
	}
	if _, err := c.AsUser(u.ID).MainSettings(ctx); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("main settings as a user: %v", err)
	}
	// I-15, I-16, I-18: settings/main of another user's email is INVALID_EMAIL, nothing changes.
	other := localUser(f, "other@example.org", seerrtest.PermRequest)
	set, err := admin.UserSettings(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Field("globalMovieQuotaLimit"); !ok {
		t.Error("settings/main without the global quota fields")
	}
	set.Email = ptr("Other@example.org")
	set.MovieQuotaLimit = ptr(5)
	if err := admin.SetUserSettings(ctx, u.ID, set); !errors.Is(err, ErrInvalidEmail) || !errors.Is(err, ErrBadRequest) {
		t.Errorf("taken email: %v", err)
	}
	if fu, _ := f.User(u.ID); fu.Email != "nora@example.org" || fu.MovieQuotaLimit != nil {
		t.Errorf("user changed after INVALID_EMAIL: %+v", fu)
	}
	set.Email = ptr("nora.new@example.org")
	if err := admin.SetUserSettings(ctx, u.ID, set); err != nil {
		t.Errorf("own new email: %v", err)
	}
	// I-20: the password route answers 204, too short 400.
	if err := admin.SetPassword(ctx, u.ID, "another-password"); err != nil {
		t.Errorf("password: %v", err)
	}
	if err := admin.SetPassword(ctx, u.ID, "short"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("short password: %v", err)
	}
	// I-31, I-32: limit 0 has no remaining; null follows defaultQuotas.
	f.UpdateUser(u.ID, func(fu *seerrtest.User) { fu.MovieQuotaLimit, fu.TVQuotaLimit = nil, ptr(0) })
	q, err := admin.Quota(ctx, u.ID)
	if err != nil || q.Movie.Limit == nil || *q.Movie.Limit != 3 || *q.Movie.Days != 14 || q.Movie.Remaining == nil || *q.Movie.Remaining != 3 ||
		q.TV.Limit == nil || *q.TV.Limit != 0 || q.TV.Remaining != nil || q.TV.Restricted {
		t.Errorf("quota = %+v, %v", q, err)
	}
	// I-2: a wrong key is 403 with the text in the field error, no key 401.
	wrong, err := New(f.URL, "wrong-key", WithLimiter(fast()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = wrong.AsAdmin().Me(ctx)
	var se *Error
	if !errors.As(err, &se) || !errors.Is(err, ErrKeyInvalid) || se.Status != http.StatusForbidden || se.Message != seerrtest.TextAccessEndpoint {
		t.Errorf("wrong key: %#v", err)
	}
	resp, err := http.Get(f.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || body.Message != seerrtest.TextNoSession {
		t.Errorf("no key: %d %q", resp.StatusCode, body.Message)
	}
	// I-10, I-11: users of every type, with their type.
	f.AddUser(seerrtest.User{ID: 20, Email: "plex@example.org", PlexUsername: "plexer", UserType: seerrtest.UserPlex})
	f.AddUser(seerrtest.User{ID: 21, Email: "jelly@example.org", JellyfinUsername: "jelly", UserType: seerrtest.UserJellyfin})
	f.AddUser(seerrtest.User{ID: 22, Email: "emby@example.org", JellyfinUsername: "emby", UserType: seerrtest.UserEmby})
	page, err := admin.Users(ctx, UserQuery{Take: 100})
	if err != nil {
		t.Fatal(err)
	}
	types := map[int]UserType{}
	for _, pu := range page.Results {
		types[pu.ID] = pu.UserType
	}
	if types[20] != UserPlex || types[21] != UserJellyfin || types[22] != UserEmby || types[u.ID] != UserLocal || types[other] != UserLocal {
		t.Errorf("types = %v", types)
	}
	if pu, _ := admin.User(ctx, 20); pu.PlexUsername != "plexer" {
		t.Errorf("plex user = %+v", pu)
	}
	ps, err := admin.PublicSettings(ctx)
	if err != nil || ps.MediaServerType != 3 {
		t.Errorf("public settings = %+v, %v", ps, err)
	}
	// I-22: DELETE /user/{id} answers 200, the user is gone.
	if err := admin.DeleteUser(ctx, other); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := admin.User(ctx, other); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted user: %v", err)
	}
}

// N-114: a refused call on behalf of a user is told apart by a second call as the administrator.
func TestExplainRefusal(t *testing.T) {
	f := seerrtest.New(t)
	ctx := context.Background()
	c := newClient(t, f)
	weak := localUser(f, "weak@example.org", 0)

	// I-3: an unknown X-API-User answers as a wrong key; the administrator finds no such user.
	_, err := c.AsUser(999).Me(ctx)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("unknown user: %v", err)
	}
	if got := c.AsUser(999).ExplainRefusal(ctx, err); !errors.Is(got, ErrUserGone) || !errors.Is(got, ErrNotFound) || errors.Is(got, ErrAccessDenied) {
		t.Errorf("unknown user explained = %v", got)
	}
	// I-23: a user without the right gets the same 403; the user exists, the refusal stays.
	_, err = c.AsUser(weak).RadarrServers(ctx)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("no right: %v", err)
	}
	if got := c.AsUser(weak).ExplainRefusal(ctx, err); !errors.Is(got, ErrAccessDenied) || errors.Is(got, ErrUserGone) {
		t.Errorf("no right explained = %v", got)
	}
	// A wrong key fails the second call too.
	wrong, err := New(f.URL, "wrong-key", WithLimiter(fast()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = wrong.AsUser(weak).Me(ctx)
	if got := wrong.AsUser(weak).ExplainRefusal(ctx, err); !errors.Is(got, ErrKeyInvalid) || errors.Is(got, ErrUserGone) {
		t.Errorf("wrong key explained = %v", got)
	}
	// Other errors and the administrator session come back unchanged.
	if got := c.AsAdmin().ExplainRefusal(ctx, err); !errors.Is(got, ErrAccessDenied) || errors.Is(got, ErrKeyInvalid) {
		t.Errorf("admin session = %v", got)
	}
	if got := c.AsUser(weak).ExplainRefusal(ctx, ErrQuota); !errors.Is(got, ErrQuota) {
		t.Errorf("other error = %v", got)
	}
	if got := c.AsUser(weak).ExplainRefusal(ctx, nil); got != nil {
		t.Errorf("nil = %v", got)
	}
}

// The guard of the discover and search routes: never as the administrator, never a parameter or
// value Seerr's validator does not know; nothing leaves in either case (appendix D 2.5, I-44).
func TestDiscoverGuardRefusesBeforeSending(t *testing.T) {
	f := seerrtest.New(t)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, Name: "The Matrix", Certification: "16"})
	uid := localUser(f, "ida@example.org", seerrtest.PermRequest)
	ctx := context.Background()
	c := newClient(t, f)
	user := c.AsUser(uid)

	for name, call := range map[string]func() error{
		"trending as admin": func() error { _, err := c.AsAdmin().Trending(ctx, TrendingQuery{}); return err },
		"movies as admin":   func() error { _, err := c.AsAdmin().DiscoverMovies(ctx, DiscoverQuery{}); return err },
		"search as admin":   func() error { _, err := c.AsAdmin().Search(ctx, SearchQuery{Query: "matrix"}); return err },
		"genres as admin":   func() error { _, err := c.AsAdmin().MovieGenreList(ctx, "de"); return err },
		"slider as admin":   func() error { _, err := c.AsAdmin().TVGenres(ctx, "de"); return err },
		"unknown media":     func() error { _, err := user.Trending(ctx, TrendingQuery{MediaType: "person"}); return err },
		"unknown window":    func() error { _, err := user.Trending(ctx, TrendingQuery{TimeWindow: "month"}); return err },
		"dotted parameter": func() error {
			_, err := user.discover(ctx, routeMovies, routeMovies, url.Values{"certification.lte": {"12"}})
			return err
		},
		"filter on upcoming": func() error {
			_, err := user.discover(ctx, routeMoviesUpcoming, routeMoviesUpcoming, url.Values{"certificationLte": {"12"}})
			return err
		},
		"unknown route": func() error {
			_, err := user.discover(ctx, "/discover/keyword/1/movies", "/discover/keyword/1/movies", nil)
			return err
		},
	} {
		if err := call(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	if log := f.CallLog(); len(log) != 0 {
		t.Fatalf("calls reached Seerr: %+v", log)
	}

	// The fake answers an unknown parameter with 400 like the real validator: proof that the guard
	// stands in front of a real refusal.
	resp, err := http.Get(f.URL + "/api/v1/discover/movies?certification.lte=12")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("fake answered %d for an unknown parameter, want 400", resp.StatusCode)
	}
}

// The parameters of appendix H 3.3 arrive as sent, on behalf of the user (X-API-User), and the
// genre lists of I-45 are typed.
func TestDiscoverParametersAndUser(t *testing.T) {
	f := seerrtest.New(t)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, Name: "The Matrix", Year: 1999, GenreIDs: []int{28}, Certification: "16"})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 604, Name: "Loud Film", Year: 2003, GenreIDs: []int{28}, Certification: "18", IgnoresCertification: true})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 1399, Name: "Game of Thrones", Year: 2011, GenreIDs: []int{18}, Certification: "16"})
	uid := localUser(f, "ida@example.org", seerrtest.PermRequest)
	ctx := context.Background()
	user := newClient(t, f).AsUser(uid)

	m, err := user.DiscoverMovies(ctx, DiscoverQuery{Page: 1, SortBy: "popularity.desc",
		Genre: "28", CertificationLte: "12", CertificationCountry: "DE"})
	// The fake ignores certificationLte for 604 the way TMDB does for a title rated elsewhere.
	if err != nil || len(m.Results) != 1 || m.Results[0].ID != 604 {
		t.Fatalf("movies = %+v, %v", m, err)
	}
	hit := m.Results[0]
	if hit.MediaType != MediaTypeMovie || hit.DisplayName() != "Loud Film" || hit.Year() != 2003 || hit.PosterPath == "" {
		t.Errorf("movie hit = %+v (%q, %d)", hit, hit.DisplayName(), hit.Year())
	}
	tv, err := user.DiscoverTV(ctx, DiscoverQuery{SortBy: "first_air_date.desc"})
	if err != nil || len(tv.Results) != 1 || tv.Results[0].DisplayName() != "Game of Thrones" || tv.Results[0].Year() != 2011 {
		t.Errorf("tv = %+v, %v", tv, err)
	}
	if g, err := user.MovieGenreList(ctx, "de"); err != nil || len(g) != 1 || g[0].ID != 28 || g[0].Name != "Action" {
		t.Errorf("movie genre list = %+v, %v", g, err)
	}
	if g, err := user.TVGenreList(ctx, ""); err != nil || len(g) != 1 || g[0].ID != 18 {
		t.Errorf("tv genre list = %+v, %v", g, err)
	}

	want := map[string]url.Values{
		"GET /discover/movies": {"page": {"1"}, "sortBy": {"popularity.desc"}, "genre": {"28"},
			"certificationLte": {"12"}, "certificationCountry": {"DE"}},
		"GET /discover/tv":  {"sortBy": {"first_air_date.desc"}},
		"GET /genres/movie": {"language": {"de"}},
		"GET /genres/tv":    {},
	}
	log := f.CallLog()
	if len(log) != len(want) {
		t.Fatalf("calls = %+v", log)
	}
	for _, call := range log {
		if !call.HasAPIUser || call.APIUser != strconv.Itoa(uid) || !call.HasKey {
			t.Errorf("%s: X-API-User %v %q, key %v", call.Route, call.HasAPIUser, call.APIUser, call.HasKey)
		}
		if w, ok := want[call.Route]; !ok || !maps.EqualFunc(call.Query, w, slices.Equal) {
			t.Errorf("%s: query %v, want %v", call.Route, call.Query, w)
		}
	}
}

// The search by id: prefix without a blank, case-insensitive (I-42), and the hits filtered by
// media type and id, since tmdb:603 also finds the series and the person with that id.
func TestDiscoverSearchByID(t *testing.T) {
	f := seerrtest.New(t)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, IMDBID: "tt0133093", Name: "The Matrix", Year: 1999})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 603, TVDBID: 70000, Name: "Some Show", Year: 2001})
	f.AddTitle(seerrtest.Title{MediaType: "person", TMDBID: 603, Name: "Some Person"})
	f.AddTitle(seerrtest.Title{MediaType: "tv", TMDBID: 1399, TVDBID: 121361, Name: "Game of Thrones"})
	uid := localUser(f, "ida@example.org", seerrtest.PermRequest)
	ctx := context.Background()
	user := newClient(t, f).AsUser(uid)

	for text, want := range map[string]struct {
		prefix IDPrefix
		id     string
	}{
		"tmdb:603": {PrefixTMDB, "603"}, "TMDB: 603": {PrefixTMDB, "603"}, " Tvdb:121361 ": {PrefixTVDB, "121361"},
		"imdb:TT0133093": {PrefixIMDB, "tt0133093"}, "tmdb:0603": {PrefixTMDB, "603"},
	} {
		if p, id, ok := ParseIDSearch(text); !ok || p != want.prefix || id != want.id {
			t.Errorf("ParseIDSearch(%q) = %q %q %v", text, p, id, ok)
		}
	}
	for _, text := range []string{"matrix", "year:1999", "tmdb:", "tmdb:abc", "imdb:0133093", "tvdb:-1", "tmdb:0", "the: matrix"} {
		if _, _, ok := ParseIDSearch(text); ok {
			t.Errorf("ParseIDSearch(%q) accepted", text)
		}
	}

	// Raw: Seerr finds film, series and person for tmdb:603, nothing with a blank.
	if raw, err := user.Search(ctx, SearchQuery{Query: "tmdb:603"}); err != nil || raw.TotalResults != 3 {
		t.Errorf("raw tmdb:603 = %+v, %v", raw, err)
	}
	if raw, err := user.Search(ctx, SearchQuery{Query: "tmdb: 603"}); err != nil || raw.TotalResults != 0 {
		t.Errorf("raw with blank = %+v, %v", raw, err)
	}
	f.ResetCalls()
	for _, c := range []struct {
		prefix    IDPrefix
		id, media string
		want      []string
	}{
		{PrefixTMDB, " 603", MediaTypeMovie, []string{"movie 603"}},
		{PrefixTMDB, "603", MediaTypeTV, []string{"tv 603"}},
		{PrefixTMDB, "603", "", []string{"movie 603", "tv 603"}},
		{PrefixTVDB, "121361", "", []string{"tv 1399"}},
		{PrefixTVDB, "121361", MediaTypeMovie, nil},
		{PrefixIMDB, "TT0133093", "", []string{"movie 603"}},
	} {
		page, err := user.SearchByID(ctx, c.prefix, c.id, c.media, PageQuery{Language: "de"})
		var got []string
		for _, r := range page.Results {
			got = append(got, fmt.Sprint(r.MediaType, " ", r.ID))
		}
		if err != nil || !slices.Equal(got, c.want) || page.TotalResults != len(c.want) {
			t.Errorf("SearchByID(%s, %q, %q) = %v (%d), %v; want %v", c.prefix, c.id, c.media, got, page.TotalResults, err, c.want)
		}
	}
	for _, call := range f.CallLog() {
		q := call.Query.Get("query")
		if strings.Contains(q, " ") || (q != "tmdb:603" && q != "tvdb:121361" && q != "imdb:tt0133093") || call.APIUser != strconv.Itoa(uid) {
			t.Errorf("search sent %q as %q", q, call.APIUser)
		}
	}
	if _, err := user.SearchByID(ctx, PrefixTMDB, "tt1", "", PageQuery{}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad id: %v", err)
	}
	if _, err := user.SearchByID(ctx, PrefixTMDB, "1", "person", PageQuery{}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad media type: %v", err)
	}
}

// The real instance (appendix I section 4a): language filters /discover/movies and /discover/tv by
// the original language and their display language is the user's locale (I-60, I-61), so the
// client never sends it there (N-115); on trending it is the display language (I-62). title.asc is
// ignored (I-63); «newest» and «rating» go with a date and a vote floor (I-64, I-65); page 501
// fails while totalPages says more, and the client never asks for it and reports at most 500
// (I-67).
func TestDiscoverAsTheRealSeerr(t *testing.T) {
	f := seerrtest.New(t)
	f.Configure(func(c *seerrtest.Config) { c.MinTotalPages = 1001 })
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 1, Name: "Old Hit", Names: map[string]string{"de": "Alter Hit"}, Released: "2001-05-01", Popularity: 90, VoteCount: 9000, VoteAverage: 7})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 2, Name: "Das Boot", OriginalLanguage: "de", Released: "2025-03-01", Popularity: 10, VoteCount: 800, VoteAverage: 8.5})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 3, Name: "Coming Soon", Released: "2028-01-01", Popularity: 50, VoteCount: 3})
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 4, Name: "A Recent One", Released: "2026-10-01", Popularity: 20, VoteCount: 60, VoteAverage: 6})
	uid := f.AddUser(seerrtest.User{Email: "ida@example.org", Username: "ida", UserType: seerrtest.UserLocal, Permissions: seerrtest.PermRequest, Locale: "en"})
	ctx := context.Background()
	user := newClient(t, f).AsUser(uid)

	raw := func(query string) []int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, f.URL+"/api/v1/discover/movies?"+query, nil)
		req.Header.Set("X-Api-Key", seerrtest.APIKey)
		req.Header.Set("X-API-User", strconv.Itoa(uid))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var p DiscoverPage
		_ = json.NewDecoder(resp.Body).Decode(&p)
		var ids []int
		for _, r := range p.Results {
			ids = append(ids, r.ID)
		}
		if resp.StatusCode != http.StatusOK {
			return []int{-resp.StatusCode}
		}
		return ids
	}
	if got := raw("language=de"); !slices.Equal(got, []int{2}) {
		t.Errorf("fake language=de = %v, want only the German production", got)
	}
	if got := raw("sortBy=title.asc"); !slices.Equal(got, []int{1, 3, 4, 2}) {
		t.Errorf("fake title.asc = %v, want the popularity order", got)
	}
	if got := raw("page=501"); !slices.Equal(got, []int{-500}) {
		t.Errorf("fake page 501 = %v, want 500", got)
	}

	pop, err := user.DiscoverMovies(ctx, DiscoverQuery{SortBy: "popularity.desc"})
	if err != nil || len(pop.Results) != 4 || pop.Results[0].DisplayName() != "Old Hit" || pop.TotalPages != MaxPage {
		t.Errorf("popular = %+v, %v; want all four in the user's locale and 500 pages", pop, err)
	}
	if tr, err := user.Trending(ctx, TrendingQuery{PageQuery: PageQuery{Language: "de"}}); err != nil || tr.Results[0].DisplayName() != "Alter Hit" {
		t.Errorf("trending in de = %+v, %v", tr, err)
	}
	newest, err := user.DiscoverMovies(ctx, DiscoverQuery{SortBy: "primary_release_date.desc", ReleasedBefore: "2026-11-17", VoteCountGte: 50})
	if ids := idsOf(newest); err != nil || !slices.Equal(ids, []int{4, 2, 1}) {
		t.Errorf("newest = %v, %v", ids, err)
	}
	rated, err := user.DiscoverMovies(ctx, DiscoverQuery{SortBy: "vote_average.desc", VoteCountGte: 500})
	if ids := idsOf(rated); err != nil || !slices.Equal(ids, []int{2, 1}) {
		t.Errorf("rating = %v, %v", ids, err)
	}
	if _, err := user.DiscoverTV(ctx, DiscoverQuery{ReleasedBefore: "2026-11-17", VoteCountGte: 20}); err != nil {
		t.Errorf("tv with date and votes: %v", err)
	}
	f.ResetCalls()
	if _, err := user.DiscoverMovies(ctx, DiscoverQuery{Page: 501}); !errors.Is(err, ErrInvalid) {
		t.Errorf("page 501: %v, want ErrInvalid", err)
	}
	if _, err := user.discover(ctx, routeTV, routeTV, url.Values{"language": {"de"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("language on /discover/tv: %v, want ErrInvalid", err)
	}
	if n := len(f.CallLog()); n != 0 {
		t.Errorf("calls reached Seerr: %d", n)
	}
	for _, c := range []struct {
		tv   bool
		want url.Values
	}{
		{false, url.Values{"primaryReleaseDateLte": {"2026-11-17"}, "voteCountGte": {"50"}, "sortBy": {"primary_release_date.desc"}}},
		{true, url.Values{"firstAirDateLte": {"2026-11-17"}, "voteCountGte": {"20"}, "sortBy": {"first_air_date.desc"}}},
	} {
		sort := "primary_release_date.desc"
		votes := 50
		if c.tv {
			sort, votes = "first_air_date.desc", 20
		}
		got := DiscoverQuery{SortBy: sort, ReleasedBefore: "2026-11-17", VoteCountGte: votes}.discoverValues(c.tv)
		if !maps.EqualFunc(got, c.want, slices.Equal) {
			t.Errorf("tv %v: %v, want %v", c.tv, got, c.want)
		}
	}
}

func idsOf(p DiscoverPage) []int {
	var ids []int
	for _, r := range p.Results {
		ids = append(ids, r.ID)
	}
	return ids
}

// POST /request/{id}/retry (concept 25, architecture 8.1): only a FAILED request, only with
// MANAGE_REQUESTS; the fake moves it to approved. Not verified against a real instance.
func TestRetryRequest(t *testing.T) {
	f := seerrtest.New(t)
	f.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, Name: "The Matrix"})
	kid := localUser(f, "kid@example.test", seerrtest.PermRequest)
	c := newClient(t, f)
	ctx := context.Background()
	r, err := c.AsUser(kid).CreateRequest(ctx, NewRequest{MediaType: MediaTypeMovie, TMDBID: 603})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.AsAdmin().RetryRequest(ctx, r.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("retry of a pending request: %v", err)
	}
	f.SetRequestStatus(r.ID, seerrtest.RequestFailed)
	if _, err := c.AsUser(kid).RetryRequest(ctx, r.ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("retry without MANAGE_REQUESTS: %v", err)
	}
	got, err := c.AsAdmin().RetryRequest(ctx, r.ID)
	if err != nil || got.ID != r.ID || got.Status != RequestApproved {
		t.Errorf("retry = %+v, %v", got, err)
	}
	if _, err := c.AsAdmin().RetryRequest(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown request: %v", err)
	}
}
