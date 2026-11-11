package radio

import (
	"context"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

// The row radio lists the reachable favourites in their order; a station with a running relay
// carries the title at its live edge, one without none, and a favourite outside the offer is left out (D-7).
func TestHomeStations(t *testing.T) {
	e := newRelayEnv(t)
	m := mp3Mount(t, false)
	m.MetaInt = 8192
	m.Titles = []radiotest.Title{{At: 0, Text: "Künstler - Lied"}, {At: 40_000, Text: "Band - Song"}}
	m.Once = true
	e.st.Set("/live", m)
	live := e.addStation("Live FM", "CH", "MP3", 128, e.st.URL("/live"))
	quiet := e.addStation("Quiet FM", "CH", "MP3", 128, e.st.URL("/quiet"))
	hidden := e.addStation("Hidden FM", "CH", "MP3", 128, e.st.URL("/hidden"))
	e.setOffered(hidden, false)
	for pos, id := range []int64{quiet, hidden, live} {
		e.exec(`INSERT INTO radio_favorites (profile_id, station_id, position, created_at) VALUES (?, ?, ?, 1)`, e.f.P18.ID, id, pos)
	}
	tiles, err := e.s.HomeStations(context.Background(), e.f.P18)
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != 2 || tiles[0].Station.ID != quiet || tiles[1].Station.ID != live || tiles[0].Now != nil || tiles[1].Now != nil ||
		!tiles[0].Station.Favorite {
		t.Fatalf("tiles without a relay: %+v", tiles)
	}
	if other, err := e.s.HomeStations(context.Background(), e.f.P6); err != nil || len(other) != 0 {
		t.Fatalf("a profile without favourites: %+v %v", other, err)
	}
	e.listen(e.f.P18, live)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if tiles, err = e.s.HomeStations(context.Background(), e.f.P18); err != nil {
			t.Fatal(err)
		}
		if (tiles[1].Now != nil && tiles[1].Now.Text == "Band - Song") || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tiles[1].Now == nil || tiles[1].Now.Artist != "Band" || tiles[1].Now.Title != "Song" || tiles[0].Now != nil {
		t.Fatalf("tiles with a relay: %+v %+v", tiles[0], tiles[1].Now)
	}
}
