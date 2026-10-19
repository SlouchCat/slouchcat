package library

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// testClock is a manually advanced clock, safe for concurrent use.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type env struct {
	t     *testing.T
	ctx   context.Context
	db    *store.DB
	queue *jobs.Queue
	clock *testClock
	svc   *Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clk := &testClock{now: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	q := jobs.NewQueue(db.Write, clk.Now)
	svc := New(db, q, transcode.Runner{FFprobePath: ffprobePath()}, discard, WithClock(clk.Now))
	return &env{t: t, ctx: ctx, db: db, queue: q, clock: clk, svc: svc}
}

// ffprobePath finds ffprobe in PATH without os/exec (reserved for package transcode).
func ffprobePath() string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, "ffprobe")
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// create adds a movies library on dir.
func (e *env) create(dir string) model.Library {
	e.t.Helper()
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Filme", Kind: "movies", Path: dir})
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

// scan runs a full scan of the library synchronously and returns its state.
func (e *env) scan(id int64) model.Library {
	e.t.Helper()
	lib, err := appq.New(e.db.Read).GetLibrary(e.ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.svc.Scan(e.ctx, lib); err != nil {
		e.t.Fatalf("scan: %v", err)
	}
	m, err := e.svc.Get(e.ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db.Read.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

func (e *env) str(query string, args ...any) string {
	e.t.Helper()
	var s sql.NullString
	if err := e.db.Read.QueryRowContext(e.ctx, query, args...).Scan(&s); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return s.String
}

func (e *env) jobCount(typ string) int {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM jobs WHERE type = ?`, typ)
}

// writeFile creates a file with content below dir, creating parent directories.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func rename(t *testing.T, dir, from, to string) {
	t.Helper()
	dst := filepath.Join(dir, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, filepath.FromSlash(from)), dst); err != nil {
		t.Fatal(err)
	}
}

func testRequest(name, dir string) model.LibraryRequest {
	return model.LibraryRequest{Name: name, Kind: "movies", Path: dir}
}
