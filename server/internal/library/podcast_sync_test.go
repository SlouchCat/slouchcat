package library

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// queuedDownloads lists the episodes with a queued or running podcast.download, sorted.
func (f *podcastFixture) queuedDownloads() []string {
	f.e.t.Helper()
	rows, err := f.e.db.Read.QueryContext(f.e.ctx, `SELECT payload FROM jobs WHERE type = ? AND state IN ('queued', 'running')`, JobPodcastDownload)
	if err != nil {
		f.e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			f.e.t.Fatal(err)
		}
		var p podcastDownloadPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			f.e.t.Fatal(err)
		}
		out = append(out, p.EpisodeID)
	}
	if err := rows.Err(); err != nil {
		f.e.t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func (f *podcastFixture) sync(p podcastSyncPayload) int {
	f.e.t.Helper()
	n, err := f.e.svc.syncPodcast(f.e.ctx, p)
	if err != nil {
		f.e.t.Fatal(err)
	}
	return n
}

func sorted(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

// episodes adds n feed episodes one day apart, the newest first, and returns their ids in that
// order.
func (f *podcastFixture) episodes(n int) []string {
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	ids := make([]string, n)
	for i := range n {
		ids[i] = f.feedEpisode("Folge "+strconv.Itoa(n-i), day.Add(time.Duration(n-i)*24*time.Hour), 0)
	}
	return ids
}

// Concept 19 P-3 and P-14: a podcast on «Nur Link» in a library on «Speichern» queues no planner
// for a fetch with new episodes, and the planner loads nothing for it; a new subscription on
// «Speichern / Nur neue Folgen» queues no planner either (its first fetch reports no new episodes).
func TestPodcastPlannerLinkAndNewSubscription(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(3)
	if err := f.e.svc.QueuePodcastSync(f.e.ctx, f.podcast, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.e.jobCount(JobPodcastSync); n != 0 {
		t.Errorf("a new subscription on «Nur neue Folgen» queued %d planner runs", n)
	}
	if err := f.e.svc.QueuePodcastSync(f.e.ctx, f.podcast, eps[:1]); err != nil {
		t.Fatal(err)
	}
	if n := f.e.jobCount(JobPodcastSync); n != 1 {
		t.Errorf("a fetch with a new episode on «Speichern» queued %d planner runs, want 1", n)
	}
	f.exec(`INSERT INTO podcast_settings (item_id, mode, updated_at) VALUES (?, 'link', 1)`, f.podcast)
	if err := f.e.svc.QueuePodcastSync(f.e.ctx, f.podcast, eps[1:2]); err != nil {
		t.Fatal(err)
	}
	if n := f.e.jobCount(JobPodcastSync); n != 1 {
		t.Errorf("a fetch of a podcast on «Nur Link» queued a planner run")
	}
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast, NewIDs: eps}); n != 0 || len(f.queuedDownloads()) != 0 {
		t.Errorf("the planner queued %d downloads for a podcast on «Nur Link»", n)
	}
	// On «Speichern / Nur neue Folgen» exactly the new episodes of the fetch are loaded.
	f.exec(`UPDATE podcast_settings SET mode = NULL WHERE item_id = ?`, f.podcast)
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast, NewIDs: eps[:1]}); n != 1 || !slices.Equal(f.queuedDownloads(), eps[:1]) {
		t.Errorf("new episodes queued %v, want %v", f.queuedDownloads(), eps[:1])
	}
}

