package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"golang.org/x/text/language"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Language and subtitle preferences of a profile (concept 13 sections 11.3 and 11.4, F-13): every
// profile reads and replaces its own on "Meine Wiedergabe" (getMyPlaybackPrefs, putMyPlaybackPrefs),
// the primary profile sets them for every profile through updateProfile. The lists hold ISO 639-1
// codes like profiles.language; the server chooses the tracks of a start from them (rule 12).

// maxPrefLanguages is the length limit of each language list (contract schema PlaybackLanguages).
const maxPrefLanguages = 10

var subtitleModes = []string{model.SubtitleModeOff, model.SubtitleModeForced, model.SubtitleModeForeign, model.SubtitleModeAlways}

// prefsOf reads the preferences of a profile row. The columns are JSON arrays (CHECK json_valid);
// anything that is not an array of strings reads as no language.
func prefsOf(p appq.Profile) model.PlaybackPrefs {
	return model.PlaybackPrefs{
		AudioLanguages: languageList(p.AudioLanguages), SubtitleMode: p.SubtitleMode,
		SubtitleLanguages: languageList(p.SubtitleLanguages),
	}
}

func languageList(column string) []string {
	var out []string
	if err := json.Unmarshal([]byte(column), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

// languageColumn encodes a validated language list for its column.
func languageColumn(langs []string) string {
	if langs == nil {
		langs = []string{}
	}
	b, _ := json.Marshal(langs) // a []string always encodes
	return string(b)
}

// iso6391 reports whether code is a two-letter ISO 639-1 code in its canonical lower-case form.
func iso6391(code string) bool {
	if len(code) != 2 {
		return false
	}
	base, err := language.ParseBase(code)
	return err == nil && base.String() == code
}

// validLanguages checks a language list for field: ISO 639-1 codes, no duplicate (invalid_value),
// at most maxPrefLanguages entries (too_long).
func validLanguages(field string, langs []string, verr *model.ValidationError) *model.ValidationError {
	if len(langs) > maxPrefLanguages {
		return verr.Add(field, "too_long")
	}
	for i, code := range langs {
		if !iso6391(code) || slices.Contains(langs[:i], code) {
			return verr.Add(field, "invalid_value")
		}
	}
	return verr
}

// applyPrefs validates the preference fields of in and writes the accepted ones into prefs.
func applyPrefs(prefs *model.PlaybackPrefs, in model.ProfileInput, verr *model.ValidationError) *model.ValidationError {
	if in.AudioLanguages != nil {
		if verr = validLanguages("audioLanguages", *in.AudioLanguages, verr); !hasField(verr, "audioLanguages") {
			prefs.AudioLanguages = slices.Clone(*in.AudioLanguages)
		}
	}
	if in.SubtitleMode != nil {
		if !slices.Contains(subtitleModes, *in.SubtitleMode) {
			verr = verr.Add("subtitleMode", "invalid_value")
		} else {
			prefs.SubtitleMode = *in.SubtitleMode
		}
	}
	if in.SubtitleLanguages != nil {
		if verr = validLanguages("subtitleLanguages", *in.SubtitleLanguages, verr); !hasField(verr, "subtitleLanguages") {
			prefs.SubtitleLanguages = slices.Clone(*in.SubtitleLanguages)
		}
	}
	return verr
}

// hasPrefs reports whether in sends any of the preference fields.
func hasPrefs(in model.ProfileInput) bool {
	return in.AudioLanguages != nil || in.SubtitleMode != nil || in.SubtitleLanguages != nil
}

// GetPlaybackPrefs returns the preferences of the session's active profile (model.ErrNotFound when
// the profile is gone).
func (s *Service) GetPlaybackPrefs(ctx context.Context, session model.Session) (model.PlaybackPrefs, error) {
	if !session.HasProfile() {
		return model.PlaybackPrefs{}, model.ErrNotFound
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: session.Profile.ID, AccountID: session.Account.ID})
	if err != nil {
		return model.PlaybackPrefs{}, notFoundOr(err, "get profile")
	}
	return prefsOf(row), nil
}

// SetPlaybackPrefs replaces all three preferences of the session's active profile. A nil list or
// an empty mode is a missing field (required); errors are a *model.ValidationError otherwise.
func (s *Service) SetPlaybackPrefs(ctx context.Context, session model.Session, prefs model.PlaybackPrefs) (model.PlaybackPrefs, error) {
	if !session.HasProfile() {
		return model.PlaybackPrefs{}, model.ErrNotFound
	}
	var verr *model.ValidationError
	var in model.ProfileInput
	if prefs.AudioLanguages == nil {
		verr = verr.Add("audioLanguages", "required")
	} else {
		in.AudioLanguages = &prefs.AudioLanguages
	}
	if prefs.SubtitleMode == "" {
		verr = verr.Add("subtitleMode", "required")
	} else {
		in.SubtitleMode = &prefs.SubtitleMode
	}
	if prefs.SubtitleLanguages == nil {
		verr = verr.Add("subtitleLanguages", "required")
	} else {
		in.SubtitleLanguages = &prefs.SubtitleLanguages
	}
	var out model.PlaybackPrefs
	if verr = applyPrefs(&out, in, verr); verr != nil {
		return model.PlaybackPrefs{}, verr
	}
	row, err := s.read.GetProfileOfAccount(ctx, appq.GetProfileOfAccountParams{ID: session.Profile.ID, AccountID: session.Account.ID})
	if err != nil {
		return model.PlaybackPrefs{}, notFoundOr(err, "get profile")
	}
	if err := s.write.SetProfilePlaybackPrefs(ctx, appq.SetProfilePlaybackPrefsParams{
		ID: row.ID, AudioLanguages: languageColumn(out.AudioLanguages), SubtitleMode: out.SubtitleMode,
		SubtitleLanguages: languageColumn(out.SubtitleLanguages), Now: s.now().UnixMilli(),
	}); err != nil {
		return model.PlaybackPrefs{}, fmt.Errorf("set playback prefs: %w", err)
	}
	return out, nil
}

// ResetSeriesTracks forgets every language the session's active profile chose per series
// (profile_series_tracks); a profile without entries is fine.
func (s *Service) ResetSeriesTracks(ctx context.Context, session model.Session) error {
	if !session.HasProfile() {
		return model.ErrNotFound
	}
	n, err := s.write.DeleteSeriesTracksByProfile(ctx, session.Profile.ID)
	if err != nil {
		return fmt.Errorf("reset series tracks: %w", err)
	}
	s.log.InfoContext(ctx, "series tracks reset", "profile_id", session.Profile.ID, "entries", n)
	return nil
}
