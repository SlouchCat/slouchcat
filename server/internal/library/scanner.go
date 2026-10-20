package library

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

const (
	// batchRows bounds the rows written per transaction (plan 3.6: lease ≫ transaction).
	batchRows = 500
	// maxRowsPerOp is the most rows one operation of a movie library writes: item, media file,
	// metadata.match and probe.file for a new file; media file, restored item and probe.file for
	// an update.
	maxRowsPerOp = 4
	// maxRowsPerSeriesOp is the most rows one operation of a series library writes: series, its
	// metadata.match, season, episode, media file and probe.file for a new series; episode, its
	// metadata.match, season, media file, probe.file and the restore of a soft-deleted series and
	// season for a known one.
	maxRowsPerSeriesOp = 8
	// lossThresholdPercent is the share of known files that may disappear in one scan.
	lossThresholdPercent = 20
	// relinkWindow is how long a soft-deleted title can come back through a new file with the same
	// TMDB, TVDB or IMDb id, keeping its user data (plan 3.2).
	relinkWindow = 30 * 24 * time.Hour
)

// Abort codes stored in libraries.last_scan_error with state aborted (plan 3.2), and the code of
// a scan the administrator cancelled (CancelScan).
const (
	abortRootMissing = "root_missing"
	abortRootEmpty   = "root_empty"
	abortLoss        = "loss_over_threshold"
	abortCancelled   = "cancelled"
)

// errScanCancelled is the cancel cause of a scan stopped by CancelScan; a server shutdown cancels
// with context.Canceled instead and leaves the scan queued.
var errScanCancelled = errors.New("scan cancelled by the administrator")

// diskFile is a candidate video file found by the walk.
type diskFile struct {
	rel    string
	size   int64
	mtime  int64 // Unix milliseconds
	id     fileID
	parsed ParsedName // ParseMoviePath or ParseSeriesPath, by library kind
}

// rootScan is the state of one root during a scan.
type rootScan struct {
	root     appq.LibraryRoot
	proot    paths.Root
	disk     []diskFile
	known    []systemq.MediaFile
	ignored  map[string]bool // rel paths of ignored_files
	deferred []diskFile      // unchanged extras an earlier scan skipped for want of a parent
	// subs lists the subtitle files of every directory the scan read (I-5 sidecar sync).
	subs map[string][]string
}

// fileStamp is the size and mtime of a file on disk.
type fileStamp struct{ size, mtime int64 }

// rememberOrphans records extras files a batch skipped because no title owns their folder, so
// the next scans neither hash nor write them while they and the library stay unchanged.
func (s *Service) rememberOrphans(files []scanOp) {
	if len(files) == 0 {
		return
	}
	s.orphanMu.Lock()
	defer s.orphanMu.Unlock()
	for _, op := range files {
		m := s.orphans[op.rs.root.ID]
		if m == nil {
			m = map[string]fileStamp{}
			s.orphans[op.rs.root.ID] = m
		}
		m[op.disk.rel] = fileStamp{op.disk.size, op.disk.mtime}
	}
}

// deferOrphan reports whether d is an extras file a scan skipped before, unchanged since.
func (s *Service) deferOrphan(rootID int64, d diskFile) bool {
	s.orphanMu.Lock()
	defer s.orphanMu.Unlock()
	st, ok := s.orphans[rootID][d.rel]
	return ok && st == fileStamp{d.size, d.mtime}
}

// reviveDeferred handles the skipped extras of a scan after planning: when the scan changes
// nothing else they count as unchanged (no hash, no item index, no write); otherwise a title may
// own their folder now, so they are planned as new files again (and remembered again if a batch
// skips them once more). It returns the operations and the number of unchanged extras.
func (s *Service) reviveDeferred(ctx context.Context, scans []*rootScan, ops []scanOp) ([]scanOp, int, error) {
	n := 0
	for _, rs := range scans {
		n += len(rs.deferred)
	}
	if n == 0 || len(ops) == 0 {
		return ops, n, nil
	}
	for _, rs := range scans {
		if len(rs.deferred) == 0 {
			continue
		}
		r, err := paths.Open(rs.proot)
		if err != nil {
			return nil, 0, err
		}
		s.orphanMu.Lock()
		for _, d := range rs.deferred {
			delete(s.orphans[rs.root.ID], d.rel)
		}
		s.orphanMu.Unlock()
		for _, d := range rs.deferred {
			h, err := PartialHash(r, d.rel)
			if err != nil {
				s.log.WarnContext(ctx, "file not readable, skipped", "root_id", rs.root.ID, "path", d.rel, "err", err)
				continue
			}
			ops = append(ops, scanOp{kind: opNew, rs: rs, disk: d, hash: h})
		}
		_ = r.Close()
		rs.deferred = nil
	}
	return ops, 0, nil
}

