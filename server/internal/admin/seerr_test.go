package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The Seerr connection and the wish settings (concept 25, data model 2.12, build step 1.3).

const fakeSeerrKey = "seerr-FAKE-key-0123456789abcd"

type fakeSeerrHooks struct {
	saved, wishSaved int
	refuse           error
	titleWishes      map[model.TitleKey]model.WishMark
	canRequest       bool
	askedTitles      []model.TitleKey
}

func (h *fakeSeerrHooks) CanEnableWishes(context.Context) error { return h.refuse }
func (h *fakeSeerrHooks) SeerrSaved(context.Context)            { h.saved++ }
func (h *fakeSeerrHooks) WishSettingsSaved(context.Context, model.WishSettings, model.WishSettings) {
	h.wishSaved++
}
func (h *fakeSeerrHooks) IntegrationStatus(context.Context) (model.IntegrationStatus, error) {
	return model.IntegrationStatus{Key: "seerr", State: model.IntegrationMissing}, nil
}
func (h *fakeSeerrHooks) DashboardWishes(context.Context) (*model.DashboardWishes, error) {
	return nil, nil
}
func (h *fakeSeerrHooks) SpotlightWishes(_ context.Context, titles []model.TitleKey) (map[model.TitleKey]model.WishMark, bool, error) {
	h.askedTitles = append(h.askedTitles, titles...)
	return h.titleWishes, h.canRequest, nil
}

func rawSetting(t *testing.T, f *fixture, key string) (string, bool) {
	t.Helper()
	var v string
	err := f.db.Read.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v, true
}

// The key is stored encrypted, shows its last four characters, an empty key keeps it, and
// SeerrConfig hands the clear key to package wishes; saving calls the hook.
func TestSeerrSettingsStoreTheKeyEncrypted(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	hooks := &fakeSeerrHooks{}
	f.svc.WithSeerr(hooks)
	st, err := f.svc.SeerrSettings(ctx)
	if err != nil || st.Enabled || st.URL != "" || st.URLSource != model.SourceNone || st.Key.Source != model.SourceNone {
		t.Fatalf("fresh server: %+v, %v", st, err)
	}
	st, err = f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{
		Enabled: ptr(true), URL: ptr(" http://192.168.1.20:5055/api/v1/ "), APIKey: ptr(fakeSeerrKey)})
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled || st.URL != "http://192.168.1.20:5055" || st.URLSource != model.SourceStored ||
		st.Key.Source != model.SourceStored || st.Key.Hint != "abcd" {
		t.Errorf("after save: %+v", st)
	}
	raw, ok := rawSetting(t, f, SettingSeerrAPIKey)
	if !ok || !strings.HasPrefix(raw, "v1:") || strings.Contains(raw, fakeSeerrKey) {
		t.Errorf("stored key %q is not encrypted", raw)
	}
	if strings.Contains(f.logs.String(), fakeSeerrKey) {
		t.Error("the key reached the log")
	}
	if hooks.saved != 1 {
		t.Errorf("hook SeerrSaved called %d times, want 1", hooks.saved)
	}
	// An empty key keeps the stored one.
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{APIKey: ptr("")}); err != nil {
		t.Fatal(err)
	}
	cfg, err := f.svc.SeerrConfig(ctx)
	if err != nil || cfg != (model.SeerrConfig{Enabled: true, URL: "http://192.168.1.20:5055", APIKey: fakeSeerrKey}) || !cfg.Configured() {
		t.Errorf("config = %+v, %v", cfg, err)
	}
	// Clear removes address and key.
	if st, err = f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{Clear: true}); err != nil || st.URL != "" || st.Key.Source != model.SourceNone {
		t.Errorf("after clear: %+v, %v", st, err)
	}
}

