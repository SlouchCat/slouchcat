package radio

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The offer (concept 19 part B, B4, contract 1.28.0): the stations the app shows, instead of the
// whole catalog. It is made of the chosen countries with their mode (all: every station of the
// country, new ones included; marked: only marked ones) and the exceptions per station (marked or
// demarked), which hang at the station, not at its country. The rule of B4.1, first hit wins:
// demarked → no, marked → yes, own station → yes, country with all → yes, else no.
//
// The offer lives in memory as one immutable snapshot (offerState behind catalogState.offer),
// loaded at the first use and replaced as a whole after every apply, so a request never reads the
// tables; the index is not rebuilt for it. A draft is a list of steps the server applies to a copy
// of the stored offer: the preview answers the result without storing, the apply stores it when
// the version it is based on is still the stored one (else model.ErrRadioOfferConflict). Steps are
// relative («choose CH», «demark these stations»), so a draft computed again on a newer version
// keeps both changes.

const (
	// offerStepsMax and offerStationIDsMax are the contract's maxItems of steps and stationIds.
	offerStepsMax      = 1000
	offerStationIDsMax = 5000
	// offerQueryMax is the contract's maxLength of q.
	offerQueryMax = 100
	// offerListMax is the largest page of the preview list (contract RadioOfferListRequest.limit).
	offerListMax = 100
	// offerTagsMax is the number of raw tags a station of the preview list carries.
	offerTagsMax = 5
)

// offerState is one offer: the stored snapshot (never changed once published) or a draft (a copy).
type offerState struct {
	version    int64
	hideBroken bool
	countries  map[string]string // country → model.RadioOfferAll or model.RadioOfferMarked
	exceptions map[int64]bool    // station → offered
}

// defaultOffered is the offer of a station without an exception: an own station is in it, a
// catalog station when its country is chosen with all.
func (o *offerState) defaultOffered(e *indexEntry) bool {
	return e.own || o.countries[e.st.Country] == model.RadioOfferAll
}

// offered applies the rule of B4.1 to a station.
func (o *offerState) offered(e *indexEntry) bool {
	if v, ok := o.exceptions[e.st.ID]; ok {
		return v
	}
	return o.defaultOffered(e)
}

// clone returns a draft of o.
func (o *offerState) clone() *offerState {
	return &offerState{version: o.version, hideBroken: o.hideBroken, countries: maps.Clone(o.countries),
		exceptions: maps.Clone(o.exceptions)}
}

// setException gives a station the exception offered; one that equals the default goes.
func (o *offerState) setException(e *indexEntry, offered bool) {
	if offered == o.defaultOffered(e) {
		delete(o.exceptions, e.st.ID)
		return
	}
	o.exceptions[e.st.ID] = offered
}

// loadOffer reads the stored offer.
func loadOffer(ctx context.Context, q *appq.Queries) (*offerState, error) {
	st, err := q.GetRadioOfferState(ctx)
	if err != nil {
		return nil, fmt.Errorf("get radio offer state: %w", err)
	}
	countries, err := q.ListRadioOfferCountries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio offer countries: %w", err)
	}
	exceptions, err := q.ListRadioOfferExceptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio offer exceptions: %w", err)
	}
	o := &offerState{version: st.OfferVersion, hideBroken: st.HideBroken == 1,
		countries: make(map[string]string, len(countries)), exceptions: make(map[int64]bool, len(exceptions))}
	for _, c := range countries {
		o.countries[c.Country] = c.Mode
	}
	for _, x := range exceptions {
		o.exceptions[x.StationID] = x.Offered == 1
	}
	return o, nil
}

// offer returns the stored offer, loading it at the first call (the fixtures of other packages
// fill the tables without calling Start).
func (s *Service) offer(ctx context.Context) (*offerState, error) {
	if o := s.cat.offer.Load(); o != nil {
		return o, nil
	}
	s.cat.offerMu.Lock()
	defer s.cat.offerMu.Unlock()
	return s.offerLocked(ctx)
}

