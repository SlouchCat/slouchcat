package offline

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestManifest(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	m := f.Rated(12)
	st := e.create(f.P12, e.devA, m.ID, model.DownloadLow)
	if _, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st.ID); !errors.Is(err, model.ErrDownloadNotReady) {
		t.Errorf("queued: %v, want ErrDownloadNotReady", err)
	}
	e.runJobs()
	man, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if man.ID != st.ID || man.ItemID != m.ID || man.SizeBytes != int64(len(wantMedia(5))) || man.InitLength != int64(len(initData)) ||
		man.DurationMs != clipDuration.Milliseconds() || man.MimeType != "video/mp4" || man.Codecs != "avc1.640029,mp4a.40.2" ||
		man.Width != 852 || man.Height != 480 || len(man.Segments) != 5 {
		t.Fatalf("manifest = %+v", man)
	}
	if s := man.Segments[4]; s.StartMs != 16000 || s.DurationMs != 4000 || s.Offset+s.Length != man.SizeBytes {
		t.Errorf("last segment = %+v", s)
	}
	if man.Audio == nil || man.Audio.Index != 1 || man.Audio.Codec != "aac" {
		t.Errorf("audio = %+v", man.Audio)
	}
	ft := e.tracks[m.FileID]
	if len(man.Subtitles) != 2 || man.Subtitles[0].Track.ID != ft.Text || man.Subtitles[1].Track.ID != ft.Sidecar ||
		!strings.HasPrefix(man.Subtitles[0].URL, "/api/v1/downloads/"+st.ID+"/subtitles/"+strconv.FormatInt(ft.Text, 10)+".vtt?dt=") {
		t.Errorf("subtitles = %+v", man.Subtitles)
	}
	// The file URL carries a token of the active profile.
	u, err := url.Parse(man.FileURL)
	if err != nil || u.Path != "/api/v1/downloads/"+st.ID+"/file" {
		t.Fatalf("file URL %q", man.FileURL)
	}
	if want := SignToken(testKey, st.ID, m.ID, f.P12.ID); u.Query().Get("dt") != want {
		t.Errorf("token = %q, want %q", u.Query().Get("dt"), want)
	}
	// Another device, another account, a profile that cannot see the title: 404.
	for _, s := range []model.Session{e.session(f.P12, e.devB), e.session(f.PG, e.devA), e.session(f.P6, e.devA)} {
		if _, err := e.svc.Manifest(ctx, s, st.ID); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s on %s: %v, want ErrNotFound", s.Profile.Name, s.DeviceID[:1], err)
		}
	}
	if _, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), "not-a-ulid"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("malformed id: %v", err)
	}
	// A ready row whose copy vanished is evicted on the spot.
	if err := os.Remove(filepath.Join(e.dir, st.ID, indexFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st.ID); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("without copy: %v, want ErrDownloadEvicted", err)
	}
	if r := e.row(st.ID); r.state != model.DownloadEvicted {
		t.Errorf("state = %s", r.state)
	}
	if _, err := e.svc.Manifest(ctx, e.session(f.P12, e.devA), st.ID); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("evicted: %v, want ErrDownloadEvicted", err)
	}
}

