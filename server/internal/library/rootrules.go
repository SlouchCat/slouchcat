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
	meta      model.RootMetadata
	fsType    string
	watch     string
}

// checkMetadata checks a folder's own text languages and images (concept 24 3.1): only a video
// folder has them (not_allowed), and with WithRootMetadataCheck their codes and kinds. Field names
// carry prefix ("roots[0]." or "").
func (s *Service) checkMetadata(content string, m model.RootMetadata, prefix string) []model.FieldError {
	if m.TextLangs == nil && m.Images == nil {
		return nil
	}
	var out []model.FieldError
	if content != model.RootMovie && content != model.RootSeries {
		if m.TextLangs != nil {
			out = append(out, model.FieldError{Field: prefix + "textLangs", Code: "not_allowed"})
		}
		if m.Images != nil {
			out = append(out, model.FieldError{Field: prefix + "images", Code: "not_allowed"})
		}
		return out
	}
	if s.checkRootMetadata == nil {
		return nil
	}
	if found := s.checkRootMetadata(m); found != nil {
		for _, f := range found.Fields {
			out = append(out, model.FieldError{Field: prefix + f.Field, Code: f.Code})
		}
	}
	return out
}

// prepareRoots applies the rules to the folders of a request for a library of family kind.
// field is the name of the list in the request ("roots", or "library.roots" in the wizard).
// It returns the folders ready to insert, or a *model.ValidationError naming every violation.
func (s *Service) prepareRoots(ctx context.Context, kind string, reqs []model.LibraryRootRequest, field string) ([]preparedRoot, error) {
	var verr *model.ValidationError
	out := make([]preparedRoot, 0, len(reqs))
	seen := make([]string, 0, len(reqs))
	for i, r := range reqs {
		p, ok, err := s.prepareRoot(ctx, kind, r, fmt.Sprintf("%s[%d].", field, i), &verr, seen)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		seen = append(seen, p.path)
		out = append(out, p)
	}
	if verr != nil {
		return nil, verr
	}
	return out, nil
}

// prepareRoot applies the rules to one new folder. Its field errors go to verr with the prefix
// name ("roots[2]."); ok is false when it broke a rule. earlier and except are those of
// prepareRootPath.
func (s *Service) prepareRoot(ctx context.Context, kind string, r model.LibraryRootRequest, name string, verr **model.ValidationError, earlier []string, except ...int64) (preparedRoot, bool, error) {
	ok := true
	add := func(field, code string) {
		*verr = (*verr).Add(name+field, code)
		ok = false
	}
	switch {
	case !model.ValidRootContent(r.Content):
		add("content", "invalid_value")
	case !model.ContentAllowed(kind, r.Content):
		add("content", model.CodeContentNotAllowed)
	}
	if !model.ValidRootProviders(r.Content, r.Providers) {
		add("providers", model.CodeProvidersNotAllowed)
	}
	for _, f := range s.checkMetadata(r.Content, r.Metadata, name) {
		*verr = (*verr).Add(f.Field, f.Code)
		ok = false
	}
	clean, err := s.prepareRootPath(ctx, r.Path, earlier, except...)
	switch {
	case errors.Is(err, errRootOverlap):
		add("path", model.CodeRootOverlap)
	case errors.Is(err, model.ErrLibraryPathInvalid):
		add("path", "library_path_invalid")
	case err != nil:
		return preparedRoot{}, false, err
	}
	if !ok {
		return preparedRoot{}, false, nil
	}
	fsType, err := paths.DetectFS(clean)
	if err != nil {
		add("path", "library_path_invalid")
		return preparedRoot{}, false, nil
	}
	watch, _ := WatchMode(string(fsType))
	return preparedRoot{path: clean, content: r.Content, providers: r.Providers, meta: r.Metadata, fsType: string(fsType), watch: watch}, true, nil
}

// prepareRootPath validates one folder path (ValidateNewRoot) and refuses it when it overlaps a
// stored root other than those in except (refuseOverlap) or one of the folders of the same
// request (earlier). It returns the cleaned path.
func (s *Service) prepareRootPath(ctx context.Context, path string, earlier []string, except ...int64) (string, error) {
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
	if err := s.refuseOverlap(ctx, clean, except...); err != nil {
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
	textLangs, images := model.RootMetadataColumns(r.meta)
	return q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{
		LibraryID: libraryID, Path: r.path, FsType: r.fsType, Watch: r.watch,
		Content: r.content, Providers: model.ProvidersJSON(r.providers), TextLangs: textLangs, Images: images, Now: now,
	})
}
