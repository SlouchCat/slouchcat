package offline

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// fakeAudio stands for visibility's Parts and Chapters (M6 stream A): the parts of the fixture's
// multi-part book through the profile's scope, one chapter per part.
type fakeAudio struct {
	vis   *visibility.Reader
	parts map[string][]string // item id → file ids in order
}

func (a fakeAudio) Parts(ctx context.Context, s visibility.Scope, itemID string) ([]visibility.MediaFileLocation, error) {
	var out []visibility.MediaFileLocation
	for _, id := range a.parts[itemID] {
		loc, err := a.vis.MediaFile(ctx, s, itemID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, loc)
	}
	return out, nil
}

func (a fakeAudio) Chapters(ctx context.Context, s visibility.Scope, itemID string) ([]model.Chapter, error) {
	if err := a.vis.Visible(ctx, s, itemID); err != nil {
		return nil, err
	}
	var out []model.Chapter
	for i := range a.parts[itemID] {
		start := int64(i) * clipDuration.Milliseconds()
		out = append(out, model.Chapter{StartMs: start, EndMs: start + clipDuration.Milliseconds(), Title: "Teil"})
	}
	return out, nil
}

// newAudioEnv is newEnv with the audio fixture: AAC 128 kbit/s books, MP3 parts and episodes.
func newAudioEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.f.AddAudio(t)
	a := e.f.Audio
	for _, m := range a.Books {
		e.f.SetMediaFile(t, m.FileID, "m4b", "", "aac", probeJSON(t, audioProbe("aac", 128_000)), nil)
	}
	for _, id := range a.Parts.PartIDs {
		e.f.SetMediaFile(t, id, "mp3", "", "mp3", probeJSON(t, audioProbe("mp3", 128_000)), nil)
	}
	for _, m := range append(slices.Clone(a.Episodes), a.Explicit) {
		e.f.SetMediaFile(t, m.FileID, "mp3", "", "mp3", probeJSON(t, audioProbe("mp3", 192_000)), nil)
	}
	e.svc.audio = fakeAudio{vis: visibility.New(e.f.DB, vistest.Logger()), parts: map[string][]string{a.Parts.ID: a.Parts.PartIDs}}
	return e
}

// O-1 for audio: AAC 128/96/64 kbit/s, estimate = rate × duration / 8 × 1.02, an AAC source within
// 1.1 × the rate is copied (single part only); a podcast is no download.
func TestAudioDownloadOptions(t *testing.T) {
	e := newAudioEnv(t)
	ctx := context.Background()
	a := e.f.Audio
	est := func(kbps int, d time.Duration) int64 { return int64(float64(kbps) * 1000 / 8 * d.Seconds() * 1.02) }
	for _, tc := range []struct {
		name  string
		m     vistest.Movie
		d     time.Duration
		kbps  []int
		copy  []bool
		media string
	}{
		{"aac book", a.Books[0], clipDuration, []int{128, 96, 64}, []bool{true, false, false}, a.Books[0].FileID},
		{"three parts", a.Parts, 3 * clipDuration, []int{128, 96, 64}, []bool{false, false, false}, a.Parts.FileID},
		{"mp3 episode", a.Episodes[0], clipDuration, []int{128, 96, 64}, []bool{false, false, false}, a.Episodes[0].FileID},
		{"enclosure episode", a.EnclosureOnly, vistest.DurationMs * time.Millisecond, []int{128, 96, 64}, []bool{false, false, false}, ""},
	} {
		o, err := e.svc.Options(ctx, e.session(e.f.P18, e.devA), tc.m.ID, "")
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if o.MediaFileID != tc.media || o.DurationMs != tc.d.Milliseconds() || len(o.AudioTracks) != 0 || len(o.Subtitles) != 0 || len(o.Qualities) != 3 {
			t.Errorf("%s: options %+v", tc.name, o)
			continue
		}
		for i, q := range o.Qualities {
			if q.AudioBitrate != tc.kbps[i] || q.Copy != tc.copy[i] || q.VideoBitrate != 0 || q.Width != 0 || q.Height != 0 ||
				q.EstimatedBytes != est(tc.kbps[i], tc.d) {
				t.Errorf("%s %s: %+v, want %d kbit/s copy %v, %d bytes", tc.name, q.Quality, q, tc.kbps[i], tc.copy[i], est(tc.kbps[i], tc.d))
			}
		}
	}
	if _, err := e.svc.Options(ctx, e.session(e.f.P18, e.devA), a.Podcast.ID, ""); !errors.Is(err, model.ErrNotDownloadable) {
		t.Errorf("podcast: %v", err)
	}
	if _, err := e.svc.Create(ctx, e.session(e.f.P18, e.devA), a.Podcast.ID, model.DownloadRequest{Quality: model.DownloadLow}); !errors.Is(err, model.ErrNotDownloadable) {
		t.Errorf("podcast download: %v", err)
	}
	for _, p := range []model.Profile{e.f.P12, e.f.P16} {
		if _, err := e.svc.Options(ctx, e.session(p, e.devA), a.Explicit.ID, ""); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("explicit episode for %s: %v", p.Name, err)
		}
	}
}

