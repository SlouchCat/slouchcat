package metadata

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library/audioseries"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The match of an audiobook against the sources of its folder (concept 19 D6.3, ADR 0031 decisions
// 3 to 6). The sources are the folder's list (library_roots.providers of the folder of the book's
// first part) without local, in its order, and only those that are configured. An ASIN or ISBN of
// the book goes to every source first (Audible with the region switch; DNB, Open Library and
// Google Books by ISBN); a hit through an identifier beats every hit through the title, so the
// title is asked only without one. Otherwise each source is asked for title and first author, with
// a second try with the cleaned title (bookCleanTitle). A candidate is a hit when its title is
// similar (bookTitleSimilarity at least 0.85, or one contains the other) and an author shares the
// last name of the book's first author. At a source with lengths (Audible) the hits within the
// tolerance of the files' length count, hits of the same title and length (±1 minute) as one: when
// exactly one remains the book is matched and the chain stops; else the hits are candidates. A
// source without lengths never matches on its own: its hits are candidates. Every candidate gets a
// score: 0.7 × length + 0.2 × title + 0.1 × author.
//
// The end state (items.book_match_state): matched; else error when a source failed (paused,
// blocked, unreachable), nothing is written to the book's fields; else review with the three best
// candidates; else none. The three best candidates are stored in every state
// (book_match_candidates, MetadataCandidate JSON). A book that is matched is never matched again
// automatically.

// Values of items.book_match_state.
const (
	BookStateMatched = "matched"
	BookStateReview  = "review"
	BookStateNone    = "none"
	BookStateError   = "error"
)

// Weights and thresholds of the match (D6.3).
const (
	bookWeightLength   = 0.7
	bookWeightTitle    = 0.2
	bookWeightAuthor   = 0.1
	bookTitleThreshold = 0.85
	bookSameLengthMs   = 60_000
	maxBookCandidates  = 3
	matchKeyPrefix     = "match:"
)

// bookCandidateJSON is a stored candidate (book_match_candidates.candidate): the fields of the
// contract's MetadataCandidate.
type bookCandidateJSON struct {
	Provider        string   `json:"provider"`
	ProviderID      string   `json:"providerId"`
	Title           string   `json:"title"`
	Year            *int     `json:"year,omitempty"`
	Subtitle        string   `json:"subtitle,omitempty"`
	Authors         []string `json:"authors,omitempty"`
	Narrators       []string `json:"narrators,omitempty"`
	Publisher       string   `json:"publisher,omitempty"`
	SeriesName      string   `json:"seriesName,omitempty"`
	SeriesIndex     *float64 `json:"seriesIndex,omitempty"`
	Language        string   `json:"language,omitempty"`
	DurationMs      int64    `json:"durationMs,omitempty"`
	ASIN            string   `json:"asin,omitempty"`
	ISBN            string   `json:"isbn,omitempty"`
	CoverURL        string   `json:"coverUrl,omitempty"`
	Region          string   `json:"region,omitempty"`
	DurationMatches *bool    `json:"durationMatches,omitempty"`
	Score           float64  `json:"score"`
}

// scoredBook is a candidate with its score against one book.
type scoredBook struct {
	BookCandidate
	score       float64
	title       float64 // similarity of the titles
	inTolerance bool    // a length within the tolerance of the files
	hasLength   bool    // both lengths are known
}

func (c scoredBook) json() bookCandidateJSON {
	out := bookCandidateJSON{Provider: c.Provider, ProviderID: c.ID, Title: c.Title, Year: c.Year, Subtitle: c.Subtitle,
		Authors: c.Authors, Narrators: c.Narrators, Publisher: c.Publisher, SeriesName: c.SeriesName,
		SeriesIndex: c.SeriesIndex, Language: c.Language, DurationMs: c.DurationMs, ASIN: c.ASIN, ISBN: c.ISBN,
		CoverURL: c.CoverURL, Region: c.Region, Score: roundScore(c.score)}
	if c.hasLength {
		m := c.inTolerance
		out.DurationMatches = &m
	}
	return out
}

func roundScore(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }

// bookTarget is what a book is compared with: its title, first author, length and tolerance.
type bookTarget struct {
	title, author string
	durationMs    int64
	tolerance     int // percent
}

