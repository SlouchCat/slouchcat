package library

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// ErrNotBrowsable is the reason of model.ErrLibraryPathInvalid when a new library folder lies
// below none of the browsable roots (idea 02, E14). It wraps model.ErrLibraryPathInvalid, so the
// API answers 422 library_path_invalid as for every other bad path; the wizard tells the two
// apart to name the real reason (admin.CheckSetupPath, problem not_browsable).
var ErrNotBrowsable = fmt.Errorf("%w: below no browsable root", model.ErrLibraryPathInvalid)

// WithBrowseRoots sets the directories a new library folder must lie below
// (SLOUCHCAT_BROWSE_ROOTS, idea 02, E4). An empty list switches the rule off: an installation
// whose media are somewhere else must still be able to finish its setup.
func WithBrowseRoots(dirs []string) Option {
	return func(s *Service) { s.browseRoots = append([]string(nil), dirs...) }
}

// BrowseRoots returns the configured browsable roots.
func (s *Service) BrowseRoots() []string { return s.browseRoots }

// ValidateNewRoot checks a folder that is about to become a library root: it must lie below one
// of the browsable roots (E14) and it must be an absolute, readable directory (ValidateDir).
//
// The root rule sits here and not in ValidateDir, because ValidateDir also serves the wizard's
// path check and RunSetup, where a path outside the list must still be describable. It runs
// before ValidateDir, so a path below no root costs no file system call on that path, and it runs
// twice: once lexically and once on the path filepath.EvalSymlinks resolved. The second pass is
// not optional. paths.Open calls os.OpenRoot on the root's own directory, and the kernel resolves
// that directory with its symlinks; only what lies below it is locked in. Without the second pass
// a root /media/ablage -> /config would pass the lexical check and hand out /config.
//
// A folder that is already a root of a library keeps its right (exact path equality, never a
// prefix): an update must not stop a running installation, and Create refuses overlapping roots
// anyway, so a prefix rule would be dead code.
func (s *Service) ValidateNewRoot(ctx context.Context, path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("%w: not an absolute path", model.ErrLibraryPathInvalid)
	}
	if err := s.checkBrowsable(ctx, filepath.Clean(path)); err != nil {
		return err
	}
	return ValidateDir(path)
}

// checkBrowsable runs the two passes of E14 on an absolute, cleaned path.
func (s *Service) checkBrowsable(ctx context.Context, path string) error {
	if len(s.browseRoots) == 0 {
		return nil
	}
	if UnderAnyRoot(path, s.browseRoots) {
		return s.checkResolved(path)
	}
	grandfathered, err := s.isLibraryRoot(ctx, path)
	if err != nil {
		return err
	}
	if !grandfathered {
		return ErrNotBrowsable
	}
	return nil
}

// checkResolved repeats the lexical check on the paths with their symlinks resolved. A path that
// cannot be resolved (it does not exist yet, or a link loops) is left to ValidateDir, which names
// the real problem; it never reaches the file system through this function.
func (s *Service) checkResolved(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil
	}
	roots := make([]string, 0, len(s.browseRoots))
	for _, root := range s.browseRoots {
		if r, err := filepath.EvalSymlinks(root); err == nil {
			roots = append(roots, r)
		}
	}
	if !UnderAnyRoot(resolved, roots) {
		return ErrNotBrowsable
	}
	return nil
}

// isLibraryRoot reports whether path is the root of a library already (exact equality).
func (s *Service) isLibraryRoot(ctx context.Context, path string) (bool, error) {
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return false, fmt.Errorf("list roots: %w", err)
	}
	for _, r := range roots {
		if filepath.Clean(r.Path) == path {
			return true, nil
		}
	}
	return false, nil
}

// UnderAnyRoot reports whether path is one of the roots or lies below one of them. The comparison
// runs along the separators of cleaned paths: strings.HasPrefix on the raw path would accept
// /mediaX for the root /media, and /media/../config for it as well.
func UnderAnyRoot(path string, roots []string) bool {
	path = filepath.Clean(path)
	for _, root := range roots {
		root = filepath.Clean(root)
		if path == root {
			return true
		}
		if strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
