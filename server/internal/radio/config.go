package radio

import (
	"context"
	"fmt"
	"regexp"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The radio settings (concept 16 section 7.3, table radio_config). bufferMinutes, budgetMb and
// maxRelays apply to new relays only. Since contract 1.28.0 the countries and the switch for broken
// stations belong to the offer (offer.go).

// countryCode is an ISO 3166-1 alpha-2 code.
var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

func configFromRow(r appq.RadioConfig) model.RadioConfig {
	return model.RadioConfig{
		BufferMinutes: int(r.BufferMinutes), BudgetMB: int(r.BudgetMb), MaxRelays: int(r.MaxRelays),
		GraceSeconds:      int(r.GraceSeconds),
		MaxRelaysByBudget: maxRelaysByBudget(int(r.BudgetMb), int(r.BufferMinutes)),
		CoverMusicBrainz:  r.CoverMusicbrainz == 1, CoverITunes: r.CoverItunes == 1,
		WikidataEnabled: r.WikidataEnabled == 1,
	}
}

// Config returns the radio settings.
func (s *Service) Config(ctx context.Context) (model.RadioConfig, error) {
	row, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return model.RadioConfig{}, fmt.Errorf("get radio config: %w", err)
	}
	c := configFromRow(row)
	c.BufferMode = s.bufferMode()
	return c, nil
}

// UpdateConfig changes the settings; a value outside the contract's bounds is a
// *model.ValidationError.
func (s *Service) UpdateConfig(ctx context.Context, upd model.RadioConfigUpdate) (model.RadioConfig, error) {
	cur, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return model.RadioConfig{}, fmt.Errorf("get radio config: %w", err)
	}
	p := appq.UpdateRadioConfigParams{BufferMinutes: cur.BufferMinutes, BudgetMb: cur.BudgetMb, MaxRelays: cur.MaxRelays,
		GraceSeconds: cur.GraceSeconds, CoverMusicbrainz: cur.CoverMusicbrainz, CoverItunes: cur.CoverItunes,
		WikidataEnabled: cur.WikidataEnabled, Now: s.now().UnixMilli()}
	var verr *model.ValidationError
	setInt := func(dst *int64, v *int, field string, ok func(int) bool) {
		if v == nil {
			return
		}
		if !ok(*v) {
			verr = verr.Add(field, "out_of_range")
			return
		}
		*dst = int64(*v)
	}
	setInt(&p.BufferMinutes, upd.BufferMinutes, "bufferMinutes", func(v int) bool { return v == 10 || v == 30 || v == 60 })
	setInt(&p.BudgetMb, upd.BudgetMB, "budgetMb", func(v int) bool { return v >= 64 && v <= 4096 })
	setInt(&p.MaxRelays, upd.MaxRelays, "maxRelays", func(v int) bool { return v >= 1 && v <= 50 })
	setInt(&p.GraceSeconds, upd.GraceSeconds, "graceSeconds", func(v int) bool { return v >= 0 && v <= 1800 })
	if upd.CoverMusicBrainz != nil {
		p.CoverMusicbrainz = flag01(*upd.CoverMusicBrainz)
	}
	if upd.CoverITunes != nil {
		p.CoverItunes = flag01(*upd.CoverITunes)
	}
	if upd.WikidataEnabled != nil {
		p.WikidataEnabled = flag01(*upd.WikidataEnabled)
	}
	if verr != nil {
		return model.RadioConfig{}, verr
	}
	if err := appq.New(s.db.Write).UpdateRadioConfig(ctx, p); err != nil {
		return model.RadioConfig{}, fmt.Errorf("update radio config: %w", err)
	}
	// «Wikidata verwenden» (B7.3): off forgets every match at once, so the providers are the
	// streams' domains again; on runs the match now.
	switch {
	case p.WikidataEnabled == 0 && cur.WikidataEnabled == 1:
		if err := s.clearWikidata(ctx); err != nil {
			return model.RadioConfig{}, err
		}
	case p.WikidataEnabled == 1 && cur.WikidataEnabled == 0 && s.o.queue != nil:
		if _, err := enqueueWikidataSync(ctx, s.o.queue, wikidataPriority); err != nil {
			return model.RadioConfig{}, err
		}
	}
	return s.Config(ctx)
}

// flag01 maps a flag to the INTEGER 0/1 of the schema.
func flag01(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
