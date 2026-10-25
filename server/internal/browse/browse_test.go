package browse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// tree builds a directory with the given subdirectories and files and returns it with a Lister
// over the real file system.
func tree(t *testing.T, dirs, files []string) (string, []Root, *Lister) {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(sub)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(f)), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, []Root{{ID: "c0", Dir: dir, Browsable: true}}, New(discard)
}

func TestListDirectories(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir, roots, l := tree(t,
		[]string{"Filme", "Serien", "Filme/Action", "@eaDir", ".hidden", "BDMV"},
		[]string{"notiz.txt"})

	got, err := l.List(ctx, roots, Request{RootID: "c0"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.AtRoot || got.Rel != "" || got.Path != dir {
		t.Errorf("at the root: %+v", got)
	}
	if names := names(got); strings.Join(names, "|") != "Filme|Serien" {
		t.Errorf("entries = %v, want the two directories sorted; files, hidden entries, @eaDir and BDMV are skipped", names)
	}
	if got.FSType == "" || got.Watch == "" {
		t.Errorf("the listed directory carries no file system or watch mode: %+v", got)
	}
	if got.Truncated || got.NextCursor != "" || got.Problem != "" {
		t.Errorf("a small directory is neither truncated nor paged nor a problem: %+v", got)
	}
	for _, e := range got.Entries {
		if e.Path != filepath.Join(dir, e.Name) || e.Rel != e.Name {
			t.Errorf("entry %+v: want rel and path below the root", e)
		}
	}

	// One level down: the parent leads back to the root.
	got, err = l.List(ctx, roots, Request{RootID: "c0", Rel: "Filme"})
	if err != nil {
		t.Fatal(err)
	}
	if got.AtRoot || got.Rel != "Filme" || got.ParentRel != "" {
		t.Errorf("below the root: %+v, want rel Filme and the root as parent", got)
	}
	if names := names(got); strings.Join(names, "|") != "Action" {
		t.Errorf("entries of Filme = %v", names)
	}
	// Two levels down: the parent is the directory above.
	got, err = l.List(ctx, roots, Request{RootID: "c0", Rel: "Filme/Action"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentRel != "Filme" || len(got.Entries) != 0 {
		t.Errorf("Filme/Action: %+v, want parent Filme and no entries", got)
	}
}

// E15: a symlinked directory is listed, marked and cannot be entered; the answer never names
// what lies behind it. E6: that 404 is the same one an unknown root gets.
func TestListSymlinkIsShownButNotEntered(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	dir, roots, l := tree(t, []string{"Filme"}, nil)
	if err := os.Symlink(outside, filepath.Join(dir, "aussen")); err != nil {
		t.Fatal(err)
	}

	got, err := l.List(ctx, roots, Request{RootID: "c0"})
	if err != nil {
		t.Fatal(err)
	}
	var link *model.DirectoryEntry
	for i, e := range got.Entries {
		if e.Name == "aussen" {
			link = &got.Entries[i]
		}
	}
	if link == nil || !link.Symlink {
		t.Fatalf("entries = %+v, want 'aussen' marked as a symlink", got.Entries)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), outside) {
		t.Errorf("the answer names the target of the symlink: %+v", got)
	}
	for _, rel := range []string{"aussen", "aussen/config"} {
		out, err := l.List(ctx, roots, Request{RootID: "c0", Rel: rel})
		if !errors.Is(err, model.ErrPathNotBrowsable) {
			t.Errorf("List(%q) = %+v, %v; want ErrPathNotBrowsable", rel, out, err)
		}
		if strings.Contains(fmt.Sprintf("%+v%v", out, err), "config") {
			t.Errorf("List(%q) names something behind the symlink: %+v %v", rel, out, err)
		}
	}
}

// A directory below a root that cannot be read is no 404: it is a listing with a problem, so the
// dialog can say why. A path below no root never gets that far (E6).
func TestListProblems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir, roots, l := tree(t, []string{"Filme"}, []string{"film.mkv"})

	got, err := l.List(ctx, roots, Request{RootID: "c0", Rel: "film.mkv"})
	if err != nil {
		t.Fatalf("a file below the root = %v, want a listing with a problem", err)
	}
	if got.Problem != "not_directory" || len(got.Entries) != 0 {
		t.Errorf("a file below the root = %+v, want problem not_directory", got)
	}
	if os.Geteuid() != 0 { // root reads every directory
		locked := filepath.Join(dir, "gesperrt")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
		got, err := l.List(ctx, roots, Request{RootID: "c0", Rel: "gesperrt"})
		if err != nil || got.Problem != "not_readable" {
			t.Errorf("an unreadable directory = %+v, %v; want problem not_readable", got, err)
		}
	}
	// Nothing of that kind below the root: the same 404 an unknown root gets.
	for _, req := range []Request{{RootID: "c0", Rel: "gibtsnicht"}, {RootID: "unbekannt"}} {
		if _, err := l.List(ctx, roots, req); !errors.Is(err, model.ErrPathNotBrowsable) {
			t.Errorf("List(%+v) = %v, want ErrPathNotBrowsable", req, err)
		}
	}
	// A root that exists but may not be entered answers alike (a grandfathered library root).
	closed := []Root{{ID: "l1", Dir: dir, Browsable: false, LibraryID: 1, LibraryName: "Alt"}}
	if _, err := l.List(ctx, closed, Request{RootID: "l1"}); !errors.Is(err, model.ErrPathNotBrowsable) {
		t.Errorf("a root that is not browsable = %v, want ErrPathNotBrowsable", err)
	}
}

// Every form of rel that could leave the root is a validation error, not a 404: the request is
// malformed before anything is looked up.
func TestListRejectsBadRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, roots, l := tree(t, []string{"Filme"}, nil)
	cases := map[string]Request{
		"dot dot":       {RootID: "c0", Rel: ".."},
		"inside":        {RootID: "c0", Rel: "a/../b"},
		"trailing":      {RootID: "c0", Rel: "Filme/.."},
		"absolute":      {RootID: "c0", Rel: "/etc/passwd"},
		"nul":           {RootID: "c0", Rel: "a\x00b"},
		"too long":      {RootID: "c0", Rel: strings.Repeat("a", MaxRel+1)},
		"no root":       {Rel: "Filme"},
		"limit high":    {RootID: "c0", Limit: MaxLimit + 1},
		"limit low":     {RootID: "c0", Limit: -1},
		"bad cursor":    {RootID: "c0", Cursor: "not base64 $$"},
		"other dir":     {RootID: "c0", Cursor: encodeCursor("c0", "Filme", "A")},
		"other root":    {RootID: "c0", Cursor: encodeCursor("c1", ".", "A")},
		"cursor length": {RootID: "c0", Cursor: strings.Repeat("a", MaxRel+1)},
	}
	for name, req := range cases {
		var verr *model.ValidationError
		if _, err := l.List(ctx, roots, req); !errors.As(err, &verr) {
			t.Errorf("%s: List = %v, want a validation error", name, err)
		}
	}
}

// E7: a large directory is paged, and a directory with more entries than the server reads says
// so instead of pretending to be complete. Counted against an injected file system, so the cost
// per page is visible: one read, one statfs, nothing per row.
func TestListPagesAndTruncates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	roots := []Root{{ID: "c0", Dir: "/media", Browsable: true}}

	f := &fakeFS{entries: dirEntries(300), fsType: paths.FSExt4}
	l := New(discard, WithFS(f))
	seen := []string{}
	req := Request{RootID: "c0", Limit: 120}
	for page := 0; ; page++ {
		if page > 5 {
			t.Fatal("the pages do not end")
		}
		got, err := l.List(ctx, roots, req)
		if err != nil {
			t.Fatal(err)
		}
		if got.Truncated {
			t.Errorf("300 entries are not truncated: %+v", got)
		}
		seen = append(seen, names(got)...)
		if got.NextCursor == "" {
			break
		}
		if len(got.Entries) != 120 {
			t.Errorf("page %d has %d rows, want the full limit", page, len(got.Entries))
		}
		req.Cursor = got.NextCursor
	}
	if len(seen) != 300 {
		t.Fatalf("%d rows over all pages, want 300", len(seen))
	}
	for i := range seen {
		if i > 0 && seen[i-1] >= seen[i] {
			t.Fatalf("the pages are not sorted and free of repeats: %q after %q", seen[i], seen[i-1])
		}
	}
	if f.reads.Load() != 3 || f.stats.Load() != 3 {
		t.Errorf("three pages cost %d reads and %d statfs, want one each per page and nothing per row",
			f.reads.Load(), f.stats.Load())
	}

	// More entries than the server reads: exactly one page plus the mark.
	f = &fakeFS{entries: dirEntries(MaxEntries + 1), fsType: paths.FSNFS}
	l = New(discard, WithFS(f))
	got, err := l.List(ctx, roots, Request{RootID: "c0"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Entries) != DefaultLimit || got.NextCursor == "" {
		t.Errorf("a huge directory = truncated %v, %d rows, cursor %q", got.Truncated, len(got.Entries), got.NextCursor)
	}
	if got.Watch != "poll" || got.Warning != "network_share" {
		t.Errorf("an NFS directory = watch %q, warning %q; want poll and the network share hint", got.Watch, got.Warning)
	}
	if f.reads.Load() != 1 || f.stats.Load() != 1 {
		t.Errorf("one page cost %d reads and %d statfs", f.reads.Load(), f.stats.Load())
	}
}

// A directory that already belongs to a library carries its name, whether the library root lies
// below it or it below the root.
func TestListMarksLibraryFolders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir, roots, l := tree(t, []string{"Filme", "Serien", "Neu"}, nil)
	roots = append(roots,
		Root{ID: "l1", Dir: filepath.Join(dir, "Filme"), Browsable: true, LibraryID: 1, LibraryName: "Filme"},
		Root{ID: "l2", Dir: filepath.Join(dir, "Serien", "SD"), Browsable: true, LibraryID: 2, LibraryName: "Serien"})

	got, err := l.List(ctx, roots, Request{RootID: "c0"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Filme": "Filme", "Serien": "Serien", "Neu": ""}
	for _, e := range got.Entries {
		if e.LibraryName != want[e.Name] {
			t.Errorf("%s belongs to %q, want %q", e.Name, e.LibraryName, want[e.Name])
		}
	}
}

// E9: a file system that never answers costs one listing its slot for good. Within the watchdog
// the caller gets 503 path_unreachable, and once every slot is gone the browser answers
// browse_busy without starting another goroutine, until the server restarts.
func TestListWatchdogAndBusy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	roots := []Root{{ID: "c0", Dir: "/media", Browsable: true}}
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) }) // let the goroutines end with the test
	f := &fakeFS{block: blocked, fsType: paths.FSNFS}
	l := New(discard, WithFS(f), WithSlots(3), WithWatchdog(20*time.Millisecond))

	for i := range 3 {
		start := time.Now()
		if _, err := l.List(ctx, roots, Request{RootID: "c0"}); !errors.Is(err, model.ErrPathUnreachable) {
			t.Fatalf("call %d = %v, want ErrPathUnreachable", i, err)
		}
		if waited := time.Since(start); waited > time.Second {
			t.Fatalf("call %d waited %s, want the watchdog to end it", i, waited)
		}
	}
	if l.LostSlots() != 3 {
		t.Errorf("lost slots = %d, want one per hanging call", l.LostSlots())
	}
	for i := range 2 {
		if _, err := l.List(ctx, roots, Request{RootID: "c0"}); !errors.Is(err, model.ErrBrowseBusy) {
			t.Errorf("call %d after every slot was lost = %v, want ErrBrowseBusy", i, err)
		}
	}
	if reads := f.reads.Load(); reads != 3 {
		t.Errorf("%d reads were started, want one per slot and none after that", reads)
	}
	if l.LostSlots() != 3 {
		t.Errorf("a busy answer must not count as another lost slot: %d", l.LostSlots())
	}
}

