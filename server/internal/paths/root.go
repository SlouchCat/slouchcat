package paths

import (
	"fmt"
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
