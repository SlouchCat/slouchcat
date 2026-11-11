package library

import (
	"context"
	"errors"
	"testing"
)

// The hourly library.sweep runs the image folder sweep of package metadata (WithImageFolders); a
// failure of it is logged and the sweep of the titles stands.
func TestSweepRunsTheImageFolderSweep(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	runs := 0
	WithImageFolders(func(context.Context) (int, error) {
		runs++
		return 0, errors.New("disk gone")
	}, nil)(e.svc)
	for range 2 {
		if err := e.svc.handleSweep(e.ctx, nil); err != nil {
			t.Fatal(err)
		}
	}
	if runs != 2 {
		t.Errorf("image folder sweep ran %d times, want 2", runs)
	}
}
