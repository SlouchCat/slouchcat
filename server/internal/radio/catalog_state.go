package radio

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// catalogState is the catalog part of the service (lane B): the Radio Browser sync, the merge of
// the entries into stations, the facets, the in-memory index behind lists, facets and search, the
// favourites and preferences of a profile and the station administration.
type catalogState struct {
	fetch *catalogClient
	// rules are the effective tag rules (tagrules.go): nil until the first load, swapped by
	// SetTagRules under rulesMu, which also orders the loads with the swaps.
	rules   atomic.Pointer[ruleSet]
	rulesMu sync.Mutex
	// rulesDirty is set while stored rules wait to be applied to the stations: an apply that met
	// a running sync (the sync applies them at its end) or failed (the sweep retries it; Start sets it while
	// admin rules exist, so an apply a restart cut off still reaches every station).
	rulesDirty atomic.Bool
	// gen counts the changes the tab «Einordnung» depends on (index swaps, rule and config
	// changes); tagRulesCache holds its answer for one generation.
	gen           atomic.Uint64
	tagRulesMu    sync.Mutex
	tagRulesCache *tagRulesAnswer
	// resolve looks up the host of an own station's address (own.go); nil is the system
	// resolver. probe reads an own station's stream after a create (nil: probeStream). Tests
	// set both; probes counts the background probes, so a test can wait for them.
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
	probe   ownProbeFunc
	probes  sync.WaitGroup
	// zone names the server's IANA time zone (R-9: the default country of a profile).
	zone func() string

	mu  sync.RWMutex
	idx *stationIndex // nil until loaded
	// writeMu orders the writers of idx (rebuildIndex, reloadStations): each reads its rows and
	// swaps idx under it.
	writeMu sync.Mutex

	// syncMu keeps a second sync of the same process from running beside the first.
	syncMu sync.Mutex
}

func newCatalogState(userAgent string) *catalogState {
	return &catalogState{fetch: newCatalogClient(userAgent), zone: serverZone}
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

// stationsChanged marks a change of the catalog the cached answers depend on.
func (c *catalogState) stationsChanged() { c.gen.Add(1) }
