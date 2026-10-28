package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// roleOf reads the account row straight from the table, so the test judges the stored state and
// not what the service claims it did.
func roleOf(t *testing.T, s *Service, id int64) appq.Account {
	t.Helper()
	acc, err := s.read.GetAccount(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

// ADR 0014 lock 2: the owner is neither demotable nor deletable. The check runs before the
// last-administrator check, so the message stays the telling one even when the owner is also the
// only administrator.
func TestSetAccountRoleOwnerProtected(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()

	if err := s.SetAccountRole(ctx, sess.Account.ID, model.RoleUser); !errors.Is(err, model.ErrOwnerProtected) {
		t.Fatalf("demoting the owner = %v, want ErrOwnerProtected", err)
	}
	if err := s.SetAccountRole(ctx, sess.Account.ID, model.RoleAdmin); !errors.Is(err, model.ErrOwnerProtected) {
		t.Errorf("re-appointing the owner = %v, want ErrOwnerProtected", err)
	}
	acc := roleOf(t, s, sess.Account.ID)
	if acc.Role != model.RoleAdmin || acc.IsOwner != 1 {
		t.Errorf("owner after the refusals = role %q, is_owner %d", acc.Role, acc.IsOwner)
	}
}

// ADR 0014 lock 1: a demotion that would leave no administrator is refused. The owner cannot be
// demoted at all, so the case shows itself on a second administrator that is the last one after
// the owner has been removed from the picture - here by having only that one plain administrator
// row besides the protected owner.
func TestSetAccountRoleLastAdmin(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()
	second := addGuestAccount(t, s, "zweiter@example.ch", 12, true, nil)

	// Appointed, then demoted again: with the owner still an administrator this is allowed.
	if err := s.SetAccountRole(ctx, second.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAccountRole(ctx, second.ID, model.RoleUser); err != nil {
		t.Fatalf("demoting the second administrator = %v, want success", err)
	}

	// Now the other way round: the owner is the only administrator left and is protected, so the
	// server can never end up without one.
	if err := s.SetAccountRole(ctx, sess.Account.ID, model.RoleUser); !errors.Is(err, model.ErrOwnerProtected) {
		t.Fatalf("demoting the last administrator = %v, want ErrOwnerProtected", err)
	}
	var admins int
	if err := s.db.QueryRow(`SELECT count(*) FROM accounts WHERE role = 'admin'`).Scan(&admins); err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Errorf("administrators = %d, want 1", admins)
	}
}

// The last-administrator lock is unreachable while an owner exists: the owner is an administrator
// and can be neither demoted nor deleted, so one is always left. The guard exists for a database
// that has lost its owner badge, and this test puts the server in exactly that state to prove the
// guard is more than a comment.
func TestLastAdminGuardWithoutAnOwner(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts SET is_owner = 0`); err != nil {
		t.Fatal(err)
	}

	if err := s.SetAccountRole(ctx, sess.Account.ID, model.RoleUser); !errors.Is(err, model.ErrLastAdmin) {
		t.Errorf("demoting the only administrator = %v, want ErrLastAdmin", err)
	}
	if err := s.DeleteAccount(ctx, sess.Account.ID, nil); !errors.Is(err, model.ErrLastAdmin) {
		t.Errorf("deleting the only administrator = %v, want ErrLastAdmin", err)
	}
	if acc := roleOf(t, s, sess.Account.ID); acc.Role != model.RoleAdmin {
		t.Errorf("the administrator survived as %q", acc.Role)
	}
}

// An administrator has no access limits: it is fixed at FSK 18 and all libraries, so the table
// CHECK of migration 00022 can never be reached from the API (ADR 0014, decision 12 f).
func TestUpdateAccountAccessRefusesAnAdministrator(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()
	second := addGuestAccount(t, s, "zugriff@example.ch", 12, true, nil)
	if err := s.SetAccountRole(ctx, second.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	for _, id := range []int64{sess.Account.ID, second.ID} {
		if err := s.UpdateAccountAccess(ctx, id, model.AccountAccessUpdate{MaxRating: num(12)}); !errors.Is(err, model.ErrAdminLimitsFixed) {
			t.Errorf("account %d = %v, want ErrAdminLimitsFixed", id, err)
		}
		if acc := roleOf(t, s, id); acc.MaxRating != 18 || acc.AllLibraries != 1 {
			t.Errorf("account %d = FSK %d, all_libraries %d; want 18 and 1", id, acc.MaxRating, acc.AllLibraries)
		}
	}
}

// ADR 0014, decision 12 f: the appointment lifts the account to FSK 18 and all libraries, and the
// demotion leaves both where they are. The profiles of the account keep their own levels.
func TestSetAccountRoleLiftsAndKeepsTheLimits(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	owner(t, s)
	ctx := context.Background()
	lib := addLibrary(t, s, "Filme")
	target := addGuestAccount(t, s, "kandidat@example.ch", 12, false, []int64{lib})
	child, err := s.write.InsertProfile(ctx, appq.InsertProfileParams{
		AccountID: target.ID, Name: "Kind", MaxRating: 6, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetAccountRole(ctx, target.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	acc := roleOf(t, s, target.ID)
	if acc.Role != model.RoleAdmin || acc.IsOwner != 0 || acc.MaxRating != 18 || acc.AllLibraries != 1 {
		t.Errorf("appointed account = %+v, want role admin, is_owner 0, 18, all libraries", acc)
	}
	prof, err := s.read.GetProfile(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prof.MaxRating != 6 {
		t.Errorf("the child profile of an administrator = FSK %d, want 6", prof.MaxRating)
	}

	if err := s.SetAccountRole(ctx, target.ID, model.RoleUser); err != nil {
		t.Fatal(err)
	}
	acc = roleOf(t, s, target.ID)
	if acc.Role != model.RoleUser || acc.MaxRating != 18 || acc.AllLibraries != 1 {
		t.Errorf("demoted account = %+v, want role user with the limits left at 18/all", acc)
	}
}

// ADR 0014: the demotion revokes every session of the account, so no browser keeps an
// administration page whose requests would all answer 403. The appointment revokes nothing.
func TestSetAccountRoleRevokesSessionsOnDemotionOnly(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	owner(t, s)
	ctx := context.Background()
	target := addGuestAccount(t, s, "sitzung@example.ch", 18, true, nil)
	prof, err := s.write.InsertProfile(ctx, appq.InsertProfileParams{
		AccountID: target.ID, Name: "Haupt", MaxRating: 18, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	start := func() string {
		t.Helper()
		_, token, err := s.StartSession(ctxOn(deviceA), model.Account{ID: target.ID, Email: target.Email},
			model.Profile{ID: prof.ID, AccountID: target.ID, Name: prof.Name, IsPrimary: true}, "ua")
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	alive := func(token string) bool {
		t.Helper()
		_, _, err := s.Resolve(ctxOn(deviceA), token)
		return err == nil
	}

	appointed := start()
	if err := s.SetAccountRole(ctx, target.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if !alive(appointed) {
		t.Error("the appointment revoked a session; nobody loses anything by being appointed")
	}

	demoted := start()
	if err := s.SetAccountRole(ctx, target.ID, model.RoleUser); err != nil {
		t.Fatal(err)
	}
	if alive(demoted) || alive(appointed) {
		t.Error("the demotion left a session of the account alive")
	}
}

// ADR 0014 lock 4: an invitation never creates an administrator. The role is a literal in
// queries/app/access.sql, so nothing in a request body can reach it.
func TestAcceptedInvitationIsNeverAnAdministrator(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()
	inv, err := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 18, AllLibraries: true})
	if err != nil {
		t.Fatal(err)
	}
	guest, _, err := s.AcceptInvitation(ctxOn(deviceA), model.AcceptInvitation{
		Token: inv.Token, Email: "neu@example.ch", Password: "neu-password-1", ProfileName: "Neu",
	}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	acc := roleOf(t, s, guest.Account.ID)
	if acc.Role != model.RoleUser || acc.IsOwner != 0 {
		t.Errorf("accepted invitation = role %q, is_owner %d; want user and 0", acc.Role, acc.IsOwner)
	}
}

// An unknown id and an unknown role value are refused before anything is written.
func TestSetAccountRoleRefusesUnknownIDAndRole(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()

	if err := s.SetAccountRole(ctx, 9999, model.RoleAdmin); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown id = %v, want ErrNotFound", err)
	}
	var verr *model.ValidationError
	if err := s.SetAccountRole(ctx, sess.Account.ID, "owner"); !errors.As(err, &verr) {
		t.Errorf("the burnt value owner = %v, want a validation error", err)
	}
}

// accountLibs reads the library rights of an account straight from the table.
func accountLibs(t *testing.T, s *Service, id int64) []int64 {
	t.Helper()
	libs, err := s.read.ListAccountLibraries(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return libs
}
