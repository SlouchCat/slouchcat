package metadata

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// fakePeople serves /3/person/{id} on the fake TMDB server: the answer per TMDB id and request
// language (de-DE, en-US), a status per id instead, and the requests seen.
type fakePeople struct {
	mu      sync.Mutex
	answers map[string]map[string]any // "<id>/<language>" → body
	status  map[int64]int
	seen    []string // "<id>/<language>/<append_to_response>"
}

func newFakePeople(t *testing.T, e *env) *fakePeople {
	t.Helper()
	fp := &fakePeople{answers: map[string]map[string]any{}, status: map[int64]int{}}
	e.tmdb.mux.HandleFunc("GET /3/person/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		lang := r.URL.Query().Get("language")
		fp.mu.Lock()
		fp.seen = append(fp.seen, r.PathValue("id")+"/"+lang+"/"+r.URL.Query().Get("append_to_response"))
		status := fp.status[id]
		body, ok := fp.answers[r.PathValue("id")+"/"+lang]
		fp.mu.Unlock()
		switch {
		case r.Header.Get("Authorization") != "Bearer "+goodToken:
			http.Error(w, `{"status_code":7}`, http.StatusUnauthorized)
		case status != 0:
			http.Error(w, "failure", status)
		case !ok:
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		}
	})
	return fp
}

func (fp *fakePeople) set(id int64, lang string, body map[string]any) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.answers[strconv.FormatInt(id, 10)+"/"+lang] = body
}

func (fp *fakePeople) requests() []string {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return append([]string(nil), fp.seen...)
}