// Concept 19 P-4 and 5.2: «Neue + letzte 3» loads the three newest episodes of the feed as far as
// they have no file; an episode among them with a file is not replaced by an older one.
func TestPodcastPlannerLastN(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'last_n', podcast_last_n = 3 WHERE id = ?`, f.lib.ID)
	eps := f.episodes(6)
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast, Changed: true}); n != 3 || !slices.Equal(f.queuedDownloads(), sorted(eps[:3]...)) {
		t.Fatalf("last 3 queued %v, want %v", f.queuedDownloads(), sorted(eps[:3]...))
	}
	g := newPodcastFixture(t)
	g.exec(`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'last_n', podcast_last_n = 3 WHERE id = ?`, g.lib.ID)
	geps := g.episodes(6)
	g.file(geps[0], "Testpodcast/Folge 6.mp3")
	if n := g.sync(podcastSyncPayload{PodcastID: g.podcast}); n != 2 || !slices.Equal(g.queuedDownloads(), sorted(geps[1:3]...)) {
		t.Errorf("last 3 with the newest stored queued %v, want %v", g.queuedDownloads(), sorted(geps[1:3]...))
	}
}

// Concept 19 P-5: «Alle Folgen» loads an episode never loaded but not one whose file was deleted
// by hand («entfernt», its row missing), nor one that failed within six hours or for the first
// time more than seven days ago, nor a video enclosure (O-3); «Laden» on the removed episode
// brings it back.
func TestPodcastPlannerAll(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'all' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(6)
	f.file(eps[0], "Testpodcast/Folge 6.mp3")
	f.exec(`UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, eps[0])
	now := f.e.clock.Now()
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'not_audio', ?, ?)`,
		eps[1], now.Add(-time.Hour).UnixMilli(), now.Add(-time.Hour).UnixMilli())
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'not_audio', ?, ?)`,
		eps[2], now.Add(-8*24*time.Hour).UnixMilli(), now.Add(-7*time.Hour).UnixMilli())
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'enclosure_unavailable', ?, ?)`,
		eps[3], now.Add(-2*24*time.Hour).UnixMilli(), now.Add(-7*time.Hour).UnixMilli())
	f.exec(`UPDATE items SET enclosure_url = 'https://cdn.example.test/video.mp4', enclosure_type = 'video/mp4' WHERE id = ?`, eps[4])
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast}); n != 2 || !slices.Equal(f.queuedDownloads(), sorted(eps[3], eps[5])) {
		t.Fatalf("all queued %v, want the retry %s and the never loaded %s", f.queuedDownloads(), eps[3], eps[5])
	}
	if n, err := f.e.svc.DownloadPodcastEpisodes(f.e.ctx, f.podcast, []string{eps[0]}); err != nil || n != 1 {
		t.Fatalf("«Laden» on the removed episode = %d %v", n, err)
	}
	if got := f.queuedDownloads(); !slices.Contains(got, eps[0]) {
		t.Errorf("«Laden» did not queue the removed episode: %v", got)
	}
	// «Fehlende nachladen» leaves out the removed episode as well, and queues every other one
	// without a file whatever failed before (a failure is tried again).
	g := newPodcastFixture(t)
	geps := g.episodes(3)
	g.file(geps[0], "Testpodcast/Folge 3.mp3")
	g.exec(`UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, geps[0])
	if n, err := g.e.svc.DownloadPodcastEpisodes(g.e.ctx, g.podcast, nil); err != nil || n != 2 || !slices.Equal(g.queuedDownloads(), sorted(geps[1:]...)) {
		t.Errorf("«Fehlende nachladen» = %d %v, queued %v", n, err, g.queuedDownloads())
	}
	if _, err := g.e.svc.DownloadPodcastEpisodes(g.e.ctx, g.e.svc.newID(), nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown podcast: %v", err)
	}
}

// Concept 19 P-5 (counters, 6.2): with «Gleichzeitig laden» 5, five downloads of five hosts load at
// once, five of one host one after the other; the library's number bounds them as well.
func TestPodcastSlots(t *testing.T) {
	t.Parallel()
	peak := func(limit int, hosts []string) int {
		t.Helper()
		slots := newPodcastSlots()
		var running, top atomic.Int32
		var wg sync.WaitGroup
		for _, h := range hosts {
			wg.Go(func() {
				release, err := slots.acquire(context.Background(), 1, limit, h)
				if err != nil {
					t.Error(err)
					return
				}
				n := running.Add(1)
				for {
					old := top.Load()
					if n <= old || top.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				release()
			})
		}
		wg.Wait()
		return int(top.Load())
	}
	if got := peak(5, []string{"a.test", "b.test", "c.test", "d.test", "e.test"}); got != 5 {
		t.Errorf("five hosts loaded %d at once, want 5", got)
	}
	if got := peak(5, []string{"a.test", "a.test", "a.test", "a.test", "a.test"}); got != 1 {
		t.Errorf("one host loaded %d at once, want 1", got)
	}
	if got := peak(2, []string{"a.test", "b.test", "c.test", "d.test", "e.test"}); got != 2 {
		t.Errorf("«Gleichzeitig laden» 2 loaded %d at once", got)
	}
	slots := newPodcastSlots()
	release, err := slots.acquire(context.Background(), 1, 1, "a.test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := slots.acquire(ctx, 1, 1, "b.test"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a waiting acquire ended with %v", err)
	}
	release()
	release() // a second release does nothing
	if r, err := slots.acquire(context.Background(), 1, 1, "b.test"); err != nil {
		t.Error(err)
	} else {
		r()
	}
}

