package radio

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Wikidata (concept 19 part B stage B2, B7.3, contract 1.30.0): once a week, and after an apply
// that added stations when the last run is more than a day ago, one SPARQL query lists every
// radio station of Wikidata with its website (P856), owner (P127) and founding date (P571),
// about 20 000 items and 2 MB as CSV (measured 11 November 2026: 2.5 s). The server matches the
// stations of the offer to those items locally through their homepage and stores only what fits
// a station in the offer; a second, small query reads the owners' labels in the server's language,
// else English. The owner becomes the station's provider (B7.4 point 2). Wikidata's data are CC0
// (ADR 0030); its images are not used (OB-5).
//
// Matching (decisions of 11 November 2026, docs/planning/04-entscheidungen.md): an address is its
// host without «www.» plus its path without the trailing slash, lower case, without query and
// fragment. A station takes the one item with its address; with none, the one item with its host,
// unless that host carries more than wikidataPlatformURLs different station addresses in the
// catalog (a hosting platform such as zeno.fm, whose own item would otherwise lend its owner and
// founding date to 514 stations). Two items for the address or the host are no match. An item
// with several owners keeps no owner; of several founding dates the earliest counts.
//
// Every request goes through the guarded client of the service (netguard: no local address), the
// limiter radio-wikidata (one request a second, a 429 waits for Retry-After) and the User-Agent
// Slouchcat/<version> (+https://github.com/SlouchCat/slouchcat), without contact data of the admin.

// JobWikidataSync is the weekly Wikidata match of the offer.
const JobWikidataSync = "radio.wikidata.sync"

const (
	// wikidataPriority is the rung of the catalog sync (jobs ladder rung 4).
	wikidataPriority = 4
	// wikidataEvery is the interval of the schedule.
	wikidataEvery = 7 * 24 * time.Hour
	// wikidataApplyGap is how long after a run an apply that added stations queues no new one.
	wikidataApplyGap = 24 * time.Hour
	// wikidataBodyLimit caps the answer of the station query (B7.3: 20 MB; measured 2 MB).
	wikidataBodyLimit = 20 << 20
	// wikidataLabelLimit caps the answer of one label query.
	wikidataLabelLimit = 4 << 20
	// wikidataTimeout bounds one request, body included; the query service itself stops a query
	// after 60 s.
	wikidataTimeout = 2 * time.Minute
	// wikidataLabelBatch is how many owners one label query asks for (a POST body of some 6 KB).
	wikidataLabelBatch = 500
	// wikidataPlatformURLs is how many different station addresses a host may carry in the catalog
	// and still match through its host alone (measured on CT 190: zeno.fm 514, laut.fm 538, the
	// largest real station somafm.com 54).
	wikidataPlatformURLs = 100
	// wikidataOwnerMax is the CHECK of radio_station_wikidata.owner.
	wikidataOwnerMax = 200
)

// wikidataWindow is the window of the weekly run, after the catalog sync (04:30 to 06:30).
var wikidataWindow = jobs.Window{Start: 6*60 + 30, End: 8*60 + 30}

// wikidataStationQuery lists every radio station (Q14350 and its subclasses) with a website.
// Only statements of the best rank count (wdt:, wikibase:BestRank); the precision of the founding
// date (9 year, 10 month, 11 day) decides how much of it is shown.
const wikidataStationQuery = `SELECT ?s ?site ?owner ?founded ?precision WHERE {
  ?s wdt:P31/wdt:P279* wd:Q14350 ;
     wdt:P856 ?site .
  OPTIONAL { ?s wdt:P127 ?owner . }
  OPTIONAL { ?s p:P571 ?fs . ?fs a wikibase:BestRank ; psv:P571 ?fv .
             ?fv wikibase:timeValue ?founded ; wikibase:timePrecision ?precision . }
}`

// wikidataState is the Wikidata part of the service.
type wikidataState struct {
	limiter *ratelimit.Limiter
	// endpoint is the SPARQL endpoint (wikidataEndpoint); "" = none (the radiotest build without
	// SLOUCHCAT_TEST_RADIO_WIKIDATA_URL). Tests point it at an httptest server.
	endpoint string
	// timeout is wikidataTimeout; tests shorten it.
	timeout time.Duration
}

func newWikidataState() *wikidataState {
	return &wikidataState{limiter: ratelimit.New("radio-wikidata", 1, 1), endpoint: wikidataEndpoint(), timeout: wikidataTimeout}
}

