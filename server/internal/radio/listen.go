package radio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Listen sessions (concept 16 sections 7.2 and 7.5, build plan S6): a listen session belongs to a
// login session, its profile and one station, and opens the live routes of that station's relay
// with its token. The registry lives in memory and writes through to radio_listen_sessions.
// Every live request slides the expiry (DB touch at most once a minute), so a locked iPhone that
// keeps fetching segments while its timers sleep keeps its session (plan M-2).

const (
	heartbeatSeconds   = 30
	startOffsetSeconds = -8
	// touchInterval limits how often live requests write last_seen_at/expires_at
	// (playback.touchInterval).
	touchInterval = time.Minute
	// sweepInterval is how often Run expires sessions, flushes listen time and stops idle relays.
	sweepInterval = 30 * time.Second
	// listenTimeKeep is how long listen time is kept (playback maxReportDays).
	listenTimeKeep = 90 * 24 * time.Hour
)

// listenSession is one open listen session.
type listenSession struct {
	id           string
	token        string
	sessionID    int64
	accountID    int64
	accountEmail string
	profile      model.Profile
	stationID    int64
	window       time.Duration // the relay's buffer length: the expiry after the last request
	created      time.Time
	expires      time.Time
	touched      time.Time // last write of last_seen_at
	playing      bool
	counted      map[int64]struct{} // segment numbers of the window already credited (listentime.go)
	done         chan struct{}
	relay        *relay
}

// relayStart ends the listen sessions the last process left open (relays do not survive a
// restart) and removes the buffer directories it left; it runs once, before the first listen
// session or sweep.
func (s *Service) relayStart(ctx context.Context) {
	s.rel.startOnce.Do(func() {
		s.removeStaleBuffers()
		n, err := appq.New(s.db.Write).EndOpenRadioListenSessions(ctx, sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true})
		if err != nil {
			s.log.WarnContext(ctx, "end radio listen sessions of the last run", "err", err)
		} else if n > 0 {
			s.log.InfoContext(ctx, "radio listen sessions of the last run ended", "count", n)
		}
	})
}

// staleBuffer matches the name of a relay buffer directory (<station>-<ulid>, before: <station>).
var staleBuffer = regexp.MustCompile(`^[0-9]+(-[0-9A-Z]{26})?$`)

// removeStaleBuffers deletes the relay buffer directories below the configured directory; only
// names a relay writes are touched.
func (s *Service) removeStaleBuffers() {
	if s.o.dir == "" {
		return
	}
	entries, err := os.ReadDir(s.o.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && staleBuffer.MatchString(e.Name()) {
			_ = os.RemoveAll(filepath.Join(s.o.dir, e.Name()))
		}
	}
}

// listenStation reads a station a profile may reach (S2 visibility table): stations outside the
// offer are model.ErrNotFound; broken and missing ones are not.
func (s *Service) listenStation(ctx context.Context, stationID int64) (appq.RadioStation, appq.RadioConfig, error) {
	q := appq.New(s.db.Read)
	st, err := q.GetRadioStation(ctx, stationID)
	if errors.Is(err, sql.ErrNoRows) {
		return st, appq.RadioConfig{}, model.ErrNotFound
	}
	if err != nil {
		return st, appq.RadioConfig{}, fmt.Errorf("get radio station: %w", err)
	}
	cfg, err := q.GetRadioConfig(ctx)
	if err != nil {
		return st, cfg, fmt.Errorf("get radio config: %w", err)
	}
	// The one visibility rule of the catalog (index.go, offer.go): stations outside the offer are
	// 404; broken ones stay reachable.
	if _, err := s.reachableEntry(ctx, stationID); err != nil {
		return st, cfg, err
	}
	return st, cfg, nil
}

