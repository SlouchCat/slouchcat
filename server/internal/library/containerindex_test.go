package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Tests of concept 14, step 3: the keyframe index from the container during probe.file and the
// write rules of the index (4.2).

const longMKV = "Slouchcat Test H264 MKV Long (2024) {tmdb-7005}/%"

// probeLib scans the generated test movies and returns the id of the file whose rel_path is LIKE
// pattern, without its queued jobs.
func (e *env) probeLib(pattern string) string {
	e.t.Helper()
	if ffprobePath() == "" {
		e.t.Fatal("ffprobe is required for the probe tests (install ffmpeg)")
	}
	lib := e.create(testMedia(e.t))
	if got := e.scan(lib.ID); got.Scan.State != "done" {
		e.t.Fatalf("scan of the test media = %+v", got.Scan)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		e.t.Fatal(err)
	}
	return e.str(`SELECT id FROM media_files WHERE rel_path LIKE ?`, pattern)
}

func (e *env) probeFile(id string) error {
	e.t.Helper()
	return e.svc.handleProbe(e.ctx, mediaJob(e.t, JobProbeFile, id))
}

// indexState is keyframes_source (or "-"), "/blob" when a blob is stored, and the count of
// probe.keyframes jobs of the file.
func (e *env) indexState(id string) string {
	e.t.Helper()
	return e.str(`SELECT coalesce(keyframes_source, '-') || CASE WHEN keyframes IS NULL THEN '' ELSE '/blob' END
	              || ' jobs=' || (SELECT count(*) FROM jobs WHERE type = 'probe.keyframes' AND idempotency_key = 'keyframes:' || ?)
	              FROM media_files WHERE id = ?`, id, id)
}

func (e *env) storedKeyframes(id string) []time.Duration {
	e.t.Helper()
	var blob []byte
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT keyframes FROM media_files WHERE id = ?`, id).Scan(&blob); err != nil {
		e.t.Fatal(err)
	}
	kf, err := DecodeKeyframes(blob)
	if err != nil {
		e.t.Fatal(err)
	}
	return kf
}

// M-4: the e2e medium «H264 MKV Long» (90 s, the last keyframe at 75 s) is indexed from its cues,
// and no full read is queued. Every file of the test media is probed once; the log line is the
// duration of probe.file per file (DoD of step 3; CT 190 is measured in step 8).
func TestProbeIndexesFromContainer(t *testing.T) {
	e := newEnv(t)
	long := e.probeLib(longMKV)
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT id, rel_path FROM media_files ORDER BY rel_path`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type file struct{ id, rel string }
	var files []file
	for rows.Next() {
		var f file
		if err := rows.Scan(&f.id, &f.rel); err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	var total time.Duration
	for _, f := range files {
		started := time.Now()
		if err := e.probeFile(f.id); err != nil {
			t.Fatalf("%s: %v", f.rel, err)
		}
		d := time.Since(started)
		total += d
		t.Logf("probe.file %4d ms  %-22s %s", d.Milliseconds(), e.indexState(f.id), f.rel)
	}
	t.Logf("probe.file: %d files in %v, mean %v", len(files), total, total/time.Duration(len(files)))

	if v := e.indexState(long); v != "cues/blob jobs=0" {
		t.Fatalf("index of the long MKV = %q, want cues without a full read", v)
	}
	want := []time.Duration{0, 2, 7, 8, 15, 21, 30, 31, 45, 60, 61, 75}
	for i := range want {
		want[i] *= time.Second
	}
	if got := e.storedKeyframes(long); !slices.Equal(got, want) {
		t.Errorf("keyframes of the long MKV = %v, want %v", got, want)
	}
	// MP4 files are indexed from their sample tables.
	if n := e.count(`SELECT count(*) FROM media_files WHERE container = 'mp4' AND keyframes_source = 'mp4'`); n == 0 {
		t.Error("no MP4 file indexed from its sample tables")
	}
	// The broken file has no probe and no index.
	if v := e.str(`SELECT coalesce(keyframes_source, '-') FROM media_files WHERE rel_path LIKE 'Slouchcat Test Broken (2024)/%'`); v != "-" {
		t.Errorf("index of the broken file = %q", v)
	}
}

// Cues that fail the plausibility rules (a gap of 40 s) are not stored: the full read is queued.
func TestProbeImplausibleCuesQueueFullRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	id := e.probeLib(longMKV)
	e.svc.indexReader = func(io.ReaderAt, int64, string) (mediaindex.Index, error) {
		return mediaindex.Index{Keyframes: []time.Duration{0, 40 * time.Second, 75 * time.Second}, Source: mediaindex.SourceCues}, nil
	}
	if err := e.probeFile(id); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(id); v != "- jobs=1" {
		t.Errorf("index = %q, want none and a full read", v)
	}
}

