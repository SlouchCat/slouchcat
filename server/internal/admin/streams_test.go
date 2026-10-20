package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

type fakeStreams struct {
	list    []model.AdminStream
	stopped []string
	ended   []string
}

func (f *fakeStreams) ListStreams(context.Context) ([]model.AdminStream, error) {
	return append([]model.AdminStream(nil), f.list...), nil
}

func (f *fakeStreams) StopStream(_ context.Context, id string) error {
	if id != "01JAAAAAAAAAAAAAAAAAAAAAAA" {
		return model.ErrNotFound
	}
	f.stopped = append(f.stopped, id)
	return nil
}

func (f *fakeStreams) EndDevice(_ context.Context, id string) error {
	f.ended = append(f.ended, id)
	return nil
}

const testDevice = "dddddddddddddddddddddddddddddddd"

// W-6: the overview names titles in plain text (episodes with series and episode code) and the
// duration of the file; stopping goes to the playback service.
func TestAdminStreams(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	if _, err := m.svc.ListStreams(m.ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("without streams: %v", err)
	}
	fs := &fakeStreams{list: []model.AdminStream{
		{ID: "01JAAAAAAAAAAAAAAAAAAAAAAA", ItemID: f.Show.E3.ID, MediaFileID: f.Show.E3.FileID, Mode: "transcode"},
		{ID: "01JBBBBBBBBBBBBBBBBBBBBBBB", ItemID: f.Rated(6).ID, MediaFileID: f.Rated(6).FileID, Mode: "direct"},
		{ID: "01JCCCCCCCCCCCCCCCCCCCCCCC", ItemID: f.Show.Special.ID, MediaFileID: f.Show.Special.FileID, Mode: "remux"},
	}}
	m.svc.WithStreams(fs)
	list, err := m.svc.ListStreams(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Film SS · S01E03-E04 · Film E3", "Film 06", "Film SS · S00E01 · Film P1"}
	for i, st := range list {
		if st.ItemTitle != want[i] || st.DurationMs == nil || *st.DurationMs != vistest.DurationMs {
			t.Errorf("stream %d = %q %v, want %q", i, st.ItemTitle, st.DurationMs, want[i])
		}
	}
	if err := m.svc.StopStream(m.ctx, "01JAAAAAAAAAAAAAAAAAAAAAAA"); err != nil || len(fs.stopped) != 1 {
		t.Errorf("stop: %v %v", err, fs.stopped)
	}
	if err := m.svc.StopStream(m.ctx, "01JZZZZZZZZZZZZZZZZZZZZZZZ"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("stop unknown: %v", err)
	}
}

// W-7: devices with owner, capabilities and running streams; forgetting one revokes its sessions,
// ends its playback and deletes its PIN counters and row.
func TestAdminDevices(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	fs := &fakeStreams{}
	m.svc.WithStreams(fs)
	now := time.Now()
	m.exec(t, `INSERT INTO devices (id, user_agent, created_at, last_seen_at, name, type, account_id, profile_id, capabilities, capabilities_at)
	           VALUES (?, 'Mozilla/5.0', 1, ?, 'Chrome 156 · Linux', 'desktop', ?, ?, '{"video":["h264","hevc"],"audio":["aac"],"hls":"mse","maxHeight":1440,"hdr":true}', 5)`,
		testDevice, now.UnixMilli(), f.Account.ID, f.P18.ID)
	m.exec(t, `INSERT INTO devices (id, created_at, last_seen_at) VALUES ('eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee', 1, 2)`)
	// The reserved device of the PIN ceiling is neither listed nor forgotten.
	m.exec(t, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, ?)`, auth.ReservedDeviceID, now.UnixMilli()+1)
	m.exec(t, `INSERT INTO pin_attempts (profile_id, device_id, failures, locked_until, updated_at) VALUES (?, ?, 20, 0, 1)`, f.P6.ID, auth.ReservedDeviceID)
	session := f.Session(t, f.P18)
	m.exec(t, `UPDATE sessions SET device_id = ? WHERE id = ?`, testDevice, session.ID)
	m.exec(t, `INSERT INTO pin_attempts (profile_id, device_id, failures, locked_until, updated_at) VALUES (?, ?, 2, 0, 1)`, f.P6.ID, testDevice)
	m.exec(t, `INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, device_id, mode, started_at, last_activity_at, expires_at)
	           VALUES ('01JAAAAAAAAAAAAAAAAAAAAAAA', ?, ?, ?, ?, ?, ?, 'direct', ?, ?, ?)`,
		session.ID, f.Account.ID, f.P18.ID, f.Rated(6).ID, f.Rated(6).FileID, testDevice, now.UnixMilli(), now.UnixMilli(), now.Add(time.Hour).UnixMilli())

	devices, err := m.svc.ListDevices(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].ID != testDevice {
		t.Fatalf("devices = %+v", devices)
	}
	d := devices[0]
	if d.Name != "Chrome 156 · Linux" || d.Type != "desktop" || d.OwnerEmail != f.Account.Email || d.OwnerProfile != "P18" || d.ActiveStreams != 1 ||
		d.Capabilities == nil || !d.Capabilities.HDR || d.Capabilities.MaxHeight != 1440 || d.CapabilitiesAt == nil || d.LastSeenAt.UnixMilli() != now.UnixMilli() {
		t.Errorf("device = %+v", d)
	}
	if devices[1].Capabilities != nil || devices[1].OwnerEmail != "" || devices[1].ActiveStreams != 0 {
		t.Errorf("bare device = %+v", devices[1])
	}

	if err := m.svc.DeleteDevice(m.ctx, testDevice); err != nil {
		t.Fatal(err)
	}
	var revoked, pins, rows int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM sessions WHERE id = ? AND revoked_at IS NOT NULL`, session.ID).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM pin_attempts WHERE device_id = ?`, testDevice).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM devices WHERE id = ?`, testDevice).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if revoked != 1 || pins != 0 || rows != 0 || len(fs.ended) != 1 || fs.ended[0] != testDevice {
		t.Errorf("after delete: revoked %d, pins %d, rows %d, ended %v", revoked, pins, rows, fs.ended)
	}
	if err := m.svc.DeleteDevice(m.ctx, testDevice); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if err := m.svc.DeleteDevice(m.ctx, auth.ReservedDeviceID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete of the reserved device: %v", err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM pin_attempts WHERE device_id = ?`, auth.ReservedDeviceID).Scan(&pins); err != nil || pins != 1 {
		t.Errorf("ceiling counters after the refused delete: %d, %v", pins, err)
	}
}
