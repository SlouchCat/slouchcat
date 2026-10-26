package vistest

import (
	"database/sql"
	"fmt"
	"testing"
)

// ReleasedEpisode is one episode of AddReleasedSeries: its own rating and its release_date
// (YYYY-MM-DD, "" = no date).
type ReleasedEpisode struct {
	Rating int
	Date   string
}

// ReleasedFilmID is the item id of film n of AddReleasedFilm.
func ReleasedFilmID(n int) string { return fmt.Sprintf("01J9RELEASEDF%013d", n) }

// ReleasedSeriesID is the item id of series n of AddReleasedSeries.
func ReleasedSeriesID(n int) string { return fmt.Sprintf("01J9RELEASEDS%013d", n) }

// AddReleasedFilm adds film n to library 1 for the tests of the row "recently released" (M11,
// S-5): own rating rating, release_date date ("" = no date) and one playable file; it returns the
// item id ReleasedFilmID(n).
func (f *Fixture) AddReleasedFilm(t testing.TB, n, rating int, date string) string {
	t.Helper()
	id := ReleasedFilmID(n)
	f.addReleasedItem(t, id, "movie", "", sql.NullInt64{Int64: int64(rating), Valid: true}, date)
	f.addReleasedFile(t, id, fmt.Sprintf("released/film-%d.mp4", n))
	return id
}

// AddReleasedSeries adds series n to library 1 for the tests of S-6 (a series counts with its
// newest visible episode): the series with own rating rating, one season without own rating and
// the episodes in their order, each with its own rating, its release_date and one playable file.
// The series and the season carry no release_date, so only the episodes can date the series. The
// triggers derive the container ratings: the series is visible to a profile that may see at least
// one episode. It returns the item id ReleasedSeriesID(n).
func (f *Fixture) AddReleasedSeries(t testing.TB, n, rating int, episodes []ReleasedEpisode) string {
	t.Helper()
	id := ReleasedSeriesID(n)
	season := fmt.Sprintf("01J9RELEASEDN%013d", n)
	f.addReleasedItem(t, id, "series", "", sql.NullInt64{Int64: int64(rating), Valid: true}, "")
	f.addReleasedItem(t, season, "season", id, sql.NullInt64{}, "")
	for k, e := range episodes {
		ep := fmt.Sprintf("01J9RELEASEDE%010d%03d", n, k+1)
		f.addReleasedItem(t, ep, "episode", season, sql.NullInt64{Int64: int64(e.Rating), Valid: true}, e.Date)
		f.addReleasedFile(t, ep, fmt.Sprintf("released/series-%d/episode-%d.mp4", n, k+1))
	}
	return id
}

// SetReleaseDate sets the release_date of the title id ("" = no date).
func (f *Fixture) SetReleaseDate(t testing.TB, id, date string) {
	t.Helper()
	exec(t, f.DB, `UPDATE items SET release_date = ? WHERE id = ?`, sql.NullString{String: date, Valid: date != ""}, id)
}

func (f *Fixture) addReleasedItem(t testing.TB, id, kind, parentID string, rating sql.NullInt64, date string) {
	t.Helper()
	title := "Released " + kind + " " + id[len(id)-4:]
	exec(t, f.DB, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, release_date, genres, own_rating,
	               metadata_state, added_at, created_at, updated_at)
	               VALUES (?, ?, ?, ?, ?, ?, ?, '[]', ?, 'matched', 1, 1, 1)`,
		id, kind, sql.NullString{String: parentID, Valid: parentID != ""}, f.libraryID, title, title,
		sql.NullString{String: date, Valid: date != ""}, rating)
}

func (f *Fixture) addReleasedFile(t testing.TB, itemID, rel string) {
	t.Helper()
	exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec, audio_codec,
	               width, height, duration_ms, created_at, updated_at)
	               VALUES (?, ?, ?, ?, 1, 1, 'mp4', 'h264', 'aac', 1920, 1080, 1000, 1, 1)`,
		"01J9RELFILE"+itemID[len(itemID)-15:], itemID, f.rootID, rel)
}
