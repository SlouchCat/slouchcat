package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
)

// Deleting files below a root (ADR 0032, concept 20 section 11). These two functions are the only
// way the server deletes anything below a library root: a forbidigo rule refuses Remove and
// RemoveAll on an *os.Root outside this package. Neither deletes recursively, neither follows a
// symlink that leaves the root, and neither touches the root itself.

// ErrNotRegular is returned by Remove for anything but a regular file (a directory, a symlink, a
// device): only files are deleted, and a symlink is never taken for the file it points at.
var ErrNotRegular = errors.New("not a regular file")

// ErrNotDir is returned by RemoveEmptyDir for anything but a directory.
var ErrNotDir = errors.New("not a directory")

// ErrChanged is returned by Remove when the file at the path is not the one expected: its device
// and inode differ from what the caller recorded (the file was replaced since the scan).
var ErrChanged = errors.New("file changed")

// FileID is the device and inode a caller expects at a path; the zero value expects nothing.
type FileID struct {
	Dev, Inode uint64
}

// Remove deletes the regular file rel below root. rel passes the lexical check of Resolve and is
// then opened through the root's *os.Root, so ".." and symlinks out of the root are refused; the
// entry itself is looked at with lstat, so a symlink is refused as ErrNotRegular instead of being
// followed. A missing file answers an error that wraps fs.ErrNotExist. For the files that belong
// to no recorded identity (companions, recorded images); a media file goes with RemoveExpected.
func Remove(root Root, rel string) error {
	return remove(root, rel, false, FileID{})
}

// RemoveExpected is Remove for a file whose device and inode the caller recorded: the file must
// still have them, else ErrChanged and nothing is deleted. A zero want (nothing recorded, or a
// platform without inodes) is ErrChanged as well: without an identity the file is not deleted.
func RemoveExpected(root Root, rel string, want FileID) error {
	return remove(root, rel, true, want)
}

func remove(root Root, rel string, check bool, want FileID) error {
	if _, err := Resolve(root, rel); err != nil {
		return err
	}
	r, err := Open(root)
	if err != nil {
		return err
	}
	defer r.Close()
	fi, err := r.Lstat(rel)
	if err != nil {
		return fmt.Errorf("remove %s: %w", rel, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("remove %s: %w", rel, ErrNotRegular)
	}
	if check && (want == FileID{} || fileIDOf(fi) != want) {
		return fmt.Errorf("remove %s: %w", rel, ErrChanged)
	}
	if err := r.Remove(rel); err != nil {
		return fmt.Errorf("remove %s: %w", rel, err)
	}
	return nil
}

// RemoveEmptyDir deletes the directory rel below root when it is empty; a directory with any
// entry left stays (the error of rmdir, ENOTEMPTY). The root itself ("." or an empty path) is
// never removed. A symlink is refused as ErrNotDir.
func RemoveEmptyDir(root Root, rel string) error {
	if rel == "." || path.Clean(rel) == "." {
		return fmt.Errorf("remove the root itself: %w", ErrOutsideRoot)
	}
	if _, err := Resolve(root, rel); err != nil {
		return err
	}
	r, err := Open(root)
	if err != nil {
		return err
	}
	defer r.Close()
	fi, err := r.Lstat(rel)
	if err != nil {
		return fmt.Errorf("remove directory %s: %w", rel, err)
	}
	if !fi.IsDir() || fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("remove directory %s: %w", rel, ErrNotDir)
	}
	if err := r.Remove(rel); err != nil {
		return fmt.Errorf("remove directory %s: %w", rel, err)
	}
	return nil
}
