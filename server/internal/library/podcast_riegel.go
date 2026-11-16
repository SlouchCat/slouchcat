package library

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The three riegel against duplicate files (concept 19 part A, section 8; ADR 0029, decision 6).
// Before the planner loads a feed episode without a file, it looks at the folders of its podcast:
//
//  1. Key: the keys of the episode (title and the stem of the name Audiobookshelf gives it) against
//     the stem of every media file in the podcast's folders, whatever item it hangs on (also on a
//     deleted episode of an earlier subscription). Exactly one hit at a live file episode without
//     guid of the podcast: the planner pairs them instead of loading. A hit anywhere else, or two:
//     unclear.
//  2. The pairing rule (model.FitPodcastEpisodes: the same minute and title, else one of the two
//     with durations that do not contradict) against the unpaired file episodes: a candidate there
//     makes the episode unclear. The planner pairs by key only; what the rule itself pairs, the
//     pairing after a feed check does (PairPodcastEpisodes).
//  3. An unpaired file episode published within 24 hours of the episode: unclear.
//
// An unclear episode is not loaded by the planner; «Laden» on the episode loads it all the same.

// riegelWindow is the window of riegel 3.
const riegelWindow = 24 * time.Hour

// verdict is what the riegel decided for a feed episode.
type verdict int

const (
	verdictLoad    verdict = iota // no file there or near: load it
	verdictPair                   // exactly one file episode by key: pair it with that one
	verdictUnclear                // a file may be there: leave it to the administrator
)

// episodeVerdict is the decision on one feed episode; pairWith is the file episode of verdictPair.
type episodeVerdict struct {
	verdict  verdict
	pairWith string
}

// podcastFolderFile is a media file in a folder of a podcast with the item it hangs on.
type podcastFolderFile struct {
	systemq.PodcastFilesInFolderRow
	key string
}