func targetOf(item systemq.Item, tolerance int) bookTarget {
	return bookTarget{title: item.Title, author: item.Author.String, durationMs: item.DurationMs.Int64, tolerance: tolerance}
}

// score rates a candidate against the book (identifier true: a hit through ASIN or ISBN, whose
// title and author need not compare).
func (t bookTarget) score(c BookCandidate) scoredBook {
	out := scoredBook{BookCandidate: c, title: bookTitleSimilarity(t.title, c.Title)}
	length := 0.0
	if t.durationMs > 0 && c.DurationMs > 0 {
		out.hasLength = true
		window := float64(t.durationMs) * float64(t.tolerance) / 100
		diff := float64(absInt64(c.DurationMs - t.durationMs))
		out.inTolerance = diff <= window
		if window > 0 {
			length = max(0, 1-diff/window)
		}
	}
	author := 0.0
	if t.author != "" && bookAuthorMatches(t.author, c.Authors) {
		author = 1
	}
	out.score = bookWeightLength*length + bookWeightTitle*out.title + bookWeightAuthor*author
	return out
}

// hit reports whether a title candidate names the book: a similar title and a common last name
// (a book without author compares by title only).
func (t bookTarget) hit(c scoredBook) bool {
	return c.title >= bookTitleThreshold && (t.author == "" || bookAuthorMatches(t.author, c.Authors))
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// bookMatchResult is the outcome of the chain of sources.
type bookMatchResult struct {
	matched    *scoredBook
	candidates []scoredBook // best first, at most maxBookCandidates
	failure    string       // "<source>:<reason>" of the first source that failed
}

// bookSourcesOf returns the configured sources of the folder of a book's first present part, in
// the folder's order, without local.
func (s *Service) bookSourcesOf(ctx context.Context, item systemq.Item) ([]BookProvider, error) {
	parts, err := s.read.ListAudiobookParts(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("book sources of %s: parts: %w", item.ID, err)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, parts[0].RootID)
	if err != nil {
		return nil, fmt.Errorf("book sources of %s: root: %w", item.ID, err)
	}
	var out []BookProvider
	for _, name := range model.ParseProviders(root.Providers) {
		if p, ok := s.bookProvider(name); ok && p.Configured(ctx) {
			out = append(out, p)
		}
	}
	return out, nil
}

// bookPartsProbed reports whether every present part of a book is probed (or failed to probe):
// only then is its length known (finishAudiobook).
func (s *Service) bookPartsProbed(ctx context.Context, itemID string) (bool, error) {
	parts, err := s.read.ListAudiobookParts(ctx, itemID)
	if err != nil {
		return false, fmt.Errorf("parts of %s: %w", itemID, err)
	}
	for _, p := range parts {
		if !p.Probe.Valid && !p.ProbeError.Valid {
			return false, nil
		}
	}
	return len(parts) > 0, nil
}

// matchAudiobook is the audiobook branch of metadata.match. A book whose parts are not all probed
// only gets its cover decided (image.fetch), as before stage D3. Otherwise Refetch fetches the
// stored match again; Book, or the setting auto_match (new or all) for a book not tried yet or in
// error, runs the match. A matched book is never matched again here. image.fetch follows.
func (s *Service) matchAudiobook(ctx context.Context, item systemq.Item, p itemPayload) error {
	probed, err := s.bookPartsProbed(ctx, item.ID)
	if err != nil {
		return err
	}
	if probed {
		state := item.BookMatchState.String
		switch {
		case p.Refetch:
			if err := s.refetchBook(ctx, item); err != nil {
				return err
			}
		case state == BookStateMatched:
		case p.Book:
			if err := s.matchBook(ctx, item); err != nil {
				return err
			}
		case state == "" || state == BookStateError:
			st, err := s.BookSettings(ctx)
			if err != nil {
				return err
			}
			if st.AutoMatch != BookAutoOff {
				if err := s.matchBook(ctx, item); err != nil {
					return err
				}
			}
		}
	}
	return s.enqueueImages(ctx, item.ID, matchManualPriority)
}

// matchBook runs the chain of sources for one book and stores the outcome. A book without a
// configured source keeps its state.
func (s *Service) matchBook(ctx context.Context, item systemq.Item) error {
	sources, err := s.bookSourcesOf(ctx, item)
	if err != nil || len(sources) == 0 {
		return err
	}
	st, err := s.BookSettings(ctx)
	if err != nil {
		return err
	}
	target := targetOf(item, st.Tolerance)
	res, err := s.runBookChain(ctx, item, sources, target)
	if err != nil {
		return err
	}
	if res.matched != nil {
		m := *res.matched
		rec, err := s.bookRecord(ctx, m.BookCandidate)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// The details failed (Audnexus paused or unreachable): nothing is written.
			res.matched, res.failure = nil, failureOf(m.Provider, err)
		} else {
			return s.writeBookMatch(ctx, item.ID, rec, bookWrite{mode: bookWriteAuto, score: m.score, candidates: res.candidates})
		}
	}
	state := BookStateNone
	switch {
	case res.failure != "":
		state = BookStateError
	case len(res.candidates) > 0:
		state = BookStateReview
	}
	return s.storeBookState(ctx, item.ID, state, res)
}

