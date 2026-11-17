package wishes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/arr"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
)

// Access to Radarr and Sonarr (concept 25 stage 4, architecture 20.2; build steps 4.6b and 4.6d).
// The profile tag and the download progress share it.
//
//   - Address and key come from Seerr (GET /settings/radarr|sonarr on the administrator's
//     session): the default server without 4K. An own address per service (arr.radarr_url,
//     arr.sonarr_url) replaces the address from Seerr; the key always comes from Seerr (F-30).
//   - The key lives in memory only (arr.KeyCache, 10 minutes; at once again after a 401, K-23).
//     It is never stored, logged or returned.
//   - Host confirmation: an address read from Seerr counts only after a successful «Testen»
//     (TestArrConnection) stored its scheme and host in arr.radarr_host resp. arr.sonarr_host.
//     Another host from Seerr later halts wishes.tag and wishes.progress until the administrator
//     tests again (ErrArrHostUnconfirmed). The own address is the administrator's own entry and
//     needs no confirmation.
//   - Rule 11: in Radarr and Sonarr Slouchcat only creates and attaches the profile tag and reads
//     the queue and the file state of wished titles (package arr has no other call).

// Settings keys of stage 4 (data model 2.12). The wishes.* switches are written by the
// administration's wish settings and read here with their defaults; the arr.* keys belong to this
// package (updateArrSettings, testArrConnection).
const (
	SettingProfileTags  = "wishes.profile_tags"  // 0/1, default 1: «Profil-Tag in Radarr und Sonarr setzen»
	SettingTagPrefix    = "wishes.tag_prefix"    // ^[a-z0-9-]{1,10}$, default "sc-" (N-21, F-25)
	SettingShowProgress = "wishes.show_progress" // 0/1, default 1: «Download-Fortschritt zeigen» (N-86)
	SettingRadarrURL    = "arr.radarr_url"       // own address; empty = the address from Seerr
	SettingSonarrURL    = "arr.sonarr_url"
	SettingRadarrHost   = "arr.radarr_host" // the confirmed scheme and host
	SettingSonarrHost   = "arr.sonarr_host"
	settingRadarrState  = "arr.radarr_state" // JSON arrRecord: the outcome of the last call
	settingSonarrState  = "arr.sonarr_state"
)

// DefaultTagPrefix is the prefix of the profile tag (N-21).
const DefaultTagPrefix = "sc-"

// Check keys of the connection test of Seerr (testSeerrConnection, data model 4.2, stage 4):
// «Radarr für Profil-Tags», «Sonarr für Profil-Tags». ArrTagChecks returns them.
const (
	CheckRadarrTags = "radarr_tags"
	CheckSonarrTags = "sonarr_tags"
)

// Sources of an address (ArrServiceStatus.Source).
const (
	ArrSourceSeerr = "seerr"
	ArrSourceOwn   = "own"
	ArrSourceNone  = "none"
)

// Errors of the access.
var (
	// ErrArrNotInSeerr: Seerr has no default server without 4K of this service.
	ErrArrNotInSeerr = errors.New("seerr has no default server of the service")
	// ErrArrHostUnconfirmed: the address from Seerr was never confirmed or points to another host
	// than the confirmed one; the jobs halt until «Testen» (architecture 20.2).
	ErrArrHostUnconfirmed = errors.New("address of the service not confirmed")
	// ErrUnknownService: a service other than radarr or sonarr (422 on the contract).
	ErrUnknownService = errors.New("unknown service")
)

// WithArrOptions adds options to every Radarr and Sonarr client (tests: a fast limiter).
func WithArrOptions(opts ...arr.Option) Option {
	return func(s *Service) { s.arr.opts = append(s.arr.opts, opts...) }
}

// arrServices holds the access to both services; its zero value is ready (made on first use).
type arrServices struct {
	opts   []arr.Option
	once   sync.Once
	radarr *arrAccess
	sonarr *arrAccess
}

// arrAccess is the access to one service.
type arrAccess struct {
	kind arr.Kind
	keys *arr.KeyCache

	mu          sync.Mutex
	client      *arr.Client
	clientOf    string // base and key of the cached client (memory only)
	tagRequests *bool  // «Tag Requests» of the server in Seerr, as last read
}

