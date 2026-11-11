package library

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The planner of stored podcast episodes, podcast.sync (concept 19 part A, 5.2, 5.3 and 6.1; ADR
// 0029, decision 3): one job per podcast that chooses the episodes to load by the effective
// setting and queues podcast.download for each. It runs after a feed fetch that reported new
// episodes (metadata.WithPodcastFetched; only on «Speichern»), after a new subscription that loads
// the N newest or all episodes, after every change of a setting, and every six hours through
// podcast.sync_all for the podcasts on «Speichern» with the scope last_n or all or with a failed
// download to try again.
//
// It leaves out episodes that have a media file row (also a missing one: «entfernt», loaded only
// on «Laden»), whose download waits or runs, that failed within six hours, for the first time more
// than seven days ago or for good (too_large, not_audio, unsupported_format), whose format the
// scanner does not read (O-3), and those the riegel find a file for (paired instead) or call
// unclear (section 8). An episode whose failure may be tried again is loaded whatever the scope. It cancels the waiting downloads that
// are no longer wanted (a smaller scope, a smaller N); after a change of the setting to «Nur
// Link» it cancels the running ones and those of the buttons as well. Stored files stay (F-4).

// Job types of the planner.
const (
	JobPodcastSync    = "podcast.sync"     // payload podcastSyncPayload, key podcast-sync:<id> for the schedule's runs
	JobPodcastSyncAll = "podcast.sync_all" // schedule, every 6 hours
)

const (
	prioPodcastSync = 4
	// prioPodcastDownloadButton and prioPodcastDownloadEpisode are the priorities of the downloads
	// of «Fehlende nachladen» and of «Laden» on one episode, above the planner's.
	prioPodcastDownloadButton  = 3
	prioPodcastDownloadEpisode = jobs.ManualPriority
	podcastSyncAllEvery        = 6 * time.Hour
	// podcastRetryAfter and podcastGiveUpAfter: a failed episode is tried again by the planner six
	// hours after its last failure, for seven days after its first one (6.2, «Fehler»).
	podcastRetryAfter  = 6 * time.Hour
	podcastGiveUpAfter = 7 * 24 * time.Hour
)

// podcastFinalFailure reports whether a failure code is final: the enclosure itself is not to be
// stored (too_large, not_audio, unsupported_format), so neither the planner nor podcast.sync_all
// tries it again; only a button does («Erneut», «Laden», «Fehlende nachladen»). The query
// PodcastsWithRetryableFailures leaves out the same codes.
func podcastFinalFailure(code string) bool {
	switch code {
	case PodcastErrTooLarge, PodcastErrNotAudio, PodcastErrUnsupportedFormat:
		return true
	}
	return false
}

// planUse names who asks plannedPodcastEpisodes for the episodes to load.
type planUse int

const (
	// planSync is the planner: an episode whose failure may be tried again counts as in the scope
	// whatever the scope says (podcast.sync_all, 6.2), and a pair of the riegel is paired.
	planSync planUse = iota
	// planEstimate is estimatePodcastDownloads: nothing is paired, failures hold back as for the
	// planner, but only the scope counts.
	planEstimate
	// planButton is «Fehlende nachladen»: failures hold nothing back (5.3).
	planButton
)

// podcastSyncPayload is the payload of podcast.sync: the podcast, the episodes a fetch reported as
// new, whether a setting changed (the waiting downloads it no longer wants are cancelled), and
// whether that change ended «Speichern» (Stop: the running downloads and those of the buttons end
// too, 5.3). A change of another value while «Nur Link» holds leaves the buttons' downloads alone.
type podcastSyncPayload struct {
	PodcastID string   `json:"podcastId"`
	NewIDs    []string `json:"newIds,omitempty"`
	Changed   bool     `json:"changed,omitempty"`
	Stop      bool     `json:"stop,omitempty"`
}

// podcastSyncTypes registers the planner and its schedule (RegisterTypes).
func (s *Service) podcastSyncTypes(r *jobs.Registry) {
	r.Register(jobs.Type{Name: JobPodcastSync, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: prioPodcastSync, Handler: s.handlePodcastSync})
	r.Register(jobs.Type{Name: JobPodcastSyncAll, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioPodcastSync, Run: s.runPodcastSyncAll, Every: podcastSyncAllEvery})
}

// QueuePodcastSync is the hook of a feed fetch and a new subscription (metadata.WithPodcastFetched,
// wired in main.go): with new episode ids it queues the planner only for a podcast on «Speichern»
// (no empty runs for the polls of podcasts on «Nur Link», P-3); without ids (a new subscription)
// only when the scope loads more than new episodes (P-14).
func (s *Service) QueuePodcastSync(ctx context.Context, podcastID string, newIDs []string) error {
	set, ok, err := s.podcastSetting(ctx, podcastID)
	if err != nil || !ok || set.EffectiveMode != model.PodcastSave || (len(newIDs) == 0 && set.EffectiveScope == model.PodcastScopeNew) {
		return err
	}
	return s.enqueuePodcastSync(ctx, podcastSyncPayload{PodcastID: podcastID, NewIDs: newIDs}, "")
}

// QueuePodcastSettingChanged queues the planner after the own setting of a podcast changed
// (metadata.WithPodcastSettingHooks, wired in main.go): it loads what the new setting adds and
// cancels what it no longer wants. stop says that the change ended «Speichern» for the podcast:
// then the running downloads and those of the buttons end as well.
func (s *Service) QueuePodcastSettingChanged(ctx context.Context, podcastID string, stop bool) error {
	return s.enqueuePodcastSync(ctx, podcastSyncPayload{PodcastID: podcastID, Changed: true, Stop: stop}, "")
}

// enqueuePodcastSync queues one planner run; key is empty for the runs of an event (each one
// carries its own new episodes), podcast-sync:<id> for the schedule's.
func (s *Service) enqueuePodcastSync(ctx context.Context, p podcastSyncPayload, key string) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobPodcastSync, Priority: prioPodcastSync, IdempotencyKey: key, Payload: raw}); err != nil {
		return fmt.Errorf("queue %s of %s: %w", JobPodcastSync, p.PodcastID, err)
	}
	return nil
}

