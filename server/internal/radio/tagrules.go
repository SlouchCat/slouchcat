package radio

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The tag rules of the administration (concept 16 sections 5.4 and 11, tab «Einordnung», plan
// D-1 to D-4, migration 00039). The effective rules are the embedded defaults with the rows of
// radio_tag_rules applied per tag and facet: (tag, facet, value) maps the tag in that facet to
// value, (tag, facet, "") takes the default mapping away and (tag, ignore) only takes the tag off
// the worklist. A change applies at once to every station: one pass over all tags and derived
// facets, then only the stations whose values changed are written, never a locked facet. The
// passes over all tags run once per change (or once per view of the tab after a change, cached by
// catalogState.gen), never per station.

const (
	// worklistMinStations is the station count below which a tag stays off the worklist (D-3).
	worklistMinStations = 20
	// worklistMax caps the worklist (contract RadioTagRules.worklist maxItems).
	worklistMax = 500
	// openTagStations: a worklist tag with more stations counts as open (status strip, paper 11).
	openTagStations = 50
	// ruleTagMax is the CHECK of radio_tag_rules.tag in runes.
	ruleTagMax = 100
	// tagRulesMax is the contract's maxItems of RadioTagRulesUpdate.rules.
	tagRulesMax = 5000
	// applyRebuildAbove: an apply that changed more stations rebuilds the index instead of
	// reloading each station.
	applyRebuildAbove = syncBatch
)

// ruleSet is a set of effective tag rules.
type ruleSet struct {
	tags   rules.TagRules       // default maps with the admin rows applied
	admin  []model.RadioTagRule // the admin rows, folded, by tag and facet
	ignore map[string]bool      // tags with an ignore row
}

// newRuleSet applies the admin rows (folded and valid) to a copy of the default rules.
func newRuleSet(admin []model.RadioTagRule) *ruleSet {
	def := rules.Default()
	clone := func(m map[string]string) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return maps.Clone(m)
	}
	rs := &ruleSet{tags: rules.TagRules{Genre: clone(def.Genre), Kind: clone(def.Kind), Mood: clone(def.Mood)},
		admin: admin, ignore: map[string]bool{}}
	for _, r := range admin {
		if r.Facet == model.RadioRuleIgnore {
			rs.ignore[r.Tag] = true
			continue
		}
		m := rs.tags.ByFacet(r.Facet)
		if r.Value == "" {
			delete(m, r.Tag)
		} else {
			m[r.Tag] = r.Value
		}
	}
	return rs
}

// mapped reports whether a folded tag gives a value in any facet, as a whole or by one of its
// words (D-3).
func (rs *ruleSet) mapped(tag string) bool {
	words := tagWords(tag)
	for _, f := range facetNames {
		m := rs.tags.ByFacet(f)
		if _, ok := m[tag]; ok {
			return true
		}
		for _, w := range words {
			if _, ok := m[w]; ok {
				return true
			}
		}
	}
	return false
}

// tagRules returns the effective rules, loading the admin rows at the first call.
func (s *Service) tagRules(ctx context.Context) (*ruleSet, error) {
	if rs := s.cat.rules.Load(); rs != nil {
		return rs, nil
	}
	s.cat.rulesMu.Lock()
	defer s.cat.rulesMu.Unlock()
	if rs := s.cat.rules.Load(); rs != nil {
		return rs, nil
	}
	rows, err := appq.New(s.db.Read).ListRadioTagRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio tag rules: %w", err)
	}
	admin := make([]model.RadioTagRule, 0, len(rows))
	for _, r := range rows {
		admin = append(admin, model.RadioTagRule{Tag: r.Tag, Facet: r.Facet, Value: r.Value.String})
	}
	rs := newRuleSet(admin)
	s.cat.rules.Store(rs)
	return rs, nil
}

