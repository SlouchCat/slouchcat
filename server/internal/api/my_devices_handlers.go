package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// «Meine Geräte» (concept 27, 4.5, contract 1.42.0): the devices of the own account and its
// sign-out. x-clients names only the app: a device session (TV) gets 403 client_not_allowed.

func (s *server) ListMyDevices(ctx context.Context, _ gen.ListMyDevicesRequestObject) (gen.ListMyDevicesResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	devices, err := svc.ListMyDevices(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListMyDevices200JSONResponse, 0, len(devices))
	for _, d := range devices {
		dev := gen.MyDevice{Id: d.ID, LastSeenAt: d.LastSeenAt, Downloads: d.Downloads, Current: d.Current, SignedInAt: d.SignedInAt}
		if d.Name != "" {
			dev.Name = &d.Name
		}
		if d.Type != "" {
			kind := gen.MyDeviceType(d.Type)
			dev.Type = &kind
		}
		if d.SessionKind != "" {
			kind := gen.SessionKind(d.SessionKind)
			dev.SessionKind = &kind
		}
		out = append(out, dev)
	}
	return out, nil
}

func (e serviceProblem) VisitListMyDevicesResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitSignOutMyDeviceResponse(w http.ResponseWriter) error { return e.visit(w) }

// signedOutDevice answers 204; a browser that signed its own device out gets its session cookie
// cleared like at logout (its session is revoked).
type signedOutDevice struct {
	r     *http.Request
	clear bool
}

func (o signedOutDevice) VisitSignOutMyDeviceResponse(w http.ResponseWriter) error {
	if o.clear {
		dropSessionCookie(w.Header()) // a cookie refreshed by the middleware must not survive the sign-out
		clearSessionCookie(w, o.r)
		w.Header().Set("Clear-Site-Data", clearSiteDataCache)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *server) SignOutMyDevice(ctx context.Context, req gen.SignOutMyDeviceRequestObject) (gen.SignOutMyDeviceResponseObject, error) {
	session, svc, err := s.accountSession(ctx)
	if err != nil {
		return nil, err
	}
	// The streams of the device's sessions end first, as at logout: playback revokes their stream
	// tokens and stops their HLS output, the listen sessions of the radio end with them.
	end := func(ctx context.Context, ids []int64) error {
		for _, id := range ids {
			if s.opts.Playback != nil {
				if err := s.opts.Playback.EndSession(ctx, id); err != nil {
					return err
				}
			}
			if s.opts.Radio != nil {
				if err := s.opts.Radio.EndSession(ctx, id, model.StoppedByLogout); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := svc.SignOutMyDevice(ctx, session, req.DeviceId, end); err != nil {
		return s.problem(ctx, err), nil
	}
	own := session.DeviceID != "" && req.DeviceId == session.DeviceID
	return signedOutDevice{r: requestFrom(ctx), clear: own && !session.IsBearer()}, nil
}
