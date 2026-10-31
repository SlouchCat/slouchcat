package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Metadata settings (concept 10 section 4.1, ADR 0018, decision 16): twelve keys in the settings
// table under metadata.*, read through Settings with a default for every missing key, so an
// absent key is never an error. The server default is the effective setting of every library
// until step 3 adds the deviation per library (library_metadata, Effective).
//
// Language codes are BCP-47 with an optional region (de, en, pt-BR, zh-TW; E-10) and are checked
// against the static list in languages.go, never fetched. Every effective text language list
// holds de or en (E-9), the server's list holds de (F-9).
const (
	SettingTextLangs        = "metadata.text_langs"         // JSON list of BCP-47 codes, ordered
	SettingImageLangsPrefix = "metadata.image_langs."       // + kind: JSON list of codes and "" (no text)
	SettingImageKinds       = "metadata.image_kinds"        // JSON list of logo, season_poster, episode_still
	SettingImageMinPrefix   = "metadata.image_min_width."   // + poster|backdrop: number, 0 = any
	SettingImageByVote      = "metadata.image_by_vote"      // "1"/"0"
	SettingLocalImages      = "metadata.local_images"       // "1"/"0"
	SettingRatingCountry    = "metadata.rating_country"     // DE, AT, CH, GB, US
	SettingTMDBIncludeAdult = "metadata.tmdb_include_adult" // "1"/"0"
	SettingFanartEnabled    = "metadata.fanart_enabled"     // "1"/"0"

	settingsPrefix = "metadata."
)

// Image kinds the administrator may switch off (poster and backdrop are mandatory, E-1) and the
// kinds with a language chain and a minimum width.
const (
	ImageKindLogo         = "logo"
	ImageKindSeasonPoster = "season_poster"
	ImageKindEpisodeStill = "episode_still"
)

// SwitchableImageKinds are the values of metadata.image_kinds in the order of the interface.
var SwitchableImageKinds = []string{ImageKindLogo, ImageKindSeasonPoster, ImageKindEpisodeStill}

// ChainImageKinds are the image kinds with a language chain (metadata.image_langs.<kind>).
var ChainImageKinds = []string{"poster", "backdrop", "logo"}

// MinWidthImageKinds are the image kinds with a minimum width (metadata.image_min_width.<kind>).
var MinWidthImageKinds = []string{"poster", "backdrop"}

// Settings are the resolved metadata settings: the server default, or the server default with
// the deviation of one library applied (Effective).
type Settings struct {
	// TextLangs is the ordered list of the languages every provider delivers for every title;
	// the position is the rank written to item_texts.rank.
	TextLangs []string
	// ImageLangs is the fallback chain per image kind (poster, backdrop, logo); "" = without
	// text.
	ImageLangs map[string][]string
	// ImageKinds are the switchable kinds that are on (logo, season_poster, episode_still).
	ImageKinds []string
	// ImageMinWidth is the minimum width per kind (poster, backdrop); 0 = any. A candidate below
	// it only wins when no candidate reaches it (6.2).
	ImageMinWidth map[string]int
	// ImageByVote picks the best-voted image within a language instead of the first.
	ImageByVote bool
	// LocalImages reads the image files next to the media as the first image source (F-5).
	LocalImages bool
	// RatingCountry is the country whose certification is read first (F-7, decision 6).
	RatingCountry string
	// TMDBIncludeAdult sends include_adult with TMDB searches.
	TMDBIncludeAdult bool
	// FanartEnabled asks Fanart.tv for images after the folder's list (F-4, step 6).
	FanartEnabled bool
}

