package api

import (
	"encoding/json"
	"errors"
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

// The offline batch routes take up to MaxBatchBodyBytes (500 events are about 150 KB), every other
// route keeps MaxBodyBytes.
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
