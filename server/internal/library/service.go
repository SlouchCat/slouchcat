package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Defaults of the schedules of scan.library and library.sweep (RegisterTypes): the first row in
// job_schedules takes them, after that the table is the source (concept 09, ADR 0017).
const (
	fullScanInterval = 6 * time.Hour
	sweepInterval    = time.Hour
	// missingGrace is how long a title whose files are all missing stays before the soft delete.
	missingGrace = 24 * time.Hour
	// runningRetry is the wait before a watcher change is queued again while a scan runs.
	runningRetry = 10 * time.Second
	maxNameLen   = 80
)

// Service implements the libraries of Slouchcat: administration, scanner, probe jobs, watchers
// and the sweep of missing titles.
type Service struct {
	db    *store.DB
	queue *jobs.Queue
	probe transcode.Runner
	log   *slog.Logger
	now   func() time.Time
	// imageCheck queues image.fetch for titles with incomplete images after a scan (optional).
	imageCheck func(ctx context.Context, libraryID int64) (int, error)
	// metadataCheck queues metadata.match for titles still pending after a scan (optional).
	metadataCheck func(ctx context.Context, libraryID int64) (int, error)
	// newEpisodes is told the ids of new series episodes after their batch commits (optional).
	newEpisodes func(ctx context.Context, ids []string) error
	// scanAborted is told when the loss protection aborts a scan (optional, M8).
	scanAborted func(ctx context.Context, libraryName, reason string) error
	// rootRemoved is told when an administrator removed a library folder (optional, idea 02,
	// E12): the project keeps no audit table, so the notification and the Warn line are the
	// only trace of it.
	rootRemoved func(ctx context.Context, libraryName, path string, items, files int64) error
	// rootContentChanged is told when an administrator changed the content of a folder, which
	// empties it like a removal does (optional, ADR 0015).
	rootContentChanged func(ctx context.Context, libraryName, path, from, to string, items, files int64) error
	// browseRoots are the directories a new library root must lie below (WithBrowseRoots, E14);
	// empty switches the rule off.
	browseRoots []string

	mu       sync.Mutex
	runCtx   context.Context // context of Start; watchers of libraries created later run under it
	wg       sync.WaitGroup
	watchers map[int64]*Watcher // by root id
	// watcherStop ends the watcher of one root (RemoveRoot); Start's context ends them all.
	watcherStop map[int64]context.CancelFunc
	retrying    map[int64]bool              // library ids with a pending re-queue (scan was running)
	scanLocks   map[int64]chan struct{}     // by library id; see lockScan
	runs        map[int64]map[*scanRun]bool // running scans by library id; see CancelScan

	statsMu    sync.Mutex
	writeStats WriteStats // see TakeWriteStats

	// orphanMu guards orphans: extras files a scan skipped because no title owns their folder,
	// by root id and rel path, with the size and mtime they had (see deferOrphan).
	orphanMu sync.Mutex
	orphans  map[int64]map[string]fileStamp

	// pathMu guards the coalescing of watcher changes into scan.path jobs.
	pathMu       sync.Mutex
	pathPending  map[int64]map[int64]map[string]bool // library id → root id → directories not yet queued
	pathRetrying map[int64]bool                      // library ids waiting for their running path scan
}

// Option configures New.
type Option func(*Service)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithImageCheck runs check after every successful scan of a library; metadata's
// QueueIncompleteImages queues image.fetch for titles whose stored images miss a size or a file.
func WithImageCheck(check func(ctx context.Context, libraryID int64) (int, error)) Option {
	return func(s *Service) { s.imageCheck = check }
}

// WithMetadataCheck runs check after every successful full scan of a library; metadata's
// QueuePendingMetadata queues metadata.match for titles with metadata_state 'pending' and no
// active job, a one-time match for titles matched before M3 and trailer.find for pending trailers,
// so the enrichment converges after interrupted or cancelled scans (design 2.2).
func WithMetadataCheck(check func(ctx context.Context, libraryID int64) (int, error)) Option {
	return func(s *Service) { s.metadataCheck = check }
}