// DefaultSettings are the settings of a server without a single metadata.* row: today's
// behaviour, except for the poster chain, which prefers German posters (F-10).
func DefaultSettings() Settings {
	return Settings{
		TextLangs: []string{"de", "en"},
		ImageLangs: map[string][]string{
			"poster":   {"de", "en", ""},
			"backdrop": {"", "de", "en"},
			"logo":     {"de", "en"},
		},
		ImageKinds:       slices.Clone(SwitchableImageKinds),
		ImageMinWidth:    map[string]int{"poster": 500, "backdrop": 1280},
		ImageByVote:      true,
		LocalImages:      true,
		RatingCountry:    "DE",
		TMDBIncludeAdult: false,
		FanartEnabled:    false,
	}
}

// HasImageKind reports whether a switchable kind is on.
func (s Settings) HasImageKind(kind string) bool { return slices.Contains(s.ImageKinds, kind) }

// Rank returns the position of lang in the text language list, or len(TextLangs) for a
// language not in it (so such a row sorts behind every listed language).
func (s Settings) Rank(lang string) int {
	if i := slices.Index(s.TextLangs, lang); i >= 0 {
		return i
	}
	return len(s.TextLangs)
}

// settingRows reads every metadata.* row.
type settingRows map[string]string

// parseSettings applies the stored rows to the defaults; a row that does not parse keeps the
// default (and is reported in err, which the reader logs once).
func parseSettings(rows settingRows) (Settings, error) {
	s := DefaultSettings()
	var errs []error
	list := func(key string, dst *[]string) {
		raw, ok := rows[key]
		if !ok {
			return
		}
		var v []string
		if err := json.Unmarshal([]byte(raw), &v); err != nil || v == nil {
			errs = append(errs, fmt.Errorf("%s: not a JSON list", key))
			return
		}
		*dst = v
	}
	flag := func(key string, dst *bool) {
		raw, ok := rows[key]
		if !ok {
			return
		}
		switch raw {
		case "1":
			*dst = true
		case "0":
			*dst = false
		default:
			errs = append(errs, fmt.Errorf("%s: not 0 or 1", key))
		}
	}
	list(SettingTextLangs, &s.TextLangs)
	for _, k := range ChainImageKinds {
		chain := s.ImageLangs[k]
		list(SettingImageLangsPrefix+k, &chain)
		s.ImageLangs[k] = chain
	}
	list(SettingImageKinds, &s.ImageKinds)
	for _, k := range MinWidthImageKinds {
		raw, ok := rows[SettingImageMinPrefix+k]
		if !ok {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s: not a number", SettingImageMinPrefix+k))
			continue
		}
		s.ImageMinWidth[k] = n
	}
	flag(SettingImageByVote, &s.ImageByVote)
	flag(SettingLocalImages, &s.LocalImages)
	flag(SettingTMDBIncludeAdult, &s.TMDBIncludeAdult)
	flag(SettingFanartEnabled, &s.FanartEnabled)
	if raw, ok := rows[SettingRatingCountry]; ok {
		if _, known := fskTables[raw]; known {
			s.RatingCountry = raw
		} else {
			errs = append(errs, fmt.Errorf("%s: unknown country %q", SettingRatingCountry, raw))
		}
	}
	if verr := s.Validate(); verr != nil {
		// A stored state that no longer validates (a language removed from the static list) falls
		// back to the defaults for the texts; images keep what parses.
		errs = append(errs, verr)
		d := DefaultSettings()
		s.TextLangs = d.TextLangs
	}
	return s, errors.Join(errs...)
}

// rows turns the settings into their stored form (the twelve keys).
func (s Settings) rows() settingRows {
	out := settingRows{}
	js := func(v any) string {
		raw, _ := json.Marshal(v)
		return string(raw)
	}
	flag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	out[SettingTextLangs] = js(s.TextLangs)
	for _, k := range ChainImageKinds {
		out[SettingImageLangsPrefix+k] = js(nonNil(s.ImageLangs[k]))
	}
	out[SettingImageKinds] = js(nonNil(s.ImageKinds))
	for _, k := range MinWidthImageKinds {
		out[SettingImageMinPrefix+k] = strconv.Itoa(s.ImageMinWidth[k])
	}
	out[SettingImageByVote] = flag(s.ImageByVote)
	out[SettingLocalImages] = flag(s.LocalImages)
	out[SettingRatingCountry] = s.RatingCountry
	out[SettingTMDBIncludeAdult] = flag(s.TMDBIncludeAdult)
	out[SettingFanartEnabled] = flag(s.FanartEnabled)
	return out
}

