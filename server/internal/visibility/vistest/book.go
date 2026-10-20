package vistest

import (
	"database/sql"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Helpers for the audiobook playback tests (M6 stream A): they write title rows, which only the
// packages the itemsaccess analyzer allows may do.

// BookPart is a part of a test audiobook: its media file id, its path below MediaDir (the file is
// not written), container, audio codec, size, duration and stored probe (JSON, "" = not probed).
type BookPart struct {
	ID, RelPath, Container, AudioCodec, Probe string
	Size, DurationMs                          int64
}

// AddBookWithParts inserts an audiobook rated 0 (rating source library) into library 1 with its
// parts (part_index 1..n for more than one part, NULL for one) and its book-wide chapters.
func (f *Fixture) AddBookWithParts(t testing.TB, id string, parts []BookPart, chapters []model.Chapter) {
	t.Helper()
	var total int64
	for _, p := range parts {
		total += p.DurationMs
	}
	exec(t, f.DB, `INSERT INTO items (id, kind, library_id, title, sort_title, own_rating, rating_source, author, duration_ms,
	               metadata_state, added_at, created_at, updated_at)
	               VALUES (?, 'audiobook', ?, 'Buch', 'buch', 0, 'library', 'Autorin', ?, 'matched', 1, 1, 1)`, id, f.libraryID, total)
	for i, p := range parts {
		index := sql.NullInt64{Int64: int64(i + 1), Valid: len(parts) > 1}
		exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, audio_codec, duration_ms,
		               probe, probed_at, part_index, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, 1, ?, 1, 1)`,
			p.ID, id, f.rootID, p.RelPath, p.Size, p.Container, p.AudioCodec, p.DurationMs,
			sql.NullString{String: p.Probe, Valid: p.Probe != ""}, index)
	}
	f.SetBookChapters(t, id, chapters)
}

// SetBookChapters replaces the chapters of an audiobook.
func (f *Fixture) SetBookChapters(t testing.TB, itemID string, chapters []model.Chapter) {
	t.Helper()
	exec(t, f.DB, `DELETE FROM chapters WHERE item_id = ?`, itemID)
	for i, c := range chapters {
		exec(t, f.DB, `INSERT INTO chapters (item_id, idx, title, start_ms, end_ms, media_file_id, file_start_ms)
		               VALUES (?, ?, ?, ?, ?, NULL, 0)`, itemID, i, c.Title, c.StartMs, c.EndMs)
	}
}
