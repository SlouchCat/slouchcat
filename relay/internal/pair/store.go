// Package pair is the meeting point service ("Vermittlung", concept 27 sections 7.1 and 7.2): a TV
// opens a meeting point, the phone drops an encrypted packet there, the TV picks it up with its
// pickup secret. Everything lives in memory for at most five minutes (D-1). The service never sees
// anything but ciphertext and holds no secret of its own (D-6).
package pair

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Errors of the store; the handler maps them to the status codes of the protocol.
var (
	ErrNotFound   = errors.New("unknown or expired meeting point") // 404
	ErrOccupied   = errors.New("a packet is already waiting")      // 409
	ErrConsumed   = errors.New("the packet was already picked up") // 410
	ErrForbidden  = errors.New("wrong pickup secret")              // 403
	ErrFull       = errors.New("too many open meeting points")     // 503
	ErrCallerFull = errors.New("too many open meeting points for this caller")
)

type state int

const (
	stateOpen     state = iota // waiting for the phone
	stateFilled                // a packet waits for the TV
	stateConsumed              // picked up; only remembered until it expires
)

type entry struct {
	pickupHash [sha256.Size]byte
	packet     []byte
	state      state
	expires    time.Time
	caller     string // the limit key of the TV that opened it, in memory only (D-5)
	network    string // the /48 or /24 around it (security review M-1)
}

// Store holds the meeting points.
type Store struct {
	ttl          time.Duration
	maxOpen      int
	maxPerCaller int
	maxPerNet    int
	now          func() time.Time

	mu       sync.Mutex
	entries  map[string]*entry
	byCaller map[string]int
	byNet    map[string]int
}

// Limits of a store: in total, per caller key (/64 or address) and per larger network (/48 or
// /24). Without the last, one cheap /48 fills every place with 1000 of its /64 (M-1).
type Limits struct {
	MaxOpen, MaxPerCaller, MaxPerNet int
}

// NewStore returns an empty store.
func NewStore(ttl time.Duration, l Limits) *Store {
	return &Store{
		ttl: ttl, maxOpen: l.MaxOpen, maxPerCaller: l.MaxPerCaller, maxPerNet: l.MaxPerNet, now: time.Now,
		entries: map[string]*entry{}, byCaller: map[string]int{}, byNet: map[string]int{},
	}
}

// SetClock replaces the clock (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Create opens a meeting point for caller (in network) with the SHA-256 of the pickup secret. It
// returns the id (16 random bytes, base64url without padding) and the time it expires.
func (s *Store) Create(caller, network string, pickupHash [sha256.Size]byte) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.expireLocked(now)
	if len(s.entries) >= s.maxOpen {
		return "", time.Time{}, ErrFull
	}
	if s.byCaller[caller] >= s.maxPerCaller || s.byNet[network] >= s.maxPerNet {
		return "", time.Time{}, ErrCallerFull
	}
	var raw [16]byte
	_, _ = rand.Read(raw[:]) // never fails (crypto/rand)
	id := base64.RawURLEncoding.EncodeToString(raw[:])
	e := &entry{pickupHash: pickupHash, expires: now.Add(s.ttl), caller: caller, network: network}
	s.entries[id] = e
	s.byCaller[caller]++
	s.byNet[network]++
	return id, e.expires, nil
}

// Put drops packet at id: ErrNotFound, ErrOccupied or ErrConsumed when that is not possible.
func (s *Store) Put(id string, packet []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveLocked(id)
	if e == nil {
		return ErrNotFound
	}
	switch e.state {
	case stateFilled:
		return ErrOccupied
	case stateConsumed:
		return ErrConsumed
	}
	e.packet = append([]byte(nil), packet...)
	e.state = stateFilled
	return nil
}

// Get picks up the packet at id with the pickup secret. It returns nil and no error while no
// packet waits. A packet is handed out once; afterwards the meeting point is consumed.
func (s *Store) Get(id string, secret []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveLocked(id)
	if e == nil {
		return nil, ErrNotFound
	}
	if !matches(e, secret) {
		return nil, ErrForbidden
	}
	switch e.state {
	case stateOpen:
		return nil, nil
	case stateConsumed:
		return nil, ErrConsumed
	}
	p := e.packet
	e.packet = nil
	e.state = stateConsumed
	return p, nil
}

// Delete cancels the meeting point id with the pickup secret. A consumed one stays until it
// expires, so a phone that drops a packet afterwards still hears "already picked up" (410), not
// "unknown" (code review minor 4): a TV that tidies up after the pick-up must not hide that.
func (s *Store) Delete(id string, secret []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveLocked(id)
	if e == nil {
		return ErrNotFound
	}
	if !matches(e, secret) {
		return ErrForbidden
	}
	if e.state != stateConsumed {
		s.removeLocked(id, e)
	}
	return nil
}

// Len is the number of meeting points held, consumed ones included.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// Expire drops what has run out. The service calls it every few seconds; every other method
// also treats an expired entry as gone.
func (s *Store) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireLocked(s.now())
}

func (s *Store) liveLocked(id string) *entry {
	e, ok := s.entries[id]
	if !ok {
		return nil
	}
	if !s.now().Before(e.expires) {
		s.removeLocked(id, e)
		return nil
	}
	return e
}

func (s *Store) expireLocked(now time.Time) {
	for id, e := range s.entries {
		if !now.Before(e.expires) {
			s.removeLocked(id, e)
		}
	}
}

func (s *Store) removeLocked(id string, e *entry) {
	delete(s.entries, id)
	if s.byCaller[e.caller]--; s.byCaller[e.caller] <= 0 {
		delete(s.byCaller, e.caller)
	}
	if s.byNet[e.network]--; s.byNet[e.network] <= 0 {
		delete(s.byNet, e.network)
	}
}

// matches compares the hash of secret with the stored hash in constant time (D-3).
func matches(e *entry, secret []byte) bool {
	h := sha256.Sum256(secret)
	return subtle.ConstantTimeCompare(h[:], e.pickupHash[:]) == 1
}