// runPodcastSyncAll is the Run of podcast.sync_all: the planner for every podcast on «Speichern»
// whose scope is last_n or all, or that has a failed download to try again.
func (s *Service) runPodcastSyncAll(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	read := systemq.New(s.db.Read)
	sets, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{})
	if err != nil {
		return nil, fmt.Errorf("podcast settings: %w", err)
	}
	now := s.now()
	failed, err := read.PodcastsWithRetryableFailures(ctx, systemq.PodcastsWithRetryableFailuresParams{
		RetryBefore: now.Add(-podcastRetryAfter).UnixMilli(), GiveUpBefore: now.Add(-podcastGiveUpAfter).UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("podcasts with failed downloads: %w", err)
	}
	var ids []int64
	for _, set := range sets {
		if set.EffectiveMode != model.PodcastSave || (set.EffectiveScope == model.PodcastScopeNew &&
			!slices.Contains(failed, sql.NullString{String: set.PodcastID, Valid: true})) {
			continue
		}
		raw, err := json.Marshal(podcastSyncPayload{PodcastID: set.PodcastID})
		if err != nil {
			return ids, err
		}
		id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobPodcastSync, Priority: prio, IdempotencyKey: "podcast-sync:" + set.PodcastID, Payload: raw})
		if err != nil {
			return ids, fmt.Errorf("queue %s: %w", JobPodcastSync, err)
		}
		if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// handlePodcastSync is the handler of podcast.sync.
func (s *Service) handlePodcastSync(ctx context.Context, j *jobs.Job) error {
	var p podcastSyncPayload
	if err := decodePayload(j, &p); err != nil {
		return jobs.Final(err)
	}
	queued, err := s.syncPodcast(ctx, p)
	if err == nil && queued == 0 {
		jobs.Skip(ctx)
	}
	return err
}

// podcastSetting reads the effective setting of a live podcast with a feed; ok is false otherwise.
func (s *Service) podcastSetting(ctx context.Context, podcastID string) (systemq.PodcastEffectiveSettingsRow, bool, error) {
	sets, err := systemq.New(s.db.Read).PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{PodcastID: podcastID})
	if err != nil {
		return systemq.PodcastEffectiveSettingsRow{}, false, fmt.Errorf("setting of podcast %s: %w", podcastID, err)
	}
	if len(sets) != 1 {
		return systemq.PodcastEffectiveSettingsRow{}, false, nil
	}
	return sets[0], true, nil
}

