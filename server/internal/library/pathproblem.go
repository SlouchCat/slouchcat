package library

import (
	"errors"
	"io/fs"
	"syscall"
)

// Why a folder is not usable as a library folder. The folder browser of idea 02 uses them for a
// directory it may list but cannot read, so it never names one state differently from the scan.
const (
	PathMissing      = "missing"
	PathNotDirectory = "not_directory"
	PathNotReadable  = "not_readable"
)

// ProblemFromError names the same states from the error of a read that already happened, without
// touching the path again: the folder browser lists a directory through an *os.Root and must not
// stat a path it assembled from a request (CLAUDE.md rule 4). It returns "" for an error it
// cannot place -- an *os.Root refusing a symlink that leaves the root, for one -- and the caller
// decides what that means; the browser answers 404 for it, so such a path reveals nothing.
func ProblemFromError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, fs.ErrNotExist):
		return PathMissing
	case errors.Is(err, syscall.ENOTDIR):
		return PathNotDirectory
	case errors.Is(err, fs.ErrPermission):
		return PathNotReadable
	default:
		return ""
	}
}
