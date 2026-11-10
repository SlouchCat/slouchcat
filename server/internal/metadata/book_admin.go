package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The administration of the book sources (contract 1.26.0, concept 19 D5.2 and D6.6): the search
// and the comparison of the editor's tab «Zuordnen», the match in the editor, the refetch, and the
// page «Hörbücher: Quellen» (settings, states of the sources, numbers, match-all, the list
// «Prüfen»). Package admin validates the input and calls these behind its owner checks.

// bookSourceError maps an error of a book source onto the field errors of the contract: provider
// provider_paused, provider_not_configured or provider_unreachable, providerId not_found for an
// unknown entry. The source's own text stays out: an address could carry the Google key.
func bookSourceError(err error) error {
	var verr *model.ValidationError
	switch {
	case errors.Is(err, errSourcePaused):
		return verr.Add("provider", "provider_paused")
	case errors.Is(err, errNotConfigured):
		return verr.Add("provider", "provider_not_configured")
	case errors.Is(err, ErrNotFound):
		return verr.Add("providerId", "not_found")
	default:
		return verr.Add("provider", "provider_unreachable")
	}
}

// searchableBook returns the configured book source called provider.
func (s *Service) searchableBook(ctx context.Context, provider string) (BookProvider, error) {
	p, ok := s.bookProvider(provider)
	if !ok {
		return nil, (&model.ValidationError{}).Add("provider", codeProviderNotSearchable)
	}
	if !p.Configured(ctx) {
		return nil, (&model.ValidationError{}).Add("provider", "provider_not_configured")
	}
	return p, nil
}

// regionOf is the Audible region of a request; the other sources have none.
func regionOf(provider, region string) string {
	if provider != ProviderAudible {
		return ""
	}
	return region
}

// liveBook loads an audiobook that is not deleted; another kind is kind invalid_value.
func (s *Service) liveBook(ctx context.Context, itemID string) (systemq.Item, error) {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return systemq.Item{}, err
	}
	if item.Kind != "audiobook" {
		return systemq.Item{}, (&model.ValidationError{}).Add("kind", "invalid_value")
	}
	return item, nil
}

// bookTargetOf is the comparison with a book: its length and the tolerance of the settings.
func (s *Service) bookTargetOf(ctx context.Context, item systemq.Item) (bookTarget, error) {
	st, err := s.BookSettings(ctx)
	if err != nil {
		return bookTarget{}, err
	}
	return targetOf(item, st.Tolerance), nil
}

// SearchBookSource searches one book source (searchMetadata with kind audiobook). With an ItemID
// the candidates carry durationMatches and score and come best first. At most maxSearchResults.
// Errors: *model.ValidationError (provider provider_not_searchable, provider_not_configured,
// provider_paused or provider_unreachable, itemId invalid_value).
func (s *Service) SearchBookSource(ctx context.Context, q model.BookSearch) ([]model.MetadataCandidate, error) {
	p, err := s.searchableBook(ctx, q.Provider)
	if err != nil {
		return nil, err
	}
	var target *bookTarget
	lang := ""
	if q.ItemID != "" {
		item, err := s.liveBook(ctx, q.ItemID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) || errors.As(err, new(*model.ValidationError)) {
				return nil, (&model.ValidationError{}).Add("itemId", "invalid_value")
			}
			return nil, err
		}
		t, err := s.bookTargetOf(ctx, item)
		if err != nil {
			return nil, err
		}
		target, lang = &t, bookLangOf(item)
	}
	cs, err := p.SearchBooks(ctx, BookQuery{Title: q.Query, Author: q.Author, ASIN: q.ASIN, ISBN: q.ISBN,
		Region: regionOf(q.Provider, q.Region), Lang: lang})
	if err != nil {
		return nil, bookSourceError(err)
	}
	scored := make([]scoredBook, 0, len(cs))
	for _, c := range cs {
		if target != nil {
			scored = append(scored, target.score(c))
		} else {
			scored = append(scored, scoredBook{BookCandidate: c})
		}
	}
	if target != nil {
		sortBooks(scored)
	}
	out := make([]model.MetadataCandidate, 0, min(len(scored), maxSearchResults))
	for _, c := range scored {
		if len(out) == maxSearchResults {
			break
		}
		m := c.json().model()
		if target == nil {
			m.Score, m.DurationMatches = nil, nil
		}
		out = append(out, m)
	}
	return out, nil
}

