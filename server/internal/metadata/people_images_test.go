package metadata

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const (
	collTMDB   = "01JTESTCOLL00000000000000A" // TMDB collection with a live member
	collManual = "01JTESTCOLL00000000000000B" // manual collection
	collEmpty  = "01JTESTCOLL00000000000000C" // TMDB collection whose member is deleted
	itemLive   = "01JTESTPEOP0000000000000AA"
	itemGone   = "01JTESTPEOP0000000000000AB"
)

// cdnAnswer is what the fake image host answers for one path.
type cdnAnswer struct {
	status int
	delay  time.Duration
	data   []byte
}

// fakeCDN serves /t/p/<size>/<file> on the fake TMDB server: answers per path, a hook per hit.
type fakeCDN struct {
	mu      sync.Mutex
	answers map[string]cdnAnswer
	hits    map[string]int
	onHit   func(path string)
}

func (c *fakeCDN) set(path string, a cdnAnswer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.answers[path] = a
}

func (c *fakeCDN) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits[path]
}

func (c *fakeCDN) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.hits {
		n += v
	}
	return n
}

// newPeopleEnv returns an env with two titles (one deleted), the image host and no people yet.
func newPeopleEnv(t *testing.T) (*env, *fakeCDN) {
	t.Helper()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	cdn := &fakeCDN{answers: map[string]cdnAnswer{}, hits: map[string]int{}}
	e.tmdb.mux.HandleFunc("GET /t/p/{size}/{file}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("token sent to the image host")
		}
		cdn.mu.Lock()
		cdn.hits[r.URL.Path]++
		a, ok := cdn.answers[r.URL.Path]
		hook := cdn.onHit
		cdn.mu.Unlock()
		if hook != nil {
			hook(r.URL.Path)
		}
		if a.delay > 0 {
			select {
			case <-time.After(a.delay):
			case <-r.Context().Done():
				return
			}
		}
		switch {
		case !ok:
			http.Error(w, "not found", http.StatusNotFound)
		case a.status != 0:
			w.Header().Set("Retry-After", "0")
			http.Error(w, "failure", a.status)
		default:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(a.data)
		}
	})
	e.addItem(t, itemSeed{id: itemLive, title: "Live"})
	e.addItem(t, itemSeed{id: itemGone, title: "Gone"})
	mustExec(t, e.db.Write, `UPDATE items SET deleted_at = 1 WHERE id = ?`, itemGone)
	return e, cdn
}

// addPerson stores a person with a role on item ("" = none) and returns its id.
func (e *env) addPerson(t *testing.T, id int64, profile any, item string) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT INTO people (id, tmdb_id, name, profile_path, created_at) VALUES (?, ?, ?, ?, 0)`,
		id, id, "Person", profile)
	if item != "" {
		mustExec(t, e.db.Write, `INSERT INTO credits (item_id, person_id, role, ord) VALUES (?, ?, 'cast', 0)`, item, id)
	}
}

func (e *env) addCollection(t *testing.T, id, source string, poster any, item string) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT INTO collections (id, source, name, poster_path, created_at, updated_at) VALUES (?, ?, 'C', ?, 0, 0)`,
		id, source, poster)
	if item != "" {
		mustExec(t, e.db.Write, `INSERT INTO collection_items (collection_id, item_id, created_at) VALUES (?, ?, 0)`, id, item)
	}
}

type subjectRow struct {
	Format, Status, Source, Size string
	RelPath, MediaType           sql.NullString
	Width, Height, Bytes         sql.NullInt64
	FetchedAt                    int64
}

// subjectRows returns the rows of a person (table person_images) or a collection by format.
func (e *env) subjectRows(t *testing.T, table, idCol string, id any) map[string]subjectRow {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT format, status, source, size, rel_path, media_type, width, height, bytes, fetched_at FROM `+
		table+` WHERE `+idCol+` = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]subjectRow{}
	for rows.Next() {
		var r subjectRow
		if err := rows.Scan(&r.Format, &r.Status, &r.Source, &r.Size, &r.RelPath, &r.MediaType, &r.Width, &r.Height, &r.Bytes, &r.FetchedAt); err != nil {
			t.Fatal(err)
		}
		out[r.Format] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *env) personRows(t *testing.T, id int64) map[string]subjectRow {
	return e.subjectRows(t, "person_images", "person_id", id)
}