// Addresses: http and https only, no user info, no query; a new host needs the key again, the same
// host does not; a new address or key voids the last connection test.
func TestSeerrSettingsAddressRules(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, bad := range []string{"ftp://seerr.lan", "http://user:pw@seerr.lan", "http://seerr.lan/?a=1", "seerr.lan", "http://"} {
		_, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr(bad), APIKey: ptr(fakeSeerrKey)})
		var verr *model.ValidationError
		if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0] != (model.FieldError{Field: "url", Code: model.CodeSeerrURLInvalid}) {
			t.Errorf("%q: %v, want url %s", bad, err, model.CodeSeerrURLInvalid)
		}
		if strings.Contains(err.Error(), "pw@") {
			t.Errorf("%q: the error quotes the address: %v", bad, err)
		}
	}
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr("http://seerr.lan:5055"), APIKey: ptr(fakeSeerrKey)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr("http://other.lan:5055")}); !errors.Is(err, model.ErrSeerrKeyRequired) {
		t.Errorf("new host without key: %v, want ErrSeerrKeyRequired", err)
	}
	// The same host on another scheme would send the stored key in clear text (concept 25, L-5).
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr("https://seerr.lan:5055")}); !errors.Is(err, model.ErrSeerrKeyRequired) {
		t.Errorf("new scheme without key: %v, want ErrSeerrKeyRequired", err)
	}
	if st, _ := f.svc.SeerrSettings(ctx); st.URL != "http://seerr.lan:5055" {
		t.Errorf("a refused save changed the address to %q", st.URL)
	}
	execSeerr(t, f, `INSERT INTO settings (key, value, updated_at) VALUES ('seerr.test', '{"ok":true}', 1)`)
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{Enabled: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawSetting(t, f, "seerr.test"); !ok {
		t.Error("switching Seerr on voided the test")
	}
	// Same host, other path: no key needed, but the test is void.
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr("http://SEERR.lan:5055/seerr")}); err != nil {
		t.Fatalf("same host: %v", err)
	}
	if _, ok := rawSetting(t, f, "seerr.test"); ok {
		t.Error("a new address kept the test")
	}
	execSeerr(t, f, `INSERT INTO settings (key, value, updated_at) VALUES ('seerr.test', '{"ok":true}', 1)`)
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{URL: ptr("http://other.lan:5055"), APIKey: ptr(fakeSeerrKey + "x")}); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawSetting(t, f, "seerr.test"); ok {
		t.Error("a new key kept the test")
	}
}

// The environment wins over the stored address and key and cannot be changed here.
func TestSeerrSettingsEnvironmentWins(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.env[EnvSeerrURL] = "http://env-seerr.lan:5055"
	f.env[EnvSeerrAPIKey] = "env-FAKE-seerr-key-123456"
	st, err := f.svc.SeerrSettings(ctx)
	if err != nil || st.URL != "http://env-seerr.lan:5055" || st.URLSource != model.SourceEnv || st.Key.Source != model.SourceEnv || st.Key.Hint != "" {
		t.Errorf("settings = %+v, %v", st, err)
	}
	for name, upd := range map[string]model.SeerrSettingsUpdate{
		"url":   {URL: ptr("http://seerr.lan")},
		"key":   {APIKey: ptr(fakeSeerrKey)},
		"clear": {Clear: true},
	} {
		if _, err := f.svc.UpdateSeerrSettings(ctx, upd); !errors.Is(err, model.ErrEnvWins) {
			t.Errorf("%s: %v, want ErrEnvWins", name, err)
		}
	}
	if _, err := f.svc.UpdateSeerrSettings(ctx, model.SeerrSettingsUpdate{Enabled: ptr(true)}); err != nil {
		t.Errorf("the switch stays changeable: %v", err)
	}
	cfg, err := f.svc.SeerrConfig(ctx)
	if err != nil || cfg.APIKey != "env-FAKE-seerr-key-123456" || !cfg.Enabled {
		t.Errorf("config = %v, %v", cfg.Enabled, err)
	}
}

