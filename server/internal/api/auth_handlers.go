package api

import (
	"context"
	"errors"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Handlers of the setup wizard and the session (contract tags setup and auth). Implemented by
// the auth/admin executor; the stubs below only keep the package compiling.

var errNotImplemented = errors.New("not implemented")

func (s *server) GetSetupStatus(context.Context, gen.GetSetupStatusRequestObject) (gen.GetSetupStatusResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) RunSetup(context.Context, gen.RunSetupRequestObject) (gen.RunSetupResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) Login(context.Context, gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) Logout(context.Context, gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetSession(context.Context, gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	return nil, errNotImplemented
}
