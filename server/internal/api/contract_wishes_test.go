package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
	"github.com/slouchcat/slouchcat/server/internal/wishes"
)

// Concept 25 stage 2 (contract 1.41.0) with Seerr set up and titles outside the library stored:
// the age cases (too high, unknown), the request right (F-9, with the exception of an own wish),
// already_available with a visible and a hidden library title, createWish 201/200/403/409, «Meine
// Wünsche» with an ItemRef only for a visible title, the images under the same rules, the search
// section and the inbox entry of a wish without a title.

// contractTitles is metadata's store of titles outside the library (the rows the test writes)
// with a fixed image and a fixed TMDB search instead of TMDB.
type contractTitles struct {
	*metadata.Service
	hits []metadata.ExternalHit
}

// contractJPEG is the start of a JPEG; the image route sends what metadata opened.
var contractJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}

func (c contractTitles) OpenExternalTitleImage(ctx context.Context, media string, tmdbID int64, kind, size string, _ bool) (model.StreamSource, error) {
	if _, err := c.ExternalTitle(ctx, media, tmdbID); err != nil || kind != "poster" {
		return model.StreamSource{}, model.ErrNotFound
	}
	return model.StreamSource{File: readSeekNopCloser{bytes.NewReader(contractJPEG)}, MediaType: "image/jpeg",
		Size: int64(len(contractJPEG)), ModTime: time.Unix(1, 0), ETag: `"poster-` + size + `"`}, nil
}

func (c contractTitles) SearchExternalTitles(_ context.Context, media, _ string) ([]metadata.ExternalHit, error) {
	var out []metadata.ExternalHit
	for _, h := range c.hits {
		if media == "" || h.Media == media {
			out = append(out, h)
		}
	}
	return out, nil
}

// TMDB ids of the test: missing films rated 0, 12 and 16, one without a known rating, one TMDB
// knows only in the search, and the library film rated 16 of vistest (tmdb_id 1003).
const (
	tmdbMissing0     = 9000
	tmdbMissing12    = 9012
	tmdbMissing16    = 9016
	tmdbUnknown      = 9099
	tmdbOnlySearch   = 9777
	tmdbLibrary16    = 1003
	contractSharedID = 5
)