// model maps a stored candidate onto the administration's view.
func (c bookCandidateJSON) model() model.MetadataCandidate {
	score := c.Score
	return model.MetadataCandidate{Provider: c.Provider, ProviderID: c.ProviderID, Title: c.Title, Year: c.Year,
		Subtitle: c.Subtitle, Authors: c.Authors, Narrators: c.Narrators, Publisher: c.Publisher, SeriesName: c.SeriesName,
		SeriesIndex: c.SeriesIndex, Language: c.Language, DurationMs: c.DurationMs, ASIN: c.ASIN, ISBN: c.ISBN,
		CoverURL: c.CoverURL, Region: c.Region, DurationMatches: c.DurationMatches, Score: &score}
}

// fetchBookEntry fetches an entry of a book source for the editor.
func (s *Service) fetchBookEntry(ctx context.Context, provider, providerID, region string) (BookRecord, error) {
	p, err := s.searchableBook(ctx, provider)
	if err != nil {
		return BookRecord{}, err
	}
	rec, err := p.Book(ctx, providerID, regionOf(provider, region))
	if err != nil {
		return BookRecord{}, bookSourceError(err)
	}
	return rec, nil
}

// currentCover is the cover of a book today as text: the chosen row, else the cover of its files,
// else the cover of its book source ("" without one).
func (s *Service) currentCover(ctx context.Context, item systemq.Item) (string, error) {
	rows, err := s.read.ListItemImagesByItem(ctx, item.ID)
	if err != nil {
		return "", fmt.Errorf("images of %s: %w", item.ID, err)
	}
	for _, r := range rows {
		if r.Kind == "poster" && r.Chosen == 1 {
			return unversioned(r.Source), nil
		}
	}
	cur, ok, err := s.bookCurrentCover(ctx, item)
	if err != nil || !ok {
		return "", err
	}
	return unversioned(cur.Source), nil
}

// previewFields compares every BookMatchField of a book with a record.
func (s *Service) previewFields(ctx context.Context, item systemq.Item, rec BookRecord) ([]model.MatchPreviewField, error) {
	v, err := loadBookView(ctx, s.read, item)
	if err != nil {
		return nil, err
	}
	cover, err := s.currentCover(ctx, item)
	if err != nil {
		return nil, err
	}
	out := make([]model.MatchPreviewField, 0, len(BookMatchFields))
	for _, f := range BookMatchFields {
		cur := v.current(f)
		if f == "cover" {
			cur = cover
		}
		prop := proposedOf(rec, f)
		out = append(out, model.MatchPreviewField{Field: f, Current: cur, Proposed: prop, Locked: v.locked(f), Differs: cur != prop})
	}
	return out, nil
}

// BookMatchPreview compares the entry providerID of a book source with an audiobook field by field
// (getMatchPreview); nothing is written. Errors: model.ErrNotFound, *model.ValidationError (kind
// invalid_value, provider provider_not_searchable, providerId not_found), the errors of the source.
func (s *Service) BookMatchPreview(ctx context.Context, itemID, provider, providerID, region string) (model.MatchPreview, error) {
	item, err := s.liveBook(ctx, itemID)
	if err != nil {
		return model.MatchPreview{}, err
	}
	rec, err := s.fetchBookEntry(ctx, provider, providerID, region)
	if err != nil {
		return model.MatchPreview{}, err
	}
	fields, err := s.previewFields(ctx, item, rec)
	if err != nil {
		return model.MatchPreview{}, err
	}
	out := model.MatchPreview{Provider: rec.Provider, ProviderID: rec.ID, Title: rec.Title, DurationMs: rec.DurationMs,
		CoverURL: bookCoverOf(rec), Fields: fields}
	t, err := s.bookTargetOf(ctx, item)
	if err != nil {
		return model.MatchPreview{}, err
	}
	if sc := t.score(rec.BookCandidate); sc.hasLength {
		out.DurationMatches = &sc.inTolerance
	}
	return out, nil
}