// A slot comes back as soon as the call returns, so a slow but living share does not switch the
// browser off.
func TestListFreesSlotAfterSlowCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	roots := []Root{{ID: "c0", Dir: "/media", Browsable: true}}
	released := make(chan struct{})
	f := &fakeFS{block: released, entries: dirEntries(2), fsType: paths.FSExt4}
	l := New(discard, WithFS(f), WithSlots(1), WithWatchdog(20*time.Millisecond))

	if _, err := l.List(ctx, roots, Request{RootID: "c0"}); !errors.Is(err, model.ErrPathUnreachable) {
		t.Fatalf("the slow call = %v, want ErrPathUnreachable", err)
	}
	close(released)
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := l.List(ctx, roots, Request{RootID: "c0"})
		if err == nil {
			if len(got.Entries) != 2 {
				t.Errorf("after the slot came back: %+v", got)
			}
			return
		}
		if !errors.Is(err, model.ErrBrowseBusy) {
			t.Fatalf("after the slot came back = %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("the slot of a call that returned never came back")
		}
		time.Sleep(time.Millisecond)
	}
}

// An unknown root is refused before anything touches the file system (E6).
func TestListTouchesNothingForAnUnknownRoot(t *testing.T) {
	t.Parallel()
	f := &fakeFS{entries: dirEntries(1), fsType: paths.FSExt4}
	l := New(discard, WithFS(f))
	roots := []Root{{ID: "c0", Dir: "/media", Browsable: true}}
	for _, id := range []string{"c1", "/media", "../config"} {
		if _, err := l.List(context.Background(), roots, Request{RootID: id}); !errors.Is(err, model.ErrPathNotBrowsable) {
			t.Errorf("rootId %q = %v, want ErrPathNotBrowsable", id, err)
		}
	}
	if f.reads.Load() != 0 || f.stats.Load() != 0 {
		t.Errorf("an unknown root cost %d reads and %d statfs, want none", f.reads.Load(), f.stats.Load())
	}
}

