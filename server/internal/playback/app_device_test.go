package playback

import (
	"context"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 27 4.2: the device of an app or device session keeps the name and type of its sign-in;
// a playback start stores the capabilities and the owner, as for a browser, but not the name.
func TestPlaybackKeepsTheAppDeviceName(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	ctx := context.Background()
	clock := &manualClock{now: time.Date(2026, 11, 21, 18, 0, 0, 0, time.UTC)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTranscoder(newFakeTranscoder()), WithTokenKey(testKey))
	a := auth.New(f.DB, vistest.Logger())
	for _, c := range []struct{ kind, name string }{{auth.AppDevicePhone, "Pixel 9"}, {auth.AppDeviceTV, "Wohnzimmer"}} {
		session, _, err := a.StartAppSession(ctx, f.Account, f.P18, c.name, c.kind, "", "okhttp/5")
		if err != nil {
			t.Fatal(err)
		}
		caps := capsChromeLinux // reports the name «Chrome 156 · Linux» and the type desktop
		info, err := s.Start(ctx, session, f.Rated(0).ID, model.PlaybackRequest{Capabilities: &caps})
		if err != nil {
			t.Fatal(err)
		}
		if info.Device == nil || info.Device.ID != session.DeviceID || info.Device.Name != c.name {
			t.Errorf("%s: device = %+v, want %s named %q", c.kind, info.Device, session.DeviceID, c.name)
		}
		var name, typ string
		var capsAt, accountID int64
		if err := f.DB.Read.QueryRow(`SELECT name, type, capabilities_at, account_id FROM devices WHERE id = ?`, session.DeviceID).
			Scan(&name, &typ, &capsAt, &accountID); err != nil {
			t.Fatal(err)
		}
		if name != c.name || typ != c.kind || capsAt != clock.Now().UnixMilli() || accountID != f.Account.ID {
			t.Errorf("%s: device row %q %q %d %d", c.kind, name, typ, capsAt, accountID)
		}
	}
}
