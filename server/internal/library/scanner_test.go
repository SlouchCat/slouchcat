package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// twentyMovies are Plex/Radarr/Jellyfin style paths of 20 different titles.
var twentyMovies = []string{
	"Alien (1979)/Alien (1979).mkv",
	"Aliens (1986) [tmdb-679]/Aliens (1986) - 1080p.mkv",
	"Blade Runner (1982) {tmdb-78}/Blade Runner (1982) {edition-Final Cut}.mkv",
	"Heat (1995) [imdb-tt0113277]/Heat (1995) [imdb-tt0113277] - [Bluray-1080p][DTS 5.1][x264]-GROUP.mkv",
	"The Matrix (1999)/The Matrix (1999) - 2160p.mkv",
	"Inception (2010)/Inception (2010).mp4",
	"Das Boot (1981)/Das Boot (1981).mkv",
	"Lola rennt (1998)/Lola rennt (1998).avi",
	"Amélie (2001) [tmdb-194]/Amélie (2001) [tmdb-194] - 1080p.mkv",
	"Schindler's List (1993)/Schindler's List (1993).mkv",
	"Fast & Furious (2009)/Fast & Furious (2009).mkv",
	"Good Bye, Lenin! (2003)/Good Bye, Lenin! (2003).m4v",
	"Ford v Ferrari (2019) [imdb-tt1950186]/Ford v Ferrari (2019) [imdb-tt1950186] - [German DL][Bluray-1080p][DTS 5.1][x264]-COiNCiDENCE.mkv",
	"13 Minutes (2021) [tmdb-787723]/13 Minutes (2021) [imdb-tt9274670] - [Bluray-1080p][AC3 5.1][x265]-omikron.mkv",
	"Dune (2021)/Dune (2021) - Part 1.mkv",
	"Heidi (2015)/Heidi (2015).ts",
	"Parasite.2019.1080p.BluRay.x264-GROUP/Parasite.2019.1080p.BluRay.x264-GROUP.mkv",
	"Toni Erdmann (2016)/Toni Erdmann (2016).webm",
	"Casablanca/Casablanca.mkv",
	"Midsommar (2019).mkv",
}

// tree writes files with distinct content (so partial hashes differ) into a new temp dir.
func tree(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	for i, f := range files {
		writeFile(t, dir, f, fmt.Sprintf("video %d %s", i, f))
	}
	return dir
}

func TestScanCreatesItemsAndConverges(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies)
	lib := e.create(dir)
	if lib.Scan.State != "queued" || e.jobCount(JobScanLibrary) != 1 {
		t.Fatalf("after create: state %q, scan jobs %d", lib.Scan.State, e.jobCount(JobScanLibrary))
	}
	if len(lib.Roots) != 1 || lib.Roots[0].Watch == "" || lib.Roots[0].FSType == "" {
		t.Fatalf("roots = %+v", lib.Roots)
	}

	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.LastError != "" {
		t.Fatalf("scan state %q (%q)", got.Scan.State, got.Scan.LastError)
	}
	if got.Scan.ItemCount != 20 || got.Scan.FileCount != 20 || got.Scan.FilesSeen != 20 || got.Scan.FilesTotal != 20 {
		t.Fatalf("scan = %+v, want 20 items, 20 files, 20/20 seen", got.Scan)
	}
	if n := e.jobCount(JobProbeFile); n != 20 {
		t.Errorf("probe.file jobs = %d, want 20", n)
	}
	if n := e.jobCount(JobMetadataMatch); n != 20 {
		t.Errorf("metadata.match jobs = %d, want 20", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE available = 1 AND metadata_state = 'pending'`); n != 20 {
		t.Errorf("available pending items = %d, want 20", n)
	}
	checks := map[string]string{
		`SELECT title FROM items WHERE tmdb_id = 679`:                                               "Aliens",
		`SELECT imdb_id FROM items WHERE title = 'Heat'`:                                            "tt0113277",
		`SELECT sort_title FROM items WHERE title = 'The Matrix'`:                                   "matrix",
		`SELECT version_label FROM media_files WHERE rel_path LIKE 'The Matrix%'`:                   "2160p",
		`SELECT version_label FROM media_files WHERE rel_path LIKE 'Blade Runner%'`:                 "Final Cut",
		`SELECT container FROM media_files WHERE rel_path LIKE 'Heidi%'`:                            "ts",
		`SELECT title FROM items WHERE tmdb_id = 787723 AND imdb_id = 'tt9274670'`:                  "13 Minutes",
		`SELECT title FROM items WHERE title LIKE 'Parasite%'`:                                      "Parasite",
		`SELECT count(*) FROM media_files WHERE partial_hash IS NULL OR length(partial_hash) <> 64`: "0",
		`SELECT count(DISTINCT partial_hash) FROM media_files`:                                      "20",
	}
	for q, want := range checks {
		if v := e.str(q); v != want {
			t.Errorf("%s = %q, want %q", q, v, want)
		}
	}

	// A second scan of the unchanged tree writes nothing and queues nothing.
	e.clock.Advance(time.Hour)
	before := e.count(`SELECT max(updated_at) FROM media_files`) + e.count(`SELECT max(updated_at) FROM items`)
	jobsBefore := e.count(`SELECT count(*) FROM jobs`)
	got = e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.ItemCount != 20 || got.Scan.FileCount != 20 {
		t.Fatalf("second scan = %+v", got.Scan)
	}
	if after := e.count(`SELECT max(updated_at) FROM media_files`) + e.count(`SELECT max(updated_at) FROM items`); after != before {
		t.Error("second scan changed rows")
	}
	if n := e.count(`SELECT count(*) FROM jobs`); n != jobsBefore {
		t.Errorf("second scan queued %d jobs", n-jobsBefore)
	}
	if n := e.count(`SELECT count(*) FROM items`); n != 20 {
		t.Errorf("items = %d after second scan", n)
	}
}

func TestScanRenameKeepsItemAndProgress(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:5])
	lib := e.create(dir)
	e.scan(lib.ID)
	itemID := e.str(`SELECT id FROM items WHERE title = 'Alien'`)
	fileID := e.str(`SELECT id FROM media_files WHERE item_id = ?`, itemID)

	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'Marco', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := appq.New(e.db.Write).UpsertProgress(e.ctx, appq.UpsertProgressParams{
		ProfileID: 1, ItemID: itemID, PositionMs: 600_000, DurationMs: 7_000_000, Now: 1,
	}); err != nil {
		t.Fatal(err)
	}
	jobsBefore := e.count(`SELECT count(*) FROM jobs`)

	// Rename folder and file (same inode).
	rename(t, dir, "Alien (1979)/Alien (1979).mkv", "Alien (1979) [tmdb-348]/Alien (1979) [tmdb-348] - 1080p.mkv")
	if err := os.Remove(filepath.Join(dir, "Alien (1979)")); err != nil {
		t.Fatal(err)
	}
	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.ItemCount != 5 || got.Scan.FileCount != 5 {
		t.Fatalf("scan after rename = %+v", got.Scan)
	}
	if id := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, "Alien (1979) [tmdb-348]/Alien (1979) [tmdb-348] - 1080p.mkv"); id != fileID {
		t.Fatalf("renamed file has id %q, want %q", id, fileID)
	}
	if v := e.str(`SELECT item_id FROM media_files WHERE id = ?`, fileID); v != itemID {
		t.Errorf("item changed to %q", v)
	}
	if v := e.str(`SELECT version_label FROM media_files WHERE id = ?`, fileID); v != "1080p" {
		t.Errorf("version label = %q, want 1080p", v)
	}
	if n := e.count(`SELECT count(*) FROM progress AS p JOIN items AS i ON i.id = p.item_id
	                 WHERE i.deleted_at IS NULL AND i.available = 1 AND p.position_ms = 600000`); n != 1 {
		t.Errorf("progress rows on the available item = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("%d files missing after rename", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs`); n != jobsBefore {
		t.Errorf("rename queued %d jobs, want none (content unchanged)", n-jobsBefore)
	}
}

