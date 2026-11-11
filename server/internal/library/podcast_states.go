package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The state of stored episodes the list of the administration shows per podcast (concept 19 part
// A, section 11; metadata.WithPodcastStates): whether «Speichern» may apply (AdminPodcast.savable)
// and how many episodes without file the riegel leave to the administrator
// (AdminPodcast.unclearEpisodeCount). The list holds about a hundred podcasts, so the whole list
// costs a fixed number of queries and one folder check per root, not per podcast.

// PodcastStates returns the state of the podcast podcastID, or of every live podcast with a feed
// when podcastID is empty. A podcast without feed is missing from the map (not savable, nothing
// unclear).
func (s *Service) PodcastStates(ctx context.Context, podcastID string) (map[string]model.PodcastState, error) {
	if podcastID != "" {
		return s.podcastState(ctx, podcastID)
	}
	read := systemq.New(s.db.Read)
	sets, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{})
	if err != nil {
		return nil, fmt.Errorf("podcast settings: %w", err)
	}
	savable, err := s.podcastsSavable(ctx, sets)
	if err != nil {
		return nil, err
	}
	unclear, err := s.unclearPodcastEpisodes(ctx, sets)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.PodcastState, len(sets))
	for _, set := range sets {
		out[set.PodcastID] = model.PodcastState{Savable: savable[set.PodcastID], UnclearEpisodes: unclear[set.PodcastID]}
	}
	return out, nil
}

// podcastState is PodcastStates of one podcast, with the queries of the planner.
func (s *Service) podcastState(ctx context.Context, podcastID string) (map[string]model.PodcastState, error) {
	set, ok, err := s.podcastSetting(ctx, podcastID)
	if err != nil || !ok {
		return map[string]model.PodcastState{}, err
	}
	state := model.PodcastState{}
	switch err := s.PodcastSavable(ctx, podcastID); {
	case err == nil:
		state.Savable = true
	case errors.Is(err, model.ErrRootNotWritable), errors.Is(err, model.ErrRootRequired), errors.Is(err, model.ErrNotFound):
	default:
		return nil, err
	}
	eps, err := systemq.New(s.db.Read).PodcastFeedEpisodes(ctx, sql.NullString{String: podcastID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("episodes of podcast %s: %w", podcastID, err)
	}
	var without []systemq.PodcastFeedEpisodesRow
	for _, e := range eps {
		if e.Files == 0 {
			without = append(without, e)
		}
	}
	if len(without) > 0 {
		verdicts, err := s.judgePodcastEpisodes(ctx, set, without)
		if err != nil {
			return nil, err
		}
		state.UnclearEpisodes = countUnclear(verdicts)
	}
	return map[string]model.PodcastState{podcastID: state}, nil
}

// podcastsSavable reports per podcast of sets whether the folder its downloads would write into
// (podcastDownloadTarget: the root and folder of its newest file, its pending folder, else the
// folder for new subscriptions of its library) passes the folder check, and whether the podcast's
// own folder is there with rights that let the server create files (podcastFolderProblem, by
// Lstat only: the list writes nothing per podcast). Every root is checked once.
func (s *Service) podcastsSavable(ctx context.Context, sets []systemq.PodcastEffectiveSettingsRow) (map[string]bool, error) {
	newest, err := systemq.New(s.db.Read).PodcastNewestRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("roots of podcasts: %w", err)
	}
	rootOf := make(map[string]int64, len(newest))
	folderOf := make(map[string]string, len(newest))
	for _, r := range newest {
		rootOf[r.PodcastID], folderOf[r.PodcastID] = r.RootID, r.Folder
	}
	app := appq.New(s.db.Read)
	opened := map[int64]*os.Root{}
	defer func() {
		for _, r := range opened {
			_ = r.Close()
		}
	}()
	folderOK := func(root appq.LibraryRoot, folder string) bool {
		r, ok := opened[root.ID]
		if !ok {
			if pr, err := paths.NewRoot(root.Path); err == nil {
				if r, err = paths.Open(pr); err != nil {
					r = nil
				}
			}
			opened[root.ID] = r
		}
		return r != nil && podcastFolderProblem(r, folder) == ""
	}
	writable := map[int64]bool{}
	targetOf := map[int64]int64{} // library → its folder for new subscriptions, 0 without one
	check := func(root appq.LibraryRoot) bool {
		w, ok := writable[root.ID]
		if !ok {
			w = s.CheckPodcastFolder(ctx, root).Writable
			writable[root.ID] = w
		}
		return w
	}
	out := make(map[string]bool, len(sets))
	for _, set := range sets {
		rootID, folder := rootOf[set.PodcastID], folderOf[set.PodcastID]
		if rootID == 0 && set.PendingRootID.Valid && set.PendingFolder.Valid {
			rootID, folder = set.PendingRootID.Int64, set.PendingFolder.String
		}
		if rootID == 0 {
			target, ok := targetOf[set.LibraryID]
			if !ok {
				lib, err := app.GetLibrary(ctx, set.LibraryID)
				if err != nil {
					return nil, fmt.Errorf("get library %d: %w", set.LibraryID, err)
				}
				switch root, err := s.podcastTargetRoot(ctx, lib); {
				case err == nil:
					target = root.ID
				case !errors.Is(err, model.ErrRootRequired):
					return nil, err
				}
				targetOf[set.LibraryID] = target
			}
			rootID = target
		}
		if rootID == 0 {
			continue
		}
		root, err := app.GetLibraryRoot(ctx, rootID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get root %d: %w", rootID, err)
		}
		out[set.PodcastID] = check(root) && (folder == "" || folderOK(root, folder))
	}
	return out, nil
}

