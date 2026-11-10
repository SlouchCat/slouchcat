package admin

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The book sources of audiobooks (contract 1.26.0, concept 19 D5.2 and D6.6): the search, the
// comparison and the match in the editor's tab «Zuordnen», the refetch, and the page «Hörbücher:
// Quellen». Package metadata does the work (BookSources); this file checks the input, adds the
// folders and the Google Books key, and answers with the administration views.

// BookSources is implemented by package metadata (book_admin.go).
type BookSources interface {
	SearchBookSource(ctx context.Context, q model.BookSearch) ([]model.MetadataCandidate, error)
	BookMatchPreview(ctx context.Context, itemID, provider, providerID, region string) (model.MatchPreview, error)
	ApplyBookMatch(ctx context.Context, itemID, provider, providerID, region string, fields []string) error
	RefetchBook(ctx context.Context, itemID string) error
	BookSources(ctx context.Context) (model.AudiobookSources, error)
	UpdateBookSources(ctx context.Context, upd model.BookSettingsUpdate) error
	MatchAllBooks(ctx context.Context) (int, error)
	BookReview(ctx context.Context) ([]model.BookReviewEntry, error)
}

// WithBooks sets the book sources of package metadata and returns the receiver.
func (s *Service) WithBooks(b BookSources) *Service {
	s.books = b
	return s
}

// bookProviders are the providers that search and match audiobooks.
var bookProviders = []string{model.ProviderAudible, model.ProviderDNB, model.ProviderAppleBooks,
	model.ProviderOpenLibrary, model.ProviderGoogleBooks}

// audibleRegions are the Audible marketplaces of the contract (AudibleRegion).
var audibleRegions = []string{"de", "com", "co.uk", "fr", "it", "es", "ca", "com.au", "in", "co.jp"}

var (
	bookIDPattern    = regexp.MustCompile(`^[A-Za-z0-9:/._-]{1,64}$`)
	searchASIN       = regexp.MustCompile(`^[A-Za-z0-9]{10}$`)
	googleKeySetting = providerKey{field: "googleBooksKey", setting: SettingGoogleBooksKey, env: EnvGoogleBooksKey}
)

// validRegion checks an optional Audible region.
func validRegion(verr *model.ValidationError, region string) *model.ValidationError {
	if region != "" && !slices.Contains(audibleRegions, region) {
		return verr.Add("region", "invalid_value")
	}
	return verr
}

// SearchBooks searches a book source for an audiobook (searchMetadata with kind audiobook): by
// query and author, or by ASIN or ISBN; at least one of query, asin and isbn.
func (s *Service) SearchBooks(ctx context.Context, q model.BookSearch) ([]model.MetadataCandidate, error) {
	if s.books == nil {
		return nil, ErrNotConfigured
	}
	q.Query, q.Author = strings.TrimSpace(q.Query), strings.TrimSpace(q.Author)
	q.ASIN, q.ISBN = strings.ToUpper(strings.TrimSpace(q.ASIN)), strings.TrimSpace(q.ISBN)
	var verr *model.ValidationError
	if !slices.Contains(bookProviders, q.Provider) {
		verr = verr.Add("provider", "provider_not_searchable")
	}
	switch {
	case q.Query == "" && q.ASIN == "" && q.ISBN == "":
		verr = verr.Add("query", "required")
	case utf8.RuneCountInString(q.Query) > maxSearchQuery:
		verr = verr.Add("query", "too_long")
	}
	if utf8.RuneCountInString(q.Author) > maxSearchQuery {
		verr = verr.Add("author", "too_long")
	}
	if q.ASIN != "" && !searchASIN.MatchString(q.ASIN) {
		verr = verr.Add("asin", "invalid_value")
	}
	if q.ISBN != "" {
		isbn, ok := normalISBN(q.ISBN)
		if !ok || isbn == "" {
			verr = verr.Add("isbn", "invalid_value")
		}
		q.ISBN = isbn
	}
	if verr = validRegion(verr, q.Region); verr != nil {
		return nil, verr
	}
	out, err := s.books.SearchBookSource(ctx, q)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.MetadataCandidate{}
	}
	return out, nil
}

