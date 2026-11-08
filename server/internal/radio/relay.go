package radio

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Relays (concept 16 sections 7.1 to 7.4, build plan S6): one per station, however many listen.
// A relay reads the best stream of its station, frames MP3 and ADTS itself (framer.go), feeds the
// clean audio to an ffmpeg packager and cuts its fragments into the segments of the window. A
// change of codec, stream or (copy mode) any source discontinuity starts a new generation: a new
// packager, a new init, EXT-X-DISCONTINUITY and a new program date-time.

// Relay states (contract RadioRelay.state).
const (
	relayConnecting   = "connecting"
	relayLive         = "live"
	relayReconnecting = "reconnecting"
	relayEnding       = "ending"
)

// maxRelayTitles bounds the title timeline of a relay: a source that changes its title every
// metadata block cannot grow it without end (the window keeps the older ones anyway).
const maxRelayTitles = 200

// relayKnobs are the timings of the relays; tests shorten them.
type relayKnobs struct {
	backoff      []time.Duration // waits before the reconnect attempts; after the last one the relay ends
	readyTimeout time.Duration   // how long Listen waits for the first two segments
	startPhase   time.Duration   // segments of one fragment (2 s) after the start, then of two (4 s)
	fragment     time.Duration
	healthyAfter time.Duration // a relay that ran this long resets the station's failures
	// paceAhead is how far the packaged audio of a connection may run ahead of the time since
	// connect: the start burst passes at once, a faster source is read at real time afterwards.
	paceAhead time.Duration
}

func defaultRelayKnobs() relayKnobs {
	return relayKnobs{
		backoff:      []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second},
		readyTimeout: 15 * time.Second,
		startPhase:   30 * time.Second,
		fragment:     2 * time.Second,
		healthyAfter: time.Minute,
		paceAhead:    10 * time.Second,
	}
}

// relay is the running relay of one station.
type relay struct {
	s           *Service
	stationID   int64
	name        string
	showsTitles bool
	streams     []appq.RadioStream // ranked
	window      time.Duration
	reservation int64
	started     time.Time // wall clock, for the start phase
	startedAt   time.Time // service clock, for the administration
	cancel      context.CancelFunc
	ready       chan struct{} // closed at the second segment
	done        chan struct{} // closed when run returned

	mu        sync.Mutex
	state     string
	err       error // why the relay ended (nil: stopped)
	buf       *buffer
	gen       int // current generation
	in        transcode.RadioInput
	out       transcode.RadioOutput
	titles    []titleEntry
	lastText  string
	fed       time.Duration // audio fed to generation gen (MP3 and ADTS only: the framer counts it)
	pdtGen    int           // generation whose first segment is published (-1 before the first)
	pdtStart  time.Time     // program date-time of that segment
	produced  time.Duration // audio of every fragment so far (pace)
	pend      []byte        // fragments of the segment being built
	pendDur   time.Duration
	pendN     int
	pendGen   int
	lastGen   int // generation of the last published segment (-1 before the first)
	readyOnce sync.Once
	healthy   bool
	idleSince time.Time     // since when no listen session is open (guarded by relayState.mu)
	joins     int           // Listen calls between joinRelay and their session (guarded by relayState.mu)
	changed   chan struct{} // closed and replaced at every new segment (tests wait on it)
}

// titleEntry is one entry of the title timeline. A title of a framed source (MP3, ADTS) is
// stamped by audio position (concept 16 section 7.6): offset is the audio fed to generation gen
// before it; it stays pending until the first segment of gen is published and its program
// date-time is known, then At = that date-time + offset. Other sources keep the stage-1 stamp
// (the end of the last published segment, plan D-12).
type titleEntry struct {
	model.RadioNowEntry
	streamURL string // StreamUrl of the ICY block (cover.go)
	coverKey  string // key of its cover lookup (coverJobFor), "" without one
	gen       int
	offset    time.Duration
	pending   bool
}

// generation is one packager run.
type generation struct {
	n    int
	pk   *transcode.RadioPackager
	in   transcode.RadioInput
	out  transcode.RadioOutput
	done chan struct{} // the fragment consumer ended
}

