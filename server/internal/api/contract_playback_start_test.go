package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Contract 1.13.0 (concept 14, step 6): the keyframe index coverage of the libraries, the one-off
// date of a schedule, the pool index, the pause override of a run and the 409 of the HLS endpoints.

// Library.keyframeIndex over HTTP: a video library carries it (zeros without files; the counts
// themselves are tested in package admin, TestLibraryKeyframeIndex), an audio library leaves the
// field out, and toLibrary maps every count.
func TestLibraryKeyframeIndexOverHTTP(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	// The fake library service lists library 1 (Filme, video) since the setup; an audio library
	// joins it.
	e.libs.libs = append(e.libs.libs, model.Library{ID: 2, Name: "Hörbücher", Kind: model.LibraryAudiobooks,
		Roots: []model.LibraryRoot{}, Scan: model.LibraryScan{State: "never"}})
	rec = e.call(t, http.MethodGet, "/api/v1/admin/libraries", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 {
		t.Fatalf("libraries = %s", rec.Body.String())
	}
	if got := string(raw[0]["keyframeIndex"]); got != `{"indexed":0,"pending":0,"unavailable":0,"videoFiles":0}` {
		t.Errorf("video library keyframeIndex = %s", got)
	}
	if got, ok := raw[1]["keyframeIndex"]; ok {
		t.Errorf("audio library carries keyframeIndex %s", got)
	}
	rec = e.call(t, http.MethodGet, "/api/v1/admin/libraries/2", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	if strings.Contains(rec.Body.String(), "keyframeIndex") {
		t.Errorf("getLibrary of the audio library carries keyframeIndex: %s", rec.Body.String())
	}

	got := toLibrary(model.Library{ID: 1, Kind: model.LibraryVideo,
		KeyframeIndex: &model.LibraryKeyframeIndex{VideoFiles: 9, Indexed: 5, Pending: 3, Unavailable: 1}}).KeyframeIndex
	if got == nil || *got != (gen.LibraryKeyframeIndex{VideoFiles: 9, Indexed: 5, Pending: 3, Unavailable: 1}) {
		t.Errorf("toLibrary keyframeIndex = %+v", got)
	}
}

// updateJobSchedule sets the one-off date as wall time of the server's time zone, answers it as
// schedule.once {local, at} and removes it with clearOnce; a date outside now + 1 min .. now + 366
// days is out_of_range, a value that is no wall time invalid_value (422 validation_failed).
func TestJobScheduleOnceOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	path := BasePath + "/admin/job-types/podcast.poll/schedule"
	patch := func(body string) gen.JobTypeInfo {
		t.Helper()
		rec := send(t, pf.h, http.MethodPatch, path, "p18", body, nil)
		wantStatus(t, rec, http.StatusOK)
		var info gen.JobTypeInfo
		decode(t, rec, &info)
		if info.Schedule == nil {
			t.Fatalf("no schedule: %s", rec.Body.String())
		}
		return info
	}
	info := patch(`{"enabled":true}`)
	if info.Schedule.Once != nil {
		t.Fatalf("once before setting it = %+v", info.Schedule.Once)
	}
	loc, err := time.LoadLocation(info.Schedule.TimeZone)
	if err != nil {
		t.Fatal(err)
	}
	local := time.Now().In(loc).Add(3 * time.Hour).Format(jobs.OnceLayout)
	at, err := time.ParseInLocation(jobs.OnceLayout, local, loc)
	if err != nil {
		t.Fatal(err)
	}
	info = patch(`{"onceLocal":"` + local + `"}`)
	if o := info.Schedule.Once; o == nil || o.Local != local || !o.At.Equal(at) || o.At.Location() != time.UTC {
		t.Fatalf("once = %+v, want local %s at %s (UTC)", o, local, at.UTC())
	}
	// The listing shows it too.
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/job-types", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var ov gen.JobOverview
	decode(t, rec, &ov)
	for _, tt := range ov.Types {
		if tt.Type == "podcast.poll" && (tt.Schedule == nil || tt.Schedule.Once == nil || tt.Schedule.Once.Local != local) {
			t.Errorf("listJobTypes: podcast.poll schedule = %+v", tt.Schedule)
		}
	}

	for _, tc := range []struct{ name, value, code string }{
		{"past", time.Now().In(loc).Add(-time.Hour).Format(jobs.OnceLayout), "out_of_range"},
		{"too far", time.Now().In(loc).AddDate(0, 0, 400).Format(jobs.OnceLayout), "out_of_range"},
		{"no wall time", "2026-13-40T25:61", "invalid_value"},
		{"no date", "morgen", "invalid_value"},
	} {
		rec := send(t, pf.h, http.MethodPatch, path, "p18", `{"onceLocal":"`+tc.value+`"}`, nil)
		wantStatus(t, rec, http.StatusUnprocessableEntity)
		var p gen.Problem
		decode(t, rec, &p)
		if p.Code != "validation_failed" || p.Errors == nil || len(*p.Errors) != 1 ||
			(*p.Errors)[0] != (gen.FieldError{Field: "onceLocal", Code: tc.code}) {
			t.Errorf("%s: problem = %s, want onceLocal %s", tc.name, rec.Body.String(), tc.code)
		}
	}
	// A refused date leaves the stored one alone.
	if info := patch(`{}`); info.Schedule.Once == nil || info.Schedule.Once.Local != local {
		t.Errorf("once after refused updates = %+v, want %s", info.Schedule.Once, local)
	}

	// clearOnce wins over onceLocal and removes the date.
	info = patch(`{"clearOnce":true,"onceLocal":"` + local + `"}`)
	if info.Schedule.Once != nil {
		t.Errorf("once after clearOnce = %+v", info.Schedule.Once)
	}
	if body := send(t, pf.h, http.MethodPatch, path, "p18", `{}`, nil).Body.String(); strings.Contains(body, `"once"`) {
		t.Errorf("schedule without a date carries once: %s", body)
	}
}

// The pool index is listed with its one worker, and a run that overrides the pause carries
// overridesPause: true; any other run leaves the field out. It proves the mapping of the handler
// against the fixtures; the wiring of the production pools is covered by cmd/slouchcat
// workers_test.go.
func TestJobPoolIndexAndOverridesPauseOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	ctx := context.Background()
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/job-types", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var ov gen.JobOverview
	decode(t, rec, &ov)
	found := false
	for _, p := range ov.Pools {
		if p.Pool == gen.JobPoolPoolIndex {
			found = p.Workers == 1
		}
	}
	if !found {
		t.Errorf("pools = %+v, want index with one worker", ov.Pools)
	}
	// No type of the fixture overrides its pause for a playback: the field is left out, and the
	// mapping sends it for one that does (probe.keyframes, package library).
	var rawTypes struct {
		Types []map[string]json.RawMessage `json:"types"`
	}
	decode(t, rec, &rawTypes)
	for _, typ := range rawTypes.Types {
		if flag, has := typ["overridesPauseForPlayback"]; has {
			t.Errorf("type %s carries overridesPauseForPlayback %s", typ["type"], flag)
		}
	}
	if got := toJobTypeInfo(model.JobTypeInfo{Type: "probe.keyframes", OverridesPauseForPlayback: true}); got.OverridesPauseForPlayback == nil || !*got.OverridesPauseForPlayback {
		t.Errorf("toJobTypeInfo overridesPauseForPlayback = %v, want true", got.OverridesPauseForPlayback)
	}

	plain, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "download.prepare", IdempotencyKey: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	override, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "download.prepare", IdempotencyKey: "override"})
	if err != nil {
		t.Fatal(err)
	}
	// The queue sets override_pause only for a playback (package library); the column is what the
	// listing maps.
	if _, err := pf.f.DB.Write.ExecContext(ctx, `UPDATE jobs SET override_pause = 1 WHERE id = ?`, override); err != nil {
		t.Fatal(err)
	}
	rec = send(t, pf.h, http.MethodGet, BasePath+"/admin/jobs?type=download.prepare&state=queued", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var raw struct {
		Runs []map[string]json.RawMessage `json:"runs"`
	}
	decode(t, rec, &raw)
	seen := 0
	for _, r := range raw.Runs {
		var id int64
		if err := json.Unmarshal(r["id"], &id); err != nil {
			t.Fatal(err)
		}
		flag, has := r["overridesPause"]
		switch id {
		case override:
			seen++
			if string(flag) != "true" {
				t.Errorf("run %d overridesPause = %s, want true", id, flag)
			}
		case plain:
			seen++
			if has {
				t.Errorf("run %d carries overridesPause %s", id, flag)
			}
		}
	}
	if seen != 2 {
		t.Errorf("listJobRuns = %s, want both runs", rec.Body.String())
	}
}

