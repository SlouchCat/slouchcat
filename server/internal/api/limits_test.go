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