// Concept 19 P-10 and 5.3: «Speichern» → «Nur Link» cancels the waiting downloads, those of the
// buttons included, and the stored files stay; a smaller scope cancels the waiting downloads that
// no longer belong to it but keeps those of new episodes and of the buttons.
func TestPodcastPlannerCancels(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'all' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(5)
	stored := f.file(eps[4], "Testpodcast/Folge 1.mp3")
	writeFile(t, f.dir, "Testpodcast/Folge 1.mp3", "audio")
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast, NewIDs: eps[:1]}); n != 4 {
		t.Fatalf("all queued %d, want 4", n)
	}
	// A smaller scope, «Neue + letzte 2»: the two newest stay, the two older ones go.
	f.exec(`UPDATE libraries SET podcast_scope = 'last_n', podcast_last_n = 2 WHERE id = ?`, f.lib.ID)
	f.sync(podcastSyncPayload{PodcastID: f.podcast, Changed: true})
	if got, want := f.queuedDownloads(), sorted(eps[0], eps[1]); !slices.Equal(got, want) {
		t.Errorf("after a smaller scope %v, want %v", got, want)
	}
	f.exec(`UPDATE libraries SET podcast_scope = 'new' WHERE id = ?`, f.lib.ID)
	f.sync(podcastSyncPayload{PodcastID: f.podcast, Changed: true})
	if got, want := f.queuedDownloads(), eps[:1]; !slices.Equal(got, want) {
		t.Errorf("after «Nur neue Folgen» %v, want the new episode %v", got, want)
	}
	if _, err := f.e.svc.DownloadPodcastEpisodes(f.e.ctx, f.podcast, []string{eps[3]}); err != nil {
		t.Fatal(err)
	}
	// «Nur Link»: every waiting download goes, the button's too; the stored file stays.
	f.exec(`UPDATE libraries SET podcast_mode = 'link' WHERE id = ?`, f.lib.ID)
	f.sync(podcastSyncPayload{PodcastID: f.podcast, Changed: true, Stop: true})
	if got := f.queuedDownloads(); len(got) != 0 {
		t.Errorf("downloads after «Nur Link»: %v", got)
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE id = ? AND missing_since IS NULL`, stored); n != 1 {
		t.Errorf("the stored file's row is gone")
	}
	if names := visible(t, f.dir+"/Testpodcast"); len(names) != 1 {
		t.Errorf("files after «Nur Link»: %v", names)
	}
}

// The schedule podcast.sync_all plans the podcasts on «Speichern» with last_n or all and those
// with a failed download to try again; not those on «Nur Link» or «Nur neue Folgen» without one.
func TestPodcastSyncAll(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(1)
	ids, err := f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2)
	if err != nil || len(ids) != 0 {
		t.Fatalf("«Nur neue Folgen» without failures planned %v %v", ids, err)
	}
	now := f.e.clock.Now()
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'enclosure_unavailable', ?, ?)`,
		eps[0], now.Add(-24*time.Hour).UnixMilli(), now.Add(-7*time.Hour).UnixMilli())
	if ids, err = f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2); err != nil || len(ids) != 1 {
		t.Fatalf("a retryable failure planned %v %v", ids, err)
	}
	if again, err := f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2); err != nil || !slices.Equal(again, ids) {
		t.Errorf("a second run = %v %v, want the waiting job %v", again, err, ids)
	}
	f.exec(`INSERT INTO podcast_settings (item_id, mode, updated_at) VALUES (?, 'link', 1)`, f.podcast)
	f.exec(`DELETE FROM jobs`)
	if ids, err = f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2); err != nil || len(ids) != 0 {
		t.Errorf("«Nur Link» planned %v %v", ids, err)
	}
}

