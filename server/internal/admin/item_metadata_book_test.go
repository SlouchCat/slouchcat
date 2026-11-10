package admin

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// A text edit of a book goes into every text row it has (de and en in the fixture), the genres
// into both languages, and the editor shows the book in the language of its texts (concept 19
// D5.3, PD-1).
func TestBookEditWritesEveryTextRow(t *testing.T) {
	t.Parallel()
	m := newMaintFixture(t)
	m.f.AddAudio(t)
	book := m.f.Audio.Books[0].ID
	title, overview, genres := "Neuer Titel", "Neu", []string{"Fantasy"}
	if _, err := m.svc.UpdateItemMetadata(m.ctx, book, model.ItemMetadataEdit{Title: &title, Overview: &overview, Genres: &genres}); err != nil {
		t.Fatal(err)
	}
	rows, err := m.f.DB.Read.QueryContext(m.ctx, `SELECT lang || '=' || title || '/' || overview FROM item_texts WHERE item_id = ? ORDER BY lang`, book)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "de=Neuer Titel/Neu" || got[1] != "en=Neuer Titel/Neu" {
		t.Errorf("text rows %v, want de and en edited", got)
	}
	var stored, locks string
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT genres, locked_fields FROM items WHERE id = ?`, book).Scan(&stored, &locks); err != nil {
		t.Fatal(err)
	}
	if stored != `{"de":["Fantasy"],"en":["Fantasy"]}` || locks != `["title:de","title:en","overview:de","overview:en","genres"]` {
		t.Errorf("genres %s, locks %s", stored, locks)
	}
	it, err := m.svc.GetAdminItem(m.ctx, book)
	if err != nil || it.Metadata == nil || it.Metadata.Lang != "de" || it.Metadata.Title != title {
		t.Fatalf("metadata %+v (%v)", it.Metadata, err)
	}

	// An English book has only its en row: the editor shows it in en and writes only that row; a
	// locked original language de decides the row it shows.
	english := m.f.Audio.Books[1].ID
	m.exec(t, `DELETE FROM item_texts WHERE item_id = ? AND lang = 'de'`, english)
	if _, err := m.svc.UpdateItemMetadata(m.ctx, english, model.ItemMetadataEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	var langs string
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT group_concat(lang || '=' || title, ',') FROM item_texts WHERE item_id = ?`, english).Scan(&langs); err != nil ||
		langs != "en=Neuer Titel" {
		t.Errorf("text rows of the English book %q (%v)", langs, err)
	}
	m.exec(t, `UPDATE items SET original_language = 'de', locked_fields = '["original_language"]' WHERE id = ?`, english)
	if it, err := m.svc.GetAdminItem(m.ctx, english); err != nil || it.Metadata == nil || it.Metadata.Lang != "de" {
		t.Errorf("language with a locked original language de: %+v (%v)", it.Metadata, err)
	}
}
