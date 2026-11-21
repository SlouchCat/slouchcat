package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/auth"
)

// Findings of the reviews of concept 27 stage 1, part 1 (code review M-1, m-1, m-3, m-6, m-8;
// security review L-1, L-2).

// Only the scheme Bearer enters the bearer way: a browser behind a reverse proxy with Basic
// authentication keeps its cookie, and login still sets it (code review M-1, security review L-2).
func TestOtherAuthorizationSchemesKeepTheBrowser(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	basic := map[string]string{"Authorization": "Basic YTpi"}
	rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "p18", "", basic)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie with Basic: %d %s", rec.Code, rec.Body.String())
	}
	e := newAuthEnv(t)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	req := httptest.NewRequest(http.MethodPost, BasePath+"/auth/login",
		strings.NewReader(`{"email":"marco@example.ch","password":"`+authTestPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(CSRFHeader, CSRFHeaderValue)
	req.Header.Set("Authorization", "Basic YTpi")
	login := httptest.NewRecorder()
	e.h.ServeHTTP(login, req)
	if login.Code != http.StatusOK || len(cookiesNamed(login, SessionCookie)) != 1 {
		t.Errorf("login with Basic: %d, cookies %v", login.Code, login.Header().Values("Set-Cookie"))
	}
}

// RFC 6750 allows more than one space after the scheme (code review m-8).
func TestBearerWithSeveralSpaces(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	rec := sendBearer(t, pf.h, http.MethodGet, BasePath+"/auth/session", "", "", map[string]string{"Authorization": "Bearer   " + pf.pa.tokens["app"]})
	if rec.Code != http.StatusOK {
		t.Errorf("Bearer with three spaces: %d %s", rec.Code, rec.Body.String())
	}
}

// A browser token sent as bearer is refused without keeping its session alive, and an app token in
// the cookie does not touch its session either (code review m-6).
func TestWrongCarrierDoesNotTouchTheSession(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	app := pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	browser := pf.sessions["p18"]
	if _, err := pf.f.DB.Write.Exec(`UPDATE sessions SET last_seen_at = 1 WHERE id IN (?, ?)`, browser.ID, app.ID); err != nil {
		t.Fatal(err)
	}
	if rec := sendBearer(t, pf.h, http.MethodGet, BasePath+"/auth/session", "p18", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("browser token as bearer: %d", rec.Code)
	}
	if rec := send(t, pf.h, http.MethodGet, BasePath+"/auth/session", "app", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("app token in the cookie: %d", rec.Code)
	}
	var touched int
	if err := pf.f.DB.Read.QueryRow(`SELECT count(*) FROM sessions WHERE id IN (?, ?) AND last_seen_at <> 1`, browser.ID, app.ID).Scan(&touched); err != nil {
		t.Fatal(err)
	}
	if touched != 0 {
		t.Errorf("%d sessions touched by a token on the wrong carrier", touched)
	}
}

// An app older than the server accepts still learns its revocations and signs out (code review m-1).
func TestAppTooOldMayRevokeAndSignOut(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	o := pf.opts
	o.MinClientVersions = map[string]string{"mobile": "9.0.0", "tv": "9.0.0"}
	h := NewHandler(o)
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	old := map[string]string{ClientHeader: "mobile/1.0.0"}
	if rec := sendBearer(t, h, http.MethodGet, BasePath+"/auth/profiles", "app", "", old); rec.Code != http.StatusUpgradeRequired {
		t.Errorf("listMyProfiles of an old app: %d, want 426", rec.Code)
	}
	if rec := sendBearer(t, h, http.MethodPost, BasePath+"/offline/revocations", "app", `{"ids":["01J9KEPT000000000000000000"]}`, old); rec.Code != http.StatusOK {
		t.Errorf("checkOfflineRevocations of an old app: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendBearer(t, h, http.MethodPost, BasePath+"/auth/logout", "app", "", old); rec.Code != http.StatusNoContent {
		t.Errorf("logout of an old app: %d %s", rec.Code, rec.Body.String())
	}
}

// With a bearer session the revocation list is the session's device, whatever the header names
// (security review L-1).
func TestRevocationsOfABearerSessionUseItsDevice(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	app := pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	foreign := "a1b2c3d4e5f60718293a4b5c6d7e8f90"
	mine, theirs := "01J9AAAA000000000000000000", "01J9BBBB000000000000000000"
	for id, device := range map[string]string{mine: app.DeviceID, theirs: foreign} {
		if _, err := pf.f.DB.Write.Exec(`INSERT INTO download_tombstones (id, device_id, removed_at) VALUES (?, ?, 1)`, id, device); err != nil {
			t.Fatal(err)
		}
	}
	rec := sendBearer(t, pf.h, http.MethodPost, BasePath+"/offline/revocations", "app", `{"ids":["`+mine+`","`+theirs+`"]}`,
		map[string]string{DeviceHeader: foreign})
	var out gen.RevocationList
	decode(t, rec, &out)
	if rec.Code != http.StatusOK || strings.Join(out.Revoked, ",") != mine {
		t.Errorf("revoked %v (%d), want only the session device's %s", out.Revoked, rec.Code, mine)
	}
}

// Signing a device out under «Meine Geräte» ends its streams: the stream URL of a running playback
// stops working (code review m-1 of stage 1 part 2).
func TestSignOutMyDeviceEndsTheStreams(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	app := pf.newBearerSession(t, "phone", pf.f.P18, auth.AppDevicePhone)
	rec := sendBearer(t, h, http.MethodPost, BasePath+"/items/"+pf.f.Rated(0).ID+"/play", "phone",
		`{"capabilities":{"video":["h264"],"audio":["aac"],"hls":"mse"}}`, nil)
	var play struct {
		StreamURL string `json:"streamUrl"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &play) != nil {
		t.Fatalf("startPlayback: %d %s", rec.Code, rec.Body.String())
	}
	if s := sendBearer(t, h, http.MethodGet, play.StreamURL, "", "", map[string]string{"Range": "bytes=0-9"}); s.Code != http.StatusPartialContent {
		t.Fatalf("stream before the sign-out: %d", s.Code)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/account/devices/"+app.DeviceID, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("signOutMyDevice: %d %s", rec.Code, rec.Body.String())
	}
	s := sendBearer(t, h, http.MethodGet, play.StreamURL, "", "", map[string]string{"Range": "bytes=0-9"})
	if s.Code != http.StatusUnauthorized && s.Code != http.StatusNotFound {
		t.Errorf("stream after the sign-out: %d, want 401 or 404", s.Code)
	}
}

// selectProfile and startPlayback with a bearer token, up to the stream with st (code review m-3).
func TestBearerSelectProfileAndPlay(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	pf.newBearerSession(t, "app", pf.f.P18, auth.AppDevicePhone)
	pf.newBearerSession(t, "tv", pf.f.P18, auth.AppDeviceTV)
	for _, token := range []string{"app", "tv"} {
		rec := sendBearer(t, h, http.MethodPost, BasePath+"/auth/profiles/"+strconv.FormatInt(pf.f.P12.ID, 10)+"/select", token, `{}`, nil)
		var info gen.SessionInfo
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &info) != nil || info.Profile == nil || info.Profile.Id != pf.f.P12.ID {
			t.Fatalf("%s selectProfile: %d %s", token, rec.Code, rec.Body.String())
		}
		noCookie(t, token+" selectProfile", rec)
		m := pf.f.Rated(0)
		rec = sendBearer(t, h, http.MethodPost, BasePath+"/items/"+m.ID+"/play", token,
			`{"capabilities":{"video":["h264"],"audio":["aac"],"hls":"mse"}}`, nil)
		var play struct {
			Mode      string `json:"mode"`
			StreamURL string `json:"streamUrl"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &play) != nil || !strings.Contains(play.StreamURL, "st=") {
			t.Fatalf("%s startPlayback: %d %s", token, rec.Code, rec.Body.String())
		}
		noCookie(t, token+" startPlayback", rec)
		stream := sendBearer(t, h, http.MethodGet, play.StreamURL, "", "", map[string]string{"Range": "bytes=0-9"})
		if stream.Code != http.StatusPartialContent || stream.Body.Len() != 10 {
			t.Errorf("%s stream with st (%s): %d, %d bytes", token, play.Mode, stream.Code, stream.Body.Len())
		}
	}
}
