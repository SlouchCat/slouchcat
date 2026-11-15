package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// setReleases replaces the release dates (the certificates) of a fixture film in every language.
func (f *fakeTMDB) setReleases(id string, releases ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fx := f.movies[id]
	for _, m := range []map[string]any{fx.de, fx.en} {
		m["release_dates"] = map[string]any{"results": releases}
	}
	f.movies[id] = fx
}

// certRows returns the stored certificates of a title as "provider:country=cert".
func (e *env) certRows(t *testing.T, id string) map[string]string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT provider, country, certification FROM item_certifications WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var p, c, cert string
		if err := rows.Scan(&p, &c, &cert); err != nil {
			t.Fatal(err)
		}
		out[p+":"+c] = cert
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// rereadAll runs metadata.ratings_reread from the first title to the last batch.
func (e *env) rereadAll(t *testing.T) {
	t.Helper()
	p := &ratingsRereadPayload{}
	for p != nil {
		next, err := e.svc.ratingsReread(context.Background(), *p)
		if err != nil {
			t.Fatal(err)
		}
		p = next
	}
}

func (e *env) setRatingCountry(t *testing.T, country string) {
	t.Helper()
	ctx := context.Background()
	m, err := e.svc.MetadataSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m.RatingCountry = country
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Fatal(err)
	}
}

// The match stores the certificates of every rating country (item_certifications), a film
// without any gets the row "asked, nothing"; countries outside the table are not stored.
func TestMatchStoresTheCertificates(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.setReleases("550", countryJSON("US", "R"), countryJSON("DE", "12"), countryJSON("FR", "-12"), countryJSON("AU", "MA15+"))
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	e.addItem(t, itemSeed{id: idNoRating, title: "Unrated Film", tmdbID: 602})
	for _, id := range []string{idByTMDB, idNoRating} {
		if err := e.svc.match(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	got := e.certRows(t, idByTMDB)
	if len(got) != 3 || got["tmdb:US"] != "R" || got["tmdb:DE"] != "12" || got["tmdb:FR"] != "-12" {
		t.Errorf("certificates of the film = %v, want US R, DE 12, FR -12 (AU is no rating country)", got)
	}
	if got := e.certRows(t, idNoRating); len(got) != 1 || got["tmdb:"] != "" {
		t.Errorf("certificates of the unrated film = %v, want the row asked, nothing", got)
	}
}

// The job «Freigaben neu lesen» (F-16, F-17): a change of the rating country queues it; it reads
// the stored certificates without a request and may lower and raise a provider's rating; a
// rating by hand, from an NFO or under a lock stays as it is; a title without stored certificates
// is asked once and its answer stored.
func TestRatingsRereadLowersAndKeepsManualRatings(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.setReleases("550", countryJSON("US", "R"), countryJSON("DE", "12"), countryJSON("FR", "-10"))
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	// By hand with the lock, from an NFO, and a provider value under a lock: all on film 550 too.
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, ownRating: 6, ratingSource: "manual", locked: `["own_rating"]`})
	e.addItem(t, itemSeed{id: idByIMDB, title: "From NFO", tmdbID: 550, ownRating: 16, ratingSource: "nfo"})
	e.addItem(t, itemSeed{id: idBySearch, title: "Locked TMDB", tmdbID: 550, ownRating: 18, ratingSource: "tmdb:DE", locked: `["own_rating"]`})
	// A title from before the update: tmdb:US 16 (R rounded down), no stored certificates yet.
	e.addItem(t, itemSeed{id: idNoRating, title: "Searched Film", tmdbID: 601, ownRating: 12, ratingSource: "tmdb:US"})
	if r := e.item(t, idByTMDB); r.Rating != (sql.NullInt64{Int64: 12, Valid: true}) || r.Source.String != "tmdb:DE" {
		t.Fatalf("after the match: %v %v", r.Rating, r.Source)
	}

	// France first: the queue gets the job, the film becomes 10 (lowered, F-16).
	e.setRatingCountry(t, "FR")
	if got := e.jobList(t, JobRatingsReread); len(got) != 1 {
		t.Fatalf("a change of the rating country queued %v", got)
	}
	before := e.tmdb.total()
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 10 || r.Source.String != "tmdb:FR" {
		t.Errorf("FR first: %v %v, want lowered to 10 from FR", r.Rating, r.Source)
	}
	// 601 had no stored certificates: asked once (one request), PG-13 is 13 now (F-17, stricter).
	if r := e.item(t, idNoRating); r.Rating.Int64 != 13 || r.Source.String != "tmdb:US" {
		t.Errorf("title from before the update: %v %v, want 13 tmdb:US", r.Rating, r.Source)
	}
	if got := e.certRows(t, idNoRating); got["tmdb:US"] != "PG-13" {
		t.Errorf("the asked title stored %v", got)
	}
	requests := e.tmdb.total() - before
	if requests != 1 {
		t.Errorf("the job made %d requests, want 1 (only the title without stored certificates)", requests)
	}
	for id, want := range map[string]struct {
		rating int64
		source string
	}{idLocked: {6, "manual"}, idByIMDB: {16, "nfo"}, idBySearch: {18, "tmdb:DE"}} {
		if r := e.item(t, id); r.Rating.Int64 != want.rating || r.Source.String != want.source {
			t.Errorf("%s changed to %v %v, want %d %s untouched", id, r.Rating, r.Source, want.rating, want.source)
		}
	}

	// The US first: raised to R = 17, without a single request now.
	e.setRatingCountry(t, "US")
	before = e.tmdb.total()
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 17 || r.Source.String != "tmdb:US" {
		t.Errorf("US first: %v %v, want 17 from the US", r.Rating, r.Source)
	}
	if n := e.tmdb.total() - before; n != 0 {
		t.Errorf("the second run made %d requests, want none", n)
	}
	// Japan first: the film has no Japanese certificate, so the German one counts (F-12).
	e.setRatingCountry(t, "JP")
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Errorf("JP first: %v %v, want 12 from DE (F-12)", r.Rating, r.Source)
	}
}

