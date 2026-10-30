package visibility

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Library views of concept 08 (ADR 0016, E-1): the libraries of the side navigation and the
// library page. A library exists for a profile only through a visible top-level title (films,
// series, audiobooks, podcasts), derived from visible_items, never from a second copy of the
// library rights (concept 08, 4.3); an unknown, a not permitted and an empty library are all
// model.ErrNotFound (404, never 403).
//
// The page (listLibraryItems) has optional filters, eight sort orders and a status per profile,
// which sqlc cannot express as one fixed statement. Like Search it is assembled from constant
// fragments, all values bound as parameters, around a fixed body that always joins visible_items
// for the profile of the scope (queries/scoped/library.sql holds the fixed statements). The page,
// its total and its facets are read in one read-only transaction.

// Pages of the library page and of the page "Cat's recommendations": at most MaxLibraryPage
// titles, DefaultLibraryPage without a limit.
const (
	DefaultLibraryPage = 50
	MaxLibraryPage     = 100
)

// libraryPage clamps offset to 0 and above and limit to 1..MaxLibraryPage (0 or less =
// DefaultLibraryPage).
func libraryPage(offset, limit int) (int, int) {
	if limit <= 0 {
		limit = DefaultLibraryPage
	}
	return max(offset, 0), min(limit, MaxLibraryPage)
}

// rootContents is the RootContent enum in contract order. Each content maps to the item kind of
// the same name (the chips "films / series" of the library page).
var rootContents = []string{model.RootMovie, model.RootSeries, model.RootAudiobook, model.RootPodcast}

// topKinds are the item kinds of the top level of a library, the titles the page lists and
// itemCount counts (never seasons, episodes or extras).
var topKinds = []string{"movie", "series", "audiobook", "podcast"}