// The setting of a library: validated, «Speichern» only on a folder that passes the check, the
// planner queued for its podcasts; the estimate counts what the setting would load once.
func TestPodcastLibrarySettingAndEstimate(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	eps := f.episodes(4)
	f.exec(`UPDATE items SET enclosure_length = NULL WHERE id = ?`, eps[3])
	f.fileEpisode(f.podcast, "Testpodcast/Alt.mp3", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	f.exec(`UPDATE media_files SET size = 5000`)
	writeFile(t, f.dir, "Testpodcast/Alt.mp3", "audio")
	bad := model.PodcastDownloads{Mode: "keep", Scope: "some", LastN: 0, Parallel: 6, RootID: new(int64(999))}
	var verr *model.ValidationError
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, bad); !errors.As(err, &verr) || len(verr.Fields) != 5 {
		t.Errorf("invalid setting: %v", err)
	}
	est, err := f.e.svc.EstimatePodcastDownloads(f.e.ctx, f.lib.ID, model.PodcastDownloadEstimateRequest{Mode: model.PodcastSave, Scope: model.PodcastScopeAll})
	if err != nil {
		t.Fatal(err)
	}
	if est.Episodes != 4 || est.Bytes != 3*1000+5000 || est.FreeBytes <= 0 {
		t.Errorf("estimate of «Alle Folgen» = %+v, want 4 episodes and 8000 bytes", est)
	}
	est, err = f.e.svc.EstimatePodcastDownloads(f.e.ctx, f.lib.ID, model.PodcastDownloadEstimateRequest{PodcastID: f.podcast,
		Mode: model.PodcastSave, Scope: model.PodcastScopeLastN, LastN: 2})
	if err != nil || est.Episodes != 2 {
		t.Errorf("estimate of a podcast's «Neue + letzte 2» = %+v %v", est, err)
	}
	if n := f.e.jobCount(JobPodcastDownload); n != 0 {
		t.Errorf("an estimate queued %d downloads", n)
	}
	f.e.svc.podcastLink = func(*os.Root, string, string) error { return errors.ErrUnsupported }
	save := model.PodcastDownloads{Mode: model.PodcastSave, Scope: model.PodcastScopeLastN, LastN: 2, Parallel: 3}
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save); !errors.Is(err, model.ErrRootNotWritable) {
		t.Errorf("«Speichern» on a folder without hard links: %v", err)
	}
	if err := f.e.svc.PodcastSavable(f.e.ctx, f.podcast); !errors.Is(err, model.ErrRootNotWritable) {
		t.Errorf("podcast savable without hard links: %v", err)
	}
	f.e.svc.podcastLink = nil
	got, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save)
	if err != nil || got.Mode != model.PodcastSave || got.LastN != 2 || got.Parallel != 3 || got.RootID != nil {
		t.Fatalf("stored setting = %+v %v", got, err)
	}
	if n := f.e.jobCount(JobPodcastSync); n != 1 {
		t.Errorf("planner runs after the setting = %d, want 1", n)
	}
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, 999, save); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown library: %v", err)
	}
}

