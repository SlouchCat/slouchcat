// Package notify keeps the in-app inbox of the profiles (plan 3.1 notify, 5.5, M7 E-8) and sends
// notifications on the channels of the accounts (M8, A-6, A-7): e-mail, Web Push and Telegram.
//
// A new episode of a series or podcast that a profile marked as favourite becomes one entry in that
// profile's inbox, but only when the profile may see the episode: the decision runs through
// package visibility before anything is stored or sent. The Dispatcher hands the entry on to the
// channels; the default dispatcher is the service itself (event new_content, prefixed with the
// profile name).
//
// Channels (design 3.2): Emit turns an event into one row in deliveries per account and switched-on
// channel that chose the event, and one job notify.deliver per row; the job renders and sends it
// through the senders of the subpackages email, push and telegram (retries are the queue's). The
// event errors goes to the main account only, once per source and hour. Announcements of the main
// user go to every account that chose them and to the start dialog of every profile. The server-side
// settings (SMTP server, bot token, VAPID pair, public URL) come through Config; without it no
// channel is available.
package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// KindNewEpisode is the kind of an entry about a new episode.
const KindNewEpisode = "new_episode"

// MaxPage is the largest page of the inbox (the contract's limit).
const MaxPage = 200

// Dispatcher sends an inbox entry on further channels (M8). It is called once per new entry,
// after the entry is stored, and only for a profile that may see the episode. It is called
// concurrently (a library scan and the podcast worker announce new episodes at the same time), so an
// implementation must be safe for concurrent use; the context of a call ends after the hook's
// timeout, also during a shutdown.
type Dispatcher interface {
	Dispatch(ctx context.Context, n model.Notification, profile model.Profile) error
}

// Service implements the inbox and the channels (api.NotifyService), the new-episode hook of
// packages library and metadata, the event hooks of auth, jobs and library and the announcements of
// the administration.
type Service struct {
	write    *systemq.Queries
	read     *systemq.Queries
	vis      *visibility.Reader
	dispatch Dispatcher
	log      *slog.Logger
	now      func() time.Time

	// Channels (M8). cfg is set once by SetConfig before the service runs; nil = no channel.
	cfg      Config
	queue    *jobs.Queue // nil = events reach no channel
	mailer   Mailer
	pusher   Pusher
	telegram TelegramAPI
	devPush  bool                        // development: http push endpoints and private addresses
	linkPoll time.Duration               // interval of the Telegram link poller
	attempt  func(context.Context) error // counts a test message against the client's limit; nil = none

	vapidMu  sync.Mutex // one VAPID pair, also when two requests need it first
	errorsMu sync.Mutex // serialises the hourly dedupe of the event errors
	botMu    sync.Mutex
	bot      botName // cached username of the bot token
	tgOffset int64   // next getUpdates offset (only the poller goroutine)
}

// Option configures New.
type Option func(*Service)

// WithDispatcher replaces the channel dispatcher (default: the service's own channels; tests).
func WithDispatcher(d Dispatcher) Option {
	return func(s *Service) {
		if d != nil {
			s.dispatch = d
		}
	}
}

// WithAttemptLimit counts every test message (TestChannel) with allow, the attempt limit of the
// client's address (main: auth.Service.AllowAttempt); its error, model.ErrTooManyAttempts, ends
// the test before anything is sent.
func WithAttemptLimit(allow func(context.Context) error) Option {
	return func(s *Service) { s.attempt = allow }
}

// WithClock replaces time.Now (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New returns the service. Without WithQueue and SetConfig it keeps the inbox only.
func New(db *store.DB, vis *visibility.Reader, log *slog.Logger, opts ...Option) *Service {
	s := &Service{write: systemq.New(db.Write), read: systemq.New(db.Read), vis: vis, log: log, now: time.Now,
		linkPoll: linkPollInterval}
	s.dispatch = s
	for _, o := range opts {
		o(s)
	}
	if s.mailer == nil {
		s.mailer = smtpMailer{}
	}
	if s.pusher == nil {
		s.pusher = webPusher{allowPrivate: s.devPush}
	}
	if s.telegram == nil {
		s.telegram = &telegram.Client{}
	}
	return s
}

// MaxPerParent caps the episodes of one series or podcast that one NewEpisodes call notifies (a
// feed that renumbers its guids or a scan of a whole season must not flood the inbox).
const MaxPerParent = 50

// targetBatch is the number of episode ids read in one query (well below SQLite's variable limit).
const targetBatch = 500

