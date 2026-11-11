package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBody(t *testing.T) {
	t.Parallel()
	var decodeErr error
	h := MaxBody(MaxBodyBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]string
		decodeErr = json.NewDecoder(r.Body).Decode(&v)
		w.WriteHeader(http.StatusNoContent)
	}))

	small := `{"email":"marco@example.ch"}`
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(small)))
	if decodeErr != nil {
		t.Fatalf("small body: %v", decodeErr)
	}

	big := `{"password":"` + strings.Repeat("x", MaxBodyBytes) + `"}`
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(big)))
	var maxErr *http.MaxBytesError
	if !errors.As(decodeErr, &maxErr) || maxErr.Limit != MaxBodyBytes {
		t.Fatalf("oversized body: decode error %v, want *http.MaxBytesError with limit %d", decodeErr, MaxBodyBytes)
	}
}

// The offline batch routes take up to MaxBatchBodyBytes (500 events are about 150 KB), the backup
// upload more (TestMaxBodyRestoreUpload), every other route keeps MaxBodyBytes.
func TestMaxBodyBatchRoutes(t *testing.T) {
	t.Parallel()
	var decodeErr error
	h := MaxBody(MaxBodyBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]string
		decodeErr = json.NewDecoder(r.Body).Decode(&v)
		w.WriteHeader(http.StatusNoContent)
	}))
	body := func(n int) string { return `{"events":"` + strings.Repeat("x", n) + `"}` }
	for _, tc := range []struct {
		method, path string
		size         int
		limit        int64 // 0 = the body passes
	}{
		{http.MethodPost, BasePath + "/offline/events", 200 << 10, 0},
		{http.MethodPost, BasePath + "/offline/sync", 200 << 10, 0},
		{http.MethodPost, BasePath + "/offline/revocations", 200 << 10, 0},
		{http.MethodPost, BasePath + "/offline/events", MaxBatchBodyBytes, MaxBatchBodyBytes},
		{http.MethodPut, BasePath + "/offline/events", 200 << 10, MaxBodyBytes},
		{http.MethodPost, BasePath + "/offline/events/x", 200 << 10, MaxBodyBytes},
		{http.MethodPost, BasePath + "/items/x/progress", 200 << 10, MaxBodyBytes},
		{http.MethodPost, BasePath + "/admin/backups/restore", 200 << 10, 0},
		{http.MethodPost, BasePath + "/admin/backups/x/restore", 200 << 10, MaxBodyBytes},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, strings.NewReader(body(tc.size))))
		var maxErr *http.MaxBytesError
		switch {
		case tc.limit == 0 && decodeErr != nil:
			t.Errorf("%s %s with %d bytes: %v", tc.method, tc.path, tc.size, decodeErr)
		case tc.limit != 0 && (!errors.As(decodeErr, &maxErr) || maxErr.Limit != tc.limit):
			t.Errorf("%s %s with %d bytes: decode error %v, want *http.MaxBytesError with limit %d",
				tc.method, tc.path, tc.size, decodeErr, tc.limit)
		}
	}
}

