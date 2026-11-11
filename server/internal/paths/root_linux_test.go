package paths

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

// LstatAll opens the directories of rel once per call, not once per name. On NFS every open is a
// GETATTR round trip, so one walk per name made a page of 102 folders four levels down cost 424
// round trips and tripped the watchdog of the folder browser on a busy NAS (CT 190, 11 November
// 2026). inotify sees every open of a watched directory, which is what this test counts.
func TestLstatAllOpensRelOnce(t *testing.T) {
	dir := t.TempDir()
	rel := "a/b/c"
	names := make([]string, 0, 40)
	for i := range 40 {
		name := fmt.Sprintf("d%02d", i)
		names = append(names, name)
		if err := os.MkdirAll(filepath.Join(dir, "a", "b", "c", name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	root, err := NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}

	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatalf("inotify: %v", err)
	}
	defer syscall.Close(fd)
	// The watch on a reports the opens of its child b by name: one per walk through rel.
	if _, err := syscall.InotifyAddWatch(fd, filepath.Join(dir, "a"), syscall.IN_OPEN); err != nil {
		t.Fatal(err)
	}

	entries, err := LstatAll(root, rel, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Fatalf("LstatAll = %d entries, want %d", len(entries), len(names))
	}
	if opens := countOpens(t, fd, "b"); opens != 1 {
		t.Errorf("b was opened %d times for %d names, want once", opens, len(names))
	}
}

// countOpens drains the inotify queue and counts the IN_OPEN events of the child name.
func countOpens(t *testing.T, fd int, name string) int {
	t.Helper()
	buf := make([]byte, 64*1024)
	count := 0
	for {
		n, err := syscall.Read(fd, buf)
		if err == syscall.EAGAIN || n <= 0 {
			return count
		}
		if err != nil {
			t.Fatal(err)
		}
		for off := 0; off+syscall.SizeofInotifyEvent <= n; {
			ev := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[off]))
			raw := buf[off+syscall.SizeofInotifyEvent : off+syscall.SizeofInotifyEvent+int(ev.Len)]
			if ev.Mask&syscall.IN_OPEN != 0 && string(bytes.TrimRight(raw, "\x00")) == name {
				count++
			}
			off += syscall.SizeofInotifyEvent + int(ev.Len)
		}
	}
}
