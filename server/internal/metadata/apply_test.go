package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Step 3 of concept 10 (ADR 0018): the server's settings, the apply run that queues one child per title without doubling
// a waiting one, and the rating country (decision 6 as amended by decision 16).

func TestServerMetadataSettingsStoredAtOnce(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	// The server's settings: stored all at once, read back with the counts; the server's list needs de.
	m, err := e.svc.MetadataSettings(ctx)
	if err != nil || m.RatingCountry != "DE" || m.AffectedTitles != 0 || len(m.KnownLanguages) < 150 {
		t.Fatalf("defaults view: %+v %v", m, err)
	}
	m.TextLangs = []string{"de", "en", "es"}
	m.RatingCountry = "CH"
	m.ImageMinWidth["poster"] = 1000
	m.FanartEnabled = true
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Settings(ctx)
	if err != nil || !slices.Equal(st.TextLangs, []string{"de", "en", "es"}) || st.RatingCountry != "CH" || st.ImageMinWidth["poster"] != 1000 || !st.FanartEnabled {
		t.Errorf("stored settings: %+v %v", st, err)
	}
	// The server's list needs no German any more (concept 22 F-7): French and English only.
	m.TextLangs = []string{"fr", "en"}
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Errorf("server list without de: %v", err)
	}
	if st, err := e.svc.Settings(ctx); err != nil || !slices.Equal(st.TextLangs, []string{"fr", "en"}) {
		t.Errorf("stored list without de: %+v %v", st.TextLangs, err)
	}
}

// apply queues one metadata.match per matched title whose home folder is one of the chosen
// folders (texts), or fill and image.fetch (images only); a second apply of the same scope is one
// run, and a waiting child is not doubled (RaiseQueuedPriority). The job type is registered as an
// event type.
func TestApplyQueuesChildrenWithoutDoubles(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Second", imdbID: "tt0000550"})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id IN (?, ?)`, idByTMDB, idByIMDB)
	mustExec(t, e.db.Write, `UPDATE items SET library_id = 2 WHERE id = ?`, idByIMDB)
	ownFolder(t, e.db.Write, 1, 1, "movie", model.RootMetadata{}, idByTMDB)
	ownFolder(t, e.db.Write, 2, 2, "movie", model.RootMetadata{}, idByIMDB)

	n, err := e.svc.CountApplyTitles(ctx, nil)
	if err != nil || n != 2 {
		t.Fatalf("titles over every folder = %d, %v", n, err)
	}
	if n, err := e.svc.CountApplyTitles(ctx, []int64{1}); err != nil || n != 1 {
		t.Fatalf("titles of folder 1 = %d, %v", n, err)
	}
	id, titles, err := e.svc.ApplyMetadata(ctx, ApplyRequest{RootIDs: []int64{1}, What: []string{ApplyTexts}})
	if err != nil || titles != 1 || id == 0 {
		t.Fatalf("apply: id %d titles %d err %v", id, titles, err)
	}
	// The same scope again while the run waits: the same run, no second one; the key names the folders.
	id2, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{RootIDs: []int64{1}, What: []string{ApplyTexts}})
	if err != nil || id2 != id {
		t.Fatalf("second apply: id %d (first %d), %v", id2, id, err)
	}
	if j, err := e.queue.Get(ctx, id); err != nil || j.IdempotencyKey != "apply:texts:roots:1" {
		t.Errorf("key = %q, %v; want apply:texts:roots:1", j.IdempotencyKey, err)
	}
	var verr *model.ValidationError
	if _, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{"covers"}}); !errors.As(err, &verr) {
		t.Errorf("unknown what: %v", err)
	}
	if _, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{RootIDs: []int64{777}, What: []string{ApplyTexts}}); !errors.As(err, &verr) || !hasFieldError(verr.Fields, "rootIds", "unknown_root") {
		t.Errorf("unknown folder: %v", err)
	}

	// Run the parent by hand: one match per title of folder 1, none for folder 2.
	j, err := e.queue.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleApply(ctx, &j); err != nil {
		t.Fatal(err)
	}
	matches := e.jobList(t, JobMatch)
	if len(matches) != 1 || matches[0] != "match:"+idByTMDB+" p1" {
		t.Fatalf("match jobs after apply(texts, folder 1) = %v", matches)
	}
	// Images only, every folder: fill and image.fetch per title; the waiting match stays single.
	id3, titles3, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{ApplyImages}})
	if err != nil || titles3 != 2 {
		t.Fatalf("apply images: %d %v", titles3, err)
	}
	j3, _ := e.queue.Get(ctx, id3)
	if err := e.svc.handleApply(ctx, &j3); err != nil {
		t.Fatal(err)
	}
	if fills := e.jobList(t, JobFill); len(fills) != 2 {
		t.Errorf("fill jobs = %v, want one per title", fills)
	}
	if imgs := e.jobList(t, JobImageFetch); len(imgs) != 2 {
		t.Errorf("image jobs = %v, want one per title", imgs)
	}
	if matches := e.jobList(t, JobMatch); len(matches) != 1 {
		t.Errorf("match jobs after the second apply = %v, want still one", matches)
	}
	// A third texts apply over every folder lifts nothing twice either: still one match for the
	// first title, one new for the second.
	id4, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{ApplyTexts}})
	if err != nil {
		t.Fatal(err)
	}
	j4, _ := e.queue.Get(ctx, id4)
	if err := e.svc.handleApply(ctx, &j4); err != nil {
		t.Fatal(err)
	}
	if matches := e.jobList(t, JobMatch); len(matches) != 2 {
		t.Errorf("match jobs after apply over every folder = %v, want two", matches)
	}
	var payload applyPayload
	if err := json.Unmarshal(j4.Payload, &payload); err != nil || !slices.Equal(payload.What, []string{ApplyTexts}) {
		t.Errorf("payload %s %v", j4.Payload, err)
	}

	// Registered as an event type of the metadata group.
	r := jobs.NewRegistry(e.queue, slog.New(slog.DiscardHandler))
	e.svc.RegisterTypes(r)
	found := false
	for _, typ := range r.Types() {
		if typ.Name == JobApply {
			found = typ.Trigger == jobs.TriggerEvent && typ.Group == jobs.GroupMetadata && typ.Run == nil && typ.Handler != nil
		}
	}
	if !found {
		t.Error("metadata.apply should be registered as an event type without Run")
	}
}

