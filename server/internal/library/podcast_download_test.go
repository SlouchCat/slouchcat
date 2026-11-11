package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// podcastEpisodeMedia returns the bytes of a generated podcast episode (testdata/media/Podcasts).
func podcastEpisodeMedia(t *testing.T, n int) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(testMedia(t)), "Podcasts", "episode-"+strconv.Itoa(n)+".mp3"))
	if err != nil {
		t.Fatalf("podcast test media missing (run `task testdata:media`): %v", err)
	}
	return data
}

// enclosureServer serves the enclosures of a test: by path, with Range; a handler set in before
// may answer instead (it returns true when it did).
type enclosureServer struct {
	*httptest.Server
	mu     sync.Mutex
	files  map[string][]byte
	ranges []string
	before func(w http.ResponseWriter, r *http.Request) bool
}

func newEnclosureServer(t *testing.T) *enclosureServer {
	t.Helper()
	s := &enclosureServer{files: map[string][]byte{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.ranges = append(s.ranges, r.Header.Get("Range"))
		before, data, ok := s.before, s.files[r.URL.Path], true
		if data == nil {
			ok = false
		}
		s.mu.Unlock()
		if before != nil && before(w, r) {
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("ETag", `"e1"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *enclosureServer) set(fn func(s *enclosureServer)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

// downloadFixture is a podcast library on «Speichern» with a subscription whose feed lives on the
// enclosure server (the loopback; package netguard lets a feed's own local host fetch there) and a
// folder «Testpodcast» with one file.
type downloadFixture struct {
	*podcastFixture
	srv *enclosureServer
	log *bytes.Buffer
}

func newDownloadFixture(t *testing.T) *downloadFixture {
	t.Helper()
	f := newPodcastFixture(t)
	if ffmpegPath() == "" || ffprobePath() == "" {
		t.Fatal("ffmpeg and ffprobe are required for the download tests")
	}
	f.e.svc.probe = transcode.Runner{FFmpegPath: ffmpegPath(), FFprobePath: ffprobePath()}
	logs := &lockedBuffer{}
	f.e.svc.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := newEnclosureServer(t)
	f.exec(`UPDATE items SET feed_url = ? WHERE id = ?`, srv.URL+"/feed.xml", f.podcast)
	f.exec(`UPDATE libraries SET podcast_mode = 'save' WHERE id = ?`, f.lib.ID)
	writeFile(t, f.dir, "Testpodcast/Alte Folge.mp3", string(podcastEpisodeMedia(t, 3)))
	f.e.scan(f.lib.ID)
	// The scan made a folder podcast of the file; its episode belongs to the subscription.
	f.exec(`UPDATE items SET parent_id = ? WHERE kind = 'podcast_episode' AND guid IS NULL`, f.podcast)
	f.exec(`UPDATE items SET deleted_at = 1 WHERE kind = 'podcast' AND id <> ?`, f.podcast)
	return &downloadFixture{podcastFixture: f, srv: srv, log: &logs.buf}
}

// lockedBuffer is a bytes.Buffer for a logger that background work writes to as well.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// episode adds a feed episode whose enclosure the server serves at name with data (nil: nothing).
func (f *downloadFixture) episode(title, name string, at time.Time, data []byte) string {
	id := f.feedEpisode(title, at, 0)
	f.exec(`UPDATE items SET enclosure_url = ? WHERE id = ?`, f.srv.URL+"/"+name, id)
	if data != nil {
		f.srv.set(func(s *enclosureServer) { s.files["/"+name] = data })
	}
	return id
}

// download runs podcast.download for an episode as attempt n of 5 and returns its error.
func (f *downloadFixture) download(ctx context.Context, episodeID string, attempt int) error {
	payload, err := json.Marshal(podcastDownloadPayload{EpisodeID: episodeID})
	if err != nil {
		f.e.t.Fatal(err)
	}
	return f.e.svc.handlePodcastDownload(ctx, &jobs.Job{ID: 1, Type: JobPodcastDownload, Payload: payload, Attempts: attempt, MaxAttempts: 5})
}

// visible lists the names in a folder that do not start with a dot.
func visible(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for _, n := range dirNames(t, dir) {
		if !strings.HasPrefix(n, ".") {
			out = append(out, n)
		}
	}
	return out
}

// Concept 19 P-1 and P-19: on «Speichern / Nur neue Folgen» a new episode becomes a file in the
// folder of its podcast, named after its title by Audiobookshelf's rule, tagged with title, podcast
// and date, entered as the media file of the feed episode with its probe; the own files are gone.
// The progress is in bytes and names the title only.
func TestPodcastDownloadStoresTheEpisode(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	content := podcastEpisodeMedia(t, 1)
	at := time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)
	ep := f.episode("Folge 12: Der Anfang", "ep12.mp3", at, content)
	rec := &jobs.Recorder{}
	if err := f.download(jobs.WithRecorder(f.e.ctx, rec), ep, 1); err != nil {
		t.Fatal(err)
	}
	rel := "Testpodcast/Folge 12 - Der Anfang.mp3"
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ? AND rel_path = ? AND root_id = ? AND missing_since IS NULL
		AND probe IS NOT NULL AND duration_ms > 0 AND audio_codec = 'mp3'`, ep, rel, f.root.ID); n != 1 {
		t.Fatalf("media file of the episode: %d rows", n)
	}
	if n := f.e.count(`SELECT count(*) FROM tracks AS t JOIN media_files AS m ON m.id = t.media_file_id WHERE m.item_id = ? AND t.kind = 'audio'`, ep); n != 1 {
		t.Errorf("audio tracks of the stored episode = %d", n)
	}
	abs := filepath.Join(f.dir, filepath.FromSlash(rel))
	fi, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o664 {
		t.Errorf("mode of the stored episode = %v, want 0664", fi.Mode().Perm())
	}
	p, err := f.e.svc.probe.Probe(f.e.ctx, abs)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"title": "Folge 12: Der Anfang", "album": "Testpodcast", "date": "2026-11-09T18:00:00Z"} {
		if p.Format.Tags[k] != want {
			t.Errorf("tag %s = %q, want %q", k, p.Format.Tags[k], want)
		}
	}
	if names := dirNames(t, filepath.Join(f.dir, "Testpodcast")); len(names) != 2 {
		t.Errorf("folder holds %v, want the old file and the stored one only", names)
	}
	last, ok := rec.Last()
	if !ok || last.Done <= 0 || last.Done != last.Total || last.Note != "Folge 12: Der Anfang" {
		t.Errorf("last progress = %+v", last)
	}
	// A second run has nothing to do.
	if err := f.download(f.e.ctx, ep, 1); err != nil {
		t.Fatal(err)
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, ep); n != 1 {
		t.Errorf("files after a second run = %d", n)
	}
}

// Concept 19 P-19: when the tagged copy does not probe, the untagged file is entered.
func TestPodcastDownloadFallsBackToTheUntaggedFile(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	content := podcastEpisodeMedia(t, 2)
	f.e.svc.podcastTag = func(_ context.Context, _, out, _ string, _ transcode.AudioTags) error {
		return os.WriteFile(out, []byte("not audio"), 0o600)
	}
	ep := f.episode("Ohne Tags", "ohne.mp3", time.Date(2026, 11, 2, 6, 0, 0, 0, time.UTC), content)
	if err := f.download(f.e.ctx, ep, 1); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(f.dir, "Testpodcast", "Ohne Tags.mp3"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("entered file is not the untagged download (%d bytes, %v)", len(got), err)
	}
	if !strings.Contains(f.log.String(), "does not probe") {
		t.Errorf("no note on the failed tags in the log")
	}
}

// Concept 19 P-2: a scan while the episode loads finds no new episode (the own files start with a
// dot); after the download the episode exists exactly once, with one file, without a file episode
// beside it, and a scan afterwards knows the file.
func TestPodcastDownloadAndScan(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	content := podcastEpisodeMedia(t, 1)
	started, resume := make(chan struct{}), make(chan struct{})
	f.srv.set(func(s *enclosureServer) {
		s.before = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/slow.mp3" {
				return false
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			_, _ = w.Write(content[:len(content)/2])
			w.(http.Flusher).Flush()
			close(started)
			<-resume
			_, _ = w.Write(content[len(content)/2:])
			return true
		}
	})
	ep := f.episode("Langsame Folge", "slow.mp3", time.Date(2026, 11, 3, 6, 0, 0, 0, time.UTC), nil)
	episodes := func() int {
		return f.e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND deleted_at IS NULL`)
	}
	before := episodes()
	done := make(chan error, 1)
	go func() { done <- f.download(f.e.ctx, ep, 1) }()
	<-started
	f.e.scan(f.lib.ID)
	if got := episodes(); got != before {
		t.Errorf("a scan during the download made %d episodes, want %d", got, before)
	}
	close(resume)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	f.e.scan(f.lib.ID)
	if got := episodes(); got != before {
		t.Errorf("episodes after the download and a scan = %d, want %d", got, before)
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ? AND missing_since IS NULL`, ep); n != 1 {
		t.Errorf("files of the episode = %d", n)
	}
	if n := f.e.count(`SELECT count(*) FROM items WHERE kind = 'podcast_episode' AND guid IS NULL AND deleted_at IS NULL AND title LIKE 'Langsame%'`); n != 0 {
		t.Errorf("a file episode of the stored file exists")
	}
	fileID := f.e.str(`SELECT id FROM media_files WHERE item_id = ?`, ep)
	if n := f.e.count(`SELECT count(*) FROM jobs WHERE type = ? AND payload LIKE ?`, JobProbeFile, "%"+fileID+"%"); n != 0 {
		t.Errorf("the scan queued a probe of the stored file")
	}
}

// Concept 19 P-7: a body cut short leaves the episode a link without a visible file; the next
// attempt continues the partial file with a Range request.
func TestPodcastDownloadResumes(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	content := podcastEpisodeMedia(t, 1)
	cut := true
	f.srv.set(func(s *enclosureServer) {
		s.files["/cut.mp3"] = content
		s.before = func(w http.ResponseWriter, r *http.Request) bool {
			s.mu.Lock()
			c := cut
			s.mu.Unlock()
			if !c || r.URL.Path != "/cut.mp3" {
				return false
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("ETag", `"e1"`)
			_, _ = w.Write(content[:len(content)/3])
			panic(http.ErrAbortHandler)
		}
	})
	ep := f.episode("Abgebrochen", "cut.mp3", time.Date(2026, 11, 4, 6, 0, 0, 0, time.UTC), nil)
	if err := f.download(f.e.ctx, ep, 1); err == nil {
		t.Fatal("cut body: no error")
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, ep); n != 0 {
		t.Errorf("a cut download entered a file")
	}
	if n := f.e.count(`SELECT count(*) FROM items WHERE id = ? AND enclosure_url IS NOT NULL AND deleted_at IS NULL`, ep); n != 1 {
		t.Errorf("the episode is no link any more")
	}
	if names := visible(t, filepath.Join(f.dir, "Testpodcast")); len(names) != 1 {
		t.Errorf("visible files after the cut: %v", names)
	}
	f.srv.set(func(*enclosureServer) { cut = false })
	if err := f.download(f.e.ctx, ep, 2); err != nil {
		t.Fatal(err)
	}
	// The stored file is the tagged copy: its size differs from the enclosure's, its duration not.
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ? AND rel_path = 'Testpodcast/Abgebrochen.mp3' AND duration_ms > 0`, ep); n != 1 {
		t.Errorf("the resumed download was not entered")
	}
	f.srv.set(func(s *enclosureServer) {
		if last := s.ranges[len(s.ranges)-1]; !strings.HasPrefix(last, "bytes="+strconv.Itoa(len(content)/3)) {
			t.Errorf("second request asked Range %q, want a resume at byte %d", last, len(content)/3)
		}
	})
}

// Concept 19 P-9: a subscription without any file gets a folder of its title with metadata.json
// (Audiobookshelf's schema with feedURL) and the cover; a scan afterwards finds the subscription by
// the feed and makes no second podcast.
func TestPodcastDownloadCreatesTheFolder(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	cover, err := os.ReadFile(filepath.Join(filepath.Dir(testMedia(t)), "Podcasts", "cover.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	f.srv.set(func(s *enclosureServer) {
		s.before = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/cover.jpg" {
				return false
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(cover)
			return true
		}
	})
	neu := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, author, feed_url, feed_origin, cover_url, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast', ?, 'Neues Abo: Teil 1', 'neues abo', 'Redaktion', ?, 'admin', ?, '["local"]', 'matched', 1, 1, 1)`,
		neu, f.lib.ID, f.srv.URL+"/neu.xml", f.srv.URL+"/cover.jpg")
	podcast := f.podcast
	f.podcast = neu
	ep := f.episode("Erste Folge", "erste.mp3", time.Date(2026, 11, 5, 6, 0, 0, 0, time.UTC), podcastEpisodeMedia(t, 2))
	f.podcast = podcast
	if err := f.download(f.e.ctx, ep, 1); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(f.dir, "Neues Abo - Teil 1")
	if names := visible(t, folder); strings.Join(names, ",") != "Erste Folge.mp3,cover.jpg,metadata.json" {
		t.Fatalf("new folder holds %v", names)
	}
	raw, err := os.ReadFile(filepath.Join(folder, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := podcastJSON(raw); !ok || m.feedURL != f.srv.URL+"/neu.xml" || m.title != "Neues Abo: Teil 1" || m.author != "Redaktion" {
		t.Errorf("metadata.json = %s", raw)
	}
	if got, err := os.ReadFile(filepath.Join(folder, "cover.jpg")); err != nil || !bytes.Equal(got, cover) {
		t.Errorf("cover.jpg: %d bytes, %v", len(got), err)
	}
	if n := f.e.count(`SELECT count(*) FROM podcast_settings WHERE item_id = ? AND (pending_folder IS NOT NULL OR pending_root_id IS NOT NULL)`, neu); n != 0 {
		t.Errorf("pending folder kept after the first file")
	}
	f.e.scan(f.lib.ID)
	if n := f.e.count(`SELECT count(*) FROM items WHERE kind = 'podcast' AND deleted_at IS NULL`); n != 2 {
		t.Errorf("live podcasts after the scan = %d, want the two subscriptions", n)
	}
	if n := f.e.count(`SELECT count(*) FROM items WHERE parent_id = ? AND kind = 'podcast_episode' AND deleted_at IS NULL`, neu); n != 1 {
		t.Errorf("episodes of the new subscription after the scan = %d", n)
	}
}

// Concept 19 P-15: an answer that is no audio (HTML with status 200) leaves no file in the folder,
// the episode a link and the failure not_audio, at once.
func TestPodcastDownloadRefusesNonAudio(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	f.srv.set(func(s *enclosureServer) {
		s.before = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != "/page.mp3" {
				return false
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>Bitte anmelden</body></html>"))
			return true
		}
	})
	ep := f.episode("Keine Datei", "page.mp3", time.Date(2026, 11, 6, 6, 0, 0, 0, time.UTC), nil)
	err := f.download(f.e.ctx, ep, 1)
	var de *podcastDownloadError
	if !errors.As(err, &de) || de.code != PodcastErrNotAudio || !de.final {
		t.Fatalf("download = %v, want a final not_audio", err)
	}
	if names := dirNames(t, filepath.Join(f.dir, "Testpodcast")); len(names) != 1 {
		t.Errorf("folder after a refused answer: %v", names)
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, ep); n != 0 {
		t.Errorf("a refused answer entered a file")
	}
	if got := f.e.str(`SELECT error FROM podcast_download_failures WHERE item_id = ?`, ep); got != PodcastErrNotAudio {
		t.Errorf("failure = %q", got)
	}
}

// Concept 19 P-16: a file of the target name that appears before the entry stays as it is, and the
// episode gets the next name (the publication date in front); a name a missing row holds is taken
// as well.
func TestPodcastDownloadKeepsAForeignFile(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	foreign := []byte("Audiobookshelf war schneller")
	f.srv.set(func(s *enclosureServer) {
		s.before = func(_ http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/race.mp3" {
				if err := os.WriteFile(filepath.Join(f.dir, "Testpodcast", "Wettlauf.mp3"), foreign, 0o600); err != nil {
					t.Error(err)
				}
			}
			return false
		}
	})
	at := time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)
	ep := f.episode("Wettlauf", "race.mp3", at, podcastEpisodeMedia(t, 1))
	if err := f.download(f.e.ctx, ep, 1); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(f.dir, "Testpodcast", "Wettlauf.mp3")); err != nil || !bytes.Equal(got, foreign) {
		t.Errorf("the foreign file changed: %q %v", got, err)
	}
	if got := f.e.str(`SELECT rel_path FROM media_files WHERE item_id = ?`, ep); got != "Testpodcast/2026-11-09 Wettlauf.mp3" {
		t.Errorf("stored as %q", got)
	}
	// A missing row of another episode holds «Zweite.mp3»: the next name again.
	other := f.fileEpisode(f.podcast, "Testpodcast/Zweite.mp3", at)
	f.exec(`UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, other)
	ep2 := f.episode("Zweite", "zweite.mp3", at, podcastEpisodeMedia(t, 2))
	if err := f.download(f.e.ctx, ep2, 1); err != nil {
		t.Fatal(err)
	}
	if got := f.e.str(`SELECT rel_path FROM media_files WHERE item_id = ?`, ep2); got != "Testpodcast/2026-11-09 Zweite.mp3" {
		t.Errorf("stored as %q next to a missing row of the name", got)
	}
}

// Concept 19 P-17 (CLAUDE.md rule 10): the address of an enclosure, with a key in its path and its
// query, appears neither in the error the job stores, nor in its progress, nor in the failure code,
// nor in the log.
func TestPodcastDownloadKeepsTheAddressSecret(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	const secret = "s3cr3t-k3y"
	rec := &jobs.Recorder{}
	ctx := jobs.WithRecorder(f.e.ctx, rec)
	ep := f.episode("Privat", "priv/"+secret+".mp3?token="+secret, time.Date(2026, 11, 7, 6, 0, 0, 0, time.UTC), nil)
	var errs []string
	for attempt := 1; attempt <= 5; attempt++ {
		err := f.download(ctx, ep, attempt)
		if err == nil {
			t.Fatal("a missing enclosure loaded")
		}
		errs = append(errs, err.Error())
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	f.exec(`UPDATE items SET enclosure_url = ? WHERE id = ?`, dead.URL+"/"+secret+".mp3?token="+secret, ep)
	if err := f.download(ctx, ep, 5); err == nil {
		t.Fatal("a dead host loaded")
	} else {
		errs = append(errs, err.Error())
	}
	for _, e := range errs {
		if strings.Contains(e, secret) {
			t.Errorf("job error names the address: %s", e)
		}
	}
	for _, r := range rec.Reports {
		if strings.Contains(r.Note, secret) {
			t.Errorf("progress names the address: %s", r.Note)
		}
	}
	if got := f.e.str(`SELECT error FROM podcast_download_failures WHERE item_id = ?`, ep); got != PodcastErrEnclosureUnavailable {
		t.Errorf("failure = %q", got)
	}
	if strings.Contains(f.log.String(), secret) {
		t.Errorf("log names the address:\n%s", f.log.String())
	}
}

// The download waits for a folder that is not writable, without using an attempt, and fails with
// root_not_writable after 24 hours; nothing is written meanwhile.
func TestPodcastDownloadWaitsForTheFolder(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	ep := f.episode("Wartet", "wartet.mp3", time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC), podcastEpisodeMedia(t, 1))
	f.e.svc.podcastLink = func(*os.Root, string, string) error { return &os.LinkError{Op: "link", Err: errors.ErrUnsupported} }
	id, err := f.e.queue.Enqueue(f.e.ctx, jobs.NewJob{Type: JobPodcastDownload, IdempotencyKey: podcastDownloadKey(ep), Payload: json.RawMessage(`{"episodeId":"` + ep + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	claim := func() *jobs.Job {
		t.Helper()
		j, err := f.e.queue.Claim(f.e.ctx, "w", []string{JobPodcastDownload})
		if err != nil || j == nil {
			t.Fatalf("claim: %v %v", j, err)
		}
		return j
	}
	j := claim()
	var later *jobs.RetryLaterError
	if err := f.e.svc.handlePodcastDownload(f.e.ctx, j); !errors.As(err, &later) || later.After != podcastWaitRetry || later.Reason != PodcastErrRootNotWritable {
		t.Fatalf("first run = %v, want RetryLater in 30 min", err)
	}
	if err := f.e.queue.Defer(f.e.ctx, "w", id, 0); err != nil {
		t.Fatal(err)
	}
	f.e.clock.Advance(podcastWaitMax)
	j = claim()
	err = f.e.svc.handlePodcastDownload(f.e.ctx, j)
	var de *podcastDownloadError
	if !errors.As(err, &de) || de.code != PodcastErrRootNotWritable || !de.final {
		t.Fatalf("run after 24 hours = %v, want a final root_not_writable", err)
	}
	if names := dirNames(t, filepath.Join(f.dir, "Testpodcast")); len(names) != 1 {
		t.Errorf("folder while it was not writable: %v", names)
	}
	if got := f.e.str(`SELECT error FROM podcast_download_failures WHERE item_id = ?`, ep); got != PodcastErrRootNotWritable {
		t.Errorf("failure = %q", got)
	}
}

// The sweep removes own files older than a week that no active download holds (a test file of a
// folder check that was cut short as well), and keeps the others.
func TestPodcastLeftoverSweep(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	old := f.e.clock.Now().Add(-8 * 24 * time.Hour)
	active := f.e.svc.newID()
	if _, err := f.e.queue.Enqueue(f.e.ctx, jobs.NewJob{Type: JobPodcastDownload, IdempotencyKey: podcastDownloadKey(active)}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".slouchcat-" + active + ".part", ".slouchcat-01J9ZZZZZZZZZZZZZZZZZZZZZZ.part", ".slouchcat-recent.part", ".hidden",
		".slouchcat-schreibtest-01J9ZZZZZZZZZZZZZZZZZZZZZZ.link"} {
		writeFile(t, f.dir, "Testpodcast/"+name, "x")
		if name != ".slouchcat-recent.part" {
			if err := os.Chtimes(filepath.Join(f.dir, "Testpodcast", name), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.e.svc.sweepPodcastLeftovers(f.e.ctx); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(dirNames(t, filepath.Join(f.dir, "Testpodcast")), ",")
	if want := ".hidden,.slouchcat-" + active + ".part,.slouchcat-recent.part,Alte Folge.mp3"; got != want {
		t.Errorf("folder after the sweep = %s, want %s", got, want)
	}
}

// Concept 19, 5.4 and 6.2: a podcast folder below a writable root that the server may not write
// (Audiobookshelf made it with 0755 for another user) is no failure of the enclosure: the download
// waits for it (root_not_writable) without using an attempt, and the podcast is not savable, in
// the list as for one podcast. The list judges it by the rights of the folder, without writing.
func TestPodcastDownloadWaitsForAFolderItMayNotWrite(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	folder := filepath.Join(f.dir, "Testpodcast")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
	ep := f.episode("Gesperrt", "gesperrt.mp3", time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC), podcastEpisodeMedia(t, 1))
	id, err := f.e.queue.Enqueue(f.e.ctx, jobs.NewJob{Type: JobPodcastDownload, IdempotencyKey: podcastDownloadKey(ep), Payload: json.RawMessage(`{"episodeId":"` + ep + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	j, err := f.e.queue.Claim(f.e.ctx, "w", []string{JobPodcastDownload})
	if err != nil || j == nil || j.ID != id {
		t.Fatalf("claim: %v %v", j, err)
	}
	var later *jobs.RetryLaterError
	if err := f.e.svc.handlePodcastDownload(f.e.ctx, j); !errors.As(err, &later) || later.Reason != PodcastErrRootNotWritable {
		t.Errorf("download into a folder it may not write = %v, want RetryLater root_not_writable", err)
	}
	if err := f.e.svc.PodcastSavable(f.e.ctx, f.podcast); !errors.Is(err, model.ErrRootNotWritable) {
		t.Errorf("savable = %v, want root_not_writable", err)
	}
	all, err := f.e.svc.PodcastStates(f.e.ctx, "")
	if err != nil || all[f.podcast].Savable {
		t.Errorf("state in the list = %+v %v, want not savable", all[f.podcast], err)
	}
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if all, err := f.e.svc.PodcastStates(f.e.ctx, ""); err != nil || !all[f.podcast].Savable {
		t.Errorf("state after the fix = %+v %v", all[f.podcast], err)
	}
}
