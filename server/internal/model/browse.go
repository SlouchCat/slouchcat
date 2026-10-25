package model

import "errors"

// The folder browser of the library form (idea 02). The administrator picks a folder below a
// browsable root instead of typing a path that only exists inside the container.

// Errors of the folder browser.
var (
	// ErrPathNotBrowsable: no such root, or rel names nothing below it (404 path_not_browsable).
	// Both answer alike, so the endpoint tells nothing about the file system outside the roots
	// (CLAUDE.md rule 2: not visible means 404, never 403).
	ErrPathNotBrowsable = errors.New("path not browsable")
	// ErrPathUnreachable: the file system did not answer within the watchdog's time
	// (503 path_unreachable). A dead network mount blocks in the kernel and cannot be cancelled.
	ErrPathUnreachable = errors.New("path unreachable")
	// ErrBrowseBusy: as many listings are already running as the server allows
	// (503 browse_busy). Once every slot is held by a call that never came back, the browser
	// answers this until the server restarts; the dashboard says so.
	ErrBrowseBusy = errors.New("browse busy")
)

// Sources of a BrowseRoot.
const (
	BrowseRootConfigured = "configured" // from SLOUCHCAT_BROWSE_ROOTS
	BrowseRootLibrary    = "library"    // a root of an existing library
)

// BrowseRoot is a directory the folder picker may start from: a configured root or the root of a
// library that exists. The wizard gets the configured ones without FSType and Warning, before any
// account exists.
type BrowseRoot struct {
	ID   string // opaque, stable while the server process runs
	Path string
	// Browsable is false for a library root that lies below no configured root: the dialog shows
	// it so a directory can carry its library's name, but it cannot be entered (E14).
	Browsable bool
	Source    string // BrowseRootConfigured or BrowseRootLibrary
	FSType    string
	Warning   string // network_share, fuse_unknown or empty
	// LibraryID and LibraryName name the library of a root with Source BrowseRootLibrary.
	LibraryID   int64
	LibraryName string
}

// BrowseRequest names the directory the folder picker wants to see: a root by its id and a path
// relative to it, never an absolute path (E1).
type BrowseRequest struct {
	RootID string
	Rel    string // empty lists the root itself
	Cursor string
	Limit  int // 0 = the server's default page size
}

// BrowseHealth is the state of the folder browser on the dashboard. Above zero LostSlots a
// listing hangs in the kernel on a mount that stopped answering; at LostSlots == Slots the
// browser answers ErrBrowseBusy until the server restarts. It is a warning, not an error: the
// rest of the server is unaffected.
type BrowseHealth struct {
	LostSlots int
	Slots     int
}

// DirectoryListing is one directory below a browsable root with its subdirectories.
type DirectoryListing struct {
	RootID string
	Rel    string // relative to the root; empty at the root itself
	Path   string // absolute, ready for the folder field
	// ParentRel is one level up, relative to the root; meaningful only when AtRoot is false,
	// where the empty string means the root itself.
	ParentRel string
	AtRoot    bool
	FSType    string
	Watch     string // inotify, poll, inotify_poll
	Warning   string // network_share, fuse_unknown or empty
	// Problem: the directory lies below a root but could not be listed (not_readable,
	// not_directory); Entries is then empty. A path below no root answers ErrPathNotBrowsable.
	Problem string
	// Truncated: the directory holds more entries than the server reads.
	Truncated  bool
	NextCursor string
	Entries    []DirectoryEntry
}

// DirectoryEntry is one subdirectory as the folder picker shows it.
type DirectoryEntry struct {
	Name string
	Rel  string // relative to the root, for the next request
	Path string // absolute, ready for the folder field
	// Symlink: a symlinked directory. It is listed, because the scanner sees it too, but it
	// cannot be entered, because that would leave the root (E15).
	Symlink bool
	// LibraryID and LibraryName name the library this directory already belongs to; a new
	// library may not overlap it.
	LibraryID   int64
	LibraryName string
}
