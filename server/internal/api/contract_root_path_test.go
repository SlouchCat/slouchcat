package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Contract 1.31.0 (concept 20, P-10): an AdminMediaFile names the folder of its library root, so
// the page shows the whole path in the container.
func TestAdminMediaFileRootPath(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var item gen.AdminItem
	decode(t, rec, &item)
	if len(item.MediaFiles) == 0 {
		t.Fatal("the film has no media file")
	}
	f := item.MediaFiles[0]
	if f.RootPath == nil || *f.RootPath == "" || strings.HasPrefix(f.Path, *f.RootPath) {
		t.Errorf("media file = rootPath %v, path %q; want the root apart from the relative path", f.RootPath, f.Path)
	}
}
