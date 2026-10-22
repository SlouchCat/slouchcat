package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M9 incoming webhooks of Radarr and Sonarr (plan 5.6, spec I-8): one public route whose whole
// authorisation is the token in its path (Options.Integrations; nil answers 501). The handler
// hands the few fields it reads to the service and never logs the token — the redacting handler
// masks the path of this route (CLAUDE.md rule 10, plan 3.8 point 8).

func (e serviceProblem) VisitReceiveWebhookResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ReceiveWebhook(ctx context.Context, req gen.ReceiveWebhookRequestObject) (gen.ReceiveWebhookResponseObject, error) {
	if s.opts.Integrations == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	var p model.WebhookPayload
	if req.Body != nil {
		if req.Body.EventType != nil {
			p.Event = *req.Body.EventType
		}
		if req.Body.Movie != nil && req.Body.Movie.FolderPath != nil {
			p.MoviePath = *req.Body.Movie.FolderPath
		}
		if req.Body.Series != nil && req.Body.Series.Path != nil {
			p.SeriesPath = *req.Body.Series.Path
		}
	}
	res, err := s.opts.Integrations.Receive(ctx, req.Token, p)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ReceiveWebhook200JSONResponse{Result: gen.WebhookResultResult(res.Result)}
	if res.Path != "" {
		out.Path = &res.Path
	}
	return out, nil
}

// webhookRoute is the route pattern of the receiver; the CSRF rule does not apply to it
// (csrfExemptRoutes) because Radarr and Sonarr are not browsers.
const webhookRoute = "/webhooks/{token}"

// csrfExemptRoutes are the routes that may arrive without the CSRF header. The header defends
// against a browser sending a request with the ambient session cookie. The receiver's handler
// never looks at a session — its only authorisation is the secret in the path — and SameSite=Lax
// keeps the cookie off a cross-site POST anyway, so the header would protect nothing while making
// the endpoint unusable for Radarr and Sonarr, which are not browsers (I-8, ADR 0010).
var csrfExemptRoutes = map[string]bool{http.MethodPost + " " + BasePath + webhookRoute: true}

// Administration of the integrations (M9): the main account with its primary profile creates,
// renews, maps and deletes the addresses and reads the event logs (Options.Admin).

func (e serviceProblem) VisitListIntegrationsResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCreateIntegrationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteIntegrationResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRenewIntegrationTokenResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSetIntegrationPathMapResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitListIntegrationEventsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) ListIntegrations(ctx context.Context, _ gen.ListIntegrationsRequestObject) (gen.ListIntegrationsResponseObject, error) {
	list, err := s.opts.Admin.ListIntegrations(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListIntegrations200JSONResponse, 0, len(list))
	for _, i := range list {
		out = append(out, toIntegration(i))
	}
	return out, nil
}

func (s *server) CreateIntegration(ctx context.Context, req gen.CreateIntegrationRequestObject) (gen.CreateIntegrationResponseObject, error) {
	b := deref(req.Body)
	created, err := s.opts.Admin.CreateIntegration(ctx, b.Name, string(b.Kind))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateIntegration201JSONResponse(toIntegrationCreated(created)), nil
}

func (s *server) RenewIntegrationToken(ctx context.Context, req gen.RenewIntegrationTokenRequestObject) (gen.RenewIntegrationTokenResponseObject, error) {
	renewed, err := s.opts.Admin.RenewIntegrationToken(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RenewIntegrationToken200JSONResponse(toIntegrationCreated(renewed)), nil
}

func (s *server) SetIntegrationPathMap(ctx context.Context, req gen.SetIntegrationPathMapRequestObject) (gen.SetIntegrationPathMapResponseObject, error) {
	b := deref(req.Body)
	pairs := make([]model.PathMapping, 0, len(b.PathMap))
	for _, p := range b.PathMap {
		pairs = append(pairs, model.PathMapping{From: p.From, To: p.To})
	}
	updated, err := s.opts.Admin.SetIntegrationPathMap(ctx, req.Id, pairs)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetIntegrationPathMap200JSONResponse(toIntegration(updated)), nil
}

func (s *server) DeleteIntegration(ctx context.Context, req gen.DeleteIntegrationRequestObject) (gen.DeleteIntegrationResponseObject, error) {
	if err := s.opts.Admin.DeleteIntegration(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteIntegration204Response{}, nil
}

func (s *server) ListIntegrationEvents(ctx context.Context, req gen.ListIntegrationEventsRequestObject) (gen.ListIntegrationEventsResponseObject, error) {
	events, err := s.opts.Admin.IntegrationEvents(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListIntegrationEvents200JSONResponse, 0, len(events))
	for _, e := range events {
		entry := gen.IntegrationEvent{
			Id: e.ID, ReceivedAt: e.ReceivedAt, Event: e.Event,
			Result: gen.IntegrationEventResult(e.Result),
		}
		if e.ReportedPath != "" {
			entry.ReportedPath = &e.ReportedPath
		}
		if e.ResolvedPath != "" {
			entry.ResolvedPath = &e.ResolvedPath
		}
		out = append(out, entry)
	}
	return out, nil
}

func toIntegration(i model.Integration) gen.Integration {
	out := gen.Integration{
		Id: i.ID, Name: i.Name, Kind: gen.IntegrationKind(i.Kind), TokenTail: i.TokenTail,
		PathMap: make([]gen.PathMapping, 0, len(i.PathMap)), CreatedAt: i.CreatedAt,
	}
	for _, p := range i.PathMap {
		out.PathMap = append(out.PathMap, gen.PathMapping{From: p.From, To: p.To})
	}
	if !i.LastSeen.IsZero() {
		last := i.LastSeen
		out.LastSeenAt = &last
	}
	return out
}

// toIntegrationCreated carries the address that exists only in this one answer (3.8 point 2).
func toIntegrationCreated(i model.Integration) gen.IntegrationCreated {
	return gen.IntegrationCreated{Integration: toIntegration(i), Url: i.URL, Token: i.Token}
}
