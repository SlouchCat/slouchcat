package offline

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func (e *env) sync(p model.Profile, device string, held ...model.HeldDownload) model.OfflineSync {
	e.t.Helper()
	out, err := e.svc.Sync(context.Background(), e.session(p, device), model.OfflineSyncRequest{Held: held, Network: "wifi"})
	if err != nil {
		e.t.Fatalf("sync of %s: %v", p.Name, err)
	}
	return out
}

func stateOf(s model.OfflineSync, id string) *model.DownloadState {
	for i := range s.Downloads {
		if s.Downloads[i].ID == id {
			return &s.Downloads[i]
		}
	}
	return nil
}

func profileIDs(s model.OfflineSync) []int64 {
	var out []int64
	for _, p := range s.Profiles {
		out = append(out, p.ID)
	}
	return out
}

func TestSyncFetchesRevocationsAndEvictedCopies(t *testing.T) {
	e := newEnv(t)
	f := e.f
	kept := e.create(f.P12, e.devA, f.Rated(6).ID, model.DownloadLow)
	gone := e.create(f.P12, e.devA, f.Rated(0).ID, model.DownloadLow)
	lost := e.create(f.P12, e.devA, f.Rated(12).ID, model.DownloadLow)
	other := e.create(f.P12, e.devB, f.Rated(6).ID, model.DownloadLow)
	e.runJobs()
	if err := e.svc.Delete(context.Background(), e.session(f.P18, e.devB), gone.ID); err != nil {
		t.Fatal(err)
	}
	// The device holds kept and gone completely, lost only partly; device B's download is not its.
	s := e.sync(f.P12, e.devA, model.HeldDownload{ID: kept.ID, Complete: true}, model.HeldDownload{ID: gone.ID, Complete: true},
		model.HeldDownload{ID: lost.ID}, model.HeldDownload{ID: other.ID, Complete: true})
	if !slices.Equal(s.Revoked, []string{gone.ID}) {
		t.Errorf("revoked = %v, want [%s]", s.Revoked, gone.ID)
	}
	if r := e.row(kept.ID); !r.fetched.Valid || r.fetched.Int64 != e.clock.Now().UnixMilli() {
		t.Errorf("kept fetched = %+v", r.fetched)
	}
	if r := e.row(lost.ID); r.fetched.Valid {
		t.Error("a partly held download counts as fetched")
	}
	if r := e.row(other.ID); r.fetched.Valid {
		t.Error("device A marked device B's download as fetched")
	}
	if len(s.Downloads) != 2 || stateOf(s, kept.ID) == nil || stateOf(s, lost.ID) == nil || !s.ServerTime.Equal(e.clock.Now()) {
		t.Errorf("downloads = %+v", s.Downloads)
	}
	// Both copies are evicted; the next sync prepares again only what the device lacks.
	e.exec(`UPDATE downloads SET state = 'evicted', files_removed_at = 1 WHERE id IN (?, ?)`, kept.ID, lost.ID)
	s = e.sync(f.P12, e.devA, model.HeldDownload{ID: kept.ID, Complete: true}, model.HeldDownload{ID: lost.ID})
	if st := stateOf(s, kept.ID); st.State != model.DownloadEvicted {
		t.Errorf("complete evicted copy = %s", st.State)
	}
	if st := stateOf(s, lost.ID); st.State != model.DownloadQueued || st.SegmentsDone != 0 {
		t.Errorf("lacking evicted copy = %+v", st)
	}
	if e.pendingJobs()[primaryKey(lost.ID)].IsZero() {
		t.Errorf("no preparation queued for the lacking copy: %v", e.pendingJobs())
	}
	e.runJobs()
	if r := e.row(lost.ID); r.state != model.DownloadReady || r.removed.Valid {
		t.Errorf("prepared again = %+v", r)
	}
}

