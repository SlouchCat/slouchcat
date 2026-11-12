package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of deleting titles (concept 20 P-9). Errors go through serviceProblem.

func (e serviceProblem) VisitGetItemDeletionResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitDeleteItemResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitListRemovedItemsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRestoreRemovedItemResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) GetItemDeletion(ctx context.Context, req gen.GetItemDeletionRequestObject) (gen.GetItemDeletionResponseObject, error) {
	p, err := s.opts.Admin.GetItemDeletion(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.GetItemDeletion200JSONResponse{
		ItemId: p.ItemID, Kind: gen.ItemKind(p.Kind), ConfirmTitle: p.ConfirmTitle, TypeTitle: p.TypeTitle,
		Files: make([]gen.DeletionFile, 0, len(p.Files)), Folder: toDeletionFolder(p.Folder),
		FileCount: p.FileCount, TotalBytes: p.TotalBytes, PlayingProfiles: append([]string{}, p.PlayingProfiles...),
		FilesDeletable: p.FilesDeletable, FilesReason: optString(p.FilesReason),
		ExpectedFiles: gen.ExpectedFiles{Count: p.Expected.Count, Hash: p.Expected.Hash},
	}
	for _, f := range p.Files {
		out.Files = append(out.Files, gen.DeletionFile{
			RootPath: f.RootPath, Path: f.Path, SizeBytes: f.SizeBytes, Deletable: f.Deletable, Reason: optString(f.Reason),
		})
	}
	return out, nil
}

func (s *server) DeleteItem(ctx context.Context, req gen.DeleteItemRequestObject) (gen.DeleteItemResponseObject, error) {
	body := deref(req.Body)
	session, _ := sessionFrom(ctx)
	expected := model.ExpectedFiles{Count: body.ExpectedFiles.Count, Hash: body.ExpectedFiles.Hash}
	r, err := s.opts.Admin.DeleteItem(ctx, req.Id, string(body.Mode), deref(body.ConfirmTitle), expected, actorOf(session))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.DeleteItem200JSONResponse{
		RemovedItems: r.RemovedItems, DeletedFiles: r.DeletedFiles, DeletedBytes: r.DeletedBytes,
		Leftover: make([]gen.DeletionLeftover, 0, len(r.Leftover)), LeftInFolder: &r.LeftInFolder, TitleDeleted: &r.TitleDeleted,
	}
	for _, l := range r.Leftover {
		out.Leftover = append(out.Leftover, gen.DeletionLeftover{RootPath: l.RootPath, Path: l.Path, Reason: l.Reason})
	}
	return out, nil
}

func (s *server) ListRemovedItems(ctx context.Context, req gen.ListRemovedItemsRequestObject) (gen.ListRemovedItemsResponseObject, error) {
	page, err := s.opts.Admin.ListRemovedItems(ctx, deref(req.Params.Limit), deref(req.Params.Offset))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ListRemovedItems200JSONResponse{Total: page.Total, Items: make([]gen.RemovedItem, 0, len(page.Items))}
	for _, it := range page.Items {
		out.Items = append(out.Items, gen.RemovedItem{
			Id: it.ID, Kind: gen.ItemKind(it.Kind), Title: it.Title, ParentTitle: optString(it.ParentTitle),
			LibraryId: it.LibraryID, LibraryName: it.LibraryName, Folder: toDeletionFolder(it.Folder), RemovedAt: it.RemovedAt.UTC(),
		})
	}
	return out, nil
}

func (s *server) RestoreRemovedItem(ctx context.Context, req gen.RestoreRemovedItemRequestObject) (gen.RestoreRemovedItemResponseObject, error) {
	n, err := s.opts.Admin.RestoreRemovedItem(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RestoreRemovedItem200JSONResponse{ItemId: req.Id, RestoredItems: n}, nil
}

func toDeletionFolder(f *model.DeletionFolder) *gen.DeletionFolder {
	if f == nil {
		return nil
	}
	return &gen.DeletionFolder{RootPath: f.RootPath, Path: f.Path}
}
