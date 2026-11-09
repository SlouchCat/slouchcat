package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
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
	// Concept 11: an episode shows the poster of its series, a film its own; the poster of an
	// episode itself is not used. The special has a series poster too; the film without one has none.
	poster := func(itemID, hash string) {
		t.Helper()
		m.exec(t, `INSERT INTO images (item_id, kind, size, format, source_url, rel_path, content_hash, media_type, width, height, bytes, created_at)
		           VALUES (?, 'poster', ?, 'jpeg', 'https://example.org/p.jpg', ?, ?, 'image/jpeg', 200, 300, 1, 1)
		           ON CONFLICT (item_id, kind, size, format) DO UPDATE SET content_hash = excluded.content_hash`,
			itemID, model.AdminPosterSize, "posters/"+hash+".jpg", hash)
	}
	m.exec(t, `DELETE FROM images WHERE item_id = ?`, f.Rated(6).ID)
	poster(f.Show.Series.ID, "seriesHash")
	poster(f.Show.E3.ID, "episodeHash")
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
	wantPosters := [][2]string{{f.Show.Series.ID, "seriesHash"}, {"", ""}, {f.Show.Series.ID, "seriesHash"}}
	for i, st := range list {
		if st.PosterItemID != wantPosters[i][0] || st.PosterHash != wantPosters[i][1] {
			t.Errorf("stream %d poster = %q %q, want %v", i, st.PosterItemID, st.PosterHash, wantPosters[i])
		}
	}
	poster(f.Rated(6).ID, "filmHash")
	if list, err = m.svc.ListStreams(m.ctx); err != nil || list[1].PosterItemID != f.Rated(6).ID || list[1].PosterHash != "filmHash" {
		t.Errorf("film poster = %+v, %v", list, err)
	}
	if err := m.svc.StopStream(m.ctx, "01JAAAAAAAAAAAAAAAAAAAAAAA"); err != nil || len(fs.stopped) != 1 {
		t.Errorf("stop: %v %v", err, fs.stopped)
	}
	if err := m.svc.StopStream(m.ctx, "01JZZZZZZZZZZZZZZZZZZZZZZZ"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("stop unknown: %v", err)
	}
}

