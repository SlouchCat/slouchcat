package admin

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Seerr and the wishes (concept 25, data model 2.12, architecture 4.2): the connection under
// Integrationen › Seerr and the settings under Medien › Wünsche › Einstellungen. The api key is
// stored encrypted (secrets, "v1:"), never leaves the server and shows only its last four
// characters; the environment wins over the stored address and key (model.EnvWinsError). Package
// wishes reads the values through SeerrConfig and WishSettings and keeps its own state keys
// (seerr.version, seerr.instance, seerr.test, seerr.last_*).

// Settings keys of the Seerr connection and the wishes.
const (
	SettingSeerrEnabled = "seerr.enabled"
	SettingSeerrURL     = "seerr.url"
	SettingSeerrAPIKey  = "seerr.api_key" //nolint:gosec // G101: the name of a setting, encrypted at rest

	// EnvSeerrURL and EnvSeerrAPIKey override the stored address and key (architecture 4.2).
	EnvSeerrURL    = "SLOUCHCAT_SEERR_URL"
	EnvSeerrAPIKey = "SLOUCHCAT_SEERR_API_KEY" //nolint:gosec // G101: the name of a variable, not a credential

	SettingWishesEnabled               = "wishes.enabled"
	SettingWishesAllowMovies           = "wishes.allow_movies"
	SettingWishesAllowShows            = "wishes.allow_shows"
	SettingWishesFromSearch            = "wishes.from_search"
	SettingWishesFromDetail            = "wishes.from_detail"
	SettingWishesAlreadyRequested      = "wishes.already_requested"
	SettingWishesUnmapped              = "wishes.unmapped"
	SettingWishesNewAccountsCanRequest = "wishes.new_accounts_can_request"
	SettingWishesAllowWithdraw         = "wishes.allow_withdraw"
	SettingWishesPINExternalHideTitle  = "wishes.pin_external_hide_title"
	SettingWishesProvisionAccounts     = "wishes.provision_accounts"
	SettingWishesSyncPassword          = "wishes.sync_password"
	SettingWishesLimitMovieCount       = "wishes.limit_movie_count"
	SettingWishesLimitMovieDays        = "wishes.limit_movie_days"
	SettingWishesLimitTVCount          = "wishes.limit_tv_count"
	SettingWishesLimitTVDays           = "wishes.limit_tv_days"

	// settingSeerrTest is the record of the last connection test, written by package wishes (T-1).
	// A new address or key makes it void, so it goes in the same transaction.
	settingSeerrTest = "seerr.test"

	maxSeerrURL    = 2048
	maxSeerrAPIKey = 1024
	// Limits per period (data model 2.13): counts 0 to 100 (0 = no limit), days 1 to 90.
	maxWishLimit = 100
	minWishDays  = 1
	maxWishDays  = 90
)

// DefaultWishSettings are the settings of a new server (data model 2.12; F-4, F-7, F-15, F-17,
// F-26, N-8): wishes off, the shared user for accounts without a Seerr user, 10 films and 5 seasons
// in 7 days.
var DefaultWishSettings = model.WishSettings{
	AllowMovies: true, AllowShows: true, FromSearch: true, FromDetail: true,
	AlreadyRequested: model.WishesAlreadyJoin, Unmapped: model.WishesUnmappedFallback,
	AllowWithdraw: true, PINExternalHideTitle: true, ProvisionAccounts: true, SyncPassword: true,
	DefaultLimit: model.WishLimits{Movie: model.WishLimit{Limit: 10, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}},
}

// SeerrHooks is implemented by package wishes.
type SeerrHooks interface {
	// CanEnableWishes returns nil when «Wünsche aktiv» may be switched on: the last connection test
	// of the saved address and key passed (T-1); else model.ErrSeerrNotTested.
	CanEnableWishes(ctx context.Context) error
	// SeerrSaved is called after the connection was saved: it queues the refresh of the Seerr users.
	SeerrSaved(ctx context.Context)
	// WishSettingsSaved is called after the wish settings were saved, with the settings before and
	// after: «Wünsche aktiv» switched on queues the shared user, a new default limit is written.
	WishSettingsSaved(ctx context.Context, before, after model.WishSettings)
}

