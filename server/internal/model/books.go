package model

import "time"

// The book sources of audiobooks (concept 19 stage D3, D5.2 and D6.6, contract 1.26.0): the search
// and the field-by-field comparison of the editor's tab «Zuordnen», and the page «Hörbücher:
// Quellen».

// BookSearch is a search at one book source: by title (Query) and Author, or by ASIN or ISBN.
// Region is the Audible marketplace ("" = the setting); ItemID names the book whose length the
// candidates are compared with ("" = none).
type BookSearch struct {
	Provider, Query, Author, ASIN, ISBN, Region, ItemID string
}

// MatchPreview compares an entry of a book source with an audiobook (getMatchPreview).
type MatchPreview struct {
	Provider, ProviderID, Title string
	DurationMs                  int64 // 0 = unknown
	DurationMatches             *bool // nil without both lengths
	CoverURL                    string
	Fields                      []MatchPreviewField
}

// MatchPreviewField is one field of a MatchPreview: the display texts today and at the source.
type MatchPreviewField struct {
	Field             string // a BookMatchField
	Current, Proposed string
	Locked, Differs   bool
}

// BookSettings are the server-wide settings of the book sources.
type BookSettings struct {
	AudibleRegion    string
	AudnexusURL      string
	AutoMatch        string // off, new, all
	TolerancePercent int
	NormalizeAuthors bool
	NormalizeSeries  bool
}

// BookSettingsUpdate changes the book settings; nil fields stay.
type BookSettingsUpdate struct {
	AudibleRegion    *string
	AudnexusURL      *string
	AutoMatch        *string
	TolerancePercent *int
	NormalizeAuthors *bool
	NormalizeSeries  *bool
}

// AudiobookSourcesUpdate is updateAudiobookSources: the settings and the Google Books key.
type AudiobookSourcesUpdate struct {
	BookSettingsUpdate
	GoogleBooksKey      *string
	ClearGoogleBooksKey bool
}

// AudiobookSources is the page «Hörbücher: Quellen».
type AudiobookSources struct {
	Settings       BookSettings
	GoogleBooksKey KeyState
	Folders        []AudiobookFolder
	Sources        []AudiobookSourceState
	Stats          AudiobookMatchStats
	Running        bool
}

// AudiobookFolder is an audiobook folder with its ordered source list.
type AudiobookFolder struct {
	LibraryID   int64
	LibraryName string
	RootID      int64
	Path        string
	Providers   []string
}

// AudiobookSourceState is the state of one book source.
type AudiobookSourceState struct {
	ID          string
	Configured  bool
	PausedUntil *time.Time
}

// AudiobookMatchStats counts the audiobooks that are not deleted per match state.
type AudiobookMatchStats struct {
	Total, Matched, Review, None, Error, Pending int
}

// BookReviewEntry is one book of the list «Prüfen».
type BookReviewEntry struct {
	ItemID     string
	Title      string
	Author     string
	LibraryID  int64
	DurationMs *int64
	State      string // review, error
	Error      string
	MatchedAt  *time.Time
	Candidates []MetadataCandidate
}
