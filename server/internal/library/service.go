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

// Schedules inside Start. The table job_schedules stays unused in M1a: two fixed intervals do not
// need a cron scheduler, and the idempotency keys keep the tickers from piling up jobs.
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

	mu        sync.Mutex
	runCtx    context.Context // context of Start; watchers of libraries created later run under it
	wg        sync.WaitGroup
	watchers  map[int64]*Watcher          // by root id
	retrying  map[int64]bool              // library ids with a pending re-queue (scan was running)
	scanLocks map[int64]chan struct{}     // by library id; see lockScan
	runs      map[int64]map[*scanRun]bool // running scans by library id; see CancelScan

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

// New returns the library service. queue must work on db.Write.
func New(db *store.DB, queue *jobs.Queue, probe transcode.Runner, log *slog.Logger, opts ...Option) *Service {
	s := &Service{
		db: db, queue: queue, probe: probe, log: log, now: time.Now,
		watchers: map[int64]*Watcher{}, retrying: map[int64]bool{}, scanLocks: map[int64]chan struct{}{},
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

// ValidatePath checks that path is absolute, exists, is a directory and can be listed.
func (s *Service) ValidatePath(_ context.Context, path string) error {
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

// Create validates the request, detects the file system of the path, inserts the library with
// its root, queues the first scan and starts the watcher when Start runs. A podcast library has
// no root and no watcher; its titles come from feed subscriptions (M6).
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
	var defaultRating sql.NullInt64
	if model.AudioLibraryKind(req.Kind) {
		defaultRating = sql.NullInt64{Int64: 0, Valid: true}
		if req.DefaultRating != nil {
			defaultRating.Int64 = int64(*req.DefaultRating)
		}
	}
	if req.Kind == model.LibraryPodcasts {
		now := s.now().UnixMilli()
		lib, err := appq.New(s.db.Write).InsertLibrary(ctx, appq.InsertLibraryParams{Name: name, Kind: req.Kind, DefaultRating: defaultRating, Now: now})
		if err != nil {
			return model.Library{}, fmt.Errorf("insert library: %w", err)
		}
		s.log.InfoContext(ctx, "library created", "library_id", lib.ID, "kind", lib.Kind)
		return s.toModel(ctx, lib)
	}
	if err := s.ValidatePath(ctx, req.Path); err != nil {
		return model.Library{}, err
	}
	path := filepath.Clean(req.Path)
	roots, err := appq.New(s.db.Read).ListAllLibraryRoots(ctx)
	if err != nil {
		return model.Library{}, fmt.Errorf("list roots: %w", err)
	}
	for _, r := range roots {
		if nested(path, r.Path) || nested(r.Path, path) {
			return model.Library{}, fmt.Errorf("%w: overlaps the root of library %d", model.ErrLibraryPathInvalid, r.LibraryID)
		}
	}
	fsType, err := paths.DetectFS(path)
	if err != nil {
		return model.Library{}, fmt.Errorf("%w: %w", model.ErrLibraryPathInvalid, err)
	}
	watch, _ := watchMode(string(fsType))

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
	root, err := q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{LibraryID: lib.ID, Path: path, FsType: string(fsType), Watch: watch, Now: now})
	if err != nil {
		return model.Library{}, fmt.Errorf("insert library root: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Library{}, fmt.Errorf("create library: %w", err)
	}
	s.log.InfoContext(ctx, "library created", "library_id", lib.ID, "kind", lib.Kind, "root", path, "fs_type", string(fsType), "watch", watch)
	s.startWatcher(root)
	return s.QueueScan(ctx, lib.ID)
}

// nested reports whether path equals dir or lies below it.
func nested(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// watchMode picks the change detection of a file system class (plan 3.4) and the warning shown
// in the wizard and the dashboard.
func watchMode(fsType string) (watch, warning string) {
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
	if _, err := s.enqueueScan(ctx, id, 0); err != nil {
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

// enqueueScan queues scan.library and marks the library queued. It returns the job.
func (s *Service) enqueueScan(ctx context.Context, libraryID int64, delay time.Duration) (jobs.Job, error) {
	payload, err := json.Marshal(libraryPayload{libraryID})
	if err != nil {
		return jobs.Job{}, err
	}
	jobID, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobScanLibrary, Priority: prioScan, IdempotencyKey: scanKey(libraryID), Payload: payload, Delay: delay,
	})
	if err != nil {
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
	j, err := s.enqueueScan(ctx, libraryID, jobs.DebounceWindow)
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
		_, warning := watchMode(r.FsType)
		m.Roots = append(m.Roots, model.LibraryRoot{ID: r.ID, Path: r.Path, FSType: r.FsType, Watch: r.Watch, Warning: warning})
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

// Start runs the watchers of all roots and the schedules (full scan of every library every 6 h,
// sweep hourly) until ctx ends. It queues a scan of every library and a sweep right away, so
// changes made while the server was down are picked up.
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

	s.scheduleScans(ctx)
	s.scheduleSweep(ctx)
	scans := time.NewTicker(fullScanInterval)
	defer scans.Stop()
	sweeps := time.NewTicker(sweepInterval)
	defer sweeps.Stop()
	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return nil
		case <-scans.C:
			s.scheduleScans(ctx)
		case <-sweeps.C:
			s.scheduleSweep(ctx)
		}
	}
}

func (s *Service) scheduleScans(ctx context.Context) {
	libs, err := appq.New(s.db.Read).ListLibraries(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.log.ErrorContext(ctx, "list libraries for the scheduled scan", "err", err)
		}
		return
	}
	for _, l := range libs {
		if _, err := s.enqueueScan(ctx, l.ID, 0); err != nil && ctx.Err() == nil {
			s.log.ErrorContext(ctx, "queue scheduled scan", "library_id", l.ID, "err", err)
		}
	}
}

func (s *Service) scheduleSweep(ctx context.Context) {
	_, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobSweep, Priority: prioSweep, IdempotencyKey: "sweep"})
	if err != nil && ctx.Err() == nil {
		s.log.ErrorContext(ctx, "queue sweep", "err", err)
	}
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
	w := NewWatcher(pr, root.Watch, s.rootChanged(ctx, libraryID, root.ID), s.log.With("library_id", libraryID, "root_id", root.ID))
	s.watchers[root.ID] = w
	s.wg.Go(func() {
		if err := w.Run(ctx); err != nil && ctx.Err() == nil {
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