// newRelay builds a relay; run starts it. Its buffer directory is its own (<dir>/<station>-<ulid>),
// so a relay that still ends never removes the files of the next relay of the station.
func (s *Service) newRelay(st appq.RadioStation, streams []appq.RadioStream, window time.Duration, reservation int64, dir string) (*relay, error) {
	bdir := ""
	if dir != "" {
		bdir = filepath.Join(dir, strconv.FormatInt(st.ID, 10)+"-"+ulid.Make().String())
	}
	buf, err := newBuffer(bdir, window, reservation)
	if err != nil {
		return nil, err
	}
	return &relay{s: s, stationID: st.ID, name: st.Name, showsTitles: st.ShowsTitles == 1, streams: streams,
		window: window, reservation: reservation, started: time.Now(), startedAt: s.now(), idleSince: s.now(),
		ready: make(chan struct{}), done: make(chan struct{}), state: relayConnecting, buf: buf, lastGen: -1,
		pdtGen: -1, changed: make(chan struct{})}, nil
}

// run plays the station until ctx ends or no stream answers any more.
func (r *relay) run(ctx context.Context) {
	defer close(r.done)
	err := r.loop(ctx)
	if ctx.Err() != nil {
		err = nil // stopped (grace, shutdown), not failed
	}
	r.mu.Lock()
	r.state, r.err = relayEnding, err
	r.buf.remove()
	r.mu.Unlock()
	r.s.relayEnded(r, err)
}

// feedState is what the loop knows across connections.
type feedState struct {
	g          *generation
	fr         *framer
	prev       frameHeader // frame parameters of the last frames fed
	hasPrev    bool
	stream     int  // index into r.streams
	newGen     bool // the next audio starts a new generation
	checkFrame bool // compare the first frames of a new connection with prev
}

func (r *relay) loop(ctx context.Context) error {
	knobs := r.s.rel.knobs
	f := &feedState{newGen: true}
	defer func() { r.closeGen(f.g) }()
	connected := false
	for attempt := 0; ; {
		if len(r.streams) == 0 {
			return fmt.Errorf("%w: the station has no stream", model.ErrRadioUnreachable)
		}
		st, err := r.s.netClient().openStream(ctx, r.streamURL(f.stream))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			r.s.log.InfoContext(ctx, "radio stream failed", "station_id", r.stationID,
				"site", netguard.Site(r.streamURL(f.stream)), "err", err)
			if !connected && f.stream+1 < len(r.streams) { // the start tries every stream once
				f.stream++
				f.newGen = true
				continue
			}
			if !connected || attempt >= len(knobs.backoff) {
				return err
			}
			if err := sleepCtx(ctx, knobs.backoff[attempt]); err != nil {
				return err
			}
			attempt++
			if len(r.streams) > 1 { // the next attempt tries another stream of the station
				f.stream = (f.stream + 1) % len(r.streams)
				f.newGen = true
			}
			continue
		}
		connected = true
		r.setState(relayLive)
		r.mu.Lock()
		seq0 := r.buf.nextSeq
		r.mu.Unlock()
		err = r.feed(ctx, st, f)
		_ = st.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		r.mu.Lock()
		published := r.buf.nextSeq > seq0
		r.mu.Unlock()
		// A connection that published a segment starts the backoff again; one that did not (the
		// packager refused the audio, the stream broke at once) counts as a failed attempt, so a
		// station that answers but never plays ends instead of reconnecting forever.
		if published {
			attempt = 0
		}
		if attempt >= len(knobs.backoff) {
			return fmt.Errorf("%w: no segment after %d attempts: %w", model.ErrRadioNotAudio, attempt, err)
		}
		r.s.log.InfoContext(ctx, "radio stream broke, reconnecting", "station_id", r.stationID, "err", err)
		r.setState(relayReconnecting)
		if !framed(f) {
			f.newGen = true // Ogg, FLAC, MPEG-TS and fMP4 are not framed by Go (plan M-1)
		}
		if f.fr != nil {
			f.fr.reset()
			f.checkFrame = true
		}
		if err := sleepCtx(ctx, knobs.backoff[attempt]); err != nil {
			return err
		}
		attempt++
	}
}

// framed reports whether the current audio goes through the framer (MP3 and ADTS).
func framed(f *feedState) bool { return f.fr != nil }

