package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Search (E-4, design 3.1) is the one title read that sqlc cannot express as a fixed statement: the
// filters are optional and combine. The builder below therefore assembles the statement from
// constant fragments, all bound by parameters, around a fixed body that always joins visible_items
// for the profile of the scope (plan 3.3: callers cannot leave the join out). It runs on the read
// pool of the database like the generated queries.
//
// Text search runs over the FTS5 table search_index (titles of every language, original title,
// author, narrator, series name; people; collections), kept by triggers in the same transaction as
// its source rows. Without a text, the filters alone browse the visible titles by title.

// Search limits: a page of titles holds at most MaxSearchPage titles (DefaultSearchPage without a
// limit); people and collections are the first SearchSideHits matches.
const (
	DefaultSearchPage = 50
	MaxSearchPage     = 100
	SearchSideHits    = 10
)

// MinSearchChars is the shortest text that searches (E-4: from the second character).
const MinSearchChars = 2

// searchKinds maps the media types of the contract to item kinds; without a media-type filter the
// search covers all six (never seasons and extras).
var searchKinds = map[string][]string{
	"movie":     {"movie"},
	"series":    {"series", "episode"},
	"audiobook": {"audiobook"},
	"podcast":   {"podcast", "podcast_episode"},
}

var allSearchKinds = []string{"movie", "series", "episode", "audiobook", "podcast", "podcast_episode"}

// maxSearchTerms caps the prefix terms of one search: each term is a prefix scan of the index.
const maxSearchTerms = 8

// MatchExpression turns user input into an FTS5 query: every word (a run of Unicode letters and
// digits) becomes a quoted prefix term, the terms are joined with AND. Words shorter than
// MinSearchChars are dropped when the input has a longer word (the prefix index starts at two
// characters, a one-character prefix walks every term of that character), and at most
// maxSearchTerms words count. It returns "" when the input has no word.
func MatchExpression(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	short := func(w string) bool { return utf8.RuneCountInString(w) < MinSearchChars }
	if slices.ContainsFunc(words, func(w string) bool { return !short(w) }) {
		words = slices.DeleteFunc(words, short)
	}
	terms := make([]string, 0, min(len(words), maxSearchTerms))
	for _, w := range words[:min(len(words), maxSearchTerms)] {
		terms = append(terms, `"`+strings.ReplaceAll(w, `"`, `""`)+`"*`)
	}
	return strings.Join(terms, " ")
}

// hasFilter reports whether q restricts the titles by anything but its text.
func hasFilter(q model.SearchQuery) bool {
	return q.Genre != "" || q.YearFrom != 0 || q.YearTo != 0 || q.MaxRating >= 0 || len(q.MediaTypes) > 0 || len(q.Availability) > 0
}

// stmt is a statement under construction: constant SQL fragments and their arguments.
type stmt struct {
	sql  strings.Builder
	args []any
}

func (b *stmt) add(fragment string, args ...any) {
	b.sql.WriteString(fragment)
	b.args = append(b.args, args...)
}

// in returns "(?, ?, …)" for n arguments.
func in(n int) string {
	return "(" + strings.TrimSuffix(strings.Repeat("?, ", n), ", ") + ")"
}

// titleFilters appends the filters of q to a statement whose items alias is i.
func titleFilters(b *stmt, s Scope, pid int64, q model.SearchQuery) {
	kinds := allSearchKinds
	if len(q.MediaTypes) > 0 {
		kinds = nil
		for _, t := range q.MediaTypes {
			for _, k := range searchKinds[t] {
				if !slices.Contains(kinds, k) {
					kinds = append(kinds, k)
				}
			}
		}
		if len(kinds) == 0 {
			kinds = []string{""} // only unknown media types: nothing matches
		}
	}
	args := make([]any, 0, len(kinds))
	for _, k := range kinds {
		args = append(args, k)
	}
	b.add(" AND i.kind IN "+in(len(kinds)), args...)
	if q.Genre != "" {
		b.add(" AND EXISTS (SELECT 1 FROM item_genres AS g WHERE g.item_id = i.id AND g.lang = ? AND g.name = ?)", s.language, q.Genre)
	}
	if q.YearFrom != 0 {
		b.add(" AND i.year >= ?", q.YearFrom)
	}
	if q.YearTo != 0 {
		b.add(" AND i.year <= ?", q.YearTo)
	}
	if q.MaxRating >= 0 {
		b.add(" AND i.effective_rating <= ?", q.MaxRating) // an unknown rating never passes
	}
	var avail []string
	var availArgs []any
	if slices.Contains(q.Availability, "local") {
		// A file is present: for a series or podcast, a file of one of its episodes the profile sees.
		avail = append(avail, `EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL)
			OR (i.kind IN ('series', 'podcast') AND EXISTS (
				SELECT 1 FROM items AS c
				JOIN visible_items AS vc ON vc.item_id = c.id AND vc.profile_id = ?
				JOIN media_files AS m ON m.item_id = c.id
				WHERE m.missing_since IS NULL AND c.deleted_at IS NULL
				  AND (c.parent_id = i.id OR c.parent_id IN (SELECT se.id FROM items AS se WHERE se.parent_id = i.id))))`)
		availArgs = append(availArgs, pid)
	}
	if slices.Contains(q.Availability, "download") {
		// Downloaded on the session's device for this profile (M5).
		avail = append(avail, `EXISTS (SELECT 1 FROM downloads AS d, json_each(d.allowed_profile_ids) AS a
			WHERE d.item_id = i.id AND d.device_id = ? AND d.state = 'ready' AND d.files_removed_at IS NULL AND a.value = ?)`)
		availArgs = append(availArgs, q.DeviceID, pid)
	}
	if len(avail) > 0 {
		b.add(" AND ("+strings.Join(avail, " OR ")+")", availArgs...)
	} else if len(q.Availability) > 0 {
		b.add(" AND 0") // only unknown availability values
	}
}