// Concept 19 P-11: while 200 episodes load in the pool podcast, a download of a device in the pool
// download starts without waiting for them.
func TestPodcastPoolLeavesDeviceDownloadsAlone(t *testing.T) {
	t.Parallel()
	f := newDownloadFixture(t)
	block := make(chan struct{})
	defer close(block)
	f.srv.set(func(s *enclosureServer) {
		s.before = func(w http.ResponseWriter, r *http.Request) bool {
			w.Header().Set("Content-Length", "1000000")
			w.(http.Flusher).Flush()
			select {
			case <-block:
			case <-r.Context().Done():
			}
			return true
		}
	})
	at := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	for i := range 200 {
		ep := f.episode("Folge "+strconv.Itoa(i), "ep"+strconv.Itoa(i)+".mp3", at.Add(time.Duration(i)*time.Hour), nil)
		if _, err := f.e.svc.enqueuePodcastDownload(f.e.ctx, podcastDownloadPayload{EpisodeID: ep}, prioPodcastDownload); err != nil {
			t.Fatal(err)
		}
	}
	r := jobs.NewRegistry(f.e.queue, discard)
	f.e.svc.RegisterTypes(r)
	device := make(chan struct{}, 1)
	r.Register(jobs.Type{Name: "download.prepare", Group: jobs.GroupDelivery, Pool: jobs.PoolDownload, Trigger: jobs.TriggerEvent, Priority: 5,
		Handler: func(context.Context, *jobs.Job) error { device <- struct{}{}; return nil }})
	ctx, cancel := context.WithCancel(f.e.ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()
	for i := range 5 {
		w := jobs.NewWorker(f.e.queue, "podcast-"+strconv.Itoa(i+1), discard)
		r.Attach(w, jobs.PoolPodcast)
		wg.Go(func() { w.Run(ctx) })
	}
	w := jobs.NewWorker(f.e.queue, "download-1", discard)
	r.Attach(w, jobs.PoolDownload)
	wg.Go(func() { w.Run(ctx) })
	deadline := time.Now().Add(10 * time.Second)
	for f.e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'running'`, JobPodcastDownload) < 5 {
		if time.Now().After(deadline) {
			t.Fatal("the five podcast workers did not take up downloads")
		}
		time.Sleep(20 * time.Millisecond)
	}
	start := time.Now()
	if _, err := f.e.queue.Enqueue(f.e.ctx, jobs.NewJob{Type: "download.prepare", Priority: 5}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-device:
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("the device download waited %v", d)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the device download did not start while podcast episodes loaded")
	}
	if n := f.e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobPodcastDownload); n < 190 {
		t.Errorf("queued podcast downloads = %d, want the rest of the 200 still waiting", n)
	}
}

// syncQueued runs every queued podcast.sync job through its handler, the way a worker would.
func (f *podcastFixture) syncQueued() {
	f.e.t.Helper()
	for {
		j, err := f.e.queue.Claim(f.e.ctx, "w", []string{JobPodcastSync})
		if err != nil {
			f.e.t.Fatal(err)
		}
		if j == nil {
			return
		}
		if err := f.e.svc.handlePodcastSync(f.e.ctx, j); err != nil {
			f.e.t.Fatal(err)
		}
		if err := f.e.queue.Complete(f.e.ctx, "w", j.ID); err != nil {
			f.e.t.Fatal(err)
		}
	}
}

// Concept 19, 6.2 («Fehler»): podcast.sync_all tries a failed episode of a podcast on «Speichern /
// Nur neue Folgen» again six hours after its last failure, for seven days: the planner run it
// queues loads the episode although it is no new episode of a fetch.
func TestPodcastSyncAllRetriesAFailureOnScopeNew(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(2)
	now := f.e.clock.Now()
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'enclosure_unavailable', ?, ?)`,
		eps[1], now.Add(-24*time.Hour).UnixMilli(), now.Add(-7*time.Hour).UnixMilli())
	if ids, err := f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2); err != nil || len(ids) != 1 {
		t.Fatalf("sync_all planned %v %v", ids, err)
	}
	f.syncQueued()
	if got := f.queuedDownloads(); !slices.Equal(got, eps[1:]) {
		t.Errorf("downloads after sync_all = %v, want the failed episode %v", got, eps[1:])
	}
}

