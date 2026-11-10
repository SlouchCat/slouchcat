package admin

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func (m maintFixture) seriesOf(t *testing.T, itemID string) string {
	t.Helper()
	var s sql.NullString
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT audio_series_id FROM items WHERE id = ?`, itemID).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s.String
}

// The editor moves a book to the series of its new key when it changes series name or author
// (concept 19 D4.4); PD-8: «Reihe bearbeiten» with «In alle Bände schreiben» merges two series into
// the one that held the key, which keeps its id and gets the locked name, and the other row goes at
// once; a rename of every volume keeps the id; only the volumes the profile may see are written.
func TestUpdateAudioSeries(t *testing.T) {
	t.Parallel()
	m := newMaintFixture(t)
	m.f.AddAudio(t)
	b := m.f.Audio.Books // rated 0, 6, 12, 16, 18 and unrated
	m.exec(t, `UPDATE items SET author = 'Autorin' WHERE kind = 'audiobook'`)
	name := func(n string) *string { return &n }
	for i, series := range []string{"Saga Eins", "Saga Eins - Band", "Saga Zwei", "Saga Zwei"} {
		if _, err := m.svc.UpdateItemMetadata(m.ctx, b[i].ID, model.ItemMetadataEdit{SeriesName: name(series)}); err != nil {
			t.Fatal(err)
		}
	}
	one, two := m.seriesOf(t, b[0].ID), m.seriesOf(t, b[2].ID)
	if one == "" || one != m.seriesOf(t, b[1].ID) || two == "" || two == one || two != m.seriesOf(t, b[3].ID) {
		t.Fatalf("series after the editor: %q %q %q %q", one, m.seriesOf(t, b[1].ID), two, m.seriesOf(t, b[3].ID))
	}
	// Another author moves a book out, the old author back in.
	if _, err := m.svc.UpdateItemMetadata(m.ctx, b[1].ID, model.ItemMetadataEdit{Author: name("Jemand")}); err != nil {
		t.Fatal(err)
	}
	if s := m.seriesOf(t, b[1].ID); s == one || s == "" {
		t.Errorf("book with another author still in %q", s)
	}
	if _, err := m.svc.UpdateItemMetadata(m.ctx, b[1].ID, model.ItemMetadataEdit{Author: name("Autorin")}); err != nil {
		t.Fatal(err)
	}
	if s := m.seriesOf(t, b[1].ID); s != one {
		t.Errorf("book back with its author in %q, want %q", s, one)
	}

	// Validation and unknown ids.
	for in, code := range map[string]string{" ": "required", strings.Repeat("ä", 201): "too_long"} {
		var verr *model.ValidationError
		if _, err := m.svc.UpdateAudioSeries(m.ctx, m.f.P18.ID, one, in, false); !errors.As(err, &verr) || len(verr.Fields) != 1 ||
			verr.Fields[0] != (model.FieldError{Field: "name", Code: code}) {
			t.Errorf("name %.10q: %v, want name %s", in, err, code)
		}
	}
	if _, err := m.svc.UpdateAudioSeries(m.ctx, m.f.P18.ID, "01J9000000000000000000NONE", "X", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown series: %v", err)
	}

	// Only the name: locked, the volumes unchanged.
	got, err := m.svc.UpdateAudioSeries(m.ctx, m.f.P18.ID, two, " Zweite Saga ", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != two || got.Name != "Zweite Saga" || !got.NameLocked || got.VolumeCount != 2 {
		t.Errorf("rename = %+v", got)
	}
	var series string
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT series_name FROM items WHERE id = ?`, b[2].ID).Scan(&series); err != nil || series != "Saga Zwei" {
		t.Errorf("volume series name %q (%v), want unchanged", series, err)
	}

	// Merge: the volumes of two take the name of one; one keeps its id, two is gone.
	got, err = m.svc.UpdateAudioSeries(m.ctx, m.f.P18.ID, two, "Saga Eins", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != one || got.Name != "Saga Eins" || !got.NameLocked || got.VolumeCount != 4 {
		t.Errorf("merge = %+v, want series %s with 4 volumes", got, one)
	}
	for _, it := range b[:4] {
		if s := m.seriesOf(t, it.ID); s != one {
			t.Errorf("volume %s in %q after the merge, want %q", it.ID, s, one)
		}
	}
	var rows int
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT count(*) FROM audio_series WHERE id = ?`, two).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("merged series still has %d rows (%v)", rows, err)
	}
	var locks string
	if err := m.f.DB.Read.QueryRowContext(m.ctx, `SELECT locked_fields FROM items WHERE id = ?`, b[3].ID).Scan(&locks); err != nil ||
		!strings.Contains(locks, `"series_name"`) {
		t.Errorf("locks of a written volume %s (%v), want series_name", locks, err)
	}

	// A rename of every volume keeps the id (D4.4).
	if got, err = m.svc.UpdateAudioSeries(m.ctx, m.f.P18.ID, one, "Saga Neu", true); err != nil || got.ID != one {
		t.Errorf("rename of every volume = %+v (%v), want id %s", got, err, one)
	}

	// P6 sees the volumes rated 0 and 6 only: those two move into a new series, the others stay.
	got, err = m.svc.UpdateAudioSeries(m.ctx, m.f.P6.ID, one, "Saga Kind", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == one || got.VolumeCount != 2 {
		t.Errorf("P6 rename = %+v, want a new series of 2", got)
	}
	moved := []string{m.seriesOf(t, b[0].ID), m.seriesOf(t, b[1].ID)}
	stayed := []string{m.seriesOf(t, b[2].ID), m.seriesOf(t, b[3].ID)}
	if !slices.Equal(moved, []string{got.ID, got.ID}) || !slices.Equal(stayed, []string{one, one}) {
		t.Errorf("after the P6 rename: moved %v, stayed %v", moved, stayed)
	}
}