// ApplyBookMatch matches an audiobook to the entry providerID of a book source in the editor
// (setItemMatch with a book source), synchronously: the fields (nil = every field that differs, is
// not locked and has a value at the source) are written, locked and credited to the source; cover
// chooses the source's cover. The book is matched by hand (book_match_manual 1) and image.fetch
// follows. Errors: model.ErrNotFound, *model.ValidationError (kind invalid_value, provider
// provider_not_searchable, providerId not_found, fields invalid_value), the errors of the source.
// An empty list (not nil) confirms the match without taking a field: a book whose files already
// say what the entry says.
func (s *Service) ApplyBookMatch(ctx context.Context, itemID, provider, providerID, region string, fields []string) error {
	for _, f := range fields {
		if !slices.Contains(BookMatchFields, f) {
			return (&model.ValidationError{}).Add("fields", "invalid_value")
		}
	}
	item, err := s.liveBook(ctx, itemID)
	if err != nil {
		return err
	}
	rec, err := s.fetchBookEntry(ctx, provider, providerID, region)
	if err != nil {
		return err
	}
	take := map[string]bool{}
	if fields == nil {
		preview, err := s.previewFields(ctx, item, rec)
		if err != nil {
			return err
		}
		for _, f := range preview {
			take[f.Field] = f.Differs && !f.Locked && f.Proposed != ""
		}
	} else {
		for _, f := range fields {
			take[f] = true
		}
	}
	t, err := s.bookTargetOf(ctx, item)
	if err != nil {
		return err
	}
	if err := s.writeBookMatch(ctx, item.ID, rec, bookWrite{mode: bookWriteManual, fields: take,
		region: firstNonEmpty(rec.Region, regionOf(provider, region)), score: t.score(rec.BookCandidate).score}); err != nil {
		return err
	}
	return s.enqueueImages(ctx, item.ID, matchManualPriority)
}

// RefetchBook queues the refetch of a matched audiobook («Vom Anbieter neu holen», refreshItem with
// fromProvider): metadata.match with the payload refetch at the manual priority. Errors:
// model.ErrNotFound, *model.ValidationError (fromProvider not_matched).
func (s *Service) RefetchBook(ctx context.Context, itemID string) error {
	item, err := s.liveItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.Kind != "audiobook" || item.BookMatchState.String != BookStateMatched || !item.BookMatchProvider.Valid {
		return (&model.ValidationError{}).Add("fromProvider", "not_matched")
	}
	return s.enqueueBookMatch(ctx, itemPayload{ItemID: item.ID, Refetch: true}, matchManualPriority)
}

// BookSources returns the page «Hörbücher: Quellen» without the folders and the Google key, which
// package admin adds: the settings, the state of every source, the numbers per match state and
// whether a match job of a book is queued or running.
func (s *Service) BookSources(ctx context.Context) (model.AudiobookSources, error) {
	st, err := s.BookSettings(ctx)
	if err != nil {
		return model.AudiobookSources{}, err
	}
	out := model.AudiobookSources{Settings: model.BookSettings{AudibleRegion: st.AudibleRegion, AudnexusURL: st.AudnexusURL,
		AutoMatch: st.AutoMatch, TolerancePercent: st.Tolerance, NormalizeAuthors: st.NormalizeAuthors, NormalizeSeries: st.NormalizeSeries}}
	for _, name := range BookProviderNames {
		p, _ := s.bookProvider(name)
		state := model.AudiobookSourceState{ID: name, Configured: p.Configured(ctx)}
		if until, ok := s.bookSourcePausedUntil(ctx, name); ok {
			u := until.UTC()
			state.PausedUntil = &u
		}
		out.Sources = append(out.Sources, state)
	}
	counts, err := s.read.CountAudiobooksByMatchState(ctx)
	if err != nil {
		return model.AudiobookSources{}, fmt.Errorf("count audiobooks: %w", err)
	}
	for _, c := range counts {
		n := int(c.N)
		out.Stats.Total += n
		switch c.State {
		case BookStateMatched:
			out.Stats.Matched = n
		case BookStateReview:
			out.Stats.Review = n
		case BookStateNone:
			out.Stats.None = n
		case BookStateError:
			out.Stats.Error = n
		default:
			out.Stats.Pending += n
		}
	}
	running, err := s.read.CountActiveAudiobookMatches(ctx)
	if err != nil {
		return model.AudiobookSources{}, fmt.Errorf("count audiobook matches: %w", err)
	}
	out.Running = running > 0
	return out, nil
}

