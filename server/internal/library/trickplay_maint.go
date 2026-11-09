package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Trickplay triggers besides the start (import, look-ahead, backfill), the maintenance of the sets
// (concept 18, 6.7) and the registration of the job types.

// trickplayTypes registers trickplay.generate, trickplay.series and trickplay.backfill.
func (s *Service) trickplayTypes(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobTrickplay, Group: jobs.GroupLibrary, Pool: jobs.PoolTrickplay, Trigger: jobs.TriggerEvent,
		Priority: prioTrickplayImport, Handler: s.handleTrickplay, Windowable: true,
	})
	r.Register(jobs.Type{
		Name: JobTrickplaySeries, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: prioTrickplaySeriesJob, Handler: s.handleTrickplaySeries,
	})
	r.Register(jobs.Type{
		Name: JobTrickplayBackfill, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: prioTrickplayBackfill, Every: trickplayBackfillEvery, Seed: jobs.SeedOff,
		Run: s.runTrickplayBackfill, Handler: s.handleTrickplayBackfill,
	})
}

// queueTrickplayAfterProbe is the import trigger (5.2 (2), 5.1): after the probe of a film or
// episode file that a scan added or changed after a complete scan of its library (newFile) at
// «Im Voraus», and after the probe of a file whose set of an earlier version exists at a setting
// that stores, it queues trickplay.generate at priority 2 with origin import (the time window of
// the type applies). It does not wait for a playback to end. A failure is logged.
func (s *Service) queueTrickplayAfterProbe(ctx context.Context, f systemq.MediaFile, item systemq.Item, newFile bool) {
	if s.cacheDir == "" || !chapterKind(item.Kind) {
		return
	}
	lib, err := systemq.New(s.db.Read).GetTrickplayFile(ctx, f.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.log.WarnContext(ctx, "trickplay after probe", "media_file_id", f.ID, "err", err)
		}
		return
	}
	setting := trickplayFileSetting(lib)
	version := FileVersion{Size: lib.Size, Mtime: lib.Mtime}
	switch {
	case newFile && setting.Mode == model.TrickplayAhead:
	case setting.Stores() && s.hadTrickplay(f.ID, version):
	default:
		return
	}
	if TrickplaySetComplete(s.cacheDir, f.ID, version) {
		return
	}
	if _, err := enqueueTrickplay(ctx, s.queue, trickplayPayload{MediaFileID: f.ID, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		s.log.WarnContext(ctx, "queue trickplay after probe", "media_file_id", f.ID, "err", err)
	}
}

// hadTrickplay reports whether a stored set of another version of the file exists (the file
// changed for real).
func (s *Service) hadTrickplay(mediaFileID string, v FileVersion) bool {
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return false
	}
	names, err := paths.ReadDirNames(cache, TrickplayDir, 0)
	if err != nil {
		return false
	}
	current := TrickplaySetName(mediaFileID, v)
	for _, n := range names {
		if strings.HasPrefix(n, mediaFileID+"-") && n != current {
			return true
		}
	}
	return false
}

// trickplayEpisode is one episode of a look-ahead.
type trickplayEpisode struct {
	itemID, fileID  string
	season, episode int64
}

// trickplayLookAhead returns the episodes the look-ahead of a start of episode episodeID covers
// in scope (6.4): ordered from the started episode forwards in season and episode order, then
// the episodes before it; specials (season 0) only for the whole series, last. The started
// episode is not in the list.
func trickplayLookAhead(all []trickplayEpisode, episodeID, scope string) []trickplayEpisode {
	var main, specials []trickplayEpisode
	for _, e := range all {
		if e.season == 0 {
			specials = append(specials, e)
		} else {
			main = append(main, e)
		}
	}
	at := -1
	for i, e := range main {
		if e.itemID == episodeID {
			at = i
		}
	}
	var out []trickplayEpisode
	switch scope {
	case model.TrickplayScopeNextEpisode:
		if at+1 < len(main) {
			out = append(out, main[at+1])
		}
	case model.TrickplayScopeSeasonRest:
		if at < 0 {
			return nil
		}
		season := main[at].season
		for _, e := range main[at+1:] {
			if e.season == season {
				out = append(out, e)
			}
		}
		if len(out) == 0 { // the last episode of its season: the first season after it
			for _, e := range main[at+1:] {
				if len(out) > 0 && e.season != out[0].season {
					break
				}
				out = append(out, e)
			}
		}
	default: // series
		out = append(out, main[at+1:]...)
		if at > 0 {
			out = append(out, main[:at]...)
		}
		for _, e := range specials {
			if e.itemID != episodeID {
				out = append(out, e)
			}
		}
	}
	return out
}

