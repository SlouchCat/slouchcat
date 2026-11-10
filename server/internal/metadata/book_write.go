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

	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Writing the record of a book source onto an audiobook (concept 19 D6.3 point 4, D5.2). Three
// modes: the automatic match fills only fields that are empty and not locked, without locking them
// (the files are empty there, so a scan does not overwrite them); the refetch («Vom Anbieter neu
// holen») writes the fields that are not locked and are empty or come from that source; the match
// in the editor writes the fields the administrator ticked that are not locked, and locks them.
// Every written field is credited to the source (item_field_sources). Texts go into every text row
// the scan writes (the book's language and en, and any other stored row), genres into both
// languages. The cover of the source becomes the item_images row of the poster (chosen 0, a
// candidate image.fetch takes only when the book has no chosen cover and its files none); the
// editor's tick on cover chooses it (chosen 1 and the lock poster, like a choice in the cover tab).
// The book is linked to its series in the same transaction (WithAudioSeriesRelink), and a written
// author queues the unified author names (WithAuthorDisplay).

// BookMatchFields are the fields a book source can deliver, in the order of the contract's
// BookMatchField.
var BookMatchFields = []string{"title", "subtitle", "author", "narrator", "seriesName", "seriesIndex", "year",
	"publisher", "originalLanguage", "genres", "overview", "isbn", "asin", "cover"}

// bookFieldInfo maps a BookMatchField onto its lock and provenance name (for a text field the
// field part of field:lang).
type bookFieldInfo struct {
	name string
	text bool
}

var bookFieldInfos = map[string]bookFieldInfo{
	"title": {"title", true}, "subtitle": {"tagline", true}, "overview": {"overview", true},
	"author": {"author", false}, "narrator": {"narrator", false}, "seriesName": {"series_name", false},
	"seriesIndex": {"series_index", false}, "year": {"year", false}, "publisher": {"publisher", false},
	"originalLanguage": {fieldOriginalLanguage, false}, "genres": {"genres", false}, "isbn": {"isbn", false},
	"asin": {"asin", false}, "cover": {"poster", false},
}

type bookWriteMode int

const (
	bookWriteAuto bookWriteMode = iota
	bookWriteRefetch
	bookWriteManual
)

// bookWrite says how writeBookMatch writes a record.
type bookWrite struct {
	mode       bookWriteMode
	fields     map[string]bool // manual: the ticked BookMatchFields
	region     string          // the Audible region of the match ("" = the record's)
	score      float64
	candidates []scoredBook // auto: the candidates to store
}

// bookView is an audiobook as a book match sees it: the item, its text rows, the languages a text
// write goes to (the book's language first), its locks and the provenance of its fields.
type bookView struct {
	item    systemq.Item
	texts   map[string]systemq.ItemText
	langs   []string
	locks   map[string]bool
	sources map[string]string // field + "|" + lang → provider
}

func loadBookView(ctx context.Context, q *systemq.Queries, item systemq.Item) (bookView, error) {
	rows, err := q.ListItemTextsByItem(ctx, item.ID)
	if err != nil {
		return bookView{}, fmt.Errorf("texts of %s: %w", item.ID, err)
	}
	srcs, err := q.ListItemFieldSources(ctx, item.ID)
	if err != nil {
		return bookView{}, fmt.Errorf("field sources of %s: %w", item.ID, err)
	}
	v := bookView{item: item, texts: map[string]systemq.ItemText{}, locks: lockedFields(item.LockedFields), sources: map[string]string{}}
	for _, t := range rows {
		v.texts[t.Lang] = t
	}
	for _, r := range srcs {
		v.sources[r.Field+"|"+r.Lang] = r.Provider
	}
	lang := "de"
	orig := item.OriginalLanguage.String
	switch _, de := v.texts["de"]; {
	case (orig == "de" || orig == "en") && v.locks[fieldOriginalLanguage]:
		lang = orig
	case de:
	case v.texts["en"].Lang == "en":
		lang = "en"
	case orig == "en":
		lang = "en"
	}
	v.langs = []string{lang}
	if lang != "en" {
		v.langs = append(v.langs, "en")
	}
	for _, t := range rows {
		if !slices.Contains(v.langs, t.Lang) {
			v.langs = append(v.langs, t.Lang)
		}
	}
	return v, nil
}