func (r *relay) streamURL(i int) string {
	s := r.streams[i]
	if s.UrlResolved.Valid && s.UrlResolved.String != "" {
		return s.UrlResolved.String
	}
	return s.Url
}

// feed copies the audio of one connection into the packager until the connection or the packager
// breaks; a packager failure is model.ErrRadioNotAudio and starts a new generation on the next
// connection.
func (r *relay) feed(ctx context.Context, st stream, f *feedState) error {
	connected := time.Now()
	r.mu.Lock()
	base := r.produced
	r.mu.Unlock()
	in := st.Info().Input
	if f.g != nil && (in.Format != f.g.in.Format || in.Codec != f.g.in.Codec ||
		transcode.RadioOutputFor(in) != transcode.RadioOutputFor(f.g.in)) {
		f.newGen = true // another codec or bitrate class
	}
	switch in.Format {
	case "mp3", "aac": // the framer's codec names are the formats
		if f.fr == nil || f.fr.codec != in.Format {
			f.fr = newFramer(in.Format)
			f.hasPrev = false
		}
	default:
		f.fr = nil
	}
	names := []string{st.Info().ICYName, r.name}
	for {
		ev, err := st.Next(ctx)
		if err != nil {
			return err
		}
		if ev.TitleSet { // before the audio of the same event: the title applies to it (HLS, Ogg)
			r.title(ctx, ev, names, f)
		}
		if ev.Discontinuity {
			switch {
			case f.fr == nil || f.g != nil && f.g.out.Copy:
				f.newGen = true // copy mode: any source discontinuity starts g+1 (plan M-1)
			default:
				f.fr.reset()
				f.checkFrame = true
			}
		}
		data := ev.Audio
		var dur time.Duration
		if f.fr != nil {
			data = f.fr.push(data)
			dur = f.fr.pushed
			if len(data) == 0 {
				continue
			}
			h, _ := f.fr.params()
			if f.checkFrame && f.hasPrev && !h.sameStream(f.prev) {
				f.newGen = true
			}
			f.checkFrame = false
			f.prev, f.hasPrev = h, true
			in.SampleRate, in.Channels = h.sampleRate, h.channels
		}
		if len(data) == 0 {
			continue
		}
		if f.newGen || f.g == nil {
			if err := r.startGen(ctx, f, in); err != nil {
				return err
			}
		}
		if err := r.pace(ctx, connected, base); err != nil {
			return err
		}
		if _, err := f.g.pk.Write(data); err != nil {
			r.s.log.WarnContext(ctx, "radio packager failed", "station_id", r.stationID, "err", err)
			f.newGen = true
			return fmt.Errorf("%w: packager: %w", model.ErrRadioNotAudio, err)
		}
		if dur > 0 {
			r.mu.Lock()
			r.fed += dur
			r.mu.Unlock()
		}
	}
}

// pace waits while the audio packaged since connect (base: r.produced at connect) runs more than
// knobs.paceAhead ahead of the time since connect, so a source sending faster than real time
// cannot make the relay transcode and store at its pace. The first paceAhead (the start burst
// that fills the first segments, concept 16 section 7.7) passes at once.
func (r *relay) pace(ctx context.Context, connected time.Time, base time.Duration) error {
	r.mu.Lock()
	ahead := r.produced - base - time.Since(connected)
	r.mu.Unlock()
	if over := ahead - r.s.rel.knobs.paceAhead; over > 0 {
		return sleepCtx(ctx, over)
	}
	return nil
}

// startGen ends the running generation (its last fragments are stored first) and starts the next.
func (r *relay) startGen(ctx context.Context, f *feedState, in transcode.RadioInput) error {
	n := 0
	if f.g != nil {
		n = f.g.n + 1
		r.closeGen(f.g)
	}
	out := transcode.RadioOutputFor(in)
	pk, err := r.s.o.runner.StartRadioPackager(ctx, in, out, r.s.rel.knobs.fragment, r.s.log)
	if err != nil {
		return err
	}
	g := &generation{n: n, pk: pk, in: in, out: out, done: make(chan struct{})}
	r.mu.Lock()
	r.gen, r.in, r.out, r.fed = n, in, out, 0
	r.mu.Unlock()
	go r.consume(g)
	f.g, f.newGen = g, false
	return nil
}

