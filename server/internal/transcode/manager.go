package transcode

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

// ErrBusy is returned by Start when the transcode budget stays exhausted for admitWait.
var ErrBusy = errors.New("transcode budget exhausted")

const (
	defaultIdleTimeout = 60 * time.Second
	// admitWait is how long Start waits for budget (plan 3.5: up to 10 s, then reject).
	admitWait = 10 * time.Second
)

// validID accepts playback session ids (ULIDs) and similar safe directory names.
var validID = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

// ManagerOptions configures a Manager.
type ManagerOptions struct {
	Runner      Runner
	WorkDir     string // <cache>/transcode, created, wiped at start
	Backend     Backend
	Caps        Capabilities
	Budget      float64 // per backend: nvenc 12, qsv/vaapi 8, software cores/2; 0 = default
	Log         *slog.Logger
	IdleTimeout time.Duration // kill ffmpeg after no request for this long (default 60 s); files stay
}

// Stats is a snapshot of the manager for calibration and benchmarks.
type Stats struct {
	Backend  Backend
	Budget   float64
	Used     float64 // cost of the sessions whose ffmpeg runs
	Sessions int     // registered sessions
	Running  int     // sessions with a running ffmpeg
}

// Manager owns all HLS sessions of the process and the admission control (plan 3.5): the cost of
// the sessions with a running ffmpeg never exceeds the budget, except that a session resuming
// after an idle stop always gets its budget back (live playback must not fail mid-film).
type Manager struct {
	runner    Runner
	caps      Capabilities
	dir       string
	budget    float64
	idle      time.Duration
	log       *slog.Logger
	admitWait time.Duration

	mu       sync.Mutex
	sessions map[string]*Session
	used     float64
	freed    chan struct{} // closed and replaced when budget is released
	closed   bool
	stop     chan struct{}
	stopped  chan struct{}
}

// DefaultBudget is the start value of the budget of a backend (plan 3.5), replaced by the
// calibrated values of docs/perf/M1b.md through ManagerOptions.Budget.
func DefaultBudget(b Backend) float64 {
	switch b {
	case BackendNVENC:
		return 12
	case BackendQSV, BackendVAAPI:
		return 8
	default:
		return max(1, float64(runtime.NumCPU())/2)
	}
}

// NewManager wipes and creates the work directory and starts the idle timer.
func NewManager(o ManagerOptions) (*Manager, error) {
	if o.Runner.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	dir := filepath.Clean(o.WorkDir)
	if !filepath.IsAbs(dir) || dir == filepath.Dir(dir) {
		return nil, fmt.Errorf("transcode work dir %q must be an absolute directory below the root", o.WorkDir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("wipe transcode work dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create transcode work dir: %w", err)
	}
	caps := o.Caps
	if o.Backend != "" {
		caps.Backend = o.Backend
	}
	if caps.Backend == "" {
		caps.Backend = BackendSoftware
	}
	budget := o.Budget
	if budget <= 0 {
		budget = DefaultBudget(caps.Backend)
	}
	idle := o.IdleTimeout
	if idle <= 0 {
		idle = defaultIdleTimeout
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := &Manager{
		runner: o.Runner, caps: caps, dir: dir, budget: budget, idle: idle, log: log, admitWait: admitWait,
		sessions: map[string]*Session{}, freed: make(chan struct{}),
		stop: make(chan struct{}), stopped: make(chan struct{}),
	}
	go m.idleLoop()
	return m, nil
}

// Start admits the session (waits up to 10 s for budget, then ErrBusy), starts ffmpeg at segment
// 0 and returns. id is the playback session id (ULID). A session that alone exceeds the budget is
// admitted when nothing else runs.
func (m *Manager) Start(ctx context.Context, id string, src Source, plan Plan) (*Session, error) {
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("invalid transcode session id %q", id)
	}
	if len(plan.Segments) == 0 {
		return nil, errors.New("plan has no segments")
	}
	timer := time.NewTimer(m.admitWait)
	defer timer.Stop()
	m.mu.Lock()
	for {
		if m.closed {
			m.mu.Unlock()
			return nil, ErrClosed
		}
		if _, ok := m.sessions[id]; ok {
			m.mu.Unlock()
			return nil, fmt.Errorf("transcode session %s exists", id)
		}
		if m.used == 0 || m.used+plan.Cost <= m.budget {
			break
		}
		freed := m.freed
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			m.log.Warn("transcode budget exhausted", "session", id, "cost", plan.Cost, "budget", m.budget)
			return nil, ErrBusy
		case <-freed:
		}
		m.mu.Lock()
	}
	s := &Session{
		m: m, id: id, dir: filepath.Join(m.dir, id), src: src, plan: plan, log: m.log,
		changed: make(chan struct{}), done: make([]bool, len(plan.Segments)), missing: map[int]bool{},
		live: map[*run]struct{}{}, lastUse: time.Now(), active: true,
	}
	m.used += plan.Cost
	m.sessions[id] = s
	m.mu.Unlock()

	err := os.MkdirAll(s.dir, 0o750)
	if err == nil {
		s.mu.Lock()
		err = s.startLocked(0)
		s.mu.Unlock()
	}
	if err != nil {
		m.Stop(id)
		return nil, err
	}
	m.log.Info("transcode session started", "session", id, "mode", plan.Target.Mode, "backend", s.Backend(),
		"segments", len(plan.Segments), "cost", plan.Cost)
	return s, nil
}

// Get returns a running session.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Stop kills the session's ffmpeg, waits for it and removes the session directory.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

// Close stops every session and the idle timer (server shutdown).
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	sessions := make([]*Session, 0, len(m.sessions))
	for id, s := range m.sessions {
		sessions = append(sessions, s)
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	close(m.stop)
	<-m.stopped
	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Go(s.close)
	}
	wg.Wait()
}

// Stats returns a snapshot of the budget and the sessions.
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	st := Stats{Backend: m.caps.Backend, Budget: m.budget, Used: m.used, Sessions: len(m.sessions)}
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		if s.running() {
			st.Running++
		}
	}
	return st
}

// acquire takes budget without waiting (resume of an admitted session).
func (m *Manager) acquire(cost float64) {
	m.mu.Lock()
	m.used += cost
	m.mu.Unlock()
}

// release returns budget and wakes waiting Starts.
func (m *Manager) release(cost float64) {
	m.mu.Lock()
	m.used -= cost
	if m.used < 1e-9 {
		m.used = 0
	}
	close(m.freed)
	m.freed = make(chan struct{})
	m.mu.Unlock()
}

// idleLoop stops ffmpeg of sessions without requests for the idle timeout.
func (m *Manager) idleLoop() {
	defer close(m.stopped)
	t := time.NewTicker(max(m.idle/4, 10*time.Millisecond))
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-t.C:
			m.mu.Lock()
			sessions := make([]*Session, 0, len(m.sessions))
			for _, s := range m.sessions {
				sessions = append(sessions, s)
			}
			m.mu.Unlock()
			for _, s := range sessions {
				s.idleKill(now, m.idle)
			}
		}
	}
}
