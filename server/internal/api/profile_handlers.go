package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the profile picker, the PIN and the profile management (contract tags auth and
// profiles, all x-visibility account). The services check that the primary profile is active.

var errNoProfileService = errors.New("profile service not configured")

func (e serviceProblem) VisitListMyProfilesResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSelectProfileResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListAccountLibrariesResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListProfilesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateProfileResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateProfileResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteProfileResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitSetProfilePinResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRemoveProfilePinResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// accountSession returns the session of an account operation together with the profile service.
func (s *server) accountSession(ctx context.Context) (model.Session, ProfileService, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return model.Session{}, nil, errNoSession
	}
	if s.opts.Profiles == nil {
		return model.Session{}, nil, errNoProfileService
	}
	return session, s.opts.Profiles, nil
}

func (s *server) ListMyProfiles(ctx context.Context, _ gen.ListMyProfilesRequestObject) (gen.ListMyProfilesResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	profiles, err := svc.ListMyProfiles(ctx, session.Account.ID)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListMyProfiles200JSONResponse, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, toProfileSummary(p))
	}
	return out, nil
}

func (s *server) SelectProfile(ctx context.Context, req gen.SelectProfileRequestObject) (gen.SelectProfileResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	ctx = model.WithClientIP(ctx, s.clientAddr(ctx))
	switched, err := svc.SelectProfile(ctx, session, req.Id, deref(deref(req.Body).Pin))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	// Stream tokens of the previous profile stop working: end the browser session's playback.
	if s.opts.Playback != nil {
		if err := s.opts.Playback.EndSession(ctx, session.ID); err != nil {
			return s.problem(ctx, err), nil
		}
	}
	return sessionSwitched{session: switched}, nil
}

func (s *server) ListProfiles(ctx context.Context, _ gen.ListProfilesRequestObject) (gen.ListProfilesResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	profiles, err := svc.ListProfiles(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListProfiles200JSONResponse, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, toProfile(p))
	}
	return out, nil
}

func (s *server) CreateProfile(ctx context.Context, req gen.CreateProfileRequestObject) (gen.CreateProfileResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	in := model.ProfileInput{
		Name: &b.Name, MaxRating: ratingPtr(b.MaxRating), Language: languagePtr(b.Language), Accent: accentPtr(b.Accent),
		AllLibraries: b.AllLibraries, LibraryIDs: b.LibraryIds, PIN: b.Pin, Limits: fromPlaybackLimits(b.Limits),
	}
	p, err := svc.CreateProfile(ctx, session, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateProfile201JSONResponse(toProfile(p)), nil
}

func (s *server) UpdateProfile(ctx context.Context, req gen.UpdateProfileRequestObject) (gen.UpdateProfileResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	b := deref(req.Body)
	in := model.ProfileInput{
		Name: b.Name, MaxRating: ratingPtr(b.MaxRating), Language: languagePtr(b.Language), Accent: accentPtr(b.Accent),
		AllLibraries: b.AllLibraries, LibraryIDs: b.LibraryIds, Limits: fromPlaybackLimits(b.Limits),
	}
	p, err := svc.UpdateProfile(ctx, session, req.Id, in)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	if err := s.recheckStreams(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateProfile200JSONResponse(toProfile(p)), nil
}

func (s *server) DeleteProfile(ctx context.Context, req gen.DeleteProfileRequestObject) (gen.DeleteProfileResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	var end func(context.Context, int64) error
	if s.opts.Playback != nil {
		end = s.opts.Playback.EndProfile
	}
	if err := svc.DeleteProfile(ctx, session, req.Id, end); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteProfile204Response{}, nil
}

func (s *server) SetProfilePin(ctx context.Context, req gen.SetProfilePinRequestObject) (gen.SetProfilePinResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.SetProfilePIN(ctx, session, req.Id, deref(deref(req.Body).Pin)); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetProfilePin204Response{}, nil
}

func (s *server) RemoveProfilePin(ctx context.Context, req gen.RemoveProfilePinRequestObject) (gen.RemoveProfilePinResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.RemoveProfilePIN(ctx, session, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveProfilePin204Response{}, nil
}

func ratingPtr(r *gen.Rating) *int {
	if r == nil {
		return nil
	}
	v := int(*r)
	return &v
}

func languagePtr(l *gen.Language) *string {
	if l == nil {
		return nil
	}
	v := string(*l)
	return &v
}

func accentPtr(a *gen.Accent) *string {
	if a == nil {
		return nil
	}
	v := string(*a)
	return &v
}

func (s *server) ListAccountLibraries(ctx context.Context, _ gen.ListAccountLibrariesRequestObject) (gen.ListAccountLibrariesResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	libs, err := svc.ListAccountLibraries(ctx, session.Account)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListAccountLibraries200JSONResponse, 0, len(libs))
	for _, l := range libs {
		out = append(out, gen.LibrarySummary{Id: l.ID, Name: l.Name, Kind: gen.LibraryKind(l.Kind)})
	}
	return out, nil
}

// recheckStreams ends open direct-play streams whose title became invisible after a change of
// ratings, ceilings or library rights.
func (s *server) recheckStreams(ctx context.Context) error {
	if s.opts.Playback == nil {
		return nil
	}
	return s.opts.Playback.RecheckVisibility(ctx)
}