func TestDownloadToken(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	m := f.Rated(12)
	st := e.create(f.P12, e.devA, m.ID, model.DownloadLow)
	e.runJobs()

	g, err := e.svc.AuthorizeDownload(ctx, SignToken(testKey, st.ID, m.ID, f.P12.ID))
	if err != nil || g != (model.DownloadGrant{DownloadID: st.ID, ItemID: m.ID, AccountID: f.Account.ID, ProfileID: f.P12.ID}) {
		t.Fatalf("grant = %+v, %v", g, err)
	}
	src, err := e.svc.OpenFile(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(src.File)
	_ = src.File.Close()
	if string(data) != string(wantMedia(5)) || src.MediaType != "video/mp4" || src.Size != int64(len(data)) {
		t.Errorf("file = %q (%s)", data, src.MediaType)
	}
	ft := e.tracks[m.FileID]
	sub, err := e.svc.OpenSubtitle(ctx, g, ft.Sidecar)
	if err != nil {
		t.Fatal(err)
	}
	_ = sub.File.Close()
	if sub.MediaType != "text/vtt; charset=utf-8" {
		t.Errorf("subtitle type %s", sub.MediaType)
	}
	for _, track := range []int64{ft.Bitmap, ft.AudioDefault, 0, 999999} {
		if _, err := e.svc.OpenSubtitle(ctx, g, track); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("subtitle %d: %v, want ErrNotFound", track, err)
		}
	}

	// Missing, malformed, forged: 401. Another title in the MAC, another key: 401.
	for _, tok := range []string{"", "x", st.ID + ".1.AAAA", SignToken([]byte("other-key"), st.ID, m.ID, f.P12.ID),
		SignToken(testKey, st.ID, f.Rated(0).ID, f.P12.ID), strings.Replace(SignToken(testKey, st.ID, m.ID, f.P12.ID), "."+strconv.FormatInt(f.P12.ID, 10)+".", "."+strconv.FormatInt(f.P18.ID, 10)+".", 1)} {
		if _, err := e.svc.AuthorizeDownload(ctx, tok); !errors.Is(err, model.ErrUnauthenticated) {
			t.Errorf("token %q: %v, want ErrUnauthenticated", tok, err)
		}
	}
	// The profile of the token decides: P18 sees it, P6 does not, a guest profile is not of the account.
	if _, err := e.svc.AuthorizeDownload(ctx, SignToken(testKey, st.ID, m.ID, f.P18.ID)); err != nil {
		t.Errorf("P18 token: %v", err)
	}
	for _, p := range []model.Profile{f.P6, f.PG18} {
		if _, err := e.svc.AuthorizeDownload(ctx, SignToken(testKey, st.ID, m.ID, p.ID)); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s token: %v, want ErrNotFound", p.Name, err)
		}
	}
	// The rating rises above P12: the token of P12 stops working on the next request.
	sixteen := 16
	f.SetRating(t, m.ID, &sixteen)
	if _, err := e.svc.AuthorizeDownload(ctx, SignToken(testKey, st.ID, m.ID, f.P12.ID)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("after the rating change: %v, want ErrNotFound", err)
	}
	// Evicted copy: 410; deleted download: 404.
	e.exec(`UPDATE downloads SET state = 'evicted', files_removed_at = 1 WHERE id = ?`, st.ID)
	if _, err := e.svc.OpenFile(ctx, g); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("evicted file: %v", err)
	}
	if _, err := e.svc.OpenSubtitle(ctx, g, ft.Sidecar); !errors.Is(err, model.ErrDownloadEvicted) {
		t.Errorf("evicted subtitle: %v", err)
	}
	e.exec(`DELETE FROM downloads WHERE id = ?`, st.ID)
	if _, err := e.svc.AuthorizeDownload(ctx, SignToken(testKey, st.ID, m.ID, f.P18.ID)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted: %v, want ErrNotFound", err)
	}
	// Without a key no token is valid.
	e.svc.key = nil
	if _, err := e.svc.AuthorizeDownload(ctx, SignToken(nil, st.ID, m.ID, f.P18.ID)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("without key: %v", err)
	}
}

func TestParseIndexStopsAtTheFirstUnusableLine(t *testing.T) {
	good := `{"init":10,"codecs":"c","width":2,"height":2}` + "\n" +
		`{"n":0,"startMs":0,"durationMs":4000,"offset":10,"length":5}` + "\n" +
		`{"n":1,"startMs":4000,"durationMs":4000,"offset":15,"length":5}` + "\n"
	for _, tc := range []struct {
		name    string
		data    string
		size    int64
		ok      bool
		entries int
	}{
		{"complete", good, 20, true, 2},
		{"partial last line", good + `{"n":2,"st`, 30, true, 2},
		{"file shorter than an entry", good, 18, true, 1},
		{"gap", `{"init":10}` + "\n" + `{"n":0,"startMs":0,"durationMs":4000,"offset":11,"length":5}` + "\n", 20, true, 0},
		{"wrong number", `{"init":10}` + "\n" + `{"n":1,"startMs":0,"durationMs":4000,"offset":10,"length":5}` + "\n", 20, true, 0},
		{"no header", "", 0, false, 0},
		{"partial header", `{"init":1`, 10, false, 0},
		{"init beyond the file", `{"init":50}` + "\n", 10, false, 0},
	} {
		ix, ok := parseIndex([]byte(tc.data), tc.size)
		if ok != tc.ok || len(ix.entries) != tc.entries {
			t.Errorf("%s: ok %v with %d entries, want %v with %d", tc.name, ok, len(ix.entries), tc.ok, tc.entries)
		}
	}
}
