package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 21 P-10 and O-19 (contract 1.33.0): P12 marks every part of the mixed collection; the
// films rated 6 and 12 it sees are watched, with the times rising in collection order, the film
// rated 18 stays open for P18. getCollection and listVisibleCollections then carry 2 of 2 seen,
// started, the years and the runtime, the first part as next (every part finished). false takes
// the marks back. An unknown collection is 404, a missing body 400.
func TestHTTPSetCollectionWatched(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	path := BasePath + "/collections/" + vistest.MixedCollectionID
	put := func(token, body string) int {
		t.Helper()
		return send(t, h, http.MethodPut, path+"/watched", token, body, nil).Code
	}
	collection := func(token string) gen.Collection {
		t.Helper()
		var c gen.Collection
		rec := send(t, h, http.MethodGet, path, token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("getCollection for %s: %d", token, rec.Code)
		}
		decode(t, rec, &c)
		return c
	}
	listed := func(token string) gen.CollectionRef {
		t.Helper()
		var list []gen.CollectionRef
		rec := send(t, h, http.MethodGet, BasePath+"/collections", token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("listVisibleCollections for %s: %d", token, rec.Code)
		}
		decode(t, rec, &list)
		i := slices.IndexFunc(list, func(c gen.CollectionRef) bool { return c.Id == vistest.MixedCollectionID })
		if i < 0 {
			t.Fatalf("listVisibleCollections for %s lacks the mixed collection", token)
		}
		return list[i]
	}
	before := collection("p12")
	if before.SeenCount == nil || *before.SeenCount != 0 || before.Started == nil || *before.Started ||
		before.Next == nil || before.Next.Id != f.Rated(6).ID || before.NextPart == nil || *before.NextPart != 1 ||
		before.RuntimeMinutes == nil || *before.RuntimeMinutes != 240 || before.FirstYear == nil || before.LastYear == nil ||
		*before.FirstYear >= *before.LastYear {
		t.Fatalf("P12 collection before: %+v", before)
	}
	if code := put("p12", `{"watched":true}`); code != http.StatusNoContent {
		t.Fatalf("mark the collection: %d", code)
	}
	c := collection("p12")
	if c.SeenCount == nil || *c.SeenCount != 2 || c.Started == nil || !*c.Started || c.Next == nil || c.Next.Id != f.Rated(6).ID ||
		c.NextPart == nil || *c.NextPart != 1 {
		t.Errorf("P12 collection after the mark: seen %v started %v next %v part %v, want 2 of 2 and part 1 again",
			c.SeenCount, c.Started, c.Next, c.NextPart)
	}
	if len(c.Items) != 2 || c.Items[0].Progress == nil || c.Items[1].Progress == nil || c.Items[0].Progress.WatchedAt == nil ||
		c.Items[1].Progress.WatchedAt == nil || !c.Items[0].Progress.WatchedAt.Before(*c.Items[1].Progress.WatchedAt) {
		t.Errorf("P12 parts after the mark: %+v, want both watched, part 2 the newest", c.Items)
	}
	if ref := listed("p12"); ref.SeenCount == nil || *ref.SeenCount != 2 || ref.Started == nil || !*ref.Started ||
		ref.RuntimeMinutes == nil || *ref.RuntimeMinutes != 240 {
		t.Errorf("P12 list entry after the mark: %+v", ref)
	}
	if p18 := collection("p18"); p18.SeenCount == nil || *p18.SeenCount != 0 || p18.Next == nil || p18.Next.Id != f.Rated(6).ID {
		t.Errorf("P18 after P12 marked: seen %v next %v, want nothing seen", p18.SeenCount, p18.Next)
	}
	if code := put("p12", `{"watched":false}`); code != http.StatusNoContent {
		t.Fatalf("unmark the collection: %d", code)
	}
	if c := collection("p12"); c.SeenCount == nil || *c.SeenCount != 0 {
		t.Errorf("P12 after taking the marks back: seen %v, want 0", c.SeenCount)
	}
	if code := send(t, h, http.MethodPut, BasePath+"/collections/01J9000000000000000000CM00/watched", "p12", `{"watched":true}`, nil).Code; code != http.StatusNotFound {
		t.Errorf("unknown collection: %d, want 404", code)
	}
	if code := put("p12", ""); code != http.StatusBadRequest {
		t.Errorf("missing body: %d, want 400", code)
	}
}