// locked reports whether a field is locked (a text field in any of its languages, or by the old
// lock without language).
func (v bookView) locked(field string) bool {
	info := bookFieldInfos[field]
	if !info.text {
		return v.locks[info.name]
	}
	if v.locks[info.name] {
		return true
	}
	for _, l := range v.langs {
		if v.locks[textLockName(info.name, l)] {
			return true
		}
	}
	return false
}

// text is the value of a text field in lang ("" without row); the title is the book's title.
func (v bookView) text(field, lang string) string {
	t, ok := v.texts[lang]
	switch {
	case field == "title":
		return v.item.Title
	case !ok:
		return ""
	case field == "subtitle":
		return t.Tagline.String
	default:
		return t.Overview.String
	}
}

// source is the provenance of a field (a text field in lang).
func (v bookView) source(field, lang string) string {
	info := bookFieldInfos[field]
	if !info.text {
		lang = ""
	}
	return v.sources[info.name+"|"+lang]
}

// current is the value of a field today as display text ("" when empty); cover is the caller's.
func (v bookView) current(field string) string {
	it := v.item
	switch field {
	case "title", "subtitle", "overview":
		return v.text(field, v.langs[0])
	case "author":
		return it.Author.String
	case "narrator":
		return it.Narrator.String
	case "seriesName":
		return it.SeriesName.String
	case "seriesIndex":
		if it.SeriesIndex.Valid {
			return formatVolume(it.SeriesIndex.Float64)
		}
	case "year":
		if it.Year.Valid && it.Year.Int64 > 0 {
			return strconv.FormatInt(it.Year.Int64, 10)
		}
	case "publisher":
		return it.Publisher.String
	case "originalLanguage":
		return it.OriginalLanguage.String
	case "genres":
		return strings.Join(bookGenres(it.Genres, v.langs[0]), ", ")
	case "isbn":
		return it.Isbn.String
	case "asin":
		return it.Asin.String
	}
	return ""
}

// proposedOf is the value of a field in a record as display text ("" when the source has none).
func proposedOf(rec BookRecord, field string) string {
	switch field {
	case "title":
		return rec.Title
	case "subtitle":
		return rec.Subtitle
	case "overview":
		return rec.Overview
	case "author":
		return strings.Join(rec.Authors, ", ")
	case "narrator":
		return strings.Join(rec.Narrators, ", ")
	case "seriesName":
		return rec.SeriesName
	case "seriesIndex":
		if rec.SeriesIndex != nil && rec.SeriesName != "" {
			return formatVolume(*rec.SeriesIndex)
		}
	case "year":
		if rec.Year != nil {
			return strconv.Itoa(*rec.Year)
		}
	case "publisher":
		return rec.Publisher
	case "originalLanguage":
		return rec.Language
	case "genres":
		return strings.Join(rec.Genres, ", ")
	case "isbn":
		return normISBN(rec.ISBN)
	case "asin":
		return rec.ASIN
	case "cover":
		return rec.CoverURL
	}
	return ""
}

