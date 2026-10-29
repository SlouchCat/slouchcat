package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// dirScope is the part of a root that a path scan covers: the files directly in the listed
// directories and everything below the directories that are gone. nil covers the whole root.
type dirScope struct {
	listed map[string]bool
	gone   []string
}

func (sc *dirScope) covers(rel string) bool {
	if sc == nil || sc.listed[path.Dir(rel)] {
		return true
	}
	for _, g := range sc.gone {
		if strings.HasPrefix(rel, g+"/") {
			return true
		}
	}
	return false
}

// gone reports whether err from a lookup means that the path does not exist (any more), also
// when a path component is now a file.
func gone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// cleanDirs normalizes the directories of a path scan payload: slash-separated, relative and
// inside the root, without ignored components (a full scan never enters those), sorted, unique.
func cleanDirs(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		d = path.Clean(d)
		if path.IsAbs(d) || d == ".." || strings.HasPrefix(d, "../") {
			continue
		}
		if d != "." && slices.ContainsFunc(strings.Split(d, "/"), Ignored) {
			continue
		}
		out = append(out, d)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (s *Service) handlePathScan(ctx context.Context, j *jobs.Job) error {
	var p pathScanPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	return s.runCancellable(ctx, j, p.LibraryID, func(ctx context.Context) error { return s.scanPaths(ctx, p.LibraryID, p.Roots) })
}

// ScanPaths runs a path scan of the given directories of a library (see the package
// documentation). Like Scan, aborts and cancellations are recorded in the library and return nil.
func (s *Service) ScanPaths(ctx context.Context, libraryID int64, roots []pathScanRoot) error {
	return s.runCancellable(ctx, nil, libraryID, func(ctx context.Context) error { return s.scanPaths(ctx, libraryID, roots) })
}

func (s *Service) scanPaths(ctx context.Context, libraryID int64, roots []pathScanRoot) error {
	unlock, err := s.lockScan(ctx, libraryID)
	if err != nil {
		return err
	}
	defer unlock()
	lib, err := appq.New(s.db.Read).GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		s.log.InfoContext(ctx, "path scan of a deleted library skipped", "library_id", libraryID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load library %d: %w", libraryID, err)
	}
	start := s.now()
	if err := s.setState(ctx, lib.ID, "running", ""); err != nil {
		return err
	}
	stats, dirs, usedLoss, err := s.scanLibraryPaths(ctx, lib, roots)
	if done, err := s.endScan(ctx, lib, "path scan", err); done {
		return err
	}
	if usedLoss {
		if err := appq.New(s.db.Write).ClearLibraryLossAccepted(ctx, appq.ClearLibraryLossAcceptedParams{
			Now: s.now().UnixMilli(), ID: lib.ID, AcceptedAt: lib.LossAcceptedAt,
		}); err != nil {
			return fmt.Errorf("clear accepted loss: %w", err)
		}
	}
	if err := s.setState(ctx, lib.ID, "done", ""); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "path scan done", "library_id", lib.ID, "dirs", dirs, "files", stats.files, "new", stats.created,
		"updated", stats.updated, "missing", stats.missing, "items_created", stats.items, "duration", s.now().Sub(start))
	return nil
}

// scanLibraryPaths plans and applies a path scan. It returns the statistics, the number of
// directories read and whether it applied a loss that only loss_accepted_at allowed.
func (s *Service) scanLibraryPaths(ctx context.Context, lib appq.Library, req []pathScanRoot) (scanStats, int, bool, error) {
	var stats scanStats
	want := map[int64][]string{}
	for _, r := range req {
		want[r.RootID] = append(want[r.RootID], r.Dirs...)
	}
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, lib.ID)
	if err != nil {
		return stats, 0, false, fmt.Errorf("list roots: %w", err)
	}
	since := s.now().Add(-missingGrace).UnixMilli()
	// All roots of the library: the item index needs their folders and the loss rule their counts.
	scans := make([]*rootScan, 0, len(roots))
	scopes := map[*rootScan]*dirScope{}
	active, recentMissing, dirCount := 0, 0, 0
	for _, root := range roots {
		rs, err := s.openRoot(ctx, root)
		if err != nil {
			return stats, 0, false, err
		}
		rootActive := 0
		for _, k := range rs.known {
			switch {
			case !k.MissingSince.Valid:
				rootActive++
			case k.MissingSince.Int64 >= since:
				recentMissing++
			}
		}
		active += rootActive
		scans = append(scans, rs)
		dirs := cleanDirs(want[root.ID])
		if len(dirs) == 0 {
			continue
		}
		scope, err := s.readDirs(ctx, lib, rs, rootActive, dirs)
		if err != nil {
			return stats, 0, false, err
		}
		scopes[rs] = scope
		dirCount += len(dirs)
	}

	var ops []scanOp
	lost := 0
	for _, rs := range scans {
		scope := scopes[rs]
		if scope == nil {
			continue
		}
		o, _, l, err := s.planRoot(ctx, rs, scope)
		if err != nil {
			return stats, 0, false, err
		}
		ops = append(ops, o...)
		lost += l
		stats.files += len(rs.disk)
	}
	usedLoss := false
	if lossTooHigh(active, recentMissing, lost) {
		if !lib.LossAcceptedAt.Valid {
			s.log.WarnContext(ctx, "too many files missing", "library_id", lib.ID, "missing", lost,
				"missing_last_24h", recentMissing, "known", active)
			return stats, 0, false, scanAbort{abortLoss}
		}
		usedLoss = true
	}
	if ops, _, err = s.reviveDeferred(ctx, scans, ops); err != nil {
		return stats, 0, false, err
	}

	if len(ops) > 0 {
		idx, err := s.loadItemIndex(ctx, lib, scans)
		if err != nil {
			return stats, 0, false, err
		}
		if _, err := s.applyOps(ctx, lib, idx, ops, 0, 0, false, &stats); err != nil {
			return stats, 0, false, err
		}
	}
	for _, rs := range scans {
		if scope := scopes[rs]; scope != nil {
			if err := s.syncSidecars(ctx, rs, scope); err != nil {
				return stats, 0, false, err
			}
		}
	}
	return stats, dirCount, usedLoss, nil
}

