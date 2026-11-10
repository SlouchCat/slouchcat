package visibility

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Audiobook series and collections of a library (contract 1.25.0, concept 19 stage D2, D4.1). One
// rule for every profile: everything comes from the volumes the profile may see. A series exists for
// a profile only with at least two visible volumes (minSeriesVolumes); its name is the locked name,
// else the most common spelling of the visible volumes without suffix (audioseries.Names.Display;
// with it under the switch metadata.books.normalize_series off, concept 19 D6.4), and its author
// the most common unified name (items.author_display, else items.author),
// so no cover, name or count of a book above the profile's ceiling reaches it. A collection counts
// in the row of a library when its visible members are all audiobooks and one of them is in that
// library (ListVisibleAudioCollections). The volumes are read with one query each and grouped in
// Go; the tiles' covers and the page's volumes are ItemSummaries like every list (ordered).

// minSeriesVolumes is the number of visible volumes a series needs to exist for a profile.
const minSeriesVolumes = 2

// seriesVolume is one visible volume of a series as the queries read it.
type seriesVolume = scopedq.ListVisibleAudioSeriesVolumesRow

// compareVolumes is the series order (D4.3): by number, 0.5 before 1, books without number last
// by title sort key; the id breaks ties, so two books with the same number both stay, in a fixed
// order.
func compareVolumes(a, b seriesVolume) int {
	switch {
	case a.SeriesIndex.Valid != b.SeriesIndex.Valid:
		if a.SeriesIndex.Valid {
			return -1
		}
		return 1
	case a.SeriesIndex.Valid && a.SeriesIndex.Float64 != b.SeriesIndex.Float64:
		return cmp.Compare(a.SeriesIndex.Float64, b.SeriesIndex.Float64)
	}
	return cmp.Or(strings.Compare(a.SortKey, b.SortKey), strings.Compare(a.ID, b.ID))
}

// seriesFacts are the values of a series that come from its visible volumes, in series order.
type seriesFacts struct {
	name, author, narrator string
	duration               int64
	completed              int
}

// factsOf computes the facts of the visible volumes; names give the shown spelling of the series
// name (seriesNames).
func factsOf(volumes []seriesVolume, names audioseries.Names) seriesFacts {
	var f seriesFacts
	shown := make([]string, 0, len(volumes))
	authors := make([]string, 0, len(volumes))
	f.narrator = volumes[0].Narrator.String
	for _, v := range volumes {
		shown = append(shown, names.Display(v.SeriesName.String))
		authors = append(authors, v.Author.String)
		if v.Narrator.String != f.narrator {
			f.narrator = ""
		}
		f.duration += v.DurationMs.Int64
		if v.Completed == 1 {
			f.completed++
		}
	}
	f.author = audioseries.MostCommon(authors)
	f.name = volumes[0].SeriesDisplay // the stored name: locked, or the spelling of all books
	if volumes[0].NameLocked == 0 {
		if n := audioseries.MostCommon(shown); n != "" {
			f.name = n
		}
	}
	return f
}

// seriesNames returns the Names that give the shown spelling of a series name: without suffix,
// or with it under the switch metadata.books.normalize_series off (concept 19 D6.4).
func (r *Reader) seriesNames(ctx context.Context) (audioseries.Names, error) {
	v, err := r.sys.GetSystemSetting(ctx, audioseries.SettingNormalizeSeries)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return audioseries.Names{}, fmt.Errorf("read %s: %w", audioseries.SettingNormalizeSeries, err)
	}
	return audioseries.Default().Shown(audioseries.SwitchOn(v, err == nil)), nil
}

// libraryFiltered reports whether q filters the titles of a library page (genre, years, rating,
// status).
func libraryFiltered(q model.LibraryQuery) bool {
	return q.Genre != "" || q.YearFrom != 0 || q.YearTo != 0 || q.MaxRating >= 0 || len(q.Status) > 0
}

