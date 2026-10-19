package library

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// itemIndex finds the items of a library during a scan.
//
// Titles (movies in a movie library, series in a series library): undeleted ones by provider id
// (TVDB, TMDB, IMDb) or by (sort_title, year), titles soft-deleted within relinkWindow by
// provider id only. Seasons by (series, season number), episodes by (season, episode number),
// both including those soft-deleted within relinkWindow, so a series that comes back keeps its
// tree. Folders map titles to the folders of their files and back, for titles without a year and
// for extras, whose parent is the title of their folder.
type itemIndex struct {
	byTMDB   map[int64]string
	byTVDB   map[int64]string
	byIMDB   map[string]string
	byTitle  map[string][]itemRef
	deleted  map[string]bool            // soft-deleted items that a new file may restore
	parent   map[string]string          // item id → parent id (seasons, episodes, extras)
	kinds    map[string]string          // item id → kind
	folders  map[string]map[string]bool // title id → its folders (see titleFolderKey)
	byFolder map[string]string          // folder → title id; "" when several titles share it
	seasons  map[string]string          // seriesKey(series id, season number) → season id
	episodes map[string]string          // seriesKey(season id, episode number) → episode id
	matched  map[string]bool            // series with metadata_state 'matched'
}

type itemRef struct {
	id         string
	tmdb, tvdb int64
	imdb       string
}

func newItemIndex() *itemIndex {
	return &itemIndex{
		byTMDB: map[int64]string{}, byTVDB: map[int64]string{}, byIMDB: map[string]string{},
		byTitle: map[string][]itemRef{}, deleted: map[string]bool{}, parent: map[string]string{}, kinds: map[string]string{},
		folders: map[string]map[string]bool{}, byFolder: map[string]string{},
		seasons: map[string]string{}, episodes: map[string]string{}, matched: map[string]bool{},
	}
}

func titleKey(sortTitle string, year sql.NullInt64) string {
	if !year.Valid {
		return sortTitle + "|"
	}
	return sortTitle + "|" + strconv.FormatInt(year.Int64, 10)
}

// seriesKey names a child of a container by its number: a season of a series, an episode of a
// season.
func seriesKey(parentID string, n int64) string {
	return parentID + "|" + strconv.FormatInt(n, 10)
}

// titleFolderKey names the folder of a title in a root; "" for no folder (a file directly in the
// root of a movie library).
func titleFolderKey(rootID int64, dir string) string {
	if dir == "" {
		return ""
	}
	return strconv.FormatInt(rootID, 10) + "/" + dir
}

// seriesFolderKey names the series folder of an episode or series extra. Episodes directly in
// the root have no folder; they share a key per series title, so they form one series.
func seriesFolderKey(rootID int64, ep *ParsedEpisode) string {
	if ep.SeriesDir != "" {
		return titleFolderKey(rootID, ep.SeriesDir)
	}
	return strconv.FormatInt(rootID, 10) + "/\x00" + SortTitle(ep.SeriesTitle)
}

// fileFolderKey returns the title folder of a media file of the library kind: the movie folder,
// or the series folder; "" when there is none or the name does not parse.
func fileFolderKey(kind string, rootID int64, rel string, p ParsedName) string {
	if kind != "series" {
		return titleFolderKey(rootID, movieFolder(rel))
	}
	if p.Series == nil {
		return ""
	}
	return seriesFolderKey(rootID, p.Series)
}

