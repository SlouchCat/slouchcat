package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Concept 15 7.5: lifting a text field whose lock is the old one without language frees only the
// server's metadata language (de in the fixture); the other languages with a stored text (en)
// keep the field locked under their own name, so the providers still leave them alone. Another
// old lock stays as it is.
func TestItemMetadataUnlockOldLock(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	pf.f.SetLockedFields(t, fixtureFilm6, `["title","overview","year"]`)
	rec := send(t, pf.h, http.MethodPatch, BasePath+"/admin/items/"+fixtureFilm6+"/metadata", "p18", `{"unlock":["title"]}`, nil)
	wantStatus(t, rec, http.StatusOK)
	var it gen.AdminItem
	decode(t, rec, &it)
	if got, want := it.LockedFields, []string{"overview", "year", "title:en"}; !slices.Equal(got, want) {
		t.Errorf("lockedFields = %v, want %v", got, want)
	}
	var locked []string
	for _, f := range it.Metadata.Locked {
		locked = append(locked, string(f))
	}
	if want := []string{"year", "overview"}; !slices.Equal(locked, want) {
		t.Errorf("locked in de = %v, want %v", locked, want)
	}
}