// unclearPodcastEpisodes counts per podcast of sets the feed episodes without any media file row
// that the riegel judge unclear, from five queries over every podcast.
func (s *Service) unclearPodcastEpisodes(ctx context.Context, sets []systemq.PodcastEffectiveSettingsRow) (map[string]int, error) {
	read := systemq.New(s.db.Read)
	eps, err := read.PodcastAllEpisodesWithoutFile(ctx)
	if err != nil {
		return nil, fmt.Errorf("podcast episodes without file: %w", err)
	}
	if len(eps) == 0 {
		return map[string]int{}, nil
	}
	folders, err := read.PodcastAllFolders(ctx)
	if err != nil {
		return nil, fmt.Errorf("podcast folders: %w", err)
	}
	files, err := read.PodcastAllFolderFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("files of podcast folders: %w", err)
	}
	fileEps, err := read.PodcastAllFileEpisodesWithoutGUID(ctx)
	if err != nil {
		return nil, fmt.Errorf("podcast file episodes: %w", err)
	}
	type folderKey struct {
		root   int64
		folder string
	}
	filesOf := map[folderKey][]podcastFolderFile{}
	for _, f := range files {
		ff := podcastFolderFile{PodcastFilesInFolderRow: systemq.PodcastFilesInFolderRow{
			ID: f.ID, ItemID: f.ItemID, RelPath: f.RelPath, MissingSince: f.MissingSince, Kind: f.Kind,
			ParentID: f.ParentID, Guid: f.Guid, DeletedAt: f.DeletedAt,
		}, key: podcastFileKey(f.RelPath)}
		if ff.key != "" {
			k := folderKey{f.RootID, f.Folder}
			filesOf[k] = append(filesOf[k], ff)
		}
	}
	epsOf := map[string][]systemq.PodcastFeedEpisodesRow{}
	for _, e := range eps {
		epsOf[e.PodcastID] = append(epsOf[e.PodcastID], systemq.PodcastFeedEpisodesRow{
			ID: e.ID, Title: e.Title, SortTitle: e.SortTitle, PublishedAt: e.PublishedAt, DurationMs: e.DurationMs, EnclosureUrl: e.EnclosureUrl,
		})
	}
	unpairedOf := map[string][]systemq.PodcastListFileEpisodesWithoutGUIDRow{}
	for _, u := range fileEps {
		unpairedOf[u.PodcastID] = append(unpairedOf[u.PodcastID], systemq.PodcastListFileEpisodesWithoutGUIDRow{
			ID: u.ID, Title: u.Title, SortTitle: u.SortTitle, PublishedAt: u.PublishedAt, DurationMs: u.DurationMs,
		})
	}
	out := map[string]int{}
	for _, set := range sets {
		mine := epsOf[set.PodcastID]
		if len(mine) == 0 {
			continue
		}
		// The folders of the podcast and of earlier subscriptions of its feed (PodcastFoldersOf).
		seen := map[folderKey]bool{}
		byKey := map[string][]podcastFolderFile{}
		for _, f := range folders {
			if f.PodcastID != set.PodcastID && (f.LibraryID != set.LibraryID || !f.FeedUrl.Valid || f.FeedUrl != set.FeedUrl) {
				continue
			}
			k := folderKey{f.RootID, f.Folder}
			if seen[k] {
				continue
			}
			seen[k] = true
			for _, ff := range filesOf[k] {
				byKey[ff.key] = append(byKey[ff.key], ff)
			}
		}
		if n := countUnclear(judgeEpisodes(set.PodcastID, mine, byKey, unpairedOf[set.PodcastID])); n > 0 {
			out[set.PodcastID] = n
		}
	}
	return out, nil
}

// countUnclear counts the unclear verdicts.
func countUnclear(verdicts map[string]episodeVerdict) int {
	n := 0
	for _, v := range verdicts {
		if v.verdict == verdictUnclear {
			n++
		}
	}
	return n
}
