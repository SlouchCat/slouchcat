package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// HTTP tests of the offline operations (M5): the whole path of a download through the API with
// a fake encoder, the sync of a shared device, the revocation of a guest (K-7, AN-4) and the
// rating change of plan 10.2.

// contractEncoder stands for transcode.Runner in the contract fixture: an init segment and one
// small segment per planned segment.
type contractEncoder struct{}

func (contractEncoder) Download(ctx context.Context, _ transcode.Capabilities, _ transcode.Source, plan transcode.Plan, _ string,
	start int, onInit func([]byte) error, onSegment func(int, []byte, time.Duration, time.Duration) error) error {
	if err := onInit([]byte("ftyp-moov")); err != nil {
		return err
	}
	for n := start; n < len(plan.Segments); n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := onSegment(n, []byte(fmt.Sprintf("segment-%d|", n)), plan.Segments[n].Start, plan.Segments[n].Duration); err != nil {
			return err
		}
	}
	return nil
}

func (contractEncoder) Probe(context.Context, string) (transcode.Probe, error) {
	return transcode.Probe{Format: transcode.Format{Name: "mp3", Duration: 8 * time.Second, BitRate: 128_000},
		Streams: []transcode.Stream{{Index: 0, Type: "audio", Codec: "mp3", Channels: 2, Default: true}}}, nil
}

func (contractEncoder) Keyframes(context.Context, string) ([]time.Duration, error) {
	return []time.Duration{0, 4 * time.Second, 8 * time.Second}, nil
}

func (contractEncoder) Subtitles(ctx context.Context, path string, index int) ([]byte, error) {
	return fakeEmbeddedVTT(ctx, path, index)
}

func (contractEncoder) SubtitleStreams(ctx context.Context, path string, indexes []int) (map[int][]byte, error) {
	out := map[int][]byte{}
	for _, i := range indexes {
		vtt, err := fakeEmbeddedVTT(ctx, path, i)
		if err != nil {
			return nil, err
		}
		out[i] = vtt
	}
	return out, nil
}

func (contractEncoder) SubtitlesFile(ctx context.Context, path string) ([]byte, error) {
	return fakeSidecarVTT(ctx, path)
}

