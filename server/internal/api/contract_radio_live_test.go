package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio"
)

// The webradio of the contract fixture (M-4): the production dialer refuses the loopback and the
// api tests build without the tag radiotest, so no api test can start a real relay. contractRadio
// therefore embeds the real radio service on the vistest database (vistest.AddRadio), which
// answers every catalog, facet, favourite, preference, image and admin operation, so visibility
// and the 404 rules are tested for real. Only the live part is fake: each live method first asks
// the real Station (a hidden or excluded station stays 404), then fakeRadioLive mints a listen id
// and a token with the real radio.SignToken and serves a canned playlist, init and segment. Real
// relays, token binding and listen time are tested in package radio and in e2e (W4).

// testRadioKey is the listen token key of the contract fixture.
var testRadioKey = []byte("contract-test-radio-token-key!!!")

type contractRadio struct {
	*radio.Service
	live *fakeRadioLive
}

// fakeListen is one listen session of the fake.
type fakeListen struct {
	token                string
	stationID            int64
	sessionID, accountID int64
	profile              model.Profile
	ended                bool
	done                 chan struct{}
}

// fakeRadioLive holds the fake listen sessions; skips records the _HLS_skip values it was asked.
type fakeRadioLive struct {
	mu      sync.Mutex
	listens map[string]*fakeListen
	seq     atomic.Int64
	skips   []string
}

func newContractRadio(svc *radio.Service) *contractRadio {
	return &contractRadio{Service: svc, live: &fakeRadioLive{listens: map[string]*fakeListen{}}}
}

// fakeRadioPlaylist is the canned live playlist; long enough for the gzip of the handler.
var fakeRadioPlaylist = func() []byte {
	b := []byte("#EXTM3U\n#EXT-X-VERSION:9\n#EXT-X-TARGETDURATION:4\n#EXT-X-SERVER-CONTROL:CAN-SKIP-UNTIL=36.0\n" +
		"#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-MAP:URI=\"init/0\"\n")
	for i := range 40 {
		b = fmt.Appendf(b, "#EXTINF:4.000000,\nseg/%d\n", i)
	}
	return b
}()

func (c *contractRadio) Listen(ctx context.Context, session model.Session, stationID int64) (model.RadioListen, error) {
	st, err := c.Station(ctx, session, stationID)
	if err != nil {
		return model.RadioListen{}, err
	}
	id := fmt.Sprintf("01JRADFAKE%016d", c.live.seq.Add(1))
	l := &fakeListen{token: radio.SignToken(testRadioKey, id, stationID), stationID: stationID, sessionID: session.ID,
		accountID: session.Account.ID, profile: session.Profile, done: make(chan struct{})}
	c.live.mu.Lock()
	c.live.listens[id] = l
	c.live.mu.Unlock()
	return model.RadioListen{ListenID: id, Token: l.token, HeartbeatSeconds: 30, StartOffsetSeconds: -8, Station: st}, nil
}

// own returns the open listen session id of the profile.
func (c *contractRadio) own(session model.Session, id string) (*fakeListen, error) {
	c.live.mu.Lock()
	defer c.live.mu.Unlock()
	l, ok := c.live.listens[id]
	if !ok || l.profile.ID != session.Profile.ID {
		return nil, model.ErrNotFound
	}
	return l, nil
}

func (c *contractRadio) Heartbeat(_ context.Context, session model.Session, id string, _ bool) (model.RadioHeartbeat, error) {
	l, err := c.own(session, id)
	if err != nil {
		return model.RadioHeartbeat{}, err
	}
	if l.ended {
		return model.RadioHeartbeat{}, model.ErrRadioUnreachable
	}
	return model.RadioHeartbeat{ExpiresAt: time.Now().Add(30 * time.Minute)}, nil
}

func (c *contractRadio) EndListen(_ context.Context, session model.Session, id string) error {
	l, err := c.own(session, id)
	if err != nil {
		return err
	}
	c.live.end(func(x *fakeListen) bool { return x == l })
	return nil
}

// end ends every open fake listen session that match selects.
func (f *fakeRadioLive) end(match func(*fakeListen) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, l := range f.listens {
		if !l.ended && match(l) {
			l.ended = true
			close(l.done)
		}
	}
}

func (c *contractRadio) AuthorizeLive(_ context.Context, id, token string) (model.RadioGrant, error) {
	c.live.mu.Lock()
	defer c.live.mu.Unlock()
	l, ok := c.live.listens[id]
	if !ok || l.ended || token != l.token {
		return model.RadioGrant{}, model.ErrUnauthenticated
	}
	return model.RadioGrant{ListenID: id, StationID: l.stationID, Profile: l.profile}, nil
}

func (c *contractRadio) Playlist(_ context.Context, _ model.RadioGrant, skip string) ([]byte, error) {
	c.live.mu.Lock()
	c.live.skips = append(c.live.skips, skip)
	c.live.mu.Unlock()
	return fakeRadioPlaylist, nil
}

func (c *contractRadio) Init(_ context.Context, _ model.RadioGrant, gen int) ([]byte, error) {
	if gen != 0 {
		return nil, model.ErrNotFound
	}
	return []byte("fake init"), nil
}

func (c *contractRadio) Segment(_ context.Context, g model.RadioGrant, n int64) ([]byte, <-chan struct{}, error) {
	if n != 0 {
		return nil, nil, model.ErrNotFound
	}
	c.live.mu.Lock()
	done := c.live.listens[g.ListenID].done
	c.live.mu.Unlock()
	return []byte("fake segment"), done, nil
}

func (c *contractRadio) Now(ctx context.Context, session model.Session, stationID int64, _ time.Time) (model.RadioNow, error) {
	if _, err := c.Station(ctx, session, stationID); err != nil {
		return model.RadioNow{}, err
	}
	return model.RadioNow{Entries: []model.RadioNowEntry{}}, nil
}

// The session hooks end the fake listen sessions as well as the real ones.

func (c *contractRadio) EndSession(ctx context.Context, sessionID int64, by string) error {
	c.live.end(func(l *fakeListen) bool { return l.sessionID == sessionID })
	return c.Service.EndSession(ctx, sessionID, by)
}

func (c *contractRadio) EndProfile(ctx context.Context, profileID int64, by string) error {
	c.live.end(func(l *fakeListen) bool { return l.profile.ID == profileID })
	return c.Service.EndProfile(ctx, profileID, by)
}

func (c *contractRadio) EndAccount(ctx context.Context, accountID int64) error {
	c.live.end(func(l *fakeListen) bool { return l.accountID == accountID })
	return c.Service.EndAccount(ctx, accountID)
}