type opKind int

const (
	opNew     opKind = iota // file never seen: item (if needed) and media file
	opUpdate                // known file at a new path, changed in place, or back after missing
	opMissing               // known file not found
)

type scanOp struct {
	kind    opKind
	rs      *rootScan
	disk    diskFile
	known   *systemq.MediaFile
	hash    string
	reprobe bool
}

// scanAbort is a scan that stopped before writing anything.
type scanAbort struct{ code string }

func (a scanAbort) Error() string { return "scan aborted: " + a.code }

// parsePath parses rel with the name parser of the library kind.
func parsePath(kind, rel string) ParsedName {
	if kind == "series" {
		return ParseSeriesPath(rel)
	}
	return ParseMoviePath(rel)
}

func (s *Service) handleScan(ctx context.Context, j *jobs.Job) error {
	var p libraryPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, p.LibraryID)
	if errors.Is(err, sql.ErrNoRows) {
		s.log.InfoContext(ctx, "scan of a deleted library skipped", "library_id", p.LibraryID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load library %d: %w", p.LibraryID, err)
	}
	return s.runCancellable(ctx, j, lib.ID, func(ctx context.Context) error { return s.scan(ctx, lib) })
}

// Scan runs a full scan of lib (see the package documentation). Aborts and cancellations
// (CancelScan) are recorded in the library and return nil; other failures are recorded as failed
// and returned.
func (s *Service) Scan(ctx context.Context, lib appq.Library) error {
	return s.runCancellable(ctx, nil, lib.ID, func(ctx context.Context) error { return s.scan(ctx, lib) })
}

// scanRun is a running scan that CancelScan can stop.
type scanRun struct{ cancel context.CancelCauseFunc }

// runCancellable runs a scan of a library so that CancelScan can stop it: the scan is registered
// before it waits for the scan lock, and a job that CancelScan cancelled between its claim and the
// registration does not start. A cancelled scan ends with state aborted and last_scan_error
// "cancelled"; the next queued scan runs normally and converges.
func (s *Service) runCancellable(ctx context.Context, j *jobs.Job, libraryID int64, scan func(context.Context) error) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	run := &scanRun{cancel: cancel}
	s.mu.Lock()
	if s.runs[libraryID] == nil {
		s.runs[libraryID] = map[*scanRun]bool{}
	}
	s.runs[libraryID][run] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.runs[libraryID], run)
		s.mu.Unlock()
	}()
	if j != nil && j.ID != 0 {
		if cur, err := s.queue.Get(ctx, j.ID); err == nil && cur.State == jobs.StateCancelled {
			return nil // CancelScan recorded the end
		}
	}
	err := scan(ctx)
	if err != nil && errors.Is(context.Cause(ctx), errScanCancelled) {
		s.log.InfoContext(ctx, "scan cancelled", "library_id", libraryID)
		return s.setState(context.WithoutCancel(ctx), libraryID, "aborted", abortCancelled)
	}
	return err
}

