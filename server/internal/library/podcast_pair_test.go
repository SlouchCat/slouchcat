package library

import (
	"database/sql"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Tests of the pairing after a feed check (addendum of 16 November 2026 to ADR 0029): the pairs the
// probe left behind because the feed's duration is not the file's, found on CT 190 as 5'023
// file_and_feed groups on the maintenance page.

// probedFile is a file episode as the probe leaves it: title and date from the tags, the duration
// of the file.
func (f *podcastFixture) probedFile(title string, at time.Time, durationMs int64) string {
	f.e.t.Helper()
	id := f.fileEpisode(f.podcast, "Testpodcast/"+title+" "+f.e.svc.newID()+".mp3", at)
	f.exec(`UPDATE items SET title = ?, sort_title = ?, duration_ms = ? WHERE id = ?`,
		title, SortTitle(title), sql.NullInt64{Int64: durationMs, Valid: durationMs > 0}, id)
	return id
}

// fileAndFeed lists the pairs the maintenance page shows as file_and_feed (listDuplicates), as
// "file title/feed id".
func (f *podcastFixture) fileAndFeed() []string {
	f.e.t.Helper()
	rows, err := systemq.New(f.e.db.Read).AdminListFileAndFeedEpisodes(f.e.ctx, sql.NullInt64{})
	if err != nil {
		f.e.t.Fatal(err)
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.FileTitle + "/" + r.FeedID
	}
	slices.Sort(out)
	return out
}

func TestPairPodcastEpisodesTakesUpThePairsTheProbeLeft(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	f.e.profiles(2)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)

	// The measured case: the same minute and title, the feed says 581 s, the file runs 548.5 s.
	godzillaFeed := f.feedEpisode("Godzilla", day, 581_000)
	godzillaFile := f.probedFile("Godzilla", day.Add(20*time.Second), 548_544)
	// Only the title, on another day, and the durations contradict: not the same episode by rule.
	mothraFeed := f.feedEpisode("Mothra", day.Add(24*time.Hour), 600_000)
	mothraFile := f.probedFile("Mothra", day.Add(26*time.Hour), 540_000)
	// The feed lists the episode twice (two guids) and the durations do not tell them apart.
	rodanFeedA := f.feedEpisode("Rodan", day.Add(48*time.Hour), 300_000)
	rodanFeedB := f.feedEpisode("Rodan", day.Add(48*time.Hour), 300_000)
	rodanFile := f.probedFile("Rodan", day.Add(48*time.Hour), 300_100)
	// A repeat months later with the same audio: the file belongs to the episode of its minute.
	gameraFeed := f.feedEpisode("Gamera", day.Add(72*time.Hour), 3_000_000)
	gameraRepeat := f.feedEpisode("Gamera", day.Add(90*24*time.Hour), 3_000_000)
	gameraFile := f.probedFile("Gamera", day.Add(72*time.Hour), 3_000_400)
	// Two copies of one episode in the folder: neither is the feed episode's file by rule.
	ebirahFeed := f.feedEpisode("Ebirah", day.Add(96*time.Hour), 0)
	ebirahFiles := []string{f.probedFile("Ebirah", day.Add(96*time.Hour), 0), f.probedFile("Ebirah", day.Add(96*time.Hour), 0)}

	// Profile 1 heard the feed episode to the end and began the file episode later; profile 2 only
	// knows the file episode.
	f.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, watched_at) VALUES (1, ?, 581000, 581000, 1, 10, 10)`, godzillaFeed)
	f.e.progress(1, godzillaFile, 300_000, false)
	f.e.progress(2, godzillaFile, 120_000, false)
	f.e.progressEvent(godzillaFile)
	f.exec(`INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays) VALUES (2, ?, '', 10, 120000, 1)`, godzillaFile)
	f.exec(`INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (2, ?, 1, 1)`, godzillaFile)

	want := []string{
		"Ebirah/" + ebirahFeed, "Ebirah/" + ebirahFeed,
		"Gamera/" + gameraFeed, "Gamera/" + gameraRepeat,
		"Godzilla/" + godzillaFeed,
		"Mothra/" + mothraFeed,
		"Rodan/" + rodanFeedA, "Rodan/" + rodanFeedB,
	}
	slices.Sort(want)
	if got := f.fileAndFeed(); !slices.Equal(got, want) {
		t.Fatalf("file_and_feed before = %v, want %v", got, want)
	}

	n, err := f.e.svc.PairPodcastEpisodes(f.e.ctx, f.podcast)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("paired = %d, want 2 (Godzilla, Gamera)", n)
	}

	// The feed episodes took the files and kept guid and enclosure; the file episodes are gone.
	for _, p := range []struct{ name, feed, file string }{{"Godzilla", godzillaFeed, godzillaFile}, {"Gamera", gameraFeed, gameraFile}} {
		if v := f.e.str(`SELECT (guid IS NOT NULL) || '/' || (enclosure_url IS NOT NULL) || '/' || (deleted_at IS NULL) || '/' ||
			(SELECT count(*) FROM media_files WHERE item_id = items.id) FROM items WHERE id = ?`, p.feed); v != "1/1/1/1" {
			t.Errorf("%s: feed episode (guid/enclosure/live/files) = %s, want 1/1/1/1", p.name, v)
		}
		if n := f.e.count(`SELECT count(*) FROM items WHERE id = ?`, p.file); n != 0 {
			t.Errorf("%s: the file episode is still there", p.name)
		}
	}
	if n := f.e.count(`SELECT count(*) FROM media_files WHERE item_id = ?`, gameraRepeat); n != 0 {
		t.Errorf("the repeat took %d files, want none", n)
	}

	// Nothing the profiles did is lost (the rules of the duplicate merge): profile 1 keeps
	// «Gehört» with its date, profile 2's progress, day, event and watchlist entry moved.
	if v := f.e.str(`SELECT completed || '/' || position_ms || '/' || watched_at FROM progress WHERE profile_id = 1 AND item_id = ?`, godzillaFeed); v != "1/581000/10" {
		t.Errorf("progress of profile 1 (completed/position/watched_at) = %s, want 1/581000/10", v)
	}
	if v := f.e.str(`SELECT completed || '/' || position_ms FROM progress WHERE profile_id = 2 AND item_id = ?`, godzillaFeed); v != "0/120000" {
		t.Errorf("progress of profile 2 (completed/position) = %s, want 0/120000", v)
	}
	for _, table := range []string{"progress", "progress_events", "progress_days", "watchlist"} {
		if n := f.e.count(`SELECT count(*) FROM `+table+` WHERE item_id = ?`, godzillaFile); n != 0 {
			t.Errorf("%s rows left on the file episode = %d, want 0", table, n)
		}
	}
	if n := f.e.count(`SELECT (SELECT count(*) FROM progress_events WHERE item_id = ?1) + (SELECT count(*) FROM progress_days WHERE item_id = ?1)
		+ (SELECT count(*) FROM watchlist WHERE item_id = ?1)`, godzillaFeed); n != 3 {
		t.Errorf("event, day and watchlist entry on the feed episode = %d, want 3", n)
	}

	// The maintenance page lists only what is unclear by rule.
	want = []string{
		"Ebirah/" + ebirahFeed, "Ebirah/" + ebirahFeed,
		"Mothra/" + mothraFeed,
		"Rodan/" + rodanFeedA, "Rodan/" + rodanFeedB,
	}
	slices.Sort(want)
	if got := f.fileAndFeed(); !slices.Equal(got, want) {
		t.Errorf("file_and_feed after = %v, want %v", got, want)
	}
	for _, id := range append([]string{mothraFile, rodanFile}, ebirahFiles...) {
		if n := f.e.count(`SELECT count(*) FROM items AS i WHERE i.id = ? AND i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)`, id); n != 1 {
			t.Errorf("unclear file episode %s no longer holds its file", id)
		}
	}

	// A second run finds nothing: the step is safe after every feed check.
	if n, err := f.e.svc.PairPodcastEpisodes(f.e.ctx, f.podcast); err != nil || n != 0 {
		t.Errorf("second run paired %d (%v), want 0", n, err)
	}
}

// The duration decides between two feed episodes that share minute and title with a file.
func TestPairPodcastEpisodesLetsTheDurationDecide(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	short := f.feedEpisode("Teil", day, 600_000)
	long := f.feedEpisode("Teil", day, 1_800_000)
	file := f.probedFile("Teil", day, 1_795_000)
	if n, err := f.e.svc.PairPodcastEpisodes(f.e.ctx, f.podcast); err != nil || n != 1 {
		t.Fatalf("paired %d (%v), want 1", n, err)
	}
	if v := f.e.str(`SELECT item_id FROM media_files WHERE rel_path LIKE 'Testpodcast/Teil %'`); v != long {
		t.Errorf("the file hangs on %s, want the episode of its length %s (not %s, file episode %s)", v, long, short, file)
	}
}

// What the pairing leaves for the planner (riegel 2 and 3): an episode with a file near it is
// never loaded, whether the rule pairs it or not, and a paired episode has its file.
func TestPairPodcastEpisodesAndThePlannerLoadNothingTwice(t *testing.T) {
	t.Parallel()
	f := newPodcastFixture(t)
	day := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	paired := f.feedEpisode("Godzilla", day, 581_000)
	f.probedFile("Godzilla", day, 548_544) // its file name is not the title: riegel 1 does not see it
	unclear := f.feedEpisode("Mothra kehrt zurück", day.Add(24*time.Hour), 600_000)
	f.probedFile("Mothra", day.Add(24*time.Hour), 540_000) // the same minute, another title, another length
	free := f.feedEpisode("Rodan", day.Add(72*time.Hour), 300_000)
	f.exec(`UPDATE libraries SET podcast_mode = ?, podcast_scope = ? WHERE id = ?`, model.PodcastSave, model.PodcastScopeAll, f.lib.ID)

	// Before the pairing ran (the planner of a setting changed right after the update): riegel 2
	// calls the pair of the new rule unclear, it is not loaded.
	verdicts := func() map[string]verdict {
		t.Helper()
		eps, err := systemq.New(f.e.db.Read).PodcastFeedEpisodes(f.e.ctx, sql.NullString{String: f.podcast, Valid: true})
		if err != nil {
			t.Fatal(err)
		}
		var without []systemq.PodcastFeedEpisodesRow
		for _, e := range eps {
			if e.Files == 0 {
				without = append(without, e)
			}
		}
		got, err := f.e.svc.judgePodcastEpisodes(f.e.ctx, f.settings(), without)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]verdict{}
		for id, v := range got {
			out[id] = v.verdict
		}
		return out
	}
	if got := verdicts(); got[paired] != verdictUnclear || got[unclear] != verdictUnclear || got[free] != verdictLoad {
		t.Fatalf("verdicts before the pairing = %v, want unclear, unclear, load", got)
	}

	if n, err := f.e.svc.PairPodcastEpisodes(f.e.ctx, f.podcast); err != nil || n != 1 {
		t.Fatalf("paired %d (%v), want 1", n, err)
	}
	got := verdicts()
	if _, judged := got[paired]; judged {
		t.Errorf("the paired episode is still judged as without a file")
	}
	if got[unclear] != verdictUnclear || got[free] != verdictLoad {
		t.Errorf("verdicts after the pairing = %v, want unclear for the episode with a file near it and load for the free one", got)
	}
	// The planner queues the one episode without any file near it, once.
	if n := f.sync(podcastSyncPayload{PodcastID: f.podcast}); n != 1 {
		t.Errorf("downloads queued = %d, want 1", n)
	}
	if got := f.queuedDownloads(); !slices.Equal(got, []string{free}) {
		t.Errorf("queued downloads = %v, want only %s", got, free)
	}
}

// podcastPairs asks both sides: a file that picks a feed episode which picks another file stays.
func TestPodcastPairsNeedBothSides(t *testing.T) {
	t.Parallel()
	at := func(minute int64) sql.NullInt64 {
		return sql.NullInt64{Int64: (29_333_334 + minute) * 60_000, Valid: true}
	}
	ms := func(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v > 0} }
	feeds := []systemq.PodcastListFeedEpisodesWithoutFileRow{
		{ID: "feed-a", SortTitle: "a", PublishedAt: at(0), DurationMs: ms(600_000)},
		{ID: "feed-b", SortTitle: "b", PublishedAt: at(10), DurationMs: ms(600_000)},
		{ID: "feed-c", SortTitle: "c", PublishedAt: at(20)},
	}
	files := []systemq.PodcastListFileEpisodesWithoutGUIDRow{
		{ID: "file-a", SortTitle: "a", PublishedAt: at(0), DurationMs: ms(540_000)},   // strong for feed-a
		{ID: "file-a2", SortTitle: "a", PublishedAt: at(99), DurationMs: ms(601_000)}, // weak for feed-a, which picks file-a
		{ID: "file-b", SortTitle: "x", PublishedAt: at(10), DurationMs: ms(600_500)},  // weak and alone for feed-b
		{ID: "file-c1", SortTitle: "c", PublishedAt: at(20)},                          // two strong files for feed-c
		{ID: "file-c2", SortTitle: "c", PublishedAt: at(20)},
		{ID: "file-d", SortTitle: "d"}, // nothing near
	}
	want := []podcastPair{{feedEpisodeID: "feed-a", fileEpisodeID: "file-a"}, {feedEpisodeID: "feed-b", fileEpisodeID: "file-b"}}
	if got := podcastPairs(feeds, files); !slices.Equal(got, want) {
		t.Errorf("pairs = %v, want %v", got, want)
	}
	if got := podcastPairs(nil, files); len(got) != 0 {
		t.Errorf("pairs without feed episodes = %v", got)
	}
}