// The HLS endpoints that rebuild a lost output document 409 playback_source_changed (concept 14,
// 6.4); step 5 answers it. The subtitle track shares their token check and answers it as well.
// The other answers and x-visibility stay as they were.
func TestContractHLSSourceChanged(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	kept := map[string][]string{
		"master.m3u8":           {"200", "401", "404", "503"},
		"media.m3u8":            {"200", "401", "404", "503"},
		"init.mp4":              {"200", "401", "404", "503"},
		"seg/{n}":               {"200", "401", "404", "503"},
		"subtitles/{track}.vtt": {"200", "401", "404"},
	}
	for p, statuses := range kept {
		item, _ := paths["/playback/sessions/{id}/"+p].(map[string]any)
		op, _ := item["get"].(map[string]any)
		if vis, _ := op["x-visibility"].(string); vis != "scoped" {
			t.Errorf("%s: x-visibility %q, want scoped", p, vis)
		}
		responses, _ := op["responses"].(map[string]any)
		conflict, _ := responses["409"].(map[string]any)
		if ref, _ := conflict["$ref"].(string); ref != "#/components/responses/StreamSourceChanged" {
			t.Errorf("%s: 409 = %v, want StreamSourceChanged", p, conflict)
		}
		for _, status := range statuses {
			if _, ok := responses[status]; !ok {
				t.Errorf("%s: response %s is gone", p, status)
			}
		}
	}
	components, _ := doc["components"].(map[string]any)
	responses, _ := components["responses"].(map[string]any)
	changed, _ := responses["StreamSourceChanged"].(map[string]any)
	if desc, _ := changed["description"].(string); !strings.Contains(desc, "`playback_source_changed`") {
		t.Errorf("StreamSourceChanged does not name playback_source_changed: %q", desc)
	}
}