// WithNewEpisodes runs fn with the ids of the episode items a scan batch created, after the batch
// committed, for full and path scans alike (notify.Service.NewEpisodes, M7 E-8). An error is
// logged; the scan goes on.
func WithNewEpisodes(fn func(ctx context.Context, ids []string) error) Option {
	return func(s *Service) { s.newEpisodes = fn }
}

// WithScanAborted runs fn when the loss protection aborts a full or path scan, with the library's
// name and the abort code (root_missing, root_empty, loss_over_threshold; notify.ScanAborted, M8
// A-3). An error is logged.
func WithScanAborted(fn func(ctx context.Context, libraryName, reason string) error) Option {
	return func(s *Service) { s.scanAborted = fn }
}

// WithRootRemoved runs fn after an administrator removed a library folder, with the library's
// name, the folder's path and how many titles and media files went with it (notify.RootRemoved,
// idea 02, E12). An error is logged; the folder is gone either way.
func WithRootRemoved(fn func(ctx context.Context, libraryName, path string, items, files int64) error) Option {
	return func(s *Service) { s.rootRemoved = fn }
}

// WithRootContentChanged sets the function UpdateRoot calls after the content of a folder changed
// and its titles left the library (ADR 0015, concept 6.3). An error is logged; the change stands
// either way.
func WithRootContentChanged(fn func(ctx context.Context, libraryName, path, from, to string, items, files int64) error) Option {
	return func(s *Service) { s.rootContentChanged = fn }
}

// New returns the library service. queue must work on db.Write.
func New(db *store.DB, queue *jobs.Queue, probe transcode.Runner, log *slog.Logger, opts ...Option) *Service {
	s := &Service{
		db: db, queue: queue, probe: probe, log: log, now: time.Now,
		watchers: map[int64]*Watcher{}, watcherStop: map[int64]context.CancelFunc{},
		retrying: map[int64]bool{}, scanLocks: map[int64]chan struct{}{},
		runs:        map[int64]map[*scanRun]bool{},
		pathPending: map[int64]map[int64]map[string]bool{}, pathRetrying: map[int64]bool{},
		orphans: map[int64]map[string]fileStamp{},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Service) newID() string {
	return ulid.MustNew(ulid.Timestamp(s.now()), ulid.DefaultEntropy()).String()
}

// ValidatePath checks that path is absolute, exists, is a directory and can be listed
// (ValidateDir), without the browsable-root rule of E14; ValidateNewRoot adds that rule for a
// path that is about to become a library root.
func (s *Service) ValidatePath(_ context.Context, path string) error {
	return ValidateDir(path)
}

// ValidateDir checks that path is absolute, exists, is a directory and can be listed; its errors
// wrap model.ErrLibraryPathInvalid. The wizard's path check (M8, A-1) uses it without a service.
func ValidateDir(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("%w: not an absolute path", model.ErrLibraryPathInvalid)
	}
	root, err := paths.NewRoot(path)
	if err != nil {
		return fmt.Errorf("%w: %w", model.ErrLibraryPathInvalid, err)
	}
	r, err := paths.Open(root)
	if err != nil {
		return fmt.Errorf("%w: %w", model.ErrLibraryPathInvalid, err)
	}
	defer r.Close()
	dir, err := r.Open(".")
	if err != nil {
		return fmt.Errorf("%w: %w", model.ErrLibraryPathInvalid, err)
	}
	defer dir.Close()
	if _, err := dir.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: not readable: %w", model.ErrLibraryPathInvalid, err)
	}
	return nil
}