// The backup upload (restoreBackupUpload) streams up to MaxRestoreBodyBytes, one byte more fails.
func TestMaxBodyRestoreUpload(t *testing.T) {
	t.Parallel()
	var n int64
	var readErr error
	h := MaxBody(MaxBodyBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, readErr = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, size := range []int64{MaxRestoreBodyBytes, MaxRestoreBodyBytes + 1} {
		req := httptest.NewRequest(http.MethodPost, BasePath+"/admin/backups/restore", io.LimitReader(endlessBody{}, size))
		h.ServeHTTP(httptest.NewRecorder(), req)
		var maxErr *http.MaxBytesError
		switch {
		case size == MaxRestoreBodyBytes && (readErr != nil || n != size):
			t.Errorf("%d bytes: read %d, %v; want all", size, n, readErr)
		case size > MaxRestoreBodyBytes && (!errors.As(readErr, &maxErr) || maxErr.Limit != MaxRestoreBodyBytes):
			t.Errorf("%d bytes: read error %v, want *http.MaxBytesError with limit %d", size, readErr, int64(MaxRestoreBodyBytes))
		}
	}
}

// endlessBody is a body of any length that costs no memory (its bytes are whatever the buffer holds).
type endlessBody struct{}

func (endlessBody) Read(p []byte) (int, error) { return len(p), nil }

// The image upload (uploadItemImage) is a route with parameters: largeBodies matches it by
// segments, so any title and kind get MaxImageUploadBodyBytes, while another method, an empty
// segment or a longer path keep MaxBodyBytes.
func TestMaxBodyImageUploadPattern(t *testing.T) {
	t.Parallel()
	var n int64
	var readErr error
	h := MaxBody(MaxBodyBytes)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, readErr = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	upload := BasePath + "/admin/items/01HXYZ/images/poster/upload"
	for _, tc := range []struct {
		method, path string
		size         int64
		limit        int64 // 0 = the body passes
	}{
		{http.MethodPut, upload, 2 << 20, 0},
		{http.MethodPut, BasePath + "/admin/items/01ABC/images/backdrop/upload", MaxImageUploadBodyBytes, 0},
		{http.MethodPut, upload, MaxImageUploadBodyBytes + 1, MaxImageUploadBodyBytes},
		{http.MethodPost, upload, 2 << 20, MaxBodyBytes},
		{http.MethodPut, BasePath + "/admin/items//images/poster/upload", 2 << 20, MaxBodyBytes},
		{http.MethodPut, upload + "/x", 2 << 20, MaxBodyBytes},
		{http.MethodPut, BasePath + "/admin/items/01HXYZ/images/poster", 2 << 20, MaxBodyBytes},
		{http.MethodPost, BasePath + "/admin/backups/restore", 2 << 20, 0},
	} {
		req := httptest.NewRequest(tc.method, tc.path, io.LimitReader(endlessBody{}, tc.size))
		h.ServeHTTP(httptest.NewRecorder(), req)
		var maxErr *http.MaxBytesError
		switch {
		case tc.limit == 0 && (readErr != nil || n != tc.size):
			t.Errorf("%s %s with %d bytes: read %d, %v; want all", tc.method, tc.path, tc.size, n, readErr)
		case tc.limit != 0 && (!errors.As(readErr, &maxErr) || maxErr.Limit != tc.limit):
			t.Errorf("%s %s with %d bytes: read error %v, want *http.MaxBytesError with limit %d",
				tc.method, tc.path, tc.size, readErr, tc.limit)
		}
	}
}

// A draft of the radio offer (applyRadioOffer, steps with up to 5000 station ids) takes
// MaxRadioOfferBodyBytes: three steps of 5000 ids, about 110 KB, pass end to end.
func TestRadioOfferBodyLimit(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/admin/radio/offer/preview", "/admin/radio/offer/apply"} {
		if got := bodyLimit(http.MethodPost + " " + BasePath + path); got != MaxRadioOfferBodyBytes {
			t.Fatalf("limit of POST %s = %d, want %d", path, got, MaxRadioOfferBodyBytes)
		}
	}
	ids := make([]string, 0, 5000)
	for i := range 5000 {
		ids = append(ids, fmt.Sprint(100000+i))
	}
	step := `{"op":"mark","stationIds":[` + strings.Join(ids, ",") + `]}`
	body := `{"version":0,"steps":[` + step + "," + step + "," + step + `]}`
	if len(body) <= MaxBodyBytes {
		t.Fatalf("body of %d bytes does not exceed the default limit", len(body))
	}
	pf := newProfileFixture(t)
	rec := send(t, pf.h, http.MethodPost, BasePath+"/admin/radio/offer/apply", "p18", body, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":1`) {
		t.Fatalf("apply of 15,000 ids: status %d (%s)", rec.Code, rec.Body.String()[:min(rec.Body.Len(), 300)])
	}
}
