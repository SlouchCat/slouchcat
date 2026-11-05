package library

import (
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Review of wave C, m-1: the indexworthy condition is stated four times, in indexWorthy and in the
// SQL of AdminCountKeyframeIndex, CountKeyframesBackfillFiles and ListKeyframesBackfillFiles
// (concept 14, 4.1). Over every combination of codec, kind, presence and source the three queries
// count exactly the files indexWorthy accepts: pending (source NULL) as the backfill lists and
// counts them, indexed and unavailable as the coverage counts them.
func TestIndexWorthyConditionMatchesTheQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(tree(t, nil))
	rootID := e.count(`SELECT id FROM library_roots WHERE library_id = ?`, lib.ID)

	type want struct{ pending, indexed, unavailable int64 }
	var expect want
	var pending []string
	n := 0
	for _, codec := range []any{"h264", "hevc", "av1", "vp9", "vc1", "mpeg2video", nil} {
		for _, kind := range []string{"movie", "episode", "audiobook", "podcast_episode"} {
			for _, missing := range []bool{false, true} {
				for _, source := range []any{nil, "cues", "none"} {
					n++
					item, file := fmt.Sprintf("01KW%022d", n), fmt.Sprintf("01KF%022d", n)
					var missingSince any
					if missing {
						missingSince = int64(5)
					}
					e.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, added_at, created_at, updated_at)
						VALUES (?, ?, ?, ?, ?, 1, 1, 1)`, item, kind, lib.ID, item, item)
					e.exec(`INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, video_codec,
						keyframes_source, missing_since, created_at, updated_at) VALUES (?, ?, ?, ?, 1, 1, 'mkv', ?, ?, ?, 1, 1)`,
						file, item, rootID, file+".mkv", codec, source, missingSince)
					c, _ := codec.(string)
					if !indexWorthy(!missing, c, kind) {
						continue
					}
					switch source {
					case nil:
						expect.pending++
						pending = append(pending, file)
					case "none":
						expect.unavailable++
					default:
						expect.indexed++
					}
				}
			}
		}
	}
	if expect.pending == 0 || expect.indexed == 0 || expect.unavailable == 0 {
		t.Fatalf("matrix without a worthy file in every class: %+v", expect)
	}

	q := systemq.New(e.db.Read)
	rows, err := q.AdminCountKeyframeIndex(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got want
	for _, r := range rows {
		if r.LibraryID != lib.ID {
			t.Errorf("coverage row of library %d, want only %d", r.LibraryID, lib.ID)
		}
		switch r.KeyframesSource {
		case sql.NullString{}:
			got.pending += r.Files
		case sql.NullString{String: "none", Valid: true}:
			got.unavailable += r.Files
		default:
			got.indexed += r.Files
		}
	}
	if got != expect {
		t.Errorf("AdminCountKeyframeIndex = %+v, want %+v (indexWorthy)", got, expect)
	}

	total, err := q.CountKeyframesBackfillFiles(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != expect.pending {
		t.Errorf("CountKeyframesBackfillFiles = %d, want %d", total, expect.pending)
	}

	var listed []string
	for after := ""; ; {
		batch, err := q.ListKeyframesBackfillFiles(e.ctx, systemq.ListKeyframesBackfillFilesParams{AfterID: after, LimitCount: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(batch) == 0 {
			break
		}
		for _, f := range batch {
			listed = append(listed, f.ID)
			if !indexWorthy(true, f.VideoCodec.String, f.Kind) {
				t.Errorf("ListKeyframesBackfillFiles lists %s (%s, %s), which indexWorthy refuses", f.ID, f.VideoCodec.String, f.Kind)
			}
		}
		after = batch[len(batch)-1].ID
	}
	if !slices.Equal(listed, pending) {
		t.Errorf("ListKeyframesBackfillFiles over all batches = %d files %v, want %d %v", len(listed), listed, len(pending), pending)
	}
}