// Create validates the request, applies the folder rules to every root (prepareRoots), inserts
// the library with all its folders in one transaction (F-13: all of them or none), queues the first
// scan and starts the watchers. A video or audiobook library needs at least one folder (422
// roots_required); a podcast library may start without, its titles come from feeds as well.
func (s *Service) Create(ctx context.Context, req model.LibraryRequest) (model.Library, error) {
	var verr *model.ValidationError
	name := strings.TrimSpace(req.Name)
	switch {
	case name == "":
		verr = verr.Add("name", "required")
	case utf8.RuneCountInString(name) > maxNameLen:
		verr = verr.Add("name", "too_long")
	}
	if !model.ValidLibraryKind(req.Kind) {
		verr = verr.Add("kind", "invalid_value")
	}
	if verr != nil {
		return model.Library{}, verr
	}
	if req.Kind != model.LibraryPodcasts && len(req.Roots) == 0 {
		return model.Library{}, verr.Add("roots", model.CodeRootsRequired)
	}
	roots, err := s.prepareRoots(ctx, req.Kind, req.Roots, "roots")
	if err != nil {
		return model.Library{}, err
	}
	var defaultRating sql.NullInt64
	if model.AudioLibraryKind(req.Kind) {
		defaultRating = sql.NullInt64{Int64: 0, Valid: true}
		if req.DefaultRating != nil {
			defaultRating.Int64 = int64(*req.DefaultRating)
		}
	}

	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return model.Library{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	lib, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: name, Kind: req.Kind, DefaultRating: defaultRating, Now: now})
	if err != nil {
		return model.Library{}, fmt.Errorf("insert library: %w", err)
	}
	rows := make([]appq.LibraryRoot, 0, len(roots))
	for _, r := range roots {
		row, err := insertRoot(ctx, q, lib.ID, r, now)
		if err != nil {
			return model.Library{}, fmt.Errorf("insert library root: %w", err)
		}
		rows = append(rows, row)
	}
	if err := tx.Commit(); err != nil {
		return model.Library{}, fmt.Errorf("create library: %w", err)
	}
	s.log.InfoContext(ctx, "library created", "library_id", lib.ID, "kind", lib.Kind, "roots", len(rows))
	for _, row := range rows {
		s.log.InfoContext(ctx, "library root added", "library_id", lib.ID, "root_id", row.ID, "root", row.Path,
			"content", row.Content, "providers", row.Providers, "fs_type", row.FsType, "watch", row.Watch)
		s.startWatcher(row)
	}
	if len(rows) == 0 {
		return s.toModel(ctx, lib)
	}
	return s.QueueScan(ctx, lib.ID)
}

// nested reports whether path equals dir or lies below it.
func nested(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// refuseOverlap rejects a folder that is, or lies below, or holds a folder of any library. path
// must be absolute and cleaned.
//
// It compares twice: once on the paths as they are written, and once on what
// filepath.EvalSymlinks makes of them. The second pass matters because the first one compares
// text while the scanner walks what the kernel resolves: /media/zwei -> /media/Filme reads as a
// different folder and indexes the same files a second time under a second root id. A title that
// then has a file "under another root" never leaves when one of the two is removed, although the
// administrator confirmed that it would. A root that cannot be resolved right now (its share is
// not mounted) keeps its written path for the comparison, so a folder is never let in because a
// NAS was briefly away.
//
// What this does not catch: a bind mount, and two spellings of one name on a case-insensitive or
// Unicode-folding share. Both need a stat of every stored root on every call; the folders of one
// server are few and set by hand, and the damage is a double index, not an escape from the
// browsable roots.
func (s *Service) refuseOverlap(ctx context.Context, path string) error {
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list roots: %w", err)
	}
	real := resolved(path)
	for _, r := range roots {
		old := filepath.Clean(r.Path)
		oldReal := resolved(old)
		if nested(path, old) || nested(old, path) || nested(real, oldReal) || nested(oldReal, real) {
			return fmt.Errorf("%w: the root of library %d", errRootOverlap, r.LibraryID)
		}
	}
	return nil
}

