package library

import (
	"path/filepath"
	"testing"
)

// TestScanGeneratedMedia scans the generated test media (testdata/generate-media.sh) and checks
// the M3 cases the web e2e specs rely on: versions, extras, the unplayable file, the duplicate,
// the title without year, specials and the multi-episode file.
func TestScanGeneratedMedia(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	movies := e.create(testMedia(t))
	if got := e.scan(movies.ID); got.Scan.State != "done" {
		t.Fatalf("movie scan = %+v", got.Scan)
	}
	checks := map[string]string{
		// B-5: two versions of one title.
		`SELECT count(*) FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE i.kind = 'movie' AND i.title = 'Slouchcat Test H264'`: "2",
		`SELECT version_label FROM media_files WHERE rel_path LIKE '%(2024) - 720p.mp4'`:                                                       "720p",
		// B-7: five extras of the extras film.
		`SELECT group_concat(extra_kind || ':' || title, ',') FROM (SELECT e.extra_kind, e.title FROM items AS e JOIN items AS m ON m.id = e.parent_id
		  WHERE e.kind = 'extra' AND m.title = 'Slouchcat Test Extras' ORDER BY e.extra_kind, e.title)`: "behind_the_scenes:On Set,deleted_scene:Scene 1,featurette:Making Of,trailer:Slouchcat Test Extras (2024),trailer:Slouchcat Test Extras Trailer",
		// B-9: duplicate by hash, title without year.
		`SELECT count(DISTINCT item_id) FROM media_files WHERE partial_hash = (SELECT partial_hash FROM media_files WHERE rel_path LIKE 'Slouchcat Test H264 Copy (2024)/%')`: "2",
		`SELECT coalesce(year, 0) || '/' || coalesce(own_rating, -1) FROM items WHERE title = 'Slouchcat Test Unrated'`:                                                       "0/-1",
	}
	for q, want := range checks {
		if v := e.str(q); v != want {
			t.Errorf("%s\n  = %q, want %q", q, v, want)
		}
	}
	broken := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Slouchcat Test Broken (2024)/%'`)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, broken)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT probe_error IS NOT NULL AND probed_at IS NOT NULL FROM media_files WHERE id = ?`, broken); v != "1" {
		t.Errorf("broken file has no probe_error")
	}

	tv := e.createSeries(filepath.Join(filepath.Dir(testMedia(t)), "TV"))
	// Four files of the test show, five of the intro season (TestSegmentsDetectGeneratedSeason) and
	// six of the download show (M5 season download).
	if got := e.scan(tv.ID); got.Scan.State != "done" || got.Scan.FileCount != 15 {
		t.Fatalf("series scan = %+v", got.Scan)
	}
	if v := e.str(`SELECT group_concat(season_number || 'x' || episode_number || '-' || coalesce(episode_end, 0), ',') FROM (
	               SELECT season_number, episode_number, episode_end FROM items WHERE kind = 'episode' AND library_id = ?
	               AND parent_id IN (SELECT s.id FROM items AS s JOIN items AS r ON r.id = s.parent_id WHERE r.title = 'Slouchcat Test Show')
	               ORDER BY season_number, episode_number)`, tv.ID); v != "0x1-0,1x1-0,1x2-0,1x3-4" {
		t.Errorf("episodes = %q", v)
	}
	if v := e.str(`SELECT group_concat(title, ',') FROM (SELECT title FROM items WHERE kind = 'season' AND library_id = ? ORDER BY sort_title)`, tv.ID); v != "Season 1,Season 1,Season 1,Specials" {
		t.Errorf("seasons = %q", v)
	}
}