func TestScanMovedAcrossDevicesMatchesByHash(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:5])
	lib := e.create(dir)
	e.scan(lib.ID)
	fileID := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, twentyMovies[0])

	// Copy to a new path and delete the original: a new inode with the same content.
	data, err := os.ReadFile(filepath.Join(dir, twentyMovies[0]))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "Moved/Alien (1979).mkv", string(data))
	remove(t, dir, twentyMovies[0])
	e.scan(lib.ID)
	if id := e.str(`SELECT id FROM media_files WHERE rel_path = 'Moved/Alien (1979).mkv'`); id != fileID {
		t.Fatalf("moved file id %q, want %q", id, fileID)
	}
	if n := e.count(`SELECT count(*) FROM media_files`); n != 5 {
		t.Errorf("media files = %d, want 5", n)
	}
}

func TestLossProtection(t *testing.T) {
	t.Parallel()
	ten := twentyMovies[:10]
	missing := func(e *env) int { return e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`) }

	t.Run("root missing", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		got := e.scan(lib.ID)
		if got.Scan.State != "aborted" || got.Scan.LastError != abortRootMissing {
			t.Fatalf("scan = %q/%q, want aborted/root_missing", got.Scan.State, got.Scan.LastError)
		}
		if missing(e) != 0 || got.Scan.FileCount != 10 || got.Scan.ItemCount != 10 {
			t.Errorf("rows changed: missing %d, scan %+v", missing(e), got.Scan)
		}
		// Accepting the loss never applies a missing root.
		if _, err := e.svc.AcceptLoss(e.ctx, lib.ID); err != nil {
			t.Fatal(err)
		}
		if got := e.scan(lib.ID); got.Scan.State != "aborted" || missing(e) != 0 {
			t.Errorf("accepted loss applied to a missing root: %+v", got.Scan)
		}
	})

	t.Run("root empty", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		for _, f := range ten {
			remove(t, dir, f)
		}
		got := e.scan(lib.ID)
		if got.Scan.State != "aborted" || got.Scan.LastError != abortRootEmpty || missing(e) != 0 {
			t.Fatalf("scan = %q/%q, missing %d; want aborted/root_empty, 0", got.Scan.State, got.Scan.LastError, missing(e))
		}
	})

	t.Run("30 percent lost, then accepted", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		for _, f := range ten[:3] {
			remove(t, dir, f)
		}
		got := e.scan(lib.ID)
		if got.Scan.State != "aborted" || got.Scan.LastError != abortLoss {
			t.Fatalf("scan = %q/%q, want aborted/loss_over_threshold", got.Scan.State, got.Scan.LastError)
		}
		if missing(e) != 0 || got.Scan.FileCount != 10 {
			t.Fatalf("rows changed by an aborted scan: missing %d, files %d", missing(e), got.Scan.FileCount)
		}

		accepted, err := e.svc.AcceptLoss(e.ctx, lib.ID)
		if err != nil {
			t.Fatal(err)
		}
		if accepted.Scan.LossAcceptedAt == nil || accepted.Scan.State != "queued" {
			t.Fatalf("after AcceptLoss: %+v", accepted.Scan)
		}
		got = e.scan(lib.ID)
		if got.Scan.State != "done" || missing(e) != 3 || got.Scan.FileCount != 7 {
			t.Fatalf("accepted scan = %+v, missing %d; want done, 3 missing, 7 files", got.Scan, missing(e))
		}
		if got.Scan.LossAcceptedAt != nil {
			t.Error("loss_accepted_at not cleared")
		}
		// Items disappear from lists (available = 0) but stay until the sweep.
		if n := e.count(`SELECT count(*) FROM items WHERE available = 0 AND deleted_at IS NULL`); n != 3 {
			t.Errorf("unavailable items = %d, want 3", n)
		}
	})

	t.Run("10 percent lost is applied", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		remove(t, dir, ten[4])
		got := e.scan(lib.ID)
		if got.Scan.State != "done" || missing(e) != 1 || got.Scan.FileCount != 9 {
			t.Fatalf("scan = %+v, missing %d; want done with 1 missing", got.Scan, missing(e))
		}
	})
}

func TestMissingFileReappearsAndSweep(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ten := twentyMovies[:10]
	dir := tree(t, ten)
	lib := e.create(dir)
	e.scan(lib.ID)
	content := func(i int) string { return fmt.Sprintf("video %d %s", i, ten[i]) }

	// File 0 goes missing and comes back within 24 h.
	remove(t, dir, ten[0])
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`); n != 1 {
		t.Fatalf("missing = %d, want 1", n)
	}
	first := e.count(`SELECT missing_since FROM media_files WHERE rel_path = ?`, ten[0])
	e.clock.Advance(time.Hour)
	e.scan(lib.ID) // still missing: the first timestamp stays
	if v := e.count(`SELECT missing_since FROM media_files WHERE rel_path = ?`, ten[0]); v != first {
		t.Errorf("missing_since moved from %d to %d", first, v)
	}
	writeFile(t, dir, ten[0], content(0))
	e.clock.Advance(time.Hour)
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`); n != 0 {
		t.Fatalf("missing = %d after reappearance, want 0", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE available = 1`); n != 10 {
		t.Errorf("available items = %d, want 10", n)
	}

	// File 1 stays missing; the sweep 25 h later soft-deletes its item, and only that one.
	remove(t, dir, ten[1])
	e.scan(lib.ID)
	itemID := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, ten[1])
	e.clock.Advance(23 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 0 {
		t.Fatalf("sweep after 23 h deleted %d (%v), want 0", n, err)
	}
	e.clock.Advance(2 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		t.Fatalf("sweep after 25 h deleted %d (%v), want 1", n, err)
	}
	if e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NOT NULL`, itemID) != 1 {
		t.Error("item of the missing file not soft-deleted")
	}
	got, err := e.svc.Get(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scan.ItemCount != 9 {
		t.Errorf("item count = %d, want 9", got.Scan.ItemCount)
	}

	// The file comes back after the soft delete: the item returns with its id.
	writeFile(t, dir, ten[1], content(1))
	e.scan(lib.ID)
	if e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL AND available = 1`, itemID) != 1 {
		t.Error("item not restored after its file reappeared")
	}
}