// runBookChain asks the sources in order: the identifiers first, then title and author. It returns
// an error only for a cancelled context; a failing source is named in failure and the next source
// is asked.
func (s *Service) runBookChain(ctx context.Context, item systemq.Item, sources []BookProvider, target bookTarget) (bookMatchResult, error) {
	var res bookMatchResult
	var all []scoredBook
	fail := func(p BookProvider, err error) {
		if res.failure == "" {
			res.failure = failureOf(p.Name(), err)
		}
		s.log.Info("audiobook source failed", "item_id", item.ID, "provider", p.Name(), "reason", failureReason(err))
	}
	lang := bookLangOf(item)
	asin, isbn := strings.TrimSpace(item.Asin.String), normISBN(item.Isbn.String)
	if asin != "" || isbn != "" {
		for _, p := range sources {
			cs, err := p.SearchBooks(ctx, BookQuery{ASIN: asin, ISBN: isbn, Lang: lang})
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			if err != nil {
				fail(p, err)
				continue
			}
			var inTol []scoredBook
			for _, c := range cs {
				sc := target.score(c)
				all = append(all, sc)
				if p.HasDuration() && sc.inTolerance {
					inTol = append(inTol, sc)
				}
			}
			if one, ok := uniqueBook(inTol); ok {
				res.matched = &one
				res.candidates = bestBooks(all)
				return res, nil
			}
		}
		if len(all) > 0 {
			res.candidates = bestBooks(all)
			return res, nil
		}
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		res.candidates = bestBooks(all)
		return res, nil
	}
	author := firstAuthor(item.Author.String)
	for _, p := range sources {
		hits, err := s.titleHits(ctx, p, target, title, author, lang)
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if err != nil {
			fail(p, err)
			continue
		}
		all = append(all, hits...)
		if !p.HasDuration() {
			continue
		}
		var inTol []scoredBook
		for _, h := range hits {
			if h.inTolerance {
				inTol = append(inTol, h)
			}
		}
		if one, ok := uniqueBook(inTol); ok {
			res.matched = &one
			break
		}
	}
	res.candidates = bestBooks(all)
	return res, nil
}

// titleHits asks a source by title and author, and with the cleaned title when the first try has
// no hit.
func (s *Service) titleHits(ctx context.Context, p BookProvider, target bookTarget, title, author, lang string) ([]scoredBook, error) {
	try := func(t string) ([]scoredBook, error) {
		cs, err := p.SearchBooks(ctx, BookQuery{Title: t, Author: author, Lang: lang})
		if err != nil {
			return nil, err
		}
		var out []scoredBook
		for _, c := range cs {
			if sc := target.score(c); target.hit(sc) {
				out = append(out, sc)
			}
		}
		return out, nil
	}
	hits, err := try(title)
	if err != nil || len(hits) > 0 {
		return hits, err
	}
	if clean := bookCleanTitle(title); clean != "" && clean != title {
		return try(clean)
	}
	return nil, nil
}

// uniqueBook returns the one book among hits within the tolerance: hits of the same title whose
// lengths differ by at most a minute count as one (two ASINs of the same recording), the best
// scored of them stands for it.
func uniqueBook(hits []scoredBook) (scoredBook, bool) {
	if len(hits) == 0 {
		return scoredBook{}, false
	}
	sorted := slices.Clone(hits)
	sortBooks(sorted)
	first := sorted[0]
	for _, h := range sorted[1:] {
		if bookNormTitle(h.Title) != bookNormTitle(first.Title) || absInt64(h.DurationMs-first.DurationMs) > bookSameLengthMs {
			return scoredBook{}, false
		}
	}
	return first, true
}

