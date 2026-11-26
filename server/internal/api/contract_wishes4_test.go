package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
	"github.com/slouchcat/slouchcat/server/internal/wishes"
)

// Concept 25 stage 4 (contract 1.41.0, build step 4.7) with Seerr set up: the season catalog of a
// series on getItem (requested and in procurement from the wishes, absent without the right), the
// co-wish of a film on the wish card, the progress (viewers without message and service, the
// administration with them), the tag of a requester and the name of the Seerr user, approve and
// decline on a wish that is not pending, the settings of stage 4, the hints of the overview, the
// tag names of the mapping, the instant notice of Seerr and the check points of Radarr and Sonarr.

// wishContract is a handler with Seerr set up and the wishes on (the frame of TestContractWishes).
type wishContract struct {
	t      *testing.T
	pf     profileFixture
	f      *vistest.Fixture
	h      http.Handler
	exec   func(query string, args ...any)
	fake   *seerrtest.Server
	titles contractTitles
	now    func() time.Time // the clock of the handler (nil: time.Now)
}

// withTitles rebuilds the handler with another store of titles outside the library.
func (c *wishContract) withTitles(titles wishes.Titles) {
	log := vistest.Logger()
	o := c.pf.opts
	o.Wishes = wishes.New(c.f.DB, c.pf.admin, c.pf.queue, log, wishes.WithLibrary(visibility.New(c.f.DB, log)), wishes.WithTitles(titles))
	o.now = c.now
	c.h = NewHandler(o)
}

// keepRightsBelow18 drops the two update guards of migration 00086 from a contract database. Since
// ADR 0045 no profile and no account below 18 has the request right (TestContractRequestRightFollowsRating);
// the wish contracts keep giving it to P12, P16 and the guest (12) as before, so the age checks of
// every wish operation stay tested as the server's second line. Only the guards on writing
// can_request stand in the way of that: the triggers that move the right with a rating and the
// guards on inserts stay, because the fixtures change no rating and insert no rows with a right.
func keepRightsBelow18(exec func(query string, args ...any)) {
	// UPDATE accounts SET can_request = 1 for the guest account aged 12.
	exec(`DROP TRIGGER accounts_request_needs_no_age_limit_update`)
	// UPDATE profiles SET can_request = 1 for P0, P12 and P16.
	exec(`DROP TRIGGER profiles_request_needs_no_age_limit_update`)
}

func newWishContract(t *testing.T) wishContract {
	t.Helper()
	pf := newProfileFixture(t)
	f := pf.f
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	fake := seerrtest.New(t)
	fake.AddUser(seerrtest.User{ID: contractSharedID, Email: wishes.SharedEmail, Username: wishes.SharedUsername, Permissions: seerrtest.PermRequest})
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/admin/seerr", `{"enabled":true,"url":"` + fake.URL + `/api/v1","apiKey":"` + seerrtest.APIKey + `"}`},
		{http.MethodPost, "/admin/seerr/test", ``},
		{http.MethodPut, "/admin/wishes/settings", `{"enabled":true,"fromSearch":true,"fromDetail":true,"allowWithdraw":true}`},
	} {
		if rec := send(t, pf.h, c.method, BasePath+c.path, "p18", c.body, nil); rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	exec(`INSERT INTO seerr_users (seerr_id, display_name, email, user_type, permissions, fetched_at) VALUES (?, ?, ?, 2, 32, 1)`,
		contractSharedID, wishes.SharedUsername, wishes.SharedEmail)
	if err := pf.wishes.MarkShared(ctx, contractSharedID); err != nil {
		t.Fatal(err)
	}
	keepRightsBelow18(exec)
	exec(`UPDATE accounts SET can_request = 1`)
	exec(`UPDATE profiles SET can_request = CASE WHEN id = ? THEN 0 ELSE 1 END`, f.P6.ID)
	for _, x := range []struct {
		tmdb  int64
		age   int
		title string
	}{{tmdbMissing0, 0, "Wunsch Null"}, {tmdbMissing12, 12, "Wunsch Zwoelf"}} {
		exec(`INSERT INTO external_titles (media, tmdb_id, year, min_age, poster_path, fetched_at) VALUES ('movie', ?, 2024, ?, '/p.jpg', ?)`,
			x.tmdb, x.age, time.Now().UnixMilli())
		exec(`INSERT INTO external_title_texts (media, tmdb_id, language, title, overview) VALUES ('movie', ?, 'de', ?, 'Inhalt')`, x.tmdb, x.title)
	}
	log := vistest.Logger()
	titles := contractTitles{Service: metadata.New(f.DB, jobs.NewQueue(f.DB.Write, nil), pf.admin, t.TempDir(), transcode.Runner{}, log)}
	c := wishContract{t: t, pf: pf, f: f, exec: exec, fake: fake, titles: titles}
	c.withTitles(titles)
	return c
}

