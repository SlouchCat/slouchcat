package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The page «Podcast» and the section «Downloads» of the administration (concept 19 part A, 12.2
// and 12.3; contract 1.29.0): the episodes of one podcast with the state of their file, the
// queue of downloads, and «Abbrechen» on one download. Every state is derived, nothing is
// stored for it (section 10): the file from media_files, waiting and loading from the job under
// the key podcast-dl:<episodeId>, a failure from podcast_download_failures, «entfernt» from file
// rows that are all missing, «unklar» from the riegel.

// podcastEpisodeLimitMax bounds a page of episodes and of the queue (the contract's maximum).
const podcastEpisodeLimitMax = 200

// PodcastEpisodeList returns one page of the live episodes of a podcast in the view filter
// (model.PodcastEpisodesAll, ...WithFile, ...WithoutFile), newest first, with the counts of the three views.
// Errors: model.ErrNotFound for an unknown or deleted podcast, *model.ValidationError for an
// unknown view.
func (s *Service) PodcastEpisodeList(ctx context.Context, podcastID, filter string, offset, limit int) (model.AdminPodcastEpisodePage, error) {
	switch filter {
	case "":
		filter = model.PodcastEpisodesAll
	case model.PodcastEpisodesAll, model.PodcastEpisodesWithFile, model.PodcastEpisodesWithoutFile:
	default:
		var verr *model.ValidationError
		return model.AdminPodcastEpisodePage{}, verr.Add("filter", "invalid")
	}
	offset, limit = max(offset, 0), min(max(limit, 1), podcastEpisodeLimitMax)
	if err := s.livePodcast(ctx, podcastID); err != nil {
		return model.AdminPodcastEpisodePage{}, err
	}
	eps, err := s.podcastEpisodeStates(ctx, podcastID)
	if err != nil {
		return model.AdminPodcastEpisodePage{}, err
	}
	page := model.AdminPodcastEpisodePage{Offset: offset, Limit: limit, All: len(eps)}
	var view []model.AdminPodcastEpisode
	for _, e := range eps {
		withFile := e.State == model.PodcastEpisodeFile
		if withFile {
			page.WithFile++
		} else {
			page.Open++
		}
		if filter == model.PodcastEpisodesAll || withFile == (filter == model.PodcastEpisodesWithFile) {
			view = append(view, e)
		}
	}
	page.Total = len(view)
	page.Episodes = []model.AdminPodcastEpisode{}
	if offset < len(view) {
		page.Episodes = view[offset:min(offset+limit, len(view))]
	}
	return page, nil
}

// livePodcast answers model.ErrNotFound unless id is a live podcast.
func (s *Service) livePodcast(ctx context.Context, id string) error {
	p, err := systemq.New(s.db.Read).GetItemSystem(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (p.Kind != "podcast" || p.DeletedAt.Valid)) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get podcast %s: %w", id, err)
	}
	return nil
}