// sortBooks orders candidates best first: score, then source order is kept.
func sortBooks(cs []scoredBook) {
	slices.SortStableFunc(cs, func(a, b scoredBook) int { return cmp.Compare(b.score, a.score) })
}

// bestBooks returns the best candidates, one per source and id.
func bestBooks(cs []scoredBook) []scoredBook {
	sorted := slices.Clone(cs)
	sortBooks(sorted)
	var out []scoredBook
	for _, c := range sorted {
		if slices.ContainsFunc(out, func(o scoredBook) bool { return o.Provider == c.Provider && o.ID == c.ID }) {
			continue
		}
		out = append(out, c)
		if len(out) == maxBookCandidates {
			break
		}
	}
	return out
}

// bookRecord fetches the record of a matched candidate: Audible's details from Audnexus (an entry
// Audnexus does not know keeps the fields of the catalog), the other sources' detail.
func (s *Service) bookRecord(ctx context.Context, c BookCandidate) (BookRecord, error) {
	p, ok := s.bookProvider(c.Provider)
	if !ok {
		return BookRecord{}, fmt.Errorf("book provider %q: %w", c.Provider, ErrNotFound)
	}
	rec, err := p.Book(ctx, c.ID, c.Region)
	if errors.Is(err, ErrNotFound) && c.Provider == ProviderAudible {
		return recordOfCandidate(c), nil
	}
	if err != nil {
		return BookRecord{}, err
	}
	// The catalog knows the length of the region it was found in; Audnexus may answer from another.
	if rec.DurationMs == 0 {
		rec.DurationMs = c.DurationMs
	}
	if rec.Region == "" {
		rec.Region = c.Region
	}
	if rec.CoverURL == "" && c.CoverURL != "" {
		rec.CoverURL = c.CoverURL
		rec.Covers = []ImageCandidate{{Kind: "poster", Source: c.CoverURL, Default: true}}
	}
	return rec, nil
}

// recordOfCandidate is the record of a candidate whose details are not to be had.
func recordOfCandidate(c BookCandidate) BookRecord {
	rec := BookRecord{BookCandidate: c}
	if c.CoverURL != "" {
		rec.Covers = []ImageCandidate{{Kind: "poster", Source: c.CoverURL, Default: true}}
	}
	return rec
}

