package radio

import (
	"context"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Now returns the title timeline of the station's running relay: the entries after since and the
// one in force at since (stage 1 stamps each title with the program date-time of the segment
// being filled, hard part 3.5). Live is false while no relay of the station plays.
func (s *Service) Now(ctx context.Context, _ model.Session, stationID int64, since time.Time) (model.RadioNow, error) {
	if _, _, err := s.listenStation(ctx, stationID); err != nil {
		return model.RadioNow{}, err
	}
	s.rel.mu.Lock()
	r := s.rel.relays[stationID]
	s.rel.mu.Unlock()
	out := model.RadioNow{Entries: []model.RadioNowEntry{}}
	if r == nil {
		return out, nil
	}
	select {
	case <-r.ready:
	default:
		return out, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out.Live = r.state != relayEnding
	start := 0 // the entry in force at since, or the first one after it
	for i, e := range r.titles {
		if !e.At.After(since) {
			start = i
		}
	}
	out.Entries = append(out.Entries, r.titles[start:]...)
	return out, nil
}