func TestContractWishes(t *testing.T) {
	pf := newProfileFixture(t)
	f := pf.f
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	// Seerr set up and tested, the wishes on, the shared user as the sender of every account.
	fake := seerrtest.New(t)
	fake.AddUser(seerrtest.User{ID: contractSharedID, Email: wishes.SharedEmail, Username: wishes.SharedUsername, Permissions: seerrtest.PermRequest})
	fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdbMissing12, Name: "Wunsch Zwoelf", Year: 2024})
	fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdbMissing0, Name: "Wunsch Null", Year: 2024})
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
	// Every account and profile may request, except P6 (F-5 by hand).
	exec(`UPDATE accounts SET can_request = 1`)
	exec(`UPDATE profiles SET can_request = CASE WHEN id = ? THEN 0 ELSE 1 END`, f.P6.ID)
	// The stored titles outside the library (metadata writes them after a TMDB fetch).
	for _, x := range []struct {
		tmdb   int64
		age    any
		title  string
		poster any
	}{
		{tmdbMissing0, 0, "Wunsch Null", "/p0.jpg"}, {tmdbMissing12, 12, "Wunsch Zwoelf", "/p12.jpg"},
		{tmdbMissing16, 16, "Wunsch Sechzehn", "/p16.jpg"}, {tmdbUnknown, nil, "Wunsch Unbekannt", nil},
		{tmdbLibrary16, 16, "Film 16", "/l16.jpg"},
	} {
		exec(`INSERT INTO external_titles (media, tmdb_id, year, min_age, poster_path, fetched_at) VALUES ('movie', ?, 2024, ?, ?, ?)`,
			x.tmdb, x.age, x.poster, time.Now().UnixMilli())
		exec(`INSERT INTO external_title_texts (media, tmdb_id, language, title, overview) VALUES ('movie', ?, 'de', ?, 'Inhalt')`, x.tmdb, x.title)
	}
	log := vistest.Logger()
	titles := contractTitles{
		Service: metadata.New(f.DB, jobs.NewQueue(f.DB.Write, nil), pf.admin, t.TempDir(), transcode.Runner{}, log),
		hits: []metadata.ExternalHit{
			{Media: "movie", TMDBID: tmdbMissing12, Title: "Wish 12"}, {Media: "movie", TMDBID: tmdbMissing16, Title: "Wish 16"},
			{Media: "movie", TMDBID: tmdbUnknown, Title: "Wish unknown"}, {Media: "movie", TMDBID: tmdbLibrary16, Title: "Film 16"},
			{Media: "movie", TMDBID: tmdbOnlySearch, Title: "Only in the search", Year: 2023},
		},
	}
	o := pf.opts
	o.Wishes = wishes.New(f.DB, pf.admin, pf.queue, log, wishes.WithLibrary(visibility.New(f.DB, log)), wishes.WithTitles(titles))
	h := NewHandler(o)

	call := func(token, method, path, body string) (int, map[string]any) {
		t.Helper()
		rec := send(t, h, method, BasePath+path, token, body, nil)
		assertNoSecrets(t, path, rec)
		if strings.Contains(rec.Body.String(), "image.tmdb.org") {
			t.Errorf("%s %s: a TMDB address in the answer: %s", method, path, rec.Body.String())
		}
		var out map[string]any
		if strings.Contains(rec.Header().Get("Content-Type"), "json") {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("%s %s: %v (%s)", method, path, err, rec.Body.String())
			}
		}
		return rec.Code, out
	}
	expect := func(token, method, path, body string, status int, code string) map[string]any {
		t.Helper()
		got, out := call(token, method, path, body)
		if got != status || (code != "" && out["code"] != code) {
			t.Errorf("%s %s %s: %d %v, want %d %s", token, method, path, got, out, status, code)
		}
		return out
	}
	title := func(tmdb int64) string { return fmt.Sprintf("/titles/movie/%d", tmdb) }

	// The age: 16 for P16 and P18, 404 for P12; an unknown rating counts as 18.
	expect("p18", http.MethodGet, title(tmdbMissing16), "", http.StatusOK, "")
	expect("p16", http.MethodGet, title(tmdbMissing16), "", http.StatusOK, "")
	expect("p12", http.MethodGet, title(tmdbMissing16), "", http.StatusNotFound, "not_found")
	expect("p18", http.MethodGet, title(tmdbUnknown), "", http.StatusOK, "")
	expect("p16", http.MethodGet, title(tmdbUnknown), "", http.StatusNotFound, "not_found")
	expect("p12", http.MethodGet, title(tmdbUnknown), "", http.StatusNotFound, "not_found")
	// The preview page with its wish card.
	pg := expect("p12", http.MethodGet, title(tmdbMissing12), "", http.StatusOK, "")
	if pg["title"] != "Wunsch Zwoelf" || pg["canRequest"] != true || pg["minAge"] != float64(12) ||
		pg["images"].(map[string]any)["poster"] != BasePath+title(tmdbMissing12)+"/images/poster/w400" {
		t.Errorf("preview = %v", pg)
	}
	// Without the request right 404, also for a title rated 0; with an own wish 200 (F-9).
	expect("p6", http.MethodGet, title(tmdbMissing0), "", http.StatusNotFound, "not_found")
	exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at) VALUES (900, 'movie', ?, 'Wunsch Null', 'slouchcat', 'declined', 1, 2)`, tmdbMissing0)
	exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (900, ?, ?, 'owner', 1)`, f.P6.ID, f.P6.AccountID)
	own := expect("p6", http.MethodGet, title(tmdbMissing0), "", http.StatusOK, "")
	if own["canRequest"] != false || own["requestReason"] != "profile_not_allowed" || own["lastWish"].(map[string]any)["state"] != "declined" {
		t.Errorf("own wish without the right = %v", own)
	}
	// A library title: 409 with the ItemRef for a profile that sees it, 404 for one that does not.
	lib := expect("p18", http.MethodGet, title(tmdbLibrary16), "", http.StatusConflict, "already_available")
	if item, _ := lib["item"].(map[string]any); item == nil || item["id"] != f.Rated(16).ID {
		t.Errorf("already_available = %v, want the item %s", lib, f.Rated(16).ID)
	}
	expect("p12", http.MethodGet, title(tmdbLibrary16), "", http.StatusNotFound, "not_found")

	// The images follow the same rules; the administration sees every stored title.
	img := func(token, path string, status int) {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+path, token, "", nil)
		if rec.Code != status {
			t.Errorf("%s %s: %d %s, want %d", token, path, rec.Code, rec.Body.String(), status)
		}
		if status == http.StatusOK && (!bytes.Equal(rec.Body.Bytes(), contractJPEG) || rec.Header().Get("ETag") == "" ||
			rec.Header().Get("Cache-Control") != externalImageCache) {
			t.Errorf("%s %s: image answer %v", token, path, rec.Header())
		}
	}
	img("p18", title(tmdbMissing16)+"/images/poster/w400", http.StatusOK)
	img("p12", title(tmdbMissing16)+"/images/poster/w400", http.StatusNotFound)
	img("p16", title(tmdbUnknown)+"/images/poster/w400", http.StatusNotFound)
	img("p6", title(tmdbMissing0)+"/images/poster/w400", http.StatusOK) // own wish
	img("p18", title(tmdbLibrary16)+"/images/poster/w400", http.StatusNotFound)
	img("p18", title(tmdbMissing16)+"/images/poster/w3840", http.StatusNotFound)
	img("p18", "/admin/titles/movie/9016/images/poster/w200", http.StatusOK)
	img("p16", "/admin/titles/movie/9016/images/poster/w200", http.StatusForbidden)

	// The search section: missing titles only, by age; empty without the right.
	search := func(token string) []float64 {
		t.Helper()
		out := expect(token, http.MethodGet, "/titles/search?q=Wish", "", http.StatusOK, "")
		var ids []float64
		items, _ := out["items"].([]any)
		for _, it := range items {
			ids = append(ids, it.(map[string]any)["tmdbId"].(float64))
		}
		return ids
	}
	if got := search("p18"); !slices.Equal(got, []float64{tmdbMissing12, tmdbMissing16, tmdbUnknown, tmdbOnlySearch}) {
		t.Errorf("search p18 = %v", got)
	}
	if got := search("p12"); !slices.Equal(got, []float64{tmdbMissing12}) {
		t.Errorf("search p12 = %v", got)
	}
	if got := search("p6"); len(got) != 0 {
		t.Errorf("search without the right = %v", got)
	}
	expect("p18", http.MethodGet, "/titles/search?q=W", "", http.StatusUnprocessableEntity, "validation_failed")

	// createWish: 201, again 200, a co-wish 200 (joined), no right 403, the limit used up 409.
	body := `{"media":"movie","tmdbId":9012,"source":"detail"}`
	w := expect("p18", http.MethodPost, "/wishes", body, http.StatusCreated, "")
	wishID := w["id"]
	if w["role"] != "owner" || w["external"].(map[string]any)["tmdbId"] != float64(tmdbMissing12) {
		t.Errorf("created = %v", w)
	}
	if again := expect("p18", http.MethodPost, "/wishes", body, http.StatusOK, ""); again["id"] != wishID {
		t.Errorf("asked again = %v", again)
	}
	if joined := expect("p16", http.MethodPost, "/wishes", body, http.StatusOK, ""); joined["id"] != wishID || joined["role"] != "joined" {
		t.Errorf("co-wish = %v", joined)
	}
	if p := expect("p6", http.MethodPost, "/wishes", `{"media":"movie","tmdbId":9000}`, http.StatusForbidden, "wish_not_allowed"); p["reason"] != "profile_not_allowed" {
		t.Errorf("no right = %v", p)
	}
	expect("p12", http.MethodPost, "/wishes", `{"media":"movie","tmdbId":9016}`, http.StatusNotFound, "not_found")
	expect("p18", http.MethodPost, "/wishes", `{"media":"movie","tmdbId":1003}`, http.StatusConflict, "already_available")
	exec(`UPDATE seerr_users SET quota_at = 1, movie_quota_used = 10 WHERE seerr_id = ?`, contractSharedID)
	q := expect("p18", http.MethodPost, "/wishes", `{"media":"movie","tmdbId":9016}`, http.StatusConflict, "quota_exceeded")
	if quota, _ := q["quota"].(map[string]any); q["needed"] != float64(1) || quota == nil || quota["movie"].(map[string]any)["remaining"] != float64(0) {
		t.Errorf("quota_exceeded = %v", q)
	}
	expect("p18", http.MethodGet, "/wishes/quota", "", http.StatusOK, "")
	expect("p6", http.MethodGet, "/wishes/quota", "", http.StatusNotFound, "not_found")
	expect("p6", http.MethodGet, "/titles/show/1399/seasons", "", http.StatusNotFound, "not_found")

	// «Meine Wünsche»: the ItemRef of a fulfilled wish only for a profile that sees it.
	exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, item_id, created_at, updated_at, available_at, closed_at)
	      VALUES (8901, 'movie', ?, 'Film 16', 'slouchcat', 'available', ?, 1, 3, 3, 3)`, tmdbLibrary16, f.Rated(16).ID)
	exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (8901, ?, ?, 'owner', 1), (8901, ?, ?, 'joined', 2)`,
		f.P18.ID, f.P18.AccountID, f.P12.ID, f.P12.AccountID)
	fulfilled := func(token string) map[string]any {
		t.Helper()
		out := expect(token, http.MethodGet, "/wishes?state=available", "", http.StatusOK, "")
		items, _ := out["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("%s: available wishes = %v", token, out)
		}
		return items[0].(map[string]any)
	}
	if it, _ := fulfilled("p18")["item"].(map[string]any); it == nil || it["id"] != f.Rated(16).ID {
		t.Errorf("p18 fulfilled wish without its ItemRef")
	}
	if it := fulfilled("p12")["item"]; it != nil {
		t.Errorf("p12 sees the ItemRef of a title rated 16: %v", it)
	}
	expect("p18", http.MethodGet, "/wishes?state=nope", "", http.StatusUnprocessableEntity, "validation_failed")

	// Withdrawing: the joined profile withdraws itself; a wish of another profile is 404.
	expect("p16", http.MethodDelete, fmt.Sprintf("/wishes/%v", wishID), "", http.StatusNoContent, "")
	expect("p12", http.MethodDelete, fmt.Sprintf("/wishes/%v", wishID), "", http.StatusNotFound, "not_found")

	// The administration: every wish, no ItemRef, the poster through the admin route.
	rec := send(t, h, http.MethodGet, BasePath+"/admin/wishes", "p18", "", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"inWatchlist"`) ||
		!strings.Contains(rec.Body.String(), BasePath+"/admin/titles/movie/9012/images/poster/w200") {
		t.Errorf("listAdminWishes: %d %s", rec.Code, rec.Body.String())
	}
	expect("p18", http.MethodPost, "/admin/seerr/sync", "", http.StatusAccepted, "")
	if st := expect("p18", http.MethodGet, "/admin/seerr/status", "", http.StatusOK, ""); st["counts"].(map[string]any)["open"] != float64(1) {
		t.Errorf("status counts = %v, want one open wish", st["counts"])
	}

	// An inbox entry about a wish without a title the profile sees: no item, the wish.
	exec(`INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (?, 'wish_failed', 900, 'wish:900:failed', 5)`, f.P6.ID)
	inbox := expect("p6", http.MethodGet, "/notifications", "", http.StatusOK, "")
	entries, _ := inbox["notifications"].([]any)
	if len(entries) != 1 {
		t.Fatalf("p6 inbox = %v", inbox)
	}
	if e := entries[0].(map[string]any); e["item"] != nil || e["kind"] != "wish_failed" || e["wish"].(map[string]any)["id"] != float64(900) {
		t.Errorf("wish entry = %v", e)
	}

	// The channels take the wish events; wish_review is for administrators only.
	expect("p18", http.MethodPut, "/me/channels/email", `{"enabled":false,"events":["wish_available","wish_status","wish_review"]}`, http.StatusOK, "")
	expect("pg", http.MethodPut, "/me/channels/email", `{"enabled":false,"events":["wish_review"]}`, http.StatusUnprocessableEntity, "event_not_allowed")
	ch := expect("pg", http.MethodPut, "/me/channels/email", `{"enabled":false,"events":["wish_available","wish_status"]}`, http.StatusOK, "")
	if ev, _ := ch["events"].([]any); len(ev) != 2 {
		t.Errorf("guest channel = %v", ch)
	}
}
