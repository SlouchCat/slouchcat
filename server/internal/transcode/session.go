package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	// lookahead: a request for a segment at most this far ahead of the running encoder waits for
	// it instead of restarting ffmpeg (plan 3.5 seek model).
	lookahead = 3
	// pollInterval is how often the segment list of a running ffmpeg is read.
	pollInterval = 50 * time.Millisecond
	// killGrace is the time between SIGTERM and SIGKILL.
	killGrace = 2 * time.Second
	// maxStartsPerRequest bounds the ffmpeg starts one request may cause (one retry).
	maxStartsPerRequest = 2
	// maxFailures is how often ffmpeg may fail from the same start segment before requests for it
	// get the error without another start (players retry failed segments several times).
	maxFailures = 3
	stderrTail  = 4 << 10
	initName    = "init.mp4"
	logName     = "ffmpeg.log"
)

// Errors of a session.
var (
	ErrClosed    = errors.New("transcode session stopped")
	ErrNoSegment = errors.New("no such segment")
	// ErrRestarted is returned to a waiting request whose ffmpeg run was replaced by a newer
	// request for a distant segment (the newest request wins). It is retryable: the API answers
	// 503 with Retry-After 1.
	ErrRestarted = errors.New("transcode restarted for a newer request")
)

// Session is one HLS output: ffmpeg runs that write the segments of its plan into the session
// directory. All methods are safe for concurrent use.
type Session struct {
	m    *Manager
	id   string
	dir  string
	src  Source
	plan Plan
	log  *slog.Logger

	pub sync.Mutex // serializes publishing; the disk I/O runs outside mu

	mu        sync.Mutex
	changed   chan struct{} // closed and replaced on every state change
	done      []bool        // segment published in dir
	missing   map[int]bool  // a finished run did not produce these segments
	initReady bool
	run       *run              // the current ffmpeg, nil when none runs
	live      map[*run]struct{} // all processes not yet reaped (killed ones exit asynchronously)
	runs      int               // run counter for directory names
	gen       uint64            // ffmpeg starts; a waiter whose run was replaced backs off
	front     int               // highest segment requested since the last start (MaxAhead)
	failures  map[int]failure   // failed runs per start segment
	err       error             // why the last run that ended on its own failed
	lastUse   time.Time
	cpu       time.Duration // CPU time of finished runs
	active    bool          // holds plan.Cost of the manager's budget
	cold      bool          // idle for RemoveAfterIdle: directory removed, restarts on demand
	closed    bool
}

// failure counts the failed ffmpeg runs from one start segment.
type failure struct {
	n   int
	err error
}

// run is one ffmpeg process writing segments start, start+1, ... into its own directory.
type run struct {
	start   int
	next    int       // next segment index this run will finish
	seen    int       // entries of the segment list already processed
	stalled time.Time // when the next listed segment was first seen incomplete
	dir     string
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	killed  bool
	exited  chan struct{}
	stderr  *tailWriter
}

// Plan returns the segment plan of the session.
func (s *Session) Plan() Plan { return s.plan }

// Mode returns remux or transcode.
func (s *Session) Mode() Mode { return s.plan.Target.Mode }

// Backend returns the encoder backend of a transcode; it is empty for remux (video copied).
func (s *Session) Backend() Backend {
	if s.plan.Target.VideoCopy {
		return ""
	}
	return s.m.caps.Backend
}

// Touch marks activity: the idle timer kills ffmpeg after IdleTimeout without requests.
func (s *Session) Touch() {
	s.mu.Lock()
	s.lastUse = time.Now()
	s.mu.Unlock()
}

// CPUTime returns the CPU time (user + system) of the ffmpeg runs that have ended; after Stop it
// is the total of the session.
func (s *Session) CPUTime() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cpu
}

// MediaPlaylist renders the VOD media playlist with the init and segment URLs of the caller.
func (s *Session) MediaPlaylist(initURL string, segmentURL func(i int) string) []byte {
	return mediaPlaylist(s.plan, initURL, segmentURL)
}

// OpenInit returns the init segment (ftyp+moov) once the first file of any run is written. When
// no ffmpeg runs (idle, throttled or cold), it starts one at the last requested position.
func (s *Session) OpenInit(ctx context.Context) (io.ReadSeekCloser, error) {
	s.Touch()
	defer s.Touch()
	starts := 0
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrClosed
		}
		if s.initReady {
			s.mu.Unlock()
			return s.open(initName)
		}
		if s.run == nil {
			if err := s.restartLocked(s.pendingFrom(s.front), &starts); err != nil {
				s.mu.Unlock()
				return nil, err
			}
		}
		ch := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		}
	}
}

