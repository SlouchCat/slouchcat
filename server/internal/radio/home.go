package radio

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The start page row radio («Meine Sender», concept 16 section 10.2, F-17, plan D-7): the
// favourites of the profile it may reach, in its order, each with the title at the live edge
// while a relay of the station runs. Nothing is kept for a station nobody listens to.

// HomeStations returns the tiles of the row radio for profile (discover.HomeRadio). Like
// Favorites, the stations handed out queue their logos.
func (s *Service) HomeStations(ctx context.Context, profile model.Profile) ([]model.RadioHomeStation, error) {
	list, _, err := s.reachableFavorites(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	out := make([]model.RadioHomeStation, 0, len(list))
	ids := make([]int64, 0, len(list))
	for _, st := range list {
		tile := model.RadioHomeStation{Station: st}
		if e, ok := s.liveTitle(st.ID); ok {
			tile.Now = &e
		}
		out = append(out, tile)
		ids = append(ids, st.ID)
	}
	if len(ids) > 0 {
		s.onListed(ctx, ids)
	}
	return out, nil
}

// liveTitle returns the title at the live edge of the station's running relay (relay.liveEntry,
// never a pending one); false while no relay of the station is ready, it ends or has no title.
func (s *Service) liveTitle(stationID int64) (model.RadioNowEntry, bool) {
	s.rel.mu.Lock()
	r := s.rel.relays[stationID]
	s.rel.mu.Unlock()
	if r == nil {
		return model.RadioNowEntry{}, false
	}
	select {
	case <-r.ready:
	default:
		return model.RadioNowEntry{}, false
	}
	r.mu.Lock()
	ending := r.state == relayEnding
	r.mu.Unlock()
	if ending {
		return model.RadioNowEntry{}, false
	}
	e, ok := r.liveEntry()
	return e, ok && e.Text != ""
}
