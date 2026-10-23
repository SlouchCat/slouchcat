package library

import (
	"testing"
)

// I-5: a sidecar subtitle placed next to a scanned file reaches the tracks after a path scan of its
// directory, without probing again; a deleted one leaves them. Full scans do the same; files that
// are not probed yet are left to their probe.
func TestSidecarSyncOnScans(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir := t.TempDir()
	film := "Film (2020)/Film (2020).mkv"
	other := "Other (2021)/Other (2021).mkv"
	writeFile(t, dir, film, "film")
	writeFile(t, dir, other, "other")
	lib := e.create(dir)
	e.scan(lib.ID)
	id := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, film)
	otherID := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, other)
	// The film was probed (its embedded tracks are stored); the other file waits for its probe.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET probed_at = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `INSERT INTO tracks (media_file_id, stream_index, kind, codec, source) VALUES (?, 1, 'audio', 'aac', 'embedded')`, id); err != nil {
		t.Fatal(err)
	}
	sidecars := func(fileID string) int {
		return e.count(`SELECT count(*) FROM tracks WHERE media_file_id = ? AND source = 'sidecar'`, fileID)
	}
	probeJobs := e.jobCount(JobProbeFile)

	writeFile(t, dir, "Film (2020)/Film (2020).de.srt", "1\n00:00:01,000 --> 00:00:02,000\nHallo\n")
	writeFile(t, dir, "Film (2020)/Film (2020).en.forced.ass", "[Script Info]\n")
	writeFile(t, dir, "Other (2021)/Other (2021).de.srt", "1\n")
	e.pathScan(lib, "Film (2020)", "Other (2021)")
	if n := sidecars(id); n != 2 {
		t.Fatalf("sidecars after the path scan = %d, want 2", n)
	}
	if got := e.str(`SELECT language || '/' || codec || '/' || stream_index || '/' || is_forced FROM tracks WHERE media_file_id = ? AND rel_path = ?`,
		id, "Film (2020)/Film (2020).de.srt"); got != "de/subrip/-1/0" {
		t.Errorf("de.srt row = %q", got)
	}
	if got := e.str(`SELECT language || '/' || codec || '/' || is_forced FROM tracks WHERE media_file_id = ? AND rel_path = ?`,
		id, "Film (2020)/Film (2020).en.forced.ass"); got != "en/ass/1" {
		t.Errorf("en.forced.ass row = %q", got)
	}
	if n := e.count(`SELECT count(*) FROM tracks WHERE media_file_id = ? AND source = 'embedded'`, id); n != 1 {
		t.Errorf("embedded tracks = %d, the sync must not touch them", n)
	}
	if n := sidecars(otherID); n != 0 {
		t.Errorf("unprobed file got %d sidecar rows", n)
	}
	if n := e.jobCount(JobProbeFile); n != probeJobs {
		t.Errorf("probe jobs %d → %d: the sync must not probe", probeJobs, n)
	}
	// A second scan changes nothing.
	e.pathScan(lib, "Film (2020)")
	if n := sidecars(id); n != 2 {
		t.Errorf("sidecars after a second scan = %d", n)
	}

	remove(t, dir, "Film (2020)/Film (2020).de.srt")
	e.pathScan(lib, "Film (2020)")
	if n := sidecars(id); n != 1 {
		t.Fatalf("sidecars after deleting de.srt = %d, want 1", n)
	}
	// A path scan of another directory leaves the film alone; a full scan reads everything.
	writeFile(t, dir, "Film (2020)/Film (2020).fr.srt", "1\n")
	e.pathScan(lib, "Other (2021)")
	if n := sidecars(id); n != 1 {
		t.Errorf("sidecars after a scan of another directory = %d, want 1", n)
	}
	e.scan(lib.ID)
	if n := sidecars(id); n != 2 {
		t.Errorf("sidecars after a full scan = %d, want 2", n)
	}
	remove(t, dir, "Film (2020)/Film (2020).fr.srt")
	remove(t, dir, "Film (2020)/Film (2020).en.forced.ass")
	e.scan(lib.ID)
	if n := sidecars(id); n != 0 {
		t.Errorf("sidecars after deleting all = %d, want 0", n)
	}
}

func TestSidecarsOf(t *testing.T) {
	t.Parallel()
	got := sidecarsOf("Film/Film.mkv", []string{"Film.de.srt", "Film.srt", "Other.de.srt", "Film.Director.en.srt", "Film.sdh.en.vtt"})
	var rels []string
	for _, sc := range got {
		rels = append(rels, sc.rel+"/"+sc.lang)
	}
	want := []string{"Film/Film.de.srt/de", "Film/Film.sdh.en.vtt/en", "Film/Film.srt/"}
	if len(rels) != len(want) {
		t.Fatalf("sidecars = %v, want %v", rels, want)
	}
	for i := range want {
		if rels[i] != want[i] {
			t.Errorf("sidecar %d = %s, want %s", i, rels[i], want[i])
		}
	}
	for name, want := range map[string]bool{"a.srt": true, "a.ASS": true, "a.vtt": true, "a.mkv": false, "._a.srt": false} {
		if isSubtitle(name) != want {
			t.Errorf("isSubtitle(%q) = %v", name, !want)
		}
	}
}
