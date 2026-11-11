package library

import (
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Two copies of one episode in a podcast folder (Audiobookshelf loaded it twice, once with its
// uuid suffix) are probed at the same time, as the probe pool does after a folder is added: both
// read the feed episode as still without file before either pairs. Only the first may pair; the
// second stays a file episode (file_and_feed on the maintenance page). Seen on CT 190 on
// 11 November 2026: three feed episodes held two files each after the first scan of the folder.
func TestPodcastPairInParallelPairsOneFile(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	feed := f.feedEpisode("Anwalt – Darf man jeden vertreten?", day, 0)
	files := []string{
		f.fileEpisode(f.podcast, "Testpodcast/Anwalt – Darf man jeden vertreten.mp3", day.Add(-30*24*time.Hour)),
		f.fileEpisode(f.podcast, "Testpodcast/Anwalt – Darf man jeden vertreten (1b59fa6f-e1ba-4be6-b280-23e1b2f7a73f).mp3", day.Add(-30*24*time.Hour)),
	}
	pr := &transcode.Probe{Format: transcode.Format{Tags: map[string]string{"title": "Anwalt – Darf man jeden vertreten?"}}}

	// Hold the only write connection, so both runs read their candidates and then wait for it.
	hold, err := f.e.db.Write.BeginTx(f.e.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	waits := f.e.db.Write.Stats().WaitCount
	var wg sync.WaitGroup
	errs := make([]error, len(files))
	for i, id := range files {
		item, err := systemq.New(f.e.db.Read).GetItemSystem(f.e.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f.e.svc.finishPodcastEpisode(f.e.ctx, item, systemq.MediaFile{ID: "mf-" + id}, pr)
		}()
	}
	deadline := time.Now().Add(10 * time.Second)
	for f.e.db.Write.Stats().WaitCount-waits < int64(len(files)) {
		if time.Now().After(deadline) {
			t.Fatal("the runs never waited for the write connection")
		}
		time.Sleep(time.Millisecond)
	}
	if err := hold.Rollback(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("finish %s: %v", files[i], err)
		}
	}

	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, feed); n != 1 {
		t.Errorf("files of the feed episode = %d, want 1", n)
	}
	if n := f.e.count(`SELECT count(*) FROM items WHERE id IN (?, ?) AND deleted_at IS NULL`, files[0], files[1]); n != 1 {
		t.Errorf("live file episodes = %d, want 1 (the one that came second)", n)
	}
	if n := f.e.count(`SELECT count(*) FROM items i WHERE i.id IN (?, ?) AND i.deleted_at IS NULL
		AND EXISTS (SELECT 1 FROM media_files m WHERE m.item_id = i.id)`, files[0], files[1]); n != 1 {
		t.Errorf("the file episode that stays has no file (%d)", n)
	}
}
