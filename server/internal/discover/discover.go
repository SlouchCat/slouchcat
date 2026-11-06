// Package discover answers the start page and the detail page (E-1, E-9) for a viewing profile.
// It reads titles only through the visibility package and opens stored images only through the
// paths package, relative to the cache directory.
//
// Start page (M7, M11, E-1, E-9, ADR 0011, concept 15 P-7): the rows spotlight (the configured
// Trakt list intersected with the library, with the main user's curation), picks (Cat's
// recommendations), continue (continue watching), recent (recently added), released (by release
// date within twelve months, a series by its newest visible episode, S-5, S-6), watchlist,
// collections, genres, libraries («Meine Medien», one tile per visible library) and one row
// library («Neu in …») per library, in the order the administration stored (table home_rows, a row
// named by its id); hidden and empty rows are omitted, the
// spotlight row also below the configured minimum of hits, spotlight_config.min_hits (AN-3). The
// page names the row the client draws as the stage (HomePage.HeroRow: spotlight when present, else
// none; picks is never the stage) and carries hasTrailer for the rows spotlight and picks only (D3).
// With a stage it names the title the stage begins with, drawn once per page view (HeroStart,
// concept 08, F-2, ADR 0016), and the rhythm of the stage (HeroIntervalSeconds, F-10).
// The library views of concept 08 (side navigation, library page, Cat's recommendations) pass
// through to visibility.
// The package also answers search, genres, collections, the watchlist and the favourites of a
// profile (m7.go).
// It holds no SQL on titles: reads go through visibility, the writes of watchlist and favourites
// through store/appq, whose tables name titles only by id.
package discover

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Home row keys (model.HomeRow.Key), in the default order of ADR 0011.
const (
	RowSpotlight   = model.RowSpotlight
	RowPicks       = model.RowPicks
	RowContinue    = model.RowContinue
	RowRecent      = model.RowRecent
	RowReleased    = model.RowReleased
	RowWatchlist   = model.RowWatchlist
	RowCollections = model.RowCollections
	RowGenres      = model.RowGenres
	RowLibraries   = model.RowLibraries
	RowLibrary     = model.RowLibrary
)

// MaxRows caps the start page (E-9, F-12; the contract's HomePage.rows maxItems).
const MaxRows = model.MaxVisibleHomeRows