// podcastEpisodeStates derives the state of every live episode of a podcast, in the order of the
// page. A file that is there wins over everything; then a download that runs or waits, a failure,
// file rows that are all missing, the riegel's «unklar», else «Nur Link».
func (s *Service) podcastEpisodeStates(ctx context.Context, podcastID string) ([]model.AdminPodcastEpisode, error) {
	read := systemq.New(s.db.Read)
	pid := sql.NullString{String: podcastID, Valid: true}
	rows, err := read.PodcastEpisodeList(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("episodes of podcast %s: %w", podcastID, err)
	}
	type fileOf struct {
		name    string
		size    int64
		present bool
	}
	files := map[string]fileOf{}
	fileRows, err := read.PodcastEpisodeFileRows(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("files of podcast %s: %w", podcastID, err)
	}
	for _, f := range fileRows {
		if _, seen := files[f.ItemID]; !seen { // the first row is the newest one that is there
			files[f.ItemID] = fileOf{name: path.Base(f.RelPath), size: f.Size, present: !f.MissingSince.Valid}
		}
	}
	active, err := read.PodcastActiveDownloadProgress(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("downloads of podcast %s: %w", podcastID, err)
	}
	activeOf := make(map[string]systemq.PodcastActiveDownloadProgressRow, len(active))
	for _, a := range active {
		if cur, ok := activeOf[a.EpisodeID]; !ok || cur.State != string(jobs.StateRunning) {
			activeOf[a.EpisodeID] = a
		}
	}
	failures, err := read.PodcastDownloadFailuresOf(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("failures of podcast %s: %w", podcastID, err)
	}
	failureOf := make(map[string]systemq.PodcastDownloadFailure, len(failures))
	for _, f := range failures {
		failureOf[f.ItemID] = f
	}
	unclear, err := s.unclearEpisodesOf(ctx, podcastID)
	if err != nil {
		return nil, err
	}

	now := s.now()
	out := make([]model.AdminPodcastEpisode, 0, len(rows))
	for _, r := range rows {
		e := model.AdminPodcastEpisode{ID: r.ID, Title: r.Title, Feed: r.Feed == 1}
		if r.PublishedAt.Valid {
			t := time.UnixMilli(r.PublishedAt.Int64).UTC()
			e.PublishedAt = &t
		}
		if r.DurationMs.Valid && r.DurationMs.Int64 > 0 {
			d := r.DurationMs.Int64
			e.DurationMs = &d
		}
		if r.EnclosureLength.Valid && r.EnclosureLength.Int64 > 0 {
			n := r.EnclosureLength.Int64
			e.SizeBytes = &n
		}
		f, hasRows := files[r.ID]
		a, isActive := activeOf[r.ID]
		fail, failed := failureOf[r.ID]
		switch {
		case hasRows && f.present:
			e.State, e.FileName = model.PodcastEpisodeFile, f.name
			size := f.size
			e.SizeBytes = &size
		case isActive && a.State == string(jobs.StateRunning):
			e.State = model.PodcastEpisodeRunning
			if a.ProgressDone.Valid {
				done := a.ProgressDone.Int64
				e.Progress = &done
			}
			if a.ProgressTotal.Valid && a.ProgressTotal.Int64 > 0 {
				total := a.ProgressTotal.Int64
				e.Total = &total
			}
		case isActive:
			e.State = model.PodcastEpisodeQueued
		case failed:
			e.State, e.ErrorCode = model.PodcastEpisodeFailed, fail.Error
			e.RetryAt = podcastRetryAt(fail, now)
		case hasRows:
			e.State, e.FileName = model.PodcastEpisodeRemoved, f.name
		case unclear[r.ID]:
			e.State = model.PodcastEpisodeUnclear
		case e.Feed:
			e.State = model.PodcastEpisodeLink
		default:
			// A file episode without any file row: nothing to play and nothing to load.
			e.State = model.PodcastEpisodeRemoved
		}
		out = append(out, e)
	}
	return out, nil
}

