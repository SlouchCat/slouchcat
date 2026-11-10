package metadata

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The breaker of an audiobook source (concept 19 D6.3 point 7, ADR 0031): three blocked answers in
// a row (403, 429, HTML instead of JSON or XML) pause the source for six hours. The end of the
// pause is stored as unix milliseconds in the setting metadata.books.paused.<source>, read at the
// first use after a start, so a restart does not ask a blocking source again. A good answer resets
// the count; a 404 and an empty result neither count nor reset.
const (
	bookBreakerStrikes = 3
	bookBreakerPause   = 6 * time.Hour
	// SettingBookPausedPrefix is the prefix of the settings that hold the pauses of the sources.
	SettingBookPausedPrefix = "metadata.books.paused."
)

// bookBreaker counts the blocked answers of one source. It is safe for concurrent use.
type bookBreaker struct {
	name  string
	read  *systemq.Queries
	write *systemq.Queries
	now   func() time.Time

	mu      sync.Mutex
	loaded  bool
	strikes int
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

// blocked counts one blocked answer; the third in a row starts the pause and stores its end.
func (b *bookBreaker) blocked(ctx context.Context) {
	b.mu.Lock()
	b.strikes++
	if b.strikes < bookBreakerStrikes {
		b.mu.Unlock()
		return
	}
	b.strikes = 0
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

// ok resets the count after a good answer.
func (b *bookBreaker) ok() {
	b.mu.Lock()
	b.strikes = 0
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