// validBookEntry checks provider, id and region of an entry of a book source.
func validBookEntry(provider, providerID, region string) *model.ValidationError {
	var verr *model.ValidationError
	if !slices.Contains(bookProviders, provider) {
		verr = verr.Add("provider", "provider_not_searchable")
	}
	if !bookIDPattern.MatchString(providerID) {
		verr = verr.Add("providerId", "invalid_value")
	}
	return validRegion(verr, region)
}

// GetMatchPreview compares an entry of a book source with an audiobook field by field.
func (s *Service) GetMatchPreview(ctx context.Context, itemID, provider, providerID, region string) (model.MatchPreview, error) {
	if s.books == nil {
		return model.MatchPreview{}, ErrNotConfigured
	}
	providerID = strings.TrimSpace(providerID)
	if verr := validBookEntry(provider, providerID, region); verr != nil {
		return model.MatchPreview{}, verr
	}
	return s.books.BookMatchPreview(ctx, itemID, provider, providerID, region)
}

// SetBookMatch matches an audiobook to an entry of a book source in the editor (setItemMatch with a
// book source): the fields (nil = every field that differs and is not locked) are written and
// locked at once; the answer is the administration view after it.
func (s *Service) SetBookMatch(ctx context.Context, itemID, provider, providerID, region string, fields []string) (model.AdminItem, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	if s.books == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	providerID = strings.TrimSpace(providerID)
	if verr := validBookEntry(provider, providerID, region); verr != nil {
		return model.AdminItem{}, verr
	}
	it, err := s.liveItem(ctx, s.sys, itemID)
	if err != nil {
		return model.AdminItem{}, err
	}
	if it.Kind != "audiobook" {
		return model.AdminItem{}, (&model.ValidationError{}).Add("provider", "invalid_value")
	}
	if err := s.books.ApplyBookMatch(ctx, itemID, provider, providerID, region, fields); err != nil {
		return model.AdminItem{}, err
	}
	s.log.InfoContext(ctx, "audiobook matched manually", "item_id", itemID, "provider", provider)
	return s.GetAdminItem(ctx, itemID)
}

// RefreshItemFromProvider fetches the entry of the stored match of an audiobook again (refreshItem
// with fromProvider); with unlockAll every field lock goes first, also those of the editor's match
// (the match itself stays). A title that is no matched audiobook is fromProvider not_matched.
func (s *Service) RefreshItemFromProvider(ctx context.Context, itemID string, unlockAll bool) (model.AdminItem, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	if s.books == nil || s.meta == nil {
		return model.AdminItem{}, ErrNotConfigured
	}
	it, err := s.liveItem(ctx, s.sys, itemID)
	if err != nil {
		return model.AdminItem{}, err
	}
	if it.Kind != "audiobook" || it.BookMatchState.String != "matched" {
		return model.AdminItem{}, (&model.ValidationError{}).Add("fromProvider", "not_matched")
	}
	if unlockAll {
		if err := s.meta.UnlockAll(ctx, itemID); err != nil {
			return model.AdminItem{}, err
		}
	}
	if err := s.books.RefetchBook(ctx, itemID); err != nil {
		return model.AdminItem{}, err
	}
	return s.GetAdminItem(ctx, itemID)
}

