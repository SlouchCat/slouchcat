package model

import (
	"errors"
	"fmt"
	"time"
)

// Webradio of concept 16 stage 1 (ADR 0025): the stations of the Radio Browser catalog, merged into
// one station per broadcaster, the favourites and preferences of a profile, listen sessions with
// their HLS relay and the administration. The contract schemas of the same names in api/openapi.yaml
// (contract 1.17.0) are the source; package radio implements the service, package api maps these
// types to the contract. A zero time, an empty string or a zero number is "absent" in the contract.

// Errors of the radio service (mapped in api/session.go).
var (
	// ErrRadioCapacity: no further relay fits the byte budget or the maximum number of relays
	// (409 radio_capacity; see RadioCapacityError).
	ErrRadioCapacity = errors.New("radio capacity reached")
	// ErrRadioUnreachable: no stream of the station answered, or the relay of a listen session
	// ended (502 radio_unreachable; the heartbeat answers it with 410).
	ErrRadioUnreachable = errors.New("radio station unreachable")
	// ErrRadioNotAudio: the stream answered with something that is no audio (422 radio_not_audio).
	ErrRadioNotAudio = errors.New("radio stream is not audio")
	// ErrRadioCatalogLoading: the catalog has not been synced yet (503 radio_catalog_loading,
	// Retry-After 30).
	ErrRadioCatalogLoading = errors.New("radio catalog loading")
)

// RadioCapacityError is ErrRadioCapacity with the numbers of the problem extensions (running, max).
type RadioCapacityError struct {
	Running int
	Max     int
}

func (e *RadioCapacityError) Error() string {
	return fmt.Sprintf("%v: %d of %d relays running", ErrRadioCapacity, e.Running, e.Max)
}

func (e *RadioCapacityError) Is(target error) bool { return target == ErrRadioCapacity }

// Values of RadioQuery.Sort (contract RadioSort).
const (
	RadioSortPopular = "popular"
	RadioSortName    = "name"
	RadioSortQuality = "quality"
	RadioSortNew     = "new"
)

// Values of RadioStation.State (contract RadioStationState).
const (
	RadioStateOK      = "ok"
	RadioStateBroken  = "broken"
	RadioStateMissing = "missing"
)

// Values of RadioStation.Quality (contract RadioQuality, concept 16 section 5.4).
const (
	RadioQualityHigh  = "high"
	RadioQualityGood  = "good"
	RadioQualityBasic = "basic"
)

// Values of RadioConfig.BufferMode (contract RadioBufferMode).
const (
	RadioBufferTmpfs  = "tmpfs"
	RadioBufferMemory = "memory"
)

// Reasons a listen session ended besides the StoppedBy values (radio_listen_sessions.ended_by).
const (
	RadioEndedExpired = "expired" // no heartbeat and no live request for the buffer time
	RadioEndedStation = "station" // the relay of the station ended
	RadioEndedRestart = "restart" // the server restarted while the session was open
)

// RadioFilter is the filter part of a station query (contract RadioFilter): values of one field
// combine with OR, fields with AND. Stored as JSON in radio_profile_state.last_filter.
type RadioFilter struct {
	Q         string   `json:"q,omitempty"`
	Countries []string `json:"country,omitempty"`  // ISO 3166-1 alpha-2
	Regions   []string `json:"region,omitempty"`   // ISO 3166-2
	Languages []string `json:"language,omitempty"` // ISO 639-1
	Genres    []string `json:"genre,omitempty"`
	Kinds     []string `json:"kind,omitempty"`
	Moods     []string `json:"mood,omitempty"`
	Qualities []string `json:"quality,omitempty"`
	Titles    bool     `json:"titles,omitempty"`    // only stations that show titles
	Favorites bool     `json:"favorites,omitempty"` // only favourites of the profile
}

// RadioQuery is one request of listRadioStations or getRadioFacets.
type RadioQuery struct {
	RadioFilter
	Sort   string // RadioSort*; "" = popular
	Offset int
	Limit  int // 1..120 (the handler applies the default 60)
}

