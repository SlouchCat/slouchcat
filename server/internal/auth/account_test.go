package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

func ptr[T any](v T) *T { return &v }

// A-2: the account changes its e-mail and password with the current password; a new password ends
// the other sessions and keeps the calling one.
func TestUpdateAccount(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.10")
	s, _ := newService(t)
	acc, prof, err := s.CreateOwner(ctx, testEmail, testPassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.write.InsertInvitedAccount(ctx, appq.InsertInvitedAccountParams{
		Email: "guest@example.ch", PasswordHash: "x", MaxRating: 12, AllLibraries: 1, InvitedBy: sql.NullInt64{Int64: acc.ID, Valid: true}, Now: 1,
	}); err != nil {
		t.Fatal(err)
	}
	calling, callingToken, err := s.StartSession(ctx, acc, prof, "calling")
	if err != nil {
		t.Fatal(err)
	}
	_, otherToken, err := s.StartSession(ctx, acc, prof, "other")
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		upd   model.AccountUpdate
		want  error
		field string // a validation error of this field
	}{
		"nothing to change":  {upd: model.AccountUpdate{CurrentPassword: testPassword}, field: "email:required"},
		"invalid e-mail":     {upd: model.AccountUpdate{CurrentPassword: testPassword, Email: ptr("not-an-email")}, field: "email:invalid_email"},
		"short password":     {upd: model.AccountUpdate{CurrentPassword: testPassword, NewPassword: ptr("short")}, field: "newPassword:too_short"},
		"no current":         {upd: model.AccountUpdate{NewPassword: ptr("a long new password")}, field: "currentPassword:required"},
		"wrong current":      {upd: model.AccountUpdate{CurrentPassword: "wrong password", Email: ptr("new@example.ch")}, want: model.ErrInvalidCredentials},
		"e-mail of a guest":  {upd: model.AccountUpdate{CurrentPassword: testPassword, Email: ptr(" Guest@Example.CH ")}, want: model.ErrEmailTaken},
		"same e-mail, no-op": {upd: model.AccountUpdate{CurrentPassword: testPassword, Email: ptr(testEmail)}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := s.UpdateAccount(ctx, calling, tc.upd)
			var verr *model.ValidationError
			switch {
			case tc.field != "":
				if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field+":"+verr.Fields[0].Code != tc.field {
					t.Fatalf("err = %v, want the validation error %s", err, tc.field)
				}
			case !errors.Is(err, tc.want):
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	updated, err := s.UpdateAccount(ctx, calling, model.AccountUpdate{
		CurrentPassword: testPassword, Email: ptr(" Neu@Example.CH "), NewPassword: ptr("a brand new password"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Account.Email != "neu@example.ch" || updated.ID != calling.ID {
		t.Errorf("session = %+v, want the calling session with the new e-mail", updated)
	}
	if _, _, err := s.Resolve(ctx, callingToken); err != nil {
		t.Errorf("calling session after the password change: %v, want valid", err)
	}
	if _, _, err := s.Resolve(ctx, otherToken); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("other session after the password change: %v, want revoked", err)
	}
	if _, _, err := s.Login(ctx, "neu@example.ch", testPassword, ""); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("login with the old password: %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := s.Login(ctx, "neu@example.ch", "a brand new password", ""); err != nil {
		t.Errorf("login with the new e-mail and password: %v", err)
	}

	// An e-mail change alone keeps the other sessions.
	_, third, err := s.StartSession(ctx, acc, prof, "third")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAccount(ctx, updated, model.AccountUpdate{CurrentPassword: "a brand new password", Email: ptr("marco@example.ch")}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, third); err != nil {
		t.Errorf("another session after an e-mail change: %v, want valid", err)
	}
}

// Every call counts against the attempt limit of the client, so the current password cannot be
// guessed faster than a login.
func TestUpdateAccountAttemptLimit(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.11")
	s, _ := newService(t)
	acc, prof, err := s.CreateOwner(ctx, testEmail, testPassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	session, _, err := s.StartSession(ctx, acc, prof, "ua")
	if err != nil {
		t.Fatal(err)
	}
	var last error
	for range attemptBurst + 1 {
		_, last = s.UpdateAccount(ctx, session, model.AccountUpdate{CurrentPassword: "guess", Email: ptr("x@example.ch")})
	}
	if !errors.Is(last, model.ErrTooManyAttempts) {
		t.Fatalf("attempt %d: %v, want ErrTooManyAttempts", attemptBurst+1, last)
	}
}
