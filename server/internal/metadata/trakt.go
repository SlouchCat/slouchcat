package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/trakt"
)

// Trakt lists (M7, AN-3, E-7, I-7). The job trakt.refresh (key "trakt", priority 2) fetches the
// top 100 of the four public lists (trending and popular, movies and shows) with the server's
// client id and replaces trakt_lists in one transaction. A failure empties the table, so the two
// start page rows disappear (I-7), and stores the error. RunTrakt queues the job once a day.
const (
	JobTraktRefresh = "trakt.refresh"
	traktJobKey     = "trakt"
	traktPriority   = 2

	// TraktInterval is the age of the last attempt after which RunTrakt queues the next run.
	TraktInterval = 24 * time.Hour
	// traktPollInterval is how often RunTrakt checks whether a run is due.
	traktPollInterval = 15 * time.Minute

	// Settings of the Trakt state (Unix ms as decimal text; the error as text).
	SettingTraktLastSuccess = "trakt_last_success"
	SettingTraktLastAttempt = "trakt_last_attempt"
	SettingTraktLastError   = "trakt_last_error"
)

// traktLists are the lists of one run in their storage order.
var traktLists = []struct{ list, media string }{
	{trakt.ListTrending, trakt.MediaMovie}, {trakt.ListTrending, trakt.MediaShow},
	{trakt.ListPopular, trakt.MediaMovie}, {trakt.ListPopular, trakt.MediaShow},
}

// traktConfig is the Trakt part of the service.
type traktConfig struct {
	clientID string // fixed client id (WithTrakt); "" asks KeySource.TraktClientID
	base     string
	http     *http.Client
	now      func() time.Time
	limiter  *ratelimit.Limiter // shared by every run (1 request per second, bursts of 2)
	agent    string
}

type traktOptions struct {
	base    string
	http    *http.Client
	now     func() time.Time
	limiter *ratelimit.Limiter
}

// TraktOption configures WithTrakt.
type TraktOption func(*traktOptions)

// WithTraktBaseURL replaces the Trakt API base URL (SLOUCHCAT_TRAKT_BASE_URL, mocks only); ""
// keeps https://api.trakt.tv.
func WithTraktBaseURL(base string) TraktOption {
	return func(o *traktOptions) { o.base = base }
}

// WithTraktHTTPClient replaces the HTTP client of the Trakt requests (tests).
func WithTraktHTTPClient(c *http.Client) TraktOption {
	return func(o *traktOptions) { o.http = c }
}

// WithTraktClock replaces the clock of the schedule and of the stored times (tests).
func WithTraktClock(now func() time.Time) TraktOption {
	return func(o *traktOptions) { o.now = now }
}

// WithTraktLimiter replaces the request limiter (tests).
func WithTraktLimiter(l *ratelimit.Limiter) TraktOption {
	return func(o *traktOptions) { o.limiter = l }
}

// WithTrakt configures the Trakt lists. clientID is a fixed client id (the environment,
// config.TraktClientID); "" means KeySource.TraktClientID is asked on every run. Without any
// client id no job runs and TraktStatus reports configured false.
func WithTrakt(clientID string, opts ...TraktOption) Option {
	return func(o *options) {
		o.traktID = clientID
		for _, opt := range opts {
			opt(&o.trakt)
		}
	}
}

func newTraktConfig(clientID string, o traktOptions, now func() time.Time, agent string) traktConfig {
	c := traktConfig{clientID: clientID, base: o.base, http: o.http, now: o.now, limiter: o.limiter, agent: agent}
	if c.now == nil {
		c.now = now
	}
	if c.limiter == nil {
		c.limiter = ratelimit.New("trakt", 1, 2)
	}
	return c
}

// traktClientID returns the client id: the fixed one, else the key source's; "" when none.
func (s *Service) traktClientID(ctx context.Context) (string, error) {
	if s.trakt.clientID != "" {
		return s.trakt.clientID, nil
	}
	if s.keys == nil {
		return "", nil
	}
	return s.keys.TraktClientID(ctx)
}

// RunTrakt queues trakt.refresh at the start and then checks every 15 minutes whether the last
// attempt is TraktInterval old, until ctx ends (E-7).
func (s *Service) RunTrakt(ctx context.Context) {
	t := time.NewTicker(traktPollInterval)
	defer t.Stop()
	for {
		if _, err := s.QueueDueTrakt(ctx); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "queue trakt refresh", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// QueueDueTrakt queues trakt.refresh when a client id is configured and the last attempt is
// TraktInterval old or there was none (one tick of RunTrakt). It reports whether it queued.
func (s *Service) QueueDueTrakt(ctx context.Context) (bool, error) {
	id, err := s.traktClientID(ctx)
	if err != nil || id == "" {
		return false, err
	}
	last, err := s.traktSettingTime(ctx, SettingTraktLastAttempt)
	if err != nil {
		return false, err
	}
	if last != nil && s.trakt.now().Sub(*last) < TraktInterval {
		return false, nil
	}
	return true, s.enqueueTrakt(ctx)
}

// RefreshTrakt queues trakt.refresh out of schedule (admin button);
// model.ErrProviderNotConfigured without a client id.
func (s *Service) RefreshTrakt(ctx context.Context) error {
	id, err := s.traktClientID(ctx)
	if err != nil {
		return err
	}
	if id == "" {
		return model.ErrProviderNotConfigured
	}
	return s.enqueueTrakt(ctx)
}

func (s *Service) enqueueTrakt(ctx context.Context) error {
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobTraktRefresh, Priority: traktPriority, IdempotencyKey: traktJobKey,
		Payload: []byte(`{}`)}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobTraktRefresh, err)
	}
	return nil
}