// AudioGroups returns the row «Reihen und Sammlungen» of a library (listAudioGroups): its series
// with at least two visible volumes and its audiobook collections, by name (folded like the title
// sort: model.SortKey in the profile's metadata language), then id. The filters genre, years,
// maxRating and status of q are those of LibraryItems over the audiobooks of the library
// (libraryStmt): a group appears when at least one of its volumes in this library passes them, and
// Matching counts its visible volumes that pass, for a collection in every library (as Count). A
// library the profile does not see is model.ErrNotFound; one without series and collections
// answers an empty list. A status outside its values is a *model.ValidationError
// (validateLibraryQuery). Everything is read in one read-only transaction.
func (r *Reader) AudioGroups(ctx context.Context, s Scope, libraryID int64, q model.LibraryQuery) (model.AudioGroupList, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.AudioGroupList{}, err
	}
	f, verr := validateLibraryQuery(model.LibraryQuery{Genre: q.Genre, YearFrom: q.YearFrom, YearTo: q.YearTo, MaxRating: q.MaxRating, Status: q.Status})
	if verr != nil {
		return model.AudioGroupList{}, verr
	}
	if _, err := r.q.GetVisibleLibrary(ctx, scopedq.GetVisibleLibraryParams{ProfileID: pid, LibraryID: libraryID}); err != nil {
		return model.AudioGroupList{}, notFound(err)
	}
	f.kinds = []string{"audiobook"}
	out := model.AudioGroupList{Groups: []model.AudioGroup{}}
	names, err := r.seriesNames(ctx)
	if err != nil {
		return model.AudioGroupList{}, err
	}

	tx, err := r.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.AudioGroupList{}, fmt.Errorf("begin audio groups: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	tr := r.inTx(tx)
	qtx := tr.q
	// The books of a library that pass the filters, read once per library (a collection can hold
	// books of other audiobook libraries).
	filtered := libraryFiltered(f.LibraryQuery)
	passingIn := map[int64]map[string]bool{}
	passes := func(lib int64) (map[string]bool, error) {
		if set, ok := passingIn[lib]; ok {
			return set, nil
		}
		ids, _, err := libraryIDs(ctx, tx, libraryStmt(s, pid, lib, f, "SELECT id, kind FROM lib WHERE 1", false))
		if err != nil {
			return nil, err
		}
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		passingIn[lib] = set
		return set, nil
	}
	var passing map[string]bool // nil: no filter, every visible volume passes
	if filtered {
		if passing, err = passes(libraryID); err != nil {
			return model.AudioGroupList{}, err
		}
	}
	volumes, err := qtx.ListVisibleAudioSeriesVolumes(ctx, scopedq.ListVisibleAudioSeriesVolumesParams{
		ProfileID: pid, MetaLang: s.metaLang, UiLang: s.language, LibraryID: libraryID,
	})
	if err != nil {
		return model.AudioGroupList{}, fmt.Errorf("list series volumes: %w", err)
	}
	collections, err := qtx.ListVisibleAudioCollections(ctx, scopedq.ListVisibleAudioCollectionsParams{ProfileID: pid, LibraryID: libraryID})
	if err != nil {
		return model.AudioGroupList{}, fmt.Errorf("list audio collections: %w", err)
	}
	// With a filter a collection counts its passing visible members in every library, like its
	// badge counts all of them, and appears only with a passing member in this library.
	matching, passesHere := map[string]int{}, map[string]bool{}
	if filtered && len(collections) > 0 {
		ids := make([]string, 0, len(collections))
		for _, c := range collections {
			ids = append(ids, c.ID)
		}
		members, err := qtx.ListVisibleCollectionMembers(ctx, scopedq.ListVisibleCollectionMembersParams{ProfileID: pid, CollectionIds: ids})
		if err != nil {
			return model.AudioGroupList{}, fmt.Errorf("list audio collection members: %w", err)
		}
		for _, m := range members {
			set, err := passes(m.LibraryID)
			if err != nil {
				return model.AudioGroupList{}, err
			}
			if set[m.ItemID] {
				matching[m.CollectionID]++
				passesHere[m.CollectionID] = passesHere[m.CollectionID] || m.LibraryID == libraryID
			}
		}
	}

	bySeries := map[string][]seriesVolume{}
	for _, v := range volumes {
		bySeries[v.SeriesID] = append(bySeries[v.SeriesID], v)
	}
	var coverIDs []string
	covers := map[int]string{} // group index → cover id
	for id, vs := range bySeries {
		if len(vs) < minSeriesVolumes {
			continue
		}
		slices.SortFunc(vs, compareVolumes)
		g := model.AudioGroup{ID: id, Kind: model.AudioGroupSeries, Count: len(vs), Matching: len(vs)}
		if passing != nil {
			g.Matching = 0
			for _, v := range vs {
				if passing[v.ID] {
					g.Matching++
				}
			}
			if g.Matching == 0 {
				continue
			}
		}
		facts := factsOf(vs, names)
		g.Name, g.Author, g.Completed = facts.name, facts.author, facts.completed
		covers[len(out.Groups)] = vs[0].ID
		coverIDs = append(coverIDs, vs[0].ID)
		out.Groups = append(out.Groups, g)
		out.SeriesCount++
	}
	for _, c := range collections {
		g := model.AudioGroup{ID: c.ID, Kind: model.AudioGroupCollection, Name: localized(s.language, c.Name, c.NameDe),
			Count: int(c.MemberCount), Matching: int(c.MemberCount), Completed: int(c.Completed)}
		if filtered {
			if !passesHere[c.ID] {
				continue
			}
			g.Matching = matching[c.ID]
		}
		if c.CoverID != "" {
			covers[len(out.Groups)] = c.CoverID
			coverIDs = append(coverIDs, c.CoverID)
		}
		out.Groups = append(out.Groups, g)
		out.CollectionCount++
	}
	summaries, err := tr.ordered(ctx, s, pid, coverIDs)
	if err != nil {
		return model.AudioGroupList{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AudioGroupList{}, fmt.Errorf("commit audio groups: %w", err)
	}
	byID := make(map[string]model.ItemSummary, len(summaries))
	for _, it := range summaries {
		byID[it.ID] = it
	}
	for i, id := range covers {
		if it, ok := byID[id]; ok {
			out.Groups[i].Cover = &it
		}
	}
	slices.SortFunc(out.Groups, func(a, b model.AudioGroup) int {
		return cmp.Or(strings.Compare(model.SortKey(s.metaLang, a.Name), model.SortKey(s.metaLang, b.Name)), strings.Compare(a.ID, b.ID))
	})
	return out, nil
}

// AudioSeries returns the page of a series of a library (getAudioSeries) over its visible volumes
// in series order (compareVolumes): name, author, narrator (when every visible volume has the
// same), duration, cover (the first volume), Next (the first volume the profile has not completed,
// else the first) and the spellings of the series name with their counts, the most common first. A
// series with fewer than two visible volumes, of another library or unknown is model.ErrNotFound.
func (r *Reader) AudioSeries(ctx context.Context, s Scope, libraryID int64, seriesID string) (model.AudioSeries, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.AudioSeries{}, err
	}
	names, err := r.seriesNames(ctx)
	if err != nil {
		return model.AudioSeries{}, err
	}
	// The volumes and their summaries in one read-only transaction: facts and list agree.
	tx, err := r.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.AudioSeries{}, fmt.Errorf("begin audio series: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	tr := r.inTx(tx)
	rows, err := tr.q.ListVisibleAudioSeriesVolumesOf(ctx, scopedq.ListVisibleAudioSeriesVolumesOfParams{
		ProfileID: pid, MetaLang: s.metaLang, UiLang: s.language, SeriesID: sql.NullString{String: seriesID, Valid: true}, LibraryID: libraryID,
	})
	if err != nil {
		return model.AudioSeries{}, fmt.Errorf("list series volumes: %w", err)
	}
	if len(rows) < minSeriesVolumes {
		return model.AudioSeries{}, model.ErrNotFound
	}
	vs := make([]seriesVolume, 0, len(rows))
	spelled := map[string]int{}
	for _, row := range rows {
		vs = append(vs, seriesVolume(row))
		if row.SeriesName.String != "" {
			spelled[row.SeriesName.String]++
		}
	}
	slices.SortFunc(vs, compareVolumes)
	facts := factsOf(vs, names)
	out := model.AudioSeries{ID: seriesID, LibraryID: libraryID, Name: facts.name, NameLocked: vs[0].NameLocked == 1,
		Author: facts.author, Narrator: facts.narrator, DurationMs: facts.duration,
		Volumes: make([]model.AudioSeriesVolume, 0, len(vs)), Spellings: make([]model.AudioSeriesSpelling, 0, len(spelled))}
	for name, n := range spelled {
		out.Spellings = append(out.Spellings, model.AudioSeriesSpelling{Name: name, Count: n})
	}
	slices.SortFunc(out.Spellings, func(a, b model.AudioSeriesSpelling) int {
		return cmp.Or(b.Count-a.Count, strings.Compare(a.Name, b.Name))
	})
	ids := make([]string, 0, len(vs))
	for _, v := range vs {
		ids = append(ids, v.ID)
	}
	items, err := tr.ordered(ctx, s, pid, ids)
	if err != nil {
		return model.AudioSeries{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AudioSeries{}, fmt.Errorf("commit audio series: %w", err)
	}
	byID := make(map[string]model.ItemSummary, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	next := ""
	for _, v := range vs {
		it, ok := byID[v.ID]
		if !ok {
			continue // ordered returns what the same transaction read; kept as a guard
		}
		vol := model.AudioSeriesVolume{Item: it, Narrator: v.Narrator.String}
		if v.SeriesIndex.Valid {
			idx := v.SeriesIndex.Float64
			vol.SeriesIndex = &idx
		}
		out.Volumes = append(out.Volumes, vol)
		if next == "" && v.Completed == 0 {
			next = v.ID
		}
	}
	if len(out.Volumes) < minSeriesVolumes {
		return model.AudioSeries{}, model.ErrNotFound
	}
	cover := out.Volumes[0].Item
	out.Cover, out.Next = &cover, &cover
	for _, vol := range out.Volumes {
		if vol.Item.ID == next {
			n := vol.Item
			out.Next = &n
		}
	}
	return out, nil
}

// inTx returns a Reader whose generated queries run in tx (a read-only transaction), so several
// reads see one state of the database.
func (r *Reader) inTx(tx *sql.Tx) *Reader {
	c := *r
	c.q = r.q.WithTx(tx)
	return &c
}

// visibleSeriesID returns the series of a visible book when it has at least two visible volumes
// for the profile (ItemDetail.seriesId), else "".
func (r *Reader) visibleSeriesID(ctx context.Context, pid int64, it scopedq.Item) (string, error) {
	if !it.AudioSeriesID.Valid {
		return "", nil
	}
	n, err := r.q.CountVisibleAudioSeriesVolumes(ctx, scopedq.CountVisibleAudioSeriesVolumesParams{ProfileID: pid, SeriesID: it.AudioSeriesID})
	if err != nil {
		return "", fmt.Errorf("count series volumes: %w", err)
	}
	if n < minSeriesVolumes {
		return "", nil
	}
	return it.AudioSeriesID.String, nil
}
