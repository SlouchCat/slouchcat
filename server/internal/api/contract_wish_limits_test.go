package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Concept 25, security finding: the operations on titles outside the library take a token per
// profile; an empty bucket answers 429 too_many_attempts with Retry-After, the bucket refills with
// time, and another profile has its own. The clock of the handler is fixed, so no test depends on
// the wall clock.
func TestContractWishLimits(t *testing.T) {
	c := newWishContract(t)
	at := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return at }
	c.withTitles(c.titles)

	drain := func(token, path string, burst int) {
		t.Helper()
		for i := range burst {
			rec := send(t, c.h, http.MethodGet, BasePath+path, token, "", nil)
			if rec.Code == http.StatusTooManyRequests {
				t.Fatalf("%s %s: 429 after %d calls, want %d", token, path, i, burst)
			}
		}
		rec := send(t, c.h, http.MethodGet, BasePath+path, token, "", nil)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" ||
			!strings.Contains(rec.Body.String(), `"too_many_attempts"`) {
			t.Errorf("%s %s after %d calls: %d %s (Retry-After %q)", token, path, burst, rec.Code, rec.Body.String(), rec.Header().Get("Retry-After"))
		}
	}
	title := fmt.Sprintf("/titles/movie/%d", tmdbMissing12)
	// The title and the seasons share one bucket of 30.
	drain("p18", title, wishTitlesBurst)
	if rec := send(t, c.h, http.MethodGet, BasePath+"/titles/show/1399/seasons", "p18", "", nil); rec.Code != http.StatusTooManyRequests {
		t.Errorf("getWishSeasons with the bucket empty: %d", rec.Code)
	}
	// Another profile has its own bucket; time refills it (60 a minute: one per second).
	if rec := send(t, c.h, http.MethodGet, BasePath+title, "p16", "", nil); rec.Code != http.StatusOK {
		t.Errorf("p16: %d %s", rec.Code, rec.Body.String())
	}
	at = at.Add(2 * time.Second)
	if rec := send(t, c.h, http.MethodGet, BasePath+title, "p18", "", nil); rec.Code != http.StatusOK {
		t.Errorf("p18 after the refill: %d", rec.Code)
	}
	// The images: 300 at once.
	drain("p12", title+"/images/poster/w400", wishImagesBurst)
	// The pages of the wishes area: 60 at once; the search its own 30.
	drain("p16", "/wishes/genres/movie", wishPagesBurst)
	drain("p16", "/wishes/search?q=Wunsch", wishSearchBurst)
}
