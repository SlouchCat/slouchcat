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
	// DefaultSlots is how many listings may run at once, server-wide. See the package comment on
	// why a lost slot never comes back and why the number is 16.
	DefaultSlots = 16
	// DefaultWatchdog is how long a listing waits for the file system before it gives up on it.
	DefaultWatchdog = 3 * time.Second
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
// injects one that blocks or counts its calls.
type FS interface {
	ReadDir(root paths.Root, rel string, max int) ([]fs.DirEntry, error)
	FSTypeAt(root paths.Root, rel string) (paths.FSType, error)
}

// realFS reads through package paths, which is the only way user-controlled path data turns into
// a file system path (CLAUDE.md rule 4).
type realFS struct{}

func (realFS) ReadDir(root paths.Root, rel string, max int) ([]fs.DirEntry, error) {
	return paths.ReadDir(root, rel, max)
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
	lost     atomic.Int64
}

// Option configures New.
type Option func(*Lister)

// WithFS replaces the file system, for tests.
func WithFS(f FS) Option { return func(l *Lister) { l.fs = f } }

// WithWatchdog replaces how long a listing waits for the file system, for tests.
func WithWatchdog(d time.Duration) Option { return func(l *Lister) { l.watchdog = d } }

// WithSlots replaces how many listings may run at once, for tests.
func WithSlots(n int) Option { return func(l *Lister) { l.slots = make(chan struct{}, n) } }

// New returns a Lister.
func New(log *slog.Logger, opts ...Option) *Lister {
	l := &Lister{fs: realFS{}, log: log, slots: make(chan struct{}, DefaultSlots), watchdog: DefaultWatchdog}
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
	read, err := l.read(ctx, dir, rel)
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
	out.Truncated = len(read.entries) > MaxEntries

	names := subdirectories(read.entries)
	slices.SortFunc(names, func(a, b dirName) int { return strings.Compare(a.name, b.name) })
	page, next := paginate(names, cursor, limit)
	out.NextCursor = encodeCursor(root.ID, rel, next)
	for _, n := range page {
		out.Entries = append(out.Entries, l.entry(root, roots, rel, n))
	}
	return out, nil
}

// entry turns one subdirectory into a row, with the library it already belongs to.
func (l *Lister) entry(root Root, roots []Root, rel string, n dirName) model.DirectoryEntry {
	childRel := n.name
	if rel != "." {
		childRel = path.Join(rel, n.name)
	}
	e := model.DirectoryEntry{Name: n.name, Rel: childRel, Path: absOf(root.Dir, childRel), Symlink: n.symlink}
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
	return e
}

// readResult is what one trip to the file system brought back.
type readResult struct {
	entries []fs.DirEntry
	fsType  paths.FSType
	err     error
}

// read does every file system call of one listing in a goroutine of its own and waits for it at
// most l.watchdog. A call that blocks in the kernel keeps its slot for good; see the package
// comment.
func (l *Lister) read(ctx context.Context, dir paths.Root, rel string) (readResult, error) {
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
		var out readResult
		// One read of at most MaxEntries+1 names, so the caller can tell a full directory from
		// one that holds more, and one fstatfs on the directory itself. Nothing per row.
		out.entries, out.err = l.fs.ReadDir(dir, rel, MaxEntries+1)
		if out.err == nil {
			out.fsType, out.err = l.fs.FSTypeAt(dir, rel)
		}
		done <- out
	}()

	timer := time.NewTimer(l.watchdog)
	defer timer.Stop()
	select {
	case out := <-done:
		return out, nil
	case <-ctx.Done():
		l.lostSlot(ctx, dir, "the request was cancelled while the file system was reading")
		return readResult{}, ctx.Err()
	case <-timer.C:
		l.lostSlot(ctx, dir, "the file system did not answer")
		return readResult{}, model.ErrPathUnreachable
	}
}

// lostSlot records a slot that the goroutine still holds. Error level, once per loss: this is
// how a dead mount becomes visible, and the count is bounded by the number of slots.
func (l *Lister) lostSlot(ctx context.Context, dir paths.Root, why string) {
	lost := l.lost.Add(1)
	l.log.ErrorContext(ctx, "browse: "+why+"; its slot stays taken until the call returns",
		"root", dir.Dir(), "lost_slots", lost, "slots", cap(l.slots), "watchdog", l.watchdog)
}

// dirName is one subdirectory of the listed directory.
type dirName struct {
	name    string
	symlink bool
}

// subdirectories keeps the directories and the symlinks of a raw listing and drops everything
// else: files, and the entries library.Ignored names (hidden, NAS metadata folders, disc
// structures), so the list shows what a scan would see.
//
// A symlink is kept without looking at its target. Asking what it points at would mean a stat
// per row, and for a link out of the root one that reads outside it; the scanner does not follow
// them either, so the row says so and cannot be entered (E15).
func subdirectories(entries []fs.DirEntry) []dirName {
	out := make([]dirName, 0, min(len(entries), DefaultLimit))
	for _, e := range entries {
		if library.Ignored(e.Name()) {
			continue
		}
		switch {
		case e.Type()&fs.ModeSymlink != 0:
			out = append(out, dirName{name: e.Name(), symlink: true})
		case e.IsDir():
			out = append(out, dirName{name: e.Name()})
		}
	}
	return out
}

// paginate returns the page that follows cursor and the name the next cursor must follow, empty
// on the last page.
func paginate(names []dirName, cursor string, limit int) (page []dirName, next string) {
	if cursor != "" {
		names = names[sortedIndexAfter(names, cursor):]
	}
	if len(names) <= limit {
		return names, ""
	}
	return names[:limit], names[limit-1].name
}

// sortedIndexAfter returns the index of the first name after cursor in a sorted slice.
func sortedIndexAfter(names []dirName, cursor string) int {
	i, _ := slices.BinarySearchFunc(names, cursor, func(a dirName, b string) int {
		return strings.Compare(a.name, b)
	})
	// BinarySearchFunc returns the position of the cursor itself when it is still there; the page
	// starts after it.
	if i < len(names) && names[i].name == cursor {
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
	if len(cursor) > MaxRel {
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
