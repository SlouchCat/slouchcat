package api

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Handlers of library administration (contract tag admin). Implemented by the auth/admin
// executor; the stubs below only keep the package compiling.

func (s *server) ListLibraries(context.Context, gen.ListLibrariesRequestObject) (gen.ListLibrariesResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) CreateLibrary(context.Context, gen.CreateLibraryRequestObject) (gen.CreateLibraryResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetLibrary(context.Context, gen.GetLibraryRequestObject) (gen.GetLibraryResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) ScanLibrary(context.Context, gen.ScanLibraryRequestObject) (gen.ScanLibraryResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) AcceptLibraryLoss(context.Context, gen.AcceptLibraryLossRequestObject) (gen.AcceptLibraryLossResponseObject, error) {
	return nil, errNotImplemented
}