// The sample finds an entry that is not a keyframe: the index is discarded, the full read is
// queued, and the warning names the file relative to its root.
func TestProbeSampleMismatchQueuesFullRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	id := e.probeLib(longMKV)
	var calls int
	e.svc.keyframeVerifier = func(_ context.Context, abs string, at []time.Duration, seed uint64) error {
		calls++
		if !filepath.IsAbs(abs) || len(at) != 12 || seed != sampleSeed(id) {
			t.Errorf("verifier called with %q, %v, %d", abs, at, seed)
		}
		return fmt.Errorf("%s: packet at entry 7s is at 2s: %w", abs, transcode.ErrKeyframeMismatch)
	}
	var logs bytes.Buffer
	e.svc.log = slog.New(slog.NewTextHandler(&logs, nil))
	if err := e.probeFile(id); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(id); v != "- jobs=1" || calls != 1 {
		t.Errorf("index = %q after %d samples, want none and a full read", v, calls)
	}
	line := logs.String()
	if !strings.Contains(line, "failed the sample, full read queued") || !strings.Contains(line, "Slouchcat Test H264 MKV Long (2024) {tmdb-7005}/") ||
		strings.Contains(line, testMedia(t)) {
		t.Errorf("warning %q, want the path relative to the root only", line)
	}
}

// VC-1 is never remuxed and an audiobook is left out of the automatic paths (concept 14, 2.1):
// neither gets an index or a full read.
func TestProbeDoesNotIndexUnworthyFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		present     bool
		codec, kind string
		want        bool
	}{
		{true, "h264", "movie", true}, {true, "hevc", "episode", true}, {true, "av1", "extra", true}, {true, "vp9", "movie", true},
		{false, "h264", "movie", false}, {true, "vc1", "movie", false}, {true, "mpeg2video", "movie", false}, {true, "", "movie", false},
		{true, "h264", "audiobook", false}, {true, "h264", "podcast_episode", false},
	} {
		if got := indexWorthy(tc.present, tc.codec, tc.kind); got != tc.want {
			t.Errorf("indexWorthy(%v, %q, %q) = %v", tc.present, tc.codec, tc.kind, got)
		}
	}

	e := newEnv(t)
	e.svc.indexReader = func(io.ReaderAt, int64, string) (mediaindex.Index, error) {
		t.Error("the container of an unworthy file was read")
		return mediaindex.Index{}, mediaindex.ErrNoIndex
	}
	vc1 := tree(t, []string{"Alien (1979)/Alien (1979).mkv"})
	e.scan(e.create(vc1).ID)
	e.svc.probe = transcode.Runner{FFprobePath: fakeFFprobe(t, "vc1")}
	id := e.str(`SELECT id FROM media_files`)
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	if err := e.probeFile(id); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(id) + " " + e.str(`SELECT video_codec FROM media_files WHERE id = ?`, id); v != "- jobs=0 vc1" {
		t.Errorf("VC-1: %q, want no index and no job", v)
	}

	books := tree(t, []string{"Author/Book/Book.m4b"})
	e.svc.probe = transcode.Runner{FFprobePath: fakeFFprobe(t, "h264")}
	e.scan(e.createAudiobooks(books).ID)
	book := e.str(`SELECT m.id FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE i.kind = 'audiobook'`)
	if err := e.probeFile(book); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(book); v != "- jobs=0" {
		t.Errorf("audiobook: %q, want no index and no job", v)
	}
}