// Concept 19, 6.2: a final failure (too_large, not_audio, unsupported_format) is never tried again
// by the planner or the schedule, only by a button; «Fehlende nachladen» also takes an episode that
// failed within six hours (the button passes over the lock of the failures).
func TestPodcastFinalFailuresOnlyByButton(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.exec(`UPDATE libraries SET podcast_mode = 'save', podcast_scope = 'all' WHERE id = ?`, f.lib.ID)
	eps := f.episodes(4)
	now := f.e.clock.Now()
	for i, code := range []string{PodcastErrTooLarge, PodcastErrNotAudio, PodcastErrUnsupportedFormat} {
		f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, ?, ?, ?)`,
			eps[i], code, now.Add(-24*time.Hour).UnixMilli(), now.Add(-7*time.Hour).UnixMilli())
	}
	f.exec(`INSERT INTO podcast_download_failures (item_id, error, first_failed_at, last_failed_at) VALUES (?, 'enclosure_unavailable', ?, ?)`,
		eps[3], now.Add(-time.Hour).UnixMilli(), now.Add(-time.Hour).UnixMilli())
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast}); n != 0 {
		t.Errorf("the planner queued %d downloads of final or recent failures: %v", n, f.queuedDownloads())
	}
	f.exec(`UPDATE libraries SET podcast_scope = 'new' WHERE id = ?`, f.lib.ID)
	if ids, err := f.e.svc.runPodcastSyncAll(f.e.ctx, f.e.queue, 2); err != nil || len(ids) != 0 {
		t.Errorf("sync_all planned %v %v for final failures", ids, err)
	}
	if n, err := f.e.svc.DownloadPodcastEpisodes(f.e.ctx, f.podcast, nil); err != nil || n != 4 || !slices.Equal(f.queuedDownloads(), sorted(eps...)) {
		t.Errorf("«Fehlende nachladen» = %d %v, queued %v, want all four", n, err, f.queuedDownloads())
	}
}

// Concept 19, 5.3: a change of the setting while «Nur Link» holds («Gleichzeitig laden») leaves the
// downloads of the buttons alone, waiting and running ones; only the change «Speichern» → «Nur
// Link» cancels them.
func TestPodcastSettingChangeKeepsButtonDownloads(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	eps := f.episodes(3)
	link := model.PodcastDownloads{Mode: model.PodcastLink, Scope: model.PodcastScopeNew, LastN: 10, Parallel: 2}
	if n, err := f.e.svc.DownloadPodcastEpisodes(f.e.ctx, f.podcast, eps[:2]); err != nil || n != 2 {
		t.Fatalf("«Laden» = %d %v", n, err)
	}
	running, err := f.e.queue.Claim(f.e.ctx, "w", []string{JobPodcastDownload})
	if err != nil || running == nil {
		t.Fatalf("claim: %v %v", running, err)
	}
	link.Parallel = 3
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, link); err != nil {
		t.Fatal(err)
	}
	f.syncQueued()
	if got := f.queuedDownloads(); !slices.Equal(got, sorted(eps[:2]...)) {
		t.Errorf("downloads after «Gleichzeitig laden» on «Nur Link» = %v, want both of «Laden»", got)
	}
	save := link
	save.Mode = model.PodcastSave
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save); err != nil {
		t.Fatal(err)
	}
	f.syncQueued()
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, link); err != nil {
		t.Fatal(err)
	}
	f.syncQueued()
	if got := f.queuedDownloads(); len(got) != 0 {
		t.Errorf("downloads after «Speichern» → «Nur Link» = %v, want none", got)
	}
}

// Concept 19, 5.4: the folder check is asked when a library changes to «Speichern» (or to another
// folder); while «Speichern» holds, the other values still change when the check fails.
func TestPodcastSettingChecksTheFolderOnlyOnSave(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	save := model.PodcastDownloads{Mode: model.PodcastSave, Scope: model.PodcastScopeNew, LastN: 10, Parallel: 2}
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save); err != nil {
		t.Fatal(err)
	}
	f.e.svc.podcastLink = func(*os.Root, string, string) error { return errors.ErrUnsupported }
	save.Parallel, save.LastN = 4, 20
	if got, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save); err != nil || got.Parallel != 4 || got.LastN != 20 {
		t.Errorf("«Gleichzeitig laden» on «Speichern» with a failing folder = %+v %v", got, err)
	}
	link := save
	link.Mode = model.PodcastLink
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, link); err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.svc.SetPodcastDownloads(f.e.ctx, f.lib.ID, save); !errors.Is(err, model.ErrRootNotWritable) {
		t.Errorf("«Nur Link» → «Speichern» with a failing folder: %v", err)
	}
}
