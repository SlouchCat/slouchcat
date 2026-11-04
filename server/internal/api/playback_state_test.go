package api

import (
	"net/http"
	"testing"
)

// getPlaybackSessionState (concept 13): the browser session that started a playback reads
// active, then stopped with stopped_by admin after the administration ended it, although the
// stream was never requested again; another browser session, a session without profile and an
// unknown id get 404 not_found (403 profile_required without profile).
func TestHTTPPlaybackSessionState(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	info, code, _ := play(t, h, "p6", f.Rated(6).ID, `{}`)
	if code != http.StatusOK {
		t.Fatalf("start = %d", code)
	}
	path := BasePath + "/playback/sessions/" + info.PlaybackSessionID + "/state"
	state := func(token string) (int, string) {
		t.Helper()
		rec := send(t, h, http.MethodGet, path, token, "", nil)
		if rec.Code != http.StatusOK {
			return rec.Code, problemCode(t, rec)
		}
		return rec.Code, rec.Body.String()
	}

	if code, body := state("p6"); code != http.StatusOK || body != `{"state":"active"}`+"\n" {
		t.Errorf("running: %d %s", code, body)
	}
	for _, token := range []string{"p18", "pg"} {
		if code, body := state(token); code != http.StatusNotFound || body != "not_found" {
			t.Errorf("%s reads the playback of p6: %d %s", token, code, body)
		}
	}
	if code, body := state("none"); code != http.StatusForbidden || body != "profile_required" {
		t.Errorf("session without profile: %d %s", code, body)
	}
	rec := send(t, h, http.MethodGet, BasePath+"/playback/sessions/01JUNKNOWN0000000000000000/state", "p6", "", nil)
	if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("unknown id: %d %s", rec.Code, rec.Body.String())
	}

	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/streams/"+info.PlaybackSessionID, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("stop by the administration: %d %s", rec.Code, rec.Body.String())
	}
	if code, body := state("p6"); code != http.StatusOK || body != `{"state":"stopped","stoppedBy":"admin"}`+"\n" {
		t.Errorf("after the administration's stop: %d %s", code, body)
	}
	if code, _ := state("p18"); code != http.StatusNotFound {
		t.Errorf("the administrator reads the state of a foreign playback: %d", code)
	}
}
