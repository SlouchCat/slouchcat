package seerr

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// redacted stands for a secret in every text the package produces.
const redacted = "[redacted]"

// Secret is a value that must not reach a log line, an error text or a JSON answer: the api key of
// Radarr or Sonarr that GET /settings/radarr|sonarr returns. Every fmt verb, slog and json.Marshal
// print "[redacted]"; Reveal returns the value.
type Secret string

// Reveal returns the secret value.
func (s Secret) Reveal() string { return string(s) }

func (s Secret) String() string   { return redacted }
func (s Secret) GoString() string { return `"` + redacted + `"` }

// Format covers every fmt verb, also %v of a struct that holds a Secret.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// LogValue keeps the value out of slog.
func (s Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON keeps the value out of JSON written by Slouchcat; reading JSON into a Secret works as
// for a string.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// Permission is Seerr's permission bit mask (appendix D 7.3).
type Permission uint64

// Permissions (GH:server/lib/permissions.ts).
const (
	PermNone               Permission = 0
	PermAdmin              Permission = 2
	PermManageSettings     Permission = 4
	PermManageUsers        Permission = 8
	PermManageRequests     Permission = 16
	PermRequest            Permission = 32
	PermVote               Permission = 64
	PermAutoApprove        Permission = 128
	PermAutoApproveMovie   Permission = 256
	PermAutoApproveTV      Permission = 512
	PermRequest4K          Permission = 1024
	PermRequest4KMovie     Permission = 2048
	PermRequest4KTV        Permission = 4096
	PermRequestAdvanced    Permission = 8192
	PermRequestView        Permission = 16384
	PermAutoApprove4K      Permission = 32768
	PermAutoApprove4KMovie Permission = 65536
	PermAutoApprove4KTV    Permission = 131072
	PermRequestMovie       Permission = 262144
	PermRequestTV          Permission = 524288
	PermManageIssues       Permission = 1048576
	PermViewIssues         Permission = 2097152
	PermCreateIssues       Permission = 4194304
	PermAutoRequest        Permission = 8388608
	PermAutoRequestMovie   Permission = 16777216
	PermAutoRequestTV      Permission = 33554432
	PermRecentView         Permission = 67108864
	PermWatchlistView      Permission = 134217728
	PermManageBlocklist    Permission = 268435456
	PermViewBlocklist      Permission = 1073741824
)

// Has reports whether p grants all of want; ADMIN grants everything (permissions.ts Z. 60-62).
func (p Permission) Has(want Permission) bool {
	return p&PermAdmin != 0 || p&want == want
}

// UserType is Seerr's user type (GH:server/constants/user.ts).
type UserType int

// User types.
const (
	UserPlex     UserType = 1
	UserLocal    UserType = 2
	UserJellyfin UserType = 3
	UserEmby     UserType = 4
)

// MediaRequestStatus is the status of a request (appendix D 5). Values, never names, are compared.
type MediaRequestStatus int

// Request statuses.
const (
	RequestPending   MediaRequestStatus = 1
	RequestApproved  MediaRequestStatus = 2
	RequestDeclined  MediaRequestStatus = 3
	RequestFailed    MediaRequestStatus = 4
	RequestCompleted MediaRequestStatus = 5
)

// MediaStatus is the status of a media row or a season (appendix D 5). 6 is BLACKLISTED in 2.x and
// BLOCKLISTED in 3.x; the OpenAPI file of Seerr wrongly calls 6 DELETED.
type MediaStatus int

// Media statuses.
const (
	MediaUnknown            MediaStatus = 1
	MediaPending            MediaStatus = 2
	MediaProcessing         MediaStatus = 3
	MediaPartiallyAvailable MediaStatus = 4
	MediaAvailable          MediaStatus = 5
	MediaBlocklisted        MediaStatus = 6
	MediaDeleted            MediaStatus = 7
)

// Media types of requests and titles.
const (
	MediaTypeMovie = "movie"
	MediaTypeTV    = "tv"
)

// Status is the answer of GET /status (appendix D 10).
type Status struct {
	Version         string `json:"version"`
	CommitTag       string `json:"commitTag"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CommitsBehind   int    `json:"commitsBehind"`
	RestartRequired bool   `json:"restartRequired"`
}

// PublicSettings is the part of GET /settings/public the integration reads (appendix D 10).
type PublicSettings struct {
	ApplicationTitle       string `json:"applicationTitle"`
	ApplicationURL         string `json:"applicationUrl"`
	LocalLogin             bool   `json:"localLogin"`
	MediaServerType        int    `json:"mediaServerType"`
	PartialRequestsEnabled bool   `json:"partialRequestsEnabled"`
	EnableSpecialEpisodes  bool   `json:"enableSpecialEpisodes"`
	Movie4KEnabled         bool   `json:"movie4kEnabled"`
	Series4KEnabled        bool   `json:"series4kEnabled"`
	EmailEnabled           bool   `json:"emailEnabled"`
	UserEmailRequired      bool   `json:"userEmailRequired"`
	Locale                 string `json:"locale"`
}

// MainSettings is the part of GET /settings/main the integration reads (appendix I, I-7): what
// every new user gets. A quota field is nil when Seerr leaves it out.
type MainSettings struct {
	DefaultPermissions Permission `json:"defaultPermissions"`
	DefaultQuotas      struct {
		Movie DefaultQuota `json:"movie"`
		TV    DefaultQuota `json:"tv"`
	} `json:"defaultQuotas"`
}

// DefaultQuota is one half of defaultQuotas; limit 0 means no limit.
type DefaultQuota struct {
	QuotaLimit *int `json:"quotaLimit"`
	QuotaDays  *int `json:"quotaDays"`
}

// MetadataSettings is GET /settings/metadatas: the metadata provider for series and anime, "tmdb"
// or "tvdb" (3.x, appendix H 3.4). The field names are not verified against an instance; a missing
// field stays "".
type MetadataSettings struct {
	TV    string `json:"tv"`
	Anime string `json:"anime"`
}

// DVRServer is one Radarr or Sonarr server of GET /settings/radarr|sonarr (appendix H 3.5). APIKey
// is a Secret: printing, logging or marshalling the struct never shows it.
type DVRServer struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Hostname          string `json:"hostname"`
	Port              int    `json:"port"`
	APIKey            Secret `json:"apiKey"`
	UseSSL            bool   `json:"useSsl"`
	BaseURL           string `json:"baseUrl"`
	ActiveProfileID   int    `json:"activeProfileId"`
	ActiveProfileName string `json:"activeProfileName"`
	ActiveDirectory   string `json:"activeDirectory"`
	Tags              []int  `json:"tags"`
	TagRequests       bool   `json:"tagRequests"`
	Is4K              bool   `json:"is4k"`
	IsDefault         bool   `json:"isDefault"`
	ExternalURL       string `json:"externalUrl"`
	SyncEnabled       bool   `json:"syncEnabled"`
}

// User is a Seerr user (appendix D 7.1). The quota fields are nil when the user follows Seerr's
// default quotas.
type User struct {
	ID               int        `json:"id"`
	Email            string     `json:"email"`
	Username         string     `json:"username"`
	PlexUsername     string     `json:"plexUsername"`
	JellyfinUsername string     `json:"jellyfinUsername"`
	JellyfinUserID   string     `json:"jellyfinUserId"`
	DisplayName      string     `json:"displayName"`
	UserType         UserType   `json:"userType"`
	Permissions      Permission `json:"permissions"`
	Avatar           string     `json:"avatar"`
	RequestCount     int        `json:"requestCount"`
	MovieQuotaLimit  *int       `json:"movieQuotaLimit"`
	MovieQuotaDays   *int       `json:"movieQuotaDays"`
	TVQuotaLimit     *int       `json:"tvQuotaLimit"`
	TVQuotaDays      *int       `json:"tvQuotaDays"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// PageInfo is the paging block of Seerr's lists.
type PageInfo struct {
	Pages    int `json:"pages"`
	PageSize int `json:"pageSize"`
	Results  int `json:"results"`
	Page     int `json:"page"`
}

// UserPage is one page of GET /user.
type UserPage struct {
	PageInfo PageInfo `json:"pageInfo"`
	Results  []User   `json:"results"`
}

// NewUser is the body of POST /user: a local user that can sign in with email and password
// (appendix H 3.1). Seerr ignores permissions in this body; set them with SetPermissions.
type NewUser struct {
	Email    string
	Username string
	Password string
}

// QuotaPart is one half of a user's quota. Limit 0 means no limit, and Seerr then leaves out
// Remaining (appendix I, I-31); a user whose limit fields are null shows the instance's
// defaultQuotas (I-32), a user with MANAGE_USERS always limit 0 (I-34). Days, Limit and Remaining
// are nil when Seerr leaves them out.
type QuotaPart struct {
	Days       *int `json:"days"`
	Limit      *int `json:"limit"`
	Used       int  `json:"used"`
	Remaining  *int `json:"remaining"`
	Restricted bool `json:"restricted"`
}

// Quota is GET /user/{id}/quota: movies count requests, series count seasons, declined requests do
// not count (appendix H 3.2).
type Quota struct {
	Movie QuotaPart `json:"movie"`
	TV    QuotaPart `json:"tv"`
}

// SeasonRequest is a season of a request.
type SeasonRequest struct {
	ID           int                `json:"id"`
	SeasonNumber int                `json:"seasonNumber"`
	Status       MediaRequestStatus `json:"status"`
}

// MediaSeason is a season of a media row.
type MediaSeason struct {
	ID           int         `json:"id"`
	SeasonNumber int         `json:"seasonNumber"`
	Status       MediaStatus `json:"status"`
	Status4K     MediaStatus `json:"status4k"`
}

// Media is a media row of Seerr (mediaInfo); ID is Seerr's own id, not the TMDB id.
type Media struct {
	ID        int           `json:"id"`
	TMDBID    int           `json:"tmdbId"`
	TVDBID    *int          `json:"tvdbId"`
	MediaType string        `json:"mediaType"`
	Status    MediaStatus   `json:"status"`
	Status4K  MediaStatus   `json:"status4k"`
	Seasons   []MediaSeason `json:"seasons"`
	Requests  []Request     `json:"requests"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// Request is a media request (MediaRequest).
type Request struct {
	ID          int                `json:"id"`
	Status      MediaRequestStatus `json:"status"`
	Type        string             `json:"type"`
	Is4K        bool               `json:"is4k"`
	Media       *Media             `json:"media"`
	Seasons     []SeasonRequest    `json:"seasons"`
	RequestedBy *User              `json:"requestedBy"`
	ModifiedBy  *User              `json:"modifiedBy"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

// RequestPage is one page of GET /request or GET /user/{id}/requests.
type RequestPage struct {
	PageInfo PageInfo  `json:"pageInfo"`
	Results  []Request `json:"results"`
}

// NewRequest is the body of POST /request. It has no userId on purpose: a request is always made
// on behalf of a user through X-API-User (concept 25, G-3). A series needs TVDBID and Seasons.
type NewRequest struct {
	MediaType string // MediaTypeMovie or MediaTypeTV
	TMDBID    int
	TVDBID    int   // series only
	Seasons   []int // series only, season numbers
	Is4K      bool
}

// ExternalIDs are the ids TMDB knows for a title. Not verified against an instance for GET /tv.
type ExternalIDs struct {
	IMDBID *string `json:"imdbId"`
	TVDBID *int    `json:"tvdbId"`
}

// TVSeason is a season of GET /tv/{id}; with the metadata provider tvdb the list comes from TVDB
// (appendix H 3.4). Field names not verified against an instance.
type TVSeason struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"seasonNumber"`
	EpisodeCount int    `json:"episodeCount"`
	Name         string `json:"name"`
	AirDate      string `json:"airDate"`
}

// MovieDetails is the part of GET /movie/{tmdbId} the integration reads.
type MovieDetails struct {
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	ReleaseDate string       `json:"releaseDate"`
	ExternalIDs *ExternalIDs `json:"externalIds"`
	MediaInfo   *Media       `json:"mediaInfo"`
}

// TVDetails is the part of GET /tv/{tmdbId} the integration reads.
type TVDetails struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	FirstAirDate string       `json:"firstAirDate"`
	Seasons      []TVSeason   `json:"seasons"`
	ExternalIDs  *ExternalIDs `json:"externalIds"`
	MediaInfo    *Media       `json:"mediaInfo"`
}

// Title is one result of a discover list or the search: a movie, a series or (search only) a
// person. Movies carry Title and ReleaseDate, series Name and FirstAirDate. Field names follow
// Seerr's result mapping and are not verified against an instance.
type Title struct {
	ID               int     `json:"id"`
	MediaType        string  `json:"mediaType"`
	Title            string  `json:"title"`
	Name             string  `json:"name"`
	OriginalTitle    string  `json:"originalTitle"`
	OriginalName     string  `json:"originalName"`
	ReleaseDate      string  `json:"releaseDate"`
	FirstAirDate     string  `json:"firstAirDate"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"posterPath"`
	BackdropPath     string  `json:"backdropPath"`
	GenreIDs         []int   `json:"genreIds"`
	OriginalLanguage string  `json:"originalLanguage"`
	VoteAverage      float64 `json:"voteAverage"`
	Popularity       float64 `json:"popularity"`
	MediaInfo        *Media  `json:"mediaInfo"`
}

// MediaTypePerson is the media type of a person among the search hits; never a title.
const MediaTypePerson = "person"

// DisplayName is the title of a film or the name of a series.
func (t Title) DisplayName() string {
	if t.MediaType == MediaTypeMovie && t.Title != "" {
		return t.Title
	}
	if t.Name != "" {
		return t.Name
	}
	return t.Title
}

// Year is the year of the release (film) or the first air date (series); 0 when unknown.
func (t Title) Year() int {
	date := t.FirstAirDate
	if t.MediaType == MediaTypeMovie {
		date = t.ReleaseDate
	}
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

// DiscoverPage is one page of a discover list or the search.
type DiscoverPage struct {
	Page         int     `json:"page"`
	TotalPages   int     `json:"totalPages"`
	TotalResults int     `json:"totalResults"`
	Results      []Title `json:"results"`
}

// Genre is one genre of GET /genres/movie|tv (I-45).
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GenreSlide is one genre tile of GET /discover/genreslider/movie|tv.
type GenreSlide struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Backdrops []string `json:"backdrops"`
}
