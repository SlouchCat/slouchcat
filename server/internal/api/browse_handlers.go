package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The folder browser of the library form (idea 02, A-2). The admin service does the work; these
// handlers map between the contract and the model and keep the answers out of every cache.

func (e serviceProblem) VisitBrowseServerPathsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListBrowseRootsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitListSetupRootsResponse(w http.ResponseWriter) error  { return e.visit(w) }

// noStoreJSON writes a 200 with Cache-Control: no-store. The problem writer sets that header for
// problem responses only, and a directory listing names folders of the media tree: no proxy and
// no browser cache keeps one.
type noStoreJSON struct{ body any }

func (n noStoreJSON) visit(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(n.body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, err := buf.WriteTo(w)
	return err
}

func (n noStoreJSON) VisitBrowseServerPathsResponse(w http.ResponseWriter) error { return n.visit(w) }
func (n noStoreJSON) VisitListBrowseRootsResponse(w http.ResponseWriter) error   { return n.visit(w) }
func (n noStoreJSON) VisitListSetupRootsResponse(w http.ResponseWriter) error    { return n.visit(w) }

func (s *server) BrowseServerPaths(ctx context.Context, req gen.BrowseServerPathsRequestObject) (gen.BrowseServerPathsResponseObject, error) {
	body := deref(req.Body)
	listing, err := s.opts.Admin.BrowsePaths(ctx, model.BrowseRequest{
		RootID: body.RootId, Rel: deref(body.Rel), Cursor: deref(body.Cursor), Limit: deref(body.Limit),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return noStoreJSON{body: toDirectoryListing(listing)}, nil
}

func (s *server) ListBrowseRoots(ctx context.Context, _ gen.ListBrowseRootsRequestObject) (gen.ListBrowseRootsResponseObject, error) {
	roots, err := s.opts.Admin.BrowseRoots(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return noStoreJSON{body: toBrowseRoots(roots)}, nil
}

func (s *server) ListSetupRoots(ctx context.Context, _ gen.ListSetupRootsRequestObject) (gen.ListSetupRootsResponseObject, error) {
	roots, err := s.opts.Admin.SetupRoots(model.WithClientIP(ctx, s.clientAddr(ctx)))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return noStoreJSON{body: toBrowseRoots(roots)}, nil
}

func toBrowseRoots(roots []model.BrowseRoot) []gen.BrowseRoot {
	out := make([]gen.BrowseRoot, 0, len(roots))
	for _, r := range roots {
		browsable := r.Browsable
		row := gen.BrowseRoot{
			Id: r.ID, Path: r.Path, Browsable: &browsable, Source: gen.BrowseRootSource(r.Source),
			FsType: optString(r.FSType), LibraryName: optString(r.LibraryName),
		}
		if r.Warning != "" {
			warning := gen.BrowseWarning(r.Warning)
			row.Warning = &warning
		}
		if r.LibraryID != 0 {
			id := r.LibraryID
			row.LibraryId = &id
		}
		out = append(out, row)
	}
	return out
}

func toDirectoryListing(l model.DirectoryListing) gen.DirectoryListing {
	out := gen.DirectoryListing{
		RootId: l.RootID, Rel: l.Rel, AtRoot: l.AtRoot, Path: optString(l.Path),
		FsType: optString(l.FSType), NextCursor: optString(l.NextCursor),
		Entries: make([]gen.DirectoryEntry, 0, len(l.Entries)),
	}
	if !l.AtRoot {
		// Present even when empty: that is the root itself, one level up from here.
		parent := l.ParentRel
		out.ParentRel = &parent
	}
	if l.Watch != "" {
		watch := gen.BrowseWatch(l.Watch)
		out.Watch = &watch
	}
	if l.Warning != "" {
		warning := gen.BrowseWarning(l.Warning)
		out.Warning = &warning
	}
	if l.Problem != "" {
		problem := gen.DirectoryListingProblem(l.Problem)
		out.Problem = &problem
	}
	if l.Truncated {
		truncated := true
		out.Truncated = &truncated
	}
	for _, e := range l.Entries {
		row := gen.DirectoryEntry{Name: e.Name, Rel: e.Rel, Path: optString(e.Path), LibraryName: optString(e.LibraryName)}
		if e.Symlink {
			symlink := true
			row.Symlink = &symlink
		}
		if e.LibraryID != 0 {
			id := e.LibraryID
			row.LibraryId = &id
		}
		out.Entries = append(out.Entries, row)
	}
	return out
}
