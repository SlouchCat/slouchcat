package admin

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

type spotlightFixture struct {
	f     *vistest.Fixture
	svc   *Service
	trakt *fakeTrakt
	logs  *bytes.Buffer
	log   *slog.Logger
	now   time.Time
	ctx   context.Context
}

// newSpotlightFixture seeds the vistest library with two spotlight films (TMDB ids 8001 and 8002)
// and logs into a buffer.
func newSpotlightFixture(t *testing.T) *spotlightFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddSpotlightFilms(t, 2)
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	sf := &spotlightFixture{f: f, trakt: &fakeTrakt{}, logs: &logs, log: log,
		now: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC), ctx: context.Background()}
	sf.svc = sf.service()
	return sf
}

// service returns a new Service over the fixture's database: a second one stands for a restart.
func (sf *spotlightFixture) service() *Service {
	svc := New(sf.f.DB, auth.New(sf.f.DB, sf.log), &fakeLibraries{}, nil, &fakeTMDB{}, func(string) string { return "" }, sf.log)
	svc.now = func() time.Time { return sf.now }
	return svc.WithTrakt(sf.trakt)
}

func (sf *spotlightFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := sf.f.DB.Write.ExecContext(sf.ctx, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (sf *spotlightFixture) entry(t *testing.T, list, media string, position, tmdbID int64, title string) {
	t.Helper()
	sf.exec(t, `INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at) VALUES (?, ?, ?, ?, ?, 2026, 1)`,
		list, media, position, tmdbID, title)
}

func validSpotlightUpdate() model.SpotlightConfigUpdate {
	return model.SpotlightConfigUpdate{Enabled: true, ListKey: "trending", Media: "both", ItemCount: 10, MinHits: 3, IntervalHours: 12}
}

// getSpotlightConfig carries the stored row (the seed of migration 00021) and the seven list kinds
// in the order of plan 5.6.
func TestSpotlightConfigCatalogue(t *testing.T) {
	sf := newSpotlightFixture(t)
	cfg, err := sf.svc.SpotlightConfig(sf.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.ListKey != "trending" || cfg.Media != "both" || cfg.ItemCount != 20 || cfg.MinHits != 5 || cfg.IntervalHours != 24 ||
		cfg.RotationSeconds != 5 || cfg.ListURL != "" {
		t.Errorf("seeded config %+v", cfg)
	}
	var keys []string
	for _, k := range cfg.ListKinds {
		keys = append(keys, k.Key)
	}
	want := []string{"trending", "popular", "watched_weekly", "collected_monthly", "anticipated", "boxoffice", "url"}
	if !slices.Equal(keys, want) {
		t.Fatalf("kinds %v, want %v", keys, want)
	}
	for _, k := range cfg.ListKinds {
		switch k.Key {
		case "boxoffice":
			if !slices.Equal(k.Media, []string{"movie"}) || k.MediaFilters || k.NeedsURL {
				t.Errorf("boxoffice %+v", k)
			}
		case "url":
			if !slices.Equal(k.Media, []string{"movie", "show", "both"}) || !k.MediaFilters || !k.NeedsURL {
				t.Errorf("url %+v", k)
			}
		default:
			if !slices.Equal(k.Media, []string{"movie", "show", "both"}) || k.MediaFilters || k.NeedsURL {
				t.Errorf("%s %+v", k.Key, k)
			}
		}
	}
}

// Every value outside its range gives 422 validation_failed with the field name; the bounds
// themselves are accepted (DoD step 4 "Grenzen").
func TestSpotlightConfigBounds(t *testing.T) {
	sf := newSpotlightFixture(t)
	cases := []struct {
		field string
		value int
		ok    bool
	}{
		{"itemCount", 4, false}, {"itemCount", 5, true}, {"itemCount", 20, true}, {"itemCount", 21, false},
		{"minHits", -1, false}, {"minHits", 0, true}, {"minHits", 10, true}, {"minHits", 11, false},
		{"intervalHours", 5, false}, {"intervalHours", 6, true}, {"intervalHours", 48, true}, {"intervalHours", 49, false},
		{"rotationSeconds", 2, false}, {"rotationSeconds", 3, true}, {"rotationSeconds", 15, true}, {"rotationSeconds", 16, false},
	}
	for _, c := range cases {
		in := validSpotlightUpdate()
		switch c.field {
		case "itemCount":
			in.ItemCount = c.value
		case "minHits":
			in.MinHits = c.value
		case "intervalHours":
			in.IntervalHours = c.value
		case "rotationSeconds":
			in.RotationSeconds = &c.value
		}
		cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in)
		if !c.ok {
			wantField(t, c.field, err, c.field, "invalid_value")
			continue
		}
		if err != nil {
			t.Errorf("%s %d: %v", c.field, c.value, err)
			continue
		}
		got := map[string]int{"itemCount": cfg.ItemCount, "minHits": cfg.MinHits, "intervalHours": cfg.IntervalHours,
			"rotationSeconds": cfg.RotationSeconds}[c.field]
		if got != c.value {
			t.Errorf("%s stored %d, want %d", c.field, got, c.value)
		}
	}
	// Several bad values name every field.
	in := validSpotlightUpdate()
	in.ItemCount, in.MinHits, in.IntervalHours = 0, 99, 0
	var verr *model.ValidationError
	if _, err := sf.svc.SetSpotlightConfig(sf.ctx, in); !errors.As(err, &verr) || len(verr.Fields) != 3 {
		t.Errorf("three bad values: %v", err)
	}
}

// rotationSeconds is optional in the update (concept 08 5.2): left out, the stored value stays,
// so an older administration client does not reset it.
func TestSpotlightConfigKeepsRotation(t *testing.T) {
	sf := newSpotlightFixture(t)
	in := validSpotlightUpdate()
	seven := 7
	in.RotationSeconds = &seven
	if cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in); err != nil || cfg.RotationSeconds != 7 {
		t.Fatalf("set 7: %+v, %v", cfg, err)
	}
	in = validSpotlightUpdate()
	in.ItemCount = 12
	cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in)
	if err != nil || cfg.RotationSeconds != 7 || cfg.ItemCount != 12 {
		t.Fatalf("update without rotationSeconds: %+v, %v", cfg, err)
	}
	if got, err := sf.svc.SpotlightConfig(sf.ctx); err != nil || got.RotationSeconds != 7 {
		t.Errorf("stored %+v, %v", got, err)
	}
}