// offerLocked is offer for a caller that holds offerMu.
func (s *Service) offerLocked(ctx context.Context) (*offerState, error) {
	if o := s.cat.offer.Load(); o != nil {
		return o, nil
	}
	o, err := loadOffer(ctx, appq.New(s.db.Read))
	if err != nil {
		return nil, err
	}
	s.cat.offer.Store(o)
	return o, nil
}

// view returns the visibility of the stations under the stored offer.
func (s *Service) view(ctx context.Context) (catalogView, error) {
	o, err := s.offer(ctx)
	if err != nil {
		return catalogView{}, err
	}
	return catalogView{offer: o}, nil
}

// validateOfferSteps checks the steps of a draft against the contract; every fault is
// invalid_value on the field steps.
func validateOfferSteps(steps []model.RadioOfferStep) error {
	bad := (*model.ValidationError)(nil).Add("steps", "invalid_value")
	if len(steps) > offerStepsMax {
		return bad
	}
	for _, st := range steps {
		switch st.Op {
		case model.RadioOfferChooseCountry, model.RadioOfferRemoveCountry, model.RadioOfferMarkAll, model.RadioOfferDemarkAll:
			if !countryCode.MatchString(st.Country) {
				return bad
			}
		case model.RadioOfferMarkMatching, model.RadioOfferDemarkMatching:
			if !countryCode.MatchString(st.Country) || utf8.RuneCountInString(st.Q) > offerQueryMax {
				return bad
			}
		case model.RadioOfferMark, model.RadioOfferDemark:
			if len(st.StationIDs) > offerStationIDsMax {
				return bad
			}
		case model.RadioOfferDemarkUnreachable, model.RadioOfferSetHideBroken:
		default:
			return bad
		}
	}
	return nil
}

// offerCalc applies steps and computes summaries over one index, with the catalog stations of
// each country grouped once (own stations are never touched by the steps on a country and do not
// count in «n of m»).
type offerCalc struct {
	idx       *stationIndex
	favorites map[int64]bool // favourites of any profile
	byCountry map[string][]*indexEntry
}

func newOfferCalc(idx *stationIndex, favorites map[int64]bool) *offerCalc {
	by := map[string][]*indexEntry{}
	for _, e := range idx.entries {
		if !e.own && e.st.Country != "" {
			by[e.st.Country] = append(by[e.st.Country], e)
		}
	}
	return &offerCalc{idx: idx, favorites: favorites, byCountry: by}
}

// removable reports whether demarkUnreachable takes a station of the offer out: unreachable, not
// own and no favourite of any profile (B6, OB-6).
func (c *offerCalc) removable(o *offerState, e *indexEntry) bool {
	return e.broken && !e.own && !c.favorites[e.st.ID] && o.offered(e)
}

// apply runs the (checked) steps on the draft o in their order and returns the stations the steps
// demarkUnreachable took out.
func (c *offerCalc) apply(o *offerState, steps []model.RadioOfferStep) []int64 {
	var demarked []int64
	clearCountry := func(country string) {
		for _, e := range c.byCountry[country] {
			delete(o.exceptions, e.st.ID)
		}
	}
	for _, st := range steps {
		switch st.Op {
		case model.RadioOfferChooseCountry:
			if _, ok := o.countries[st.Country]; !ok {
				o.countries[st.Country] = model.RadioOfferAll
			}
		case model.RadioOfferRemoveCountry:
			delete(o.countries, st.Country)
			clearCountry(st.Country)
		case model.RadioOfferMarkAll, model.RadioOfferDemarkAll:
			o.countries[st.Country] = model.RadioOfferAll
			if st.Op == model.RadioOfferDemarkAll {
				o.countries[st.Country] = model.RadioOfferMarked
			}
			clearCountry(st.Country)
		case model.RadioOfferMark, model.RadioOfferDemark:
			for _, id := range st.StationIDs {
				if e := c.idx.byID[id]; e != nil {
					o.setException(e, st.Op == model.RadioOfferMark)
				}
			}
		case model.RadioOfferMarkMatching, model.RadioOfferDemarkMatching:
			q := rules.Fold(st.Q)
			for _, e := range c.byCountry[st.Country] {
				if q == "" || strings.Contains(e.search, q) {
					o.setException(e, st.Op == model.RadioOfferMarkMatching)
				}
			}
		case model.RadioOfferDemarkUnreachable:
			for _, e := range c.idx.entries {
				if c.removable(o, e) {
					o.setException(e, false)
					demarked = append(demarked, e.st.ID)
				}
			}
		case model.RadioOfferSetHideBroken:
			o.hideBroken = st.Value
		}
	}
	return demarked
}