// Validate checks the server settings (concept 9.5): languages known, the list not empty and
// holding de (F-9: the server's list must hold de; a library's list de or en, see
// ValidateTextLangs), chains without duplicates, widths not negative, the country known.
// Field names follow the contract (MetadataSettings).
func (s Settings) Validate() *model.ValidationError {
	verr := ValidateTextLangs("textLangs", s.TextLangs, true)
	for _, k := range ChainImageKinds {
		verr = validateChain("imageLangs."+k, s.ImageLangs[k], verr)
	}
	for _, k := range s.ImageKinds {
		if !slices.Contains(SwitchableImageKinds, k) {
			verr = verr.Add("imageKinds", "invalid_value")
			break
		}
	}
	for _, k := range MinWidthImageKinds {
		if s.ImageMinWidth[k] < 0 {
			verr = verr.Add("imageMinWidth."+k, "invalid_value")
		}
	}
	if _, ok := fskTables[s.RatingCountry]; !ok {
		verr = verr.Add("ratingCountry", "country_unknown")
	}
	return verr
}

// ValidateTextLangs checks a text language list: not empty (langs_empty), every code known
// (lang_unknown), no duplicates (chain_duplicate), and de in it when server is set, else de or
// en (ui_lang_required; E-9, F-9).
func ValidateTextLangs(field string, langs []string, server bool) *model.ValidationError {
	var verr *model.ValidationError
	if len(langs) == 0 {
		return verr.Add(field, "langs_empty")
	}
	seen := map[string]bool{}
	for _, l := range langs {
		if !KnownLanguage(l) {
			verr = verr.Add(field, "lang_unknown")
			break
		}
		if seen[l] {
			verr = verr.Add(field, "chain_duplicate")
			break
		}
		seen[l] = true
	}
	switch {
	case server && !slices.Contains(langs, "de"):
		verr = verr.Add(field, "ui_lang_required")
	case !server && !slices.Contains(langs, "de") && !slices.Contains(langs, "en"):
		verr = verr.Add(field, "ui_lang_required")
	}
	return verr
}

// validateChain checks an image language chain: codes known or "", no duplicates.
func validateChain(field string, chain []string, verr *model.ValidationError) *model.ValidationError {
	seen := map[string]bool{}
	for _, l := range chain {
		if l != "" && !KnownLanguage(l) {
			return verr.Add(field, "lang_unknown")
		}
		if seen[l] {
			return verr.Add(field, "chain_duplicate")
		}
		seen[l] = true
	}
	return verr
}

