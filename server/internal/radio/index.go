package radio

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"math/bits"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The in-memory index of the catalog (concept 16 section 5.4, M-5): every station as a compact
// entry, so a list, the facet counts and the search run as one pass over a slice (40 000 entries,
// no FTS5 in stage 1). It is loaded at the first use, rebuilt after every sync and reloaded per
// station through reloadStations (the hooks onStationChanged and onStationsChanged and the admin
// writes). The visibility
// rules of contract 1.17.0 apply per request with the current settings: hidden stations and those
// of an excluded country never appear, broken ones only while hide_broken is off.

// removalDelay is how long a station stays after its last alias left the catalog (F-18).
const removalDelay = 7 * 24 * time.Hour

// indexEntry is one station of the index. st is the station without the per-profile favourite flag.
type indexEntry struct {
	st         model.RadioStation
	sortKey    string
	search     string
	hidden     bool
	broken     bool // reachable = 0 or broken_at set
	locked     bool // the admin changed a field (exceptions view)
	own        bool // an own station (source own, concept 16 section 5.6)
	popularity int64
	createdAt  int64
	genres     uint32 // bit i = rules.Genres[i]
	kinds      uint32
	moods      uint32
}

// stationIndex holds the entries by id; entries is sorted by id. An entry is never changed in
// place: reloadStation swaps the pointer, so a reader keeps a consistent entry after the lock.
type stationIndex struct {
	entries []*indexEntry
	byID    map[int64]*indexEntry
}

// noValue is a facet bit no entry carries (25 genres, 5 kinds and 5 moods use the low bits).
const noValue = 1 << 31

// facetBits returns the bit set of values within the known values of a facet.
func facetBits(facet string, values []string) uint32 {
	var b uint32
	known := rules.Values(facet)
	for _, v := range values {
		if i := slices.Index(known, v); i >= 0 {
			b |= 1 << i
		}
	}
	return b
}

// newIndexEntry builds an entry from its index row, facets and lock flag.
func newIndexEntry(r appq.ListRadioStationsForIndexRow, facets map[string][]string, locked bool) *indexEntry {
	var langs []string
	_ = json.Unmarshal([]byte(r.Languages), &langs) // written by the sync; a bad value lists none
	st := model.RadioStation{
		ID: r.ID, Name: r.Name, Country: r.Country.String, Region: r.Region.String, Languages: langs,
		Genres: facets[rules.FacetGenre], Kinds: facets[rules.FacetKind], Moods: facets[rules.FacetMood],
		Quality: r.Quality.String, Codec: r.BestCodec.String, BitrateKbps: int(r.BestBitrate.Int64),
		HLS: r.BestHls == 1, ShowsTitles: r.ShowsTitles == 1, Recommended: r.Recommended == 1,
		State: model.RadioStateOK, LogoHash: r.LogoHash.String, Homepage: r.Homepage.String,
	}
	broken := r.Reachable == 0 || r.BrokenAt.Valid
	switch {
	case r.MissingSince.Valid:
		st.State = model.RadioStateMissing
		st.MissingSince = time.UnixMilli(r.MissingSince.Int64).UTC()
		st.RemovalAt = st.MissingSince.Add(removalDelay)
	case broken:
		st.State = model.RadioStateBroken
	}
	return &indexEntry{
		st: st, sortKey: r.SortKey, search: r.SearchText, hidden: r.Hidden == 1, broken: broken, locked: locked,
		own:        r.Source == sourceOwn,
		popularity: r.Votes + r.Clicks, createdAt: r.CreatedAt,
		genres: facetBits(rules.FacetGenre, st.Genres), kinds: facetBits(rules.FacetKind, st.Kinds),
		moods: facetBits(rules.FacetMood, st.Moods),
	}
}

// loadEntries reads the entries of every station (id invalid) or of one.
func loadEntries(ctx context.Context, q *appq.Queries, id sql.NullInt64) ([]*indexEntry, error) {
	if id.Valid {
		e, err := loadEntry(ctx, q, id.Int64)
		if err != nil || e == nil {
			return nil, err
		}
		return []*indexEntry{e}, nil
	}
	rows, err := q.ListRadioStationsForIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stations for the index: %w", err)
	}
	facetRows, err := q.ListRadioStationFacets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list station facets: %w", err)
	}
	lockRows, err := q.ListRadioStationLockFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("list station locks: %w", err)
	}
	facets := make(map[int64]map[string][]string)
	for _, f := range facetRows {
		m := facets[f.StationID]
		if m == nil {
			m = map[string][]string{}
			facets[f.StationID] = m
		}
		m[f.Facet] = append(m[f.Facet], f.Value)
	}
	for _, m := range facets {
		sortFacetValues(m)
	}
	locked := make(map[int64]bool)
	for _, l := range lockRows {
		locked[l.StationID] = true
	}
	out := make([]*indexEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, newIndexEntry(r, facets[r.ID], locked[r.ID]))
	}
	return out, nil
}

