package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The job seerr.users (concept 25, architecture 13): the mirror of the Seerr users with their
// rights, the limit fields as Seerr holds them and the creation time of each user, which is part
// of the fingerprint of the instance (architecture 7.4).

// JobUsers is the job type of the refresh of the Seerr users.
const JobUsers = "seerr.users"

const (
	usersEvery           = 24 * time.Hour
	usersPriority        = 4
	usersManualPriority  = jobs.ManualPriority
	usersPageSize        = 100
	usersMaxPages        = 100 // 10'000 users: far above a household
	maxDisplayNameLength = 200
)

// RegisterTypes registers the job types of the wishes.
func (s *Service) RegisterTypes(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobUsers, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: usersPriority, Handler: s.handleUsers, Run: s.runUsers, Every: usersEvery,
	})
	r.Register(jobs.Type{
		Name: JobProvision, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: provisionPriority, Handler: s.handleProvision,
	})
	// The outbox and the withdrawal: a person waits (architecture 13). Their own ladder lives in
	// wishes.next_attempt_at; a handler that waits returns jobs.RetryLater.
	r.Register(jobs.Type{
		Name: JobSubmit, Group: jobs.GroupDelivery, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: submitPrio, Handler: s.handleSubmit,
	})
	r.Register(jobs.Type{
		Name: JobWithdraw, Group: jobs.GroupDelivery, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: submitPrio, Handler: s.handleWithdraw,
	})
	// The sync with Seerr every 5 minutes and the reconcile with the library every hour; the
	// initial sync is a run of the sync type with its own key (architecture 7.2).
	r.Register(jobs.Type{
		Name: JobSync, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: syncPriority, Handler: s.handleSync, Run: s.runSync, Every: syncEvery,
	})
	r.Register(jobs.Type{
		Name: JobReconcile, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: reconcilePriority, Handler: s.handleReconcile, Run: s.runReconcile, Every: reconcileEvery,
	})
	s.registerArrTypes(r) // wishes.tag and wishes.progress (arraccess.go, tags.go, progress.go)
	// wishes.discover (discover.go)
	s.registerDiscoverType(r)
	// wishes.import: the take-over of requests made in Seerr (takeover.go)
	s.registerImportType(r)
}

func (s *Service) runUsers(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	id, err := s.queueUsers(ctx, prio)
	if err != nil {
		if errors.Is(err, model.ErrSeerrNotConfigured) {
			return nil, nil
		}
		return nil, err
	}
	return []int64{id}, nil
}

// queueUsers queues one refresh with prio, or raises the waiting one to prio.
func (s *Service) queueUsers(ctx context.Context, prio int) (int64, error) {
	if s.queue == nil {
		return 0, errors.New("wishes: no job queue")
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobUsers, Priority: prio, IdempotencyKey: JobUsers, Payload: []byte(`{}`)})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobUsers, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return 0, fmt.Errorf("raise %s: %w", JobUsers, err)
	}
	return id, nil
}

// RefreshUsers queues the refresh with manual priority («Benutzerliste neu laden»,
// refreshSeerrUsers); model.ErrSeerrNotConfigured without address or key.
func (s *Service) RefreshUsers(ctx context.Context) error {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Configured() {
		return model.ErrSeerrNotConfigured
	}
	_, err = s.queueUsers(ctx, usersManualPriority)
	return err
}

// handleUsers runs one refresh. Without a connection or with Seerr switched off it does nothing.
// A refused key ends the run without an error (the state shows it); an unreachable Seerr fails the
// attempt, so the queue tries again. Another instance stops the run before anything is written.
func (s *Service) handleUsers(ctx context.Context, _ *jobs.Job) error {
	_, c, err := s.connection(ctx, false)
	if errors.Is(err, model.ErrSeerrNotConfigured) || errors.Is(err, errDisabled) {
		return nil
	}
	if err != nil {
		return err
	}
	err = s.refreshUsers(ctx, c.AsAdmin())
	if !errors.Is(err, model.ErrInstanceChanged) {
		s.recordOutcome(ctx, seerrErr(err))
	}
	if err == nil && s.queue != nil {
		// Accounts that wait for Seerr and a missing shared user are tried again (18.2, N-101).
		if qerr := s.queueAfterUsers(ctx); qerr != nil {
			s.log.WarnContext(ctx, "seerr provisioning not queued after the refresh", "err", qerr)
		}
		s.refreshQuotas(ctx, c.AsAdmin()) // the limit state of every mapped user (architecture 19.3)
	}
	switch {
	case err == nil, errors.Is(err, model.ErrInstanceChanged), errors.Is(err, seerr.ErrKeyInvalid):
		return nil
	}
	return err
}

// seerrErr is err when it came from Seerr, nil otherwise (a local error says nothing about the
// connection).
func seerrErr(err error) error {
	var se *seerr.Error
	if errors.As(err, &se) {
		return err
	}
	return nil
}

