package library

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const showDir = "Show (2020) {tvdb-100}"

// seriesTree is a series library with specials, a multi-episode file, two versions of one
// episode, extras at series and season level, a file without marker, a series in scene naming
// directly in the root and a daily show.
var seriesTree = []string{
	showDir + "/Season 01/Show - S01E01 - Pilot.mkv",
	showDir + "/Season 01/Show - S01E01 - Pilot - 2160p.mkv",
	showDir + "/Season 01/Show - S01E02.mkv",
	showDir + "/Season 01/Show - S01E03-E04 - Double.mkv",
	showDir + "/Specials/Show - S00E01 - Christmas.mkv",
	showDir + "/Trailers/Show Trailer.mkv",
	showDir + "/Season 01/Featurettes/Recap.mkv",
	showDir + "/Season 01/Show - S01E01-trailer.mkv",
	showDir + "/Season 01/Show - Recap.mkv",
	"Other.Show.S02E05.720p.HDTV.x264-GRP.mkv",
	"Other.Show.S02E06.720p.HDTV.x264-GRP.mkv",
	"Daily (2000)/Daily - 2024-03-15 - Guest.mkv",
}

const tvshowNFOFixture = `<tvshow><title>Show</title><originaltitle>The Show</originaltitle><mpaa>FSK 12</mpaa>
<uniqueid type="tmdb">555</uniqueid></tvshow>`

const episodeNFOFixture = `<episodedetails><title>Zwei</title><season>1</season><episode>2</episode>
<aired>2020-01-08</aired><mpaa>FSK 16</mpaa></episodedetails>`

