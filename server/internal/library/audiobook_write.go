package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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
		SeriesIndex: nullVolume(m.volume), Explicit: boolInt(m.explicit), CoverPath: nullString(m.cover),
		OwnRating: rating, RatingSource: source, Now: sql.NullInt64{Int64: w.ms, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("insert audiobook: %w", err)
	}
	if m.description != "" || m.subtitle != "" {
		if err := w.q.UpsertItemText(ctx, systemq.UpsertItemTextParams{
			ItemID: id, Lang: m.lang, Title: title, Overview: nullString(m.description), Tagline: nullString(m.subtitle),
		}); err != nil {
			return "", fmt.Errorf("insert audiobook text: %w", err)
		}
	}
	w.ix().kinds[id] = "audiobook"
	w.ix().addFolder(id, info.folder)
	w.stats.items++
	return id, nil
}

// finishAudiobooks completes the books a scan batch touched (new, moved or missing parts). A
// failure is logged; the next probe or scan of the book completes it.
func (s *Service) finishAudiobooks(ctx context.Context, ids map[string]bool) {
	for id := range ids {
		if err := s.finishAudiobook(ctx, id); err != nil {
			s.log.WarnContext(ctx, "audiobook not completed", "item_id", id, "err", err)
		}
	}
}

// finishAudiobook brings a book up to date with its present parts. It numbers the parts in order
// (part_index 1..n; NULL for a book of one file). Once every part is probed (or failed to probe)
// it also writes the book-wide chapters (parts that failed to probe are left out), the duration
// (sum of the parts), the metadata (metadata.json > tags of the first part > folder names; locked
// fields and manual ratings stay), the cover (folder image, else the picture embedded in the first
// part that has one) and queues image.fetch for the cover. Anything but an undeleted audiobook is
// left alone.
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
	if done {
		root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, files[0].RootID)
		if err != nil {
			return fmt.Errorf("load library root: %w", err)
		}
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
	if done {
		if err := s.writeBook(ctx, q, item, lib, parts, meta); err != nil {
			return err
		}
	}
	return tx.Commit()
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
// probed, and queues image.fetch when it has a cover.
func (s *Service) writeBook(ctx context.Context, q *systemq.Queries, item systemq.Item, lib appq.Library, parts []bookPart, meta bookMeta) error {
	locked := lockedSet(item.LockedFields)
	title, sortTitle := item.Title, item.SortTitle
	if !locked["title"] && meta.title != "" {
		title, sortTitle = meta.title, SortTitle(meta.title)
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
		return fmt.Errorf("delete chapters: %w", err)
	}
	for i, c := range bookChapters(probed, title) {
		if err := q.InsertChapter(ctx, systemq.InsertChapterParams{
			ItemID: item.ID, Idx: int64(i), Title: c.title, StartMs: c.startMs, EndMs: c.endMs,
			MediaFileID: nullString(c.mediaFileID), FileStartMs: c.fileStartMs,
		}); err != nil {
			return fmt.Errorf("insert chapter: %w", err)
		}
	}

	p := systemq.UpdateAudiobookParams{
		ID: item.ID, Title: title, SortTitle: sortTitle, Year: item.Year, Genres: item.Genres, Author: item.Author,
		Narrator: item.Narrator, SeriesName: item.SeriesName, SeriesIndex: item.SeriesIndex, Explicit: boolInt(meta.explicit),
		CoverPath: nullString(meta.cover), OwnRating: item.OwnRating, RatingSource: item.RatingSource,
		DurationMs: sql.NullInt64{Int64: duration, Valid: len(probed) > 0}, Now: s.now().UnixMilli(),
	}
	set := func(field string, value string, dst *sql.NullString) {
		if !locked[field] && value != "" {
			*dst = nullString(value)
		}
	}
	set("author", meta.author, &p.Author)
	set("narrator", meta.narrator, &p.Narrator)
	set("series_name", meta.series, &p.SeriesName)
	if !locked["series_index"] && meta.volume != nil {
		p.SeriesIndex = nullVolume(meta.volume)
	}
	if !locked["year"] && meta.year > 0 {
		p.Year = nullYear(meta.year)
	}
	if !locked["genres"] && len(meta.genres) > 0 {
		g, err := genresJSON(meta.genres)
		if err != nil {
			return err
		}
		p.Genres = g
	}
	switch src := item.RatingSource.String; {
	case locked["own_rating"] || (src != "" && src != "library" && src != "tags"):
	default:
		p.OwnRating, p.RatingSource = bookRating(lib, meta.explicit)
	}
	if err := q.UpdateAudiobook(ctx, p); err != nil {
		return fmt.Errorf("update audiobook: %w", err)
	}
	if err := writeBookText(ctx, q, item.ID, title, meta, locked); err != nil {
		return err
	}
	if meta.cover != "" {
		return enqueueTx(ctx, q, s.now(), imageJob(item.ID))
	}
	return nil
}

// writeBookText stores the title, description and subtitle of a book in its language and, for a
// book in another language, in English as well: English is the fallback of every profile language
// (visibility.pickTexts), so an English profile sees a German book's description. Locked texts
// keep the stored value.
func writeBookText(ctx context.Context, q *systemq.Queries, itemID, title string, meta bookMeta, locked map[string]bool) error {
	texts, err := q.ListItemTextsByItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("list audiobook texts: %w", err)
	}
	langs := []string{meta.lang}
	if meta.lang != "en" {
		langs = append(langs, "en")
	}
	for _, lang := range langs {
		p := systemq.UpsertItemTextParams{ItemID: itemID, Lang: lang, Title: title,
			Overview: nullString(meta.description), Tagline: nullString(meta.subtitle)}
		for _, t := range texts {
			if t.Lang != lang {
				continue
			}
			if locked["title"] {
				p.Title = t.Title
			}
			if locked["overview"] || meta.description == "" {
				p.Overview = t.Overview
			}
			if locked["tagline"] || meta.subtitle == "" {
				p.Tagline = t.Tagline
			}
		}
		if err := q.UpsertItemText(ctx, p); err != nil {
			return fmt.Errorf("store audiobook text: %w", err)
		}
	}
	return nil
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
