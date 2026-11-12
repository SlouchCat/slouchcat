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
