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
	// set both.
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
	probe   ownProbeFunc
	// bg runs the background work of the catalog part (own-station probes, the sweep's apply of
	// pending tag rules) on a context Run cancels at shutdown, and Run waits for it; applying keeps
	// a second background apply from starting while one runs.
	bg       background
	applying atomic.Bool
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
	c := &catalogState{fetch: newCatalogClient(userAgent), zone: serverZone}
	c.bg.ctx, c.bg.cancel = context.WithCancel(context.Background())
	return c
}

// background is a set of goroutines on one context that stop ends: it cancels the context, waits
// for the goroutines and refuses new ones, so none writes to the database after the shutdown.
type background struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex // orders go with stop, so no goroutine is added once stop waits
	closed bool
	wg     sync.WaitGroup
}

// goFn runs fn on the background context; false after stop.
func (b *background) goFn(fn func(ctx context.Context)) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.wg.Go(func() { fn(b.ctx) })
	return true
}

// stop cancels the background context and waits for the running goroutines.
func (b *background) stop() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.cancel()
	b.wg.Wait()
}

// wait waits for the running goroutines (tests).
func (b *background) wait() { b.wg.Wait() }

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