// judgePodcastEpisodes applies the three riegel to the feed episodes eps of a podcast (all without
// a media file) and returns the verdict per episode id. A file episode two episodes would pair
// with is paired with neither: both are unclear.
func (s *Service) judgePodcastEpisodes(ctx context.Context, p systemq.PodcastEffectiveSettingsRow, eps []systemq.PodcastFeedEpisodesRow) (map[string]episodeVerdict, error) {
	read := systemq.New(s.db.Read)
	folders, err := read.PodcastFoldersOf(ctx, systemq.PodcastFoldersOfParams{PodcastID: p.PodcastID, LibraryID: p.LibraryID, FeedUrl: p.FeedUrl})
	if err != nil {
		return nil, fmt.Errorf("folders of podcast %s: %w", p.PodcastID, err)
	}
	byKey := map[string][]podcastFolderFile{}
	for _, f := range folders {
		rows, err := read.PodcastFilesInFolder(ctx, systemq.PodcastFilesInFolderParams{RootID: f.RootID, Prefix: f.Folder + "/", Upper: f.Folder + "0"})
		if err != nil {
			return nil, fmt.Errorf("files of a folder of podcast %s: %w", p.PodcastID, err)
		}
		for _, r := range rows {
			ff := podcastFolderFile{PodcastFilesInFolderRow: r, key: podcastFileKey(r.RelPath)}
			if ff.key != "" {
				byKey[ff.key] = append(byKey[ff.key], ff)
			}
		}
	}
	unpaired, err := read.PodcastListFileEpisodesWithoutGUID(ctx, sql.NullString{String: p.PodcastID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("file episodes of podcast %s: %w", p.PodcastID, err)
	}
	return judgeEpisodes(p.PodcastID, eps, byKey, unpaired), nil
}

// judgeEpisodes is the decision of the three riegel on the feed episodes eps of a podcast, given
// the files of its folders by key and its file episodes without guid.
func judgeEpisodes(podcastID string, eps []systemq.PodcastFeedEpisodesRow, byKey map[string][]podcastFolderFile,
	unpaired []systemq.PodcastListFileEpisodesWithoutGUIDRow,
) map[string]episodeVerdict {
	out := make(map[string]episodeVerdict, len(eps))
	claims := map[string][]string{} // file episode → feed episodes that would pair with it
	for _, e := range eps {
		hits := map[string]podcastFolderFile{}
		for _, k := range podcastEpisodeKeys(e.Title, e.EnclosureUrl.String) {
			for _, f := range byKey[k] {
				hits[f.ID] = f
			}
		}
		switch {
		case len(hits) == 1:
			var f podcastFolderFile
			for _, h := range hits {
				f = h
			}
			if f.Kind == "podcast_episode" && f.ParentID.String == podcastID && !f.Guid.Valid && !f.DeletedAt.Valid {
				out[e.ID] = episodeVerdict{verdict: verdictPair, pairWith: f.ItemID}
				claims[f.ItemID] = append(claims[f.ItemID], e.ID)
				continue
			}
			out[e.ID] = episodeVerdict{verdict: verdictUnclear}
			continue
		case len(hits) > 1:
			out[e.ID] = episodeVerdict{verdict: verdictUnclear}
			continue
		}
		out[e.ID] = episodeVerdict{verdict: verdictLoad}
		for _, u := range unpaired {
			near := e.PublishedAt.Valid && u.PublishedAt.Valid && absDuration(e.PublishedAt.Int64-u.PublishedAt.Int64) <= riegelWindow
			fit := model.FitPodcastEpisodes(podcastTraits(e.PublishedAt, e.SortTitle, e.DurationMs), podcastTraits(u.PublishedAt, u.SortTitle, u.DurationMs))
			if near || fit.Grade != model.PodcastPairNone {
				out[e.ID] = episodeVerdict{verdict: verdictUnclear}
				break
			}
		}
	}
	for _, feedIDs := range claims {
		if len(feedIDs) > 1 {
			for _, id := range feedIDs {
				out[id] = episodeVerdict{verdict: verdictUnclear}
			}
		}
	}
	return out
}

// absDuration is the length of a span of Unix milliseconds.
func absDuration(ms int64) time.Duration {
	if ms < 0 {
		ms = -ms
	}
	return time.Duration(ms) * time.Millisecond
}

// pairPodcastFile pairs a file episode with a feed episode in a transaction of its own
// (pairPodcastEpisodes). paired is false when either changed meanwhile.
func (s *Service) pairPodcastFile(ctx context.Context, feedEpisodeID, fileEpisodeID string) (paired bool, err error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	paired, err = pairPodcastEpisodes(ctx, systemq.New(tx), s.now().UnixMilli(), feedEpisodeID, fileEpisodeID)
	if err != nil || !paired {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// pairPodcastEpisodes makes one episode of a feed episode without a file and a file episode of the
// same podcast, inside the caller's transaction: the feed episode takes the media files and keeps
// its guid, enclosure and texts; what the profiles did with the file episode moves over by the
// rules of the duplicate merge (relinker.moveRows: per profile the further progress and the later
// «Gehört» win, events, days, watchlist, recommendations, notifications and device downloads
// follow); then the file episode is deleted for good, like the source of a merge: the merge
// queries leave the rows the feed episode already had on it, and they go with it. Its texts go
// too -- the feed's description is the better one. paired is false, and nothing is written, when
// the pair no longer holds (PodcastPairable: the feed episode has a file, the file episode is gone
// or took a guid).
func pairPodcastEpisodes(ctx context.Context, q *systemq.Queries, now int64, feedEpisodeID, fileEpisodeID string) (paired bool, err error) {
	ok, err := q.PodcastPairable(ctx, systemq.PodcastPairableParams{FeedEpisodeID: feedEpisodeID, FileEpisodeID: fileEpisodeID})
	if err != nil {
		return false, fmt.Errorf("check pair: %w", err)
	}
	if ok == 0 {
		return false, nil
	}
	if err := (&relinker{q: q, now: now}).moveRows(ctx, feedEpisodeID, fileEpisodeID); err != nil {
		return false, fmt.Errorf("pair episode %s with %s: %w", fileEpisodeID, feedEpisodeID, err)
	}
	if err := q.AdminDeleteItem(ctx, fileEpisodeID); err != nil {
		return false, fmt.Errorf("delete file episode %s: %w", fileEpisodeID, err)
	}
	return true, nil
}

// PairPodcastEpisodes pairs the file episodes of a podcast with its feed episodes without a file
// wherever the pairing rule is clear from both sides (podcastPairs), one transaction per pair, and
// returns how many it paired. It reads the database only, never a file: what the probe wrote on the
// file episode is enough. It runs after every feed check of the podcast (metadata.WithPodcastChecked,
// wired in main.go): the probe pairs a file only against the feed episodes of its moment and a
// fetch only its new episodes, so pairs neither met stay behind (the feed corrected a title or a
// date, a rule changed), and this step takes them up. Nothing to pair costs two reads.
func (s *Service) PairPodcastEpisodes(ctx context.Context, podcastID string) (int, error) {
	read := systemq.New(s.db.Read)
	parent := sql.NullString{String: podcastID, Valid: true}
	files, err := read.PodcastListFileEpisodesWithoutGUID(ctx, parent)
	if err != nil || len(files) == 0 {
		return 0, err
	}
	feeds, err := read.PodcastListFeedEpisodesWithoutFile(ctx, parent)
	if err != nil || len(feeds) == 0 {
		return 0, err
	}
	paired := 0
	for _, p := range podcastPairs(feeds, files) {
		if err := ctx.Err(); err != nil {
			return paired, err
		}
		ok, err := s.pairPodcastFile(ctx, p.feedEpisodeID, p.fileEpisodeID)
		if err != nil {
			return paired, err
		}
		if ok {
			paired++
		}
	}
	if paired > 0 {
		s.log.InfoContext(ctx, "podcast files paired with their feed episodes", "item_id", podcastID, "episodes", paired)
	}
	return paired, nil
}

// podcastPair is a feed episode and the file episode that is the same episode.
type podcastPair struct {
	feedEpisodeID, fileEpisodeID string
}

// podcastPairs returns the pairs among the feed episodes without a file and the file episodes of
// one podcast: a file and a feed episode that pick each other (model.PickPodcastPair, once among
// the feed episodes for the file and once among the files for the feed episode). A file two feed
// episodes would take, or a feed episode two files stand for, stays apart unless the rule tells
// them apart. The pairs come in the order of the files.
func podcastPairs(feeds []systemq.PodcastListFeedEpisodesWithoutFileRow, files []systemq.PodcastListFileEpisodesWithoutGUIDRow) []podcastPair {
	feedTraits := make([]model.PodcastEpisodeTraits, len(feeds))
	feedsByMinute, feedsByTitle := map[int64][]int{}, map[string][]int{}
	for i, e := range feeds {
		feedTraits[i] = podcastTraits(e.PublishedAt, e.SortTitle, e.DurationMs)
		if e.PublishedAt.Valid {
			feedsByMinute[e.PublishedAt.Int64/60000] = append(feedsByMinute[e.PublishedAt.Int64/60000], i)
		}
		if e.SortTitle != "" {
			feedsByTitle[e.SortTitle] = append(feedsByTitle[e.SortTitle], i)
		}
	}
	fileTraits := make([]model.PodcastEpisodeTraits, len(files))
	filesByMinute, filesByTitle := map[int64][]int{}, map[string][]int{}
	for i, f := range files {
		fileTraits[i] = podcastTraits(f.PublishedAt, f.SortTitle, f.DurationMs)
		if f.PublishedAt.Valid {
			filesByMinute[f.PublishedAt.Int64/60000] = append(filesByMinute[f.PublishedAt.Int64/60000], i)
		}
		if f.SortTitle != "" {
			filesByTitle[f.SortTitle] = append(filesByTitle[f.SortTitle], i)
		}
	}
	// near lists the episodes of the other side that share the minute or the title: only those can
	// fit (model.FitPodcastEpisodes), so nothing else is compared.
	near := func(t model.PodcastEpisodeTraits, byMinute map[int64][]int, byTitle map[string][]int) []int {
		var out []int
		if t.Dated {
			out = append(out, byMinute[t.PublishedAt/60000]...)
		}
		for _, i := range byTitle[t.SortTitle] {
			if !slices.Contains(out, i) {
				out = append(out, i)
			}
		}
		return out
	}
	// fileOf is the file each feed episode picks, -1 for none.
	fileOf := make([]int, len(feeds))
	for i := range feeds {
		fileOf[i] = -1
		candidates := near(feedTraits[i], filesByMinute, filesByTitle)
		fits := make([]model.PodcastPairFit, len(candidates))
		for k, f := range candidates {
			fits[k] = model.FitPodcastEpisodes(feedTraits[i], fileTraits[f])
		}
		if hit, ok := model.PickPodcastPair(fits); ok {
			fileOf[i] = candidates[hit]
		}
	}
	var pairs []podcastPair
	for i := range files {
		candidates := near(fileTraits[i], feedsByMinute, feedsByTitle)
		fits := make([]model.PodcastPairFit, len(candidates))
		for k, e := range candidates {
			fits[k] = model.FitPodcastEpisodes(feedTraits[e], fileTraits[i])
		}
		if hit, ok := model.PickPodcastPair(fits); ok && fileOf[candidates[hit]] == i {
			pairs = append(pairs, podcastPair{feedEpisodeID: feeds[candidates[hit]].ID, fileEpisodeID: files[i].ID})
		}
	}
	return pairs
}
