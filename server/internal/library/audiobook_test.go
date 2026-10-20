package library

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const (
	testAuthor   = "Slouchcat Test Author"
	testSeries   = "Slouchcat Test Audio Series"
	m4bFolder    = testAuthor + "/" + testSeries + "/01 - Slouchcat Test Audiobook M4B"
	mp3Folder    = testAuthor + "/" + testSeries + "/02 - Slouchcat Test Audiobook MP3"
	m4bTitle     = "Slouchcat Test Audiobook M4B"
	mp3BookTitle = "Slouchcat Test Audiobook MP3"
)

// audiobookMedia returns the generated audiobook fixtures (testdata/media/Audiobooks).
func audiobookMedia(t *testing.T) string {
	t.Helper()
	return filepath.Join(filepath.Dir(testMedia(t)), "Audiobooks")
}

// createAudiobooks adds an audiobook library on dir with the default rating 6.
func (e *env) createAudiobooks(dir string) model.Library {
	e.t.Helper()
	six := 6
	lib, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Hörbücher", Kind: model.LibraryAudiobooks, Path: dir, DefaultRating: &six})
	if err != nil {
		e.t.Fatal(err)
	}
	return lib
}

// copyTree copies the regular files below src into dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// bookChaptersOf returns the chapters of a book as "title@startMs" and the start times.
func (e *env) bookChaptersOf(itemID string) (titles []string, starts []int64) {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT title, start_ms FROM chapters WHERE item_id = ? ORDER BY idx`, itemID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var title string
		var start int64
		if err := rows.Scan(&title, &start); err != nil {
			e.t.Fatal(err)
		}
		titles = append(titles, title)
		starts = append(starts, start)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return titles, starts
}

// partDurations returns the durations of the parts of a book in part order.
func (e *env) partDurations(itemID string) []int64 {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT duration_ms FROM media_files WHERE item_id = ? AND missing_since IS NULL
	                                           ORDER BY part_index`, itemID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var d int64
		if err := rows.Scan(&d); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// checkMP3Chapters asserts the chapters of the MP3 book: "Kapitel 1".."Kapitel n" in part order,
// each starting at the sum of the durations of the parts before it (±1 s), and the book duration.
func (e *env) checkMP3Chapters(itemID string, n int) {
	e.t.Helper()
	titles, starts := e.bookChaptersOf(itemID)
	durs := e.partDurations(itemID)
	if len(titles) != n || len(durs) != n {
		e.t.Fatalf("%d chapters, %d parts, want %d: %v", len(titles), len(durs), n, titles)
	}
	var sum int64
	for i := range n {
		if want := "Kapitel " + itoa(i+1); titles[i] != want {
			e.t.Errorf("chapter %d = %q, want %q", i, titles[i], want)
		}
		if d := starts[i] - sum; d < -1000 || d > 1000 {
			e.t.Errorf("chapter %d starts at %d ms, want %d ±1 s", i, starts[i], sum)
		}
		sum += durs[i]
	}
	if got := e.count(`SELECT duration_ms FROM items WHERE id = ?`, itemID); int64(got) != sum {
		e.t.Errorf("book duration %d, want %d", got, sum)
	}
	if got := e.str(`SELECT group_concat(part_index, ',') FROM (SELECT part_index FROM media_files WHERE item_id = ?
	                  AND missing_since IS NULL ORDER BY part_index)`, itemID); got != strings.Join(seq(n), ",") {
		e.t.Errorf("part indexes %q", got)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func seq(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = itoa(i + 1)
	}
	return out
}

func TestScanAudiobookFixtures(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	lib := e.createAudiobooks(audiobookMedia(t))
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 11 {
		t.Fatalf("scan = %+v", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'audiobook' AND library_id = ?`, lib.ID); n != 2 {
		t.Fatalf("%d audiobooks, want 2", n)
	}
	if n := e.jobCount(JobMetadataMatch); n != 0 {
		t.Errorf("%d metadata.match jobs, want none for audiobooks", n)
	}
	e.probeQueued()

	m4b := e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE ?`, m4bFolder+"/%")
	mp3 := e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE ?`, mp3Folder+"/01 - %")
	checks := []struct{ q, want string }{
		{`SELECT title || '|' || author || '|' || narrator || '|' || series_name || '|' || series_index || '|' || year FROM items WHERE id = ?`,
			m4bTitle + "|" + testAuthor + "|Slouchcat Test Narrator|" + testSeries + "|1.0|2024"},
		{`SELECT metadata_state || '|' || own_rating || '|' || rating_source || '|' || explicit FROM items WHERE id = ?`, "matched|6|library|0"},
		{`SELECT overview FROM item_texts WHERE item_id = ? AND lang = 'de'`, "Ein Testhoerbuch mit zwoelf Kapiteln aus Sinustoenen."},
		{`SELECT count(*) || '|' || coalesce(max(part_index), 'null') FROM media_files WHERE item_id = ?`, "1|null"},
		{`SELECT duration_ms FROM items WHERE id = ?`, "60000"},
	}
	for _, c := range checks {
		if got := e.str(c.q, m4b); got != c.want {
			t.Errorf("m4b: %s\n  = %q, want %q", c.q, got, c.want)
		}
	}
	if got, want := e.str(`SELECT cover_path FROM items WHERE id = ?`, m4b), embeddedCover+e.str(`SELECT id FROM media_files WHERE item_id = ?`, m4b); got != want {
		t.Errorf("m4b cover %q, want %q", got, want)
	}
	titles, starts := e.bookChaptersOf(m4b)
	if len(titles) != 12 {
		t.Fatalf("m4b chapters %v, want 12", titles)
	}
	for i := range 12 {
		if want := "Kapitel " + itoa(i+1); titles[i] != want {
			t.Errorf("m4b chapter %d = %q, want %q", i, titles[i], want)
		}
		if d := starts[i] - int64(i*5000); d < -1000 || d > 1000 {
			t.Errorf("m4b chapter %d starts at %d ms, want %d ±1 s", i, starts[i], i*5000)
		}
	}

	mp3Checks := []struct{ q, want string }{
		{`SELECT title || '|' || author || '|' || narrator || '|' || series_name || '|' || series_index FROM items WHERE id = ?`,
			mp3BookTitle + "|" + testAuthor + "|Slouchcat Test Narrator|" + testSeries + "|2.0"},
		{`SELECT cover_path FROM items WHERE id = ?`, mp3Folder + "/cover.jpg"},
		{`SELECT own_rating || '|' || rating_source || '|' || metadata_state FROM items WHERE id = ?`, "6|library|matched"},
		{`SELECT count(*) FROM media_files WHERE item_id = ?`, "10"},
		// Part 10 sorts after part 9 (numbers, not text).
		{`SELECT rel_path FROM media_files WHERE item_id = ? AND part_index = 10`, mp3Folder + "/10 - Kapitel 10.mp3"},
	}
	for _, c := range mp3Checks {
		if got := e.str(c.q, mp3); got != c.want {
			t.Errorf("mp3: %s\n  = %q, want %q", c.q, got, c.want)
		}
	}
	e.checkMP3Chapters(mp3, 10)
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key IN (?, ?)`, JobImageFetch, "image:"+m4b, "image:"+mp3); n != 2 {
		t.Errorf("%d image.fetch jobs for the covers, want 2", n)
	}
	// Audio files have no keyframe index to compute.
	for _, id := range e.queuedJobs(JobProbeKeyframes) {
		payload := e.str(`SELECT payload FROM jobs WHERE id = ?`, id)
		var p mediaFilePayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			t.Fatal(err)
		}
		if err := e.svc.handleKeyframes(e.ctx, mediaJob(t, JobProbeKeyframes, p.MediaFileID)); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE keyframes_at IS NULL`); n != 0 {
		t.Errorf("%d files without keyframe run", n)
	}
	// A second scan changes nothing.
	before := e.count(`SELECT count(*) FROM items`)
	if got := e.scan(lib.ID); got.Scan.State != "done" || e.count(`SELECT count(*) FROM items`) != before {
		t.Errorf("rescan: %+v, items %d → %d", got.Scan, before, e.count(`SELECT count(*) FROM items`))
	}
}

func TestAudiobookPathScanPartAddedAndRemoved(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	dir := t.TempDir()
	copyTree(t, filepath.Join(audiobookMedia(t), filepath.FromSlash(mp3Folder)), filepath.Join(dir, "Autorin", "07 - Ein Buch"))
	lib := e.createAudiobooks(dir)
	e.scan(lib.ID)
	e.probeQueued()
	book := e.str(`SELECT id FROM items WHERE kind = 'audiobook'`)
	if got := e.str(`SELECT title || '|' || author || '|' || series_index FROM items WHERE id = ?`, book); got != mp3BookTitle+"|"+testAuthor+"|7.0" {
		// Tags (album, artist) win over the folder names; the volume comes from the folder.
		t.Errorf("book = %q", got)
	}
	e.checkMP3Chapters(book, 10)

	// A new part (track 11 by tag would be missing; the file name orders it last).
	folder := filepath.Join(dir, "Autorin", "07 - Ein Buch")
	if err := copyFile(filepath.Join(folder, "03 - Kapitel 3.mp3"), filepath.Join(folder, "11 - Kapitel 11.mp3")); err != nil {
		t.Fatal(err)
	}
	if got := e.pathScan(lib, "Autorin/07 - Ein Buch"); got.Scan.State != "done" {
		t.Fatalf("path scan %+v", got.Scan)
	}
	if n := e.count(`SELECT count(*) FROM items WHERE kind = 'audiobook'`); n != 1 {
		t.Fatalf("%d books after a new part, want 1", n)
	}
	if got := e.str(`SELECT part_index FROM media_files WHERE rel_path LIKE '%11 - Kapitel 11.mp3'`); got != "11" {
		t.Errorf("new part index %q, want 11", got)
	}
	e.probeQueued()
	titles, _ := e.bookChaptersOf(book)
	if len(titles) != 11 || titles[10] != "Kapitel 3" {
		// The copy carries the title tag of part 3 and track 3/10 twice: file-name order.
		t.Errorf("chapters after the new part: %v", titles)
	}

	// Removing a part renumbers the rest and drops its chapter at once.
	if err := os.Remove(filepath.Join(folder, "05 - Kapitel 5.mp3")); err != nil {
		t.Fatal(err)
	}
	if got := e.pathScan(lib, "Autorin/07 - Ein Buch"); got.Scan.State != "done" {
		t.Fatalf("path scan %+v", got.Scan)
	}
	titles, _ = e.bookChaptersOf(book)
	if len(titles) != 10 || slices.Contains(titles, "Kapitel 5") {
		t.Errorf("chapters after removing part 5: %v", titles)
	}
	if got := e.str(`SELECT group_concat(part_index, ',') FROM (SELECT part_index FROM media_files WHERE item_id = ?
	                 AND missing_since IS NULL ORDER BY part_index)`, book); got != "1,2,3,4,5,6,7,8,9,10" {
		t.Errorf("part indexes after removal %q", got)
	}
	durs := e.partDurations(book)
	var sum int64
	for _, d := range durs {
		sum += d
	}
	if got := e.count(`SELECT duration_ms FROM items WHERE id = ?`, book); int64(got) != sum {
		t.Errorf("duration %d, want %d", got, sum)
	}
}

func TestAudiobookLossProtectionCountsAudio(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "A/Buch/01.mp3", "x")
	writeFile(t, dir, "A/Buch/02.mp3", "y")
	writeFile(t, dir, "Film (2020)/Film (2020).mkv", "video") // ignored in an audiobook library
	lib := e.createAudiobooks(dir)
	if got := e.scan(lib.ID); got.Scan.State != "done" || got.Scan.FileCount != 2 {
		t.Fatalf("scan = %+v", got.Scan)
	}
	remove(t, dir, "A/Buch/01.mp3")
	remove(t, dir, "A/Buch/02.mp3")
	// Only the video is left: no audio file means an empty root, the scan stops.
	if got := e.scan(lib.ID); got.Scan.State != "aborted" || got.Scan.LastError != abortRootEmpty {
		t.Errorf("scan without audio = %+v, want aborted root_empty", got.Scan)
	}
}

func TestAudiobookMetadataPrecedence(t *testing.T) {
	if ffprobePath() == "" {
		t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	e := newEnv(t)
	dir := t.TempDir()
	src := filepath.Join(audiobookMedia(t), filepath.FromSlash(m4bFolder), m4bTitle+".m4b")
	if err := copyFile(src, filepath.Join(dir, "Ordner Verfasser", "Ordner Reihe", "03 - Ordner Titel", "buch.m4b")); err != nil {
		t.Fatal(err)
	}
	// Older Audiobookshelf schema with objects; no narrator: the tag fills it.
	writeFile(t, dir, "Ordner Verfasser/Ordner Reihe/03 - Ordner Titel/metadata.json", `{"metadata": {"title": "JSON Titel",
		"subtitle": "Untertitel", "authors": [{"name": "JSON Autorin"}, "Zweite"], "series": ["JSON Reihe #5.5"],
		"publishedYear": 1999, "description": "JSON Text", "genres": ["Krimi"], "language": "English", "explicit": true}}`)
	writeFile(t, dir, "Ordner Verfasser/Ordner Reihe/03 - Ordner Titel/folder.png", "not really a png")
	lib := e.createAudiobooks(dir)
	e.scan(lib.ID)
	book := e.str(`SELECT id FROM items WHERE kind = 'audiobook'`)
	// Before the probe: metadata.json and folder.
	if got := e.str(`SELECT title || '|' || author || '|' || coalesce(narrator, '-') || '|' || own_rating || '|' || rating_source FROM items WHERE id = ?`, book); got != "JSON Titel|JSON Autorin, Zweite|-|18|tags" {
		t.Errorf("after scan %q", got)
	}
	e.probeQueued()
	checks := []struct{ q, want string }{
		{`SELECT title || '|' || author || '|' || narrator || '|' || series_name || '|' || series_index || '|' || year FROM items WHERE id = ?`,
			"JSON Titel|JSON Autorin, Zweite|Slouchcat Test Narrator|JSON Reihe|5.5|1999"},
		{`SELECT own_rating || '|' || rating_source || '|' || explicit || '|' || genres FROM items WHERE id = ?`, `18|tags|1|{"de":["Krimi"],"en":["Krimi"]}`},
		{`SELECT cover_path FROM items WHERE id = ?`, "Ordner Verfasser/Ordner Reihe/03 - Ordner Titel/folder.png"},
		{`SELECT lang || '|' || title || '|' || overview || '|' || tagline FROM item_texts WHERE item_id = ?`, "en|JSON Titel|JSON Text|Untertitel"},
	}
	for _, c := range checks {
		if got := e.str(c.q, book); got != c.want {
			t.Errorf("%s\n  = %q, want %q", c.q, got, c.want)
		}
	}
	// A locked title and a manual rating survive the next probe.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE items SET title = 'Eigener Titel', locked_fields = '["title"]',
	        own_rating = 12, rating_source = 'manual' WHERE id = ?`, book); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done'`); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleProbe(e.ctx, mediaJob(t, JobProbeFile, e.str(`SELECT id FROM media_files`))); err != nil {
		t.Fatal(err)
	}
	if got := e.str(`SELECT title || '|' || own_rating || '|' || rating_source FROM items WHERE id = ?`, book); got != "Eigener Titel|12|manual" {
		t.Errorf("locked fields after a probe: %q", got)
	}
}