// Listen starts or joins the station's relay and opens a listen session (F-19: no stream or
// bitrate limit applies to radio, only the daily watch time).
func (s *Service) Listen(ctx context.Context, session model.Session, stationID int64) (model.RadioListen, error) {
	s.relayStart(ctx)
	st, cfg, err := s.listenStation(ctx, stationID)
	if err != nil {
		return model.RadioListen{}, err
	}
	if s.o.watch != nil {
		wt, err := s.o.watch.WatchTime(ctx, session.Profile, s.now())
		if err != nil {
			return model.RadioListen{}, fmt.Errorf("watch time: %w", err)
		}
		if wt != nil && wt.RemainingMinutes == 0 {
			return model.RadioListen{}, &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
		}
	}
	r, err := s.joinRelay(ctx, st, cfg)
	if err != nil {
		return model.RadioListen{}, err
	}
	defer s.leaveRelay(r)
	timer := time.NewTimer(s.rel.knobs.readyTimeout)
	defer timer.Stop()
	select {
	case <-r.ready:
	case <-r.done:
		return model.RadioListen{}, relayError(r)
	case <-timer.C:
		return model.RadioListen{}, fmt.Errorf("%w: no audio within %v", model.ErrRadioUnreachable, s.rel.knobs.readyTimeout)
	case <-ctx.Done():
		return model.RadioListen{}, ctx.Err()
	}
	now := s.now()
	ls := &listenSession{id: ulid.Make().String(), sessionID: session.ID, accountID: session.Account.ID,
		accountEmail: session.Account.Email, profile: session.Profile, stationID: stationID, window: r.window,
		created: now, expires: now.Add(r.window), touched: now, playing: true, counted: map[int64]struct{}{},
		done: make(chan struct{}), relay: r}
	ls.token = SignToken(s.o.tokenKey, ls.id, stationID)
	if err := appq.New(s.db.Write).InsertRadioListenSession(ctx, appq.InsertRadioListenSessionParams{
		ID: ls.id, SessionID: session.ID, AccountID: session.Account.ID, ProfileID: session.Profile.ID,
		StationID: stationID, Now: now.UnixMilli(), ExpiresAt: ls.expires.UnixMilli(),
	}); err != nil {
		return model.RadioListen{}, fmt.Errorf("insert radio listen session: %w", err)
	}
	s.rel.mu.Lock()
	running := s.rel.relays[stationID] == r
	if running {
		s.rel.sessions[ls.id] = ls
		r.idleSince = time.Time{}
	}
	s.rel.mu.Unlock()
	if !running { // the relay ended between ready and now
		s.endRows(ctx, []string{ls.id}, model.RadioEndedStation)
		return model.RadioListen{}, relayError(r)
	}
	r.mu.Lock()
	in, out := r.in, r.out
	r.mu.Unlock()
	s.log.InfoContext(ctx, "radio listen started", "listen_id", ls.id, "station_id", stationID,
		"profile_id", session.Profile.ID, "codec", in.Codec, "transcoded", !out.Copy)
	view := stationView(st, r)
	if e, err := s.reachableEntry(ctx, stationID); err == nil {
		// The index view carries the facets; the relay's stream gives codec and bitrate.
		codec, kbps, hls := view.Codec, view.BitrateKbps, view.HLS
		view = e.st
		view.Codec, view.BitrateKbps, view.HLS = codec, kbps, hls
		if ids, err := s.favoriteIDs(ctx, session.Profile.ID); err == nil {
			view.Favorite = slices.Contains(ids, stationID)
		}
	}
	shift := 0 // memory mode keeps only the live edge (F-4)
	if s.bufferMode() == model.RadioBufferTmpfs {
		shift = int(r.window / time.Second)
	}
	return model.RadioListen{ListenID: ls.id, Token: ls.token, HeartbeatSeconds: heartbeatSeconds,
		StartOffsetSeconds: startOffsetSeconds, TimeShiftSeconds: shift, Station: view}, nil
}

// stationView is the station of a new listen session as far as the relay part knows it; the
// catalog part may answer the full view instead.
func stationView(st appq.RadioStation, r *relay) model.RadioStation {
	v := model.RadioStation{ID: st.ID, Name: st.Name, Country: st.Country.String, Region: st.Region.String,
		Homepage: st.Homepage.String, Recommended: st.Recommended == 1, ShowsTitles: st.ShowsTitles == 1,
		LogoHash: st.LogoHash.String, Quality: st.Quality.String, State: model.RadioStateOK}
	switch {
	case st.MissingSince.Valid:
		v.State = model.RadioStateMissing
		v.MissingSince = time.UnixMilli(st.MissingSince.Int64)
		v.RemovalAt = v.MissingSince.Add(7 * 24 * time.Hour)
	case st.Reachable == 0 || st.BrokenAt.Valid:
		v.State = model.RadioStateBroken
	}
	_ = json.Unmarshal([]byte(st.Languages), &v.Languages)
	if len(r.streams) > 0 {
		best := r.streams[0]
		v.Codec, v.BitrateKbps, v.HLS = best.Codec.String, int(best.Bitrate.Int64), best.Hls == 1
	}
	return v
}

// relayError is the error a listen gets from an ended relay.
func relayError(r *relay) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	return model.ErrRadioUnreachable
}

