package library

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// titleInfo is the data of a movie or a series from the names and NFO files of a new file.
type titleInfo struct {
	title, sortTitle, originalTitle string
	year                            sql.NullInt64
	tmdb, tvdb                      int64
	imdb                            string
	rating                          *int
}

// fileInfo is what a new file contributes: its title (movie, or the series of an episode or a
// series extra), the folder key of that title, and the episode or extra data.
type fileInfo struct {
	titleInfo
	folder       string
	versionLabel string

	extra                  bool
	extraKind, extraTitle  string
	seasonDir              bool // series extra inside a season folder: its parent is the season
	season                 int64
	episode, episodeEnd    int64
	episodeTitle, released string
	episodeRating          *int
}

// nfoCache holds the tvshow.nfo files read in one batch, by root and series folder.
type nfoCache map[string]NFO

// describe reads the title data of a new file from its parsed name and its NFO files: movie.nfo
// or <stem>.nfo next to a movie; tvshow.nfo in the series folder and <stem>.nfo
// (episodedetails) next to an episode. Ids in the names win over the NFO.
func describe(r *os.Root, rootID int64, kind string, d diskFile, cache nfoCache) fileInfo {
	p := d.parsed
	var f fileInfo
	f.versionLabel = p.VersionLabel
	f.extra, f.extraKind, f.extraTitle = p.Extra, p.ExtraKind, p.ExtraTitle
	if kind != "series" {
		f.folder = titleFolderKey(rootID, movieFolder(d.rel))
		if p.Extra {
			return f
		}
		nfo := readNFO(r, d.rel, NFOMovie)
		f.titleInfo = titleInfo{title: p.Title, tmdb: p.TMDBID, imdb: p.IMDBID, originalTitle: nfo.OriginalTitle}
		if f.title == "" {
			f.title = strings.TrimSuffix(path.Base(d.rel), path.Ext(d.rel))
		}
		f.year = yearValue(p.Year, nfo.Year)
		f.fillIDs(nfo)
		f.rating = nfo.GermanRating()
		f.sortTitle = SortTitle(f.title)
		return f
	}

	ep := p.Series
	f.folder = seriesFolderKey(rootID, ep)
	f.seasonDir = ep.SeasonDir != ""
	f.season = int64(ep.Season)
	if p.Extra {
		return f
	}
	show, ok := cache[f.folder]
	if !ok && ep.SeriesDir != "" {
		show = readNFOFile(r, path.Join(ep.SeriesDir, "tvshow.nfo"), NFOTVShow)
		cache[f.folder] = show
	}
	f.titleInfo = titleInfo{title: ep.SeriesTitle, tmdb: ep.TMDBID, tvdb: ep.TVDBID, imdb: ep.IMDBID, originalTitle: show.OriginalTitle}
	f.year = yearValue(ep.SeriesYear, show.Year)
	f.fillIDs(show)
	f.rating = show.GermanRating()
	f.sortTitle = SortTitle(f.title)

	stem := strings.TrimSuffix(d.rel, path.Ext(d.rel))
	epNFO := readNFOFile(r, stem+".nfo", NFOEpisode)
	f.episode, f.episodeEnd = int64(ep.Episode), int64(ep.EpisodeEnd)
	f.episodeTitle = epNFO.Title
	if f.episodeTitle == "" {
		f.episodeTitle = ep.Title
	}
	if f.episodeTitle == "" {
		f.episodeTitle = "Episode " + strconv.Itoa(ep.Episode)
	}
	f.released = ep.Date
	if f.released == "" {
		f.released = epNFO.Aired
	}
	f.episodeRating = epNFO.GermanRating()
	return f
}

func yearValue(name *int, nfo int) sql.NullInt64 {
	switch {
	case name != nil:
		return sql.NullInt64{Int64: int64(*name), Valid: true}
	case nfo != 0:
		return sql.NullInt64{Int64: int64(nfo), Valid: true}
	}
	return sql.NullInt64{}
}

func (f *fileInfo) fillIDs(n NFO) {
	if f.tmdb == 0 {
		f.tmdb = n.TMDBID
	}
	if f.tvdb == 0 {
		f.tvdb = n.TVDBID
	}
	if f.imdb == "" {
		f.imdb = n.IMDBID
	}
}

