package library

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// Why a folder is not usable as a library folder. The wizard shows these codes next to its path
// field (PathCheck.problem), and the folder browser of idea 02 uses the same two inner ones for a
// directory it may list but cannot read, so the two places never name one state differently.
const (
	PathNotAbsolute  = "not_absolute"
	PathMissing      = "missing"
	PathNotDirectory = "not_directory"
	PathNotReadable  = "not_readable"
	// PathNotBrowsable: the folder lies below none of the browsable roots (E14). The wizard says
	// so instead of describing the folder, which would tell a caller without an account whether
	// a path exists.
	PathNotBrowsable = "not_browsable"
)

// PathProblem names why a path that failed ValidateDir is not usable; the errors of the check do
// not tell a file from a directory (os.OpenRoot), a stat of the path does. Only for a path the
// caller is allowed to learn about: never for one below no browsable root.
func PathProblem(path string) string {
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return PathMissing
	case errors.Is(err, syscall.ENOTDIR), err == nil && !fi.IsDir():
		return PathNotDirectory
	default:
		return PathNotReadable
	}
}

// ProblemFromError names the same states from the error of a read that already happened, without
// touching the path again: the folder browser lists a directory through an *os.Root and must not
// stat a path it assembled from a request (CLAUDE.md rule 4).
func ProblemFromError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, fs.ErrNotExist):
		return PathMissing
	case errors.Is(err, syscall.ENOTDIR):
		return PathNotDirectory
	default:
		return PathNotReadable
	}
}