func TestSyncProfilesPinsWatchTimeAndRules(t *testing.T) {
	e := newEnv(t)
	f := e.f
	// The 6 film on device A: P18, P16, P12 and P6 may play it, P0 not.
	st := e.create(f.P12, e.devA, f.Rated(6).ID, model.DownloadLow)
	s := e.sync(f.P12, e.devA)
	if want := []int64{f.P18.ID, f.P16.ID, f.P12.ID, f.P6.ID}; !slices.Equal(profileIDs(s), want) {
		t.Errorf("profiles = %v, want %v", profileIDs(s), want)
	}
	if asked := e.pins.asked[len(e.pins.asked)-1]; slices.Contains(asked, f.P0.ID) {
		t.Errorf("PIN material asked for P0, which has no download here: %v", asked)
	}
	for _, p := range s.Profiles {
		if p.PinSalt == "" || p.PinIterations != model.OfflinePinIterations || !p.HasPin {
			t.Errorf("profile %s: %+v", p.Name, p)
		}
		if (p.ID == f.P16.ID) != (p.OfflinePin != nil) {
			t.Errorf("profile %s offline pin = %+v", p.Name, p.OfflinePin)
		}
		if (p.ID == f.P6.ID) != (p.WatchTime != nil) {
			t.Errorf("profile %s watch time = %+v", p.Name, p.WatchTime)
		}
	}
	if s.Rules != model.DefaultDownloadRules {
		t.Errorf("rules = %+v", s.Rules)
	}
	// P0 is active on device A: it gets its own data although no download is for it, and the
	// download without its title.
	s = e.sync(f.P0, e.devA)
	if !slices.Contains(profileIDs(s), f.P0.ID) || stateOf(s, st.ID).Item != nil {
		t.Errorf("P0 sync: profiles %v, item %+v", profileIDs(s), stateOf(s, st.ID).Item)
	}
	if s = e.sync(f.P6, e.devA); stateOf(s, st.ID).Item == nil || stateOf(s, st.ID).Item.ID != f.Rated(6).ID {
		t.Errorf("P6 sync lacks the item it may see")
	}
	// Rules: stored per profile and device.
	ctx := context.Background()
	r := model.DownloadRules{Enabled: true, NextEpisodes: true, NewPodcastEpisodes: true, WifiOnly: false, MaxCount: 5, Quality: model.DownloadHigh}
	if got, err := e.svc.SetRules(ctx, e.session(f.P6, e.devA), r); err != nil || got != r {
		t.Fatalf("set rules = %+v, %v", got, err)
	}
	if got, _ := e.svc.Rules(ctx, e.session(f.P6, e.devA)); got != r {
		t.Errorf("rules = %+v", got)
	}
	if got, _ := e.svc.Rules(ctx, e.session(f.P6, e.devB)); got != model.DefaultDownloadRules {
		t.Errorf("device B rules = %+v", got)
	}
	if got := e.sync(f.P6, e.devA).Rules; got != r {
		t.Errorf("sync rules = %+v", got)
	}
	var verr *model.ValidationError
	for _, bad := range []model.DownloadRules{{MaxCount: 0, Quality: model.DownloadLow}, {MaxCount: 21, Quality: model.DownloadLow}, {MaxCount: 3, Quality: "4k"}} {
		if _, err := e.svc.SetRules(ctx, e.session(f.P6, e.devA), bad); !errors.As(err, &verr) {
			t.Errorf("rules %+v: %v", bad, err)
		}
	}
	if _, err := e.svc.Rules(ctx, e.session(f.P6, "")); !errors.Is(err, model.ErrDeviceRequired) {
		t.Errorf("rules without device: %v", err)
	}
}

// The response for P6 on a device where P18 holds an FSK-18 download names neither the title nor
// its id; the download appears with its allowed profiles only.
func TestSyncHidesTitlesOfOtherProfiles(t *testing.T) {
	e := newEnv(t)
	f := e.f
	m18 := f.Rated(18)
	st := e.create(f.P18, e.devA, m18.ID, model.DownloadHigh)
	s := e.sync(f.P6, e.devA)
	got := stateOf(s, st.ID)
	if got == nil || got.Item != nil || !slices.Equal(got.AllowedProfileIDs, []int64{f.P18.ID}) {
		t.Fatalf("P6 sees %+v", got)
	}
	if want := []int64{f.P18.ID, f.P6.ID}; !slices.Equal(profileIDs(s), want) {
		t.Errorf("profiles = %v, want %v", profileIDs(s), want)
	}
}

// Plan 10.2: the series goes from 12 to 16; at the next sync P12 drops out of the allowed
// profiles of its episode download and its sync no longer carries the title.
func TestAllowedProfilesAfterRatingChange(t *testing.T) {
	e := newEnv(t)
	f := e.f
	st := e.create(f.P12, e.devA, f.Show.E1.ID, model.DownloadLow)
	if want := []int64{f.P18.ID, f.P16.ID, f.P12.ID}; !slices.Equal(st.AllowedProfileIDs, sortedIDs(want)) {
		t.Fatalf("allowed = %v", st.AllowedProfileIDs)
	}
	sixteen := 16
	f.SetRating(t, f.Show.Series.ID, &sixteen)
	s := e.sync(f.P12, e.devA)
	if got := stateOf(s, st.ID); !slices.Equal(got.AllowedProfileIDs, sortedIDs([]int64{f.P18.ID, f.P16.ID})) || got.Item != nil {
		t.Errorf("after the rating change: %+v", got)
	}
	if r := e.row(st.ID); r.allowed != encodeIDs(sortedIDs([]int64{f.P18.ID, f.P16.ID})) {
		t.Errorf("stored allowed = %s", r.allowed)
	}
	// P12 itself is no longer among the offline profiles' downloads, only as the active profile.
	twelve := 12
	f.SetRating(t, f.Show.Series.ID, &twelve)
	if got := stateOf(e.sync(f.P16, e.devA), st.ID); !slices.Contains(got.AllowedProfileIDs, f.P12.ID) {
		t.Errorf("back at 12: %v", got.AllowedProfileIDs)
	}
}

