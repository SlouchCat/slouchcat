package radio

import (
	"context"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Now returns the title timeline of the station's running relay: the entries after since and the
// one in force at since, stamped by audio position (relay.go titleEntry); a title whose time is
// not known yet is left out. Live is false while no relay of the station plays.
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
	out.Live = r.state != relayEnding
	entries := r.timelineLocked()
	r.mu.Unlock()
	start := 0 // the entry in force at since, or the first one after it
	for i, e := range entries {
		if !e.At.After(since) {
			start = i
		}
	}
	out.Entries = append(out.Entries, entries[start:]...)
	return out, nil
}
