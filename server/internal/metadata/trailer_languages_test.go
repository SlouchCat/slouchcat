package metadata

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 21 P-4: the trailer languages of the library in include_video_language, the country of
// a region variant first (O-8), a trailer without language as the last rank after YouTube (O-7).

func TestTrailerRegions(t *testing.T) {
	t.Parallel()
	got := trailerRegions([]string{"de", "pt-BR", "en", "pt-PT", "es-mx", "pt-BR"})
	want := map[string][]string{"pt": {"BR", "PT"}, "es": {"MX"}}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("trailerRegions = %v, want %v", got, want)
	}
}

func TestPickTMDBTrailersPrefersCountry(t *testing.T) {
	t.Parallel()
	pt := func(key, country string, size int, official bool) Video {
		v := tmdbVideo("pt", key, size, official, "2020-01-01T00:00:00.000Z")
		v.Country = country
		return v
	}
	videos := []Video{
		pt("portugal001", "PT", 2160, true),
		pt("brazil00001", "BR", 480, false),
		pt("brazil00002", "BR", 720, false),
	}
	for _, c := range []struct {
		name    string
		regions map[string][]string
		want    string
	}{
		{"no country in the list: the rank alone", nil, "portugal001"},
		{"pt-BR: Brazil first, then the rank among them", map[string][]string{"pt": {"BR"}}, "brazil00002"},
		{"pt-PT before pt-BR: Portugal first", map[string][]string{"pt": {"PT", "BR"}}, "portugal001"},
		{"a country without trailer: the rank alone", map[string][]string{"pt": {"AO"}}, "portugal001"},
	} {
		hits := pickTMDBTrailers(videos, []string{"pt"}, c.regions)
		if len(hits) != 1 || hits[0].YouTubeID != c.want {
			t.Errorf("%s: hits %+v, want %s", c.name, hits, c.want)
		}
	}
}

func TestPickLanguagelessTrailer(t *testing.T) {
	t.Parallel()
	noLang := func(key string, size int, official bool, published string) Video {
		return tmdbVideo("", key, size, official, published)
	}
	videos := []Video{
		tmdbVideo("de", "german00001", 2160, true, "2024-01-01T00:00:00.000Z"),
		noLang("offic480000", 480, true, "2024-01-01T00:00:00.000Z"),
		noLang("unoff108000", 1080, false, "2020-01-01T00:00:00.000Z"),
		noLang("unoff108001", 1080, false, "2021-01-01T00:00:00.000Z"),
		{Key: "teaser00001", Site: "YouTube", Type: "Teaser", Official: true, Size: 2160},
	}
	h, ok := pickLanguagelessTrailer(videos)
	if !ok || h.YouTubeID != "unoff108001" || h.Lang != "" || h.Size != 1080 || h.Source != sourceTMDB {
		t.Errorf("languageless = %+v, %v; want the newer unofficial 1080 without language", h, ok)
	}
	if _, ok := pickLanguagelessTrailer(videos[:1]); ok {
		t.Error("a trailer with language served as languageless")
	}
}

// A folder with de, en and it: include_video_language names the three and null; TMDB 550 has de
// and en, YouTube finds nothing in Italian, so it takes the trailer without language.
func TestTrailersLanguagelessAfterYouTube(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	langs := []string{"de", "en", "it"}
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	ownFolder(t, e.db.Write, 1, 1, model.RootMovie, model.RootMetadata{TextLangs: &langs}, idByTMDB)
	if err := e.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e.tmdb.mu.Lock()
	include := slices.Clone(e.tmdb.videoLangs)
	e.tmdb.mu.Unlock()
	if len(include) != 3 || slices.ContainsFunc(include, func(s string) bool { return s != "de,en,it,null" }) {
		t.Errorf("include_video_language %q, want de,en,it,null on each of the three requests", include)
	}
	if q := e.yt.searches(); len(q) != 1 {
		t.Errorf("YouTube searches %q, want one for it before the fallback", q)
	}
	want := []string{"de/deOfficial1/tmdb/1080", "en/enOfficial1/tmdb/1080", "it/noLang00001/tmdb/1080"}
	if got := e.trailerSizes(t, idByTMDB); !slices.Equal(got, want) {
		t.Errorf("trailers %q, want %q", got, want)
	}
	if state, _ := e.trailerState(t, idByTMDB); state != "found" {
		t.Errorf("state %s, want found", state)
	}

	// The quota blocks the Italian search: it stays pending for YouTube, without the fallback.
	e2 := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	e2.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	ownFolder(t, e2.db.Write, 1, 1, model.RootMovie, model.RootMetadata{TextLangs: &langs}, idByTMDB)
	if !e2.quota.Take(ctx, youtubeDailyUnits) {
		t.Fatal("quota setup")
	}
	if err := e2.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e2.trailerSizes(t, idByTMDB); !slices.Equal(got, want[:2]) {
		t.Errorf("trailers with a blocked quota %q, want %q", got, want[:2])
	}
	if state, _ := e2.trailerState(t, idByTMDB); state != "pending" {
		t.Errorf("state with a blocked quota %s, want pending", state)
	}

	// The default list de, en asks for de,en,null as before.
	e3 := newEnvWith(t, testKeys{tmdb: goodToken})
	e3.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e3.svc.findTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	e3.tmdb.mu.Lock()
	defer e3.tmdb.mu.Unlock()
	if !slices.Equal(e3.tmdb.videoLangs, []string{"de,en,null", "de,en,null"}) {
		t.Errorf("include_video_language %q, want de,en,null twice", e3.tmdb.videoLangs)
	}
}

// The recheck applies the fallback too: a language without TMDB trailer and without an earlier
// automatic row takes the trailer without language, without a YouTube search.
func TestTrailerRecheckLanguageless(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	langs := []string{"de", "en", "it"}
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	ownFolder(t, e.db.Write, 1, 1, model.RootMovie, model.RootMetadata{TextLangs: &langs}, idByTMDB)
	mustExec(t, e.db.Write, `UPDATE items SET trailer_state = 'found' WHERE id = ?`, idByTMDB)
	mustExec(t, e.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, source, official, created_at) VALUES (?, 'de', 'deTrailer01', 'tmdb', 0, 1)`, idByTMDB)
	if err := e.svc.recheckTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	want := []string{"de/deOfficial1/tmdb/1080", "en/enOfficial1/tmdb/1080", "it/noLang00001/tmdb/1080"}
	if got := e.trailerSizes(t, idByTMDB); !slices.Equal(got, want) {
		t.Errorf("trailers %q, want %q", got, want)
	}
	if q := e.yt.searches(); len(q) != 0 {
		t.Errorf("YouTube searches %q during the recheck, want none", q)
	}
}
