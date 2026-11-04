package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Contract 1.11.0 (concept 13 11.4, F-13) over HTTP: every profile reads and replaces its own
// preferences, another profile keeps its own, the primary profile sees them in listProfiles, bad
// values answer 422 with the field, a session without profile 403 profile_required, and
// resetMySeriesTracks forgets the series languages of the active profile only.
func TestHTTPPlaybackPrefs(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	const path = BasePath + "/me/playback-prefs"
	get := func(token string) gen.PlaybackPrefs {
		t.Helper()
		rec := send(t, h, http.MethodGet, path, token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("get for %s: %d %s", token, rec.Code, rec.Body.String())
		}
		var p gen.PlaybackPrefs
		decode(t, rec, &p)
		return p
	}
	equal := func(a, b gen.PlaybackPrefs) bool {
		return slices.Equal(a.AudioLanguages, b.AudioLanguages) && a.SubtitleMode == b.SubtitleMode &&
			slices.Equal(a.SubtitleLanguages, b.SubtitleLanguages)
	}
	none := gen.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: gen.SubtitleModeForced, SubtitleLanguages: []string{}}
	if got := get("p6"); !equal(got, none) {
		t.Errorf("defaults = %+v", got)
	}

	want := gen.PlaybackPrefs{AudioLanguages: []string{"en", "de"}, SubtitleMode: gen.SubtitleModeForeign, SubtitleLanguages: []string{"de"}}
	rec := send(t, h, http.MethodPut, path, "p6", `{"audioLanguages":["en","de"],"subtitleMode":"foreign","subtitleLanguages":["de"]}`, nil)
	var put gen.PlaybackPrefs
	if decode(t, rec, &put); rec.Code != http.StatusOK || !equal(put, want) {
		t.Fatalf("put = %d %s", rec.Code, rec.Body.String())
	}
	if got := get("p6"); !equal(got, want) {
		t.Errorf("get after put = %+v", got)
	}
	if got := get("p12"); !equal(got, none) {
		t.Errorf("another profile changed with p6: %+v", got)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/profiles", "p18", "", nil)
	var profiles []gen.Profile
	decode(t, rec, &profiles)
	i := slices.IndexFunc(profiles, func(p gen.Profile) bool { return p.Id == f.P6.ID })
	if rec.Code != http.StatusOK || i < 0 || !slices.Equal(profiles[i].AudioLanguages, want.AudioLanguages) ||
		profiles[i].SubtitleMode != want.SubtitleMode || !slices.Equal(profiles[i].SubtitleLanguages, want.SubtitleLanguages) {
		t.Errorf("listProfiles = %d %s", rec.Code, rec.Body.String())
	}

	for body, wantProblem := range map[string]string{
		`{"subtitleMode":"off","subtitleLanguages":[]}`:                              "audioLanguages required",
		`{"audioLanguages":["xx"],"subtitleMode":"off","subtitleLanguages":[]}`:      "audioLanguages invalid_value",
		`{"audioLanguages":[],"subtitleMode":"never","subtitleLanguages":[]}`:        "subtitleMode invalid_value",
		`{"audioLanguages":[],"subtitleMode":"off","subtitleLanguages":["de","de"]}`: "subtitleLanguages invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPut, path, "p6", body, nil)); got != wantProblem {
			t.Errorf("put %s: %s, want %s", body, got, wantProblem)
		}
	}
	// The primary profile changes the fields of another profile through updateProfile.
	rec = send(t, h, http.MethodPatch, BasePath+"/profiles/"+strconv.FormatInt(f.P12.ID, 10), "p18", `{"subtitleMode":"always","subtitleLanguages":["fr"]}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("updateProfile: %d %s", rec.Code, rec.Body.String())
	}
	if got := get("p12"); !equal(got, gen.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: gen.SubtitleModeAlways, SubtitleLanguages: []string{"fr"}}) {
		t.Errorf("p12 after updateProfile = %+v", got)
	}
	if got := fieldProblem(t, send(t, h, http.MethodPatch, BasePath+"/profiles/"+strconv.FormatInt(f.P12.ID, 10), "p18", `{"audioLanguages":["deu"]}`, nil)); got != "audioLanguages invalid_value" {
		t.Errorf("updateProfile with a three-letter code: %s", got)
	}

	for _, req := range []struct{ method, path string }{
		{http.MethodGet, path}, {http.MethodPut, path}, {http.MethodDelete, path + "/series"},
	} {
		rec := send(t, h, req.method, req.path, "none", `{"audioLanguages":[],"subtitleMode":"off","subtitleLanguages":[]}`, nil)
		if rec.Code != http.StatusForbidden || problemCode(t, rec) != "profile_required" {
			t.Errorf("%s %s without profile: %d %s", req.method, req.path, rec.Code, rec.Body.String())
		}
	}

	// resetMySeriesTracks deletes the entries of the active profile, not those of another one.
	ctx := context.Background()
	q := appq.New(f.DB.Write)
	series := f.Rated(6).ID
	for _, p := range []int64{f.P6.ID, f.P12.ID} {
		if err := q.UpsertSeriesTracks(ctx, appq.UpsertSeriesTracksParams{ProfileID: p, SeriesID: series, AudioLanguage: "en", Now: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if rec := send(t, h, http.MethodDelete, path+"/series", "p6", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := q.GetSeriesTracks(ctx, appq.GetSeriesTracksParams{ProfileID: f.P6.ID, SeriesID: series}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("entry of p6 after the reset: %v", err)
	}
	if row, err := q.GetSeriesTracks(ctx, appq.GetSeriesTracksParams{ProfileID: f.P12.ID, SeriesID: series}); err != nil || row.AudioLanguage != "en" {
		t.Errorf("entry of p12 after the reset of p6: %+v, %v", row, err)
	}
	if rec := send(t, h, http.MethodDelete, path+"/series", "p6", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("reset without entries: %d %s", rec.Code, rec.Body.String())
	}
}

// The new fields of PlaybackRequest (contract 1.11.0) over HTTP: original and 2160p have no rung
// and a remembered cap has no effect in the playback yet, so all three are refused instead of
// played as something else; subtitleOff together with a subtitle track is a contradiction;
// subtitleOff and rememberForSeries alone start without subtitles.
func TestHTTPPlaybackRequestFields(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	film := f.Rated(6).ID
	text := pf.tracks[f.Rated(6).FileID].Text
	for body, want := range map[string]string{
		`{"quality":"original"}`: "quality invalid_value",
		`{"quality":"2160p"}`:    "quality invalid_value",
		`{"qualityCap":"720p"}`:  "qualityCap invalid_value",
		`{"subtitleOff":true,"subtitleTrack":` + strconv.FormatInt(text, 10) + `}`: "subtitleOff not_allowed",
	} {
		_, code, p := play(t, h, "p6", film, body)
		if code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field+" "+p.Errors[0].Code != want {
			t.Errorf("play %s: %d %+v, want %s", body, code, p, want)
		}
	}
	info, code, p := play(t, h, "p6", film, `{"subtitleOff":true,"rememberForSeries":true,"capabilities":`+chromeCaps+`}`)
	if code != http.StatusOK || info.SubtitleURL != "" {
		t.Errorf("play with subtitleOff: %d %+v %+v", code, info, p)
	}
}