// joinRelay returns the running relay of the station or starts one, if the budget and the
// maximum number of relays admit it (settings apply to new relays only). The relay counts the
// join until leaveRelay, so the sweep never stops a relay a Listen is about to use. A relay that
// is ending is refused with its error; after Run ended no relay starts (shutdown).
func (s *Service) joinRelay(ctx context.Context, st appq.RadioStation, cfg appq.RadioConfig) (*relay, error) {
	if r, err := s.joinRunning(st.ID); r != nil || err != nil {
		return r, err
	}
	streams, err := appq.New(s.db.Read).ListRadioStreamsRanked(ctx, st.ID)
	if err != nil {
		return nil, fmt.Errorf("list radio streams: %w", err)
	}
	s.rel.mu.Lock()
	defer s.rel.mu.Unlock()
	if r, err := s.joinRunningLocked(st.ID); r != nil || err != nil { // another Listen started one meanwhile
		return r, err
	}
	window := time.Duration(cfg.BufferMinutes) * time.Minute
	reservation := reservationBytes(reserveKbps(streams), window)
	var reserved int64
	for _, r := range s.rel.relays {
		reserved += r.reservation
	}
	running := len(s.rel.relays)
	if running >= int(cfg.MaxRelays) || reserved+reservation > cfg.BudgetMb<<20 {
		return nil, &model.RadioCapacityError{Running: running, Max: int(cfg.MaxRelays)}
	}
	dir := ""
	if s.bufferMode() == model.RadioBufferTmpfs {
		dir = s.o.dir
	}
	r, err := s.newRelay(st, streams, window, reservation, dir)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	r.cancel = cancel
	r.joins, r.idleSince = 1, time.Time{}
	s.rel.relays[st.ID] = r
	go r.run(rctx)
	s.log.InfoContext(ctx, "radio relay started", "station_id", st.ID, "streams", len(streams),
		"reservation_bytes", reservation, "mode", s.bufferMode())
	return r, nil
}

// joinRunning joins the running relay of the station (nil, nil: none runs).
func (s *Service) joinRunning(stationID int64) (*relay, error) {
	s.rel.mu.Lock()
	defer s.rel.mu.Unlock()
	return s.joinRunningLocked(stationID)
}

func (s *Service) joinRunningLocked(stationID int64) (*relay, error) {
	if s.rel.closed {
		return nil, fmt.Errorf("%w: the server is shutting down", model.ErrRadioUnreachable)
	}
	r := s.rel.relays[stationID]
	if r == nil {
		return nil, nil
	}
	select {
	case <-r.done:
		return nil, nil
	default:
	}
	r.mu.Lock()
	ending := r.state == relayEnding
	r.mu.Unlock()
	if ending {
		return nil, relayError(r)
	}
	r.joins++
	r.idleSince = time.Time{}
	return r, nil
}

// leaveRelay ends the join of a Listen call (its session is registered or it failed).
func (s *Service) leaveRelay(r *relay) {
	s.rel.mu.Lock()
	r.joins--
	s.rel.mu.Unlock()
}

// reserveKbps is the bitrate a reservation assumes for the best stream: the AAC source for copy,
// the chosen AAC bitrate for a transcode.
func reserveKbps(streams []appq.RadioStream) int {
	if len(streams) == 0 {
		return defaultReserveKbps
	}
	best := streams[0]
	codec, kbps := best.Codec.String, int(best.Bitrate.Int64)
	switch codec {
	case "AAC", "AAC+", "aac", "aac+", "he-aac", "HE-AAC":
		if kbps <= 0 {
			return defaultReserveKbps
		}
		return kbps
	}
	return transcodeKbps(kbps)
}

// relayReady runs once a relay has two segments: a broken station plays again.
func (s *Service) relayReady(r *relay) {
	ctx := context.Background()
	n, err := appq.New(s.db.Write).ClearRadioStationBroken(ctx, r.stationID)
	if err != nil {
		s.log.Warn("clear radio station broken", "station_id", r.stationID, "err", err)
	}
	if n > 0 {
		s.onStationChanged(ctx, r.stationID)
	}
}

// relayHealthy runs once a relay ran knobs.healthyAfter: the aborts before no longer count.
func (s *Service) relayHealthy(r *relay) {
	ctx := context.Background()
	n, err := appq.New(s.db.Write).ResetRadioRelayFailures(ctx, r.stationID)
	if err != nil {
		s.log.Warn("reset radio relay failures", "station_id", r.stationID, "err", err)
	}
	if n > 0 {
		s.onStationChanged(ctx, r.stationID)
	}
}

