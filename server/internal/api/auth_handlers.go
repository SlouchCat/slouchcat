package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the setup wizard and the session (contract tags setup and auth). The strict
// interface passes no ResponseWriter, so responses that set the cookie or carry a mapped problem
// are small types with the generated Visit methods.

// serviceProblem answers a service error through problems.mapError. Each handler file adds the
// Visit methods of its operations (auth_handlers.go, admin_handlers.go, item_handlers.go).
type serviceProblem struct {
	p   problems
	r   *http.Request
	err error
}

func (e serviceProblem) visit(w http.ResponseWriter) error {
	e.p.mapError(w, e.r, e.err)
	return nil
}

func (e serviceProblem) VisitGetSetupStatusResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRunSetupResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitLoginResponse(w http.ResponseWriter) error          { return e.visit(w) }
func (e serviceProblem) VisitLogoutResponse(w http.ResponseWriter) error         { return e.visit(w) }
func (e serviceProblem) VisitGetSessionResponse(w http.ResponseWriter) error     { return e.visit(w) }

func (s *server) problem(ctx context.Context, err error) serviceProblem {
	return serviceProblem{p: s.p, r: requestFrom(ctx), err: err}
}

// sessionStarted answers setup (201) and login (200): it sets the session cookie and returns the
// session info. The token only travels in the cookie.
type sessionStarted struct {
	status  int
	r       *http.Request
	session model.Session
	token   string
}

func (s sessionStarted) visit(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(toSessionInfo(s.session)); err != nil {
		return err
	}
	setSessionCookie(w, s.r, s.token, s.session.ExpiresAt)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(s.status)
	_, err := buf.WriteTo(w)
	return err
}

func (s sessionStarted) VisitRunSetupResponse(w http.ResponseWriter) error { return s.visit(w) }
func (s sessionStarted) VisitLoginResponse(w http.ResponseWriter) error    { return s.visit(w) }

// loggedOut clears the cookie and answers 204.
type loggedOut struct{ r *http.Request }

func (l loggedOut) VisitLogoutResponse(w http.ResponseWriter) error {
	clearSessionCookie(w, l.r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func userAgent(ctx context.Context) string {
	if r := requestFrom(ctx); r != nil {
		return r.UserAgent()
	}
	return ""
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func (s *server) GetSetupStatus(ctx context.Context, _ gen.GetSetupStatusRequestObject) (gen.GetSetupStatusResponseObject, error) {
	required, err := s.opts.Auth.SetupRequired(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSetupStatus200JSONResponse{SetupRequired: required}, nil
}

func (s *server) RunSetup(ctx context.Context, req gen.RunSetupRequestObject) (gen.RunSetupResponseObject, error) {
	body := deref(req.Body)
	in := model.SetupRequest{
		Email:    string(body.Email),
		Password: deref(body.Password),
		Language: string(deref(body.Language)),
		TMDBKey:  deref(body.TmdbApiKey),
		Library:  model.LibraryRequest{Name: body.Library.Name, Kind: string(body.Library.Kind), Path: body.Library.Path},
	}
	session, token, err := s.opts.Admin.RunSetup(ctx, in, userAgent(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return sessionStarted{status: http.StatusCreated, r: requestFrom(ctx), session: session, token: token}, nil
}

func (s *server) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	body := deref(req.Body)
	session, token, err := s.opts.Auth.Login(ctx, string(body.Email), deref(body.Password), userAgent(ctx))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return sessionStarted{status: http.StatusOK, r: requestFrom(ctx), session: session, token: token}, nil
}

func (s *server) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if session, ok := sessionFrom(ctx); ok {
		if err := s.opts.Auth.Logout(ctx, session.ID); err != nil {
			return s.problem(ctx, err), nil
		}
	}
	return loggedOut{r: requestFrom(ctx)}, nil
}

func (s *server) GetSession(ctx context.Context, _ gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		// The scoped gate only lets requests with a session through.
		return nil, errors.New("getSession reached without a session")
	}
	return gen.GetSession200JSONResponse(toSessionInfo(session)), nil
}