// resolved is the path with its symlinks resolved, or the path itself when it cannot be resolved.
func resolved(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// WatchMode picks the change detection of a file system class (plan 3.4) and the warning shown
// in the wizard and the dashboard.
func WatchMode(fsType string) (watch, warning string) {
	switch paths.FSType(fsType) {
	case paths.FSExt4, paths.FSXFS, paths.FSBtrfs, paths.FSZFS:
		return "inotify", ""
	case paths.FSNFS, paths.FSSMB:
		return "poll", "network_share"
	default:
		return "inotify_poll", "fuse_unknown"
	}
}

// List returns all libraries with roots, scan state and counts.
func (s *Service) List(ctx context.Context) ([]model.Library, error) {
	libs, err := appq.New(s.db.Read).ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	out := make([]model.Library, 0, len(libs))
	for _, l := range libs {
		m, err := s.toModel(ctx, l)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Get returns one library or model.ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (model.Library, error) {
	l, err := appq.New(s.db.Read).GetLibrary(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, model.ErrNotFound
	}
	if err != nil {
		return model.Library{}, fmt.Errorf("get library %d: %w", id, err)
	}
	return s.toModel(ctx, l)
}

// QueueScan queues a full scan of the library; while one is queued or running it returns that one.
func (s *Service) QueueScan(ctx context.Context, id int64) (model.Library, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return model.Library{}, err
	}
	if _, err := s.enqueueScan(ctx, id, 0, prioScan); err != nil {
		return model.Library{}, err
	}
	return s.Get(ctx, id)
}

// AcceptLoss lifts the loss protection for the next scan of the library and queues it.
func (s *Service) AcceptLoss(ctx context.Context, id int64) (model.Library, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return model.Library{}, err
	}
	if err := appq.New(s.db.Write).AcceptLibraryLoss(ctx, appq.AcceptLibraryLossParams{Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: id}); err != nil {
		return model.Library{}, fmt.Errorf("accept loss: %w", err)
	}
	s.log.InfoContext(ctx, "library loss accepted", "library_id", id)
	return s.QueueScan(ctx, id)
}

// CancelScan stops the queued and running scans of a library (scan.library and scan.path,
// design 2.2): their jobs are cancelled and a running scan sees its context cancelled at once.
// The scan ends with state aborted and last_scan_error "cancelled"; changes it wrote before
// stay, and the next scan converges. Without an active scan nothing changes. It returns
// model.ErrNotFound for an unknown library.
func (s *Service) CancelScan(ctx context.Context, libraryID int64) (model.Library, error) {
	if _, err := s.Get(ctx, libraryID); err != nil {
		return model.Library{}, err
	}
	cancelled := false
	sq := systemq.New(s.db.Write)
	for _, key := range []string{scanKey(libraryID), pathScanKey(libraryID)} {
		id, err := sq.GetActiveJobIDByKey(ctx, sql.NullString{String: key, Valid: true})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return model.Library{}, fmt.Errorf("find scan job %s: %w", key, err)
		}
		ok, err := s.queue.Cancel(ctx, id)
		if err != nil {
			return model.Library{}, err
		}
		cancelled = cancelled || ok
	}
	// After the jobs: a scan that registers later checks its job and does not start.
	s.mu.Lock()
	runs := slices.Collect(maps.Keys(s.runs[libraryID]))
	s.mu.Unlock()
	for _, r := range runs {
		r.cancel(errScanCancelled)
	}
	if cancelled || len(runs) > 0 {
		if err := s.setState(ctx, libraryID, "aborted", abortCancelled); err != nil {
			return model.Library{}, err
		}
		s.log.InfoContext(ctx, "scan cancelled", "library_id", libraryID, "running", len(runs))
	}
	return s.Get(ctx, libraryID)
}

// ignoreBatch bounds the media files IgnoreFiles handles per transaction (about three rows each).
const ignoreBatch = batchRows / 3

// IgnoreFiles removes media files from the library for good (B-9 cleanup): each existing file is
// entered into ignored_files (so the scanner skips it from now on) and its media_files row is
// deleted; a movie, episode or extra left without any media file is soft-deleted. Unknown ids are
// skipped. It returns the number of files removed and of items soft-deleted.
func (s *Service) IgnoreFiles(ctx context.Context, mediaFileIDs []string, reason string) (files, itemsDeleted int, err error) {
	for chunk := range slices.Chunk(mediaFileIDs, ignoreBatch) {
		f, d, err := s.ignoreChunk(ctx, chunk, reason)
		files += f
		itemsDeleted += d
		if err != nil {
			return files, itemsDeleted, err
		}
	}
	if files > 0 {
		s.log.InfoContext(ctx, "media files ignored", "files", files, "items_deleted", itemsDeleted, "reason", reason)
	}
	return files, itemsDeleted, nil
}