func TestFolderAndTagMeta(t *testing.T) {
	vol := func(m bookMeta) float64 {
		if m.volume == nil {
			return -1
		}
		return *m.volume
	}
	cases := []struct {
		path                  string
		title, author, series string
		volume                float64
	}{
		{"Verfasser/Reihe/05 - Titel", "Titel", "Verfasser", "Reihe", 5},
		{"Verfasser/1.5 - Titel", "Titel", "Verfasser", "", 1.5},
		{"Verfasser - Titel", "Titel", "Verfasser", "", -1},
		{"03 - Titel", "Titel", "", "", 3},
		{"Titel", "Titel", "", "", -1},
		{"Root/Verfasser/Reihe/Titel", "Titel", "Verfasser", "Reihe", -1},
	}
	for _, c := range cases {
		m := folderMeta(c.path)
		if m.title != c.title || m.author != c.author || m.series != c.series || vol(m) != c.volume {
			t.Errorf("folderMeta(%q) = %q %q %q %v", c.path, m.title, m.author, m.series, vol(m))
		}
	}
	tags := map[string]string{"title": "Kapitel 1", "album": "Buch", "artist": "A", "composer": "N", "show": "S",
		"episode_id": "4", "date": "2021-03-01", "genre": "Krimi", "comment": "C"}
	if m := tagMeta(tags, true); m.title != "Buch" || m.author != "A" || m.narrator != "N" || m.series != "S" || vol(m) != 4 ||
		m.year != 2021 || m.description != "C" || !slices.Equal(m.genres, []string{"Krimi"}) {
		t.Errorf("tagMeta multi = %+v", m)
	}
	if m := tagMeta(tags, false); m.title != "Kapitel 1" {
		t.Errorf("tagMeta single title %q", m.title)
	}
	j, ok := jsonMeta([]byte(`{"title":"T","authors":["A"],"narrators":["N1","N2"],"series":[{"name":"S","sequence":"2"}],"publishedYear":"2020","explicit":false}`))
	if !ok || j.title != "T" || j.narrator != "N1, N2" || j.series != "S" || vol(j) != 2 || j.year != 2020 || j.explicit {
		t.Errorf("jsonMeta = %+v", j)
	}
	if _, ok := jsonMeta([]byte(`{broken`)); ok {
		t.Error("broken metadata.json accepted")
	}
	merged := mergeBook(bookMeta{title: "J"}, bookMeta{title: "T", author: "TA"}, bookMeta{title: "F", author: "FA", series: "FS"})
	if merged.title != "J" || merged.author != "TA" || merged.series != "FS" || merged.lang != "de" {
		t.Errorf("mergeBook = %+v", merged)
	}
}

