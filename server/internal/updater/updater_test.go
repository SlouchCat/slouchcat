package updater

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPhase(t *testing.T) {
	for _, tc := range []struct {
		p              Phase
		running, final bool
	}{
		{PhaseIdle, false, false},
		{PhaseChecking, true, false},
		{PhaseBackingUp, true, false},
		{PhasePulling, true, false},
		{PhaseVerifying, true, false},
		{PhaseRestarting, true, false},
		{PhaseWaiting, true, false},
		{PhaseDone, false, true},
		{PhaseRolledBack, false, true},
		{PhaseFailed, false, true},
		{Phase("unknown"), false, false},
	} {
		if got := tc.p.Running(); got != tc.running {
			t.Errorf("%s.Running() = %v, want %v", tc.p, got, tc.running)
		}
		if got := tc.p.Final(); got != tc.final {
			t.Errorf("%s.Final() = %v, want %v", tc.p, got, tc.final)
		}
	}
}

func TestStateJSON(t *testing.T) {
	// An idle state carries only its phase: zero times and empty fields stay out.
	data, err := json.Marshal(State{Phase: PhaseIdle})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != `{"phase":"idle"}` {
		t.Errorf("idle state = %s", got)
	}

	started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	s := State{
		Phase: PhaseFailed, From: "1.15.0", To: "1.16.0", StartedAt: started,
		FinishedAt: started.Add(3 * time.Minute), Migrated: true, Backup: "pre-migration-00037.db",
		Code: CodeMigratedNoRollback, Message: "migration ran",
	}
	data, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"phase":"failed"`, `"from":"1.15.0"`, `"to":"1.16.0"`,
		`"startedAt":"2026-11-07T10:00:00Z"`, `"finishedAt":"2026-11-07T10:03:00Z"`, `"migrated":true`,
		`"backup":"pre-migration-00037.db"`, `"code":"migrated_no_rollback"`, `"message":"migration ran"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("state JSON %s lacks %s", data, key)
		}
	}
	var back State
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back != s {
		t.Errorf("round trip = %+v, want %+v", back, s)
	}
}

func TestServerRecordJSON(t *testing.T) {
	r := ServerRecord{Version: "1.16.0", SchemaBefore: 37, StartedAt: time.Date(2026, 11, 7, 10, 1, 0, 0, time.UTC)}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != `{"version":"1.16.0","schemaBefore":37,"startedAt":"2026-11-07T10:01:00Z"}` {
		t.Errorf("record before the migrations = %s", got)
	}
	r.SchemaAfter, r.Backup, r.MigratedAt = 38, "pre-migration-00038.db", r.StartedAt.Add(time.Second)
	data, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"schemaAfter":38`, `"backup":"pre-migration-00038.db"`, `"migratedAt":"2026-11-07T10:01:01Z"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("record JSON %s lacks %s", data, key)
		}
	}
}

