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
	"github.com/slouchcat/slouchcat/server/internal/seerr"
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
	if pg["title"] != "Wunsch Zwoelf" || pg["canRequest"] != true || pg["minAge"] != float64(12) {
		t.Errorf("preview = %v", pg)
	}
	// The images as ImageRef: the own route with v and p; nominal sizes until the image is scaled.
	imgs := pg["images"].(map[string]any)
	for key, want := range map[string][3]any{
		"poster": {"poster/w400", 400.0, 600.0}, "posterSmall": {"poster/w200", 200.0, 300.0}, "posterLarge": {"poster/w800", 800.0, 1200.0},
	} {
		ref, _ := imgs[key].(map[string]any)
		u, _ := ref["url"].(string)
		if ref == nil || !strings.HasPrefix(u, BasePath+title(tmdbMissing12)+"/images/"+want[0].(string)+"?v=") ||
			!strings.HasSuffix(u, fmt.Sprintf("&p=%d", f.P12.ID)) || ref["width"] != want[1] || ref["height"] != want[2] {
			t.Errorf("images.%s = %v", key, ref)
		}
	}
	if imgs["backdrop"] != nil || imgs["logo"] != nil {
		t.Errorf("a kind without a TMDB image: %v", imgs)
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
	// p12 may not see the title rated 16: its wish reads «In Beschaffung · wird eingelesen»
	// (architecture 8.3), without ItemRef and time, and counts as processing, not available.
	if out := expect("p12", http.MethodGet, "/wishes?state=available", "", http.StatusOK, ""); out["total"] != float64(0) {
		t.Errorf("p12 available wishes = %v", out)
	}
	all := expect("p12", http.MethodGet, "/wishes?state=all", "", http.StatusOK, "")
	var hidden map[string]any
	items, _ := all["items"].([]any)
	for _, it := range items {
		if w := it.(map[string]any); w["id"] == float64(8901) {
			hidden = w
		}
	}
	if hidden == nil || hidden["state"] != "processing" || hidden["stateDetail"] != "importing" || hidden["item"] != nil || hidden["availableAt"] != nil {
		t.Errorf("p12 wish of a title rated 16 = %v", hidden)
	}
	if counts, _ := all["counts"].(map[string]any); counts == nil || counts["available"] != float64(0) || counts["processing"] == float64(0) {
		t.Errorf("p12 counts = %v", all["counts"])
	}
	// A partial series p12 may not see: «wird eingelesen» without episode counts or progress
	// (security review L-2); p18 sees them.
	exec(`UPDATE wishes SET media = 'show', state = 'partial', available_at = NULL WHERE id = 8901`)
	exec(`INSERT INTO wish_progress (wish_id, service, stage, episodes_done, episodes_total, read_at) VALUES (8901, 'sonarr', 'download', 3, 8, ?)`, time.Now().UnixMilli())
	progressOf := func(token string) any {
		t.Helper()
		for _, it := range expect(token, http.MethodGet, "/wishes?state=all", "", http.StatusOK, "")["items"].([]any) {
			if w := it.(map[string]any); w["id"] == float64(8901) {
				return w["progress"]
			}
		}
		t.Fatalf("%s: wish 8901 missing", token)
		return nil
	}
	if pr := progressOf("p18"); pr == nil {
		t.Errorf("p18 progress of its partial wish missing")
	}
	if pr := progressOf("p12"); pr != nil {
		t.Errorf("p12 progress of a hidden wish = %v", pr)
	}
	exec(`DELETE FROM wish_progress WHERE wish_id = 8901`)
	// Viewers get the safe fields of the events only (security review L-3); the administration
	// keeps the detail.
	exec(`INSERT INTO wish_events (wish_id, at, kind, detail) VALUES (8901, 5, 'owner_changed', '{"from":11,"to":12,"reason":"profile_deleted","accountId":3,"seerrUserId":4,"requestIds":[9],"seasons":[1]}')`)
	for _, it := range expect("p18", http.MethodGet, "/wishes?state=all", "", http.StatusOK, "")["items"].([]any) {
		w := it.(map[string]any)
		for _, e := range w["events"].([]any) {
			ev := e.(map[string]any)
			d, _ := ev["detail"].(string)
			for _, bad := range []string{"profileId", "accountId", "seerrUserId", "requestId", `"from"`, `"to":12`} {
				if strings.Contains(d, bad) {
					t.Errorf("viewer event %v carries %s", ev, bad)
				}
			}
			if w["id"] == float64(8901) && ev["kind"] == "owner_changed" && d != `{"reason":"profile_deleted","seasons":[1]}` {
				t.Errorf("viewer owner_changed = %q", d)
			}
		}
	}
	exec(`UPDATE wishes SET media = 'movie', state = 'available', available_at = 3 WHERE id = 8901`)
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

	// retryWish (architecture 8.1): only failed wishes; a submission failure goes back to the outbox,
	// arr_failed is retried in Seerr; declined, account_deleted, an unknown wish and a title another
	// open wish holds are refused.
	state := func(id int) (st, reason string) {
		t.Helper()
		var r *string
		if err := f.DB.Read.QueryRowContext(ctx, `SELECT state, fail_reason FROM wishes WHERE id = ?`, id).Scan(&st, &r); err != nil {
			t.Fatal(err)
		}
		if r != nil {
			reason = *r
		}
		return st, reason
	}
	exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, fail_reason, seerr_user_id, created_at, updated_at, closed_at)
	      VALUES (910, 'movie', 9016, 'Wunsch Sechzehn', 'slouchcat', 'failed', 'quota', ?, 1, 2, 2),
	             (911, 'movie', 9099, 'Wunsch Unbekannt', 'slouchcat', 'failed', 'account_deleted', NULL, 1, 2, 2),
	             (912, 'movie', 9012, 'Wunsch Zwoelf', 'slouchcat', 'failed', 'quota', ?, 1, 2, 2)`, contractSharedID, contractSharedID)
	expect("p18", http.MethodPost, "/admin/wishes/910/retry", "", http.StatusAccepted, "")
	if st, reason := state(910); st != "queued" || reason != "" {
		t.Errorf("retried submission failure = %s %s, want queued", st, reason)
	}
	expect("p18", http.MethodPost, "/admin/wishes/910/retry", "", http.StatusConflict, "wish_not_retryable")
	expect("p18", http.MethodPost, "/admin/wishes/900/retry", "", http.StatusConflict, "wish_not_retryable") // declined
	expect("p18", http.MethodPost, "/admin/wishes/911/retry", "", http.StatusConflict, "wish_not_retryable") // account_deleted
	expect("p18", http.MethodPost, "/admin/wishes/912/retry", "", http.StatusConflict, "wish_not_retryable") // 9012 is open in another wish
	expect("p18", http.MethodPost, "/admin/wishes/99999/retry", "", http.StatusNotFound, "not_found")
	expect("p16", http.MethodPost, "/admin/wishes/910/retry", "", http.StatusForbidden, "")
	sc, err := seerr.New(fake.URL, seerrtest.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	failedReq, err := sc.AsUser(contractSharedID).CreateRequest(ctx, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: tmdbMissing0})
	if err != nil {
		t.Fatal(err)
	}
	fake.SetRequestStatus(failedReq.ID, seerrtest.RequestFailed)
	exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, fail_reason, seerr_user_id, seerr_request_id, created_at, updated_at, closed_at)
	      VALUES (913, 'movie', 9000, 'Wunsch Null', 'slouchcat', 'failed', 'arr_failed', ?, ?, 1, 2, 2)`, contractSharedID, failedReq.ID)
	expect("p18", http.MethodPost, "/admin/wishes/913/retry", "", http.StatusAccepted, "")
	if st, _ := state(913); st != "processing" || fake.Calls("POST /request/{id}/retry") != 1 {
		t.Errorf("arr_failed retried = %s, %d calls", st, fake.Calls("POST /request/{id}/retry"))
	}

	// listAccountWishes: the wishes of every profile of the account for the primary profile only,
	// with the profile's name; entries of other profiles cannot be withdrawn from here.
	acc := expect("p18", http.MethodGet, "/account/wishes", "", http.StatusOK, "")
	names := map[string]bool{}
	accItems, _ := acc["items"].([]any)
	for _, it := range accItems {
		e := it.(map[string]any)
		name, _ := e["profileName"].(string)
		names[name] = true
		if name != f.P18.Name && e["canWithdraw"] == true {
			t.Errorf("entry of %s can be withdrawn by the primary profile: %v", name, e)
		}
	}
	if !names[f.P18.Name] || !names[f.P12.Name] || !names[f.P6.Name] || acc["total"] != float64(len(accItems)) {
		t.Errorf("account wishes = %v", acc)
	}
	expect("p12", http.MethodGet, "/account/wishes", "", http.StatusForbidden, "primary_profile_required")
	expect("p18", http.MethodGet, "/account/wishes?state=nope", "", http.StatusUnprocessableEntity, "validation_failed")

	// 429: more than 20 wishes of a profile in an hour, more than 30 searches in a minute.
	for i := range 20 {
		exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at, available_at, closed_at)
		      VALUES (?, 'movie', ?, 'Viele', 'slouchcat', 'available', 1, 1, 1, 1)`, 921+i, 100+i)
		exec(`INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (?, ?, ?, 'owner', ?)`,
			921+i, f.P16.ID, f.P16.AccountID, time.Now().UnixMilli())
	}
	rec = send(t, h, http.MethodPost, BasePath+"/wishes", "p16", `{"media":"movie","tmdbId":9000}`, nil)
	if rec.Code != http.StatusTooManyRequests || problemCode(t, rec) != "too_many_attempts" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("21st wish in an hour: %d %s", rec.Code, rec.Body.String())
	}
	for i := range 31 {
		rec = send(t, h, http.MethodGet, BasePath+"/titles/search?q=Wish", "p16", "", nil)
		if want := http.StatusOK; i < 30 && rec.Code != want {
			t.Fatalf("search %d: %d", i, rec.Code)
		}
	}
	if rec.Code != http.StatusTooManyRequests || problemCode(t, rec) != "too_many_attempts" {
		t.Errorf("31st search in a minute: %d %s", rec.Code, rec.Body.String())
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