func (s *Service) ignoreChunk(ctx context.Context, ids []string, reason string) (files, itemsDeleted int, err error) {
	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for _, id := range ids {
		mf, err := q.GetMediaFileSystem(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, 0, fmt.Errorf("load media file %s: %w", id, err)
		}
		if err := q.InsertIgnoredFile(ctx, systemq.InsertIgnoredFileParams{RootID: mf.RootID, RelPath: mf.RelPath, Reason: reason, Now: now}); err != nil {
			return 0, 0, fmt.Errorf("ignore media file %s: %w", id, err)
		}
		if err := q.DeleteMediaFile(ctx, id); err != nil {
			return 0, 0, fmt.Errorf("delete media file %s: %w", id, err)
		}
		n, err := q.SoftDeleteItemWithoutFiles(ctx, systemq.SoftDeleteItemWithoutFilesParams{Now: sql.NullInt64{Int64: now, Valid: true}, ID: mf.ItemID})
		if err != nil {
			return 0, 0, fmt.Errorf("soft delete item %s: %w", mf.ItemID, err)
		}
		files++
		itemsDeleted += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("store ignored files: %w", err)
	}
	return files, itemsDeleted, nil
}

// scannable reports whether a full scan of the library would have anything to walk. A library
// without a folder has not: a podcast library never had one (its titles come from feeds, M6),
// and a video or audio library can lose its last one (idea 02, E13). Both keep their settings
// and their titles; they simply do not scan.
func (s *Service) scannable(ctx context.Context, libraryID int64) (bool, error) {
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return false, fmt.Errorf("list roots of library %d: %w", libraryID, err)
	}
	return len(roots) > 0, nil
}

// enqueueScan queues scan.library with prio and marks the library queued. A scan that already
// waits under the key is raised to prio and never lowered (RaiseQueuedPriority), so a manual run
// overtakes the scheduled one. It returns the job, or the zero job when the library has no folder
// to walk: queueing one would leave the library "queued" for good, and letting it run would end
// in "done" with zero files, which reads like a successful scan of an empty folder (idea 02,
// step 14).
func (s *Service) enqueueScan(ctx context.Context, libraryID int64, delay time.Duration, prio int) (jobs.Job, error) {
	switch ok, err := s.scannable(ctx, libraryID); {
	case err != nil:
		return jobs.Job{}, err
	case !ok:
		return jobs.Job{}, nil
	}
	payload, err := json.Marshal(libraryPayload{libraryID})
	if err != nil {
		return jobs.Job{}, err
	}
	jobID, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobScanLibrary, Priority: prio, IdempotencyKey: scanKey(libraryID), Payload: payload, Delay: delay,
	})
	if err != nil {
		return jobs.Job{}, err
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, jobID, prio); err != nil {
		return jobs.Job{}, err
	}
	if err := appq.New(s.db.Write).MarkLibraryScanQueued(ctx, appq.MarkLibraryScanQueuedParams{Now: s.now().UnixMilli(), ID: libraryID}); err != nil {
		return jobs.Job{}, fmt.Errorf("mark scan queued: %w", err)
	}
	return s.queue.Get(ctx, jobID)
}

// triggerScan is called by the watchers of a library: it queues a debounced scan. When the scan
// of that library is already running, the change may have been missed by its walk, so the scan
// is queued again once the running one has finished.
func (s *Service) triggerScan(ctx context.Context, libraryID int64) {
	j, err := s.enqueueScan(ctx, libraryID, jobs.DebounceWindow, prioScan)
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "queue scan after change", "library_id", libraryID, "err", err)
		}
		return
	}
	if j.State != jobs.StateRunning {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retrying[libraryID] {
		return
	}
	s.retrying[libraryID] = true
	s.wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-time.After(runningRetry):
		}
		s.mu.Lock()
		delete(s.retrying, libraryID)
		s.mu.Unlock()
		if ctx.Err() == nil {
			s.triggerScan(ctx, libraryID)
		}
	})
}