func names(l model.DirectoryListing) []string {
	out := make([]string, 0, len(l.Entries))
	for _, e := range l.Entries {
		out = append(out, e.Name)
	}
	return out
}

// fakeFS counts its calls and can block for ever, like a dead network mount.
type fakeFS struct {
	entries      []fs.DirEntry
	err          error
	fsType       paths.FSType
	block        chan struct{} // nil: answer at once; else wait for it to close
	reads, stats atomic.Int64
}

func (f *fakeFS) ReadDir(_ paths.Root, _ string, max int) ([]fs.DirEntry, error) {
	f.reads.Add(1)
	if f.block != nil {
		<-f.block
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.entries[:min(len(f.entries), max)], nil
}

func (f *fakeFS) FSTypeAt(paths.Root, string) (paths.FSType, error) {
	f.stats.Add(1)
	return f.fsType, nil
}

// dirEntries returns n directories in an order getdents could return them: not sorted.
func dirEntries(n int) []fs.DirEntry {
	out := make([]fs.DirEntry, 0, n)
	for i := range n {
		out = append(out, fakeEntry{name: fmt.Sprintf("d%06d", (i*7919)%n)})
	}
	return out
}

type fakeEntry struct {
	name string
	mode fs.FileMode
}

func (e fakeEntry) Name() string               { return e.name }
func (e fakeEntry) IsDir() bool                { return e.mode&fs.ModeSymlink == 0 }
func (e fakeEntry) Type() fs.FileMode          { return e.mode | fs.ModeDir }
func (e fakeEntry) Info() (fs.FileInfo, error) { return nil, errors.New("not needed") }
