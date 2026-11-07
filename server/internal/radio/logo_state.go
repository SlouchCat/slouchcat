package radio

// logoState is the logo part of the service (lane A): the logo chain of concept 16 section 6
// (catalog favicon, website, own upload) run through the job queue, scaled to squares.
type logoState struct{}

func newLogoState() *logoState { return &logoState{} }
