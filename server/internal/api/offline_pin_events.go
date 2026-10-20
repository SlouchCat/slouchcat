package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handlers of the offline PIN (AN-19) and of the upload of offline progress events (O-5, AN-10,
// AN-18); all x-visibility account. The services check the primary profile and the account.

func (e serviceProblem) VisitSetOfflinePinResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitRemoveOfflinePinResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUploadOfflineEventsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) SetOfflinePin(ctx context.Context, req gen.SetOfflinePinRequestObject) (gen.SetOfflinePinResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.SetOfflinePIN(ctx, session, req.Id, deref(deref(req.Body).Pin)); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SetOfflinePin204Response{}, nil
}

func (s *server) RemoveOfflinePin(ctx context.Context, req gen.RemoveOfflinePinRequestObject) (gen.RemoveOfflinePinResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.RemoveOfflinePIN(ctx, session, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RemoveOfflinePin204Response{}, nil
}

func (s *server) UploadOfflineEvents(ctx context.Context, req gen.UploadOfflineEventsRequestObject) (gen.UploadOfflineEventsResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	if s.opts.Playback == nil {
		return nil, errNoItemsServices
	}
	b := deref(req.Body)
	events := make([]model.OfflineEvent, 0, len(b.Events))
	for _, e := range b.Events {
		events = append(events, model.OfflineEvent{
			ProfileID: e.ProfileId, ItemID: e.ItemId, PlaybackID: e.PlaybackId, DownloadID: deref(e.DownloadId),
			MediaFileID: deref(e.MediaFileId), PositionMs: e.PositionMs, DurationMs: e.DurationMs, PlayedMs: e.PlayedMs,
			Seq: e.Seq, State: string(e.State), ClientTime: e.ClientTime, PlaybackRate: rate32(e.PlaybackRate),
		})
	}
	res, err := s.opts.Playback.ReportOffline(ctx, session, b.SentAt, events)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.UploadOfflineEvents200JSONResponse{
		Accepted: res.Accepted, Duplicates: res.Duplicates, Rejected: res.Rejected,
		Results: make([]gen.OfflineEventResultResults, 0, len(res.Results)),
	}
	for _, r := range res.Results {
		out.Results = append(out.Results, gen.OfflineEventResultResults(r))
	}
	return out, nil
}