// formatVolume writes a volume number without trailing zeros (3, 1.5).
func formatVolume(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// bookGenres reads the genre names of lang from items.genres ({"de": […]}, or an old plain list).
func bookGenres(raw, lang string) []string {
	var byLang map[string][]string
	if err := json.Unmarshal([]byte(raw), &byLang); err == nil {
		return byLang[lang]
	}
	var list []string
	_ = json.Unmarshal([]byte(raw), &list) // an unreadable value has no genres
	return list
}

// takes decides whether a write takes a field from the record (lang for a text field): never a
// locked field or a value the source does not have; automatically only an empty field; a refetch
// an empty field or one from the same source; the editor a ticked field.
func (v bookView) takes(w bookWrite, rec BookRecord, field, lang string) bool {
	if v.locked(field) || proposedOf(rec, field) == "" {
		return false
	}
	cur := v.current(field)
	if bookFieldInfos[field].text {
		cur = v.text(field, lang)
	}
	switch w.mode {
	case bookWriteAuto:
		return cur == ""
	case bookWriteRefetch:
		return cur == "" || v.source(field, lang) == rec.Provider
	default:
		return w.fields[field]
	}
}

// writeBookMatch writes rec onto the book itemID as w says and records the match. An automatic
// write to a book matched meanwhile writes nothing.
func (s *Service) writeBookMatch(ctx context.Context, itemID string, rec BookRecord, w bookWrite) error {
	now := s.now()
	ms := now.UnixMilli()
	var written []string
	err := s.inTx(ctx, func(q *systemq.Queries) error {
		cur, err := q.GetItemSystem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (cur.DeletedAt.Valid || cur.Kind != "audiobook")) {
			return model.ErrNotFound
		}
		if err != nil {
			return err
		}
		if w.mode == bookWriteAuto && cur.BookMatchState.String == BookStateMatched {
			return nil
		}
		v, err := loadBookView(ctx, q, cur)
		if err != nil {
			return err
		}
		if written, err = s.applyBookRecord(ctx, q, v, rec, w, ms); err != nil {
			return err
		}
		if err := s.storeBookCover(ctx, q, v, rec, w, ms); err != nil {
			return err
		}
		p := systemq.SetBookMatchParams{State: nullString(BookStateMatched), Provider: nullString(rec.Provider),
			ProviderID: nullString(rec.ID), Region: nullString(firstNonEmpty(w.region, rec.Region)),
			MatchedAt: sql.NullInt64{Int64: ms, Valid: true}, Score: sql.NullFloat64{Float64: roundScore(w.score), Valid: true},
			Now: ms, ID: cur.ID}
		switch w.mode {
		case bookWriteManual:
			p.Manual = 1
		case bookWriteRefetch:
			p.Provider, p.ProviderID, p.Region = cur.BookMatchProvider, cur.BookMatchID, cur.BookMatchRegion
			p.Score, p.Manual = cur.BookMatchScore, cur.BookMatchManual
		}
		if err := q.SetBookMatch(ctx, p); err != nil {
			return err
		}
		if w.mode == bookWriteAuto {
			if err := replaceBookCandidates(ctx, q, cur.ID, w.candidates); err != nil {
				return err
			}
		}
		if s.relinkSeries != nil {
			// The series follows the series name and the author the book has now (concept 19 D4.4).
			return s.relinkSeries(ctx, q, []string{cur.ID}, now)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("write book match of %s: %w", itemID, err)
	}
	s.log.Info("audiobook matched", "item_id", itemID, "provider", rec.Provider, "mode", int(w.mode),
		"fields", strings.Join(written, ","))
	if slices.Contains(written, "author") && s.authorDisplay != nil {
		if err := s.authorDisplay(ctx); err != nil {
			s.log.Warn("unified author names not queued", "item_id", itemID, "err", err)
		}
	}
	return nil
}

// applyBookRecord writes the fields of rec the write takes, their provenance and, for the editor,
// their locks. It returns the BookMatchFields it wrote (without cover).
func (s *Service) applyBookRecord(ctx context.Context, q *systemq.Queries, v bookView, rec BookRecord, w bookWrite, ms int64) ([]string, error) {
	it := v.item
	p := systemq.UpdateAudiobookParams{
		ID: it.ID, Title: it.Title, SortTitle: it.SortTitle, Year: it.Year, Genres: it.Genres, Author: it.Author,
		Narrator: it.Narrator, SeriesName: it.SeriesName, SeriesIndex: it.SeriesIndex, Publisher: it.Publisher,
		Isbn: it.Isbn, Asin: it.Asin, OriginalLanguage: it.OriginalLanguage, Explicit: it.Explicit,
		CoverPath: it.CoverPath, OwnRating: it.OwnRating, RatingSource: it.RatingSource, DurationMs: it.DurationMs, Now: ms,
	}
	var written []string
	var locks []string
	column := func(field string, set func(value string)) {
		if v.takes(w, rec, field, "") {
			set(proposedOf(rec, field))
			written = append(written, field)
			locks = append(locks, bookFieldInfos[field].name)
		}
	}
	column("author", func(x string) { p.Author = nullString(x) })
	column("narrator", func(x string) { p.Narrator = nullString(x) })
	column("seriesName", func(x string) { p.SeriesName = nullString(x) })
	// The volume belongs to the source's series: automatically and in a refetch it is taken only
	// with that series name, or when the book's series is the same series (its key), so a book of
	// series X without volume does not become volume 5 of X because it is volume 5 of Y.
	if w.mode == bookWriteManual || slices.Contains(written, "seriesName") ||
		audioseries.Default().Name(it.SeriesName.String) == audioseries.Default().Name(rec.SeriesName) {
		column("seriesIndex", func(string) { p.SeriesIndex = sql.NullFloat64{Float64: *rec.SeriesIndex, Valid: true} })
	}
	column("year", func(string) { p.Year = sql.NullInt64{Int64: int64(*rec.Year), Valid: true} })
	column("publisher", func(x string) { p.Publisher = nullString(x) })
	column("originalLanguage", func(x string) { p.OriginalLanguage = nullString(x) })
	column("isbn", func(x string) { p.Isbn = nullString(x) })
	column("asin", func(x string) { p.Asin = nullString(x) })
	var genreErr error
	column("genres", func(string) { p.Genres, genreErr = bookGenresJSON(it.Genres, v.langs, rec.Genres) })
	if genreErr != nil {
		return nil, genreErr
	}

	// The texts, per row of the book.
	type textWrite struct{ field, lang string }
	var texts []textWrite
	for _, field := range []string{"title", "subtitle", "overview"} {
		took := false
		for _, l := range v.langs {
			if v.takes(w, rec, field, l) {
				texts = append(texts, textWrite{field, l})
				took = true
				if w.mode == bookWriteManual {
					locks = append(locks, textLockName(bookFieldInfos[field].name, l))
				}
			}
		}
		if took {
			written = append(written, field)
		}
	}
	sortLocked := v.locks[fieldSortTitle] || slices.ContainsFunc(v.langs, func(l string) bool { return v.locks[textLockName(fieldSortTitle, l)] })
	if slices.ContainsFunc(texts, func(t textWrite) bool { return t.field == "title" }) {
		p.Title = rec.Title
		if !sortLocked {
			p.SortTitle = sortTitle(rec.Title)
		}
	}
	if err := q.UpdateAudiobook(ctx, p); err != nil {
		return nil, fmt.Errorf("update audiobook: %w", err)
	}
	for _, l := range v.langs {
		row, had := v.texts[l]
		changed := false
		if !had {
			row = systemq.ItemText{ItemID: it.ID, Lang: l, Title: it.Title, Rank: int64(len(v.texts))}
		}
		for _, t := range texts {
			if t.lang != l {
				continue
			}
			changed = true
			switch t.field {
			case "title":
				row.Title = rec.Title
			case "subtitle":
				row.Tagline = nullString(rec.Subtitle)
			case "overview":
				row.Overview = nullString(rec.Overview)
			}
		}
		if !changed {
			continue
		}
		if !had || (!v.locks[fieldSortTitle] && !v.locks[textLockName(fieldSortTitle, l)]) {
			row.SortKey, row.SortTitle = sortKey(l, row.Title), sql.NullString{}
		}
		if err := q.EditItemText(ctx, systemq.EditItemTextParams{ItemID: it.ID, Lang: l, Title: row.Title, Overview: row.Overview,
			Tagline: row.Tagline, SortKey: row.SortKey, SortTitle: row.SortTitle, Rank: row.Rank}); err != nil {
			return nil, fmt.Errorf("write %s text: %w", l, err)
		}
	}

	// Provenance and, in the editor, the locks.
	for _, field := range written {
		info := bookFieldInfos[field]
		if info.text {
			continue
		}
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: it.ID, Field: info.name, Provider: rec.Provider}); err != nil {
			return nil, fmt.Errorf("store source of %s: %w", info.name, err)
		}
	}
	for _, t := range texts {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: it.ID, Field: bookFieldInfos[t.field].name,
			Lang: t.lang, Provider: rec.Provider}); err != nil {
			return nil, fmt.Errorf("store source of %s:%s: %w", t.field, t.lang, err)
		}
	}
	if w.mode == bookWriteManual && len(locks) > 0 {
		if err := q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: it.ID,
			LockedFields: addLocked(it.LockedFields, locks...), Now: ms}); err != nil {
			return nil, fmt.Errorf("lock fields: %w", err)
		}
	}
	return written, nil
}