// Service implements the catalog service of the API.
type Service struct {
	vis      *visibility.Reader
	read     *appq.Queries // home_rows
	write    *appq.Queries // watchlist, favorites
	cacheDir string
	log      *slog.Logger
	now      func() time.Time

	// loadRow loads one row of the start page for loadRows: row, replaced only by tests of the
	// package to inject a failing or panicking row.
	loadRow func(context.Context, visibility.Scope, model.HomeRowSetting) (model.HomeRow, error)

	// The draw of the stage's first title (concept 08, F-2): rnd is not safe for concurrent use,
	// so rndMu guards it; heroStart >= 0 replaces the draw by a fixed index (WithHeroStart).
	rndMu     sync.Mutex
	rnd       *rand.Rand
	heroStart int
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces time.Now: the time windows of the picks (E-2) and the time of watchlist and
// favourite entries.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithRand replaces the random source of the stage's first title (concept 08, F-2; no security
// concern), so tests draw deterministically.
func WithRand(r *rand.Rand) Option {
	return func(s *Service) { s.rnd = r }
}

// WithHeroStart fixes the index the stage begins with instead of a draw (tests and the e2e suite,
// SLOUCHCAT_HERO_START, concept 08); a negative index keeps the draw, an index beyond the row
// takes its last title. A start passed to Home still wins.
func WithHeroStart(k int) Option {
	return func(s *Service) { s.heroStart = k }
}

// New returns the catalog service. cacheDir is the absolute cache root that image rel_paths are
// relative to; log is the redacting logger.
func New(db *store.DB, cacheDir string, log *slog.Logger, opts ...Option) *Service {
	s := &Service{read: appq.New(db.Read), write: appq.New(db.Write), cacheDir: cacheDir, log: log, now: time.Now, heroStart: -1}
	s.loadRow = s.row
	for _, o := range opts {
		o(s)
	}
	s.vis = visibility.New(db, log, visibility.WithClock(s.now))
	if s.rnd == nil {
		now := uint64(time.Now().UnixNano())
		s.rnd = rand.New(rand.NewPCG(now, now>>32)) //nolint:gosec // picks a slide, no security concern (ADR 0016)
	}
	return s
}

// Home returns the rows of the start page in the stored order (E-9), without hidden and empty
// rows and without a spotlight row below its configured minimum of hits; never more than MaxRows.
// HeroRow names the stage (plan 3.5): spotlight when that row is in the answer, at its stored
// place, else nothing; picks is never the stage. The titles of the rows spotlight and picks carry
// HasTrailer (D3); every other row leaves it nil. With a stage, HeroStart is the index into the
// stage row's titles the stage begins with (concept 08, F-2): start when it lies in the row (the
// app shell's draw repeated), else the fixed index of WithHeroStart, else a fresh draw; and
// HeroIntervalSeconds is spotlight_config.rotation_seconds (F-10). Without a stage both stay unset.
func (s *Service) Home(ctx context.Context, profile model.Profile, start *int) (model.HomePage, error) {
	scope := visibility.Profile(profile)
	page, err := s.page(ctx, scope, false)
	if err != nil {
		return model.HomePage{}, err
	}
	for i := range page.Rows {
		if row := &page.Rows[i]; row.Key == RowSpotlight || row.Key == RowPicks {
			if err := s.markTrailers(ctx, scope, row.Items); err != nil {
				return model.HomePage{}, fmt.Errorf("home row %s: %w", row.Key, err)
			}
		}
	}
	if i := slices.IndexFunc(page.Rows, func(r model.HomeRow) bool { return r.Key == page.HeroRow }); i >= 0 {
		cfg, err := s.read.GetSpotlightConfig(ctx)
		if err != nil {
			return model.HomePage{}, fmt.Errorf("get spotlight config: %w", err)
		}
		k := s.drawHeroStart(len(page.Rows[i].Items), start)
		page.HeroStart, page.HeroIntervalSeconds = &k, int(cfg.RotationSeconds)
	}
	return page, nil
}

// drawHeroStart returns the index the stage begins with for a stage row of n > 0 titles (see Home).
func (s *Service) drawHeroStart(n int, start *int) int {
	if start != nil && *start >= 0 && *start < n {
		return *start
	}
	if s.heroStart >= 0 {
		return min(s.heroStart, n-1)
	}
	s.rndMu.Lock()
	defer s.rndMu.Unlock()
	return s.rnd.IntN(n)
}

// page builds the rows of the start page and names the stage; it is the one computation behind
// Home, FirstHero and FirstRow. With untilHero it ends at the spotlight row, the only row that can
// be the stage: the rows after it change neither the stage nor its place, so they are not loaded
// (callers with untilHero read only the stage).
//
// The rows are loaded side by side (loadRows) and then placed in their order exactly as one after
// the other: a row's error counts only when the page reaches that row, so a row behind the cut to
// MaxRows (or behind the stage, with untilHero) never fails the page.
func (s *Service) page(ctx context.Context, scope visibility.Scope, untilHero bool) (model.HomePage, error) {
	order, err := s.rowOrder(ctx)
	if err != nil {
		return model.HomePage{}, err
	}
	if j := slices.IndexFunc(order, func(r model.HomeRowSetting) bool { return r.Key == RowSpotlight }); untilHero && j >= 0 {
		order = order[:j+1]
	}
	rows, errs := s.loadRows(ctx, scope, order)
	page := model.HomePage{Rows: make([]model.HomeRow, 0, len(order))}
	for i, setting := range order {
		row, err := rows[i], errs[i]
		if err != nil {
			return model.HomePage{}, fmt.Errorf("home row %s: %w", setting.ID, err)
		}
		if filled(row) {
			page.Rows = append(page.Rows, row)
			if row.Key == RowSpotlight {
				page.HeroRow = RowSpotlight
				if untilHero {
					break
				}
			}
		}
		if len(page.Rows) == MaxRows {
			break
		}
	}
	return page, nil
}

// loadRows loads the rows of order side by side, in at most min(GOMAXPROCS, 4) goroutines that
// take the rows in their order, and returns each row with its error at the row's index. A row
// holds at most one connection of the read pool at a time (its statements read all rows before the
// next one, a transaction runs only its own statements), so the goroutines never wait on each other
// for the pool (store.Open: at least four connections). On two cores the start page of the CT 190
// database takes about a sixth less time (docs/perf/feinschliff.md section 3).
func (s *Service) loadRows(ctx context.Context, scope visibility.Scope, order []model.HomeRowSetting) ([]model.HomeRow, []error) {
	rows := make([]model.HomeRow, len(order))
	errs := make([]error, len(order))
	next := make(chan int, len(order))
	for i := range order {
		next <- i
	}
	close(next)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), 4, len(order)) {
		wg.Go(func() {
			for i := range next {
				rows[i], errs[i] = s.safeRow(ctx, scope, order[i])
			}
		})
	}
	wg.Wait()
	return rows, errs
}