// wikidataJobType is the job type the wiring registers (RegisterTypes).
func (s *Service) wikidataJobType() jobs.Type {
	return jobs.Type{Name: JobWikidataSync, Group: jobs.GroupMetadata, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: wikidataPriority, Handler: s.handleWikidataSync, Run: s.runWikidataSync, Every: wikidataEvery,
		Seed: func(context.Context) (jobs.ScheduleSeed, error) {
			w := wikidataWindow
			return jobs.ScheduleSeed{Enabled: true, Window: &w}, nil
		}}
}

// runWikidataSync queues one run with prio (Type.Run); a waiting one is raised to prio.
func (s *Service) runWikidataSync(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := enqueueWikidataSync(ctx, q, prio)
	if err != nil {
		return nil, err
	}
	return []int64{id}, nil
}

func enqueueWikidataSync(ctx context.Context, q *jobs.Queue, prio int) (int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobWikidataSync, Priority: prio, IdempotencyKey: JobWikidataSync,
		Payload: []byte(`{}`), MaxAttempts: 3})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", JobWikidataSync, err)
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return 0, fmt.Errorf("raise %s: %w", JobWikidataSync, err)
	}
	return id, nil
}

// queueWikidataAfterApply queues a run after an apply that added stations to the offer, when the
// setting is on and the last run is more than wikidataApplyGap ago (B7.3).
func (s *Service) queueWikidataAfterApply(ctx context.Context) {
	if s.o.queue == nil {
		return
	}
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "radio config for the wikidata run", "err", err)
		return
	}
	if cfg.WikidataEnabled != 1 {
		return
	}
	if cfg.WikidataSyncedAt.Valid && s.now().Sub(time.UnixMilli(cfg.WikidataSyncedAt.Int64)) < wikidataApplyGap {
		return
	}
	if _, err := enqueueWikidataSync(ctx, s.o.queue, wikidataPriority); err != nil {
		s.log.WarnContext(ctx, "queue the wikidata run", "err", err)
	}
}

// wdItem is what the station query says about one item.
type wdItem struct {
	owners  []string // distinct owner items
	founded string   // the earliest founding date, as stored
	at      string   // its timeValue, for the comparison
}

// wdCatalog is the answer of the station query: the items by address and by host.
type wdCatalog struct {
	items  map[string]*wdItem
	byURL  map[string][]string // address key: distinct items
	byHost map[string][]string
}

// wdMatch is the match of one station.
type wdMatch struct {
	qid, ownerQID, owner, founded string
}

var (
	wdEntity  = regexp.MustCompile(`^https?://www\.wikidata\.org/entity/(Q[1-9][0-9]{0,18})$`)
	wdTime    = regexp.MustCompile(`^([0-9]{4})-([0-1][0-9])-([0-3][0-9])T`)
	wdLangTag = regexp.MustCompile(`^[a-z]{2,3}$`)
)

// wdQID returns the item id of an entity URI, "" for anything else.
func wdQID(uri string) string {
	if m := wdEntity.FindStringSubmatch(strings.TrimSpace(uri)); m != nil {
		return m[1]
	}
	return ""
}

// wdFounded turns a timeValue with its precision into YYYY, YYYY-MM or YYYY-MM-DD; "" for a
// date before year 1000, one less precise than a year or one that does not parse.
func wdFounded(value, precision string) string {
	m := wdTime.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil || m[1] < "1000" {
		return ""
	}
	p, err := strconv.Atoi(strings.TrimSpace(precision))
	switch {
	case err != nil || p < 9:
		return ""
	case p == 9 || m[2] == "00" || m[2] > "12":
		return m[1]
	case p == 10 || m[3] == "00" || m[3] > "31":
		return m[1] + "-" + m[2]
	}
	return m[1] + "-" + m[2] + "-" + m[3]
}

// wdAddress returns the host key (host without «www.», lower case) and the address key (host key
// plus the path without the trailing slash, lower case) of a website; "" when it has no host.
func wdAddress(raw string) (host, addr string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" && u.Scheme != "https" {
		return "", ""
	}
	host = strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	host = strings.TrimPrefix(host, "www.")
	if host == "" {
		return "", ""
	}
	return host, host + strings.TrimRight(strings.ToLower(u.EscapedPath()), "/")
}

