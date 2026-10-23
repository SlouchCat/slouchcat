package library

import (
	"context"
	"os"
	"testing"
)

// M8 A-3: the loss protection tells the hook which library it stopped and why; a scan that runs
// through does not.
func TestScanAbortedHook(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	type abort struct{ name, reason string }
	var got []abort
	WithScanAborted(func(_ context.Context, name, reason string) error {
		got = append(got, abort{name, reason})
		return nil
	})(e.svc)
	dir := tree(t, twentyMovies[:5])
	lib := e.create(dir)
	e.scan(lib.ID)
	if len(got) != 0 {
		t.Fatalf("hook after a good scan: %v", got)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if s := e.scan(lib.ID); s.Scan.State != "aborted" {
		t.Fatalf("scan state %q", s.Scan.State)
	}
	if len(got) != 1 || got[0] != (abort{"Filme", abortRootMissing}) {
		t.Errorf("hook calls = %v", got)
	}
}