// validateTagRules folds and checks the administration's rules (contract setRadioTagRules) and
// returns them by tag and facet; every fault is invalid_value on the field rules.
func validateTagRules(list []model.RadioTagRule) ([]model.RadioTagRule, error) {
	bad := (*model.ValidationError)(nil).Add("rules", "invalid_value")
	if len(list) > tagRulesMax {
		return nil, bad
	}
	out := make([]model.RadioTagRule, 0, len(list))
	seen := make(map[[2]string]bool, len(list))
	for _, r := range list {
		r.Tag = rules.Fold(r.Tag)
		if n := utf8.RuneCountInString(r.Tag); n < 1 || n > ruleTagMax {
			return nil, bad
		}
		switch r.Facet {
		case model.RadioRuleIgnore:
			if r.Value != "" {
				return nil, bad
			}
		case rules.FacetGenre, rules.FacetKind, rules.FacetMood:
			if r.Value != "" && !knownFacetValue(r.Facet, r.Value) {
				return nil, bad
			}
		default:
			return nil, bad
		}
		key := [2]string{r.Tag, r.Facet}
		if seen[key] {
			return nil, bad
		}
		seen[key] = true
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b model.RadioTagRule) int {
		return cmp.Or(cmp.Compare(a.Tag, b.Tag), cmp.Compare(a.Facet, b.Facet))
	})
	return out, nil
}

// SetTagRules replaces the administration's rules (contract setRadioTagRules): the rows are stored
// in one transaction and the effective rules swapped, then the rules apply to every station at
// once, unless a sync runs: it holds syncMu and applies them before it ends (critic M-3). A failed
// apply keeps them pending; the sweep retries it. The answer is the tab after the change.
func (s *Service) SetTagRules(ctx context.Context, list []model.RadioTagRule) (model.RadioTagRules, error) {
	admin, err := validateTagRules(list)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	s.cat.rulesMu.Lock()
	err = s.storeTagRules(ctx, admin)
	if err == nil {
		s.cat.rules.Store(newRuleSet(admin))
		s.cat.rulesDirty.Store(true)
		s.cat.stationsChanged()
	}
	s.cat.rulesMu.Unlock()
	if err != nil {
		return model.RadioTagRules{}, err
	}
	s.retryTagRules(context.WithoutCancel(ctx))
	return s.TagRules(ctx)
}

// storeTagRules replaces the rows of radio_tag_rules in one transaction.
func (s *Service) storeTagRules(ctx context.Context, admin []model.RadioTagRule) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store radio tag rules: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	if err := q.DeleteRadioTagRules(ctx); err != nil {
		return fmt.Errorf("delete radio tag rules: %w", err)
	}
	now := s.now().UnixMilli()
	for _, r := range admin {
		if err := q.InsertRadioTagRule(ctx, appq.InsertRadioTagRuleParams{Tag: r.Tag, Facet: r.Facet,
			Value: nullText(r.Value), Now: now}); err != nil {
			return fmt.Errorf("insert radio tag rule: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store radio tag rules: commit: %w", err)
	}
	return nil
}

// retryTagRules applies pending rules when no sync holds syncMu (a running sync applies them
// itself before it ends).
func (s *Service) retryTagRules(ctx context.Context) {
	if !s.cat.rulesDirty.Load() || !s.cat.syncMu.TryLock() {
		return
	}
	defer s.cat.syncMu.Unlock()
	if err := s.applyTagRules(ctx, true); err != nil {
		s.log.WarnContext(ctx, "apply radio tag rules", "err", err)
	}
}

// ruleChange is the new derived facets of one station.
type ruleChange struct {
	id     int64
	values map[string][]string
}

// applyTagRules derives the facets of every station with tags under the effective rules and writes
// those whose unlocked values changed, syncBatch stations per transaction; the caller holds
// syncMu. reload puts the written stations into the index (the sync's handler rebuilds it anyway).
// An error keeps the rules pending; the batches written before it stay, so the next round has
// less to do.
func (s *Service) applyTagRules(ctx context.Context, reload bool) error {
	s.cat.rulesDirty.Store(false)
	done, err := s.applyRules(ctx)
	if err != nil {
		s.cat.rulesDirty.Store(true)
	}
	switch {
	case !reload || len(done) == 0:
	case len(done) > applyRebuildAbove:
		if _, rerr := s.rebuildIndex(ctx); rerr != nil && err == nil {
			err = rerr
		}
	default:
		s.reloadStations(ctx, done)
	}
	s.cat.stationsChanged()
	return err
}