func (s *Service) scan(ctx context.Context, lib appq.Library) error {
	unlock, err := s.lockScan(ctx, lib.ID)
	if err != nil {
		return err
	}
	defer unlock()
	// Reloaded under the lock: a path scan that ran meanwhile may have used the accepted loss.
	lib, err = appq.New(s.db.Read).GetLibrary(ctx, lib.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load library: %w", err)
	}
	start := s.now()
	if err := s.setState(ctx, lib.ID, "running", ""); err != nil {
		return err
	}
	stats, err := s.scanLibrary(ctx, lib)
	if done, err := s.endScan(ctx, lib, "library scan", err); done {
		return err
	}
	s.log.InfoContext(ctx, "library scan done", "library_id", lib.ID, "kind", lib.Kind, "files", stats.files, "new", stats.created,
		"updated", stats.updated, "missing", stats.missing, "items_created", stats.items, "duration", s.now().Sub(start))
	// Artwork and metadata are not part of the scan result: a failed check is logged, the scan
	// stays done.
	if s.imageCheck != nil {
		if n, err := s.imageCheck(ctx, lib.ID); err != nil {
			s.log.WarnContext(ctx, "image check after scan failed", "library_id", lib.ID, "err", err)
		} else if n > 0 {
			s.log.InfoContext(ctx, "titles with incomplete images queued for image.fetch", "library_id", lib.ID, "titles", n)
		}
	}
	if s.metadataCheck != nil {
		if n, err := s.metadataCheck(ctx, lib.ID); err != nil {
			s.log.WarnContext(ctx, "metadata check after scan failed", "library_id", lib.ID, "err", err)
		} else if n > 0 {
			s.log.InfoContext(ctx, "titles queued for metadata.match or trailer.find", "library_id", lib.ID, "titles", n)
		}
	}
	return nil
}

// endScan records the end of a scan that returned err: an abort as aborted (nil error), an
// interruption as queued again (the job runs again and converges), a cancellation not at all
// (runCancellable records it), any other error as failed. It reports whether the scan ended that
// way; for err == nil the caller finishes the scan.
func (s *Service) endScan(ctx context.Context, lib appq.Library, what string, err error) (bool, error) {
	var abort scanAbort
	switch {
	case err == nil:
		return false, nil
	case errors.As(err, &abort):
		s.log.WarnContext(ctx, what+" aborted, nothing changed", "library_id", lib.ID, "reason", abort.code)
		return true, s.finish(ctx, lib, "aborted", abort.code, 0, 0)
	case errors.Is(context.Cause(ctx), errScanCancelled):
		return true, err
	case ctx.Err() != nil:
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.setState(bg, lib.ID, "queued", "")
		return true, err
	default:
		s.log.ErrorContext(ctx, what+" failed", "library_id", lib.ID, "err", err)
		if ferr := s.finish(context.WithoutCancel(ctx), lib, "failed", err.Error(), 0, 0); ferr != nil {
			return true, errors.Join(err, ferr)
		}
		return true, err
	}
}

// lockScan serializes the scans of one library: a full scan and a path scan on two workers would
// plan against the same rows and could insert the same new file twice. It waits until ctx ends.
func (s *Service) lockScan(ctx context.Context, libraryID int64) (func(), error) {
	s.mu.Lock()
	ch := s.scanLocks[libraryID]
	if ch == nil {
		ch = make(chan struct{}, 1)
		s.scanLocks[libraryID] = ch
	}
	s.mu.Unlock()
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) setState(ctx context.Context, id int64, state, msg string) error {
	err := appq.New(s.db.Write).SetLibraryScanStatus(ctx, appq.SetLibraryScanStatusParams{
		State: state, Error: sql.NullString{String: msg, Valid: msg != ""}, Now: s.now().UnixMilli(), ID: id,
	})
	if err != nil {
		return fmt.Errorf("set scan state %s: %w", state, err)
	}
	return nil
}

func (s *Service) finish(ctx context.Context, lib appq.Library, state, msg string, seen, total int) error {
	if state == "done" {
		q := appq.New(s.db.Write)
		now := s.now().UnixMilli()
		if err := q.SetLibraryScanProgress(ctx, appq.SetLibraryScanProgressParams{Seen: int64(seen), Total: int64(total), Now: now, ID: lib.ID}); err != nil {
			return fmt.Errorf("set scan progress: %w", err)
		}
		if lib.LossAcceptedAt.Valid {
			if err := q.ClearLibraryLossAccepted(ctx, appq.ClearLibraryLossAcceptedParams{Now: now, ID: lib.ID, AcceptedAt: lib.LossAcceptedAt}); err != nil {
				return fmt.Errorf("clear accepted loss: %w", err)
			}
		}
	}
	return s.setState(ctx, lib.ID, state, msg)
}

type scanStats struct {
	files, created, updated, missing, items int
}