// A run queued before contract 1.40.0 carries libraryIds: it covers the folders of those libraries,
// and a library without folders (or one that went) covers nothing, never every folder.
func TestApplyRunWithLibraryIDsCoversTheirFolders(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Second", imdbID: "tt0000550"})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id IN (?, ?)`, idByTMDB, idByIMDB)
	mustExec(t, e.db.Write, `UPDATE items SET library_id = 2 WHERE id = ?`, idByIMDB)
	ownFolder(t, e.db.Write, 1, 1, "movie", model.RootMetadata{}, idByTMDB)
	ownFolder(t, e.db.Write, 2, 2, "movie", model.RootMetadata{}, idByIMDB)
	mustExec(t, e.db.Write, `INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (9, 'Leer', 'video', 0, 0)`)
	run := func(payload string) []string {
		t.Helper()
		id, err := e.queue.Enqueue(ctx, jobs.NewJob{Type: JobApply, Priority: applyPriority, Payload: json.RawMessage(payload), IdempotencyKey: payload})
		if err != nil {
			t.Fatal(err)
		}
		j, err := e.queue.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.svc.handleApply(ctx, &j); err != nil {
			t.Fatal(err)
		}
		return e.jobList(t, JobMatch)
	}
	if got := run(`{"libraryIds":[9],"what":["texts"]}`); len(got) != 0 {
		t.Fatalf("a library without folders queued %v, want nothing", got)
	}
	if got := run(`{"libraryIds":[777],"what":["texts"]}`); len(got) != 0 {
		t.Fatalf("a library that went queued %v, want nothing", got)
	}
	// Library 2 also holds a matched film without a home folder: such a run covered every matched
	// title of the library, so it takes that one too.
	e.addItem(t, itemSeed{id: idBySearch, title: "Third"})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched', library_id = 2, root_id = NULL WHERE id = ?`, idBySearch)
	if got := run(`{"libraryIds":[2],"what":["texts"]}`); !slices.Equal(slices.Sorted(slices.Values(got)), []string{"match:" + idByIMDB + " p1", "match:" + idBySearch + " p1"}) {
		t.Fatalf("library 2 queued %v, want the matches of its folder's title and of the title without a home folder", got)
	}
	// A folder that went since the request covers nothing either.
	if got := run(`{"rootIds":[555],"what":["texts"]}`); len(got) != 2 {
		t.Fatalf("a folder that went queued %v, want still only the two matches", got)
	}
}

