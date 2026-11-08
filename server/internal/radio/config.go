package radio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The radio settings (concept 16 section 7.3, table radio_config). bufferMinutes, budgetMb and
// maxRelays apply to new relays only; hideBroken and countries apply at once to the index view.

// countryCode is an ISO 3166-1 alpha-2 code.
var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// parseCountries reads radio_config.countries; nil = every country.
func parseCountries(v sql.NullString) []string {
	if !v.Valid {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(v.String), &out); err != nil {
		return nil
	}
	return out
}

func configFromRow(r appq.RadioConfig) model.RadioConfig {
	return model.RadioConfig{
		BufferMinutes: int(r.BufferMinutes), BudgetMB: int(r.BudgetMb), MaxRelays: int(r.MaxRelays),
		GraceSeconds: int(r.GraceSeconds), HideBroken: r.HideBroken == 1, Countries: parseCountries(r.Countries),
		MaxRelaysByBudget: maxRelaysByBudget(int(r.BudgetMb), int(r.BufferMinutes)),
		CoverMusicBrainz:  r.CoverMusicbrainz == 1, CoverITunes: r.CoverItunes == 1,
	}
}

// view returns the visibility of the stations under the current settings.
func (s *Service) view(ctx context.Context) (catalogView, appq.RadioConfig, error) {
	row, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return catalogView{}, appq.RadioConfig{}, fmt.Errorf("get radio config: %w", err)
	}
	v := catalogView{hideBroken: row.HideBroken == 1}
	if row.Countries.Valid {
		v.countries = stringSet(parseCountries(row.Countries))
		if v.countries == nil {
			v.countries = map[string]bool{}
		}
	}
	return v, row, nil
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
// *model.ValidationError. An empty country list admits every country, like null.
func (s *Service) UpdateConfig(ctx context.Context, upd model.RadioConfigUpdate) (model.RadioConfig, error) {
	cur, err := appq.New(s.db.Read).GetRadioConfig(ctx)
	if err != nil {
		return model.RadioConfig{}, fmt.Errorf("get radio config: %w", err)
	}
	p := appq.UpdateRadioConfigParams{BufferMinutes: cur.BufferMinutes, BudgetMb: cur.BudgetMb, MaxRelays: cur.MaxRelays,
		GraceSeconds: cur.GraceSeconds, HideBroken: cur.HideBroken, Countries: cur.Countries,
		CoverMusicbrainz: cur.CoverMusicbrainz, CoverItunes: cur.CoverItunes, Now: s.now().UnixMilli()}
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
	if upd.HideBroken != nil {
		p.HideBroken = flag01(*upd.HideBroken)
	}
	if upd.CoverMusicBrainz != nil {
		p.CoverMusicbrainz = flag01(*upd.CoverMusicBrainz)
	}
	if upd.CoverITunes != nil {
		p.CoverItunes = flag01(*upd.CoverITunes)
	}
	switch {
	case upd.ClearCountries:
		p.Countries = sql.NullString{}
	case upd.Countries != nil:
		list := slices.Clone(*upd.Countries)
		for _, c := range list {
			if !countryCode.MatchString(c) {
				verr = verr.Add("countries", "invalid_value")
				break
			}
		}
		slices.Sort(list)
		list = slices.Compact(list)
		if len(list) == 0 {
			p.Countries = sql.NullString{}
		} else {
			raw, _ := json.Marshal(list) // a []string always marshals
			p.Countries = sql.NullString{String: string(raw), Valid: true}
		}
	}
	if verr != nil {
		return model.RadioConfig{}, verr
	}
	if err := appq.New(s.db.Write).UpdateRadioConfig(ctx, p); err != nil {
		return model.RadioConfig{}, fmt.Errorf("update radio config: %w", err)
	}
	s.closeUnreachable(ctx) // stations of a country now excluded
	return s.Config(ctx)
}

// flag01 maps a flag to the INTEGER 0/1 of the schema.
func flag01(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