func (s *Service) loadItemIndex(ctx context.Context, lib appq.Library, scans []*rootScan) (*itemIndex, error) {
	q := systemq.New(s.db.Read)
	items, err := q.ListItemsByLibrary(ctx, lib.ID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	deleted, err := q.ListRecentlyDeletedItemsByLibrary(ctx, systemq.ListRecentlyDeletedItemsByLibraryParams{
		LibraryID: lib.ID, Since: sql.NullInt64{Int64: s.now().Add(-relinkWindow).UnixMilli(), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list deleted items: %w", err)
	}
	titleKind := "movie"
	if lib.Kind == "series" {
		titleKind = "series"
	}
	idx := newItemIndex()
	add := func(it systemq.Item, isDeleted bool) {
		idx.kinds[it.ID] = it.Kind
		if it.ParentID.Valid {
			idx.parent[it.ID] = it.ParentID.String
		}
		switch it.Kind {
		case titleKind:
			if isDeleted {
				idx.addDeleted(it.ID, it.TmdbID.Int64, it.TvdbID.Int64, it.ImdbID.String)
				return
			}
			idx.add(it.ID, it.SortTitle, it.Year, it.TmdbID.Int64, it.TvdbID.Int64, it.ImdbID.String)
			if it.MetadataState == "matched" {
				idx.matched[it.ID] = true
			}
		case "season":
			if it.ParentID.Valid && it.SeasonNumber.Valid {
				idx.addChild(idx.seasons, seriesKey(it.ParentID.String, it.SeasonNumber.Int64), it.ID, isDeleted)
			}
		case "episode":
			if it.ParentID.Valid && it.EpisodeNumber.Valid {
				idx.addChild(idx.episodes, seriesKey(it.ParentID.String, it.EpisodeNumber.Int64), it.ID, isDeleted)
			}
		}
	}
	for _, it := range items {
		add(it, false)
	}
	// After the undeleted items, so an undeleted item with the same id or number wins.
	for _, it := range deleted {
		add(it, true)
	}
	for _, rs := range scans {
		parsed := make(map[string]*ParsedName, len(rs.disk))
		for i := range rs.disk {
			parsed[rs.disk[i].rel] = &rs.disk[i].parsed
		}
		for _, k := range rs.known {
			title := idx.title(k.ItemID, titleKind)
			if title == "" {
				continue
			}
			p := parsed[k.RelPath]
			if p == nil {
				pp := parsePath(lib.Kind, k.RelPath)
				p = &pp
			}
			idx.addFolder(title, fileFolderKey(lib.Kind, rs.root.ID, k.RelPath, *p))
		}
	}
	return idx, nil
}

// title returns the title (movie or series) that item id belongs to: itself or an ancestor of
// kind titleKind; "" when there is none in the index.
func (idx *itemIndex) title(id, titleKind string) string {
	for range 4 {
		if idx.kinds[id] == titleKind {
			return id
		}
		p, ok := idx.parent[id]
		if !ok {
			return ""
		}
		id = p
	}
	return ""
}

func (idx *itemIndex) add(id, sortTitle string, year sql.NullInt64, tmdb, tvdb int64, imdb string) {
	idx.addIDs(id, tmdb, tvdb, imdb)
	key := titleKey(sortTitle, year)
	idx.byTitle[key] = append(idx.byTitle[key], itemRef{id: id, tmdb: tmdb, tvdb: tvdb, imdb: imdb})
}

func (idx *itemIndex) addIDs(id string, tmdb, tvdb int64, imdb string) {
	if tmdb != 0 {
		if _, ok := idx.byTMDB[tmdb]; !ok {
			idx.byTMDB[tmdb] = id
		}
	}
	if tvdb != 0 {
		if _, ok := idx.byTVDB[tvdb]; !ok {
			idx.byTVDB[tvdb] = id
		}
	}
	if imdb != "" {
		if _, ok := idx.byIMDB[imdb]; !ok {
			idx.byIMDB[imdb] = id
		}
	}
}

// addDeleted makes a soft-deleted title reachable by its provider ids, not by its title.
func (idx *itemIndex) addDeleted(id string, tmdb, tvdb int64, imdb string) {
	if tmdb == 0 && tvdb == 0 && imdb == "" {
		return
	}
	idx.deleted[id] = true
	idx.addIDs(id, tmdb, tvdb, imdb)
}

// addChild adds a season or an episode; a soft-deleted one only when no undeleted one holds the
// number.
func (idx *itemIndex) addChild(m map[string]string, key, id string, isDeleted bool) {
	if _, ok := m[key]; ok {
		return
	}
	m[key] = id
	if isDeleted {
		idx.deleted[id] = true
	}
}

func (idx *itemIndex) addFolder(id, folder string) {
	if folder == "" {
		return
	}
	if idx.folders[id] == nil {
		idx.folders[id] = map[string]bool{}
	}
	idx.folders[id][folder] = true
	if cur, ok := idx.byFolder[folder]; !ok {
		idx.byFolder[folder] = id
	} else if cur != id {
		idx.byFolder[folder] = "" // two titles in one folder: no parent for extras
	}
}

// folderTitle returns the undeleted title of a folder, the parent of an extra there.
func (idx *itemIndex) folderTitle(folder string) string {
	id := idx.byFolder[folder]
	if folder == "" || idx.deleted[id] {
		return ""
	}
	return id
}

// find returns the title of a file: by TVDB id, else TMDB id, else IMDb id, else (sort_title,
// year) when the provider ids do not contradict. Without a year, a title only matches an item
// that already has a file in the same folder (as titleFolderKey or seriesFolderKey name it), so
// two titles of the same name in different folders stay apart.
func (idx *itemIndex) find(m titleInfo, folder string) string {
	if m.tvdb != 0 {
		if id, ok := idx.byTVDB[m.tvdb]; ok {
			return id
		}
	}
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
		if !m.year.Valid && (folder == "" || !idx.folders[ref.id][folder]) {
			continue
		}
		if (m.tmdb == 0 || ref.tmdb == 0 || m.tmdb == ref.tmdb) && (m.tvdb == 0 || ref.tvdb == 0 || m.tvdb == ref.tvdb) &&
			(m.imdb == "" || ref.imdb == "" || m.imdb == ref.imdb) {
			return ref.id
		}
	}
	return ""
}

// undelete forgets id and its ancestors as soft-deleted after RestoreItem brought them back.
func (idx *itemIndex) undelete(id string) {
	for range 3 {
		delete(idx.deleted, id)
		p, ok := idx.parent[id]
		if !ok {
			return
		}
		id = p
	}
}
