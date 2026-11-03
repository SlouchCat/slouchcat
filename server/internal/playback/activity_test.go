package playback

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 11: on shutdown WaitActivity waits for the start events still being written (bounded by
// its context) and drops every later one, so no write reaches a closed database.
func TestWaitActivityDrainsStarts(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	release := make(chan struct{})
	var written atomic.Int32
	s := New(f.DB, vistest.Logger()).WithActivity(func(context.Context, model.ActivityEvent) {
		<-release
		written.Add(1)
	})
	ctx := context.Background()
	session := f.Session(t, f.P18)
	if _, err := s.Start(ctx, session, f.Rated(12).ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	// Bounded: the write still hangs, WaitActivity returns with its context.
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	s.WaitActivity(short)
	if written.Load() != 0 {
		t.Fatal("the hanging write finished early")
	}
	close(release)
	s.WaitActivity(ctx)
	if written.Load() != 1 {
		t.Errorf("writes after the drain = %d, want 1", written.Load())
	}
	// After the drain a start still plays, but writes nothing.
	if _, err := s.Start(ctx, session, f.Rated(6).ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	s.WaitActivity(ctx)
	if written.Load() != 1 {
		t.Errorf("a start after the drain was written")
	}
}