// summary computes the totals and the countries of o, and with base the difference to it (added,
// removed); one pass over the index.
func (c *offerCalc) summary(o, base *offerState) (model.RadioOffer, int, int) {
	out := model.RadioOffer{Version: o.version, HideBroken: o.hideBroken, Countries: []model.RadioOfferCountry{}}
	t := &out.Totals
	countries := map[string]*model.RadioOfferCountry{}
	country := func(code string) *model.RadioOfferCountry {
		rc := countries[code]
		if rc == nil {
			rc = &model.RadioOfferCountry{Code: code, Continent: rules.Continent(code)}
			countries[code] = rc
		}
		return rc
	}
	var added, removed int
	for _, e := range c.idx.entries {
		in := o.offered(e)
		if base != nil {
			switch was := base.offered(e); {
			case in && !was:
				added++
			case !in && was:
				removed++
			}
		}
		if v, ok := o.exceptions[e.st.ID]; ok {
			if v {
				t.Marked++
			} else {
				t.Demarked++
			}
		}
		if !e.own && e.st.Country != "" && rules.Continent(e.st.Country) != "" {
			rc := country(e.st.Country)
			rc.Total++
			if in {
				rc.Offered++
				if e.broken {
					rc.Unreachable++
				}
			}
		}
		if !in {
			continue
		}
		t.Offered++
		if e.broken {
			t.OfferedUnreachable++
		}
		if e.own {
			t.Own++
		}
		if e.st.LogoHash != "" {
			t.WithLogo++
		}
		if c.removable(o, e) {
			t.RemovableUnreachable++
		}
	}
	for code, mode := range o.countries {
		if rules.Continent(code) != "" {
			country(code).Mode = mode
		}
	}
	t.Countries = len(o.countries)
	for _, code := range slices.Sorted(maps.Keys(countries)) {
		out.Countries = append(out.Countries, *countries[code])
	}
	return out, added, removed
}

// favoriteSet returns the stations that are a favourite of any profile.
func (s *Service) favoriteSet(ctx context.Context) (map[int64]bool, error) {
	ids, err := appq.New(s.db.Read).ListFavoriteStationIDsOfAllProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list favourite radio stations: %w", err)
	}
	return idSet(ids), nil
}

// Offer returns the stored offer (contract getRadioOffer).
func (s *Service) Offer(ctx context.Context) (model.RadioOffer, error) {
	idx, err := s.index(ctx)
	if err != nil {
		return model.RadioOffer{}, err
	}
	o, err := s.offer(ctx)
	if err != nil {
		return model.RadioOffer{}, err
	}
	favs, err := s.favoriteSet(ctx)
	if err != nil {
		return model.RadioOffer{}, err
	}
	out, _, _ := newOfferCalc(idx, favs).summary(o, nil)
	return out, nil
}