func (s *Service) arrOf(kind arr.Kind) *arrAccess {
	s.arr.once.Do(func() {
		for _, a := range []**arrAccess{&s.arr.radarr, &s.arr.sonarr} {
			k := arr.Radarr
			if a == &s.arr.sonarr {
				k = arr.Sonarr
			}
			acc := &arrAccess{kind: k}
			acc.keys = arr.NewKeyCache(func(ctx context.Context) (arr.Credentials, error) { return s.fetchArr(ctx, acc) },
				arr.WithKeyClock(s.now))
			*a = acc
		}
	})
	if kind == arr.Sonarr {
		return s.arr.sonarr
	}
	return s.arr.radarr
}

// fetchArr reads address and key of the default server without 4K from Seerr.
func (s *Service) fetchArr(ctx context.Context, a *arrAccess) (arr.Credentials, error) {
	admin, err := s.AdminSession(ctx)
	if err != nil {
		return arr.Credentials{}, err
	}
	list := admin.RadarrServers
	if a.kind == arr.Sonarr {
		list = admin.SonarrServers
	}
	servers, err := list(ctx)
	if err != nil {
		return arr.Credentials{}, err
	}
	var pick *seerr.DVRServer
	for i := range servers {
		if servers[i].IsDefault && !servers[i].Is4K {
			pick = &servers[i]
			break
		}
	}
	if pick == nil {
		return arr.Credentials{}, ErrArrNotInSeerr
	}
	base, err := dvrAddress(*pick)
	if err != nil {
		return arr.Credentials{}, ErrArrNotInSeerr
	}
	tr := pick.TagRequests
	a.mu.Lock()
	a.tagRequests = &tr
	a.mu.Unlock()
	return arr.Credentials{BaseURL: base, Key: arr.Secret(pick.APIKey.Reveal())}, nil
}

// dvrAddress forms the address of a server in Seerr: scheme from useSsl, hostname, port and the
// URL base.
func dvrAddress(d seerr.DVRServer) (string, error) {
	host := strings.TrimSpace(d.Hostname)
	if host == "" || strings.ContainsAny(host, "/@?#") {
		return "", errors.New("server in seerr has no usable host name")
	}
	scheme := "http"
	if d.UseSSL {
		scheme = "https"
	}
	if d.Port > 0 {
		host = net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(d.Port))
	}
	base := strings.TrimSpace(d.BaseURL)
	if base != "" && !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	u := scheme + "://" + host + strings.TrimRight(base, "/")
	if _, err := url.Parse(u); err != nil {
		return "", errors.New("server in seerr has no usable address")
	}
	return u, nil
}

func arrKeys(kind arr.Kind) (urlKey, hostKey, stateKey string) {
	if kind == arr.Sonarr {
		return SettingSonarrURL, SettingSonarrHost, settingSonarrState
	}
	return SettingRadarrURL, SettingRadarrHost, settingRadarrState
}

// arrTarget is where a client goes, without the key.
type arrTarget struct {
	source string // seerr or own
	base   string
	site   string // scheme and host
}

