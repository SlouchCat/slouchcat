package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

const (
	// batchRows bounds the rows written per transaction (plan 3.6: lease ≫ transaction).
	batchRows = 500
	// maxRowsPerOp is the most rows one scan operation writes: item, media file, two jobs.
	maxRowsPerOp = 4
	// lossThresholdPercent is the share of known files that may disappear in one scan.
	lossThresholdPercent = 20
)

// Abort codes stored in libraries.last_scan_error with state aborted (plan 3.2).
const (
	abortRootMissing = "root_missing"
	abortRootEmpty   = "root_empty"
	abortLoss        = "loss_over_threshold"
)

// diskFile is a candidate video file found by the walk.
type diskFile struct {
	rel   string
	size  int64
	mtime int64 // Unix milliseconds
	id    fileID
}

// rootScan is the state of one root during a scan.
type rootScan struct {
	root  appq.LibraryRoot
	proot paths.Root
	disk  []diskFile
	known []systemq.MediaFile
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
	return s.Scan(ctx, lib)
}

// Scan runs a full scan of lib (see the package documentation). Aborts are recorded in the
// library and return nil; other failures are recorded as failed and returned.
func (s *Service) Scan(ctx context.Context, lib appq.Library) error {
	start := s.now()
	if err := s.setState(ctx, lib.ID, "running", ""); err != nil {
		return err
	}
	if lib.Kind != "movies" {
		s.log.InfoContext(ctx, "series libraries are scanned from M3 on; nothing to do", "library_id", lib.ID)
		return s.finish(ctx, lib, "done", "", 0, 0)
	}
	stats, err := s.scanMovies(ctx, lib)
	var abort scanAbort
	switch {
	case errors.As(err, &abort):
		s.log.WarnContext(ctx, "library scan aborted, nothing changed", "library_id", lib.ID, "reason", abort.code)
		return s.finish(ctx, lib, "aborted", abort.code, 0, 0)
	case err != nil && ctx.Err() != nil:
		// Interrupted (shutdown, cancel): the job runs again; the next run converges.
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.setState(bg, lib.ID, "queued", "")
		return err
	case err != nil:
		s.log.ErrorContext(ctx, "library scan failed", "library_id", lib.ID, "err", err)
		if ferr := s.finish(context.WithoutCancel(ctx), lib, "failed", err.Error(), 0, 0); ferr != nil {
			return errors.Join(err, ferr)
		}
		return err
	}
	s.log.InfoContext(ctx, "library scan done", "library_id", lib.ID, "files", stats.files, "new", stats.created,
		"updated", stats.updated, "missing", stats.missing, "items_created", stats.items, "duration", s.now().Sub(start))
	if s.imageCheck != nil {
		// Artwork is not part of the scan result: a failed check is logged, the scan stays done.
		if n, err := s.imageCheck(ctx, lib.ID); err != nil {
			s.log.WarnContext(ctx, "image check after scan failed", "library_id", lib.ID, "err", err)
		} else if n > 0 {
			s.log.InfoContext(ctx, "titles with incomplete images queued for image.fetch", "library_id", lib.ID, "titles", n)
		}
	}
	return nil
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

func (s *Service) scanMovies(ctx context.Context, lib appq.Library) (scanStats, error) {
	var stats scanStats
	roots, err := appq.New(s.db.Read).ListLibraryRoots(ctx, lib.ID)
	if err != nil {
		return stats, fmt.Errorf("list roots: %w", err)
	}
	sq := systemq.New(s.db.Read)
	scans := make([]*rootScan, 0, len(roots))
	knownActive := 0
	for _, root := range roots {
		rs := &rootScan{root: root}
		rs.proot, err = paths.NewRoot(root.Path)
		if err != nil {
			return stats, scanAbort{abortRootMissing}
		}
		if rs.known, err = sq.ListMediaFilesByRoot(ctx, root.ID); err != nil {
			return stats, fmt.Errorf("list media files of root %d: %w", root.ID, err)
		}
		if rs.disk, err = walkRoot(ctx, rs.proot); err != nil {
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
		o, u, l, err := s.planRoot(ctx, rs)
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

	idx, err := s.loadItemIndex(ctx, lib.ID)
	if err != nil {
		return stats, err
	}
	total := unchanged + len(ops)
	seen := unchanged
	for i := 0; i < len(ops); {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		end := min(len(ops), i+batchRows/maxRowsPerOp)
		n, err := s.applyBatch(ctx, lib, idx, ops[i:end], seen+end-i, total, &stats)
		if err != nil {
			return stats, err
		}
		seen += n
		i = end
	}
	return stats, s.finish(ctx, lib, "done", "", seen, total)
}

var errRootMissing = errors.New("library root missing")

// walkRoot collects the video files of a root. Extras, samples and ignored entries are skipped.
// A root that cannot be opened is errRootMissing; any other walk error fails the scan, because a
// partial walk would look like lost files.
func walkRoot(ctx context.Context, root paths.Root) ([]diskFile, error) {
	r, err := paths.Open(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRootMissing, err)
	}
	defer r.Close()
	if fi, err := r.Stat("."); err != nil || !fi.IsDir() {
		return nil, errRootMissing
	}
	var files []diskFile
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
		if d.IsDir() || !IsVideo(d.Name()) {
			return nil
		}
		parsed := ParseMoviePath(p)
		if parsed.Extra || parsed.Sample {
			return nil
		}
		var fi fs.FileInfo
		if d.Type()&fs.ModeSymlink != 0 {
			fi, err = r.Stat(p) // follows the link, refused when it leaves the root
		} else {
			fi, err = d.Info()
		}
		if errors.Is(err, fs.ErrNotExist) {
			return nil // removed during the walk
		}
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		files = append(files, diskFile{rel: p, size: fi.Size(), mtime: fi.ModTime().UnixMilli(), id: fileIdentity(fi)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root.Dir(), err)
	}
	return files, nil
}

// planRoot matches the files on disk against the known files of the root: same rel_path, then
// (dev, inode, size), then partial_hash; the rest is new, and known files left over are missing.
// It returns the operations, the count of unchanged files and the count of newly missing ones.
func (s *Service) planRoot(ctx context.Context, rs *rootScan) (ops []scanOp, unchanged, lost int, err error) {
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
		op := scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: k.PartialHash.String, reprobe: changed}
		if changed {
			if op.hash, err = PartialHash(r, d.rel); err != nil {
				s.log.WarnContext(ctx, "hash changed file", "root_id", rs.root.ID, "path", d.rel, "err", err)
				op.hash = ""
			}
		}
		ops = append(ops, op)
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
			if k := byID[identity{toInt64(d.id.dev), toInt64(d.id.inode), d.size}]; k != nil && !claimed[k.ID] {
				claimed[k.ID] = true
				ops = append(ops, scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: k.PartialHash.String, reprobe: k.Mtime != d.mtime})
				continue
			}
		}
		h, err := PartialHash(r, d.rel)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed since the walk
		}
		if err != nil {
			s.log.WarnContext(ctx, "file not readable, skipped", "root_id", rs.root.ID, "path", d.rel, "err", err)
			continue
		}
		if k := byHash[h]; k != nil && !claimed[k.ID] {
			claimed[k.ID] = true
			ops = append(ops, scanOp{kind: opUpdate, rs: rs, disk: d, known: k, hash: h, reprobe: true})
			continue
		}
		ops = append(ops, scanOp{kind: opNew, rs: rs, disk: d, hash: h})
	}
	for i := range rs.known {
		k := &rs.known[i]
		if !claimed[k.ID] && !k.MissingSince.Valid {
			ops = append(ops, scanOp{kind: opMissing, rs: rs, known: k})
			lost++
		}
	}
	return ops, unchanged, lost, nil
}

