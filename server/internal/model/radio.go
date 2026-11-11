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
	// ErrRadioOfferConflict: the offer changed since the version a draft is based on (409
	// offer_conflict, contract 1.28.0).
	ErrRadioOfferConflict = errors.New("radio offer changed")
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
	// Provider is the registrable domain of the best stream's host (contract 1.28.0); "" = unknown.
	Provider string
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
// ignore the facet's own selection. Titles counts the stations that show titles; OfferEmpty says
// that the offer holds no station at all (contract 1.28.0).
type RadioFacets struct {
	Countries  []RadioFacetCount
	Regions    []RadioFacetCount
	Languages  []RadioFacetCount
	Genres     []RadioFacetCount
	Kinds      []RadioFacetCount
	Moods      []RadioFacetCount
	Qualities  []RadioFacetCount
	Titles     int
	OfferEmpty bool
}

// RadioListen is a new listen session (contract RadioListen). Token is the listen token of the live
// routes; the API puts it into the playlist URL as rt and never logs it.
type RadioListen struct {
	ListenID           string
	Token              string
	HeartbeatSeconds   int // 30
	StartOffsetSeconds int // -8
	// TimeShiftSeconds is how far behind live the listener may play (contract 1.19.0): the
	// buffer of the relay, 0 in memory mode (no tmpfs, live only).
	TimeShiftSeconds int
	Station          RadioStation
}

// RadioHeartbeat answers a heartbeat (contract RadioHeartbeat); WatchTime is nil for a profile
// without a daily limit.
type RadioHeartbeat struct {
	ExpiresAt time.Time
	WatchTime *WatchTime
}

// RadioNowEntry is one title the stream announced; At is the program date-time of the audio it
// starts at (stage 2: frames counted for MP3 and AAC, else the segment it arrived in). Artist and
// Title are set when Text splits into both. CoverTrackID and CoverHash name the cover found for it
// (radio_tracks; 0 and "" while none is known).
type RadioNowEntry struct {
	At           time.Time
	Text         string
	Artist       string
	Title        string
	CoverTrackID int64
	CoverHash    string
}

// RadioHomeStation is one tile of the start page row radio («Meine Sender», contract 1.19.0): a
// favourite and, while its relay runs, the title at the live edge (nil otherwise).
type RadioHomeStation struct {
	Station RadioStation
	Now     *RadioNowEntry
}

// RadioNow is the title timeline of a station's running relay (Live false: none runs).
type RadioNow struct {
	Live    bool
	Entries []RadioNowEntry
}

// RadioPreferences are the radio preferences of a profile (F-12). DefaultCountry is filled by the
// service when the profile has none (server time zone, else the metadata rating country).
// EffectiveCountry is DefaultCountry while the offer lists stations of it, else "" (contract
// 1.28.0, OB-10); the service ignores it in SetPreferences.
type RadioPreferences struct {
	DefaultCountry   string
	EffectiveCountry string
	LastFilter       *RadioFilter // nil = none
	LastSort         string       // RadioSort*; "" = none
}

