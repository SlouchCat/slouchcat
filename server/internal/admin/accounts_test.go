package admin

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// A-2: the account overview lists the main account first, then the guests newest first, each with
// profiles (primary first), library rights, last use and channel switches.
func TestListAccounts(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	owner, _, err := f.auth.CreateOwner(ctx, "marco@example.ch", fakePassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	q := appq.New(f.db.Write)
	lib, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Filme", Kind: "movies", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	var guests []int64
	for i, email := range []string{"older@example.ch", "newer@example.ch"} {
		g, err := q.InsertInvitedAccount(ctx, appq.InsertInvitedAccountParams{Email: email, PasswordHash: "x", MaxRating: 12,
			AllLibraries: int64(i), InvitedBy: sql.NullInt64{Int64: owner.ID, Valid: true}, Now: time.Now().UnixMilli() + int64(i)*1000})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: g.ID, Name: "Gast", MaxRating: 12, Language: "de",
			AllLibraries: 1, IsDefault: 1, Now: 1}); err != nil {
			t.Fatal(err)
		}
		if err := systemq.New(f.db.Write).ChannelEnsureDefaults(ctx, systemq.ChannelEnsureDefaultsParams{AccountID: g.ID, Now: 1}); err != nil {
			t.Fatal(err)
		}
		guests = append(guests, g.ID)
	}
	if err := q.InsertAccountLibrary(ctx, appq.InsertAccountLibraryParams{AccountID: guests[0], LibraryID: lib.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertProfile(ctx, appq.InsertProfileParams{AccountID: owner.ID, Name: "Kind", MaxRating: 6, Language: "de",
		AllLibraries: 1, Now: 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.TouchAccount(ctx, appq.TouchAccountParams{ID: owner.ID, Now: sql.NullInt64{Int64: 1_760_000_000_000, Valid: true}}); err != nil {
		t.Fatal(err)
	}

	list, err := f.svc.ListAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != owner.ID || list[1].ID != guests[1] || list[2].ID != guests[0] {
		t.Fatalf("order = %+v, want owner, newer guest, older guest", list)
	}
	o := list[0]
	if o.Role != model.RoleAdmin || !o.IsOwner || !o.AllLibraries || len(o.LibraryIDs) != 0 || len(o.Profiles) != 2 || !o.Profiles[0].IsPrimary ||
		o.Profiles[1].Name != "Kind" || o.Profiles[1].MaxRating != 6 || o.LastSeenAt == nil || o.LastSeenAt.UnixMilli() != 1_760_000_000_000 {
		t.Errorf("owner = %+v", o)
	}
	older := list[2]
	if older.AllLibraries || len(older.LibraryIDs) != 1 || older.LibraryIDs[0] != lib.ID || older.MaxRating != 12 || older.LastSeenAt != nil {
		t.Errorf("older guest = %+v", older)
	}
	if len(older.Channels) != 3 || older.Channels[0].Channel != "email" || !older.Channels[0].Enabled || older.Channels[1].Enabled {
		t.Errorf("guest channels = %+v, want e-mail on (5.2), push and telegram off", older.Channels)
	}
	if list[1].LibraryIDs == nil || list[1].Channels == nil || list[1].Profiles == nil {
		t.Errorf("empty lists must not be nil: %+v", list[1])
	}
}