// PreviewOffer applies steps to a copy of the stored offer and answers the result without storing
// it or queueing anything (contract previewRadioOffer); list, when set, asks for one page of the
// catalog stations of a country as the draft has them.
func (s *Service) PreviewOffer(ctx context.Context, steps []model.RadioOfferStep, list *model.RadioOfferListQuery) (model.RadioOfferPreview, error) {
	if err := validateOfferSteps(steps); err != nil {
		return model.RadioOfferPreview{}, err
	}
	if list != nil && (!countryCode.MatchString(list.Country) || utf8.RuneCountInString(list.Q) > offerQueryMax ||
		list.Offset < 0 || list.Limit < 1 || list.Limit > offerListMax) {
		return model.RadioOfferPreview{}, (*model.ValidationError)(nil).Add("list", "invalid_value")
	}
	idx, err := s.index(ctx)
	if err != nil {
		return model.RadioOfferPreview{}, err
	}
	cur, err := s.offer(ctx)
	if err != nil {
		return model.RadioOfferPreview{}, err
	}
	favs, err := s.favoriteSet(ctx)
	if err != nil {
		return model.RadioOfferPreview{}, err
	}
	return s.previewOn(ctx, idx, cur, favs, steps, list)
}

// previewOn is the computation of PreviewOffer on a given index, offer and favourite set; the
// list reads the raw tags of its page.
func (s *Service) previewOn(ctx context.Context, idx *stationIndex, cur *offerState, favs map[int64]bool,
	steps []model.RadioOfferStep, list *model.RadioOfferListQuery) (model.RadioOfferPreview, error) {
	calc := newOfferCalc(idx, favs)
	draft := cur.clone()
	calc.apply(draft, steps)
	offer, added, removed := calc.summary(draft, cur)
	out := model.RadioOfferPreview{RadioOffer: offer, Added: added, Removed: removed}
	if list == nil {
		return out, nil
	}
	page := calc.page(draft, *list)
	if err := s.fillOfferTags(ctx, page); err != nil {
		return model.RadioOfferPreview{}, err
	}
	out.List = page
	return out, nil
}

// page returns one page of the catalog stations of a country under the draft o, by popularity,
// then name; q filters like the search of the lists.
func (c *offerCalc) page(o *offerState, l model.RadioOfferListQuery) *model.RadioOfferStationPage {
	q := rules.Fold(l.Q)
	var hits []*indexEntry
	for _, e := range c.byCountry[l.Country] {
		if q == "" || strings.Contains(e.search, q) {
			hits = append(hits, e)
		}
	}
	sortEntries(hits, model.RadioSortPopular, nil)
	page := &model.RadioOfferStationPage{Total: len(hits), Offset: l.Offset, Items: []model.RadioOfferStation{}}
	if l.Offset >= len(hits) {
		return page
	}
	for _, e := range hits[l.Offset:min(len(hits), l.Offset+l.Limit)] {
		page.Items = append(page.Items, model.RadioOfferStation{ID: e.st.ID, Name: e.st.Name, LogoHash: e.st.LogoHash,
			Tags: []string{}, Broken: e.broken, Offered: o.offered(e), Own: e.own})
	}
	return page
}

// fillOfferTags reads the raw tags of the stations of a page (at most offerTagsMax each).
func (s *Service) fillOfferTags(ctx context.Context, page *model.RadioOfferStationPage) error {
	if len(page.Items) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(page.Items))
	pos := make(map[int64]int, len(page.Items))
	for i, it := range page.Items {
		ids, pos[it.ID] = append(ids, it.ID), i
	}
	raw, _ := json.Marshal(ids) // an []int64 always marshals
	rows, err := appq.New(s.db.Read).ListRadioStationTagsOf(ctx, string(raw))
	if err != nil {
		return fmt.Errorf("list radio tags: %w", err)
	}
	for _, r := range rows {
		it := &page.Items[pos[r.StationID]]
		if len(it.Tags) < offerTagsMax {
			it.Tags = append(it.Tags, r.Tag)
		}
	}
	return nil
}

