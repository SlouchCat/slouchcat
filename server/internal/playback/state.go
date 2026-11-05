package playback

import (
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Stop watch (concept 13, 11.13 "Stopp durch die Verwaltung erkennen"): a player that buffered its
// whole title makes no further stream request, so the 401 of a revoked stream token never reaches
// it. It asks getPlaybackSessionState every 4 s while it plays instead. The answer comes from
// memory only, never from the database or the transcoder, so the polling costs a map lookup:
//
//   - Start (and startEnclosure) registers every playback session with the browser session that
//     starts it, just before its row is written (and removes it when the insert fails); that is
//     where the owner check of State comes from.
//   - end, the one path every stop takes (Stop, EndSession, EndAccount, endProfile, endReplaced,
//     recheckWatchLimit, StopStream, EndDevice), marks it stopped with its stopped_by value.
//     StopStream and EndDevice register a playback the registry does not know first (one from
//     before a restart): their rows name its browser session.
//   - endSourceChanged marks a remux whose file changed since its start (concept 14, 6.4) stopped
//     for a changed source, a cause that is never persisted (the row keeps stopped_by NULL):
//     Authorize and rebuild answer model.ErrPlaybackSourceChanged for its revoked row instead of
//     401, so every later stream request (and one that raced the first 409) restarts the
//     playback, and State answers model.ErrNotFound for it ("keep playing", no stop), because the
//     next stream request restarts it. The mark lasts stateRetention like any stop.
//   - The sweep marks the playback sessions that ended elsewhere (expiry) and forgets stopped
//     entries stateRetention after their stop, and running ones after activeRetention (rows the
//     sweep never lists again, deleted with their browser session), so the map holds at most the
//     running playback and the stops of the last minutes.
//
// After a restart the registry is empty: State answers model.ErrNotFound for every older playback
// until the administration stops it, which the client takes as "keep playing" (its next stream
// request answers 401 if it ended). A stop that also ends the browser session or its profile
// (a logout, a deleted account or profile) reaches the client as 401 or 403 profile_required of
// the request itself, which the client takes as a stop.

// stateRetention is how long State still reports a stopped playback session.
const stateRetention = 10 * time.Minute

// activeRetention bounds a running entry the sweep never marked: far longer than a playback that
// still makes token requests (SessionLifetime after the last one it expires and is marked).
const activeRetention = 7 * 24 * time.Hour

type stateEntry struct {
	sessionID int64 // browser session that started the playback
	addedAt   time.Time
	stopped   bool
	stoppedBy string
	stoppedAt time.Time
	// sourceChanged marks a stop for a changed source (endSourceChanged); itemID is the title of
	// the playback then, so Authorize can check the token of its revoked row.
	sourceChanged bool
	itemID        string
}

// stateRegistry holds the playback sessions of this server process. It has no database handle:
// State cannot query.
type stateRegistry struct {
	mu      sync.Mutex
	entries map[string]*stateEntry
}

// add registers a playback session of a browser session as running; a known id stays as it is.
func (r *stateRegistry) add(id string, sessionID int64, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]*stateEntry{}
	}
	if _, ok := r.entries[id]; !ok {
		r.entries[id] = &stateEntry{sessionID: sessionID, addedAt: now}
	}
}

// remove forgets a playback session whose row was never written.
func (r *stateRegistry) remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}

// stop marks a known playback session stopped; the first stop keeps its reason. Unknown ids (a
// playback from before the restart) are ignored.
func (r *stateRegistry) stop(id, stoppedBy string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[id]; ok && !e.stopped {
		e.stopped, e.stoppedBy, e.stoppedAt = true, stoppedBy, now
	}
}

// stopSourceChanged marks a playback session of the browser session sessionID stopped for a changed
// source of the title itemID (registering it first, after a restart). An earlier stop keeps its
// reason.
func (r *stateRegistry) stopSourceChanged(id string, sessionID int64, itemID string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]*stateEntry{}
	}
	e, ok := r.entries[id]
	if !ok {
		e = &stateEntry{sessionID: sessionID, addedAt: now}
		r.entries[id] = e
	}
	if !e.stopped {
		e.stopped, e.stoppedAt, e.sourceChanged, e.itemID = true, now, true, itemID
	}
}

// sourceChanged returns the title of a playback session stopped for a changed source; ok is false
// for any other playback session.
func (r *stateRegistry) sourceChanged(id string) (itemID string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, found := r.entries[id]
	if !found || !e.sourceChanged {
		return "", false
	}
	return e.itemID, true
}

// prune forgets the playback sessions stopped more than stateRetention before now and the running
// ones registered more than activeRetention before now.
func (r *stateRegistry) prune(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.entries {
		if e.stopped && now.Sub(e.stoppedAt) > stateRetention || !e.stopped && now.Sub(e.addedAt) > activeRetention {
			delete(r.entries, id)
		}
	}
}

// get returns the state of a playback session of the browser session sessionID.
func (r *stateRegistry) get(id string, sessionID int64) (model.PlaybackState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok || e.sessionID != sessionID || e.sourceChanged {
		return model.PlaybackState{}, false
	}
	return model.PlaybackState{Stopped: e.stopped, StoppedBy: e.stoppedBy}, true
}

// State reports whether a playback session of the browser session still runs, from memory only.
// A playback session of another browser session, an unknown one and one this process no longer
// knows (a restart, stateRetention after its stop) return model.ErrNotFound, and so does one
// stopped for a changed source: that is no stop for the player, its next stream request answers
// model.ErrPlaybackSourceChanged and it starts again.
func (s *Service) State(session model.Session, id string) (model.PlaybackState, error) {
	st, ok := s.states.get(id, session.ID)
	if !ok {
		return model.PlaybackState{}, model.ErrNotFound
	}
	return st, nil
}
