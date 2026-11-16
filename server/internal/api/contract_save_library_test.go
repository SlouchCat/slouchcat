package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// saveLibrary against the services of packages admin and library as the server wires them
// (concept 24 4.3): getLibrary hands out the revision, a save with it writes the library settings
// and the folders in one transaction, the old revision is stale afterwards, and a forgotten folder
// is refused.
func TestContractSaveLibrary(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.New(f.DB, jobs.NewQueue(f.DB.Write, nil), transcode.Runner{}, log)
	o := pf.opts
	o.Admin = admin.New(f.DB, pf.a, lib, ring, &authTestTMDB{}, func(string) string { return "" }, log)
	h := NewHandler(o)
	path := fmt.Sprintf("%s/admin/libraries/%d", BasePath, f.Audio.AudiobookLibraryID)

	get := func() gen.Library {
		t.Helper()
		rec := send(t, h, http.MethodGet, path, "p18", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("getLibrary: %d %s", rec.Code, rec.Body.String())
		}
		var out gen.Library
		decode(t, rec, &out)
		if out.Revision == nil || *out.Revision == "" {
			t.Fatalf("getLibrary without revision: %s", rec.Body.String())
		}
		return out
	}
	form := func(l gen.Library, name string, rating int) string {
		t.Helper()
		roots := make([]map[string]any, 0, len(l.Roots))
		for _, r := range l.Roots {
			roots = append(roots, map[string]any{"id": r.Id, "path": r.Path, "content": r.Content, "providers": r.Providers})
		}
		raw, err := json.Marshal(map[string]any{"revision": *l.Revision, "name": name, "defaultRating": rating, "roots": roots, "removeRoots": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	before := get()
	rec := send(t, h, http.MethodPut, path, "p18", form(before, "Hörspiele", 12), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("saveLibrary: %d %s", rec.Code, rec.Body.String())
	}
	var saved gen.LibrarySaved
	decode(t, rec, &saved)
	if saved.Library.Name != "Hörspiele" || saved.Library.DefaultRating == nil || *saved.Library.DefaultRating != 12 ||
		saved.MetadataChanged == nil || len(saved.MetadataChanged) != 0 || *saved.Library.Revision == *before.Revision {
		t.Fatalf("saved = %s", rec.Body.String())
	}
	if after := get(); *after.Revision != *saved.Library.Revision {
		t.Fatalf("getLibrary revision %s, saveLibrary answered %s", *after.Revision, *saved.Library.Revision)
	}

	// The form loaded before the save is stale now.
	rec = send(t, h, http.MethodPut, path, "p18", form(before, "Anders", 12), nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "stale" {
		t.Fatalf("stale form: %d %s", rec.Code, rec.Body.String())
	}
	// A form that forgets a folder removes nothing.
	current := get()
	if len(current.Roots) != 1 {
		t.Fatalf("the audiobook library of the fixture has %d folders, want 1", len(current.Roots))
	}
	current.Roots = nil
	rec = send(t, h, http.MethodPut, path, "p18", form(current, "Anders", 12), nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "root_unlisted" {
		t.Fatalf("forgotten folder: %d %s", rec.Code, rec.Body.String())
	}
	if l := get(); l.Name != "Hörspiele" {
		t.Fatalf("name after the refused saves = %s", l.Name)
	}
}