// openRoot prepares the scan of one root: its known media files and its ignored files.
func (s *Service) openRoot(ctx context.Context, root appq.LibraryRoot) (*rootScan, error) {
	sq := systemq.New(s.db.Read)
	rs := &rootScan{root: root, ignored: map[string]bool{}}
	var err error
	if rs.known, err = sq.ListMediaFilesByRoot(ctx, root.ID); err != nil {
		return nil, fmt.Errorf("list media files of root %d: %w", root.ID, err)
	}
	ignored, err := sq.ListIgnoredFilesByRoot(ctx, root.ID)
	if err != nil {
		return nil, fmt.Errorf("list ignored files of root %d: %w", root.ID, err)
	}
	for _, rel := range ignored {
		rs.ignored[rel] = true
	}
	return rs, nil
}

func (s *Service) scanLibrary(ctx context.Context, lib appq.Library) (scanStats, error) {
	var stats scanStats
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, lib.ID)
	if err != nil {
		return stats, fmt.Errorf("list roots: %w", err)
	}
	scans := make([]*rootScan, 0, len(roots))
	knownActive := 0
	for _, root := range roots {
		rs, err := s.openRoot(ctx, root)
		if err != nil {
			return stats, err
		}
		rs.proot, err = paths.NewRoot(root.Path)
		if err != nil {
			return stats, scanAbort{abortRootMissing}
		}
		if rs.disk, rs.subs, err = walkRoot(ctx, rs.proot, lib.Kind, rs.ignored, s.log); err != nil {
			if errors.Is(err, errRootMissing) {
				return stats, scanAbort{abortRootMissing}
			}
			return stats, err
		}
		active := 0
		for _, k := range rs.known {
			if !k.MissingSince.Valid {
				active++
			}
		}
		if len(rs.disk) == 0 && active > 0 && !lib.LossAcceptedAt.Valid {
			return stats, scanAbort{abortRootEmpty}
		}
		knownActive += active
		scans = append(scans, rs)
	}

	var ops []scanOp
	unchanged, lost := 0, 0
	for _, rs := range scans {
		o, u, l, err := s.planRoot(ctx, rs, nil)
		if err != nil {
			return stats, err
		}
		ops = append(ops, o...)
		unchanged += u
		lost += l
		stats.files += len(rs.disk)
	}
	if knownActive > 0 && lost*100 > knownActive*lossThresholdPercent && !lib.LossAcceptedAt.Valid {
		s.log.WarnContext(ctx, "too many files missing", "library_id", lib.ID, "missing", lost, "known", knownActive)
		return stats, scanAbort{abortLoss}
	}
	ops, deferred, err := s.reviveDeferred(ctx, scans, ops)
	if err != nil {
		return stats, err
	}
	unchanged += deferred

	total := unchanged + len(ops)
	seen := unchanged
	if len(ops) > 0 {
		idx, err := s.loadItemIndex(ctx, lib, scans)
		if err != nil {
			return stats, err
		}
		if seen, err = s.applyOps(ctx, lib, idx, ops, seen, total, true, &stats); err != nil {
			return stats, err
		}
	}
	for _, rs := range scans {
		if err := s.syncSidecars(ctx, rs, nil); err != nil {
			return stats, err
		}
	}
	return stats, s.finish(ctx, lib, "done", "", seen, total)
}

// applyOps writes ops in batches of at most batchRows rows, checking ctx between batches. With
// progress (full scan) each batch records seen and total; it returns seen after the last batch.
func (s *Service) applyOps(ctx context.Context, lib appq.Library, idx *itemIndex, ops []scanOp, seen, total int, progress bool, stats *scanStats) (int, error) {
	orderOps(ops)
	per := batchRows / maxRowsPerOp
	if lib.Kind == "series" {
		per = batchRows / maxRowsPerSeriesOp
	}
	for i := 0; i < len(ops); {
		if err := ctx.Err(); err != nil {
			return seen, err
		}
		end := min(len(ops), i+per)
		var prog *scanProgress
		if progress {
			prog = &scanProgress{seen: seen + end - i, total: total}
		}
		n, err := s.applyBatch(ctx, lib, idx, ops[i:end], prog, stats)
		if err != nil {
			return seen, err
		}
		seen += n
		i = end
	}
	return seen, nil
}