// addDistinct appends v to list unless it is there.
func addDistinct(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

// parseWikidataStations reads the CSV answer of wikidataStationQuery (columns s, site, owner,
// founded, precision; one row per combination of their values).
func parseWikidataStations(r io.Reader) (*wdCatalog, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	head, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("wikidata answer: %w", err)
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF"))] = i
	}
	for _, name := range []string{"s", "site", "owner", "founded", "precision"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("wikidata answer without column %q", name)
		}
	}
	cat := &wdCatalog{items: map[string]*wdItem{}, byURL: map[string][]string{}, byHost: map[string][]string{}}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return cat, nil
		}
		if err != nil {
			return nil, fmt.Errorf("wikidata answer: %w", err)
		}
		if len(rec) != len(head) {
			continue
		}
		qid := wdQID(rec[col["s"]])
		if qid == "" {
			continue
		}
		it := cat.items[qid]
		if it == nil {
			it = &wdItem{}
			cat.items[qid] = it
		}
		if host, addr := wdAddress(rec[col["site"]]); host != "" {
			cat.byURL[addr] = addDistinct(cat.byURL[addr], qid)
			cat.byHost[host] = addDistinct(cat.byHost[host], qid)
		}
		if o := wdQID(rec[col["owner"]]); o != "" {
			it.owners = addDistinct(it.owners, o)
		}
		at := strings.TrimSpace(rec[col["founded"]])
		if f := wdFounded(at, rec[col["precision"]]); f != "" && (it.founded == "" || at < it.at) {
			it.founded, it.at = f, at
		}
	}
}

// platformHosts returns the hosts that carry more than wikidataPlatformURLs different station
// addresses in the catalog.
func platformHosts(idx *stationIndex) map[string]bool {
	addrs := map[string]map[string]bool{}
	for _, e := range idx.entries {
		host, addr := wdAddress(e.st.Homepage)
		if host == "" {
			continue
		}
		m := addrs[host]
		if m == nil {
			m = map[string]bool{}
			addrs[host] = m
		}
		m[addr] = true
	}
	out := map[string]bool{}
	for h, m := range addrs {
		if len(m) > wikidataPlatformURLs {
			out[h] = true
		}
	}
	return out
}

// match returns the item of a homepage: the one item of its address, else the one item of its
// host unless the host is a platform; "" for none or several.
func (c *wdCatalog) match(homepage string, platforms map[string]bool) string {
	host, addr := wdAddress(homepage)
	if host == "" {
		return ""
	}
	switch list := c.byURL[addr]; {
	case len(list) == 1:
		return list[0]
	case len(list) > 1:
		return ""
	}
	if platforms[host] {
		return ""
	}
	if list := c.byHost[host]; len(list) == 1 {
		return list[0]
	}
	return ""
}

// matchOffer matches every station of the offer with a homepage; the owners still lack labels.
func matchOffer(idx *stationIndex, o *offerState, cat *wdCatalog) map[int64]wdMatch {
	platforms := platformHosts(idx)
	out := map[int64]wdMatch{}
	for _, e := range idx.entries {
		if e.st.Homepage == "" || !o.offered(e) {
			continue
		}
		qid := cat.match(e.st.Homepage, platforms)
		if qid == "" {
			continue
		}
		it := cat.items[qid]
		m := wdMatch{qid: qid, founded: it.founded}
		if len(it.owners) == 1 {
			m.ownerQID = it.owners[0]
		}
		out[e.st.ID] = m
	}
	return out
}

// wikidataLanguage is the language of the owners' labels: the first metadata language of the
// server, else English.
func (s *Service) wikidataLanguage(ctx context.Context) string {
	if s.o.language == nil {
		return "en"
	}
	tag := strings.ToLower(strings.TrimSpace(s.o.language(ctx)))
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	if !wdLangTag.MatchString(tag) {
		return "en"
	}
	return tag
}

