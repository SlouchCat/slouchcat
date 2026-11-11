package library

import (
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Concept 19 part A, 12.2 (contract 1.29.0): the page «Podcast» lists every live episode of a
// podcast, from the feed and from the folder, with a state the server derives -- a file that is
// there, a running or waiting download, a failure with its next attempt, file rows that are all
// missing, the riegel's «unklar», else «Nur Link» -- and counts the three views.
func TestPodcastEpisodeList(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	now := f.e.clock.Now()
	day := now.Add(-60 * 24 * time.Hour).Truncate(time.Hour)
	at := func(d int) time.Time { return day.Add(time.Duration(d) * 24 * time.Hour) }

	stored := f.feedEpisode("Gespeichert", at(10), 0)
	f.file(stored, "Testpodcast/Gespeichert.mp3")
	link := f.feedEpisode("Nur ein Link", at(9), 0)
	queued := f.feedEpisode("Wartet", at(8), 0)
	running := f.feedEpisode("Lädt", at(7), 0)
	failed := f.feedEpisode("Fehlgeschlagen", at(6), 0)
	final := f.feedEpisode("Kein Audio", at(5), 0)
	removed := f.feedEpisode("Entfernt", at(4), 0)
	f.file(removed, "Testpodcast/Entfernt.mp3")
	f.exec(`UPDATE media_files SET missing_since = 1 WHERE item_id = ?`, removed)
	unclear := f.feedEpisode("Interview", at(3), 0)
	f.fileEpisode(f.podcast, "Testpodcast/a.mp3", at(3).Add(10*time.Second))
	f.fileEpisode(f.podcast, "Testpodcast/b.mp3", at(3).Add(20*time.Second))
	undated := f.feedEpisode("Ohne Datum", at(0), 0)
	f.exec(`UPDATE items SET published_at = NULL WHERE id = ?`, undated)

	for _, id := range []string{queued, running} {
		if _, err := f.e.svc.enqueuePodcastDownload(f.e.ctx, podcastDownloadPayload{EpisodeID: id, Manual: true}, prioPodcastDownloadEpisode); err != nil {
			t.Fatal(err)
		}
	}
	f.exec(`UPDATE jobs SET state = 'running', progress_done = 300, progress_total = 1000 WHERE idempotency_key = ?`, podcastDownloadKey(running))
	write := systemq.New(f.e.db.Write)
	last := now.Add(-time.Hour)
	for id, code := range map[string]string{failed: "enclosure_unavailable", final: "not_audio"} {
		if err := write.PodcastRecordDownloadFailure(f.e.ctx, systemq.PodcastRecordDownloadFailureParams{ItemID: id, Error: code, Now: last.UnixMilli()}); err != nil {
			t.Fatal(err)
		}
	}

	page, err := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, model.PodcastEpisodesAll, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	// Nine feed episodes and the two file episodes of the folder (stored, newest of their day).
	if page.All != 11 || page.WithFile != 3 || page.Open != 8 || page.Total != 11 || len(page.Episodes) != 11 {
		t.Fatalf("counts = all %d, with file %d, open %d, total %d, page %d", page.All, page.WithFile, page.Open, page.Total, len(page.Episodes))
	}
	got := map[string]model.AdminPodcastEpisode{}
	for _, e := range page.Episodes {
		got[e.ID] = e
	}
	for id, want := range map[string]string{
		stored: model.PodcastEpisodeFile, link: model.PodcastEpisodeLink, queued: model.PodcastEpisodeQueued,
		running: model.PodcastEpisodeRunning, failed: model.PodcastEpisodeFailed, final: model.PodcastEpisodeFailed,
		removed: model.PodcastEpisodeRemoved, unclear: model.PodcastEpisodeUnclear, undated: model.PodcastEpisodeLink,
	} {
		if got[id].State != want {
			t.Errorf("state of %s = %q, want %q", got[id].Title, got[id].State, want)
		}
	}
	if e := got[stored]; e.FileName != "Gespeichert.mp3" || e.SizeBytes == nil || *e.SizeBytes != 1 || !e.Feed {
		t.Errorf("stored episode = %+v", e)
	}
	if e := got[running]; e.Progress == nil || *e.Progress != 300 || e.Total == nil || *e.Total != 1000 {
		t.Errorf("running episode = %+v", e)
	}
	if e := got[failed]; e.ErrorCode != "enclosure_unavailable" || e.RetryAt == nil || !e.RetryAt.Equal(last.Add(podcastRetryAfter).Truncate(time.Millisecond)) {
		t.Errorf("failed episode = %+v", e)
	}
	if e := got[final]; e.ErrorCode != "not_audio" || e.RetryAt != nil {
		t.Errorf("final failure = %+v", e)
	}
	if e := got[removed]; e.FileName != "Entfernt.mp3" {
		t.Errorf("removed episode = %+v", e)
	}
	// Newest first, the undated one last; the file episodes of the folder are «Gespeichert».
	if first, lastEp := page.Episodes[0], page.Episodes[len(page.Episodes)-1]; first.ID != stored || lastEp.ID != undated || lastEp.PublishedAt != nil {
		t.Errorf("order: first %s, last %s", first.Title, lastEp.Title)
	}
	for _, e := range page.Episodes {
		if !e.Feed && e.State != model.PodcastEpisodeFile {
			t.Errorf("file episode %s = %q", e.Title, e.State)
		}
	}

	// The views and the paging.
	files, err := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, model.PodcastEpisodesWithFile, 0, 50)
	if err != nil || files.Total != 3 || len(files.Episodes) != 3 || files.All != 11 {
		t.Errorf("view with file = %+v %v", files, err)
	}
	open, err := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, model.PodcastEpisodesWithoutFile, 2, 3)
	if err != nil || open.Total != 8 || len(open.Episodes) != 3 || open.Offset != 2 || open.Limit != 3 || open.Episodes[0].ID != running {
		t.Errorf("view without file, offset 2 = %+v %v", open, err)
	}
	if beyond, err := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, "", 50, 10); err != nil || len(beyond.Episodes) != 0 || beyond.Total != 11 {
		t.Errorf("page beyond the end = %+v %v", beyond, err)
	}
	var verr *model.ValidationError
	if _, err := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, "open", 0, 10); !errors.As(err, &verr) {
		t.Errorf("unknown view: %v", err)
	}
	if _, err := f.e.svc.PodcastEpisodeList(f.e.ctx, stored, "", 0, 10); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("an episode as podcast: %v", err)
	}
	f.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, link)
	if page, _ := f.e.svc.PodcastEpisodeList(f.e.ctx, f.podcast, "", 0, 50); page.All != 10 {
		t.Errorf("a deleted episode is listed: %d", page.All)
	}
}