// fakeFFprobe writes an ffprobe that prints a probe of one 90 s video stream of codec.
func fakeFFprobe(t *testing.T, codec string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffprobe")
	probe := `{"format":{"format_name":"matroska,webm","duration":"90.000000"},"streams":[{"index":0,"codec_type":"video","codec_name":"` +
		codec + `","width":1920,"height":1080}]}`
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+probe+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// setIndex stores a cues index on a file row as probe.file would.
func (e *env) setIndex(id string) {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET keyframes = ?, keyframes_at = 1, keyframes_source = 'cues' WHERE id = ?`,
		EncodeKeyframes([]time.Duration{0, 2 * time.Second}), id); err != nil {
		e.t.Fatal(err)
	}
}

// Z-1, m-3: a file changed in place or matched by its identity with another mtime loses its
// index; a rename keeps it. An index computed for the old version is not stored afterwards (Z-9).
func TestScanResetsIndexOfChangedFiles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{"Alien (1979)/Alien (1979).mkv", "Heat (1995)/Heat (1995).mkv", "Ran (1985)/Ran (1985).mkv"}
	dir := tree(t, files)
	lib := e.create(dir)
	e.scan(lib.ID)
	ids := make([]string, len(files))
	old := make([]FileVersion, len(files))
	for i, rel := range files {
		ids[i] = e.str(`SELECT id FROM media_files WHERE rel_path = ?`, rel)
		old[i] = FileVersion{Size: int64(e.count(`SELECT size FROM media_files WHERE id = ?`, ids[i])), Mtime: e.mtime(ids[i])}
		e.setIndex(ids[i])
	}
	// Changed in place (matched by path): another size.
	writeFile(t, dir, files[0], "a longer replacement of the first file")
	// Moved and touched (matched by dev, inode and size): the same size, another mtime.
	rename(t, dir, files[1], "Heat (1995)/Heat (1995) - moved.mkv")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "Heat (1995)", "Heat (1995) - moved.mkv"), later, later); err != nil {
		t.Fatal(err)
	}
	// Renamed only.
	rename(t, dir, files[2], "Ran (1985)/Ran (1985) - renamed.mkv")
	e.scan(lib.ID)

	for i, want := range []string{"-", "-", "cues/blob"} {
		if v := e.str(`SELECT coalesce(keyframes_source, '-') || CASE WHEN keyframes IS NULL THEN '' ELSE '/blob' END
		               || CASE WHEN keyframes_at IS NULL THEN '' ELSE '/at' END FROM media_files WHERE id = ?`, ids[i]); v != want && v != want+"/at" {
			t.Errorf("%s: index %q, want %q", files[i], v, want)
		}
	}
	if n := e.count(`SELECT count(*) FROM media_files WHERE id = ? AND rel_path = 'Heat (1995)/Heat (1995) - moved.mkv'`, ids[1]); n != 1 {
		t.Fatal("the moved file was not matched by its identity")
	}
	// The index of the old version of a changed file is not stored any more.
	for _, i := range []int{0, 1} {
		stored, err := StoreKeyframes(e.ctx, e.db.Write, ids[i], []time.Duration{0, time.Second}, KeyframesScan, old[i], e.clock.Now())
		if err != nil || stored {
			t.Errorf("%s: index of the old version stored = %v, %v", files[i], stored, err)
		}
	}
}

func (e *env) mtime(id string) int64 {
	e.t.Helper()
	var ms int64
	if err := e.db.Read.QueryRowContext(e.ctx, `SELECT mtime FROM media_files WHERE id = ?`, id).Scan(&ms); err != nil {
		e.t.Fatal(err)
	}
	return ms
}

// I-1: an index is final per file version; a second write changes nothing.
func TestStoreKeyframesIsFinal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.scan(e.create(tree(t, []string{"Alien (1979)/Alien (1979).mkv"})).ID)
	id := e.str(`SELECT id FROM media_files`)
	v := FileVersion{Size: int64(e.count(`SELECT size FROM media_files WHERE id = ?`, id)), Mtime: e.mtime(id)}
	first := []time.Duration{0, 2 * time.Second}
	if stored, err := StoreKeyframes(e.ctx, e.db.Write, id, first, KeyframesCues, v, e.clock.Now()); err != nil || !stored {
		t.Fatalf("first write = %v, %v", stored, err)
	}
	if stored, err := StoreKeyframes(e.ctx, e.db.Write, id, []time.Duration{0, time.Second}, KeyframesScan, v, e.clock.Now()); err != nil || stored {
		t.Fatalf("second write = %v, %v", stored, err)
	}
	if got := e.storedKeyframes(id); !slices.Equal(got, first) || e.str(`SELECT keyframes_source FROM media_files WHERE id = ?`, id) != KeyframesCues {
		t.Errorf("index after the second write = %v", got)
	}
	// An empty index is "none" without a blob.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET keyframes = NULL, keyframes_source = NULL WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if stored, err := StoreKeyframes(e.ctx, e.db.Write, id, nil, KeyframesScan, v, e.clock.Now()); err != nil || !stored {
		t.Fatalf("empty write = %v, %v", stored, err)
	}
	if s := e.indexState(id); s != "none jobs=0" {
		t.Errorf("empty index = %q, want none without a blob", s)
	}
}

// Z-9: the file changes while probe.file reads it. The probe of the old version is not written,
// the run fails for a retry, and the retry stores probe and index of the new version.
func TestProbeOfOldVersionIsNotWritten(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	id := e.probeLib(longMKV)
	var once sync.Once
	e.svc.indexReader = func(r io.ReaderAt, size int64, container string) (mediaindex.Index, error) {
		once.Do(func() {
			// What the scanner writes for a changed file (UpdateMediaFileLocation with reset_index).
			if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET mtime = mtime + 1000 WHERE id = ?`, id); err != nil {
				t.Error(err)
			}
		})
		return mediaindex.Read(r, size, container)
	}
	err := e.probeFile(id)
	if !errors.Is(err, errFileChanged) {
		t.Fatalf("probe of the old version = %v, want errFileChanged", err)
	}
	if v := e.str(`SELECT coalesce(probed_at, '-') || ' ' || coalesce(video_codec, '-') FROM media_files WHERE id = ?`, id); v != "- -" {
		t.Errorf("probe of the old version written: %q", v)
	}
	if v := e.indexState(id); v != "- jobs=0" {
		t.Errorf("index of the old version = %q", v)
	}
	if err := e.probeFile(id); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if v := e.indexState(id); v != "cues/blob jobs=0" {
		t.Errorf("index after the retry = %q", v)
	}
}

// M-1: probe.file reads the container before its write transaction. While the parser blocks,
// another write on the one write connection succeeds.
func TestProbeReadsOutsideTheTransaction(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	id := e.probeLib(longMKV)
	entered, release := make(chan struct{}), make(chan struct{})
	e.svc.indexReader = func(r io.ReaderAt, size int64, container string) (mediaindex.Index, error) {
		close(entered)
		<-release
		return mediaindex.Read(r, size, container)
	}
	done := make(chan error, 1)
	go func() { done <- e.probeFile(id) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("probe ended before the parser ran: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("parser not reached")
	}
	ctx, cancel := context.WithTimeout(e.ctx, 2*time.Second)
	_, err := e.db.Write.ExecContext(ctx, `UPDATE libraries SET name = 'Während der Probe' WHERE id = (SELECT min(id) FROM libraries)`)
	cancel()
	close(release)
	if err != nil {
		t.Errorf("write while the parser blocks: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(id); v != "cues/blob jobs=0" {
		t.Errorf("index = %q", v)
	}
}
