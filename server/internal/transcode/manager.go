package transcode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sync"
	"time"
)

// ErrBusy is returned by Start when the transcode budget stays exhausted for admitWait.
var ErrBusy = errors.New("transcode budget exhausted")

const (
	defaultIdleTimeout = 60 * time.Second
	// defaultMaxAhead is how many complete segments ffmpeg may run ahead of the player (32 s).
	defaultMaxAhead = 8
	// defaultRemoveAfterIdle is when an idle session loses its files and turns cold.
	defaultRemoveAfterIdle = 10 * time.Minute
	// admitWait is how long Start waits for budget (plan 3.5: up to 10 s, then reject).
	admitWait = 10 * time.Second
	// incompleteWait bounds the wait for the data of a listed segment (ADR 0003, Ablage).
	incompleteWait = 2 * time.Second
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
	// MaxAhead pauses ffmpeg once it has more than this many complete segments beyond the highest
	// segment requested since it started (default 8 ≈ 32 s; negative: never pause). The next
	// request for a missing segment restarts it there.
	MaxAhead int
	// RemoveAfterIdle removes the files of a session without requests for this long and releases
	// its budget (default 10 min; negative: never). The session stays registered ("cold") and the
	// next request restarts ffmpeg at the requested segment.
	RemoveAfterIdle time.Duration
}

// Stats is a snapshot of the manager for calibration and benchmarks.
type Stats struct {
	Backend   Backend
	Budget    float64
	Used      float64 // cost of the sessions holding budget (ffmpeg running or paused ahead)
	Sessions  int     // registered sessions
	Running   int     // sessions with a running ffmpeg
	Cold      int     // sessions whose files were removed after RemoveAfterIdle
	DiskBytes int64   // size of the files in the work directory
	// Downloads counts the download leases (AcquireDownload) holding budget, preempted ones until
	// they are released; DownloadUsed is their cost. Used + DownloadUsed is the budget in use.
	Downloads    int
	DownloadUsed float64
}

// Manager owns all HLS sessions of the process and the admission control (plan 3.5): the cost of
// the sessions holding budget never exceeds it, except that a session resuming after an idle stop
// always gets its budget back (live playback must not fail mid-film). Waiting Starts are admitted
// in arrival order. Download encodes hold budget through leases (AcquireDownload) and yield to
// live playback.
type Manager struct {
	runner         Runner
	caps           Capabilities
	dir            string
	budget         float64
	idle           time.Duration
	removeAfter    time.Duration
	maxAhead       int
	log            *slog.Logger
	admitWait      time.Duration
	incompleteWait time.Duration
	ctx            context.Context // parent of every ffmpeg; canceled by Close
	cancel         context.CancelFunc
	afterRegister  func() // test hook between registering a session and starting its ffmpeg

	mu       sync.Mutex
	sessions map[string]*Session
	used     float64
	waiting  []*admission     // Starts waiting for budget, in arrival order
	leases   []*DownloadLease // download leases holding budget, oldest first
	freed    chan struct{}    // closed and replaced when budget is released or the queue changes
	closed   bool
	stop     chan struct{}
	stopped  chan struct{}
}

// admission is one Start waiting for budget.
type admission struct{ cost float64 }

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
	maxAhead := o.MaxAhead
	if maxAhead == 0 {
		maxAhead = defaultMaxAhead
	}
	removeAfter := o.RemoveAfterIdle
	if removeAfter == 0 {
		removeAfter = defaultRemoveAfterIdle
	}
	log := o.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		runner: o.Runner, caps: caps, dir: dir, budget: budget, idle: idle, removeAfter: removeAfter,
		maxAhead: maxAhead, log: log, admitWait: admitWait, incompleteWait: incompleteWait, ctx: ctx, cancel: cancel,
		sessions: map[string]*Session{}, freed: make(chan struct{}),
		stop: make(chan struct{}), stopped: make(chan struct{}),
	}
	go m.idleLoop()
	return m, nil
}