// safeRow loads one row with loadRow and turns a panic into the row's error with its stack: a row
// runs in a goroutine of loadRows, where the recoverer of the HTTP handler cannot catch it, so the
// error takes its place and fails the page like any other error of a row the page reaches.
func (s *Service) safeRow(ctx context.Context, scope visibility.Scope, setting model.HomeRowSetting) (row model.HomeRow, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			row, err = model.HomeRow{}, fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
		}
	}()
	return s.loadRow(ctx, scope, setting)
}

// filled reports whether a row goes on the page: «Meine Medien» counts its tiles (8.3), the
// other rows their entries.
func filled(row model.HomeRow) bool {
	return len(row.Items) > 0 || len(row.Collections) > 0 || len(row.Genres) > 0 || len(row.Libraries) > 0
}

// markTrailers sets HasTrailer on every title of a row from one visibility query (D3).
func (s *Service) markTrailers(ctx context.Context, scope visibility.Scope, items []model.ItemSummary) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	flags, err := s.vis.TrailerFlags(ctx, scope, ids)
	if err != nil {
		return err
	}
	for i := range items {
		has := flags[items[i].ID]
		items[i].HasTrailer = &has
	}
	return nil
}

// rowOrder returns the rows not hidden in their stored order (migrations 00015, 00021 and 00035
// store every row, the rows of concept 15 P-7 hidden).
func (s *Service) rowOrder(ctx context.Context) ([]model.HomeRowSetting, error) {
	rows, err := s.read.ListHomeRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list home rows: %w", err)
	}
	out := make([]model.HomeRowSetting, 0, len(rows))
	for _, r := range rows {
		if r.Hidden == 0 {
			out = append(out, model.HomeRowSetting{ID: r.ID, Key: r.Key, LibraryID: r.LibraryID.Int64, LibraryName: r.LibraryName})
		}
	}
	return out, nil
}

// row loads one row of the start page; an unknown key gives an empty row.
func (s *Service) row(ctx context.Context, scope visibility.Scope, setting model.HomeRowSetting) (model.HomeRow, error) {
	row := model.HomeRow{ID: setting.ID, Key: setting.Key}
	var err error
	switch setting.Key {
	case RowPicks:
		row.Items, err = s.vis.Picks(ctx, scope, visibility.MaxRowItems)
	case RowContinue:
		row.Items, err = s.vis.ContinueWatching(ctx, scope, visibility.MaxRowItems)
	case RowRecent:
		row.Items, err = s.vis.Recent(ctx, scope, visibility.MaxRowItems)
	case RowWatchlist:
		row.Items, err = s.vis.WatchlistRow(ctx, scope, visibility.MaxRowItems)
	case RowCollections:
		row.Collections, err = s.vis.Collections(ctx, scope, visibility.MaxRowItems)
	case RowGenres:
		row.Genres, err = s.vis.Genres(ctx, scope, visibility.MaxRowItems)
	case RowReleased:
		row.Items, err = s.vis.Released(ctx, scope, visibility.MaxRowItems)
	case RowSpotlight:
		row.Items, err = s.spotlightRow(ctx, scope)
	case RowLibraries:
		row.Libraries, err = s.vis.LibraryTiles(ctx, scope)
	case RowLibrary:
		row.LibraryID, row.LibraryName = setting.LibraryID, setting.LibraryName
		row.Items, err = s.vis.LibraryNewest(ctx, scope, setting.LibraryID, visibility.MaxRowItems)
	}
	return row, err
}

