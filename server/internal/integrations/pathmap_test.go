package integrations

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestMapTakesTheLongestMatchingPrefix(t *testing.T) {
	pairs := []model.PathMapping{
		{From: "/data", To: "/media"},
		{From: "/data/movies", To: "/media/Filme"},
		{From: "/data/tv", To: "/media/Serien"},
	}
	cases := []struct{ reported, want string }{
		// The longer prefix wins over the shorter one, whatever the order in the table.
		{"/data/movies/Dune (2021)", "/media/Filme/Dune (2021)"},
		{"/data/tv/Show/Season 01", "/media/Serien/Show/Season 01"},
		{"/data/other/Thing", "/media/other/Thing"},
		// The prefix itself maps to the target itself.
		{"/data/movies", "/media/Filme"},
		// A whole component has to match: /datafiles is not below /data.
		{"/datafiles/Dune", "/datafiles/Dune"},
		// Without a match the path stays as it is (the default table is empty).
		{"/srv/Filme/Dune", "/srv/Filme/Dune"},
		// A trailing slash and a doubled separator do not change the result.
		{"/data/movies/Dune (2021)/", "/media/Filme/Dune (2021)"},
		{"/data//movies/Dune (2021)", "/media/Filme/Dune (2021)"},
	}
	for _, c := range cases {
		if got := Map(pairs, c.reported); got != c.want {
			t.Errorf("Map(%q) = %q, want %q", c.reported, got, c.want)
		}
	}
}

func TestMapWithoutPairsReturnsTheReportedPath(t *testing.T) {
	if got := Map(nil, "/media/Filme/Dune"); got != "/media/Filme/Dune" {
		t.Errorf("Map without pairs = %q", got)
	}
}

func TestResolveFindsTheRootAndRejectsPathsOutsideEveryRoot(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieLib, movieRoot := e.addLibrary(t, "Filme", "/media/Filme")
	_, tvRoot := e.addLibrary(t, "Serien", "/media/Serien")

	pairs := []model.PathMapping{
		{From: "/data", To: "/media"},
		{From: "/data/movies", To: "/media/Filme"},
	}

	target, err := e.svc.Resolve(ctx, pairs, "/data/movies/Dune (2021)")
	if err != nil {
		t.Fatal(err)
	}
	if target.LibraryID != movieLib || target.RootID != movieRoot {
		t.Errorf("library %d root %d, want %d and %d", target.LibraryID, target.RootID, movieLib, movieRoot)
	}
	if target.Dir != "Dune (2021)" {
		t.Errorf("directory %q, want %q", target.Dir, "Dune (2021)")
	}
	if target.Abs != "/media/Filme/Dune (2021)" {
		t.Errorf("mapped path %q", target.Abs)
	}

	// The root itself is the directory ".".
	root, err := e.svc.Resolve(ctx, pairs, "/data/movies")
	if err != nil {
		t.Fatal(err)
	}
	if root.Dir != "." || root.RootID != movieRoot {
		t.Errorf("root itself: directory %q, root %d", root.Dir, root.RootID)
	}

	// A path of the other root resolves without a mapping.
	series, err := e.svc.Resolve(ctx, pairs, "/media/Serien/Show/Season 01")
	if err != nil {
		t.Fatal(err)
	}
	if series.RootID != tvRoot || series.Dir != "Show/Season 01" {
		t.Errorf("series: root %d, directory %q", series.RootID, series.Dir)
	}

	// Outside every root: 422 with the mapped path for the log (5.6).
	outside, err := e.svc.Resolve(ctx, pairs, "/data/other/Thing")
	if !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("error %v, want ErrOutsideRoots", err)
	}
	if outside.Abs != "/media/other/Thing" {
		t.Errorf("mapped path %q, want the result of the mapping", outside.Abs)
	}
	if outside.RootID != 0 {
		t.Errorf("a root was chosen anyway: %d", outside.RootID)
	}

	// A relative report resolves against nothing.
	if _, err := e.svc.Resolve(ctx, nil, "Dune (2021)"); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("relative path: error %v, want ErrOutsideRoots", err)
	}
	// A report that climbs upwards is cleaned first and then lies outside every root; it never
	// reaches the file system (rule 4: the resolution runs through package paths).
	if _, err := e.svc.Resolve(ctx, pairs, "/data/movies/../../../etc"); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("escaping path: error %v, want ErrOutsideRoots", err)
	}
}

func TestResolvePrefersTheLongestRoot(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_, outer := e.addLibrary(t, "Alles", "/media")
	_, inner := e.addLibrary(t, "Filme", "/media/Filme")

	target, err := e.svc.Resolve(ctx, nil, "/media/Filme/Dune (2021)")
	if err != nil {
		t.Fatal(err)
	}
	if target.RootID != inner {
		t.Errorf("root %d, want the inner root %d (outer is %d)", target.RootID, inner, outer)
	}
	if target.Dir != "Dune (2021)" {
		t.Errorf("directory %q", target.Dir)
	}
}

func TestSetPathMapValidatesThePairs(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := e.svc.SetPathMap(ctx, created.ID, []model.PathMapping{
		{From: "/data/movies/", To: "/media/Filme"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.PathMap) != 1 || updated.PathMap[0].From != "/data/movies" {
		t.Errorf("stored mapping %+v", updated.PathMap)
	}

	if _, err := e.svc.SetPathMap(ctx, created.ID, []model.PathMapping{
		{From: "relative", To: "/media/Filme"},
	}); !isValidation(t, err, "pathMap.0.from") {
		t.Errorf("relative source: %v", err)
	}
	if _, err := e.svc.SetPathMap(ctx, created.ID, []model.PathMapping{
		{From: "/data", To: "relative"},
	}); !isValidation(t, err, "pathMap.0.to") {
		t.Errorf("relative target: %v", err)
	}
	if _, err := e.svc.SetPathMap(ctx, created.ID, []model.PathMapping{
		{From: "/data", To: "/media/A"}, {From: "/data", To: "/media/B"},
	}); !isValidation(t, err, "pathMap.1.from") {
		t.Errorf("duplicate source: %v", err)
	}
	// The mapping survived every rejected attempt.
	again, err := e.svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.PathMap) != 1 || again.PathMap[0].To != "/media/Filme" {
		t.Errorf("mapping after the rejected attempts: %+v", again.PathMap)
	}
}
