package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The settings of stages 4 and 5 in the wish settings: profile tag, prefix, download progress and
// the pages of the wishes area, stored under the keys package wishes reads.
func TestWishSettingsStage4And5(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ws, err := f.svc.WishSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ws.ProfileTags || ws.TagPrefix != "sc-" || !ws.ShowProgress || !ws.DiscoverPages {
		t.Errorf("defaults = %+v", ws)
	}
	for _, bad := range []string{"", "SC-", "sc_", "elfzeichen-x", "sc ", "ä-"} {
		var verr *model.ValidationError
		if _, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{TagPrefix: ptr(bad)}); !errors.As(err, &verr) {
			t.Errorf("prefix %q: %v, want a validation error", bad, err)
		}
	}
	ws, err = f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{TagPrefix: ptr("kino-2"), ProfileTags: ptr(false),
		ShowProgress: ptr(false), DiscoverPages: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if ws.ProfileTags || ws.TagPrefix != "kino-2" || ws.ShowProgress || ws.DiscoverPages {
		t.Errorf("after set = %+v", ws)
	}
	for key, want := range map[string]string{
		"wishes.profile_tags": "0", "wishes.tag_prefix": "kino-2", "wishes.show_progress": "0", "wishes.discover_pages": "0",
	} {
		var got string
		if err := f.db.Read.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&got); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", key, got, err, want)
		}
	}
	// Other fields stay when only one changes; an unreadable prefix falls back to the default.
	if ws, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{ShowProgress: ptr(true)}); err != nil || !ws.ShowProgress || ws.TagPrefix != "kino-2" || ws.ProfileTags {
		t.Errorf("partial update = %+v, %v", ws, err)
	}
	execSeerr(t, f, `UPDATE settings SET value = 'BAD PREFIX' WHERE key = 'wishes.tag_prefix'`)
	if ws, _ := f.svc.WishSettings(ctx); ws.TagPrefix != "sc-" {
		t.Errorf("unreadable prefix = %q", ws.TagPrefix)
	}
}
