package radio

// relayState is the relay part of the service (lane A): one relay per station with its ffmpeg
// packager, the buffer window and the live playlist, the listen sessions with their token, the
// byte budget and the listen time of the K-5 limit.
type relayState struct{}

func newRelayState() *relayState { return &relayState{} }
