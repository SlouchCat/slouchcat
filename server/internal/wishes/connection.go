package wishes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The connection test, the fingerprint of the instance and the state of the connection (concept
// 25, architecture 7.4 and 15.1, administration 4.1).

// fingerprint identifies a Seerr instance: email and creation time of user 1 (architecture 7.4).
// Not verified against an instance which values never change; an administrator who changes the
// email of user 1 in Seerr sees «Andere Seerr-Instanz erkannt» and confirms.
type fingerprint struct {
	AdminEmail     string `json:"adminEmail"`
	AdminCreatedAt int64  `json:"adminCreatedAt"` // Unix ms
	SeenAt         int64  `json:"seenAt,omitempty"`
}

func fingerprintOf(me seerr.User) fingerprint {
	return fingerprint{AdminEmail: strings.ToLower(strings.TrimSpace(me.Email)), AdminCreatedAt: me.CreatedAt.UnixMilli()}
}

func (f fingerprint) same(o fingerprint) bool {
	return f.AdminEmail == o.AdminEmail && f.AdminCreatedAt == o.AdminCreatedAt
}

// testRecord is the last test of the saved connection; package admin deletes it with a new address
// or key, so it always speaks of the saved values (T-1).
type testRecord struct {
	Site string `json:"site"`
	OK   bool   `json:"ok"` // no check failed
	T1   bool   `json:"t1"` // reachable, key valid, acting as a user not failed: «Wünsche aktiv» may go on
	At   int64  `json:"at"`
}

// instancePending returns the fingerprint that differs from the confirmed one, nil without one.
func (s *Service) instancePending(ctx context.Context) (*fingerprint, error) {
	var f fingerprint
	ok, err := s.stateJSON(ctx, settingInstanceNew, &f)
	if err != nil || !ok {
		return nil, err
	}
	return &f, nil
}

// checkInstance compares the fingerprint of me with the confirmed one. The first one seen is
// stored; a different one is stored as pending, which stops every call on behalf of a user. It
// returns false when the instance differs (now or still).
func (s *Service) checkInstance(ctx context.Context, me seerr.User) (bool, error) {
	seen := fingerprintOf(me)
	var known fingerprint
	ok, err := s.stateJSON(ctx, settingInstance, &known)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, s.setStateJSON(ctx, s.write, settingInstance, seen)
	}
	if known.same(seen) {
		return true, nil
	}
	return false, s.markInstanceChanged(ctx, seen)
}

// markInstanceChanged stores the fingerprint that differs (architecture 7.4).
func (s *Service) markInstanceChanged(ctx context.Context, seen fingerprint) error {
	if pending, err := s.instancePending(ctx); err != nil || pending != nil {
		return err
	}
	seen.SeenAt = s.now().UnixMilli()
	s.log.WarnContext(ctx, "other seerr instance detected; calls on behalf of users stop until the administrator decides")
	return s.setStateJSON(ctx, s.write, settingInstanceNew, seen)
}

