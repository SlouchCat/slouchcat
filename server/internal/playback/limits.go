package playback

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Limits of a profile at the start of a playback (plan 3.5 "an genau dieser Stelle", W-8, K-5):
// the daily watch time (K-5) and the number of concurrent streams (W-8). The bandwidth and height
// limits shape the decision itself (decide.go).

// ActiveWindow is how recent the last request of a playback session must be for it to count as a
// running stream (W-8 maxStreams, the stream overview of W-6): token requests touch the row at
// most once a minute, progress events every few seconds.
const ActiveWindow = 2 * time.Minute

// Watch-report bounds (getWatchReport days).
const (
	defaultReportDays = 30
	maxReportDays     = 90
)

// startLocks serialises the stream counts of the starts of a profile (or, under a guest cap, of
// every profile of the account), so two starts cannot both take the last stream. The lock is held
// only while a start counts and reserves its stream and while it inserts its row, not while its
// output waits for the transcode budget; a reserved stream counts until its row exists.
type startLocks struct {
	mu      sync.Mutex
	locks   map[startKey]*startLock
	profile map[int64]int // reserved streams per profile
	account map[int64]int // reserved streams per account
}

// startKey names the lock of a start: the account under a guest cap, else the profile.
type startKey struct {
	account bool
	id      int64
}

type startLock struct {
	ch    chan struct{} // capacity 1: holding the lock is a value in the channel
	users int
}

// lock takes the start lock of key; it gives up when ctx ends.
func (l *startLocks) lock(ctx context.Context, key startKey) (func(), error) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = map[startKey]*startLock{}
	}
	sl := l.locks[key]
	if sl == nil {
		sl = &startLock{ch: make(chan struct{}, 1)}
		l.locks[key] = sl
	}
	sl.users++
	l.mu.Unlock()
	leave := func() {
		l.mu.Lock()
		if sl.users--; sl.users == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}
	select {
	case sl.ch <- struct{}{}:
		return func() { <-sl.ch; leave() }, nil
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
}

// reserved returns the streams reserved by starts of the profile and of the account that have no
// row yet.
func (l *startLocks) reserved(profileID, accountID int64) (profile, account int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.profile[profileID], l.account[accountID]
}

// reserve adds (delta 1) or removes (delta -1) a reserved stream of the profile and the account.
func (l *startLocks) reserve(profileID, accountID int64, delta int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.profile == nil {
		l.profile, l.account = map[int64]int{}, map[int64]int{}
	}
	if l.profile[profileID] += delta; l.profile[profileID] <= 0 {
		delete(l.profile, profileID)
	}
	if l.account[accountID] += delta; l.account[accountID] <= 0 {
		delete(l.account, accountID)
	}
}

// slot is the stream a start under a stream limit reserved in admit. insert writes the playback
// row under the start lock and ends the reservation in the same step, so the new row and the
// reservation never count together; release ends a reservation that was not used. A nil slot (no
// stream limit) just inserts.
type slot struct {
	s       *Service
	key     startKey
	profile int64
	account int64
	done    bool
}

// insert runs the insert of the playback row under the start lock and ends the reservation.
func (sl *slot) insert(ctx context.Context, insert func() error) error {
	if sl == nil {
		return insert()
	}
	// The output already runs: the insert does not give up because the client went away.
	unlock, err := sl.s.starts.lock(context.WithoutCancel(ctx), sl.key)
	if err != nil {
		return err
	}
	defer unlock()
	sl.release()
	return insert()
}

// release ends the reservation (once).
func (sl *slot) release() {
	if sl == nil || sl.done {
		return
	}
	sl.done = true
	sl.s.starts.reserve(sl.profile, sl.account, -1)
}