// readNFO reads the NFO of a movie file: <stem>.nfo, else movie.nfo in its folder.
func readNFO(r *os.Root, rel, kind string) NFO {
	dir := path.Dir(rel)
	stem := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	for _, name := range []string{path.Join(dir, stem+".nfo"), path.Join(dir, "movie.nfo")} {
		if n := readNFOFile(r, name, kind); !n.empty() {
			return n
		}
	}
	return NFO{}
}

// readNFOFile reads one NFO file of the given kind; anything else (missing, too large, another
// root element) is an empty NFO.
func readNFOFile(r *os.Root, name, kind string) NFO {
	fi, err := r.Stat(name)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxNFOSize {
		return NFO{}
	}
	data, err := r.ReadFile(name)
	if err != nil {
		return NFO{}
	}
	n := ParseNFO(data)
	if n.Kind != kind {
		return NFO{}
	}
	return n
}

func (n NFO) empty() bool {
	return n.Title == "" && n.OriginalTitle == "" && n.Year == 0 && n.TMDBID == 0 && n.TVDBID == 0 && n.IMDBID == "" &&
		n.MPAA == "" && n.Certification == ""
}

// scanProgress is the progress a full scan records with each batch.
type scanProgress struct{ seen, total int }

// WriteStats describes the write transactions of the scans since the last TakeWriteStats call
// (scan benchmark, plan M3 step 5).
type WriteStats struct {
	Batches int           // transactions committed
	Ops     int           // operations written: new, changed, moved or missing files
	Longest time.Duration // longest transaction, from begin to commit
}

// TakeWriteStats returns the write statistics collected since the last call and resets them.
func (s *Service) TakeWriteStats() WriteStats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	st := s.writeStats
	s.writeStats = WriteStats{}
	return st
}

func (s *Service) recordBatch(ops int, d time.Duration) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.writeStats.Batches++
	s.writeStats.Ops += ops
	s.writeStats.Longest = max(s.writeStats.Longest, d)
}

