package library

import (
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// A folder's own text languages and images (concept 24 3.1 and 3.6): only a video folder has them,
// the check of WithRootMetadataCheck names its fields below the folder, a change has three states
// per field, and the folder counts the titles whose home folder it is.
func TestRootMetadataOnCreateAndUpdate(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	// The check refuses "xx"; package metadata's ValidateRootMetadata is wired in main.
	e.svc = New(e.db, jobs.NewQueue(e.db.Write, e.clock.Now), transcode.Runner{FFprobePath: ffprobePath()}, discard, WithClock(e.clock.Now),
		WithRootMetadataCheck(func(m model.RootMetadata) *model.ValidationError {
			var verr *model.ValidationError
			if m.TextLangs != nil && slices.Contains(*m.TextLangs, "xx") {
				verr = verr.Add("textLangs", "lang_unknown")
			}
			return verr
		}))
	dir := tree(t, []string{alienFilm, "Heat (1995)/Heat (1995).mkv"})
	ja := []string{"ja", "en"}
	bad := []string{"xx"}
	req := testRequest("Filme", dir)
	req.Roots[0].Metadata.TextLangs = &bad
	if _, err := e.svc.Create(e.ctx, req); !hasFieldError(err, "roots[0].textLangs", "lang_unknown") {
		t.Fatalf("unknown language: %v", err)
	}
	req.Roots[0].Metadata.TextLangs = &ja
	lib, err := e.svc.Create(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	got, err := e.svc.Get(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := got.Roots[0]
	if r.Metadata.TextLangs == nil || !slices.Equal(*r.Metadata.TextLangs, ja) || r.Metadata.Images != nil || r.HomeTitleCount != 2 {
		t.Fatalf("root = %+v %+v, want ja,en, images like the default and two home titles", r.Metadata, r.HomeTitleCount)
	}
	// Own images with no kind on; the text languages stay when the change leaves them out.
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, r.ID, model.LibraryRootUpdate{SetImages: true, Images: &model.RootImages{}}); err != nil {
		t.Fatal(err)
	}
	got, _ = e.svc.Get(e.ctx, lib.ID)
	if m := got.Roots[0].Metadata; m.TextLangs == nil || m.Images == nil || len(m.Images.Kinds) != 0 {
		t.Fatalf("after the images: %+v", m)
	}
	// null for the languages: like the default again, the images stay.
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, r.ID, model.LibraryRootUpdate{SetTextLangs: true}); err != nil {
		t.Fatal(err)
	}
	got, _ = e.svc.Get(e.ctx, lib.ID)
	if m := got.Roots[0].Metadata; m.TextLangs != nil || m.Images == nil {
		t.Fatalf("after textLangs null: %+v", m)
	}
	if _, err := e.svc.UpdateRoot(e.ctx, lib.ID, r.ID, model.LibraryRootUpdate{SetTextLangs: true, TextLangs: &bad}); !hasFieldError(err, "textLangs", "lang_unknown") {
		t.Fatalf("unknown language on a change: %v", err)
	}
	// An audiobook folder has none (concept 24 3.1).
	books := model.LibraryRequest{Name: "Hörbücher", Kind: model.LibraryAudiobooks, Roots: rootReq(t.TempDir(), model.RootAudiobook)}
	books.Roots[0].Metadata.Images = &model.RootImages{}
	if _, err := e.svc.Create(e.ctx, books); !hasFieldError(err, "roots[0].images", "not_allowed") {
		t.Fatalf("images on an audiobook folder: %v", err)
	}
}