// storeBookState records a run without match: the state, the time of the run, the reason of an
// error and the candidates (a run in error without candidates keeps the stored ones).
func (s *Service) storeBookState(ctx context.Context, itemID, state string, res bookMatchResult) error {
	now := s.now().UnixMilli()
	err := s.inTx(ctx, func(q *systemq.Queries) error {
		cur, err := q.GetItemSystem(ctx, itemID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if cur.BookMatchState.String == BookStateMatched || cur.DeletedAt.Valid {
			return nil // matched meanwhile (the editor), or gone
		}
		if err := q.SetBookMatch(ctx, systemq.SetBookMatchParams{State: nullString(state),
			MatchedAt: sql.NullInt64{Int64: now, Valid: true}, Error: nullString(res.failure), Now: now, ID: itemID}); err != nil {
			return err
		}
		if state == BookStateError && len(res.candidates) == 0 {
			return nil
		}
		return replaceBookCandidates(ctx, q, itemID, res.candidates)
	})
	if err != nil {
		return fmt.Errorf("store book match of %s: %w", itemID, err)
	}
	s.log.Info("audiobook not matched", "item_id", itemID, "state", state, "candidates", len(res.candidates))
	return nil
}

// replaceBookCandidates stores the candidates of a book (rank 1 to 3).
func replaceBookCandidates(ctx context.Context, q *systemq.Queries, itemID string, cs []scoredBook) error {
	if err := q.DeleteBookMatchCandidates(ctx, itemID); err != nil {
		return err
	}
	for i, c := range cs {
		raw, err := json.Marshal(c.json())
		if err != nil {
			return err
		}
		if err := q.InsertBookMatchCandidate(ctx, systemq.InsertBookMatchCandidateParams{ItemID: itemID, Rank: int64(i + 1),
			Provider: c.Provider, ProviderID: c.ID, Score: roundScore(c.score), Candidate: string(raw)}); err != nil {
			return err
		}
	}
	return nil
}

// failureReason names the kind of a source error without its text (an address could carry the
// Google key): paused, blocked, not_configured, region, unreachable.
func failureReason(err error) string {
	switch {
	case errors.Is(err, errSourcePaused):
		return "paused"
	case errors.Is(err, errSourceBlocked):
		return "blocked"
	case errors.Is(err, errNotConfigured):
		return "not_configured"
	case errors.Is(err, errBookRegion):
		return "region"
	default:
		return "unreachable"
	}
}

// failureOf is the stored reason of a run in error: "<source>:<reason>".
func failureOf(provider string, err error) string { return provider + ":" + failureReason(err) }

// bookLangOf is the language of a book for the sources: its original language as ISO 639-1.
func bookLangOf(item systemq.Item) string {
	return PrimaryLanguage(item.OriginalLanguage.String)
}

// firstAuthor is the first author of an author field (the part before «,», «;», «&», « und »,
// « and » or « / »), as written.
func firstAuthor(author string) string {
	a := strings.TrimSpace(author)
	lower := strings.ToLower(a)
	cut := len(a)
	for _, sep := range []string{",", ";", "&", " und ", " and ", " / "} {
		if i := strings.Index(lower, sep); i >= 0 && i < cut {
			cut = i
		}
	}
	return strings.TrimSpace(a[:cut])
}

// bookAuthorMatches reports whether one of the candidate's authors shares the last name of the
// book's first author (the key of package audioseries, accents folded).
func bookAuthorMatches(author string, candidates []string) bool {
	want := lastWord(foldAccents(audioseries.Author(author)))
	if want == "" {
		return false
	}
	for _, c := range candidates {
		if lastWord(foldAccents(audioseries.Author(c))) == want {
			return true
		}
	}
	return false
}

func lastWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// foldAccents drops the combining marks of s (NFD), so «Kängurus» and «Kangurus» compare equal.
func foldAccents(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return norm.NFC.String(b.String())
}

// bookFillers are the words a title comparison leaves out.
var bookFillers = map[string]bool{
	"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true, "ein": true, "eine": true,
	"einen": true, "und": true, "the": true, "a": true, "an": true, "and": true, "of": true,
}

// bookNormTitle is a title for the comparison: lower case, accents folded, punctuation as space,
// filler words left out.
func bookNormTitle(s string) string {
	s = strings.ToLower(foldAccents(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	words := strings.Fields(s)
	out := words[:0]
	for _, w := range words {
		if !bookFillers[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// bookTitleSimilarity compares two titles after bookNormTitle: 1 − edit distance / longer length,
// and at least the threshold when one contains the other.
func bookTitleSimilarity(a, b string) float64 {
	na, nb := bookNormTitle(a), bookNormTitle(b)
	if na == "" || nb == "" {
		return 0
	}
	ra, rb := []rune(na), []rune(nb)
	sim := 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
	if strings.Contains(" "+na+" ", " "+nb+" ") || strings.Contains(" "+nb+" ", " "+na+" ") {
		sim = max(sim, bookTitleThreshold)
	}
	return sim
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

var (
	bookBrackets  = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	bookNumbered  = regexp.MustCompile(`\s-\s*\d+\s*-\s`)
	bookLeadNum   = regexp.MustCompile(`^\s*\d+\s*[-._:]\s*`)
	bookAddition  = regexp.MustCompile(`(?i)\s*\b(live und ungekürzt|ungekürzte (lesung|fassung|ausgabe)|ungekürzt|hörbuch)\b`)
	bookTrailing  = regexp.MustCompile(`\s+[-–—:]\s+.*$|:\s*$`)
	bookSeparator = regexp.MustCompile(`\s+`)
)

// bookCleanTitle is the title of the second try (D6.3): brackets, «- …», «Live und ungekürzt»,
// «ungekürzt», «Hörbuch» and leading or separating numbers («-03-») left out.
func bookCleanTitle(title string) string {
	s := bookBrackets.ReplaceAllString(title, " ")
	s = bookNumbered.ReplaceAllString(" "+s+" ", " ")
	s = bookLeadNum.ReplaceAllString(strings.TrimSpace(s), "")
	s = bookAddition.ReplaceAllString(s, " ")
	s = bookTrailing.ReplaceAllString(strings.TrimSpace(s), "")
	s = bookSeparator.ReplaceAllString(s, " ")
	return strings.Trim(s, " -–—:,;")
}

// enqueueBookMatch queues metadata.match for a book with the payload p. A queued match job of the
// book at a lower priority or without the book's flags is replaced (a scan's plain match would
// swallow a refetch); a running one is left alone.
func (s *Service) enqueueBookMatch(ctx context.Context, p itemPayload, priority int) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("%s payload: %w", JobMatch, err)
	}
	nj := jobs.NewJob{Type: JobMatch, Priority: priority, IdempotencyKey: matchKeyPrefix + p.ItemID, Payload: payload}
	id, err := s.queue.Enqueue(ctx, nj)
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, p.ItemID, err)
	}
	j, err := s.queue.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, p.ItemID, err)
	}
	if j.State != jobs.StateQueued || (j.Priority >= priority && string(j.Payload) == string(payload)) {
		return nil
	}
	if _, err := s.queue.Cancel(ctx, id); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, p.ItemID, err)
	}
	if _, err := s.queue.Enqueue(ctx, nj); err != nil {
		return fmt.Errorf("enqueue %s %s: %w", JobMatch, p.ItemID, err)
	}
	return nil
}