// expect sends a request and checks status and problem code; it returns the decoded JSON object.
func (c wishContract) expect(token, method, path, body string, status int, code string) map[string]any {
	c.t.Helper()
	rec := send(c.t, c.h, method, BasePath+path, token, body, nil)
	assertNoSecrets(c.t, path, rec)
	var out map[string]any
	if strings.Contains(rec.Header().Get("Content-Type"), "json") {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			c.t.Fatalf("%s %s: %v (%s)", method, path, err, rec.Body.String())
		}
	}
	if rec.Code != status || (code != "" && out["code"] != code) {
		c.t.Errorf("%s %s %s: %d %s, want %d %s", token, method, path, rec.Code, rec.Body.String(), status, code)
	}
	return out
}

func TestContractWishesStage4(t *testing.T) {
	c := newWishContract(t)
	f := c.f
	now := time.Now().UnixMilli()

	// A film wish in procurement of P18 as the shared user, with a stalled download and a failed tag.
	c.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, seerr_user_id, seerr_request_id, created_at, updated_at)
	        VALUES (7001, 'movie', ?, 'Wunsch Zwoelf', 'slouchcat', 'processing', ?, 41, 1, 2)`, tmdbMissing12, contractSharedID)
	c.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (7001, ?, ?, 'owner', 1)`, f.P18.ID, f.P18.AccountID)
	c.exec(`INSERT INTO wish_progress (wish_id, service, stage, admin_message, read_at) VALUES (7001, 'radarr', 'stalled', 'No seeders at /downloads', ?)`, now)
	c.exec(`INSERT INTO wish_tags (wish_id, profile_id, service, tag_name, state, last_error) VALUES (7001, ?, 'radarr', 'sc-p18', 'failed', 'unreachable')`, f.P18.ID)

	// Viewers: the progress without message, service or error (N-83), on «Meine Wünsche» and the card.
	list := c.expect("p18", http.MethodGet, "/wishes?state=open", "", http.StatusOK, "")
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("open wishes of p18 = %v", list)
	}
	pr, _ := items[0].(map[string]any)["progress"].(map[string]any)
	if pr == nil || pr["stage"] != "stalled" || pr["message"] != nil || pr["service"] != nil || pr["error"] != nil {
		t.Errorf("viewer progress = %v", pr)
	}
	card := c.expect("p18", http.MethodGet, fmt.Sprintf("/titles/movie/%d", tmdbMissing12), "", http.StatusOK, "")
	if w, _ := card["wish"].(map[string]any); w == nil || w["progress"] == nil || strings.Contains(fmt.Sprint(w), "No seeders") {
		t.Errorf("wish card = %v", card["wish"])
	}
	// The administration: the message, the service, the tag of the requester and the Seerr user.
	adminList := c.expect("p18", http.MethodGet, "/admin/wishes?state=open", "", http.StatusOK, "")
	aw := adminList["items"].([]any)[0].(map[string]any)
	ap, _ := aw["progress"].(map[string]any)
	if ap == nil || ap["service"] != "radarr" || ap["stage"] != "stalled" || ap["message"] == nil || ap["stale"] != false {
		t.Errorf("admin progress = %v", ap)
	}
	if aw["seerrUserName"] != wishes.SharedUsername || aw["seerrUserId"] != float64(contractSharedID) {
		t.Errorf("admin wish user = %v %v", aw["seerrUserId"], aw["seerrUserName"])
	}
	tag, _ := aw["requesters"].([]any)[0].(map[string]any)["tag"].(map[string]any)
	if tag == nil || tag["name"] != "sc-p18" || tag["state"] != "failed" || tag["lastError"] != "unreachable" || tag["service"] != "radarr" {
		t.Errorf("requester tag = %v", tag)
	}
	if strings.Contains(fmt.Sprint(aw), `"item"`) {
		t.Errorf("an ItemRef in an admin wish: %v", aw)
	}

	// Approve and decline: only a pending wish; the acting account comes from the session.
	c.expect("p18", http.MethodPost, "/admin/wishes/7001/approve", "", http.StatusConflict, "wish_not_pending")
	c.expect("p18", http.MethodPost, "/admin/wishes/7001/decline", "", http.StatusConflict, "wish_not_pending")
	c.expect("p18", http.MethodPost, "/admin/wishes/99999/approve", "", http.StatusNotFound, "not_found")
	c.expect("p16", http.MethodPost, "/admin/wishes/7001/approve", "", http.StatusForbidden, "")

	// The hints of the overview: the failed tag and the stalled download, without the message.
	var dash struct {
		Attention struct {
			Wishes *struct {
				TagsFailed       []map[string]any `json:"tagsFailed"`
				DownloadsStalled []map[string]any `json:"downloadsStalled"`
			} `json:"wishes"`
		} `json:"attention"`
	}
	rec := send(t, c.h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d %v", rec.Code, err)
	}
	if w := dash.Attention.Wishes; w == nil || len(w.TagsFailed) != 1 || w.TagsFailed[0]["error"] != "unreachable" ||
		len(w.DownloadsStalled) != 1 || w.DownloadsStalled[0]["service"] != "radarr" || strings.Contains(rec.Body.String(), "No seeders") {
		t.Errorf("dashboard wishes = %s", rec.Body.String())
	}

	// The co-wish of a film (F-7): the state of another profile's wish, never who; canJoin follows
	// «Bereits angeforderte Titel».
	c.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (7002, 'movie', ?, 'Wunsch Null', 'slouchcat', 'pending', 1, 2)`, tmdbMissing0)
	c.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (7002, ?, ?, 'owner', 1)`, f.P18.ID, f.P18.AccountID)
	shared := func(want bool) {
		t.Helper()
		out := c.expect("p12", http.MethodGet, fmt.Sprintf("/titles/movie/%d", tmdbMissing0), "", http.StatusOK, "")
		req, _ := out["request"].(map[string]any)
		sh, _ := req["shared"].(map[string]any)
		if sh == nil || sh["state"] != "pending" || sh["canJoin"] != want || len(sh) > 3 {
			t.Errorf("request.shared = %v, want canJoin %v", req, want)
		}
	}
	shared(true)
	if own := c.expect("p18", http.MethodGet, fmt.Sprintf("/titles/movie/%d", tmdbMissing0), "", http.StatusOK, ""); own["request"].(map[string]any)["shared"] != nil {
		t.Errorf("own wish carries shared: %v", own["request"])
	}

	// The settings of stage 4 and 5; a bad prefix is validation_failed on tagPrefix.
	st := c.expect("p18", http.MethodPut, "/admin/wishes/settings", `{"alreadyRequested":"status","tagPrefix":"kids-","showProgress":true}`, http.StatusOK, "")
	if st["tagPrefix"] != "kids-" || st["profileTags"] != true || st["showProgress"] != true || st["discoverPages"] == nil {
		t.Errorf("settings = %v", st)
	}
	shared(false)
	if bad := c.expect("p18", http.MethodPut, "/admin/wishes/settings", `{"tagPrefix":"Ä!"}`, http.StatusUnprocessableEntity, "validation_failed"); !strings.Contains(fmt.Sprint(bad["errors"]), "tagPrefix") {
		t.Errorf("bad prefix = %v", bad)
	}
	c.expect("p18", http.MethodPut, "/admin/wishes/settings", `{"alreadyRequested":"join"}`, http.StatusOK, "")

	// The season catalog of the series rated 12: present, missing, in procurement, announced.
	// The series of vistest carries the TMDB id 1020 (its fixture index).
	c.exec(`INSERT INTO series_catalog (item_id, tmdb_id, status, in_production, seasons, fetched_at) VALUES (?, 1020, 'Returning Series', 1, ?, ?)`,
		f.Show.Series.ID, `[{"number":1,"episodeCount":4,"airDate":"2019-01-01"},{"number":2,"episodeCount":8,"airDate":"2020-01-01"},`+
			`{"number":3,"episodeCount":8,"airDate":"2021-01-01"},{"number":4,"episodeCount":0,"airDate":"2099-01-01"}]`, now)
	c.exec(`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (7003, 'show', 1020, 77, 'Show', 'slouchcat', 'processing', 1, 2)`)
	c.exec(`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (7003, 3, 1020, 'processing')`)
	item := c.expect("p12", http.MethodGet, "/items/"+f.Show.Series.ID, "", http.StatusOK, "")
	cat, _ := item["catalog"].(map[string]any)
	if cat == nil || item["canRequestSeasons"] != true || cat["present"] != float64(1) || cat["total"] != float64(3) {
		t.Fatalf("catalog = %v, canRequestSeasons %v", cat, item["canRequestSeasons"])
	}
	var states []string
	for _, se := range cat["seasons"].([]any) {
		states = append(states, se.(map[string]any)["state"].(string))
	}
	if strings.Join(states, ",") != "present,missing,processing,announced" {
		t.Errorf("season states = %v", states)
	}
	// Every missing season held by the profile's own open wishes: nothing to request or join
	// (review Minor 3).
	c.exec(`INSERT INTO wishes (id, media, tmdb_id, tvdb_id, title, origin, state, created_at, updated_at) VALUES (7004, 'show', 1020, 77, 'Show', 'slouchcat', 'pending', 1, 2)`)
	c.exec(`INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (7004, 2, 1020, 'pending')`)
	c.exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (7003, ?, ?, 'owner', 1), (7004, ?, ?, 'owner', 1)`,
		f.P12.ID, f.P12.AccountID, f.P12.ID, f.P12.AccountID)
	if item := c.expect("p12", http.MethodGet, "/items/"+f.Show.Series.ID, "", http.StatusOK, ""); item["canRequestSeasons"] != false {
		t.Errorf("own wishes hold every missing season: canRequestSeasons %v", item["canRequestSeasons"])
	}
	c.exec(`DELETE FROM wish_requesters WHERE wish_id IN (7003, 7004)`)
	c.exec(`DELETE FROM wish_seasons WHERE wish_id = 7004`)
	c.exec(`DELETE FROM wishes WHERE id = 7004`)
	// A series wish carries its Seerr requests per season in the administration.
	c.exec(`UPDATE wish_seasons SET seerr_request_id = 88 WHERE wish_id = 7003`)
	for _, it := range c.expect("p18", http.MethodGet, "/admin/wishes?state=open", "", http.StatusOK, "")["items"].([]any) {
		if w := it.(map[string]any); w["id"] == float64(7003) {
			if se := w["seasons"].([]any); len(se) != 1 || se[0].(map[string]any)["seerrRequestId"] != float64(88) {
				t.Errorf("series wish seasons = %v", se)
			}
		}
	}
	// Without the right for series: no catalog (F-9); a film never has one.
	c.exec(`UPDATE profiles SET can_request = 0 WHERE id = ?`, f.P12.ID)
	if item := c.expect("p12", http.MethodGet, "/items/"+f.Show.Series.ID, "", http.StatusOK, ""); item["catalog"] != nil || item["canRequestSeasons"] != nil {
		t.Errorf("catalog without the right = %v", item["catalog"])
	}
	c.exec(`UPDATE profiles SET can_request = 1 WHERE id = ?`, f.P12.ID)
	if m := c.expect("p18", http.MethodGet, "/items/"+f.Rated(12).ID, "", http.StatusOK, ""); m["catalog"] != nil {
		t.Errorf("a film with a catalog: %v", m["catalog"])
	}

	// The tag names of the profiles in the mapping and setting one.
	maps := c.expect("p18", http.MethodGet, "/admin/wishes/mappings", "", http.StatusOK, "")
	found := false
	for _, a := range maps["accounts"].([]any) {
		for _, p := range a.(map[string]any)["profiles"].([]any) {
			pm := p.(map[string]any)
			if pm["profileId"] == float64(f.P12.ID) {
				found = pm["tagName"] == "kids-p12" && pm["tagNameCustom"] == false // the prefix set above
			}
		}
	}
	if !found {
		t.Errorf("mapping profiles without the tag of p12: %v", maps["accounts"])
	}
	path := fmt.Sprintf("/admin/wishes/profiles/%d/tag", f.P12.ID)
	if tg := c.expect("p18", http.MethodPut, path, `{"name":"kino-kind"}`, http.StatusOK, ""); tg["name"] != "kino-kind" || tg["custom"] != true {
		t.Errorf("set tag = %v", tg)
	}
	c.expect("p18", http.MethodPut, fmt.Sprintf("/admin/wishes/profiles/%d/tag", f.P16.ID), `{"name":"kino-kind"}`, http.StatusConflict, "tag_name_taken")
	c.expect("p18", http.MethodPut, path, `{"name":"Kino Kind!"}`, http.StatusUnprocessableEntity, "validation_failed")
	if tg := c.expect("p18", http.MethodPut, path, `{"name":null}`, http.StatusOK, ""); tg["custom"] != false {
		t.Errorf("reset tag = %v", tg)
	}

	// Radarr and Sonarr: the state without a key; an unknown service is validation_failed.
	arrSt := c.expect("p18", http.MethodGet, "/admin/seerr/arr", "", http.StatusOK, "")
	if arrSt["radarr"] == nil || arrSt["sonarr"] == nil || strings.Contains(fmt.Sprint(arrSt), "apiKey") {
		t.Errorf("arr status = %v", arrSt)
	}
	c.expect("p18", http.MethodPost, "/admin/seerr/arr/test", `{"service":"lidarr"}`, http.StatusUnprocessableEntity, "validation_failed")
	c.expect("p18", http.MethodPut, "/admin/seerr/arr", `{"radarrUrl":"http://user:pw@radarr.lan"}`, http.StatusUnprocessableEntity, "validation_failed")
	// The check points of the connection test include the two of stage 4.
	test := c.expect("p18", http.MethodPost, "/admin/seerr/test", "", http.StatusOK, "")
	keys := map[string]bool{}
	for _, ch := range test["checks"].([]any) {
		keys[ch.(map[string]any)["key"].(string)] = true
	}
	if !keys["radarr_tags"] || !keys["sonarr_tags"] {
		t.Errorf("check keys = %v", keys)
	}

	// Taking over and counting: the shared user belongs to no account (404).
	c.expect("p18", http.MethodPost, "/admin/wishes/mappings/import", fmt.Sprintf(`{"seerrUserId":%d}`, contractSharedID), http.StatusNotFound, "not_found")
	c.expect("p18", http.MethodPost, "/admin/wishes/mappings/import", `{"seerrUserId":0}`, http.StatusUnprocessableEntity, "validation_failed")
	if n := c.expect("p18", http.MethodGet, fmt.Sprintf("/admin/wishes/mappings/users/%d/requests/count", contractSharedID), "", http.StatusOK, ""); n["count"] != float64(0) {
		t.Errorf("count of the shared user = %v", n)
	}

	// The instant notice of Seerr: the setup data, the kind seerr and a tolerant payload.
	setup := c.expect("p18", http.MethodGet, "/admin/seerr/webhook", "", http.StatusOK, "")
	if !strings.Contains(fmt.Sprint(setup["jsonPayload"]), "notification_type") || setup["types"] == float64(0) {
		t.Errorf("webhook setup = %v", setup)
	}
	created := c.expect("p18", http.MethodPost, "/admin/integrations", `{"name":"Seerr","kind":"seerr"}`, http.StatusCreated, "")
	token, _ := created["token"].(string)
	if created["integration"].(map[string]any)["kind"] != "seerr" || token == "" {
		t.Fatalf("seerr integration = %v", created)
	}
	hook := send(t, c.h, http.MethodPost, BasePath+"/webhooks/"+token, "", `{"notification_type":"MEDIA_APPROVED","subject":"X",`+
		`"media":{"media_type":"movie","tmdbId":"603"},"request":{"request_id":"4"},"extra":[]}`, nil)
	if hook.Code != http.StatusOK || !strings.Contains(hook.Body.String(), `"result"`) {
		t.Errorf("seerr webhook: %d %s", hook.Code, hook.Body.String())
	}
	test2 := send(t, c.h, http.MethodPost, BasePath+"/webhooks/"+token, "", `{"notification_type":"TEST_NOTIFICATION","media":null,"extra":[]}`, nil)
	if test2.Code != http.StatusOK || !strings.Contains(test2.Body.String(), "acknowledged") {
		t.Errorf("seerr test notice: %d %s", test2.Code, test2.Body.String())
	}
}