// The wish settings of a new server, their validation, and «Wünsche aktiv» only after a successful
// test (T-1).
func TestSeerrWishSettings(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	ws, err := f.svc.WishSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ws != DefaultWishSettings || ws.Enabled || ws.Unmapped != model.WishesUnmappedFallback ||
		ws.DefaultLimit != (model.WishLimits{Movie: model.WishLimit{Limit: 10, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}}) ||
		!ws.ProvisionAccounts || !ws.SyncPassword || ws.NewAccountsCanRequest {
		t.Errorf("defaults = %+v", ws)
	}
	for name, upd := range map[string]model.WishSettingsUpdate{
		"unmapped":  {Unmapped: ptr("shared")},
		"already":   {AlreadyRequested: ptr("ignore")},
		"limit":     {DefaultLimit: &model.WishLimits{Movie: model.WishLimit{Limit: 101, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}}},
		"days zero": {DefaultLimit: &model.WishLimits{Movie: model.WishLimit{Limit: 1, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 0}}},
		"days high": {DefaultLimit: &model.WishLimits{Movie: model.WishLimit{Limit: 1, Days: 91}, TV: model.WishLimit{Limit: 5, Days: 7}}},
	} {
		var verr *model.ValidationError
		if _, err := f.svc.SetWishSettings(ctx, upd); !errors.As(err, &verr) {
			t.Errorf("%s: %v, want a validation error", name, err)
		}
	}
	ws, err = f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{
		Unmapped: ptr(model.WishesUnmappedDeny), SyncPassword: ptr(false), NewAccountsCanRequest: ptr(true),
		DefaultLimit: &model.WishLimits{Movie: model.WishLimit{Limit: 0, Days: 1}, TV: model.WishLimit{Limit: 100, Days: 90}}})
	if err != nil {
		t.Fatal(err)
	}
	if ws.Unmapped != model.WishesUnmappedDeny || ws.SyncPassword || !ws.NewAccountsCanRequest || ws.DefaultLimit.TV.Days != 90 || ws.DefaultLimit.Movie.Limit != 0 {
		t.Errorf("after set: %+v", ws)
	}
	// Without package wishes nothing can prove a test: «Wünsche aktiv» stays off.
	if _, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{Enabled: ptr(true)}); !errors.Is(err, model.ErrSeerrNotTested) {
		t.Errorf("enable without hooks: %v, want ErrSeerrNotTested", err)
	}
	hooks := &fakeSeerrHooks{refuse: model.ErrSeerrNotTested}
	f.svc.WithSeerr(hooks)
	if _, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{Enabled: ptr(true), AllowShows: ptr(false)}); !errors.Is(err, model.ErrSeerrNotTested) {
		t.Errorf("enable without a test: %v, want ErrSeerrNotTested", err)
	}
	if ws, _ := f.svc.WishSettings(ctx); ws.Enabled || !ws.AllowShows {
		t.Errorf("a refused request changed the settings: %+v", ws)
	}
	hooks.refuse = nil
	if ws, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{Enabled: ptr(true)}); err != nil || !ws.Enabled {
		t.Errorf("enable after a test: %+v, %v", ws, err)
	}
	// Switching off needs no test; garbage in the table falls back to the default.
	hooks.refuse = model.ErrSeerrNotTested
	if ws, err := f.svc.SetWishSettings(ctx, model.WishSettingsUpdate{Enabled: ptr(false)}); err != nil || ws.Enabled {
		t.Errorf("disable: %+v, %v", ws, err)
	}
	execSeerr(t, f, `UPDATE settings SET value = 'x' WHERE key = 'wishes.limit_movie_days'`)
	if ws, _ := f.svc.WishSettings(ctx); ws.DefaultLimit.Movie.Days != 7 {
		t.Errorf("unreadable days = %d, want the default 7", ws.DefaultLimit.Movie.Days)
	}
}

func execSeerr(t *testing.T, f *fixture, query string) {
	t.Helper()
	if _, err := f.db.Write.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}