func TestIdentityByProviderIDs(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := append([]string{"Heat (1995) [tmdb-949]/Heat (1995).mkv"}, twentyMovies[4:10]...)
	dir := tree(t, files)
	lib := e.create(dir)
	e.scan(lib.ID)
	itemID := e.str(`SELECT id FROM items WHERE tmdb_id = 949`)

	// The folder is renamed and the file replaced (new inode, new content): the [tmdb-…] id in the
	// new name finds the same item.
	remove(t, dir, files[0])
	if err := os.Remove(filepath.Join(dir, "Heat (1995) [tmdb-949]")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "Heat Directors Cut [tmdb-949]/Heat.mkv", "a different encode")
	e.scan(lib.ID)
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat Directors Cut [tmdb-949]/Heat.mkv'`); v != itemID {
		t.Fatalf("new file joined item %q, want %q", v, itemID)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL AND available = 1 AND id = ?`, itemID); n != 1 {
		t.Error("item not available")
	}

	// A folder without ids but with a movie.nfo carrying the TMDB id: same item, second version.
	writeFile(t, dir, "Some Other Name/Some Other Name.mkv", "third encode")
	writeFile(t, dir, "Some Other Name/movie.nfo", `<?xml version="1.0" encoding="utf-8"?>
<movie><title>Heat</title><year>1995</year><tmdbid>949</tmdbid><mpaa>FSK 16</mpaa></movie>`)
	e.scan(lib.ID)
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Some Other Name/Some Other Name.mkv'`); v != itemID {
		t.Fatalf("NFO file joined item %q, want %q", v, itemID)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE deleted_at IS NULL`); n != 7 {
		t.Errorf("items = %d, want 7", n)
	}

	// An NFO on a new title sets its own rating from the FSK certification.
	writeFile(t, dir, "Neuer Film (2024)/Neuer Film (2024).mkv", "fourth")
	writeFile(t, dir, "Neuer Film (2024)/Neuer Film (2024).nfo", `<movie><originaltitle>New Film</originaltitle><mpaa>FSK-12</mpaa><uniqueid type="imdb">tt7654321</uniqueid></movie>`)
	e.scan(lib.ID)
	if v := e.str(`SELECT own_rating || '/' || rating_source || '/' || effective_rating || '/' || imdb_id || '/' || original_title FROM items WHERE title = 'Neuer Film'`); v != "12/nfo/12/tt7654321/New Film" {
		t.Errorf("NFO fields = %q", v)
	}
}

func TestExtrasBecomeExtraItemsAndSamplesAreSkipped(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{
		"Alien (1979)/Alien (1979).mkv",
		"Alien (1979)/Alien (1979)-trailer.mkv",
		"Alien (1979)/Trailers/Alien Official Trailer.mkv",
		"Alien (1979)/Featurettes/Featurette 1.mkv",
		"Alien (1979)/Behind The Scenes/Making Of.mkv",
		"Alien (1979)/Deleted Scenes/Facehugger.mkv",
		"Alien (1979)/Alien (1979)-sample.mkv",
		"Alien (1979)/Alien (1979).de.srt",
		"Alien (1979)/movie.nfo",
		"Alien (1979)/folder.jpg",
		"Alien (1979)/Alien (1979)-320-10.bif",
		"Parasite.2019.1080p.BluRay.x264-GROUP/Sample/parasite.sample.mkv",
		"Parasite.2019.1080p.BluRay.x264-GROUP/Parasite.2019.1080p.BluRay.x264-GROUP.mkv",
		".hidden/Hidden (2000).mkv",
		"@eaDir/Alien (1979).mkv/SYNOVIDEO_VIDEO_SCREENSHOT.jpg",
		"#recycle/Old (1990)/Old (1990).mkv",
		"Trailers/Orphan Trailer.mkv",
		"Orphan (2009)-trailer.mkv",
	})
	lib := e.create(dir)
	got := e.scan(lib.ID)
	// 2 movies and 5 extras of Alien; samples, hidden entries and extras without a movie folder
	// are skipped. The top-level "Trailers" folder is a category, so its file is a movie.
	if got.Scan.ItemCount != 8 || got.Scan.FileCount != 8 {
		t.Fatalf("scan = %+v, want 8 items and 8 files", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'movie' AND title IN ('Alien', 'Parasite', 'Orphan Trailer')`); n != 3 {
		t.Errorf("movies wrong: %d", n)
	}
	alien := e.str(`SELECT id FROM items WHERE kind = 'movie' AND title = 'Alien'`)
	if v := e.str(`SELECT group_concat(extra_kind || ':' || title, ',') FROM (SELECT extra_kind, title FROM items
	               WHERE kind = 'extra' AND parent_id = ? ORDER BY extra_kind, title)`, alien); v !=
		"behind_the_scenes:Making Of,deleted_scene:Facehugger,featurette:Featurette 1,trailer:Alien (1979),trailer:Alien Official Trailer" {
		t.Errorf("extras = %q", v)
	}
	// Extras get media files and probes, no metadata job; they inherit the rating of the movie.
	if n := e.jobCount(JobMetadataMatch); n != 3 {
		t.Errorf("metadata.match jobs = %d, want 3 (movies only)", n)
	}
	if n := e.jobCount(JobProbeFile); n != 8 {
		t.Errorf("probe.file jobs = %d, want 8", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'extra' AND available = 1 AND metadata_state = 'pending'`); n != 5 {
		t.Errorf("available extras = %d, want 5", n)
	}
	// A second scan finds every extra by its path and writes nothing.
	e.clock.Advance(time.Hour)
	since := e.clock.Now()
	e.scan(lib.ID)
	if items, files, jobs := e.touched(since); items+files+jobs != 0 {
		t.Errorf("second scan touched %d items, %d files, %d jobs", items, files, jobs)
	}
}

// M3 review, finding 2: a new extra never attaches to a soft-deleted film, also when the film has
// no provider id (it is not reachable for a relink then, but still owns its folder).
func TestExtraNeverAttachesToDeletedFilm(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:10]) // Alien first; one lost file stays below the loss threshold
	lib := e.create(dir)
	e.scan(lib.ID)
	alien := e.str(`SELECT id FROM items WHERE kind = 'movie' AND title = 'Alien'`)
	remove(t, dir, "Alien (1979)/Alien (1979).mkv")
	e.scan(lib.ID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want the film", n, err)
	}
	writeFile(t, dir, "Alien (1979)/Trailers/Alien Trailer.mkv", "trailer")
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'extra'`); n != 0 {
		t.Errorf("extras = %d, want none below the soft-deleted film", n)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NOT NULL`, alien); n != 1 {
		t.Error("the soft-deleted film came back without a file of its own")
	}
}

// M3 review, finding 19: a returning extra file restores the extra but not its soft-deleted film
// while the film's own file is still missing; the film's file brings the film back.
func TestReturningExtraKeepsDeletedFilmDeleted(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	extraRel := "Alien (1979)/Trailers/Alien Trailer.mkv"
	dir := tree(t, append(slices.Clone(twentyMovies[:10]), extraRel))
	lib := e.create(dir)
	e.scan(lib.ID)
	film := e.str(`SELECT id FROM items WHERE kind = 'movie' AND title = 'Alien'`)
	extra := e.str(`SELECT id FROM items WHERE kind = 'extra'`)
	away := t.TempDir()
	for _, rel := range []string{twentyMovies[0], extraRel} {
		if err := os.Rename(filepath.Join(dir, filepath.FromSlash(rel)), filepath.Join(away, filepath.Base(rel))); err != nil {
			t.Fatal(err)
		}
	}
	e.scan(lib.ID)
	e.clock.Advance(25 * time.Hour)
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 2 {
		t.Fatalf("sweep = %d, %v; want the film and its extra", n, err)
	}
	back := func(rel string) {
		t.Helper()
		if err := os.Rename(filepath.Join(away, filepath.Base(rel)), filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		e.scan(lib.ID)
	}
	back(extraRel)
	if v := e.str(`SELECT group_concat(id || ':' || (deleted_at IS NULL), ',') FROM (SELECT id, deleted_at FROM items WHERE id IN (?, ?) ORDER BY id = ?)`,
		film, extra, film); v != extra+":1,"+film+":0" {
		t.Errorf("after the extra returned: %s, want the extra restored and the film still deleted", v)
	}
	back(twentyMovies[0])
	if n := e.count(`SELECT count(*) FROM items WHERE id IN (?, ?) AND deleted_at IS NULL`, film, extra); n != 2 {
		t.Errorf("after the film returned: %d restored, want 2", n)
	}
}

// M3 review, finding 16: an extra without a parent title (two films share its folder) is skipped
// once; unchanged rescans neither hash nor write it, and a scan with other changes tries again.
func TestOrphanExtraIsNotReplannedByUnchangedRescans(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Double/Film A (2001).mkv", "Double/Film B (2002).mkv", "Double/Trailers/Trailer.mkv"})
	lib := e.create(dir)
	e.scan(lib.ID)
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'extra'`); n != 0 {
		t.Fatalf("extras = %d, want none (no single parent)", n)
	}
	e.svc.TakeWriteStats()
	got := e.scan(lib.ID)
	if st := e.svc.TakeWriteStats(); st.Batches != 0 || st.Ops != 0 {
		t.Errorf("unchanged rescan wrote %+v, want nothing", st)
	}
	if got.Scan.State != "done" {
		t.Errorf("rescan = %+v", got.Scan)
	}
	// Another change in the library: the extra is planned again (and skipped again).
	writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "heat")
	e.scan(lib.ID)
	if st := e.svc.TakeWriteStats(); st.Ops != 2 {
		t.Errorf("scan with a new film wrote %d operations, want 2 (the film and the retried extra)", st.Ops)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'extra'`); n != 0 {
		t.Errorf("extras = %d, want none", n)
	}
	e.scan(lib.ID)
	if st := e.svc.TakeWriteStats(); st.Batches != 0 {
		t.Errorf("unchanged rescan after the retry wrote %+v", st)
	}
}

func TestSecondVersionAttachesToSameItem(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Avatar (2009)/Avatar (2009) - 1080p.mkv", "Avatar (2009)/Avatar (2009) - 2160p.mkv"})
	lib := e.create(dir)
	got := e.scan(lib.ID)
	if got.Scan.ItemCount != 1 || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v, want 1 item with 2 files", got.Scan)
	}
	if n := e.jobCount(JobMetadataMatch); n != 1 {
		t.Errorf("metadata.match jobs = %d, want 1", n)
	}
}

func TestScanRunsImageCheck(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var checked []int64
	fail := false
	WithImageCheck(func(_ context.Context, libraryID int64) (int, error) {
		checked = append(checked, libraryID)
		if fail {
			return 0, errors.New("cache unreadable")
		}
		return 1, nil
	})(e.svc)
	dir := tree(t, twentyMovies[:2])
	lib := e.create(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" || !slices.Equal(checked, []int64{lib.ID}) {
		t.Fatalf("after scan: state %q, checked %v", got.Scan.State, checked)
	}
	// A failing check is logged; the scan stays done.
	fail = true
	if got := e.scan(lib.ID); got.Scan.State != "done" || len(checked) != 2 {
		t.Fatalf("failing check: state %q, checked %v", got.Scan.State, checked)
	}
	// An aborted scan changes nothing and checks nothing.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if got := e.scan(lib.ID); got.Scan.State != "aborted" || len(checked) != 2 {
		t.Fatalf("aborted scan: state %q, checked %v", got.Scan.State, checked)
	}
}

func TestLibraryAdministration(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:2])

	file := filepath.Join(dir, twentyMovies[0])
	for _, p := range []string{"", "relative/path", filepath.Join(dir, "missing"), file} {
		if err := e.svc.ValidatePath(e.ctx, p); !errors.Is(err, model.ErrLibraryPathInvalid) {
			t.Errorf("ValidatePath(%q) = %v, want ErrLibraryPathInvalid", p, err)
		}
	}
	if err := e.svc.ValidatePath(e.ctx, dir); err != nil {
		t.Errorf("ValidatePath(dir) = %v", err)
	}

	var verr *model.ValidationError
	if _, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: " ", Kind: "music", Roots: rootReq(dir, model.RootMovie)}); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Errorf("Create with bad name and kind = %v", err)
	}
	lib := e.create(dir)
	if _, err := e.svc.Create(e.ctx, testRequest("Again", dir)); !hasFieldError(err, "roots[0].path", model.CodeRootOverlap) {
		t.Errorf("second library on the same root = %v, want roots[0].path root_overlap", err)
	}
	if _, err := e.svc.Create(e.ctx, testRequest("Nested", filepath.Join(dir, "Alien (1979)"))); !hasFieldError(err, "roots[0].path", model.CodeRootOverlap) {
		t.Errorf("nested root = %v, want roots[0].path root_overlap", err)
	}
	if _, err := e.svc.Get(e.ctx, 999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Get(999) = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.QueueScan(e.ctx, 999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("QueueScan(999) = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.AcceptLoss(e.ctx, 999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("AcceptLoss(999) = %v, want ErrNotFound", err)
	}

	// QueueScan is idempotent while the scan is queued.
	if _, err := e.svc.QueueScan(e.ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobScanLibrary); n != 1 {
		t.Errorf("scan jobs = %d, want 1", n)
	}
	e.scan(lib.ID)
	libs, err := e.svc.List(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 || libs[0].Name != "Filme" || libs[0].Scan.State != "done" || libs[0].Scan.ItemCount != 2 ||
		libs[0].Scan.StartedAt == nil || libs[0].Scan.FinishedAt == nil {
		t.Errorf("List = %+v", libs)
	}
}

func TestWatchMode(t *testing.T) {
	t.Parallel()
	cases := map[string][2]string{
		"ext4": {"inotify", ""}, "xfs": {"inotify", ""}, "btrfs": {"inotify", ""}, "zfs": {"inotify", ""},
		"nfs": {"poll", "network_share"}, "smb": {"poll", "network_share"},
		"fuse": {"inotify_poll", "fuse_unknown"}, "unknown": {"inotify_poll", "fuse_unknown"},
	}
	for fs, want := range cases {
		if w, warn := WatchMode(fs); w != want[0] || warn != want[1] {
			t.Errorf("WatchMode(%s) = %s, %s; want %v", fs, w, warn, want)
		}
	}
}

// A symlink to a file inside the root is followed and counted; one whose target leaves the root
// is skipped with a warning and the scan still ends done.
func TestScanSkipsSymlinkLeavingRoot(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var logs bytes.Buffer
	e.svc.log = slog.New(slog.NewTextHandler(&logs, nil))
	dir := tree(t, []string{"Real (2000)/Real (2000).mkv"})
	outside := t.TempDir()
	writeFile(t, outside, "Outside (2002).mkv", "outside the root")
	if err := os.Symlink(filepath.Join("Real (2000)", "Real (2000).mkv"), filepath.Join(dir, "Link (2001).mkv")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "Outside (2002).mkv"), filepath.Join(dir, "Outside (2002).mkv")); err != nil {
		t.Fatal(err)
	}
	lib := e.create(dir)
	got := e.scan(lib.ID)
	if got.Scan.State != "done" || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v, want done with 2 files", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE rel_path = 'Link (2001).mkv'`); n != 1 {
		t.Error("symlink inside the root not followed")
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE rel_path LIKE 'Outside%'`); n != 0 {
		t.Error("symlink leaving the root was followed")
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, `path="Outside (2002).mkv"`) {
		t.Errorf("no warning for the skipped symlink: %s", out)
	}
}

// The sweep deletes a title only when every one of its files has been missing for 24 h.
func TestSweepNeedsAllFilesMissingForGrace(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := append([]string{"Avatar (2009)/Avatar (2009) - 1080p.mkv", "Avatar (2009)/Avatar (2009) - 2160p.mkv"}, twentyMovies[:9]...)
	dir := tree(t, files)
	lib := e.create(dir)
	e.scan(lib.ID)
	itemID := e.str(`SELECT item_id FROM media_files WHERE rel_path = ?`, files[0])

	remove(t, dir, files[0])
	e.scan(lib.ID)
	e.clock.Advance(24 * time.Hour)
	remove(t, dir, files[1])
	e.scan(lib.ID)
	e.clock.Advance(time.Hour) // first file missing for 25 h, second for 1 h
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 0 {
		t.Fatalf("sweep with one file missing for 1 h deleted %d (%v), want 0", n, err)
	}
	e.clock.Advance(23*time.Hour + time.Minute) // both missing for at least 24 h
	if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
		t.Fatalf("sweep with both files missing for 24 h deleted %d (%v), want 1", n, err)
	}
	if e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NOT NULL`, itemID) != 1 {
		t.Error("title with all files missing not soft-deleted")
	}
}

