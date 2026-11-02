package admin

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 11 (11.5): restartServer schedules the restart like a restore (the fake runs after
// about a second), answers the announced time and records server_restart with the account; a
// second restart and a restore answer restart_pending; without a restart function it is not
// configured.
func TestRestartServer(t *testing.T) {
	a := newActivityFixture(t)
	a.svc.WithBackups(t.TempDir(), "test")
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("without WithRestart = %v, want ErrNotConfigured", err)
	}
	backup, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var restarts atomic.Int32
	a.svc.WithRestart(func() { restarts.Add(1) })

	res, err := a.svc.RestartServer(a.ctx, a.f.Account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.RestartAt.Equal(a.now.Add(restartAnnounced)) {
		t.Errorf("restartAt = %v, want %v", res.RestartAt, a.now.Add(restartAnnounced))
	}
	if n := a.count(t, `kind = 'server_restart' AND account_id = ? AND profile_id IS NULL`, a.f.Account.ID); n != 1 {
		t.Errorf("server_restart rows of the account = %d, want 1", n)
	}
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); !errors.Is(err, model.ErrRestartPending) {
		t.Errorf("second restart = %v, want ErrRestartPending", err)
	}
	if _, err := a.svc.RestoreBackup(a.ctx, backup.Name, ""); !errors.Is(err, model.ErrRestartPending) {
		t.Errorf("restore while a restart is pending = %v, want ErrRestartPending", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for restarts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if restarts.Load() != 1 {
		t.Errorf("restart function called %d times, want 1", restarts.Load())
	}
	if n := a.count(t, `kind = 'server_restart'`); n != 1 {
		t.Errorf("server_restart rows = %d, want 1 (the refused ones write nothing)", n)
	}
}

// A backup or restore holds the lock: the restart would cut it off and answers backup_busy, and
// nothing is scheduled or recorded.
func TestRestartServerWhileBackupRuns(t *testing.T) {
	a := newActivityFixture(t)
	a.svc.WithBackups(t.TempDir(), "test")
	var restarts atomic.Int32
	a.svc.WithRestart(func() { restarts.Add(1) })
	a.svc.backups.mu.Lock()
	_, err := a.svc.RestartServer(a.ctx, a.f.Account.ID)
	a.svc.backups.mu.Unlock()
	if !errors.Is(err, model.ErrBackupBusy) {
		t.Fatalf("restart during a backup = %v, want ErrBackupBusy", err)
	}
	if a.svc.backups.restartPending.Load() || a.count(t, `kind = 'server_restart'`) != 0 {
		t.Error("a refused restart left a pending restart or a log row")
	}
	// Once the backup is done the restart goes through.
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); err != nil {
		t.Errorf("restart after the backup = %v", err)
	}
}

// A restore that scheduled its restart blocks a restart and a second restore.
func TestRestoreSetsRestartPending(t *testing.T) {
	a := newActivityFixture(t)
	a.svc.WithBackups(t.TempDir(), "test")
	backup, err := a.svc.CreateBackup(a.ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var restarts atomic.Int32
	a.svc.WithRestart(func() { restarts.Add(1) })
	if _, err := a.svc.RestoreBackup(a.ctx, backup.Name, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.svc.RestartServer(a.ctx, a.f.Account.ID); !errors.Is(err, model.ErrRestartPending) {
		t.Errorf("restart after a restore = %v, want ErrRestartPending", err)
	}
	if _, err := a.svc.RestoreBackup(a.ctx, backup.Name, ""); !errors.Is(err, model.ErrRestartPending) {
		t.Errorf("second restore = %v, want ErrRestartPending", err)
	}
}