// Migrated is the updater's reading of server.json: a changed schema, or migrations that started
// and never finished, count as migrated; a finished start on the same schema does not.
func TestServerRecordMigrated(t *testing.T) {
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		rec  ServerRecord
		want bool
	}{
		{"migrations not through", ServerRecord{SchemaBefore: 37}, true},
		{"schema changed", ServerRecord{SchemaBefore: 37, SchemaAfter: 38, MigratedAt: at}, true},
		{"same schema", ServerRecord{SchemaBefore: 38, SchemaAfter: 38, MigratedAt: at}, false},
	} {
		if got := tc.rec.Migrated(); got != tc.want {
			t.Errorf("%s: Migrated() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWriteReadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StateFile)
	want := State{Phase: PhasePulling, From: "1.15.0", To: "1.16.0"}
	if err := WriteJSON(path, want, 0o640); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o640 {
		t.Errorf("mode = %v, want 0640", got)
	}
	var got State
	if err := ReadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("read %+v, want %+v", got, want)
	}

	// Replacing keeps no temporary file behind and takes the new mode.
	want.Phase = PhaseDone
	if err := WriteJSON(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(path, &got); err != nil || got.Phase != PhaseDone {
		t.Errorf("after replace: %+v, %v", got, err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("mode after replace: %v, %v", fi.Mode().Perm(), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want only %s", len(entries), StateFile)
	}
}

func TestWriteJSONFails(t *testing.T) {
	dir := t.TempDir()
	if err := WriteJSON(filepath.Join(dir, "missing", StateFile), State{}, 0o600); err == nil {
		t.Error("write into a missing directory succeeded")
	}
	if err := WriteJSON(filepath.Join(dir, StateFile), func() {}, 0o600); err == nil {
		t.Error("write of a value without JSON form succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("failed writes left %d entries", len(entries))
	}
}

func TestReadJSONFails(t *testing.T) {
	dir := t.TempDir()
	var s State
	if err := ReadJSON(filepath.Join(dir, StateFile), &s); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v, want fs.ErrNotExist", err)
	}

	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte(`{"phase":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(broken, &s); err == nil {
		t.Error("broken JSON read without error")
	}

	// Exactly MaxFileBytes is read; one byte more is refused.
	fits := filepath.Join(dir, "fits.json")
	pad := MaxFileBytes - len(`{"phase":"idle"}`)
	if err := os.WriteFile(fits, []byte(`{"phase":"idle"}`+strings.Repeat(" ", pad)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(fits, &s); err != nil || s.Phase != PhaseIdle {
		t.Errorf("file of MaxFileBytes: %+v, %v", s, err)
	}
	large := filepath.Join(dir, "large.json")
	if err := os.WriteFile(large, []byte(`{"phase":"idle"}`+strings.Repeat(" ", pad+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReadJSON(large, &s); !errors.Is(err, ErrTooLarge) {
		t.Errorf("file over MaxFileBytes: %v, want ErrTooLarge", err)
	}
}

// The other side can write the shared folder: a symbolic link, a FIFO or a folder at the path is
// refused at once, and a FIFO does not block the reader.
func TestReadJSONRefusesNonRegular(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"phase":"done"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "dir.json")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, fifo, sub} {
		done := make(chan error, 1)
		go func() {
			var s State
			done <- ReadJSON(path, &s)
		}()
		select {
		case err := <-done:
			if !errors.Is(err, ErrNotRegular) {
				t.Errorf("ReadJSON(%s) = %v, want ErrNotRegular", filepath.Base(path), err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("ReadJSON(%s) blocks", filepath.Base(path))
		}
	}
}

// A symbolic link planted at the path is replaced by the file; its target stays as it was.
func TestWriteJSONOwnedReplacesLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, StateFile)
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSONOwned(path, State{Phase: PhaseIdle}, 0o644, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
		t.Fatalf("state.json = %v, %v, want a regular file 0644", info, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
		t.Errorf("link target = %q, %v, want it untouched", data, err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("link target mode = %v, %v, want 0600 as before", info, err)
	}
}

func TestValidRelease(t *testing.T) {
	for _, tc := range []struct {
		v  string
		ok bool
	}{
		{"1.16.0", true},
		{"0.0.1", true},
		{"10.20.30", true},
		{"v1.16.0", false},
		{"1.16", false},
		{"1.16.0.1", false},
		{"1.16.0-rc.1", false},
		{"1.16.0+build", false},
		{"1.16.0-3-gabc1234", false},
		{" 1.16.0", false},
		{"1.16.0\n", false},
		{"1.16.0\nSLOUCHCAT_X=1", false},
		{"01.16.0", false},
		{"1.16.99999999999999999999", false},
		{"1.١6.0", false}, // a non-ASCII digit
		{"", false},
	} {
		if got := ValidRelease(tc.v); got != tc.ok {
			t.Errorf("ValidRelease(%q) = %v, want %v", tc.v, got, tc.ok)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		candidate, running string
		want               bool
	}{
		{"1.16.0", "1.15.0", true},
		{"1.15.1", "1.15.0", true},
		{"2.0.0", "1.99.99", true},
		{"1.15.0", "1.15.0", false},
		{"1.14.0", "1.15.0", false},
		// A self-built image counts as its release (package version).
		{"1.16.0", "1.15.0-12-gabc1234", true},
		{"1.15.0", "1.15.0-12-gabc1234", false},
		{"1.15.0", "1.15.0-dirty", false},
		{"1.15.0", "v1.15.0", false},
		// A pre-release is older than its release.
		{"1.16.0", "1.16.0-rc.1", true},
		// Only plain releases are installed.
		{"1.16.0-rc.1", "1.15.0", false},
		{"v1.16.0", "1.15.0", false},
		{"1.16.0+build", "1.15.0", false},
		// A running version that does not parse is never updated.
		{"1.16.0", "dev", false},
		{"1.16.0", "", false},
	} {
		if got := Newer(tc.candidate, tc.running); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.candidate, tc.running, got, tc.want)
		}
	}
}
