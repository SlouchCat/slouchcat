package admin

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/browse"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The folder browser of the library form (idea 02, A-2). Package browse does the reading; the
// policy is here: which roots a caller may see, which limit the wizard's list has and that the
// configured roots are classified once at start-up and never per request.

// setupRootsPerMin is the wizard's own limit for listSetupRoots, per client address. It is not
// the shared attempt limit of login, setup, PIN and invitations: the setup page fetches this
// list on every load, and ten reloads would have locked the operator out of their own setup
// (E5). The answer is parameterless and the same for everyone, so it is no oracle; the limit is
// only there so an open port cannot be hammered.
const setupRootsPerMin = 60

// browseState is what the service needs for the folder browser; zero means not configured, and
// then the browser answers as if nothing were browsable.
type browseState struct {
	// dirs is SLOUCHCAT_BROWSE_ROOTS as it was configured, unfiltered. It is the list E14 is
	// measured against, so that the boundary of the browser and the boundary of a new library
	// root are the same one -- a root that was not mounted at start-up must not widen either.
	dirs []string
	// configured are those of dirs that browse.ClassifyRoots found at start-up: only the ones
	// that exist, with their file system class. They are what the dialog offers.
	configured []model.BrowseRoot
	lister     *browse.Lister
	limit      *auth.Limiter
}

// WithBrowse wires the folder browser: the configured roots as they stand in the configuration,
// the same roots classified once at start-up, and the lister that reads directories. Returns the
// receiver.
func (s *Service) WithBrowse(dirs []string, configured []model.BrowseRoot, lister *browse.Lister) *Service {
	s.browse = browseState{dirs: dirs, configured: configured, lister: lister,
		limit: auth.NewLimiter(setupRootsPerMin, setupRootsPerMin)}
	return s
}

// SetupRoots returns the configured roots for the wizard: no file system class, because before
// an account exists nobody learns which mount is a network share, and nothing below them. It
// answers from the list filled at start-up, so it makes no system call, and it has a limit of
// its own instead of the shared attempt limit (E5).
func (s *Service) SetupRoots(ctx context.Context) ([]model.BrowseRoot, error) {
	if s.browse.limit != nil {
		if err := s.browse.limit.Allow(model.ClientIP(ctx), s.now()); err != nil {
			return nil, err
		}
	}
	required, err := s.auth.SetupRequired(ctx)
	if err != nil {
		return nil, err
	}
	if !required {
		return nil, model.ErrSetupDone
	}
	out := make([]model.BrowseRoot, 0, len(s.browse.configured))
	for _, r := range s.browse.configured {
		out = append(out, model.BrowseRoot{ID: r.ID, Path: r.Path, Browsable: r.Browsable, Source: r.Source})
	}
	return out, nil
}

// BrowseRoots returns the configured roots together with the roots of the libraries that exist,
// sorted by path. A library root below no configured root is listed but not browsable: the
// dialog shows it so a directory can carry its library's name, and E14 keeps it from being
// entered. Without a configured root the rule is off and every library root may be entered.
func (s *Service) BrowseRoots(ctx context.Context) ([]model.BrowseRoot, error) {
	out := slices.Clone(s.browse.configured)
	libs, err := s.libraries.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	for _, lib := range libs {
		for _, root := range lib.Roots {
			if i := slices.IndexFunc(out, func(r model.BrowseRoot) bool { return r.Path == root.Path }); i >= 0 {
				// A configured root that is also a library root stays one row, with the library.
				out[i].LibraryID, out[i].LibraryName = lib.ID, lib.Name
				continue
			}
			out = append(out, model.BrowseRoot{
				ID: "l" + strconv.FormatInt(root.ID, 10), Path: root.Path,
				Browsable:   len(s.browse.dirs) == 0 || library.UnderAnyRoot(root.Path, s.browse.dirs),
				Source:      model.BrowseRootLibrary,
				FSType:      root.FSType,
				Warning:     root.Warning,
				LibraryID:   lib.ID,
				LibraryName: lib.Name,
			})
		}
	}
	slices.SortFunc(out, func(a, b model.BrowseRoot) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// BrowsePaths lists the subdirectories of one directory below a browsable root. The roots the
// caller may see are built here; package browse decides nothing about them.
func (s *Service) BrowsePaths(ctx context.Context, req model.BrowseRequest) (model.DirectoryListing, error) {
	if s.browse.lister == nil {
		return model.DirectoryListing{}, model.ErrPathNotBrowsable
	}
	roots, err := s.BrowseRoots(ctx)
	if err != nil {
		return model.DirectoryListing{}, err
	}
	return s.browse.lister.List(ctx, browse.ToRoots(roots), browse.Request{
		RootID: req.RootID, Rel: req.Rel, Cursor: req.Cursor, Limit: req.Limit,
	})
}

// browseHealth is the dashboard's line about the folder browser (idea 02, E9).
func (s *Service) browseHealth() model.BrowseHealth {
	if s.browse.lister == nil {
		return model.BrowseHealth{}
	}
	return model.BrowseHealth{LostSlots: s.browse.lister.LostSlots(), Slots: s.browse.lister.Slots()}
}
