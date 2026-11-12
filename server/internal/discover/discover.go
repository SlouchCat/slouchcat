// Package discover answers the start page and the detail page (E-1, E-9) for a viewing profile.
// It reads titles only through the visibility package and opens stored images only through the
// paths package, relative to the cache directory.
//
// Start page (M7, M11, E-1, E-9, ADR 0011, concept 15 P-7): the rows spotlight (the configured
// Trakt list intersected with the library, with the main user's curation), picks (the user
// picks of the profiles, concept 18 part B), continue (continue watching), recent (recently
// added), released (by release date within twelve months, a series by its newest visible
// episode, S-5, S-6), watchlist,
// collections, genres, libraries («Meine Medien», one tile per visible library), one row
// library («Neu in …») per library and radio («Meine Sender», the profile's favourite stations,
// concept 16 section 10.2, through HomeRadio), in the order the administration stored (table
// home_rows, a row named by its id); hidden and empty rows are omitted, the
// spotlight row also below the configured minimum of hits, spotlight_config.min_hits (AN-3). The
// page names the row the client draws as the stage (HomePage.HeroRow: spotlight when present, else
// none; picks is never the stage) and carries hasTrailer for the rows spotlight and picks only (D3).
// With a stage it names the title the stage begins with, drawn once per page view (HeroStart,
// concept 08, F-2, ADR 0016), and the rhythm of the stage (HeroIntervalSeconds, F-10).
// The library views of concept 08 (side navigation, library page, user picks) pass
// through to visibility.
// The package also answers search, genres, collections, and the watchlist of a profile
// (m7.go).
// It holds no SQL on titles: reads go through visibility, the watchlist writes through
// store/appq, whose tables name titles only by id.
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
	"strconv"
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
	RowRadio       = model.RowRadio
)

// HomeRadio gives the start page row radio its tiles (radio.Service): the favourite stations the
// profile may reach, each with the title at the live edge of a running relay.
type HomeRadio interface {
	HomeStations(ctx context.Context, profile model.Profile) ([]model.RadioHomeStation, error)
}

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
	loadRow func(context.Context, model.Profile, model.HomeRowSetting) (model.HomeRow, error)
	// radio fills the row radio (WithRadio); nil leaves it empty, so it never appears.
	radio HomeRadio

	// The draw of the stage's first title (concept 08, F-2): rnd is not safe for concurrent use,
	// so rndMu guards it; heroStart >= 0 replaces the draw by a fixed index (WithHeroStart).
	rndMu     sync.Mutex
	rnd       *rand.Rand
	heroStart int
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces time.Now: the retention of the user picks (E-2) and the time of user picks,
// and watchlist entries.
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