// rootIds sent twice count once: affectedTitles is what the run queues.
func TestApplyCountsAFolderOnce(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, idByTMDB)
	ownFolder(t, e.db.Write, 1, 1, "movie", model.RootMetadata{}, idByTMDB)
	if _, titles, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{ApplyTexts}, RootIDs: []int64{1, 1}}); err != nil || titles != 1 {
		t.Fatalf("apply with folder 1 twice = %d titles, %v; want 1", titles, err)
	}
}

// The rating country (F-7, 6.6; concept 22 F-12, F-16, F-17): a new title gets the age of the
// country read first, then DE → AT → CH → GB → US; with CH first "12 (CH)" where DE says 16, with
// FR first the French "-10", and a title without a French certificate the German one. The same
// film matched again from another country takes that country's age, lower or higher: a change
// of the rating country reads every rating anew (F-16, the test of concept 10 6.6 turned round).
func TestRatingCountryReadFirst(t *testing.T) {
	t.Parallel()
	certs := map[string]string{"DE": "16", "CH": "12", "US": "R", "FR": "10"}
	if r, c := ageFromCertifications(certs, "DE"); r == nil || *r != 16 || c != "DE" {
		t.Fatalf("default order: %v %s", r, c)
	}
	if r, c := ageFromCertifications(certs, "CH"); r == nil || *r != 12 || c != "CH" {
		t.Fatalf("CH first: %v %s", r, c)
	}
	if r, c := ageFromCertifications(certs, "FR"); r == nil || *r != 10 || c != "FR" {
		t.Fatalf("FR first: %v %s", r, c)
	}
	if r, c := ageFromCertifications(map[string]string{"DE": "16", "US": "R"}, "FR"); r == nil || *r != 16 || c != "DE" {
		t.Fatalf("FR first without a French value falls back to DE (F-12): %v %s", r, c)
	}
	if r, c := ageFromCertifications(map[string]string{"DE": "16"}, "CH"); r == nil || *r != 16 || c != "DE" {
		t.Fatalf("CH first without CH value falls back to DE: %v %s", r, c)
	}
	if got := ratingOrder("US"); !slices.Equal(got, []string{"US", "DE", "AT", "CH", "GB"}) {
		t.Errorf("order US = %v", got)
	}
	if got := ratingOrder("JP"); !slices.Equal(got, []string{"JP", "DE", "AT", "CH", "GB", "US"}) {
		t.Errorf("order JP = %v", got)
	}
	if got := ratingOrder("XX"); !slices.Equal(got, ratingFallback) {
		t.Errorf("unknown country = %v", got)
	}

	e := newEnv(t, goodToken)
	ctx := context.Background()
	// Film 550 of the fixture: DE 12 next to US R. Default: 12 from DE. With US first: 17 (R).
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Fatalf("default: %v %v", r.Rating, r.Source)
	}
	m, _ := e.svc.MetadataSettings(ctx)
	m.RatingCountry = "US"
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Fatal(err)
	}
	// A new title with US first: the US age (PG-13 = 13, F-17).
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2001})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idBySearch); r.Rating.Int64 != 13 || r.Source.String != "tmdb:US" {
		t.Fatalf("new title with US first: %v %v (fixture 601 is PG-13 = 13 in the US)", r.Rating, r.Source)
	}
	// The stored 12 from DE of film 550 matched again with US first (R = 17): raised; then DE
	// first again: lowered to the German 12 (F-16).
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 17 || r.Source.String != "tmdb:US" {
		t.Fatalf("US first on the stored film: %v %v, want 17", r.Rating, r.Source)
	}
	m.RatingCountry = "DE"
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 12 || r.Source.String != "tmdb:DE" {
		t.Fatalf("DE first again: %v %v, want lowered to 12 from DE (F-16)", r.Rating, r.Source)
	}
}

