package paths

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// Open opens the root directory for file access that cannot escape it: every Open, Stat or
// Create on the returned *os.Root rejects "..", absolute paths and symlinks that leave the root
// (plan 3.8 point 6). Callers close it when done. Pass relative paths that went through Resolve
// or came from a Walk of the same root.
func Open(root Root) (*os.Root, error) {
	if root.dir == "" {
		return nil, fmt.Errorf("open root: %w", errUnregistered)
	}
	r, err := os.OpenRoot(root.dir)
	if err != nil {
		return nil, fmt.Errorf("open root %s: %w", root.dir, err)
	}
	return r, nil
}

// Walk walks the root with fs.WalkDir without following symlinked directories; fn receives
// slash-separated paths relative to the root ("." for the root itself), which are valid inputs
// for Resolve and for the methods of an *os.Root of the same root.
func Walk(root Root, fn fs.WalkDirFunc) error {
	r, err := Open(root)
	if err != nil {
		return err
	}
	defer r.Close()
	return fs.WalkDir(r.FS(), ".", fn)
}

// readDirBatch is how many names one getdents round trip asks for; large directories are read in
// batches so a capped read does not pull the whole directory into memory first.
const readDirBatch = 1024

// ReadDir reads one level of the subtree rel of root (slash-separated, "." for the root itself)
// through the root's *os.Root, so a symlink that leaves the root is not followed and rel cannot
// escape the root. max caps the number of entries read (0 or less reads all of them); a directory
// with more entries than max is read up to max, which the caller sees from the result's length.
// The entries come in directory order, not sorted (getdents).
func ReadDir(root Root, rel string, max int) ([]fs.DirEntry, error) {
	dir, err := openDir(root, rel)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if max <= 0 {
		return dir.ReadDir(-1)
	}
	out := make([]fs.DirEntry, 0, min(max, readDirBatch))
	for len(out) < max {
		batch, err := dir.ReadDir(min(readDirBatch, max-len(out)))
		out = append(out, batch...)
		switch {
		case errors.Is(err, io.EOF), err == nil && len(batch) == 0:
			return out, nil
		case err != nil:
			// Not the end of the directory but a real failure, a read of something that is no
			// directory above all: the caller must see it, not an empty listing.
			return out, fmt.Errorf("read dir %s: %w", rel, err)
		}
	}
	return out, nil
}

// FSTypeAt classifies the file system of rel through the descriptor of the directory itself
// (fstatfs), so no path is assembled from request data and no symlink is followed. DetectFS stays
// for directories that are not below a root, such as the configured roots at start-up.
func FSTypeAt(root Root, rel string) (FSType, error) {
	dir, err := openDir(root, rel)
	if err != nil {
		return FSUnknown, err
	}
	defer dir.Close()
	return fstatfs(dir)
}

// openDir opens rel below root after the lexical check of Resolve. The *os.Root is closed right
// away: the returned file keeps its own descriptor, and it was opened through the root, so it is
// the directory the root contains and not one a symlink pointed at.
func openDir(root Root, rel string) (*os.File, error) {
	if _, err := Resolve(root, rel); err != nil {
		return nil, err
	}
	r, err := Open(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	dir, err := r.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, err)
	}
	return dir, nil
}