// A title soft-deleted less than 30 days ago comes back with its id and progress when a new file
// (different content) with the same TMDB id appears; after 30 days a new title is created.
func TestRelinkDeletedTitleByProviderID(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := append([]string{"Heat (1995) [tmdb-949]/Heat (1995).mkv"}, twentyMovies[4:10]...)
	dir := tree(t, files)
	lib := e.create(dir)
	e.scan(lib.ID)
	itemID := e.str(`SELECT id FROM items WHERE tmdb_id = 949`)
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO accounts (id, email, password_hash, created_at, updated_at) VALUES (1, 'a@b.c', 'x', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO profiles (id, account_id, name, created_at, updated_at) VALUES (1, 1, 'Marco', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := appq.New(e.db.Write).UpsertProgress(e.ctx, appq.UpsertProgressParams{
		ProfileID: 1, ItemID: itemID, PositionMs: 600_000, DurationMs: 7_000_000, Now: 1,
	}); err != nil {
		t.Fatal(err)
	}

	gone := func(rel string) {
		t.Helper()
		remove(t, dir, rel)
		e.scan(lib.ID)
		e.clock.Advance(25 * time.Hour)
		if n, err := e.svc.Sweep(e.ctx); err != nil || n != 1 {
			t.Fatalf("sweep deleted %d (%v), want 1", n, err)
		}
	}
	gone(files[0])
	writeFile(t, dir, "Heat (1995) [tmdb-949]/Heat (1995) - 2160p.mkv", "a new download")
	got := e.scan(lib.ID)
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995) [tmdb-949]/Heat (1995) - 2160p.mkv'`); v != itemID {
		t.Fatalf("new file joined item %q, want the deleted item %q", v, itemID)
	}
	if e.count(`SELECT count(*) FROM items WHERE id = ? AND deleted_at IS NULL AND available = 1`, itemID) != 1 {
		t.Error("deleted item not restored")
	}
	if got.Scan.ItemCount != 7 {
		t.Errorf("item count = %d, want 7 (no new item)", got.Scan.ItemCount)
	}
	if n := e.count(`SELECT count(*) FROM progress WHERE item_id = ? AND position_ms = 600000`, itemID); n != 1 {
		t.Errorf("progress rows = %d, want 1", n)
	}

	// Deleted again and back only after the relink window: a new title.
	gone("Heat (1995) [tmdb-949]/Heat (1995) - 2160p.mkv")
	e.clock.Advance(31 * 24 * time.Hour)
	writeFile(t, dir, "Heat (1995) [tmdb-949]/Heat (1995) - 720p.mkv", "yet another download")
	e.scan(lib.ID)
	if v := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Heat (1995) [tmdb-949]/Heat (1995) - 720p.mkv'`); v == itemID {
		t.Error("file joined a title deleted more than 30 days ago")
	}
}