// createSeries adds a series library on dir.
func (e *env) createSeries(dir string) model.Library {
	e.t.Helper()
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Serien", Kind: "series", Path: dir})
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

func seriesDirTree(t *testing.T) string {
	t.Helper()
	dir := tree(t, seriesTree)
	writeFile(t, dir, showDir+"/tvshow.nfo", tvshowNFOFixture)
	writeFile(t, dir, showDir+"/Season 01/Show - S01E02.nfo", episodeNFOFixture)
	writeFile(t, dir, showDir+"/Season 01/season.nfo", `<season><title>Staffel 1</title></season>`)
	return dir
}

// episode returns the id of the episode of a series title by numbers.
func (e *env) episode(series string, season, episode int) string {
	e.t.Helper()
	return e.str(`SELECT ep.id FROM items AS ep JOIN items AS se ON se.id = ep.parent_id JOIN items AS sr ON sr.id = se.parent_id
	              WHERE ep.kind = 'episode' AND sr.title = ? AND ep.season_number = ? AND ep.episode_number = ? AND ep.deleted_at IS NULL`,
		series, season, episode)
}

func TestSeriesScanBuildsTree(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.FilesSeen != 11 || got.Scan.FileCount != 11 {
		t.Fatalf("scan = %+v, want done with 11 files", got.Scan)
	}
	counts := e.str(`SELECT group_concat(kind || '=' || n, ',') FROM (SELECT kind, count(*) AS n FROM items
	                 WHERE deleted_at IS NULL GROUP BY kind ORDER BY kind)`)
	if counts != "episode=7,extra=3,season=4,series=3" {
		t.Fatalf("items = %s", counts)
	}

	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	checks := map[string]string{
		`SELECT tvdb_id || '/' || tmdb_id || '/' || year || '/' || original_title || '/' || own_rating || '/' || rating_source FROM items WHERE id = '` + show + `'`:                     "100/555/2020/The Show/12/nfo",
		`SELECT group_concat(title || ':' || season_number, ',') FROM (SELECT title, season_number FROM items WHERE kind = 'season' AND parent_id = '` + show + `' ORDER BY sort_title)`: "Season 1:1,Specials:0",
		`SELECT title || '/' || episode_end FROM items WHERE kind = 'episode' AND episode_number = 3`:                                                                                    "Double/4",
		`SELECT title FROM items WHERE kind = 'episode' AND season_number = 0`:                                                                                                           "Christmas",
		`SELECT title || '/' || release_date || '/' || own_rating || '/' || effective_rating FROM items WHERE kind = 'episode' AND title = 'Zwei'`:                                       "Zwei/2020-01-08/16/16",
		`SELECT count(*) FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE i.kind = 'episode' AND i.title = 'Pilot'`:                                                       "2",
		`SELECT group_concat(coalesce(version_label, '-'), ',') FROM (SELECT version_label FROM media_files WHERE rel_path LIKE '%S01E01 - Pilot%' ORDER BY rel_path)`:                   "2160p,-",
		`SELECT effective_rating FROM items WHERE kind = 'episode' AND title = 'Pilot'`:                                                                                                  "12",
		`SELECT available || '/' || min_child_rating FROM items WHERE id = '` + show + `'`:                                                                                               "1/12",
		`SELECT group_concat(title || ':' || season_number || ':' || episode_number, ',') FROM (SELECT title, season_number, episode_number FROM items WHERE kind = 'episode' AND parent_id IN (SELECT s.id FROM items AS s JOIN items AS r ON r.id = s.parent_id WHERE r.title = 'Other Show') ORDER BY episode_number)`: "Episode 5:2:5,Episode 6:2:6",
		`SELECT season_number || '/' || episode_number || '/' || release_date || '/' || title FROM items WHERE kind = 'episode' AND release_date = '2024-03-15'`:                                                                                                                                                          "2024/315/2024-03-15/Guest",
	}
	for q, want := range checks {
		if v := e.str(q); v != want {
			t.Errorf("%s\n  = %q, want %q", q, v, want)
		}
	}
	season1 := e.str(`SELECT id FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 1`, show)
	if v := e.str(`SELECT group_concat(parent_id || ':' || extra_kind, ',') FROM (SELECT parent_id, extra_kind FROM items WHERE kind = 'extra' ORDER BY extra_kind, title)`); v !=
		season1+":featurette,"+season1+":trailer,"+show+":trailer" {
		t.Errorf("extras = %q (series %s, season %s)", v, show, season1)
	}
	// Only the series queue metadata.match (seasons, episodes and extras follow the series job).
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND priority = 4 AND payload LIKE '%' || (SELECT id FROM items WHERE kind = 'series' AND title = 'Show') || '%'`, JobMetadataMatch); n != 1 {
		t.Errorf("match job of the series = %d", n)
	}
	if n := e.jobCount(JobMetadataMatch); n != 3 {
		t.Errorf("metadata.match jobs = %d, want 3", n)
	}
	if n := e.jobCount(JobProbeFile); n != 11 {
		t.Errorf("probe.file jobs = %d, want 11", n)
	}

	// A second scan of the unchanged tree writes nothing.
	e.clock.Advance(time.Hour)
	since := e.clock.Now()
	e.svc.TakeWriteStats()
	e.scan(lib.ID)
	if items, files, jobs := e.touched(since); items+files+jobs != 0 {
		t.Errorf("second scan touched %d items, %d files, %d jobs", items, files, jobs)
	}
	if st := e.svc.TakeWriteStats(); st.Batches != 0 || st.Ops != 0 {
		t.Errorf("second scan wrote %+v", st)
	}

	// A new episode of a matched series queues its own match at priority 3.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, show); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, showDir+"/Season 01/Show - S01E05.mkv", "five")
	writeFile(t, dir, "Daily (2000)/Daily - 2024-03-16.mkv", "next day")
	e.scan(lib.ID)
	ep5 := e.episode("Show", 1, 5)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND priority = 3 AND idempotency_key = ?`, JobMetadataMatch, "match:"+ep5); n != 1 {
		t.Errorf("episode match jobs = %d, want 1", n)
	}
	daily := e.episode("Daily", 2024, 316)
	if n := e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ?`, "match:"+daily); n != 0 {
		t.Errorf("episode of an unmatched series queued its own match")
	}
}

func TestSeriesRenameKeepsEpisodeAndProgress(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	ep := e.episode("Show", 1, 2)
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'Marco', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := appq.New(e.db.Write).UpsertProgress(e.ctx, appq.UpsertProgressParams{ProfileID: 1, ItemID: ep, PositionMs: 60_000, DurationMs: 1_000_000, Now: 1}); err != nil {
		t.Fatal(err)
	}
	rename(t, dir, showDir+"/Season 01/Show - S01E02.mkv", showDir+"/Season 01/Show (2020) - S01E02 - Zwei - 1080p.mkv")
	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.FileCount != 11 {
		t.Fatalf("scan after rename = %+v", got.Scan)
	}
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE '%Zwei - 1080p.mkv'`); v != ep {
		t.Errorf("renamed file belongs to %q, want %q", v, ep)
	}
	if n := e.count(`SELECT count(*) FROM progress WHERE item_id = ? AND position_ms = 60000`, ep); n != 1 {
		t.Errorf("progress rows = %d", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'episode' AND deleted_at IS NULL`); n != 7 {
		t.Errorf("episodes = %d after rename, want 7", n)
	}
}

func TestSeriesMissingEpisodesAndSweep(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)
	specials := e.str(`SELECT id FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 0`, show)
	daily := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Daily'`)

	// The only special and the only file of the daily show disappear (2 of 11, below 20 %).
	remove(t, dir, showDir+"/Specials/Show - S00E01 - Christmas.mkv")
	remove(t, dir, "Daily (2000)/Daily - 2024-03-15 - Guest.mkv")
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		t.Fatalf("scan = %+v", got.Scan)
	}
	avail := func(id string) string {
		return e.str(`SELECT available || '/' || (deleted_at IS NOT NULL) FROM items WHERE id = ?`, id)
	}
	if v := avail(specials); v != "0/0" {
		t.Errorf("specials season available/deleted = %s, want 0/0", v)
	}
	if v := avail(show); v != "1/0" {
		t.Errorf("series available/deleted = %s, want 1/0", v)
	}
	if v := avail(daily); v != "0/0" {
		t.Errorf("daily available/deleted = %s, want 0/0", v)
	}

	// Within the grace period nothing is deleted; after it the episodes, the empty season and the
	// empty series go in one sweep. The series Show stays with its other seasons.
	e.clock.Advance(23 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 0 {
		t.Fatalf("early sweep = %d, %v", n, err)
	}
	e.clock.Advance(2 * time.Hour)
	n, err := e.svc.Sweep(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Christmas, the daily episode, the specials season, the daily season and the daily series.
	if n != 5 {
		t.Errorf("sweep deleted %d items, want 5", n)
	}
	if v := avail(specials); v != "0/1" {
		t.Errorf("specials after sweep = %s", v)
	}
	if v := avail(daily); v != "0/1" {
		t.Errorf("daily after sweep = %s", v)
	}
	if v := avail(show); v != "1/0" {
		t.Errorf("series after sweep = %s", v)
	}

	// The special comes back (same content, new inode): episode, season restored, ids kept.
	christmas := e.str(`SELECT id FROM items WHERE kind = 'episode' AND season_number = 0`)
	writeFile(t, dir, showDir+"/Specials/Show - S00E01 - Christmas.mkv", fmt.Sprintf("video %d %s", 4, seriesTree[4]))
	e.scan(lib.ID)
	if v := avail(specials); v != "1/0" {
		t.Errorf("specials after return = %s", v)
	}
	if v := avail(christmas); v != "1/0" {
		t.Errorf("special episode after return = %s", v)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 0`, show); n != 1 {
		t.Errorf("specials seasons = %d, want 1", n)
	}
}

func TestSeriesPathScanNewSeason(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	e.clock.Advance(time.Hour)
	since := e.clock.Now()
	show := e.str(`SELECT id FROM items WHERE kind = 'series' AND title = 'Show'`)

	writeFile(t, dir, showDir+"/Season 02/Show - S02E01 - Neu.mkv", "s2e1")
	writeFile(t, dir, showDir+"/Season 02/Show - S02E02.mkv", "s2e2")
	writeFile(t, dir, showDir+"/Season 02/Trailers/Season 2 Trailer.mkv", "s2 trailer")
	got := e.pathScan(lib, showDir, showDir+"/Season 02", showDir+"/Season 02/Trailers")
	if got.Scan.State != "done" || got.Scan.FileCount != 14 {
		t.Fatalf("path scan = %+v", got.Scan)
	}
	season2 := e.str(`SELECT id FROM items WHERE kind = 'season' AND parent_id = ? AND season_number = 2`, show)
	if season2 == "" {
		t.Fatal("season 2 not created below the existing series")
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'episode' AND parent_id = ?`, season2); n != 2 {
		t.Errorf("season 2 episodes = %d", n)
	}
	if v := e.str(`SELECT extra_kind FROM items WHERE kind = 'extra' AND parent_id = ?`, season2); v != "trailer" {
		t.Errorf("season 2 extra = %q", v)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series'`); n != 3 {
		t.Errorf("series = %d, want 3", n)
	}
	// New rows only: season, two episodes, one extra; three files; probes, no new series match.
	items, files, jobsN := e.touched(since)
	if items != 4 || files != 3 || jobsN != 3 {
		t.Errorf("path scan touched %d items, %d files, %d jobs; want 4, 3, 3", items, files, jobsN)
	}
}

// cancelRunsAt returns a clock for a service that cancels the running scans of a library like
// CancelScan does, at the n-th reading of the clock (inside the scan's write phase).
func cancelRunsAt(svc **Service, clk *testClock, libraryID *int64, n int64) func() time.Time {
	var calls atomic.Int64
	return func() time.Time {
		if calls.Add(1) == n {
			s := *svc
			s.mu.Lock()
			for r := range s.runs[*libraryID] {
				r.cancel(errScanCancelled)
			}
			s.mu.Unlock()
		}
		return clk.Now()
	}
}

func TestCancelScanMidScanAndResume(t *testing.T) {
	e := newEnv(t)
	var files []string
	for s := 1; s <= 4; s++ {
		for ep := 1; ep <= 100; ep++ {
			files = append(files, fmt.Sprintf("Big (2010)/Season %02d/Big - S%02dE%02d.mkv", s, s, ep))
		}
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)

	var svc *Service
	libID := lib.ID
	svc = New(e.db, e.queue, transcode.Runner{}, discard, WithClock(cancelRunsAt(&svc, e.clock, &libID, 300)))
	appLib, err := appq.New(e.db.Read).GetLibrary(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Scan(e.ctx, appLib); err != nil {
		t.Fatalf("cancelled scan returned %v, want nil", err)
	}
	got, err := svc.Get(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scan.State != "aborted" || got.Scan.LastError != "cancelled" {
		t.Fatalf("after cancel: state %q error %q", got.Scan.State, got.Scan.LastError)
	}
	partial := e.count(`SELECT count(*) FROM media_files`)
	if partial == 0 || partial >= 400 {
		t.Fatalf("cancel mid-scan left %d media files, want some of 400", partial)
	}
	if got.Scan.FilesSeen != partial || got.Scan.FilesTotal != 400 {
		t.Errorf("progress %d/%d, want %d/400", got.Scan.FilesSeen, got.Scan.FilesTotal, partial)
	}
	t.Logf("cancelled after %d of 400 files", partial)

	// The next scan converges to the full result, without duplicates.
	if _, err := e.svc.QueueScan(e.ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	full := e.scan(lib.ID)
	if full.Scan.State != "done" || full.Scan.FileCount != 400 || full.Scan.ItemCount != 1+4+400 {
		t.Fatalf("resumed scan = %+v", full.Scan)
	}
	if n := e.count(`SELECT count(*) FROM (SELECT idempotency_key FROM jobs WHERE state = 'queued' GROUP BY idempotency_key HAVING count(*) > 1)`); n != 0 {
		t.Errorf("%d duplicate queued jobs", n)
	}
}

func TestCancelScanQueuedAndWaiting(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	if _, err := e.svc.CancelScan(e.ctx, 999); err == nil {
		t.Error("CancelScan of an unknown library succeeded")
	}

	// A queued scan job is cancelled; the library records the cancellation.
	got, err := e.svc.CancelScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scan.State != "aborted" || got.Scan.LastError != "cancelled" {
		t.Fatalf("after cancel: %+v", got.Scan)
	}
	if v := e.str(`SELECT state FROM jobs WHERE type = ?`, JobScanLibrary); v != string(jobs.StateCancelled) {
		t.Errorf("scan job state = %q", v)
	}
	// A cancelled job that a worker claims anyway does not scan.
	id := e.count(`SELECT id FROM jobs WHERE type = ?`, JobScanLibrary)
	if err := e.svc.handleScan(e.ctx, &jobs.Job{ID: int64(id), Type: JobScanLibrary, Payload: []byte(fmt.Sprintf(`{"libraryId":%d}`, lib.ID))}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT count(*) FROM items`); n != 0 {
		t.Errorf("cancelled job scanned %d items", n)
	}

	// A scan waiting for the scan lock (another scan of the library runs) stops at once.
	unlock, err := e.svc.lockScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	appLib, err := appq.New(e.db.Read).GetLibrary(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	go func() { done <- e.svc.Scan(e.ctx, appLib) }()
	for deadline := time.Now().Add(5 * time.Second); ; {
		e.svc.mu.Lock()
		n := len(e.svc.runs[lib.ID])
		e.svc.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("scan did not register")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := e.svc.CancelScan(e.ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("cancelled waiting scan returned %v", err)
	}
	unlock()
	if got, _ := e.svc.Get(e.ctx, lib.ID); got.Scan.State != "aborted" || got.Scan.LastError != "cancelled" {
		t.Errorf("after cancelling the waiting scan: %+v", got.Scan)
	}
	// The next scan runs normally.
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 11 {
		t.Errorf("scan after cancel = %+v", got.Scan)
	}
}

func TestIgnoreFiles(t *testing.T) {
	e := newEnv(t)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	ep2 := e.episode("Show", 1, 2)
	pilot := e.episode("Show", 1, 1)
	ep2File := e.str(`SELECT id FROM media_files WHERE item_id = ?`, ep2)
	pilot4K := e.str(`SELECT id FROM media_files WHERE item_id = ? AND version_label = '2160p'`, pilot)

	files, deleted, err := e.svc.IgnoreFiles(e.ctx, []string{ep2File, pilot4K, "01ZZZZZZZZZZZZZZZZZZZZZZZZ"}, "unplayable")
	if err != nil || files != 2 || deleted != 1 {
		t.Fatalf("IgnoreFiles = %d files, %d items, %v; want 2, 1", files, deleted, err)
	}
	if v := e.str(`SELECT group_concat(reason, ',') FROM ignored_files`); v != "unplayable,unplayable" {
		t.Errorf("ignored_files reasons = %q", v)
	}
	if v := e.str(`SELECT (deleted_at IS NOT NULL) || '/' || available FROM items WHERE id = ?`, ep2); v != "1/0" {
		t.Errorf("episode without files: deleted/available = %s", v)
	}
	if v := e.str(`SELECT (deleted_at IS NOT NULL) || '/' || available FROM items WHERE id = ?`, pilot); v != "0/1" {
		t.Errorf("pilot with one version left: deleted/available = %s", v)
	}

	// The next full scan and a path scan skip the ignored files.
	if got := e.scan(lib.ID); got.Scan.FileCount != 9 || got.Scan.FilesSeen != 9 {
		t.Errorf("scan after ignore = %+v", got.Scan)
	}
	e.pathScan(lib, showDir+"/Season 01")
	if n := e.count(`SELECT count(*) FROM media_files`); n != 9 {
		t.Errorf("media files after rescans = %d, want 9", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'episode' AND deleted_at IS NULL`); n != 6 {
		t.Errorf("episodes = %d, want 6", n)
	}
}

func TestProbeErrorOnBrokenFile(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Broken (2024)/Broken (2024).mkv", "this is a text file, not a video\n")
	writeFile(t, dir, "Empty (2024)/Empty (2024).mkv", "")
	lib := e.create(dir)
	e.scan(lib.ID)
	empty := e.str(`SELECT id FROM media_files WHERE size = 0`)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, empty)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT probe_error FROM media_files WHERE id = ? AND probed_at IS NOT NULL`, empty); v != "empty file" {
		t.Errorf("probe_error of an empty file = %q", v)
	}
	id := e.str(`SELECT id FROM media_files WHERE size > 0`)
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
		t.Fatalf("probe of a broken file = %v, want nil (no retry)", err)
	}
	msg := e.str(`SELECT probe_error FROM media_files WHERE id = ? AND probed_at IS NOT NULL`, id)
	if msg == "" || strings.Contains(msg, dir) || !strings.Contains(msg, "Broken (2024).mkv") {
		t.Errorf("probe_error = %q, want a message with the file name and without the host path %s", msg, dir)
	}
	t.Logf("probe_error: %s", msg)
	if n := e.count(`SELECT count(*) FROM tracks WHERE media_file_id = ?`, id); n != 0 {
		t.Errorf("tracks = %d, want none", n)
	}
	if n := e.jobCount(JobProbeKeyframes); n != 0 {
		t.Errorf("keyframe jobs = %d, want none", n)
	}

	// The file is replaced by a real video: the next scan probes again and clears the error.
	src := filepath.Join(testMedia(t), "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Broken (2024)", "Broken (2024).mkv"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Minute)
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE type = ?`, JobProbeFile); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobProbeFile); n != 1 {
		t.Fatalf("probe jobs after the change = %d, want 1", n)
	}
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, id)); err != nil {
		t.Fatal(err)
	}
	if v := e.str(`SELECT coalesce(probe_error, 'none') || '/' || video_codec FROM media_files WHERE id = ?`, id); v != "none/h264" {
		t.Errorf("after the re-probe: %q, want none/h264", v)
	}
}