// ImageLanguages is the union of every chain without "" (TMDB's include_image_language).
func (s Settings) ImageLanguages() []string {
	var out []string
	for _, k := range ChainImageKinds {
		for _, l := range s.ImageLangs[k] {
			if l != "" && !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	return out
}

// Settings reads the server's metadata settings (the defaults for every missing key). A stored
// value that does not parse is logged and keeps its default.
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	rows, err := s.read.ListSystemSettingsByPrefix(ctx, settingsPrefix+"%")
	if err != nil {
		return Settings{}, fmt.Errorf("metadata settings: %w", err)
	}
	stored := settingRows{}
	for _, r := range rows {
		stored[r.Key] = r.Value
	}
	out, perr := parseSettings(stored)
	if perr != nil {
		s.log.Warn("metadata settings: stored value ignored, default used", "err", perr)
	}
	return out, nil
}

// LibraryDeviation is what a library overrides (concept 10 4.5): nil = like the server.
type LibraryDeviation struct {
	TextLangs *[]string
	Images    *LibraryImages
}

// LibraryImages is the image part of a library's deviation: the chains per kind and the
// switchable kinds that are on.
type LibraryImages struct {
	Langs map[string][]string `json:"langs"`
	Kinds []string            `json:"kinds"`
}

// apply returns the server settings with the deviation applied.
func (d LibraryDeviation) apply(st Settings) Settings {
	out := st
	if d.TextLangs != nil {
		out.TextLangs = slices.Clone(*d.TextLangs)
	}
	if d.Images != nil {
		out.ImageLangs = make(map[string][]string, len(ChainImageKinds))
		for _, k := range ChainImageKinds {
			chain := d.Images.Langs[k]
			if chain == nil {
				chain = st.ImageLangs[k]
			}
			out.ImageLangs[k] = slices.Clone(chain)
		}
		out.ImageKinds = slices.Clone(nonNil(d.Images.Kinds))
	}
	return out
}

// Validate checks a deviation: a library's list holds de or en (E-9), chains and kinds as the
// server's (concept 9.5).
func (d LibraryDeviation) Validate() *model.ValidationError {
	var verr *model.ValidationError
	if d.TextLangs != nil {
		verr = ValidateTextLangs("textLangs", *d.TextLangs, false)
	}
	if d.Images != nil {
		for _, k := range ChainImageKinds {
			if chain, ok := d.Images.Langs[k]; ok {
				verr = validateChain("images.langs."+k, chain, verr)
			}
		}
		for k := range d.Images.Langs {
			if !slices.Contains(ChainImageKinds, k) {
				verr = verr.Add("images.langs", "invalid_value")
				break
			}
		}
		for _, k := range d.Images.Kinds {
			if !slices.Contains(SwitchableImageKinds, k) {
				verr = verr.Add("images.kinds", "invalid_value")
				break
			}
		}
	}
	return verr
}

// deviationOf decodes a library_metadata row.
func deviationOf(row systemq.LibraryMetadatum) (LibraryDeviation, error) {
	var d LibraryDeviation
	if row.TextLangs.Valid {
		var langs []string
		if err := json.Unmarshal([]byte(row.TextLangs.String), &langs); err != nil {
			return d, fmt.Errorf("library %d text_langs: %w", row.LibraryID, err)
		}
		d.TextLangs = &langs
	}
	if row.Images.Valid {
		var imgs LibraryImages
		if err := json.Unmarshal([]byte(row.Images.String), &imgs); err != nil {
			return d, fmt.Errorf("library %d images: %w", row.LibraryID, err)
		}
		d.Images = &imgs
	}
	return d, nil
}

// LibraryDeviation reads the deviation of a library (empty when it has no row).
func (s *Service) LibraryDeviation(ctx context.Context, libraryID int64) (LibraryDeviation, error) {
	row, err := s.read.GetLibraryMetadata(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return LibraryDeviation{}, nil
	}
	if err != nil {
		return LibraryDeviation{}, fmt.Errorf("library metadata %d: %w", libraryID, err)
	}
	return deviationOf(row)
}

// UpdateLibraryDeviation stores (or, when everything is nil, removes) the deviation of a library.
func (s *Service) UpdateLibraryDeviation(ctx context.Context, libraryID int64, d LibraryDeviation) error {
	if verr := d.Validate(); verr != nil {
		return verr
	}
	if d.TextLangs == nil && d.Images == nil {
		if err := s.write.DeleteLibraryMetadata(ctx, libraryID); err != nil {
			return fmt.Errorf("delete library metadata %d: %w", libraryID, err)
		}
		return nil
	}
	p := systemq.UpsertLibraryMetadataParams{LibraryID: libraryID, Now: s.now().UnixMilli()}
	if d.TextLangs != nil {
		raw, _ := json.Marshal(*d.TextLangs)
		p.TextLangs = sql.NullString{String: string(raw), Valid: true}
	}
	if d.Images != nil {
		imgs := *d.Images
		imgs.Kinds = nonNil(imgs.Kinds)
		if imgs.Langs == nil {
			imgs.Langs = map[string][]string{}
		}
		raw, _ := json.Marshal(imgs)
		p.Images = sql.NullString{String: string(raw), Valid: true}
	}
	if err := s.write.UpsertLibraryMetadata(ctx, p); err != nil {
		return fmt.Errorf("write library metadata %d: %w", libraryID, err)
	}
	return nil
}

// UpdateSettings validates and stores the server's metadata settings (all twelve keys at once);
// it queues nothing (F-8).
func (s *Service) UpdateSettings(ctx context.Context, st Settings) error {
	if verr := st.Validate(); verr != nil {
		return verr
	}
	now := s.now().UnixMilli()
	for key, value := range st.rows() {
		if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{Key: key, Value: value, Now: now}); err != nil {
			return fmt.Errorf("write setting %s: %w", key, err)
		}
	}
	return nil
}