// Without a year, equal titles only merge within the same movie folder.
func TestTitleWithoutYearMergesOnlyInSameFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, []string{"Horror/Halloween/Halloween.mkv", "Klassiker/Halloween/Halloween.mkv"})
	lib := e.create(dir)
	got := e.scan(lib.ID)
	if got.Scan.ItemCount != 2 || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v, want 2 items with one file each", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE title = 'Halloween' AND year IS NULL`); n != 2 {
		t.Fatalf("Halloween items = %d, want 2", n)
	}

	// A second file in the same folder (a later scan) joins that folder's item.
	writeFile(t, dir, "Horror/Halloween/Halloween.mp4", "second version")
	got = e.scan(lib.ID)
	if got.Scan.ItemCount != 2 || got.Scan.FileCount != 3 {
		t.Fatalf("scan = %+v, want 2 items with 3 files", got.Scan)
	}
	if a, b := e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Horror/Halloween/Halloween.mkv'`),
		e.str(`SELECT item_id FROM media_files WHERE rel_path = 'Horror/Halloween/Halloween.mp4'`); a != b {
		t.Errorf("same-folder files on items %q and %q", a, b)
	}
}

// pathScan runs a path scan of dirs in the first root of the library and returns its state.
func (e *env) pathScan(lib model.Library, dirs ...string) model.Library {
	e.t.Helper()
	if err := e.svc.ScanPaths(e.ctx, lib.ID, []pathScanRoot{{RootID: lib.Roots[0].ID, Dirs: dirs}}); err != nil {
		e.t.Fatalf("path scan: %v", err)
	}
	m, err := e.svc.Get(e.ctx, lib.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

// touched counts the items, media files and jobs written at or after since.
func (e *env) touched(since time.Time) (items, files, jobs int) {
	e.t.Helper()
	ms := since.UnixMilli()
	return e.count(`SELECT count(*) FROM items WHERE updated_at >= ?`, ms),
		e.count(`SELECT count(*) FROM media_files WHERE updated_at >= ?`, ms),
		e.count(`SELECT count(*) FROM jobs WHERE created_at >= ?`, ms)
}

func TestPathScanNewFileTouchesOnlyItsFolder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:10])
	lib := e.create(dir)
	e.scan(lib.ID)
	e.clock.Advance(time.Hour)
	since := e.clock.Now()

	writeFile(t, dir, "Up (2009)/Up (2009).mkv", "up")
	got := e.pathScan(lib, ".", "Up (2009)")
	if got.Scan.State != "done" || got.Scan.ItemCount != 11 || got.Scan.FileCount != 11 {
		t.Fatalf("path scan = %+v, want done with 11 items and files", got.Scan)
	}
	// The counts of the last full scan stay; the path scan does not replace them.
	if got.Scan.FilesTotal != 10 || got.Scan.FilesSeen != 10 {
		t.Errorf("scan progress = %d/%d, want the full scan's 10/10", got.Scan.FilesSeen, got.Scan.FilesTotal)
	}
	fileID := e.str(`SELECT id FROM media_files WHERE rel_path = 'Up (2009)/Up (2009).mkv'`)
	itemID := e.str(`SELECT item_id FROM media_files WHERE id = ?`, fileID)
	if e.str(`SELECT title || ' ' || year FROM items WHERE id = ?`, itemID) != "Up 2009" {
		t.Errorf("new item = %q", e.str(`SELECT title || ' ' || year FROM items WHERE id = ?`, itemID))
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobProbeFile, "probe:"+fileID); n != 1 {
		t.Errorf("probe.file jobs for the new file = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobMetadataMatch, "match:"+itemID); n != 1 {
		t.Errorf("metadata.match jobs for the new item = %d, want 1", n)
	}
	if items, files, jobs := e.touched(since); items != 1 || files != 1 || jobs != 2 {
		t.Errorf("rows written: %d items, %d media files, %d jobs; want 1, 1, 2 (only the new title)", items, files, jobs)
	}

	// The same path scan again converges: nothing written.
	e.clock.Advance(time.Hour)
	since = e.clock.Now()
	if got := e.pathScan(lib, ".", "Up (2009)"); got.Scan.State != "done" || got.Scan.ItemCount != 11 {
		t.Fatalf("second path scan = %+v", got.Scan)
	}
	if items, files, jobs := e.touched(since); items != 0 || files != 0 || jobs != 0 {
		t.Errorf("second path scan wrote %d items, %d media files, %d jobs", items, files, jobs)
	}
}

