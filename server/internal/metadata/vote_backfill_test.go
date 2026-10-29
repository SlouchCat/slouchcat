package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// Tests of the vote average (concept 08 4.2, ADR 0016): a field of the fill order of ADR 0015,
// carried by the match for the identifier, and metadata.vote_backfill for the titles matched
// before migration 00026; and of item_texts.sort_key written with every text.

// setTMDBVote sets vote_average of a fixture film in both languages (0 removes it).
func (e *env) setTMDBVote(id string, vote float64) {
	e.tmdb.mu.Lock()
	defer e.tmdb.mu.Unlock()
	fx := e.tmdb.movies[id]
	fx.de["vote_average"], fx.en["vote_average"] = vote, vote
}

func (e *env) setAniListScore(id int64, score int) {
	e.al.mu.Lock()
	defer e.al.mu.Unlock()
	e.al.scores[id] = score
}

// vote returns items.vote_average of an item.
func (e *env) vote(t *testing.T, id string) sql.NullFloat64 {
	t.Helper()
	var v sql.NullFloat64
	if err := e.db.Read.QueryRow(`SELECT vote_average FROM items WHERE id = ?`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// sortKeys returns item_texts.sort_key of an item per language.
func (e *env) sortKeys(t *testing.T, id string) map[string]string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT lang, sort_key FROM item_texts WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var lang string
		var key sql.NullString
		if err := rows.Scan(&lang, &key); err != nil {
			t.Fatal(err)
		}
		out[lang] = key.String
		if !key.Valid {
			out[lang] = "<null>"
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMatchWritesSortKeys: the match writes the sort key of every text row from the shown title
// of its language: the German article goes, the English title keeps its words.
func TestMatchWritesSortKeys(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.mu.Lock()
	e.tmdb.movies["550"].de["title"] = "Der Herr der Ringe"
	e.tmdb.mu.Unlock()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.sortKeys(t, idByTMDB); got["de"] != "herr der ringe" || got["en"] != "first film" {
		t.Fatalf("sort keys after the match %v", got)
	}
	// The fill rewrites the texts with the decided titles; the keys follow.
	e.setProviders(t, `["tmdb","anilist"]`)
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.sortKeys(t, idByTMDB); got["de"] != "herr der ringe" || got["en"] != "first film" {
		t.Fatalf("sort keys after the fill %v", got)
	}
}

// TestVoteAverageThroughMatchAndFill: TMDB's 7.3 lands in items.vote_average with the match (a
// list of one never decides fields) and keeps TMDB as its source in the fill; with TMDB silent,
// AniList's averageScore 81 gives 8.1; a locked vote_average stays NULL and gets no source.
func TestVoteAverageThroughMatchAndFill(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBVote("550", 7.3)
	e.setAniListScore(1001, 81)
	e.setAniListScore(1002, 81)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601})
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, locked: `["vote_average"]`})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]'`)
	for _, id := range []string{idByTMDB, idByIMDB, idLocked} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if v := e.vote(t, idByTMDB); !v.Valid || v.Float64 != 7.3 {
		t.Fatalf("vote after the match %+v, want 7.3", v)
	}
	if v := e.vote(t, idByIMDB); v.Valid {
		t.Fatalf("vote of a film TMDB has none for %+v, want NULL before the fill", v)
	}
	for _, id := range []string{idByTMDB, idByIMDB, idLocked} {
		if err := e.svc.fill(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if v, src := e.vote(t, idByTMDB), e.sources(t, idByTMDB)["vote_average/"]; v.Float64 != 7.3 || src != "tmdb" {
		t.Fatalf("TMDB first: vote %+v from %q, want 7.3 from tmdb", v, src)
	}
	if v, src := e.vote(t, idByIMDB), e.sources(t, idByIMDB)["vote_average/"]; !v.Valid || v.Float64 != 8.1 || src != "anilist" {
		t.Fatalf("AniList filler: vote %+v from %q, want 8.1 from anilist", v, src)
	}
	if v, src := e.vote(t, idLocked), e.sources(t, idLocked); v.Valid || src["vote_average/"] != "" {
		t.Fatalf("locked vote %+v, sources %v", v, src)
	}
	// Idempotent: a second fill keeps the row.
	snap := fmt.Sprint(e.vote(t, idByIMDB), e.sources(t, idByIMDB))
	if err := e.svc.fill(ctx, idByIMDB); err != nil {
		t.Fatal(err)
	}
	if again := fmt.Sprint(e.vote(t, idByIMDB), e.sources(t, idByIMDB)); again != snap {
		t.Fatalf("second fill differs:\n%s\n%s", snap, again)
	}
}

// TestAniListAverageScore: averageScore 81 becomes the field value 8.1, no score no field.
func TestAniListAverageScore(t *testing.T) {
	t.Parallel()
	if got := (AniListRecord{AverageScore: 81}).fields().values[fieldKey{fieldVoteAverage, ""}]; got != "8.1" {
		t.Errorf("averageScore 81 = %q, want 8.1", got)
	}
	if _, ok := (AniListRecord{}).fields().values[fieldKey{fieldVoteAverage, ""}]; ok {
		t.Error("a record without score sets the field")
	}
	e := newEnv(t, goodToken)
	e.setAniListScore(1001, 81)
	rec, err := e.svc.anilist.Media(context.Background(), 1001)
	if err != nil || rec.AverageScore != 81 {
		t.Fatalf("Media(1001) = %+v, %v", rec, err)
	}
	if rec, err := e.svc.anilist.Media(context.Background(), 1002); err != nil || rec.AverageScore != 0 {
		t.Fatalf("Media(1002) without score = %+v, %v", rec, err)
	}
}

// TestVoteBackfill: titles matched before 00026 get their vote from the backfill (TMDB 7.3);
// a title whose providers have none is marked with its identifier and not asked again, yet a
// later fill writes the vote AniList gives then; a locked vote and a list without vote provider
// stay out; QueueVoteBackfill starts one chain only.
func TestVoteBackfill(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setTMDBVote("550", 7.3)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Searched", tmdbID: 601, imdbID: "tt0000601"})
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, locked: `["vote_average"]`})
	e.addItem(t, itemSeed{id: idNoMatch, title: "Pending", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id <> ?`, idNoMatch)
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","anilist"]' WHERE id = ?`, idByIMDB)

	if ok, err := e.svc.QueueVoteBackfill(ctx); err != nil || !ok {
		t.Fatalf("QueueVoteBackfill = %v, %v", ok, err)
	}
	if ok, err := e.svc.QueueVoteBackfill(ctx); err != nil || ok {
		t.Fatalf("second QueueVoteBackfill = %v, %v; want nothing while the job is queued", ok, err)
	}
	if got := e.activeJobs(t, JobVoteBackfill); !slices.Equal(got, []string{"vote_backfill p2"}) {
		t.Fatalf("backfill jobs %v", got)
	}
	if err := e.svc.handleVoteBackfill(ctx, &jobs.Job{Type: JobVoteBackfill, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if v, src := e.vote(t, idByTMDB), e.sources(t, idByTMDB)["vote_average/"]; !v.Valid || v.Float64 != 7.3 || src != "tmdb" {
		t.Fatalf("backfilled vote %+v from %q, want 7.3 from tmdb", v, src)
	}
	if v, src := e.vote(t, idByIMDB), e.sources(t, idByIMDB)["vote_average/"]; v.Valid || src != "tmdb" {
		t.Fatalf("no vote anywhere: vote %+v, mark %q; want NULL marked by the identifier", v, src)
	}
	if v, src := e.vote(t, idLocked), e.sources(t, idLocked); v.Valid || len(src) != 0 {
		t.Fatalf("locked title touched: %+v %v", v, src)
	}
	if v := e.vote(t, idNoMatch); v.Valid {
		t.Fatalf("unmatched title backfilled: %+v", v)
	}
	// A short batch queues no successor; the next chain finds nothing and asks nobody.
	if got := e.activeJobs(t, JobVoteBackfill); !slices.Equal(got, []string{"vote_backfill p2"}) {
		t.Fatalf("backfill jobs after the last batch %v (the handler ran outside the queue)", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	calls, alCalls := e.tmdb.total(), e.al.count("/graphql")
	if ok, err := e.svc.QueueVoteBackfill(ctx); err != nil || ok {
		t.Fatalf("QueueVoteBackfill with nothing left = %v, %v", ok, err)
	}
	if last, err := e.svc.voteBackfill(ctx, ""); err != nil || last != "" {
		t.Fatalf("voteBackfill = %q, %v", last, err)
	}
	if e.tmdb.total() != calls || e.al.count("/graphql") != alCalls {
		t.Fatalf("a marked title was asked again (%d/%d requests)", e.tmdb.total()-calls, e.al.count("/graphql")-alCalls)
	}
	// The mark defends no value: AniList has a score now, the next fill writes it.
	e.setAniListScore(1002, 64)
	if err := e.svc.fill(ctx, idByIMDB); err != nil {
		t.Fatal(err)
	}
	if v, src := e.vote(t, idByIMDB), e.sources(t, idByIMDB)["vote_average/"]; !v.Valid || v.Float64 != 6.4 || src != "anilist" {
		t.Fatalf("fill after the mark: vote %+v from %q, want 6.4 from anilist", v, src)
	}
}

// TestVoteBackfillContinues: a full batch queues its successor with the cursor, so a restart
// continues there; titles whose identifier has no key are passed over without a request or mark.
func TestVoteBackfillContinues(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken) // no TVDB key
	ctx := context.Background()
	ids := make([]string, 0, voteBackfillBatch+1)
	for i := range voteBackfillBatch + 1 {
		id := fmt.Sprintf("01JTESTVOTE%015d", i)
		ids = append(ids, id)
		mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, metadata_state, providers,
		                         added_at, created_at, updated_at)
		                         VALUES (?, 'series', 1, 'Show', 'show', 'matched', '["tvdb","tmdb"]', 0, 0, 0)`, id)
	}
	calls := e.tmdb.total()
	if err := e.svc.handleVoteBackfill(ctx, &jobs.Job{Type: JobVoteBackfill, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	next := ids[voteBackfillBatch-1]
	if got := e.activeJobs(t, JobVoteBackfill); !slices.Equal(got, []string{"vote_backfill:" + next + " p2"}) {
		t.Fatalf("successor %v, want the cursor %s", got, next)
	}
	if ok, err := e.svc.QueueVoteBackfill(ctx); err != nil || ok {
		t.Fatalf("QueueVoteBackfill beside a running chain = %v, %v", ok, err)
	}
	payload := fmt.Sprintf(`{"after":%q}`, next)
	if err := e.svc.handleVoteBackfill(ctx, &jobs.Job{Type: JobVoteBackfill, Payload: []byte(payload)}); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() != calls {
		t.Fatalf("titles without identifier key made %d requests", e.tmdb.total()-calls)
	}
	var marks int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM item_field_sources WHERE field = 'vote_average'`).Scan(&marks); err != nil || marks != 0 {
		t.Fatalf("marks %d, %v; want none for titles passed over", marks, err)
	}
	if err := e.svc.handleVoteBackfill(ctx, &jobs.Job{Type: JobVoteBackfill, Payload: []byte(`{"after":"../x"}`)}); err == nil {
		t.Fatal("an invalid cursor was accepted")
	}
}
