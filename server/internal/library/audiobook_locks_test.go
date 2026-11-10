package library

import (
	"path/filepath"
	"testing"
)

// The scan of a book keeps what the editor locked (concept 19 D5.3, PD-1): the editor locks a text
// per language (title:de, title:en …) and the book fields by column name; older writers locked a
// text without language. A German book has the text rows de and en, an English one only en. The
// files also give publisher, ISBN, ASIN and the original language, and every value from the files
// carries the provenance local.
func TestAudiobookScanKeepsEditorLocks(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	dir := t.TempDir()
	src := filepath.Join(audiobookMedia(t), filepath.FromSlash(m4bFolder), m4bTitle+".m4b")
	for _, folder := range []string{"Autorin/Deutsch", "Author/English"} {
		if err := copyFile(src, filepath.Join(dir, filepath.FromSlash(folder), "buch.m4b")); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, dir, "Autorin/Deutsch/metadata.json", `{"title": "Deutsches Buch", "subtitle": "Teil eins",
		"authors": ["Datei Autorin"], "narrators": ["Datei Sprecher"], "series": [{"name": "Datei Reihe", "sequence": "2"}],
		"description": "Datei Text", "language": "German", "publisher": "Datei Verlag", "isbn": "9783000000000",
		"asin": "B0DEUTSCH1"}`)
	writeFile(t, dir, "Author/English/metadata.json", `{"title": "English Book", "description": "File text",
		"language": "en", "asin": "B0ENGLISH1"}`)
	lib := e.createAudiobooks(dir)
	e.scan(lib.ID)
	e.probeQueued()
	de := e.str(`SELECT id FROM items WHERE kind = 'audiobook' AND title = 'Deutsches Buch'`)
	en := e.str(`SELECT id FROM items WHERE kind = 'audiobook' AND title = 'English Book'`)

	for _, c := range []struct{ q, id, want string }{
		{`SELECT publisher || '|' || isbn || '|' || asin || '|' || original_language FROM items WHERE id = ?`, de,
			"Datei Verlag|9783000000000|B0DEUTSCH1|de"},
		{`SELECT coalesce(publisher, '-') || '|' || asin || '|' || original_language FROM items WHERE id = ?`, en, "-|B0ENGLISH1|en"},
		{`SELECT group_concat(lang, ',') FROM (SELECT lang FROM item_texts WHERE item_id = ? ORDER BY lang)`, de, "de,en"},
		{`SELECT group_concat(lang, ',') FROM (SELECT lang FROM item_texts WHERE item_id = ? ORDER BY lang)`, en, "en"},
		{`SELECT group_concat(field || ':' || lang || '=' || provider, ',') FROM (SELECT * FROM item_field_sources
		   WHERE item_id = ? AND field IN ('author', 'asin', 'original_language', 'title', 'series_index') ORDER BY field, lang)`, de,
			"asin:=local,author:=local,original_language:=local,series_index:=local,title:de=local,title:en=local"},
	} {
		if got := e.str(c.q, c.id); got != c.want {
			t.Errorf("%s\n  = %q, want %q", c.q, got, c.want)
		}
	}

	// What the editor writes: both text rows of each book with their per-language locks, the book
	// fields with their column locks, and an older lock without language on the subtitle.
	edit := []struct{ q, id string }{
		{`UPDATE items SET author = 'Hand Autorin', narrator = 'Hand Sprecher', series_name = 'Hand Reihe', series_index = 7,
		  publisher = 'Hand Verlag', isbn = '9780000000002', asin = 'B0HANDDE01', locked_fields =
		  '["title:de","title:en","overview:de","overview:en","sort_title:de","tagline","author","narrator","series_name","series_index","publisher","isbn","asin"]'
		  WHERE id = ?`, de},
		{`UPDATE item_texts SET title = 'Hand Titel', overview = 'Hand Text', tagline = 'Hand Untertitel', sort_key = 'zzz hand' WHERE item_id = ?`, de},
		{`UPDATE items SET asin = 'B0HANDEN01', locked_fields = '["title:en","overview:en","asin"]' WHERE id = ?`, en},
		{`UPDATE item_texts SET title = 'Hand Title', overview = 'Hand text' WHERE item_id = ?`, en},
		{`DELETE FROM item_field_sources WHERE item_id = ?`, de},
	}
	for _, s := range edit {
		if _, err := e.db.Write.ExecContext(e.ctx, s.q, s.id); err != nil {
			t.Fatal(err)
		}
	}
	e.reprobe()

	for _, c := range []struct{ q, id, want string }{
		{`SELECT title || '|' || author || '|' || narrator || '|' || series_name || '|' || series_index || '|' || publisher || '|' || isbn || '|' || asin
		   FROM items WHERE id = ?`, de, "Deutsches Buch|Hand Autorin|Hand Sprecher|Hand Reihe|7.0|Hand Verlag|9780000000002|B0HANDDE01"},
		{`SELECT group_concat(lang || '=' || title || '/' || overview || '/' || tagline, ',') FROM (SELECT * FROM item_texts WHERE item_id = ? ORDER BY lang)`, de,
			"de=Hand Titel/Hand Text/Hand Untertitel,en=Hand Titel/Hand Text/Hand Untertitel"},
		{`SELECT sort_key FROM item_texts WHERE item_id = ? AND lang = 'de'`, de, "zzz hand"},
		{`SELECT title || '|' || asin FROM items WHERE id = ?`, en, "English Book|B0HANDEN01"},
		{`SELECT group_concat(lang || '=' || title || '/' || overview, ',') FROM item_texts WHERE item_id = ?`, en, "en=Hand Title/Hand text"},
		// A locked field gets no provenance from the files; a free one does.
		{`SELECT group_concat(field || ':' || lang, ',') FROM (SELECT * FROM item_field_sources WHERE item_id = ?
		   AND field IN ('author', 'asin', 'title', 'overview', 'tagline', 'original_language') ORDER BY field, lang)`, de, "original_language:"},
	} {
		if got := e.str(c.q, c.id); got != c.want {
			t.Errorf("%s\n  = %q, want %q", c.q, got, c.want)
		}
	}

	// A locked original language among the text languages decides the row the scan writes: the
	// German book set to English keeps its German row as it is and writes only en.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET original_language = 'en', locked_fields = '["original_language"]' WHERE id = ?`, de); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE item_texts SET overview = 'Alt' WHERE item_id = ?`, de); err != nil {
		t.Fatal(err)
	}
	e.reprobe()
	if got := e.str(`SELECT group_concat(lang || '=' || overview, ',') FROM (SELECT * FROM item_texts WHERE item_id = ? ORDER BY lang)`, de); got != "de=Alt,en=Datei Text" {
		t.Errorf("texts with the original language locked to en: %q", got)
	}
	if got := e.str(`SELECT original_language || '|' || author FROM items WHERE id = ?`, de); got != "en|Datei Autorin" {
		t.Errorf("after the locks went: %q", got)
	}
}

// reprobe marks every job done and probes every media file again, as a refresh of the books does.
func (e *env) reprobe() {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done'`); err != nil {
		e.t.Fatal(err)
	}
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT id FROM media_files ORDER BY id`)
	if err != nil {
		e.t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			e.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		e.t.Fatal(err)
	}
	for _, id := range ids {
		if err := e.svc.handleProbe(e.ctx, mediaJob(e.t, JobProbeFile, id)); err != nil {
			e.t.Fatal(err)
		}
	}
}
