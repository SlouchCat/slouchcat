package radio

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Job types of the radio (concept 16 section 5.2, ADR 0025).

// JobCatalogSync is the daily sync of the Radio Browser catalog.
const JobCatalogSync = "radio.catalog.sync"

const (
	// catalogSyncPriority is below the scans (jobs ladder rung 4): the sync reads one list and
	// writes in batches; a scan the household waits for goes first.
	catalogSyncPriority = 4
	// catalogSyncEvery is the interval of the schedule.
	catalogSyncEvery = 24 * time.Hour
	// firstSyncDelay is the wait of the first sync after a start with an empty catalog (R-3).
	firstSyncDelay = 10 * time.Minute
)

// catalogSyncWindow is the window of the daily sync, 04:30 to 06:30 server time.
var catalogSyncWindow = jobs.Window{Start: 4*60 + 30, End: 6*60 + 30}

// RegisterTypes registers the job types of the radio.
func (s *Service) RegisterTypes(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobCatalogSync, Group: jobs.GroupMetadata, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: catalogSyncPriority, Handler: s.handleCatalogSync, Run: s.runCatalogSync, Every: catalogSyncEvery,
		Seed: s.catalogSeed,
	})
	r.Register(s.logoJobType())
}

// catalogSeed is the first schedule row: daily, on, in the window 04:30 to 06:30. No start delay:
// the first sync of an empty catalog is the one-off date of Start.
func (s *Service) catalogSeed(context.Context) (jobs.ScheduleSeed, error) {
	w := catalogSyncWindow
	return jobs.ScheduleSeed{Enabled: true, Window: &w}, nil
}

// runCatalogSync queues one sync with prio (Type.Run); a waiting one is raised to prio.
func (s *Service) runCatalogSync(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := enqueueCatalogSync(ctx, q, prio)
	if err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

func enqueueCatalogSync(ctx context.Context, q *jobs.Queue, prio int) (int64, error) {
	// One attempt: a failed sync waits for the retry dates of recordSyncFailure, not for the
	// queue's seconds of backoff.
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobCatalogSync, Priority: prio, IdempotencyKey: JobCatalogSync,
		Payload: []byte(`{}`), MaxAttempts: 1})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobCatalogSync, err)
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return 0, fmt.Errorf("raise %s: %w", JobCatalogSync, err)
	}
	return id, nil
}

// SyncCatalog queues the sync with manual priority (or names the queued or running one).
func (s *Service) SyncCatalog(ctx context.Context) (int64, error) {
	if s.o.queue == nil {
		return 0, errors.New("radio: no job queue")
	}
	return enqueueCatalogSync(ctx, s.o.queue, jobs.ManualPriority)
}

// activeSyncJob returns the id of the queued or running sync, 0 when none.
func (s *Service) activeSyncJob(ctx context.Context) (int64, error) {
	if s.o.queue == nil {
		return 0, nil
	}
	list, err := s.o.queue.ListActive(ctx, JobCatalogSync)
	if err != nil {
		return 0, fmt.Errorf("list active %s: %w", JobCatalogSync, err)
	}
	if len(list) == 0 {
		return 0, nil
	}
	return list[0].ID, nil
}

// Start loads the index and the offer, prepares the offer at the first start after migration
// 00048 (seedOffer) and, while the catalog never synced, sets the one-off date of the first sync
// ten minutes ahead (R-3) unless a sync is queued or a date is set. Call it after the scheduler
// started (its row must exist). No read of the app triggers a sync.
func (s *Service) Start(ctx context.Context) error {
	if _, err := s.index(ctx); err != nil {
		return err
	}
	if err := s.seedOffer(ctx); err != nil {
		return err
	}
	if _, err := s.offer(ctx); err != nil {
		return err
	}
	s.cat.bg.goFn(func(ctx context.Context) {
		s.removeStoredLargePNGs(ctx) // the PNGs of 256 and 512 px of earlier versions
		s.dropLogosOutsideOffer(ctx) // after the update the offer is empty: every fetched logo goes
	})
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return fmt.Errorf("get radio config: %w", err)
	}
	if cfg.StationsTotal != 0 || cfg.LastSyncAt.Valid || s.o.scheduler == nil {
		return nil
	}
	if id, err := s.activeSyncJob(ctx); err != nil || id != 0 {
		return err
	}
	sched, err := s.o.scheduler.Schedule(ctx, JobCatalogSync)
	if errors.Is(err, model.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get schedule of %s: %w", JobCatalogSync, err)
	}
	if !sched.OnceAt.IsZero() {
		return nil
	}
	at := s.now().Add(firstSyncDelay).In(s.o.scheduler.Location()).Format(jobs.OnceLayout)
	if _, err := s.o.scheduler.UpdateSchedule(ctx, JobCatalogSync, jobs.ScheduleUpdate{OnceLocal: &at}); err != nil {
		return fmt.Errorf("set the first radio sync: %w", err)
	}
	s.log.InfoContext(ctx, "first radio catalog sync requested", "at", at)
	return nil
}

// CatalogState returns the state of the catalog and its sync.
func (s *Service) CatalogState(ctx context.Context) (model.RadioCatalogState, error) {
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return model.RadioCatalogState{}, fmt.Errorf("get radio config: %w", err)
	}
	st := model.RadioCatalogState{Source: "radio-browser", LastSyncError: cfg.LastSyncError.String,
		SyncFailures: int(cfg.SyncFailures), EntriesTotal: int(cfg.EntriesTotal), EntriesReachable: int(cfg.EntriesReachable),
		StationsTotal: int(cfg.StationsTotal), LastAdded: int(cfg.LastAdded), LastRemoved: int(cfg.LastRemoved)}
	if cfg.LastSyncAt.Valid {
		st.LastSyncAt = time.UnixMilli(cfg.LastSyncAt.Int64).UTC()
	}
	if s.o.scheduler != nil {
		sched, err := s.o.scheduler.Schedule(ctx, JobCatalogSync)
		switch {
		case err == nil:
			st.NextRunAt = sched.NextRunAt
			if !sched.OnceAt.IsZero() && (!sched.Enabled || sched.OnceAt.Before(st.NextRunAt)) {
				st.NextRunAt = sched.OnceAt
			}
		case !errors.Is(err, model.ErrNotFound):
			return model.RadioCatalogState{}, fmt.Errorf("get schedule of %s: %w", JobCatalogSync, err)
		}
	}
	if st.RunningJobID, err = s.activeSyncJob(ctx); err != nil {
		return model.RadioCatalogState{}, err
	}
	return st, nil
}
