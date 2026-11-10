package metadata

import (
	"slices"
	"sync"
	"time"
)

// The search cache of the audiobook sources (concept 19 D6.5): the candidates of a search stay
// seven days in memory (gone after a restart), at most 5000 searches; a full cache drops the
// expired entries and then the oldest. The key is source, region and the normalised query
// (bookCacheKey).
const (
	bookCacheTTL = 7 * 24 * time.Hour
	bookCacheMax = 5000
)

type bookCacheEntry struct {
	at    time.Time
	cands []BookCandidate
}

// bookCache is safe for concurrent use.
type bookCache struct {
	now func() time.Time
	max int

	mu      sync.Mutex
	entries map[string]bookCacheEntry
}

func newBookCache(now func() time.Time) *bookCache {
	if now == nil {
		now = time.Now
	}
	return &bookCache{now: now, max: bookCacheMax, entries: map[string]bookCacheEntry{}}
}

// get returns a copy of the candidates stored under key, if they are younger than seven days.
func (c *bookCache) get(key string) ([]BookCandidate, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.now().Sub(e.at) >= bookCacheTTL {
		delete(c.entries, key)
		return nil, false
	}
	return slices.Clone(e.cands), true
}

// put stores a copy of cands under key.
func (c *bookCache) put(key string, cands []BookCandidate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.max {
		oldestKey, oldest := "", time.Time{}
		for k, e := range c.entries {
			if now.Sub(e.at) >= bookCacheTTL {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || e.at.Before(oldest) {
				oldestKey, oldest = k, e.at
			}
		}
		if len(c.entries) >= c.max {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = bookCacheEntry{at: now, cands: slices.Clone(cands)}
}
