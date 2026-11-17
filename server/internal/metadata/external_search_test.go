package metadata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The search for titles outside the library: films and series from TMDB in TMDB's order, the year
// from the date; the raw hits are kept 10 minutes per medium and query; without a key
// model.ErrProviderNotConfigured.
func TestExternalTitleSearch(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	now := time.Now()
	e.svc.now = func() time.Time { return now }
	hits, err := e.svc.SearchExternalTitles(ctx, "", "Searched  Film")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0] != (ExternalHit{Media: MediaMovie, TMDBID: 999, Year: 1990}) || hits[1].TMDBID != 601 || hits[1].Year != 2002 {
		t.Errorf("hits = %+v", hits)
	}
	if _, err := e.svc.SearchExternalTitles(ctx, MediaShow, "Test Show"); err != nil {
		t.Fatal(err)
	}
	shows, _ := e.svc.SearchExternalTitles(ctx, MediaShow, "test show")
	if len(shows) != 1 || shows[0] != (ExternalHit{Media: MediaShow, TMDBID: 900, Title: "Test Show", OriginalTitle: "Test Show", Year: 2010}) {
		t.Errorf("shows = %+v", shows)
	}
	if n := e.tmdb.count("/3/search/tv"); n != 2 { // "" asked both, "show" once, its second call came from the cache
		t.Errorf("search/tv asked %d times, want 2", n)
	}
	now = now.Add(externalSearchTTL)
	if _, err := e.svc.SearchExternalTitles(ctx, MediaShow, "Test Show"); err != nil || e.tmdb.count("/3/search/tv") != 3 {
		t.Errorf("after 10 minutes: %v, search/tv %d times", err, e.tmdb.count("/3/search/tv"))
	}
	if _, err := newEnv(t, "").svc.SearchExternalTitles(ctx, "", "Searched Film"); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Errorf("without a key: %v", err)
	}
}
