package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// levelRecorder keeps the level of every "http request" record.
type levelRecorder struct {
	mu     sync.Mutex
	levels []slog.Level
}

func (l *levelRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (l *levelRecorder) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.levels = append(l.levels, r.Level)
	return nil
}
func (l *levelRecorder) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *levelRecorder) WithGroup(string) slog.Handler      { return l }

// The stop watch's answers (every 4 s per playing player, concept 13) log at Debug; its errors and
// every other request stay at Info.
func TestRequestLogQuietsStopWatch(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
		want         slog.Level
	}{
		{http.MethodGet, BasePath + "/playback/sessions/01JUNKNOWN0000000000000000/state", http.StatusOK, slog.LevelDebug},
		{http.MethodGet, BasePath + "/playback/sessions/01JUNKNOWN0000000000000000/state", http.StatusNotFound, slog.LevelInfo},
		{http.MethodGet, BasePath + "/playback/sessions/01JUNKNOWN0000000000000000/state", http.StatusUnauthorized, slog.LevelInfo},
		{http.MethodDelete, BasePath + "/playback/sessions/01JUNKNOWN0000000000000000", http.StatusOK, slog.LevelInfo},
		{http.MethodGet, BasePath + "/items/x", http.StatusOK, slog.LevelInfo},
	} {
		rec := &levelRecorder{}
		h := requestLog(slog.New(rec))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
		if len(rec.levels) != 1 || rec.levels[0] != tc.want {
			t.Errorf("%s %s %d: levels %v, want %v", tc.method, tc.path, tc.status, rec.levels, tc.want)
		}
	}
}
