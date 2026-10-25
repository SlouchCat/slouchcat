package browse

import (
	"context"
	"encoding/base64"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// Limits of one listing.
const (
	// DefaultLimit and MaxLimit bound how many rows one page carries.
	DefaultLimit = 200
	MaxLimit     = 500
	// MaxEntries is how many names the server reads from one directory before it gives up and
	// says so (Truncated). Fifty thousand names are about 5 MB; the path can still be typed.
	MaxEntries = 50_000
	// MaxRel is the longest relative path a request may carry, as in the contract.
	MaxRel = 4096
	// MaxCursor is the longest cursor a request may carry. A cursor carries the root id, the
	// directory and the last name of the page, base64 encoded, so it is longer than the path it
	// belongs to: a limit of MaxRel would make a deep directory unpageable.
	MaxCursor = 3 * MaxRel
	// DefaultSlots is how many listings may run at once, server-wide. See the package comment on
	// why a lost slot never comes back and why the number is 16.
	DefaultSlots = 16
	// DefaultWatchdog is how long a listing waits for the file system before it gives up on it.
	DefaultWatchdog = 3 * time.Second
	// DefaultGrace is how much longer a given-up call may still come back before its slot counts
	// as lost. A cancelled request and a slow but living share come back well inside it; a call
	// blocked in the kernel never does.
	DefaultGrace = time.Minute
)

// Root is a directory the picker may browse, with the library it already belongs to. The caller
// builds the list; this package asks nothing.
type Root struct {
	ID string
	// Dir is the absolute directory of the root.
	Dir string
	// Browsable is false for a library root below no configured root: it is listed so a directory
	// can carry its library's name, but it cannot be entered (E14).
	Browsable bool
	// LibraryID is 0 for a configured root that is no library root.
	LibraryID   int64
	LibraryName string
}

// Request names the directory to list.
type Request struct {
	RootID string
	// Rel is the directory below the root, slash-separated; empty lists the root itself.
	Rel string
	// Cursor is the NextCursor of the previous page; empty returns the first page.
	Cursor string
	// Limit is the page size; 0 means DefaultLimit.
	Limit int
}

// FS is the file system a Lister reads. The default one goes through package paths; a test
// injects one that blocks or counts its calls. The split between names and lstat is what bounds
// the cost of a page: the names of a whole directory are cheap, an lstat per entry is not.
type FS interface {
	ReadDirNames(root paths.Root, rel string, max int) ([]string, error)
	LstatAll(root paths.Root, rel string, names []string) ([]fs.DirEntry, error)
	FSTypeAt(root paths.Root, rel string) (paths.FSType, error)
}

// realFS reads through package paths, which is the only way user-controlled path data turns into
// a file system path (CLAUDE.md rule 4).
type realFS struct{}

func (realFS) ReadDirNames(root paths.Root, rel string, max int) ([]string, error) {
	return paths.ReadDirNames(root, rel, max)
}

func (realFS) LstatAll(root paths.Root, rel string, names []string) ([]fs.DirEntry, error) {
	return paths.LstatAll(root, rel, names)
}

func (realFS) FSTypeAt(root paths.Root, rel string) (paths.FSType, error) {
	return paths.FSTypeAt(root, rel)
}

// Lister lists directories below browsable roots. Its semaphore and watchdog are server-wide, so
// one is made once and shared.
type Lister struct {
	fs       FS
	log      *slog.Logger
	slots    chan struct{}
	watchdog time.Duration
	grace    time.Duration
	lost     atomic.Int64
}

// Option configures New.
type Option func(*Lister)

// WithFS replaces the file system, for tests.
func WithFS(f FS) Option { return func(l *Lister) { l.fs = f } }

// WithWatchdog replaces how long a listing waits for the file system, for tests.
func WithWatchdog(d time.Duration) Option { return func(l *Lister) { l.watchdog = d } }

// WithGrace replaces how long a given-up call may still come back before its slot counts as
// lost, for tests.
func WithGrace(d time.Duration) Option { return func(l *Lister) { l.grace = d } }

// WithSlots replaces how many listings may run at once, for tests.
func WithSlots(n int) Option { return func(l *Lister) { l.slots = make(chan struct{}, n) } }

// New returns a Lister.
func New(log *slog.Logger, opts ...Option) *Lister {
	l := &Lister{fs: realFS{}, log: log, slots: make(chan struct{}, DefaultSlots),
		watchdog: DefaultWatchdog, grace: DefaultGrace}
	for _, o := range opts {
		o(l)
	}
	return l
}

// LostSlots returns how many slots are held by a call that never came back. Above zero the
// dashboard warns; at the number of slots the browser answers model.ErrBrowseBusy until the
// server restarts.
func (l *Lister) LostSlots() int { return int(l.lost.Load()) }

// Slots returns how many listings may run at once.
func (l *Lister) Slots() int { return cap(l.slots) }

// List returns one page of the subdirectories of req.Rel below the root req.RootID.
//
// Errors: *model.ValidationError (the request is malformed), model.ErrPathNotBrowsable (no such
// root, rel names nothing below it, or the read failed in a way that must not be described),
// model.ErrPathUnreachable and model.ErrBrowseBusy. A directory that lies below a root but
// cannot be listed is no error: the listing carries Problem and no entries.
func (l *Lister) List(ctx context.Context, roots []Root, req Request) (model.DirectoryListing, error) {
	rel, limit, cursor, verr := parseRequest(req)
	if verr != nil {
		return model.DirectoryListing{}, verr
	}
	root, ok := findRoot(roots, req.RootID)
	if !ok {
		return model.DirectoryListing{}, model.ErrPathNotBrowsable
	}
	dir, err := paths.NewRoot(root.Dir)
	if err != nil {
		// A configured root that is not absolute never reaches here; treat it as unbrowsable
		// rather than as a path the caller may learn about.
		return model.DirectoryListing{}, model.ErrPathNotBrowsable
	}
	read, err := l.guard(ctx, root.ID, func() readResult { return l.readPage(dir, rel, cursor, limit) })
	if err != nil {
		return model.DirectoryListing{}, err
	}

	out := model.DirectoryListing{RootID: root.ID, AtRoot: rel == ".", Entries: []model.DirectoryEntry{}}
	if !out.AtRoot {
		out.Rel = rel
		out.ParentRel = parentOf(rel)
	}
	out.Path = absOf(root.Dir, rel)
	if read.err != nil {
		problem := library.ProblemFromError(read.err)
		if problem != library.PathNotDirectory && problem != library.PathNotReadable {
			// Missing, or an error that cannot be placed -- an *os.Root refusing a symlink that
			// leaves the root, for one. Both answer like an unknown root (E6, E15).
			return model.DirectoryListing{}, model.ErrPathNotBrowsable
		}
		out.Problem = problem
		return out, nil
	}
	out.FSType = string(read.fsType)
	out.Watch, out.Warning = library.WatchMode(string(read.fsType))
	out.Truncated = read.truncated
	out.NextCursor = encodeCursor(root.ID, rel, read.next)
	for _, e := range read.entries {
		if row, ok := l.entry(root, roots, rel, e); ok {
			out.Entries = append(out.Entries, row)
		}
	}
	return out, nil
}

// readPage does every file system call of one listing: the names of the directory, the lstat of
// the names this page shows, and one fstatfs on the directory itself. Nothing per name that the
// page does not show -- that is the whole reason the names are read without their type.
//
// The page is cut from the sorted names, not from the directories among them, so a page of a
// folder full of files can hold fewer rows than `limit` asked for. The alternative is to stat
// until the page is full, which on a folder with 50 000 files means 50 000 stats for one page.
func (l *Lister) readPage(dir paths.Root, rel, cursor string, limit int) readResult {
	var out readResult
	names, err := l.fs.ReadDirNames(dir, rel, MaxEntries+1)
	if err != nil {
		out.err = err
		return out
	}
	out.truncated = len(names) > MaxEntries
	if out.truncated {
		names = names[:MaxEntries]
	}
	kept := make([]string, 0, min(len(names), DefaultLimit))
	for _, name := range names {
		// The same filter the scanner uses, so the dialog shows what a scan would see.
		if !library.Ignored(name) {
			kept = append(kept, name)
		}
	}
	slices.Sort(kept)
	page, next := paginate(kept, cursor, limit)
	out.next = next
	if out.entries, out.err = l.fs.LstatAll(dir, rel, page); out.err != nil {
		return out
	}
	out.fsType, out.err = l.fs.FSTypeAt(dir, rel)
	return out
}

// entry turns one directory entry into a row, with the library it already belongs to. Files are
// dropped: the picker shows folders only, and a listing with 50 000 files would be the normal
// case otherwise. A symlink is kept without looking at its target -- that would be a stat on a
// path outside the root, and the scanner does not follow it either (E15).
func (l *Lister) entry(root Root, roots []Root, rel string, entry fs.DirEntry) (model.DirectoryEntry, bool) {
	symlink := entry.Type()&fs.ModeSymlink != 0
	if !symlink && !entry.IsDir() {
		return model.DirectoryEntry{}, false
	}
	childRel := entry.Name()
	if rel != "." {
		childRel = path.Join(rel, entry.Name())
	}
	e := model.DirectoryEntry{Name: entry.Name(), Rel: childRel, Path: absOf(root.Dir, childRel), Symlink: symlink}
	for _, r := range roots {
		if r.LibraryID == 0 {
			continue
		}
		// Either direction counts: a new library may not overlap an existing root, whether it
		// lies below one or holds one (library.Service.Create refuses both).
		if library.UnderAnyRoot(e.Path, []string{r.Dir}) || library.UnderAnyRoot(r.Dir, []string{e.Path}) {
			e.LibraryID, e.LibraryName = r.LibraryID, r.LibraryName
			break
		}
	}
	return e, true
}

// readResult is what one trip to the file system brought back.
type readResult struct {
	entries   []fs.DirEntry
	fsType    paths.FSType
	next      string // the name the next cursor must follow; empty on the last page
	truncated bool
	err       error
}

// guard runs the file system work of one listing in a goroutine of its own and waits for it at
// most l.watchdog. A call that blocks in the kernel keeps its slot for good; see the package
// comment.
func (l *Lister) guard(ctx context.Context, rootID string, work func() readResult) (readResult, error) {
	select {
	case l.slots <- struct{}{}:
	default:
		// Debug, not Warn: the error ring of the dashboard holds 50 entries, and a dialog
		// clicked once a second would flush the real errors out of it.
		l.log.DebugContext(ctx, "browse: all listing slots are in use", "lost_slots", l.LostSlots(), "slots", cap(l.slots))
		return readResult{}, model.ErrBrowseBusy
	}
	done := make(chan readResult, 1)
	go func() {
		defer func() { <-l.slots }()
		done <- work()
	}()

	timer := time.NewTimer(l.watchdog)
	defer timer.Stop()
	select {
	case out := <-done:
		return out, nil
	case <-ctx.Done():
		go l.watchSlot(done, rootID, "the request was cancelled while the file system was reading")
		return readResult{}, ctx.Err()
	case <-timer.C:
		go l.watchSlot(done, rootID, "the file system did not answer")
		return readResult{}, model.ErrPathUnreachable
	}
}

// watchSlot waits a while longer for the call the caller gave up on. Comes it back, the slot was
// never lost and nothing is counted or logged -- a closed dialog cancels its request, and a slow
// but living share answers late; neither is a fault. Only a call that is still gone after the
// grace period counts as a lost slot, once, on Error: that is how a dead mount becomes visible,
// and the count is bounded by the number of slots.
func (l *Lister) watchSlot(done <-chan readResult, rootID, why string) {
	grace := time.NewTimer(l.grace)
	defer grace.Stop()
	select {
	case <-done:
	case <-grace.C:
		lost := l.lost.Add(1)
		// The root is named by its opaque id, not by its path: this line lands in the error ring
		// of the dashboard, and a root path carries folder names (CLAUDE.md rule 10).
		l.log.Error("browse: "+why+"; its slot stays taken until the call returns",
			"root_id", rootID, "lost_slots", lost, "slots", cap(l.slots), "watchdog", l.watchdog)
	}
}

// paginate returns the page that follows cursor and the name the next cursor must follow, empty
// on the last page.
func paginate(names []string, cursor string, limit int) (page []string, next string) {
	if cursor != "" {
		names = names[sortedIndexAfter(names, cursor):]
	}
	if len(names) <= limit {
		return names, ""
	}
	return names[:limit], names[limit-1]
}

// sortedIndexAfter returns the index of the first name after cursor in a sorted slice.
func sortedIndexAfter(names []string, cursor string) int {
	i, _ := slices.BinarySearch(names, cursor)
	// BinarySearch returns the position of the cursor itself when it is still there; the page
	// starts after it.
	if i < len(names) && names[i] == cursor {
		return i + 1
	}
	return i
}

// findRoot returns the root with this id, if it may be entered. A root that exists but is not
// browsable answers like one that does not exist (E14).
func findRoot(roots []Root, id string) (Root, bool) {
	for _, r := range roots {
		if r.ID == id && r.Browsable {
			return r, true
		}
	}
	return Root{}, false
}

// parentOf returns one level up of a cleaned, relative, slash-separated path; "" is the root.
func parentOf(rel string) string {
	parent := path.Dir(rel)
	if parent == "." {
		return ""
	}
	return parent
}

// absOf is the absolute path of rel below dir, for the folder field. It is shown, never used to
// reach the file system: every read goes through the root's *os.Root.
func absOf(dir, rel string) string {
	if rel == "." {
		return filepath.Clean(dir)
	}
	return filepath.Join(dir, filepath.FromSlash(rel))
}

// parseRequest checks the request and returns the directory to list ("." for the root itself),
// the page size and the name the page starts after.
func parseRequest(req Request) (rel string, limit int, cursor string, verr *model.ValidationError) {
	if req.RootID == "" {
		verr = verr.Add("rootId", "required")
	}
	rel, err := normalizeRel(req.Rel)
	if err != "" {
		verr = verr.Add("rel", err)
	}
	limit = req.Limit
	switch {
	case limit == 0:
		limit = DefaultLimit
	case limit < 1 || limit > MaxLimit:
		verr = verr.Add("limit", "invalid_value")
	}
	cursor, err = decodeCursor(req.Cursor, req.RootID, rel)
	if err != "" {
		verr = verr.Add("cursor", err)
	}
	if verr != nil {
		return "", 0, "", verr
	}
	return rel, limit, cursor, nil
}

// normalizeRel checks a relative path from a request and turns the empty one into ".", which is
// what paths.Resolve takes for the root itself (it refuses the empty string). Every form that
// could leave the root is refused before it is cleaned, so "a/../b" does not slip through as "b".
func normalizeRel(rel string) (string, string) {
	if rel == "" {
		return ".", ""
	}
	switch {
	case len(rel) > MaxRel:
		return "", "too_long"
	case strings.ContainsRune(rel, 0), strings.HasPrefix(rel, "/"), filepath.IsAbs(rel):
		return "", "invalid_value"
	}
	for seg := range strings.SplitSeq(rel, "/") {
		if seg == ".." {
			return "", "invalid_value"
		}
	}
	clean := path.Clean(rel)
	if !filepath.IsLocal(filepath.FromSlash(clean)) && clean != "." {
		return "", "invalid_value"
	}
	return clean, ""
}

// encodeCursor names the page that follows after, bound to the directory it came from; empty
// after is the last page.
func encodeCursor(rootID, rel, after string) string {
	if after == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(rootID + "\x00" + rel + "\x00" + after))
}

// decodeCursor returns the name the page starts after. A cursor from another directory is
// refused, so a page cannot be carried from one listing into another.
func decodeCursor(cursor, rootID, rel string) (string, string) {
	if cursor == "" {
		return "", ""
	}
	if len(cursor) > MaxCursor {
		return "", "too_long"
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "invalid_value"
	}
	parts := strings.SplitN(string(raw), "\x00", 3)
	if len(parts) != 3 || parts[0] != rootID || parts[1] != rel || parts[2] == "" {
		return "", "invalid_value"
	}
	return parts[2], ""
}