func TestPathScanRenameKeepsFile(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:10])
	lib := e.create(dir)
	e.scan(lib.ID)
	fileID := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, twentyMovies[0])
	itemID := e.str(`SELECT item_id FROM media_files WHERE id = ?`, fileID)
	jobsBefore := e.count(`SELECT count(*) FROM jobs`)

	// Inside the folder.
	rename(t, dir, twentyMovies[0], "Alien (1979)/Alien (1979) - 1080p.mkv")
	e.pathScan(lib, "Alien (1979)")
	if v := e.str(`SELECT id || ' ' || item_id || ' ' || version_label FROM media_files WHERE rel_path = 'Alien (1979)/Alien (1979) - 1080p.mkv'`); v != fileID+" "+itemID+" 1080p" {
		t.Fatalf("renamed file = %q, want the same file and item", v)
	}

	// Into another folder; the scan names only the new folder (its events came first).
	rename(t, dir, "Alien (1979)/Alien (1979) - 1080p.mkv", "Alien Directors Cut (1979)/Alien (1979).mkv")
	e.pathScan(lib, "Alien Directors Cut (1979)")
	if v := e.str(`SELECT id FROM media_files WHERE rel_path = 'Alien Directors Cut (1979)/Alien (1979).mkv'`); v != fileID {
		t.Fatalf("moved file has id %q, want %q", v, fileID)
	}
	if n := e.count(`SELECT count(*) FROM media_files`); n != 10 {
		t.Errorf("media files = %d, want 10", n)
	}
	// The old folder's scan finds nothing left to do.
	e.pathScan(lib, "Alien (1979)")
	if n := e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("%d files missing after the moves", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs`); n != jobsBefore {
		t.Errorf("renames queued %d jobs, want none", n-jobsBefore)
	}

	// A hard link in another folder is a copy, not a move: the original stays where it is.
	if err := os.Link(filepath.Join(dir, twentyMovies[5]), filepath.Join(dir, "Lola rennt (1998)", "Inception Copy (2010).mp4")); err != nil {
		t.Fatal(err)
	}
	e.pathScan(lib, "Lola rennt (1998)")
	if v := e.str(`SELECT rel_path FROM media_files WHERE id = (SELECT id FROM media_files WHERE rel_path = ?)`, twentyMovies[5]); v != twentyMovies[5] {
		t.Errorf("hard-linked original moved to %q", v)
	}
	if n := e.count(`SELECT count(*) FROM media_files`); n != 11 {
		t.Errorf("media files = %d, want 11 (the link is a file of its own)", n)
	}
}

func TestPathScanMissingFilesAndFolders(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := append([]string{"Avatar (2009)/Avatar (2009) - 1080p.mkv", "Avatar (2009)/Avatar (2009) - 2160p.mkv"}, twentyMovies[:10]...)
	dir := tree(t, files)
	lib := e.create(dir)
	e.scan(lib.ID)
	missing := func() int { return e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`) }

	// A removed file.
	remove(t, dir, twentyMovies[1])
	if got := e.pathScan(lib, path.Dir(twentyMovies[1])); got.Scan.State != "done" || missing() != 1 || got.Scan.FileCount != 11 {
		t.Fatalf("after a removed file: %+v, missing %d", got.Scan, missing())
	}
	// A removed folder: both its files go missing (3 of 12 within 24 h: 25 %, accepted below).
	if err := os.RemoveAll(filepath.Join(dir, "Avatar (2009)")); err != nil {
		t.Fatal(err)
	}
	got := e.pathScan(lib, ".", "Avatar (2009)")
	if got.Scan.State != "aborted" || got.Scan.LastError != abortLoss || missing() != 1 {
		t.Fatalf("crossing 20 %% within 24 h: %+v, missing %d; want aborted/loss_over_threshold, 1", got.Scan, missing())
	}
	if _, err := e.svc.AcceptLoss(e.ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	got = e.pathScan(lib, ".", "Avatar (2009)")
	if got.Scan.State != "done" || missing() != 3 || got.Scan.FileCount != 9 || got.Scan.LossAcceptedAt != nil {
		t.Fatalf("accepted: %+v, missing %d; want done, 3 missing, acceptance cleared", got.Scan, missing())
	}
	if n := e.count(`SELECT count(*) FROM items WHERE available = 0 AND deleted_at IS NULL`); n != 2 {
		t.Errorf("unavailable items = %d, want 2", n)
	}
	// The file reappears: missing_since clears.
	writeFile(t, dir, twentyMovies[1], fmt.Sprintf("video %d %s", 3, twentyMovies[1]))
	if got := e.pathScan(lib, path.Dir(twentyMovies[1])); got.Scan.State != "done" || missing() != 2 {
		t.Fatalf("after reappearance: %+v, missing %d", got.Scan, missing())
	}
}