// loadEntry reads the entry of one station through its keys (nil when the station is gone). The
// reload after every logo and every admin write and each row of the admin list run it, so it never
// reads the whole catalog: on CT 190 with 55,000 stations the shared query took about 300 ms per
// station and an admin page of 50 stations 15 s.
func loadEntry(ctx context.Context, q *appq.Queries, id int64) (*indexEntry, error) {
	rows, err := q.GetRadioStationForIndex(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get station for the index: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	facetRows, err := q.ListRadioStationFacetsByStation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list station facets: %w", err)
	}
	facets := map[string][]string{}
	for _, f := range facetRows {
		facets[f.Facet] = append(facets[f.Facet], f.Value)
	}
	sortFacetValues(facets)
	locks, err := q.ListRadioStationLocks(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list station locks: %w", err)
	}
	return newIndexEntry(appq.ListRadioStationsForIndexRow(rows[0]), facets, len(locks) > 0), nil
}

// sortFacetValues puts the values of each facet in the order of the rules.
func sortFacetValues(m map[string][]string) {
	for facet, vals := range m {
		known := rules.Values(facet)
		slices.SortFunc(vals, func(a, b string) int { return cmp.Compare(slices.Index(known, a), slices.Index(known, b)) })
	}
}

// index returns the loaded index, loading it at the first call.
func (s *Service) index(ctx context.Context) (*stationIndex, error) {
	s.cat.mu.RLock()
	idx := s.cat.idx
	s.cat.mu.RUnlock()
	if idx != nil {
		return idx, nil
	}
	return s.rebuildIndex(ctx)
}

// rebuildIndex reads every station from the database and swaps the index.
func (s *Service) rebuildIndex(ctx context.Context) (*stationIndex, error) {
	s.cat.writeMu.Lock()
	defer s.cat.writeMu.Unlock()
	entries, err := loadEntries(ctx, appq.New(s.db.Read), sql.NullInt64{})
	if err != nil {
		return nil, err
	}
	idx := &stationIndex{entries: entries, byID: make(map[int64]*indexEntry, len(entries))}
	for _, e := range entries {
		idx.byID[e.st.ID] = e
	}
	s.cat.mu.Lock()
	s.cat.idx = idx
	s.cat.mu.Unlock()
	s.cat.stationsChanged()
	return idx, nil
}

// reloadStation reloads one station into the index (the hook onStationChanged and the admin
// writes; reloadStations).
func (s *Service) reloadStation(ctx context.Context, id int64) {
	s.reloadStations(ctx, []int64{id})
}

// reloadStations reloads stations into the index (the hook onStationsChanged) with one copy of
// the index for all of them; a station that is gone leaves the index. Before the first load it
// does nothing, the load reads the rows anyway. writeMu orders it with rebuildIndex and other
// reloads: each reads its rows and swaps the index under it, so an older read never replaces the
// entry of a newer one.
func (s *Service) reloadStations(ctx context.Context, ids []int64) {
	if len(ids) == 0 {
		return
	}
	s.cat.writeMu.Lock()
	defer s.cat.writeMu.Unlock()
	s.cat.mu.RLock()
	old := s.cat.idx
	s.cat.mu.RUnlock()
	if old == nil {
		return
	}
	fresh := make(map[int64]*indexEntry, len(ids)) // nil: the station is gone
	for _, id := range ids {
		entries, err := loadEntries(ctx, appq.New(s.db.Read), sql.NullInt64{Int64: id, Valid: true})
		if err != nil {
			s.log.WarnContext(ctx, "reload radio station into the index", "station_id", id, "err", err)
			continue
		}
		fresh[id] = nil
		if len(entries) == 1 {
			fresh[id] = entries[0]
		}
	}
	if len(fresh) == 0 {
		return
	}
	// Copy on write: readers that took the old slice and map keep them unchanged.
	idx := &stationIndex{entries: slices.Clone(old.entries), byID: maps.Clone(old.byID)}
	for id, e := range fresh {
		pos, found := sort.Find(len(idx.entries), func(i int) int { return cmp.Compare(id, idx.entries[i].st.ID) })
		switch {
		case e == nil && found:
			idx.entries = slices.Delete(idx.entries, pos, pos+1)
			delete(idx.byID, id)
		case e != nil && found:
			idx.entries[pos] = e
			idx.byID[id] = e
		case e != nil:
			idx.entries = slices.Insert(idx.entries, pos, e)
			idx.byID[id] = e
		}
	}
	s.cat.mu.Lock()
	s.cat.idx = idx
	s.cat.mu.Unlock()
	s.cat.stationsChanged()
}