// TestConnection runs the check list of architecture 15.1 against the saved connection or, with
// in.URL or in.APIKey, against values not saved yet. A host other than the saved one needs the key
// again (model.ErrSeerrKeyRequired): the saved key never goes to another address. Without any
// address or key it answers model.ErrSeerrNotConfigured, a bad address a validation error. A
// warning never fails the test. Only a test of the saved values counts for «Wünsche aktiv» (T-1)
// and may store or compare the fingerprint of the instance. The test runs while Seerr is switched
// off; it never creates or changes anything in Seerr.
func (s *Service) TestConnection(ctx context.Context, in model.SeerrTestInput) (model.SeerrTestResult, error) {
	saved, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return model.SeerrTestResult{}, err
	}
	target := model.SeerrConfig{URL: saved.URL, APIKey: saved.APIKey}
	if u := strings.TrimSpace(in.URL); u != "" {
		base, err := seerr.NormalizeAddress(u)
		if err != nil {
			return model.SeerrTestResult{}, (&model.ValidationError{}).Add("url", model.CodeSeerrURLInvalid)
		}
		target.URL = base
		if saved.URL == "" || !sameHost(saved.URL, base) {
			target.APIKey = ""
		}
	}
	if k := strings.TrimSpace(in.APIKey); k != "" {
		target.APIKey = k
	}
	switch {
	case target.URL == "":
		return model.SeerrTestResult{}, model.ErrSeerrNotConfigured
	case target.APIKey == "":
		return model.SeerrTestResult{}, model.ErrSeerrKeyRequired
	}
	isSaved := saved.Configured() && sameAddress(saved.URL, target.URL) && target.APIKey == saved.APIKey
	var c *seerr.Client
	if isSaved {
		if _, c, err = s.connection(ctx, true); err != nil {
			return model.SeerrTestResult{}, err
		}
	} else {
		base, _ := seerr.NormalizeAddress(target.URL)
		if c, err = s.newClient(base, target.APIKey); err != nil {
			return model.SeerrTestResult{}, model.ErrSeerrNotConfigured
		}
	}
	res, outcome := s.runChecks(ctx, c, target.URL, isSaved)
	if isSaved {
		s.recordOutcome(ctx, outcome)
		rec := testRecord{Site: netguard.Site(target.URL), OK: res.OK, T1: t1(res), At: s.now().UnixMilli()}
		if err := s.setStateJSON(ctx, s.write, settingTest, rec); err != nil {
			return model.SeerrTestResult{}, err
		}
		if res.Version != "" {
			if err := s.setState(ctx, s.write, settingVersion, res.Version); err != nil {
				return model.SeerrTestResult{}, err
			}
		}
	}
	s.log.InfoContext(ctx, "seerr connection tested", "saved", isSaved, "ok", res.OK)
	return res, nil
}

// t1 says whether the checks that count for «Wünsche aktiv» passed (T-1, architecture 15.1):
// reachable, key valid, and acting on behalf of users did not fail (a warning, such as «nicht
// prüfbar» on an instance with user 1 only, does not stop it).
func t1(res model.SeerrTestResult) bool {
	need := map[string]bool{model.SeerrCheckReachable: false, model.SeerrCheckKeyValid: false, model.SeerrCheckActAsUser: false}
	for _, c := range res.Checks {
		if _, ok := need[c.Key]; ok {
			need[c.Key] = c.Level != model.SeerrLevelFail
		}
	}
	for _, ok := range need {
		if !ok {
			return false
		}
	}
	return true
}

