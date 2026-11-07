package radio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The catalog sync (concept 16 section 5.3, F-14, F-18): the whole Radio Browser list is streamed
// once a day and merged into stations in transactions of syncBatch entries. An entry whose
// lastchangetime equals the one of its last import only refreshes seen_at and its counters (m-8),
// so a daily run does not rewrite ~40 000 rows. A new entry joins the station of its group key or
// founds one; an alias never moves and two stations never merge (R-2). After the entries, every
// station whose entries changed is derived again (name, country, region, languages, quality, tags,
// facets; a locked field keeps the admin's value). Entries that stayed away are marked missing;
// a catalog station missing for seven days is deleted with its favourites (F-18). A cancelled run
// stops between two batches and marks nothing missing.

const (
	syncBatch = 1000
	// syncNameMax is the CHECK of radio_stations.name.
	syncNameMax = 200
)

// isoRegion is an ISO 3166-2 code as Radio Browser writes it into iso_3166_2.
var isoRegion = regexp.MustCompile(`^[A-Z]{2}-[A-Z0-9]{1,3}$`)

// syncAlias is a known alias at the start of a run.
type syncAlias struct {
	station int64
	changed string
	ok      bool
}

// syncStation are the stored counters of a station at the start of a run.
type syncStation struct {
	votes, clicks int64
	reachable     bool
}

// entryMeta is what the recompute of a station reads of one of its entries.
type entryMeta struct {
	uuid, name, homepage, favicon, country, iso, state, language, langCodes, tags string
	codec                                                                         string
	bitrate, votes, clicks                                                        int64
	ok                                                                            bool
}

// syncRun is the state of one sync.
type syncRun struct {
	s        *Service
	now      int64
	aliases  map[string]syncAlias
	groups   map[string]int64
	stations map[int64]syncStation
	locks    map[int64]map[string]bool
	metas    map[int64][]entryMeta
	full     map[int64]bool // stations whose entries changed
	created  map[int64]bool // stations this run founded (no tags, facets or locks yet)
	added    int
	known    int // aliases at the start of the run
	entries  int
	okCount  int
	batch    []*rbStation
}

// syncStats is the outcome of a successful run.
type syncStats struct {
	entries, reachable, stations, added, removed int
}

// syncCatalog runs one sync.
func (s *Service) syncCatalog(ctx context.Context) (syncStats, error) {
	s.cat.syncMu.Lock()
	defer s.cat.syncMu.Unlock()
	run, err := s.newSyncRun(ctx)
	if err != nil {
		return syncStats{}, err
	}
	if err := s.cat.fetch.fetch(ctx, func(e *rbStation) error { return run.add(ctx, e) }); err != nil {
		return syncStats{}, err
	}
	if err := run.flush(ctx); err != nil {
		return syncStats{}, err
	}
	// A list with less than half of the known entries is a broken answer, not a catalog that
	// lost half its stations: nothing is marked missing (the entries read are stored).
	if run.known > 0 && run.entries*2 < run.known {
		s.log.WarnContext(ctx, "radio catalog list too short, stations not marked missing",
			"entries", run.entries, "known", run.known)
		return syncStats{}, fmt.Errorf("%w: %d entries, %d known", errCatalogShrunk, run.entries, run.known)
	}
	return run.finish(ctx)
}

// errCatalogShrunk is the sync error of a station list with less than half of the known entries.
var errCatalogShrunk = errors.New("radio browser station list shorter than half the known entries")