// stationShowsTitles records the first real title of a station.
func (s *Service) stationShowsTitles(ctx context.Context, stationID int64) {
	n, err := appq.New(s.db.Write).SetRadioStationShowsTitles(context.WithoutCancel(ctx), stationID)
	if err != nil {
		s.log.Warn("set radio station shows titles", "station_id", stationID, "err", err)
	}
	if n > 0 {
		s.onStationChanged(context.WithoutCancel(ctx), stationID)
	}
}

// relayEnded removes an ended relay. A relay that failed (err set) ends its listen sessions with
// ended_by 'station' and counts an abort of the station (three in a row mark it broken).
func (s *Service) relayEnded(r *relay, err error) {
	ctx := context.Background()
	s.rel.mu.Lock()
	if s.rel.relays[r.stationID] == r {
		delete(s.rel.relays, r.stationID)
	}
	s.rel.mu.Unlock()
	if err == nil {
		s.log.Info("radio relay stopped", "station_id", r.stationID)
		return
	}
	s.log.Warn("radio relay ended", "station_id", r.stationID, "err", err)
	ids, qerr := appq.New(s.db.Write).EndRadioListenSessionsOfStation(ctx, appq.EndRadioListenSessionsOfStationParams{
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, EndedBy: sql.NullString{String: model.RadioEndedStation, Valid: true},
		StationID: r.stationID,
	})
	if qerr != nil {
		s.log.Warn("end radio listen sessions of station", "station_id", r.stationID, "err", qerr)
	}
	s.endLocal(ctx, ids)
	row, qerr := appq.New(s.db.Write).RecordRadioRelayAbort(ctx, appq.RecordRadioRelayAbortParams{
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: r.stationID})
	if qerr != nil {
		s.log.Warn("record radio relay abort", "station_id", r.stationID, "err", qerr)
		return
	}
	if row.BrokenAt.Valid {
		s.log.Warn("radio station marked broken", "station_id", r.stationID, "failures", row.Failures)
	}
	s.onStationChanged(ctx, r.stationID)
}

// Heartbeat keeps a listen session open and stores the play state (playing false only marks a
// pause, plan M-2); it also checks the daily watch time.
func (s *Service) Heartbeat(ctx context.Context, session model.Session, listenID string, playing bool) (model.RadioHeartbeat, error) {
	s.rel.mu.Lock()
	ls, ok := s.rel.sessions[listenID]
	if ok && ls.profile.ID != session.Profile.ID {
		ok = false
	}
	var expires time.Time
	now := s.now()
	if ok {
		ls.expires, ls.playing, ls.touched = now.Add(ls.window), playing, now
		expires = ls.expires
	}
	s.rel.mu.Unlock()
	if !ok {
		return model.RadioHeartbeat{}, s.endedError(ctx, session, listenID)
	}
	if _, err := appq.New(s.db.Write).TouchRadioListenSession(ctx, appq.TouchRadioListenSessionParams{
		Now: now.UnixMilli(), ExpiresAt: expires.UnixMilli(), Playing: boolInt(playing), ID: listenID,
	}); err != nil {
		s.log.WarnContext(ctx, "touch radio listen session", "listen_id", listenID, "err", err)
	}
	wt, err := s.checkLimit(ctx, session.Profile)
	if err != nil {
		return model.RadioHeartbeat{}, err
	}
	if wt != nil && wt.RemainingMinutes == 0 {
		return model.RadioHeartbeat{}, &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
	}
	return model.RadioHeartbeat{ExpiresAt: expires, WatchTime: wt}, nil
}

