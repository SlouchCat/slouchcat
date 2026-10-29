package library

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Tests of the podcast folders of ADR 0015 (concept 4.5, step 3 DoD) on the generated test media
// (testdata/media/PodcastFolder and the tagged episodes of testdata/media/Podcasts).

// podcastFolderMedia is the generated podcast folder (one podcast, two tagged episodes, one loose
// file in the root).
func podcastFolderMedia(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(testMedia(t)), "PodcastFolder")
	if _, err := os.Stat(filepath.Join(dir, "Slouchcat Test Folder Podcast", "metadata.json")); err != nil {
		t.Fatalf("podcast folder test media missing (run `task testdata:media`): %v", err)
	}
	return dir
}

// podcastDir builds a podcast folder in a temp root: the files of the generated folder (or of
// the mock feed's episodes) copied below name, with the given metadata.json.
func podcastDir(t *testing.T, root, name string, meta map[string]any, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		raw, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for target, source := range files {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, target), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// createPodcasts adds a podcast library with one folder.
func (e *env) createPodcasts(dir string) model.Library {
	e.t.Helper()
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Podcasts", Kind: model.LibraryPodcasts, Roots: rootReq(dir, model.RootPodcast)})
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

// mediaFileIDs lists every media file, by path.
func (e *env) mediaFileIDs(t *testing.T) []string {
	t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT id FROM media_files ORDER BY rel_path`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

// probeAll runs probe.file for every queued probe job of the library synchronously.
func (e *env) probeAll(t *testing.T) {
	t.Helper()
	ids := e.mediaFileIDs(t)
	for _, id := range ids {
		if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
			t.Fatalf("probe %s: %v", id, err)
		}
	}
}

func TestPodcastFolderScanReadsPodcastsAndEpisodes(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required (install ffmpeg)")
	}
	e := newEnv(t)
	lib := e.createPodcasts(podcastFolderMedia(t))
	got := e.scan(lib.ID)
	// The loose file in the root is passed over: two files, one podcast, two episodes.
	if got.Scan.State != "done" || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v, want done with 2 files", got.Scan)
	}
	checks := map[string]string{
		`SELECT group_concat(kind || '=' || n, ',') FROM (SELECT kind, count(*) AS n FROM items WHERE deleted_at IS NULL GROUP BY kind ORDER BY kind)`:                                                       "podcast=1,podcast_episode=2",
		`SELECT title || '/' || author || '/' || explicit || '/' || coalesce(feed_url, '-') || '/' || coalesce(feed_origin, '-') || '/' || providers || '/' || cover_path FROM items WHERE kind = 'podcast'`: `Slouchcat Test Folder Podcast/Slouchcat Test Author/0/-/-/["local"]/Slouchcat Test Folder Podcast/cover.jpg`,
		`SELECT genres FROM items WHERE kind = 'podcast'`:                                                                                     `{"de":["Education","Podcasts"],"en":["Education","Podcasts"]}`,
		`SELECT overview FROM item_texts WHERE lang = 'de' AND item_id = (SELECT id FROM items WHERE kind = 'podcast')`:                       "Ein Podcast aus einem Ordner, wie Audiobookshelf ihn ablegt.",
		`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND guid IS NULL AND enclosure_url IS NULL AND metadata_state = 'matched'`: "2",
		`SELECT count(*) FROM jobs WHERE type = ?`:                                                                                            "", // filled below
	}
	delete(checks, `SELECT count(*) FROM jobs WHERE type = ?`)
	for q, want := range checks {
		if v := e.str(q); v != want {
			t.Errorf("%s\n  = %q, want %q", q, v, want)
		}
	}
	// An image job for the folder cover, a probe per file, no match job (local), no keyframes.
	if n := e.jobCount(JobImageFetch); n != 1 {
		t.Errorf("image jobs = %d, want 1", n)
	}
	if n := e.jobCount(JobMetadataMatch); n != 0 {
		t.Errorf("match jobs = %d, want 0", n)
	}
	if n := e.jobCount(JobProbeFile); n != 2 {
		t.Errorf("probe jobs = %d, want 2", n)
	}
	// The first scan announces nothing (26 000 files would be 26 000 events).
	// After the probe: titles from the tags, except the one that repeats the podcast's title;
	// dates from the RFC 2822 tag; the description from the comment; no keyframe job for audio.
	e.probeAll(t)
	if n := e.jobCount(JobProbeKeyframes); n != 0 {
		t.Errorf("keyframe jobs for audio = %d, want 0", n)
	}
	titles := e.str(`SELECT group_concat(title, '|') FROM (SELECT title FROM items WHERE kind = 'podcast_episode' ORDER BY title)`)
	if titles != "Ordnerfolge Eins|Slouchcat Test Folder Podcast (3f5a2c1e-0000-4000-8000-000000000002)" {
		t.Errorf("episode titles = %q", titles)
	}
	one := e.str(`SELECT id FROM items WHERE title = 'Ordnerfolge Eins'`)
	want := time.Date(2025, 11, 3, 6, 0, 0, 0, time.UTC).UnixMilli()
	if v := e.count(`SELECT published_at FROM items WHERE id = ?`, one); int64(v) != want {
		t.Errorf("published_at of episode 1 = %d, want %d (the date tag)", v, want)
	}
	if v := e.str(`SELECT overview FROM item_texts WHERE item_id = ? AND lang = 'de'`, one); v != "Ordnerfolge 1." {
		t.Errorf("overview of episode 1 = %q", v)
	}
	if v := e.count(`SELECT coalesce(duration_ms, 0) FROM items WHERE id = ?`, one); v < 5000 || v > 7000 {
		t.Errorf("duration of episode 1 = %d ms, want about 6 s", v)
	}
	// A second scan of the unchanged folder writes nothing.
	e.svc.TakeWriteStats()
	e.scan(lib.ID)
	if st := e.svc.TakeWriteStats(); st.Ops != 0 {
		t.Errorf("second scan wrote %d operations", st.Ops)
	}
}

func TestPodcastFolderAdoptsAndRefusesFeeds(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	root := t.TempDir()
	media := filepath.Join(filepath.Dir(testMedia(t)), "Podcasts")
	// A folder whose feed the library subscribed to by hand, a second folder with the same feed,
	// a folder with a feed on a local host, and a folder with a public feed nobody subscribed to.
	podcastDir(t, root, "Abonniert", map[string]any{"title": "Abonniert", "feedURL": "https://feeds.example.test/a.xml"},
		map[string]string{"a1.mp3": filepath.Join(media, "episode-1.mp3")})
	podcastDir(t, root, "Abonniert Zwei", map[string]any{"title": "Abonniert Zwei", "feedURL": "https://feeds.example.test/a.xml"},
		map[string]string{"a2.mp3": filepath.Join(media, "episode-2.mp3")})
	podcastDir(t, root, "Lokal", map[string]any{"title": "Lokal", "feedURL": "http://127.0.0.1:9/feed.xml"},
		map[string]string{"l1.mp3": filepath.Join(media, "episode-3.mp3")})
	podcastDir(t, root, "Neu", map[string]any{"title": "Neu", "feedURL": "https://feeds.example.test/neu.xml", "explicit": true},
		map[string]string{"n1.mp3": filepath.Join(media, "episode-1.mp3")})
	lib := e.createPodcasts(root)
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers,
	    metadata_state, added_at, created_at, updated_at) VALUES ('01ABONNIERT00000000000000A', 'podcast', ?, 'Abo von Hand', 'abo von hand',
	    'https://feeds.example.test/a.xml', 'admin', '["local"]', 'matched', 0, 0, 0)`, lib.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 4 {
		t.Fatalf("scan = %+v", got.Scan)
	}
	// Three podcasts: the subscription took both "Abonniert" folders, "Lokal" and "Neu" are new.
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast' AND deleted_at IS NULL`); n != 3 {
		t.Fatalf("podcasts = %d, want 3", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND parent_id = '01ABONNIERT00000000000000A'`); n != 2 {
		t.Fatalf("episodes of the subscription = %d, want the files of both folders", n)
	}
	if v := e.str(`SELECT feed_error FROM items WHERE id = '01ABONNIERT00000000000000A'`); v != "two_folders_one_feed" {
		t.Errorf("note on the shared feed = %q", v)
	}
	if v := e.str(`SELECT coalesce(feed_url, '-') || '/' || coalesce(feed_origin, '-') || '/' || coalesce(feed_error, '-') FROM items WHERE title = 'Lokal'`); v != "-/-/feed_url_not_allowed: http://127.0.0.1:9" {
		t.Errorf("local feed from a file: %q, want refused with scheme and host only", v)
	}
	if v := e.str(`SELECT feed_url || '/' || feed_origin || '/' || own_rating || '/' || rating_source FROM items WHERE title = 'Neu'`); v != "https://feeds.example.test/neu.xml/file/18/tags" {
		t.Errorf("new feed from a file: %q", v)
	}
	// The episodes carry the library's default rating through the podcast; the explicit one 18.
	if v := e.str(`SELECT effective_rating FROM items WHERE kind = 'podcast_episode' AND parent_id = (SELECT id FROM items WHERE title = 'Neu')`); v != "18" {
		t.Errorf("effective rating of the explicit podcast's episode = %q", v)
	}
	// Subscriptions with a feed are due for a fetch, the folder podcast without one never.
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast' AND deleted_at IS NULL AND feed_url IS NULL`); n != 1 {
		t.Errorf("podcasts without feed = %d, want 1 (Lokal)", n)
	}
}

func TestPodcastFolderRemovalKeepsFeedPodcastsAsSubscriptions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	root := t.TempDir()
	media := filepath.Join(filepath.Dir(testMedia(t)), "Podcasts")
	podcastDir(t, root, "Mit Feed", map[string]any{"title": "Mit Feed", "feedURL": "https://feeds.example.test/f.xml"},
		map[string]string{"f1.mp3": filepath.Join(media, "episode-1.mp3")})
	podcastDir(t, root, "Ohne Feed", map[string]any{"title": "Ohne Feed"},
		map[string]string{"o1.mp3": filepath.Join(media, "episode-2.mp3")})
	lib := e.createPodcasts(root)
	e.scan(lib.ID)
	rootID := rootIDOf(t, e, lib.ID, root)
	if _, err := e.svc.RemoveRoot(e.ctx, lib.ID, rootID, root); err != nil {
		t.Fatalf("remove root: %v", err)
	}
	if v := e.str(`SELECT coalesce(deleted_at, 0) || '/' || feed_origin FROM items WHERE title = 'Mit Feed'`); v != "0/admin" {
		t.Errorf("podcast with feed after the removal = %q, want kept as a subscription by hand", v)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE title = 'Ohne Feed' AND deleted_at IS NOT NULL`); n != 1 {
		t.Errorf("podcast without feed survived the removal")
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND deleted_at IS NULL`); n != 0 {
		t.Errorf("file episodes left after the removal = %d", n)
	}
}

func TestSweepTakesFileEpisodesWithoutEnclosure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	root := t.TempDir()
	media := filepath.Join(filepath.Dir(testMedia(t)), "Podcasts")
	podcastDir(t, root, "Show", map[string]any{"title": "Show"},
		map[string]string{"e1.mp3": filepath.Join(media, "episode-1.mp3"), "e2.mp3": filepath.Join(media, "episode-2.mp3")})
	lib := e.createPodcasts(root)
	e.scan(lib.ID)
	// One episode gets an enclosure (a feed knows it), the other stays file-only; both files go.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET enclosure_url = 'https://cdn.example.test/e1.mp3', guid = 'g1' WHERE title = 'e1'`); err != nil {
		t.Fatal(err)
	}
	remove(t, root, "Show/e1.mp3")
	remove(t, root, "Show/e2.mp3")
	// Every file of the folder is gone: the loss protection would stop the scan, so the
	// administrator's acceptance stands in for it here.
	if _, err := e.svc.AcceptLoss(e.ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID) // marks both missing
	e.clock.Advance(missingGrace + time.Hour)
	if _, err := e.svc.Sweep(e.ctx); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT group_concat(title || ':' || (deleted_at IS NOT NULL), ',') FROM (SELECT title, deleted_at FROM items WHERE kind = 'podcast_episode' ORDER BY title)`); v != "e1:0,e2:1" {
		t.Errorf("episodes after the sweep = %q, want the feed episode kept and the file episode gone", v)
	}
	// The podcast without feed stays while an episode is alive; it goes with the last one.
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast' AND deleted_at IS NULL`); n != 1 {
		t.Errorf("podcast with a live feed episode was swept")
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET deleted_at = 1 WHERE title = 'e1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Sweep(e.ctx); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast' AND deleted_at IS NULL`); n != 0 {
		t.Errorf("podcast without feed and episodes survived the sweep")
	}
}

func TestPodcastFolderPairsFileWithFeedEpisodeAfterProbe(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required (install ffmpeg)")
	}
	e := newEnv(t)
	root := t.TempDir()
	media := filepath.Join(filepath.Dir(testMedia(t)), "Podcasts")
	// episode-1.mp3 is tagged with the pubDate of the mock feed's first episode; episode-2.mp3
	// with its second. The podcast is a subscription with three feed episodes, none with a file:
	// the first pairs by the minute, the second has two feed candidates with the same minute and
	// stays apart, the third has no file at all.
	podcastDir(t, root, "Paar", map[string]any{"title": "Paar", "feedURL": "https://feeds.example.test/p.xml"},
		map[string]string{"one.mp3": filepath.Join(media, "episode-1.mp3"), "two.mp3": filepath.Join(media, "episode-2.mp3")})
	lib := e.createPodcasts(root)
	exec := func(q string, args ...any) {
		if _, err := e.db.Write.ExecContext(e.ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers, metadata_state, added_at, created_at, updated_at)
	      VALUES ('01PAAR0000000000000000000P', 'podcast', ?, 'Paar', 'paar', 'https://feeds.example.test/p.xml', 'admin', '["local"]', 'matched', 0, 0, 0)`, lib.ID)
	first := time.Date(2025, 10, 6, 6, 0, 0, 0, time.UTC).UnixMilli()
	second := time.Date(2025, 10, 13, 6, 0, 30, 0, time.UTC).UnixMilli() // same minute as the tag of episode-2
	for _, ep := range []struct {
		id, title, guid string
		published       int64
		duration        int64
	}{
		{"01PAAR0000000000000000000E", "Feed One", "g1", first, 8000},
		{"01PAAR0000000000000000000F", "Feed Two A", "g2a", second, 10000},
		{"01PAAR0000000000000000000G", "Feed Two B", "g2b", second, 10000},
	} {
		exec(`INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, published_at, duration_ms, enclosure_url, providers,
		      metadata_state, added_at, created_at, updated_at) VALUES (?, 'podcast_episode', '01PAAR0000000000000000000P', ?, ?, ?, ?, ?, ?,
		      'https://cdn.example.test/' || ? || '.mp3', '["local"]', 'matched', 0, 0, 0)`,
			ep.id, lib.ID, ep.title, SortTitle(ep.title), ep.guid, ep.published, ep.duration, ep.guid)
	}
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND deleted_at IS NULL`); n != 5 {
		t.Fatalf("episodes after the scan = %d, want 3 feed + 2 file", n)
	}
	e.probeAll(t)
	// Feed One holds the file now and kept its guid and enclosure; the file episode left.
	if v := e.str(`SELECT guid || '/' || enclosure_url || '/' || (SELECT count(*) FROM media_files WHERE item_id = items.id) FROM items WHERE id = '01PAAR0000000000000000000E'`); v != "g1/https://cdn.example.test/g1.mp3/1" {
		t.Errorf("paired feed episode = %q", v)
	}
	if v := e.str(`SELECT group_concat(title || ':' || (SELECT count(*) FROM media_files WHERE item_id = i.id), ',') FROM (SELECT id, title FROM items AS i WHERE kind = 'podcast_episode' AND deleted_at IS NULL ORDER BY title) AS i`); v != "Feed One:1,Feed Two A:0,Feed Two B:0,Slouchcat Test Episode 2:1" {
		t.Errorf("episodes after the probe = %q, want the ambiguous file kept apart", v)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND deleted_at IS NOT NULL`); n != 1 {
		t.Errorf("retired file episodes = %d, want 1", n)
	}
	var published sql.NullInt64
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT published_at FROM items WHERE title = 'Slouchcat Test Episode 2'`).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2025, 10, 13, 6, 0, 0, 0, time.UTC).UnixMilli(); published.Int64 != want {
		t.Errorf("published_at from the tag = %d, want %d", published.Int64, want)
	}
}
