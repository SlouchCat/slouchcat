package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The metadata editor (concept 15 7.4, 7.5, P-6; contract 1.15.0): the administrator writes texts
// in the server's metadata language and the title columns, and every field he writes is locked so
// metadata.match and metadata.fill keep it. Text fields are locked per language (title:de,
// overview:de, tagline:de, sort_title:de), the other fields by their column name as before.

const (
	maxMetaText     = 500   // title, original title, sort title, tagline (runes)
	maxMetaOverview = 10000 // runes
	maxMetaGenres   = 50
	maxMetaGenre    = 64
	minMetaYear     = 1800
	maxMetaYear     = 2200
)

// editableKinds are the kinds the editor writes; titleOnly fields exist for movies and series.
var editableKinds = []string{"movie", "series", "season", "episode"}

// metaFieldInfo maps an editor field onto its lock and its provenance row.
type metaFieldInfo struct {
	lock      string // the lock name; for a text field the field part of field:lang
	text      bool   // locked per language (and by the old lock without language)
	legacy    bool   // the old lock without language counts too
	source    string // item_field_sources.field; "" for none
	titleOnly bool   // movies and series only
}

var metaFields = map[string]metaFieldInfo{
	model.MetaFieldTitle:         {lock: "title", text: true, legacy: true, source: "title"},
	model.MetaFieldOriginalTitle: {lock: "original_title", source: "original_title", titleOnly: true},
	model.MetaFieldSortTitle:     {lock: "sort_title", text: true},
	model.MetaFieldYear:          {lock: "year"},
	model.MetaFieldReleaseDate:   {lock: "release_date", source: "release_date"},
	model.MetaFieldTagline:       {lock: "tagline", text: true, legacy: true, source: "tagline", titleOnly: true},
	model.MetaFieldOverview:      {lock: "overview", text: true, legacy: true, source: "overview"},
	model.MetaFieldGenres:        {lock: "genres", source: "genres", titleOnly: true},
}

// lockName is the lock of an editor field in lang.
func (f metaFieldInfo) lockName(lang string) string {
	if f.text {
		return model.TextLockName(f.lock, lang)
	}
	return f.lock
}

// lockedIn reports whether the field is locked in lang.
func (f metaFieldInfo) lockedIn(locks []string, lang string) bool {
	return slices.Contains(locks, f.lockName(lang)) || (f.legacy && slices.Contains(locks, f.lock))
}

// serverTextLang is the server's metadata language: the first of its text languages (F-10); de
// without one (the server's list always holds de, F-9).
func (s *Service) serverTextLang(ctx context.Context) (string, error) {
	st, err := s.meta.MetadataSettings(ctx)
	if err != nil {
		return "", fmt.Errorf("metadata settings: %w", err)
	}
	if len(st.TextLangs) == 0 {
		return "de", nil
	}
	return st.TextLangs[0], nil
}

// itemMetadata builds the editor's view of a title in lang.
func (s *Service) itemMetadata(ctx context.Context, it systemq.Item, lang string, sources []systemq.ListItemFieldSourcesRow) (*model.AdminItemMetadata, error) {
	texts, err := s.sys.ListItemTextsByItem(ctx, it.ID)
	if err != nil {
		return nil, fmt.Errorf("list item texts: %w", err)
	}
	locks := parseLocks(it.LockedFields)
	out := &model.AdminItemMetadata{Lang: lang, Title: it.Title, Year: nullInt(it.Year),
		OriginalTitle: optText(it.OriginalTitle), ReleaseDate: optText(it.ReleaseDate),
		Genres: genresIn(it.Genres, lang), Locked: []string{}, Sources: []model.MetadataSource{}}
	for _, t := range texts {
		if t.Lang != lang {
			continue
		}
		out.Title, out.Tagline, out.Overview, out.SortTitle = t.Title, optText(t.Tagline), optText(t.Overview), optText(t.SortTitle)
		if out.SortTitle == nil {
			out.SortTitle = optText(t.SortKey) // locked before the typed text was kept
		}
	}
	for _, name := range model.MetaFields {
		if metaFields[name].lockedIn(locks, lang) {
			out.Locked = append(out.Locked, name)
		}
	}
	if !slices.Contains(out.Locked, model.MetaFieldSortTitle) {
		// A free sort title follows the title: the field shows that, not the folded key.
		out.SortTitle = &out.Title
	}
	for _, name := range model.MetaFields {
		f := metaFields[name]
		src := f.source
		if name == model.MetaFieldYear && !slices.Contains(out.Locked, name) {
			src = "release_date" // the fill writes the year with the release date
		}
		if src == "" {
			continue
		}
		for _, row := range sources {
			if row.Field == src && (row.Lang == lang || (!f.text && row.Lang == "")) {
				out.Sources = append(out.Sources, model.MetadataSource{Field: name, Provider: row.Provider})
				break
			}
		}
	}
	return out, nil
}