// wikidataPost sends one SPARQL query as a form POST and returns the CSV answer, at most limit
// bytes. Errors name the host only.
func (s *Service) wikidataPost(ctx context.Context, query string, limit int64) ([]byte, error) {
	endpoint := s.wd.endpoint
	if endpoint == "" {
		return nil, errors.New("radio: no wikidata endpoint")
	}
	ctx, cancel := context.WithTimeout(ctx, s.wd.timeout)
	defer cancel()
	form := url.Values{"query": {query}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form))
	if err != nil {
		return nil, fmt.Errorf("wikidata request: %w", netguard.Clean(err))
	}
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(form)), nil }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", s.coverAgent())
	resp, err := s.wd.limiter.Do(ctx, s.netClient().http, req)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return nil, fmt.Errorf("wikidata %s: %w", netguard.Site(endpoint), ctx.Err())
		}
		return nil, fmt.Errorf("wikidata %s: %w", netguard.Site(endpoint), netguard.Clean(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("wikidata %s answered %d", netguard.Site(endpoint), resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read wikidata %s: %w", netguard.Site(endpoint), netguard.Clean(err))
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("wikidata %s sent more than %d bytes", netguard.Site(endpoint), limit)
	}
	return b, nil
}

// wikidataLabels returns the labels of items in lang, else English; an item without one is left
// out.
func (s *Service) wikidataLabels(ctx context.Context, qids []string, lang string) (map[string]string, error) {
	langs := lang
	if lang != "en" {
		langs += ",en"
	}
	out := map[string]string{}
	for batch := range slices.Chunk(qids, wikidataLabelBatch) {
		var values strings.Builder
		for _, q := range batch {
			values.WriteString(" wd:" + q)
		}
		query := "SELECT ?o ?label WHERE { VALUES ?o {" + values.String() + ` }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "` + langs + `". ?o rdfs:label ?label . } }`
		b, err := s.wikidataPost(ctx, query, wikidataLabelLimit)
		if err != nil {
			return nil, err
		}
		cr := csv.NewReader(strings.NewReader(string(b)))
		if _, err := cr.Read(); err != nil {
			return nil, fmt.Errorf("wikidata labels: %w", err)
		}
		for {
			rec, err := cr.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("wikidata labels: %w", err)
			}
			if len(rec) != 2 {
				continue
			}
			q, label := wdQID(rec[0]), wdLabel(rec[1])
			// The label service answers the id itself for an item without a label.
			if q != "" && label != "" && label != q {
				out[q] = label
			}
		}
	}
	return out, nil
}

// wdLabel cleans a label: valid UTF-8, one line, at most wikidataOwnerMax runes.
func wdLabel(s string) string {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	if utf8.RuneCountInString(s) > wikidataOwnerMax {
		s = string([]rune(s)[:wikidataOwnerMax])
	}
	return s
}

// handleWikidataSync is the handler of radio.wikidata.sync. With the setting off it fetches
// nothing; with an empty offer it fetches nothing and forgets every stored match.
func (s *Service) handleWikidataSync(ctx context.Context, _ *jobs.Job) error {
	cfg, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return fmt.Errorf("get radio config: %w", err)
	}
	if cfg.WikidataEnabled != 1 {
		s.log.InfoContext(ctx, "radio wikidata run skipped, the setting is off")
		return nil
	}
	idx, err := s.index(ctx)
	if err != nil {
		return err
	}
	o, err := s.offer(ctx)
	if err != nil {
		return err
	}
	matches := map[int64]wdMatch{}
	if slices.ContainsFunc(idx.entries, func(e *indexEntry) bool { return e.st.Homepage != "" && o.offered(e) }) {
		b, err := s.wikidataPost(ctx, wikidataStationQuery, wikidataBodyLimit)
		if err != nil {
			return err
		}
		jobs.Progress(ctx, 1, 3, "")
		cat, err := parseWikidataStations(strings.NewReader(string(b)))
		if err != nil {
			return err
		}
		matches = matchOffer(idx, o, cat)
		var owners []string
		for _, m := range matches {
			if m.ownerQID != "" {
				owners = addDistinct(owners, m.ownerQID)
			}
		}
		slices.Sort(owners)
		labels, err := s.wikidataLabels(ctx, owners, s.wikidataLanguage(ctx))
		if err != nil {
			return err
		}
		for id, m := range matches {
			m.owner = labels[m.ownerQID]
			matches[id] = m
		}
		jobs.Progress(ctx, 2, 3, "")
		s.log.InfoContext(ctx, "radio wikidata answer read", "items", len(cat.items), "matched", len(matches),
			"owners", len(owners), "bytes", len(b))
	}
	changed, err := s.storeWikidata(ctx, matches)
	if err != nil {
		return err
	}
	s.onStationsChanged(ctx, changed)
	// A station may have left the offer while the run read Wikidata.
	s.dropWikidataOutsideOffer(ctx)
	return nil
}

