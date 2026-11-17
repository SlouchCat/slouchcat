package wishes

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The parts of other packages the requests of stage 2 use (concept 25, architecture 2.3): the
// library through visibility (rule 2: this package reads no title table), the titles outside the
// library through metadata and the messages through notify. Each is set with an option; without
// one the methods that need it answer errNoLibrary or errNoTitles, and without a Notifier no
// message goes out.

// Library is the part of package visibility the wishes ask (*visibility.Reader).
type Library interface {
	MissingTitle(ctx context.Context, s visibility.Scope, right visibility.RequestRight, ref visibility.ExternalRef) error
	MatchExternalTitles(ctx context.Context, s visibility.Scope, right visibility.RequestRight, refs []visibility.ExternalRef) ([]visibility.ExternalMatch, error)
	TitleByExternalIDs(ctx context.Context, ref visibility.ExternalRef) (itemID string, found bool, err error)
	Visible(ctx context.Context, s visibility.Scope, itemID string) error
	VisibleSeasons(ctx context.Context, s visibility.Scope, seriesID string) (map[int]int, error)
	// CatalogSeasonTarget is the episode target of a season by series_catalog; found false sends
	// the caller to the title outside the library.
	CatalogSeasonTarget(ctx context.Context, seriesID string, season int) (visibility.SeasonTarget, bool, error)
}

// Titles is the part of package metadata the wishes ask (*metadata.Service): the stored title
// outside the library, fetched from TMDB when missing.
type Titles interface {
	EnsureExternalTitle(ctx context.Context, media string, tmdbID int64) (metadata.ExternalTitle, error)
}

// Notifier receives the changes of a wish that reach its requesters (architecture 12; package
// notify builds the messages, checks the visibility per profile and writes notified_at itself:
// this package never sets it for «verfügbar»). Every method answers model.ErrNotFound for an
// unknown wish. A failed call is logged; the reconcile calls the available methods again while
// profiles wait.
type Notifier interface {
	// WishAvailable: the film, or the series with every wished season, is playable; itemID is the
	// library title visibility returned. Waiting are the profiles that cannot see it yet.
	WishAvailable(ctx context.Context, wishID int64, itemID string) (NotifyOutcome, error)
	// WishSeasonAvailable: one wished season is complete while the wish is not (partial).
	WishSeasonAvailable(ctx context.Context, season SeasonNotice) (NotifyOutcome, error)
	// WishApproved: the request went from pending to approved (not an approval at creation).
	WishApproved(ctx context.Context, wishID int64) error
	// WishDeclined: Seerr declined the request.
	WishDeclined(ctx context.Context, wishID int64) error
	// WishFailed: the wish failed; reason is its fail_reason.
	WishFailed(ctx context.Context, wishID int64, reason string) error
	// WishReview: the request waits for an approval in Seerr («Freigabe nötig», administrators).
	WishReview(ctx context.Context, wishID int64) error
}

// NotifyOutcome says which profiles got the message and which wait for the title to become
// visible to them.
type NotifyOutcome struct {
	Notified, Waiting []int64
}

// SeasonNotice is one complete season of a series wish. Episodes is the target count (aired so
// far with Airing for a running season).
type SeasonNotice struct {
	WishID   int64
	SeriesID string
	Season   int
	Episodes int
	Airing   bool
}

// WithLibrary sets the library (visibility.Reader).
func WithLibrary(l Library) Option { return func(s *Service) { s.lib = l } }

// WithTitles sets the titles outside the library (metadata.Service).
func WithTitles(t Titles) Option { return func(s *Service) { s.titles = t } }

// WithNotifier sets the receiver of the messages (an adapter of package notify).
func WithNotifier(n Notifier) Option { return func(s *Service) { s.notifier = n } }