// spotlightRow reads the spotlight row with the configured size and omits it below the configured
// minimum of hits (AN-3, S-3). min_hits counts the hits before the cut to item_count (plan 5.3):
// "show 5 if there are at least 10" is a valid setting, not a row that never appears.
func (s *Service) spotlightRow(ctx context.Context, scope visibility.Scope) ([]model.ItemSummary, error) {
	cfg, err := s.read.GetSpotlightConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get spotlight config: %w", err)
	}
	items, hits, err := s.vis.SpotlightRow(ctx, scope, int(cfg.ItemCount))
	if err != nil {
		return nil, err
	}
	if hits < int(cfg.MinHits) {
		return nil, nil
	}
	return items, nil
}

// FirstHero returns the title the stage begins with, its index in the stage row (HomePage.HeroRow)
// and whether that row is the first row of the page, or model.ErrNotFound when the page has no
// stage. The app shell of the start page draws the stage's first title with it, once per page view
// (concept 08, 6): it preloads that title's image, paints it ahead only when the stage comes first
// (P-10, plan 3.5), and hands the index on to GET /home?start=K, which repeats the draw. It derives
// from the same computation as Home and draws like it (drawHeroStart, so WithHeroStart fixes both).
func (s *Service) FirstHero(ctx context.Context, profile model.Profile) (model.ItemSummary, int, bool, error) {
	page, err := s.page(ctx, visibility.Profile(profile), true)
	if err != nil {
		return model.ItemSummary{}, 0, false, err
	}
	if page.HeroRow == "" {
		return model.ItemSummary{}, 0, false, model.ErrNotFound
	}
	i := slices.IndexFunc(page.Rows, func(r model.HomeRow) bool { return r.Key == page.HeroRow })
	items := page.Rows[i].Items
	k := s.drawHeroStart(len(items), nil)
	return items[k], k, i == 0, nil
}

// FirstRow returns the first row of the start page when the page has no stage, or
// model.ErrNotFound when it has one (the stage image is preloaded and painted instead, FirstHero),
// when it has no rows, or when its first row holds collections or genres rather than titles or
// library tiles. Its titles (or, for «Meine Medien», its tiles, concept 15 P-7) are the first cards
// of a page without a stage, the images the client loads first (P-10); the app shell of the start
// page preloads the first poster and paints the first cards ahead. It derives from the same
// computation as Home, so it follows its rule, but it loads the rows only up to the first one on
// the page (the prepaint of GET / waits for it): a stage further down is found from the spotlight
// row alone while it can only land within MaxRows, else from the page up to the stage.
func (s *Service) FirstRow(ctx context.Context, profile model.Profile) (model.HomeRow, error) {
	scope := visibility.Profile(profile)
	order, err := s.rowOrder(ctx)
	if err != nil {
		return model.HomeRow{}, err
	}
	for i, setting := range order {
		row, err := s.row(ctx, scope, setting)
		if err != nil {
			return model.HomeRow{}, fmt.Errorf("home row %s: %w", setting.ID, err)
		}
		if !filled(row) {
			continue
		}
		if row.Key == RowSpotlight || (len(row.Items) == 0 && len(row.Libraries) == 0) {
			return model.HomeRow{}, model.ErrNotFound
		}
		stage, err := s.stageBelow(ctx, scope, order, i)
		if err != nil {
			return model.HomeRow{}, err
		}
		if stage {
			return model.HomeRow{}, model.ErrNotFound
		}
		row.Items = firstRowCards(row)
		return row, nil
	}
	return model.HomeRow{}, model.ErrNotFound
}

// stageBelow reports whether the page whose first row stands at order[first] has a stage further
// down, as page would place it: the spotlight row after first, when it has titles and lands
// within MaxRows. Before the MaxRows-th setting it always lands (at most that many rows precede
// it); later only the page up to the stage tells.
func (s *Service) stageBelow(ctx context.Context, scope visibility.Scope, order []model.HomeRowSetting, first int) (bool, error) {
	j := slices.IndexFunc(order[first+1:], func(r model.HomeRowSetting) bool { return r.Key == RowSpotlight })
	if j < 0 {
		return false, nil
	}
	j += first + 1
	if j >= MaxRows {
		page, err := s.page(ctx, scope, true)
		return page.HeroRow != "", err
	}
	items, err := s.spotlightRow(ctx, scope)
	if err != nil {
		return false, fmt.Errorf("home row %s: %w", order[j].ID, err)
	}
	return len(items) > 0, nil
}