// storeWikidata replaces the stored matches with matches in one transaction and returns the
// stations whose row came, changed or went; an unchanged row keeps its fetched_at (the time it
// was found). Nothing is written when the setting went off meanwhile.
func (s *Service) storeWikidata(ctx context.Context, matches map[int64]wdMatch) ([]int64, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store radio wikidata: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	cfg, err := q.GetRadioConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get radio config: %w", err)
	}
	if cfg.WikidataEnabled != 1 {
		return nil, nil
	}
	rows, err := q.ListRadioStationWikidata(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio wikidata: %w", err)
	}
	now := s.now().UnixMilli()
	var changed []int64
	stored := make(map[int64]wdMatch, len(rows))
	for _, r := range rows {
		stored[r.StationID] = wdMatch{qid: r.Qid, ownerQID: r.OwnerQid.String, owner: r.Owner.String, founded: r.Founded.String}
		if _, ok := matches[r.StationID]; !ok {
			if err := q.DeleteRadioStationWikidata(ctx, r.StationID); err != nil {
				return nil, fmt.Errorf("delete radio wikidata: %w", err)
			}
			changed = append(changed, r.StationID)
		}
	}
	ids := make([]int64, 0, len(matches))
	for id := range matches {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		m := matches[id]
		if was, ok := stored[id]; ok && was == m {
			continue // unchanged: no write, no reload of the index entry
		}
		err := q.UpsertRadioStationWikidata(ctx, appq.UpsertRadioStationWikidataParams{StationID: id, Qid: m.qid,
			OwnerQid: nullText(m.ownerQID), Owner: nullText(m.owner), Founded: nullText(m.founded), Now: now})
		if err != nil {
			// The station may have gone since the index was read (ON DELETE CASCADE has nothing to do).
			if _, gerr := q.GetRadioStation(ctx, id); errors.Is(gerr, sql.ErrNoRows) {
				continue
			}
			return nil, fmt.Errorf("upsert radio wikidata: %w", err)
		}
		changed = append(changed, id)
	}
	if err := q.SetRadioWikidataSynced(ctx, sql.NullInt64{Int64: now, Valid: true}); err != nil {
		return nil, fmt.Errorf("set radio wikidata synced: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store radio wikidata: commit: %w", err)
	}
	return changed, nil
}

// dropWikidataOutsideOffer forgets the Wikidata matches of the stations outside the offer (B7.1:
// details exist only for stations in the offer). It runs at the start, after an apply, after a
// catalog sync and after a run; holding offerMu keeps an apply from adding a station meanwhile.
func (s *Service) dropWikidataOutsideOffer(ctx context.Context) {
	s.cat.offerMu.Lock()
	defer s.cat.offerMu.Unlock()
	ids, err := appq.New(s.db.Read).ListRadioStationWikidataIDs(ctx)
	if err != nil || len(ids) == 0 {
		if err != nil {
			s.log.WarnContext(ctx, "list radio wikidata", "err", err)
		}
		return
	}
	idx, err := s.index(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "drop radio wikidata outside the offer", "err", err)
		return
	}
	o, err := s.offerLocked(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "drop radio wikidata outside the offer", "err", err)
		return
	}
	q := appq.New(s.db.Write)
	var dropped []int64
	for _, id := range ids {
		if e := idx.byID[id]; e != nil && o.offered(e) {
			continue
		}
		if err := q.DeleteRadioStationWikidata(ctx, id); err != nil {
			s.log.WarnContext(ctx, "delete radio wikidata", "station_id", id, "err", err)
			continue
		}
		dropped = append(dropped, id)
	}
	s.onStationsChanged(ctx, dropped)
}

// clearWikidata forgets every stored match (the setting went off).
func (s *Service) clearWikidata(ctx context.Context) error {
	ids, err := appq.New(s.db.Read).ListRadioStationWikidataIDs(ctx)
	if err != nil {
		return fmt.Errorf("list radio wikidata: %w", err)
	}
	if _, err := appq.New(s.db.Write).DeleteAllRadioStationWikidata(ctx); err != nil {
		return fmt.Errorf("delete radio wikidata: %w", err)
	}
	s.onStationsChanged(ctx, ids)
	return nil
}

// stationWikidata returns the stored match of a station; nil without one.
func stationWikidata(ctx context.Context, q *appq.Queries, id int64) (*model.RadioWikidata, error) {
	row, err := q.GetRadioStationWikidata(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get radio wikidata: %w", err)
	}
	return &model.RadioWikidata{QID: row.Qid, OwnerQID: row.OwnerQid.String, Owner: row.Owner.String,
		Founded: row.Founded.String, FetchedAt: time.UnixMilli(row.FetchedAt).UTC()}, nil
}
