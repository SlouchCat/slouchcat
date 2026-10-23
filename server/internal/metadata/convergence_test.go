package metadata

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// Tests of the M3 review fixes in this package: convergence of trailers and of titles matched
// before M3 (finding 3), matches that see a manual match in the meantime (finding 5, 13) and the
// visibility recheck after a rating change (finding 20).

// activeJobs returns "<key> p<priority>" of the queued and running jobs of a type, by key.
func (e *env) activeJobs(t *testing.T, jobType string) []string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT idempotency_key || ' p' || priority FROM jobs
	                              WHERE type = ? AND state IN ('queued', 'running') ORDER BY idempotency_key`, jobType)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// After a scan, a title matched before M3 (no image paths, no trailer check) gets one match, and
// titles whose trailers are pending (YouTube quota) get trailer.find; neither repeats once done.
func TestQueuePendingMetadataUpgradesAndQueuesTrailers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "Matched in M2", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Quota postponed", tmdbID: 601})
	e.addItem(t, itemSeed{id: idNoMatch, title: "Trailer locked", tmdbID: 602, locked: `["trailer"]`})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched'`)
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_poster_path = '/poster601.jpg', trailer_checked_at = 1 WHERE id = ?`, idByIMDB)

	n, err := e.svc.QueuePendingMetadata(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	// The locked title gets a trailer job too (its handler skips it) but no upgrade match.
	if got := e.activeJobs(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p4"}) {
		t.Errorf("match jobs %q, want the M2 title only", got)
	}
	wantTrailers := []string{"trailer:" + idByIMDB + " p1", "trailer:" + idByTMDB + " p1", "trailer:" + idNoMatch + " p1"}
	slices.Sort(wantTrailers)
	if got := e.activeJobs(t, JobTrailerFind); !slices.Equal(got, wantTrailers) {
		t.Errorf("trailer jobs %q, want %q", got, wantTrailers)
	}
	if n != 4 {
		t.Errorf("queued %d, want 4", n)
	}

	// The M3 match stores image paths and credits; the trailer check finishes the M2 title.
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idByTMDB); r.Poster.String != "/poster550.jpg" || len(e.credits(t, idByTMDB)) == 0 {
		t.Fatalf("after the upgrade match: poster %v, credits %v", r.Poster, e.credits(t, idByTMDB))
	}
	mustExec(t, e.db.Write, `UPDATE items SET trailer_state = 'found', trailer_checked_at = 2 WHERE id = ?`, idByTMDB)
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueuePendingMetadata(ctx, 1); err != nil || n != 2 {
		t.Fatalf("second check queued %d, %v; want the two pending trailer titles", n, err)
	}
	if got := e.activeJobs(t, JobMatch); len(got) != 0 {
		t.Errorf("match jobs after the upgrade %q, want none", got)
	}
}

// Finding 5: a match that resolved the title with the old ids writes nothing when a manual match
// changed the ids and locked fields meanwhile; it returns a retryable error and the retry uses the
// new ids.
func TestMatchWritesNothingAfterAManualMatchMeanwhile(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	snap, err := e.svc.read.GetItemSystem(ctx, idByTMDB)
	if err != nil {
		t.Fatal(err)
	}
	rec, found, err := e.svc.resolveMovie(ctx, snap)
	if err != nil || !found {
		t.Fatalf("resolve = %v, %v", found, err)
	}
	if err := e.svc.AssignMatch(ctx, idByTMDB, "tmdb", "601"); err != nil {
		t.Fatal(err)
	}
	if _, applied, err := e.svc.applyTitle(ctx, snap, rec.TitleRecord); applied || !errors.Is(err, errItemChanged) {
		t.Fatalf("applyTitle with the old snapshot = %v, %v; want errItemChanged", applied, err)
	}
	if r := e.item(t, idByTMDB); r.State != "pending" || r.TMDB.Int64 != 601 {
		t.Fatalf("after the refused write %+v", r)
	}
	if tx := e.texts(t, idByTMDB); len(tx) != 0 {
		t.Fatalf("texts of the old candidate written: %+v", tx)
	}
	// The retry reads the item again and matches the assigned title.
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.State != "matched" || r.TMDB.Int64 != 601 {
		t.Fatalf("after the retry %+v", r)
	}
	if tx := e.texts(t, idByTMDB); tx["en"].Overview.String != "English overview of Searched Film" {
		t.Fatalf("texts after the retry %+v", tx)
	}
}

// Finding 5 for the season transactions: a series whose ids changed after the job resolved it
// gets no season or episode texts of the old candidate.
func TestSeasonWriteChecksTheSeriesSnapshot(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	snap, err := e.svc.read.GetItemSystem(ctx, idSeries)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := e.svc.tmdb.Series(ctx, "900", textLangs)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AssignMatch(ctx, idSeries, "tvdb", "424242"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.matchSeasons(ctx, snap, e.svc.tmdb, rec); !errors.Is(err, errItemChanged) {
		t.Fatalf("matchSeasons with the old snapshot = %v, want errItemChanged", err)
	}
	for _, id := range []string{idSeason1, idE0101} {
		if r := e.child(t, id); r.State != "pending" {
			t.Errorf("%s written with the old candidate: %+v", id, r)
		}
	}
}

// Finding 13: a manual match or refresh moves a match job a scan queued at priority 4 to 7.
func TestManualMatchRaisesTheQueuedJob(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.enqueueMatch(ctx, idByTMDB, matchPriority); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.AssignMatch(ctx, idByTMDB, "tmdb", "601"); err != nil {
		t.Fatal(err)
	}
	if got := e.activeJobs(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("after the manual match %q, want one job at priority 7", got)
	}
	// A refresh of a title whose job already waits at 7 changes nothing.
	if err := e.svc.Refresh(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.activeJobs(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("after the refresh %q", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.enqueueMatch(ctx, idByTMDB, matchPriority); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Refresh(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.activeJobs(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("refresh over a scan job %q, want one job at priority 7", got)
	}
}

// Finding 20: a match that changes the own rating rechecks the open streams; one that keeps it
// does not.
func TestMatchRatingChangeRechecksVisibility(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	calls := 0
	e.svc.recheck = func(context.Context) error { calls++; return nil }
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 12 || calls != 1 {
		t.Fatalf("first match: rating %v, %d rechecks; want 12 and 1", r.Rating, calls)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("a match with the same rating rechecked: %d", calls)
	}
}