// handleTrickplaySeries is the look-ahead of «Beim Start alle Folgen» (6.4): for every episode in
// the scope of the library without a complete set it queues trickplay.generate with origin series
// through EnqueueOverridingWindow (a waiting import or backfill job of the file is raised), the
// next two at priority 3, the rest at priority 2 throttled, and ends. Per episode only the file
// playback takes without a choice, only present files. A system job without a profile: the
// previews are served only through the visibility of the viewer (rule 2).
func (s *Service) handleTrickplaySeries(ctx context.Context, j *jobs.Job) error {
	var p trickplaySeriesPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	if s.cacheDir == "" || p.SeriesID == "" {
		return nil
	}
	read := systemq.New(s.db.Read)
	rows, err := read.ListTrickplaySeriesFiles(ctx, sql.NullString{String: p.SeriesID, Valid: true})
	if err != nil {
		return fmt.Errorf("list episodes of %s: %w", p.SeriesID, err)
	}
	if len(rows) == 0 {
		return nil
	}
	var episodes []trickplayEpisode
	for _, r := range rows {
		if n := len(episodes); n > 0 && episodes[n-1].itemID == r.ItemID {
			continue // a further version of the same episode
		}
		episodes = append(episodes, trickplayEpisode{itemID: r.ItemID, fileID: r.MediaFileID, season: r.Season, episode: r.Episode})
	}
	file, err := read.GetTrickplayFile(ctx, episodes[0].fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load media file: %w", err)
	}
	setting := trickplayFileSetting(file)
	if setting.Mode != model.TrickplaySeriesStart {
		return nil
	}
	if !s.trickplaySpaceOK() {
		s.log.WarnContext(ctx, "cache drive below the free-space floor, no trickplay look-ahead", "series_id", p.SeriesID)
		return nil
	}
	queued := 0
	for i, e := range trickplayLookAhead(episodes, p.EpisodeID, setting.Scope) {
		f, err := read.GetTrickplayFile(ctx, e.fileID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load media file: %w", err)
		}
		if TrickplaySetComplete(s.cacheDir, e.fileID, FileVersion{Size: f.Size, Mtime: f.Mtime}) {
			continue
		}
		prio, throttled := prioTrickplaySeriesNext, false
		if i >= trickplaySeriesFull {
			prio, throttled = prioTrickplaySeries, true
		}
		if _, err := enqueueTrickplay(ctx, s.queue, trickplayPayload{MediaFileID: e.fileID, Origin: TrickplayOriginSeries, Throttled: throttled}, prio, true); err != nil {
			return err
		}
		queued++
	}
	s.log.InfoContext(ctx, "trickplay look-ahead queued", "series_id", p.SeriesID, "episode_id", p.EpisodeID, "scope", setting.Scope, "queued", queued)
	return nil
}

// runTrickplayBackfill is the Run of trickplay.backfill: one run under its key at prio.
func (s *Service) runTrickplayBackfill(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobTrickplayBackfill, Priority: prio, IdempotencyKey: trickplayBackfillKey})
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobTrickplayBackfill, err)
	}
	return []int64{id}, nil
}

