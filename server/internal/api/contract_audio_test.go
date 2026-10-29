package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract tests of M6 (design section 5, plan 10.2): the audio titles of vistest.AddAudio for every
// profile through getItem and the paged episode list (the file-based operations cover them in
// TestContractChildProfiles), and the administration of audio libraries and podcasts.

func TestContractAudio(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	a := f.Audio
	titles := f.All()
	episodes := slices.Concat([]vistest.Movie{a.FileOnlyExplicit}, a.Episodes, []vistest.Movie{a.Explicit, a.EnclosureOnly})
	for _, token := range []string{"p0", "p6", "p12", "p16", "p18", "px", "pg", "pg18"} {
		profile := pf.tokens[token]
		for _, m := range a.Titles() {
			rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, token, "", nil)
			if want := f.VisibleTo(profile, m); want != (rec.Code == http.StatusOK) {
				t.Errorf("%s getItem %s %s: status %d, visible want %v", profile.Name, m.Kind, m.Label, rec.Code, want)
			} else if !want && problemCode(t, rec) != "not_found" {
				t.Errorf("%s getItem %s: code %s", profile.Name, m.Label, problemCode(t, rec))
			}
			if rec.Code == http.StatusOK {
				assertOnlyVisible(t, f, "getItem "+m.Label, profile, titles, itemIDsIn(t, rec, titles))
			}
		}
		// The explicit podcast (AN-2: channel itunes:explicit = FSK 18): its episode list and its
		// inheriting episode answer 404 below 18, with the episode's play and stream as well.
		rec18 := send(t, h, http.MethodGet, BasePath+"/items/"+a.ExplicitPodcast.ID+"/episodes", token, "", nil)
		if want := f.VisibleTo(profile, a.ExplicitPodcast); want != (rec18.Code == http.StatusOK) {
			t.Errorf("%s: episodes of the explicit podcast: status %d, visible want %v", profile.Name, rec18.Code, want)
		}
		play := send(t, h, http.MethodPost, BasePath+"/items/"+a.ExplicitPodcastEpisode.ID+"/play", token, `{}`, nil)
		if want := f.VisibleTo(profile, a.ExplicitPodcastEpisode); !want && play.Code != http.StatusNotFound {
			t.Errorf("%s: play of the episode of the explicit podcast: %d, want 404", profile.Name, play.Code)
		}
		// The episodes of the podcast, newest first (published one day apart in fixture order).
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+a.Podcast.ID+"/episodes?limit=200", token, "", nil)
		if !f.VisibleTo(profile, a.Podcast) {
			if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
				t.Errorf("%s: episodes of the invisible podcast: %d", profile.Name, rec.Code)
			}
			continue
		}
		var page struct {
			Episodes []struct {
				ID          string `json:"id"`
				Kind        string `json:"kind"`
				PublishedAt string `json:"publishedAt"`
			} `json:"episodes"`
			Total, Offset, Limit int
		}
		decode(t, rec, &page)
		var want, got []string
		for _, m := range episodes {
			if f.VisibleTo(profile, m) {
				want = append(want, m.ID)
			}
		}
		slices.Reverse(want)
		for _, e := range page.Episodes {
			got = append(got, e.ID)
			if e.Kind != "podcast_episode" || e.PublishedAt == "" {
				t.Errorf("%s: episode %+v", profile.Name, e)
			}
		}
		if rec.Code != http.StatusOK || !slices.Equal(got, want) || page.Total != len(want) || page.Limit != 200 || page.Offset != 0 {
			t.Errorf("%s: episodes %v (total %d, limit %d), want %v", profile.Name, got, page.Total, page.Limit, want)
		}
		assertOnlyVisible(t, f, "listPodcastEpisodes", profile, titles, itemIDsIn(t, rec, titles))
		// Paging.
		rec = send(t, h, http.MethodGet, BasePath+"/items/"+a.Podcast.ID+"/episodes?offset=1&limit=1", token, "", nil)
		decode(t, rec, &page)
		if len(want) > 1 && (len(page.Episodes) != 1 || page.Episodes[0].ID != want[1] || page.Total != len(want)) {
			t.Errorf("%s: page 2 = %+v", profile.Name, page)
		}
	}
	// The explicit episode (AN-2): 404 for P12 and P16 on detail and play, visible for P18.
	for _, token := range []string{"p12", "p16"} {
		for _, path := range []string{"/items/" + a.Explicit.ID, "/items/" + a.Explicit.ID + "/download-options"} {
			if rec := send(t, h, http.MethodGet, BasePath+path, token, "", nil); rec.Code != http.StatusNotFound {
				t.Errorf("%s %s: %d", token, path, rec.Code)
			}
		}
		if rec := send(t, h, http.MethodPost, BasePath+"/items/"+a.Explicit.ID+"/play", token, `{}`, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s play explicit: %d", token, rec.Code)
		}
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+a.Explicit.ID, "p18", "", nil); rec.Code != http.StatusOK {
		t.Errorf("p18 explicit episode: %d", rec.Code)
	}
	// The podcast's detail lists its newest episodes with their count and names no other podcast.
	var d struct {
		Episodes     []struct{ ID string } `json:"episodes"`
		EpisodeCount *int                  `json:"episodeCount"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+a.Podcast.ID, "p12", "", nil), &d)
	if d.EpisodeCount == nil || *d.EpisodeCount != 4 || len(d.Episodes) != 4 {
		t.Errorf("p12 podcast detail: %+v", d)
	}
	var ep struct {
		Podcast *struct{ ID string } `json:"podcast"`
		Images  struct {
			Poster struct{ URL string } `json:"poster"`
		} `json:"images"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+a.EnclosureOnly.ID, "p0", "", nil), &ep)
	if ep.Podcast == nil || ep.Podcast.ID != a.Podcast.ID || ep.Images.Poster.URL == "" {
		t.Errorf("enclosure episode detail: %+v", ep)
	} else if img := send(t, h, http.MethodGet, ep.Images.Poster.URL, "p0", "", nil); img.Code != http.StatusOK {
		t.Errorf("episode poster (the podcast's): %d", img.Code)
	}
	// M6 review m-12: an episode without a file downloads from its enclosure; the image of the
	// explicit episode is not found for P12 (404, never 403).
	if rec := send(t, h, http.MethodPost, BasePath+"/items/"+a.EnclosureOnly.ID+"/downloads", "p12", `{"quality":"low"}`, nil); rec.Code != http.StatusCreated {
		t.Errorf("p12 createDownloads of the enclosure episode: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodPost, BasePath+"/items/"+a.Explicit.ID+"/downloads", "p12", `{"quality":"low"}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("p12 createDownloads of the explicit episode: %d", rec.Code)
	}
	ep.Images.Poster.URL = ""
	decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+a.Explicit.ID, "p18", "", nil), &ep)
	if ep.Images.Poster.URL == "" {
		t.Error("explicit episode without poster for P18")
	} else if img := send(t, h, http.MethodGet, ep.Images.Poster.URL, "p12", "", nil); img.Code != http.StatusNotFound || problemCode(t, img) != "not_found" {
		t.Errorf("p12 poster of the explicit episode: %d", img.Code)
	}
}