// UpdateItemMetadata writes the fields of edit, locks each of them and lifts the locks of
// edit.Unlock, in one transaction, and answers with the administration view. Errors:
// model.ErrNotFound, *model.ValidationError.
func (s *Service) UpdateItemMetadata(ctx context.Context, itemID string, edit model.ItemMetadataEdit) (model.AdminItem, error) {
	if s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	it, err := s.liveItem(ctx, s.sys, itemID)
	if err != nil {
		return model.AdminItem{}, err
	}
	set, err := validateMetadataEdit(it.Kind, &edit)
	if err != nil {
		return model.AdminItem{}, err
	}
	lang, err := s.serverTextLang(ctx)
	if err != nil {
		return model.AdminItem{}, err
	}
	if len(set) > 0 || len(edit.Unlock) > 0 {
		if err := s.writeMetadataEdit(ctx, itemID, lang, edit, set); err != nil {
			return model.AdminItem{}, err
		}
		s.log.InfoContext(ctx, "item metadata edited", "item_id", itemID, "lang", lang, "fields", strings.Join(set, ","),
			"unlocked", strings.Join(edit.Unlock, ","))
	}
	return s.GetAdminItem(ctx, itemID)
}

// validateMetadataEdit trims and checks the fields of edit and returns the names of the fields it
// sets, in the order of model.MetaFields.
func validateMetadataEdit(kind string, edit *model.ItemMetadataEdit) ([]string, error) {
	var verr *model.ValidationError
	if !slices.Contains(editableKinds, kind) {
		return nil, verr.Add("kind", "invalid_value")
	}
	var set []string
	text := func(name string, v *string, max int, required bool) {
		if v == nil {
			return
		}
		set = append(set, name)
		*v = strings.TrimSpace(*v)
		switch {
		case required && *v == "":
			verr = verr.Add(name, "required")
		case utf8.RuneCountInString(*v) > max:
			verr = verr.Add(name, "too_long")
		}
	}
	text(model.MetaFieldTitle, edit.Title, maxMetaText, true)
	text(model.MetaFieldOriginalTitle, edit.OriginalTitle, maxMetaText, false)
	text(model.MetaFieldSortTitle, edit.SortTitle, maxMetaText, true)
	if edit.Year != nil {
		set = append(set, model.MetaFieldYear)
		if *edit.Year < minMetaYear || *edit.Year > maxMetaYear {
			verr = verr.Add(model.MetaFieldYear, "invalid_value")
		}
	}
	if edit.ReleaseDate != nil {
		set = append(set, model.MetaFieldReleaseDate)
		*edit.ReleaseDate = strings.TrimSpace(*edit.ReleaseDate)
		if d := *edit.ReleaseDate; d != "" {
			if _, err := time.Parse(time.DateOnly, d); err != nil {
				verr = verr.Add(model.MetaFieldReleaseDate, "invalid_value")
			}
		}
	}
	text(model.MetaFieldTagline, edit.Tagline, maxMetaText, false)
	text(model.MetaFieldOverview, edit.Overview, maxMetaOverview, false)
	if edit.Genres != nil {
		set = append(set, model.MetaFieldGenres)
		var names []string
		for _, g := range *edit.Genres {
			g = strings.TrimSpace(g)
			if g == "" || utf8.RuneCountInString(g) > maxMetaGenre {
				verr = verr.Add(model.MetaFieldGenres, "invalid_value")
				break
			}
			if !slices.Contains(names, g) {
				names = append(names, g)
			}
		}
		if len(names) > maxMetaGenres {
			verr = verr.Add(model.MetaFieldGenres, "too_long")
		}
		*edit.Genres = names
	}
	if kind != "movie" && kind != "series" {
		for _, name := range set {
			if metaFields[name].titleOnly {
				verr = verr.Add(name, "invalid_value")
			}
		}
	}
	for _, name := range edit.Unlock {
		if _, ok := metaFields[name]; !ok || slices.Contains(set, name) {
			verr = verr.Add("unlock", "invalid_value")
			break
		}
	}
	if verr != nil {
		return nil, verr
	}
	return set, nil
}