// ErrScanNotQueued means that the directories of a RequestPathScan reached neither a queued job
// nor a scheduled retry, so nothing will scan them unless something else changes.
var ErrScanNotQueued = errors.New("path scan was not queued")

// RequestPathScan queues a targeted scan of directories of a root for a caller outside the
// watcher: the webhook receiver of package integrations reports what Radarr or Sonarr changed
// (I-8, decision 5.6). The directories are relative to the root ("." is the root itself) and
// join the same coalescing as a watcher event, so a burst of webhooks becomes one scan.
//
// Unlike the watcher, that caller answers a request and records what came of it, so it needs to
// know: the directories are either queued (the library has none pending any more, which also
// covers the switch to a full scan) or waiting for the retry of a running job. Anything else —
// a failed enqueue, an unreadable payload — leaves them pending with nobody to pick them up and
// returns ErrScanNotQueued.
func (s *Service) RequestPathScan(ctx context.Context, libraryID, rootID int64, dirs []string) error {
	s.triggerPathScan(ctx, libraryID, rootID, dirs)
	s.pathMu.Lock()
	defer s.pathMu.Unlock()
	if _, pending := s.pathPending[libraryID]; !pending || s.pathRetrying[libraryID] {
		return nil
	}
	return fmt.Errorf("%w: library %d", ErrScanNotQueued, libraryID)
}

// triggerPathScan is called by the watcher of a root with the directories that changed: it
// merges them into the pending scan.path job of the library, debounced like triggerScan. When
// that job is already running, the directories wait here and are queued once it has finished.
func (s *Service) triggerPathScan(ctx context.Context, libraryID, rootID int64, dirs []string) {
	s.pathMu.Lock()
	defer s.pathMu.Unlock()
	byRoot := s.pathPending[libraryID]
	if byRoot == nil {
		byRoot = map[int64]map[string]bool{}
		s.pathPending[libraryID] = byRoot
	}
	if byRoot[rootID] == nil {
		byRoot[rootID] = map[string]bool{}
	}
	for _, d := range dirs {
		byRoot[rootID][d] = true
	}
	if !s.pathRetrying[libraryID] {
		s.flushPathScan(ctx, libraryID)
	}
}

// flushPathScan queues the pending directories of a library; s.pathMu must be held.
func (s *Service) flushPathScan(ctx context.Context, libraryID int64) {
	pending := pathScanPayload{LibraryID: libraryID}
	for rootID, dirs := range s.pathPending[libraryID] {
		pending.Roots = append(pending.Roots, pathScanRoot{RootID: rootID, Dirs: slices.Collect(maps.Keys(dirs))})
	}
	if len(pending.Roots) == 0 {
		return
	}
	payload, err := json.Marshal(normalizePathScan(pending))
	if err != nil {
		s.log.ErrorContext(ctx, "encode path scan", "library_id", libraryID, "err", err)
		return
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobScanPath, Priority: prioPathScan, IdempotencyKey: pathScanKey(libraryID), Payload: payload, Delay: jobs.DebounceWindow,
	})
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "queue path scan after change", "library_id", libraryID, "err", err)
		}
		return // the directories stay pending for the next change
	}
	j, err := s.queue.Get(ctx, id)
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "load queued path scan", "library_id", libraryID, "err", err)
		}
		return
	}
	queued := false
	if j.State == jobs.StateQueued {
		// Enqueue returned either the new job or a pending one holding the key: merge.
		var held pathScanPayload
		if err := json.Unmarshal(j.Payload, &held); err != nil {
			s.log.ErrorContext(ctx, "decode queued path scan", "job_id", j.ID, "err", err)
			return
		}
		held.Roots = append(held.Roots, pending.Roots...)
		merged := normalizePathScan(held)
		if pathScanDirs(merged) > MaxChangedDirs {
			delete(s.pathPending, libraryID)
			s.triggerScan(ctx, libraryID)
			return
		}
		data, err := json.Marshal(merged)
		if err != nil {
			s.log.ErrorContext(ctx, "encode path scan", "library_id", libraryID, "err", err)
			return
		}
		queued = string(data) == string(j.Payload)
		if !queued {
			if queued, err = s.queue.SetQueuedPayload(ctx, j.ID, data); err != nil {
				if ctx.Err() == nil {
					s.log.WarnContext(ctx, "merge path scan", "job_id", j.ID, "err", err)
				}
				return
			}
		}
	}
	if queued {
		delete(s.pathPending, libraryID)
		if err := appq.New(s.db.Write).MarkLibraryScanQueued(ctx, appq.MarkLibraryScanQueuedParams{Now: s.now().UnixMilli(), ID: libraryID}); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "mark scan queued", "library_id", libraryID, "err", err)
		}
		return
	}
	// The job holding the key runs (or has just ended): its directories were read before these
	// changes, so they are queued again once it has finished.
	s.pathRetrying[libraryID] = true
	s.wg.Go(func() {
		select {
		case <-ctx.Done():
		case <-time.After(runningRetry):
		}
		s.pathMu.Lock()
		defer s.pathMu.Unlock()
		delete(s.pathRetrying, libraryID)
		if ctx.Err() == nil {
			s.flushPathScan(ctx, libraryID)
		}
	})
}