// M3 review, finding 17: files without an episode marker are logged once per directory and scan
// with their count, not once per file.
func TestNoEpisodeMarkerLoggedPerDirectory(t *testing.T) {
	e := newEnv(t)
	var logs bytes.Buffer
	e.svc.log = slog.New(slog.NewTextHandler(&logs, nil))
	dir := tree(t, []string{"Show (2020)/Season 01/Show - S01E01.mkv", "Show (2020)/Season 01/Recap one.mkv",
		"Show (2020)/Season 01/Recap two.mkv", "Show (2020)/Bonus.mkv"})
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	out := logs.String()
	if n := strings.Count(out, "no episode marker"); n != 2 {
		t.Errorf("%d warnings, want one per directory:\n%s", n, out)
	}
	if !strings.Contains(out, `dir="Show (2020)/Season 01" files=2`) || !strings.Contains(out, `dir="Show (2020)" files=1`) {
		t.Errorf("warnings without directory and count:\n%s", out)
	}
}

// M3 review, finding 6: the metadata job replaces the folder year with the first air date; a new
// episode in the same series folder still joins the series instead of creating a second one.
func TestSeriesFolderWinsOverChangedYear(t *testing.T) {
	e := newEnv(t)
	dir := tree(t, []string{"Show (2019)/Season 01/Show - S01E01.mkv"})
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	series := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET year = 2020, metadata_state = 'matched' WHERE id = ?`, series); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "Show (2019)/Season 01/Show - S01E02.mkv", "episode two")
	writeFile(t, dir, "Show (2019)/Season 02/Show - S02E01.mkv", "season two")
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series'`); n != 1 {
		t.Fatalf("series = %d, want 1", n)
	}
	for _, ep := range [][2]int{{1, 1}, {1, 2}, {2, 1}} {
		if id := e.episode("Show", ep[0], ep[1]); id == "" {
			t.Errorf("S%02dE%02d is not below the series", ep[0], ep[1])
		}
	}
	// A path scan of the folder finds the same series.
	writeFile(t, dir, "Show (2019)/Season 01/Show - S01E03.mkv", "episode three")
	e.pathScan(lib, "Show (2019)/Season 01")
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series'`); n != 1 {
		t.Errorf("series after the path scan = %d, want 1", n)
	}
}

