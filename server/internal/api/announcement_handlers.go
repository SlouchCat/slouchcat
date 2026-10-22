package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 announcements (plan 5.2, A-7; design section 1): the main user writes, publishes and deletes
// them (admin service); every profile reads the published ones it has not acknowledged and
// acknowledges them (notify service, Options.Notify; nil answers 501). They carry no title, so the
// profile operations are scoped only because they belong to the active profile.

func (e serviceProblem) VisitListAnnouncementsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCreateAnnouncementResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateAnnouncementResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitPublishAnnouncementResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteAnnouncementResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListMyAnnouncementsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitAckAnnouncementResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListAnnouncements(ctx context.Context, _ gen.ListAnnouncementsRequestObject) (gen.ListAnnouncementsResponseObject, error) {
	list, err := s.opts.Admin.ListAnnouncements(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListAnnouncements200JSONResponse, 0, len(list))
	for _, a := range list {
		out = append(out, toAnnouncement(a))
	}
	return out, nil
}

func (s *server) CreateAnnouncement(ctx context.Context, req gen.CreateAnnouncementRequestObject) (gen.CreateAnnouncementResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	b := deref(req.Body)
	a, err := s.opts.Admin.CreateAnnouncement(ctx, session.Account, model.AnnouncementInput{
		Title: b.Title, Body: b.Body, Publish: deref(b.Publish),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateAnnouncement201JSONResponse(toAnnouncement(a)), nil
}

func (s *server) UpdateAnnouncement(ctx context.Context, req gen.UpdateAnnouncementRequestObject) (gen.UpdateAnnouncementResponseObject, error) {
	b := deref(req.Body)
	a, err := s.opts.Admin.UpdateAnnouncement(ctx, req.Id, b.Title, b.Body)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAnnouncement200JSONResponse(toAnnouncement(a)), nil
}

func (s *server) PublishAnnouncement(ctx context.Context, req gen.PublishAnnouncementRequestObject) (gen.PublishAnnouncementResponseObject, error) {
	a, err := s.opts.Admin.PublishAnnouncement(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.PublishAnnouncement200JSONResponse(toAnnouncement(a)), nil
}

func (s *server) DeleteAnnouncement(ctx context.Context, req gen.DeleteAnnouncementRequestObject) (gen.DeleteAnnouncementResponseObject, error) {
	if err := s.opts.Admin.DeleteAnnouncement(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteAnnouncement204Response{}, nil
}

func (s *server) ListMyAnnouncements(ctx context.Context, _ gen.ListMyAnnouncementsRequestObject) (gen.ListMyAnnouncementsResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	list, err := s.opts.Notify.Announcements(ctx, session.Profile)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListMyAnnouncements200JSONResponse, 0, len(list))
	for _, a := range list {
		out = append(out, gen.ProfileAnnouncement{Id: a.ID, Title: a.Title, Body: a.Body, PublishedAt: a.PublishedAt.UTC()})
	}
	return out, nil
}

func (s *server) AckAnnouncement(ctx context.Context, req gen.AckAnnouncementRequestObject) (gen.AckAnnouncementResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.AckAnnouncement(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.AckAnnouncement204Response{}, nil
}

func toAnnouncement(a model.Announcement) gen.Announcement {
	out := gen.Announcement{
		Id: a.ID, Title: a.Title, Body: a.Body, CreatedAt: a.CreatedAt.UTC(), PublishedAt: utc(a.PublishedAt),
		CreatedBy: optString(a.CreatedBy), AcknowledgedCount: a.AcknowledgedCount, ProfileCount: a.ProfileCount,
	}
	if !a.UpdatedAt.IsZero() {
		u := a.UpdatedAt.UTC()
		out.UpdatedAt = &u
	}
	return out
}