// Effective resolves the settings a title of library libraryID is filled with: the server
// default with the library's deviation (library_metadata, concept 10 4.5) applied. libraryID 0
// (a title without library, as in some tests) is the server default.
func (s *Service) Effective(ctx context.Context, libraryID int64) (Settings, error) {
	st, err := s.Settings(ctx)
	if err != nil || libraryID == 0 {
		return st, err
	}
	d, err := s.LibraryDeviation(ctx, libraryID)
	if err != nil {
		s.log.Warn("library metadata ignored", "library_id", libraryID, "err", err)
		return st, nil
	}
	return d.apply(st), nil
}

// textLangsOf returns the effective text language list of an item (its library's).
func (s *Service) textLangsOf(ctx context.Context, item systemq.Item) ([]string, error) {
	st, err := s.Effective(ctx, item.LibraryID)
	if err != nil {
		return nil, err
	}
	return st.TextLangs, nil
}

// AvailableMetadataLanguages is the union of every effective text language list (the server's and
// every library's), the server's first: the languages a profile may choose (4.3).
func (s *Service) AvailableMetadataLanguages(ctx context.Context) ([]string, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(st.TextLangs)
	rows, err := s.read.ListLibraryMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("list library metadata: %w", err)
	}
	for _, r := range rows {
		d, err := deviationOf(r)
		if err != nil || d.TextLangs == nil {
			continue
		}
		for _, l := range *d.TextLangs {
			if !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	return out, nil
}

// ProviderLanguages names, per provider, the languages of the server's list it delivers (for
// listMetadataProviders; a language of the list missing at a provider is skipped there).
func (s *Service) ProviderLanguages(ctx context.Context) (map[string][]string, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{
		model.ProviderTMDB:    {},
		model.ProviderTVDB:    {},
		model.ProviderAniList: {},
		model.ProviderLocal:   {},
	}
	for _, l := range st.TextLangs {
		out[model.ProviderTMDB] = append(out[model.ProviderTMDB], l)
		if _, ok := s.tvdb.langCode(ctx, l); ok {
			out[model.ProviderTVDB] = append(out[model.ProviderTVDB], l)
		}
		if l == "en" {
			out[model.ProviderAniList] = append(out[model.ProviderAniList], l)
		}
	}
	return out, nil
}

// PrimaryLanguage returns the primary language subtag of a BCP-47 code (pt-BR -> pt), for the
// providers and tables that know no region (trailers.lang, TVDB, Fanart.tv).
func PrimaryLanguage(code string) string {
	if i := strings.IndexByte(code, '-'); i > 0 {
		return code[:i]
	}
	return code
}

// primaryLanguages returns the distinct primary subtags of langs in order.
func primaryLanguages(langs []string) []string {
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		if p := PrimaryLanguage(l); !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}
