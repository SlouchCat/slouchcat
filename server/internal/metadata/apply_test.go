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

// Step 3 of concept 10 (ADR 0018): the settings with a library's deviation, the union of the
// languages a profile may choose, the apply run that queues one child per title without doubling
// a waiting one, and the rating country (decision 6 as amended by decision 16).

func TestEffectiveSettingsWithLibraryDeviation(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	// Server: de, en (default). Library 2 (Serien) reads ja, en and textless posters first.
	ja := []string{"ja", "en"}
	if err := e.svc.UpdateLibraryDeviation(ctx, 2, LibraryDeviation{TextLangs: &ja, Images: &LibraryImages{Langs: map[string][]string{"poster": {"", "ja"}}, Kinds: []string{ImageKindLogo}}}); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Effective(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(st.TextLangs, ja) || !slices.Equal(st.ImageLangs["poster"], []string{"", "ja"}) ||
		!slices.Equal(st.ImageLangs["backdrop"], DefaultSettings().ImageLangs["backdrop"]) || !slices.Equal(st.ImageKinds, []string{ImageKindLogo}) {
		t.Errorf("effective of library 2: %+v", st)
	}
	one, err := e.svc.Effective(ctx, 1)
	if err != nil || !slices.Equal(one.TextLangs, []string{"de", "en"}) {
		t.Errorf("library 1 should read the server's list: %v %v", one.TextLangs, err)
	}
	// The union a profile may choose: the server's list first, then the libraries' extra languages.
	avail, err := e.svc.AvailableMetadataLanguages(ctx)
	if err != nil || !slices.Equal(avail, []string{"de", "en", "ja"}) {
		t.Errorf("available = %v, %v", avail, err)
	}
	// A library without de and en is refused (E-9); an unknown kind too.
	bad := []string{"ja"}
	err = e.svc.UpdateLibraryDeviation(ctx, 2, LibraryDeviation{TextLangs: &bad})
	var verr *model.ValidationError
	if !errors.As(err, &verr) || !hasFieldError(verr.Fields, "textLangs", "ui_lang_required") {
		t.Errorf("ja only: %v", err)
	}
	err = e.svc.UpdateLibraryDeviation(ctx, 2, LibraryDeviation{Images: &LibraryImages{Kinds: []string{"thumb"}}})
	if !errors.As(err, &verr) || !hasFieldError(verr.Fields, "images.kinds", "invalid_value") {
		t.Errorf("thumb: %v", err)
	}
	// Everything nil removes the row; the server's list applies again.
	if err := e.svc.UpdateLibraryDeviation(ctx, 2, LibraryDeviation{}); err != nil {
		t.Fatal(err)
	}
	if lm, err := e.svc.LibraryMetadata(ctx, 2); err != nil || lm.TextLangs != nil || lm.Images != nil || !slices.Equal(lm.Effective.TextLangs, []string{"de", "en"}) {
		t.Errorf("after the reset: %+v %v", lm, err)
	}
	if _, err := e.svc.LibraryMetadata(ctx, 777); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}

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
	st, err = e.svc.Settings(ctx)
	if err != nil || !slices.Equal(st.TextLangs, []string{"de", "en", "es"}) || st.RatingCountry != "CH" || st.ImageMinWidth["poster"] != 1000 || !st.FanartEnabled {
		t.Errorf("stored settings: %+v %v", st, err)
	}
	m.TextLangs = []string{"en", "es"}
	_, err = e.svc.UpdateMetadataSettings(ctx, m)
	if !errors.As(err, &verr) || !hasFieldError(verr.Fields, "textLangs", "ui_lang_required") {
		t.Errorf("server list without de: %v", err)
	}
}