// ApplyOffer applies steps to the stored offer and stores the result, when version is the stored
// version (else model.ErrRadioOfferConflict; contract applyRadioOffer). After the swap the listen
// sessions on stations that left the offer end, and the logos of the stations that came in
// without one are queued, favourites first, then by popularity.
func (s *Service) ApplyOffer(ctx context.Context, version int64, steps []model.RadioOfferStep) (model.RadioOfferApplied, error) {
	if err := validateOfferSteps(steps); err != nil {
		return model.RadioOfferApplied{}, err
	}
	idx, err := s.index(ctx)
	if err != nil {
		return model.RadioOfferApplied{}, err
	}
	favs, err := s.favoriteSet(ctx)
	if err != nil {
		return model.RadioOfferApplied{}, err
	}
	s.cat.offerMu.Lock()
	cur, err := s.offerLocked(ctx)
	if err != nil {
		s.cat.offerMu.Unlock()
		return model.RadioOfferApplied{}, err
	}
	if version != cur.version {
		s.cat.offerMu.Unlock()
		return model.RadioOfferApplied{}, model.ErrRadioOfferConflict
	}
	calc := newOfferCalc(idx, favs)
	draft := cur.clone()
	demarked := calc.apply(draft, steps)
	draft.version = cur.version + 1
	if err := s.storeOffer(ctx, cur, draft); err != nil {
		s.cat.offerMu.Unlock()
		return model.RadioOfferApplied{}, err
	}
	s.cat.offer.Store(draft)
	s.cat.offerMu.Unlock()
	s.log.InfoContext(ctx, "radio offer stored", "version", draft.version, "countries", len(draft.countries),
		"exceptions", len(draft.exceptions))
	s.closeUnreachable(ctx)
	queued := s.queueOfferLogos(ctx, idx, cur, draft, favs)
	offer, _, _ := calc.summary(draft, nil)
	if demarked == nil {
		demarked = []int64{}
	}
	return model.RadioOfferApplied{Offer: offer, QueuedLogos: queued, DemarkedStationIDs: demarked}, nil
}

// storeOffer writes the difference between the stored offer and the draft in one transaction,
// with the next version only while the stored one is still cur's (another process may have
// stored meanwhile: model.ErrRadioOfferConflict).
func (s *Service) storeOffer(ctx context.Context, cur, next *offerState) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store radio offer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	now := s.now().UnixMilli()
	for c := range cur.countries {
		if _, ok := next.countries[c]; !ok {
			if err := q.DeleteRadioOfferCountry(ctx, c); err != nil {
				return fmt.Errorf("delete radio offer country: %w", err)
			}
		}
	}
	for c, mode := range next.countries {
		if cur.countries[c] != mode {
			if err := q.UpsertRadioOfferCountry(ctx, appq.UpsertRadioOfferCountryParams{Country: c, Mode: mode, Now: now}); err != nil {
				return fmt.Errorf("upsert radio offer country: %w", err)
			}
		}
	}
	for id := range cur.exceptions {
		if _, ok := next.exceptions[id]; !ok {
			if err := q.DeleteRadioOfferException(ctx, id); err != nil {
				return fmt.Errorf("delete radio offer exception: %w", err)
			}
		}
	}
	for id, v := range next.exceptions {
		if was, ok := cur.exceptions[id]; !ok || was != v {
			if err := q.UpsertRadioOfferException(ctx, appq.UpsertRadioOfferExceptionParams{StationID: id, Offered: flag01(v), Now: now}); err != nil {
				return fmt.Errorf("upsert radio offer exception: %w", err)
			}
		}
	}
	n, err := q.StoreRadioOfferVersion(ctx, appq.StoreRadioOfferVersionParams{Version: next.version,
		HideBroken: flag01(next.hideBroken), Now: now, Expected: cur.version})
	if err != nil {
		return fmt.Errorf("store radio offer version: %w", err)
	}
	if n == 0 {
		return model.ErrRadioOfferConflict
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store radio offer: commit: %w", err)
	}
	return nil
}

