package metadata

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// A genre backdrop is fetched once in w780 and served from the cache afterwards; a path that is no
// plain TMDB image path is refused without a download.
func TestOpenGenreBackdrop(t *testing.T) {
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.serveImage("/t/p/w780/genre28.jpg", e.tmdb.poster, "image/jpeg")
	for range 2 {
		src, err := e.svc.OpenGenreBackdrop(ctx, "/genre28.jpg")
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(src.File)
		_ = src.File.Close()
		if err != nil || len(data) == 0 || src.MediaType != "image/jpeg" || src.ETag == "" {
			t.Errorf("backdrop = %s, %d bytes, %v", src.MediaType, len(data), err)
		}
	}
	if n := e.tmdb.count("/t/p/w780/genre28.jpg"); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}
	for _, bad := range []string{"", "genre28.jpg", "/../x.jpg", "/a/b.jpg", "/x.jpg?y"} {
		if _, err := e.svc.OpenGenreBackdrop(ctx, bad); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%q = %v, want ErrNotFound", bad, err)
		}
	}
}
