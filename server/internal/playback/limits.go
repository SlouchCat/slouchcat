package playback

import (
	"context"
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

// profileLocks serialises the starts of a profile with a stream limit, so the count and the insert
// of the new playback session cannot interleave.
type profileLocks struct {
	mu    sync.Mutex
	locks map[int64]*profileLock
}

type profileLock struct {
	mu    sync.Mutex
	users int
}

// lockProfile locks the starts of a profile and returns the unlock function.
func (s *Service) lockProfile(id int64) func() {
	s.profiles.mu.Lock()
	if s.profiles.locks == nil {
		s.profiles.locks = map[int64]*profileLock{}
	}
	l := s.profiles.locks[id]
	if l == nil {
		l = &profileLock{}
		s.profiles.locks[id] = l
	}
	l.users++
	s.profiles.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		s.profiles.mu.Lock()
		if l.users--; l.users == 0 {
			delete(s.profiles.locks, id)
		}
		s.profiles.mu.Unlock()
	}
}

// admit checks the limits of a start: a profile whose watch time of today is used up gets
// *model.WatchLimitError, a profile that runs its maximum of streams *model.StreamLimitError. A
// playback of the same browser session and title (a track or quality switch) does not count. It
// returns the watch time of today when the profile has a daily limit.
func (s *Service) admit(ctx context.Context, session model.Session, itemID string, limits model.PlaybackLimits, now time.Time) (*model.WatchTime, error) {
	var watch *model.WatchTime
	if limits.WatchLimitMinutes > 0 {
		wt, err := s.watchTime(ctx, session.Profile.ID, limits.WatchLimitMinutes, now)
		if err != nil {
			return nil, err
		}
		if wt.RemainingMinutes == 0 {
			return nil, &model.WatchLimitError{LimitMinutes: wt.LimitMinutes, UsedMinutes: wt.UsedMinutes}
		}
		watch = &wt
	}
	if limits.MaxStreams > 0 {
		n, err := s.read.CountActivePlaybackSessions(ctx, appq.CountActivePlaybackSessionsParams{
			ProfileID: session.Profile.ID, Now: now.UnixMilli(), ActiveAfter: now.Add(-ActiveWindow).UnixMilli(),
			SessionID: session.ID, ItemID: itemID,
		})
		if err != nil {
			return nil, fmt.Errorf("count active playback sessions: %w", err)
		}
		if int(n) >= limits.MaxStreams {
			return nil, &model.StreamLimitError{MaxStreams: limits.MaxStreams, ActiveStreams: int(n)}
		}
	}
	return watch, nil
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
	out := model.WatchReport{ProfileID: row.ID, LimitMinutes: auth.ProfileLimits(row).Cap(session.Account.Limits).WatchLimitMinutes,
		Days: make([]model.WatchDay, 0, days)}
	for i := days - 1; i >= 0; i-- {
		from := today.AddDate(0, 0, -i)
		to := from.AddDate(0, 0, 1) // calendar days, also across a DST change
		ms, err := s.playedMs(ctx, row.ID, from, to)
		if err != nil {
			return model.WatchReport{}, err
		}
		out.Days = append(out.Days, model.WatchDay{Date: from.Format(time.DateOnly), Minutes: int(ms / time.Minute.Milliseconds())})
	}
	return out, nil
}