// itemIndex finds the undeleted items of a library by provider id or by (sort_title, year).
type itemIndex struct {
	byTMDB  map[int64]string
	byIMDB  map[string]string
	byTitle map[string][]itemRef
}

type itemRef struct {
	id   string
	tmdb int64
	imdb string
}

func titleKey(sortTitle string, year sql.NullInt64) string {
	if !year.Valid {
		return sortTitle + "|"
	}
	return sortTitle + "|" + strconv.FormatInt(year.Int64, 10)
}

func (s *Service) loadItemIndex(ctx context.Context, libraryID int64) (*itemIndex, error) {
	items, err := systemq.New(s.db.Read).ListItemsByLibrary(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	idx := &itemIndex{byTMDB: map[int64]string{}, byIMDB: map[string]string{}, byTitle: map[string][]itemRef{}}
	for _, it := range items {
		if it.Kind == "movie" {
			idx.add(it.ID, it.SortTitle, it.Year, it.TmdbID.Int64, it.ImdbID.String)
		}
	}
	return idx, nil
}

func (idx *itemIndex) add(id, sortTitle string, year sql.NullInt64, tmdb int64, imdb string) {
	if tmdb != 0 {
		if _, ok := idx.byTMDB[tmdb]; !ok {
			idx.byTMDB[tmdb] = id
		}
	}
	if imdb != "" {
		if _, ok := idx.byIMDB[imdb]; !ok {
			idx.byIMDB[imdb] = id
		}
	}
	key := titleKey(sortTitle, year)
	idx.byTitle[key] = append(idx.byTitle[key], itemRef{id: id, tmdb: tmdb, imdb: imdb})
}

// find returns the item of a title: by TMDB id, else IMDb id, else (sort_title, year) when the
// provider ids do not contradict.
func (idx *itemIndex) find(m movieInfo) string {
	if m.tmdb != 0 {
		if id, ok := idx.byTMDB[m.tmdb]; ok {
			return id
		}
	}
	if m.imdb != "" {
		if id, ok := idx.byIMDB[m.imdb]; ok {
			return id
		}
	}
	for _, ref := range idx.byTitle[titleKey(m.sortTitle, m.year)] {
		if (m.tmdb == 0 || ref.tmdb == 0 || m.tmdb == ref.tmdb) && (m.imdb == "" || ref.imdb == "" || m.imdb == ref.imdb) {
			return ref.id
		}
	}
	return ""
}

// movieInfo is a new file's title data from its name and NFO.
type movieInfo struct {
	title, sortTitle, originalTitle string
	year                            sql.NullInt64
	tmdb                            int64
	imdb                            string
	rating                          *int
	versionLabel                    string
}

// describe parses the file's name and its NFO (movie.nfo next to it, or <stem>.nfo). Ids in the
// name win over the NFO.
func describe(r *os.Root, rel string) movieInfo {
	p := ParseMoviePath(rel)
	nfo := readNFO(r, rel)
	m := movieInfo{title: p.Title, tmdb: p.TMDBID, imdb: p.IMDBID, versionLabel: p.VersionLabel, originalTitle: nfo.OriginalTitle}
	if m.title == "" {
		m.title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	}
	m.sortTitle = SortTitle(m.title)
	switch {
	case p.Year != nil:
		m.year = sql.NullInt64{Int64: int64(*p.Year), Valid: true}
	case nfo.Year != 0:
		m.year = sql.NullInt64{Int64: int64(nfo.Year), Valid: true}
	}
	if m.tmdb == 0 {
		m.tmdb = nfo.TMDBID
	}
	if m.imdb == "" {
		m.imdb = nfo.IMDBID
	}
	m.rating = nfo.GermanRating()
	return m
}

func readNFO(r *os.Root, rel string) NFO {
	dir := path.Dir(rel)
	stem := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	for _, name := range []string{path.Join(dir, stem+".nfo"), path.Join(dir, "movie.nfo")} {
		fi, err := r.Stat(name)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxNFOSize {
			continue
		}
		data, err := r.ReadFile(name)
		if err != nil {
			continue
		}
		if n := ParseNFO(data); !n.empty() {
			return n
		}
	}
	return NFO{}
}

func (n NFO) empty() bool {
	return n.Title == "" && n.OriginalTitle == "" && n.Year == 0 && n.TMDBID == 0 && n.IMDBID == "" &&
		n.MPAA == "" && n.Certification == ""
}

// applyBatch writes one batch of operations in one transaction, together with the follow-up jobs
// and the scan progress. It returns the number of operations applied.
func (s *Service) applyBatch(ctx context.Context, lib appq.Library, idx *itemIndex, ops []scanOp, seen, total int, stats *scanStats) (int, error) {
	// File reads (NFO) happen before the transaction starts.
	infos := make([]movieInfo, len(ops))
	roots := map[int64]*os.Root{}
	defer func() {
		for _, r := range roots {
			_ = r.Close()
		}
	}()
	for i, op := range ops {
		switch op.kind {
		case opNew:
			r, ok := roots[op.rs.root.ID]
			if !ok {
				var err error
				if r, err = paths.Open(op.rs.proot); err != nil {
					return 0, err
				}
				roots[op.rs.root.ID] = r
			}
			infos[i] = describe(r, op.disk.rel)
		case opUpdate:
			infos[i].versionLabel = ParseMoviePath(op.disk.rel).VersionLabel
		case opMissing:
		}
	}

	now := s.now()
	ms := now.UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	var local scanStats
	for i, op := range ops {
		switch op.kind {
		case opNew:
			m := infos[i]
			itemID := idx.find(m)
			if itemID == "" {
				itemID = s.newID()
				params := systemq.InsertItemParams{
					ID: itemID, Kind: "movie", LibraryID: lib.ID, Title: m.title, SortTitle: m.sortTitle,
					OriginalTitle: sql.NullString{String: m.originalTitle, Valid: m.originalTitle != ""},
					Year:          m.year,
					TmdbID:        sql.NullInt64{Int64: m.tmdb, Valid: m.tmdb != 0},
					ImdbID:        sql.NullString{String: m.imdb, Valid: m.imdb != ""},
					Now:           ms,
				}
				if m.rating != nil {
					params.OwnRating = sql.NullInt64{Int64: int64(*m.rating), Valid: true}
					params.RatingSource = sql.NullString{String: "nfo", Valid: true}
				}
				if _, err := q.InsertItem(ctx, params); err != nil {
					return 0, fmt.Errorf("insert item: %w", err)
				}
				if err := enqueueTx(ctx, q, now, matchJob(itemID)); err != nil {
					return 0, err
				}
				// Later files of the same title in this scan attach to this item. A failed
				// transaction fails the whole scan, so the index never outlives a rollback.
				idx.add(itemID, m.sortTitle, m.year, m.tmdb, m.imdb)
				local.items++
			}
			fileID := s.newID()
			if _, err := q.InsertMediaFile(ctx, systemq.InsertMediaFileParams{
				ID: fileID, ItemID: itemID, RootID: op.rs.root.ID, RelPath: op.disk.rel,
				Dev: toInt64(op.disk.id.dev), Inode: toInt64(op.disk.id.inode), Size: op.disk.size, Mtime: op.disk.mtime,
				PartialHash:  sql.NullString{String: op.hash, Valid: op.hash != ""},
				VersionLabel: sql.NullString{String: m.versionLabel, Valid: m.versionLabel != ""},
				Container:    extOf(op.disk.rel), Now: ms,
			}); err != nil {
				return 0, fmt.Errorf("insert media file: %w", err)
			}
			if err := enqueueTx(ctx, q, now, probeJob(fileID)); err != nil {
				return 0, err
			}
			local.created++
		case opUpdate:
			label := infos[i].versionLabel
			if err := q.UpdateMediaFileLocation(ctx, systemq.UpdateMediaFileLocationParams{
				RelPath: op.disk.rel, Dev: toInt64(op.disk.id.dev), Inode: toInt64(op.disk.id.inode),
				Size: op.disk.size, Mtime: op.disk.mtime,
				PartialHash:  sql.NullString{String: op.hash, Valid: op.hash != ""},
				VersionLabel: sql.NullString{String: label, Valid: label != ""},
				Now:          ms, ID: op.known.ID,
			}); err != nil {
				return 0, fmt.Errorf("update media file: %w", err)
			}
			if op.known.MissingSince.Valid {
				if err := q.RestoreItem(ctx, systemq.RestoreItemParams{Now: ms, ID: op.known.ItemID}); err != nil {
					return 0, fmt.Errorf("restore item: %w", err)
				}
			}
			if op.reprobe {
				if err := enqueueTx(ctx, q, now, probeJob(op.known.ID)); err != nil {
					return 0, err
				}
			}
			local.updated++
		case opMissing:
			if err := q.SetMediaFileMissing(ctx, systemq.SetMediaFileMissingParams{Now: ms, ID: op.known.ID}); err != nil {
				return 0, fmt.Errorf("mark media file missing: %w", err)
			}
			local.missing++
		}
	}
	if err := appq.New(tx).SetLibraryScanProgress(ctx, appq.SetLibraryScanProgressParams{Seen: int64(seen), Total: int64(total), Now: ms, ID: lib.ID}); err != nil {
		return 0, fmt.Errorf("set scan progress: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit scan batch: %w", err)
	}
	stats.created += local.created
	stats.updated += local.updated
	stats.missing += local.missing
	stats.items += local.items
	return len(ops), nil
}
