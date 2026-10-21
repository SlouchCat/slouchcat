package api

import (
	"context"
	"net/http"

	"github.com/oapi-codegen/runtime/types"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 notification channels of the signed-in account (plan 5.5, A-6, E-8; design sections 1 and
// 3.2): e-mail, Web Push and Telegram with their events, the Telegram link, the push subscriptions
// of the browsers and a test message per channel. All of them are account operations (any session
// of the account, profile optional) through the notify service (Options.Notify; nil answers 501);
// the service lets only a session without a profile or with the primary profile change them
// (403 primary_profile_required). Endpoint and keys of a push subscription only travel into the
// service.

func (e serviceProblem) VisitGetMyChannelsResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitUpdateMyChannelResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitTestMyChannelResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitCreateTelegramLinkResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeleteTelegramLinkResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetPushPublicKeyResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCreatePushSubscriptionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitDeletePushSubscriptionResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

func (s *server) GetMyChannels(ctx context.Context, _ gen.GetMyChannelsRequestObject) (gen.GetMyChannelsResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	list, err := s.opts.Notify.Channels(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.ChannelSettings{Channels: make([]gen.ChannelSetting, 0, len(list))}
	for _, c := range list {
		out.Channels = append(out.Channels, toChannelSetting(c))
	}
	return gen.GetMyChannels200JSONResponse(out), nil
}

func (s *server) UpdateMyChannel(ctx context.Context, req gen.UpdateMyChannelRequestObject) (gen.UpdateMyChannelResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	b := deref(req.Body)
	upd := model.ChannelSettingUpdate{Enabled: b.Enabled, Events: make([]string, 0, len(b.Events))}
	for _, e := range b.Events {
		upd.Events = append(upd.Events, string(e))
	}
	if b.Email != nil {
		address := string(b.Email.Address)
		upd.EmailAddress = &address
	}
	c, err := s.opts.Notify.UpdateChannel(ctx, session, string(req.Channel), upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateMyChannel200JSONResponse(toChannelSetting(c)), nil
}

func (s *server) TestMyChannel(ctx context.Context, req gen.TestMyChannelRequestObject) (gen.TestMyChannelResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.TestChannel(model.WithClientIP(ctx, s.clientAddr(ctx)), session, string(req.Channel)); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.TestMyChannel204Response{}, nil
}

func (s *server) CreateTelegramLink(ctx context.Context, _ gen.CreateTelegramLinkRequestObject) (gen.CreateTelegramLinkResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	c, err := s.opts.Notify.CreateTelegramLink(ctx, session)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateTelegramLink200JSONResponse(toChannelSetting(c)), nil
}

func (s *server) DeleteTelegramLink(ctx context.Context, _ gen.DeleteTelegramLinkRequestObject) (gen.DeleteTelegramLinkResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.DeleteTelegramLink(ctx, session); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteTelegramLink204Response{}, nil
}

func (s *server) GetPushPublicKey(ctx context.Context, _ gen.GetPushPublicKeyRequestObject) (gen.GetPushPublicKeyResponseObject, error) {
	_, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	key, err := s.opts.Notify.PushPublicKey(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetPushPublicKey200JSONResponse{PublicKey: key}, nil
}

func (s *server) CreatePushSubscription(ctx context.Context, req gen.CreatePushSubscriptionRequestObject) (gen.CreatePushSubscriptionResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	b := deref(req.Body)
	sub, err := s.opts.Notify.CreatePushSubscription(ctx, session, model.PushSubscriptionInput{
		Endpoint: deref(b.Endpoint), P256dh: deref(b.Keys.P256dh), Auth: deref(b.Keys.Auth),
		DeviceName: deref(b.DeviceName), UserAgent: userAgent(ctx),
	})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreatePushSubscription201JSONResponse(toPushSubscription(sub)), nil
}

func (s *server) DeletePushSubscription(ctx context.Context, req gen.DeletePushSubscriptionRequestObject) (gen.DeletePushSubscriptionResponseObject, error) {
	session, ok, err := s.notifySession(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	if err := s.opts.Notify.DeletePushSubscription(ctx, session, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeletePushSubscription204Response{}, nil
}

func toChannelSetting(c model.ChannelSetting) gen.ChannelSetting {
	out := gen.ChannelSetting{
		Channel: gen.Channel(c.Channel), Enabled: c.Enabled, Available: c.Available, Events: make([]gen.EventType, 0, len(c.Events)),
	}
	for _, e := range c.Events {
		out.Events = append(out.Events, gen.EventType(e))
	}
	if c.Email != nil {
		out.Email = &gen.EmailChannel{Address: types.Email(c.Email.Address)}
	}
	if t := c.Telegram; t != nil {
		out.Telegram = &gen.TelegramChannel{
			Linked: t.Linked, ChatId: optString(t.ChatID), LinkCode: optString(t.LinkCode), LinkExpiresAt: utc(t.LinkExpiresAt),
			BotUsername: optString(t.BotUsername),
		}
	}
	if p := c.Push; p != nil {
		out.Push = &gen.PushChannel{Subscriptions: make([]gen.PushSubscriptionSummary, 0, len(p.Subscriptions))}
		for _, sub := range p.Subscriptions {
			out.Push.Subscriptions = append(out.Push.Subscriptions, toPushSubscription(sub))
		}
	}
	return out
}

func toPushSubscription(sub model.PushSubscription) gen.PushSubscriptionSummary {
	return gen.PushSubscriptionSummary{
		Id: sub.ID, DeviceName: optString(sub.DeviceName), CreatedAt: sub.CreatedAt.UTC(), ThisDevice: sub.ThisDevice,
	}
}