// catalogView is the visibility of the stations under the current settings.
type catalogView struct {
	countries  map[string]bool // nil = every country
	hideBroken bool
}

// reachable reports whether a profile may reach the station at all (else 404).
func (v catalogView) reachable(e *indexEntry) bool {
	return !e.hidden && (v.countries == nil || v.countries[e.st.Country])
}

// listed reports whether the station appears in lists, facets and search.
func (v catalogView) listed(e *indexEntry) bool {
	return v.reachable(e) && (!e.broken || !v.hideBroken)
}

// Filters of a query, as bits of a fail mask.
const (
	fQ = 1 << iota
	fCountry
	fRegion
	fLanguage
	fGenre
	fKind
	fMood
	fQuality
	fTitles
	fFavorites
)

// stationFilter is a parsed model.RadioFilter.
type stationFilter struct {
	q                             string
	countries, regions, languages map[string]bool
	qualities                     map[string]bool
	genres, kinds, moods          uint32
	titles                        bool
	favorites                     map[int64]bool // nil = no favourite filter
}

func stringSet(v []string) map[string]bool {
	if len(v) == 0 {
		return nil
	}
	m := make(map[string]bool, len(v))
	for _, s := range v {
		m[s] = true
	}
	return m
}

// newStationFilter checks and parses f; fav is the profile's favourite set (used with f.Favorites).
func newStationFilter(f model.RadioFilter, fav map[int64]bool) (stationFilter, error) {
	var verr *model.ValidationError
	for _, q := range f.Qualities {
		if q != model.RadioQualityHigh && q != model.RadioQualityGood && q != model.RadioQualityBasic {
			verr = verr.Add("quality", "invalid_value")
			break
		}
	}
	if verr != nil {
		return stationFilter{}, verr
	}
	sf := stationFilter{
		q: rules.Fold(f.Q), countries: stringSet(f.Countries), regions: stringSet(f.Regions),
		languages: stringSet(f.Languages), qualities: stringSet(f.Qualities),
		genres: facetBits(rules.FacetGenre, f.Genres), kinds: facetBits(rules.FacetKind, f.Kinds),
		moods: facetBits(rules.FacetMood, f.Moods), titles: f.Titles,
	}
	// A selection of unknown values only still filters: the bit noValue is on no entry.
	for _, p := range []struct {
		bits *uint32
		sel  []string
	}{{&sf.genres, f.Genres}, {&sf.kinds, f.Kinds}, {&sf.moods, f.Moods}} {
		if len(p.sel) > 0 && *p.bits == 0 {
			*p.bits = noValue
		}
	}
	if f.Favorites {
		sf.favorites = fav
		if sf.favorites == nil {
			sf.favorites = map[int64]bool{}
		}
	}
	return sf, nil
}

// fails returns the mask of the filters e does not pass.
func (f *stationFilter) fails(e *indexEntry) int {
	var m int
	if f.q != "" && !strings.Contains(e.search, f.q) {
		m |= fQ
	}
	if f.countries != nil && !f.countries[e.st.Country] {
		m |= fCountry
	}
	if f.regions != nil && !f.regions[e.st.Region] {
		m |= fRegion
	}
	if f.languages != nil && !slices.ContainsFunc(e.st.Languages, func(l string) bool { return f.languages[l] }) {
		m |= fLanguage
	}
	if f.genres != 0 && e.genres&f.genres == 0 {
		m |= fGenre
	}
	if f.kinds != 0 && e.kinds&f.kinds == 0 {
		m |= fKind
	}
	if f.moods != 0 && e.moods&f.moods == 0 {
		m |= fMood
	}
	if f.qualities != nil && !f.qualities[e.st.Quality] {
		m |= fQuality
	}
	if f.titles && !e.st.ShowsTitles {
		m |= fTitles
	}
	if f.favorites != nil && !f.favorites[e.st.ID] {
		m |= fFavorites
	}
	return m
}