// normalizePathScan merges the entries of the same root and sorts roots and directories.
func normalizePathScan(p pathScanPayload) pathScanPayload {
	byRoot := map[int64][]string{}
	for _, r := range p.Roots {
		byRoot[r.RootID] = append(byRoot[r.RootID], r.Dirs...)
	}
	out := pathScanPayload{LibraryID: p.LibraryID, Roots: make([]pathScanRoot, 0, len(byRoot))}
	for _, id := range slices.Sorted(maps.Keys(byRoot)) {
		out.Roots = append(out.Roots, pathScanRoot{RootID: id, Dirs: cleanDirs(byRoot[id])})
	}
	return out
}

func pathScanDirs(p pathScanPayload) int {
	n := 0
	for _, r := range p.Roots {
		n += len(r.Dirs)
	}
	return n
}

func (s *Service) toModel(ctx context.Context, l appq.Library) (model.Library, error) {
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, l.ID)
	if err != nil {
		return model.Library{}, fmt.Errorf("list roots of library %d: %w", l.ID, err)
	}
	sq := systemq.New(s.db.Read)
	items, err := sq.CountLibraryItems(ctx, l.ID)
	if err != nil {
		return model.Library{}, fmt.Errorf("count items: %w", err)
	}
	files, err := sq.CountLibraryMediaFiles(ctx, l.ID)
	if err != nil {
		return model.Library{}, fmt.Errorf("count media files: %w", err)
	}
	m := model.Library{
		ID: l.ID, Name: l.Name, Kind: l.Kind, CreatedAt: time.UnixMilli(l.CreatedAt).UTC(),
		DefaultRating: optRating(l.DefaultRating),
		Roots:         make([]model.LibraryRoot, 0, len(roots)),
		Scan: model.LibraryScan{
			State:          l.LastScanState,
			StartedAt:      timePtr(l.LastScanStartedAt),
			FinishedAt:     timePtr(l.LastScanFinishedAt),
			FilesSeen:      int(l.ScanFilesSeen),
			FilesTotal:     int(l.ScanFilesTotal),
			LastError:      l.LastScanError.String,
			LossAcceptedAt: timePtr(l.LossAcceptedAt),
			ItemCount:      int(items),
			FileCount:      int(files),
		},
	}
	for _, r := range roots {
		_, warning := WatchMode(r.FsType)
		content, err := sq.CountLibraryRootContent(ctx, r.ID)
		if err != nil {
			return model.Library{}, fmt.Errorf("count content of root %d: %w", r.ID, err)
		}
		m.Roots = append(m.Roots, model.LibraryRoot{
			ID: r.ID, Path: r.Path, FSType: r.FsType, Watch: r.Watch, Warning: warning,
			Content: r.Content, Providers: model.ParseProviders(r.Providers),
			ItemCount: int(content.Items), FileCount: int(content.Files),
		})
	}
	return m, nil
}