// applyRules is the pass of applyTagRules; it returns the stations it wrote.
func (s *Service) applyRules(ctx context.Context) ([]int64, error) {
	rs, err := s.tagRules(ctx)
	if err != nil {
		return nil, err
	}
	idx, err := s.index(ctx)
	if err != nil {
		return nil, err
	}
	q := appq.New(s.db.Read)
	tags, err := allStationTags(ctx, q)
	if err != nil {
		return nil, err
	}
	derivedRows, err := q.ListDerivedRadioStationFacets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list derived radio facets: %w", err)
	}
	current := make(map[int64]map[string][]string)
	for _, f := range derivedRows {
		m := current[f.StationID]
		if m == nil {
			m = map[string][]string{}
			current[f.StationID] = m
		}
		m[f.Facet] = append(m[f.Facet], f.Value)
	}
	locks, err := allStationLocks(ctx, q)
	if err != nil {
		return nil, err
	}
	var todo []ruleChange
	for _, e := range idx.entries {
		t := tags[e.st.ID]
		if len(t) == 0 {
			continue // no tag: no rule touches the station
		}
		want, _ := deriveFacets(rs.tags, e.st.Name, t)
		for _, f := range facetNames {
			if !locks[e.st.ID][f] && !sameValues(want[f], current[e.st.ID][f]) {
				todo = append(todo, ruleChange{id: e.st.ID, values: want})
				break
			}
		}
	}
	var done []int64
	for start := 0; start < len(todo); start += syncBatch {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		batch := todo[start:min(len(todo), start+syncBatch)]
		if err := s.writeRuleBatch(ctx, batch); err != nil {
			return done, err
		}
		for _, c := range batch {
			done = append(done, c.id)
		}
	}
	return done, nil
}

// writeRuleBatch writes the derived facets of one batch in one transaction; the locks are read
// again inside it, so a facet the admin locked since the pass keeps its values.
func (s *Service) writeRuleBatch(ctx context.Context, batch []ruleChange) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("apply radio tag rules: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stx := newStmtTx(tx)
	defer stx.close()
	q := appq.New(stx)
	for _, c := range batch {
		locked, err := q.ListRadioStationLocks(ctx, c.id)
		if err != nil {
			return fmt.Errorf("list radio locks: %w", err)
		}
		var open []string
		for _, f := range facetNames {
			if !slices.Contains(locked, f) {
				open = append(open, f)
			}
		}
		if err := writeFacets(ctx, q, c.id, c.values, open, false, true); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("apply radio tag rules: commit: %w", err)
	}
	return nil
}

// sameValues reports whether two value lists hold the same values.
func sameValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// allStationTags reads the raw tags of every station, by station (one pass).
func allStationTags(ctx context.Context, q *appq.Queries) (map[int64][]string, error) {
	rows, err := q.ListAllRadioStationTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio tags: %w", err)
	}
	out := make(map[int64][]string)
	for _, r := range rows {
		out[r.StationID] = append(out[r.StationID], r.Tag)
	}
	return out, nil
}

// allStationLocks reads the locked fields of every station.
func allStationLocks(ctx context.Context, q *appq.Queries) (map[int64]map[string]bool, error) {
	rows, err := q.ListRadioStationLockFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("list radio locks: %w", err)
	}
	out := make(map[int64]map[string]bool)
	for _, l := range rows {
		m := out[l.StationID]
		if m == nil {
			m = map[string]bool{}
			out[l.StationID] = m
		}
		m[l.Field] = true
	}
	return out, nil
}

// tagRulesAnswer is the cached tab «Einordnung» of one generation.
type tagRulesAnswer struct {
	gen uint64
	out model.RadioTagRules
}

// TagRules returns the effective rules with their source and station counts, the worklist and
// the coverage (contract getRadioTagRules). The answer is cached until the next change of the
// stations, the rules or the settings (catalogState.gen).
func (s *Service) TagRules(ctx context.Context) (model.RadioTagRules, error) {
	gen := s.cat.gen.Load()
	s.cat.tagRulesMu.Lock()
	c := s.cat.tagRulesCache
	s.cat.tagRulesMu.Unlock()
	if c != nil && c.gen == gen {
		return c.out, nil
	}
	out, err := s.computeTagRules(ctx)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	s.cat.tagRulesMu.Lock()
	s.cat.tagRulesCache = &tagRulesAnswer{gen: gen, out: out}
	s.cat.tagRulesMu.Unlock()
	return out, nil
}

