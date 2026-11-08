package radio

import "github.com/slouchcat/slouchcat/server/internal/ratelimit"

// logoState is the logo part of the service (lane A): the logo chain of concept 16 section 6
// (catalog favicon, website, own upload) run through the job queue, scaled to squares.
type logoState struct {
	limiter *ratelimit.Limiter // two requests a second for all logo jobs together
}

func newLogoState() *logoState { return &logoState{limiter: ratelimit.New("radio-logo", 2, 2)} }