// RadioGrant is what radioAuth resolved for a live request: the listen session, its station and the
// profile it plays for.
type RadioGrant struct {
	ListenID  string
	StationID int64
	Profile   Profile
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
	Offered    bool   // in the stored offer (contract 1.28.0)
	Failures   int
	BrokenAt   time.Time // zero = not marked by the server's own relay
	Tags       []string
	Locks      []string // RadioLockField values
	LogoState  string   // none, queued, catalog, website, own, failed
	AliasCount int
	Streams    []RadioStream // best first
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AdminRadioQuery is one request of listAdminRadioStations (contract 1.28.0).
type AdminRadioQuery struct {
	Q        string
	Country  string
	Provider string // exact; "" = any
	State    string // reachable, broken, noLogo; "" = any
	Source   string // catalog, own; "" = any (contract 1.19.0)
	Offered  bool   // only the stations of the stored offer; false = the whole catalog
	Sort     string // popularity (""), name
	Offset   int
	Limit    int
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
	PreferredStreamID    *int64
	ClearPreferredStream bool
	Recommended          *bool
	Locks                *[]string
	// Own stations only (contract 1.19.0): the stream address, the homepage and the country;
	// ClearHomepage and ClearCountry come from a JSON null.
	StreamURL     *string
	Homepage      *string
	ClearHomepage bool
	Country       *string
	ClearCountry  bool
	Languages     *[]string // ISO 639-1; an empty list clears them
}

// AdminRadioStationCreate is an own station (contract AdminRadioStationCreate, concept 16 section
// 5.6). Empty strings and nil lists are absent.
type AdminRadioStationCreate struct {
	Name      string
	StreamURL string
	Homepage  string
	Country   string
	Region    string
	Languages []string // ISO 639-1
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

// RadioConfig are the radio settings (contract RadioConfig, table radio_config).
type RadioConfig struct {
	BufferMinutes     int
	BudgetMB          int
	MaxRelays         int
	GraceSeconds      int
	BufferMode        string // RadioBuffer*
	MaxRelaysByBudget int
	CoverMusicBrainz  bool // contract 1.19.0
	CoverITunes       bool
}

// RadioConfigUpdate changes radio settings (contract RadioConfigUpdate): nil keeps a field.
type RadioConfigUpdate struct {
	BufferMinutes    *int
	BudgetMB         *int
	MaxRelays        *int
	GraceSeconds     *int
	CoverMusicBrainz *bool
	CoverITunes      *bool
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
	// Device is the device of the listen session's login session; nil without one.
	Device *DeviceRef
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
	State         string // connecting, live, reconnecting, ending, failed
	Listeners     []RadioListenerInfo
	// LogoHash is the content hash of the station's logo ("" without one).
	LogoHash string
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

// Radio offer of concept 19 part B (contract 1.28.0): the chosen countries with their mode and the
// exceptions per station decide which stations the app shows (B4.1).

// Values of RadioOfferCountry.Mode (contract RadioOfferMode).
const (
	RadioOfferAll    = "all"
	RadioOfferMarked = "marked"
)

// Values of RadioOfferStep.Op (contract RadioOfferStepOp).
const (
	RadioOfferChooseCountry     = "chooseCountry"
	RadioOfferRemoveCountry     = "removeCountry"
	RadioOfferMarkAll           = "markAll"
	RadioOfferDemarkAll         = "demarkAll"
	RadioOfferMark              = "mark"
	RadioOfferDemark            = "demark"
	RadioOfferMarkMatching      = "markMatching"
	RadioOfferDemarkMatching    = "demarkMatching"
	RadioOfferDemarkUnreachable = "demarkUnreachable"
	RadioOfferSetHideBroken     = "setHideBroken"
)

// RadioOfferStep is one step of a draft (contract RadioOfferStep); the fields an op does not use
// are ignored.
type RadioOfferStep struct {
	Op         string
	Country    string
	Q          string
	StationIDs []int64
	Value      bool
}

// RadioOfferListQuery asks a preview for one page of the catalog stations of a country.
type RadioOfferListQuery struct {
	Country string
	Q       string
	Offset  int
	Limit   int // 1..100 (the handler applies the default 100)
}

// RadioOfferTotals are the sums of an offer (contract RadioOfferTotals).
type RadioOfferTotals struct {
	Offered              int
	OfferedUnreachable   int
	Countries            int
	Marked               int
	Demarked             int
	Own                  int
	WithLogo             int
	RemovableUnreachable int
}

// RadioOfferCountry is one country of an offer; Mode "" = not chosen.
type RadioOfferCountry struct {
	Code        string
	Continent   string
	Mode        string
	Offered     int
	Total       int
	Unreachable int
}

// RadioOffer is the stored offer or the offer of a draft (contract RadioOffer).
type RadioOffer struct {
	Version    int64
	HideBroken bool
	Totals     RadioOfferTotals
	Countries  []RadioOfferCountry
}

// RadioOfferStation is one station of the list of a preview; LogoHash "" = no logo in the cache.
type RadioOfferStation struct {
	ID       int64
	Name     string
	LogoHash string
	Tags     []string // at most five
	Broken   bool
	Offered  bool
}

// RadioOfferStationPage is the list of a preview.
type RadioOfferStationPage struct {
	Total  int
	Offset int
	Items  []RadioOfferStation
}

// RadioOfferPreview is the result of previewRadioOffer: the draft's offer with the stored version,
// the difference to the stored offer and, when asked, a page of the list.
type RadioOfferPreview struct {
	RadioOffer
	Added   int
	Removed int
	List    *RadioOfferStationPage
}

// RadioOfferApplied is the result of applyRadioOffer.
type RadioOfferApplied struct {
	Offer              RadioOffer
	QueuedLogos        int
	DemarkedStationIDs []int64
}
