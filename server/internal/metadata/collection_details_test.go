package metadata

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
)

// fakeCollections serves /3/collection/{id} on the fake TMDB server: the overview per TMDB id and
// request language, and the requests seen ("<id>/<language>").
type fakeCollections struct {
	mu        sync.Mutex
	overviews map[string]string
	seen      []string
}

func newFakeCollections(t *testing.T, e *env) *fakeCollections {
	t.Helper()
	fc := &fakeCollections{overviews: map[string]string{}}
	e.tmdb.mux.HandleFunc("GET /3/collection/{id}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("id") + "/" + r.URL.Query().Get("language")
		fc.mu.Lock()
		fc.seen = append(fc.seen, key)
		overview, ok := fc.overviews[key]
		fc.mu.Unlock()
		switch {
		case r.Header.Get("Authorization") != "Bearer "+goodToken:
			http.Error(w, `{"status_code":7}`, http.StatusUnauthorized)
		case !ok:
			http.Error(w, `{"status_code":34}`, http.StatusNotFound)
		default:
			id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "Collection", "overview": overview})
		}
	})
	return fc
}

func (fc *fakeCollections) requests() []string {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return append([]string(nil), fc.seen...)
}

type collectionDetailsRow struct {
	Description, DescriptionLang, DetailsLang sql.NullString
	FetchedAt                                 sql.NullInt64
}

func (e *env) collectionDetails(t *testing.T, id string) collectionDetailsRow {
	t.Helper()
	var r collectionDetailsRow
	if err := e.db.Read.QueryRow(`SELECT description, description_lang, details_lang, details_fetched_at FROM collections WHERE id = ?`,
		id).Scan(&r.Description, &r.DescriptionLang, &r.DetailsLang, &r.FetchedAt); err != nil {
		t.Fatal(err)
	}
	return r
}

// Concept 21 O-18: after the people, people.details asks TMDB for the overview of every TMDB
// collection with a live member in the main language; an empty German one falls back to English,
// a collection TMDB does not know keeps no description with its fetch time. Manual collections and
// collections only of deleted titles are never asked; a second run asks nothing.
func TestCollectionDetailsFetchesOpenCollections(t *testing.T) {
	t.Parallel()
	e, _ := newDetailsEnv(t, goodToken)
	fc := newFakeCollections(t, e)
	const (
		german  = "01J9000000000000000000CD01"
		english = "01J9000000000000000000CD02"
		unknown = "01J9000000000000000000CD03"
		manual  = "01J9000000000000000000CD04"
		gone    = "01J9000000000000000000CD05"
	)
	for i, id := range []string{german, english, unknown, gone} {
		item := itemLive
		if id == gone {
			item = itemGone
		}
		e.addCollection(t, id, "tmdb", nil, item)
		mustExec(t, e.db.Write, `UPDATE collections SET tmdb_id = ? WHERE id = ?`, 101+i, id)
	}
	e.addCollection(t, manual, "manual", nil, itemLive)
	fc.overviews["101/"+langDE] = "Eine deutsche Beschreibung."
	fc.overviews["102/"+langDE] = " "
	fc.overviews["102/"+langEN] = "An English overview."

	if queued, err := e.svc.QueuePeopleDetails(t.Context()); err != nil || !queued {
		t.Fatalf("QueuePeopleDetails with open collections only = %v, %v; want queued", queued, err)
	}
	rec, err := e.runPeopleDetails(t)
	if err != nil {
		t.Fatal(err)
	}
	if last, _ := rec.Last(); rec.Skipped || last.Done != 3 || last.Total != 3 {
		t.Errorf("run skipped %v, last progress %+v; want 3 of 3", rec.Skipped, last)
	}
	now := testNow.UnixMilli()
	if r := e.collectionDetails(t, german); r.Description.String != "Eine deutsche Beschreibung." || r.DescriptionLang.String != "de" ||
		r.DetailsLang.String != "de" || r.FetchedAt.Int64 != now {
		t.Errorf("German collection %+v", r)
	}
	if r := e.collectionDetails(t, english); r.Description.String != "An English overview." || r.DescriptionLang.String != "en" ||
		r.DetailsLang.String != "de" {
		t.Errorf("collection with the English fallback %+v", r)
	}
	if r := e.collectionDetails(t, unknown); r.Description.Valid || r.DescriptionLang.Valid || r.DetailsLang.String != "de" || r.FetchedAt.Int64 != now {
		t.Errorf("collection unknown to TMDB %+v, want no description and the fetch time", r)
	}
	for _, id := range []string{manual, gone} {
		if r := e.collectionDetails(t, id); r.FetchedAt.Valid {
			t.Errorf("collection %s fetched: %+v", id, r)
		}
	}
	want := []string{"101/de-DE", "102/de-DE", "102/en-US", "103/de-DE"}
	if got := fc.requests(); !slices.Equal(got, want) {
		t.Errorf("requests %v, want %v", got, want)
	}
	rec, err = e.runPeopleDetails(t)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(fc.requests()); n != len(want) || !rec.Skipped {
		t.Errorf("second run: %d requests, skipped %v; want %d and skipped", n, rec.Skipped, len(want))
	}
	if open, err := e.svc.peopleDetailsLeft(t.Context(), "de", nil); err != nil || open {
		t.Errorf("open after the run = %v, %v; want false", open, err)
	}
}