// The interval of the spotlight is the interval of the schedule trakt.refresh (one value, two
// doors, concept 09 4.3): SetSpotlightConfig writes it into the schedule row; the switch of the
// band is not the switch of the schedule.
func TestSpotlightConfigSyncsSchedule(t *testing.T) {
	sf := newSpotlightFixture(t)
	sched := &fakeSchedules{}
	sf.svc.WithSchedules(sched)
	in := validSpotlightUpdate()
	in.IntervalHours = 6
	in.Enabled = false
	if _, err := sf.svc.SetSpotlightConfig(sf.ctx, in); err != nil {
		t.Fatal(err)
	}
	if len(sched.updates) != 1 || sched.updates[0].typ != jobTraktRefresh {
		t.Fatalf("schedule updates = %+v, want one for trakt.refresh", sched.updates)
	}
	if u := sched.updates[0].u; u.Every == nil || *u.Every != 6*time.Hour || u.Enabled != nil || u.Window != nil || u.ClearWindow {
		t.Errorf("schedule update = %+v, want the interval 6 h only", u)
	}
}

// The media follows the list kind (plan 5.6), a watchlist address is refused (plan 5.7, rule 11),
// a user list address is stored with the kind url only, and the address never reaches the log.
func TestSpotlightConfigLists(t *testing.T) {
	sf := newSpotlightFixture(t)
	listURL := func() sql.NullString {
		var u sql.NullString
		if err := sf.f.DB.Read.QueryRowContext(sf.ctx, `SELECT list_url FROM spotlight_config WHERE id = 1`).Scan(&u); err != nil {
			t.Fatal(err)
		}
		return u
	}

	in := validSpotlightUpdate()
	in.ListKey, in.Media = "boxoffice", "show"
	_, err := sf.svc.SetSpotlightConfig(sf.ctx, in)
	wantField(t, "boxoffice show", err, "media", "invalid_value")
	in.Media = "movie"
	if cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in); err != nil || cfg.ListKey != "boxoffice" || cfg.Media != "movie" {
		t.Errorf("boxoffice movie: %+v, %v", cfg, err)
	}

	in = validSpotlightUpdate()
	in.ListKey = "weekly"
	_, err = sf.svc.SetSpotlightConfig(sf.ctx, in)
	wantField(t, "unknown kind", err, "listKey", "invalid_value")
	in.ListKey, in.Media = "trending", "all"
	_, err = sf.svc.SetSpotlightConfig(sf.ctx, in)
	wantField(t, "unknown media", err, "media", "invalid_value")

	in = validSpotlightUpdate()
	in.ListKey = "url"
	_, err = sf.svc.SetSpotlightConfig(sf.ctx, in)
	wantField(t, "url without address", err, "listUrl", "required")
	for _, bad := range []string{
		"https://trakt.tv/users/secretwatcher/watchlist",
		"https://trakt.tv/users/secretwatcher/watchlist/movies",
		"https://example.com/users/secretwatcher/lists/abc",
		"https://trakt.tv/movies/trending",
	} {
		in.ListURL = bad
		_, err = sf.svc.SetSpotlightConfig(sf.ctx, in)
		wantField(t, bad, err, "listUrl", "invalid_value")
	}
	in.ListURL = "https://trakt.tv/users/" + strings.Repeat("x", 500) + "/lists/abc"
	_, err = sf.svc.SetSpotlightConfig(sf.ctx, in)
	wantField(t, "long address", err, "listUrl", "too_long")
	if u := listURL(); u.Valid {
		t.Errorf("a refused address was stored: %q", u.String)
	}

	in.ListURL = " https://trakt.tv/users/secretwatcher/lists/cat-favourites "
	cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in)
	if err != nil || cfg.ListKey != "url" || cfg.ListURL != "https://trakt.tv/users/secretwatcher/lists/cat-favourites" {
		t.Fatalf("user list: %+v, %v", cfg, err)
	}
	if got, err := sf.svc.SpotlightConfig(sf.ctx); err != nil || got.ListURL != cfg.ListURL {
		t.Errorf("read back: %+v, %v", got, err)
	}

	// Another kind stores the address as NULL, even when the body carries one.
	in = validSpotlightUpdate()
	in.ListURL = "https://trakt.tv/users/secretwatcher/lists/cat-favourites"
	if cfg, err := sf.svc.SetSpotlightConfig(sf.ctx, in); err != nil || cfg.ListURL != "" {
		t.Errorf("trending: %+v, %v", cfg, err)
	}
	if u := listURL(); u.Valid {
		t.Errorf("trending kept the address %q", u.String)
	}

	logs := sf.logs.String()
	if !strings.Contains(logs, "spotlight config set") {
		t.Errorf("no log line:\n%s", logs)
	}
	if strings.Contains(logs, "secretwatcher") || strings.Contains(logs, "cat-favourites") {
		t.Errorf("log names the list address:\n%s", logs)
	}
}