// applyBatch writes one batch of operations in one transaction, together with the follow-up jobs
// and the scan progress (prog; nil for a path scan, which leaves the counts of the last full scan).
// It returns the number of operations applied.
func (s *Service) applyBatch(ctx context.Context, lib appq.Library, idx *itemIndex, ops []scanOp, prog *scanProgress, stats *scanStats) (int, error) {
	// File reads (NFO) happen before the transaction starts.
	infos := make([]fileInfo, len(ops))
	roots := map[int64]*os.Root{}
	defer func() {
		for _, r := range roots {
			_ = r.Close()
		}
	}()
	cache := nfoCache{}
	for i, op := range ops {
		if op.kind != opNew {
			continue
		}
		r, ok := roots[op.rs.root.ID]
		if !ok {
			var err error
			if r, err = paths.Open(op.rs.proot); err != nil {
				return 0, err
			}
			roots[op.rs.root.ID] = r
		}
		infos[i] = describe(r, op.rs.root.ID, lib.Kind, op.disk, cache)
	}

	now := s.now()
	began := time.Now()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	w := &batchWriter{q: systemq.New(tx), lib: lib, idx: idx, newID: s.newID, now: now, ms: now.UnixMilli()}
	for i, op := range ops {
		if err := w.apply(ctx, op, infos[i]); err != nil {
			return 0, err
		}
	}
	if prog != nil {
		if err := appq.New(tx).SetLibraryScanProgress(ctx, appq.SetLibraryScanProgressParams{Seen: int64(prog.seen), Total: int64(prog.total), Now: w.ms, ID: lib.ID}); err != nil {
			return 0, fmt.Errorf("set scan progress: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit scan batch: %w", err)
	}
	s.recordBatch(len(ops), time.Since(began))
	if w.skipped > 0 {
		s.log.InfoContext(ctx, "extras without a parent title skipped", "library_id", lib.ID, "files", w.skipped)
	}
	stats.created += w.stats.created
	stats.updated += w.stats.updated
	stats.missing += w.stats.missing
	stats.items += w.stats.items
	return len(ops), nil
}

// batchWriter writes the operations of one batch in its transaction. The item index learns every
// item it creates, so later files of the same title attach to it; a failed transaction fails the
// whole scan, so the index never outlives a rollback.
type batchWriter struct {
	q       *systemq.Queries
	lib     appq.Library
	idx     *itemIndex
	newID   func() string
	now     time.Time
	ms      int64
	stats   scanStats
	skipped int
}

func (w *batchWriter) apply(ctx context.Context, op scanOp, info fileInfo) error {
	q := w.q
	switch op.kind {
	case opNew:
		itemID, err := w.item(ctx, info)
		if err != nil || itemID == "" {
			return err
		}
		fileID := w.newID()
		if _, err := q.InsertMediaFile(ctx, systemq.InsertMediaFileParams{
			ID: fileID, ItemID: itemID, RootID: op.rs.root.ID, RelPath: op.disk.rel,
			Dev: toInt64(op.disk.id.dev), Inode: toInt64(op.disk.id.inode), Size: op.disk.size, Mtime: op.disk.mtime,
			PartialHash:  nullString(op.hash),
			VersionLabel: nullString(info.versionLabel),
			Container:    extOf(op.disk.rel), Now: w.ms,
		}); err != nil {
			return fmt.Errorf("insert media file: %w", err)
		}
		if err := enqueueTx(ctx, q, w.now, probeJob(fileID)); err != nil {
			return err
		}
		w.stats.created++
	case opUpdate:
		if err := q.UpdateMediaFileLocation(ctx, systemq.UpdateMediaFileLocationParams{
			RelPath: op.disk.rel, Dev: toInt64(op.disk.id.dev), Inode: toInt64(op.disk.id.inode),
			Size: op.disk.size, Mtime: op.disk.mtime,
			PartialHash:  nullString(op.hash),
			VersionLabel: nullString(op.disk.parsed.VersionLabel),
			Now:          w.ms, ID: op.known.ID,
		}); err != nil {
			return fmt.Errorf("update media file: %w", err)
		}
		titleKind := "movie"
		if w.lib.Kind == "series" {
			titleKind = "series"
		}
		if title := w.idx.title(op.known.ItemID, titleKind); title != "" {
			w.idx.addFolder(title, fileFolderKey(w.lib.Kind, op.rs.root.ID, op.disk.rel, op.disk.parsed))
		}
		if op.known.MissingSince.Valid {
			if err := w.restore(ctx, op.known.ItemID, true); err != nil {
				return err
			}
		}
		if op.reprobe {
			if err := enqueueTx(ctx, q, w.now, probeJob(op.known.ID)); err != nil {
				return err
			}
		}
		w.stats.updated++
	case opMissing:
		if err := q.SetMediaFileMissing(ctx, systemq.SetMediaFileMissingParams{Now: w.ms, ID: op.known.ID}); err != nil {
			return fmt.Errorf("mark media file missing: %w", err)
		}
		w.stats.missing++
	}
	return nil
}

// item returns the item a new file belongs to, creating it (and its series and season) when
// needed; "" for an extra without a parent title, which is skipped.
func (w *batchWriter) item(ctx context.Context, info fileInfo) (string, error) {
	switch {
	case info.extra:
		return w.extraItem(ctx, info)
	case w.lib.Kind == "series":
		return w.episodeItem(ctx, info)
	default:
		return w.titleItem(ctx, info, "movie")
	}
}

// restore brings back a soft-deleted item with its soft-deleted ancestors. force restores even
// when the index does not know the item as deleted (a missing file reappeared).
func (w *batchWriter) restore(ctx context.Context, id string, force bool) error {
	if !force && !w.idx.deleted[id] {
		return nil
	}
	if err := w.q.RestoreItem(ctx, systemq.RestoreItemParams{Now: w.ms, ID: id}); err != nil {
		return fmt.Errorf("restore item: %w", err)
	}
	w.idx.undelete(id)
	return nil
}

func (w *batchWriter) insert(ctx context.Context, p systemq.InsertItemParams) error {
	p.LibraryID, p.Now = w.lib.ID, w.ms
	if _, err := w.q.InsertItem(ctx, p); err != nil {
		return fmt.Errorf("insert %s item: %w", p.Kind, err)
	}
	w.idx.kinds[p.ID] = p.Kind
	if p.ParentID.Valid {
		w.idx.parent[p.ID] = p.ParentID.String
	}
	w.stats.items++
	return nil
}

// titleItem finds or creates the movie or series of a file. A new title queues metadata.match.
func (w *batchWriter) titleItem(ctx context.Context, info fileInfo, kind string) (string, error) {
	id := w.idx.find(info.titleInfo, info.folder)
	if id != "" {
		// Re-downloaded within relinkWindow: the title returns with its user data.
		if err := w.restore(ctx, id, false); err != nil {
			return "", err
		}
	} else {
		id = w.newID()
		p := systemq.InsertItemParams{
			ID: id, Kind: kind, Title: info.title, SortTitle: info.sortTitle,
			OriginalTitle: nullString(info.originalTitle), Year: info.year,
			TmdbID: nullInt(info.tmdb), TvdbID: nullInt(info.tvdb), ImdbID: nullString(info.imdb),
		}
		if info.rating != nil {
			p.OwnRating = sql.NullInt64{Int64: int64(*info.rating), Valid: true}
			p.RatingSource = sql.NullString{String: "nfo", Valid: true}
		}
		if err := w.insert(ctx, p); err != nil {
			return "", err
		}
		if err := enqueueTx(ctx, w.q, w.now, matchJob(id)); err != nil {
			return "", err
		}
		w.idx.add(id, info.sortTitle, info.year, info.tmdb, info.tvdb, info.imdb)
	}
	w.idx.addFolder(id, info.folder)
	return id, nil
}

// seasonTitle is items.title of a season; the metadata job writes the texts.
func seasonTitle(n int64) (title, sortTitle string) {
	if n == 0 {
		return "Specials", "specials"
	}
	return "Season " + strconv.FormatInt(n, 10), fmt.Sprintf("season %04d", n)
}

// episodeItem finds or creates the series, season and episode of a new episode file. A second
// file of the same episode attaches to it as a version. A new episode of a series that is
// already matched queues its own metadata.match; otherwise the series job fills it later.
func (w *batchWriter) episodeItem(ctx context.Context, info fileInfo) (string, error) {
	seriesID, err := w.titleItem(ctx, info, "series")
	if err != nil {
		return "", err
	}
	seasonKey := seriesKey(seriesID, info.season)
	seasonID := w.idx.seasons[seasonKey]
	if seasonID != "" {
		if err := w.restore(ctx, seasonID, false); err != nil {
			return "", err
		}
	} else {
		seasonID = w.newID()
		title, sortTitle := seasonTitle(info.season)
		if err := w.insert(ctx, systemq.InsertItemParams{
			ID: seasonID, Kind: "season", ParentID: nullString(seriesID), Title: title, SortTitle: sortTitle,
			SeasonNumber: sql.NullInt64{Int64: info.season, Valid: true},
		}); err != nil {
			return "", err
		}
		w.idx.seasons[seasonKey] = seasonID
	}

	episodeKey := seriesKey(seasonID, info.episode)
	if id := w.idx.episodes[episodeKey]; id != "" {
		return id, w.restore(ctx, id, false)
	}
	id := w.newID()
	p := systemq.InsertItemParams{
		ID: id, Kind: "episode", ParentID: nullString(seasonID), Title: info.episodeTitle,
		SortTitle:     fmt.Sprintf("%04d %04d", info.season, info.episode),
		ReleaseDate:   nullString(info.released),
		SeasonNumber:  sql.NullInt64{Int64: info.season, Valid: true},
		EpisodeNumber: sql.NullInt64{Int64: info.episode, Valid: true},
	}
	if info.episodeEnd > info.episode {
		p.EpisodeEnd = sql.NullInt64{Int64: info.episodeEnd, Valid: true}
	}
	if info.episodeRating != nil {
		p.OwnRating = sql.NullInt64{Int64: int64(*info.episodeRating), Valid: true}
		p.RatingSource = sql.NullString{String: "nfo", Valid: true}
	}
	if err := w.insert(ctx, p); err != nil {
		return "", err
	}
	w.idx.episodes[episodeKey] = id
	if w.idx.matched[seriesID] {
		if err := enqueueTx(ctx, w.q, w.now, episodeMatchJob(id)); err != nil {
			return "", err
		}
	}
	return id, nil
}

// extraItem creates the extra item of a new extras file below its movie, series or season (the
// season of a season folder when the scan knows it, else the series). Extras get no metadata job.
func (w *batchWriter) extraItem(ctx context.Context, info fileInfo) (string, error) {
	parent := w.idx.folderTitle(info.folder)
	if parent == "" {
		w.skipped++
		return "", nil
	}
	if info.seasonDir {
		if season := w.idx.seasons[seriesKey(parent, info.season)]; season != "" && !w.idx.deleted[season] {
			parent = season
		}
	}
	id := w.newID()
	title := info.extraTitle
	if title == "" {
		title = info.extraKind
	}
	if err := w.insert(ctx, systemq.InsertItemParams{
		ID: id, Kind: "extra", ParentID: nullString(parent), Title: title, SortTitle: SortTitle(title),
		ExtraKind: nullString(info.extraKind),
	}); err != nil {
		return "", err
	}
	return id, nil
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullInt(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: n != 0} }