// runChecks runs the check list; outcome is the error that decides the state of the connection
// (nil when Seerr answered and took the key).
func (s *Service) runChecks(ctx context.Context, c *seerr.Client, address string, isSaved bool) (res model.SeerrTestResult, outcome error) {
	add := func(key, level, detail string) {
		res.Checks = append(res.Checks, model.SeerrCheck{Key: key, Level: level, Detail: detail})
	}
	defer func() {
		res.OK = true
		for _, ch := range res.Checks {
			if ch.Level == model.SeerrLevelFail {
				res.OK = false
			}
		}
	}()
	// Reachable and version: GET /status needs no key.
	st, err := c.Status(ctx)
	if err != nil {
		add(model.SeerrCheckReachable, model.SeerrLevelFail, reachDetail(err))
		return res, err
	}
	add(model.SeerrCheckReachable, model.SeerrLevelOK, "")
	res.Version = st.Version
	if versionTested(st.Version) {
		add(model.SeerrCheckVersion, model.SeerrLevelOK, st.Version)
	} else {
		add(model.SeerrCheckVersion, model.SeerrLevelWarn, model.SeerrDetailNotVerified)
	}
	// The key: GET /auth/me without X-API-User is user 1.
	admin := c.AsAdmin()
	me, err := admin.Me(ctx)
	switch {
	case errors.Is(err, seerr.ErrKeyInvalid):
		add(model.SeerrCheckKeyValid, model.SeerrLevelFail, "")
		return res, err
	case err != nil:
		add(model.SeerrCheckKeyValid, model.SeerrLevelFail, reachDetail(err))
		return res, err
	case me.ID != 1:
		add(model.SeerrCheckKeyValid, model.SeerrLevelFail, model.SeerrDetailNotUserOne)
		return res, nil
	}
	add(model.SeerrCheckKeyValid, model.SeerrLevelOK, "")
	// «Benutzer anlegen und ändern» from the rights of the key's user; the test never creates one.
	if me.Permissions.Has(seerr.PermManageUsers) {
		add(model.SeerrCheckManageUsers, model.SeerrLevelOK, "")
	} else {
		add(model.SeerrCheckManageUsers, model.SeerrLevelFail, "")
	}
	// The instance: only the saved connection stores or compares the fingerprint.
	sameInstance := true
	if isSaved {
		pending, err := s.instancePending(ctx)
		if err != nil {
			return res, err
		}
		if pending == nil {
			if sameInstance, err = s.checkInstance(ctx, me); err != nil {
				return res, err
			}
		} else {
			sameInstance = false
		}
	} else {
		var known fingerprint
		if ok, err := s.stateJSON(ctx, settingInstance, &known); err != nil {
			return res, err
		} else if ok {
			sameInstance = known.same(fingerprintOf(me))
		}
	}
	if sameInstance {
		add(model.SeerrCheckInstance, model.SeerrLevelOK, "")
	} else {
		add(model.SeerrCheckInstance, model.SeerrLevelFail, model.SeerrDetailInstanceChanged)
	}
	// Acting on behalf of users: GET /auth/me with X-API-User. Never while another instance waits.
	switch id, err := s.probeUser(ctx, admin); {
	case !sameInstance:
		add(model.SeerrCheckActAsUser, model.SeerrLevelWarn, model.SeerrDetailInstanceChanged)
	case err != nil:
		add(model.SeerrCheckActAsUser, model.SeerrLevelFail, reachDetail(err))
	case id == 0:
		add(model.SeerrCheckActAsUser, model.SeerrLevelWarn, model.SeerrDetailNotTestable)
	default:
		got, err := c.AsUser(id).Me(ctx)
		if err == nil && got.ID == id {
			add(model.SeerrCheckActAsUser, model.SeerrLevelOK, "")
		} else {
			add(model.SeerrCheckActAsUser, model.SeerrLevelFail, "")
		}
	}
	// CSRF protection: POST /request {} is refused by the validator (400) when it is off.
	if on, err := admin.CSRFProtection(ctx); err != nil {
		add(model.SeerrCheckCSRFOff, model.SeerrLevelWarn, reachDetail(err))
	} else if on {
		add(model.SeerrCheckCSRFOff, model.SeerrLevelFail, "")
	} else {
		add(model.SeerrCheckCSRFOff, model.SeerrLevelOK, "")
	}
	// Radarr and Sonarr in Seerr: warnings only.
	for _, dvr := range []struct {
		key  string
		list func(context.Context) ([]seerr.DVRServer, error)
	}{{model.SeerrCheckRadarr, admin.RadarrServers}, {model.SeerrCheckSonarr, admin.SonarrServers}} {
		servers, err := dvr.list(ctx)
		switch {
		case err != nil:
			add(dvr.key, model.SeerrLevelWarn, reachDetail(err))
		case len(servers) == 0:
			add(dvr.key, model.SeerrLevelWarn, "0")
		default:
			add(dvr.key, model.SeerrLevelOK, strconv.Itoa(len(servers)))
		}
	}
	// Local login (N-11): opened users sign in to Seerr with Slouchcat's email and password.
	if pub, err := admin.PublicSettings(ctx); err != nil {
		add(model.SeerrCheckLocalLogin, model.SeerrLevelWarn, reachDetail(err))
	} else if pub.LocalLogin {
		add(model.SeerrCheckLocalLogin, model.SeerrLevelOK, "")
	} else {
		add(model.SeerrCheckLocalLogin, model.SeerrLevelWarn, model.SeerrDetailOff)
	}
	// Seasons from TVDB (N-64, F-29): Slouchcat never changes the setting, it only says so.
	switch md, err := admin.MetadataSettings(ctx); {
	case errors.Is(err, seerr.ErrNotFound):
		add(model.SeerrCheckTVDBSeasons, model.SeerrLevelWarn, model.SeerrDetailNotAvailable)
	case err != nil:
		add(model.SeerrCheckTVDBSeasons, model.SeerrLevelWarn, reachDetail(err))
	case strings.EqualFold(md.TV, "tvdb"):
		add(model.SeerrCheckTVDBSeasons, model.SeerrLevelOK, "")
	default:
		add(model.SeerrCheckTVDBSeasons, model.SeerrLevelWarn, md.TV)
	}
	// Password transport (N-12): a warning for http outside the household's network.
	if !secureTransport(ctx, address) {
		add(model.SeerrCheckPasswordTransport, model.SeerrLevelWarn, "")
	}
	// Radarr and Sonarr for the profile tags and the progress (stage 4): they read the addresses
	// of the saved connection, so an unsaved address is not tested against them.
	if isSaved {
		res.Checks = append(res.Checks, s.ArrTagChecks(ctx)...)
	}
	return res, nil
}