// newSyncRun loads the alias, group, counter and lock maps once.
func (s *Service) newSyncRun(ctx context.Context) (*syncRun, error) {
	q := appq.New(s.db.Read)
	run := &syncRun{s: s, now: s.now().UnixMilli(), aliases: map[string]syncAlias{}, groups: map[string]int64{},
		stations: map[int64]syncStation{}, locks: map[int64]map[string]bool{}, metas: map[int64][]entryMeta{},
		full: map[int64]bool{}, created: map[int64]bool{}}
	aliases, err := q.ListRadioAliasesForSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio aliases: %w", err)
	}
	for _, a := range aliases {
		run.aliases[a.Stationuuid] = syncAlias{station: a.StationID, changed: a.CatalogChanged.String, ok: a.Lastcheckok == 1}
	}
	run.known = len(run.aliases)
	stations, err := q.ListRadioStationsForSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio stations: %w", err)
	}
	for _, st := range stations {
		run.stations[st.ID] = syncStation{votes: st.Votes, clicks: st.Clicks, reachable: st.Reachable == 1}
		if _, ok := run.groups[st.GroupKey]; !ok && st.Source == "catalog" {
			run.groups[st.GroupKey] = st.ID
		}
	}
	locks, err := q.ListRadioStationLockFields(ctx, sql.NullInt64{})
	if err != nil {
		return nil, fmt.Errorf("list radio locks: %w", err)
	}
	for _, l := range locks {
		m := run.locks[l.StationID]
		if m == nil {
			m = map[string]bool{}
			run.locks[l.StationID] = m
		}
		m[l.Field] = true
	}
	return run, nil
}

