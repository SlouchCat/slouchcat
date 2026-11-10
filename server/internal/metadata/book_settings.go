package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The settings of the audiobook sources (concept 19 D6.3 to D6.6, the page «Hörbücher: Quellen»):
// keys in the settings table under metadata.books.*, each with a default, so an absent or unreadable
// key is never an error. The pauses of the sources live under metadata.books.paused.<source>
// (book_breaker.go). Package library reads auto_match directly (finishAudiobook) and the two
// normalise switches (author_display), so the stored forms stay: "off"/"new"/"all", "0"/"1".
const (
	SettingBookAudibleRegion    = "metadata.books.audible_region"
	SettingBookAudnexusURL      = "metadata.books.audnexus_url"
	SettingBookAutoMatch        = "metadata.books.auto_match"
	SettingBookTolerance        = "metadata.books.tolerance"
	SettingBookNormalizeAuthors = audioseries.SettingNormalizeAuthors
	SettingBookNormalizeSeries  = audioseries.SettingNormalizeSeries
	// SettingBookFirstRunQueued marks that the first start with auto_match all queued every book
	// without match state (QueueFirstBookMatches).
	SettingBookFirstRunQueued = "metadata.books.first_run_queued"
)

// Values of metadata.books.auto_match.
const (
	BookAutoOff = "off"
	BookAutoNew = "new"
	BookAutoAll = "all"
)

// Defaults and limits of the book settings.
const (
	DefaultBookTolerance = 3
	MinBookTolerance     = 1
	MaxBookTolerance     = 10
	maxAudnexusURL       = 2048
)

// BookSettings are the server-wide settings of the audiobook sources.
type BookSettings struct {
	AudibleRegion    string // an AudibleRegions entry
	AudnexusURL      string // http(s) address without trailing slash
	AutoMatch        string // BookAutoOff, BookAutoNew, BookAutoAll
	Tolerance        int    // percent, MinBookTolerance..MaxBookTolerance
	NormalizeAuthors bool
	NormalizeSeries  bool
}

// DefaultBookSettings are the settings of a server without a metadata.books.* row.
func DefaultBookSettings() BookSettings {
	return BookSettings{AudibleRegion: DefaultAudibleRegion, AudnexusURL: DefaultAudnexusURL, AutoMatch: BookAutoAll,
		Tolerance: DefaultBookTolerance, NormalizeAuthors: true, NormalizeSeries: true}
}

// BookSettings reads the settings of the audiobook sources; a stored value that does not parse
// keeps its default.
func (s *Service) BookSettings(ctx context.Context) (BookSettings, error) {
	rows, err := s.read.ListSystemSettingsByPrefix(ctx, "metadata.books.%")
	if err != nil {
		return BookSettings{}, fmt.Errorf("book settings: %w", err)
	}
	st := DefaultBookSettings()
	for _, r := range rows {
		v := strings.TrimSpace(r.Value)
		switch r.Key {
		case SettingBookAudibleRegion:
			if slices.Contains(AudibleRegions, v) {
				st.AudibleRegion = v
			}
		case SettingBookAudnexusURL:
			if u, ok := normalAudnexusURL(v); ok {
				st.AudnexusURL = u
			}
		case SettingBookAutoMatch:
			if v == BookAutoOff || v == BookAutoNew || v == BookAutoAll {
				st.AutoMatch = v
			}
		case SettingBookTolerance:
			if n, err := strconv.Atoi(v); err == nil && n >= MinBookTolerance && n <= MaxBookTolerance {
				st.Tolerance = n
			}
		case SettingBookNormalizeAuthors:
			st.NormalizeAuthors = audioseries.SwitchOn(v, true)
		case SettingBookNormalizeSeries:
			st.NormalizeSeries = audioseries.SwitchOn(v, true)
		}
	}
	return st, nil
}

// normalAudnexusURL checks an Audnexus address: http or https with a host, no user info, query or
// fragment; the trailing slash goes.
func normalAudnexusURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxAudnexusURL {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return strings.TrimRight(u.String(), "/"), true
}

// BookSettingsUpdate changes the book settings; nil fields stay.
type BookSettingsUpdate struct {
	AudibleRegion    *string
	AudnexusURL      *string
	AutoMatch        *string
	Tolerance        *int
	NormalizeAuthors *bool
	NormalizeSeries  *bool
	// Extra are further rows of the settings table written in the same transaction (the encrypted
	// Google Books key of package admin); "" deletes the row.
	Extra map[string]string
}

// UpdateBookSettings validates and stores the changed book settings (*model.ValidationError with
// the contract's field names) in one transaction; an empty audnexusUrl deletes the row, so the
// default holds again. It returns the settings before and after the change.
func (s *Service) UpdateBookSettings(ctx context.Context, upd BookSettingsUpdate) (before, after BookSettings, err error) {
	var verr *model.ValidationError
	rows := map[string]string{}
	if r := upd.AudibleRegion; r != nil {
		if !slices.Contains(AudibleRegions, *r) {
			verr = verr.Add("audibleRegion", "invalid_value")
		}
		rows[SettingBookAudibleRegion] = *r
	}
	if a := upd.AudnexusURL; a != nil {
		u, ok := normalAudnexusURL(*a)
		switch {
		case strings.TrimSpace(*a) == "":
			u = "" // back to the default: the row goes
		case len(strings.TrimSpace(*a)) > maxAudnexusURL:
			verr = verr.Add("audnexusUrl", "too_long")
		case !ok:
			verr = verr.Add("audnexusUrl", "invalid_value")
		}
		rows[SettingBookAudnexusURL] = u
	}
	if m := upd.AutoMatch; m != nil {
		if *m != BookAutoOff && *m != BookAutoNew && *m != BookAutoAll {
			verr = verr.Add("autoMatch", "invalid_value")
		}
		rows[SettingBookAutoMatch] = *m
	}
	if t := upd.Tolerance; t != nil {
		if *t < MinBookTolerance || *t > MaxBookTolerance {
			verr = verr.Add("tolerancePercent", "invalid_value")
		}
		rows[SettingBookTolerance] = strconv.Itoa(*t)
	}
	flag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	if b := upd.NormalizeAuthors; b != nil {
		rows[SettingBookNormalizeAuthors] = flag(*b)
	}
	if b := upd.NormalizeSeries; b != nil {
		rows[SettingBookNormalizeSeries] = flag(*b)
	}
	if verr != nil {
		return BookSettings{}, BookSettings{}, verr
	}
	if before, err = s.BookSettings(ctx); err != nil {
		return BookSettings{}, BookSettings{}, err
	}
	for key, value := range upd.Extra {
		rows[key] = value
	}
	now := s.now().UnixMilli()
	err = s.inTxApp(ctx, func(q *systemq.Queries, app *appq.Queries) error {
		for key, value := range rows {
			if value == "" {
				if err := app.DeleteSetting(ctx, key); err != nil {
					return fmt.Errorf("delete setting %s: %w", key, err)
				}
				continue
			}
			if err := q.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{Key: key, Value: value, Now: now}); err != nil {
				return fmt.Errorf("write setting %s: %w", key, err)
			}
		}
		return nil
	})
	if err != nil {
		return BookSettings{}, BookSettings{}, err
	}
	after, err = s.BookSettings(ctx)
	return before, after, err
}

// bookSetting reads one metadata.books.* value; "" when it is not set.
func (s *Service) bookSetting(ctx context.Context, key string) (string, error) {
	v, err := s.read.GetSystemSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read setting %s: %w", key, err)
	}
	return v, nil
}