// offlineProbe is a probe of the fixture files for preparations: 1080p H.264 with the tracks of
// vistest.AddTracks, 12 s (three segments).
func offlineProbe(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(transcode.Probe{Format: transcode.Format{Duration: 12 * time.Second, BitRate: 4_000_000}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080, PixFmt: "yuv420p"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2, Default: true},
		{Index: 2, Type: "audio", Codec: "ac3", Channels: 6},
		{Index: 3, Type: "subtitle", Codec: "subrip"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// prepare runs the download worker of the fixture until the download is ready.
func (pf profileFixture) prepare(t *testing.T, id string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w := jobs.NewWorker(pf.queue, "download-1", vistest.Logger())
	pf.off.RegisterJobs(w)
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var state string
		if err := pf.f.DB.Read.QueryRow(`SELECT state FROM downloads WHERE id = ?`, id).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == model.DownloadReady {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("download %s still %s", id, state)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type syncAnswer struct {
	Downloads []struct {
		ID                string  `json:"id"`
		State             string  `json:"state"`
		AllowedProfileIDs []int64 `json:"allowedProfileIds"`
		SizeBytes         int64   `json:"sizeBytes"`
		Item              *struct {
			ID string `json:"id"`
		} `json:"item"`
	} `json:"downloads"`
	Revoked  []string `json:"revoked"`
	Profiles []struct {
		ID      int64  `json:"id"`
		PinSalt string `json:"pinSalt"`
	} `json:"profiles"`
}

func syncDevice(t *testing.T, h http.Handler, token, body string) (syncAnswer, *httptest.ResponseRecorder) {
	t.Helper()
	rec := send(t, h, http.MethodPost, BasePath+"/offline/sync", token, body, nil)
	var s syncAnswer
	if rec.Code == http.StatusOK {
		decode(t, rec, &s)
	}
	return s, rec
}

func createDownload(t *testing.T, h http.Handler, token, itemID, body string) string {
	t.Helper()
	rec := send(t, h, http.MethodPost, BasePath+"/items/"+itemID+"/downloads", token, body, nil)
	var list struct {
		Downloads []struct {
			ID string `json:"id"`
		} `json:"downloads"`
	}
	decode(t, rec, &list)
	if rec.Code != http.StatusCreated || len(list.Downloads) == 0 {
		t.Fatalf("create download of %s for %s: %d %s", itemID, token, rec.Code, rec.Body.String())
	}
	return list.Downloads[0].ID
}

// A device that P18 and P6 share: P18's FSK-18 download appears in P6's sync with its allowed
// profiles only; no part of the answer names the title (plan 3.3, design 3.4).
func TestOfflineSyncOfASharedDevice(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	shared := pf.sessions["p6"].DeviceID
	session, token, err := pf.a.StartSession(model.WithDeviceID(context.Background(), shared), f.Account, f.P18, "contract-test")
	if err != nil || session.DeviceID != shared {
		t.Fatalf("P18 session on P6's device: %v, %+v", err, session)
	}
	pf.pa.tokens["p18-shared"] = token
	if err := pf.a.SetProfilePIN(context.Background(), pf.sessions["p18"], f.P18.ID, "4711"); err != nil {
		t.Fatal(err)
	}
	m18 := f.Rated(18)
	id := createDownload(t, h, "p18-shared", m18.ID, `{"quality":"high"}`)

	s, rec := syncDevice(t, h, "p6", `{"held":[{"id":"`+id+`","complete":false}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("P6 sync: %d %s", rec.Code, rec.Body.String())
	}
	if found := itemIDsIn(t, rec, f.All()); len(found) != 0 {
		t.Errorf("P6's sync names titles %v: %s", found, rec.Body.String())
	}
	if len(s.Downloads) != 1 || s.Downloads[0].ID != id || s.Downloads[0].Item != nil || slices.Contains(s.Downloads[0].AllowedProfileIDs, f.P6.ID) {
		t.Errorf("P6 sync downloads = %+v", s.Downloads)
	}
	// P18's PIN salt goes to the device that holds a download P18 may play (AN-19), PX may play it
	// too; P6 is there as the active profile.
	var ids []int64
	for _, p := range s.Profiles {
		ids = append(ids, p.ID)
		if p.ID == f.P18.ID && len(p.PinSalt) != 32 {
			t.Errorf("P18 without PIN salt: %+v", p)
		}
	}
	slices.Sort(ids)
	if want := []int64{f.P18.ID, f.P6.ID, f.PX.ID}; !slices.Equal(ids, want) {
		t.Errorf("profiles = %v, want P18, P6 and PX %v", ids, want)
	}
	assertNoSecrets(t, "syncOffline", rec)
	// P18 on the same device sees the title.
	if s, _ := syncDevice(t, h, "p18-shared", `{"held":[]}`); len(s.Downloads) != 1 || s.Downloads[0].Item == nil || s.Downloads[0].Item.ID != m18.ID {
		t.Errorf("P18 sync = %+v", s.Downloads)
	}
	// P6 cannot read the manifest; a token of P6 for the file gets 404.
	if rec := send(t, h, http.MethodGet, BasePath+"/downloads/"+id+"/manifest", "p6", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("P6 manifest: %d", rec.Code)
	}
	// Validation: a malformed id is 422.
	if _, rec := syncDevice(t, h, "p6", `{"held":[{"id":"../x","complete":true}]}`); rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("malformed id: %d %s", rec.Code, rec.Body.String())
	}
}

// The path of a download through the API: create, prepare, sync, manifest, file and subtitle with
// the token URLs, the complete fetch, the rating change of plan 10.2 and the deletion.
func TestOfflineDownloadFlow(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	e1 := f.Show.E1
	f.SetMediaFile(t, e1.FileID, "mp4", "h264", "aac", offlineProbe(t), nil)

	// Options and creation: P12 downloads E1 in low.
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+e1.ID+"/download-options", "p12", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quality":"low"`) {
		t.Fatalf("options: %d %s", rec.Code, rec.Body.String())
	}
	id := createDownload(t, h, "p12", e1.ID, `{"quality":"low"}`)
	if rec := send(t, h, http.MethodGet, BasePath+"/downloads/"+id+"/manifest", "p12", "", nil); rec.Code != http.StatusConflict || problemCode(t, rec) != "download_not_ready" {
		t.Errorf("manifest before the preparation: %d %s", rec.Code, rec.Body.String())
	}
	pf.prepare(t, id)

	s, _ := syncDevice(t, h, "p12", `{"held":[]}`)
	if len(s.Downloads) != 1 || s.Downloads[0].State != "ready" || s.Downloads[0].Item == nil || s.Downloads[0].SizeBytes == 0 {
		t.Fatalf("sync after the preparation = %+v", s.Downloads)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/downloads/"+id+"/manifest", "p12", "", nil)
	var man struct {
		SizeBytes int64  `json:"sizeBytes"`
		FileURL   string `json:"fileUrl"`
		Segments  []struct {
			Offset, Length int64
		} `json:"segments"`
		Subtitles []struct {
			URL string `json:"url"`
		} `json:"subtitles"`
	}
	decode(t, rec, &man)
	if rec.Code != http.StatusOK || len(man.Segments) != 3 || len(man.Subtitles) != 2 {
		t.Fatalf("manifest: %d %s", rec.Code, rec.Body.String())
	}
	want := "ftyp-moovsegment-0|segment-1|segment-2|"
	whole := send(t, h, http.MethodGet, man.FileURL, "", "", nil)
	part := send(t, h, http.MethodGet, man.FileURL, "", "", map[string]string{"Range": "bytes=9-18"})
	if whole.Code != http.StatusOK || whole.Body.String() != want || part.Code != http.StatusPartialContent || part.Body.String() != "segment-0|" {
		t.Errorf("file: %d %q, range %d %q", whole.Code, whole.Body.String(), part.Code, part.Body.String())
	}
	// M5 review K9: a strong ETag of the preparation; If-Range with it resumes, with another one
	// (a copy prepared again) the whole file comes.
	etag := whole.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`+id+"-") || !strings.HasSuffix(etag, `"`) || part.Header().Get("ETag") != etag {
		t.Errorf("ETag = %q (range %q)", etag, part.Header().Get("ETag"))
	}
	if rec := send(t, h, http.MethodGet, man.FileURL, "", "", map[string]string{"Range": "bytes=9-18", "If-Range": etag}); rec.Code != http.StatusPartialContent || rec.Body.String() != "segment-0|" {
		t.Errorf("If-Range with the ETag: %d %q", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodGet, man.FileURL, "", "", map[string]string{"Range": "bytes=9-18", "If-Range": `"` + id + `-0"`}); rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("If-Range with an old ETag: %d %q", rec.Code, rec.Body.String())
	}
	for _, sub := range man.Subtitles {
		if rec := send(t, h, http.MethodGet, sub.URL, "", "", nil); rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "WEBVTT") {
			t.Errorf("subtitle %s: %d", sub.URL, rec.Code)
		}
	}
	// Token errors: forged 401, another download's id in the path 404, an unsatisfiable range 416.
	forged := strings.Replace(man.FileURL, "dt=", "dt=x", 1)
	if rec := send(t, h, http.MethodGet, forged, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("forged token: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, strings.Replace(man.FileURL, id, "01J9AAAAAAAAAAAAAAAAAAAAAA", 1), "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("token of another download: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, man.FileURL, "", "", map[string]string{"Range": "bytes=9999-"}); rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("unsatisfiable range: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, strings.Replace(man.Subtitles[0].URL, ".vtt", "9.vtt", 1), "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown subtitle: %d", rec.Code)
	}
	// The device reports the file complete.
	syncDevice(t, h, "p12", `{"held":[{"id":"`+id+`","complete":true}]}`)
	var fetched *int64
	if err := f.DB.Read.QueryRow(`SELECT fetched_at FROM downloads WHERE id = ?`, id).Scan(&fetched); err != nil || fetched == nil {
		t.Errorf("fetched_at = %v, %v", fetched, err)
	}

	// Plan 10.2: the owner raises the series to 16. P12 drops out at the next sync and its token stops.
	if rec := setRating(t, h, "p18", f.Show.Series.ID, `{"rating":16}`); rec.Code != http.StatusOK {
		t.Fatalf("set rating: %d %s", rec.Code, rec.Body.String())
	}
	s, rec = syncDevice(t, h, "p12", `{"held":[{"id":"`+id+`","complete":true}]}`)
	if len(s.Downloads) != 1 || s.Downloads[0].Item != nil || slices.Contains(s.Downloads[0].AllowedProfileIDs, f.P12.ID) {
		t.Errorf("P12 after the rating change = %+v", s.Downloads)
	}
	if found := itemIDsIn(t, rec, f.All()); len(found) != 0 {
		t.Errorf("P12's sync names %v", found)
	}
	if rec := send(t, h, http.MethodGet, man.FileURL, "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("P12's token after the rating change: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/downloads/"+id+"/manifest", "p12", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("P12's manifest after the rating change: %d", rec.Code)
	}

	// Deletion (here by the primary profile, M5 review K1): the device learns it at its next sync.
	if rec := send(t, h, http.MethodDelete, BasePath+"/downloads/"+id, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/downloads/"+id, "pg", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("guest deletes: %d", rec.Code)
	}
	s, _ = syncDevice(t, h, "p12", `{"held":[{"id":"`+id+`","complete":true}]}`)
	if !slices.Equal(s.Revoked, []string{id}) || len(s.Downloads) != 0 {
		t.Errorf("after the delete: revoked %v, downloads %+v", s.Revoked, s.Downloads)
	}
	// M5 review K8: a token of a download that is gone answers like a forged one (401, no
	// existence oracle), for the file and the subtitles.
	for _, u := range []string{man.FileURL, man.Subtitles[0].URL} {
		if rec := send(t, h, http.MethodGet, u, "", "", nil); rec.Code != http.StatusUnauthorized || problemCode(t, rec) != "unauthenticated" {
			t.Errorf("token of a deleted download %s: %d %s", u, rec.Code, rec.Body.String())
		}
	}
	if _, err := os.Stat(pf.dlDir + "/" + id); err == nil {
		t.Error("the server copy remains after the delete")
	}
}

// K-7, AN-4: revoking a guest deletes its downloads through the cascade; its sync answers 401 and
// checkOfflineRevocations, by the device cookie alone, lists the ids the device must delete.
func TestOfflineGuestRevocation(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	id := createDownload(t, h, "pg", f.Rated(6).ID, `{"quality":"medium"}`)
	device := pf.sessions["pg"].DeviceID
	revocations := func(cookie string) (int, []string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, BasePath+"/offline/revocations",
			strings.NewReader(`{"ids":["`+id+`","01J9AAAAAAAAAAAAAAAAAAAAAA"]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(CSRFHeader, CSRFHeaderValue)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: DeviceCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out struct {
			Revoked []string `json:"revoked"`
		}
		if rec.Code == http.StatusOK {
			decode(t, rec, &out)
		}
		return rec.Code, out.Revoked
	}
	if code, got := revocations(device); code != http.StatusOK || len(got) != 0 {
		t.Fatalf("before the revocation: %d %v", code, got)
	}
	if rec := send(t, h, http.MethodDelete, fmt.Sprintf("%s/admin/guests/%d", BasePath, f.Guest.ID), "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke guest: %d %s", rec.Code, rec.Body.String())
	}
	if _, rec := syncDevice(t, h, "pg", `{"held":[{"id":"`+id+`","complete":true}]}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest sync after the revocation: %d", rec.Code)
	}
	if code, got := revocations(device); code != http.StatusOK || !slices.Equal(got, []string{id}) {
		t.Errorf("revocations of the guest's device: %d %v", code, got)
	}
	if code, got := revocations(""); code != http.StatusOK || len(got) != 0 {
		t.Errorf("without cookie: %d %v", code, got)
	}
	if code, got := revocations(pf.sessions["p18"].DeviceID); code != http.StatusOK || len(got) != 0 {
		t.Errorf("another device: %d %v", code, got)
	}
}
