package paths

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot is returned for relative paths that could leave their root.
var ErrOutsideRoot = errors.New("path is not inside its root")

// Root is a registered directory that relative paths are resolved against.
type Root struct {
	dir string
}

// NewRoot registers dir, which must be an absolute path.
func NewRoot(dir string) (Root, error) {
	if !filepath.IsAbs(dir) {
		return Root{}, fmt.Errorf("root %q must be absolute", dir)
	}
	return Root{dir: filepath.Clean(dir)}, nil
}

// Dir returns the absolute directory of the root.
func (r Root) Dir() string { return r.dir }

// Resolve joins rel to root after rejecting every form that could leave the root.
func Resolve(root Root, rel string) (string, error) {
	if root.dir == "" {
		return "", errors.New("root is not registered")
	}
	if rel == "" || strings.ContainsRune(rel, 0) || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("%w: %q", ErrOutsideRoot, rel)
	}
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == filepath.Separator }) {
		if seg == ".." {
			return "", fmt.Errorf("%w: %q", ErrOutsideRoot, rel)
		}
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%w: %q", ErrOutsideRoot, rel)
	}
	return filepath.Join(root.dir, rel), nil
}
