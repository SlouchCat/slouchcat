package vistest

import (
	"fmt"
	"testing"
)

// Discover is the M7 fixture of AddDiscover (design 3.1, plan 10.2).
type Discover struct {
	// MixedCollection is the TMDB collection "Test Collection" / "Sammlung Test" with the films
	// rated 6, 12 and 18 in this order (mixed ratings, 10.2).
	MixedCollection string
	// ManualCollection is the admin's collection "Late Night" with only the film rated 18.
	ManualCollection string
	// PickNote is the note of the pick of P12 on the film rated 12.
	PickNote string
}

// Fixed ids and names of AddDiscover.
const (
	MixedCollectionID    = "01J9000000000000000000CM1X"
	MixedCollectionName  = "Test Collection"
	MixedCollectionDe    = "Sammlung Test"
	ManualCollectionID   = "01J9000000000000000000CM18"
	ManualCollectionName = "Late Night"
	// ExpiredPickEnd is the end of the window of the expired pick (1970-01-01T00:16:40Z); the
	// future pick starts at FuturePickStart (2100-01-01T00:00:00Z). Both are global picks of the
	// film rated 0.
	ExpiredPickEnd  int64 = 1_000_000
	FuturePickStart int64 = 4_102_444_800_000
)

// AddDiscover adds the M7 data: the TMDB collection MixedCollectionID with the films rated 6, 12
// and 18, the manual collection ManualCollectionID with only the film rated 18, the watchlist
// entries of P6 (film rated 6) and P18 (film rated 18), the favourite of P18 on the series of
// AddSeries (which it needs), a pick only for P12 on the film rated 12 with a note, and two global
// picks of the film rated 0 outside their time window (one expired, one in the future). The start
// page rows keep the default order of migration 00015.
func (f *Fixture) AddDiscover(t testing.TB) {
	t.Helper()
	if f.Show.Series.ID == "" {
		t.Fatal("vistest: AddDiscover needs AddSeries")
	}
	f.Discover = Discover{MixedCollection: MixedCollectionID, ManualCollection: ManualCollectionID, PickNote: "Für P12 ausgesucht"}
	exec(t, f.DB, `INSERT INTO collections (id, source, tmdb_id, name, name_de, overview, overview_de, created_at, updated_at)
	               VALUES (?, 'tmdb', 5000, ?, ?, 'Three films', 'Drei Filme', 1, 1),
	                      (?, 'manual', NULL, ?, NULL, NULL, NULL, 1, 1)`,
		MixedCollectionID, MixedCollectionName, MixedCollectionDe, ManualCollectionID, ManualCollectionName)
	for i, m := range []Movie{f.Rated(6), f.Rated(12), f.Rated(18)} {
		exec(t, f.DB, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, ?, 1)`, MixedCollectionID, m.ID, i)
	}
	exec(t, f.DB, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 0, 1)`, ManualCollectionID, f.Rated(18).ID)
	exec(t, f.DB, `INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (?, ?, 10), (?, ?, 10)`,
		f.P6.ID, f.Rated(6).ID, f.P18.ID, f.Rated(18).ID)
	exec(t, f.DB, `INSERT INTO favorites (profile_id, item_id, created_at) VALUES (?, ?, 10)`, f.P18.ID, f.Show.Series.ID)
	exec(t, f.DB, `INSERT INTO top_picks (item_id, profile_id, position, note, created_at) VALUES (?, ?, 5, ?, 1)`,
		f.Rated(12).ID, f.P12.ID, f.Discover.PickNote)
	exec(t, f.DB, `INSERT INTO top_picks (item_id, position, ends_at, created_at) VALUES (?, 6, ?, 1)`, f.Rated(0).ID, ExpiredPickEnd)
	exec(t, f.DB, `INSERT INTO top_picks (item_id, position, starts_at, created_at) VALUES (?, 7, ?, 1)`, f.Rated(0).ID, FuturePickStart)
}

// AddSpotlightFilms adds n films rated 0 (visible to every profile) with the TMDB ids 8001..8000+n
// and one playable file each, for the spotlight tests of M11: a spotlight_entries row with tmdb_id
// 8000+k maps to film k. The ids are fixed, so a test can name a film by k.
func (f *Fixture) AddSpotlightFilms(t testing.TB, n int) {
	t.Helper()
	for k := 1; k <= n; k++ {
		id := fmt.Sprintf("01J9SPOTLIGHT00000000%05d", k)
		exec(t, f.DB, `INSERT INTO items (id, kind, library_id, title, sort_title, genres, tmdb_id, own_rating, metadata_state, added_at, created_at, updated_at)
		               VALUES (?, 'movie', ?, ?, ?, '[]', ?, 0, 'matched', 1, 1, 1)`, id, f.libraryID, fmt.Sprintf("Spotlight %02d", k), fmt.Sprintf("spotlight %02d", k), 8000+k)
		exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec, width, height, duration_ms, created_at, updated_at)
		               VALUES (?, ?, ?, ?, 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, 1000, 1, 1)`, fmt.Sprintf("01J9SPOTFILE000000000%05d", k), id, f.rootID, fmt.Sprintf("spotlight-%02d.mp4", k))
	}
}