// Start admits the session (waits up to 10 s for budget, in arrival order, then ErrBusy), starts
// ffmpeg at startSegment (0 for playback from the start, the resume segment otherwise) and
// returns. id is the playback session id (ULID). A session that alone exceeds the budget is
// admitted when nothing else holds budget. When download leases hold the budget it lacks, the
// first waiting Start preempts them, newest first, as far as that makes room, and is admitted
// once they are released.
func (m *Manager) Start(ctx context.Context, id string, src Source, plan Plan, startSegment int) (*Session, error) {
	if !validID.MatchString(id) {
		return nil, fmt.Errorf("invalid transcode session id %q", id)
	}
	if len(plan.Segments) == 0 {
		return nil, errors.New("plan has no segments")
	}
	if startSegment < 0 || startSegment >= len(plan.Segments) {
		return nil, fmt.Errorf("%w: start segment %d of %d", ErrNoSegment, startSegment, len(plan.Segments))
	}
	timer := time.NewTimer(m.admitWait)
	defer timer.Stop()
	var w *admission
	m.mu.Lock()
	for {
		if m.closed {
			m.leaveLocked(w)
			m.mu.Unlock()
			return nil, ErrClosed
		}
		if _, ok := m.sessions[id]; ok {
			m.leaveLocked(w)
			m.mu.Unlock()
			return nil, fmt.Errorf("transcode session %s exists", id)
		}
		cost, held := m.cost(plan), m.used+m.leaseUnitsLocked(false)
		first := len(m.waiting) == 0 || m.waiting[0] == w
		if first && (held == 0 || held+cost <= m.budget) {
			m.leaveLocked(w)
			break
		}
		if first && (m.used == 0 || m.used+cost <= m.budget) {
			// Without the download leases there is room: live playback has priority.
			m.preemptLocked(func(leases float64) bool { return m.used+leases == 0 || m.used+leases+cost <= m.budget })
		}
		if w == nil {
			w = &admission{cost: m.cost(plan)}
			m.waiting = append(m.waiting, w)
		}
		freed := m.freed
		m.mu.Unlock()
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer.C:
			m.log.Warn("transcode budget exhausted", "session", id, "cost", m.cost(plan), "budget", m.budget)
			err = ErrBusy
		case <-freed:
		}
		m.mu.Lock()
		if err != nil {
			m.leaveLocked(w)
			m.mu.Unlock()
			return nil, err
		}
	}
	s := &Session{
		m: m, id: id, dir: filepath.Join(m.dir, id), src: src, plan: plan, log: m.log,
		changed: make(chan struct{}), done: make([]bool, len(plan.Segments)), missing: map[int]bool{},
		live: map[*run]struct{}{}, failures: map[int]failure{}, lastUse: time.Now(), active: true,
		front: startSegment,
	}
	m.used += m.cost(plan)
	m.sessions[id] = s
	m.mu.Unlock()

	if m.afterRegister != nil {
		m.afterRegister()
	}
	s.mu.Lock()
	err := s.startLocked(startSegment)
	s.mu.Unlock()
	if err != nil {
		m.mu.Lock()
		if m.sessions[id] == s {
			delete(m.sessions, id)
		}
		m.mu.Unlock()
		s.close()
		return nil, err
	}
	m.log.Info("transcode session started", "session", id, "mode", plan.Target.Mode, "backend", s.Backend(),
		"segments", len(plan.Segments), "start", startSegment, "cost", m.cost(plan))
	return s, nil
}

// leaveLocked removes w (nil: nothing) from the admission queue and wakes the other waiters, so
// the new head re-checks the budget.
func (m *Manager) leaveLocked(w *admission) {
	if w == nil {
		return
	}
	if i := slices.Index(m.waiting, w); i >= 0 {
		m.waiting = slices.Delete(m.waiting, i, i+1)
	}
	m.wakeLocked()
}

func (m *Manager) wakeLocked() {
	close(m.freed)
	m.freed = make(chan struct{})
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
	m.cancel() // every ffmpeg gets SIGTERM, even one of a session that is still starting
	// Download encodes stop too.
	for _, l := range m.leases {
		if !l.stopping {
			l.stopping = true
			close(l.preempted)
		}
	}
	m.wakeLocked()
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

// Stats returns a snapshot of the budget, the sessions and the disk usage of the work directory.
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	st := Stats{Backend: m.caps.Backend, Budget: m.budget, Used: m.used, Sessions: len(m.sessions),
		Downloads: len(m.leases), DownloadUsed: m.leaseUnitsLocked(false)}
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		running, cold := s.state()
		if running {
			st.Running++
		}
		if cold {
			st.Cold++
		}
	}
	_ = filepath.WalkDir(m.dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // files come and go while ffmpeg runs; count what is there
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				st.DiskBytes += fi.Size()
			}
		}
		return nil
	})
	return st
}

// cost is the admission cost of a plan on this manager's backend: the plan's cost (plan 3.5),
// raised to costHDRCPU for an HDR transcode whose tonemapping runs on the CPU.
func (m *Manager) cost(p Plan) float64 {
	if p.Target.Tonemap && !p.Target.VideoCopy && !m.caps.HWTonemap && p.Cost < costHDRCPU {
		return costHDRCPU
	}
	return p.Cost
}

// acquire takes budget without waiting (resume of an admitted session). Download leases that no
// longer fit next to the live sessions are preempted.
func (m *Manager) acquire(cost float64) {
	m.mu.Lock()
	m.used += cost
	m.preemptLocked(func(leases float64) bool { return m.used+leases <= m.budget })
	m.mu.Unlock()
}

// release returns budget and wakes waiting Starts.
func (m *Manager) release(cost float64) {
	m.mu.Lock()
	m.used -= cost
	if m.used < 1e-9 {
		m.used = 0
	}
	m.wakeLocked()
	m.mu.Unlock()
}

// idleLoop stops ffmpeg of sessions without requests for the idle timeout and removes the files
// of sessions idle for RemoveAfterIdle.
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
				s.idleCheck(now, m.idle, m.removeAfter)
			}
		}
	}
}
