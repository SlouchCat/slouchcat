package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBody(t *testing.T) {
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
