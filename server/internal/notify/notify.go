// Package notify keeps the in-app inbox of the profiles (plan 3.1 notify, 5.5, M7 E-8). A new
// episode of a series or podcast that a profile marked as favourite becomes one entry in that
// profile's inbox, but only when the profile may see the episode: the decision runs through
// package visibility before anything is stored or sent. Channels (e-mail, push, Telegram) follow
// in M8 behind the Dispatcher interface; the default dispatcher does nothing.
package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// KindNewEpisode is the kind of an entry about a new episode.
const KindNewEpisode = "new_episode"

// MaxPage is the largest page of the inbox (the contract's limit).
const MaxPage = 200

// Dispatcher sends an inbox entry on further channels (M8). It is called once per new entry,
// after the entry is stored, and only for a profile that may see the episode.
type Dispatcher interface {
	Dispatch(ctx context.Context, n model.Notification, profile model.Profile) error
}

type noopDispatcher struct{}

func (noopDispatcher) Dispatch(context.Context, model.Notification, model.Profile) error { return nil }

// Service implements the inbox (api.NotifyService) and the new-episode hook of packages library
// and metadata.
type Service struct {
	write    *systemq.Queries
	read     *systemq.Queries
	vis      *visibility.Reader
	dispatch Dispatcher
	log      *slog.Logger
	now      func() time.Time
}

// Option configures New.
type Option func(*Service)

// WithDispatcher sets the channel dispatcher (default: none).
func WithDispatcher(d Dispatcher) Option {
	return func(s *Service) {
		if d != nil {
			s.dispatch = d
		}
	}
}

// WithClock replaces time.Now (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New returns the service.
func New(db *store.DB, vis *visibility.Reader, log *slog.Logger, opts ...Option) *Service {
	s := &Service{write: systemq.New(db.Write), read: systemq.New(db.Read), vis: vis, dispatch: noopDispatcher{}, log: log, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// NewEpisodes records new series and podcast episodes: for every profile with a favourite on the
// episode's series or podcast that may see the episode, one inbox entry (a second call for the
// same episode adds nothing) and one call of the dispatcher. Unknown ids and other kinds are
// skipped. A dispatcher error is logged; the entry stays.
func (s *Service) NewEpisodes(ctx context.Context, episodeIDs []string) error {
	var errs []error
	for _, id := range episodeIDs {
		if err := s.newEpisode(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) newEpisode(ctx context.Context, episodeID string) error {
	ep, err := s.read.NotifyEpisodeParent(ctx, episodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("parent of episode %s: %w", episodeID, err)
	}
	if ep.ParentID == "" {
		return nil
	}
	profiles, err := s.read.ListFavoriteProfiles(ctx, ep.ParentID)
	if err != nil {
		return fmt.Errorf("favourites of %s: %w", ep.ParentID, err)
	}
	var errs []error
	for _, row := range profiles {
		p := model.Profile{ID: row.ID, AccountID: row.AccountID, Name: row.Name, MaxRating: int(row.MaxRating), Language: row.Language}
		if err := s.notifyProfile(ctx, p, episodeID, ep.ParentID); err != nil {
			errs = append(errs, err)
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