// WithRadio sets the source of the start page row radio («Meine Sender», concept 16).
func WithRadio(r HomeRadio) Option {
	return func(s *Service) { s.radio = r }
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
	page, err := s.page(ctx, profile, false)
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
// (callers with untilHero read only the stage), and without a spotlight row there is no stage and
// the page stays empty.
//
// The rows are loaded side by side (loadRows) and placed in their order exactly as one after the
// other: a row's error counts only when the page reaches that row, so a row behind the cut to
// MaxRows (or behind the stage, with untilHero) never fails the page. Once the page is complete
// the rows still loading are cancelled and the rows not yet started are not loaded.
func (s *Service) page(ctx context.Context, profile model.Profile, untilHero bool) (model.HomePage, error) {
	order, err := s.rowOrder(ctx)
	if err != nil {
		return model.HomePage{}, err
	}
	if untilHero {
		j := slices.IndexFunc(order, func(r model.HomeRowSetting) bool { return r.Key == RowSpotlight })
		if j < 0 {
			return model.HomePage{}, nil
		}
		order = order[:j+1]
	}
	ctx, cancel := context.WithCancel(ctx)
	results, wait := s.loadRows(ctx, profile, order)
	defer func() {
		cancel()
		wait()
	}()
	page := model.HomePage{Rows: make([]model.HomeRow, 0, min(len(order), MaxRows))}
	for i, setting := range order {
		r := <-results[i]
		if r.err != nil {
			return model.HomePage{}, fmt.Errorf("home row %s: %w", setting.ID, r.err)
		}
		if filled(r.row) {
			page.Rows = append(page.Rows, r.row)
			if r.row.Key == RowSpotlight {
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

// rowResult is one row of the start page loaded by loadRows, with its error.
type rowResult struct {
	row model.HomeRow
	err error
}

// loadRows starts loading the rows of order side by side, in at most min(GOMAXPROCS, 4) goroutines
// that take the rows in their order, and returns one channel per row that receives the row with its
// error, and a function that waits for the goroutines. A row taken after ctx is done is not loaded
// and receives the error of ctx; the caller cancels ctx when it needs no further row and then waits.
// A row holds at most one connection of the read pool at a time (its statements read all rows
// before the next one, a transaction runs only its own statements), so the goroutines never wait
// on each other for the pool (store.Open: at least four connections). On two cores the start page
// of the CT 190 database takes about a sixth less time (docs/perf/feinschliff.md section 3).
func (s *Service) loadRows(ctx context.Context, profile model.Profile, order []model.HomeRowSetting) ([]chan rowResult, func()) {
	results := make([]chan rowResult, len(order))
	next := make(chan int, len(order))
	for i := range order {
		results[i] = make(chan rowResult, 1)
		next <- i
	}
	close(next)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), 4, len(order)) {
		wg.Go(func() {
			for i := range next {
				if err := ctx.Err(); err != nil {
					results[i] <- rowResult{err: err}
					continue
				}
				row, err := s.safeRow(ctx, profile, order[i])
				results[i] <- rowResult{row: row, err: err}
			}
		})
	}
	return results, wg.Wait
}

// safeRow loads one row with loadRow and turns a panic into the row's error with its stack: a row
// runs in a goroutine of loadRows, where the recoverer of the HTTP handler cannot catch it, so the
// error takes its place and fails the page like any other error of a row the page reaches.
func (s *Service) safeRow(ctx context.Context, profile model.Profile, setting model.HomeRowSetting) (row model.HomeRow, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			row, err = model.HomeRow{}, fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
		}
	}()
	return s.loadRow(ctx, profile, setting)
}

// filled reports whether a row goes on the page: «Meine Medien» counts its tiles (8.3), «Meine
// Sender» its stations, the other rows their entries.
func filled(row model.HomeRow) bool {
	return len(row.Items) > 0 || len(row.Collections) > 0 || len(row.Genres) > 0 || len(row.Libraries) > 0 ||
		len(row.Stations) > 0
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

// row loads one row of the start page; an unknown key gives an empty row. An error of the row
// radio is logged and leaves that row empty: the radio is no reason to lose the page (critic m-8).
func (s *Service) row(ctx context.Context, profile model.Profile, setting model.HomeRowSetting) (model.HomeRow, error) {
	scope := visibility.Profile(profile)
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
	case RowRadio:
		row.Stations = s.radioRow(ctx, profile)
	}
	return row, err
}

// radioRow returns the tiles of the row radio, none without a radio service or on its error.
func (s *Service) radioRow(ctx context.Context, profile model.Profile) []model.RadioHomeStation {
	if s.radio == nil {
		return nil
	}
	stations, err := s.radio.HomeStations(ctx, profile)
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "home row radio left out", "err", err)
		}
		return nil
	}
	return stations
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
	page, err := s.page(ctx, profile, true)
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
// when it has no rows, or when its first row holds collections or genres rather than titles,
// library tiles or stations. Its titles (or, for «Meine Medien», its tiles, concept 15 P-7, and
// for «Meine Sender» its stations, concept 16 section 10.2) are the first cards
// of a page without a stage, the images the client loads first (P-10); the app shell of the start
// page preloads the first poster and paints the first cards ahead. It derives from the same
// computation as Home, so it follows its rule, but it loads the rows only up to the first one on
// the page (the prepaint of GET / waits for it): a stage further down is found from the spotlight
// row alone while it can only land within MaxRows, else from the page up to the stage.
func (s *Service) FirstRow(ctx context.Context, profile model.Profile) (model.HomeRow, error) {
	order, err := s.rowOrder(ctx)
	if err != nil {
		return model.HomeRow{}, err
	}
	for i, setting := range order {
		row, err := s.row(ctx, profile, setting)
		if err != nil {
			return model.HomeRow{}, fmt.Errorf("home row %s: %w", setting.ID, err)
		}
		if !filled(row) {
			continue
		}
		if row.Key == RowSpotlight || (len(row.Items) == 0 && len(row.Libraries) == 0 && len(row.Stations) == 0) {
			return model.HomeRow{}, model.ErrNotFound
		}
		stage, err := s.stageBelow(ctx, profile, order, i)
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
func (s *Service) stageBelow(ctx context.Context, profile model.Profile, order []model.HomeRowSetting, first int) (bool, error) {
	j := slices.IndexFunc(order[first+1:], func(r model.HomeRowSetting) bool { return r.Key == RowSpotlight })
	if j < 0 {
		return false, nil
	}
	j += first + 1
	if j >= MaxRows {
		page, err := s.page(ctx, profile, true)
		return page.HeroRow != "", err
	}
	items, err := s.spotlightRow(ctx, visibility.Profile(profile))
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

// AudioGroups returns the row «Reihen und Sammlungen» of an audiobook library (listAudioGroups,
// concept 19 D4.2).
func (s *Service) AudioGroups(ctx context.Context, profile model.Profile, libraryID int64, q model.LibraryQuery) (model.AudioGroupList, error) {
	return s.vis.AudioGroups(ctx, visibility.Profile(profile), libraryID, q)
}

// AudioSeries returns the page of an audiobook series (getAudioSeries, concept 19 D4.3).
func (s *Service) AudioSeries(ctx context.Context, profile model.Profile, libraryID int64, seriesID string) (model.AudioSeries, error) {
	return s.vis.AudioSeries(ctx, visibility.Profile(profile), libraryID, seriesID)
}

// PicksPage returns one page of the user picks the profile may see and their number (listPicks,
// concept 18 part B), the row picks without its cut; without HasTrailer.
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
	return s.openImage(ctx, loc, itemID+"/"+kind+"/"+size)
}

// PersonImage opens the portrait of a person with a role on a visible title (concept 17 5.1):
// WebP when preferWebP is set and stored, JPEG otherwise; model.ErrNotFound without one. The caller
// closes the file.
func (s *Service) PersonImage(ctx context.Context, profile model.Profile, personID int64, preferWebP bool) (model.StreamSource, error) {
	loc, err := s.vis.PersonImage(ctx, visibility.Profile(profile), personID, preferWebP)
	if err != nil {
		return model.StreamSource{}, err
	}
	return s.openImage(ctx, loc, "people/"+strconv.FormatInt(personID, 10))
}

// CollectionImage opens the TMDB poster of a collection that is fully visible to the profile
// (concept 17 5.2); model.ErrNotFound otherwise. The caller closes the file.
func (s *Service) CollectionImage(ctx context.Context, profile model.Profile, collectionID string, preferWebP bool) (model.StreamSource, error) {
	loc, err := s.vis.CollectionImage(ctx, visibility.Profile(profile), collectionID, preferWebP)
	if err != nil {
		return model.StreamSource{}, err
	}
	return s.openImage(ctx, loc, "collections/"+collectionID)
}

// openImage opens a stored image below the cache directory through package paths; a row whose
// file is gone answers model.ErrNotFound. name names the image in the log and the source.
func (s *Service) openImage(ctx context.Context, loc visibility.ImageLocation, name string) (model.StreamSource, error) {
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
			s.log.WarnContext(ctx, "image row without file", "image", name)
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
		Name:      name,
	}, nil
}
