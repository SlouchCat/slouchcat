package main

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 11: the server records server_started with its version once it listens, and
// backup_restored only when store.Open swapped in a staged restore.
func TestStartupActivity(t *testing.T) {
	var got []model.ActivityEvent
	record := func(_ context.Context, e model.ActivityEvent) { got = append(got, e) }
	ctx := context.Background()

	recordRestored(ctx, record, false)
	if len(got) != 0 {
		t.Fatalf("a start without restore recorded %+v", got)
	}
	recordRestored(ctx, record, true)
	recordStarted(ctx, record, "1.7.0")
	if len(got) != 2 || got[0].Kind != model.ActivityBackupRestored ||
		got[1].Kind != model.ActivityServerStarted || got[1].Detail.Version != "1.7.0" {
		t.Errorf("recorded %+v", got)
	}
}