func TestOrderPartsAndChapters(t *testing.T) {
	probe := func(track string, d time.Duration, marks ...transcode.Chapter) *transcode.Probe {
		p := &transcode.Probe{Format: transcode.Format{Duration: d, Tags: map[string]string{}}, Chapters: marks}
		if track != "" {
			p.Format.Tags["track"] = track
		}
		return p
	}
	parts := []bookPart{
		{id: "c", rel: "b/10 - Zehn.mp3", durationMs: 3000, probe: probe("", 3*time.Second)},
		{id: "a", rel: "b/2 - Zwei.mp3", durationMs: 2000, probe: probe("", 2*time.Second)},
		{id: "b", rel: "b/1.mp3", durationMs: 1000, probe: probe("", time.Second)},
	}
	orderParts(parts)
	if got := []string{parts[0].id, parts[1].id, parts[2].id}; !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Errorf("file name order %v", got)
	}
	parts[0].probe.Format.Tags["track"], parts[1].probe.Format.Tags["track"], parts[2].probe.Format.Tags["track"] = "3/3", "2/3", "1/3"
	orderParts(parts)
	if got := []string{parts[0].id, parts[1].id, parts[2].id}; !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Errorf("track order %v", got)
	}
	parts[1].probe.Chapters = []transcode.Chapter{{Start: 0, End: time.Second, Title: ""}, {Start: time.Second, End: 5 * time.Second, Title: "Mitte"}}
	rows := bookChapters(parts, "Buch")
	want := []chapterRow{
		{title: "Zehn", startMs: 0, endMs: 3000, mediaFileID: "c"},
		{title: "Kapitel 2", startMs: 3000, endMs: 4000, mediaFileID: "a"},
		{title: "Mitte", startMs: 4000, endMs: 5000, mediaFileID: "a", fileStartMs: 1000},
		{title: "Teil 3", startMs: 5000, endMs: 6000, mediaFileID: "b"},
	}
	if !slices.Equal(rows, want) {
		t.Errorf("chapters %+v\nwant %+v", rows, want)
	}
	single := bookChapters([]bookPart{{id: "x", rel: "x.m4b", durationMs: 9000, probe: probe("", 9*time.Second)}}, "Buch")
	if len(single) != 1 || single[0].title != "Buch" || single[0].endMs != 9000 {
		t.Errorf("single file chapters %+v", single)
	}
	if naturalCompare("Teil 2", "teil 10") >= 0 || naturalCompare("b/9 - x", "b/10 - x") >= 0 {
		t.Errorf("naturalCompare")
	}
}
