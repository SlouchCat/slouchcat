package metadata

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The breaker of an audiobook source (concept 19 D6.3 point 7, ADR 0031): three blocked answers in
// a row (403, 429, HTML instead of JSON or XML) pause the source for six hours. The end of the
// pause is stored as unix milliseconds in the setting metadata.books.paused.<source>, read at the
// first use after a start, so a restart does not ask a blocking source again. The count runs per
// host of the source (Audible asks its catalog and Audnexus, each through its own limiter), the
// pause holds for the whole source: three blocked answers in a row from either host pause it, and a
// good answer resets only the count of the host that gave it. A 404 and an empty result neither
// count nor reset.
const (
	bookBreakerStrikes = 3
	bookBreakerPause   = 6 * time.Hour
	// SettingBookPausedPrefix is the prefix of the settings that hold the pauses of the sources.
	SettingBookPausedPrefix = "metadata.books.paused."
)

// bookBreaker counts the blocked answers of one source per host; a host is named by its limiter,
// which every host of a source has of its own. It is safe for concurrent use.
type bookBreaker struct {
	name  string
	read  *systemq.Queries
	write *systemq.Queries
	now   func() time.Time

	mu      sync.Mutex
	loaded  bool
	strikes map[*ratelimit.Limiter]int
	until   time.Time
}

func newBookBreaker(s *Service, name string) *bookBreaker {
	return &bookBreaker{name: name, read: s.read, write: s.write, now: s.now}
}

// load reads the stored pause once (b.mu held). A failed read is retried at the next use.
func (b *bookBreaker) load(ctx context.Context) {
	if b.loaded {
		return
	}
	v, err := b.read.GetSystemSetting(ctx, SettingBookPausedPrefix+b.name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		b.loaded = true
	case err == nil:
		b.loaded = true
		if ms, perr := strconv.ParseInt(v, 10, 64); perr == nil && ms > 0 {
			b.until = time.UnixMilli(ms)
		}
	}
}

// allow returns errSourcePaused while the source pauses.
func (b *bookBreaker) allow(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.load(ctx)
	if b.now().Before(b.until) {
		return errSourcePaused
	}
	return nil
}

// blocked counts one blocked answer of host; the third in a row starts the pause of the source and
// stores its end.
func (b *bookBreaker) blocked(ctx context.Context, host *ratelimit.Limiter) {
	b.mu.Lock()
	if b.strikes == nil {
		b.strikes = map[*ratelimit.Limiter]int{}
	}
	b.strikes[host]++
	if b.strikes[host] < bookBreakerStrikes {
		b.mu.Unlock()
		return
	}
	clear(b.strikes)
	now := b.now()
	b.until = now.Add(bookBreakerPause)
	b.loaded = true
	until := b.until.UnixMilli()
	b.mu.Unlock()
	// The pause holds in memory even when it cannot be stored; it then ends with a restart.
	_ = b.write.UpsertSystemSetting(context.WithoutCancel(ctx), systemq.UpsertSystemSettingParams{
		Key: SettingBookPausedPrefix + b.name, Value: strconv.FormatInt(until, 10), Now: now.UnixMilli(),
	})
}

// ok resets the count of host after a good answer of it.
func (b *bookBreaker) ok(host *ratelimit.Limiter) {
	b.mu.Lock()
	delete(b.strikes, host)
	b.mu.Unlock()
}

// pausedUntil returns the end of a running pause.
func (b *bookBreaker) pausedUntil(ctx context.Context) (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.load(ctx)
	if b.now().Before(b.until) {
		return b.until, true
	}
	return time.Time{}, false
}