func TestSeriesRelinkByProviderID(t *testing.T) {
	e := newEnv(t)
	files := []string{
		"Keep (2001)/Season 01/Keep - S01E01.mkv", "Keep (2001)/Season 01/Keep - S01E02.mkv",
		"Keep (2001)/Season 01/Keep - S01E03.mkv", "Keep (2001)/Season 01/Keep - S01E04.mkv",
		"Keep (2001)/Season 01/Keep - S01E05.mkv", "Keep (2001)/Season 01/Keep - S01E06.mkv",
		"Gone (2010) {tvdb-4242}/Season 01/Gone - S01E01.mkv",
	}
	dir := tree(t, files)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	series := e.str(`SELECT id FROM items WHERE kind = 'series' AND tvdb_id = 4242`)
	ep := e.episode("Gone", 1, 1)

	if err := os.RemoveAll(filepath.Join(dir, "Gone (2010) {tvdb-4242}")); err != nil {
		t.Fatal(err)
	}
	e.scan(lib.ID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 3 {
		t.Fatalf("sweep = %d, %v; want episode, season and series", n, err)
	}

	// Re-downloaded under another folder name and with other content within 30 days: the series
	// comes back by its TVDB id with its season and episode.
	e.clock.Advance(10 * 24 * time.Hour)
	writeFile(t, dir, "Gone {tvdb-4242}/Season 1/Gone.S01E01.1080p.WEB.x264-GRP.mkv", "new release")
	got := e.scan(lib.ID)
	if got.Scan.State != "done" {
		t.Fatalf("scan = %+v", got.Scan)
	}
	if v := e.str(`SELECT count(*) || '/' || min(deleted_at IS NULL) FROM items WHERE id IN (?, ?)`, series, ep); v != "2/1" {
		t.Errorf("relinked series and episode: %s", v)
	}
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE 'Gone {tvdb-4242}/%'`); v != ep {
		t.Errorf("new file belongs to %q, want the old episode %q", v, ep)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'series' AND deleted_at IS NULL`); n != 2 {
		t.Errorf("series = %d, want 2", n)
	}
}

func TestScanRunsMetadataCheck(t *testing.T) {
	e := newEnv(t)
	var checked []int64
	WithMetadataCheck(func(_ context.Context, libraryID int64) (int, error) {
		checked = append(checked, libraryID)
		return 2, nil
	})(e.svc)
	dir := seriesDirTree(t)
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	if len(checked) != 1 || checked[0] != lib.ID {
		t.Fatalf("checked after the full scan = %v", checked)
	}
	writeFile(t, dir, showDir+"/Season 01/Show - S01E09.mkv", "nine")
	e.pathScan(lib, showDir+"/Season 01")
	if len(checked) != 1 {
		t.Errorf("path scan ran the metadata check: %v", checked)
	}
}