func TestPathScanLossProtection(t *testing.T) {
	t.Parallel()
	ten := twentyMovies[:10]
	missing := func(e *env) int { return e.count(`SELECT count(*) FROM media_files WHERE missing_since IS NOT NULL`) }

	t.Run("root missing", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		got := e.pathScan(lib, ".", "Alien (1979)")
		if got.Scan.State != "aborted" || got.Scan.LastError != abortRootMissing || missing(e) != 0 {
			t.Fatalf("path scan = %q/%q, missing %d; want aborted/root_missing", got.Scan.State, got.Scan.LastError, missing(e))
		}
	})

	t.Run("root empty", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range entries {
			if err := os.RemoveAll(filepath.Join(dir, d.Name())); err != nil {
				t.Fatal(err)
			}
		}
		got := e.pathScan(lib, ".", "Alien (1979)")
		if got.Scan.State != "aborted" || got.Scan.LastError != abortRootEmpty || missing(e) != 0 {
			t.Fatalf("path scan = %q/%q, missing %d; want aborted/root_empty", got.Scan.State, got.Scan.LastError, missing(e))
		}
	})

	t.Run("30 percent at once", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		e.clock.Advance(time.Hour)
		since := e.clock.Now()
		var dirs []string
		for _, f := range ten[:3] {
			if err := os.RemoveAll(filepath.Join(dir, path.Dir(f))); err != nil {
				t.Fatal(err)
			}
			dirs = append(dirs, path.Dir(f))
		}
		got := e.pathScan(lib, append(dirs, ".")...)
		if got.Scan.State != "aborted" || got.Scan.LastError != abortLoss {
			t.Fatalf("path scan = %q/%q, want aborted/loss_over_threshold", got.Scan.State, got.Scan.LastError)
		}
		if items, files, jobs := e.touched(since); items != 0 || files != 0 || jobs != 0 || missing(e) != 0 {
			t.Errorf("aborted path scan wrote %d items, %d files, %d jobs; %d missing", items, files, jobs, missing(e))
		}
	})

	t.Run("small losses add up within 24 h", func(t *testing.T) {
		e := newEnv(t)
		dir := tree(t, ten)
		lib := e.create(dir)
		e.scan(lib.ID)
		for i, want := range []string{"done", "done", "aborted"} {
			remove(t, dir, ten[i])
			got := e.pathScan(lib, path.Dir(ten[i]))
			if got.Scan.State != want {
				t.Fatalf("loss %d: state %q (%q), want %q", i+1, got.Scan.State, got.Scan.LastError, want)
			}
		}
		if missing(e) != 2 {
			t.Fatalf("missing = %d, want 2 (the third loss crosses 20 %%)", missing(e))
		}
		// After 24 h the earlier losses no longer count.
		e.clock.Advance(25 * time.Hour)
		if got := e.pathScan(lib, path.Dir(ten[2])); got.Scan.State != "done" || missing(e) != 3 {
			t.Fatalf("after 24 h: %+v, missing %d", got.Scan, missing(e))
		}
	})
}

// A path scan waits for a running full scan of the same library instead of racing it.
func TestPathScanWaitsForFullScan(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := tree(t, twentyMovies[:3])
	lib := e.create(dir)
	unlock, err := e.svc.lockScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- e.svc.ScanPaths(e.ctx, lib.ID, []pathScanRoot{{RootID: lib.Roots[0].ID, Dirs: []string{"."}}})
	}()
	select {
	case err := <-done:
		t.Fatalf("path scan ran while the library was locked: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(e.ctx)
	unlock, err = e.svc.lockScan(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	cancel()
	if err := e.svc.ScanPaths(ctx, lib.ID, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("waiting path scan with a cancelled context = %v", err)
	}
}

func TestCleanDirs(t *testing.T) {
	t.Parallel()
	got := cleanDirs([]string{"b/", "a", ".", "a", "../x", "/abs", "@eaDir/x", "a/.hidden", "c/./d"})
	if want := []string{".", "a", "b", "c/d"}; !slices.Equal(got, want) {
		t.Errorf("cleanDirs = %q, want %q", got, want)
	}
}
