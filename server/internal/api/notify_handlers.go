package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M7 in-app inbox operations (plan M7, 5.5, design section 3.3). The base commit answers 501
// not_implemented; stream B2 replaces these bodies with the notify service (Options.Notify).

func (e serviceProblem) VisitListNotificationsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMarkNotificationReadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitMarkAllNotificationsReadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) ListNotifications(ctx context.Context, _ gen.ListNotificationsRequestObject) (gen.ListNotificationsResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) MarkNotificationRead(ctx context.Context, _ gen.MarkNotificationReadRequestObject) (gen.MarkNotificationReadResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}

func (s *server) MarkAllNotificationsRead(ctx context.Context, _ gen.MarkAllNotificationsReadRequestObject) (gen.MarkAllNotificationsReadResponseObject, error) {
	return s.problem(ctx, model.ErrNotImplemented), nil
}
