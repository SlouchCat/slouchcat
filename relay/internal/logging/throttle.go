package logging

import (
	"log/slog"
	"sync"
	"time"
)

// Throttle writes a warning at most once per interval for each reason and counts what it held
// back, so a flood of bad requests cannot fill the disk with log lines (security review L-2).
type Throttle struct {
	Log   *slog.Logger
	Every time.Duration
	now   func() time.Time

	mu      sync.Mutex
	last    map[string]time.Time
	skipped map[string]int
}

// NewThrottle returns a throttle for log.
func NewThrottle(log *slog.Logger, every time.Duration) *Throttle {
	return &Throttle{Log: log, Every: every, now: time.Now, last: map[string]time.Time{}, skipped: map[string]int{}}
}

// Warn logs msg with the reason and the number of lines suppressed since the last one, unless one
// for the same reason was written less than Every ago.
func (t *Throttle) Warn(reason, msg string, args ...any) {
	t.mu.Lock()
	now := t.now()
	if last, ok := t.last[reason]; ok && now.Sub(last) < t.Every {
		t.skipped[reason]++
		t.mu.Unlock()
		return
	}
	n := t.skipped[reason]
	t.last[reason] = now
	t.skipped[reason] = 0
	t.mu.Unlock()
	t.Log.Warn(msg, append([]any{"reason", reason, "suppressed", n}, args...)...)
}