// closeGen closes a packager and waits until its fragments are stored.
func (r *relay) closeGen(g *generation) {
	if g == nil {
		return
	}
	if err := g.pk.Close(); err != nil {
		r.s.log.Warn("radio packager ended with an error", "station_id", r.stationID, "err", err)
	}
	<-g.done
}

// consume stores the fragments of one generation.
func (r *relay) consume(g *generation) {
	defer close(g.done)
	for {
		frag, err := g.pk.Next()
		if err != nil {
			if err := r.flush(g.n); err != nil {
				r.s.log.Warn("radio buffer write", "station_id", r.stationID, "err", err)
			}
			return
		}
		if frag.Init {
			r.mu.Lock()
			err = r.buf.addInit(g.n, frag.Data)
			r.mu.Unlock()
		} else {
			err = r.fragment(g.n, frag)
		}
		if err != nil {
			r.s.log.Warn("radio buffer write", "station_id", r.stationID, "err", err)
		}
	}
}

// fragment adds a fragment to the segment being built: one fragment per segment in the start
// phase, two afterwards (EXT-X-TARGETDURATION 4).
func (r *relay) fragment(gen int, frag transcode.RadioFragment) error {
	r.mu.Lock()
	r.pend = append(r.pend, frag.Data...)
	r.pendDur += frag.Duration
	r.produced += frag.Duration
	r.pendN++
	r.pendGen = gen
	full := r.pendN >= 2 || time.Since(r.started) < r.s.rel.knobs.startPhase
	r.mu.Unlock()
	if full {
		return r.flush(gen)
	}
	return nil
}

// flush publishes the segment being built.
func (r *relay) flush(gen int) error {
	r.mu.Lock()
	if r.pendN == 0 || r.pendGen != gen {
		r.mu.Unlock()
		return nil
	}
	data, dur := r.pend, r.pendDur
	r.pend, r.pendDur, r.pendN = nil, 0, 0
	seg := &segment{gen: gen, dur: dur}
	now := r.s.now()
	switch n := len(r.buf.segs); {
	case n > 0 && r.lastGen == gen:
		last := r.buf.segs[n-1]
		seg.pdt = last.pdt.Add(last.dur)
	default:
		seg.pdt = now.Add(-dur)
		if n > 0 {
			last := r.buf.segs[n-1]
			seg.pdt = maxTime(seg.pdt, last.pdt.Add(last.dur))
		}
		seg.disc = r.lastGen >= 0
	}
	err := r.buf.add(seg, data)
	if err == nil {
		if r.lastGen != gen { // the first segment of gen: its titles get their time
			r.pdtGen, r.pdtStart = gen, seg.pdt
			r.resolveTitlesLocked()
		}
		r.lastGen = gen
		r.trimTitlesLocked()
		close(r.changed)
		r.changed = make(chan struct{})
	}
	segs := len(r.buf.segs)
	healthy := !r.healthy && time.Since(r.started) >= r.s.rel.knobs.healthyAfter
	r.healthy = r.healthy || healthy
	r.mu.Unlock()
	if segs >= 2 {
		r.readyOnce.Do(func() {
			close(r.ready)
			r.s.relayReady(r)
		})
	}
	if healthy {
		r.s.relayHealthy(r)
	}
	return err
}