// arrClient returns a client of the service. With confirm the address from Seerr must be the
// confirmed host (ErrArrHostUnconfirmed otherwise); only TestArrConnection passes false.
func (s *Service) arrClient(ctx context.Context, kind arr.Kind, confirm bool) (*arr.Client, arrTarget, error) {
	a := s.arrOf(kind)
	creds, err := a.keys.Get(ctx)
	if err != nil {
		return nil, arrTarget{}, err
	}
	urlKey, hostKey, _ := arrKeys(kind)
	own, err := s.state(ctx, urlKey)
	if err != nil {
		return nil, arrTarget{}, err
	}
	t := arrTarget{source: ArrSourceSeerr, base: creds.BaseURL}
	if strings.TrimSpace(own) != "" {
		t = arrTarget{source: ArrSourceOwn, base: strings.TrimSpace(own)}
	}
	t.site = netguard.Site(t.base)
	if confirm && t.source == ArrSourceSeerr {
		confirmed, err := s.state(ctx, hostKey)
		if err != nil {
			return nil, arrTarget{}, err
		}
		if confirmed == "" || !strings.EqualFold(confirmed, t.site) {
			return nil, t, ErrArrHostUnconfirmed
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil && a.clientOf == t.base+"\x00"+creds.Key.Reveal() {
		return a.client, t, nil
	}
	opts := append([]arr.Option{arr.WithLogger(s.log)}, s.arr.opts...)
	c, err := arr.New(kind, t.base, creds.Key, opts...)
	if err != nil {
		return nil, t, err
	}
	a.client, a.clientOf = c, t.base+"\x00"+creds.Key.Reveal()
	return c, t, nil
}

// arrFailed handles an error of a call: after a refused key the next call reads the key from
// Seerr again (K-23).
func (s *Service) arrFailed(kind arr.Kind, err error) {
	if errors.Is(err, arr.ErrKeyInvalid) {
		s.arrOf(kind).keys.Invalidate()
	}
}

// arrErrorClass names an error of the access or of a call, without address or key.
func arrErrorClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrArrNotInSeerr):
		return "not_in_seerr"
	case errors.Is(err, ErrArrHostUnconfirmed):
		return "not_confirmed"
	case errors.Is(err, model.ErrSeerrNotConfigured), errors.Is(err, errDisabled):
		return "seerr_off"
	case errors.Is(err, arr.ErrRedirect):
		return "redirect"
	case errors.Is(err, context.DeadlineExceeded), isTimeout(err):
		return "timeout"
	case errors.Is(err, arr.ErrUnreachable):
		return "unreachable"
	case errors.Is(err, arr.ErrKeyInvalid):
		return "key_invalid"
	case errors.Is(err, arr.ErrNotFound):
		return "not_found"
	case errors.Is(err, arr.ErrValidation):
		return "validation"
	case errors.Is(err, arr.ErrServer):
		return "server"
	case errors.Is(err, arr.ErrBadRequest):
		return "bad_request"
	case errors.Is(err, arr.ErrInvalid):
		return "invalid"
	}
	var se *seerr.Error
	if errors.As(err, &se) {
		return "seerr_" + errorClass(err)
	}
	return "failed"
}

// serviceDown says whether an error of a call concerns the whole service (the remaining calls of
// the run would fail the same way).
func serviceDown(err error) bool {
	return errors.Is(err, arr.ErrUnreachable) || errors.Is(err, arr.ErrKeyInvalid) || errors.Is(err, arr.ErrServer) ||
		errors.Is(err, context.DeadlineExceeded)
}

// arrRecord is the outcome of the last call to a service (settings arr.*_state).
type arrRecord struct {
	At    int64  `json:"at"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"` // an error class
}

func (s *Service) recordArr(ctx context.Context, kind arr.Kind, err error) {
	_, _, key := arrKeys(kind)
	rec := arrRecord{At: s.now().UnixMilli(), OK: err == nil, Error: arrErrorClass(err)}
	if e := s.setStateJSON(ctx, s.write, key, rec); e != nil {
		s.log.WarnContext(ctx, "arr state not written", "service", string(kind), "err", e)
	}
}