// Once after the update: the chain is queued at the first start when titles wait and never again;
// saving the settings without a change of the country queues nothing, and a server without titles
// queues nothing either (the wizard before the first scan, m2-11).
func TestRatingsRereadOnceAndOnlyWithTitles(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.setRatingCountry(t, "FR")
	if got := e.jobList(t, JobRatingsReread); len(got) != 0 {
		t.Fatalf("a change of the country without titles queued %v", got)
	}
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || ok {
		t.Fatalf("once without titles = %v, %v", ok, err)
	}
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550, ownRating: 12, ratingSource: "tmdb:DE"})
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || ok {
		t.Fatalf("once after the mark = %v, %v; want nothing queued", ok, err)
	}
	mustExec(t, e.db.Write, `DELETE FROM settings WHERE key = ?`, SettingRatingsRereadDone)
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || !ok {
		t.Fatalf("once with a title = %v, %v; want queued", ok, err)
	}
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || ok {
		t.Fatalf("a second start = %v, %v; want nothing", ok, err)
	}
	if rereading, err := e.svc.RatingsRereading(ctx); err != nil || !rereading {
		t.Errorf("RatingsRereading with a queued run = %v, %v", rereading, err)
	}
	e.setRatingCountry(t, "FR") // the same country: nothing new
	if got := e.jobList(t, JobRatingsReread); len(got) != 1 {
		t.Errorf("jobs after saving the same country = %v, want the one chain", got)
	}
}