// syncPodcast runs the planner for one podcast and returns how many downloads it queued.
func (s *Service) syncPodcast(ctx context.Context, p podcastSyncPayload) (int, error) {
	set, ok, err := s.podcastSetting(ctx, p.PodcastID)
	if err != nil {
		return 0, err
	}
	wanted := map[string]bool{}
	var load []systemq.PodcastFeedEpisodesRow
	if ok && set.EffectiveMode == model.PodcastSave {
		load, err = s.plannedPodcastEpisodes(ctx, set, set.EffectiveScope, int(set.EffectiveLastN), p.NewIDs, planSync)
		if err != nil {
			return 0, err
		}
		for _, e := range load {
			wanted[e.ID] = true
		}
	}
	saving := ok && set.EffectiveMode == model.PodcastSave
	if !saving || p.Changed {
		if err := s.cancelPodcastDownloads(ctx, p.PodcastID, wanted, saving, !saving && p.Stop); err != nil {
			return 0, err
		}
	}
	isNew := map[string]bool{}
	for _, id := range p.NewIDs {
		isNew[id] = true
	}
	for _, e := range load {
		if _, err := s.enqueuePodcastDownload(ctx, podcastDownloadPayload{EpisodeID: e.ID, New: isNew[e.ID]}, prioPodcastDownload); err != nil {
			return 0, err
		}
	}
	if len(load) > 0 {
		s.log.InfoContext(ctx, "podcast episodes queued for storing", "item_id", p.PodcastID, "episodes", len(load))
	}
	return len(load), nil
}

