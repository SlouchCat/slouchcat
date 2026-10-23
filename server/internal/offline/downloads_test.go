package offline

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestOptionsOfMovieSeasonAndOtherKinds(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	m12 := f.Rated(12)

	o, err := e.svc.Options(ctx, e.session(f.P12, e.devA), m12.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if o.MediaFileID != m12.FileID || o.DurationMs != clipDuration.Milliseconds() || len(o.Qualities) != 3 || o.Existing != nil {
		t.Fatalf("options = %+v", o)
	}
	// testProbe: 1080p H.264 at 4 Mbit/s (video 3360 kbit/s): copied in high, encoded below.
	if q := o.Qualities[0]; q.Quality != model.DownloadHigh || !q.Copy || q.Height != 1080 || q.VideoBitrate != 3360 || q.AudioBitrate != 160 {
		t.Errorf("high = %+v", q)
	}
	if q := o.Qualities[2]; q.Quality != model.DownloadLow || q.Copy || q.Width != 852 || q.Height != 480 || q.VideoBitrate != 1200 ||
		q.EstimatedBytes != estimate(1200+160, clipDuration) {
		t.Errorf("low = %+v", q)
	}
	// Two audio tracks; only the text subtitles (embedded SubRip and the sidecar) travel along.
	ft := e.tracks[m12.FileID]
	var subs []int64
	for _, s := range o.Subtitles {
		subs = append(subs, s.ID)
	}
	if len(o.AudioTracks) != 2 || !slices.Equal(subs, []int64{ft.Text, ft.Sidecar}) {
		t.Errorf("tracks = %+v, subtitles %v, want %v", o.AudioTracks, subs, []int64{ft.Text, ft.Sidecar})
	}
	// The second version and an invisible title.
	if o, err := e.svc.Options(ctx, e.session(f.P12, e.devA), m12.ID, f.Version2FileID); err != nil || o.MediaFileID != f.Version2FileID {
		t.Errorf("version 2: %+v, %v", o, err)
	}
	if _, err := e.svc.Options(ctx, e.session(f.P6, e.devA), m12.ID, ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 on the 12 film: %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Options(ctx, e.session(f.P12, e.devA), m12.ID, f.Rated(6).FileID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("file of another title: %v, want ErrNotFound", err)
	}
	for _, id := range []string{f.Show.Series.ID, f.Show.Extra.ID} {
		if _, err := e.svc.Options(ctx, e.session(f.P18, e.devA), id, ""); !errors.Is(err, model.ErrNotDownloadable) {
			t.Errorf("options of %s: %v, want ErrNotDownloadable", id, err)
		}
	}
	// A season: the sums over the visible episodes (E2 is rated 16: hidden from P12).
	for _, tc := range []struct {
		p    model.Profile
		want int
	}{{f.P12, 2}, {f.P16, 3}} {
		o, err := e.svc.Options(ctx, e.session(tc.p, e.devA), f.Show.Season.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if o.EpisodeCount != tc.want || o.MediaFileID != "" || len(o.AudioTracks) != 0 || len(o.Subtitles) != 0 ||
			o.Qualities[2].EstimatedBytes != int64(tc.want)*estimate(1200+160, clipDuration) || !o.Qualities[0].Copy {
			t.Errorf("%s season options = %+v", tc.p.Name, o)
		}
	}
	// The existing download of the device.
	st := e.create(f.P12, e.devA, m12.ID, model.DownloadLow)
	if o, _ := e.svc.Options(ctx, e.session(f.P12, e.devA), m12.ID, ""); o.Existing == nil || o.Existing.ID != st.ID || o.Existing.Item == nil {
		t.Errorf("existing = %+v", o.Existing)
	}
	if o, _ := e.svc.Options(ctx, e.session(f.P12, e.devB), m12.ID, ""); o.Existing != nil {
		t.Errorf("another device sees the download as existing: %+v", o.Existing)
	}
}

func TestCreateDownloads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	m6 := f.Rated(6)

	st := e.create(f.P12, e.devA, m6.ID, model.DownloadMedium)
	if st.State != model.DownloadQueued || st.Quality != model.DownloadMedium || st.Auto || st.Item == nil || st.Item.ID != m6.ID ||
		st.EstimatedBytes != estimate(3000+160, clipDuration) {
		t.Fatalf("state = %+v", st)
	}
	// Every profile of the owner account that sees the 6 film (P0 does not).
	if want := []int64{f.P18.ID, f.P16.ID, f.P12.ID, f.P6.ID}; !slices.Equal(st.AllowedProfileIDs, sortedIDs(want)) {
		t.Errorf("allowed = %v, want %v", st.AllowedProfileIDs, sortedIDs(want))
	}
	if jobs := e.pendingJobs(); len(jobs) != 1 || jobs[primaryKey(st.ID)].IsZero() {
		t.Errorf("jobs = %v", jobs)
	}
	// The same title on the same device again: the existing download, whatever the quality.
	again := e.create(f.P18, e.devA, m6.ID, model.DownloadHigh)
	if again.ID != st.ID || again.Quality != model.DownloadMedium || len(e.pendingJobs()) != 1 {
		t.Errorf("second request = %+v", again)
	}
	// Another device gets its own download; another account on the same device none.
	if other := e.create(f.P12, e.devB, m6.ID, model.DownloadLow); other.ID == st.ID {
		t.Error("device B reused the download of device A")
	}
	_, err := e.svc.Create(ctx, e.session(f.PG, e.devA), m6.ID, model.DownloadRequest{Quality: model.DownloadLow})
	if !errors.Is(err, model.ErrNotDownloadable) {
		t.Errorf("guest on the owner's device: %v, want ErrNotDownloadable", err)
	}
	// The audio track: an AC-3 track by index, an unknown index is a validation error.
	m0 := f.Rated(0)
	ac3 := 2
	list, err := e.svc.Create(ctx, e.session(f.P18, e.devA), m0.ID, model.DownloadRequest{Quality: model.DownloadHigh, AudioTrack: &ac3})
	if err != nil || len(list) != 1 {
		t.Fatalf("create with audio track: %v", err)
	}
	var audio int
	if err := f.DB.Read.QueryRow(`SELECT audio_stream FROM downloads WHERE id = ?`, list[0].ID).Scan(&audio); err != nil || audio != 2 {
		t.Errorf("audio stream = %d, %v", audio, err)
	}
	bad := 7
	var verr *model.ValidationError
	if _, err := e.svc.Create(ctx, e.session(f.P18, e.devA), f.Rated(16).ID, model.DownloadRequest{Quality: model.DownloadHigh, AudioTrack: &bad}); !errors.As(err, &verr) {
		t.Errorf("unknown audio track: %v", err)
	}
	if _, err := e.svc.Create(ctx, e.session(f.P18, e.devA), m0.ID, model.DownloadRequest{Quality: "ultra"}); !errors.As(err, &verr) {
		t.Errorf("unknown quality: %v", err)
	}
	// Invisible, not downloadable, without device.
	if _, err := e.svc.Create(ctx, e.session(f.P6, e.devA), f.Rated(12).ID, model.DownloadRequest{Quality: model.DownloadLow}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible: %v", err)
	}
	if _, err := e.svc.Create(ctx, e.session(f.P18, e.devA), f.Show.Series.ID, model.DownloadRequest{Quality: model.DownloadLow}); !errors.Is(err, model.ErrNotDownloadable) {
		t.Errorf("series: %v", err)
	}
	if _, err := e.svc.Create(ctx, e.session(f.P18, ""), f.Rated(18).ID, model.DownloadRequest{Quality: model.DownloadLow}); !errors.Is(err, model.ErrDeviceRequired) {
		t.Errorf("without device: %v", err)
	}
}

func TestCreateSeasonDownloadsVisibleEpisodes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	// P12 gets E1 and E3 (E2 is rated 16), with the English track by language.
	list, err := e.svc.Create(ctx, e.session(f.P12, e.devA), f.Show.Season.ID, model.DownloadRequest{Quality: model.DownloadLow, AudioLanguage: "en"})
	if err != nil {
		t.Fatal(err)
	}
	var items []string
	for _, st := range list {
		items = append(items, st.Item.ID)
		var audio int
		if err := f.DB.Read.QueryRow(`SELECT audio_stream FROM downloads WHERE id = ?`, st.ID).Scan(&audio); err != nil || audio != 2 {
			t.Errorf("episode %s audio stream = %d, %v; want 2 (en)", st.Item.ID, audio, err)
		}
	}
	if want := []string{f.Show.E1.ID, f.Show.E3.ID}; !slices.Equal(items, want) {
		t.Errorf("P12 season = %v, want %v", items, want)
	}
	// P16 adds E2; E1 and E3 come back as they are.
	list, err = e.svc.Create(ctx, e.session(f.P16, e.devA), f.Show.Season.ID, model.DownloadRequest{Quality: model.DownloadHigh})
	if err != nil || len(list) != 3 {
		t.Fatalf("P16 season: %d, %v", len(list), err)
	}
	if list[0].Quality != model.DownloadLow || list[1].Quality != model.DownloadHigh {
		t.Errorf("P16 season qualities = %s, %s", list[0].Quality, list[1].Quality)
	}
	// A season P6 cannot see.
	if _, err := e.svc.Create(ctx, e.session(f.P6, e.devA), f.Show.Season.ID, model.DownloadRequest{Quality: model.DownloadLow}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 season: %v", err)
	}
}

func TestDeleteWritesTombstone(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	st := e.create(f.P12, e.devA, f.Rated(6).ID, model.DownloadLow)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Fatalf("state %s", r.state)
	}
	// Another account's id is unknown; the owner's account deletes from any profile.
	if err := e.svc.Delete(ctx, e.session(f.PG, e.devA), st.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("guest deletes the owner's download: %v", err)
	}
	if err := e.svc.Delete(ctx, e.session(f.P18, e.devB), st.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, e.session(f.P18, e.devB), st.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if exists(t, e, st.ID) {
		t.Error("the files of the deleted download remain")
	}
	revoked, err := e.svc.Revocations(ctx, e.devA, []string{st.ID, "01J9ZZZZZZZZZZZZZZZZZZZZZZ"})
	if err != nil || !slices.Equal(revoked, []string{st.ID}) {
		t.Errorf("revocations = %v, %v", revoked, err)
	}
	if revoked, _ := e.svc.Revocations(ctx, e.devB, []string{st.ID}); len(revoked) != 0 {
		t.Errorf("device B is told to delete device A's download: %v", revoked)
	}
}

func sortedIDs(ids []int64) []int64 {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}