// handleTrickplayBackfill queues trickplay.generate (priority 1, origin backfill) for every present,
// probed film and episode file whose kind stands at «Im Voraus» in its library and that has no
// complete set, or a complete fallback set while the file has a keyframe index now (5.4: the
// fallback set is removed and made again). Below the free-space floor it queues nothing.
func (s *Service) handleTrickplayBackfill(ctx context.Context, _ *jobs.Job) error {
	if s.cacheDir == "" {
		return nil
	}
	if !s.trickplaySpaceOK() {
		s.log.WarnContext(ctx, "cache drive below the free-space floor, no trickplay backfill")
		return nil
	}
	read := systemq.New(s.db.Read)
	rows, err := read.ListTrickplayFiles(ctx)
	if err != nil {
		return fmt.Errorf("list trickplay files: %w", err)
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return err
	}
	queued := 0
	for i, r := range rows {
		jobs.Progress(ctx, int64(i), int64(len(rows)), "")
		setting := model.LibraryTrickplay{
			Movies: model.TrickplaySetting{Mode: r.TrickplayMovies, Keep: r.TrickplayMoviesKeep == 1},
			Series: model.TrickplaySetting{Mode: r.TrickplaySeries, Keep: r.TrickplaySeriesKeep == 1, Scope: r.TrickplaySeriesScope},
		}.For(r.Kind)
		if r.Missing || !r.Probed.Bool || setting.Mode != model.TrickplayAhead {
			continue
		}
		v := FileVersion{Size: r.Size, Mtime: r.Mtime}
		if m, ok := ReadTrickplayManifest(s.cacheDir, r.ID, v); ok {
			indexed := r.KeyframesSource.Valid && r.KeyframesSource.String != KeyframesNone
			if m.Source != TrickplaySourceFallback || !indexed {
				continue
			}
			if running, err := trickplayJobRunning(ctx, read, r.ID); err != nil || running {
				continue
			}
			if rel, found := findTrickplaySet(cache, TrickplaySetName(r.ID, v)); found {
				if err := removeTrickplaySet(cache, rel); err != nil {
					s.log.WarnContext(ctx, "remove fallback trickplay set", "media_file_id", r.ID, "err", err)
					continue
				}
			}
		}
		if _, err := enqueueTrickplay(ctx, s.queue, trickplayPayload{MediaFileID: r.ID, Origin: TrickplayOriginBackfill}, prioTrickplayBackfill, false); err != nil {
			return err
		}
		queued++
	}
	jobs.Progress(ctx, int64(len(rows)), int64(len(rows)), "")
	s.log.InfoContext(ctx, "trickplay backfill queued", "files", len(rows), "queued", queued)
	return nil
}

// trickplayJobRunning reports whether trickplay.generate of a file runs right now.
func trickplayJobRunning(ctx context.Context, read *systemq.Queries, mediaFileID string) (bool, error) {
	id, err := read.GetActiveJobIDByKey(ctx, sql.NullString{String: trickplayKey(mediaFileID), Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	j, err := read.GetJob(ctx, id)
	if err != nil {
		return false, err
	}
	return j.State == string(jobs.StateRunning), nil
}

// removeTrickplaySet removes the set folder rel below the cache root.
func removeTrickplaySet(cache paths.Root, rel string) error {
	root, err := paths.Open(cache)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(rel)
}

// parseTrickplaySet splits a set folder name into media file id and version.
func parseTrickplaySet(name string) (id string, v FileVersion, ok bool) {
	if len(name) < 30 || name[26] != '-' {
		return "", FileVersion{}, false
	}
	size, mtime, found := strings.Cut(name[27:], "-")
	if !found {
		return "", FileVersion{}, false
	}
	var err1, err2 error
	v.Size, err1 = strconv.ParseInt(size, 10, 64)
	v.Mtime, err2 = strconv.ParseInt(mtime, 10, 64)
	return name[:26], v, err1 == nil && err2 == nil
}

// trickplayFileInfo is what the maintenance knows of a media file.
type trickplayFileInfo struct {
	version   FileVersion
	libraryID int64
	kind      string
	setting   model.TrickplaySetting
}

// sweepTrickplay is the hourly maintenance of the sets in library.sweep (6.7): it removes stored
// sets of other versions, of files that are gone or whose title was deleted, and stored sets of a
// kind whose setting no longer stores once no playback session of the file is open; it removes
// temporary sheets older than 6 hours, runs SweepTrickplaySessions and counts the bytes of the
// stored sets per library and kind (TrickplayUsage). The folder of a running job is never touched.
func (s *Service) sweepTrickplay(ctx context.Context) error {
	if s.cacheDir == "" {
		return nil
	}
	read := systemq.New(s.db.Read)
	rows, err := read.ListTrickplayFiles(ctx)
	if err != nil {
		return fmt.Errorf("list trickplay files: %w", err)
	}
	files := make(map[string]trickplayFileInfo, len(rows))
	for _, r := range rows {
		files[r.ID] = trickplayFileInfo{
			version: FileVersion{Size: r.Size, Mtime: r.Mtime}, libraryID: r.LibraryID, kind: r.Kind,
			setting: model.LibraryTrickplay{
				Movies: model.TrickplaySetting{Mode: r.TrickplayMovies, Keep: r.TrickplayMoviesKeep == 1},
				Series: model.TrickplaySetting{Mode: r.TrickplaySeries, Keep: r.TrickplaySeriesKeep == 1, Scope: r.TrickplaySeriesScope},
			}.For(r.Kind),
		}
	}
	cache, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return err
	}
	names, err := paths.ReadDirNames(cache, TrickplayDir, 0)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("list trickplay sets: %w", err)
	}
	now := s.now()
	usage := map[int64][2]int64{}
	removed := 0
	for _, name := range names {
		rel := TrickplayDir + "/" + name
		id, v, ok := parseTrickplaySet(name)
		if !ok {
			continue
		}
		if running, err := trickplayJobRunning(ctx, read, id); err != nil || running {
			continue
		}
		f, known := files[id]
		keep := known && f.version == v
		if keep && !f.setting.Stores() {
			open, err := read.CountActivePlaybacksOfFile(ctx, systemq.CountActivePlaybacksOfFileParams{MediaFileID: id, Now: now.UnixMilli(), ActiveAfter: 0})
			keep = err != nil || open > 0
		}
		if !keep {
			if err := removeTrickplaySet(cache, rel); err != nil {
				s.log.WarnContext(ctx, "remove trickplay set", "folder", name, "err", err)
				continue
			}
			removed++
			continue
		}
		bytes := s.sweepTrickplayTemp(ctx, cache, rel, now)
		u := usage[f.libraryID]
		if f.kind == "episode" {
			u[1] += bytes
		} else {
			u[0] += bytes
		}
		usage[f.libraryID] = u
	}
	s.trickplayMu.Lock()
	s.trickplayBytes = usage
	s.trickplayMu.Unlock()
	if removed > 0 {
		s.log.InfoContext(ctx, "stale trickplay sets removed", "sets", removed)
	}
	return SweepTrickplaySessions(ctx, s.db.Read, s.cacheDir, now)
}