// WithSeerr sets the hooks of package wishes and returns the receiver.
func (s *Service) WithSeerr(h SeerrHooks) *Service {
	s.seerr = h
	return s
}

// SeerrSettings returns the connection without its key.
func (s *Service) SeerrSettings(ctx context.Context) (model.SeerrSettings, error) {
	enabled, err := s.boolSetting(ctx, SettingSeerrEnabled, false)
	if err != nil {
		return model.SeerrSettings{}, err
	}
	st := model.SeerrSettings{Enabled: enabled, URLSource: model.SourceNone}
	if env := strings.TrimSpace(s.getenv(EnvSeerrURL)); env != "" {
		st.URL, st.URLSource = env, model.SourceEnv
	} else if v, ok, err := s.setting(ctx, SettingSeerrURL); err != nil {
		return model.SeerrSettings{}, err
	} else if ok {
		st.URL, st.URLSource = v, model.SourceStored
	}
	if st.Key, err = s.keyState(ctx, providerKey{field: "apiKey", setting: SettingSeerrAPIKey, env: EnvSeerrAPIKey}); err != nil {
		return model.SeerrSettings{}, err
	}
	return st, nil
}

// SeerrConfig returns the address and the key package wishes calls Seerr with; the environment
// wins over the stored values. The key never reaches the log.
func (s *Service) SeerrConfig(ctx context.Context) (model.SeerrConfig, error) {
	st, err := s.SeerrSettings(ctx)
	if err != nil {
		return model.SeerrConfig{}, err
	}
	key, err := s.secretKey(ctx, EnvSeerrAPIKey, SettingSeerrAPIKey)
	if err != nil {
		return model.SeerrConfig{}, err
	}
	return model.SeerrConfig{Enabled: st.Enabled, URL: st.URL, APIKey: key}, nil
}

// UpdateSeerrSettings checks the whole request and stores it in one transaction. The address must
// be http or https without user info, query or fragment (model.CodeSeerrURLInvalid). A new host
// needs the key again (model.ErrSeerrKeyRequired): the stored key never goes to another address.
// A value of the environment cannot be changed (model.EnvWinsError). A new address or key voids
// the last connection test. Afterwards package wishes queues the refresh of the Seerr users.
func (s *Service) UpdateSeerrSettings(ctx context.Context, upd model.SeerrSettingsUpdate) (model.SeerrSettings, error) {
	cur, err := s.SeerrSettings(ctx)
	if err != nil {
		return model.SeerrSettings{}, err
	}
	verr := &model.ValidationError{}
	var newURL, newKey string
	if upd.URL != nil {
		if newURL, err = normalizeSeerrURL(*upd.URL); err != nil {
			verr = verr.Add("url", model.CodeSeerrURLInvalid)
		}
	}
	if upd.APIKey != nil {
		newKey = strings.TrimSpace(*upd.APIKey)
		if utf8.RuneCountInString(newKey) > maxSeerrAPIKey {
			verr = verr.Add("apiKey", "too_long")
		}
	}
	if len(verr.Fields) > 0 {
		return model.SeerrSettings{}, verr
	}
	urlChanged := upd.URL != nil && newURL != cur.URL
	switch {
	case urlChanged && cur.URLSource == model.SourceEnv:
		return model.SeerrSettings{}, &model.EnvWinsError{Field: "url"}
	case newKey != "" && cur.Key.Source == model.SourceEnv:
		return model.SeerrSettings{}, &model.EnvWinsError{Field: "apiKey"}
	case upd.Clear && (cur.URLSource == model.SourceEnv || cur.Key.Source == model.SourceEnv):
		return model.SeerrSettings{}, &model.EnvWinsError{Field: "url"}
	}
	// A stored key never travels to another host: a new host needs the key again.
	if urlChanged && newURL != "" && newKey == "" && cur.Key.Source == model.SourceStored && !sameSeerrHost(cur.URL, newURL) {
		return model.SeerrSettings{}, model.ErrSeerrKeyRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.SeerrSettings{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	w := settingsWriter{s: s, ctx: ctx, q: appq.New(tx), now: s.now().UnixMilli()}
	if upd.Enabled != nil {
		w.set(SettingSeerrEnabled, boolValue(*upd.Enabled))
	}
	switch {
	case upd.Clear:
		w.del(SettingSeerrURL)
		w.del(SettingSeerrAPIKey)
		w.del(settingSeerrTest)
	default:
		if urlChanged {
			w.setOrDel(SettingSeerrURL, newURL)
		}
		if newKey != "" {
			w.setSecret(SettingSeerrAPIKey, newKey)
		}
		if urlChanged || newKey != "" {
			w.del(settingSeerrTest)
		}
	}
	if w.err != nil {
		return model.SeerrSettings{}, w.err
	}
	if err := tx.Commit(); err != nil {
		return model.SeerrSettings{}, fmt.Errorf("commit seerr settings: %w", err)
	}
	s.log.InfoContext(ctx, "seerr settings saved", "url_changed", urlChanged, "key_changed", newKey != "", "cleared", upd.Clear)
	if s.seerr != nil {
		s.seerr.SeerrSaved(ctx)
	}
	return s.SeerrSettings(ctx)
}

// normalizeSeerrURL checks an address of Seerr; "" clears it.
func normalizeSeerrURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxSeerrURL {
		return "", seerr.ErrInvalid
	}
	return seerr.NormalizeAddress(raw)
}

// sameSeerrHost reports whether two addresses name the same host and port.
func sameSeerrHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	return errA == nil && errB == nil && strings.EqualFold(ua.Host, ub.Host)
}

