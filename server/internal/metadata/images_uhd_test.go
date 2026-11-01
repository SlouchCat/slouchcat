package metadata

import (
	"context"
	"slices"
	"testing"
)

// The backdrop size for 4K and high-density screens: only from an original of at least 2560 px,
// never scaled up.
func TestSizesFor(t *testing.T) {
	t.Parallel()
	names := func(specs []sizeSpec) []string {
		var out []string
		for _, s := range specs {
			out = append(out, s.name)
		}
		return out
	}
	if got := names(sizesFor("backdrop", 1920)); !slices.Equal(got, []string{"w780", "w1280", "w1920"}) {
		t.Errorf("1920 px original: %v", got)
	}
	if got := sizesFor("backdrop", 3000); got[3] != (sizeSpec{name: "w3840", width: 3000}) {
		t.Errorf("3000 px original: %v, want w3840 at 3000 px", got)
	}
	if got := sizesFor("backdrop", 5120); got[3] != (sizeSpec{name: "w3840", width: 3840}) {
		t.Errorf("5120 px original: %v, want w3840 at 3840 px", got)
	}
	if got := names(sizesFor("poster", 6000)); !slices.Equal(got, []string{"w200", "w400", "w800"}) {
		t.Errorf("poster: %v, the 4K size is for backdrops only", got)
	}
	if got := names(sizesFor("backdrop", 0)); len(got) != 3 {
		t.Errorf("unknown width: %v, want no 4K size", got)
	}
}

// image.fetch derives w3840 from a wide TMDB backdrop and leaves it out for a narrow one; a stored
// kind without it counts as incomplete once the original is wide enough.
func TestBackdropUhdFromWideOriginal(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		width, want int
	}{
		"4000 px scaled to 3840": {4000, 3840},
		"2600 px kept":           {2600, 2600},
		"2000 px without 4K":     {2000, 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, goodToken)
			e.tmdb.setImages("550", nil, []map[string]any{tmdbImage("/wide.jpg", nil, tc.width, tc.width*9/16, 5, 5)}, nil)
			e.tmdb.serveImage("/t/p/original/wide.jpg", jpegBytes(t, tc.width, tc.width*9/16), "image/jpeg")
			e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
			if err := e.svc.match(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			imgs := e.images(t, idByTMDB)
			jpg, webp := imgs["backdrop/w3840/jpeg"], imgs["backdrop/w3840/webp"]
			if tc.want == 0 {
				if jpg.RelPath != "" || webp.RelPath != "" {
					t.Fatalf("w3840 made from a %d px original", tc.width)
				}
			} else if jpg.Width != int64(tc.want) || webp.RelPath == "" {
				t.Fatalf("w3840 jpeg %+v webp %+v, want width %d", jpg, webp, tc.want)
			}
			for _, key := range []string{"backdrop/w780/jpeg", "backdrop/w1280/webp", "backdrop/w1920/jpeg"} {
				if imgs[key].RelPath == "" {
					t.Errorf("%s missing", key)
				}
			}
			// A second run finds the kind complete and downloads nothing.
			before := e.tmdb.count("/t/p/original/wide.jpg")
			if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
				t.Fatal(err)
			}
			if after := e.tmdb.count("/t/p/original/wide.jpg"); after != before {
				t.Errorf("second run downloaded again (%d → %d)", before, after)
			}
			// A title stored before the 4K size (rows without w3840) is incomplete for the check
			// after a scan once its original is wide enough, so the next scan queues image.fetch.
			mustExec(t, e.db.Write, `DELETE FROM images WHERE item_id = ? AND size = 'w3840'`, idByTMDB)
			mustExec(t, e.db.Write, `DELETE FROM jobs`)
			n, err := e.svc.QueueIncompleteImages(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			if want := map[bool]int{true: 1, false: 0}[tc.want > 0]; n != want {
				t.Errorf("image checks queued %d, want %d", n, want)
			}
		})
	}
}