// RadioStation is a station as the app shows it (contract RadioStation). Codec, BitrateKbps and
// HLS describe the best stream (appq ListRadioStreamsRanked).
type RadioStation struct {
	ID           int64
	Name         string
	Country      string // ISO 3166-1 alpha-2; "" = unknown
	Region       string // ISO 3166-2; "" = unknown
	Languages    []string
	Genres       []string
	Kinds        []string
	Moods        []string
	Quality      string // RadioQuality*; "" = unknown
	Codec        string
	BitrateKbps  int // 0 = unknown
	HLS          bool
	ShowsTitles  bool
	Favorite     bool // a favourite of the asking profile
	Recommended  bool
	State        string    // RadioState*
	MissingSince time.Time // zero unless State is missing
	RemovalAt    time.Time // MissingSince + 7 days
	// LogoHash is the content hash of the station's logo; "" = no logo (the app shows initials).
	// The API builds the image URL from it.
	LogoHash string
	Homepage string
}

// RadioStationPage is one page of listRadioStations.
type RadioStationPage struct {
	Items  []RadioStation
	Total  int
	Offset int
}

// RadioFacetCount is one value of a facet with its number of stations.
type RadioFacetCount struct {
	Value string
	Count int
}

// RadioFacets are the counts per value of every filter (contract RadioFacets); the counts of a facet
// ignore the facet's own selection. Titles counts the stations that show titles.
type RadioFacets struct {
	Countries []RadioFacetCount
	Regions   []RadioFacetCount
	Languages []RadioFacetCount
	Genres    []RadioFacetCount
	Kinds     []RadioFacetCount
	Moods     []RadioFacetCount
	Qualities []RadioFacetCount
	Titles    int
}

// RadioListen is a new listen session (contract RadioListen). Token is the listen token of the live
// routes; the API puts it into the playlist URL as rt and never logs it.
type RadioListen struct {
	ListenID           string
	Token              string
	HeartbeatSeconds   int // 30
	StartOffsetSeconds int // -8
	Station            RadioStation
}

// RadioHeartbeat answers a heartbeat (contract RadioHeartbeat); WatchTime is nil for a profile
// without a daily limit.
type RadioHeartbeat struct {
	ExpiresAt time.Time
	WatchTime *WatchTime
}

// RadioNowEntry is one title the stream announced; At is the program date-time of the segment it
// arrived in. Artist and Title are set when Text splits into both.
type RadioNowEntry struct {
	At     time.Time
	Text   string
	Artist string
	Title  string
}

// RadioNow is the title timeline of a station's running relay (Live false: none runs).
type RadioNow struct {
	Live    bool
	Entries []RadioNowEntry
}

// RadioPreferences are the radio preferences of a profile (F-12). DefaultCountry is filled by the
// service when the profile has none (server time zone, else the metadata rating country).
type RadioPreferences struct {
	DefaultCountry string
	LastFilter     *RadioFilter // nil = none
	LastSort       string       // RadioSort*; "" = none
}

// RadioGrant is what radioAuth resolved for a live request: the listen session, its station and the
// profile it plays for.
type RadioGrant struct {
	ListenID  string
	StationID int64
	Profile   Profile
}

// RadioStationFacet is one derived or manual filter value of a station (admin view).
type RadioStationFacet struct {
	Facet  string // genre, kind, mood
	Value  string
	Manual bool
}

// RadioStream is one stream of a station (admin view); Rank 0 is the stream the relay plays.
type RadioStream struct {
	ID          int64
	URL         string
	URLResolved string
	Codec       string
	BitrateKbps int
	HLS         bool
	Reachable   bool
	Preferred   bool
	Rank        int
}