// WishSettings returns the settings of the wishes; absent keys have their default.
func (s *Service) WishSettings(ctx context.Context) (model.WishSettings, error) {
	d := DefaultWishSettings
	out := d
	bools := []struct {
		key string
		dst *bool
		def bool
	}{
		{SettingWishesEnabled, &out.Enabled, d.Enabled}, {SettingWishesAllowMovies, &out.AllowMovies, d.AllowMovies},
		{SettingWishesAllowShows, &out.AllowShows, d.AllowShows}, {SettingWishesFromSearch, &out.FromSearch, d.FromSearch},
		{SettingWishesFromDetail, &out.FromDetail, d.FromDetail},
		{SettingWishesNewAccountsCanRequest, &out.NewAccountsCanRequest, d.NewAccountsCanRequest},
		{SettingWishesAllowWithdraw, &out.AllowWithdraw, d.AllowWithdraw},
		{SettingWishesPINExternalHideTitle, &out.PINExternalHideTitle, d.PINExternalHideTitle},
		{SettingWishesProvisionAccounts, &out.ProvisionAccounts, d.ProvisionAccounts},
		{SettingWishesSyncPassword, &out.SyncPassword, d.SyncPassword},
	}
	for _, b := range bools {
		v, err := s.boolSetting(ctx, b.key, b.def)
		if err != nil {
			return model.WishSettings{}, err
		}
		*b.dst = v
	}
	for _, e := range []struct {
		key   string
		dst   *string
		valid []string
	}{
		{SettingWishesAlreadyRequested, &out.AlreadyRequested, []string{model.WishesAlreadyJoin, model.WishesAlreadyStatus}},
		{SettingWishesUnmapped, &out.Unmapped, []string{model.WishesUnmappedFallback, model.WishesUnmappedDeny}},
	} {
		v, ok, err := s.setting(ctx, e.key)
		if err != nil {
			return model.WishSettings{}, err
		}
		if ok && slices.Contains(e.valid, v) {
			*e.dst = v
		}
	}
	for _, n := range []struct {
		key      string
		dst      *int
		min, max int
	}{
		{SettingWishesLimitMovieCount, &out.DefaultLimit.Movie.Limit, 0, maxWishLimit},
		{SettingWishesLimitMovieDays, &out.DefaultLimit.Movie.Days, minWishDays, maxWishDays},
		{SettingWishesLimitTVCount, &out.DefaultLimit.TV.Limit, 0, maxWishLimit},
		{SettingWishesLimitTVDays, &out.DefaultLimit.TV.Days, minWishDays, maxWishDays},
	} {
		v, ok, err := s.setting(ctx, n.key)
		if err != nil {
			return model.WishSettings{}, err
		}
		if i, err := strconv.Atoi(v); ok && err == nil && i >= n.min && i <= n.max {
			*n.dst = i
		}
	}
	return out, nil
}