// orderOps moves new extras behind all other operations, so the title an extra belongs to (a
// movie, series or season found in the same scan) exists when the extra is written.
func orderOps(ops []scanOp) {
	extra := func(op scanOp) int {
		if op.kind == opNew && op.disk.parsed.Extra {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(ops, func(a, b scanOp) int { return cmp.Compare(extra(a), extra(b)) })
}

var errRootMissing = errors.New("library root missing")

// walkRoot collects the video files of a root that become titles or extras of the library kind
// (see wanted) and the subtitle files by directory (sidecars, I-5); samples, ignored entries and
// ignored files are skipped. A root that cannot be
// opened is errRootMissing; any other walk error fails the scan, because a partial walk would
// look like lost files. Symlinked directories are not followed; a symlinked file whose target
// cannot be read inside the root (it leaves the root, dangles, or is not readable) is logged and
// skipped.
func walkRoot(ctx context.Context, root paths.Root, kind string, ignored map[string]bool, log *slog.Logger) ([]diskFile, map[string][]string, error) {
	r, err := paths.Open(root)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errRootMissing, err)
	}
	defer r.Close()
	if fi, err := r.Stat("."); err != nil || !fi.IsDir() {
		return nil, nil, errRootMissing
	}
	var files []diskFile
	subs := map[string][]string{}
	noMarker := map[string]int{}
	err = paths.Walk(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if Ignored(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isSubtitle(d.Name()) {
			subs[path.Dir(p)] = append(subs[path.Dir(p)], d.Name())
			return nil
		}
		f, ok, err := videoFile(ctx, r, p, d, kind, ignored, noMarker, log)
		if ok {
			files = append(files, f)
		}
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", root.Dir(), err)
	}
	logNoMarker(ctx, log, noMarker)
	return files, subs, nil
}

// wanted reports whether a parsed video file is scanned: never a sample; in a movie library a
// movie or an extra inside a movie folder; in a series library an episode or an extra below a
// series folder. A file of a series library without an episode marker is counted per directory
// in noMarker (logged once per directory, see logNoMarker).
func wanted(kind, rel string, p ParsedName, noMarker map[string]int) bool {
	switch {
	case p.Sample:
		return false
	case kind == "series":
		if p.Series == nil && !p.Extra {
			noMarker[path.Dir(rel)]++
		}
		return p.Series != nil
	default:
		return !p.Extra || movieFolder(rel) != ""
	}
}

// logNoMarker logs the files without an episode marker of one walk once per directory, not once
// per file and scan (the 6-hour schedule would repeat every line).
func logNoMarker(ctx context.Context, log *slog.Logger, noMarker map[string]int) {
	for _, dir := range slices.Sorted(maps.Keys(noMarker)) {
		log.WarnContext(ctx, "no episode marker in the name, files skipped", "dir", dir, "files", noMarker[dir])
	}
}

// videoFile turns the non-directory entry d at p into a candidate video file. ok is false for
// entries a scan skips: no video, an ignored file, a file wanted rejects (files without an episode
// marker are counted in noMarker), a vanished entry, anything but a regular file, and a symlink
// whose target cannot be read inside the root (logged).
func videoFile(ctx context.Context, r *os.Root, p string, d fs.DirEntry, kind string, ignored map[string]bool, noMarker map[string]int, log *slog.Logger) (diskFile, bool, error) {
	if !IsVideo(d.Name()) || ignored[p] {
		return diskFile{}, false, nil
	}
	parsed := parsePath(kind, p)
	if !wanted(kind, p, parsed, noMarker) {
		return diskFile{}, false, nil
	}
	var fi fs.FileInfo
	var err error
	if d.Type()&fs.ModeSymlink != 0 {
		// Follows the link; refused when the target leaves the root.
		if fi, err = r.Stat(p); err != nil {
			log.WarnContext(ctx, "symlink not followed, skipped", "path", p, "err", err)
			return diskFile{}, false, nil
		}
	} else {
		fi, err = d.Info()
	}
	if errors.Is(err, fs.ErrNotExist) {
		return diskFile{}, false, nil // removed during the walk
	}
	if err != nil {
		return diskFile{}, false, err
	}
	if !fi.Mode().IsRegular() {
		return diskFile{}, false, nil
	}
	return diskFile{rel: p, size: fi.Size(), mtime: fi.ModTime().UnixMilli(), id: fileIdentity(fi), parsed: parsed}, true, nil
}

// planRoot matches the files on disk against the known files of the root: same rel_path, then
// (dev, inode, size), then partial_hash; the rest is new, and known files left over are missing.
// It returns the operations, the count of unchanged files and the count of newly missing ones.
//
// scope limits a path scan (nil for a full scan): rs.disk holds only the files of the scanned
// directories, so only known files inside scope can become missing. Known files outside scope
// still match a renamed or moved file, but only when they are missing already or their old path
// is gone; otherwise the new file is a copy or a hard link and becomes a file of its own, as in
// a full scan.
func (s *Service) planRoot(ctx context.Context, rs *rootScan, scope *dirScope) (ops []scanOp, unchanged, lost int, err error) {
	r, err := paths.Open(rs.proot)
	if err != nil {
		return nil, 0, 0, err
	}
	defer r.Close()

	claimed := make(map[string]bool, len(rs.known))
	byPath := make(map[string]*systemq.MediaFile, len(rs.known))
	for i := range rs.known {
		byPath[rs.known[i].RelPath] = &rs.known[i]
	}
	var unmatched []diskFile
	for _, d := range rs.disk {
		k := byPath[d.rel]
		if k == nil {
			unmatched = append(unmatched, d)
			continue
		}
		claimed[k.ID] = true
		changed := k.Size != d.size || k.Mtime != d.mtime
		sameID := toInt64(d.id.dev) == k.Dev && toInt64(d.id.inode) == k.Inode
		if !changed && sameID && !k.MissingSince.Valid {
			unchanged++
			continue
		}
		// A file back from missing_since is probed again: its last probe may have failed while
		// it was away (a share that dropped), and a stored probe_error would stay otherwise.
		op := scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: k.PartialHash.String, reprobe: changed || k.MissingSince.Valid}
		if changed {
			if op.hash, err = PartialHash(r, d.rel); err != nil {
				s.log.WarnContext(ctx, "hash changed file", "root_id", rs.root.ID, "path", d.rel, "err", err)
				op.hash = ""
			}
		}
		ops = append(ops, op)
	}

	// usable reports whether an unclaimed known file may be the origin of a file on disk.
	usable := func(k *systemq.MediaFile) bool {
		if claimed[k.ID] {
			return false
		}
		if scope.covers(k.RelPath) || k.MissingSince.Valid {
			return true
		}
		_, err := r.Lstat(k.RelPath)
		return gone(err)
	}
	type identity struct {
		dev, inode, size int64
	}
	byID := map[identity]*systemq.MediaFile{}
	byHash := map[string]*systemq.MediaFile{}
	for i := range rs.known {
		k := &rs.known[i]
		if claimed[k.ID] {
			continue
		}
		if k.Inode != 0 {
			byID[identity{k.Dev, k.Inode, k.Size}] = k
		}
		if k.PartialHash.Valid {
			if _, dup := byHash[k.PartialHash.String]; !dup {
				byHash[k.PartialHash.String] = k
			}
		}
	}
	for _, d := range unmatched {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		if d.id.inode != 0 {
			if k := byID[identity{toInt64(d.id.dev), toInt64(d.id.inode), d.size}]; k != nil && usable(k) {
				claimed[k.ID] = true
				ops = append(ops, scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: k.PartialHash.String,
					reprobe: k.Mtime != d.mtime || k.MissingSince.Valid})
				continue
			}
		}
		if d.parsed.Extra && s.deferOrphan(rs.root.ID, d) {
			rs.deferred = append(rs.deferred, d) // see reviveDeferred
			continue
		}
		h, err := PartialHash(r, d.rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the walk
		}
		if err != nil {
			s.log.WarnContext(ctx, "file not readable, skipped", "root_id", rs.root.ID, "path", d.rel, "err", err)
			continue
		}
		if k := byHash[h]; k != nil && usable(k) {
			claimed[k.ID] = true
			ops = append(ops, scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: h, reprobe: true})
			continue
		}
		ops = append(ops, scanOp{kind: opNew, rs: rs, disk: d, hash: h})
	}
	for i := range rs.known {
		k := &rs.known[i]
		if !claimed[k.ID] && !k.MissingSince.Valid && scope.covers(k.RelPath) {
			ops = append(ops, scanOp{kind: opMissing, rs: rs, known: k})
			lost++
		}
	}
	return ops, unchanged, lost, nil
}