// An audiobook download: prepared through the encoder, the manifest says audio/mp4 with the
// book's chapters, allowed_profile_ids lists the account's profiles that see the book (H-4).
func TestAudiobookDownload(t *testing.T) {
	e := newAudioEnv(t)
	ctx := context.Background()
	a := e.f.Audio
	book := a.Books[1] // rated 6
	st := e.create(e.f.P6, e.devA, book.ID, model.DownloadHigh)
	if want := []int64{e.f.P18.ID, e.f.P16.ID, e.f.P12.ID, e.f.P6.ID}; !sameIDs(st.AllowedProfileIDs, want) {
		t.Errorf("allowed = %v, want %v", st.AllowedProfileIDs, want)
	}
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || r.total != 5 {
		t.Fatalf("row %+v", r)
	}
	man, err := e.svc.Manifest(ctx, e.session(e.f.P6, e.devA), st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if man.MimeType != "audio/mp4" || man.Codecs != "mp4a.40.2" || man.Width != 0 || man.Height != 0 || len(man.Chapters) != 0 || man.Chapters == nil {
		t.Errorf("manifest = %+v", man)
	}
	src, err := e.svc.OpenFile(ctx, model.DownloadGrant{DownloadID: st.ID, ItemID: book.ID, AccountID: e.f.Account.ID, ProfileID: e.f.P6.ID})
	if err != nil || src.MediaType != "audio/mp4" {
		t.Errorf("file = %+v %v", src, err)
	} else {
		_ = src.File.Close()
	}
	// The book of three parts: one download, chapters on the book's timeline.
	st = e.create(e.f.P18, e.devA, a.Parts.ID, model.DownloadLow)
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady {
		t.Fatalf("parts row %+v", r)
	}
	man, err = e.svc.Manifest(ctx, e.session(e.f.P18, e.devA), st.ID)
	if err != nil || len(man.Chapters) != 3 || man.Chapters[2].StartMs != 2*clipDuration.Milliseconds() {
		t.Errorf("parts manifest = %+v %v", man.Chapters, err)
	}
}

func sameIDs(got, want []int64) bool {
	g, w := slices.Clone(got), slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	return slices.Equal(g, w)
}

// A podcast episode without file downloads from its enclosure: the job fetches it into
// <id>/source.mp3, encodes it and removes the source once ready; a host that keeps failing ends
// the download as source_unavailable.
func TestEpisodeDownloadFromEnclosure(t *testing.T) {
	e := newAudioEnv(t)
	a := e.f.Audio
	st := e.create(e.f.P12, e.devA, a.EnclosureOnly.ID, model.DownloadMedium)
	var source string
	if err := e.f.DB.Read.QueryRow(`SELECT COALESCE(source_url, '') FROM downloads WHERE id = ? AND media_file_id IS NULL`, st.ID).Scan(&source); err != nil || source != a.EnclosureOnly.Enclosure {
		t.Fatalf("source_url %q %v", source, err)
	}
	e.runJobs()
	if r := e.row(st.ID); r.state != model.DownloadReady || r.total != 5 {
		t.Fatalf("row %+v", r)
	}
	if _, err := os.Stat(filepath.Join(e.dir, st.ID, "source.mp3")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("source kept after ready: %v", err)
	}
	man, err := e.svc.Manifest(context.Background(), e.session(e.f.P12, e.devA), st.ID)
	if err != nil || man.MimeType != "audio/mp4" || man.Audio != nil || len(man.Subtitles) != 0 {
		t.Errorf("manifest = %+v %v", man, err)
	}

	// A dead host: every attempt fails, the last one leaves source_unavailable.
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	e.f.SetEnclosure(t, a.EnclosureOnly.ID, dead.URL+"/gone.mp3")
	st = e.create(e.f.P12, e.devB, a.EnclosureOnly.ID, model.DownloadLow)
	for range 10 {
		e.runJobs()
		e.clock.Advance(time.Hour)
	}
	if r := e.row(st.ID); r.state != model.DownloadFailed || r.err != downloadErrSourceUnavailable {
		t.Errorf("dead host row %+v", r)
	}
}

// The enclosure fetch continues a partial file with a Range request.
func TestFetchSourceResumes(t *testing.T) {
	e := newAudioEnv(t)
	ep := e.f.Audio.EnclosureOnly
	id := "01J9AAAAAAAAAAAAAAAAAAAAAA"
	if err := os.MkdirAll(filepath.Join(e.dir, id), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, id, "source.mp3.part"), ep.Content[:1000], 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := e.svc.fetchSource(context.Background(), id, ep.Enclosure)
	if err != nil || name != "source.mp3" {
		t.Fatalf("fetch = %q %v", name, err)
	}
	if got := e.file(id, "source.mp3"); !bytes.Equal(got, ep.Content) {
		t.Errorf("resumed file has %d bytes, want the %d of the enclosure", len(got), len(ep.Content))
	}
	// A complete file is reused without a request.
	e.f.SetEnclosure(t, ep.ID, "http://127.0.0.1:9/unused.mp3")
	if name, err := e.svc.fetchSource(context.Background(), id, ep.Enclosure); err != nil || name != "source.mp3" {
		t.Errorf("second fetch = %q %v", name, err)
	}
	if _, err := e.svc.fetchSource(context.Background(), id, "ftp://example.test/x.mp3"); !errors.Is(err, errSourceUnavailable) {
		t.Errorf("ftp enclosure: %v", err)
	}
}

// O-3: a profile that listened to a podcast gets the visible episodes published after the newest
// one it started, newest first, up to maxCount; the explicit episode never for a profile under 18.
func TestPodcastAutoDownloads(t *testing.T) {
	e := newAudioEnv(t)
	ctx := context.Background()
	a := e.f.Audio
	rule := func(p model.Profile, maxCount int) {
		t.Helper()
		if _, err := e.svc.SetRules(ctx, e.session(p, e.devA), model.DownloadRules{Enabled: true, NewPodcastEpisodes: true,
			MaxCount: maxCount, Quality: model.DownloadLow}); err != nil {
			t.Fatal(err)
		}
	}
	started := func(p model.Profile, itemID string) {
		e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, 2000, 0, ?)`,
			p.ID, itemID, e.clock.Now().UnixMilli())
	}
	auto := func(s model.OfflineSync) []string {
		var out []string
		for _, d := range s.Downloads {
			if d.Auto && d.Item != nil {
				out = append(out, d.Item.ID)
			}
		}
		slices.Sort(out)
		return out
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }

	// Without listening there is nothing to download.
	rule(e.f.P16, 3)
	if got := auto(e.sync(e.f.P16, e.devA)); len(got) != 0 {
		t.Fatalf("auto-downloads without listening: %v", got)
	}
	// P16 started episode 6: the newer visible episodes come, the explicit one does not.
	started(e.f.P16, a.Episodes[1].ID)
	if got, want := auto(e.sync(e.f.P16, e.devA)), sorted(a.EnclosureOnly.ID, a.Episodes[3].ID, a.Episodes[2].ID); !slices.Equal(got, want) {
		t.Errorf("P16 auto-downloads = %v, want %v", got, want)
	}
	// P18 on device B with maxCount 2: the two newest, the explicit episode included.
	if _, err := e.svc.SetRules(ctx, e.session(e.f.P18, e.devB), model.DownloadRules{Enabled: true, NewPodcastEpisodes: true,
		MaxCount: 2, Quality: model.DownloadLow}); err != nil {
		t.Fatal(err)
	}
	started(e.f.P18, a.Episodes[1].ID)
	if got, want := auto(e.sync(e.f.P18, e.devB)), sorted(a.EnclosureOnly.ID, a.Explicit.ID); !slices.Equal(got, want) {
		t.Errorf("P18 auto-downloads = %v, want %v", got, want)
	}
	// The next sync adds nothing: both places are taken by unfinished auto-downloads.
	if got := auto(e.sync(e.f.P18, e.devB)); len(got) != 2 {
		t.Errorf("P18 after a second sync = %v", got)
	}
}
