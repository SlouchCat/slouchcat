package radio

import (
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// relayState is the relay part of the service (lane A): one relay per station with its ffmpeg
// packager, the buffer window and the live playlist, the listen sessions with their token, the
// byte budget and the listen time of the K-5 limit.
type relayState struct {
	// control is the dial control of every outgoing connection: controlFunc, which a test of
	// this package may replace (withDialControl) before the first connection.
	control dialControl
	netOnce sync.Once
	net     *netClient

	knobs     relayKnobs
	tmpfs     func(dir string) bool // bufferTmpfs; tests replace it
	startOnce sync.Once             // ends the listen sessions of the last process
	modeOnce  sync.Once
	mode      string // model.RadioBuffer*

	mu       sync.Mutex
	closed   bool                      // Run ended: no relay starts any more (shutdown)
	relays   map[int64]*relay          // running relays by station
	sessions map[string]*listenSession // open listen sessions by id
	credits  map[creditKey]int64       // listen time in ms per profile and quarter, not yet written
	pruned   time.Time                 // last DeleteOldRadioListenTime
}

func newRelayState() *relayState {
	return &relayState{control: controlFunc, knobs: defaultRelayKnobs(), tmpfs: bufferTmpfs,
		relays: map[int64]*relay{}, sessions: map[string]*listenSession{}, credits: map[creditKey]int64{}}
}

// netClient returns the guarded network client of the radio part: streams, HLS, the catalog and
// the logos all go through it.
func (s *Service) netClient() *netClient {
	s.rel.netOnce.Do(func() { s.rel.net = newNetClient(s.rel.control, s.o.userAgent, s.log) })
	return s.rel.net
}

// bufferMode reports where the relays keep their window: model.RadioBufferTmpfs when the
// configured directory is a tmpfs, else model.RadioBufferMemory (live only, F-4). Decided once.
func (s *Service) bufferMode() string {
	s.rel.modeOnce.Do(func() {
		s.rel.mode = model.RadioBufferMemory
		if s.o.dir != "" && s.rel.tmpfs(s.o.dir) {
			s.rel.mode = model.RadioBufferTmpfs
		}
	})
	return s.rel.mode
}

// maxRelaysByBudget is how many relays of the largest transcoded bitrate (192 kbit/s) fit the
// budget with the buffer length of the settings (RadioConfig.MaxRelaysByBudget).
func maxRelaysByBudget(budgetMB, bufferMinutes int) int {
	per := reservationBytes(192, time.Duration(bufferMinutes)*time.Minute)
	if per <= 0 {
		return 0
	}
	return int(int64(budgetMB) << 20 / per)
}