// contractLibraries answers Get from the libraries table (updateLibrary returns the library).
type contractLibraries struct {
	authTestLibraries
	q *appq.Queries
}

func (c *contractLibraries) Get(ctx context.Context, id int64) (model.Library, error) {
	l, err := c.q.GetLibrary(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, err
	}
	out := model.Library{ID: l.ID, Name: l.Name, Kind: l.Kind}
	if l.DefaultRating.Valid {
		r := int(l.DefaultRating.Int64)
		out.DefaultRating = &r
	}
	return out, nil
}

// M6 administration: the default rating of an audio library (AN-2) and the podcast subscriptions.
func TestContractAudioAdmin(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	a := f.Audio
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	pods := metadata.New(f.DB, jobs.NewQueue(f.DB.Write, nil), nil, t.TempDir(), transcode.Runner{}, log)
	o := pf.opts
	o.Admin = admin.New(f.DB, pf.a, &contractLibraries{q: appq.New(f.DB.Read)}, ring, &authTestTMDB{}, func(string) string { return "" }, log).
		WithPodcasts(pods)
	h := NewHandler(o)

	// The default rating 12: books rated by the library follow, P6 loses them.
	rec := send(t, h, http.MethodPatch, fmt.Sprintf("%s/admin/libraries/%d", BasePath, a.AudiobookLibraryID), "p18", `{"defaultRating":12}`, nil)
	var lib struct {
		DefaultRating *int `json:"defaultRating"`
	}
	decode(t, rec, &lib)
	if rec.Code != http.StatusOK || lib.DefaultRating == nil || *lib.DefaultRating != 12 {
		t.Fatalf("updateLibrary: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+a.Books[0].ID, "p6", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("book rated by the library for P6 after the default 12: %d", rec.Code)
	}
	// A manual rating below the new default is raised to it (AN-2 "only stricter", M6 review m-8).
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+a.Books[1].ID, "p6", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("book rated 6 by hand for P6 after the default 12: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+a.Books[1].ID, "p12", "", nil); rec.Code != http.StatusOK {
		t.Errorf("book rated 6 by hand, raised to 12, for P12: %d", rec.Code)
	}
	for _, tc := range []struct {
		method, path, body, code string
		status                   int
	}{
		{http.MethodPatch, fmt.Sprintf("/admin/libraries/%d", f.LibraryID()), `{"defaultRating":12}`, "validation_failed", http.StatusUnprocessableEntity},
		{http.MethodPut, "/admin/items/" + a.Books[3].ID + "/rating", `{"rating":6}`, "rating_below_library_default", http.StatusUnprocessableEntity},
		{http.MethodPost, "/admin/podcasts", fmt.Sprintf(`{"libraryId":%d,"feedUrl":"%s/feed.xml"}`, f.LibraryID(), a.EnclosureServer.URL), "validation_failed", http.StatusUnprocessableEntity},
		{http.MethodPost, "/admin/podcasts", fmt.Sprintf(`{"libraryId":%d,"feedUrl":"ftp://example.test/feed.xml"}`, a.PodcastLibraryID), "validation_failed", http.StatusUnprocessableEntity},
		{http.MethodPost, "/admin/podcasts", fmt.Sprintf(`{"libraryId":%d,"feedUrl":"%s/feed.xml"}`, a.PodcastLibraryID, a.EnclosureServer.URL), "feed_exists", http.StatusConflict},
		{http.MethodPost, "/admin/podcasts", fmt.Sprintf(`{"libraryId":%d,"feedUrl":"%s/other.xml"}`, a.PodcastLibraryID, a.EnclosureServer.URL), "feed_unreachable", http.StatusUnprocessableEntity},
		{http.MethodPost, "/admin/podcasts", fmt.Sprintf(`{"libraryId":%d,"feedUrl":"%s/episodes/episode-q0.mp3"}`, a.PodcastLibraryID, a.EnclosureServer.URL), "feed_invalid", http.StatusUnprocessableEntity},
	} {
		rec := send(t, h, tc.method, BasePath+tc.path, "p18", tc.body, nil)
		if rec.Code != tc.status || problemCode(t, rec) != tc.code {
			t.Errorf("%s %s %s: %d %s, want %d %s", tc.method, tc.path, tc.body, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}
	// The subscriptions of the fixture, refresh and removal.
	var list []struct {
		ID           string `json:"id"`
		EpisodeCount int    `json:"episodeCount"`
		FeedURL      string `json:"feedUrl"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/admin/podcasts", "p18", "", nil), &list)
	counts := map[string]int{}
	for _, p := range list {
		counts[p.ID] = p.EpisodeCount
	}
	if len(list) != 3 || counts[a.Podcast.ID] != 7 || counts[a.Empty.ID] != 0 || counts[a.ExplicitPodcast.ID] != 1 {
		t.Errorf("listPodcasts = %+v", list)
	}
	if rec := send(t, h, http.MethodPost, BasePath+"/admin/podcasts/"+a.Podcast.ID+"/refresh", "p18", "", nil); rec.Code != http.StatusAccepted {
		t.Errorf("refreshPodcast: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/podcasts/"+a.Podcast.ID, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("deletePodcast: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/podcasts/"+a.Podcast.ID, "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("second deletePodcast: %d", rec.Code)
	}
	for _, m := range []vistest.Movie{a.Podcast, a.EnclosureOnly} {
		if rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, "p18", "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s after unsubscribe: %d", m.Label, rec.Code)
		}
	}
}