// getSpotlightEntries lists the entries of the configured list with and without a library hit
// (S-4, DoD step 4 "Fehlende Titel sind sichtbar"), in fetch order, and the orphaned overrides.
func TestSpotlightEntries(t *testing.T) {
	sf := newSpotlightFixture(t)
	sf.entry(t, "trending", "movie", 2, 9999, "Not In The Library")
	sf.entry(t, "trending", "movie", 1, 8001, "Spotlight One")
	sf.entry(t, "trending", "show", 1, 8002, "A Show With A Film's Id")
	sf.entry(t, "popular", "movie", 1, 8002, "Other List")
	sf.exec(t, `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at) VALUES ('trending', 'movie', 9999, 1, 2, 'Fehlt noch', 1)`)
	sf.exec(t, `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at) VALUES ('trending', 'show', 7777, 0, 1, 'Weg', 1)`)
	sf.exec(t, `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, updated_at) VALUES ('popular', 'movie', 6666, 1, 1)`)

	got, err := sf.svc.SpotlightEntries(sf.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ListKey != "trending" || len(got.Entries) != 3 {
		t.Fatalf("entries %+v", got)
	}
	hit, miss, show := got.Entries[0], got.Entries[1], got.Entries[2]
	if hit.Media != "movie" || hit.TmdbID != 8001 || hit.Position != 1 || hit.Title != "Spotlight One" || hit.Year == nil || *hit.Year != 2026 ||
		hit.ItemID != "01J9SPOTLIGHT0000000000001" || hit.ItemTitle != "Spotlight 01" || hit.Hidden || hit.PinnedRank != nil || hit.Note != "" || hit.Orphaned {
		t.Errorf("hit %+v", hit)
	}
	if miss.TmdbID != 9999 || miss.Position != 2 || miss.ItemID != "" || miss.ItemTitle != "" || !miss.Hidden ||
		miss.PinnedRank == nil || *miss.PinnedRank != 2 || miss.Note != "Fehlt noch" || miss.Orphaned {
		t.Errorf("entry without a hit %+v", miss)
	}
	// A show matches series only: the film with TMDB id 8002 is no hit for it.
	if show.Media != "show" || show.TmdbID != 8002 || show.ItemID != "" {
		t.Errorf("show %+v", show)
	}
	if len(got.Orphans) != 1 {
		t.Fatalf("orphans %+v", got.Orphans)
	}
	if o := got.Orphans[0]; o.Media != "show" || o.TmdbID != 7777 || o.Position != 0 || !o.Orphaned || o.Hidden ||
		o.PinnedRank == nil || *o.PinnedRank != 1 || o.Note != "Weg" {
		t.Errorf("orphan %+v", o)
	}
}

// setSpotlightEntryOverride: hidden, pinned rank and note; null clears, an empty update deletes;
// a rank held by another title is model.ErrRankTaken (DoD step 2 "Rang ist eindeutig").
func TestSpotlightOverride(t *testing.T) {
	sf := newSpotlightFixture(t)
	sf.entry(t, "trending", "movie", 1, 8001, "Spotlight One")
	sf.entry(t, "trending", "movie", 2, 9999, "Not In The Library")
	yes, rank, note := true, 3, "Cat mag ihn"
	set := func(media string, tmdbID int64, upd model.SpotlightOverrideUpdate) (model.SpotlightEntry, error) {
		return sf.svc.SetSpotlightEntryOverride(sf.ctx, media, tmdbID, upd)
	}

	e, err := set("movie", 8001, model.SpotlightOverrideUpdate{Hidden: &yes})
	if err != nil || !e.Hidden || e.PinnedRank != nil || e.Note != "" || e.ItemID == "" || e.Position != 1 {
		t.Fatalf("hidden: %+v, %v", e, err)
	}
	e, err = set("movie", 8001, model.SpotlightOverrideUpdate{PinnedRank: &rank})
	if err != nil || !e.Hidden || e.PinnedRank == nil || *e.PinnedRank != 3 {
		t.Fatalf("rank keeps hidden: %+v, %v", e, err)
	}
	e, err = set("movie", 8001, model.SpotlightOverrideUpdate{Note: &note})
	if err != nil || !e.Hidden || e.PinnedRank == nil || *e.PinnedRank != 3 || e.Note != note {
		t.Fatalf("note keeps the rest: %+v, %v", e, err)
	}
	// The same title may keep its own rank.
	if _, err := set("movie", 8001, model.SpotlightOverrideUpdate{PinnedRank: &rank}); err != nil {
		t.Errorf("own rank again: %v", err)
	}

	// Another title on the same rank: 409 rank_taken, nothing written.
	if _, err := set("movie", 9999, model.SpotlightOverrideUpdate{PinnedRank: &rank}); !errors.Is(err, model.ErrRankTaken) {
		t.Errorf("second title on rank 3: %v", err)
	}
	if e, err := sf.svc.SpotlightEntries(sf.ctx); err != nil || e.Entries[1].PinnedRank != nil || e.Entries[1].Hidden {
		t.Errorf("refused rank was written: %+v, %v", e, err)
	}

	zero, long, empty := 0, strings.Repeat("ä", 281), ""
	_, err = set("movie", 8001, model.SpotlightOverrideUpdate{PinnedRank: &zero})
	wantField(t, "rank 0", err, "pinnedRank", "invalid_value")
	_, err = set("movie", 8001, model.SpotlightOverrideUpdate{Note: &long})
	wantField(t, "long note", err, "note", "too_long")
	_, err = set("film", 8001, model.SpotlightOverrideUpdate{Hidden: &yes})
	wantField(t, "media film", err, "media", "invalid_value")
	if e, err := set("movie", 8001, model.SpotlightOverrideUpdate{Note: &empty}); err != nil || e.Note != "" || *e.PinnedRank != 3 {
		t.Errorf("empty note: %+v, %v", e, err)
	}
	if _, err := set("movie", 8001, model.SpotlightOverrideUpdate{Note: &note}); err != nil {
		t.Fatal(err)
	}

	// null clears a single value.
	e, err = set("movie", 8001, model.SpotlightOverrideUpdate{ClearPinnedRank: true, ClearNote: true})
	if err != nil || !e.Hidden || e.PinnedRank != nil || e.Note != "" {
		t.Fatalf("clear: %+v, %v", e, err)
	}
	// The rank is free again.
	if e, err := set("movie", 9999, model.SpotlightOverrideUpdate{PinnedRank: &rank}); err != nil || *e.PinnedRank != 3 || e.ItemID != "" {
		t.Errorf("rank after clearing: %+v, %v", e, err)
	}

	// An empty body removes the override; on an entry without override it is a no-op.
	e, err = set("movie", 8001, model.SpotlightOverrideUpdate{Empty: true})
	if err != nil || e.Hidden || e.PinnedRank != nil || e.Note != "" || e.Position != 1 {
		t.Fatalf("empty: %+v, %v", e, err)
	}
	if _, err := set("movie", 8001, model.SpotlightOverrideUpdate{Empty: true}); err != nil {
		t.Errorf("empty again: %v", err)
	}
	var n int
	if err := sf.f.DB.Read.QueryRowContext(sf.ctx, `SELECT count(*) FROM spotlight_overrides WHERE tmdb_id = 8001`).Scan(&n); err != nil || n != 0 {
		t.Errorf("override rows of 8001: %d, %v", n, err)
	}

	// Neither entry nor override: 404. The same TMDB id as a show or in another list is another title.
	if _, err := set("movie", 4242, model.SpotlightOverrideUpdate{Hidden: &yes}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}
	if _, err := set("show", 8001, model.SpotlightOverrideUpdate{Hidden: &yes}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("show with a film's id: %v", err)
	}

	// An orphan (override without entry) can be changed and deleted.
	sf.exec(t, `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, note, updated_at) VALUES ('trending', 'show', 7777, 1, 'Weg', 1)`)
	no := false
	if e, err := set("show", 7777, model.SpotlightOverrideUpdate{Hidden: &no}); err != nil || !e.Orphaned || e.Hidden || e.Note != "Weg" {
		t.Errorf("orphan change: %+v, %v", e, err)
	}
	if e, err := set("show", 7777, model.SpotlightOverrideUpdate{Empty: true}); err != nil || !e.Orphaned || e.Note != "" {
		t.Errorf("orphan delete: %+v, %v", e, err)
	}
	if got, err := sf.svc.SpotlightEntries(sf.ctx); err != nil || len(got.Orphans) != 0 {
		t.Errorf("orphans after delete: %+v, %v", got.Orphans, err)
	}
	if _, err := set("show", 7777, model.SpotlightOverrideUpdate{Empty: true}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted orphan: %v", err)
	}
}

// refreshSpotlight: at most once a minute, also across a restart (the stamp is in the database,
// DoD step 4 "Grenzen"); without a client id 422 and no stamp.
func TestSpotlightRefresh(t *testing.T) {
	sf := newSpotlightFixture(t)
	stamp := func() sql.NullInt64 {
		var v sql.NullInt64
		if err := sf.f.DB.Read.QueryRowContext(sf.ctx, `SELECT last_manual_refresh FROM spotlight_config WHERE id = 1`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	sf.trakt.err = model.ErrProviderNotConfigured
	if err := sf.svc.RefreshSpotlight(sf.ctx); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Fatalf("without client id: %v", err)
	}
	if v := stamp(); v.Valid {
		t.Errorf("stamped without client id: %d", v.Int64)
	}
	sf.trakt.err = nil
	if err := sf.svc.RefreshSpotlight(sf.ctx); err != nil || sf.trakt.refreshes != 2 {
		t.Fatalf("first refresh with client id: %v, %d calls", err, sf.trakt.refreshes)
	}
	if v := stamp(); !v.Valid || v.Int64 != sf.now.UnixMilli() {
		t.Errorf("stamp %+v, want %d", v, sf.now.UnixMilli())
	}

	sf.now = sf.now.Add(59 * time.Second)
	if err := sf.svc.RefreshSpotlight(sf.ctx); !errors.Is(err, model.ErrRefreshTooSoon) {
		t.Errorf("second within a minute: %v", err)
	}
	if err := sf.service().RefreshSpotlight(sf.ctx); !errors.Is(err, model.ErrRefreshTooSoon) {
		t.Errorf("after a restart within a minute: %v", err)
	}
	if sf.trakt.refreshes != 2 {
		t.Errorf("refused refreshes reached Trakt: %d calls", sf.trakt.refreshes)
	}

	sf.now = sf.now.Add(time.Second)
	if err := sf.service().RefreshSpotlight(sf.ctx); err != nil || sf.trakt.refreshes != 3 {
		t.Errorf("after 60 s: %v, %d calls", err, sf.trakt.refreshes)
	}

	// A stamp in the future (clock set back) does not block.
	sf.now = sf.now.Add(-time.Hour)
	if err := sf.svc.RefreshSpotlight(sf.ctx); err != nil {
		t.Errorf("clock set back: %v", err)
	}

	sf.svc.WithTrakt(nil)
	sf.now = sf.now.Add(time.Hour)
	if err := sf.svc.RefreshSpotlight(sf.ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("without WithTrakt: %v", err)
	}
}
