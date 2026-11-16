package admin

import (
	"errors"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// saveLibrary, the admin half (concept 24 4.3): name, default rating and trickplay are checked
// like updateLibrary and written in the transaction package library opens for the folders, so a
// failure of the folders takes them back. The folder rules are tested in package library.
func TestSaveLibrarySettings(t *testing.T) {
	a := newAudioFixture(t)
	libs := a.svc.libraries.(*dbLibraries)
	libs.saveDB = a.f.DB.Write
	au := a.f.Audio
	name := func(id int64) string {
		t.Helper()
		var n string
		if err := a.f.DB.Read.QueryRow(`SELECT name FROM libraries WHERE id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The new default reaches the titles rated by the library (AN-2), the name is trimmed.
	twelve := 12
	if _, err := a.svc.SaveLibrary(a.ctx, au.AudiobookLibraryID, model.LibrarySave{Revision: "r", Name: " Hörspiele ", DefaultRating: &twelve}); err != nil {
		t.Fatal(err)
	}
	if got := name(au.AudiobookLibraryID); got != "Hörspiele" {
		t.Fatalf("name = %q", got)
	}
	if own, eff, src := a.rating(t, au.Books[0].ID); own.Int64 != 12 || eff.Int64 != 12 || src.String != "library" {
		t.Fatalf("rating after the save: %v %v %v", own, eff, src)
	}
	if len(libs.saved) != 1 || libs.saved[0].Revision != "r" {
		t.Fatalf("library got %+v", libs.saved)
	}

	// The folders fail after the settings ran: name and ratings stay as they were.
	libs.saveErr = model.ErrScanRunning
	sixteen := 16
	if _, err := a.svc.SaveLibrary(a.ctx, au.AudiobookLibraryID, model.LibrarySave{Revision: "r", Name: "Anders", DefaultRating: &sixteen}); !errors.Is(err, model.ErrScanRunning) {
		t.Fatalf("err = %v, want ErrScanRunning", err)
	}
	if got := name(au.AudiobookLibraryID); got != "Hörspiele" {
		t.Fatalf("name after the failed save = %q", got)
	}
	if own, _, _ := a.rating(t, au.Books[0].ID); own.Int64 != 12 {
		t.Fatalf("rating after the failed save: %v", own)
	}
	libs.saveErr = nil

	// What is refused here never reaches package library.
	video := a.f.LibraryID()
	trick := model.DefaultLibraryTrickplay()
	for _, c := range []struct {
		id    int64
		req   model.LibrarySave
		field string
		code  string
	}{
		{video, model.LibrarySave{Name: "Filme", DefaultRating: &twelve}, "defaultRating", "not_allowed"},
		{au.AudiobookLibraryID, model.LibrarySave{Name: "Hörspiele", Trickplay: &trick}, "trickplay", "not_allowed"},
		{au.AudiobookLibraryID, model.LibrarySave{Name: "  "}, "name", "required"},
		{au.AudiobookLibraryID, model.LibrarySave{Name: "H", Roots: []model.LibrarySaveRoot{{LibraryRootRequest: model.LibraryRootRequest{Content: model.RootAudiobook}}}}, "roots[0].path", "required"},
		{au.AudiobookLibraryID, model.LibrarySave{Name: "H", Remove: []model.LibrarySaveRemoval{{ID: 1, ConfirmPath: "/" + strings.Repeat("x", maxLibraryPath)}}}, "removeRoots[0].confirmPath", "too_long"},
	} {
		_, err := a.svc.SaveLibrary(a.ctx, c.id, c.req)
		var verr *model.ValidationError
		if !errors.As(err, &verr) || !hasField(verr, c.field, c.code) {
			t.Errorf("%s %s: %v", c.field, c.code, err)
		}
	}
	if len(libs.saved) != 2 {
		t.Fatalf("refused requests reached package library: %d calls", len(libs.saved))
	}
	if _, err := a.svc.SaveLibrary(a.ctx, 999, model.LibrarySave{Name: "X"}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown library: %v", err)
	}
}

func hasField(verr *model.ValidationError, field, code string) bool {
	for _, f := range verr.Fields {
		if f.Field == field && f.Code == code {
			return true
		}
	}
	return false
}

// A step after the commit that fails (here the count of the keyframe index, whose INDEXED BY fails
// without the index) is logged: the save answers with the saved library, not with an error the
// form would retry with a revision that is gone.
func TestSaveLibraryAnswersAfterAFailingCount(t *testing.T) {
	a := newAudioFixture(t)
	libs := a.svc.libraries.(*dbLibraries)
	libs.saveDB = a.f.DB.Write
	libs.saveKind = model.LibraryVideo
	if _, err := a.f.DB.Write.Exec(`DROP INDEX media_files_keyframes`); err != nil {
		t.Fatal(err)
	}
	video := a.f.LibraryID()
	saved, err := a.svc.SaveLibrary(a.ctx, video, model.LibrarySave{Revision: "r", Name: "Kino"})
	if err != nil {
		t.Fatalf("save with a failing count: %v", err)
	}
	if saved.Library.ID != video || saved.Library.KeyframeIndex != nil {
		t.Fatalf("answer = %+v, want the saved library without the count", saved.Library)
	}
	var name string
	if err := a.f.DB.Read.QueryRow(`SELECT name FROM libraries WHERE id = ?`, video).Scan(&name); err != nil || name != "Kino" {
		t.Fatalf("name = %q, %v", name, err)
	}
}
