package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Writes of the audiobook scanner (M6 design 3.1): a new book from the folder (metadata.json and
// folder names), and the book completed once all its parts are probed: part order, book-wide
// chapters, duration, the tags of the first part, the cover and the image job.

// embeddedCover is the cover_path prefix of a picture embedded in a media file.
const embeddedCover = "embedded:"

// bookCache holds the folder metadata read in one batch, by folder key.
type bookCache map[string]*bookMeta

// describeBook reads the folder metadata of a new audio file (the tags follow with its probe).
func describeBook(r *os.Root, rootID int64, d diskFile, cache bookCache) fileInfo {
	key := audioFolderKey(rootID, d.rel)
	m, ok := cache[key]
	if !ok {
		meta, folder := bookFiles(r, d.rel)
		merged := mergeBook(meta, folder)
		m = &merged
		cache[key] = m
	}
	return fileInfo{folder: key, book: m}
}

// bookRating is the age rating of a book: 18 from metadata.json for an explicit book (rating
// source 'tags'), else the default rating of the library (rating source 'library', AN-2).
func bookRating(lib appq.Library, explicit bool) (sql.NullInt64, sql.NullString) {
	if explicit {
		return sql.NullInt64{Int64: 18, Valid: true}, nullString("tags")
	}
	if !lib.DefaultRating.Valid {
		return sql.NullInt64{}, sql.NullString{}
	}
	return lib.DefaultRating, nullString("library")
}

// genresJSON is items.genres of a book: the same names for both text languages.
func genresJSON(names []string) (string, error) {
	if names == nil {
		names = []string{}
	}
	raw, err := json.Marshal(struct {
		DE []string `json:"de"`
		EN []string `json:"en"`
	}{names, names})
	return string(raw), err
}

func nullVolume(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func nullYear(y int) sql.NullInt64 { return sql.NullInt64{Int64: int64(y), Valid: y > 0} }

// audiobookItem returns the book of a new audio file: the book that owns its folder key (a
// soft-deleted one comes back), else a new one from the folder metadata. Books need no provider
// match; the probe job completes them (Service.finishAudiobook).
func (w *batchWriter) audiobookItem(ctx context.Context, info fileInfo) (string, error) {
	if id := w.ix().byFolder[info.folder]; id != "" && w.ix().kinds[id] == "audiobook" {
		return id, w.restore(ctx, id, false)
	}
	m := info.book
	if m == nil {
		m = &bookMeta{}
	}
	title := m.title
	if title == "" {
		title = "?"
	}
	genres, err := genresJSON(m.genres)
	if err != nil {
		return "", err
	}
	rating, source := bookRating(w.lib, m.explicit)
	id := w.newID()
	if _, err := w.q.InsertAudiobook(ctx, systemq.InsertAudiobookParams{
		ID: id, LibraryID: w.lib.ID, Title: title, SortTitle: SortTitle(title), Year: nullYear(m.year), Genres: genres,
		Author: nullString(m.author), Narrator: nullString(m.narrator), SeriesName: nullString(m.series),
		SeriesIndex: nullVolume(m.volume), Publisher: nullString(m.publisher), Isbn: nullString(m.isbn),
		Asin: nullString(m.asin), OriginalLanguage: nullString(m.originalLanguage()),
		Explicit: boolInt(m.explicit), CoverPath: nullString(m.cover),
		OwnRating: rating, RatingSource: source, Now: sql.NullInt64{Int64: w.ms, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("insert audiobook: %w", err)
	}
	if m.description != "" || m.subtitle != "" {
		if err := w.q.UpsertItemText(ctx, systemq.UpsertItemTextParams{
			ItemID: id, Lang: m.lang, Title: title, Overview: nullString(m.description), Tagline: nullString(m.subtitle),
			SortKey: sortKey(m.lang, title),
		}); err != nil {
			return "", fmt.Errorf("insert audiobook text: %w", err)
		}
	}
	w.ix().kinds[id] = "audiobook"
	w.ix().addFolder(id, info.folder)
	w.stats.items++
	return id, nil
}

// finishAudiobooks completes the books a scan batch touched (new, moved or missing parts) and
// then queues library.author_display for their authors (concept 19 D6.4). A failure is logged;
// the next probe or scan of the book completes it.
func (s *Service) finishAudiobooks(ctx context.Context, ids map[string]bool) {
	for id := range ids {
		if err := s.finishAudiobook(ctx, id); err != nil {
			s.log.WarnContext(ctx, "audiobook not completed", "item_id", id, "err", err)
		}
	}
	if len(ids) > 0 {
		s.queueAuthorDisplayLogged(ctx)
	}
}

// queueAuthorDisplayLogged is QueueAuthorDisplay for the scan and the probe: a failure is logged,
// the next scan queues the job again.
func (s *Service) queueAuthorDisplayLogged(ctx context.Context) {
	if err := s.QueueAuthorDisplay(ctx); err != nil {
		s.log.WarnContext(ctx, "unified author names not queued", "err", err)
	}
}

// finishAudiobook brings a book up to date with its present parts. It numbers the parts in order
// (part_index 1..n; NULL for a book of one file). Once every part is probed (or failed to probe)
// it also writes the book-wide chapters (parts that failed to probe are left out), the duration
// (sum of the parts), the metadata (metadata.json > tags of the first part > folder names; locked
// fields and manual ratings stay), the cover (folder image, else the picture embedded in the first
// part that has one) and queues image.fetch for the cover, and metadata.match for the match against
// the sources of its folder (queueBookMatch). In the same transaction the book is linked to its
// series (RelinkAudioSeries). When the author changed, library.author_display follows the commit.
// Anything but an undeleted audiobook is left alone.
func (s *Service) finishAudiobook(ctx context.Context, itemID string) error {
	sq := systemq.New(s.db.Read)
	item, err := sq.GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load audiobook: %w", err)
	}
	if item.Kind != "audiobook" || item.DeletedAt.Valid {
		return nil
	}
	files, err := sq.ListAudiobookParts(ctx, itemID)
	if err != nil {
		return fmt.Errorf("list audiobook parts: %w", err)
	}
	if len(files) == 0 {
		return nil
	}
	parts := make([]bookPart, 0, len(files))
	done := true
	for _, f := range files {
		p := bookPart{id: f.ID, rel: f.RelPath, durationMs: f.DurationMs.Int64}
		if f.Probe.Valid {
			var pr transcode.Probe
			if err := json.Unmarshal([]byte(f.Probe.String), &pr); err == nil {
				p.probe = &pr
			}
		}
		done = done && (f.Probe.Valid || f.ProbeError.Valid)
		parts = append(parts, p)
	}
	orderParts(parts)

	var meta bookMeta
	var lib appq.Library
	var sources []string // the folder's list of sources
	if done {
		root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, files[0].RootID)
		if err != nil {
			return fmt.Errorf("load library root: %w", err)
		}
		sources = model.ParseProviders(root.Providers)
		if lib, err = appq.New(s.db.Read).GetLibrary(ctx, item.LibraryID); err != nil {
			return fmt.Errorf("load library: %w", err)
		}
		pr, err := paths.NewRoot(root.Path)
		if err != nil {
			return err
		}
		r, err := paths.Open(pr)
		if err != nil {
			return err
		}
		jsonLayer, folder := bookFiles(r, parts[0].rel)
		_ = r.Close()
		var tags map[string]string
		for _, p := range parts {
			if p.probe != nil {
				tags = p.probe.Format.Tags
				break
			}
		}
		meta = mergeBook(jsonLayer, tagMeta(tags, len(parts) > 1), folder)
		if meta.cover == "" {
			for _, p := range parts {
				if p.probe != nil && hasCover(p.probe) {
					meta.cover = embeddedCover + p.id
					break
				}
			}
		}
	}

	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	for i, p := range parts {
		idx := sql.NullInt64{Int64: int64(i + 1), Valid: len(parts) > 1}
		if err := q.SetMediaFilePartIndex(ctx, systemq.SetMediaFilePartIndexParams{PartIndex: idx, ID: p.id}); err != nil {
			return fmt.Errorf("set part index: %w", err)
		}
	}
	authorChanged := false
	if done {
		author, err := s.writeBook(ctx, q, item, lib, parts, meta)
		if err != nil {
			return err
		}
		if err := queueBookMatch(ctx, q, item, sources, s.now()); err != nil {
			return err
		}
		authorChanged = author != item.Author
	}
	// The series follows the series name and the author the book has now (concept 19 D4.4).
	if err := RelinkAudioSeries(ctx, q, []string{item.ID}, s.now()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if authorChanged {
		s.queueAuthorDisplayLogged(ctx)
	}
	return nil
}

// settingBookAutoMatch is the automatic match of audiobooks (metadata.SettingBookAutoMatch: off,
// new, all; absent = all).
const settingBookAutoMatch = "metadata.books.auto_match"

// queueBookMatch queues metadata.match for a book whose parts are all probed, so its length is known
// (concept 19 D6.3 point 6), unless its folder lists no source besides local, the automatic match
// is off or the book was matched or reviewed already: only a book not tried yet or in state error
// goes again.
func queueBookMatch(ctx context.Context, q *systemq.Queries, item systemq.Item, sources []string, now time.Time) error {
	if st := item.BookMatchState.String; (st != "" && st != "error") || !slices.ContainsFunc(sources, func(p string) bool { return p != model.ProviderLocal }) {
		return nil
	}
	mode, err := q.GetSystemSetting(ctx, settingBookAutoMatch)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read %s: %w", settingBookAutoMatch, err)
	}
	if strings.TrimSpace(mode) == "off" {
		return nil
	}
	return enqueueTx(ctx, q, now, matchJob(item.ID))
}

// hasCover reports whether a probe has an attached picture.
func hasCover(p *transcode.Probe) bool {
	for _, st := range p.Streams {
		if st.AttachedPic {
			return true
		}
	}
	return false
}

// writeBook writes chapters, duration, metadata, texts and cover of a book whose parts are all
// probed, and queues image.fetch when it has a cover. A field the editor or an older writer locked
// keeps its value (bookLocks); every value it takes from the files gets the provenance local
// (concept 19 D5.3: «Datei» in the editor). It returns the author the book has now.
func (s *Service) writeBook(ctx context.Context, q *systemq.Queries, item systemq.Item, lib appq.Library, parts []bookPart, meta bookMeta) (sql.NullString, error) {
	locked := bookLocks(lockedSet(item.LockedFields))
	title, sortTitle := item.Title, item.SortTitle
	if !locked.anyText("title") && meta.title != "" {
		title, sortTitle = meta.title, SortTitle(meta.title)
	}
	if locked.anyText("sort_title") {
		sortTitle = item.SortTitle
	}
	probed := make([]bookPart, 0, len(parts))
	var duration int64
	for _, p := range parts {
		if p.probe != nil {
			probed = append(probed, p)
			duration += p.durationMs
		}
	}
	if err := q.DeleteChapters(ctx, item.ID); err != nil {
		return sql.NullString{}, fmt.Errorf("delete chapters: %w", err)
	}
	for i, c := range bookChapters(probed, title) {
		if err := q.InsertChapter(ctx, systemq.InsertChapterParams{
			ItemID: item.ID, Idx: int64(i), Title: c.title, StartMs: c.startMs, EndMs: c.endMs,
			MediaFileID: nullString(c.mediaFileID), FileStartMs: c.fileStartMs,
		}); err != nil {
			return sql.NullString{}, fmt.Errorf("insert chapter: %w", err)
		}
	}

	p := systemq.UpdateAudiobookParams{
		ID: item.ID, Title: title, SortTitle: sortTitle, Year: item.Year, Genres: item.Genres, Author: item.Author,
		Narrator: item.Narrator, SeriesName: item.SeriesName, SeriesIndex: item.SeriesIndex, Publisher: item.Publisher,
		Isbn: item.Isbn, Asin: item.Asin, OriginalLanguage: item.OriginalLanguage, Explicit: boolInt(meta.explicit),
		CoverPath: nullString(meta.cover), OwnRating: item.OwnRating, RatingSource: item.RatingSource,
		DurationMs: sql.NullInt64{Int64: duration, Valid: len(probed) > 0}, Now: s.now().UnixMilli(),
	}
	var fromFiles []string // the fields (without language) the files wrote
	set := func(field string, value string, dst *sql.NullString) {
		if !locked[field] && value != "" {
			*dst = nullString(value)
			fromFiles = append(fromFiles, field)
		}
	}
	set("author", meta.author, &p.Author)
	set("narrator", meta.narrator, &p.Narrator)
	set("series_name", meta.series, &p.SeriesName)
	set("publisher", meta.publisher, &p.Publisher)
	set("isbn", meta.isbn, &p.Isbn)
	set("asin", meta.asin, &p.Asin)
	set(fieldOriginalLanguage, meta.originalLanguage(), &p.OriginalLanguage)
	if !locked["series_index"] && meta.volume != nil {
		p.SeriesIndex = nullVolume(meta.volume)
		fromFiles = append(fromFiles, "series_index")
	}
	if !locked["year"] && meta.year > 0 {
		p.Year = nullYear(meta.year)
		fromFiles = append(fromFiles, "year")
	}
	if !locked["genres"] && len(meta.genres) > 0 {
		g, err := genresJSON(meta.genres)
		if err != nil {
			return sql.NullString{}, err
		}
		p.Genres = g
		fromFiles = append(fromFiles, "genres")
	}
	switch src := item.RatingSource.String; {
	case locked["own_rating"] || (src != "" && src != "library" && src != "tags"):
	default:
		p.OwnRating, p.RatingSource = bookRating(lib, meta.explicit)
	}
	if err := q.UpdateAudiobook(ctx, p); err != nil {
		return sql.NullString{}, fmt.Errorf("update audiobook: %w", err)
	}
	// The language of the text rows: the original language the administrator locked, when it is
	// one of them, else the one of the files (writeBookText adds en).
	if l := p.OriginalLanguage.String; locked[fieldOriginalLanguage] && (l == "de" || l == "en") {
		meta.lang = l
	}
	texts, err := writeBookText(ctx, q, item.ID, title, meta, locked)
	if err != nil {
		return sql.NullString{}, err
	}
	for _, field := range fromFiles {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: item.ID, Field: field, Provider: model.ProviderLocal}); err != nil {
			return sql.NullString{}, fmt.Errorf("store source of %s: %w", field, err)
		}
	}
	for _, t := range texts {
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{ItemID: item.ID, Field: t.field, Lang: t.lang, Provider: model.ProviderLocal}); err != nil {
			return sql.NullString{}, fmt.Errorf("store source of %s:%s: %w", t.field, t.lang, err)
		}
	}
	if meta.cover != "" {
		return p.Author, enqueueTx(ctx, q, s.now(), imageJob(item.ID))
	}
	return p.Author, nil
}