// OpenSegment returns segment i when it is complete. If it is not produced yet and the running
// ffmpeg is at most lookahead segments before it, OpenSegment waits (bounded by ctx); otherwise
// it restarts ffmpeg at i. Produced segments stay on disk and remain valid. A waiting request
// whose run is replaced by a newer request's restart returns ErrRestarted.
func (s *Session) OpenSegment(ctx context.Context, i int) (io.ReadSeekCloser, error) {
	if i < 0 || i >= len(s.plan.Segments) {
		return nil, fmt.Errorf("%w: %d", ErrNoSegment, i)
	}
	s.Touch()
	defer s.Touch()
	starts := 0
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrClosed
		}
		s.front = max(s.front, i)
		if s.done[i] {
			s.mu.Unlock()
			return s.open(fmt.Sprintf(segmentPattern, i))
		}
		if s.missing[i] {
			s.mu.Unlock()
			return nil, fmt.Errorf("%w: segment %d was not produced by ffmpeg", ErrNoSegment, i)
		}
		if r := s.run; r == nil || i < r.next || i > r.next+lookahead {
			if r != nil && s.gen != gen {
				s.mu.Unlock()
				return nil, fmt.Errorf("%w: segment %d", ErrRestarted, i)
			}
			if err := s.restartLocked(i, &starts); err != nil {
				s.mu.Unlock()
				return nil, err
			}
			gen = s.gen
		}
		ch := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		}
	}
}

// open opens a published file. A file that vanished belongs to a session that was stopped.
func (s *Session) open(name string) (io.ReadSeekCloser, error) {
	f, err := os.Open(filepath.Join(s.dir, name)) //nolint:gosec // G304: session work dir and a fixed file name
	if err == nil {
		return f, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return nil, ErrClosed
		}
		return nil, fmt.Errorf("%w: %s: %w", ErrNoSegment, name, err)
	}
	return nil, err
}

// restartLocked kills the current run and starts one at segment i, unless this request already
// started one that failed (then its error is returned) or ffmpeg failed maxFailures times from i.
func (s *Session) restartLocked(i int, starts *int) error {
	if *starts > 0 && s.err != nil {
		return s.err
	}
	if *starts >= maxStartsPerRequest {
		return fmt.Errorf("transcode session %s: ffmpeg stopped twice before segment %d", s.id, i)
	}
	if f := s.failures[i]; f.n >= maxFailures {
		return fmt.Errorf("transcode session %s: ffmpeg failed %d times from segment %d: %w", s.id, f.n, i, f.err)
	}
	*starts++
	if s.run != nil {
		s.killLocked(s.run, true)
	}
	return s.startLocked(i)
}

// pendingFrom returns the first segment at or after i that is neither published nor known to be
// missing, else the first pending one from the start.
func (s *Session) pendingFrom(i int) int {
	for k := max(i, 0); k < len(s.done); k++ {
		if !s.done[k] && !s.missing[k] {
			return k
		}
	}
	for k, d := range s.done {
		if !d && !s.missing[k] {
			return k
		}
	}
	return 0
}

func (s *Session) segmentPath(i int) string {
	return filepath.Join(s.dir, fmt.Sprintf(segmentPattern, i))
}