// An images apply reaches a title whose folder has one provider: the ordinary fill makes no
// request there and leaves the images to the match, so a changed chain never reached the title
// (CT 190, 1 November 2026). The fill with the images flag fetches the records and decides again;
// the key "fill:images:<id>" keeps it apart from a waiting fill of a match.
func TestApplyImagesRedecidesWithOneProvider(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.setImages("550",
		[]map[string]any{tmdbImage("/poster550.jpg", "en", 2000, 3000, 7.0, 20)},
		[]map[string]any{tmdbImage("/backdrop550.jpg", nil, 1920, 1080, 7.0, 20), tmdbImage("/backdrop550-en.jpg", "en", 1920, 1080, 9.0, 50)},
		nil)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["backdrop"]; got.Source != "/backdrop550.jpg" {
		t.Fatalf("backdrop after the match = %+v, want the one without text", got)
	}
	// The chain now prefers English backdrops.
	raw, _ := json.Marshal([]string{"en", ""})
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, 0) ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		SettingImageLangsPrefix+"backdrop", string(raw))

	// The ordinary fill of a one-provider folder: no request, nothing decided (1.1).
	calls := e.tmdb.total()
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() != calls {
		t.Fatalf("the ordinary fill made %d requests", e.tmdb.total()-calls)
	}
	if got := e.itemImages(t, idByTMDB)["backdrop"]; got.Source != "/backdrop550.jpg" {
		t.Fatalf("backdrop after the ordinary fill = %+v, want unchanged", got)
	}

	// The apply for the images queues the fill with the flag under its own key.
	id, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{ApplyImages}})
	if err != nil {
		t.Fatal(err)
	}
	j, err := e.queue.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleApply(ctx, &j); err != nil {
		t.Fatal(err)
	}
	// Beside the fill the match queued (key fill:<id>), not in its place.
	if fills := e.jobList(t, JobFill); !slices.Contains(fills, "fill:images:"+idByTMDB+" p1") || len(fills) != 2 {
		t.Fatalf("fill jobs after apply(images) = %v", fills)
	}
	var fillID int64
	if err := e.db.Read.QueryRow(`SELECT id FROM jobs WHERE type = ? AND idempotency_key = ?`, JobFill, "fill:images:"+idByTMDB).Scan(&fillID); err != nil {
		t.Fatal(err)
	}
	fillJob, err := e.queue.Get(ctx, fillID)
	if err != nil {
		t.Fatal(err)
	}
	var p itemPayload
	if err := json.Unmarshal(fillJob.Payload, &p); err != nil || !p.Images || p.ItemID != idByTMDB {
		t.Fatalf("fill payload %s (%v), want the images flag", fillJob.Payload, err)
	}
	if err := e.svc.handleFill(ctx, &fillJob); err != nil {
		t.Fatal(err)
	}
	if e.tmdb.total() == calls {
		t.Fatal("the images fill made no request")
	}
	if got := e.itemImages(t, idByTMDB)["backdrop"]; got.Source != "/backdrop550-en.jpg" || got.Lang != "en" {
		t.Errorf("backdrop after the images fill = %+v, want the English one (chain en, '')", got)
	}
	if imgs := e.jobList(t, JobImageFetch); len(imgs) != 1 {
		t.Errorf("image jobs = %v, want one", imgs)
	}
}
