package radio

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
)

// catalogState is the catalog part of the service (lane B): the Radio Browser sync, the merge of
// the entries into stations, the facets, the in-memory index behind lists, facets and search, the
// favourites and preferences of a profile and the station administration.
type catalogState struct {
	fetch *catalogClient
	rules rules.TagRules
	// zone names the server's IANA time zone (R-9: the default country of a profile).
	zone func() string

	mu  sync.RWMutex
	idx *stationIndex // nil until loaded

	// syncMu keeps a second sync of the same process from running beside the first.
	syncMu sync.Mutex
}

func newCatalogState(userAgent string) *catalogState {
	return &catalogState{fetch: newCatalogClient(userAgent), rules: rules.Default(), zone: serverZone}
}

// serverZone returns the IANA name of the server's time zone: TZ (the compose file sets
// Europe/Zurich), else the target of /etc/localtime; "" when neither names one.
func serverZone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" {
		return tz
	}
	target, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	if _, zone, ok := strings.Cut(filepath.ToSlash(target), "zoneinfo/"); ok {
		return zone
	}
	return ""
}