// The section «Downloads» (12.3): the running downloads first, then the waiting ones in claim
// order, with the titles of episode and podcast; «Abbrechen» ends one, an episode of another
// podcast is not found, an episode without download is left alone; the pause of the job type
// shows.
func TestPodcastDownloadQueueAndCancel(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	a := f.feedEpisode("Erste", day, 0)
	b := f.feedEpisode("Zweite", day.Add(24*time.Hour), 0)
	c := f.feedEpisode("Dritte", day.Add(48*time.Hour), 0)
	for _, id := range []string{a, b, c} {
		if _, err := f.e.svc.enqueuePodcastDownload(f.e.ctx, podcastDownloadPayload{EpisodeID: id}, prioPodcastDownload); err != nil {
			t.Fatal(err)
		}
	}
	f.exec(`UPDATE jobs SET state = 'running', progress_done = 5 WHERE idempotency_key = ?`, podcastDownloadKey(c))

	q, err := f.e.svc.PodcastDownloadQueue(f.e.ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if q.Total != 3 || q.Running != 1 || q.Paused || len(q.Downloads) != 3 {
		t.Fatalf("queue = %+v", q)
	}
	if d := q.Downloads[0]; d.EpisodeID != c || d.State != model.PodcastEpisodeRunning || d.Progress == nil || *d.Progress != 5 ||
		d.PodcastID != f.podcast || d.PodcastTitle != "Testpodcast" || d.EpisodeTitle != "Dritte" || d.LibraryID != f.lib.ID ||
		d.SizeBytes == nil || *d.SizeBytes != 1000 {
		t.Errorf("first download = %+v", d)
	}
	if q.Downloads[1].EpisodeID != a || q.Downloads[2].EpisodeID != b || q.Downloads[1].State != model.PodcastEpisodeQueued {
		t.Errorf("order = %s %s", q.Downloads[1].EpisodeTitle, q.Downloads[2].EpisodeTitle)
	}
	if short, err := f.e.svc.PodcastDownloadQueue(f.e.ctx, 1); err != nil || short.Total != 3 || len(short.Downloads) != 1 {
		t.Errorf("queue limited to 1 = %+v %v", short, err)
	}

	if err := f.e.svc.CancelPodcastDownload(f.e.ctx, f.podcast, a); err != nil {
		t.Fatal(err)
	}
	if n := f.e.count(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'cancelled'`, podcastDownloadKey(a)); n != 1 {
		t.Errorf("cancelled downloads of the first episode = %d", n)
	}
	if err := f.e.svc.CancelPodcastDownload(f.e.ctx, f.podcast, a); err != nil {
		t.Errorf("a second cancel: %v", err)
	}
	other := f.e.svc.newID()
	f.exec(`INSERT INTO items (id, kind, library_id, title, sort_title, feed_url, feed_origin, providers, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'podcast', ?, 'Anderer', 'anderer', 'https://feeds.example.test/other.xml', 'admin', '["local"]', 'matched', 1, 1, 1)`, other, f.lib.ID)
	if err := f.e.svc.CancelPodcastDownload(f.e.ctx, other, b); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("an episode of another podcast: %v", err)
	}
	if err := f.e.svc.CancelPodcastDownload(f.e.ctx, unknownPodcastID, b); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("an unknown podcast: %v", err)
	}

	f.exec(`INSERT INTO job_types (type, paused_at, updated_at) VALUES (?, 1, 1)
		ON CONFLICT (type) DO UPDATE SET paused_at = 1`, JobPodcastDownload)
	if q, err := f.e.svc.PodcastDownloadQueue(f.e.ctx, 100); err != nil || !q.Paused || q.Total != 2 {
		t.Errorf("queue after cancel and pause = %+v %v", q, err)
	}
	// A deleted podcast takes its downloads out of the queue.
	f.exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, f.podcast)
	if q, err := f.e.svc.PodcastDownloadQueue(f.e.ctx, 100); err != nil || q.Total != 0 || len(q.Downloads) != 0 {
		t.Errorf("queue of a deleted podcast = %+v %v", q, err)
	}
}

const unknownPodcastID = "01J0000000000000000000XXXX"
