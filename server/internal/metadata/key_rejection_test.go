package metadata

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// swapKeys is a KeySource whose TVDB key a test changes.
type swapKeys struct {
	mu sync.Mutex
	k  testKeys
}

func (s *swapKeys) keys() testKeys {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.k
}

func (s *swapKeys) setTVDB(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.k.tvdb = key
}

func (s *swapKeys) TMDBKey(ctx context.Context) (string, error) { return s.keys().TMDBKey(ctx) }
func (s *swapKeys) TVDBKey(ctx context.Context) (string, string, error) {
	return s.keys().TVDBKey(ctx)
}
func (s *swapKeys) YouTubeKey(ctx context.Context) (string, error) { return s.keys().YouTubeKey(ctx) }
func (s *swapKeys) TraktClientID(ctx context.Context) (string, error) {
	return s.keys().TraktClientID(ctx)
}
func (s *swapKeys) FanartKeys(ctx context.Context) (string, string, error) {
	return s.keys().FanartKeys(ctx)
}
func (s *swapKeys) GoogleBooksKey(ctx context.Context) (string, error) {
	return s.keys().GoogleBooksKey(ctx)
}

// TestFillRejectedKeyNoRetryNoRequeue (concept 21 P-2): a fill that meets a TVDB key TVDB rejects
// ends without error and fills from TMDB; later fills send nothing to TVDB, QueuePendingFills
// leaves TVDB out like a provider without key and skips titles TVDB identifies; a new key that
// passes its check lifts the rejection and queues the titles that waited, once.
func TestFillRejectedKeyNoRetryNoRequeue(t *testing.T) {
	t.Parallel()
	keys := &swapKeys{k: testKeys{tmdb: goodToken, tvdb: "rejected-tvdb-key", pin: tvdbPin}}
	e := newEnvWith(t, keys)
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", year: 2010})
	e.setProviders(t, `["tmdb","tvdb"]`)
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET tvdb_id = 424242 WHERE id = ?`, idSeries)
	mustExec(t, e.db.Write, `DELETE FROM jobs`)

	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatalf("fill with a rejected TVDB key = %v, want nil (no retry)", err)
	}
	if !e.svc.KeyRejected(ctx, ProviderTVDB) || e.svc.KeyRejected(ctx, ProviderTMDB) {
		t.Fatalf("rejected tvdb/tmdb = %v/%v, want true/false", e.svc.KeyRejected(ctx, ProviderTVDB), e.svc.KeyRejected(ctx, ProviderTMDB))
	}
	if v := queryString(t, e, `SELECT filled_providers FROM items WHERE id = ?`, idSeries); v != `["tmdb"]` {
		t.Errorf("filled_providers = %s, want [\"tmdb\"]", v)
	}
	if src := e.sources(t, idSeries); src["overview/en"] != "tmdb" {
		t.Errorf("sources %v, want TMDB's", src)
	}
	asked := e.tvdb.total()
	if asked == 0 {
		t.Fatal("TVDB was not asked")
	}
	if err := e.svc.fill(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if n := e.tvdb.total(); n != asked {
		t.Errorf("second fill sent %d requests to TVDB, want none", n-asked)
	}
	if n := strings.Count(e.logs.String(), "rejected its key"); n != 1 {
		t.Errorf("%d warnings for the rejected key, want 1:\n%s", n, e.logs.String())
	}
	if strings.Contains(e.logs.String(), "not configured, skipped") {
		t.Errorf("a rejected key is reported as missing:\n%s", e.logs.String())
	}

	// No re-queue: the fill ran with the list a fill would run with now; a title TVDB identifies
	// waits for the key.
	if n, err := e.svc.QueuePendingFills(ctx, 2); err != nil || n != 0 {
		t.Fatalf("QueuePendingFills = %d, %v; want 0", n, err)
	}
	e.setProviders(t, `["tvdb","tmdb"]`)
	if n, err := e.svc.QueuePendingFills(ctx, 2); err != nil || n != 0 {
		t.Fatalf("QueuePendingFills with TVDB first = %d, %v; want 0", n, err)
	}
	if err := e.svc.fill(ctx, idSeries); err != nil || e.tvdb.total() != asked {
		t.Fatalf("fill with the rejected identifier: %v, %d TVDB requests", err, e.tvdb.total()-asked)
	}

	// A new key: not rejected until TVDB says so; its check lifts the rejection and queues the
	// fill once.
	keys.setTVDB(tvdbKey)
	if e.svc.KeyRejected(ctx, ProviderTVDB) {
		t.Fatal("a new key counts as rejected")
	}
	if c, err := e.svc.CheckProviderKey(ctx, KeyCheckTVDB); err != nil || !c.OK {
		t.Fatalf("check of the new key = %+v, %v", c, err)
	}
	if got := e.fillJobs(t); !slices.Equal(got, []string{"fill:" + idSeries + " p2"}) {
		t.Fatalf("fill jobs after the new key = %v", got)
	}
	// A check that TVDB refuses stops the requests again.
	keys.setTVDB("another-rejected-key")
	if c, err := e.svc.CheckProviderKey(ctx, KeyCheckTVDB); err != nil || c.Problem != model.KeyCheckInvalid {
		t.Fatalf("check of a refused key = %+v, %v", c, err)
	}
	if !e.svc.KeyRejected(ctx, ProviderTVDB) {
		t.Error("a refused check leaves the key usable")
	}
}

// TestKeyRejectedErrorNamesTheProvider (concept 21 P-2): the error names the provider that
// rejected the key and still is model.ErrTMDBKeyInvalid (contract code tmdb_key_invalid); a fill
// that meets it after the title records ends without retry.
func TestKeyRejectedErrorNamesTheProvider(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tvdb: "rejected-tvdb-key", pin: tvdbPin})
	_, err := e.svc.tvdb.Series(context.Background(), "424242", []string{"en"})
	if !errors.Is(err, model.ErrTMDBKeyInvalid) || rejectedBy(err) != ProviderTVDB {
		t.Fatalf("err = %v, want a TVDB key rejection", err)
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "tvdb key invalid") || strings.Contains(msg, "tmdb") {
		t.Errorf("message %q, want it to name TVDB only", msg)
	}
	if f := jobs.Final(fmt.Errorf("metadata.fill: %w", err)); rejectedBy(f) != ProviderTVDB || !errors.Is(f, model.ErrTMDBKeyInvalid) {
		t.Errorf("wrapped: provider %q", rejectedBy(f))
	}
}

// TestDeleteKeyRejectedFillsOnce (concept 21 O-4): the failed fills a rejected key ended go once;
// other failures stay, and a second start removes nothing.
func TestDeleteKeyRejectedFillsOnce(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	add := func(typ, state, lastError string) {
		mustExec(t, e.db.Write, `INSERT INTO jobs (type, state, run_after, attempts, max_attempts, last_error, created_at, updated_at)
			VALUES (?, ?, 0, 5, 5, ?, 0, 0)`, typ, state, lastError)
	}
	add(JobFill, "failed", "metadata.fill: tvdb 81189: tmdb key invalid: tvdb /login: status 401")
	add(JobFill, "failed", "metadata.fill: tvdb 81190: tvdb key invalid: /login: status 401")
	add(JobFill, "failed", "metadata.fill: tmdb 550: tmdb unreachable: /tv/550: status 503")
	add(JobFill, "done", "metadata.fill: tvdb 81191: tmdb key invalid: tvdb /login: status 401")
	add(JobMatch, "failed", "metadata.match: tmdb key invalid: /search/movie: status 401")
	n, err := e.svc.DeleteKeyRejectedFillsOnce(ctx)
	if err != nil || n != 2 {
		t.Fatalf("DeleteKeyRejectedFillsOnce = %d, %v; want 2", n, err)
	}
	if left := queryString(t, e, `SELECT count(*) FROM jobs`); left != "3" {
		t.Errorf("%s jobs left, want 3", left)
	}
	add(JobFill, "failed", "metadata.fill: tvdb 81192: tvdb key invalid: /login: status 401")
	if n, err := e.svc.DeleteKeyRejectedFillsOnce(ctx); err != nil || n != 0 {
		t.Errorf("second run = %d, %v; want 0", n, err)
	}
}

func queryString(t *testing.T, e *env, query string, args ...any) string {
	t.Helper()
	var v string
	if err := e.db.Read.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