// SetWishSettings checks the whole request and stores it in one transaction; every change applies
// at once. Switching «Wünsche aktiv» on needs a successful connection test (T-1,
// model.ErrSeerrNotTested).
func (s *Service) SetWishSettings(ctx context.Context, upd model.WishSettingsUpdate) (model.WishSettings, error) {
	cur, err := s.WishSettings(ctx)
	if err != nil {
		return model.WishSettings{}, err
	}
	verr := &model.ValidationError{}
	if v := upd.AlreadyRequested; v != nil && *v != model.WishesAlreadyJoin && *v != model.WishesAlreadyStatus {
		verr = verr.Add("alreadyRequested", "invalid_value")
	}
	if v := upd.Unmapped; v != nil && *v != model.WishesUnmappedFallback && *v != model.WishesUnmappedDeny {
		verr = verr.Add("unmapped", "invalid_value")
	}
	if l := upd.DefaultLimit; l != nil {
		checkWishLimit(verr, "defaultLimit.movie", l.Movie)
		checkWishLimit(verr, "defaultLimit.tv", l.TV)
	}
	if len(verr.Fields) > 0 {
		return model.WishSettings{}, verr
	}
	if upd.Enabled != nil && *upd.Enabled && !cur.Enabled {
		if s.seerr == nil {
			return model.WishSettings{}, model.ErrSeerrNotTested
		}
		if err := s.seerr.CanEnableWishes(ctx); err != nil {
			return model.WishSettings{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.WishSettings{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	w := settingsWriter{s: s, ctx: ctx, q: appq.New(tx), now: s.now().UnixMilli()}
	for key, v := range map[string]*bool{
		SettingWishesEnabled: upd.Enabled, SettingWishesAllowMovies: upd.AllowMovies, SettingWishesAllowShows: upd.AllowShows,
		SettingWishesFromSearch: upd.FromSearch, SettingWishesFromDetail: upd.FromDetail,
		SettingWishesNewAccountsCanRequest: upd.NewAccountsCanRequest, SettingWishesAllowWithdraw: upd.AllowWithdraw,
		SettingWishesPINExternalHideTitle: upd.PINExternalHideTitle, SettingWishesProvisionAccounts: upd.ProvisionAccounts,
		SettingWishesSyncPassword: upd.SyncPassword,
	} {
		if v != nil {
			w.set(key, boolValue(*v))
		}
	}
	if v := upd.AlreadyRequested; v != nil {
		w.set(SettingWishesAlreadyRequested, *v)
	}
	if v := upd.Unmapped; v != nil {
		w.set(SettingWishesUnmapped, *v)
	}
	if l := upd.DefaultLimit; l != nil {
		w.set(SettingWishesLimitMovieCount, strconv.Itoa(l.Movie.Limit))
		w.set(SettingWishesLimitMovieDays, strconv.Itoa(l.Movie.Days))
		w.set(SettingWishesLimitTVCount, strconv.Itoa(l.TV.Limit))
		w.set(SettingWishesLimitTVDays, strconv.Itoa(l.TV.Days))
	}
	if w.err != nil {
		return model.WishSettings{}, w.err
	}
	if err := tx.Commit(); err != nil {
		return model.WishSettings{}, fmt.Errorf("commit wish settings: %w", err)
	}
	after, err := s.WishSettings(ctx)
	if err != nil {
		return model.WishSettings{}, err
	}
	if s.seerr != nil {
		s.seerr.WishSettingsSaved(ctx, cur, after)
	}
	return after, nil
}

func checkWishLimit(verr *model.ValidationError, field string, l model.WishLimit) {
	if l.Limit < 0 || l.Limit > maxWishLimit {
		_ = verr.Add(field+".limit", "out_of_range")
	}
	if l.Days < minWishDays || l.Days > maxWishDays {
		_ = verr.Add(field+".days", "out_of_range")
	}
}

// boolSetting reads a 0/1 setting; absent or unreadable is def.
func (s *Service) boolSetting(ctx context.Context, key string, def bool) (bool, error) {
	v, ok, err := s.setting(ctx, key)
	if err != nil || !ok {
		return def, err
	}
	switch v {
	case "1":
		return true, nil
	case "0":
		return false, nil
	}
	return def, nil
}

func boolValue(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