// VisibleLibraries returns the libraries with at least one visible top-level title for the
// profile in administration order (listVisibleLibraries), each with its folder contents in enum
// order and the number of its visible top-level titles.
func (r *Reader) VisibleLibraries(ctx context.Context, s Scope) ([]model.LibraryNav, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListVisibleLibraries(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("list visible libraries: %w", err)
	}
	contents, err := r.libraryContents(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make([]model.LibraryNav, 0, len(rows))
	for _, l := range rows {
		out = append(out, model.LibraryNav{ID: l.ID, Name: l.Name, Kind: l.Kind, Contents: orderContents(l.Kind, contents[l.ID]), ItemCount: int(l.ItemCount)})
	}
	return out, nil
}

// libraryContents returns the folder contents of the libraries with a visible title, by library.
func (r *Reader) libraryContents(ctx context.Context, pid int64) (map[int64]map[string]bool, error) {
	rows, err := r.q.ListVisibleLibraryContents(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("list library contents: %w", err)
	}
	out := map[int64]map[string]bool{}
	for _, row := range rows {
		if out[row.LibraryID] == nil {
			out[row.LibraryID] = map[string]bool{}
		}
		out[row.LibraryID][row.Content] = true
	}
	return out, nil
}

// orderContents returns the contents of a library in enum order. A podcast library holds the feed
// subscriptions, which have no folder (H-2), so it always holds podcasts.
func orderContents(libraryKind string, set map[string]bool) []string {
	out := make([]string, 0, len(rootContents))
	for _, c := range rootContents {
		if set[c] || (c == model.RootPodcast && libraryKind == model.LibraryPodcasts) {
			out = append(out, c)
		}
	}
	return out
}

// librarySortColumns are the columns of the CTE lib (see libraryStmt) the sort orders other than
// title order by; NULLs (and an empty author) come last in either direction.
var librarySortColumns = map[string]string{
	model.LibrarySortAdded:    "added_at",
	model.LibrarySortReleased: "release_date",
	model.LibrarySortVote:     "vote_average",
	model.LibrarySortRuntime:  "runtime_minutes",
	model.LibrarySortAge:      "effective_rating",
	model.LibrarySortPlayed:   "played_at",
	model.LibrarySortAuthor:   "NULLIF(author, '')",
}

// libraryFilter is a validated LibraryQuery.
type libraryFilter struct {
	model.LibraryQuery
	desc  bool
	kinds []string
}

// validateLibraryQuery checks what does not depend on the library: sort, order, status and letter.
// It returns the query with its defaults applied.
func validateLibraryQuery(q model.LibraryQuery) (libraryFilter, *model.ValidationError) {
	var verr *model.ValidationError
	if q.Sort == "" {
		q.Sort = model.LibrarySortTitle
	}
	if !slices.Contains(model.LibrarySorts, q.Sort) {
		verr = verr.Add("sort", "invalid_value")
	}
	f := libraryFilter{LibraryQuery: q, desc: !model.LibrarySortAscending(q.Sort)}
	switch q.Order {
	case "":
	case "asc":
		f.desc = false
	case "desc":
		f.desc = true
	default:
		verr = verr.Add("order", "invalid_value")
	}
	for _, st := range q.Status {
		if st != model.LibraryStatusUnwatched && st != model.LibraryStatusStarted && st != model.LibraryStatusWatched {
			verr = verr.Add("status", "invalid_value")
			break
		}
	}
	if q.Letter != "" {
		switch {
		case len(q.Letter) != 1 || (q.Letter != "#" && (q.Letter[0] < 'A' || q.Letter[0] > 'Z')):
			verr = verr.Add("letter", "invalid_value")
		case q.Sort != model.LibrarySortTitle:
			verr = verr.Add("letter", "letter_requires_title_sort")
		}
	}
	f.Offset, f.Limit = libraryPage(q.Offset, q.Limit)
	return f, verr
}

// LibraryItems returns one page of the visible top-level titles of a library with its total and
// facets (listLibraryItems, concept 08, 4.2). A library without a visible top-level title for the
// profile (unknown, not permitted or empty) is model.ErrNotFound; an invalid query is a
// *model.ValidationError: sort, order, status or letter outside their values (invalid_value),
// letter without sort title (letter_requires_title_sort), sort author outside an audiobook library
// and content outside the library's folder contents (invalid_value).
//
// Sort title orders by the sort key of the title in the profile language (item_texts.sort_key),
// the English one, then items.sort_title; every order breaks ties by that key and then the id.
// Status is exclusive per title: films and audiobooks by their own progress, series and podcasts
// by their visible episodes (watched = at least one and all completed, started = some progress
// and not all completed, unwatched otherwise); series and podcasts carry it as Progress (watched:
// Completed, started: not completed, both at the time of the latest episode progress).
// MaxRating lowers the ceiling of visible_items for this page (an unknown rating never passes
// it). Facets: the letters of the filtered set without the letter filter (A..Z, then #), the
// genres of the library's visible top-level titles in the profile language and their known
// ratings, all at or below the profile's ceiling.
func (r *Reader) LibraryItems(ctx context.Context, s Scope, libraryID int64, q model.LibraryQuery) (model.LibraryPage, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.LibraryPage{}, err
	}
	f, verr := validateLibraryQuery(q)
	if verr != nil {
		return model.LibraryPage{}, verr
	}
	lib, err := r.q.GetVisibleLibrary(ctx, scopedq.GetVisibleLibraryParams{ProfileID: pid, LibraryID: libraryID})
	if err != nil {
		return model.LibraryPage{}, notFound(err)
	}
	contents, err := r.libraryContents(ctx, pid)
	if err != nil {
		return model.LibraryPage{}, err
	}
	if f.Sort == model.LibrarySortAuthor && lib.Kind != model.LibraryAudiobooks {
		verr = verr.Add("sort", "invalid_value")
	}
	f.kinds = topKinds
	if f.Content != "" {
		if !slices.Contains(orderContents(lib.Kind, contents[libraryID]), f.Content) {
			verr = verr.Add("content", "invalid_value")
		}
		f.kinds = []string{f.Content} // a content names the item kind of the same name
	}
	if verr != nil {
		return model.LibraryPage{}, verr
	}

	tx, err := r.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.LibraryPage{}, fmt.Errorf("begin library page: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	out := model.LibraryPage{Facets: model.LibraryFacets{Letters: []string{}, Genres: []model.GenreRef{}, Ratings: []int{}}}
	count := libraryStmt(s, pid, libraryID, f, "SELECT count(*) FROM lib WHERE 1", true)
	if err := tx.QueryRowContext(ctx, count.sql.String(), count.args...).Scan(&out.Total); err != nil {
		return model.LibraryPage{}, fmt.Errorf("count library titles: %w", err)
	}
	pageStmt := libraryStmt(s, pid, libraryID, f, "SELECT id, kind FROM lib WHERE 1", true)
	pageStmt.add(" ORDER BY " + libraryOrderBy(f))
	pageStmt.add(" LIMIT ? OFFSET ?", f.Limit, f.Offset)
	ids, containers, err := libraryIDs(ctx, tx, pageStmt)
	if err != nil {
		return model.LibraryPage{}, err
	}
	stats, err := episodeStats(ctx, tx, pid, libraryID, containers)
	if err != nil {
		return model.LibraryPage{}, err
	}
	letters := libraryStmt(s, pid, libraryID, f, "SELECT DISTINCT lower(substr(sort_key, 1, 1)) FROM lib WHERE 1", false)
	firsts, err := textColumn(ctx, tx, letters.sql.String(), letters.args...)
	if err != nil {
		return model.LibraryPage{}, fmt.Errorf("library letters: %w", err)
	}
	out.Facets.Letters = sortLetters(firsts)
	qtx := r.q.WithTx(tx)
	genres, err := qtx.ListLibraryGenres(ctx, scopedq.ListLibraryGenresParams{ProfileID: pid, Lang: s.language, LibraryID: libraryID})
	if err != nil {
		return model.LibraryPage{}, fmt.Errorf("list library genres: %w", err)
	}
	for _, g := range genres {
		out.Facets.Genres = append(out.Facets.Genres, model.GenreRef{Name: g.Name, Count: int(g.ItemCount)})
	}
	ratings, err := qtx.ListLibraryRatings(ctx, scopedq.ListLibraryRatingsParams{ProfileID: pid, LibraryID: libraryID})
	if err != nil {
		return model.LibraryPage{}, fmt.Errorf("list library ratings: %w", err)
	}
	for _, rt := range ratings {
		out.Facets.Ratings = append(out.Facets.Ratings, int(rt.Int64))
	}
	if err := tx.Commit(); err != nil {
		return model.LibraryPage{}, fmt.Errorf("commit library page: %w", err)
	}

	if out.Items, err = r.ordered(ctx, s, pid, ids); err != nil {
		return model.LibraryPage{}, err
	}
	for i := range out.Items {
		it := &out.Items[i]
		if it.Kind == "series" || it.Kind == "podcast" {
			it.Progress = stats[it.ID].progress()
		}
	}
	return out, nil
}

// libraryStmt returns a statement over the filtered set of a library page: the CTE lib with one
// row per visible top-level title of the content kinds and the filters genre, years and maxRating,
// followed by head (a SELECT from lib ending in a WHERE clause) and the filters status and, with
// withLetter, letter. lib carries the sort key and the sort columns; with a status filter or the
// sort order played also the status and the time of the latest progress, from the CTE ep over the
// visible episodes of the library's series and podcasts.
func libraryStmt(s Scope, pid, libraryID int64, f libraryFilter, head string, withLetter bool) *stmt {
	b := &stmt{}
	withEp := len(f.Status) > 0 || f.Sort == model.LibrarySortPlayed
	b.add("WITH ")
	if withEp {
		b.add("ep AS (")
		episodeStatsBody(b, pid, libraryID)
		b.add(" GROUP BY COALESCE(se.parent_id, e.parent_id)), ")
	}
	b.add(`lib AS (SELECT i.id, i.kind,
		COALESCE(NULLIF(tl.sort_key, ''), NULLIF(te.sort_key, ''), i.sort_title, '') AS sort_key,
		i.added_at, i.release_date, i.vote_average, i.runtime_minutes, i.effective_rating, i.author`)
	if withEp {
		b.add(`,
		CASE WHEN i.kind IN ('series', 'podcast') THEN
		       CASE WHEN ep.n > 0 AND ep.done = ep.n THEN 'watched' WHEN ep.touched > 0 THEN 'started' ELSE 'unwatched' END
		     ELSE
		       CASE WHEN p.completed = 1 THEN 'watched' WHEN p.position_ms > 0 THEN 'started' ELSE 'unwatched' END
		END AS status,
		CASE WHEN i.kind IN ('series', 'podcast') THEN ep.played_at ELSE p.updated_at END AS played_at`)
	}
	b.add(`
		FROM items AS i
		JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = ?
		LEFT JOIN item_texts AS tl ON tl.item_id = i.id AND tl.lang = ?
		LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'`, pid, s.language)
	if withEp {
		b.add(`
		LEFT JOIN progress AS p ON p.profile_id = ? AND p.item_id = i.id
		LEFT JOIN ep ON ep.top_id = i.id`, pid)
	}
	kinds := make([]any, 0, len(f.kinds))
	for _, k := range f.kinds {
		kinds = append(kinds, k)
	}
	b.add(" WHERE i.library_id = ? AND i.kind IN "+in(len(kinds)), append([]any{libraryID}, kinds...)...)
	if f.Genre != "" {
		b.add(" AND EXISTS (SELECT 1 FROM item_genres AS g WHERE g.item_id = i.id AND g.lang = ? AND g.name = ?)", s.language, f.Genre)
	}
	if f.YearFrom != 0 {
		b.add(" AND i.year >= ?", f.YearFrom)
	}
	if f.YearTo != 0 {
		b.add(" AND i.year <= ?", f.YearTo)
	}
	if f.MaxRating >= 0 {
		b.add(" AND i.effective_rating <= ?", f.MaxRating) // visible_items keeps the profile's ceiling; unknown never passes
	}
	b.add(") " + head)
	if len(f.Status) > 0 {
		status := make([]any, 0, len(f.Status))
		for _, st := range f.Status {
			status = append(status, st)
		}
		b.add(" AND status IN "+in(len(status)), status...)
	}
	if withLetter && f.Letter != "" {
		if f.Letter == "#" {
			b.add(" AND NOT (lower(substr(sort_key, 1, 1)) BETWEEN 'a' AND 'z')")
		} else {
			b.add(" AND lower(substr(sort_key, 1, 1)) = ?", strings.ToLower(f.Letter))
		}
	}
	return b
}

// libraryOrderBy returns the ORDER BY terms of f (columns of lib, chosen from constants).
func libraryOrderBy(f libraryFilter) string {
	dir := " ASC"
	if f.desc {
		dir = " DESC"
	}
	if f.Sort == model.LibrarySortTitle {
		return "sort_key" + dir + ", id" + dir
	}
	col := librarySortColumns[f.Sort]
	return col + " IS NULL, " + col + dir + ", sort_key, id"
}

// libraryIDs runs the page statement and returns the title ids in order and the ids of the series
// and podcasts among them.
func libraryIDs(ctx context.Context, tx *sql.Tx, b *stmt) ([]string, []string, error) {
	rows, err := tx.QueryContext(ctx, b.sql.String(), b.args...)
	if err != nil {
		return nil, nil, fmt.Errorf("library titles: %w", err)
	}
	defer rows.Close()
	var ids, containers []string
	for rows.Next() {
		var id, kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, nil, fmt.Errorf("scan library title: %w", err)
		}
		ids = append(ids, id)
		if kind == "series" || kind == "podcast" {
			containers = append(containers, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("library titles: %w", err)
	}
	return ids, containers, nil
}

// episodeStatsBody appends the aggregate over the visible episodes of a library per series or
// podcast (top_id): their number, the completed ones, those with any progress and the time of the
// latest progress of the profile. An episode's parent is its season (whose parent is the series)
// or its podcast.
func episodeStatsBody(b *stmt, pid, libraryID int64) {
	b.add(`SELECT COALESCE(se.parent_id, e.parent_id) AS top_id, count(*) AS n,
		sum(COALESCE(pe.completed, 0)) AS done,
		sum(CASE WHEN pe.position_ms > 0 OR pe.completed = 1 THEN 1 ELSE 0 END) AS touched,
		max(pe.updated_at) AS played_at
		FROM items AS e
		JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = ?
		LEFT JOIN items AS se ON se.id = e.parent_id AND se.kind = 'season'
		LEFT JOIN progress AS pe ON pe.profile_id = ? AND pe.item_id = e.id
		WHERE e.library_id = ? AND e.kind IN ('episode', 'podcast_episode')`, pid, pid, libraryID)
}

// episodeStat is the episode aggregate of one series or podcast.
type episodeStat struct {
	n, done, touched int
	playedAt         sql.NullInt64
}

// progress returns the progress a series or podcast shows for its status: completed when every
// visible episode is (watched), not completed when some episode has progress (started), nil
// otherwise (unwatched); positions stay 0, UpdatedAt is the latest episode progress.
func (e episodeStat) progress() *model.Progress {
	if e.touched == 0 { // done counts completed episodes, which are touched too
		return nil
	}
	return &model.Progress{Completed: e.n > 0 && e.done == e.n, UpdatedAt: time.UnixMilli(e.playedAt.Int64).UTC()}
}

// episodeStats returns the episode aggregates of the given series and podcasts of a library.
func episodeStats(ctx context.Context, tx *sql.Tx, pid, libraryID int64, ids []string) (map[string]episodeStat, error) {
	out := make(map[string]episodeStat, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var b stmt
	b.add("SELECT top_id, n, done, touched, played_at FROM (")
	episodeStatsBody(&b, pid, libraryID)
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	b.add(" AND COALESCE(se.parent_id, e.parent_id) IN "+in(len(args)), args...)
	b.add(" GROUP BY COALESCE(se.parent_id, e.parent_id))")
	rows, err := tx.QueryContext(ctx, b.sql.String(), b.args...)
	if err != nil {
		return nil, fmt.Errorf("episode progress: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var e episodeStat
		if err := rows.Scan(&id, &e.n, &e.done, &e.touched, &e.playedAt); err != nil {
			return nil, fmt.Errorf("scan episode progress: %w", err)
		}
		out[id] = e
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("episode progress: %w", err)
	}
	return out, nil
}

// sortLetters maps the first characters of sort keys to the letters of the letter bar
// (model.SortLetter), each once, A..Z and then #.
func sortLetters(firsts []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range firsts {
		if l := model.SortLetter(c); !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		switch {
		case a == b:
			return 0
		case a == "#":
			return 1
		case b == "#":
			return -1
		}
		return strings.Compare(a, b)
	})
	return out
}