// probeUser picks the user to test X-API-User with (architecture 4.1): a user of an account other
// than 1, else the first user other than 1 of GET /user; 0 when there is none.
func (s *Service) probeUser(ctx context.Context, admin seerr.Session) (int, error) {
	rows, err := s.read.ListAccountMappings(ctx)
	if err != nil {
		return 0, fmt.Errorf("list mappings: %w", err)
	}
	for _, r := range rows {
		if r.SeerrUserID.Valid && r.SeerrUserID.Int64 != 1 {
			return int(r.SeerrUserID.Int64), nil
		}
	}
	page, err := admin.Users(ctx, seerr.UserQuery{Take: 10, Sort: "created"})
	if err != nil {
		return 0, err
	}
	for _, u := range page.Results {
		if u.ID != 1 {
			return u.ID, nil
		}
	}
	return 0, nil
}

// versionTested: Jellyseerr from 2.7 and Seerr from 3.0 (architecture 15.1).
func versionTested(v string) bool {
	parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major >= 3 || (major == 2 && minor >= 7)
}

// reachDetail names why a check failed, without host or key.
func reachDetail(err error) string {
	switch {
	case errors.Is(err, seerr.ErrRedirect):
		return model.SeerrDetailRedirect
	case errors.Is(err, context.DeadlineExceeded), isTimeout(err):
		return model.SeerrDetailTimeout
	case errors.Is(err, seerr.ErrUnreachable):
		return model.SeerrDetailRefused
	}
	return errorClass(err)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// secureTransport: https, or an address in the household's network.
func secureTransport(ctx context.Context, address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	return u.Scheme == "https" || netguard.LocalHost(ctx, address)
}

// sameHost reports whether two addresses name the same scheme, host and port: the key goes again
// only to where it went before, never in clear text after https (security review L-5).
func sameHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	return errA == nil && errB == nil && strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host)
}

func sameAddress(a, b string) bool {
	na, errA := seerr.NormalizeAddress(a)
	nb, errB := seerr.NormalizeAddress(b)
	return errA == nil && errB == nil && strings.EqualFold(na, nb)
}

// CanEnableWishes is the hook of package admin (T-1): «Wünsche aktiv» may go on once the last test
// of the saved address and key passed the three checks that count and no other instance waits.
func (s *Service) CanEnableWishes(ctx context.Context) error {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Configured() {
		return model.ErrSeerrNotTested
	}
	var rec testRecord
	ok, err := s.stateJSON(ctx, settingTest, &rec)
	if err != nil {
		return err
	}
	if !ok || !rec.T1 || rec.Site != netguard.Site(cfg.URL) {
		return model.ErrSeerrNotTested
	}
	if pending, err := s.instancePending(ctx); err != nil {
		return err
	} else if pending != nil {
		return model.ErrSeerrNotTested
	}
	return nil
}

// SeerrSaved is the hook of package admin after the connection was saved: it queues the refresh of
// the Seerr users (administration 4.1, «Speichern löst seerr.users aus»).
func (s *Service) SeerrSaved(ctx context.Context) {
	if _, err := s.queueUsers(ctx, usersManualPriority); err != nil && !errors.Is(err, model.ErrSeerrNotConfigured) {
		s.log.WarnContext(ctx, "seerr users not queued after saving", "err", err)
	}
}

// Instance actions of confirmSeerrInstance.
const (
	InstanceConfirm = "confirm"
	InstanceReset   = "reset"
)