// storeBookCover stores the cover of the source as the poster row of the book: a candidate
// (chosen 0) unless the administrator chose or locked the poster; the editor's tick chooses it and
// locks the poster.
func (s *Service) storeBookCover(ctx context.Context, q *systemq.Queries, v bookView, rec BookRecord, w bookWrite, ms int64) error {
	src := bookCoverOf(rec)
	if src == "" {
		return nil
	}
	if w.mode == bookWriteManual && w.fields["cover"] && !v.locks["poster"] {
		if err := q.UpsertItemImage(ctx, systemq.UpsertItemImageParams{ItemID: v.item.ID, Kind: "poster", Provider: rec.Provider,
			Source: src, Chosen: 1, Now: ms}); err != nil {
			return err
		}
		// The lock is set in the same transaction after applyBookRecord's; reread the stored value.
		cur, err := q.GetItemSystem(ctx, v.item.ID)
		if err != nil {
			return err
		}
		if err := q.SetItemLockedFields(ctx, systemq.SetItemLockedFieldsParams{ID: v.item.ID,
			LockedFields: addLocked(cur.LockedFields, "poster"), Now: ms}); err != nil {
			return err
		}
		return q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: v.item.ID, Field: "poster", Provider: rec.Provider})
	}
	if v.locks["poster"] {
		return nil
	}
	rows, err := q.ListItemImagesByItem(ctx, v.item.ID)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(rows, func(r systemq.ItemImage) bool { return r.Kind == "poster" && r.Chosen == 1 }) {
		return nil
	}
	return q.UpsertItemImage(ctx, systemq.UpsertItemImageParams{ItemID: v.item.ID, Kind: "poster", Provider: rec.Provider,
		Source: src, Chosen: 0, Now: ms})
}

