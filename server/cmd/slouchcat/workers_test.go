package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// Concept 14 F-2: the pool index has exactly one worker, index-1, next to the general workers; it
// never runs two jobs at once and reports a final failure like the general workers.
func TestIndexWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storetest.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	queue := jobs.NewQueue(db.Write, nil)
	registry := jobs.NewRegistry(queue, log)

	var mu sync.Mutex
	running, most := 0, 0
	registry.Register(jobs.Type{Name: "test.read", Group: jobs.GroupLibrary, Pool: jobs.PoolIndex, Trigger: jobs.TriggerEvent, Priority: 1,
		Handler: func(context.Context, *jobs.Job) error {
			mu.Lock()
			running++
			most = max(most, running)
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			return nil
		}})
	errRead := errors.New("read failed")
	registry.Register(jobs.Type{Name: "test.broken", Group: jobs.GroupLibrary, Pool: jobs.PoolIndex, Trigger: jobs.TriggerEvent, Priority: 1,
		Handler: func(context.Context, *jobs.Job) error { return errRead }})
	if err := jobs.NewScheduler(queue, registry, log).Start(ctx); err != nil {
		t.Fatal(err)
	}

	counts := poolWorkers(3)
	if counts[jobs.PoolIndex] != 1 || counts[jobs.PoolGeneral] != 3 {
		t.Fatalf("workers per pool = %v, want one index worker next to three general ones", counts)
	}
	failed := make(chan jobs.Job, 4)
	var wg sync.WaitGroup
	startWorkers(ctx, &wg, queue, registry, counts, log, func(_ context.Context, j jobs.Job, err error) {
		if errors.Is(err, errRead) {
			failed <- j
		}
	})
	defer wg.Wait()
	defer cancel()

	var ids []int64
	for i := range 4 {
		id, err := queue.Enqueue(ctx, jobs.NewJob{Type: "test.read", IdempotencyKey: fmt.Sprintf("read:%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	broken, err := queue.Enqueue(ctx, jobs.NewJob{Type: "test.broken", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case j := <-failed:
		if j.ID != broken || j.Type != "test.broken" {
			t.Errorf("final failure of %+v, want job %d", j, broken)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("index-1 did not report the final failure")
	}
	deadline := time.Now().Add(10 * time.Second)
	for _, id := range ids {
		for {
			j, err := queue.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if j.State == jobs.StateDone {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("job %d stays %s", id, j.State)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if most != 1 {
		t.Errorf("index jobs at once = %d, want 1", most)
	}
}