// AN-4, K-7: every deleted download leaves a tombstone, also through the cascades of a revoked
// guest account, a forgotten device and a title removed for good (SQLite fires the trigger of the
// child table for cascaded deletes).
func TestTombstonesThroughCascades(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	devG := e.device("c")
	guest := e.create(f.PG, devG, f.Rated(6).ID, model.DownloadLow)
	device := e.create(f.P18, e.devB, f.Rated(0).ID, model.DownloadLow)
	e.exec(`DELETE FROM accounts WHERE id = ?`, f.Guest.ID)
	e.exec(`DELETE FROM devices WHERE id = ?`, e.devB)
	if got, err := e.svc.Revocations(ctx, devG, []string{guest.ID}); err != nil || !slices.Equal(got, []string{guest.ID}) {
		t.Errorf("revoked guest: %v, %v", got, err)
	}
	if got, _ := e.svc.Revocations(ctx, e.devB, []string{device.ID}); !slices.Equal(got, []string{device.ID}) {
		t.Errorf("forgotten device: %v", got)
	}
	// A title removed for good (the maintenance cleanup deletes the row).
	m := e.create(f.P18, e.devA, f.Rated(16).ID, model.DownloadLow)
	e.exec(`DELETE FROM items WHERE id = ?`, f.Rated(16).ID)
	if got := e.sync(f.P18, e.devA, model.HeldDownload{ID: m.ID, Complete: true}).Revoked; !slices.Equal(got, []string{m.ID}) {
		t.Errorf("removed title: %v", got)
	}
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM download_tombstones`).Scan(&n); err != nil || n != 3 {
		t.Errorf("tombstones = %d, %v", n, err)
	}
}

// O-3, AN-13: the next episode of the last watched one, while fewer than maxCount unfinished
// auto-downloads are on the device; only the device of the rule; invisible episodes skipped.
func TestAutoDownloads(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	show := f.Show
	progress := func(p model.Profile, itemID string, completed bool, ago time.Duration) {
		c := 0
		if completed {
			c = 1
		}
		e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, 2000, ?, ?)
		        ON CONFLICT (profile_id, item_id) DO UPDATE SET completed = excluded.completed, updated_at = excluded.updated_at`,
			p.ID, itemID, c, e.clock.Now().Add(-ago).UnixMilli())
	}
	rule := func(p model.Profile, device string, maxCount int) {
		t.Helper()
		if _, err := e.svc.SetRules(ctx, e.session(p, device), model.DownloadRules{Enabled: true, NextEpisodes: true, WifiOnly: true,
			MaxCount: maxCount, Quality: model.DownloadLow}); err != nil {
			t.Fatal(err)
		}
	}
	autoItems := func(s model.OfflineSync) []string {
		var out []string
		for _, d := range s.Downloads {
			if d.Auto && d.Item != nil {
				out = append(out, d.Item.ID)
			}
		}
		return out
	}

	// P16 finished E1: E2 comes on device A, not on device B.
	rule(f.P16, e.devA, 1)
	progress(f.P16, show.E1.ID, true, 24*time.Hour)
	s := e.sync(f.P16, e.devA)
	if got := autoItems(s); !slices.Equal(got, []string{show.E2.ID}) {
		t.Fatalf("P16 auto-downloads = %v, want E2", got)
	}
	if d := s.Downloads[0]; d.Quality != model.DownloadLow || d.State != model.DownloadQueued {
		t.Errorf("auto-download = %+v", d)
	}
	if s := e.sync(f.P16, e.devB); len(s.Downloads) != 0 {
		t.Errorf("device B without rule got %+v", s.Downloads)
	}
	// P16 starts E2: E3 would be next, but E2 is an unfinished auto-download and maxCount is 1.
	progress(f.P16, show.E2.ID, false, time.Hour)
	if got := autoItems(e.sync(f.P16, e.devA)); !slices.Equal(got, []string{show.E2.ID}) {
		t.Fatalf("over maxCount: %v", got)
	}
	// E2 finished: it stays (O-6) but no longer counts, so E3 comes.
	progress(f.P16, show.E2.ID, true, time.Minute)
	if got := autoItems(e.sync(f.P16, e.devA)); !slices.Equal(got, []string{show.E2.ID, show.E3.ID}) {
		t.Fatalf("after E2 finished: %v", got)
	}
	// P12 does not see E2: after E1 its next episode is E3.
	rule(f.P12, e.devB, 3)
	progress(f.P12, show.E1.ID, true, time.Hour)
	if got := autoItems(e.sync(f.P12, e.devB)); !slices.Equal(got, []string{show.E3.ID}) {
		t.Errorf("P12 auto-downloads = %v, want E3", got)
	}
	// Watched more than 60 days ago: no auto-download; within 60 days: the next one.
	devC := e.device("c")
	rule(f.P18, devC, 3)
	progress(f.P18, show.E1.ID, true, 61*24*time.Hour)
	if s := e.sync(f.P18, devC); len(s.Downloads) != 0 {
		t.Errorf("old progress gave %+v", s.Downloads)
	}
	progress(f.P18, show.E1.ID, true, 59*24*time.Hour)
	if got := autoItems(e.sync(f.P18, devC)); !slices.Equal(got, []string{show.E2.ID}) {
		t.Errorf("P18 auto-downloads = %v, want E2", got)
	}
	// A disabled rule does nothing.
	devD := e.device("d")
	if _, err := e.svc.SetRules(ctx, e.session(f.P18, devD), model.DownloadRules{MaxCount: 3, Quality: model.DownloadLow, NextEpisodes: true}); err != nil {
		t.Fatal(err)
	}
	if s := e.sync(f.P18, devD); len(s.Downloads) != 0 {
		t.Errorf("disabled rule gave %+v", s.Downloads)
	}
}

func TestSyncValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.session(e.f.P18, e.devA)
	held := make([]model.HeldDownload, MaxHeld+1)
	ids := make([]string, MaxHeld+1)
	for i := range held {
		held[i] = model.HeldDownload{ID: "01J9AAAAAAAAAAAAAAAAAAAAAA"}
		ids[i] = held[i].ID
	}
	var verr *model.ValidationError
	for name, req := range map[string]model.OfflineSyncRequest{
		"too many":    {Held: held},
		"invalid id":  {Held: []model.HeldDownload{{ID: "../../etc/passwd"}}},
		"bad network": {Network: "5g"},
	} {
		if _, err := e.svc.Sync(ctx, s, req); !errors.As(err, &verr) {
			t.Errorf("%s: %v, want a validation error", name, err)
		}
	}
	if _, err := e.svc.Sync(ctx, e.session(e.f.P18, ""), model.OfflineSyncRequest{}); !errors.Is(err, model.ErrDeviceRequired) {
		t.Errorf("without device: %v", err)
	}
	if _, err := e.svc.Revocations(ctx, e.devA, ids); !errors.As(err, &verr) {
		t.Errorf("revocations over the limit: %v", err)
	}
	if got, err := e.svc.Revocations(ctx, "", []string{"01J9AAAAAAAAAAAAAAAAAAAAAA"}); err != nil || got == nil || len(got) != 0 {
		t.Errorf("without device: %v, %v", got, err)
	}
	// At the limit the sync works.
	if _, err := e.svc.Sync(ctx, s, model.OfflineSyncRequest{Held: held[:MaxHeld]}); err != nil {
		t.Errorf("2000 held: %v", err)
	}
	e.svc.pins = nil
	if _, err := e.svc.Sync(ctx, s, model.OfflineSyncRequest{}); !errors.Is(err, ErrNoPins) {
		t.Errorf("without pin port: %v", err)
	}
}

// O-6: a download on the device has no expiry, so the table has no expiry-like column.
func TestDownloadsHaveNoExpiryColumn(t *testing.T) {
	e := newEnv(t)
	rows, err := e.f.DB.Read.Query(`SELECT name FROM pragma_table_info('downloads')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
		for _, bad := range []string{"expir", "valid_until", "ttl", "lifetime", "deadline"} {
			if strings.Contains(strings.ToLower(name), bad) {
				t.Errorf("downloads.%s looks like an expiry (O-6)", name)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cols, "fetched_at") || !slices.Contains(cols, "allowed_profile_ids") {
		t.Fatalf("columns = %v", cols)
	}
}
