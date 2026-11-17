package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Concept 25 stage 1 (contract 1.41.0) against a fake Seerr: the owner saves the connection, tests
// it, switches the wishes on, chooses Seerr users by hand; the problem codes of the operations and
// no key in any answer.
func TestContractSeerr(t *testing.T) {
	pf := newProfileFixture(t)
	fake := seerrtest.New(t)
	fake.AddUser(seerrtest.User{ID: 3, Email: "kinder@example.test", Username: "kinder", Permissions: seerrtest.PermRequest})
	call := func(method, path, body string, want int) *json.Decoder {
		t.Helper()
		rec := send(t, pf.h, method, BasePath+path, "p18", body, nil)
		if rec.Code != want {
			t.Fatalf("%s %s: status %d (%s), want %d", method, path, rec.Code, rec.Body.String(), want)
		}
		for _, secret := range []string{seerrtest.APIKey, "argon2id"} {
			if strings.Contains(rec.Body.String(), secret) {
				t.Errorf("%s %s: the answer carries %q", method, path, secret)
			}
		}
		assertNoSecrets(t, path, rec)
		return json.NewDecoder(rec.Body)
	}
	code := func(method, path, body string, status int, want string) {
		t.Helper()
		rec := send(t, pf.h, method, BasePath+path, "p18", body, nil)
		if rec.Code != status || problemCode(t, rec) != want {
			t.Errorf("%s %s: %d %s, want %d %s", method, path, rec.Code, rec.Body.String(), status, want)
		}
	}

	code(http.MethodPut, "/admin/wishes/settings", `{"enabled":true}`, http.StatusConflict, "seerr_not_tested")
	code(http.MethodPost, "/admin/wishes/mappings/users/refresh", ``, http.StatusUnprocessableEntity, "seerr_not_configured")
	code(http.MethodPost, "/admin/seerr/test", `{}`, http.StatusUnprocessableEntity, "seerr_not_configured")

	var st struct {
		Enabled bool   `json:"enabled"`
		URL     string `json:"url"`
		APIKey  struct {
			Source string `json:"source"`
			Hint   string `json:"hint"`
		} `json:"apiKey"`
	}
	body := `{"enabled":true,"url":"` + fake.URL + `/api/v1","apiKey":"` + seerrtest.APIKey + `"}`
	if err := call(http.MethodPut, "/admin/seerr", body, http.StatusOK).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.URL != fake.URL || st.APIKey.Source != "stored" || st.APIKey.Hint != seerrtest.APIKey[len(seerrtest.APIKey)-4:] {
		t.Errorf("saved = %+v", st)
	}
	// Another host without the key: refused on save and on test.
	code(http.MethodPut, "/admin/seerr", `{"url":"http://other.invalid:5055"}`, http.StatusUnprocessableEntity, "seerr_key_required")
	code(http.MethodPost, "/admin/seerr/test", `{"url":"http://other.invalid:5055"}`, http.StatusUnprocessableEntity, "seerr_key_required")
	code(http.MethodPut, "/admin/seerr", `{"url":"http://user:pw@seerr.lan"}`, http.StatusUnprocessableEntity, "validation_failed")

	var res struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Key, Level string
		} `json:"checks"`
	}
	if err := call(http.MethodPost, "/admin/seerr/test", ``, http.StatusOK).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(res.Checks) < 10 {
		t.Errorf("test = %+v", res)
	}
	var ws struct {
		Enabled  bool   `json:"enabled"`
		Unmapped string `json:"unmapped"`
	}
	if err := call(http.MethodPut, "/admin/wishes/settings", `{"enabled":true}`, http.StatusOK).Decode(&ws); err != nil {
		t.Fatal(err)
	}
	if !ws.Enabled || ws.Unmapped != "fallback" {
		t.Errorf("settings = %+v", ws)
	}
	code(http.MethodPut, "/admin/wishes/settings", `{"defaultLimit":{"movie":{"limit":101,"days":7},"tv":{"limit":5,"days":7}}}`,
		http.StatusUnprocessableEntity, "validation_failed")

	var status struct {
		State string `json:"state"`
		Log   []any  `json:"log"`
	}
	if err := call(http.MethodGet, "/admin/seerr/status", ``, http.StatusOK).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.State != "connected" || len(status.Log) == 0 {
		t.Errorf("status = %+v", status)
	}
	code(http.MethodPost, "/admin/seerr/instance", `{"action":"confirm"}`, http.StatusConflict, "instance_unchanged")

	// The mirror as the job seerr.users would write it (the fixture runs no workers).
	if _, err := pf.f.DB.Write.ExecContext(context.Background(),
		`INSERT INTO seerr_users (seerr_id, display_name, email, user_type, permissions, fetched_at) VALUES (3, 'kinder', 'kinder@example.test', 2, 32, 1)`); err != nil {
		t.Fatal(err)
	}
	guest, owner := strconv.FormatInt(pf.f.Guest.ID, 10), strconv.FormatInt(pf.f.Account.ID, 10)
	var m struct {
		State         string `json:"state"`
		SeerrUserID   int    `json:"seerrUserId"`
		PasswordState string `json:"passwordState"`
	}
	if err := call(http.MethodPut, "/admin/wishes/mappings/accounts/"+guest, `{"seerrUserId":3}`, http.StatusOK).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.State != "manual" || m.SeerrUserID != 3 || m.PasswordState != "off" {
		t.Errorf("mapping = %+v", m)
	}
	code(http.MethodPut, "/admin/wishes/mappings/accounts/"+owner, `{"seerrUserId":3}`, http.StatusConflict, "seerr_user_taken")
	code(http.MethodPut, "/admin/wishes/mappings/accounts/"+owner, `{"seerrUserId":77}`, http.StatusNotFound, "not_found")
	var all struct {
		Accounts []struct {
			AccountID int64 `json:"accountId"`
		} `json:"accounts"`
		Total int `json:"total"`
	}
	if err := call(http.MethodGet, "/admin/wishes/mappings", ``, http.StatusOK).Decode(&all); err != nil {
		t.Fatal(err)
	}
	if all.Total == 0 || len(all.Accounts) != all.Total {
		t.Errorf("mappings = %+v", all)
	}
	var users []struct {
		ID       int `json:"id"`
		MappedTo *struct {
			AccountID int64 `json:"accountId"`
		} `json:"mappedTo"`
	}
	if err := call(http.MethodGet, "/admin/wishes/mappings/users", ``, http.StatusOK).Decode(&users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].MappedTo == nil || users[0].MappedTo.AccountID != pf.f.Guest.ID {
		t.Errorf("users = %+v", users)
	}
	if err := call(http.MethodPut, "/admin/wishes/mappings/accounts/"+guest, `{"seerrUserId":null}`, http.StatusOK).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.State != "none" {
		t.Errorf("after unlinking: %+v", m)
	}
	call(http.MethodPost, "/admin/wishes/mappings/users/refresh", ``, http.StatusAccepted)

	// «In Seerr eröffnen» (build step 1.5a): Seerr unreachable is 202 waiting, then 200 opened.
	fake.SetFailure("*", seerrtest.Failure{Status: http.StatusServiceUnavailable})
	m.SeerrUserID, m.PasswordState = 0, ""
	if err := call(http.MethodPost, "/admin/wishes/accounts/"+owner+"/provision", ``, http.StatusAccepted).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.State != "waiting" || m.SeerrUserID != 0 {
		t.Errorf("unreachable = %+v", m)
	}
	fake.SetFailure("*", seerrtest.Failure{})
	if err := call(http.MethodPost, "/admin/wishes/accounts/"+guest+"/provision", ``, http.StatusOK).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.State != "created" || m.SeerrUserID == 0 || m.PasswordState != "next_login" || fake.Calls("POST /user") != 1 {
		t.Errorf("opened = %+v", m)
	}
	code(http.MethodPost, "/admin/wishes/accounts/9999/provision", ``, http.StatusNotFound, "not_found")
	var counts struct {
		Link   *int `json:"link"`
		Create *int `json:"create"`
	}
	if err := call(http.MethodPost, "/admin/wishes/accounts/provision", ``, http.StatusAccepted).Decode(&counts); err != nil {
		t.Fatal(err)
	}
	if counts.Link == nil || counts.Create == nil || *counts.Create < 1 {
		t.Errorf("all = %+v", counts)
	}
	call(http.MethodPut, "/admin/wishes/settings", `{"provisionAccounts":false}`, http.StatusOK)
	code(http.MethodPost, "/admin/wishes/accounts/"+owner+"/provision", ``, http.StatusConflict, "provision_disabled")
	code(http.MethodPost, "/admin/wishes/accounts/provision", ``, http.StatusConflict, "provision_disabled")
}