// personBody is a TMDB /person answer; credits nil leaves combined_credits out.
func personBody(bio, birthday, deathday string, credits map[string]any) map[string]any {
	b := map[string]any{"id": 1, "name": "Person", "biography": bio, "birthday": nilIfEmpty(birthday),
		"deathday": nilIfEmpty(deathday), "place_of_birth": "Queens, New York City, New York, USA",
		"known_for_department": "Acting"}
	if credits != nil {
		b["combined_credits"] = credits
	}
	return b
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// threeTitles are combined credits of three titles: a film in two roles, a series and a film of
// the same id as the series.
var threeTitles = map[string]any{
	"cast": []any{
		map[string]any{"id": 10, "media_type": "movie", "character": "Moe"},
		map[string]any{"id": 20, "media_type": "tv", "character": "Moe"},
	},
	"crew": []any{
		map[string]any{"id": 10, "media_type": "movie", "job": "Producer"},
		map[string]any{"id": 20, "media_type": "movie", "job": "Director"},
	},
}

type detailsRow struct {
	Biography, BiographyLang, Birthday, Deathday, Place, Dept, DetailsLang sql.NullString
	Total, FetchedAt                                                       sql.NullInt64
}

func (e *env) personDetails(t *testing.T, id int64) detailsRow {
	t.Helper()
	var r detailsRow
	if err := e.db.Read.QueryRow(`SELECT biography, biography_lang, birthday, deathday, place_of_birth, known_for_department,
		details_lang, total_credits, details_fetched_at FROM people WHERE id = ?`, id).Scan(&r.Biography, &r.BiographyLang,
		&r.Birthday, &r.Deathday, &r.Place, &r.Dept, &r.DetailsLang, &r.Total, &r.FetchedAt); err != nil {
		t.Fatal(err)
	}
	return r
}

func (e *env) runPeopleDetails(t *testing.T) (*jobs.Recorder, error) {
	t.Helper()
	rec := &jobs.Recorder{}
	return rec, e.svc.handlePeopleDetails(jobs.WithRecorder(t.Context(), rec), nil)
}

// newDetailsEnv returns an env with a live and a deleted title and the fake person endpoint.
func newDetailsEnv(t *testing.T, key string) (*env, *fakePeople) {
	t.Helper()
	e := newEnv(t, key)
	e.addItem(t, itemSeed{id: itemLive, title: "Live"})
	e.addItem(t, itemSeed{id: itemGone, title: "Gone"})
	mustExec(t, e.db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, itemGone)
	return e, newFakePeople(t, e)
}

// The TMDB answer maps to the stored facts: the biography in the main language, the dates, the
// place, the department and every title of the combined credits once; null and a date that is no
// day stay NULL.
func TestPersonDetailsRow(t *testing.T) {
	t.Parallel()
	var d PersonDetail
	raw, _ := json.Marshal(personBody("  Bio.  ", "1964-04-25", "", threeTitles))
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	p := personDetailsRow(d, "de")
	if p.Biography.String != "Bio." || p.BiographyLang.String != "de" || p.Birthday.String != "1964-04-25" || p.Deathday.Valid ||
		p.PlaceOfBirth.String != "Queens, New York City, New York, USA" || p.KnownForDepartment.String != "Acting" ||
		!p.TotalCredits.Valid || p.TotalCredits.Int64 != 3 {
		t.Errorf("row %+v", p)
	}
	d = PersonDetail{Biography: " ", Birthday: "1964", Deathday: "unknown"}
	if p := personDetailsRow(d, "de"); p.Biography.Valid || p.BiographyLang.Valid || p.Birthday.Valid || p.Deathday.Valid ||
		p.PlaceOfBirth.Valid || p.KnownForDepartment.Valid || p.TotalCredits.Valid {
		t.Errorf("empty answer row %+v, want all NULL", p)
	}
}

// A run fetches every open person in the main language with combined_credits; an empty German
// biography falls back to English (O-13), a person TMDB does not know keeps NULL facts, people
// without a role on a live title are left alone, and a second run asks nothing.
func TestPeopleDetailsFetchesOpenPeople(t *testing.T) {
	t.Parallel()
	e, fp := newDetailsEnv(t, goodToken)
	fp.set(1, langDE, personBody("Deutsche Biografie.", "1964-04-25", "", threeTitles))
	fp.set(2, langDE, personBody("", "1930-01-01", "2020-02-02", map[string]any{"cast": []any{}, "crew": []any{}}))
	fp.set(2, langEN, personBody("English biography.", "1930-01-01", "2020-02-02", nil))
	e.addPerson(t, 1, nil, itemLive) // no portrait: the facts come anyway
	e.addPerson(t, 2, "/p2.jpg", itemLive)
	e.addPerson(t, 3, nil, itemLive) // TMDB answers 404
	e.addPerson(t, 4, nil, itemGone) // only on a deleted title
	e.addPerson(t, 5, nil, "")       // no role

	rec, err := e.runPeopleDetails(t)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Skipped {
		t.Error("a run with fetches is marked skipped")
	}
	if last, _ := rec.Last(); last.Done != 3 || last.Total != 3 {
		t.Errorf("last progress %+v, want 3 of 3", last)
	}
	now := testNow.UnixMilli()
	if r := e.personDetails(t, 1); r.Biography.String != "Deutsche Biografie." || r.BiographyLang.String != "de" ||
		r.DetailsLang.String != "de" || r.Birthday.String != "1964-04-25" || r.Deathday.Valid || r.Dept.String != "Acting" ||
		r.Place.String != "Queens, New York City, New York, USA" || r.Total.Int64 != 3 || r.FetchedAt.Int64 != now {
		t.Errorf("person 1 %+v", r)
	}
	if r := e.personDetails(t, 2); r.Biography.String != "English biography." || r.BiographyLang.String != "en" ||
		r.DetailsLang.String != "de" || r.Deathday.String != "2020-02-02" || !r.Total.Valid || r.Total.Int64 != 0 {
		t.Errorf("person 2 (English fallback) %+v", r)
	}
	if r := e.personDetails(t, 3); r.Biography.Valid || r.Total.Valid || r.DetailsLang.String != "de" || r.FetchedAt.Int64 != now {
		t.Errorf("person 3 (unknown to TMDB) %+v, want no facts and the fetch time", r)
	}
	for _, id := range []int64{4, 5} {
		if r := e.personDetails(t, id); r.FetchedAt.Valid {
			t.Errorf("person %d fetched: %+v", id, r)
		}
	}
	want := []string{"1/de-DE/combined_credits", "2/de-DE/combined_credits", "2/en-US/", "3/de-DE/combined_credits"}
	if got := fp.requests(); !slices.Equal(got, want) {
		t.Errorf("requests %v, want %v", got, want)
	}

	// Up to date: nothing is asked again.
	rec, err = e.runPeopleDetails(t)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(fp.requests()); n != len(want) || !rec.Skipped {
		t.Errorf("second run: %d requests, skipped %v; want %d and skipped", n, rec.Skipped, len(want))
	}

	// A new main language and facts older than peopleDetailsMaxAge are open again; English as the
	// main language asks no fallback.
	if err := e.svc.write.UpsertSystemSetting(t.Context(), systemq.UpsertSystemSettingParams{Key: SettingTextLangs, Value: `["en","de"]`}); err != nil {
		t.Fatal(err)
	}
	fp.set(1, langEN, personBody("", "1964-04-25", "", threeTitles))
	if _, err := e.runPeopleDetails(t); err != nil {
		t.Fatal(err)
	}
	if r := e.personDetails(t, 1); r.Biography.Valid || r.BiographyLang.Valid || r.DetailsLang.String != "en" {
		t.Errorf("person 1 in English without a biography: %+v, want none and no fallback", r)
	}
	if got := fp.requests(); len(got) != len(want)+3 || got[len(want)] != "1/en-US/combined_credits" {
		t.Errorf("requests after the language change %v", got)
	}
	mustExec(t, e.db.Write, `UPDATE people SET details_fetched_at = ? WHERE id = 1`,
		testNow.Add(-peopleDetailsMaxAge-time.Hour).UnixMilli())
	if open, err := e.svc.peopleDetailsLeft(t.Context(), "en", nil); err != nil || !open {
		t.Errorf("stale facts open = %v, %v; want true", open, err)
	}
}

// Without a key or with a key TMDB rejected a run asks nothing; a 401 during the run remembers
// the rejection, ends the run without error and leaves the people open.
func TestPeopleDetailsKey(t *testing.T) {
	t.Parallel()
	t.Run("no key", func(t *testing.T) {
		t.Parallel()
		e, fp := newDetailsEnv(t, "")
		e.addPerson(t, 1, nil, itemLive)
		rec, err := e.runPeopleDetails(t)
		if err != nil || !rec.Skipped || len(fp.requests()) != 0 {
			t.Errorf("run = %v, skipped %v, requests %v; want skipped without requests", err, rec.Skipped, fp.requests())
		}
		if queued, err := e.svc.QueuePeopleDetails(t.Context()); err != nil || queued {
			t.Errorf("QueuePeopleDetails without a key = %v, %v; want false", queued, err)
		}
	})
	t.Run("refused during the run", func(t *testing.T) {
		t.Parallel()
		e, fp := newDetailsEnv(t, "refused-token")
		e.addPerson(t, 1, nil, itemLive)
		e.addPerson(t, 2, nil, itemLive)
		if _, err := e.runPeopleDetails(t); err != nil {
			t.Fatalf("run = %v, want no error", err)
		}
		if n := len(fp.requests()); n != 1 {
			t.Errorf("%d requests, want 1 (the run stops at the 401)", n)
		}
		if !e.svc.KeyRejected(t.Context(), ProviderTMDB) {
			t.Error("the 401 was not remembered")
		}
		if r := e.personDetails(t, 1); r.FetchedAt.Valid {
			t.Errorf("person 1 after a 401: %+v, want open", r)
		}
		// Rejected: the next run and QueuePeopleDetails ask nothing.
		rec, err := e.runPeopleDetails(t)
		if err != nil || !rec.Skipped || len(fp.requests()) != 1 {
			t.Errorf("run with the rejected key = %v, skipped %v, %d requests", err, rec.Skipped, len(fp.requests()))
		}
		if queued, err := e.svc.QueuePeopleDetails(t.Context()); err != nil || queued {
			t.Errorf("QueuePeopleDetails with a rejected key = %v, %v; want false", queued, err)
		}
		// A key that passes its check again queues the people that waited.
		if err := e.svc.acceptKey(t.Context(), ProviderTMDB); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ?`, JobPeopleDetails).Scan(&n); err != nil || n != 1 {
			t.Errorf("%d people.details jobs after the key passed (%v), want 1", n, err)
		}
	})
}

// A failure of the transport writes nothing and is not asked again in the same run;
// peopleDetailsFailures in a row end the run with an error. Another 4xx marks the fetch and keeps
// the facts.
func TestPeopleDetailsFailures(t *testing.T) {
	t.Parallel()
	e, fp := newDetailsEnv(t, goodToken)
	for id := int64(1); id <= 4; id++ {
		e.addPerson(t, id, nil, itemLive)
	}
	fp.status[1], fp.status[2] = http.StatusServiceUnavailable, http.StatusServiceUnavailable
	fp.status[3] = http.StatusBadRequest
	fp.set(4, langDE, personBody("Bio.", "", "", threeTitles))
	mustExec(t, e.db.Write, `UPDATE people SET biography = 'Old.', biography_lang = 'de' WHERE id = 3`)
	if _, err := e.runPeopleDetails(t); err != nil {
		t.Fatalf("run = %v, want no error below %d failures in a row", err, peopleDetailsFailures)
	}
	if r := e.personDetails(t, 1); r.FetchedAt.Valid {
		t.Errorf("person 1 after a 503: %+v, want nothing written", r)
	}
	if r := e.personDetails(t, 3); r.Biography.String != "Old." || r.FetchedAt.Int64 != testNow.UnixMilli() {
		t.Errorf("person 3 after a 400: %+v, want the old facts and the fetch marked", r)
	}
	if r := e.personDetails(t, 4); r.Biography.String != "Bio." {
		t.Errorf("person 4 %+v", r)
	}
	for _, id := range []string{"1", "2"} {
		n := 0
		for _, r := range fp.requests() {
			if r == id+"/de-DE/combined_credits" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("person %s asked %d times in one run, want 1", id, n)
		}
	}

	fp.status[3], fp.status[4] = http.StatusServiceUnavailable, http.StatusServiceUnavailable
	mustExec(t, e.db.Write, `UPDATE people SET details_fetched_at = NULL`)
	if _, err := e.runPeopleDetails(t); !errors.Is(err, errPeopleDetailsFailures) {
		t.Errorf("run with three failures in a row = %v, want %v", err, errPeopleDetailsFailures)
	}
}

// QueuePeopleRefreshOnce queues people.images for the w185 portraits and people.details for the
// people never fetched, once; the second start queues nothing.
func TestPeopleRefreshOnce(t *testing.T) {
	t.Parallel()
	e, _ := newDetailsEnv(t, goodToken)
	e.addPerson(t, 1, "/p1.jpg", itemLive)
	mustExec(t, e.db.Write, `INSERT INTO person_images VALUES (1, 'jpeg', 'ok', '/p1.jpg', 'w185', 'people/1/w185.jpg', 'h', 'image/jpeg', 185, 278, 10, 1),
		(1, 'webp', 'ok', '/p1.jpg', 'w185', 'people/1/w185.webp', 'h', 'image/webp', 185, 278, 10, 1)`)
	queued, err := e.svc.QueuePeopleRefreshOnce(t.Context())
	if err != nil || !queued {
		t.Fatalf("first start = %v, %v; want queued", queued, err)
	}
	count := func(typ string) int {
		var n int
		if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ?`, typ).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(JobPeopleImages) != 1 || count(JobPeopleDetails) != 1 {
		t.Errorf("jobs images %d, details %d; want 1 each", count(JobPeopleImages), count(JobPeopleDetails))
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if queued, err := e.svc.QueuePeopleRefreshOnce(t.Context()); err != nil || queued || count(JobPeopleImages)+count(JobPeopleDetails) != 0 {
		t.Errorf("second start = %v, %v; want nothing queued", queued, err)
	}
}

// A portrait stored in w185 before concept 21 O-12 is open: the run fetches w300, the rows name
// the new size and the w185 files are gone.
func TestPeopleImagesReplaceOldSize(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w300/p1.jpg", cdnAnswer{data: jpegBytes(t, 300, 450)})
	e.addPerson(t, 1, "/p1.jpg", itemLive)
	dir := filepath.Join(e.cache, "people", "1")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"w185.jpg", "w185.webp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, e.db.Write, `INSERT INTO person_images VALUES (1, 'jpeg', 'ok', '/p1.jpg', 'w185', 'people/1/w185.jpg', 'h', 'image/jpeg', 185, 278, 3, 1),
		(1, 'webp', 'ok', '/p1.jpg', 'w185', 'people/1/w185.webp', 'h', 'image/webp', 185, 278, 3, 1)`)
	if queued, err := e.svc.QueuePeopleImages(t.Context()); err != nil || !queued {
		t.Fatalf("QueuePeopleImages with a w185 row = %v, %v; want queued", queued, err)
	}
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	p := e.personRows(t, 1)
	if j, w := p["jpeg"], p["webp"]; j.Size != "w300" || j.RelPath.String != "people/1/w300.jpg" || j.Width.Int64 != 300 ||
		w.Size != "w300" || w.RelPath.String != "people/1/w300.webp" || w.Width.Int64 != 300 {
		t.Fatalf("rows %+v", p)
	}
	for rel, want := range map[string]bool{"people/1/w300.jpg": true, "people/1/w300.webp": true, "people/1/w185.jpg": false, "people/1/w185.webp": false} {
		if e.cacheFile(rel) != want {
			t.Errorf("%s exists %v, want %v", rel, !want, want)
		}
	}
	if cdn.count("/t/p/w300/p1.jpg") != 1 || cdn.count("/t/p/w185/p1.jpg") != 0 {
		t.Errorf("downloads w300 %d, w185 %d; want 1 and 0", cdn.count("/t/p/w300/p1.jpg"), cdn.count("/t/p/w185/p1.jpg"))
	}
}

func TestRegisterTypesPeopleDetails(t *testing.T) {
	t.Parallel()
	e := newEnv(t, "")
	r := jobs.NewRegistry(e.queue, slog.New(slog.DiscardHandler))
	e.svc.RegisterTypes(r)
	pd, ok := r.Lookup(JobPeopleDetails)
	if !ok || pd.Group != jobs.GroupMetadata || pd.Pool != jobs.PoolGeneral || pd.Trigger != jobs.TriggerEvent ||
		!pd.Windowable || pd.Priority != peopleImagesPriority || pd.Handler == nil || pd.Run != nil {
		t.Errorf("people.details = %+v", pd)
	}
}