// writeMetadataEdit writes a validated edit in lang inside one transaction, on the item as it
// stands in that transaction.
func (s *Service) writeMetadataEdit(ctx context.Context, itemID, lang string, edit model.ItemMetadataEdit, set []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	it, err := s.liveItem(ctx, q, itemID)
	if err != nil {
		return err
	}
	locks := parseLocks(it.LockedFields)
	for _, name := range set {
		if l := metaFields[name].lockName(lang); !slices.Contains(locks, l) {
			locks = append(locks, l)
		}
	}
	for _, name := range edit.Unlock {
		f := metaFields[name]
		if f.legacy && slices.Contains(locks, f.lock) {
			// The old lock without language held every language: only lang is freed, the other
			// languages with a stored text keep the field locked in their own name.
			texts, err := q.ListItemTextsByItem(ctx, it.ID)
			if err != nil {
				return fmt.Errorf("list item texts: %w", err)
			}
			for _, t := range texts {
				if l := f.lockName(t.Lang); t.Lang != lang && !slices.Contains(locks, l) {
					locks = append(locks, l)
				}
			}
		}
		locks = slices.DeleteFunc(locks, func(l string) bool { return l == f.lockName(lang) || (f.legacy && l == f.lock) })
	}
	sortLocked := metaFields[model.MetaFieldSortTitle].lockedIn(locks, lang)

	// The text row of lang.
	if edit.Title != nil || edit.SortTitle != nil || edit.Tagline != nil || edit.Overview != nil ||
		slices.Contains(edit.Unlock, model.MetaFieldSortTitle) {
		texts, err := q.ListItemTextsByItem(ctx, it.ID)
		if err != nil {
			return fmt.Errorf("list item texts: %w", err)
		}
		row := systemq.EditItemTextParams{ItemID: it.ID, Lang: lang, Title: it.Title}
		had := false
		for _, t := range texts {
			if t.Lang == lang {
				row.Title, row.Overview, row.Tagline, row.SortKey, row.SortTitle, row.Rank, had = t.Title, t.Overview, t.Tagline, t.SortKey, t.SortTitle, t.Rank, true
			}
		}
		if !had {
			row.Rank = int64(len(texts)) // the next match puts the row at its place in the list
		}
		if edit.Title != nil {
			row.Title = *edit.Title
		}
		if edit.Tagline != nil {
			row.Tagline = nullText(*edit.Tagline)
		}
		if edit.Overview != nil {
			row.Overview = nullText(*edit.Overview)
		}
		switch {
		case edit.SortTitle != nil:
			// The administrator's order: folded and lower-cased like every key, no article stripped;
			// the text stays as typed for the editor.
			row.SortKey = sql.NullString{String: model.SortKey("", *edit.SortTitle), Valid: true}
			row.SortTitle = sql.NullString{String: *edit.SortTitle, Valid: true}
		case !sortLocked || !had:
			row.SortKey = sql.NullString{String: model.SortKey(lang, row.Title), Valid: true}
			row.SortTitle = sql.NullString{}
		}
		if err := q.EditItemText(ctx, row); err != nil {
			return fmt.Errorf("write %s text: %w", lang, err)
		}
	}

	// The columns and the locks.
	p := systemq.EditItemColumnsParams{ID: it.ID, OriginalTitle: it.OriginalTitle, Year: it.Year, ReleaseDate: it.ReleaseDate,
		Genres: it.Genres, Now: s.now().UnixMilli()}
	if edit.OriginalTitle != nil {
		p.OriginalTitle = nullText(*edit.OriginalTitle)
	}
	if edit.Year != nil {
		p.Year = sql.NullInt64{Int64: int64(*edit.Year), Valid: true}
	}
	if edit.ReleaseDate != nil {
		p.ReleaseDate = nullText(*edit.ReleaseDate)
	}
	if edit.Genres != nil {
		if p.Genres, err = withGenres(it.Genres, lang, *edit.Genres); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(locks)
	if err != nil {
		return fmt.Errorf("locked fields: %w", err)
	}
	p.LockedFields = string(raw)
	if err := q.EditItemColumns(ctx, p); err != nil {
		return fmt.Errorf("write columns: %w", err)
	}

	// The provenance of what the administrator wrote goes.
	for _, name := range set {
		f := metaFields[name]
		if f.source == "" {
			continue
		}
		srcLang := ""
		if f.text {
			srcLang = lang
		}
		if err := q.DeleteItemFieldSourceLang(ctx, systemq.DeleteItemFieldSourceLangParams{ItemID: it.ID, Field: f.source, Lang: srcLang}); err != nil {
			return fmt.Errorf("delete source of %s: %w", f.source, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListAdminGenres returns every genre name of the live films and series in the server's metadata
// language, sorted without regard to case (the suggestions of the editor).
func (s *Service) ListAdminGenres(ctx context.Context) (string, []string, error) {
	if s.meta == nil {
		return "", nil, ErrNotConfigured
	}
	lang, err := s.serverTextLang(ctx)
	if err != nil {
		return "", nil, err
	}
	rows, err := s.sys.ListLiveGenres(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list genres: %w", err)
	}
	out := []string{}
	for _, raw := range rows {
		for _, g := range genresIn(raw, lang) {
			if !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return lang, out, nil
}

// parseLocks reads items.locked_fields in its stored order; an invalid value locks nothing.
func parseLocks(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

// genresIn returns the genre names of lang from items.genres ({"de": […]}); none for another form.
func genresIn(raw, lang string) []string {
	var byLang map[string][]string
	if err := json.Unmarshal([]byte(raw), &byLang); err != nil || byLang[lang] == nil {
		return []string{}
	}
	return byLang[lang]
}

// withGenres returns items.genres with the list of lang replaced; the other languages stay.
func withGenres(raw, lang string, names []string) (string, error) {
	byLang := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(raw), &byLang); err != nil || byLang == nil {
		byLang = map[string]json.RawMessage{} // an old list or nothing: start the object
	}
	list, err := json.Marshal(names)
	if err != nil {
		return "", fmt.Errorf("genres: %w", err)
	}
	byLang[lang] = list
	out, err := json.Marshal(byLang)
	if err != nil {
		return "", fmt.Errorf("genres: %w", err)
	}
	return string(out), nil
}

// optText is the value of a nullable text column, nil for NULL.
func optText(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