// add takes one entry of the stream and writes a full batch; a cancelled ctx stops the run
// between two batches.
func (r *syncRun) add(ctx context.Context, e *rbStation) error {
	e.StationUUID = strings.TrimSpace(e.StationUUID)
	e.Name = strings.TrimSpace(e.Name)
	if e.StationUUID == "" || e.Name == "" || strings.TrimSpace(e.URL) == "" {
		return nil
	}
	r.batch = append(r.batch, e)
	if len(r.batch) < syncBatch {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.flush(ctx); err != nil {
		return err
	}
	jobs.Progress(ctx, int64(r.entries), 0, "")
	return nil
}

// flush writes the pending entries in one transaction.
func (r *syncRun) flush(ctx context.Context) error {
	if len(r.batch) == 0 {
		return nil
	}
	tx, err := r.s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("radio sync: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stx := newStmtTx(tx)
	defer stx.close()
	q := appq.New(stx)
	for _, e := range r.batch {
		if err := r.write(ctx, q, e); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("radio sync: commit: %w", err)
	}
	r.batch = r.batch[:0]
	return nil
}

// write merges one entry.
func (r *syncRun) write(ctx context.Context, q *appq.Queries, e *rbStation) error {
	ok := e.LastCheckOK == 1
	r.entries++
	if ok {
		r.okCount++
	}
	a, known := r.aliases[e.StationUUID]
	switch {
	case known && a.changed == e.LastChangeTime:
		if err := q.TouchRadioAlias(ctx, appq.TouchRadioAliasParams{Now: r.now, Stationuuid: e.StationUUID}); err != nil {
			return fmt.Errorf("touch radio alias: %w", err)
		}
		if a.ok != ok {
			if err := q.SetRadioStreamCheck(ctx, appq.SetRadioStreamCheckParams{Lastcheckok: flag01(ok), Now: r.now,
				Stationuuid: sql.NullString{String: e.StationUUID, Valid: true}}); err != nil {
				return fmt.Errorf("set radio stream check: %w", err)
			}
			r.full[a.station] = true
		}
	case known:
		if err := r.upsert(ctx, q, a.station, e); err != nil {
			return err
		}
		r.full[a.station] = true
	default:
		key := groupKey(e)
		id, ok := r.groups[key]
		if !ok {
			var err error
			id, err = q.InsertRadioCatalogStation(ctx, appq.InsertRadioCatalogStationParams{GroupKey: key, Name: clipName(e.Name),
				SortKey: rules.Fold(e.Name), SearchText: rules.Fold(e.Name), Country: countryOf(e), Now: r.now})
			if err != nil {
				return fmt.Errorf("insert radio station: %w", err)
			}
			r.groups[key] = id
			r.created[id] = true
			r.added++
		}
		if err := r.upsert(ctx, q, id, e); err != nil {
			return err
		}
		a = syncAlias{station: id}
		r.full[id] = true
	}
	a.changed, a.ok = e.LastChangeTime, ok
	r.aliases[e.StationUUID] = a
	r.metas[a.station] = append(r.metas[a.station], entryMeta{
		uuid: e.StationUUID, name: e.Name, homepage: strings.TrimSpace(e.Homepage), favicon: strings.TrimSpace(e.Favicon),
		country: e.CountryCode, iso: e.ISO31662, state: e.State, language: e.Language, langCodes: e.LanguageCodes,
		tags: e.Tags, codec: strings.ToLower(strings.TrimSpace(e.Codec)), bitrate: e.Bitrate, votes: e.Votes,
		clicks: e.ClickCount, ok: ok,
	})
	return nil
}

// upsert writes the alias and the stream of a new or changed entry.
func (r *syncRun) upsert(ctx context.Context, q *appq.Queries, station int64, e *rbStation) error {
	if err := q.UpsertRadioAlias(ctx, appq.UpsertRadioAliasParams{Stationuuid: e.StationUUID, StationID: station, Now: r.now,
		CatalogChanged: nullText(e.LastChangeTime)}); err != nil {
		return fmt.Errorf("upsert radio alias: %w", err)
	}
	var bitrate sql.NullInt64
	if e.Bitrate > 0 {
		bitrate = sql.NullInt64{Int64: e.Bitrate, Valid: true}
	}
	if err := q.UpsertRadioCatalogStream(ctx, appq.UpsertRadioCatalogStreamParams{StationID: station,
		Stationuuid: sql.NullString{String: e.StationUUID, Valid: true}, Url: strings.TrimSpace(e.URL),
		UrlResolved: nullText(strings.TrimSpace(e.URLResolved)), Codec: nullText(strings.ToLower(strings.TrimSpace(e.Codec))),
		Bitrate: bitrate, Hls: flag01(e.HLS == 1), Lastcheckok: flag01(e.LastCheckOK == 1), Now: r.now}); err != nil {
		return fmt.Errorf("upsert radio stream: %w", err)
	}
	return nil
}

// finish marks and deletes the missing, derives the changed stations, updates the counters of the
// unchanged ones and stores the stats.
func (r *syncRun) finish(ctx context.Context) (syncStats, error) {
	if err := ctx.Err(); err != nil {
		return syncStats{}, err
	}
	w := appq.New(r.s.db.Write)
	now := sql.NullInt64{Int64: r.now, Valid: true}
	before := sql.NullInt64{Int64: r.now - removalDelay.Milliseconds(), Valid: true}
	if _, err := w.MarkRadioAliasesMissing(ctx, appq.MarkRadioAliasesMissingParams{Now: now, RunStart: r.now}); err != nil {
		return syncStats{}, fmt.Errorf("mark radio aliases missing: %w", err)
	}
	gone, err := w.DeleteMissingRadioAliases(ctx, before)
	if err != nil {
		return syncStats{}, fmt.Errorf("delete missing radio aliases: %w", err)
	}
	for _, id := range gone {
		if len(r.metas[id]) > 0 {
			r.full[id] = true
		}
	}
	if _, err := w.MarkRadioStationsMissing(ctx, now); err != nil {
		return syncStats{}, fmt.Errorf("mark radio stations missing: %w", err)
	}
	if _, err := w.ClearRadioStationsMissing(ctx, r.now); err != nil {
		return syncStats{}, fmt.Errorf("clear radio stations missing: %w", err)
	}
	removed, err := w.DeleteMissingRadioStations(ctx, before)
	if err != nil {
		return syncStats{}, fmt.Errorf("delete missing radio stations: %w", err)
	}
	if err := r.recomputeAll(ctx); err != nil {
		return syncStats{}, err
	}
	if err := r.counters(ctx); err != nil {
		return syncStats{}, err
	}
	total, err := w.CountRadioStations(ctx)
	if err != nil {
		return syncStats{}, fmt.Errorf("count radio stations: %w", err)
	}
	return syncStats{entries: r.entries, reachable: r.okCount, stations: int(total), added: r.added, removed: int(removed)}, nil
}

// recomputeAll derives every changed station again, syncBatch stations per transaction.
func (r *syncRun) recomputeAll(ctx context.Context) error {
	ids := make([]int64, 0, len(r.full))
	for id := range r.full {
		if len(r.metas[id]) > 0 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	for start := 0; start < len(ids); start += syncBatch {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.recomputeBatch(ctx, ids[start:min(len(ids), start+syncBatch)]); err != nil {
			return err
		}
		jobs.Progress(ctx, int64(r.entries+start), int64(r.entries+len(ids)), "")
	}
	return nil
}

func (r *syncRun) recomputeBatch(ctx context.Context, ids []int64) error {
	tx, err := r.s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("radio recompute: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stx := newStmtTx(tx)
	defer stx.close()
	q := appq.New(stx)
	for _, id := range ids {
		if err := r.recompute(ctx, q, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("radio recompute: commit: %w", err)
	}
	return nil
}

// recompute derives the columns, tags and facets of one station from the entries of this run, best
// stream first; locked fields keep their values.
func (r *syncRun) recompute(ctx context.Context, q *appq.Queries, id int64) error {
	locks := r.locks[id]
	var cur appq.RadioStation
	if locks["name"] || locks["region"] {
		var err error
		cur, err = q.GetRadioStation(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get radio station: %w", err)
		}
	}
	metas := slices.Clone(r.metas[id])
	var streams []appq.RadioStream
	if r.created[id] && len(metas) == 1 {
		// A new station with one entry has one stream, which is its best.
		m := metas[0]
		streams = []appq.RadioStream{{Stationuuid: sql.NullString{String: m.uuid, Valid: true},
			Codec: nullText(m.codec), Bitrate: sql.NullInt64{Int64: m.bitrate, Valid: m.bitrate > 0}}}
	} else {
		var err error
		if streams, err = q.ListRadioStreamsRanked(ctx, id); err != nil {
			return fmt.Errorf("list radio streams: %w", err)
		}
	}
	rank := make(map[string]int, len(streams))
	for i, st := range streams {
		rank[st.Stationuuid.String] = i
	}
	slices.SortStableFunc(metas, func(a, b entryMeta) int {
		ra, oka := rank[a.uuid]
		rb, okb := rank[b.uuid]
		if !oka {
			ra = len(streams)
		}
		if !okb {
			rb = len(streams)
		}
		return ra - rb
	})
	d := deriveStation(metas)
	if locks["name"] {
		d.name = cur.Name
	}
	if locks["region"] {
		d.region = cur.Region.String
	}
	if len(streams) > 0 {
		d.quality = qualityOf(streams[0].Codec.String, streams[0].Bitrate.Int64)
	}
	if err := q.UpdateRadioStationFromSync(ctx, appq.UpdateRadioStationFromSyncParams{
		Name: d.name, SortKey: rules.Fold(d.name), SearchText: searchText(d.name, d.region, d.tags),
		Country: nullText(d.country), Region: nullText(d.region), Languages: jsonList(d.languages), Homepage: nullText(d.homepage),
		FaviconUrl: nullText(d.favicon), Votes: d.votes, Clicks: d.clicks, Quality: nullText(d.quality),
		Reachable: flag01(d.reachable), Now: r.now, ID: id,
	}); err != nil {
		return fmt.Errorf("update radio station: %w", err)
	}
	fresh := r.created[id]
	if !fresh {
		if err := q.DeleteRadioStationTags(ctx, id); err != nil {
			return fmt.Errorf("delete radio tags: %w", err)
		}
	}
	if len(d.tags) > 0 {
		if err := q.InsertRadioStationTags(ctx, appq.InsertRadioStationTagsParams{StationID: id, Tags: jsonList(d.tags)}); err != nil {
			return fmt.Errorf("insert radio tags: %w", err)
		}
	}
	var open []string
	for _, facet := range facetNames {
		if !locks[facet] {
			open = append(open, facet)
		}
	}
	if err := writeFacets(ctx, q, id, deriveFacets(r.s.cat.rules, d.name, d.tags), open, false, !fresh); err != nil {
		return err
	}
	r.stations[id] = syncStation{votes: d.votes, clicks: d.clicks, reachable: d.reachable}
	return nil
}

// counters writes votes, clicks and reachability of the stations whose entries came unchanged,
// where the sum differs from the stored one.
func (r *syncRun) counters(ctx context.Context) error {
	w := appq.New(r.s.db.Write)
	for id, metas := range r.metas {
		if r.full[id] {
			continue
		}
		var c syncStation
		for _, m := range metas {
			c.votes += m.votes
			c.clicks += m.clicks
			c.reachable = c.reachable || m.ok
		}
		if c == r.stations[id] {
			continue
		}
		if err := w.UpdateRadioStationCounters(ctx, appq.UpdateRadioStationCountersParams{Votes: c.votes, Clicks: c.clicks,
			Reachable: flag01(c.reachable), Now: r.now, ID: id}); err != nil {
			return fmt.Errorf("update radio station counters: %w", err)
		}
	}
	return nil
}

// derived is a station as its entries describe it.
type derived struct {
	name, country, region, homepage, favicon, quality string
	languages, tags                                   []string
	votes, clicks                                     int64
	reachable                                         bool
}

// deriveStation reads the station from its entries, best first.
func deriveStation(metas []entryMeta) derived {
	d := derived{name: clipName(stationName(metas)), languages: []string{}, tags: []string{}}
	for _, m := range metas {
		d.votes += m.votes
		d.clicks += m.clicks
		d.reachable = d.reachable || m.ok
		if d.country == "" {
			if c := strings.ToUpper(strings.TrimSpace(m.country)); countryCode.MatchString(c) {
				d.country = c
			}
		}
		if d.homepage == "" {
			d.homepage = m.homepage
		}
		if d.favicon == "" {
			d.favicon = m.favicon
		}
		for _, l := range entryLanguages(m) {
			if !slices.Contains(d.languages, l) {
				d.languages = append(d.languages, l)
			}
		}
		for _, t := range strings.Split(m.tags, ",") {
			if t = strings.ToLower(strings.TrimSpace(t)); t != "" && utf8.RuneCountInString(t) <= 64 && !slices.Contains(d.tags, t) {
				d.tags = append(d.tags, t)
			}
		}
	}
	slices.Sort(d.tags)
	for _, m := range metas {
		if iso := strings.ToUpper(strings.TrimSpace(m.iso)); isoRegion.MatchString(iso) && (d.country == "" || strings.HasPrefix(iso, d.country+"-")) {
			d.region = iso
			break
		}
	}
	if d.region == "" && d.country == "CH" {
		for _, m := range metas {
			if r := rules.CHRegion(m.state); r != "" {
				d.region = r
				break
			}
		}
	}
	return d
}

// stationName is the name of the best entry without a bitrate or format suffix («Bayern 3» before
// «Bayern 3 HD»), else the best entry's name.
func stationName(metas []entryMeta) string {
	for _, m := range metas {
		if foldGroupName(m.name) == strings.TrimSpace(groupSpace.ReplaceAllString(rules.Fold(m.name), " ")) {
			return m.name
		}
	}
	return metas[0].name
}

// entryLanguages returns the ISO 639-1 codes of an entry: languagecodes (two-letter codes only),
// else the names of language through the language table.
func entryLanguages(m entryMeta) []string {
	var out []string
	for _, c := range strings.Split(m.langCodes, ",") {
		if c = strings.ToLower(strings.TrimSpace(c)); len(c) == 2 && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, n := range strings.Split(m.language, ",") {
		if c := rules.Language(n); c != "" && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

// qualityOf is the quality of the best stream (section 5.4): high from 256 kbit/s or FLAC, good
// from 128, basic below; "" for an unknown bitrate.
func qualityOf(codec string, kbps int64) string {
	switch {
	case strings.EqualFold(codec, "flac") || kbps >= 256:
		return model.RadioQualityHigh
	case kbps >= 128:
		return model.RadioQualityGood
	case kbps > 0:
		return model.RadioQualityBasic
	}
	return ""
}

// searchText is the folded text the search matches: name, region and tags.
func searchText(name, region string, tags []string) string {
	parts := append([]string{rules.Fold(name), strings.ToLower(region)}, tags...)
	return rules.Fold(strings.Join(parts, " "))
}

// clipName trims a name to the CHECK of radio_stations.name.
func clipName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= syncNameMax {
		return name
	}
	return strings.TrimSpace(string([]rune(name)[:syncNameMax]))
}

// countryOf returns the country of an entry, NULL when it names none.
func countryOf(e *rbStation) sql.NullString {
	c := strings.ToUpper(strings.TrimSpace(e.CountryCode))
	return sql.NullString{String: c, Valid: countryCode.MatchString(c)}
}

// writeFacets replaces the values of the named facets of a station in two statements (clear
// false: the station has none yet).
func writeFacets(ctx context.Context, q *appq.Queries, id int64, values map[string][]string, facets []string, manual, clear bool) error {
	if len(facets) == 0 {
		return nil
	}
	if clear {
		if err := q.DeleteRadioStationFacets(ctx, appq.DeleteRadioStationFacetsParams{StationID: id, Facets: jsonList(facets)}); err != nil {
			return fmt.Errorf("delete radio facets: %w", err)
		}
	}
	var pairs [][2]string
	for _, f := range facets {
		for _, v := range values[f] {
			pairs = append(pairs, [2]string{f, v})
		}
	}
	if len(pairs) == 0 {
		return nil
	}
	raw, _ := json.Marshal(pairs) // string pairs always marshal
	if err := q.InsertRadioStationFacets(ctx, appq.InsertRadioStationFacetsParams{StationID: id, Manual: flag01(manual), Pairs: string(raw)}); err != nil {
		return fmt.Errorf("insert radio facets: %w", err)
	}
	return nil
}

// jsonList is the JSON array of a string list (json_each in the queries).
func jsonList(v []string) string {
	raw, _ := json.Marshal(v) // a []string always marshals
	return string(raw)
}

// stmtTx is a sync transaction that prepares each statement once and reuses it for every entry:
// without it, parsing the SQL dominates a sync of 40 000 entries. The statements close with the
// transaction.
type stmtTx struct {
	tx    *sql.Tx
	stmts map[string]*sql.Stmt
}

func newStmtTx(tx *sql.Tx) *stmtTx { return &stmtTx{tx: tx, stmts: map[string]*sql.Stmt{}} }

// close closes the prepared statements (the end of the transaction would as well).
func (t *stmtTx) close() {
	for _, st := range t.stmts {
		_ = st.Close()
	}
}

func (t *stmtTx) stmt(ctx context.Context, query string) (*sql.Stmt, error) {
	if st, ok := t.stmts[query]; ok {
		return st, nil
	}
	st, err := t.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	t.stmts[query] = st
	return st, nil
}

func (t *stmtTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	st, err := t.stmt(ctx, query) //nolint:sqlclosecheck // cached; close closes every statement
	if err != nil {
		return nil, err
	}
	return st.ExecContext(ctx, args...)
}

func (t *stmtTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.stmt(ctx, query)
}

func (t *stmtTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	st, err := t.stmt(ctx, query) //nolint:sqlclosecheck // cached; close closes every statement
	if err != nil {
		return nil, err
	}
	return st.QueryContext(ctx, args...)
}

func (t *stmtTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	st, err := t.stmt(ctx, query) //nolint:sqlclosecheck // cached; close closes every statement
	if err != nil {
		return t.tx.QueryRowContext(ctx, query, args...) // carries the same error
	}
	return st.QueryRowContext(ctx, args...)
}

// nullText maps "" to NULL.
func nullText(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// handleCatalogSync is the handler of radio.catalog.sync: one sync, then the stats, the index and
// the logo prefetch. A failure is stored (last_sync_error, sync_failures) and asks for a retry
// through the one-off date of the schedule: 15 min, 1 h, 6 h, then the daily run.
func (s *Service) handleCatalogSync(ctx context.Context, _ *jobs.Job) error {
	stats, err := s.syncCatalog(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.recordSyncFailure(ctx, err)
		}
		return err
	}
	if err := appq.New(s.db.Write).UpdateRadioSyncState(ctx, appq.UpdateRadioSyncStateParams{
		LastSyncAt: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, EntriesTotal: int64(stats.entries),
		EntriesReachable: int64(stats.reachable), StationsTotal: int64(stats.stations), LastAdded: int64(stats.added),
		LastRemoved: int64(stats.removed), Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("update radio sync state: %w", err)
	}
	s.log.InfoContext(ctx, "radio catalog synced", "entries", stats.entries, "stations", stats.stations,
		"added", stats.added, "removed", stats.removed)
	if _, err := s.rebuildIndex(ctx); err != nil {
		return err
	}
	s.queueSyncLogos(ctx)
	return nil
}

// syncRetryDelays are the waits after the first, second and third failed sync in a row; later
// failures wait for the daily run (like the release check, admin/update.go).
var syncRetryDelays = [...]time.Duration{15 * time.Minute, time.Hour, 6 * time.Hour}

// syncErrorMax bounds the stored error text.
const syncErrorMax = 500

// recordSyncFailure stores a failed sync and sets the retry as the one-off date of the schedule.
// The error names hosts only (netguard.Clean), never a URL with a query.
func (s *Service) recordSyncFailure(ctx context.Context, cause error) {
	q := appq.New(s.db.Write)
	cur, err := q.GetRadioConfig(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "get radio config after a failed sync", "err", err)
		return
	}
	msg := cause.Error()
	if len(msg) > syncErrorMax {
		msg = msg[:syncErrorMax]
	}
	failures := cur.SyncFailures + 1
	if err := q.UpdateRadioSyncState(ctx, appq.UpdateRadioSyncStateParams{
		LastSyncAt: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, LastSyncError: nullText(strings.ToValidUTF8(msg, "")),
		SyncFailures: failures, EntriesTotal: cur.EntriesTotal, EntriesReachable: cur.EntriesReachable,
		StationsTotal: cur.StationsTotal, LastAdded: cur.LastAdded, LastRemoved: cur.LastRemoved, Now: s.now().UnixMilli(),
	}); err != nil {
		s.log.WarnContext(ctx, "store the failed radio sync", "err", err)
	}
	s.log.WarnContext(ctx, "radio catalog sync failed", "failures", failures, "err", cause)
	if failures > int64(len(syncRetryDelays)) || s.o.scheduler == nil {
		return
	}
	at := s.now().Add(syncRetryDelays[failures-1])
	cur2, err := s.o.scheduler.Schedule(ctx, JobCatalogSync)
	if err != nil {
		s.log.WarnContext(ctx, "read the radio sync schedule", "err", err)
		return
	}
	if !cur2.OnceAt.IsZero() && cur2.OnceAt.Before(at) {
		return // an earlier date stands in for the retry
	}
	local := at.In(s.o.scheduler.Location()).Format(jobs.OnceLayout)
	if _, err := s.o.scheduler.UpdateSchedule(ctx, JobCatalogSync, jobs.ScheduleUpdate{OnceLocal: &local}); err != nil {
		s.log.WarnContext(ctx, "set the radio sync retry", "err", err)
	}
}
