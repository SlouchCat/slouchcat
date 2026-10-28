package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setAccountRole over HTTP (ADR 0014): who gets through is the middleware's answer, what happens
// then is the service's. TestContractOwnerOperations already covers 401, the four 403s and the
// 404; this test drives the two successful directions and the conflict on real accounts of the
// fixture (the owner, a second administrator and an account that administers nothing).
func TestContractSetAccountRole(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	setRole := func(token string, id int64, want string) *httptest.ResponseRecorder {
		t.Helper()
		path := fmt.Sprintf("%s/admin/accounts/%d/role", BasePath, id)
		return send(t, h, http.MethodPut, path, token, `{"role":"`+want+`"}`, nil)
	}
	stored := func(id int64) (role string, isOwner, rating, allLibraries int) {
		t.Helper()
		err := f.DB.Read.QueryRow(`SELECT role, is_owner, max_rating, all_libraries FROM accounts WHERE id = ?`, id).
			Scan(&role, &isOwner, &rating, &allLibraries)
		if err != nil {
			t.Fatal(err)
		}
		return role, isOwner, rating, allLibraries
	}

	// The owner appoints the account that administers nothing. Decision 12 f: the appointment
	// lifts the ceiling and the libraries, because the administration views read system-wide.
	if rec := setRole("p18", f.Guest.ID, "admin"); rec.Code != http.StatusNoContent {
		t.Fatalf("appoint: %d %s", rec.Code, rec.Body.String())
	}
	if role, owner, rating, all := stored(f.Guest.ID); role != "admin" || owner != 0 || rating != 18 || all != 1 {
		t.Errorf("appointed account = role %q, is_owner %d, FSK %d, all_libraries %d; want admin, 0, 18, 1", role, owner, rating, all)
	}

	// Taking the role away leaves the lifted limits where they are (decision 12 f).
	if rec := setRole("p18", f.Guest.ID, "user"); rec.Code != http.StatusNoContent {
		t.Fatalf("demote: %d %s", rec.Code, rec.Body.String())
	}
	if role, _, rating, all := stored(f.Guest.ID); role != "user" || rating != 18 || all != 1 {
		t.Errorf("demoted account = role %q, FSK %d, all_libraries %d; want user with 18 and 1 kept", role, rating, all)
	}

	// The second administrator may be demoted while the owner is still one.
	if rec := setRole("p18", f.Admin2.ID, "user"); rec.Code != http.StatusNoContent {
		t.Fatalf("demote the second administrator: %d %s", rec.Code, rec.Body.String())
	}

	// The owner itself is refused, and the message stays the telling one even though the owner is
	// now the only administrator left: owner_protected is checked before last_admin.
	rec := setRole("p18", f.Account.ID, "user")
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "owner_protected" {
		t.Fatalf("demote the owner: %d %s", rec.Code, rec.Body.String())
	}
	if role, owner, _, _ := stored(f.Account.ID); role != "admin" || owner != 1 {
		t.Errorf("owner after the refusal = role %q, is_owner %d", role, owner)
	}

	// Lock 3: a second administrator is not the owner and never reaches the service. Its sessions
	// died with the demotion above, so the test signs it in again after the re-appointment.
	if rec := setRole("p18", f.Admin2.ID, "admin"); rec.Code != http.StatusNoContent {
		t.Fatalf("re-appoint the second administrator: %d %s", rec.Code, rec.Body.String())
	}
	pf.newSession(t, "pa2again", f.PA2)
	again := setRole("pa2again", f.Guest.ID, "admin")
	if again.Code != http.StatusForbidden || problemCode(t, again) != "owner_required" {
		t.Errorf("second administrator appointing: %d %s", again.Code, again.Body.String())
	}
}
