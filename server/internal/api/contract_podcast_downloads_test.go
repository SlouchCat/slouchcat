package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Stored podcast episodes (concept 19 part A, contract 1.27.0): the setting of a podcast library
// with its folder check, the own setting of a podcast, the estimate and the buttons, against the
// services of packages library and metadata as the server wires them.
func TestContractPodcastDownloads(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	a := f.Audio
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.New(f.DB, jobs.NewQueue(f.DB.Write, nil), transcode.Runner{}, log)
	pods := metadata.New(f.DB, jobs.NewQueue(f.DB.Write, nil), nil, t.TempDir(), transcode.Runner{}, log,
		metadata.WithPodcastSettingHooks(lib.PodcastSavable, lib.QueuePodcastSettingChanged),
		metadata.WithPodcastStates(lib.PodcastStates))
	o := pf.opts
	o.Admin = admin.New(f.DB, pf.a, lib, ring, &authTestTMDB{}, func(string) string { return "" }, log).
		WithPodcasts(pods).WithPodcastStorage(lib)
	h := NewHandler(o)
	libPath := fmt.Sprintf("%s/admin/libraries/%d", BasePath, a.PodcastLibraryID)

	type folder struct {
		RootID    int64  `json:"rootId"`
		Writable  bool   `json:"writable"`
		Problem   string `json:"problem"`
		FreeBytes int64  `json:"freeBytes"`
	}
	type library struct {
		PodcastDownloads *struct {
			Mode     string `json:"mode"`
			Scope    string `json:"scope"`
			LastN    int    `json:"lastN"`
			Parallel int    `json:"parallel"`
		} `json:"podcastDownloads"`
		PodcastFolder *folder `json:"podcastFolder"`
	}
	getLib := func() library {
		t.Helper()
		var out library
		rec := send(t, h, http.MethodGet, libPath, "p18", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("getLibrary: %d %s", rec.Code, rec.Body.String())
		}
		decode(t, rec, &out)
		return out
	}

	// Ab Werk «Nur Link» (O-2); without a podcast folder no folder check and no «Speichern».
	l := getLib()
	if d := l.PodcastDownloads; d == nil || d.Mode != "link" || d.Scope != "new" || d.LastN != 10 || d.Parallel != 2 {
		t.Fatalf("podcastDownloads ab Werk = %+v", d)
	}
	if l.PodcastFolder != nil {
		t.Errorf("podcastFolder without a podcast folder = %+v", l.PodcastFolder)
	}
	save := `{"podcastDownloads":{"mode":"save","scope":"all","lastN":10,"parallel":2}}`
	for _, tc := range []struct {
		path, body, code, field string
	}{
		{libPath, save, "root_required", ""},
		{libPath, `{"podcastDownloads":{"mode":"save","scope":"all","lastN":0,"parallel":6}}`, "validation_failed", "podcastDownloads.lastN"},
		{fmt.Sprintf("%s/admin/libraries/%d", BasePath, a.AudiobookLibraryID), save, "validation_failed", "podcastDownloads"},
	} {
		rec := send(t, h, http.MethodPatch, tc.path, "p18", tc.body, nil)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != tc.code {
			t.Errorf("updateLibrary %s %s: %d %s, want 422 %s", tc.path, tc.body, rec.Code, rec.Body.String(), tc.code)
		}
		if tc.field != "" && !problemHasField(t, rec, tc.field) {
			t.Errorf("updateLibrary %s: no field error %s in %s", tc.body, tc.field, rec.Body.String())
		}
	}

	// A podcast folder that passes the folder check: «Speichern» applies.
	dir := t.TempDir()
	pr, err := paths.NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	fsType, err := paths.FSTypeAt(pr, ".")
	if err != nil {
		t.Fatal(err)
	}
	root, err := appq.New(f.DB.Write).InsertLibraryRoot(context.Background(), appq.InsertLibraryRootParams{LibraryID: a.PodcastLibraryID,
		Path: dir, FsType: string(fsType), Watch: "poll", Content: model.RootPodcast,
		Providers: model.ProvidersJSON(model.DefaultProviders(model.RootPodcast)), Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	if l := getLib(); l.PodcastFolder == nil || !l.PodcastFolder.Writable || l.PodcastFolder.Problem != "" || l.PodcastFolder.RootID != root.ID {
		t.Fatalf("podcastFolder of a writable folder = %+v", l.PodcastFolder)
	}
	type podcast struct {
		ID                  string `json:"id"`
		Mode                string `json:"mode"`
		EffectiveMode       string `json:"effectiveMode"`
		EffectiveScope      string `json:"effectiveScope"`
		LastN               *int   `json:"lastN"`
		EpisodeCount        int    `json:"episodeCount"`
		LocalEpisodeCount   int    `json:"localEpisodeCount"`
		UnclearEpisodeCount int    `json:"unclearEpisodeCount"`
		Savable             bool   `json:"savable"`
		Downloads           *struct {
			Queued int `json:"queued"`
		} `json:"downloads"`
	}
	list := func() map[string]podcast {
		t.Helper()
		var rows []podcast
		decode(t, send(t, h, http.MethodGet, BasePath+"/admin/podcasts", "p18", "", nil), &rows)
		out := map[string]podcast{}
		for _, p := range rows {
			out[p.ID] = p
		}
		return out
	}
	if p := list()[a.Podcast.ID]; !p.Savable || p.Mode != "inherit" || p.EffectiveMode != "link" || p.Downloads == nil {
		t.Errorf("listPodcasts before «Speichern»: %+v", p)
	}
	rec := send(t, h, http.MethodPatch, libPath, "p18", save, nil)
	var updated library
	decode(t, rec, &updated)
	if rec.Code != http.StatusOK || updated.PodcastDownloads == nil || updated.PodcastDownloads.Mode != "save" || updated.PodcastDownloads.Scope != "all" {
		t.Fatalf("updateLibrary «Speichern»: %d %s", rec.Code, rec.Body.String())
	}

	// The own setting of a podcast.
	podPath := BasePath + "/admin/podcasts/" + a.Podcast.ID
	rec = send(t, h, http.MethodPatch, podPath, "p18", `{"mode":"link","scope":"last_n","lastN":3}`, nil)
	var p podcast
	decode(t, rec, &p)
	if rec.Code != http.StatusOK || p.Mode != "link" || p.EffectiveMode != "link" || p.EffectiveScope != "last_n" || p.LastN == nil || *p.LastN != 3 {
		t.Errorf("updatePodcast «Nur Link»: %d %s", rec.Code, rec.Body.String())
	}
	if p := list()[a.ExplicitPodcast.ID]; p.EffectiveMode != "save" || p.Mode != "inherit" {
		t.Errorf("a podcast that follows the library after «Speichern»: %+v", p)
	}
	for _, tc := range []struct {
		path, body, code string
		status           int
	}{
		{podPath, `{"lastN":0}`, "validation_failed", http.StatusUnprocessableEntity},
		{podPath, `{"mode":"keep"}`, "validation_failed", http.StatusUnprocessableEntity},
		{BasePath + "/admin/podcasts/" + unknownItemID, `{"mode":"save"}`, "not_found", http.StatusNotFound},
	} {
		rec := send(t, h, http.MethodPatch, tc.path, "p18", tc.body, nil)
		if rec.Code != tc.status || problemCode(t, rec) != tc.code {
			t.Errorf("updatePodcast %s %s: %d %s, want %d %s", tc.path, tc.body, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}

	// The estimate of «Fehlende nachladen» and the button: the episodes without file, which the
	// riegel do not hold back.
	estPath := libPath + "/podcast-downloads/estimate"
	var est struct {
		Episodes  int   `json:"episodes"`
		Bytes     int64 `json:"bytes"`
		FreeBytes int64 `json:"freeBytes"`
	}
	rec = send(t, h, http.MethodPost, estPath, "p18", `{"podcastId":"`+a.Podcast.ID+`","mode":"save","scope":"all"}`, nil)
	decode(t, rec, &est)
	p = list()[a.Podcast.ID]
	want := p.EpisodeCount - p.LocalEpisodeCount - p.UnclearEpisodeCount
	if rec.Code != http.StatusOK || est.Episodes != want || want < 1 || est.FreeBytes <= 0 {
		t.Errorf("estimatePodcastDownloads: %d %s, want %d episodes (%+v)", rec.Code, rec.Body.String(), want, p)
	}
	for _, tc := range []struct {
		path, body, code string
		status           int
	}{
		{estPath, `{"mode":"inherit","scope":"all"}`, "validation_failed", http.StatusUnprocessableEntity},
		{estPath, `{"mode":"save","scope":"last_n","lastN":101}`, "validation_failed", http.StatusUnprocessableEntity},
		{fmt.Sprintf("%s/admin/libraries/%d/podcast-downloads/estimate", BasePath, a.AudiobookLibraryID), `{"mode":"save","scope":"all"}`,
			"not_found", http.StatusNotFound},
		{estPath, `{"podcastId":"` + unknownItemID + `","mode":"save","scope":"all"}`, "not_found", http.StatusNotFound},
	} {
		rec := send(t, h, http.MethodPost, tc.path, "p18", tc.body, nil)
		if rec.Code != tc.status || problemCode(t, rec) != tc.code {
			t.Errorf("estimatePodcastDownloads %s %s: %d %s, want %d %s", tc.path, tc.body, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
	}
	var queued struct {
		Queued int `json:"queued"`
	}
	rec = send(t, h, http.MethodPost, podPath+"/download", "p18", "", nil)
	decode(t, rec, &queued)
	if rec.Code != http.StatusAccepted || queued.Queued != est.Episodes {
		t.Errorf("downloadPodcastEpisodes «Fehlende nachladen»: %d %s, want %d", rec.Code, rec.Body.String(), est.Episodes)
	}
	if p := list()[a.Podcast.ID]; p.Downloads == nil || p.Downloads.Queued != est.Episodes {
		t.Errorf("downloads of the podcast after the button: %+v", p.Downloads)
	}
	rec = send(t, h, http.MethodPost, podPath+"/download", "p18", `{"episodeIds":["`+a.EnclosureOnly.ID+`"]}`, nil)
	decode(t, rec, &queued)
	if rec.Code != http.StatusAccepted || queued.Queued != 1 {
		t.Errorf("downloadPodcastEpisodes «Laden»: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodPost, BasePath+"/admin/podcasts/"+unknownItemID+"/download", "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("downloadPodcastEpisodes of an unknown podcast: %d", rec.Code)
	}

	// A folder on another file system than when it was added (an NAS that is not mounted): the
	// check says not_mounted, the list is not savable, a change to «Speichern» is refused (while
	// the library stores already, its other values still change).
	if rec := send(t, h, http.MethodPatch, libPath, "p18", strings.Replace(save, `"save"`, `"link"`, 1), nil); rec.Code != http.StatusOK {
		t.Fatalf("updateLibrary «Nur Link»: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := f.DB.Write.Exec(`UPDATE library_roots SET fs_type = 'nfs4' WHERE id = ?`, root.ID); err != nil {
		t.Fatal(err)
	}
	if l := getLib(); l.PodcastFolder == nil || l.PodcastFolder.Writable || l.PodcastFolder.Problem != model.PodcastFolderNotMounted {
		t.Errorf("podcastFolder of an unmounted folder = %+v", l.PodcastFolder)
	}
	if p := list()[a.ExplicitPodcast.ID]; p.Savable {
		t.Errorf("podcast on an unmounted folder is savable: %+v", p)
	}
	if rec := send(t, h, http.MethodPatch, libPath, "p18", save, nil); rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "root_not_writable" {
		t.Errorf("updateLibrary «Speichern» on an unmounted folder: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodPatch, podPath, "p18", `{"mode":"save"}`, nil); rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "root_not_writable" {
		t.Errorf("updatePodcast «Speichern» on an unmounted folder: %d %s", rec.Code, rec.Body.String())
	}
}

// problemHasField reports whether a problem lists an error of field.
func problemHasField(t *testing.T, rec *httptest.ResponseRecorder, field string) bool {
	t.Helper()
	var p struct {
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for _, e := range p.Errors {
		if e.Field == field {
			return true
		}
	}
	return false
}