// Search searches the titles, people and collections the profile may see (E-4). With a text of at
// least MinSearchChars characters it matches words as prefixes over the search index, titles by
// relevance, then title; people and collections are the first SearchSideHits matches. Without a
// text it browses the titles by the filters, ordered by title, without people and collections.
// Neither text nor filter, or a text shorter than MinSearchChars, gives an empty result without a
// query. q.Limit is clamped to 1..MaxSearchPage (0 = DefaultSearchPage).
func (r *Reader) Search(ctx context.Context, s Scope, q model.SearchQuery) (model.SearchResult, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.SearchResult{}, err
	}
	out := model.SearchResult{Items: []model.ItemSummary{}, People: []model.PersonRef{}, Collections: []model.CollectionRef{}}
	text := strings.TrimSpace(q.Text)
	match := MatchExpression(text)
	browse := text == ""
	if (browse && !hasFilter(q)) || (!browse && (utf8.RuneCountInString(text) < MinSearchChars || match == "")) {
		return out, nil
	}
	offset, limit := max(q.Offset, 0), q.Limit
	if limit <= 0 {
		limit = DefaultSearchPage
	}
	limit = min(limit, MaxSearchPage)

	var ids, count stmt
	if browse {
		from := func(b *stmt, head string) {
			b.add(head+" FROM items AS i JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = ? WHERE 1", pid)
			titleFilters(b, s, pid, q)
		}
		from(&ids, "SELECT i.id")
		from(&count, "SELECT count(*)")
		ids.add(" ORDER BY i.sort_title, i.id LIMIT ? OFFSET ?", limit, offset)
	} else {
		from := func(b *stmt, head string) {
			b.add(head+" FROM search_index AS s JOIN items AS i ON i.id = s.item_id"+
				" JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = ?", pid)
			b.add(" WHERE s.search_index MATCH ? AND s.kind IN ('item', 'text')", match)
			titleFilters(b, s, pid, q)
		}
		from(&ids, "SELECT s.item_id")
		from(&count, "SELECT count(DISTINCT s.item_id)")
		ids.add(" GROUP BY s.item_id ORDER BY min(s.rank), i.sort_title, s.item_id LIMIT ? OFFSET ?", limit, offset)
	}
	order, err := r.titlePage(ctx, &count, &ids, &out.Total)
	if err != nil {
		return model.SearchResult{}, err
	}
	if out.Items, err = r.ordered(ctx, s, pid, order); err != nil {
		return model.SearchResult{}, err
	}
	if browse {
		return out, nil
	}

	people, err := r.searchPeople(ctx, pid, match)
	if err != nil {
		return model.SearchResult{}, err
	}
	out.People = people
	collections, err := textColumn(ctx, r.read, `SELECT s.ref_id FROM search_index AS s
		JOIN visible_items AS v ON v.item_id = s.item_id AND v.profile_id = ?
		WHERE s.search_index MATCH ? AND s.kind = 'collection'
		GROUP BY s.ref_id ORDER BY min(s.rank), s.ref_id LIMIT ?`, pid, match, SearchSideHits)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("search collections: %w", err)
	}
	if out.Collections, err = r.collectionsByID(ctx, s, pid, collections); err != nil {
		return model.SearchResult{}, err
	}
	return out, nil
}