// unclearEpisodesOf is the set of feed episodes without any media file row that the riegel judge
// unclear; empty for a podcast without feed.
func (s *Service) unclearEpisodesOf(ctx context.Context, podcastID string) (map[string]bool, error) {
	set, ok, err := s.podcastSetting(ctx, podcastID)
	if err != nil || !ok {
		return nil, err
	}
	eps, err := systemq.New(s.db.Read).PodcastFeedEpisodes(ctx, sql.NullString{String: podcastID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("feed episodes of podcast %s: %w", podcastID, err)
	}
	var without []systemq.PodcastFeedEpisodesRow
	for _, e := range eps {
		if e.Files == 0 {
			without = append(without, e)
		}
	}
	if len(without) == 0 {
		return nil, nil
	}
	verdicts, err := s.judgePodcastEpisodes(ctx, set, without)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for id, v := range verdicts {
		if v.verdict == verdictUnclear {
			out[id] = true
		}
	}
	return out, nil
}

// podcastRetryAt is when podcast.sync_all tries a failure again at the earliest (the rule of
// PodcastsWithRetryableFailures and plannedPodcastEpisodes): six hours after the last attempt,
// for seven days after the first; nil for a final code and once the seven days are over.
func podcastRetryAt(f systemq.PodcastDownloadFailure, now time.Time) *time.Time {
	if podcastFinalFailure(f.Error) {
		return nil
	}
	at := time.UnixMilli(f.LastFailedAt).Add(podcastRetryAfter)
	giveUp := time.UnixMilli(f.FirstFailedAt).Add(podcastGiveUpAfter)
	if at.After(giveUp) || now.After(giveUp) {
		return nil
	}
	at = at.UTC()
	return &at
}

// PodcastDownloadQueue returns the downloads that run or wait, of every podcast library (at most
// limit, 1..200), the running ones first, then in claim order; with how many there are, how many
// run and whether the job type podcast.download is paused.
func (s *Service) PodcastDownloadQueue(ctx context.Context, limit int) (model.PodcastDownloadPage, error) {
	limit = min(max(limit, 1), podcastEpisodeLimitMax)
	read := systemq.New(s.db.Read)
	rows, err := read.PodcastDownloadQueue(ctx, int64(limit))
	if err != nil {
		return model.PodcastDownloadPage{}, fmt.Errorf("podcast download queue: %w", err)
	}
	counts, err := read.PodcastDownloadQueueCounts(ctx)
	if err != nil {
		return model.PodcastDownloadPage{}, fmt.Errorf("count podcast downloads: %w", err)
	}
	page := model.PodcastDownloadPage{Total: int(counts.Total), Running: int(counts.Running), Downloads: make([]model.PodcastDownload, 0, len(rows))}
	switch typ, err := read.GetJobType(ctx, JobPodcastDownload); {
	case err == nil:
		page.Paused = typ.PausedAt.Valid
	case !errors.Is(err, sql.ErrNoRows):
		return model.PodcastDownloadPage{}, fmt.Errorf("job type %s: %w", JobPodcastDownload, err)
	}
	for _, r := range rows {
		d := model.PodcastDownload{EpisodeID: r.EpisodeID, EpisodeTitle: r.EpisodeTitle, PodcastID: r.PodcastID,
			PodcastTitle: r.PodcastTitle, LibraryID: r.LibraryID, State: model.PodcastEpisodeQueued}
		if r.EnclosureLength.Valid && r.EnclosureLength.Int64 > 0 {
			n := r.EnclosureLength.Int64
			d.SizeBytes = &n
		}
		if r.State == string(jobs.StateRunning) {
			d.State = model.PodcastEpisodeRunning
			if r.ProgressDone.Valid {
				done := r.ProgressDone.Int64
				d.Progress = &done
			}
			if r.ProgressTotal.Valid && r.ProgressTotal.Int64 > 0 {
				total := r.ProgressTotal.Int64
				d.Total = &total
			}
		}
		page.Downloads = append(page.Downloads, d)
	}
	return page, nil
}

// CancelPodcastDownload is «Abbrechen» on the download of one episode (cancelPodcastDownload): a
// waiting download leaves the queue, a running one ends at once through the cancel hook, which
// removes its partial file. Without a download nothing happens. model.ErrNotFound unless
// episodeID is a live episode of the live podcast podcastID.
func (s *Service) CancelPodcastDownload(ctx context.Context, podcastID, episodeID string) error {
	if err := s.livePodcast(ctx, podcastID); err != nil {
		return err
	}
	ep, err := systemq.New(s.db.Read).GetItemSystem(ctx, episodeID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (ep.Kind != "podcast_episode" || ep.DeletedAt.Valid || ep.ParentID.String != podcastID)) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get episode %s: %w", episodeID, err)
	}
	ids, err := systemq.New(s.db.Read).PodcastActiveDownloadOf(ctx, sql.NullString{String: podcastDownloadKey(episodeID), Valid: true})
	if err != nil {
		return fmt.Errorf("download of episode %s: %w", episodeID, err)
	}
	for _, id := range ids {
		if s.registry != nil {
			if _, err := s.registry.CancelJob(ctx, id); err != nil {
				return err
			}
		} else if _, err := s.queue.Cancel(ctx, id); err != nil {
			return err
		}
		s.log.InfoContext(ctx, "podcast download cancelled", "item_id", episodeID, "job_id", id)
	}
	return nil
}