// A session whose title was deleted between the list of sessions and the read of their titles is
// left out: the overview has no kind for it, and the contract requires one.
func TestAdminStreamsTitleGone(t *testing.T) {
	m := newMaintFixture(t)
	film := m.f.Rated(6)
	m.svc.WithStreams(&fakeStreams{list: []model.AdminStream{
		{ID: "01JAAAAAAAAAAAAAAAAAAAAAAA", ItemID: "01JZZZZZZZZZZZZZZZZZZZZZZZ", MediaFileID: "gone", Mode: "direct"},
		{ID: "01JBBBBBBBBBBBBBBBBBBBBBBB", ItemID: film.ID, MediaFileID: film.FileID, Mode: "direct"},
	}})
	list, err := m.svc.ListStreams(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "01JBBBBBBBBBBBBBBBBBBBBBBB" || list[0].Kind != "movie" {
		t.Fatalf("streams = %+v, want only the film's with kind movie", list)
	}
}

// Concept 18, 17.4: the overview names what each stream plays. A film, an episode and an extra show
// the backdrop of themselves, their series and their film; a podcast episode names its podcast and
// shows its own cover, from the feed the podcast's and the feed's duration; an audiobook of several
// parts shows its cover, the chapter at the position, the number of parts and the whole book's
// length, and audio names its codec and bit rate.
func TestAdminStreamsKinds(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	f.AddAudio(t)
	a := f.Audio
	film, extra, episode, podcastFile := f.Rated(6), f.Extras[0], a.Episodes[0], a.Episodes[0]
	m.exec(t, `UPDATE items SET duration_ms = 1234567 WHERE id = ?`, a.EnclosureOnly.ID)
	m.exec(t, `INSERT INTO chapters (item_id, idx, title, start_ms, end_ms) VALUES (?, 0, 'Kapitel 1', 0, 1000), (?, 1, 'Kapitel 2', 1000, 9000)`,
		a.Parts.ID, a.Parts.ID)
	pos := func(ms int64) *int64 { return &ms }
	m.svc.WithStreams(&fakeStreams{list: []model.AdminStream{
		{ID: "01JAAAAAAAAAAAAAAAAAAAAAAA", ItemID: film.ID, MediaFileID: film.FileID, Mode: "direct"},
		{ID: "01JBBBBBBBBBBBBBBBBBBBBBBB", ItemID: f.Show.E1.ID, MediaFileID: f.Show.E1.FileID, Mode: "transcode", Bitrate: 8_000_000},
		{ID: "01JCCCCCCCCCCCCCCCCCCCCCCC", ItemID: extra.ID, MediaFileID: extra.FileID, Mode: "remux"},
		{ID: "01JDDDDDDDDDDDDDDDDDDDDDDD", ItemID: podcastFile.ID, MediaFileID: podcastFile.FileID, Mode: "direct"},
		{ID: "01JEEEEEEEEEEEEEEEEEEEEEEE", ItemID: a.EnclosureOnly.ID, Mode: "direct", PositionMs: pos(60_000)},
		{ID: "01JFFFFFFFFFFFFFFFFFFFFFFF", ItemID: a.Parts.ID, MediaFileID: a.Parts.FileID, Mode: "remux", Bitrate: 64_000, PositionMs: pos(2000)},
	}})
	hash := func(itemID, kind, size string) string {
		t.Helper()
		var h string
		if err := f.DB.Read.QueryRow(`SELECT content_hash FROM images WHERE item_id = ? AND kind = ? AND size = ? AND format = 'jpeg'`,
			itemID, kind, size).Scan(&h); err != nil {
			t.Fatalf("hash of %s %s: %v", itemID, kind, err)
		}
		return h
	}
	type want struct {
		kind, title, parent, chapter, poster, backdrop, codec, source string
		duration                                                      int64
		parts, bitrate                                                int
	}
	wants := []want{
		{kind: "movie", title: "Film 06", poster: film.ID, backdrop: film.ID, source: "file", duration: vistest.DurationMs},
		{kind: "episode", title: "Film SS · S01E01 · Film E1", poster: f.Show.Series.ID, backdrop: f.Show.Series.ID, source: "file", duration: vistest.DurationMs},
		{kind: "extra", title: "Film XT", parent: "Film 06", poster: film.ID, backdrop: film.ID, source: "file", duration: vistest.DurationMs},
		{kind: "podcast_episode", title: "Film " + episode.Label, parent: "Film PC", poster: episode.ID, codec: "mp3", bitrate: 256000,
			source: "file", duration: vistest.DurationMs},
		{kind: "podcast_episode", title: "Film QE", parent: "Film PC", poster: a.Podcast.ID, codec: "mp3", source: "feed", duration: 1234567},
		{kind: "audiobook", title: "Film AP", chapter: "Kapitel 2", poster: a.Parts.ID, codec: "aac", bitrate: 64_000, parts: 3,
			source: "file", duration: 3 * vistest.DurationMs},
	}
	list, err := m.svc.ListStreams(m.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(wants) {
		t.Fatalf("streams = %d, want %d", len(list), len(wants))
	}
	for i, w := range wants {
		st := list[i]
		if st.Kind != w.kind || st.ItemTitle != w.title || st.ParentTitle != w.parent || st.ChapterTitle != w.chapter ||
			st.Source != w.source || st.PartCount != w.parts || st.AudioCodec != w.codec || st.AudioBitrate != w.bitrate {
			t.Errorf("stream %d = kind %q title %q parent %q chapter %q source %q parts %d audio %q %d, want %+v",
				i, st.Kind, st.ItemTitle, st.ParentTitle, st.ChapterTitle, st.Source, st.PartCount, st.AudioCodec, st.AudioBitrate, w)
		}
		if st.DurationMs == nil || *st.DurationMs != w.duration {
			t.Errorf("stream %d (%s) duration = %v, want %d", i, w.kind, st.DurationMs, w.duration)
		}
		if st.PosterItemID != w.poster || st.PosterHash != hash(w.poster, "poster", model.AdminPosterSize) {
			t.Errorf("stream %d (%s) poster = %q %q, want %q", i, w.kind, st.PosterItemID, st.PosterHash, w.poster)
		}
		switch {
		case w.backdrop == "" && (st.BackdropItemID != "" || st.BackdropHash != ""):
			t.Errorf("stream %d (%s) backdrop = %q, want none", i, w.kind, st.BackdropItemID)
		case w.backdrop != "" && (st.BackdropItemID != w.backdrop || st.BackdropHash != hash(w.backdrop, "backdrop", model.AdminBackdropSize)):
			t.Errorf("stream %d (%s) backdrop = %q %q, want %q", i, w.kind, st.BackdropItemID, st.BackdropHash, w.backdrop)
		}
	}
}

// The chapter at a position is the last one that starts at or before it; without a position or
// before the first chapter there is none.
func TestChapterAt(t *testing.T) {
	chapters := []systemq.AdminStreamChaptersRow{{Title: "Eins", StartMs: 500, EndMs: 1000}, {Title: "Zwei", StartMs: 1000, EndMs: 2000}}
	for _, c := range []struct {
		pos  *int64
		want string
	}{{nil, ""}, {ptr64(0), ""}, {ptr64(500), "Eins"}, {ptr64(999), "Eins"}, {ptr64(1000), "Zwei"}, {ptr64(5000), "Zwei"}} {
		if got := chapterAt(chapters, c.pos); got != c.want {
			t.Errorf("chapterAt(%v) = %q, want %q", c.pos, got, c.want)
		}
	}
}

func ptr64(v int64) *int64 { return &v }

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