// plannedPodcastEpisodes chooses the episodes the setting (scope with lastN, the new episodes of
// the fetch) asks for and the planner may load (5.2, 6.1): live feed episodes without any media
// file row, without a recent, abandoned or final failure (none of these for planButton), of a
// format the scanner reads, and not unclear by the riegel. For planSync an episode whose failure
// may be tried again (the last one six hours ago, the first one within seven days) is in the scope
// whatever the scope says (6.2). An episode the riegel pair with a file episode is paired here and
// left out; for planEstimate it is only left out.
func (s *Service) plannedPodcastEpisodes(ctx context.Context, set systemq.PodcastEffectiveSettingsRow, scope string, lastN int,
	newIDs []string, use planUse,
) ([]systemq.PodcastFeedEpisodesRow, error) {
	read := systemq.New(s.db.Read)
	eps, err := read.PodcastFeedEpisodes(ctx, sql.NullString{String: set.PodcastID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("episodes of podcast %s: %w", set.PodcastID, err)
	}
	failures, err := read.PodcastDownloadFailuresOf(ctx, sql.NullString{String: set.PodcastID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed downloads of podcast %s: %w", set.PodcastID, err)
	}
	now := s.now()
	skip, retry := map[string]bool{}, map[string]bool{}
	for _, f := range failures {
		switch {
		case use == planButton:
		case podcastFinalFailure(f.Error) || f.LastFailedAt > now.Add(-podcastRetryAfter).UnixMilli() ||
			f.FirstFailedAt < now.Add(-podcastGiveUpAfter).UnixMilli():
			skip[f.ItemID] = true
		default:
			retry[f.ItemID] = true
		}
	}
	isNew := map[string]bool{}
	for _, id := range newIDs {
		isNew[id] = true
	}
	var candidates []systemq.PodcastFeedEpisodesRow
	for i, e := range eps {
		inScope := isNew[e.ID] || (use == planSync && retry[e.ID]) || scope == model.PodcastScopeAll ||
			(scope == model.PodcastScopeLastN && i < lastN)
		if inScope && e.Files == 0 && !skip[e.ID] && podcastFormatKnown(e.EnclosureUrl.String, e.EnclosureType.String) {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	verdicts, err := s.judgePodcastEpisodes(ctx, set, candidates)
	if err != nil {
		return nil, err
	}
	var out []systemq.PodcastFeedEpisodesRow
	for _, e := range candidates {
		switch v := verdicts[e.ID]; v.verdict {
		case verdictLoad:
			out = append(out, e)
		case verdictPair:
			if use != planEstimate {
				if _, err := s.pairPodcastFile(ctx, e.ID, v.pairWith); err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
}

// cancelPodcastDownloads cancels the waiting downloads of a podcast's episodes that are not wanted,
// were not asked for by a button and do not load a new episode of a fetch (a smaller scope keeps
// those; «Nur Link» does not); with all (the change «Speichern» → «Nur Link») also the running ones
// and those of the buttons. A running download ends at once and removes its partial file.
func (s *Service) cancelPodcastDownloads(ctx context.Context, podcastID string, wanted map[string]bool, saving, all bool) error {
	active, err := systemq.New(s.db.Read).PodcastActiveDownloads(ctx, sql.NullString{String: podcastID, Valid: true})
	if err != nil {
		return fmt.Errorf("downloads of podcast %s: %w", podcastID, err)
	}
	n := 0
	for _, a := range active {
		var p podcastDownloadPayload
		_ = json.Unmarshal([]byte(a.Payload), &p)
		if !all && (wanted[a.EpisodeID] || p.Manual || (p.New && saving) || a.State != string(jobs.StateQueued)) {
			continue
		}
		if s.registry != nil {
			if _, err := s.registry.CancelJob(ctx, a.ID); err != nil {
				return err
			}
		} else if _, err := s.queue.Cancel(ctx, a.ID); err != nil {
			return err
		}
		n++
	}
	if n > 0 {
		s.log.InfoContext(ctx, "podcast downloads cancelled", "item_id", podcastID, "downloads", n)
	}
	return nil
}

// enqueuePodcastDownload queues the download of an episode with prio; a waiting one under the key
// is raised to prio and, for a button, marked manual.
func (s *Service) enqueuePodcastDownload(ctx context.Context, p podcastDownloadPayload, prio int) (int64, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobPodcastDownload, Priority: prio, IdempotencyKey: podcastDownloadKey(p.EpisodeID), Payload: raw})
	if err != nil {
		return 0, fmt.Errorf("queue %s: %w", JobPodcastDownload, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return 0, err
	}
	if p.Manual {
		if _, err := s.queue.SetQueuedPayload(ctx, id, raw); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// DownloadPodcastEpisodes is downloadPodcastEpisodes (12.2, 5.3): without episode ids «Fehlende
// nachladen», once every live feed episode of the podcast without a media file row whatever scope
// applies (paired or left out when the riegel find a file); with ids «Laden» on those episodes,
// loaded whatever the setting and the riegel say, a removed one included (its row is revived). The
// downloads are manual: a change of the scope does not cancel them. A failure of an episode is
// tried again. It returns how many downloads it queued; model.ErrNotFound for an unknown podcast.
func (s *Service) DownloadPodcastEpisodes(ctx context.Context, podcastID string, episodeIDs []string) (int, error) {
	set, ok, err := s.podcastSetting(ctx, podcastID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, model.ErrNotFound
	}
	prio := prioPodcastDownloadEpisode
	var ids []string
	if len(episodeIDs) == 0 {
		prio = prioPodcastDownloadButton
		load, err := s.plannedPodcastEpisodes(ctx, set, model.PodcastScopeAll, 0, nil, planButton)
		if err != nil {
			return 0, err
		}
		for _, e := range load {
			ids = append(ids, e.ID)
		}
	} else {
		eps, err := systemq.New(s.db.Read).PodcastFeedEpisodes(ctx, sql.NullString{String: podcastID, Valid: true})
		if err != nil {
			return 0, fmt.Errorf("episodes of podcast %s: %w", podcastID, err)
		}
		for _, e := range eps {
			if slices.Contains(episodeIDs, e.ID) && e.Files == e.MissingFiles {
				ids = append(ids, e.ID)
			}
		}
	}
	write := systemq.New(s.db.Write)
	for _, id := range ids {
		if err := write.PodcastClearDownloadFailure(ctx, id); err != nil {
			return 0, err
		}
		if _, err := s.enqueuePodcastDownload(ctx, podcastDownloadPayload{EpisodeID: id, Manual: true}, prio); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// PodcastDownloads returns the setting of stored episodes of a podcast library
// (Library.podcastDownloads); model.ErrNotFound for an unknown library or one of another kind.
func (s *Service) PodcastDownloads(ctx context.Context, libraryID int64) (model.PodcastDownloads, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && lib.Kind != model.LibraryPodcasts) {
		return model.PodcastDownloads{}, model.ErrNotFound
	}
	if err != nil {
		return model.PodcastDownloads{}, fmt.Errorf("get library: %w", err)
	}
	return podcastDownloadsOf(lib), nil
}

func podcastDownloadsOf(lib appq.Library) model.PodcastDownloads {
	d := model.PodcastDownloads{Mode: lib.PodcastMode, Scope: lib.PodcastScope, LastN: int(lib.PodcastLastN), Parallel: int(lib.PodcastParallel)}
	if lib.PodcastRootID.Valid {
		id := lib.PodcastRootID.Int64
		d.RootID = &id
	}
	return d
}

// SetPodcastDownloads stores the setting of stored episodes of a podcast library
// (LibraryUpdate.podcastDownloads) and runs the planner for each of its podcasts; for a podcast
// whose effective setting changes from «Speichern» to «Nur Link» it ends every download (5.3).
// Errors: model.ErrNotFound (unknown library or another kind), *model.ValidationError (mode,
// scope, lastN, parallel, rootId: a podcast folder of the library), model.ErrRootRequired
// («Speichern» without a podcast folder), model.ErrRootNotWritable (a change to «Speichern», or to
// another folder while it stores, on a folder that fails the folder check, 5.4; while «Speichern»
// holds, the other values change whatever the check says).
func (s *Service) SetPodcastDownloads(ctx context.Context, libraryID int64, d model.PodcastDownloads) (model.PodcastDownloads, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && lib.Kind != model.LibraryPodcasts) {
		return model.PodcastDownloads{}, model.ErrNotFound
	}
	if err != nil {
		return model.PodcastDownloads{}, fmt.Errorf("get library: %w", err)
	}
	var verr *model.ValidationError
	if d.Mode != model.PodcastSave && d.Mode != model.PodcastLink {
		verr = verr.Add("mode", "invalid")
	}
	if d.Scope != model.PodcastScopeNew && d.Scope != model.PodcastScopeLastN && d.Scope != model.PodcastScopeAll {
		verr = verr.Add("scope", "invalid")
	}
	if d.LastN < 1 || d.LastN > model.PodcastLastNMax {
		verr = verr.Add("lastN", "out_of_range")
	}
	if d.Parallel < 1 || d.Parallel > model.PodcastParallelMax {
		verr = verr.Add("parallel", "out_of_range")
	}
	if d.RootID != nil {
		root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, *d.RootID)
		if err != nil || root.LibraryID != libraryID || root.Content != model.RootPodcast {
			verr = verr.Add("rootId", "unknown_root")
		}
	}
	if verr != nil {
		return model.PodcastDownloads{}, verr
	}
	was := lib
	lib.PodcastRootID = sql.NullInt64{}
	if d.RootID != nil {
		lib.PodcastRootID = sql.NullInt64{Int64: *d.RootID, Valid: true}
	}
	if d.Mode == model.PodcastSave {
		root, err := s.podcastTargetRoot(ctx, lib)
		if err != nil {
			return model.PodcastDownloads{}, err
		}
		before, err := s.podcastTargetRoot(ctx, was)
		if was.PodcastMode != model.PodcastSave || err != nil || before.ID != root.ID {
			if check := s.CheckPodcastFolder(ctx, root); !check.Writable {
				return model.PodcastDownloads{}, fmt.Errorf("%w: %s", model.ErrRootNotWritable, check.Problem)
			}
		}
	}
	read := systemq.New(s.db.Read)
	old, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{LibraryID: libraryID})
	if err != nil {
		return model.PodcastDownloads{}, err
	}
	saved := map[string]bool{}
	for _, set := range old {
		saved[set.PodcastID] = set.EffectiveMode == model.PodcastSave
	}
	if _, err := systemq.New(s.db.Write).PodcastSetLibraryDownloads(ctx, systemq.PodcastSetLibraryDownloadsParams{
		Mode: d.Mode, Scope: d.Scope, LastN: int64(d.LastN), Parallel: int64(d.Parallel), RootID: lib.PodcastRootID,
		Now: s.now().UnixMilli(), ID: libraryID,
	}); err != nil {
		return model.PodcastDownloads{}, fmt.Errorf("store podcast downloads of library %d: %w", libraryID, err)
	}
	sets, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{LibraryID: libraryID})
	if err != nil {
		return model.PodcastDownloads{}, err
	}
	for _, set := range sets {
		if err := s.QueuePodcastSettingChanged(ctx, set.PodcastID, saved[set.PodcastID] && set.EffectiveMode != model.PodcastSave); err != nil {
			return model.PodcastDownloads{}, err
		}
	}
	return s.PodcastDownloads(ctx, libraryID)
}

// PodcastSavable reports whether «Speichern» may apply to a podcast (metadata.WithPodcastSavable,
// before updatePodcast stores an own «Speichern»): nil when the folder its downloads would write
// into passes the folder check, model.ErrRootNotWritable or model.ErrRootRequired otherwise,
// model.ErrNotFound for an unknown podcast or one without feed.
func (s *Service) PodcastSavable(ctx context.Context, podcastID string) error {
	set, ok, err := s.podcastSetting(ctx, podcastID)
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrNotFound
	}
	target, err := s.podcastDownloadTarget(ctx, set)
	if errors.Is(err, errPodcastFolderMissing) {
		return fmt.Errorf("%w: %s", model.ErrRootNotWritable, model.PodcastFolderNotMounted)
	}
	if err != nil {
		return err
	}
	if check := s.CheckPodcastFolder(ctx, target.root); !check.Writable {
		return fmt.Errorf("%w: %s", model.ErrRootNotWritable, check.Problem)
	}
	return nil
}

// EstimatePodcastDownloads is estimatePodcastDownloads (12.3): what an imagined setting would load
// once, without the new episodes of later fetches and without changing anything. For a library
// (req.PodcastID empty) every podcast that follows the library's setting counts with req's mode,
// scope and N; for a podcast only that one, with req's own values (PodcastInherit: the library's).
// FreeBytes is the free space of the folder for new subscriptions. model.ErrNotFound for an
// unknown library, another kind, or a podcast of another library.
func (s *Service) EstimatePodcastDownloads(ctx context.Context, libraryID int64, req model.PodcastDownloadEstimateRequest) (model.PodcastDownloadEstimate, error) {
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && lib.Kind != model.LibraryPodcasts) {
		return model.PodcastDownloadEstimate{}, model.ErrNotFound
	}
	if err != nil {
		return model.PodcastDownloadEstimate{}, fmt.Errorf("get library: %w", err)
	}
	read := systemq.New(s.db.Read)
	sets, err := read.PodcastEffectiveSettings(ctx, systemq.PodcastEffectiveSettingsParams{LibraryID: libraryID})
	if err != nil {
		return model.PodcastDownloadEstimate{}, err
	}
	var out model.PodcastDownloadEstimate
	found := req.PodcastID == ""
	for _, set := range sets {
		mode, scope, lastN := "", "", 0
		switch {
		case req.PodcastID == "":
			if set.OwnMode.Valid && set.OwnScope.Valid {
				continue // the podcast does not follow the library
			}
			mode, scope, lastN = cmp.Or(set.OwnMode.String, req.Mode), cmp.Or(set.OwnScope.String, req.Scope), req.LastN
			if set.OwnScope.Valid {
				lastN = int(set.EffectiveLastN)
			}
		case set.PodcastID == req.PodcastID:
			found = true
			mode, scope, lastN = inherit(req.Mode, lib.PodcastMode), inherit(req.Scope, lib.PodcastScope), req.LastN
			if req.Scope == model.PodcastInherit || req.Scope == "" {
				lastN = int(lib.PodcastLastN)
			}
		default:
			continue
		}
		if mode != model.PodcastSave || scope == model.PodcastScopeNew {
			continue
		}
		load, err := s.plannedPodcastEpisodes(ctx, set, scope, lastN, nil, planEstimate)
		if err != nil {
			return model.PodcastDownloadEstimate{}, err
		}
		if len(load) == 0 {
			continue
		}
		mean, err := read.PodcastMeanFileSize(ctx, sql.NullString{String: set.PodcastID, Valid: true})
		if err != nil {
			return model.PodcastDownloadEstimate{}, err
		}
		for _, e := range load {
			out.Episodes++
			if e.EnclosureLength.Valid && e.EnclosureLength.Int64 > 0 {
				out.Bytes += e.EnclosureLength.Int64
			} else {
				out.Bytes += mean
			}
		}
	}
	if !found {
		return model.PodcastDownloadEstimate{}, model.ErrNotFound
	}
	if root, err := s.podcastTargetRoot(ctx, lib); err == nil {
		if _, free, err := s.diskSpace(root.Path); err == nil {
			out.FreeBytes = free
		}
	}
	return out, nil
}

// inherit is v, or def for PodcastInherit and the empty value.
func inherit(v, def string) string {
	if v == "" || v == model.PodcastInherit {
		return def
	}
	return v
}
