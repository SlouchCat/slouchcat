package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// hasOfflinePinOf returns hasOfflinePin of every profile in GET /profiles for token.
func hasOfflinePinOf(t *testing.T, h http.Handler, token string) map[int64]bool {
	t.Helper()
	rec := send(t, h, http.MethodGet, BasePath+"/profiles", token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list profiles: %d %s", rec.Code, rec.Body.String())
	}
	for _, leak := range []string{"salt", "iterations", "offline_pin", "offlinePin\""} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("profile list carries %q: %s", leak, rec.Body.String())
		}
	}
	var list []gen.Profile
	decode(t, rec, &list)
	out := map[int64]bool{}
	for _, p := range list {
		out[p.Id] = p.HasOfflinePin
	}
	return out
}

// AN-19 over HTTP: the primary profile sets and removes the offline PIN of a profile of its
// account; Profile.hasOfflinePin follows (also in the session); a short PIN is 422, a non-primary
// profile 403 primary_profile_required, a profile of another account 404.
func TestOfflinePinEndpoints(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	path := func(id int64) string { return fmt.Sprintf("%s/profiles/%d/offline-pin", BasePath, id) }

	if rec := send(t, h, http.MethodPut, path(f.P12.ID), "p18", `{"pin":"135790"}`, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	for id, has := range hasOfflinePinOf(t, h, "p18") {
		if has != (id == f.P12.ID) {
			t.Errorf("profile %d: hasOfflinePin %v", id, has)
		}
	}
	rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "p12", "", nil)
	var info gen.SessionInfo
	decode(t, rec, &info)
	if info.Profile == nil || !info.Profile.HasOfflinePin {
		t.Errorf("session of P12 = %s", rec.Body.String())
	}

	rec = send(t, h, http.MethodPut, path(f.P12.ID), "p18", `{"pin":"1234"}`, nil)
	var problem gen.Problem
	decode(t, rec, &problem)
	if rec.Code != http.StatusUnprocessableEntity || problem.Code != "validation_failed" || problem.Errors == nil ||
		!slices.Equal(*problem.Errors, []gen.FieldError{{Field: "pin", Code: "invalid_pin"}}) {
		t.Errorf("short PIN: %d %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := send(t, h, method, path(f.P12.ID), "p12", `{"pin":"246802"}`, nil)
		if rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
			t.Errorf("%s as P12: %d %s", method, rec.Code, rec.Body.String())
		}
		rec = send(t, h, method, path(f.PG.ID), "p18", `{"pin":"246802"}`, nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s for the guest's profile: %d %s", method, rec.Code, rec.Body.String())
		}
	}

	for range 2 { // removing is idempotent
		if rec := send(t, h, http.MethodDelete, path(f.P12.ID), "p18", "", nil); rec.Code != http.StatusNoContent {
			t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
		}
	}
	if hasOfflinePinOf(t, h, "p18")[f.P12.ID] {
		t.Error("hasOfflinePin after the removal")
	}
}

// offlineEventJSON is one event of an upload body.
func offlineEventJSON(profileID int64, itemID, playbackID string, seq int64, at time.Time, positionMs int64) string {
	return fmt.Sprintf(`{"profileId":%d,"itemId":%q,"playbackId":%q,"positionMs":%d,"durationMs":%d,"playedMs":10000,"seq":%d,"state":"playing","clientTime":%q}`,
		profileID, itemID, playbackID, positionMs, vistest.DurationMs, seq, at.UTC().Format(time.RFC3339Nano))
}

func offlineBatch(sentAt time.Time, events []string) string {
	return fmt.Sprintf(`{"sentAt":%q,"events":[%s]}`, sentAt.UTC().Format(time.RFC3339Nano), strings.Join(events, ","))
}

type offlineResult struct {
	Accepted, Duplicates, Rejected int
	Results                        []string
}

