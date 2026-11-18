package notify

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// addWish inserts a wish in processing and its requesters: the first profile is the owner, the
// others joined (F-7).
func (e *chanEnv) addWish(t *testing.T, id int64, media, title string, profiles ...model.Profile) {
	t.Helper()
	e.exec(t, `INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at)
	           VALUES (?, ?, ?, ?, 'slouchcat', 'processing', 1, 1)`, id, media, 1000+id, title)
	for i, p := range profiles {
		role := "joined"
		if i == 0 {
			role = "owner"
		}
		e.exec(t, `INSERT INTO wish_requesters (wish_id, profile_id, account_id, role, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, p.ID, p.AccountID, role, i+1)
	}
}

type wishDelivery struct {
	account, profile           int64
	event, subject, body, link string
}

// wishDeliveries returns the deliveries of an event in the order they were stored.
func (e *chanEnv) wishDeliveries(t *testing.T, event string) []wishDelivery {
	t.Helper()
	rows, err := e.f.DB.Read.Query(`SELECT account_id, COALESCE(profile_id, 0), event, subject, body, COALESCE(link, '')
	                                FROM deliveries WHERE event = ? ORDER BY id`, event)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []wishDelivery
	for rows.Next() {
		var d wishDelivery
		if err := rows.Scan(&d.account, &d.profile, &d.event, &d.subject, &d.body, &d.link); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func ids(profiles ...model.Profile) []int64 {
	out := make([]int64, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.ID)
	}
	return out
}

// F-9, F-13, F-7: «verfügbar» reaches only the requesters that may see the title, each with an
// inbox entry of its own and a message of its own on the channels of its account; the others wait
// and get theirs once they may see it, the profiles notified before get nothing a second time.
func TestWishAvailableOnlyVisibleProfiles(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable)
	film := f.Rated(16)
	e.addWish(t, 1, "movie", "Movie 16", f.P18, f.P12, f.PG)

	out, err := e.svc.WishAvailable(ctx, 1, film.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Notified, ids(f.P18)) || !slices.Equal(out.Waiting, ids(f.P12, f.PG)) {
		t.Fatalf("first pass = %+v, want notified %v and waiting %v", out, ids(f.P18), ids(f.P12, f.PG))
	}
	title := func(p model.Profile) string {
		s, err := e.svc.vis.Summary(ctx, visibility.Profile(p), film.ID)
		if err != nil {
			t.Fatal(err)
		}
		return s.Title
	}
	subject := func(p model.Profile) string {
		return "[" + p.Name + "] " + localeText(t, p.Language, "wish_available_subject", vars{"title": title(p)})
	}
	got := e.wishDeliveries(t, model.EventWishAvailable)
	want := []wishDelivery{{f.Account.ID, f.P18.ID, model.EventWishAvailable, subject(f.P18),
		localeText(t, "de", "wish_available_movie_body", nil), "/items/" + film.ID}}
	if !slices.Equal(got, want) {
		t.Fatalf("deliveries = %+v, want %+v", got, want)
	}
	for _, p := range []model.Profile{f.P12, f.PG, f.P16} {
		if n := e.count(t, `SELECT count(*) FROM notifications WHERE profile_id = ?`, p.ID); n != 0 {
			t.Errorf("profile %s has %d entries before it may see the title", p.Name, n)
		}
	}

	// The title is rated 12 now: the two who waited get theirs, each on its own account.
	f.SetRating(t, film.ID, ptr(12))
	out, err = e.svc.WishAvailable(ctx, 1, film.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Notified, ids(f.P12, f.PG)) || len(out.Waiting) != 0 {
		t.Fatalf("second pass = %+v, want notified %v", out, ids(f.P12, f.PG))
	}
	got = e.wishDeliveries(t, model.EventWishAvailable)
	if len(got) != 3 || got[1].account != f.Account.ID || got[1].profile != f.P12.ID || got[1].subject != subject(f.P12) ||
		got[2].account != f.Guest.ID || got[2].profile != f.PG.ID || got[2].subject != subject(f.PG) ||
		got[2].body != localeText(t, "en", "wish_available_movie_body", nil) {
		t.Errorf("deliveries after the second pass = %+v", got)
	}
	for _, p := range []model.Profile{f.P18, f.P12, f.PG} {
		if n := e.count(t, `SELECT count(*) FROM notifications WHERE profile_id = ? AND kind = 'wish_available'
		                    AND item_id = ? AND wish_id = 1 AND dedupe_key = 'wish:1:available'`, p.ID, film.ID); n != 1 {
			t.Errorf("profile %s: %d entries, want 1", p.Name, n)
		}
		if n := e.count(t, `SELECT count(*) FROM wish_requesters WHERE wish_id = 1 AND profile_id = ? AND notified_at IS NOT NULL`, p.ID); n != 1 {
			t.Errorf("profile %s: notified_at not set", p.Name)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM notifications WHERE profile_id = ?`, f.P16.ID); n != 0 {
		t.Errorf("a profile that did not wish got %d entries", n)
	}
	if _, err := e.svc.WishAvailable(ctx, 99, film.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown wish: %v", err)
	}
}

// Section 7, AK-62, AK-126: a second pass sends nothing, also after the read inbox entries were
// cleared away, because notified_at is set.
func TestWishAvailableOnceAlsoAfterCleanup(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable)
	film := f.Rated(6)
	e.addWish(t, 1, "movie", "Movie 06", f.P18, f.P6)
	if out, err := e.svc.WishAvailable(ctx, 1, film.ID); err != nil || len(out.Notified) != 2 {
		t.Fatalf("first pass = %+v, %v", out, err)
	}
	if out, err := e.svc.WishAvailable(ctx, 1, film.ID); err != nil || len(out.Notified)+len(out.Waiting) != 0 {
		t.Fatalf("second pass = %+v, %v; want nothing", out, err)
	}
	e.exec(t, `DELETE FROM notifications`)
	if out, err := e.svc.WishAvailable(ctx, 1, film.ID); err != nil || len(out.Notified)+len(out.Waiting) != 0 {
		t.Fatalf("pass after the cleanup = %+v, %v; want nothing", out, err)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications`); n != 0 {
		t.Errorf("%d entries came back after the cleanup", n)
	}
	if got := e.wishDeliveries(t, model.EventWishAvailable); len(got) != 2 {
		t.Errorf("deliveries = %+v, want one per profile", got)
	}
}

// Section 8: an entry of a wish without a title is in the list, the bell counter and «mark read» of
// the requester and the co-requester, never of another profile, also not when a row for that
// profile exists.
func TestWishEntryWithoutItemReadPath(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.addWish(t, 1, "movie", "Paddington in Peru", f.P18, f.PG)
	for _, p := range []model.Profile{f.P18, f.PG, f.P16} {
		e.exec(t, `INSERT INTO notifications (profile_id, kind, wish_id, dedupe_key, created_at) VALUES (?, 'wish_declined', 1, 'wish:1:declined', 5)`, p.ID)
	}
	for _, p := range []model.Profile{f.P18, f.PG} {
		list, total, unread, err := e.svc.Notifications(ctx, p, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || unread != 1 || len(list) != 1 || list[0].Kind != "wish_declined" || list[0].Item.ID != "" ||
			list[0].Wish == nil || *list[0].Wish != (model.NotificationWish{ID: 1, Media: "movie", Title: "Paddington in Peru", State: "processing"}) {
			t.Fatalf("%s: list %+v, total %d, unread %d", p.Name, list, total, unread)
		}
	}
	list, total, unread, err := e.svc.Notifications(ctx, f.P16, 0, 50)
	if err != nil || len(list) != 0 || total != 0 || unread != 0 {
		t.Errorf("another profile: list %+v, total %d, unread %d, err %v", list, total, unread, err)
	}
	var p16Entry int64
	if err := e.f.DB.Read.QueryRow(`SELECT id FROM notifications WHERE profile_id = ?`, f.P16.ID).Scan(&p16Entry); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.MarkRead(ctx, f.P16, p16Entry); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("another profile marked the entry read: %v", err)
	}
	if err := e.svc.MarkAllRead(ctx, f.P16); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications WHERE profile_id = ? AND read_at IS NULL`, f.P16.ID); n != 1 {
		t.Errorf("mark all read of another profile touched the entry")
	}
	mine, _, _, err := e.svc.Notifications(ctx, f.P18, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.MarkRead(ctx, f.P18, mine[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.MarkAllRead(ctx, f.PG); err != nil {
		t.Fatal(err)
	}
	for _, p := range []model.Profile{f.P18, f.PG} {
		if _, _, unread, err := e.svc.Notifications(ctx, p, 0, 50); err != nil || unread != 0 {
			t.Errorf("%s: unread %d after marking read (%v)", p.Name, unread, err)
		}
	}
}

// F-15: the external message of a PIN profile names no title and links to the wishes; its inbox
// entry keeps the title. With the setting off the message names it. A series says that every
// wished season is there.
func TestWishAvailablePINProfileWithoutTitle(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable)
	e.exec(t, `UPDATE profiles SET pin_hash = 'argon2id-test-hash' WHERE id = ?`, f.P16.ID)
	film := f.Rated(12)
	e.addWish(t, 1, "movie", "Movie 12", f.P16)
	if _, err := e.svc.WishAvailable(ctx, 1, film.ID); err != nil {
		t.Fatal(err)
	}
	got := e.wishDeliveries(t, model.EventWishAvailable)
	want := []wishDelivery{{f.Account.ID, f.P16.ID, model.EventWishAvailable,
		"[" + f.P16.Name + "] " + localeText(t, "de", "wish_hidden_subject", nil), localeText(t, "de", "wish_hidden_body", nil), "/wishes"}}
	if !slices.Equal(got, want) {
		t.Fatalf("deliveries of the PIN profile = %+v, want %+v", got, want)
	}
	list, _, _, err := e.svc.Notifications(ctx, f.P16, 0, 50)
	if err != nil || len(list) != 1 || list[0].Item.ID != film.ID {
		t.Fatalf("inbox of the PIN profile = %+v, %v", list, err)
	}

	e.exec(t, `INSERT INTO settings (key, value, updated_at) VALUES ('wishes.pin_external_hide_title', '0', 1)`)
	e.addWish(t, 2, "show", "Show", f.P16)
	if _, err := e.svc.WishAvailable(ctx, 2, f.Show.Series.ID); err != nil {
		t.Fatal(err)
	}
	series, err := e.svc.vis.Summary(ctx, visibility.Profile(f.P16), f.Show.Series.ID)
	if err != nil {
		t.Fatal(err)
	}
	got = e.wishDeliveries(t, model.EventWishAvailable)
	want = append(want, wishDelivery{f.Account.ID, f.P16.ID, model.EventWishAvailable,
		"[" + f.P16.Name + "] " + localeText(t, "de", "wish_available_subject", vars{"title": series.Title}),
		localeText(t, "de", "wish_available_show_body", nil), "/items/" + f.Show.Series.ID})
	if !slices.Equal(got, want) {
		t.Errorf("deliveries with the setting off = %+v, want %+v", got, want)
	}
}

// addSeason adds a wished season in processing to a series wish.
func (e *chanEnv) addSeason(t *testing.T, wishID int64, season int) {
	t.Helper()
	e.exec(t, `INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, state) VALUES (?, ?, ?, 'processing')`, wishID, season, 1000+wishID)
}

// Section 3.3, AK-82: a season message comes only once the season is complete for the profile
// (its playable, visible episodes reach the count), with its own text for a season still on air,
// once per profile; wish_seasons.notified_at ends it once nobody waits, and no season message
// follows the message about the whole series.
func TestWishSeasonOnlyWhenComplete(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable)
	// Season 1 of the fixture series: four episodes for P18 (S01E01, E02 rated 16, E03-E04), three for P12.
	e.addWish(t, 1, "show", "Show", f.P18, f.P12)
	e.addSeason(t, 1, 1)
	in := WishSeason{WishID: 1, SeriesID: f.Show.Series.ID, Season: 1, Episodes: 5}
	out, err := e.svc.WishSeasonAvailable(ctx, in)
	if err != nil || len(out.Notified) != 0 || !slices.Equal(out.Waiting, ids(f.P18, f.P12)) {
		t.Fatalf("incomplete season = %+v, %v; want both waiting", out, err)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications`); n != 0 {
		t.Fatalf("%d entries for an incomplete season", n)
	}
	in.Episodes = 4
	out, err = e.svc.WishSeasonAvailable(ctx, in)
	if err != nil || !slices.Equal(out.Notified, ids(f.P18)) || !slices.Equal(out.Waiting, ids(f.P12)) {
		t.Fatalf("season complete for P18 = %+v, %v", out, err)
	}
	series, err := e.svc.vis.Summary(ctx, visibility.Profile(f.P18), f.Show.Series.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := e.wishDeliveries(t, model.EventWishAvailable)
	want := []wishDelivery{{f.Account.ID, f.P18.ID, model.EventWishAvailable,
		"[" + f.P18.Name + "] " + localeText(t, "de", "wish_season_subject", vars{"title": series.Title, "season": 1}),
		localeText(t, "de", "wish_season_body", nil), "/items/" + f.Show.Season.ID}}
	if !slices.Equal(got, want) {
		t.Fatalf("deliveries = %+v, want %+v", got, want)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications WHERE profile_id = ? AND kind = 'wish_season_available'
	                    AND item_id = ? AND parent_id = ? AND dedupe_key = 'wish:1:season:1'`, f.P18.ID, f.Show.Season.ID, f.Show.Series.ID); n != 1 {
		t.Errorf("season entries of P18 = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM wish_seasons WHERE wish_id = 1 AND notified_at IS NULL`); n != 1 {
		t.Errorf("notified_at set while P12 waits")
	}
	// Again: P18 has its entry, P12 still waits.
	if out, err := e.svc.WishSeasonAvailable(ctx, in); err != nil || len(out.Notified) != 0 || !slices.Equal(out.Waiting, ids(f.P12)) {
		t.Errorf("second pass = %+v, %v", out, err)
	}

	// A season on air: the three episodes aired so far are there for both; then notified_at ends it.
	e.addWish(t, 2, "show", "Show", f.P18, f.P12)
	e.addSeason(t, 2, 1)
	aired := WishSeason{WishID: 2, SeriesID: f.Show.Series.ID, Season: 1, Episodes: 3, Airing: true}
	if out, err := e.svc.WishSeasonAvailable(ctx, aired); err != nil || !slices.Equal(out.Notified, ids(f.P18, f.P12)) {
		t.Fatalf("aired season = %+v, %v", out, err)
	}
	got = e.wishDeliveries(t, model.EventWishAvailable)
	if len(got) != 3 || got[2].subject != "["+f.P12.Name+"] "+localeText(t, "de", "wish_season_aired_subject", vars{"title": series.Title, "season": 1}) ||
		got[2].body != localeText(t, "de", "wish_season_aired_body", nil) {
		t.Errorf("deliveries of the aired season = %+v", got)
	}
	if n := e.count(t, `SELECT count(*) FROM wish_seasons WHERE wish_id = 2 AND notified_at IS NOT NULL`); n != 1 {
		t.Errorf("notified_at of the aired season not set")
	}
	e.exec(t, `DELETE FROM notifications`)
	if out, err := e.svc.WishSeasonAvailable(ctx, aired); err != nil || len(out.Notified)+len(out.Waiting) != 0 {
		t.Errorf("pass after notified_at = %+v, %v; want nothing", out, err)
	}

	// The whole series of wish 1 is available: one message, and no season message after it.
	if out, err := e.svc.WishAvailable(ctx, 1, f.Show.Series.ID); err != nil || !slices.Equal(out.Notified, ids(f.P18, f.P12)) {
		t.Fatalf("series available = %+v, %v", out, err)
	}
	if n := e.count(t, `SELECT count(*) FROM wish_seasons WHERE wish_id = 1 AND notified_at IS NOT NULL`); n != 1 {
		t.Errorf("the seasons of a fulfilled wish are not marked notified")
	}
	before := len(e.wishDeliveries(t, model.EventWishAvailable))
	in.Episodes = 3
	if out, err := e.svc.WishSeasonAvailable(ctx, in); err != nil || len(out.Notified)+len(out.Waiting) != 0 {
		t.Errorf("season after the series = %+v, %v; want nothing", out, err)
	}
	if after := len(e.wishDeliveries(t, model.EventWishAvailable)); after != before {
		t.Errorf("%d season messages after the series message", after-before)
	}
	for name, bad := range map[string]WishSeason{
		"film wish":         {WishID: 3, SeriesID: f.Show.Series.ID, Season: 1, Episodes: 1},
		"season not wished": {WishID: 1, SeriesID: f.Show.Series.ID, Season: 2, Episodes: 1},
	} {
		if name == "film wish" {
			e.addWish(t, 3, "movie", "Movie 06", f.P18)
		}
		if _, err := e.svc.WishSeasonAvailable(ctx, bad); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: %v, want not found", name, err)
		}
	}
}

// Section 2, F-14: approved, declined and failed reach the inbox of every requester (without a
// title, read through the wish) and the channels only where the event wish_status was chosen by
// hand; each once; failed names its reason; a PIN profile's message names no title.
func TestWishStatusOnlyWhenChannelEventOn(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable)
	e.addWish(t, 1, "movie", "Paddington in Peru", f.P18, f.PG)
	if err := e.svc.WishApproved(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := e.wishDeliveries(t, model.EventWishStatus); len(got) != 0 {
		t.Fatalf("status sent without the event chosen: %+v", got)
	}
	for _, p := range []model.Profile{f.P18, f.PG} {
		list, _, unread, err := e.svc.Notifications(ctx, p, 0, 50)
		if err != nil || len(list) != 1 || list[0].Kind != KindWishApproved || list[0].Wish == nil || list[0].Wish.ID != 1 || unread != 1 {
			t.Errorf("%s: inbox %+v, unread %d, %v", p.Name, list, unread, err)
		}
	}

	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishAvailable, model.EventWishStatus)
	for range 2 {
		if err := e.svc.WishDeclined(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	got := e.wishDeliveries(t, model.EventWishStatus)
	want := []wishDelivery{{f.Account.ID, f.P18.ID, model.EventWishStatus,
		"[" + f.P18.Name + "] " + localeText(t, "de", "wish_declined_subject", vars{"title": "Paddington in Peru"}),
		localeText(t, "de", "wish_declined_body", nil), "/wishes?focus=1"}}
	if !slices.Equal(got, want) {
		t.Fatalf("declined deliveries = %+v, want %+v", got, want)
	}

	e.exec(t, `UPDATE profiles SET pin_hash = 'argon2id-test-hash' WHERE id = ?`, f.P16.ID)
	e.addWish(t, 2, "movie", "Movie 18", f.P18, f.P16)
	if err := e.svc.WishFailed(ctx, 2, "quota"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.WishFailed(ctx, 1, "something_new"); err != nil {
		t.Fatal(err)
	}
	got = e.wishDeliveries(t, model.EventWishStatus)[1:]
	want = []wishDelivery{
		{f.Account.ID, f.P18.ID, model.EventWishStatus, "[" + f.P18.Name + "] " + localeText(t, "de", "wish_failed_subject", vars{"title": "Movie 18"}),
			localeText(t, "de", "wish_failed_quota", nil), "/wishes?focus=2"},
		{f.Account.ID, f.P16.ID, model.EventWishStatus, "[" + f.P16.Name + "] " + localeText(t, "de", "wish_status_hidden_subject", nil),
			localeText(t, "de", "wish_hidden_body", nil), "/wishes"},
		{f.Account.ID, f.P18.ID, model.EventWishStatus, "[" + f.P18.Name + "] " + localeText(t, "de", "wish_failed_subject", vars{"title": "Paddington in Peru"}),
			localeText(t, "de", "wish_failed_body", nil), "/wishes?focus=1"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("failed deliveries = %+v, want %+v", got, want)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications WHERE item_id IS NULL AND wish_id IS NOT NULL`); n != 8 {
		t.Errorf("%d status entries, want 8 (approved, declined, failed of wish 1 for two profiles, failed of wish 2 for two)", n)
	}
	if err := e.svc.WishApproved(ctx, 99); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown wish: %v", err)
	}
}

// F-14, section 5: wish_review reaches the switched-on channels of the administrators that chose
// it, never another account (also not one whose row carries it), without a profile prefix, once
// per wish; an account without administration may not choose it.
func TestWishReviewOnlyAdministrators(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	f := e.f
	e.enable(t, e.owner(), model.ChannelEmail, model.EventWishReview)
	e.enable(t, e.admin2(), model.ChannelEmail, model.EventWishReview)
	if _, err := e.svc.UpdateChannel(ctx, e.guest(), model.ChannelEmail,
		model.ChannelSettingUpdate{Enabled: true, Events: []string{model.EventWishReview}}); !errors.Is(err, model.ErrEventNotAllowed) {
		t.Errorf("an account without administration chose wish_review: %v", err)
	}
	e.exec(t, `UPDATE account_channels SET events = '["wish_review"]', enabled = 1 WHERE account_id = ? AND channel = 'email'`, f.Guest.ID)
	e.addWish(t, 1, "show", "Tulsa King", f.PG)
	for range 2 {
		if err := e.svc.WishReview(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	subject := localeText(t, "de", "wish_review_subject", vars{"title": "Tulsa King"})
	body := localeText(t, "de", "wish_review_show_body", vars{"account": f.Guest.Email, "profile": f.PG.Name})
	got := e.wishDeliveries(t, model.EventWishReview)
	want := []wishDelivery{
		{f.Account.ID, 0, model.EventWishReview, subject, body, "/admin/wishes?state=pending"},
		{f.Admin2.ID, 0, model.EventWishReview, subject, body, "/admin/wishes?state=pending"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("review deliveries = %+v, want %+v", got, want)
	}
	if n := e.count(t, `SELECT count(*) FROM notifications`); n != 0 {
		t.Errorf("wish_review wrote %d inbox entries", n)
	}
	if err := e.svc.WishReview(ctx, 99); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown wish: %v", err)
	}
}