// endedError explains why a listen session of the profile is not open: the limit ended it
// (*model.WatchLimitError), its relay ended (model.ErrRadioUnreachable), else model.ErrNotFound.
func (s *Service) endedError(ctx context.Context, session model.Session, listenID string) error {
	row, err := appq.New(s.db.Read).GetRadioListenSession(ctx, listenID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && row.ProfileID != session.Profile.ID {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get radio listen session: %w", err)
	}
	switch row.EndedBy.String {
	case model.StoppedByLimit:
		wt := &model.WatchTime{}
		if s.o.watch != nil {
			if w, err := s.o.watch.WatchTime(ctx, session.Profile, s.now()); err == nil && w != nil {
				wt = w
			}
		}
		return &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
	case model.RadioEndedStation:
		return model.ErrRadioUnreachable
	}
	return model.ErrNotFound
}

// EndListen ends a listen session of the profile (the client stopped).
func (s *Service) EndListen(ctx context.Context, session model.Session, listenID string) error {
	row, err := appq.New(s.db.Read).GetRadioListenSession(ctx, listenID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && row.ProfileID != session.Profile.ID {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get radio listen session: %w", err)
	}
	if row.EndedAt.Valid {
		return nil
	}
	s.endRows(ctx, []string{listenID}, model.StoppedByClient)
	return nil
}

// StopListener ends a listen session from the administration (ended_by admin).
func (s *Service) StopListener(ctx context.Context, listenID string) error {
	n, err := appq.New(s.db.Write).EndRadioListenSession(ctx, appq.EndRadioListenSessionParams{
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, EndedBy: sql.NullString{String: model.StoppedByAdmin, Valid: true},
		ID: listenID,
	})
	if err != nil {
		return fmt.Errorf("end radio listen session: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.endLocal(ctx, []string{listenID})
	return nil
}

// EndSession ends the listen sessions of a login session (logout, profile switch).
func (s *Service) EndSession(ctx context.Context, sessionID int64, by string) error {
	ids, err := appq.New(s.db.Write).EndRadioListenSessionsBySession(ctx, appq.EndRadioListenSessionsBySessionParams{
		Now: s.nowMs(), EndedBy: sql.NullString{String: by, Valid: true}, SessionID: sessionID})
	if err != nil {
		return fmt.Errorf("end radio listen sessions of session: %w", err)
	}
	s.endLocal(ctx, ids)
	return nil
}

// EndProfile ends the listen sessions of a profile (the playback hook OnProfileEnd: watch limit,
// profile deleted).
func (s *Service) EndProfile(ctx context.Context, profileID int64, by string) error {
	ids, err := appq.New(s.db.Write).EndRadioListenSessionsByProfile(ctx, appq.EndRadioListenSessionsByProfileParams{
		Now: s.nowMs(), EndedBy: sql.NullString{String: by, Valid: true}, ProfileID: profileID})
	if err != nil {
		return fmt.Errorf("end radio listen sessions of profile: %w", err)
	}
	s.endLocal(ctx, ids)
	return nil
}

// EndAccount ends the listen sessions of a revoked account.
func (s *Service) EndAccount(ctx context.Context, accountID int64) error {
	ids, err := appq.New(s.db.Write).EndRadioListenSessionsByAccount(ctx, appq.EndRadioListenSessionsByAccountParams{
		Now: s.nowMs(), EndedBy: sql.NullString{String: model.StoppedByRevoked, Valid: true}, AccountID: accountID})
	if err != nil {
		return fmt.Errorf("end radio listen sessions of account: %w", err)
	}
	s.endLocal(ctx, ids)
	return nil
}

// endRows ends listen sessions by id in the table and in memory.
func (s *Service) endRows(ctx context.Context, ids []string, by string) {
	var ended []string
	for _, id := range ids {
		n, err := appq.New(s.db.Write).EndRadioListenSession(ctx, appq.EndRadioListenSessionParams{
			Now: s.nowMs(), EndedBy: sql.NullString{String: by, Valid: true}, ID: id})
		if err != nil {
			s.log.WarnContext(ctx, "end radio listen session", "listen_id", id, "err", err)
			continue
		}
		if n > 0 {
			ended = append(ended, id)
		}
	}
	s.endLocal(ctx, ended)
}

// endLocal removes ended listen sessions from memory: their done channel closes, so a running
// segment write is cut, and their next live request answers 401. Their listen time is written.
func (s *Service) endLocal(ctx context.Context, ids []string) {
	if len(ids) == 0 {
		return
	}
	now := s.now()
	s.rel.mu.Lock()
	for _, id := range ids {
		if ls, ok := s.rel.sessions[id]; ok {
			delete(s.rel.sessions, id)
			close(ls.done)
			if !slices.ContainsFunc(slices.Collect(maps.Values(s.rel.sessions)), func(o *listenSession) bool { return o.relay == ls.relay }) {
				ls.relay.idleSince = now
			}
		}
	}
	s.rel.mu.Unlock()
	s.flushListenTime(context.WithoutCancel(ctx))
}

// AuthorizeLive resolves the listen token of a live request and slides the session's expiry.
func (s *Service) AuthorizeLive(ctx context.Context, listenID, token string) (model.RadioGrant, error) {
	now := s.now()
	s.rel.mu.Lock()
	ls, ok := s.rel.sessions[listenID]
	if !ok || !checkToken(s.o.tokenKey, token, listenID, ls.stationID) || !now.Before(ls.expires) {
		s.rel.mu.Unlock()
		return model.RadioGrant{}, model.ErrUnauthenticated
	}
	ls.expires = now.Add(ls.window)
	touch := now.Sub(ls.touched) >= touchInterval
	if touch {
		ls.touched = now
	}
	grant := model.RadioGrant{ListenID: listenID, StationID: ls.stationID, Profile: ls.profile}
	expires, playing, sessionID := ls.expires, ls.playing, ls.sessionID
	s.rel.mu.Unlock()
	if touch {
		// The token outlives no login session: once a minute the listen session checks that its
		// login session still authorises (logout, password change, role demotion, expiry).
		valid, err := appq.New(s.db.Read).RadioLoginSessionValid(ctx, appq.RadioLoginSessionValidParams{
			SessionID: sessionID, Now: now.UnixMilli()})
		if err != nil {
			s.log.WarnContext(ctx, "check the login session of a radio listen session", "listen_id", listenID, "err", err)
		} else if !valid {
			s.endRows(ctx, []string{listenID}, model.StoppedByRevoked)
			return model.RadioGrant{}, model.ErrUnauthenticated
		}
		if _, err := appq.New(s.db.Write).TouchRadioListenSession(ctx, appq.TouchRadioListenSessionParams{
			Now: now.UnixMilli(), ExpiresAt: expires.UnixMilli(), Playing: boolInt(playing), ID: listenID,
		}); err != nil {
			s.log.WarnContext(ctx, "touch radio listen session", "listen_id", listenID, "err", err)
		}
	}
	return grant, nil
}

// liveSession returns the open session of a grant with its relay (bound to the grant's station).
func (s *Service) liveSession(g model.RadioGrant) (*listenSession, *relay, error) {
	s.rel.mu.Lock()
	defer s.rel.mu.Unlock()
	ls, ok := s.rel.sessions[g.ListenID]
	if !ok || ls.stationID != g.StationID {
		return nil, nil, model.ErrUnauthenticated
	}
	r := s.rel.relays[g.StationID]
	if r == nil || r != ls.relay {
		return nil, nil, model.ErrNotFound
	}
	return ls, r, nil
}

// Playlist returns the live playlist of the grant's relay.
func (s *Service) Playlist(_ context.Context, g model.RadioGrant, skip string) ([]byte, error) {
	ls, r, err := s.liveSession(g)
	if err != nil {
		return nil, err
	}
	return r.playlist(ls.token, skip == "YES" || skip == "v2"), nil
}

// Init returns the init part of a generation of the grant's relay.
func (s *Service) Init(_ context.Context, g model.RadioGrant, gen int) ([]byte, error) {
	_, r, err := s.liveSession(g)
	if err != nil {
		return nil, err
	}
	return r.initData(gen)
}

// Segment returns one segment of the grant's relay and credits its duration to the profile's
// listen time the first time the session fetches it (listentime.go).
func (s *Service) Segment(_ context.Context, g model.RadioGrant, n int64) ([]byte, <-chan struct{}, error) {
	ls, r, err := s.liveSession(g)
	if err != nil {
		return nil, nil, err
	}
	data, dur, err := r.segmentData(n)
	if err != nil {
		return nil, nil, err
	}
	s.creditSegment(ls, n, dur, r.oldestSeq())
	return data, ls.done, nil
}

// Relays lists the running relays with their listeners and the load (administration). Each
// listener names the device of its login session and each relay the logo of its station (concept
// 18, 17.4). An open listen session whose relay no longer runs (its relay ended and closing the
// session failed, or the station's relay was started anew) is listed under its ended relay in the
// state relayFailed, so the overview shows it instead of hiding it; the load counts no such relay.
func (s *Service) Relays(ctx context.Context) (model.RadioRelays, error) {
	q := appq.New(s.db.Read)
	cfg, err := q.GetRadioConfig(ctx)
	if err != nil {
		return model.RadioRelays{}, fmt.Errorf("get radio config: %w", err)
	}
	s.rel.mu.Lock()
	relays := make([]*relay, 0, len(s.rel.relays))
	for _, r := range s.rel.relays {
		relays = append(relays, r)
	}
	listeners := map[*relay][]model.RadioListenerInfo{}
	var failed []*relay // ended relays that still have open listen sessions
	sessionIDs := map[string]int64{}
	for _, ls := range s.rel.sessions {
		if s.rel.relays[ls.stationID] != ls.relay && listeners[ls.relay] == nil {
			failed = append(failed, ls.relay)
		}
		listeners[ls.relay] = append(listeners[ls.relay], model.RadioListenerInfo{ListenID: ls.id,
			ProfileName: ls.profile.Name, AccountEmail: ls.accountEmail, Playing: ls.playing, Since: ls.created})
		sessionIDs[ls.id] = ls.sessionID
	}
	s.rel.mu.Unlock()
	devices, err := s.listenDevices(ctx, sessionIDs)
	if err != nil {
		return model.RadioRelays{}, err
	}
	stationIDs := make([]int64, 0, len(relays)+len(failed))
	for _, r := range slices.Concat(relays, failed) {
		stationIDs = append(stationIDs, r.stationID)
	}
	logos := map[int64]string{}
	if len(stationIDs) > 0 {
		rows, err := q.RadioStationLogoHashes(ctx, stationIDs)
		if err != nil {
			return model.RadioRelays{}, fmt.Errorf("radio station logos: %w", err)
		}
		for _, row := range rows {
			logos[row.ID] = row.LogoHash.String
		}
	}
	out := model.RadioRelays{Relays: []model.RadioRelayInfo{}, Load: model.RadioLoad{
		Relays: len(relays), MaxRelays: int(cfg.MaxRelays), BudgetBytes: cfg.BudgetMb << 20, BufferMode: s.bufferMode()}}
	for _, r := range slices.Concat(relays, failed) {
		running := !slices.Contains(failed, r)
		r.mu.Lock()
		kbps := r.out.BitrateKbps
		if r.out.Copy && kbps == 0 {
			kbps = r.in.BitrateKbps
		}
		info := model.RadioRelayInfo{StationID: r.stationID, Name: r.name, Codec: r.in.Codec, BitrateKbps: kbps,
			Transcoded: !r.out.Copy && r.in.Codec != "", StartedAt: r.startedAt, BufferSeconds: int(r.buf.total.Seconds()),
			BufferBytes: r.buf.bytes, State: r.state, Listeners: listeners[r], LogoHash: logos[r.stationID]}
		r.mu.Unlock()
		if !running {
			info.State, info.BufferSeconds, info.BufferBytes = relayFailed, 0, 0
		}
		for i := range info.Listeners {
			info.Listeners[i].Device = devices[info.Listeners[i].ListenID]
		}
		slices.SortFunc(info.Listeners, func(a, b model.RadioListenerInfo) int { return a.Since.Compare(b.Since) })
		if info.Listeners == nil {
			info.Listeners = []model.RadioListenerInfo{}
		}
		if running {
			for _, l := range info.Listeners {
				if l.Playing {
					out.Load.UploadKbps += kbps
				}
			}
			out.Load.BufferBytes += info.BufferBytes
		}
		out.Relays = append(out.Relays, info)
	}
	slices.SortFunc(out.Relays, func(a, b model.RadioRelayInfo) int { return a.StartedAt.Compare(b.StartedAt) })
	return out, nil
}

// listenDevices returns listen id -> the device of its login session (sessionIDs: listen id ->
// login session id), for the listen sessions whose login session has a device.
func (s *Service) listenDevices(ctx context.Context, sessionIDs map[string]int64) (map[string]*model.DeviceRef, error) {
	out := map[string]*model.DeviceRef{}
	if len(sessionIDs) == 0 {
		return out, nil
	}
	ids := slices.Sorted(maps.Values(sessionIDs))
	rows, err := appq.New(s.db.Read).RadioListenDevices(ctx, slices.Compact(ids))
	if err != nil {
		return nil, fmt.Errorf("radio listen devices: %w", err)
	}
	bySession := make(map[int64]*model.DeviceRef, len(rows))
	for _, row := range rows {
		bySession[row.SessionID] = &model.DeviceRef{ID: row.DeviceID, Name: row.DeviceName.String}
	}
	for listenID, sessionID := range sessionIDs {
		if d := bySession[sessionID]; d != nil {
			out[listenID] = d
		}
	}
	return out, nil
}

// Run sweeps the listen sessions and relays every 30 s until ctx ends, then stops every relay
// (started by the server next to playback.Service.Run); it also runs the cover worker (cover.go)
// and returns only after the worker has ended.
func (s *Service) Run(ctx context.Context) {
	s.relayStart(ctx)
	covers := make(chan struct{})
	go func() {
		defer close(covers)
		s.runCovers(ctx)
	}()
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.cat.bg.stop() // own-station probes and a background rule apply end first
			s.rel.mu.Lock()
			s.rel.closed = true
			s.rel.mu.Unlock()
			s.stopRelays()
			s.flushListenTime(context.WithoutCancel(ctx))
			<-covers // no lookup writes after Run returned (the database closes after it)
			return
		case <-t.C:
			s.sweep(ctx)
		}
	}
}

// sweep expires listen sessions past their expiry, writes the listen time, stops relays without
// a listener for grace_seconds, deletes old listen time and, once a day, expired covers.
func (s *Service) sweep(ctx context.Context) {
	now := s.now()
	var expired []string
	s.rel.mu.Lock()
	for id, ls := range s.rel.sessions {
		if !now.Before(ls.expires) {
			expired = append(expired, id)
		}
	}
	s.rel.mu.Unlock()
	s.endRows(ctx, expired, model.RadioEndedExpired)
	s.flushListenTime(ctx)
	s.stopIdleRelays(ctx, now)
	s.sweepCovers(ctx, now)
	s.rel.mu.Lock()
	prune := now.Sub(s.rel.pruned) >= time.Hour
	if prune {
		s.rel.pruned = now
	}
	s.rel.mu.Unlock()
	if prune {
		if _, err := appq.New(s.db.Write).DeleteOldRadioListenTime(ctx, now.Add(-listenTimeKeep).UnixMilli()/quarterMs); err != nil {
			s.log.WarnContext(ctx, "delete old radio listen time", "err", err)
		}
	}
}

// stopIdleRelays stops the relays that had no open listen session for grace_seconds.
func (s *Service) stopIdleRelays(ctx context.Context, now time.Time) {
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "get radio config", "err", err)
		return
	}
	grace := time.Duration(cfg.GraceSeconds) * time.Second
	var stop []*relay
	s.rel.mu.Lock()
	busy := map[int64]bool{}
	for _, ls := range s.rel.sessions {
		busy[ls.stationID] = true
	}
	for id, r := range s.rel.relays {
		busy[id] = busy[id] || r.joins > 0 // a Listen is starting or joining it
		switch {
		case busy[id]:
			r.idleSince = time.Time{}
		case r.idleSince.IsZero():
			r.idleSince = now
		}
		if !busy[id] && now.Sub(r.idleSince) >= grace {
			stop = append(stop, r)
		}
	}
	for _, r := range stop {
		delete(s.rel.relays, r.stationID)
	}
	s.rel.mu.Unlock()
	for _, r := range stop {
		r.stop()
	}
}

