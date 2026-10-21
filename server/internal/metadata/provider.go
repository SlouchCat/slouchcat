package metadata

import (
	"context"
	"errors"
)

// Provider delivers title data of one external metadata source (design 3.1). Every request runs
// through the provider's ratelimit.Limiter. IDs are the provider's own IDs as decimal strings.
// Languages are ISO 639-1 codes ("de", "en"); a record holds the texts of every requested
// language the provider knows. ErrNotFound means the provider does not know the ID.
type Provider interface {
	Name() string                        // "tmdb", "tvdb"
	Configured(ctx context.Context) bool // a key is present
	SearchMovie(ctx context.Context, q MovieQuery) ([]Candidate, error)
	SearchSeries(ctx context.Context, q SeriesQuery) ([]Candidate, error)
	Movie(ctx context.Context, id string, langs []string) (MovieRecord, error)
	Series(ctx context.Context, id string, langs []string) (SeriesRecord, error)
	Season(ctx context.Context, seriesID string, n int, langs []string) (SeasonRecord, error)
	Episode(ctx context.Context, seriesID string, s, e int, langs []string) (EpisodeRecord, error)
}

// Provider names (also the prefix of items.rating_source and MetadataCandidate.Provider).
const (
	ProviderTMDB = "tmdb"
	ProviderTVDB = "tvdb"
)

// textLangs are the languages of item_texts (B-3).
var textLangs = []string{"de", "en"}

// ErrNotFound is returned when a provider does not know an ID (HTTP 404).
var ErrNotFound = errors.New("metadata provider: not found")

// errNotConfigured is returned by a provider call without key.
var errNotConfigured = errors.New("metadata provider: no key configured")

// MovieQuery searches a movie: by IMDb ID when set, else by title and optional year.
type MovieQuery struct {
	Title  string
	Year   *int
	IMDBID string
}

// SeriesQuery searches a series: by TVDB ID, else IMDb ID, else title and optional year (first
// air date).
type SeriesQuery struct {
	Title  string
	Year   *int
	TVDBID int64
	IMDBID string
}

// Candidate is one search result.
type Candidate struct {
	Provider      string
	ID            string
	Title         string
	OriginalTitle string
	Year          *int
	Overview      string
	PosterURL     string // small poster, absolute URL at the provider's image host; may be empty
}

// Text holds the texts of a title in one language.
type Text struct {
	Title    string
	Overview string
	Tagline  string
}

// Person is a cast or crew member. TMDBID is 0 for people of providers without TMDB IDs; such
// people are not stored (people.tmdb_id is the upsert key).
type Person struct {
	TMDBID      int64
	Name        string
	ProfilePath string
}

// Credit roles (credits.role).
const (
	RoleCast     = "cast"
	RoleDirector = "director"
	RoleWriter   = "writer"
	RoleCreator  = "creator"
)

// maxCast is the number of cast members kept per title (design 3.1).
const maxCast = 15

// Credit is one role of a person in a title.
type Credit struct {
	Person    Person
	Role      string
	Character string
	Order     int
}

// TitleRecord is the part movies and series share.
type TitleRecord struct {
	Provider      string
	ID            string
	TMDBID        int64
	TVDBID        int64
	IMDBID        string
	OriginalTitle string
	Date          string // release date (movie) or first air date (series), YYYY-MM-DD or empty
	Runtime       int    // minutes (series: episode run time), 0 when unknown
	Texts         map[string]Text
	Genres        map[string][]string
	// Certifications per country (ISO 3166-1 alpha-2, upper case): the certification of the main
	// release (movie) or the content rating (series), mapped after AN-8.
	Certifications map[string]string
	Credits        []Credit
	PosterPath     string // TMDB image path (/abc.jpg) or absolute artwork URL (TVDB)
	BackdropPath   string
	// Collection is the TMDB collection of a film (E-6); nil for series and films without one.
	Collection *CollectionRecord
}

// CollectionRecord is a TMDB collection with its names and overviews per language (de, en).
type CollectionRecord struct {
	TMDBID     int64
	Name       map[string]string
	Overview   map[string]string
	PosterPath string
}

// MovieRecord is the detail of a movie.
type MovieRecord struct {
	TitleRecord
}

// SeasonInfo is one season of a series as the series detail lists it.
type SeasonInfo struct {
	Number       int
	EpisodeCount int
	PosterPath   string
}

// SeriesRecord is the detail of a series.
type SeriesRecord struct {
	TitleRecord
	Seasons []SeasonInfo
}

// SeasonRecord is the detail of one season with its episodes.
type SeasonRecord struct {
	Number     int
	Texts      map[string]Text
	AirDate    string
	PosterPath string
	Episodes   []EpisodeRecord
}

// EpisodeRecord is the detail of one episode.
type EpisodeRecord struct {
	Season    int
	Number    int
	Texts     map[string]Text
	AirDate   string
	Runtime   int
	StillPath string
}

// pickCandidate returns the first candidate released within year ±1, or the first one when the
// year is unknown. ok is false when none fits.
func pickCandidate(cs []Candidate, year *int) (Candidate, bool) {
	for _, c := range cs {
		if year == nil {
			return c, true
		}
		if c.Year != nil && *c.Year >= *year-1 && *c.Year <= *year+1 {
			return c, true
		}
	}
	return Candidate{}, false
}