func (e *env) collectionRows(t *testing.T, id string) map[string]subjectRow {
	return e.subjectRows(t, "collection_images", "collection_id", id)
}

func (e *env) cacheFile(rel string) bool {
	_, err := os.Stat(filepath.Join(e.cache, filepath.FromSlash(rel)))
	return err == nil
}

func (e *env) peopleImageJobs(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ?`, JobPeopleImages).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *env) runPeopleImages(t *testing.T) (*jobs.Recorder, error) {
	t.Helper()
	rec := &jobs.Recorder{}
	return rec, e.svc.handlePeopleImages(jobs.WithRecorder(t.Context(), rec), nil)
}

// A person with a portrait and a role, and a TMDB collection with a live member, get their files
// and rows; people and collections that are not candidates are left alone, and a second run
// downloads nothing.
func TestPeopleImagesFetchesOpenSubjects(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w185/p1.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	cdn.set("/t/p/w500/c1.jpg", cdnAnswer{data: jpegBytes(t, 500, 750)})
	e.addPerson(t, 1, "/p1.jpg", itemLive)
	e.addPerson(t, 2, "/p2.jpg", "")       // no role
	e.addPerson(t, 3, "/p3.jpg", itemGone) // only on a deleted title
	e.addPerson(t, 4, nil, itemLive)       // no portrait
	e.addCollection(t, collTMDB, "tmdb", "/c1.jpg", itemLive)
	e.addCollection(t, collManual, "manual", nil, itemLive)
	e.addCollection(t, collEmpty, "tmdb", "/c2.jpg", itemGone)

	rec, err := e.runPeopleImages(t)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Skipped {
		t.Error("a run with downloads is marked skipped")
	}
	if last, _ := rec.Last(); last.Done != 2 || last.Total != 2 {
		t.Errorf("last progress %+v, want 2 of 2", last)
	}
	p := e.personRows(t, 1)
	if j, w := p["jpeg"], p["webp"]; j.Status != "ok" || j.Source != "/p1.jpg" || j.Size != "w185" || j.RelPath.String != "people/1/w185.jpg" ||
		j.Width.Int64 != 185 || j.Height.Int64 != 278 || j.MediaType.String != "image/jpeg" || j.FetchedAt != testNow.UnixMilli() ||
		w.Status != "ok" || w.RelPath.String != "people/1/w185.webp" || w.Width.Int64 != 185 || w.MediaType.String != "image/webp" {
		t.Fatalf("person rows %+v", p)
	}
	data, err := os.ReadFile(filepath.Join(e.cache, "people", "1", "w185.jpg"))
	if err != nil || string(data) != string(cdn.answers["/t/p/w185/p1.jpg"].data) {
		t.Errorf("person JPEG not stored as fetched: %v", err)
	}
	if !e.cacheFile("people/1/w185.webp") {
		t.Error("person WebP missing")
	}
	c := e.collectionRows(t, collTMDB)
	if j, w := c["jpeg"], c["webp"]; j.Status != "ok" || j.Size != "w400" || j.RelPath.String != "collections/"+collTMDB+"/w400.jpg" ||
		j.Width.Int64 != 400 || j.Height.Int64 != 600 || w.RelPath.String != "collections/"+collTMDB+"/w400.webp" || w.Width.Int64 != 400 {
		t.Fatalf("collection rows %+v", c)
	}
	for _, id := range []int64{2, 3, 4} {
		if rows := e.personRows(t, id); len(rows) != 0 {
			t.Errorf("person %d rows %+v, want none", id, rows)
		}
	}
	for _, id := range []string{collManual, collEmpty} {
		if rows := e.collectionRows(t, id); len(rows) != 0 {
			t.Errorf("collection %s rows %+v, want none", id, rows)
		}
	}
	if n := cdn.total(); n != 2 {
		t.Errorf("%d downloads, want 2", n)
	}
	if got := e.tmdb.count("/3/configuration"); got != 0 {
		t.Errorf("API requests: %d", got)
	}

	// Up to date: nothing is fetched again.
	rec, err = e.runPeopleImages(t)
	if err != nil {
		t.Fatal(err)
	}
	if n := cdn.total(); n != 2 || !rec.Skipped {
		t.Errorf("second run: %d downloads, skipped %v; want 2 and skipped", n, rec.Skipped)
	}
}

// A changed path is fetched again, a deleted file of an 'ok' row too, and a person without a path
// loses rows and files.
func TestPeopleImagesChangedPathDeletedFileAndNoPath(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w185/p1.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	cdn.set("/t/p/w185/p1b.jpg", cdnAnswer{data: jpegBytes(t, 185, 260)})
	cdn.set("/t/p/w185/p2.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	e.addPerson(t, 1, "/p1.jpg", itemLive)
	e.addPerson(t, 2, "/p2.jpg", itemLive)
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}

	mustExec(t, e.db.Write, `UPDATE people SET profile_path = '/p1b.jpg' WHERE id = 1`)
	if err := os.Remove(filepath.Join(e.cache, "people", "2", "w185.webp")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if p := e.personRows(t, 1); p["jpeg"].Source != "/p1b.jpg" || p["jpeg"].Height.Int64 != 260 || p["webp"].Source != "/p1b.jpg" {
		t.Errorf("changed path: rows %+v", p)
	}
	if n := cdn.count("/t/p/w185/p2.jpg"); n != 2 || !e.cacheFile("people/2/w185.webp") {
		t.Errorf("deleted file: %d downloads of p2, webp there %v; want 2 and true", n, e.cacheFile("people/2/w185.webp"))
	}

	mustExec(t, e.db.Write, `UPDATE people SET profile_path = NULL WHERE id = 1`)
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if p := e.personRows(t, 1); len(p) != 0 || e.cacheFile("people/1") {
		t.Errorf("no path: rows %+v, directory there %v", p, e.cacheFile("people/1"))
	}
	if n := cdn.total(); n != 4 {
		t.Errorf("%d downloads, want 4", n)
	}
}

// A 404 and an answer that is no image record one 'missing' row; a run does not ask again before
// the row is 30 days old.
func TestPeopleImagesMissing(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w185/broken.jpg", cdnAnswer{data: []byte("not an image")})
	e.addPerson(t, 1, "/gone.jpg", itemLive) // 404
	e.addPerson(t, 2, "/broken.jpg", itemLive)
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{1, 2} {
		p := e.personRows(t, id)
		if len(p) != 1 || p["jpeg"].Status != "missing" || p["jpeg"].RelPath.Valid || p["jpeg"].FetchedAt != testNow.UnixMilli() {
			t.Fatalf("person %d rows %+v, want one missing JPEG row", id, p)
		}
	}
	if open, err := e.svc.QueuePeopleImages(t.Context()); err != nil || open {
		t.Errorf("QueuePeopleImages with fresh missing rows = %v, %v; want false", open, err)
	}

	// 29 days: still not asked.
	mustExec(t, e.db.Write, `UPDATE person_images SET fetched_at = ?`, testNow.Add(-29*24*time.Hour).UnixMilli())
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if n := cdn.total(); n != 2 {
		t.Fatalf("%d downloads with missing rows of 29 days, want 2", n)
	}
	// 31 days: asked again, the image is there now.
	cdn.set("/t/p/w185/gone.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	mustExec(t, e.db.Write, `UPDATE person_images SET fetched_at = ?`, testNow.Add(-31*24*time.Hour).UnixMilli())
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if p := e.personRows(t, 1); p["jpeg"].Status != "ok" || p["webp"].Status != "ok" {
		t.Errorf("person 1 after 31 days: %+v", p)
	}
	if n := cdn.total(); n != 4 {
		t.Errorf("%d downloads, want 4", n)
	}
}

// 5xx, 429 and a timeout write nothing; three of them in a row end the run with an error, a
// success in between resets the count.
func TestPeopleImagesFailuresWriteNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(e *env, cdn *fakeCDN)
	}{
		{"5xx", func(e *env, cdn *fakeCDN) {
			for _, p := range []string{"a", "b", "c"} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{status: http.StatusBadGateway})
			}
		}},
		{"429", func(e *env, cdn *fakeCDN) {
			for _, p := range []string{"a", "b", "c"} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{status: http.StatusTooManyRequests})
			}
		}},
		{"timeout", func(e *env, cdn *fakeCDN) {
			e.svc.client.http = &http.Client{Timeout: 50 * time.Millisecond}
			for _, p := range []string{"a", "b", "c"} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{delay: time.Second, data: jpegBytes(t, 185, 278)})
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cdn := newPeopleEnv(t)
			tc.setup(e, cdn)
			for i, p := range []string{"a", "b", "c"} {
				e.addPerson(t, int64(i+1), "/"+p+".jpg", itemLive)
			}
			_, err := e.runPeopleImages(t)
			if !errors.Is(err, errPeopleImagesFailures) {
				t.Fatalf("run = %v, want %v", err, errPeopleImagesFailures)
			}
			var n int
			if err := e.db.Read.QueryRow(`SELECT count(*) FROM person_images`).Scan(&n); err != nil || n != 0 {
				t.Errorf("%d rows written, want none (%v)", n, err)
			}
			if entries, _ := os.ReadDir(filepath.Join(e.cache, "people", "3")); len(entries) != 0 {
				t.Errorf("files left behind: %v", entries)
			}
		})
	}

	t.Run("reset", func(t *testing.T) {
		t.Parallel()
		e, cdn := newPeopleEnv(t)
		cdn.set("/t/p/w185/a.jpg", cdnAnswer{status: http.StatusServiceUnavailable})
		cdn.set("/t/p/w185/b.jpg", cdnAnswer{status: http.StatusServiceUnavailable})
		cdn.set("/t/p/w185/c.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
		cdn.set("/t/p/w185/d.jpg", cdnAnswer{status: http.StatusServiceUnavailable})
		for i, p := range []string{"a", "b", "c", "d"} {
			e.addPerson(t, int64(i+1), "/"+p+".jpg", itemLive)
		}
		if _, err := e.runPeopleImages(t); err != nil {
			t.Fatalf("run with a success between the failures: %v", err)
		}
		if p := e.personRows(t, 3); p["jpeg"].Status != "ok" {
			t.Errorf("person 3 rows %+v", p)
		}
		// The failed ones are not asked again in further passes of the run.
		if n := cdn.count("/t/p/w185/a.jpg"); n != 1 {
			t.Errorf("%d requests of a failed image in one run, want 1", n)
		}
	})
}

// Answers that would come again the same way record 'missing' rows and never end the run, however
// many come in a row: another 4xx, an answer above maxImageBytes, an image ffmpeg cannot decode, a
// path with a space or a control character (not asked) and a request that cannot be built.
func TestPeopleImagesDeterministicFailuresMissing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		paths     []string
		downloads int
		setup     func(e *env, cdn *fakeCDN)
	}{
		{"4xx", []string{"/a.jpg", "/b.jpg", "/c.jpg"}, 3, func(e *env, cdn *fakeCDN) {
			for p, status := range map[string]int{"a": http.StatusForbidden, "b": http.StatusBadRequest, "c": http.StatusGone} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{status: status})
			}
		}},
		{"oversize", []string{"/a.jpg", "/b.jpg", "/c.jpg"}, 3, func(e *env, cdn *fakeCDN) {
			big := append(jpegBytes(t, 185, 278), make([]byte, maxImageBytes)...)
			for _, p := range []string{"a", "b", "c"} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{data: big})
			}
		}},
		{"ffmpeg", []string{"/a.jpg", "/b.jpg", "/c.jpg"}, 3, func(e *env, cdn *fakeCDN) {
			e.svc.runner = transcode.Runner{FFmpegPath: failingFFmpeg(t)}
			for _, p := range []string{"a", "b", "c"} {
				cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
			}
		}},
		{"path", []string{"/a b.jpg", "/a\x01.jpg", "/a\t.jpg"}, 0, func(*env, *fakeCDN) {}},
		{"request", []string{"/a.jpg", "/b.jpg", "/c.jpg"}, 0, func(e *env, _ *fakeCDN) {
			e.svc.client.imageBase = "http://bad host/t/p"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cdn := newPeopleEnv(t)
			tc.setup(e, cdn)
			for i, p := range tc.paths {
				e.addPerson(t, int64(i+1), p, itemLive)
			}
			if _, err := e.runPeopleImages(t); err != nil {
				t.Fatalf("run = %v, want no error", err)
			}
			for i := range tc.paths {
				p := e.personRows(t, int64(i+1))
				if len(p) != 1 || p["jpeg"].Status != "missing" || p["jpeg"].RelPath.Valid {
					t.Errorf("person %d rows %+v, want one missing JPEG row", i+1, p)
				}
			}
			if n := cdn.total(); n != tc.downloads {
				t.Errorf("%d downloads, want %d", n, tc.downloads)
			}
			if entries, _ := os.ReadDir(filepath.Join(e.cache, "people")); len(entries) != 0 {
				t.Errorf("files left behind: %v", entries)
			}
		})
	}
}

// A collection id that is no ULID names no directory: a 'missing' row without a download, and the
// next run leaves it alone.
func TestPeopleImagesCollectionIDNotULID(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w500/c1.jpg", cdnAnswer{data: jpegBytes(t, 500, 750)})
	const bad = "../people/0000000000000000" // 26 characters, as the table demands
	e.addCollection(t, bad, "tmdb", "/c1.jpg", itemLive)
	for range 2 {
		if _, err := e.runPeopleImages(t); err != nil {
			t.Fatal(err)
		}
	}
	if c := e.collectionRows(t, bad); len(c) != 1 || c["jpeg"].Status != "missing" {
		t.Errorf("rows %+v, want one missing JPEG row", c)
	}
	if n := cdn.total(); n != 0 {
		t.Errorf("%d downloads, want none", n)
	}
}

// failingFFmpeg returns an ffmpeg that always fails.
func failingFFmpeg(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho broken >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// Work that appears during a run is taken in a further pass, at most three passes in all.
func TestPeopleImagesPasses(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	for _, p := range []string{"a", "b", "c", "d"} {
		cdn.set("/t/p/w185/"+p+".jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	}
	e.addPerson(t, 1, "/a.jpg", itemLive)
	// Every download changes the path of person 1 again: always open, three passes and done.
	next := map[string]string{"/t/p/w185/a.jpg": "/b.jpg", "/t/p/w185/b.jpg": "/c.jpg", "/t/p/w185/c.jpg": "/d.jpg", "/t/p/w185/d.jpg": "/a.jpg"}
	cdn.onHit = func(path string) {
		mustExec(t, e.db.Write, `UPDATE people SET profile_path = ? WHERE id = 1`, next[path])
	}
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if n := cdn.total(); n != peopleImagesPasses {
		t.Errorf("%d downloads, want one per pass (%d)", n, peopleImagesPasses)
	}
}

// The backfill deletes the 'missing' rows and queues people.images; a backfill during a run
// lets the running job fetch them in its next pass.
func TestPeopleImagesBackfill(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	e.addPerson(t, 1, "/a.jpg", itemLive)
	e.addPerson(t, 2, "/b.jpg", itemLive)
	e.addCollection(t, collTMDB, "tmdb", "/c1.jpg", itemLive)
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if n := cdn.total(); n != 3 {
		t.Fatalf("%d downloads, want 3 (all 404)", n)
	}
	ids, err := e.svc.runPeopleImagesBackfill(t.Context(), e.queue, jobs.ManualPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("backfill = %v, %v", ids, err)
	}
	var n, prio int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM person_images`).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d person rows after the backfill, want 0 (%v)", n, err)
	}
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM collection_images`).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d collection rows after the backfill, want 0 (%v)", n, err)
	}
	if err := e.db.Read.QueryRow(`SELECT priority FROM jobs WHERE id = ?`, ids[0]).Scan(&prio); err != nil || prio != jobs.ManualPriority {
		t.Errorf("job priority %d (%v), want %d", prio, err, jobs.ManualPriority)
	}

	// During a run: the second person's download triggers the backfill; the first person's fresh
	// 'missing' row is fetched in the next pass of the same run.
	cdn.set("/t/p/w185/a.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	if _, err := e.runPeopleImages(t); err != nil { // both 'missing' again, a fetched
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `UPDATE people SET profile_path = '/a2.jpg' WHERE id = 1`)
	cdn.set("/t/p/w185/b.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	var once sync.Once
	cdn.onHit = func(path string) {
		if path == "/t/p/w185/a2.jpg" {
			once.Do(func() {
				if _, err := e.svc.runPeopleImagesBackfill(context.Background(), e.queue, peopleImagesPriority); err != nil {
					t.Errorf("backfill during the run: %v", err)
				}
			})
		}
	}
	if _, err := e.runPeopleImages(t); err != nil {
		t.Fatal(err)
	}
	if p := e.personRows(t, 2); p["jpeg"].Status != "ok" {
		t.Errorf("person 2 after the backfill in the run: %+v, want ok", p)
	}
}

// A pause or a cancel ends the run through ctx.
func TestPeopleImagesStopsOnCancel(t *testing.T) {
	t.Parallel()
	e, cdn := newPeopleEnv(t)
	cdn.set("/t/p/w185/a.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	cdn.set("/t/p/w185/b.jpg", cdnAnswer{data: jpegBytes(t, 185, 278)})
	e.addPerson(t, 1, "/a.jpg", itemLive)
	e.addPerson(t, 2, "/b.jpg", itemLive)
	ctx, cancel := context.WithCancel(t.Context())
	cdn.onHit = func(string) { cancel() }
	err := e.svc.handlePeopleImages(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run = %v, want context.Canceled", err)
	}
	if n := cdn.count("/t/p/w185/b.jpg"); n != 0 {
		t.Errorf("%d downloads after the cancel, want 0", n)
	}
}

// A match that writes credits queues one people.images, however many matches follow; so does a
// series match, and QueuePendingMetadata when something is open.
func TestPeopleImagesQueuedByMatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.addItem(t, itemSeed{id: idByTMDB, title: "first film", year: 2001, tmdbID: 550})
	for range 10 {
		if err := e.svc.match(t.Context(), idByTMDB); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.peopleImageJobs(t); n != 1 {
		t.Errorf("%d people.images jobs after ten matches, want 1", n)
	}
	var prio int
	if err := e.db.Read.QueryRow(`SELECT priority FROM jobs WHERE type = ?`, JobPeopleImages).Scan(&prio); err != nil || prio != peopleImagesPriority {
		t.Errorf("priority %d (%v), want %d", prio, err, peopleImagesPriority)
	}

	s := newEnv(t, goodToken)
	s.addTree(t, node{title: "Test Show", year: 2011})
	if err := s.svc.match(t.Context(), idSeries); err != nil {
		t.Fatal(err)
	}
	if n := s.peopleImageJobs(t); n != 1 {
		t.Errorf("%d people.images jobs after a series match, want 1", n)
	}

	q := newEnv(t, goodToken)
	if _, err := q.svc.QueuePendingMetadata(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if n := q.peopleImageJobs(t); n != 0 {
		t.Errorf("%d people.images jobs without people, want 0", n)
	}
	q.addItem(t, itemSeed{id: itemLive, title: "Live"})
	q.addPerson(t, 1, "/a.jpg", itemLive)
	if _, err := q.svc.QueuePendingMetadata(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if n := q.peopleImageJobs(t); n != 1 {
		t.Errorf("%d people.images jobs with an open person, want 1", n)
	}
}

// people.images is a windowable event type of the metadata group below image.fetch, the backfill
// a schedule type without handler whose schedule is off.
func TestRegisterTypesPeopleImages(t *testing.T) {
	t.Parallel()
	e := newEnv(t, "")
	r := jobs.NewRegistry(e.queue, slog.New(slog.DiscardHandler))
	e.svc.RegisterTypes(r)
	pi, ok := r.Lookup(JobPeopleImages)
	if !ok || pi.Group != jobs.GroupMetadata || pi.Pool != jobs.PoolGeneral || pi.Trigger != jobs.TriggerEvent ||
		!pi.Windowable || pi.Priority != 1 || pi.Priority >= imageFetchPriority || pi.Handler == nil || pi.Run != nil {
		t.Errorf("people.images = %+v", pi)
	}
	bf, ok := r.Lookup(JobPeopleImagesBackfill)
	if !ok || bf.Group != jobs.GroupMetadata || bf.Trigger != jobs.TriggerSchedule || bf.Handler != nil || bf.Run == nil ||
		bf.Every != 7*24*time.Hour || bf.Seed == nil || bf.Windowable {
		t.Errorf("people.images_backfill = %+v", bf)
	} else if seed, err := bf.Seed(t.Context()); err != nil || seed.Enabled {
		t.Errorf("people.images_backfill seed = %+v, %v; want the schedule off", seed, err)
	}
}

func TestTMDBImagePath(t *testing.T) {
	t.Parallel()
	for p, want := range map[string]bool{
		"/abc.jpg": true, "": false, "/": false, "abc.jpg": false, "/a/b.jpg": false, "/../x": false,
		"/a.jpg?x=1": false, "/a%2F.jpg": false, "/a b.jpg": false, "/a\x00.jpg": false, "/a\n.jpg": false,
		"/a\u00a0.jpg": false, "/a\x7f.jpg": false, "/ä.jpg": true,
	} {
		if got := tmdbImagePath(p); got != want {
			t.Errorf("tmdbImagePath(%q) = %v, want %v", p, got, want)
		}
	}
}