// A partial read keeps the value (review of stage 3): a series rated by TVDB (US 16) whose TVDB
// is not readable now (no key, no stored certificates) keeps 16 although TMDB says DE 12; once
// TVDB's answer is stored too («asked, nothing»), the strictest over both providers is TMDB's 12.
func TestRatingsRereadKeepsTheValueWhenAProviderIsNotRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken) // TMDB only: TVDB has no key
	e.addNode(t, node{id: idSeries, kind: "series", title: "Both Providers", tmdbID: 900, tvdbID: 424242})
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","tvdb"]', own_rating = 16, rating_source = 'tvdb:US' WHERE id = ?`, idSeries)
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tmdb', 'DE', '12', 1)`, idSeries)
	rating := func() (sql.NullInt64, sql.NullString) {
		t.Helper()
		var r sql.NullInt64
		var src sql.NullString
		if err := e.db.Read.QueryRow(`SELECT own_rating, rating_source FROM items WHERE id = ?`, idSeries).Scan(&r, &src); err != nil {
			t.Fatal(err)
		}
		return r, src
	}
	e.rereadAll(t)
	if r, src := rating(); r.Int64 != 16 || src.String != "tvdb:US" {
		t.Fatalf("with TVDB unread: %v %v, want 16 tvdb:US kept", r, src)
	}
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tvdb', '', '', 1)`, idSeries)
	e.rereadAll(t)
	if r, src := rating(); r.Int64 != 12 || src.String != "tmdb:DE" {
		t.Errorf("with both providers read: %v %v, want 12 tmdb:DE", r, src)
	}
}

// A title whose providers know no certificate becomes unrated, and the provenance of own_rating
// goes with the value (review of stage 3): no provider is the source of an unknown rating.
func TestRatingsRereadUnratedDropsTheSource(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550, ownRating: 12, ratingSource: "tmdb:DE"})
	mustExec(t, e.db.Write, `INSERT INTO item_field_sources (item_id, field, provider) VALUES (?, 'own_rating', 'tmdb')`, idByTMDB)
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tmdb', '', '', 1)`, idByTMDB)
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Valid || r.Source.Valid {
		t.Fatalf("rating %v %v, want unrated", r.Rating, r.Source)
	}
	var n int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM item_field_sources WHERE item_id = ? AND field = 'own_rating'`, idByTMDB).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d provenance rows of own_rating left on an unrated title", n)
	}
}

// One title the provider keeps failing (TMDB answers 503 for film 503) does not stop the chain
// (review of stage 3): it keeps its value, the other titles are read, and the chain marks itself
// done, so the next start does not queue it again.
func TestRatingsRereadGoesOnAfterAFailingTitle(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "Down", tmdbID: 503, ownRating: 12, ratingSource: "tmdb:DE"})
	e.addItem(t, itemSeed{id: idBySearch, title: "First Film", tmdbID: 550, ownRating: 16, ratingSource: "tmdb:US"})
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tmdb', 'US', 'R', 1), (?, 'tmdb', 'DE', '12', 1)`, idBySearch, idBySearch)
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Errorf("failing title: %v %v, want 12 tmdb:DE kept", r.Rating, r.Source)
	}
	if r := e.item(t, idBySearch); r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Errorf("readable title: %v %v, want 12 tmdb:DE", r.Rating, r.Source)
	}
	if _, err := e.svc.read.GetSystemSetting(ctx, SettingRatingsRereadDone); err != nil {
		t.Errorf("the finished chain left no done mark: %v", err)
	}
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || ok {
		t.Errorf("a start after the finished chain = %v, %v; want nothing queued", ok, err)
	}
}