// apply queues one metadata.match per matched title of the chosen libraries (texts), or fill and
// image.fetch (images only); a second apply of the same scope is one run, and a waiting child is
// not doubled (RaiseQueuedPriority). The job type is registered as an event type.
func TestApplyQueuesChildrenWithoutDoubles(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Second", imdbID: "tt0000550"})
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'matched' WHERE id IN (?, ?)`, idByTMDB, idByIMDB)
	mustExec(t, e.db.Write, `UPDATE items SET library_id = 2 WHERE id = ?`, idByIMDB)

	n, err := e.svc.CountApplyTitles(ctx, nil)
	if err != nil || n != 2 {
		t.Fatalf("titles over every library = %d, %v", n, err)
	}
	if n, err := e.svc.CountApplyTitles(ctx, []int64{1}); err != nil || n != 1 {
		t.Fatalf("titles of library 1 = %d, %v", n, err)
	}
	id, titles, err := e.svc.ApplyMetadata(ctx, ApplyRequest{LibraryIDs: []int64{1}, What: []string{ApplyTexts}})
	if err != nil || titles != 1 || id == 0 {
		t.Fatalf("apply: id %d titles %d err %v", id, titles, err)
	}
	// The same scope again while the run waits: the same run, no second one.
	id2, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{LibraryIDs: []int64{1}, What: []string{ApplyTexts}})
	if err != nil || id2 != id {
		t.Fatalf("second apply: id %d (first %d), %v", id2, id, err)
	}
	var verr *model.ValidationError
	if _, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{"covers"}}); !errors.As(err, &verr) {
		t.Errorf("unknown what: %v", err)
	}
	if _, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{LibraryIDs: []int64{777}, What: []string{ApplyTexts}}); !errors.As(err, &verr) {
		t.Errorf("unknown library: %v", err)
	}

	// Run the parent by hand: one match per title of library 1, none for library 2.
	j, err := e.queue.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleApply(ctx, &j); err != nil {
		t.Fatal(err)
	}
	matches := e.jobList(t, JobMatch)
	if len(matches) != 1 || matches[0] != "match:"+idByTMDB+" p1" {
		t.Fatalf("match jobs after apply(texts, lib 1) = %v", matches)
	}
	// Images only, every library: fill and image.fetch per title; the waiting match stays single.
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
	// A third texts apply over every library lifts nothing twice either: still one match for the
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
		t.Errorf("match jobs after apply over every library = %v, want two", matches)
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

// The rating country (F-7, 6.6): a new title gets the step of the country read first; with CH
// first "12 (CH)" where DE says 16. A stored rating is never lowered (K-3): the same film matched
// again with CH first keeps its 16 from DE.
func TestRatingCountryReadFirst(t *testing.T) {
	t.Parallel()
	certs := map[string]string{"DE": "16", "CH": "12", "US": "R"}
	if r, c := FSKFromCertifications(certs); r == nil || *r != 16 || c != "DE" {
		t.Fatalf("default order: %v %s", r, c)
	}
	if r, c := fskFromCertificationsFirst(certs, "CH"); r == nil || *r != 12 || c != "CH" {
		t.Fatalf("CH first: %v %s", r, c)
	}
	if r, c := fskFromCertificationsFirst(map[string]string{"DE": "16"}, "CH"); r == nil || *r != 16 || c != "DE" {
		t.Fatalf("CH first without CH value falls back to DE: %v %s", r, c)
	}
	if got := ratingOrder("US"); !slices.Equal(got, []string{"US", "DE", "AT", "CH", "GB"}) {
		t.Errorf("order US = %v", got)
	}
	if got := ratingOrder("XX"); !slices.Equal(got, ratingCountries) {
		t.Errorf("unknown country = %v", got)
	}

	e := newEnv(t, goodToken)
	ctx := context.Background()
	// Film 550 of the fixture: DE 12 next to US R. Default: 12 from DE. With US first: 16 (R).
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
	// A new title with US first: the US step (16).
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2001})
	if err := e.svc.match(ctx, idBySearch); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idBySearch); r.Rating.Int64 != 12 || r.Source.String != "tmdb:US" {
		t.Fatalf("new title with US first: %v %v (fixture 601 is PG-13 = 12 in the US)", r.Rating, r.Source)
	}
	// The stored 12 from DE of film 550 matched again with US first (R = 16): raised, because a
	// match may raise; then DE first again: never lowered (K-3).
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 16 || r.Source.String != "tmdb:US" {
		t.Fatalf("US first on the stored film: %v %v, want raised to 16", r.Rating, r.Source)
	}
	m.RatingCountry = "DE"
	if _, err := e.svc.UpdateMetadataSettings(ctx, m); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idByTMDB)
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); r.Rating.Int64 != 16 {
		t.Fatalf("DE first again lowered the stored rating to %v (K-3)", r.Rating)
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
