package arr

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// redacted stands for a secret in every text the package produces.
const redacted = "[redacted]"

// Secret is the api key of Radarr or Sonarr. Every fmt verb, slog and json.Marshal print
// "[redacted]"; Reveal returns the value.
type Secret string

// Reveal returns the secret value.
func (s Secret) Reveal() string { return string(s) }

func (s Secret) String() string   { return redacted }
func (s Secret) GoString() string { return `"` + redacted + `"` }

// Format covers every fmt verb, also %v of a struct that holds a Secret.
func (s Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// LogValue keeps the value out of slog.
func (s Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON keeps the value out of JSON written by Slouchcat.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// Tag is a tag of GET /api/v3/tag. Both services store the label in lower case (I-51).
type Tag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// Movie is the part of a Radarr movie the client reads (08-datenmodell-api 6): never path or
// movieFile.path.
type Movie struct {
	ID      int   // the Radarr id, movieId in the queue
	TMDBID  int   // the TMDB id
	Tags    []int // tag ids
	HasFile bool  // Radarr has a file of the movie
}

// Statistics are the episode counts of a series or a season in Sonarr.
type Statistics struct {
	EpisodeFileCount  int `json:"episodeFileCount"`  // episodes with a file
	EpisodeCount      int `json:"episodeCount"`      // monitored or present episodes that aired
	TotalEpisodeCount int `json:"totalEpisodeCount"` // every episode Sonarr knows
}

// Season is a season of a Sonarr series. Statistics is nil when Sonarr sent none: the fields in
// seasons are not verified against an instance (architecture 22.2).
type Season struct {
	Number     int
	Monitored  bool
	Statistics *Statistics
}

// Series is the part of a Sonarr series the client reads: never path.
type Series struct {
	ID         int   // the Sonarr id, seriesId in the queue
	TVDBID     int   // the TVDB id
	Tags       []int // tag ids
	Statistics Statistics
	Seasons    []Season
}

// StatusMessage is one entry of statusMessages of a queue record.
type StatusMessage struct {
	Title    string
	Messages []string
}

// QueueRecord is one entry of GET /api/v3/queue (appendix I, I-56; Sonarr fields after the
// documentation, not verified). Missing fields stay zero and are marked by the *Known flags.
type QueueRecord struct {
	ID      int // the queue entry
	TitleID int // movieId (Radarr) or seriesId (Sonarr); never 0 in Queue.Records

	EpisodeID    int  // Sonarr; 0 when missing
	SeasonNumber int  // Sonarr; season 0 holds the specials
	SeasonKnown  bool // seasonNumber was sent

	Size, SizeLeft int64 // bytes
	SizeKnown      bool  // size > 0 and sizeleft >= 0 were sent

	TimeLeft      time.Duration
	TimeLeftKnown bool      // timeleft was sent and parses (ParseTimeLeft)
	EstimatedEnd  time.Time // estimatedCompletionTime; zero when missing or unreadable

	Status                string // queued, downloading, paused, completed, warning, failed, …
	TrackedDownloadState  string // downloading, importPending, importBlocked, importing, …
	TrackedDownloadStatus string // ok, warning, error
	ErrorMessage          string // shortened and cleaned; only for the administration (N-83)
	StatusMessages        []StatusMessage
	DownloadClient        string
	Protocol              string // usenet, torrent
}

// Queue is the whole queue of one service.
type Queue struct {
	// Records are the entries that belong to a title of the service (movieId resp. seriesId set).
	Records []QueueRecord
	// Skipped counts entries without movieId (Radarr) resp. seriesId (Sonarr): Slouchcat cannot
	// map them to a wish and passes over them (09-umsetzungsplan DoD 4, architecture 22.3).
	Skipped int
	// Total is totalRecords of the last page.
	Total int
}

// ParseTimeLeft reads timeleft of a queue record, a .NET TimeSpan in the form
// [d.]hh:mm:ss[.fffffff] ("00:12:03", "1.02:03:04"). Empty, negative or malformed values give
// false.
func ParseTimeLeft(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "-") {
		return 0, false
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, false
	}
	var days int64
	hoursText := parts[0]
	if d, h, ok := strings.Cut(hoursText, "."); ok {
		n, ok := digits(d)
		if !ok {
			return 0, false
		}
		days, hoursText = n, h
	}
	hours, ok := digits(hoursText)
	if !ok {
		return 0, false
	}
	minutes, ok := digits(parts[1])
	if !ok || minutes > 59 {
		return 0, false
	}
	secText, fracText, hasFrac := strings.Cut(parts[2], ".")
	seconds, ok := digits(secText)
	if !ok || seconds > 59 {
		return 0, false
	}
	var frac time.Duration
	if hasFrac {
		if len(fracText) > 9 {
			fracText = fracText[:9]
		}
		n, ok := digits(fracText)
		if !ok {
			return 0, false
		}
		for range 9 - len(fracText) {
			n *= 10
		}
		frac = time.Duration(n)
	}
	const day = 24 * time.Hour
	if days > 100_000 || hours > 1_000_000 { // far beyond any download; keeps the sum from overflowing
		return 0, false
	}
	return time.Duration(days)*day + time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second + frac, true
}

// digits parses a non-empty run of ASCII digits.
func digits(s string) (int64, bool) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}
