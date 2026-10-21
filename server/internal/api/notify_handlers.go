package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 in-app inbox operations (plan M7, 5.5, design section 3.3): the entries of the active profile
// through the notify service (Options.Notify; nil answers 501). Entries whose episode the profile
// may no longer see are neither listed nor counted, and marking one answers 404.

// Inbox page bounds of the contract.
const (
	defaultNotificationPage = 50
	maxNotificationPage     = 200
)

func (e serviceProblem) VisitListNotificationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMarkNotificationReadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMarkAllNotificationsReadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// notifySession returns the session of an inbox operation; ok is false on a server without the
// notify service, which answers 501.
func (s *server) notifySession(ctx context.Context) (session model.Session, ok bool, err error) {
	session, found := sessionFrom(ctx)
	if !found {
		return model.Session{}, false, errNoSession
	}
	return session, s.opts.Notify != nil, nil
}

func (s *server) ListNotifications(ctx context.Context, req gen.ListNotificationsRequestObject) (gen.ListNotificationsResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	offset, limit := max(deref(req.Params.Offset), 0), deref(req.Params.Limit)
	if limit <= 0 {
		limit = defaultNotificationPage
	}
	limit = min(limit, maxNotificationPage)
	list, total, unread, err := s.opts.Notify.Notifications(ctx, session.Profile, offset, limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.NotificationPage{Notifications: make([]gen.Notification, 0, len(list)), Total: total, Unread: unread,
		Offset: offset, Limit: limit}
	for _, n := range list {
		g := gen.Notification{Id: n.ID, Kind: gen.NotificationKind(n.Kind), CreatedAt: n.CreatedAt.UTC(),
			ReadAt: utc(n.ReadAt), Item: toItemRef(session.Profile.ID, n.Item)}
		if n.Parent != nil {
			parent := toItemRef(session.Profile.ID, *n.Parent)
			g.Parent = &parent
		}
		out.Notifications = append(out.Notifications, g)
	}
	return gen.ListNotifications200JSONResponse(out), nil
}

func (s *server) MarkNotificationRead(ctx context.Context, req gen.MarkNotificationReadRequestObject) (gen.MarkNotificationReadResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.MarkRead(ctx, session.Profile, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.MarkNotificationRead204Response{}, nil
}

func (s *server) MarkAllNotificationsRead(ctx context.Context, _ gen.MarkAllNotificationsReadRequestObject) (gen.MarkAllNotificationsReadResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.MarkAllRead(ctx, session.Profile); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.MarkAllNotificationsRead204Response{}, nil
}