// sweepTrickplayTemp removes the temporary files of a set older than 6 hours and returns the bytes
// of the files that stay.
func (s *Service) sweepTrickplayTemp(ctx context.Context, cache paths.Root, rel string, now time.Time) int64 {
	root, err := paths.Open(cache)
	if err != nil {
		return 0
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), rel)
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.HasPrefix(e.Name(), trickplayTempPrefix) && now.Sub(info.ModTime()) >= chapterTempMaxAge {
			if err := root.Remove(rel + "/" + e.Name()); err != nil {
				s.log.WarnContext(ctx, "remove temporary trickplay file", "file", e.Name(), "err", err)
			}
			continue
		}
		total += info.Size()
	}
	return total
}

// TrickplayUsage returns the bytes of the stored sets of a library per kind, as counted by the last
// hourly sweep (0 before it), and the free space of the cache drive (-1: unknown); admin's
// estimate reads it.
func (s *Service) TrickplayUsage(libraryID int64) (movies, series, cacheFree int64) {
	s.trickplayMu.Lock()
	u := s.trickplayBytes[libraryID]
	s.trickplayMu.Unlock()
	cacheFree = -1
	if s.cacheDir != "" {
		if _, free, err := s.diskSpace(s.cacheDir); err == nil {
			cacheFree = free
		}
	}
	return u[0], u[1], cacheFree
}

// SweepTrickplaySessions removes the temporary sets (6.5): of files that are gone or changed, and
// of files without a playback session open within the last TrickplaySessionKeep when the set
// itself is older than that (a download copies a fresh set first). Not the folder of a running
// job. Package playback runs it every minute (6.7), library.sweep every hour. db is a read
// connection.
func SweepTrickplaySessions(ctx context.Context, db *sql.DB, cacheDir string, now time.Time) error {
	if cacheDir == "" {
		return nil
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return err
	}
	names, err := paths.ReadDirNames(cache, TrickplaySessionDir, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list temporary trickplay sets: %w", err)
	}
	read := systemq.New(db)
	var errs []error
	for _, name := range names {
		rel := TrickplaySessionDir + "/" + name
		id, v, ok := parseTrickplaySet(name)
		if !ok {
			continue
		}
		if running, err := trickplayJobRunning(ctx, read, id); err != nil || running {
			continue
		}
		keep := false
		switch f, err := read.GetTrickplayFile(ctx, id); {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			errs = append(errs, err)
			continue
		case (FileVersion{Size: f.Size, Mtime: f.Mtime}) == v:
			abs, err := paths.Resolve(cache, rel)
			if err != nil {
				continue
			}
			if info, err := os.Stat(abs); err == nil && now.Sub(info.ModTime()) < TrickplaySessionKeep {
				keep = true
				break
			}
			open, err := read.CountPlaybacksOfFileOpenSince(ctx, systemq.CountPlaybacksOfFileOpenSinceParams{
				MediaFileID: id, Since: now.Add(-TrickplaySessionKeep).UnixMilli(),
			})
			keep = err != nil || open > 0
		}
		if keep {
			continue
		}
		if err := removeTrickplaySet(cache, rel); err != nil {
			errs = append(errs, fmt.Errorf("remove temporary trickplay set %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
