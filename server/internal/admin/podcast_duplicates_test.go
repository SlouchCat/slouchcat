package admin

import (
	"testing"
)

// ADR 0015: a file episode of a podcast folder and a feed episode without file that share the
// publication minute (or the title) but were not paired by the scanner show up as one
// file_and_feed group of listDuplicates, with the file on the file episode.
func TestAdminMaintenanceFileAndFeedDuplicates(t *testing.T) {
	m := newMaintFixture(t)
	m.f.AddAudio(t)
	a := m.f.Audio
	// A file episode at the minute of the enclosure-only episode, and a second file episode at a
	// minute nobody else has: only the first pairs.
	m.exec(t, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, published_at, providers, metadata_state, added_at, created_at, updated_at)
	           SELECT '01FILE00000000000000000DUP', 'podcast_episode', parent_id, library_id, 'Aus dem Ordner', 'aus dem ordner', published_at + 20000, '["local"]', 'matched', 0, 0, 0
	           FROM items WHERE id = ?`, a.EnclosureOnly.ID)
	m.exec(t, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	           SELECT '01FILE00000000000000000DUF', '01FILE00000000000000000DUP', root_id, 'Podcast/aus-dem-ordner.mp3', 1000, 1, 'mp3', 1, 1
	           FROM media_files WHERE item_id = ?`, a.Episodes[0].ID)
	m.exec(t, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, published_at, providers, metadata_state, added_at, created_at, updated_at)
	           SELECT '01FILE00000000000000000ALN', 'podcast_episode', parent_id, library_id, 'Allein', 'allein', 1000, '["local"]', 'matched', 0, 0, 0
	           FROM items WHERE id = ?`, a.EnclosureOnly.ID)
	groups, err := m.svc.ListDuplicates(m.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var pairs [][]string
	for _, g := range groups {
		if g.Reason != DuplicateFileAndFeed {
			continue
		}
		var ids []string
		for _, it := range g.Items {
			ids = append(ids, it.ItemID)
		}
		pairs = append(pairs, ids)
		if len(g.Items) != 2 || len(g.Items[0].Files) != 1 || g.Items[0].Files[0].Path != "Podcast/aus-dem-ordner.mp3" || len(g.Items[1].Files) != 0 {
			t.Errorf("file_and_feed group = %+v, want the file on the file episode only", g.Items)
		}
	}
	if len(pairs) != 1 || pairs[0][0] != "01FILE00000000000000000DUP" || pairs[0][1] != a.EnclosureOnly.ID {
		t.Fatalf("file_and_feed groups = %v, want one pair of the file episode and the enclosure-only episode", pairs)
	}
	lib := a.PodcastLibraryID + 1000
	if groups, err := m.svc.ListDuplicates(m.ctx, &lib); err != nil || len(groups) != 0 {
		t.Errorf("duplicates of another library = %+v, %v", groups, err)
	}
}
