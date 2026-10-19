package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract tests of the series tree, extras, versions, cast and trailers (M3, design 4.1) on the
// vistest fixture: what the detail of each kind lists for a profile, in which order.

// treeRef is the part of an ItemRef the tree tests read.
type treeRef struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	ParentID      string `json:"parentId"`
	SeasonNumber  *int   `json:"seasonNumber"`
	EpisodeNumber *int   `json:"episodeNumber"`
	EpisodeEnd    *int   `json:"episodeEnd"`
	SeriesTitle   string `json:"seriesTitle"`
	ExtraKind     string `json:"extraKind"`
	ChildCount    *int   `json:"childCount"`
}

// treeDetail is the part of an ItemDetail the tree tests read.
type treeDetail struct {
	treeRef
	Seasons     []treeRef `json:"seasons"`
	Episodes    []treeRef `json:"episodes"`
	Extras      []treeRef `json:"extras"`
	Series      *treeRef  `json:"series"`
	Season      *treeRef  `json:"season"`
	NextEpisode *treeRef  `json:"nextEpisode"`
	Cast        []struct {
		PersonID  int64  `json:"personId"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		Character string `json:"character"`
	} `json:"cast"`
	TrailerCount int `json:"trailerCount"`
}

func getDetail(t *testing.T, h http.Handler, token, id string) treeDetail {
	t.Helper()
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s getItem %s: status %d (%s)", token, id, rec.Code, rec.Body.String())
	}
	var d treeDetail
	decode(t, rec, &d)
	return d
}

func refIDs(refs []treeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.ID)
	}
	return out
}

func refID(r *treeRef) string {
	if r == nil {
		return ""
	}
	return r.ID
}

func intIs(v *int, want int) bool { return v != nil && *v == want }

func TestContractSeriesTree(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	show := f.Show

	// Series: seasons by number with the specials last, its extras, the trailer count.
	d := getDetail(t, h, "p18", show.Series.ID)
	if want := []string{show.Season.ID, show.Specials.ID}; !slices.Equal(refIDs(d.Seasons), want) {
		t.Errorf("P18 seasons = %v, want %v (specials last)", refIDs(d.Seasons), want)
	}
	if !slices.Equal(refIDs(d.Extras), []string{show.Extra.ID}) || d.Extras[0].ExtraKind != "behind_the_scenes" || d.Extras[0].ParentID != show.Series.ID {
		t.Errorf("series extras = %+v", d.Extras)
	}
	if !intIs(d.ChildCount, 2) || d.TrailerCount != 2 || d.Episodes != nil {
		t.Errorf("series childCount %v, trailerCount %d, episodes %v", d.ChildCount, d.TrailerCount, d.Episodes)
	}
	if s := d.Seasons[0]; !intIs(s.SeasonNumber, 1) || !intIs(s.ChildCount, 3) || s.SeriesTitle != "Film SS" || s.ParentID != show.Series.ID {
		t.Errorf("P18 season 1 ref = %+v", s)
	}
	if s := d.Seasons[1]; !intIs(s.SeasonNumber, 0) || !intIs(s.ChildCount, 1) {
		t.Errorf("specials ref = %+v", s)
	}

	// Season: P12 does not see E2 (own rating 16), neither in the list nor in the count.
	d = getDetail(t, h, "p12", show.Season.ID)
	if want := []string{show.E1.ID, show.E3.ID}; !slices.Equal(refIDs(d.Episodes), want) {
		t.Errorf("P12 episodes = %v, want %v", refIDs(d.Episodes), want)
	}
	if !intIs(d.ChildCount, 2) || refID(d.Series) != show.Series.ID || d.SeriesTitle != "Film SS" || d.Seasons != nil {
		t.Errorf("P12 season: childCount %v, series %v, seriesTitle %q", d.ChildCount, refID(d.Series), d.SeriesTitle)
	}
	if e3 := d.Episodes[1]; !intIs(e3.EpisodeNumber, 3) || !intIs(e3.EpisodeEnd, 4) || !intIs(e3.SeasonNumber, 1) || e3.SeriesTitle != "Film SS" {
		t.Errorf("multi-episode ref = %+v", e3)
	}
	d = getDetail(t, h, "p18", show.Season.ID)
	if want := []string{show.E1.ID, show.E2.ID, show.E3.ID}; !slices.Equal(refIDs(d.Episodes), want) || !intIs(d.ChildCount, 3) {
		t.Errorf("P18 episodes = %v (count %v), want %v", refIDs(d.Episodes), d.ChildCount, want)
	}

	// Episode: season, series and the next visible episode (E2 for P18, E3 for P12; none after
	// the multi-episode file, whose last episode is 4; the special continues with season 1).
	for _, tc := range []struct {
		token string
		ep    vistest.Movie
		next  string
	}{
		{"p18", show.E1, show.E2.ID},
		{"p12", show.E1, show.E3.ID},
		{"p18", show.E2, show.E3.ID},
		{"p18", show.E3, ""},
		{"p12", show.Special, show.E1.ID},
	} {
		d := getDetail(t, h, tc.token, tc.ep.ID)
		if refID(d.NextEpisode) != tc.next {
			t.Errorf("%s nextEpisode of %s = %q, want %q", tc.token, tc.ep.Label, refID(d.NextEpisode), tc.next)
		}
		if refID(d.Series) != show.Series.ID || refID(d.Season) != tc.ep.ParentID || d.SeriesTitle != "Film SS" || d.ParentID != tc.ep.ParentID {
			t.Errorf("%s episode %s: series %q, season %q, seriesTitle %q", tc.token, tc.ep.Label, refID(d.Series), refID(d.Season), d.SeriesTitle)
		}
		if d.Extras != nil || d.Seasons != nil || d.Episodes != nil {
			t.Errorf("episode %s carries lists of another kind", tc.ep.Label)
		}
	}
	if got := getTitle(t, h, "p6", show.Special.ID); got != http.StatusNotFound {
		t.Errorf("P6 getItem of the 12-rated special: %d, want 404", got)
	}

	// Film: extras by kind (featurette before trailer), cast with the person, trailers.
	m6 := f.Rated(6)
	d = getDetail(t, h, "p6", m6.ID)
	if want := []string{f.Extras[1].ID, f.Extras[0].ID}; !slices.Equal(refIDs(d.Extras), want) {
		t.Errorf("film extras = %v, want %v (by kind)", refIDs(d.Extras), want)
	}
	if len(d.Cast) != 1 || d.Cast[0].PersonID != f.Person.ID || d.Cast[0].Name != f.Person.Name || d.Cast[0].Role != "cast" || d.Cast[0].Character != "Hero" {
		t.Errorf("film cast = %+v", d.Cast)
	}
	if d.TrailerCount != 2 || d.Series != nil || d.Season != nil || d.NextEpisode != nil || d.ChildCount != nil {
		t.Errorf("film detail = %+v", d)
	}
	if d := getDetail(t, h, "p18", f.Rated(18).ID); len(d.Cast) != 1 || d.Cast[0].Role != "director" {
		t.Errorf("director credit = %+v", d.Cast)
	}
	if d := getDetail(t, h, "p6", f.Extras[0].ID); d.ExtraKind != "trailer" || d.ParentID != m6.ID {
		t.Errorf("extra detail = %+v", d.treeRef)
	}

	// Trailers in the profile language first: P6 (en) and P12 (de) on the 6-rated film.
	for token, first := range map[string]string{"p6": "en", "p12": "de"} {
		var list []struct {
			Lang   string `json:"lang"`
			Source string `json:"source"`
		}
		decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+m6.ID+"/trailers", token, "", nil), &list)
		if len(list) != 2 || list[0].Lang != first {
			t.Errorf("%s trailers = %+v, want %s first", token, list, first)
		}
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+f.Rated(18).ID+"/trailers", "p6", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("P6 trailers of the 18-rated film: %d, want 404", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/people/999", "p18", "", nil); rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("unknown person: %d", rec.Code)
	}

	// Versions (B-5): the 1080p file is the primary one, the 720p file second; both labelled.
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+f.Rated(12).ID, "p12", "", nil)
	var versions struct {
		MediaFiles []struct {
			ID           string `json:"id"`
			VersionLabel string `json:"versionLabel"`
		} `json:"mediaFiles"`
	}
	decode(t, rec, &versions)
	if len(versions.MediaFiles) != 2 || versions.MediaFiles[0].VersionLabel != "1080p" || versions.MediaFiles[1].ID != f.Version2FileID ||
		versions.MediaFiles[1].VersionLabel != "720p" {
		t.Errorf("versions = %+v", versions.MediaFiles)
	}
	play := send(t, h, http.MethodPost, BasePath+"/items/"+f.Rated(12).ID+"/play", "p12",
		`{"mediaFileId":"`+f.Version2FileID+`","capabilities":{"video":["h264"],"audio":["aac"],"hls":"mse"}}`, nil)
	if play.Code != http.StatusOK || !strings.Contains(play.Body.String(), f.Version2FileID) {
		t.Errorf("play the 720p version: %d %s", play.Code, play.Body.String())
	}

	// Continue watching shows an episode with its series title.
	progress := `{"positionMs":60000,"durationMs":7200000,"playedMs":60000,"seq":1,"state":"playing","clientTime":"2026-10-18T20:00:00Z"}`
	if rec := send(t, h, http.MethodPost, BasePath+"/items/"+show.E1.ID+"/progress", "p12", progress, nil); rec.Code != http.StatusOK {
		t.Fatalf("progress on E1: %d", rec.Code)
	}
	var page struct {
		Rows []struct {
			Key   string    `json:"key"`
			Items []treeRef `json:"items"`
		} `json:"rows"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/home", "p12", "", nil), &page)
	found := false
	for _, row := range page.Rows {
		for _, it := range row.Items {
			if row.Key == "continue" && it.ID == show.E1.ID {
				found = it.SeriesTitle == "Film SS" && intIs(it.EpisodeNumber, 1)
			}
			if row.Key == "recent" && it.Kind != "movie" && it.Kind != "series" {
				t.Errorf("recent lists a %s", it.Kind)
			}
		}
	}
	if !found {
		t.Errorf("continue row of P12 lacks E1 with its series title: %+v", page.Rows)
	}
}

// Design 1: an extra alone makes no series available and lowers no minimum rating; the extra's
// own reply does not reveal the hidden series.
func TestContractExtrasNeverContainers(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	series, extra := f.AddLoneExtraSeries(t)
	if available, minChild := f.ContainerState(t, series.ID); available || minChild != nil {
		t.Fatalf("series with only an extra: available %v, min_child_rating %v", available, minChild)
	}
	for _, token := range []string{"p18", "p0"} {
		if got := getTitle(t, h, token, series.ID); got != http.StatusNotFound {
			t.Errorf("%s getItem of the series with only an extra: %d, want 404", token, got)
		}
		if slices.Contains(homeIDs(t, h, token), series.ID) {
			t.Errorf("%s start page lists the series with only an extra", token)
		}
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+extra.ID, token, "", nil)
		if strings.Contains(rec.Body.String(), series.ID) {
			t.Errorf("%s: the extra reveals its unavailable series: %s", token, rec.Body.String())
		}
	}
	// The extra under the fixture series does not count as a season either.
	d := getDetail(t, h, "p18", f.Show.Series.ID)
	if !intIs(d.ChildCount, 2) {
		t.Errorf("series childCount = %v, want 2 (the extra does not count)", d.ChildCount)
	}
}