// closeUnreachable ends the listen sessions and stops the relay of every station of ids (none:
// every station with a relay or a listen session) that profiles may no longer reach, because it
// left the offer (ApplyOffer): invisible means 404, also for a running listen.
func (s *Service) closeUnreachable(ctx context.Context, ids ...int64) {
	if len(ids) == 0 {
		s.rel.mu.Lock()
		for id := range s.rel.relays {
			ids = append(ids, id)
		}
		for _, ls := range s.rel.sessions {
			ids = append(ids, ls.stationID)
		}
		s.rel.mu.Unlock()
		slices.Sort(ids)
		ids = slices.Compact(ids)
	}
	for _, id := range ids {
		_, err := s.reachableEntry(ctx, id)
		if err == nil {
			continue
		}
		if !errors.Is(err, model.ErrNotFound) {
			s.log.WarnContext(ctx, "check radio station visibility", "station_id", id, "err", err)
			continue
		}
		s.closeStation(ctx, id)
	}
}

// closeStation ends the listen sessions of a station (ended_by admin) and stops its relay.
func (s *Service) closeStation(ctx context.Context, stationID int64) {
	s.rel.mu.Lock()
	r := s.rel.relays[stationID]
	if r != nil {
		delete(s.rel.relays, stationID)
	}
	s.rel.mu.Unlock()
	ids, err := appq.New(s.db.Write).EndRadioListenSessionsOfStation(ctx, appq.EndRadioListenSessionsOfStationParams{
		Now: s.nowMs(), EndedBy: sql.NullString{String: model.StoppedByAdmin, Valid: true}, StationID: stationID})
	if err != nil {
		s.log.WarnContext(ctx, "end radio listen sessions of station", "station_id", stationID, "err", err)
	}
	s.endLocal(ctx, ids)
	if r != nil {
		r.stop()
		s.log.InfoContext(ctx, "radio relay stopped: station no longer reachable", "station_id", stationID)
	}
}

// stopRelays stops every relay (shutdown).
func (s *Service) stopRelays() {
	s.rel.mu.Lock()
	relays := make([]*relay, 0, len(s.rel.relays))
	for _, r := range s.rel.relays {
		relays = append(relays, r)
	}
	s.rel.mu.Unlock()
	for _, r := range relays {
		r.stop()
	}
}

func (s *Service) nowMs() sql.NullInt64 {
	return sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