// ConfirmInstance decides about another instance (architecture 7.4): confirm takes over its
// fingerprint and keeps every mapping; reset deletes every mapping and the mirror of the users,
// which belong to the old instance. Both queue the refresh of the users. model.ErrInstanceUnchanged
// when no other instance waits; a validation error for another action.
func (s *Service) ConfirmInstance(ctx context.Context, action string) (model.SeerrStatus, error) {
	if action != InstanceConfirm && action != InstanceReset {
		return model.SeerrStatus{}, (&model.ValidationError{}).Add("action", "invalid_value")
	}
	pending, err := s.instancePending(ctx)
	if err != nil {
		return model.SeerrStatus{}, err
	}
	if pending == nil {
		return model.SeerrStatus{}, model.ErrInstanceUnchanged
	}
	seen := fingerprint{AdminEmail: pending.AdminEmail, AdminCreatedAt: pending.AdminCreatedAt}
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		if action == InstanceReset {
			if err := q.DeleteAllAccountMappings(ctx); err != nil {
				return fmt.Errorf("delete mappings: %w", err)
			}
			if err := q.DeleteAllSeerrUsers(ctx); err != nil {
				return fmt.Errorf("delete seerr users: %w", err)
			}
		}
		if err := s.setStateJSON(ctx, q, settingInstance, seen); err != nil {
			return err
		}
		return s.delState(ctx, q, settingInstanceNew)
	})
	if err != nil {
		return model.SeerrStatus{}, err
	}
	s.log.InfoContext(ctx, "seerr instance decided", "action", action)
	s.SeerrSaved(ctx)
	return s.Status(ctx)
}

// Status returns the state of the connection, the counters and the call log (getSeerrStatus).
func (s *Service) Status(ctx context.Context) (model.SeerrStatus, error) {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return model.SeerrStatus{}, err
	}
	var st model.SeerrStatus
	pending, err := s.instancePending(ctx)
	if err != nil {
		return model.SeerrStatus{}, err
	}
	st.InstanceChanged = pending != nil
	vals := map[string]*string{settingVersion: &st.Version, settingLastError: &st.LastError}
	for key, dst := range vals {
		if *dst, err = s.state(ctx, key); err != nil {
			return model.SeerrStatus{}, err
		}
	}
	for key, dst := range map[string]**time.Time{settingLastSuccess: &st.LastSuccess, settingLastAttempt: &st.LastAttempt} {
		v, err := s.state(ctx, key)
		if err != nil {
			return model.SeerrStatus{}, err
		}
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			t := time.UnixMilli(ms).UTC()
			*dst = &t
		}
	}
	st.Tested = s.CanEnableWishes(ctx) == nil
	switch {
	case !cfg.Configured():
		st.State = model.SeerrStateNotConfigured
	case !cfg.Enabled:
		st.State = model.SeerrStateDisabled
	case st.InstanceChanged:
		st.State = model.SeerrStateInstanceChanged
	case st.LastError == "key_invalid":
		st.State = model.SeerrStateKeyInvalid
	case st.LastError != "":
		st.State = model.SeerrStateUnreachable
	case st.LastSuccess != nil:
		st.State = model.SeerrStateConnected
	default:
		st.State = model.SeerrStateUntested
	}
	unmapped, err := s.read.CountUnmappedRequesters(ctx)
	if err != nil {
		return model.SeerrStatus{}, fmt.Errorf("count unmapped: %w", err)
	}
	st.Counts.Unmapped = int(unmapped)
	open, err := s.read.ListOpenWishes(ctx)
	if err != nil {
		return model.SeerrStatus{}, fmt.Errorf("open wishes: %w", err)
	}
	st.Counts.Open = len(open)
	for _, w := range open {
		if w.State == model.WishQueued {
			st.Counts.Queued++
		}
	}
	rows, err := s.read.ListSeerrLog(ctx)
	if err != nil {
		return model.SeerrStatus{}, fmt.Errorf("list seerr log: %w", err)
	}
	st.Log = make([]model.SeerrLogEntry, 0, len(rows))
	for _, r := range rows {
		st.Log = append(st.Log, model.SeerrLogEntry{
			At: time.UnixMilli(r.At).UTC(), Method: r.Method, Path: r.Path, Status: int(r.Status.Int64),
			DurationMS: r.DurationMs, Error: r.Error.String,
		})
	}
	return st, nil
}
