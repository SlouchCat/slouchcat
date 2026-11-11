package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
//  2. Today's pairing rule (metadataPairs: same minute or same title, duration within two percent)
//     against the unpaired file episodes: a candidate there makes the episode unclear.
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
			if near || metadataPairs(e.PublishedAt, e.SortTitle, e.DurationMs, u.PublishedAt, u.SortTitle, u.DurationMs) {
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

// pairPodcastFile moves the media files of a file episode to a feed episode and retires the file
// episode, the way finishPodcastEpisode pairs them (riegel 1). paired is false when either changed
// meanwhile (the feed episode has a file, the file episode is gone).
func (s *Service) pairPodcastFile(ctx context.Context, feedEpisodeID, fileEpisodeID string) (paired bool, err error) {
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	ok, err := q.PodcastPairable(ctx, systemq.PodcastPairableParams{FeedEpisodeID: feedEpisodeID, FileEpisodeID: fileEpisodeID})
	if err != nil {
		return false, fmt.Errorf("check pair: %w", err)
	}
	if ok == 0 {
		return false, nil
	}
	if err := q.PodcastMoveMediaFiles(ctx, systemq.PodcastMoveMediaFilesParams{TargetID: feedEpisodeID, Now: now, SourceID: fileEpisodeID}); err != nil {
		return false, fmt.Errorf("move file to feed episode: %w", err)
	}
	if err := q.PodcastSoftDeleteEpisode(ctx, systemq.PodcastSoftDeleteEpisodeParams{Now: sql.NullInt64{Int64: now, Valid: true}, ID: fileEpisodeID}); err != nil {
		return false, fmt.Errorf("retire file episode: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	s.log.InfoContext(ctx, "podcast file paired with its feed episode by name", "item_id", feedEpisodeID, "file_episode_id", fileEpisodeID)
	return true, nil
}