// A change of the rating country while the chain runs (review of stage 3): the chain that read
// its first batch for the old country starts again from the first title for the new one, and the
// start for the new country is not folded into the old chain, so no title keeps the old age.
func TestRatingsRereadStartsAgainAfterACountryChangeMidChain(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	// More titles than one batch, each with stored certificates: US R, FR -12.
	for i := range ratingsRereadBatch + 5 {
		id := fmt.Sprintf("01JCHAIN%018d", i)
		mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, tmdb_id, own_rating, rating_source, added_at, created_at, updated_at)
			VALUES (?, 'movie', 1, 'x', 'x', 550, 12, 'tmdb:DE', 0, 0, 0)`, id)
		mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tmdb', 'US', 'R', 1), (?, 'tmdb', 'FR', '-12', 1)`, id, id)
	}
	e.setRatingCountry(t, "US")
	// The first batch reads for the US …
	next, err := e.svc.ratingsReread(ctx, ratingsRereadPayload{Country: "US"})
	if err != nil || next == nil || next.Country != "US" {
		t.Fatalf("first batch: %+v, %v", next, err)
	}
	// … then the country becomes FR: its start has its own key beside the waiting US chain.
	e.setRatingCountry(t, "FR")
	keys := e.jobList(t, JobRatingsReread)
	if !slices.ContainsFunc(keys, func(k string) bool { return strings.HasPrefix(k, "ratings_reread:FR") }) {
		t.Fatalf("the change to FR queued no chain of its own: %v", keys)
	}
	// The US chain's next batch sees FR and ends instead of reading on.
	if again, err := e.svc.ratingsReread(ctx, *next); err != nil || again != nil {
		t.Fatalf("the old chain after the change: %+v, %v; want it ended", again, err)
	}
	e.rereadAll(t) // the FR chain from the first title
	var old int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM items WHERE id LIKE '01JCHAIN%' AND (own_rating <> 12 OR rating_source <> 'tmdb:FR')`).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if old != 0 {
		t.Errorf("%d titles kept an age of the old country", old)
	}
}

// A change of the rating country whose chain is lost (not queued, failed, cancelled; here: the
// queue forgets it) is not lost for good (review of stage 3): until a chain finished for the new
// country, saving the settings again or the next start queues it; after it finished, neither does.
func TestRatingsRereadPendingAfterALostChain(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550, ownRating: 12, ratingSource: "tmdb:DE"})
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tmdb', 'US', 'R', 1)`, idByTMDB)
	e.setRatingCountry(t, "US")
	mustExec(t, e.db.Write, `DELETE FROM jobs WHERE type = ?`, JobRatingsReread)
	e.setRatingCountry(t, "US") // the same country, saved again
	if got := e.jobList(t, JobRatingsReread); len(got) != 1 {
		t.Fatalf("saving again after a lost chain queued %v, want the chain", got)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs WHERE type = ?`, JobRatingsReread)
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || !ok {
		t.Fatalf("a start after a lost chain = %v, %v; want queued", ok, err)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs WHERE type = ?`, JobRatingsReread)
	e.rereadAll(t)
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 17 {
		t.Fatalf("after the chain: %v, want 17", r.Rating)
	}
	if ok, err := e.svc.QueueRatingsRereadPending(ctx); err != nil || ok {
		t.Errorf("a start after the finished chain = %v, %v; want nothing", ok, err)
	}
	e.setRatingCountry(t, "US")
	if got := e.jobList(t, JobRatingsReread); len(got) != 0 {
		t.Errorf("saving after the finished chain queued %v", got)
	}
}

// Another title at a provider makes every stored certificate worthless (review of stage 3): a
// manual assignment and a match that finds another title delete the certificates of every
// provider; the match stores the new title's.
func TestAnotherTitleDropsTheCertificates(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tvdb', 'US', 'TV-MA', 1)`, idByTMDB)
	if err := e.svc.AssignMatch(ctx, idByTMDB, "tmdb", "601"); err != nil {
		t.Fatal(err)
	}
	if got := e.certRows(t, idByTMDB); len(got) != 0 {
		t.Fatalf("certificates after the assignment = %v, want none", got)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.certRows(t, idByTMDB); len(got) != 1 || got["tmdb:US"] != "PG-13" {
		t.Fatalf("certificates after the match = %v, want only 601's PG-13", got)
	}

	// A match that finds another title by itself (a search, the title had no TMDB id): the rows
	// of the old title, TVDB's too, go as well.
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2001})
	mustExec(t, e.db.Write, `INSERT INTO item_certifications VALUES (?, 'tvdb', 'US', 'TV-MA', 1), (?, 'tmdb', 'DE', '12', 1)`, idBySearch, idBySearch)
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if got := e.certRows(t, idBySearch); len(got) != 1 || got["tmdb:US"] != "PG-13" {
		t.Errorf("certificates after the match = %v, want only the new title's PG-13", got)
	}
}