// logoOrder sorts stations for the logo queue: favourites of any profile first, then by
// popularity, then by id.
func logoOrder(list []*indexEntry, favs map[int64]bool) {
	slices.SortFunc(list, func(a, b *indexEntry) int {
		fa, fb := favs[a.st.ID], favs[b.st.ID]
		if fa != fb {
			if fa {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(b.popularity, a.popularity), cmp.Compare(a.st.ID, b.st.ID))
	})
}

// queueOfferLogos queues the logos of the stations the apply took into the offer that have none
// (B7.2) and returns how many it queued.
func (s *Service) queueOfferLogos(ctx context.Context, idx *stationIndex, before, after *offerState, favs map[int64]bool) int {
	var added []*indexEntry
	for _, e := range idx.entries {
		if e.st.LogoHash == "" && after.offered(e) && !before.offered(e) {
			added = append(added, e)
		}
	}
	logoOrder(added, favs)
	ids := make([]int64, 0, len(added))
	for _, e := range added {
		ids = append(ids, e.st.ID)
	}
	return s.queueLogos(ctx, ids)
}

// errOfferSeed wraps a failure of the first-start preparation.
var errOfferSeed = errors.New("prepare the radio offer")

// seedOffer prepares the offer at the first start after migration 00048 (B4.4): every station the
// administration hid (own ones too) is demarked, the filter values of every station are derived
// again from the rules file (B5), the provider of every station is filled (B7.4), then the flag
// offer_seeded is set. It writes in batches of syncBatch stations and repeats as a whole after an
// interrupted run (every write is idempotent). It queues no logo: the offer starts empty.
func (s *Service) seedOffer(ctx context.Context) error {
	q := appq.New(s.db.Read)
	st, err := q.GetRadioOfferState(ctx)
	if err != nil {
		return fmt.Errorf("%w: get offer state: %w", errOfferSeed, err)
	}
	if st.OfferSeeded == 1 {
		return nil
	}
	hidden, err := q.ListHiddenRadioStationIDs(ctx)
	if err != nil {
		return fmt.Errorf("%w: list hidden stations: %w", errOfferSeed, err)
	}
	stations, err := q.ListRadioStationsForSeed(ctx)
	if err != nil {
		return fmt.Errorf("%w: list stations: %w", errOfferSeed, err)
	}
	tags, err := q.ListAllRadioStationTags(ctx)
	if err != nil {
		return fmt.Errorf("%w: list tags: %w", errOfferSeed, err)
	}
	byStation := map[int64][]string{}
	for _, t := range tags {
		byStation[t.StationID] = append(byStation[t.StationID], t.Tag)
	}
	now := s.now().UnixMilli()
	def := rules.Default()
	hiddenSet := idSet(hidden)
	for start := 0; start < len(stations); start += syncBatch {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := s.inTx(ctx, func(q *appq.Queries) error {
			for _, r := range stations[start:min(len(stations), start+syncBatch)] {
				if hiddenSet[r.ID] {
					if err := q.UpsertRadioOfferException(ctx, appq.UpsertRadioOfferExceptionParams{StationID: r.ID, Offered: 0, Now: now}); err != nil {
						return fmt.Errorf("demark hidden station: %w", err)
					}
				}
				values, _ := deriveFacets(def, r.Name, byStation[r.ID])
				if err := writeFacets(ctx, q, r.ID, values, facetNames, false, true); err != nil {
					return err
				}
				provider := providerOf(bestURL(r.BestUrl.String, r.BestUrlResolved.String))
				if err := q.SetRadioStationProvider(ctx, appq.SetRadioStationProviderParams{Provider: nullText(provider), ID: r.ID}); err != nil {
					return fmt.Errorf("set provider: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("%w: %w", errOfferSeed, err)
		}
	}
	if err := appq.New(s.db.Write).SetRadioOfferSeeded(ctx, now); err != nil {
		return fmt.Errorf("%w: set the flag: %w", errOfferSeed, err)
	}
	s.cat.offerMu.Lock()
	s.cat.offer.Store(nil) // the demarked stations are exceptions now
	s.cat.offerMu.Unlock()
	if _, err := s.rebuildIndex(ctx); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "radio offer prepared", "stations", len(stations), "demarked", len(hidden))
	return nil
}

// inTx runs fn in one write transaction with prepared statements (stmtTx).
func (s *Service) inTx(ctx context.Context, fn func(q *appq.Queries) error) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stx := newStmtTx(tx)
	defer stx.close()
	if err := fn(appq.New(stx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