func (s *Service) computeTagRules(ctx context.Context) (model.RadioTagRules, error) {
	rs, err := s.tagRules(ctx)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	idx, err := s.index(ctx)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	v, _, err := s.view(ctx)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	q := appq.New(s.db.Read)
	tags, err := allStationTags(ctx, q)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	locks, err := allStationLocks(ctx, q)
	if err != nil {
		return model.RadioTagRules{}, err
	}
	// Stations per folded tag: raw tags that fold alike count a station once (m-11).
	counts := map[string]int{}
	for _, raw := range tags {
		seen := make(map[string]bool, len(raw))
		for _, t := range raw {
			if f := rules.Fold(t); f != "" && !seen[f] {
				seen[f] = true
				counts[f]++
			}
		}
	}
	out := model.RadioTagRules{Rules: tagRuleViews(rs, counts), Worklist: []model.RadioTagCount{},
		WorklistMinStations: worklistMinStations}
	for tag, n := range counts {
		if n < worklistMinStations || rs.ignore[tag] || rs.mapped(tag) {
			continue
		}
		out.Worklist = append(out.Worklist, model.RadioTagCount{Tag: tag, Stations: n})
		if n > openTagStations {
			out.OpenTags++
		}
	}
	slices.SortFunc(out.Worklist, func(a, b model.RadioTagCount) int {
		return cmp.Or(cmp.Compare(b.Stations, a.Stations), cmp.Compare(a.Tag, b.Tag))
	})
	out.Worklist = out.Worklist[:min(len(out.Worklist), worklistMax)]
	out.Coverage = coverage(idx, v, rs, tags, locks)
	return out, nil
}

// tagRuleViews lists the default rules the administration did not replace and the administration's
// rules, by tag and facet, with the stations of their tag.
func tagRuleViews(rs *ruleSet, counts map[string]int) []model.RadioTagRuleView {
	replaced := make(map[[2]string]bool, len(rs.admin))
	out := make([]model.RadioTagRuleView, 0, len(rs.admin))
	for _, r := range rs.admin {
		replaced[[2]string{r.Tag, r.Facet}] = true
		out = append(out, model.RadioTagRuleView{RadioTagRule: r, Source: model.RadioRuleAdmin, Stations: counts[r.Tag]})
	}
	def := rules.Default()
	for _, f := range facetNames {
		for tag, value := range def.ByFacet(f) {
			if replaced[[2]string{tag, f}] {
				continue
			}
			out = append(out, model.RadioTagRuleView{RadioTagRule: model.RadioTagRule{Tag: tag, Facet: f, Value: value},
				Source: model.RadioRuleDefault, Stations: counts[tag]})
		}
	}
	slices.SortFunc(out, func(a, b model.RadioTagRuleView) int {
		return cmp.Or(cmp.Compare(a.Tag, b.Tag), cmp.Compare(a.Facet, b.Facet))
	})
	return out
}

// coverage counts, over the reachable stations the app shows (not hidden, not excluded, not
// broken; D-4), those with a value per facet: genre, kind and mood as the effective rules derive
// them (a locked facet with its manual values), where the default kind «music» does not count;
// region and language from the index.
func coverage(idx *stationIndex, v catalogView, rs *ruleSet, tags map[int64][]string, locks map[int64]map[string]bool) []model.RadioCoverage {
	var total, genre, kind, mood, region, language int
	for _, e := range idx.entries {
		if !v.reachable(e) || e.broken {
			continue
		}
		total++
		if e.st.Region != "" {
			region++
		}
		if len(e.st.Languages) > 0 {
			language++
		}
		l := locks[e.st.ID]
		d, kindDefault := deriveFacets(rs.tags, e.st.Name, tags[e.st.ID])
		if (l[rules.FacetGenre] && len(e.st.Genres) > 0) || (!l[rules.FacetGenre] && len(d[rules.FacetGenre]) > 0) {
			genre++
		}
		if (l[rules.FacetKind] && len(e.st.Kinds) > 0) || (!l[rules.FacetKind] && !kindDefault) {
			kind++
		}
		if (l[rules.FacetMood] && len(e.st.Moods) > 0) || (!l[rules.FacetMood] && len(d[rules.FacetMood]) > 0) {
			mood++
		}
	}
	return []model.RadioCoverage{
		{Facet: rules.FacetGenre, Assigned: genre, Total: total},
		{Facet: rules.FacetKind, Assigned: kind, Total: total},
		{Facet: rules.FacetMood, Assigned: mood, Total: total},
		{Facet: "region", Assigned: region, Total: total},
		{Facet: "language", Assigned: language, Total: total},
	}
}