// boolState reads a 0/1 setting; absent or unreadable is def.
func (s *Service) boolState(ctx context.Context, key string, def bool) (bool, error) {
	v, err := s.state(ctx, key)
	if err != nil {
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

// ParseArrService reads "radarr" or "sonarr"; ErrUnknownService otherwise.
func ParseArrService(v string) (arr.Kind, error) {
	switch arr.Kind(v) {
	case arr.Radarr, arr.Sonarr:
		return arr.Kind(v), nil
	}
	return "", ErrUnknownService
}

// ArrTestResult is the answer of testArrConnection. It never carries the key.
type ArrTestResult struct {
	Reachable   bool
	KeyValid    bool
	TagRequests *bool  // «Tag Requests» of the server in Seerr; nil with an own address unknown
	Host        string // scheme and host the test reached; confirmed when Reachable and KeyValid
	Source      string // seerr or own
	Error       string // error class when not both are true
}

// TestArrConnection tests the service (testArrConnection, «Testen»): it reads address and key
// from Seerr anew, asks GET /api/v3/tag and, when that answers, confirms the host
// (arr.radarr_host resp. arr.sonarr_host). Errors of Seerr come back as errors; errors of the
// service are in the result.
func (s *Service) TestArrConnection(ctx context.Context, kind arr.Kind) (ArrTestResult, error) {
	if _, err := ParseArrService(string(kind)); err != nil {
		return ArrTestResult{}, err
	}
	a := s.arrOf(kind)
	a.keys.Invalidate()
	c, t, err := s.arrClient(ctx, kind, false)
	if err != nil {
		s.recordArr(ctx, kind, err)
		if errors.Is(err, arr.ErrInvalid) {
			return ArrTestResult{Source: t.source, Host: t.site, Error: arrErrorClass(err)}, nil
		}
		return ArrTestResult{}, err
	}
	res := ArrTestResult{Host: t.site, Source: t.source}
	if t.source == ArrSourceSeerr {
		a.mu.Lock()
		res.TagRequests = a.tagRequests
		a.mu.Unlock()
	}
	_, err = c.Tags(ctx)
	s.arrFailed(kind, err)
	s.recordArr(ctx, kind, err)
	switch {
	case err == nil:
		res.Reachable, res.KeyValid = true, true
		_, hostKey, _ := arrKeys(kind)
		if err := s.setState(ctx, s.write, hostKey, t.site); err != nil {
			return ArrTestResult{}, err
		}
		s.log.InfoContext(ctx, "arr host confirmed", "service", string(kind), "source", t.source)
	case errors.Is(err, arr.ErrKeyInvalid):
		res.Reachable, res.Error = true, arrErrorClass(err)
	default:
		res.Error = arrErrorClass(err)
	}
	return res, nil
}

// ArrSettingsUpdate changes the own addresses (updateArrSettings): nil stays, "" goes back to the
// address from Seerr.
type ArrSettingsUpdate struct {
	RadarrURL, SonarrURL *string
}

// maxArrURL is the longest own address.
const maxArrURL = 500

// UpdateArrSettings stores the own addresses after checking them (http or https, a host, no user,
// query or fragment; model.ValidationError with radarrUrl resp. sonarrUrl otherwise) and returns
// the status.
func (s *Service) UpdateArrSettings(ctx context.Context, upd ArrSettingsUpdate) (ArrStatus, error) {
	var verr *model.ValidationError
	for _, f := range []struct {
		field string
		kind  arr.Kind
		v     *string
	}{{"radarrUrl", arr.Radarr, upd.RadarrURL}, {"sonarrUrl", arr.Sonarr, upd.SonarrURL}} {
		if f.v == nil || strings.TrimSpace(*f.v) == "" {
			continue
		}
		v := strings.TrimSpace(*f.v)
		if len(v) > maxArrURL {
			verr = verr.Add(f.field, "too_long")
			continue
		}
		// The client checks the address the same way it will use it; the key is a stand-in.
		if _, err := arr.New(f.kind, v, arr.Secret("check")); err != nil {
			verr = verr.Add(f.field, "invalid_value")
		}
	}
	if verr != nil {
		return ArrStatus{}, verr
	}
	for _, f := range []struct {
		key string
		v   *string
	}{{SettingRadarrURL, upd.RadarrURL}, {SettingSonarrURL, upd.SonarrURL}} {
		switch {
		case f.v == nil:
		case strings.TrimSpace(*f.v) == "":
			if err := s.delState(ctx, s.write, f.key); err != nil {
				return ArrStatus{}, err
			}
		default:
			if err := s.setState(ctx, s.write, f.key, strings.TrimSpace(*f.v)); err != nil {
				return ArrStatus{}, err
			}
		}
	}
	s.log.InfoContext(ctx, "arr settings saved", "radarr_changed", upd.RadarrURL != nil, "sonarr_changed", upd.SonarrURL != nil)
	return s.ArrStatus(ctx)
}

// TagCounts are the tag rows of a service per state.
type TagCounts struct {
	Set, Pending, Failed int
}

// ArrServiceStatus is the state of one service (ArrStatus of the contract). It never carries the
// key.
type ArrServiceStatus struct {
	Source      string // seerr, own or none
	Address     string // the address the jobs use (own or from Seerr), "" with source none
	Confirmed   bool   // the jobs may call it: own address, or the address from Seerr is the confirmed host
	Reachable   *bool  // outcome of the last call; nil before the first
	TagRequests *bool  // «Tag Requests» of the server in Seerr
	LastError   string // error class of the last call or of reading Seerr
	LastAt      time.Time
	Counts      TagCounts
	Progress    ArrProgressStatus
}

// ArrStatus is the state of Radarr and Sonarr (getArrStatus). Reading it may ask Seerr for the
// addresses (through the key cache, at most every 10 minutes); it never calls Radarr or Sonarr.
type ArrStatus struct {
	Radarr, Sonarr ArrServiceStatus
}

// ArrStatus returns the state of both services.
func (s *Service) ArrStatus(ctx context.Context) (ArrStatus, error) {
	counts, err := s.read.CountWishTagsByState(ctx)
	if err != nil {
		return ArrStatus{}, fmt.Errorf("count wish tags: %w", err)
	}
	var out ArrStatus
	for _, kind := range []arr.Kind{arr.Radarr, arr.Sonarr} {
		st, err := s.serviceStatus(ctx, kind)
		if err != nil {
			return ArrStatus{}, err
		}
		for _, c := range counts {
			if c.Service != string(kind) {
				continue
			}
			switch c.State {
			case tagSet:
				st.Counts.Set = int(c.N)
			case tagPending:
				st.Counts.Pending = int(c.N)
			case tagFailed:
				st.Counts.Failed = int(c.N)
			}
		}
		if st.Progress, err = s.progressStatus(ctx, kind); err != nil {
			return ArrStatus{}, err
		}
		if kind == arr.Radarr {
			out.Radarr = st
		} else {
			out.Sonarr = st
		}
	}
	return out, nil
}

func (s *Service) serviceStatus(ctx context.Context, kind arr.Kind) (ArrServiceStatus, error) {
	urlKey, hostKey, stateKey := arrKeys(kind)
	st := ArrServiceStatus{Source: ArrSourceNone}
	var rec arrRecord
	if ok, err := s.stateJSON(ctx, stateKey, &rec); err != nil {
		return ArrServiceStatus{}, err
	} else if ok {
		reach := rec.OK || rec.Error == "key_invalid"
		st.Reachable, st.LastError, st.LastAt = &reach, rec.Error, time.UnixMilli(rec.At)
	}
	own, err := s.state(ctx, urlKey)
	if err != nil {
		return ArrServiceStatus{}, err
	}
	if own = strings.TrimSpace(own); own != "" {
		st.Source, st.Address, st.Confirmed = ArrSourceOwn, own, true
		return st, nil
	}
	a := s.arrOf(kind)
	creds, err := a.keys.Get(ctx)
	if err != nil {
		st.LastError = arrErrorClass(err)
		return st, nil
	}
	confirmed, err := s.state(ctx, hostKey)
	if err != nil {
		return ArrServiceStatus{}, err
	}
	st.Source, st.Address = ArrSourceSeerr, creds.BaseURL
	st.Confirmed = confirmed != "" && strings.EqualFold(confirmed, netguard.Site(creds.BaseURL))
	a.mu.Lock()
	st.TagRequests = a.tagRequests
	a.mu.Unlock()
	return st, nil
}

// ArrTagChecks are the check points «Radarr für Profil-Tags» and «Sonarr für Profil-Tags» of the
// connection test of Seerr (keys CheckRadarrTags, CheckSonarrTags): each reads the tag list and
// the queue of the service. They are warnings at most; an unconfirmed address from Seerr is not
// called (detail not_confirmed). The detail of an ok check says whether «Tag Requests» is on in
// Seerr (tag_requests_on, tag_requests_off; N-28).
func (s *Service) ArrTagChecks(ctx context.Context) []model.SeerrCheck {
	out := make([]model.SeerrCheck, 0, 2)
	for _, svc := range []struct {
		key  string
		kind arr.Kind
	}{{CheckRadarrTags, arr.Radarr}, {CheckSonarrTags, arr.Sonarr}} {
		c, _, err := s.arrClient(ctx, svc.kind, true)
		if err == nil {
			_, err = c.Tags(ctx)
			if err == nil {
				_, err = c.Queue(ctx)
			}
			s.arrFailed(svc.kind, err)
			s.recordArr(ctx, svc.kind, err)
		}
		if err != nil {
			out = append(out, model.SeerrCheck{Key: svc.key, Level: model.SeerrLevelWarn, Detail: arrErrorClass(err)})
			continue
		}
		detail := ""
		a := s.arrOf(svc.kind)
		a.mu.Lock()
		if a.tagRequests != nil {
			detail = "tag_requests_off"
			if *a.tagRequests {
				detail = "tag_requests_on"
			}
		}
		a.mu.Unlock()
		out = append(out, model.SeerrCheck{Key: svc.key, Level: model.SeerrLevelOK, Detail: detail})
	}
	return out
}

// TestArrService is TestArrConnection by the service name of the contract (radarr, sonarr); an
// unknown name is a *model.ValidationError on service.
func (s *Service) TestArrService(ctx context.Context, service string) (ArrTestResult, error) {
	kind, err := ParseArrService(service)
	if err != nil {
		return ArrTestResult{}, &model.ValidationError{Fields: []model.FieldError{{Field: "service", Code: "invalid"}}}
	}
	return s.TestArrConnection(ctx, kind)
}