// lossTooHigh applies the 20 % rule to a path scan that makes lost more files missing. Path
// scans are small, so the rule counts the library as a whole over the grace period of 24 h:
// recentMissing files went missing in that period, active files are present. The scan stops
// when it pushes the missing share of the library above 20 %, or when its own loss alone exceeds
// 20 % (as in a full scan), so neither one large change nor a series of small ones removes more
// than a fifth of the library without the administrator's consent.
func lossTooHigh(active, recentMissing, lost int) bool {
	base := active + recentMissing
	if lost == 0 || base == 0 {
		return false
	}
	after := (recentMissing+lost)*100 > base*lossThresholdPercent
	crossed := recentMissing*100 <= base*lossThresholdPercent
	return after && (crossed || lost*100 > active*lossThresholdPercent)
}

// readDirs checks the root of rs like a full scan does and lists dirs into rs.disk. It returns
// the scope of the scan; a directory that does not exist any more goes into scope.gone.
func (s *Service) readDirs(ctx context.Context, lib appq.Library, rs *rootScan, active int, dirs []string) (*dirScope, error) {
	var err error
	if rs.proot, err = paths.NewRoot(rs.root.Path); err != nil {
		return nil, scanAbort{abortRootMissing}
	}
	r, err := paths.Open(rs.proot)
	if err != nil {
		return nil, scanAbort{abortRootMissing}
	}
	defer r.Close()
	top, err := r.Open(".")
	if err != nil {
		return nil, scanAbort{abortRootMissing}
	}
	if fi, err := top.Stat(); err != nil || !fi.IsDir() {
		_ = top.Close()
		return nil, scanAbort{abortRootMissing}
	}
	// An empty root while files are known is an unmounted mount point (root_empty, as in a full
	// scan); one entry is enough to tell.
	_, err = top.ReadDir(1)
	_ = top.Close()
	if errors.Is(err, io.EOF) && active > 0 && !lib.LossAcceptedAt.Valid {
		return nil, scanAbort{abortRootEmpty}
	}

	scope := &dirScope{listed: map[string]bool{}}
	rs.subs = map[string][]string{}
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, subs, isGone, err := listDirFiles(ctx, r, dir, rs.root.Content, rs.ignored, s.log)
		if err != nil {
			return nil, fmt.Errorf("list %s in root %d: %w", dir, rs.root.ID, err)
		}
		if isGone {
			scope.gone = append(scope.gone, dir)
			continue
		}
		scope.listed[dir] = true
		rs.disk = append(rs.disk, files...)
		rs.subs[dir] = subs
	}
	return scope, nil
}

// listDirFiles lists the video files directly in dir that a full scan would pick up (videoFile)
// and the names of its subtitle files (sidecars, I-5); subdirectories are not entered (a path scan
// names every directory it reads). isGone reports a directory that does not exist any more. A
// symlinked directory is not read, like in the walk of a full scan.
func listDirFiles(ctx context.Context, r *os.Root, dir, content string, ignored map[string]bool, log *slog.Logger) (files []diskFile, subs []string, isGone bool, err error) {
	fi, err := r.Lstat(dir)
	switch {
	case gone(err):
		return nil, nil, true, nil
	case err != nil:
		return nil, nil, false, err
	case fi.Mode()&fs.ModeSymlink != 0:
		return nil, nil, false, nil
	case !fi.IsDir():
		return nil, nil, true, nil
	}
	f, err := r.Open(dir)
	if gone(err) {
		return nil, nil, true, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil, nil, false, err
	}
	noMarker := map[string]int{}
	for _, e := range entries {
		if e.IsDir() || Ignored(e.Name()) {
			continue
		}
		if isSubtitle(e.Name()) {
			subs = append(subs, e.Name())
			continue
		}
		df, ok, err := videoFile(ctx, r, path.Join(dir, e.Name()), e, content, ignored, noMarker, log)
		if err != nil {
			return nil, nil, false, err
		}
		if ok {
			files = append(files, df)
		}
	}
	logNoMarker(ctx, log, noMarker)
	return files, subs, false, nil
}