// TraktStatus returns whether a client id is configured, the times of the last success and
// attempt, the last error and the stored entries per list.
func (s *Service) TraktStatus(ctx context.Context) (model.TraktStatus, error) {
	id, err := s.traktClientID(ctx)
	if err != nil {
		return model.TraktStatus{}, err
	}
	st := model.TraktStatus{Configured: id != ""}
	if st.LastSuccess, err = s.traktSettingTime(ctx, SettingTraktLastSuccess); err != nil {
		return model.TraktStatus{}, err
	}
	if st.LastAttempt, err = s.traktSettingTime(ctx, SettingTraktLastAttempt); err != nil {
		return model.TraktStatus{}, err
	}
	if st.LastError, err = s.traktSetting(ctx, SettingTraktLastError); err != nil {
		return model.TraktStatus{}, err
	}
	counts, err := s.read.TraktCountLists(ctx)
	if err != nil {
		return model.TraktStatus{}, fmt.Errorf("count trakt lists: %w", err)
	}
	for _, c := range counts {
		switch c.List {
		case trakt.ListTrending:
			st.Trending = int(c.Entries)
		case trakt.ListPopular:
			st.Popular = int(c.Entries)
		}
	}
	return st, nil
}

// handleTraktRefresh runs one refresh. A run without client id does nothing. A failure has
// emptied the lists and stored the error; an unreachable Trakt is returned, so the queue retries
// with backoff, a refused client id is not (the next daily run or the admin tries again).
func (s *Service) handleTraktRefresh(ctx context.Context, _ *jobs.Job) error {
	id, err := s.traktClientID(ctx)
	if err != nil || id == "" {
		return err
	}
	err = s.refreshTraktLists(ctx, id)
	if errors.Is(err, trakt.ErrUnauthorized) {
		s.log.WarnContext(ctx, "trakt refused the client id; lists emptied", "err", err)
		return nil
	}
	return err
}

// refreshTraktLists fetches the four lists and stores them, or empties the table and records the
// error. The client id never reaches the log or the stored error.
func (s *Service) refreshTraktLists(ctx context.Context, clientID string) error {
	c := trakt.New(clientID, trakt.WithBaseURL(s.trakt.base), trakt.WithHTTPClient(s.trakt.http),
		trakt.WithLimiter(s.trakt.limiter), trakt.WithUserAgent(s.trakt.agent))
	attempt := s.trakt.now()
	var entries [][]trakt.Entry
	var fetchErr error
	for _, l := range traktLists {
		e, err := c.List(ctx, l.list, l.media, trakt.MaxLimit)
		if err != nil {
			fetchErr = fmt.Errorf("%s %s: %w", l.list, l.media, err)
			break
		}
		entries = append(entries, e)
	}
	if fetchErr != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	ms := attempt.UnixMilli()
	stamp := strconv.FormatInt(ms, 10)
	err := s.inTxApp(ctx, func(q *systemq.Queries, app *appq.Queries) error {
		if err := q.TraktDeleteLists(ctx); err != nil {
			return fmt.Errorf("empty trakt lists: %w", err)
		}
		if err := app.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingTraktLastAttempt, Value: stamp, Now: ms}); err != nil {
			return fmt.Errorf("store trakt attempt: %w", err)
		}
		if fetchErr != nil {
			if err := app.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingTraktLastError, Value: fetchErr.Error(), Now: ms}); err != nil {
				return fmt.Errorf("store trakt error: %w", err)
			}
			return nil
		}
		for i, list := range entries {
			for _, e := range list {
				if err := q.TraktInsertEntry(ctx, systemq.TraktInsertEntryParams{
					List: traktLists[i].list, Media: e.Media, Position: int64(e.Position), TmdbID: e.TMDBID,
					Title: nullStr(e.Title), Year: positive(int64(e.Year)), Now: ms,
				}); err != nil {
					return fmt.Errorf("store trakt entry: %w", err)
				}
			}
		}
		if err := app.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingTraktLastSuccess, Value: stamp, Now: ms}); err != nil {
			return fmt.Errorf("store trakt success: %w", err)
		}
		if err := app.DeleteSetting(ctx, SettingTraktLastError); err != nil {
			return fmt.Errorf("clear trakt error: %w", err)
		}
		return nil
	})
	if err != nil {
		return errors.Join(fetchErr, err)
	}
	if fetchErr != nil {
		return fmt.Errorf("trakt refresh: %w", fetchErr)
	}
	n := 0
	for _, l := range entries {
		n += len(l)
	}
	s.log.InfoContext(ctx, "trakt lists refreshed", "entries", n)
	return nil
}

// inTxApp runs fn with the system and app queries of one write transaction.
func (s *Service) inTxApp(ctx context.Context, fn func(q *systemq.Queries, app *appq.Queries) error) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(systemq.New(tx), appq.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// traktSetting reads a setting; "" when it is not set.
func (s *Service) traktSetting(ctx context.Context, key string) (string, error) {
	v, err := appq.New(s.db.Read).GetSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", key, err)
	}
	return v, nil
}

// traktSettingTime reads a time setting (Unix ms); nil when it is not set or not a number.
func (s *Service) traktSettingTime(ctx context.Context, key string) (*time.Time, error) {
	v, err := s.traktSetting(ctx, key)
	if err != nil || v == "" {
		return nil, err
	}
	ms, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, nil //nolint:nilerr // an unreadable time counts as never
	}
	t := time.UnixMilli(ms).UTC()
	return &t, nil
}
