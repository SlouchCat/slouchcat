package radio

import "sync"

// relayState is the relay part of the service (lane A): one relay per station with its ffmpeg
// packager, the buffer window and the live playlist, the listen sessions with their token, the
// byte budget and the listen time of the K-5 limit.
type relayState struct {
	// control is the dial control of every outgoing connection: controlFunc, which a test of
	// this package may replace (withDialControl) before the first connection.
	control dialControl
	netOnce sync.Once
	net     *netClient
}

func newRelayState() *relayState { return &relayState{control: controlFunc} }

// netClient returns the guarded network client of the radio part: streams, HLS, the catalog and
// the logos all go through it.
func (s *Service) netClient() *netClient {
	s.rel.netOnce.Do(func() { s.rel.net = newNetClient(s.rel.control, s.o.userAgent, s.log) })
	return s.rel.net
}