// admit checks the limits of a start: a profile whose watch time of today is used up gets
// *model.WatchLimitError, a profile that runs its maximum of streams *model.StreamLimitError, and
// so does a guest account (Account.Limits.MaxStreams) whose profiles together run its maximum.
// The playback the start replaces (same browser session and title: a track, quality or version
// switch) ends first (stopped_by client), so it does not count and frees its transcode budget.
// It returns the watch time of today when the profile has a daily limit and, under a stream
// limit, the reserved stream: the caller inserts the row through slot.insert and releases it.
func (s *Service) admit(ctx context.Context, session model.Session, itemID string, limits model.PlaybackLimits, now time.Time) (*model.WatchTime, *slot, error) {
	var watch *model.WatchTime
	if limits.WatchLimitMinutes > 0 {
		wt, err := s.watchTime(ctx, session.Profile.ID, limits.WatchLimitMinutes, now)
		if err != nil {
			return nil, nil, err
		}
		if wt.RemainingMinutes == 0 {
			return nil, nil, &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
		}
		watch = &wt
	}
	accountMax := session.Account.Limits.MaxStreams
	if limits.MaxStreams <= 0 && accountMax <= 0 {
		return watch, nil, s.endReplaced(ctx, session, itemID)
	}
	key := startKey{id: session.Profile.ID}
	if accountMax > 0 {
		key = startKey{account: true, id: session.Account.ID}
	}
	unlock, err := s.starts.lock(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	defer unlock()
	if err := s.endReplaced(ctx, session, itemID); err != nil {
		return nil, nil, err
	}
	resProfile, resAccount := s.starts.reserved(session.Profile.ID, session.Account.ID)
	activeAfter := now.Add(-ActiveWindow).UnixMilli()
	if limits.MaxStreams > 0 {
		n, err := s.read.CountActivePlaybackSessions(ctx, appq.CountActivePlaybackSessionsParams{
			ProfileID: session.Profile.ID, Now: now.UnixMilli(), ActiveAfter: activeAfter,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("count active playback sessions: %w", err)
		}
		if running := int(n) + resProfile; running >= limits.MaxStreams {
			return nil, nil, &model.StreamLimitError{MaxStreams: limits.MaxStreams, ActiveStreams: running}
		}
	}
	if accountMax > 0 {
		n, err := s.read.CountActiveAccountPlaybackSessions(ctx, appq.CountActiveAccountPlaybackSessionsParams{
			AccountID: session.Account.ID, Now: now.UnixMilli(), ActiveAfter: activeAfter,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("count active playback sessions of account: %w", err)
		}
		if running := int(n) + resAccount; running >= accountMax {
			return nil, nil, &model.StreamLimitError{MaxStreams: accountMax, ActiveStreams: running}
		}
	}
	s.starts.reserve(session.Profile.ID, session.Account.ID, 1)
	return watch, &slot{s: s, key: key, profile: session.Profile.ID, account: session.Account.ID}, nil
}

// endReplaced ends the playback sessions a start of the browser session replaces, those of the
// same title (stopped_by client); the client's own stop of the old playback then answers 404.
func (s *Service) endReplaced(ctx context.Context, session model.Session, itemID string) error {
	ids, err := s.write.RevokeReplacedPlaybackSessions(ctx, appq.RevokeReplacedPlaybackSessionsParams{
		SessionID: session.ID, ItemID: itemID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke replaced playback sessions: %w", err)
	}
	for _, id := range ids {
		s.end(id)
	}
	return nil
}

// recheckWatchLimit runs after the insert of a playback row of a profile with a daily limit: a
// progress event that used up the limit while the start was admitted revoked the profile's
// playback before this row existed, so the new playback ends here (stopped_by limit).
func (s *Service) recheckWatchLimit(ctx context.Context, profileID int64, limitMinutes int, id string) error {
	wt, err := s.watchTime(ctx, profileID, limitMinutes, s.now())
	if err != nil || wt.RemainingMinutes > 0 {
		return err
	}
	if _, err := s.write.StopPlaybackSession(ctx, appq.StopPlaybackSessionParams{
		ID: id, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(model.StoppedByLimit),
	}); err != nil {
		return fmt.Errorf("stop playback session: %w", err)
	}
	s.end(id)
	return &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
}

// startOfDay is midnight of the calendar day of t in t's location (the server's time zone for
// the clock time.Now).
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// playedMs sums the played time of a profile in [from, to) from the raw progress events.
func (s *Service) playedMs(ctx context.Context, profileID int64, from, to time.Time) (int64, error) {
	ms, err := s.read.SumPlayedMs(ctx, appq.SumPlayedMsParams{ProfileID: profileID, FromMs: from.UnixMilli(), ToMs: to.UnixMilli()})
	if err != nil {
		return 0, fmt.Errorf("sum played time: %w", err)
	}
	return ms, nil
}

// watchTime is the watch time of a profile on the day of now against a daily limit in minutes:
// used minutes rounded down, remaining minutes rounded up, so the remaining time is 0 exactly when
// the limit is reached.
func (s *Service) watchTime(ctx context.Context, profileID int64, limitMinutes int, now time.Time) (model.WatchTime, error) {
	used, err := s.playedMs(ctx, profileID, startOfDay(now), now.Add(time.Millisecond))
	if err != nil {
		return model.WatchTime{}, err
	}
	limit := int64(limitMinutes) * time.Minute.Milliseconds()
	wt := model.WatchTime{LimitMinutes: limitMinutes, UsedMinutes: int(used / time.Minute.Milliseconds())}
	if used < limit {
		wt.RemainingMinutes = int((limit - used + time.Minute.Milliseconds() - 1) / time.Minute.Milliseconds())
	}
	return wt, nil
}

// WatchTime returns the watch time of a profile today (server time zone) against its daily limit
// (K-5), or nil when the profile has none.
func (s *Service) WatchTime(ctx context.Context, profile model.Profile, now time.Time) (*model.WatchTime, error) {
	if profile.Effective.WatchLimitMinutes <= 0 {
		return nil, nil
	}
	wt, err := s.watchTime(ctx, profile.ID, profile.Effective.WatchLimitMinutes, now)
	if err != nil {
		return nil, err
	}
	return &wt, nil
}

// checkWatchLimit runs after every progress event of a profile with a daily limit: once the limit
// is reached it revokes every playback session of the profile (stopped_by limit), so streams and
// HLS answer 401 from their next request (K-5 "Stopp mit Hinweis").
func (s *Service) checkWatchLimit(ctx context.Context, profile model.Profile) (*model.WatchTime, error) {
	wt, err := s.WatchTime(ctx, profile, s.now())
	if err != nil || wt == nil {
		return wt, err
	}
	if wt.RemainingMinutes == 0 {
		if err := s.endProfile(ctx, profile.ID, model.StoppedByLimit); err != nil {
			return nil, err
		}
		s.log.InfoContext(ctx, "watch-time limit reached, playback ended", "profile_id", profile.ID,
			"limit_minutes", wt.LimitMinutes, "used_minutes", wt.UsedMinutes)
	}
	return wt, nil
}

// WatchReport returns the minutes a profile of the session's account watched per calendar day
// (server time zone), the last days days including today, oldest first (K-5 report). Only the
// account's primary profile may read it (model.ErrPrimaryProfileRequired); a profile of another
// account is model.ErrNotFound; days outside 1–90 is a validation error (0 = 30).
func (s *Service) WatchReport(ctx context.Context, session model.Session, profileID int64, days int) (model.WatchReport, error) {
	if days == 0 {
		days = defaultReportDays
	}
	if days < 1 || days > maxReportDays {
		return model.WatchReport{}, (&model.ValidationError{}).Add("days", "invalid_value")
	}
	if !session.IsPrimary() {
		return model.WatchReport{}, model.ErrPrimaryProfileRequired
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: profileID, AccountID: session.Account.ID})
	if err != nil {
		return model.WatchReport{}, notFoundOr(err, "get profile")
	}
	now := s.now()
	today := startOfDay(now)
	first := today.AddDate(0, 0, -(days - 1))
	quarters, err := s.read.SumPlayedMsByQuarterHour(ctx, appq.SumPlayedMsByQuarterHourParams{
		ProfileID: row.ID, FromMs: first.UnixMilli(), ToMs: today.AddDate(0, 0, 1).UnixMilli(),
	})
	if err != nil {
		return model.WatchReport{}, fmt.Errorf("sum played time: %w", err)
	}
	perDay := map[string]int64{}
	for _, q := range quarters {
		// Calendar days of the server's time zone, also across a DST change.
		day := startOfDay(time.UnixMilli(q.Quarter * quarterHour).In(now.Location())).Format(time.DateOnly)
		perDay[day] += q.PlayedMs
	}
	out := model.WatchReport{ProfileID: row.ID, LimitMinutes: auth.ProfileLimits(row).Cap(session.Account.Limits).WatchLimitMinutes,
		Days: make([]model.WatchDay, 0, days)}
	for i := range days {
		date := first.AddDate(0, 0, i).Format(time.DateOnly)
		out.Days = append(out.Days, model.WatchDay{Date: date, Minutes: int(perDay[date] / time.Minute.Milliseconds())})
	}
	return out, nil
}

// quarterHour is the bucket of SumPlayedMsByQuarterHour in milliseconds.
const quarterHour = int64(15 * time.Minute / time.Millisecond)

// sweepWatchLimits is the K-5 backstop of the sweep: a profile whose daily watch time is used up
// and that still has running playback sessions (started in a race with the progress event that
// reached the limit, or played by a client that sends no progress events while another one used
// up the limit) gets them revoked (stopped_by limit). Played time is only known from progress
// events, so a client that never reports cannot use up the limit by itself.
func (s *Service) sweepWatchLimits(ctx context.Context, now time.Time) {
	ids, err := s.read.ListPlayingProfiles(ctx, now.UnixMilli())
	if err != nil {
		s.log.WarnContext(ctx, "list playing profiles", "err", err)
		return
	}
	for _, id := range ids {
		prof, err := s.read.GetProfile(ctx, id)
		if err != nil {
			continue // deleted meanwhile; its sessions end with it
		}
		acc, err := s.read.GetAccount(ctx, prof.AccountID)
		if err != nil {
			continue
		}
		limit := auth.ProfileLimits(prof).Cap(auth.AccountLimits(acc)).WatchLimitMinutes
		if limit <= 0 {
			continue
		}
		wt, err := s.watchTime(ctx, id, limit, now)
		if err != nil {
			s.log.WarnContext(ctx, "watch time of playing profile", "profile_id", id, "err", err)
			continue
		}
		if wt.RemainingMinutes > 0 {
			continue
		}
		if err := s.endProfile(ctx, id, model.StoppedByLimit); err != nil {
			s.log.WarnContext(ctx, "end playback over the watch-time limit", "profile_id", id, "err", err)
			continue
		}
		s.log.InfoContext(ctx, "watch-time limit reached, playback ended by the sweep", "profile_id", id,
			"limit_minutes", wt.LimitMinutes, "used_minutes", wt.UsedMinutes)
	}
}
