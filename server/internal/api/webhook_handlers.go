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
// against a browser sending a request with the ambient session cookie; the webhook receiver
// authorises by a secret in its path and ignores cookies, so the header would protect nothing
// while making the endpoint unusable for Radarr and Sonarr (I-8).
var csrfExemptRoutes = map[string]bool{http.MethodPost + " " + BasePath + webhookRoute: true}