// fieldOriginalLanguage is the column, lock and provenance field of the original language.
const fieldOriginalLanguage = "original_language"

// originalLanguage is the original language of a book: the one metadata.json names, else the
// language of its texts (de when nothing names one).
func (m bookMeta) originalLanguage() string {
	if m.origLang != "" {
		return m.origLang
	}
	return m.lang
}

// bookLocks are the locks of a book (items.locked_fields). The editor locks a text field per
// language (title:de, overview:en, tagline:de, sort_title:de; concept 15 7.5), older writers
// without language (title, overview, tagline); both keep the scan away (concept 19 D5.3).
type bookLocks map[string]bool

// text reports whether a text field is locked in lang.
func (l bookLocks) text(field, lang string) bool {
	return l[field] || l[model.TextLockName(field, lang)]
}

// anyText reports whether a text field is locked in any language.
func (l bookLocks) anyText(field string) bool {
	if l[field] {
		return true
	}
	for name := range l {
		if strings.HasPrefix(name, field+":") {
			return true
		}
	}
	return false
}

// textSource is a text field of one language whose value came from the files.
type textSource struct{ field, lang string }

// writeBookText stores the title, description and subtitle of a book in its language and, for a
// book in another language, in English as well: English is the fallback of every profile language
// (visibility.pickTexts), so an English profile sees a German book's description. Locked texts
// keep the stored value. It returns the texts it took from the files.
func writeBookText(ctx context.Context, q *systemq.Queries, itemID, title string, meta bookMeta, locked bookLocks) ([]textSource, error) {
	texts, err := q.ListItemTextsByItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("list audiobook texts: %w", err)
	}
	langs := []string{meta.lang}
	if meta.lang != "en" {
		langs = append(langs, "en")
	}
	var out []textSource
	for _, lang := range langs {
		p := systemq.UpsertItemTextParams{ItemID: itemID, Lang: lang, Title: title,
			Overview: nullString(meta.description), Tagline: nullString(meta.subtitle)}
		titleKept, overviewKept, taglineKept := locked.text("title", lang), locked.text("overview", lang) || meta.description == "",
			locked.text("tagline", lang) || meta.subtitle == ""
		for _, t := range texts {
			if t.Lang != lang {
				continue
			}
			if titleKept {
				p.Title = t.Title
			}
			if overviewKept {
				p.Overview = t.Overview
			}
			if taglineKept {
				p.Tagline = t.Tagline
			}
		}
		p.SortKey = sortKey(lang, p.Title)
		if err := q.UpsertItemText(ctx, p); err != nil {
			return nil, fmt.Errorf("store audiobook text: %w", err)
		}
		if !titleKept && meta.title != "" {
			out = append(out, textSource{"title", lang})
		}
		if !overviewKept {
			out = append(out, textSource{"overview", lang})
		}
		if !taglineKept {
			out = append(out, textSource{"tagline", lang})
		}
	}
	return out, nil
}

// lockedSet parses items.locked_fields (a JSON array of column names).
func lockedSet(raw string) map[string]bool {
	var names []string
	_ = json.Unmarshal([]byte(raw), &names) // an unreadable value locks nothing
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[strings.TrimSpace(n)] = true
	}
	return out
}