// broadcastLocked wakes every waiter.
func (s *Session) broadcastLocked() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// startLocked starts ffmpeg at segment i in a fresh run directory. It refuses a stopped session,
// so a Stop racing with Start or a request never leaves an orphaned ffmpeg.
func (s *Session) startLocked(i int) error {
	if s.closed {
		return ErrClosed
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return err
	}
	s.cold = false
	s.runs++
	dir := filepath.Join(s.dir, fmt.Sprintf("run%04d", s.runs))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	args, err := ffmpegArgs(s.m.caps, s.src, s.plan, i, dir)
	if err != nil {
		return err
	}
	tail := &tailWriter{}
	var stderr io.Writer = tail
	logf, err := os.OpenFile(filepath.Join(s.dir, logName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = fmt.Fprintf(logf, "--- run %d from segment %d\n", s.runs, i)
		stderr = io.MultiWriter(logf, tail)
	}
	ctx, cancel := context.WithCancel(s.m.ctx)
	cmd, err := startFFmpeg(ctx, s.m.runner.FFmpegPath, args, stderr, s.log)
	if err != nil {
		cancel()
		if logf != nil {
			_ = logf.Close()
		}
		_ = os.RemoveAll(dir)
		return err
	}
	r := &run{start: i, next: i, dir: dir, cmd: cmd, cancel: cancel, exited: make(chan struct{}), stderr: tail}
	s.log.Debug("ffmpeg started", "session", s.id, "segment", i, "pid", cmd.Process.Pid)
	s.err = nil
	s.run = r
	s.live[r] = struct{}{}
	s.gen++
	s.front = i
	if !s.active {
		s.m.acquire(s.m.cost(s.plan))
		s.active = true
	}
	s.broadcastLocked()
	go s.watch(r, logf)
	return nil
}

// killLocked stops a run: SIGTERM, SIGKILL after killGrace. Segments it lists from now on are
// ignored, because ffmpeg closes the interrupted segment as if it were complete. keepBudget keeps
// the session's budget (restart or MaxAhead pause); otherwise it is released.
func (s *Session) killLocked(r *run, keepBudget bool) {
	if r.killed {
		return
	}
	r.killed = true
	r.cancel()
	if s.run == r {
		s.run = nil
		if !keepBudget {
			s.releaseLocked()
		}
	}
	s.broadcastLocked()
}

func (s *Session) releaseLocked() {
	if s.active {
		s.active = false
		s.m.release(s.m.cost(s.plan))
	}
}

// watch polls the segment list of a run until the process exits.
func (s *Session) watch(r *run, logf *os.File) {
	err := pollUntilExit(r.cmd, func() { s.collect(r, false) })
	s.collect(r, true)
	s.finish(r, err)
	if logf != nil {
		_ = logf.Close()
	}
}

// collect publishes the segments the run has finished since the last call. ffmpeg lists a
// segment before it writes its data, so a listed segment counts only once checkComplete accepts
// it; until then collect keeps its position and retries on the next tick, and gives up after
// incompleteWait (or at once when the process has exited: final). The disk I/O runs outside mu;
// pub keeps publishers of different runs apart.
func (s *Session) collect(r *run, final bool) {
	data, err := os.ReadFile(filepath.Join(r.dir, segmentList))
	if err != nil {
		return
	}
	idx := parseSegmentList(data)
	s.pub.Lock()
	defer s.pub.Unlock()
	for {
		s.mu.Lock()
		if r.killed || s.closed || r.seen >= len(idx) {
			s.mu.Unlock()
			break
		}
		k := idx[r.seen]
		if k < 0 || k >= len(s.done) {
			r.seen++
			s.mu.Unlock()
			continue
		}
		already, needInit := s.done[k], !s.initReady
		s.mu.Unlock()

		src := filepath.Join(r.dir, fmt.Sprintf(segmentPattern, k))
		var wrote bool
		if already {
			_ = os.Remove(src)
		} else if err = checkComplete(src); err == nil {
			wrote, err = publishSegment(src, s.segmentPath(k), filepath.Join(s.dir, initName), needInit)
		}

		s.mu.Lock()
		var retry bool
		if retry, err = stallCheck(err, final, &r.stalled, s.m.incompleteWait, k); retry {
			s.mu.Unlock()
			return
		}
		if err != nil {
			if !r.killed && !s.closed {
				s.log.Error("publish segment", "session", s.id, "segment", k, "err", err)
				s.err = err
				s.killLocked(r, false)
			}
			s.mu.Unlock()
			return
		}
		if !already {
			s.initReady = s.initReady || wrote
			s.done[k] = true
		}
		r.seen++
		r.next = k + 1
		delete(s.failures, r.start)
		s.broadcastLocked()
		s.mu.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if r.killed || s.closed || s.run != r {
		return
	}
	switch {
	case r.next < len(s.done) && s.done[r.next]:
		// The run reached segments an earlier run already wrote: stop it; a request for the next
		// missing segment starts a new run there.
		s.killLocked(r, false)
	case s.m.maxAhead > 0 && r.next-1-s.front > s.m.maxAhead:
		// Far enough ahead of the player: pause ffmpeg, keep the budget; the next request for a
		// missing segment restarts it there.
		s.log.Debug("ffmpeg ahead, pausing", "session", s.id, "next", r.next, "front", s.front)
		s.killLocked(r, true)
	}
}

// finish records the end of a run.
func (s *Session) finish(r *run, waitErr error) {
	s.mu.Lock()
	defer func() {
		s.broadcastLocked()
		s.mu.Unlock()
		close(r.exited)
	}()
	delete(s.live, r)
	if ps := r.cmd.ProcessState; ps != nil {
		s.cpu += ps.UserTime() + ps.SystemTime()
	}
	_ = os.RemoveAll(r.dir)
	if s.run != r {
		return // killed: replaced, paused, idle or stopped
	}
	s.run = nil
	s.releaseLocked()
	if waitErr != nil {
		s.err = fmt.Errorf("ffmpeg from segment %d: %w: %s", r.start, waitErr, r.stderr.String())
		f := s.failures[r.start]
		s.failures[r.start] = failure{n: f.n + 1, err: s.err}
		s.log.Warn("ffmpeg failed", "session", s.id, "segment", r.start, "err", s.err)
		return
	}
	for k := r.next; k < len(s.done); k++ {
		if !s.done[k] {
			s.missing[k] = true
		}
	}
}

// close kills all runs, waits for them and removes the session directory.
func (s *Session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	var runs []*run
	for r := range s.live {
		s.killLocked(r, false)
		runs = append(runs, r)
	}
	s.releaseLocked()
	s.broadcastLocked()
	s.mu.Unlock()
	for _, r := range runs {
		<-r.exited
	}
	if err := os.RemoveAll(s.dir); err != nil {
		s.log.Warn("remove transcode dir", "session", s.id, "err", err)
	}
}

// idleCheck stops ffmpeg and releases the budget when nobody asked for a segment within idle;
// the files stay. After removeAfter without requests (0 = never) the directory is removed too
// and the session turns cold: the next request restarts ffmpeg at the requested segment.
func (s *Session) idleCheck(now time.Time, idle, removeAfter time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	since := now.Sub(s.lastUse)
	if since > idle {
		if s.run != nil {
			s.log.Debug("ffmpeg idle, stopping", "session", s.id)
			s.killLocked(s.run, false)
		}
		s.releaseLocked() // also a session paused by MaxAhead
	}
	if removeAfter <= 0 || since <= removeAfter || s.cold || len(s.live) > 0 {
		return
	}
	s.log.Debug("transcode session idle, removing files", "session", s.id)
	if err := os.RemoveAll(s.dir); err != nil {
		s.log.Warn("remove transcode dir", "session", s.id, "err", err)
		return
	}
	clear(s.done)
	clear(s.missing)
	clear(s.failures)
	s.initReady = false
	s.err = nil
	s.cold = true
	s.broadcastLocked()
}

// state reports whether ffmpeg runs and whether the session is cold.
func (s *Session) state() (running, cold bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run != nil, s.cold
}

// stallCheck applies the rule for a listed segment whose data is not completely on disk yet
// (err wraps errIncomplete): while the process runs (not final) the caller retries it on the next
// poll for up to wait from the first incomplete check; after that, or once the process has exited,
// it is an error. since holds the time of the first incomplete check (zero when none is pending).
// Any other err (nil included) is returned unchanged.
func stallCheck(err error, final bool, since *time.Time, wait time.Duration, k int) (retry bool, _ error) {
	if errors.Is(err, errIncomplete) && !final {
		now := time.Now()
		if since.IsZero() {
			*since = now
		}
		if now.Sub(*since) <= wait {
			return true, nil
		}
		err = fmt.Errorf("segment %d still incomplete after %v: %w", k, wait, err)
	}
	*since = time.Time{}
	return false, err
}

// startFFmpeg starts ffmpeg with args. Canceling ctx sends SIGTERM, SIGKILL follows after
// killGrace; the process runs with a lower priority (deprioritize).
func startFFmpeg(ctx context.Context, ffmpeg string, args []string, stderr io.Writer, log *slog.Logger) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, ffmpeg, args...) //nolint:gosec // G204: configured binary, argument list from ffmpegArgs
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = killGrace
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	deprioritize(cmd.Process.Pid, log)
	return cmd, nil
}

// pollUntilExit waits for cmd to exit, calling poll every pollInterval meanwhile, and returns the
// result of cmd.Wait.
func pollUntilExit(cmd *exec.Cmd, poll func()) error {
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case err := <-waitErr:
			return err
		case <-t.C:
			poll()
		}
	}
}

// deprioritize lowers the CPU priority (nice 10) and makes ffmpeg the preferred victim of the
// OOM killer (oom_score_adj 500), so the server itself stays responsive (plan 3.5). Best effort.
func deprioritize(pid int, log *slog.Logger) {
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, 10); err != nil {
		log.Debug("set ffmpeg priority", "pid", pid, "err", err)
	}
	path := filepath.Join("/proc", strconv.Itoa(pid), "oom_score_adj")
	if err := os.WriteFile(path, []byte("500"), 0o600); err != nil {
		log.Debug("set ffmpeg oom_score_adj", "pid", pid, "err", err)
	}
}

// tailWriter keeps the last stderrTail bytes written to it.
type tailWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	if len(w.buf) > stderrTail {
		w.buf = append(w.buf[:0], w.buf[len(w.buf)-stderrTail:]...)
	}
	return len(p), nil
}

func (w *tailWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return tail(w.buf, stderrTail)
}