// bookCoverOf is the best cover of a record: its default poster, else its first, else CoverURL.
func bookCoverOf(rec BookRecord) string {
	for _, c := range rec.Covers {
		if c.Kind == "poster" && c.Default && c.Source != "" {
			return c.Source
		}
	}
	for _, c := range rec.Covers {
		if c.Kind == "poster" && c.Source != "" {
			return c.Source
		}
	}
	return rec.CoverURL
}

// bookGenresJSON is items.genres with the names of the source in de, en and every language of the
// book (the other languages stay).
func bookGenresJSON(raw string, langs, names []string) (string, error) {
	byLang := map[string][]string{}
	if err := json.Unmarshal([]byte(raw), &byLang); err != nil || byLang == nil {
		byLang = map[string][]string{}
	}
	for _, l := range append([]string{"de", "en"}, langs...) {
		byLang[l] = slices.Clone(names)
	}
	out, err := json.Marshal(byLang)
	if err != nil {
		return "", fmt.Errorf("genres: %w", err)
	}
	return string(out), nil
}

// refetchBook fetches the entry of a book's stored match again and writes it (refetch mode). A
// source that pauses or is not configured ends the job without a write; any other failure is
// returned for a retry.
func (s *Service) refetchBook(ctx context.Context, item systemq.Item) error {
	if item.BookMatchState.String != BookStateMatched || !item.BookMatchProvider.Valid || !item.BookMatchID.Valid {
		return nil
	}
	p, ok := s.bookProvider(item.BookMatchProvider.String)
	if !ok {
		return nil
	}
	rec, err := p.Book(ctx, item.BookMatchID.String, item.BookMatchRegion.String)
	switch {
	case errors.Is(err, errSourcePaused), errors.Is(err, errNotConfigured), errors.Is(err, ErrNotFound):
		s.log.Info("audiobook not refetched", "item_id", item.ID, "provider", p.Name(), "reason", failureReason(err))
		return nil
	case err != nil:
		return fmt.Errorf("refetch %s from %s: %s", item.ID, p.Name(), failureReason(err))
	}
	return s.writeBookMatch(ctx, item.ID, rec, bookWrite{mode: bookWriteRefetch})
}