// AudiobookSources returns the page «Hörbücher: Quellen»: the settings, the Google Books key, every
// audiobook folder with its list, the sources, the numbers and whether a match runs.
func (s *Service) AudiobookSources(ctx context.Context) (model.AudiobookSources, error) {
	if s.books == nil {
		return model.AudiobookSources{}, ErrNotConfigured
	}
	out, err := s.books.BookSources(ctx)
	if err != nil {
		return model.AudiobookSources{}, err
	}
	if out.GoogleBooksKey, err = s.keyState(ctx, googleKeySetting); err != nil {
		return model.AudiobookSources{}, err
	}
	libs, err := s.read.ListLibraries(ctx)
	if err != nil {
		return model.AudiobookSources{}, fmt.Errorf("list libraries: %w", err)
	}
	names := map[int64]string{}
	for _, l := range libs {
		names[l.ID] = l.Name
	}
	roots, err := s.read.ListAllLibraryRoots(ctx)
	if err != nil {
		return model.AudiobookSources{}, fmt.Errorf("list folders: %w", err)
	}
	out.Folders = []model.AudiobookFolder{}
	for _, r := range roots {
		if r.Content != model.RootAudiobook {
			continue
		}
		out.Folders = append(out.Folders, model.AudiobookFolder{LibraryID: r.LibraryID, LibraryName: names[r.LibraryID], RootID: r.ID,
			Path: r.Path, Providers: model.ParseProviders(r.Providers)})
	}
	return out, nil
}

// UpdateAudiobookSources changes the settings of the page and the Google Books key. Errors:
// *model.ValidationError, *model.EnvWinsError (googleBooksKey while the environment sets it).
func (s *Service) UpdateAudiobookSources(ctx context.Context, upd model.AudiobookSourcesUpdate) (model.AudiobookSources, error) {
	if s.books == nil {
		return model.AudiobookSources{}, ErrNotConfigured
	}
	var verr *model.ValidationError
	key := ""
	if k := upd.GoogleBooksKey; k != nil {
		key = strings.TrimSpace(*k)
		switch {
		case upd.ClearGoogleBooksKey:
			verr = verr.Add("googleBooksKey", "invalid_value") // set and clear at once
		case key == "":
			verr = verr.Add("googleBooksKey", "invalid_value")
		case len(key) > maxProviderKey:
			verr = verr.Add("googleBooksKey", "too_long")
		}
	}
	if verr != nil {
		return model.AudiobookSources{}, verr
	}
	if (upd.GoogleBooksKey != nil || upd.ClearGoogleBooksKey) && s.envSet(EnvGoogleBooksKey) {
		return model.AudiobookSources{}, &model.EnvWinsError{Field: "googleBooksKey"}
	}
	// The key goes into the transaction of the settings (package metadata), which queues its jobs
	// only after the commit: a failure leaves neither settings nor key nor jobs behind.
	settings := upd.BookSettingsUpdate
	if upd.GoogleBooksKey != nil || upd.ClearGoogleBooksKey {
		enc := ""
		if !upd.ClearGoogleBooksKey {
			var err error
			if enc, err = s.keyring.Encrypt(key); err != nil {
				return model.AudiobookSources{}, fmt.Errorf("encrypt setting %s: %w", SettingGoogleBooksKey, err)
			}
		}
		settings.Extra = map[string]string{SettingGoogleBooksKey: enc}
	}
	if err := s.books.UpdateBookSources(ctx, settings); err != nil {
		return model.AudiobookSources{}, err
	}
	if settings.Extra != nil {
		s.log.InfoContext(ctx, "google books key changed", "cleared", upd.ClearGoogleBooksKey)
	}
	return s.AudiobookSources(ctx)
}

// MatchAllAudiobooks queues the match of every audiobook without match («Jetzt alle ohne Zuordnung
// zuordnen») and returns the number of books.
func (s *Service) MatchAllAudiobooks(ctx context.Context) (int, error) {
	defer s.ForgetDashboardCounts() // the counts of the sidebar (concept 11)
	if s.books == nil {
		return 0, ErrNotConfigured
	}
	n, err := s.books.MatchAllBooks(ctx)
	if err != nil {
		return n, err
	}
	s.log.InfoContext(ctx, "audiobook match queued", "books", n)
	return n, nil
}

// ListAudiobookReview returns the list «Prüfen».
func (s *Service) ListAudiobookReview(ctx context.Context) ([]model.BookReviewEntry, error) {
	if s.books == nil {
		return nil, ErrNotConfigured
	}
	return s.books.BookReview(ctx)
}