// O-5 over HTTP: ten minutes of offline playback of P12 (device clock 2 h behind), uploaded from a
// session of the account without a chosen profile, move the progress P12 sees on the detail page
// to the last position (±10 s). A guest's session only books its own profiles. The upload of 500
// events passes the body limit; 501 events and a batch without sentAt are 422.
func TestUploadOfflineEvents(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(12)
	now := time.Now()
	device := now.Add(-2 * time.Hour)
	start := device.Add(-15 * time.Minute)
	var events []string
	for i := int64(1); i <= 60; i++ {
		events = append(events, offlineEventJSON(f.P12.ID, m.ID, "01JC0000000000000000000001", i, start.Add(time.Duration(i)*10*time.Second), 1_800_000+i*10_000))
	}
	rec := send(t, h, http.MethodPost, BasePath+"/offline/events", "none", offlineBatch(device, events), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var res offlineResult
	decode(t, rec, &res)
	if res.Accepted != 60 || res.Duplicates != 0 || res.Rejected != 0 || len(res.Results) != 60 {
		t.Fatalf("result = %+v", res)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/items/"+m.ID, "p12", "", nil)
	var detail struct {
		Progress *gen.Progress `json:"progress"`
	}
	decode(t, rec, &detail)
	last := int64(1_800_000 + 60*10_000)
	if detail.Progress == nil || detail.Progress.PositionMs < last-10_000 || detail.Progress.PositionMs > last+10_000 {
		t.Fatalf("progress of P12 after the upload = %s", rec.Body.String())
	}
	// The corrected time of the last event lies 5 minutes before the upload.
	if want := now.Add(-5 * time.Minute); detail.Progress.UpdatedAt.Sub(want).Abs() > 2*time.Second {
		t.Errorf("progress updated at %v, want about %v", detail.Progress.UpdatedAt, want)
	}

	// The guest's session: its own profile counts, the owner's profile and an invisible title not.
	rec = send(t, h, http.MethodPost, BasePath+"/offline/events", "pg", offlineBatch(now, []string{
		offlineEventJSON(f.PG.ID, f.Rated(6).ID, "01JC0000000000000000000002", 1, now, 10_000),
		offlineEventJSON(f.P18.ID, f.Rated(6).ID, "01JC0000000000000000000003", 1, now, 10_000),
		offlineEventJSON(f.PG.ID, f.Rated(16).ID, "01JC0000000000000000000004", 1, now, 10_000),
	}), nil)
	decode(t, rec, &res)
	if rec.Code != http.StatusOK || !slices.Equal(res.Results, []string{"accepted", "rejected", "rejected"}) {
		t.Errorf("guest upload: %d %s", rec.Code, rec.Body.String())
	}

	// 500 events (about 150 KB) pass the body limit, 501 are a validation error.
	var many []string
	for i := int64(1); i <= 501; i++ {
		many = append(many, offlineEventJSON(f.P18.ID, f.Rated(18).ID, "01JC0000000000000000000005", i, now.Add(time.Duration(i-600)*10*time.Second), i*10_000))
	}
	body := offlineBatch(now, many[:500])
	if len(body) <= MaxBodyBytes {
		t.Fatalf("500 events are only %d bytes", len(body))
	}
	rec = send(t, h, http.MethodPost, BasePath+"/offline/events", "p18", body, nil)
	decode(t, rec, &res)
	if rec.Code != http.StatusOK || res.Accepted != 500 {
		t.Errorf("500 events (%d bytes): %d %+v", len(body), rec.Code, res)
	}
	for name, body := range map[string]string{
		"501 events": offlineBatch(now, many),
		"no sentAt":  `{"events":[` + many[0] + `]}`,
		"no events":  offlineBatch(now, nil),
	} {
		rec := send(t, h, http.MethodPost, BasePath+"/offline/events", "p18", body, nil)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := send(t, h, http.MethodPost, BasePath+"/offline/events", "", offlineBatch(now, many[:1]), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a session: %d", rec.Code)
	}

	// The device of the uploading session is stored with the events.
	var device1 string
	if err := f.DB.Read.QueryRow(`SELECT device_id FROM progress_events WHERE playback_session_id = '01JC0000000000000000000001' LIMIT 1`).Scan(&device1); err != nil {
		t.Fatal(err)
	}
	if device1 == "" || device1 != pf.sessions["none"].DeviceID {
		t.Errorf("stored device %q, session device %q", device1, pf.sessions["none"].DeviceID)
	}
}