// firstRowCards returns the titles of row as its cards show them: only the row continue shows an
// episode with the poster and title of its series (concept 15 P-1), so in any other row an
// episode leaves SeriesRef out and the prepaint keeps the episode's own (missing) poster, like the
// card.
func firstRowCards(row model.HomeRow) []model.ItemSummary {
	if row.Key == RowContinue {
		return row.Items
	}
	items := slices.Clone(row.Items)
	for i := range items {
		items[i].SeriesRef = nil
	}
	return items
}

// VisibleLibraries returns the libraries with a visible top-level title for the side navigation
// (concept 08, listVisibleLibraries).
func (s *Service) VisibleLibraries(ctx context.Context, profile model.Profile) ([]model.LibraryNav, error) {
	return s.vis.VisibleLibraries(ctx, visibility.Profile(profile))
}

// LibraryItems returns one page of a library with its facets (concept 08, listLibraryItems);
// model.ErrNotFound for a library without a visible title, *model.ValidationError for an invalid
// query. The titles carry no HasTrailer (only the start page rows spotlight and picks do).
func (s *Service) LibraryItems(ctx context.Context, profile model.Profile, libraryID int64, q model.LibraryQuery) (model.LibraryPage, error) {
	return s.vis.LibraryItems(ctx, visibility.Profile(profile), libraryID, q)
}

// PicksPage returns one page of all of Cat's recommendations for the profile and their number
// (concept 08, listPicks), the row picks without its cut; without HasTrailer.
func (s *Service) PicksPage(ctx context.Context, profile model.Profile, offset, limit int) ([]model.ItemSummary, int, error) {
	return s.vis.PicksPage(ctx, visibility.Profile(profile), offset, limit)
}

// Item returns the detail of a visible title (with its series tree, extras, cast and trailer
// count); media files carry their MIME type for the client's canPlayType check.
func (s *Service) Item(ctx context.Context, profile model.Profile, itemID string) (model.ItemDetail, error) {
	d, err := s.vis.Item(ctx, visibility.Profile(profile), itemID)
	if err != nil {
		return model.ItemDetail{}, err
	}
	for i := range d.MediaFiles {
		f := &d.MediaFiles[i]
		f.MimeType = playback.MimeType(f.Container, f.VideoCodec, f.AudioCodec)
	}
	return d, nil
}

// Trailers returns the trailers of a visible title, the profile language first, or
// model.ErrNotFound.
func (s *Service) Trailers(ctx context.Context, profile model.Profile, itemID string) ([]model.Trailer, error) {
	return s.vis.Trailers(ctx, visibility.Profile(profile), itemID)
}

// Person returns a person with the credits on titles the profile may see, or model.ErrNotFound
// when none is visible.
func (s *Service) Person(ctx context.Context, profile model.Profile, personID int64) (model.Person, error) {
	return s.vis.Person(ctx, visibility.Profile(profile), personID)
}

// PodcastEpisodes returns one page of the visible episodes of a visible podcast, newest first, and
// their total (M6); model.ErrNotFound for an invisible title or one that is no podcast.
func (s *Service) PodcastEpisodes(ctx context.Context, profile model.Profile, podcastID string, offset, limit int) ([]model.ItemSummary, int, error) {
	return s.vis.PodcastEpisodes(ctx, visibility.Profile(profile), podcastID, offset, limit)
}

// Image opens a stored image of a visible title: WebP when preferWebP is set and the WebP row
// exists, JPEG otherwise. The caller closes the file.
func (s *Service) Image(ctx context.Context, profile model.Profile, itemID, kind, size string, preferWebP bool) (model.StreamSource, error) {
	loc, err := s.vis.Image(ctx, visibility.Profile(profile), itemID, kind, size, preferWebP)
	if err != nil {
		return model.StreamSource{}, err
	}
	root, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return model.StreamSource{}, err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return model.StreamSource{}, err
	}
	defer dir.Close() // files opened through the root stay usable after it is closed
	f, err := dir.Open(loc.RelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.log.WarnContext(ctx, "image row without file", "item_id", itemID, "kind", kind, "size", size)
			return model.StreamSource{}, model.ErrNotFound
		}
		return model.StreamSource{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat image: %w", err), f.Close())
	}
	return model.StreamSource{
		File:      f,
		MediaType: loc.MediaType,
		Size:      info.Size(),
		ModTime:   time.UnixMilli(loc.ModTime).UTC(),
		Name:      itemID + "/" + kind + "/" + size,
	}, nil
}
