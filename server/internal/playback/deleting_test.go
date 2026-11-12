package playback

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 20 (review finding 5): a title that a delete holds does not start; once the delete
// released it, it starts again.
func TestStartRefusesATitleBeingDeleted(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	deleting := model.NewDeletingSet()
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey)).WithDeleting(deleting)
	ctx := context.Background()
	m0 := f.Rated(0)
	release := deleting.Hold([]string{m0.ID})
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), m0.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrDeleting) {
		t.Fatalf("start while deleting = %v", err)
	}
	release()
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), m0.ID, model.PlaybackRequest{}); err != nil {
		t.Errorf("start after the delete released it: %v", err)
	}
}

// Review N2: a delete that begins between the check of Start and the new row holds the title; the
// recheck after the insert ends the new playback (stopped by the administration) and answers
// deleting.
func TestStartRechecksDeletingAfterTheRow(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	deleting := model.NewDeletingSet()
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey)).WithDeleting(deleting)
	ctx := context.Background()
	m0 := f.Rated(0)
	info, err := s.Start(ctx, browserSession(t, f, f.P18), m0.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.recheckDeleting(ctx, m0.ID, info.PlaybackSessionID); err != nil {
		t.Fatalf("recheck without a delete = %v", err)
	}
	defer deleting.Hold([]string{m0.ID})()
	if err := s.recheckDeleting(ctx, m0.ID, info.PlaybackSessionID); !errors.Is(err, model.ErrDeleting) {
		t.Fatalf("recheck while deleting = %v", err)
	}
	var by string
	if err := f.DB.Read.QueryRow(`SELECT COALESCE(stopped_by, '') FROM playback_sessions WHERE id = ? AND revoked_at IS NOT NULL`,
		info.PlaybackSessionID).Scan(&by); err != nil || by != model.StoppedByAdmin {
		t.Errorf("playback after the recheck: stopped_by %q, %v", by, err)
	}
}
