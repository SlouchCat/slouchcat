package model

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// Deleting a library and titles (concept 20 sections 9 to 11, stage 5, ADR 0032).

// ErrPlaying: a profile plays a title of what is about to be deleted; the delete is refused, the
// playback is not stopped (O-13; 409 playing, the problem names the profiles).
var ErrPlaying = errors.New("playing")

// PlayingError names the profiles that play a title of what is about to be deleted.
type PlayingError struct {
	Profiles []string
}

func (e *PlayingError) Error() string { return "playing for " + strings.Join(e.Profiles, ", ") }

func (e *PlayingError) Unwrap() error { return ErrPlaying }

// Actor is who asked for a delete: the administrator's account and profile, for the activity log.
type Actor struct {
	AccountID int64
	ProfileID int64
}

// LibraryDeletion is the outcome of deleteLibrary: the library's name, how many folders and
// titles (every row of items, episodes included) went with it, and the file name of the backup
// written before (empty when the server keeps no backups, as in tests).
type LibraryDeletion struct {
	Name   string
	Roots  int
	Items  int
	Backup string
}

// Deleting a title (concept 20 P-9, sections 10 and 11).

// Modes of a title delete: only out of Slouchcat, or the files as well (ADR 0032).
const (
	DeleteModeLibrary = "library"
	DeleteModeFiles   = "files"
)

// ErrNotDeletable: the title is of a kind that has no delete (an extra; a podcast episode that
// is only a reference from the feed, O-5; 422 not_deletable).
var ErrNotDeletable = errors.New("not deletable")

// ErrFilesNotDeletable: «Auch die Dateien löschen» while a file cannot be deleted (a folder
// mounted read-only, missing rights; 422 files_not_deletable, ADR 0032). Nothing was deleted.
var ErrFilesNotDeletable = errors.New("files not deletable")

// FilesNotDeletableError is ErrFilesNotDeletable with the fixed code of the first file's reason.
type FilesNotDeletableError struct {
	Reason string
}

func (e *FilesNotDeletableError) Error() string { return "files not deletable: " + e.Reason }

func (e *FilesNotDeletableError) Unwrap() error { return ErrFilesNotDeletable }

// Reasons of a file that cannot be deleted or stayed (DeletionFile.Reason, DeletionLeftover.Reason).
const (
	DeleteReasonReadOnly     = "read_only"     // the folder's file system is mounted read-only
	DeleteReasonNoPermission = "no_permission" // the server may not write in the file's folder
	DeleteReasonShared       = "shared"        // the file (a hard link) is also the file of another title
	DeleteReasonNotRegular   = "not_regular"   // a symlink or something else that is no regular file
	DeleteReasonUnreachable  = "unreachable"   // the root or the folder cannot be opened
	DeleteReasonMissing      = "missing"       // already gone; counts as deleted
	DeleteReasonFailed       = "failed"        // the delete itself failed
	DeleteReasonChanged      = "changed"       // another file than the scanned one lies at the path
)

// ItemDeletionPreview is what a delete of a title would touch: its media files and those of every
// title below it with the folder of their library root, how many and how big, the title the
// administrator types when there is more than one file (TypeTitle), the folder of the title when
// its files share one, and the profiles that play one of them right now (O-13).
type ItemDeletionPreview struct {
	ItemID          string
	Kind            string
	ConfirmTitle    string
	TypeTitle       bool
	Files           []DeletionFile
	Folder          *DeletionFolder
	FileCount       int
	TotalBytes      int64
	PlayingProfiles []string
	// FilesDeletable: «Auch die Dateien löschen» is possible (every file deletable, or shared and
	// staying, and one at least deletable); FilesReason is why not (the first file's reason, or
	// no_files).
	FilesDeletable bool
	FilesReason    string
}

// DeletionFile is one media file of a preview.
type DeletionFile struct {
	RootPath  string
	Path      string // relative to RootPath
	SizeBytes int64
	Deletable bool
	Reason    string // DeleteReason*; empty for a deletable file
}

// DeletionFolder is the folder the files of a title share, below its library root.
type DeletionFolder struct {
	RootPath string
	Path     string // relative to RootPath; "" for the root itself
}

// ItemDeletionResult is the outcome of deleteItem: how many titles left (the title and every one
// below it), how many files and bytes went, and which files stayed and why.
type ItemDeletionResult struct {
	RemovedItems int
	DeletedFiles int
	DeletedBytes int64
	Leftover     []DeletionLeftover
	// LeftInFolder counts the entries Slouchcat did not know that stay in the folders of the
	// title (no recursive delete); TitleDeleted says whether the title went for good (O-14).
	LeftInFolder int
	TitleDeleted bool
}

// DeletionLeftover is a file that stayed, with the fixed code of why.
type DeletionLeftover struct {
	RootPath string
	Path     string
	Reason   string
}

// RemovedItem is a title taken out of Slouchcat that can come back (the page «Entfernte Titel»).
type RemovedItem struct {
	ID          string
	Kind        string
	Title       string
	ParentTitle string // the series of a season, the series and season of an episode, the podcast of an episode
	LibraryID   int64
	LibraryName string
	Folder      *DeletionFolder
	RemovedAt   time.Time
}

// RemovedItemPage is one page of removed titles and the number of all of them.
type RemovedItemPage struct {
	Items []RemovedItem
	Total int
}

// ErrDeleting: a playback start of a title that is being deleted right now (409 deleting).
var ErrDeleting = errors.New("deleting")

// DeletingSet holds the titles a delete is working on, between its last check for running
// playbacks and its commit (review finding 5): package library holds them, package playback refuses
// to start one (ErrDeleting). Safe for concurrent use; the zero value is not usable, use
// NewDeletingSet.
type DeletingSet struct {
	mu  sync.Mutex
	ids map[string]int
}

// NewDeletingSet returns an empty set.
func NewDeletingSet() *DeletingSet { return &DeletingSet{ids: map[string]int{}} }

// Hold adds the ids and returns the function that takes them out again. A nil set holds nothing.
func (d *DeletingSet) Hold(ids []string) (release func()) {
	if d == nil {
		return func() {}
	}
	d.mu.Lock()
	for _, id := range ids {
		d.ids[id]++
	}
	d.mu.Unlock()
	return func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		for _, id := range ids {
			if d.ids[id]--; d.ids[id] <= 0 {
				delete(d.ids, id)
			}
		}
	}
}

// Has reports whether a delete holds the id. A nil set holds nothing.
func (d *DeletingSet) Has(id string) bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ids[id] > 0
}