func (s *Service) refreshUsers(ctx context.Context, admin seerr.Session) error {
	if pending, err := s.instancePending(ctx); err != nil {
		return err
	} else if pending != nil {
		return model.ErrInstanceChanged
	}
	me, err := admin.Me(ctx)
	if err != nil {
		return err
	}
	if same, err := s.checkInstance(ctx, me); err != nil {
		return err
	} else if !same {
		return model.ErrInstanceChanged
	}
	var users []seerr.User
	for page := 0; page < usersMaxPages; page++ {
		p, err := admin.Users(ctx, seerr.UserQuery{Take: usersPageSize, Skip: page * usersPageSize, Sort: "created"})
		if err != nil {
			return err
		}
		users = append(users, p.Results...)
		if len(p.Results) < usersPageSize || len(users) >= p.PageInfo.Results {
			break
		}
	}
	// A known user whose creation time changed belongs to another instance.
	known, err := s.read.ListSeerrUserCreatedAt(ctx)
	if err != nil {
		return fmt.Errorf("list seerr users: %w", err)
	}
	created := make(map[int64]int64, len(known))
	for _, k := range known {
		created[k.SeerrID] = k.CreatedAt.Int64
	}
	for _, u := range users {
		if at, ok := created[int64(u.ID)]; ok && !u.CreatedAt.IsZero() && at != u.CreatedAt.UnixMilli() {
			return s.markChanged(ctx, me)
		}
	}
	fetched := s.now().UnixMilli()
	skipped := 0
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		for _, u := range users {
			p, ok := upsertParams(u, fetched)
			if !ok {
				skipped++
				continue
			}
			if err := q.UpsertSeerrUser(ctx, p); err != nil {
				return fmt.Errorf("store seerr user %d: %w", u.ID, err)
			}
		}
		gone, err := q.MarkSeerrUsersGone(ctx, wishq.MarkSeerrUsersGoneParams{Now: sql.NullInt64{Int64: fetched, Valid: true}, FetchedAt: fetched})
		if err != nil {
			return fmt.Errorf("mark gone seerr users: %w", err)
		}
		if gone > 0 {
			s.log.InfoContext(ctx, "seerr users gone", "count", gone)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if skipped > 0 {
		s.log.WarnContext(ctx, "seerr users with an unknown user type skipped", "count", skipped)
	}
	s.log.InfoContext(ctx, "seerr users refreshed", "count", len(users)-skipped)
	return nil
}

func (s *Service) markChanged(ctx context.Context, me seerr.User) error {
	if err := s.markInstanceChanged(ctx, fingerprintOf(me)); err != nil {
		return err
	}
	return model.ErrInstanceChanged
}

// upsertParams maps a Seerr user to its row; false for a user type the table does not know or a
// permission mask beyond the column.
func upsertParams(u seerr.User, fetched int64) (wishq.UpsertSeerrUserParams, bool) {
	if u.ID < 1 || u.UserType < seerr.UserPlex || u.UserType > seerr.UserEmby {
		if u.Permissions > math.MaxInt64 {
			return wishq.UpsertSeerrUserParams{}, false
		}
		return wishq.UpsertSeerrUserParams{}, false
	}
	name := strings.TrimSpace(u.DisplayName)
	for _, alt := range []string{u.Username, u.JellyfinUsername, u.PlexUsername, u.Email} {
		if name == "" {
			name = strings.TrimSpace(alt)
		}
	}
	if name == "" {
		name = fmt.Sprintf("#%d", u.ID)
	}
	if r := []rune(name); len(r) > maxDisplayNameLength {
		name = string(r[:maxDisplayNameLength])
	}
	var createdAt sql.NullInt64
	if !u.CreatedAt.IsZero() {
		createdAt = sql.NullInt64{Int64: u.CreatedAt.UnixMilli(), Valid: true}
	}
	isAdmin := int64(0)
	if u.Permissions.Has(seerr.PermManageUsers) {
		isAdmin = 1 // ADMIN grants MANAGE_USERS too: no limit, no password sync (N-6, N-34)
	}
	if u.Permissions > math.MaxInt64 {
		return wishq.UpsertSeerrUserParams{}, false
	}
	return wishq.UpsertSeerrUserParams{
		SeerrID: int64(u.ID), DisplayName: name, Email: nullString(u.Email), UserType: int64(u.UserType),
		Permissions:     int64(u.Permissions),
		MovieQuotaLimit: nullInt(u.MovieQuotaLimit),
		MovieQuotaDays:  nullInt(u.MovieQuotaDays), TvQuotaLimit: nullInt(u.TVQuotaLimit),
		TvQuotaDays: nullInt(u.TVQuotaDays), IsAdmin: isAdmin, CreatedAt: createdAt, FetchedAt: fetched,
	}, true
}
