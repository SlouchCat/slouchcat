package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 settings and integrations of the main user (plan M8, A-2, A-6; design section 1): the public
// URL, the update check, the provider keys, the e-mail server, the Telegram bot and the Web Push key
// pair. Secrets only travel into the service; the answers carry where a value comes from, never the
// value itself.

func (e serviceProblem) VisitGetAdminSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitUpdateAdminSettingsResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitSendTestEmailResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitRotatePushKeysResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetAdminSettings(ctx context.Context, _ gen.GetAdminSettingsRequestObject) (gen.GetAdminSettingsResponseObject, error) {
	st, err := s.opts.Admin.Settings(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetAdminSettings200JSONResponse(toAdminSettings(st)), nil
}

func (s *server) UpdateAdminSettings(ctx context.Context, req gen.UpdateAdminSettingsRequestObject) (gen.UpdateAdminSettingsResponseObject, error) {
	b := deref(req.Body)
	upd := model.AdminSettingsUpdate{PublicURL: b.PublicUrl, UpdateCheckEnabled: b.UpdateCheckEnabled}
	if k := b.Keys; k != nil {
		upd.Keys = model.AdminKeysUpdate{TMDB: k.Tmdb, TVDB: k.Tvdb, TVDBPin: k.TvdbPin, YouTube: k.Youtube, Trakt: k.Trakt}
	}
	if e := b.Email; e != nil {
		upd.Email = &model.EmailSettingsUpdate{
			Host: e.Host, Port: e.Port, User: deref(e.User), Password: e.Password, From: e.From, TLS: string(e.Tls),
		}
	}
	if t := b.Telegram; t != nil {
		upd.Telegram = &model.TelegramSettingsUpdate{BotToken: deref(t.BotToken)}
	}
	for _, c := range deref(b.Clear) {
		upd.Clear = append(upd.Clear, string(c))
	}
	st, err := s.opts.Admin.UpdateSettings(ctx, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateAdminSettings200JSONResponse(toAdminSettings(st)), nil
}

func (s *server) SendTestEmail(ctx context.Context, req gen.SendTestEmailRequestObject) (gen.SendTestEmailResponseObject, error) {
	var to string
	if req.Body != nil && req.Body.To != nil {
		to = string(*req.Body.To)
	}
	if err := s.opts.Admin.SendTestEmail(model.WithClientIP(ctx, s.clientAddr(ctx)), to); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.SendTestEmail204Response{}, nil
}

func (s *server) RotatePushKeys(ctx context.Context, _ gen.RotatePushKeysRequestObject) (gen.RotatePushKeysResponseObject, error) {
	st, err := s.opts.Admin.RotatePushKeys(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RotatePushKeys200JSONResponse(toAdminSettings(st)), nil
}

func toAdminSettings(st model.AdminSettings) gen.AdminSettings {
	out := gen.AdminSettings{
		PublicUrl: optString(st.PublicURL), PublicUrlSource: settingSource(st.PublicURLSource),
		UpdateCheckEnabled: st.UpdateCheckEnabled,
		Keys: gen.AdminSettingsKeys{
			Tmdb: toKeyState(st.Keys.TMDB), Tvdb: toKeyState(st.Keys.TVDB), TvdbPin: toKeyState(st.Keys.TVDBPin),
			Youtube: toKeyState(st.Keys.YouTube), Trakt: toKeyState(st.Keys.Trakt),
		},
		Email: gen.AdminSettingsEmail{
			Source: settingSource(st.Email.Source), Host: optString(st.Email.Host), Port: optPositive(st.Email.Port),
			User: optString(st.Email.User), From: optString(st.Email.From),
		},
		Telegram: gen.AdminSettingsTelegram{Source: settingSource(st.Telegram.Source), BotUsername: optString(st.Telegram.BotUsername)},
		Push:     gen.AdminSettingsPush{Configured: st.Push.Configured, PublicKey: optString(st.Push.PublicKey)},
	}
	if st.Email.TLS != "" {
		tls := gen.SMTPTLS(st.Email.TLS)
		out.Email.Tls = &tls
	}
	return out
}

func toKeyState(k model.KeyState) gen.KeyState {
	return gen.KeyState{Source: settingSource(k.Source), Hint: optString(k.Hint)}
}

// settingSource maps a source; empty reads as none.
func settingSource(source string) gen.SettingSource {
	if source == "" {
		return gen.SettingSourceNone
	}
	return gen.SettingSource(source)
}
