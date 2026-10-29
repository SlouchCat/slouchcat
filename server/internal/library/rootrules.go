package library

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The rules of a library folder (ADR 0015, concept section 2), applied once per request for all
// its folders: a folder's content must fit the family of its library (only video mixes, F-3), its
// provider list must start with an allowed identifier and continue with allowed fillers, its path
// must be absolute, readable and below a browsable root (idea 02, E14), and no two folders may
// overlap -- neither two of the same request nor one of them and a stored root. Every violation
// is a field error with the index of the folder that caused it (roots[1].providers), so a request
// with three folders fails as a whole and names the one to fix.

// errRootOverlap is the reason a folder is refused because it is, lies below or holds another
// library root; it is reported as the field code root_overlap.
var errRootOverlap = errors.New("folder overlaps a library root")

// preparedRoot is a folder of a request after the rules passed: its cleaned path and the file
// system facts the row needs.
type preparedRoot struct {
	path      string
	content   string
	providers []string
	fsType    string
	watch     string
}

// prepareRoots applies the rules to the folders of a request for a library of family kind.
// field is the name of the list in the request ("roots", or "library.roots" in the wizard).
// It returns the folders ready to insert, or a *model.ValidationError naming every violation.
func (s *Service) prepareRoots(ctx context.Context, kind string, reqs []model.LibraryRootRequest, field string) ([]preparedRoot, error) {
	var verr *model.ValidationError
	out := make([]preparedRoot, 0, len(reqs))
	seen := make([]string, 0, len(reqs))
	for i, r := range reqs {
		name := fmt.Sprintf("%s[%d].", field, i)
		ok := true
		switch {
		case !model.ValidRootContent(r.Content):
			verr = verr.Add(name+"content", "invalid_value")
			ok = false
		case !model.ContentAllowed(kind, r.Content):
			verr = verr.Add(name+"content", model.CodeContentNotAllowed)
			ok = false
		}
		if !model.ValidRootProviders(r.Content, r.Providers) {
			verr = verr.Add(name+"providers", model.CodeProvidersNotAllowed)
			ok = false
		}
		clean, err := s.prepareRootPath(ctx, r.Path, seen)
		switch {
		case errors.Is(err, errRootOverlap):
			verr = verr.Add(name+"path", model.CodeRootOverlap)
			ok = false
		case errors.Is(err, model.ErrLibraryPathInvalid):
			verr = verr.Add(name+"path", "library_path_invalid")
			ok = false
		case err != nil:
			return nil, err
		}
		if !ok {
			continue
		}
		seen = append(seen, clean)
		fsType, err := paths.DetectFS(clean)
		if err != nil {
			verr = verr.Add(name+"path", "library_path_invalid")
			continue
		}
		watch, _ := WatchMode(string(fsType))
		out = append(out, preparedRoot{path: clean, content: r.Content, providers: r.Providers, fsType: string(fsType), watch: watch})
	}
	if verr != nil {
		return nil, verr
	}
	return out, nil
}

// prepareRootPath validates one folder path (ValidateNewRoot) and refuses it when it overlaps a
// stored root (refuseOverlap) or one of the folders of the same request (earlier). It returns the
// cleaned path.
func (s *Service) prepareRootPath(ctx context.Context, path string, earlier []string) (string, error) {
	path = strings.TrimSpace(path)
	if err := s.ValidateNewRoot(ctx, path); err != nil {
		return "", err
	}
	clean := filepath.Clean(path)
	real := resolved(clean)
	for _, e := range earlier {
		eReal := resolved(e)
		if nested(clean, e) || nested(e, clean) || nested(real, eReal) || nested(eReal, real) {
			return "", errRootOverlap
		}
	}
	if err := s.refuseOverlap(ctx, clean); err != nil {
		return "", err
	}
	return clean, nil
}

// ValidateRoots runs the folder rules of a request without writing anything: the setup wizard
// checks the first library this way before it creates the account, so a bad folder leaves no
// half-finished installation behind. field names the list in the request ("library.roots").
func (s *Service) ValidateRoots(ctx context.Context, kind string, reqs []model.LibraryRootRequest, field string) error {
	_, err := s.prepareRoots(ctx, kind, reqs, field)
	return err
}

// insertRoot writes one prepared folder.
func insertRoot(ctx context.Context, q *appq.Queries, libraryID int64, r preparedRoot, now int64) (appq.LibraryRoot, error) {
	return q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{
		LibraryID: libraryID, Path: r.path, FsType: r.fsType, Watch: r.watch,
		Content: r.content, Providers: model.ProvidersJSON(r.providers), Now: now,
	})
}
