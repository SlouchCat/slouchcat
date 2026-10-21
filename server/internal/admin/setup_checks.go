package admin

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// Wizard checks of M8 (A-1, design 3.3): the library path and the TMDB key are checked while the
// wizard runs, before RunSetup. Both count against the attempt limit of the client and are refused
// once an account exists, like RunSetup. A bad path or key is described in the result, not an error.

// Problems of PathCheck and TMDBCheck.
const (
	pathNotAbsolute  = "not_absolute"
	pathMissing      = "missing"
	pathNotDirectory = "not_directory"
	pathNotReadable  = "not_readable"
	tmdbInvalid      = "invalid"
	tmdbUnreachable  = "unreachable"
)

// setupCheckAllowed counts the attempt and refuses the checks once the wizard ran.
func (s *Service) setupCheckAllowed(ctx context.Context) error {
	if err := s.auth.AllowAttempt(ctx); err != nil {
		return err
	}
	required, err := s.auth.SetupRequired(ctx)
	if err != nil {
		return err
	}
	if !required {
		return model.ErrSetupDone
	}
	return nil
}

// CheckSetupPath describes a library path for the wizard: whether it is an absolute, readable
// directory, its file system, the watch mode the scanner would use and the warning for network
// shares and unknown FUSE file systems (plan 3.4).
func (s *Service) CheckSetupPath(ctx context.Context, path string) (model.PathCheck, error) {
	if err := s.setupCheckAllowed(ctx); err != nil {
		return model.PathCheck{}, err
	}
	switch {
	case strings.TrimSpace(path) == "":
		return model.PathCheck{}, (&model.ValidationError{}).Add("path", "required")
	case len(path) > maxLibraryPath:
		return model.PathCheck{}, (&model.ValidationError{}).Add("path", "too_long")
	case !filepath.IsAbs(path):
		return model.PathCheck{Problem: pathNotAbsolute}, nil
	}
	if err := library.ValidateDir(path); err != nil {
		return model.PathCheck{Problem: pathProblem(path)}, nil
	}
	fsType, err := paths.DetectFS(filepath.Clean(path))
	if err != nil {
		return model.PathCheck{Problem: pathProblem(path)}, nil
	}
	watch, warning := library.WatchMode(string(fsType))
	return model.PathCheck{OK: true, FSType: string(fsType), WatchMode: watch, Warning: warning}, nil
}

// pathProblem names why a path that failed the check is not usable; the errors of the check do
// not tell a file from a directory (os.OpenRoot), a stat of the path does.
func pathProblem(path string) string {
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return pathMissing
	case errors.Is(err, syscall.ENOTDIR), err == nil && !fi.IsDir():
		return pathNotDirectory
	default:
		return pathNotReadable
	}
}

// CheckSetupTMDBKey asks TMDB whether the key is valid; the key is neither stored nor logged.
func (s *Service) CheckSetupTMDBKey(ctx context.Context, key string) (model.TMDBCheck, error) {
	if err := s.setupCheckAllowed(ctx); err != nil {
		return model.TMDBCheck{}, err
	}
	key = strings.TrimSpace(key)
	switch {
	case key == "":
		return model.TMDBCheck{}, (&model.ValidationError{}).Add("tmdbApiKey", "required")
	case len(key) > maxTMDBKeyLen:
		return model.TMDBCheck{}, (&model.ValidationError{}).Add("tmdbApiKey", "too_long")
	}
	switch err := s.tmdb.CheckKey(ctx, key); {
	case err == nil:
		return model.TMDBCheck{OK: true}, nil
	case errors.Is(err, model.ErrTMDBKeyInvalid):
		return model.TMDBCheck{Problem: tmdbInvalid}, nil
	case errors.Is(err, model.ErrTMDBUnreachable):
		return model.TMDBCheck{Problem: tmdbUnreachable}, nil
	default:
		return model.TMDBCheck{}, err
	}
}