// title adds an announced title to the timeline. Texts that announce no title are left out. A
// framed source stamps it with the audio position (titleEntry): the audio fed to the running
// generation so far, or the start of the next one when a new generation is due (codec or stream
// change); a new generation decided only by the audio after it (a discontinuity) leaves the
// title at the end of the old one, at most a segment off. Other sources keep the stage-1 stamp.
func (r *relay) title(ctx context.Context, ev streamEvent, names []string, f *feedState) {
	e, ok := cleanTitle(ev.Title, names...)
	if !ok {
		return
	}
	r.mu.Lock()
	if e.Text == r.lastText {
		r.mu.Unlock()
		return
	}
	r.lastText = e.Text
	te := titleEntry{RadioNowEntry: e, streamURL: ev.StreamURL}
	switch {
	case !framed(f):
		te.At = r.s.now()
		if n := len(r.buf.segs); n > 0 {
			last := r.buf.segs[n-1]
			te.At = last.pdt.Add(last.dur)
		}
		if n := len(r.titles); n > 0 && !r.titles[n-1].pending {
			te.At = maxTime(te.At, r.titles[n-1].At)
		}
	case f.g == nil:
		te.pending = true // the first generation, from its start
	case f.newGen:
		te.gen, te.pending = f.g.n+1, true
	default:
		te.gen, te.offset, te.pending = f.g.n, r.fed, true
	}
	job, lookup := coverJobFor(te)
	te.coverKey = job.key
	r.titles = append(r.titles, te)
	if n := len(r.titles); n > maxRelayTitles {
		r.titles = slices.Delete(r.titles, 0, n-maxRelayTitles)
	}
	r.resolveTitlesLocked()
	r.trimTitlesLocked()
	first := !r.showsTitles
	r.showsTitles = true
	r.mu.Unlock()
	if lookup {
		r.s.queueCover(job)
	}
	if first {
		r.s.stationShowsTitles(ctx, r.stationID)
	}
}

// resolveTitlesLocked gives the pending titles of every generation up to pdtGen their time: the
// start of pdtGen plus their offset, or its start for a title of an earlier generation that
// published nothing. The timeline stays in order.
func (r *relay) resolveTitlesLocked() {
	if r.pdtGen < 0 {
		return
	}
	for i := range r.titles {
		t := &r.titles[i]
		if !t.pending || t.gen > r.pdtGen {
			continue
		}
		t.At = r.pdtStart
		if t.gen == r.pdtGen {
			t.At = t.At.Add(t.offset)
		}
		if i > 0 && !r.titles[i-1].pending {
			t.At = maxTime(t.At, r.titles[i-1].At)
		}
		t.pending = false
	}
}

// trimTitlesLocked drops the titles before the window, keeping the one in force at its start.
func (r *relay) trimTitlesLocked() {
	if len(r.buf.segs) == 0 {
		return
	}
	start := r.buf.segs[0].pdt
	i := 0
	for i+1 < len(r.titles) && !r.titles[i+1].pending && !r.titles[i+1].At.After(start) {
		i++
	}
	r.titles = r.titles[i:]
}

// timelineLocked returns the resolved entries of the timeline (never a pending one).
func (r *relay) timelineLocked() []model.RadioNowEntry {
	out := make([]model.RadioNowEntry, 0, len(r.titles))
	for _, t := range r.titles {
		if !t.pending {
			out = append(out, t.RadioNowEntry)
		}
	}
	return out
}

// liveEntry returns the title at the live edge: the last resolved entry at or before the end of
// the last published segment (the start page row radio, plan B3). ok is false without one.
func (r *relay) liveEntry() (model.RadioNowEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.buf.segs)
	if n == 0 {
		return model.RadioNowEntry{}, false
	}
	edge := r.buf.segs[n-1].pdt.Add(r.buf.segs[n-1].dur)
	for i := len(r.titles) - 1; i >= 0; i-- {
		if t := r.titles[i]; !t.pending && !t.At.After(edge) {
			return t.RadioNowEntry, true
		}
	}
	return model.RadioNowEntry{}, false
}

func (r *relay) setState(state string) {
	r.mu.Lock()
	r.state = state
	r.mu.Unlock()
}

// segmentData returns segment seq of the window.
func (r *relay) segmentData(seq int64) ([]byte, time.Duration, error) {
	r.mu.Lock()
	seg, ok := r.buf.find(seq)
	var path string
	if ok {
		path = r.buf.path(seg)
	}
	r.mu.Unlock()
	if !ok {
		return nil, 0, model.ErrNotFound
	}
	data, err := readSegment(seg, path)
	return data, seg.dur, err
}

// oldestSeq returns the number of the first segment of the window (the next one while empty).
func (r *relay) oldestSeq() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buf.segs) == 0 {
		return r.buf.nextSeq
	}
	return r.buf.segs[0].seq
}

// initData returns the init part of generation gen.
func (r *relay) initData(gen int) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, ok := r.buf.inits[gen]
	if !ok {
		return nil, model.ErrNotFound
	}
	return data, nil
}

// stop ends the relay and waits for it.
func (r *relay) stop() {
	r.cancel()
	<-r.done
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
