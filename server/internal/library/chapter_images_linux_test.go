package library

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A read test that hangs (a FIFO without writer stands in for a hard-mounted share that went
// away) times out; while it hangs, every further file of the same root counts as unreadable at
// once without a new read, other roots are read; once the hanging read returns, the root is read
// again.
func TestReadableHangingRoot(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	root, other := t.TempDir(), t.TempDir()
	fifo := filepath.Join(root, "hang.mkv")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(root, "film.mkv")
	elsewhere := filepath.Join(other, "film.mkv")
	for _, f := range []string{plain, elsewhere} {
		if err := os.WriteFile(f, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.svc.readable(root, plain, time.Second); err != nil {
		t.Fatalf("plain file: %v", err)
	}
	if err := e.svc.readable(root, fifo, 50*time.Millisecond); err == nil || errors.Is(err, errReadHanging) {
		t.Fatalf("hanging read: %v, want a timeout", err)
	}
	if err := e.svc.readable(root, plain, time.Second); !errors.Is(err, errReadHanging) {
		t.Errorf("same root while a read hangs: %v, want errReadHanging", err)
	}
	if err := e.svc.readable(other, elsewhere, time.Second); err != nil {
		t.Errorf("other root while a read hangs: %v", err)
	}
	// A writer ends the hanging read (EOF on close); then the root is read again.
	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := e.svc.readable(root, plain, time.Second)
		if err == nil {
			break
		}
		if !errors.Is(err, errReadHanging) || time.Now().After(deadline) {
			t.Fatalf("after the hanging read returned: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