// AdminRadioStation is the administration view of a station (contract AdminRadioStation); the
// embedded RadioStation's Favorite is not used.
type AdminRadioStation struct {
	RadioStation
	Source     string // catalog, own
	Hidden     bool
	Failures   int
	BrokenAt   time.Time // zero = not marked by the server's own relay
	Tags       []string
	Facets     []RadioStationFacet
	Locks      []string // RadioLockField values
	LogoState  string   // none, queued, catalog, website, own, failed
	AliasCount int
	Streams    []RadioStream // best first
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AdminRadioQuery is one request of listAdminRadioStations.
type AdminRadioQuery struct {
	Q          string
	Country    string
	Genre      string
	State      string // reachable, broken, hidden; "" = any
	Exceptions bool   // only changed, hidden, broken and recommended stations
	Offset     int
	Limit      int
}

// AdminRadioStationPage is one page of listAdminRadioStations.
type AdminRadioStationPage struct {
	Items  []AdminRadioStation
	Total  int
	Offset int
}

// AdminRadioStationPatch changes a station (contract AdminRadioStationPatch): nil keeps a field. A
// set field locks itself; Locks, when set, replaces the set of locks. ClearRegion and
// ClearPreferredStream come from a JSON null.
type AdminRadioStationPatch struct {
	Name                 *string
	Region               *string
	ClearRegion          bool
	Genres               *[]string
	Kinds                *[]string
	Moods                *[]string
	PreferredStreamID    *int64
	ClearPreferredStream bool
	Hidden               *bool
	Recommended          *bool
	Locks                *[]string
}

// RadioProbe is the result of probing a stream (contract RadioProbe).
type RadioProbe struct {
	Format      string
	Codec       string
	BitrateKbps int
	SampleRate  int
	Channels    int
	HasMetadata bool
	Title       string // first title seen; "" = none
	Response    time.Duration
}

// RadioConfig are the radio settings (contract RadioConfig, table radio_config). Countries nil
// admits every country.
type RadioConfig struct {
	BufferMinutes     int
	BudgetMB          int
	MaxRelays         int
	GraceSeconds      int
	HideBroken        bool
	Countries         []string
	BufferMode        string // RadioBuffer*
	MaxRelaysByBudget int
}

// RadioConfigUpdate changes radio settings (contract RadioConfigUpdate): nil keeps a field,
// ClearCountries (a JSON null) admits every country again.
type RadioConfigUpdate struct {
	BufferMinutes  *int
	BudgetMB       *int
	MaxRelays      *int
	GraceSeconds   *int
	HideBroken     *bool
	Countries      *[]string
	ClearCountries bool
}

// RadioCatalogState is the state of the catalog and its sync (contract RadioCatalogState).
type RadioCatalogState struct {
	Source           string // radio-browser
	LastSyncAt       time.Time
	LastSyncError    string // redacted
	SyncFailures     int
	EntriesTotal     int
	EntriesReachable int
	StationsTotal    int
	LastAdded        int
	LastRemoved      int
	NextRunAt        time.Time
	RunningJobID     int64 // 0 = none
}

// RadioListenerInfo is one listener of a running relay (admin view).
type RadioListenerInfo struct {
	ListenID     string
	ProfileName  string
	AccountEmail string
	Playing      bool
	Since        time.Time
}

// RadioRelayInfo is one running relay (admin view, contract RadioRelay).
type RadioRelayInfo struct {
	StationID     int64
	Name          string
	Codec         string
	BitrateKbps   int
	Transcoded    bool
	StartedAt     time.Time
	BufferSeconds int
	BufferBytes   int64
	State         string // connecting, live, reconnecting, ending
	Listeners     []RadioListenerInfo
}

// RadioLoad is the load of all relays together (contract RadioLoad).
type RadioLoad struct {
	Relays      int
	MaxRelays   int
	BufferBytes int64
	BudgetBytes int64
	UploadKbps  int
	BufferMode  string // RadioBuffer*
}

// RadioRelays are the running relays and the load (contract RadioRelays).
type RadioRelays struct {
	Relays []RadioRelayInfo
	Load   RadioLoad
}