// UpdateBookSources stores the changed settings of the page «Hörbücher: Quellen» and the extra rows
// of the caller in one transaction (*model.ValidationError). After the commit, a changed switch
// «Vereinheitlichen» queues the unified names again and the switch of auto_match to all queues the
// match of every book without state.
func (s *Service) UpdateBookSources(ctx context.Context, upd model.BookSettingsUpdate) error {
	before, after, err := s.UpdateBookSettings(ctx, BookSettingsUpdate{AudibleRegion: upd.AudibleRegion,
		AudnexusURL: upd.AudnexusURL, AutoMatch: upd.AutoMatch, Tolerance: upd.TolerancePercent,
		NormalizeAuthors: upd.NormalizeAuthors, NormalizeSeries: upd.NormalizeSeries, Extra: upd.Extra})
	if err != nil {
		return err
	}
	if (before.NormalizeAuthors != after.NormalizeAuthors || before.NormalizeSeries != after.NormalizeSeries) && s.authorDisplay != nil {
		if err := s.authorDisplay(ctx); err != nil {
			return fmt.Errorf("queue the unified names: %w", err)
		}
	}
	if before.AutoMatch != BookAutoAll && after.AutoMatch == BookAutoAll {
		if _, err := s.QueueUnmatchedBooks(ctx); err != nil {
			return err
		}
	}
	return nil
}

// MatchAllBooks queues the match of every audiobook that is not matched and whose folder lists a
// book source («Jetzt alle ohne Zuordnung zuordnen»); it returns the number of books.
func (s *Service) MatchAllBooks(ctx context.Context) (int, error) {
	ids, err := s.read.ListAudiobooksToMatch(ctx)
	if err != nil {
		return 0, fmt.Errorf("list audiobooks to match: %w", err)
	}
	return s.QueueBookMatches(ctx, ids)
}

// BookReview returns the list «Prüfen»: the audiobooks in review or error by title, with their
// stored candidates best first.
func (s *Service) BookReview(ctx context.Context) ([]model.BookReviewEntry, error) {
	rows, err := s.read.ListAudiobookReview(ctx)
	if err != nil {
		return nil, fmt.Errorf("list audiobook review: %w", err)
	}
	out := make([]model.BookReviewEntry, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	cands, err := s.read.ListBookMatchCandidates(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list book candidates: %w", err)
	}
	byItem := map[string][]model.MetadataCandidate{}
	for _, c := range cands {
		var j bookCandidateJSON
		if err := json.Unmarshal([]byte(c.Candidate), &j); err != nil {
			s.log.Warn("stored book candidate unreadable", "item_id", c.ItemID, "rank", c.Rank)
			continue
		}
		byItem[c.ItemID] = append(byItem[c.ItemID], j.model())
	}
	for _, r := range rows {
		e := model.BookReviewEntry{ItemID: r.ID, Title: r.Title, Author: r.Author, LibraryID: r.LibraryID, State: r.State,
			Error: r.BookMatchError.String, Candidates: byItem[r.ID]}
		if e.Candidates == nil {
			e.Candidates = []model.MetadataCandidate{}
		}
		if r.DurationMs.Valid {
			d := r.DurationMs.Int64
			e.DurationMs = &d
		}
		if r.BookMatchedAt.Valid {
			t := time.UnixMilli(r.BookMatchedAt.Int64).UTC()
			e.MatchedAt = &t
		}
		out = append(out, e)
	}
	return out, nil
}
