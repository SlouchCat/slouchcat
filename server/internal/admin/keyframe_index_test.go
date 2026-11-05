package admin

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Library.KeyframeIndex counts the files worth indexing of a video library by keyframes_source
// (concept 14, 4.1, F-7): cues, mp4 and scan are indexed, NULL pending, none unavailable. A VC-1
// file, an audiobook file, a podcast episode and a missing file are not counted; a second video
// library has its own numbers, a video library without files shows zeros, an audio library has no
// KeyframeIndex at all. Listing and single answers agree.
func TestLibraryKeyframeIndex(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	// The fake library service lists the four libraries; the counts come from the database, which
	// gets the same four.
	f.libs.created = []model.LibraryRequest{
		{Name: "Filme", Kind: model.LibraryVideo},
		{Name: "Hörbücher", Kind: model.LibraryAudiobooks},
		{Name: "Leer", Kind: model.LibraryVideo},
		{Name: "Serien", Kind: model.LibraryVideo},
	}
	exec(t, f.db.Write,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'video', 1, 1), (2, 'Hörbücher', 'audiobooks', 1, 1),
		 (3, 'Leer', 'video', 1, 1), (4, 'Serien', 'video', 1, 1)`,
		`INSERT INTO library_roots (id, library_id, path, fs_type, watch, content, providers, created_at)
		 VALUES (1, 1, '/media/Filme', 'ext4', 'inotify', 'movie', '["tmdb"]', 1), (2, 2, '/media/Bücher', 'ext4', 'inotify', 'audiobook', '["local"]', 1),
		 (4, 4, '/media/Serien', 'ext4', 'inotify', 'series', '["tmdb"]', 1)`)
	files := []struct {
		library int
		kind    string
		codec   string
		source  any // keyframes_source; nil = NULL
		missing bool
	}{
		{1, "movie", "h264", "cues", false},        // indexed
		{1, "movie", "hevc", "mp4", false},         // indexed
		{1, "episode", "av1", "scan", false},       // indexed
		{1, "movie", "vp9", "none", false},         // unavailable
		{1, "movie", "h264", nil, false},           // pending
		{1, "movie", "vc1", nil, false},            // a codec that cannot be remuxed: not counted
		{1, "movie", "h264", "cues", true},         // missing on disk: not counted
		{1, "audiobook", "h264", nil, false},       // an audiobook with a video stream (cover): not counted
		{1, "podcast_episode", "h264", nil, false}, // a video podcast episode: not counted
		{2, "audiobook", "h264", "none", false},    // audio library: no coverage at all
		{4, "episode", "h264", "cues", false},      // second library: indexed
		{4, "episode", "hevc", "scan", false},      // indexed
		{4, "episode", "h264", "none", false},      // unavailable
		{4, "episode", "av1", nil, false},          // pending
		{4, "episode", "av1", nil, false},          // pending
		{4, "episode", "mpeg2video", nil, false},   // not counted
	}
	for i, fl := range files {
		item, file := fmt.Sprintf("01KF0000000000000000ITEM%02d", i), fmt.Sprintf("01KF0000000000000000FILE%02d", i)
		var missing any
		if fl.missing {
			missing = int64(5)
		}
		if _, err := f.db.Write.ExecContext(ctx, `INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, 1, 1)`, item, fl.kind, fl.library, item, item); err != nil {
			t.Fatal(err)
		}
		if _, err := f.db.Write.ExecContext(ctx, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container,
			video_codec, keyframes_source, missing_since, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 1, 'mkv', ?, ?, ?, 1, 1)`,
			file, item, fl.library, file+".mkv", fl.codec, fl.source, missing); err != nil {
			t.Fatal(err)
		}
	}
	want := model.LibraryKeyframeIndex{VideoFiles: 5, Indexed: 3, Pending: 1, Unavailable: 1}
	series := model.LibraryKeyframeIndex{VideoFiles: 5, Indexed: 2, Pending: 2, Unavailable: 1}

	libs, err := f.svc.ListLibraries(ctx)
	if err != nil || len(libs) != 4 {
		t.Fatalf("list = %+v, %v", libs, err)
	}
	if k := libs[0].KeyframeIndex; k == nil || *k != want {
		t.Errorf("Filme: KeyframeIndex = %+v, want %+v", k, want)
	}
	if k := libs[1].KeyframeIndex; k != nil {
		t.Errorf("Hörbücher: KeyframeIndex = %+v, want none", k)
	}
	if k := libs[2].KeyframeIndex; k == nil || *k != (model.LibraryKeyframeIndex{}) {
		t.Errorf("Leer: KeyframeIndex = %+v, want zeros", k)
	}
	if k := libs[3].KeyframeIndex; k == nil || *k != series {
		t.Errorf("Serien: KeyframeIndex = %+v, want %+v", k, series)
	}
	for id, want := range map[int64]*model.LibraryKeyframeIndex{1: &want, 2: nil, 3: {}, 4: &series} {
		lib, err := f.svc.GetLibrary(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got := lib.KeyframeIndex; (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Errorf("GetLibrary(%d): KeyframeIndex = %+v, want %+v", id, got, want)
		}
	}
	// A refused call passes its error through untouched.
	if _, err := f.svc.GetLibrary(ctx, 99); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("GetLibrary(99) = %v, want model.ErrNotFound", err)
	}
}
