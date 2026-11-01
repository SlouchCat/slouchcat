package metadata

import (
	"context"
	"errors"
)

// Provider delivers title data of one external metadata source (design 3.1). Every request runs
// through the provider's ratelimit.Limiter. IDs are the provider's own IDs as decimal strings.
// Languages are BCP-47 codes ("de", "en", "pt-BR"; ADR 0018) from the effective metadata language
// list; a record holds the texts of every requested language the provider knows. ErrNotFound means the provider does not know the ID.
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
	// VoteAverage is the provider's average user vote, 0 to 10; 0 when unknown (items.vote_average,
	// concept 08 4.2). TMDB delivers it; TVDB v4 has none (tvdb.go).
	VoteAverage float64
	Texts       map[string]Text
	Genres      map[string][]string
	// Certifications per country (ISO 3166-1 alpha-2, upper case): the certification of the main
	// release (movie) or the content rating (series), mapped after AN-8.
	Certifications map[string]string
	Credits        []Credit
	PosterPath     string // TMDB image path (/abc.jpg) or absolute artwork URL (TVDB)
	BackdropPath   string
	// Images are every poster, backdrop and logo the provider offers for the title with language,
	// size and votes (ADR 0018, concept 10 6.2); PosterPath and BackdropPath stay the provider's
	// own default (the candidate of step 5 of the choice).
	Images []ImageCandidate
	// Fanart are the Fanart.tv candidates of the title, set by the match before it writes (not by
	// a provider; concept 10 step 6); the choice takes them last.
	Fanart []ImageCandidate
	// Collection is the TMDB collection of a film (E-6); nil for series and films without one.
	Collection *CollectionRecord
}

// ImageCandidate is one image a source offers for a title (concept 10 6.2). Lang is a BCP-47
// primary subtag or "" for an image without text; Source is the provider's address (a TMDB path
// /abc.jpg, an absolute artwork URL, or file:<rel>?<size>-<mtime> for a local file); Votes and
// VoteCount are the provider's rating of the image (0 when it has none).
type ImageCandidate struct {
	Kind      string // poster, backdrop, logo
	Lang      string
	Width     int
	Height    int
	Votes     float64
	VoteCount int
	Source    string
	// Default marks the image the provider itself names for the title (poster_path,
	// backdrop_path): step 5 of the choice, taken when no candidate matches the chain.
	Default bool
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
	Images     []ImageCandidate // season posters with language (ADR 0018)
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
