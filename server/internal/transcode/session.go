package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	stderrTail          = 4 << 10
	initName            = "init.mp4"
	logName             = "ffmpeg.log"
)

// Errors of a session.
var (
	ErrClosed    = errors.New("transcode session stopped")
	ErrNoSegment = errors.New("no such segment")
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

	mu        sync.Mutex
	changed   chan struct{} // closed and replaced on every state change
	done      []bool        // segment published in dir
	missing   map[int]bool  // a finished run did not produce these segments
	initReady bool
	run       *run              // the current ffmpeg, nil when none runs
	live      map[*run]struct{} // all processes not yet reaped (killed ones exit asynchronously)
	runs      int               // run counter for directory names
	err       error             // why the last run that ended on its own failed
	lastUse   time.Time
	cpu       time.Duration // CPU time of finished runs
	active    bool          // holds plan.Cost of the manager's budget
	closed    bool
}

// run is one ffmpeg process writing segments start, start+1, ... into its own directory.
type run struct {
	start  int
	next   int // next segment index this run will finish
	seen   int // entries of the segment list already processed
	dir    string
	cmd    *exec.Cmd
	cancel context.CancelFunc
	killed bool
	exited chan struct{}
	stderr *tailWriter
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

// OpenInit returns the init segment (ftyp+moov) once the first segment is written.
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
			return os.Open(filepath.Join(s.dir, initName))
		}
		if s.run == nil {
			if err := s.restartLocked(s.firstPending(), &starts); err != nil {
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
// it restarts ffmpeg at i. Produced segments stay on disk and remain valid.
func (s *Session) OpenSegment(ctx context.Context, i int) (io.ReadSeekCloser, error) {
	if i < 0 || i >= len(s.plan.Segments) {
		return nil, fmt.Errorf("%w: %d", ErrNoSegment, i)
	}
	s.Touch()
	defer s.Touch()
	starts := 0
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrClosed
		}
		if s.done[i] {
			s.mu.Unlock()
			return os.Open(s.segmentPath(i))
		}
		if s.missing[i] {
			s.mu.Unlock()
			return nil, fmt.Errorf("%w: segment %d was not produced by ffmpeg", ErrNoSegment, i)
		}
		if r := s.run; r == nil || i < r.next || i > r.next+lookahead {
			if err := s.restartLocked(i, &starts); err != nil {
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

// restartLocked kills the current run and starts one at segment i, unless this request already
// started one that failed (then its error is returned).
func (s *Session) restartLocked(i int, starts *int) error {
	if *starts > 0 && s.err != nil {
		return s.err
	}
	if *starts >= maxStartsPerRequest {
		return fmt.Errorf("transcode session %s: ffmpeg stopped twice before segment %d", s.id, i)
	}
	*starts++
	if s.run != nil {
		s.killLocked(s.run)
	}
	return s.startLocked(i)
}

func (s *Session) firstPending() int {
	for i, d := range s.done {
		if !d {
			return i
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

// startLocked starts ffmpeg at segment i in a fresh run directory.
func (s *Session) startLocked(i int) error {
	s.runs++
	dir := filepath.Join(s.dir, fmt.Sprintf("run%04d", s.runs))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	args, err := ffmpegArgs(s.m.caps, s.src, s.plan, i, dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, s.m.runner.FFmpegPath, args...) //nolint:gosec // G204: configured binary, argument list from ffmpegArgs
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = killGrace
	r := &run{start: i, next: i, dir: dir, cmd: cmd, cancel: cancel, exited: make(chan struct{}), stderr: &tailWriter{}}
	logf, err := os.OpenFile(filepath.Join(s.dir, logName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = fmt.Fprintf(logf, "--- run %d from segment %d\n", s.runs, i)
		cmd.Stderr = io.MultiWriter(logf, r.stderr)
	} else {
		cmd.Stderr = r.stderr
	}
	if err := cmd.Start(); err != nil {
		cancel()
		if logf != nil {
			_ = logf.Close()
		}
		_ = os.RemoveAll(dir)
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	deprioritize(cmd.Process.Pid, s.log)
	s.log.Debug("ffmpeg started", "session", s.id, "segment", i, "pid", cmd.Process.Pid)
	s.err = nil
	s.run = r
	s.live[r] = struct{}{}
	if !s.active {
		s.m.acquire(s.plan.Cost)
		s.active = true
	}
	s.broadcastLocked()
	go s.watch(r, logf)
	return nil
}

// killLocked stops a run: SIGTERM, SIGKILL after killGrace. Segments it lists from now on are
// ignored, because ffmpeg closes the interrupted segment as if it were complete.
func (s *Session) killLocked(r *run) {
	if r.killed {
		return
	}
	r.killed = true
	r.cancel()
	if s.run == r {
		s.run = nil
		s.releaseLocked()
	}
	s.broadcastLocked()
}

func (s *Session) releaseLocked() {
	if s.active {
		s.active = false
		s.m.release(s.plan.Cost)
	}
}

// watch polls the segment list of a run until the process exits.
func (s *Session) watch(r *run, logf *os.File) {
	waitErr := make(chan error, 1)
	go func() { waitErr <- r.cmd.Wait() }()
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case err := <-waitErr:
			s.collect(r)
			s.finish(r, err)
			if logf != nil {
				_ = logf.Close()
			}
			return
		case <-t.C:
			s.collect(r)
		}
	}
}

// collect publishes the segments the run has finished since the last call.
func (s *Session) collect(r *run) {
	data, err := os.ReadFile(filepath.Join(r.dir, segmentList))
	if err != nil {
		return
	}
	idx := parseSegmentList(data)
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.killed || s.closed || len(idx) <= r.seen {
		return
	}
	for _, k := range idx[r.seen:] {
		r.seen++
		if k < 0 || k >= len(s.done) {
			continue
		}
		src := filepath.Join(r.dir, fmt.Sprintf(segmentPattern, k))
		if s.done[k] {
			_ = os.Remove(src)
		} else {
			wrote, err := publishSegment(src, s.segmentPath(k), filepath.Join(s.dir, initName), !s.initReady)
			if err != nil {
				s.log.Error("publish segment", "session", s.id, "segment", k, "err", err)
				s.err = err
				s.killLocked(r)
				return
			}
			s.initReady = s.initReady || wrote
			s.done[k] = true
		}
		r.next = k + 1
	}
	s.broadcastLocked()
	// The run reached segments an earlier run already wrote: stop it; a request for the next
	// missing segment starts a new run there.
	if s.run == r && r.next < len(s.done) && s.done[r.next] {
		s.killLocked(r)
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
		return // killed: replaced, idle or stopped
	}
	s.run = nil
	s.releaseLocked()
	if waitErr != nil {
		s.err = fmt.Errorf("ffmpeg from segment %d: %w: %s", r.start, waitErr, r.stderr.String())
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
		s.killLocked(r)
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

// idleKill stops ffmpeg when nobody asked for a segment within timeout; the files stay.
func (s *Session) idleKill(now time.Time, timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != nil && now.Sub(s.lastUse) > timeout {
		s.log.Debug("ffmpeg idle, stopping", "session", s.id)
		s.killLocked(s.run)
	}
}

func (s *Session) running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run != nil
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