// QueueBookMatches queues the book match (payload book) of the given books at the priority of the
// film match; it returns how many it queued.
func (s *Service) QueueBookMatches(ctx context.Context, ids []string) (int, error) {
	n := 0
	for _, id := range ids {
		if err := s.enqueueBookMatch(ctx, itemPayload{ItemID: id, Book: true}, matchPriority); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// QueueUnmatchedBooks queues the match of every book not tried yet whose folder lists a source (the
// switch to all on the page «Hörbücher: Quellen»).
func (s *Service) QueueUnmatchedBooks(ctx context.Context) (int, error) {
	rows, err := s.read.ListAudiobooksInMatchState(ctx, systemq.ListAudiobooksInMatchStateParams{State: ""})
	if err != nil {
		return 0, fmt.Errorf("list books without match: %w", err)
	}
	var ids []string
	for _, r := range rows {
		if len(model.ParseProviders(r.Providers)) > 1 {
			ids = append(ids, r.ID)
		}
	}
	return s.QueueBookMatches(ctx, ids)
}

// QueueFirstBookMatches queues the match of every book without match state once, at the first start
// with auto_match all (concept 19 D6.3 point 6, OD-4), and marks it done.
func (s *Service) QueueFirstBookMatches(ctx context.Context) (int, error) {
	done, err := s.bookSetting(ctx, SettingBookFirstRunQueued)
	if err != nil || done != "" {
		return 0, err
	}
	st, err := s.BookSettings(ctx)
	if err != nil || st.AutoMatch != BookAutoAll {
		return 0, err
	}
	n, err := s.QueueUnmatchedBooks(ctx)
	if err != nil {
		return n, err
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{Key: SettingBookFirstRunQueued, Value: "1",
		Now: s.now().UnixMilli()}); err != nil {
		return n, fmt.Errorf("write setting %s: %w", SettingBookFirstRunQueued, err)
	}
	if n > 0 {
		s.log.Info("audiobook match queued for every book", "books", n)
	}
	return n, nil
}

// queueErrorBooks queues the match of the books of a library in state error again, unless every
// source of their folder pauses (QueuePendingMetadata).
func (s *Service) queueErrorBooks(ctx context.Context, libraryID int64) (int, error) {
	rows, err := s.read.ListAudiobooksInMatchState(ctx, systemq.ListAudiobooksInMatchStateParams{State: BookStateError, LibraryID: libraryID})
	if err != nil {
		return 0, fmt.Errorf("list books in error: %w", err)
	}
	var ids []string
	for _, r := range rows {
		paused, listed := true, false
		for _, name := range model.ParseProviders(r.Providers) {
			p, ok := s.bookProvider(name)
			if !ok || !p.Configured(ctx) {
				continue
			}
			listed = true
			if _, rests := s.bookSourcePausedUntil(ctx, name); !rests {
				paused = false
			}
		}
		if listed && !paused {
			ids = append(ids, r.ID)
		}
	}
	return s.QueueBookMatches(ctx, ids)
}
