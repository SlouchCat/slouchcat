package arr

import (
	"context"
	"sync"
	"time"
)

// KeyTTL is how long a KeyCache keeps the address and key it read from Seerr (K-23).
const KeyTTL = 10 * time.Minute

// Credentials are the address and the api key of one service, as the caller reads them from
// GET /settings/radarr|sonarr of Seerr or from its own address setting (architecture 20.2).
type Credentials struct {
	BaseURL string
	Key     Secret
}

// KeyCache holds the credentials of one service in memory (never on disk) for KeyTTL. It calls
// fetch only when it holds none, after KeyTTL, or after Invalidate; the caller invalidates after
// an answer ErrKeyInvalid (K-23). A run per minute thus reads Seerr at most every 10 minutes. It is
// safe for concurrent use; concurrent Gets share one fetch.
type KeyCache struct {
	fetch func(context.Context) (Credentials, error)
	ttl   time.Duration
	now   func() time.Time

	mu    sync.Mutex
	creds Credentials
	at    time.Time
	valid bool
}

// KeyCacheOption configures NewKeyCache.
type KeyCacheOption func(*KeyCache)

// WithKeyClock replaces the clock (tests).
func WithKeyClock(now func() time.Time) KeyCacheOption {
	return func(k *KeyCache) {
		if now != nil {
			k.now = now
		}
	}
}

// NewKeyCache returns a cache that reads the credentials with fetch.
func NewKeyCache(fetch func(context.Context) (Credentials, error), opts ...KeyCacheOption) *KeyCache {
	k := &KeyCache{fetch: fetch, ttl: KeyTTL, now: time.Now}
	for _, o := range opts {
		o(k)
	}
	return k
}

// Get returns the held credentials, or fetches them when none are held, they are older than
// KeyTTL or were invalidated. A failed fetch keeps nothing.
func (k *KeyCache) Get(ctx context.Context) (Credentials, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.valid && k.now().Sub(k.at) < k.ttl {
		return k.creds, nil
	}
	creds, err := k.fetch(ctx)
	if err != nil {
		k.valid = false
		k.creds = Credentials{}
		return Credentials{}, err
	}
	k.creds, k.at, k.valid = creds, k.now(), true
	return creds, nil
}

// Invalidate drops the held credentials; the next Get fetches them anew.
func (k *KeyCache) Invalidate() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.valid = false
	k.creds = Credentials{}
}