// optRating maps a nullable rating column.
func optRating(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	r := int(v.Int64)
	return &r
}

func timePtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}

// Start runs the watchers of all roots until ctx ends. The full scan of every library (every
// 6 h and at the start, so changes made while the server was down are picked up) and the hourly
// sweep are schedules of the job scheduler (RegisterTypes), not loops of this service.
func (s *Service) Start(ctx context.Context) error {
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return fmt.Errorf("list library roots: %w", err)
	}
	s.mu.Lock()
	s.runCtx = ctx
	s.mu.Unlock()
	for _, r := range roots {
		s.startWatcher(r)
	}
	<-ctx.Done()
	s.wg.Wait()
	return nil
}

// runScans is the Run of scan.library (jobs.Type): it queues a scan of every library with prio
// and returns the ids of the jobs (a library without a folder queues none).
func (s *Service) runScans(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	libs, err := appq.New(s.db.Read).ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries for the scheduled scan: %w", err)
	}
	ids := make([]int64, 0, len(libs))
	for _, l := range libs {
		j, err := s.enqueueScan(ctx, l.ID, 0, prio)
		if err != nil {
			return ids, fmt.Errorf("queue scan of library %d: %w", l.ID, err)
		}
		if j.ID != 0 {
			ids = append(ids, j.ID)
		}
	}
	return ids, nil
}

// runSweep is the Run of library.sweep: one sweep job under the key "sweep", raised to prio when
// it already waits.
func (s *Service) runSweep(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobSweep, Priority: prio, IdempotencyKey: "sweep"})
	if err != nil {
		return nil, err
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

// startWatcher starts the watcher of root when Start runs; before Start it does nothing (Start
// starts all roots).
func (s *Service) startWatcher(root appq.LibraryRoot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := s.runCtx
	if ctx == nil || ctx.Err() != nil || s.watchers[root.ID] != nil {
		return
	}
	pr, err := paths.NewRoot(root.Path)
	if err != nil {
		s.log.Error("watch library root", "root_id", root.ID, "err", err)
		return
	}
	libraryID := root.LibraryID
	// Every watcher gets its own cancel, so RemoveRoot can end one without ending the rest.
	wctx, stop := context.WithCancel(ctx)
	w := NewWatcher(pr, root.Watch, s.rootChanged(wctx, libraryID, root.ID), s.log.With("library_id", libraryID, "root_id", root.ID))
	s.watchers[root.ID] = w
	s.watcherStop[root.ID] = stop
	s.wg.Go(func() {
		defer stop()
		if err := w.Run(wctx); err != nil && wctx.Err() == nil {
			s.log.Error("library watcher stopped", "library_id", libraryID, "root_id", root.ID, "err", err)
		}
	})
}

// rootChanged returns the change callback of the watcher of a root: a path scan of the changed
// directories, or a full scan when the watcher reports nil (too many changes, lost events).
func (s *Service) rootChanged(ctx context.Context, libraryID, rootID int64) func(dirs []string) {
	return func(dirs []string) {
		if dirs == nil {
			s.triggerScan(ctx, libraryID)
			return
		}
		s.triggerPathScan(ctx, libraryID, rootID, dirs)
	}
}

// WatchedDirs returns the number of directories the running watchers hold with inotify, for the
// inotify use on the dashboard (M8, A-3).
func (s *Service) WatchedDirs() int64 {
	var n int64
	for _, st := range s.WatcherStats() {
		n += int64(st.Dirs)
	}
	return n
}

// WatcherStats returns the statistics of the running watchers by root id.
func (s *Service) WatcherStats() map[int64]WatchStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int64]WatchStats, len(s.watchers))
	for id, w := range s.watchers {
		out[id] = w.Stats()
	}
	return out
}