// searchPeople returns the people whose name matches and who have a credit on a visible title.
func (r *Reader) searchPeople(ctx context.Context, pid int64, match string) ([]model.PersonRef, error) {
	rows, err := r.read.QueryContext(ctx, `SELECT p.id, p.name FROM search_index AS s
		JOIN visible_items AS v ON v.item_id = s.item_id AND v.profile_id = ?
		JOIN people AS p ON p.id = CAST(s.ref_id AS INTEGER)
		WHERE s.search_index MATCH ? AND s.kind = 'person'
		GROUP BY p.id ORDER BY min(s.rank), p.name, p.id LIMIT ?`, pid, match, SearchSideHits)
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	defer rows.Close()
	out := []model.PersonRef{}
	for rows.Next() {
		var p model.PersonRef
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	return out, nil
}

// titlePage runs the count and the page of a search in one read-only transaction, so the total
// and the page see the same state of the database; it returns the title ids of the page in order.
func (r *Reader) titlePage(ctx context.Context, count, ids *stmt, total *int) ([]string, error) {
	tx, err := r.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin search: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, count.sql.String(), count.args...).Scan(total); err != nil {
		return nil, fmt.Errorf("count search hits: %w", err)
	}
	order, err := textColumn(ctx, tx, ids.sql.String(), ids.args...)
	if err != nil {
		return nil, fmt.Errorf("search titles: %w", err)
	}
	return order, tx.Commit()
}

// textColumn runs a statement with one text column and returns its values in order.
func textColumn(ctx context.Context, db scopedq.DBTX, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ordered returns the summaries of visible titles in the order of ids.
func (r *Reader) ordered(ctx context.Context, s Scope, pid int64, ids []string) ([]model.ItemSummary, error) {
	if len(ids) == 0 {
		return []model.ItemSummary{}, nil
	}
	items, err := r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list search hits: %w", err)
	}
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	slices.SortFunc(items, func(a, b scopedq.Item) int { return pos[a.ID] - pos[b.ID] })
	return r.summaries(ctx, s, pid, items)
}

// MaxAdminHits caps the administration's title search.
const MaxAdminHits = 50

// AdminSearch finds titles for the administration (picks, collections): the same word matching as
// Search over titles of every language, without any profile filter, for movies, series, episodes,
// audiobooks, podcasts and podcast episodes that are not deleted, by relevance, then title. It
// accepts only the System scope (ErrNotSystem otherwise); reason goes to the debug log. limit is
// clamped to 1..MaxAdminHits (0 = 20). The title is the German one, then the English one, then the
// parsed title; ParentTitle names the series of an episode or the podcast of an episode.
func (r *Reader) AdminSearch(ctx context.Context, s Scope, text string, limit int) ([]model.AdminItemHit, error) {
	if !s.system {
		return nil, ErrNotSystem
	}
	r.log.DebugContext(ctx, "system scope read", "reason", s.reason)
	out := []model.AdminItemHit{}
	text = strings.TrimSpace(text)
	match := MatchExpression(text)
	if utf8.RuneCountInString(text) < MinSearchChars || match == "" {
		return out, nil
	}
	if limit <= 0 {
		limit = 20
	}
	limit = min(limit, MaxAdminHits)
	rows, err := r.read.QueryContext(ctx, `SELECT i.id, i.kind,
		       COALESCE((SELECT t.title FROM item_texts AS t WHERE t.item_id = i.id AND t.lang = 'de'),
		                (SELECT t.title FROM item_texts AS t WHERE t.item_id = i.id AND t.lang = 'en'), i.title),
		       i.year, i.library_id,
		       CASE i.kind
		         WHEN 'episode' THEN COALESCE((SELECT t.title FROM item_texts AS t WHERE t.item_id = gp.id AND t.lang = 'de'),
		                (SELECT t.title FROM item_texts AS t WHERE t.item_id = gp.id AND t.lang = 'en'), gp.title)
		         WHEN 'podcast_episode' THEN COALESCE((SELECT t.title FROM item_texts AS t WHERE t.item_id = pa.id AND t.lang = 'de'),
		                (SELECT t.title FROM item_texts AS t WHERE t.item_id = pa.id AND t.lang = 'en'), pa.title)
		       END
		FROM search_index AS s
		JOIN items AS i ON i.id = s.item_id
		LEFT JOIN items AS pa ON pa.id = i.parent_id
		LEFT JOIN items AS gp ON gp.id = pa.parent_id
		WHERE s.search_index MATCH ? AND s.kind IN ('item', 'text') AND i.deleted_at IS NULL
		  AND i.kind IN ('movie', 'series', 'episode', 'audiobook', 'podcast', 'podcast_episode')
		GROUP BY i.id ORDER BY min(s.rank), i.sort_title, i.id LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("admin search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var h model.AdminItemHit
		var year sql.NullInt64
		var parent sql.NullString
		if err := rows.Scan(&h.ID, &h.Kind, &h.Title, &year, &h.LibraryID, &parent); err != nil {
			return nil, fmt.Errorf("scan admin hit: %w", err)
		}
		if year.Valid {
			y := int(year.Int64)
			h.Year = &y
		}
		h.ParentTitle = parent.String
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin search: %w", err)
	}
	return out, nil
}
