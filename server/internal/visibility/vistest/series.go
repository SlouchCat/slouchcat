package vistest

import "testing"

// Fixed ids of AddAudioSeries (concept 19 stage D2).
const (
	// AudioSeriesID is the series «Reihe» of the books rated 0 (volume 1), 12 (volume 2, spelled
	// «Reihe - Band») and 18 (volume 0.5): P12 sees two volumes, P6 one and so no series (PD-4).
	AudioSeriesID   = "01J9000000000000000000SER1"
	AudioSeriesName = "Reihe"
	// AudioCollectionID is the manual collection «Hörspiele» of the book rated 0 and the
	// book of three parts: all its members are audiobooks (D4.1, PD-6).
	AudioCollectionID   = "01J9000000000000000000CMAB"
	AudioCollectionName = "Hörspiele"
)

// AddAudioSeries adds AudioSeriesID and AudioCollectionID to the books of AddAudio (which it
// needs); the three volumes get the author «Autorin». The series row and the links are written
// directly, with the key library.RelinkAudioSeries computes for them, so the visibility tests do
// not depend on the scanner and a relink keeps them in place.
func (f *Fixture) AddAudioSeries(t testing.TB) {
	t.Helper()
	if len(f.Audio.Books) == 0 {
		t.Fatal("vistest: AddAudioSeries needs AddAudio")
	}
	a := f.Audio
	exec(t, f.DB, `INSERT INTO audio_series (id, library_id, series_key, name, name_locked, created_at, updated_at)
	               VALUES (?, ?, 'reihe' || char(31) || 'autorin', ?, 0, 1, 1)`, AudioSeriesID, a.AudiobookLibraryID, AudioSeriesName)
	for _, v := range []struct {
		book     Movie
		name     string
		index    float64
		narrator string
	}{{a.Books[0], "Reihe", 1, "Sprecher"}, {a.Books[2], "Reihe - Band", 2, "Sprecher"}, {a.Books[4], "Reihe", 0.5, "Sprecherin"}} {
		exec(t, f.DB, `UPDATE items SET series_name = ?, series_index = ?, narrator = ?, author = 'Autorin', audio_series_id = ? WHERE id = ?`,
			v.name, v.index, v.narrator, AudioSeriesID, v.book.ID)
	}
	exec(t, f.DB, `INSERT INTO collections (id, source, name, created_at, updated_at) VALUES (?, 'manual', ?, 1, 1)`,
		AudioCollectionID, AudioCollectionName)
	exec(t, f.DB, `INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 0, 1), (?, ?, 1, 1)`,
		AudioCollectionID, a.Books[0].ID, AudioCollectionID, a.Parts.ID)
}