// NewEpisodes records new series and podcast episodes: for every profile with a favourite on the
// episode's series or podcast that may see the episode, one inbox entry (a second call for the
// same episode adds nothing) and one call of the dispatcher. Unknown ids and other kinds are
// skipped; per series or podcast at most MaxPerParent episodes of one call notify, in the order of
// episodeIDs. Without any favourite it reads nothing else. A dispatcher error is logged; the entry
// stays. Library scans and podcast fetches call it concurrently.
func (s *Service) NewEpisodes(ctx context.Context, episodeIDs []string) error {
	if len(episodeIDs) == 0 {
		return nil
	}
	has, err := s.read.NotifyHasFavorites(ctx)
	if err != nil {
		return fmt.Errorf("any favourite: %w", err)
	}
	if !has {
		return nil
	}
	perParent := map[string]int{}
	var errs []error
	for chunk := range slices.Chunk(episodeIDs, targetBatch) {
		rows, err := s.read.NotifyFavoriteTargets(ctx, chunk)
		if err != nil {
			return errors.Join(append(errs, fmt.Errorf("favourites of new episodes: %w", err))...)
		}
		targets := make(map[string][]systemq.NotifyFavoriteTargetsRow, len(rows))
		for _, r := range rows {
			targets[r.EpisodeID] = append(targets[r.EpisodeID], r)
		}
		for _, id := range chunk {
			rows := targets[id]
			if len(rows) == 0 {
				continue
			}
			delete(targets, id) // an id passed twice notifies once
			parent := rows[0].ParentID
			if perParent[parent]++; perParent[parent] > MaxPerParent {
				continue
			}
			for _, row := range rows {
				p := model.Profile{ID: row.ID, AccountID: row.AccountID, Name: row.Name, MaxRating: int(row.MaxRating), Language: row.Language}
				if err := s.notifyProfile(ctx, p, id, parent); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	for parent, n := range perParent {
		if n > MaxPerParent {
			s.log.InfoContext(ctx, "new episodes over the cap not notified", "parent_id", parent, "episodes", n, "cap", MaxPerParent)
		}
	}
	return errors.Join(errs...)
}

// notifyProfile stores and dispatches the entry of one profile when the profile sees the episode
// (plan 5.5: invisible means nothing, neither an entry nor a message).
func (s *Service) notifyProfile(ctx context.Context, p model.Profile, episodeID, parentID string) error {
	scope := visibility.Profile(p)
	if err := s.vis.Visible(ctx, scope, episodeID); errors.Is(err, model.ErrNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("visibility of %s for profile %d: %w", episodeID, p.ID, err)
	}
	now := s.now()
	res, err := s.write.NotifyInsert(ctx, systemq.NotifyInsertParams{
		ProfileID: p.ID, Kind: KindNewEpisode, ItemID: episodeID, ParentID: sql.NullString{String: parentID, Valid: true},
		Now: now.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("store notification: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return err // already notified
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("notification id: %w", err)
	}
	n, err := s.entry(ctx, scope, systemq.NotifyListRow{ID: id, Kind: KindNewEpisode, ItemID: episodeID,
		ParentID: sql.NullString{String: parentID, Valid: true}, CreatedAt: now.UnixMilli()})
	if err != nil {
		return err
	}
	n.ProfileID = p.ID
	if err := s.dispatch.Dispatch(ctx, n, p); err != nil {
		s.log.WarnContext(ctx, "dispatch notification", "notification_id", id, "profile_id", p.ID, "err", err)
	}
	return nil
}

// Notifications returns one page of the profile's inbox, newest first, with the total and the
// unread count; entries whose episode the profile may no longer see are left out.
func (s *Service) Notifications(ctx context.Context, profile model.Profile, offset, limit int) ([]model.Notification, int, int, error) {
	limit = max(1, min(limit, MaxPage))
	offset = max(0, offset)
	count, err := s.read.NotifyCount(ctx, profile.ID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("count notifications: %w", err)
	}
	rows, err := s.read.NotifyList(ctx, systemq.NotifyListParams{ProfileID: profile.ID, OffsetCount: int64(offset), LimitCount: int64(limit)})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list notifications: %w", err)
	}
	scope := visibility.Profile(profile)
	out := make([]model.Notification, 0, len(rows))
	for _, r := range rows {
		n, err := s.entry(ctx, scope, r)
		if errors.Is(err, model.ErrNotFound) {
			continue // hidden between the two reads
		}
		if err != nil {
			return nil, 0, 0, err
		}
		n.ProfileID = profile.ID
		out = append(out, n)
	}
	return out, int(count.Total), int(count.Unread), nil
}

// entry builds an inbox entry with the summaries of the episode and, when visible, its parent.
func (s *Service) entry(ctx context.Context, scope visibility.Scope, r systemq.NotifyListRow) (model.Notification, error) {
	item, err := s.vis.Summary(ctx, scope, r.ItemID)
	if err != nil {
		return model.Notification{}, err
	}
	n := model.Notification{ID: r.ID, Kind: r.Kind, CreatedAt: time.UnixMilli(r.CreatedAt).UTC(), Item: item}
	if r.ReadAt.Valid {
		t := time.UnixMilli(r.ReadAt.Int64).UTC()
		n.ReadAt = &t
	}
	if r.ParentID.Valid {
		parent, err := s.vis.Summary(ctx, scope, r.ParentID.String)
		switch {
		case err == nil:
			n.Parent = &parent
		case !errors.Is(err, model.ErrNotFound):
			return model.Notification{}, err
		}
	}
	return n, nil
}

// MarkRead marks one entry of the profile read; model.ErrNotFound for an unknown entry, another
// profile's entry or one whose episode the profile may no longer see.
func (s *Service) MarkRead(ctx context.Context, profile model.Profile, id int64) error {
	n, err := s.write.NotifyMarkRead(ctx, systemq.NotifyMarkReadParams{
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: id, ProfileID: profile.ID,
	})
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// MarkAllRead marks every unread entry of the profile read.
func (s *Service) MarkAllRead(ctx context.Context, profile model.Profile) error {
	if err := s.write.NotifyMarkAllRead(ctx, systemq.NotifyMarkAllReadParams{
		Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ProfileID: profile.ID,
	}); err != nil {
		return fmt.Errorf("mark notifications read: %w", err)
	}
	return nil
}