// qualityRank orders the qualities for the sort quality.
func qualityRank(q string) int {
	switch q {
	case model.RadioQualityHigh:
		return 3
	case model.RadioQualityGood:
		return 2
	case model.RadioQualityBasic:
		return 1
	}
	return 0
}

// sortEntries sorts by the RadioSort value; popular puts the recommended stations of a selected
// country first.
func sortEntries(list []*indexEntry, sortBy string, countries map[string]bool) {
	byName := func(a, b *indexEntry) int {
		return cmp.Or(cmp.Compare(a.sortKey, b.sortKey), cmp.Compare(a.st.ID, b.st.ID))
	}
	var fn func(a, b *indexEntry) int
	switch sortBy {
	case model.RadioSortName:
		fn = byName
	case model.RadioSortQuality:
		fn = func(a, b *indexEntry) int {
			return cmp.Or(cmp.Compare(qualityRank(b.st.Quality), qualityRank(a.st.Quality)),
				cmp.Compare(b.st.BitrateKbps, a.st.BitrateKbps), cmp.Compare(b.popularity, a.popularity), byName(a, b))
		}
	case model.RadioSortNew:
		fn = func(a, b *indexEntry) int {
			return cmp.Or(cmp.Compare(b.createdAt, a.createdAt), cmp.Compare(b.st.ID, a.st.ID))
		}
	default:
		boost := func(e *indexEntry) int {
			if countries != nil && e.st.Recommended && countries[e.st.Country] {
				return 1
			}
			return 0
		}
		fn = func(a, b *indexEntry) int {
			return cmp.Or(cmp.Compare(boost(b), boost(a)), cmp.Compare(b.popularity, a.popularity), byName(a, b))
		}
	}
	slices.SortFunc(list, fn)
}

// facetCounter counts the values of the facets over the entries that pass every filter but the
// facet's own (a facet's counts ignore its own selection).
type facetCounter struct {
	countries, regions, languages, genres, kinds, moods, qualities map[string]int
	titles                                                         int
}

func newFacetCounter() *facetCounter {
	return &facetCounter{countries: map[string]int{}, regions: map[string]int{}, languages: map[string]int{},
		genres: map[string]int{}, kinds: map[string]int{}, moods: map[string]int{}, qualities: map[string]int{}}
}

// add counts e for the facets whose filters are all it fails (mask 0: every facet; one facet's bit:
// that facet only).
func (c *facetCounter) add(e *indexEntry, mask int) {
	if mask != 0 && bits.OnesCount(uint(mask)) != 1 {
		return
	}
	on := func(bit int) bool { return mask == 0 || mask == bit }
	if on(fCountry) && e.st.Country != "" {
		c.countries[e.st.Country]++
	}
	if on(fRegion) && e.st.Region != "" {
		c.regions[e.st.Region]++
	}
	if on(fLanguage) {
		for _, l := range e.st.Languages {
			c.languages[l]++
		}
	}
	if on(fGenre) {
		for _, v := range e.st.Genres {
			c.genres[v]++
		}
	}
	if on(fKind) {
		for _, v := range e.st.Kinds {
			c.kinds[v]++
		}
	}
	if on(fMood) {
		for _, v := range e.st.Moods {
			c.moods[v]++
		}
	}
	if on(fQuality) && e.st.Quality != "" {
		c.qualities[e.st.Quality]++
	}
	if on(fTitles) && e.st.ShowsTitles {
		c.titles++
	}
}

// facetList turns counts into the contract list: most stations first, then by value.
func facetList(m map[string]int) []model.RadioFacetCount {
	out := make([]model.RadioFacetCount, 0, len(m))
	for v, n := range m {
		out = append(out, model.RadioFacetCount{Value: v, Count: n})
	}
	slices.SortFunc(out, func(a, b model.RadioFacetCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Value, b.Value))
	})
	return out
}

func (c *facetCounter) result() model.RadioFacets {
	return model.RadioFacets{
		Countries: facetList(c.countries), Regions: facetList(c.regions), Languages: facetList(c.languages),
		Genres: facetList(c.genres), Kinds: facetList(c.kinds), Moods: facetList(c.moods),
		Qualities: facetList(c.qualities), Titles: c.titles,
	}
}
