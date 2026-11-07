package radio

import (
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

// The probe opens the best stream through the guarded client and reports format, bitrate and the
// first ICY title; an unknown station or stream is 404.
func TestProbeStation(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	st := startStation(t)
	st.Set("/probe", radiotest.Mount{Audio: [][]byte{testAudio(t, "tone-mp3-128.mp3")}, MetaInt: 4096,
		Titles: []radiotest.Title{{At: 0, Text: "Künstler - Titel"}}})
	withDialControl(e.s, loopbackOnly)
	id := e.stationID("Radio 24")
	e.exec(`UPDATE radio_streams SET url = ?, url_resolved = NULL WHERE station_id = ?`, st.URL("/probe"), id)
	p, err := e.s.Probe(e.ctx, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != "mp3" || p.Codec != "mp3" || p.BitrateKbps != 128 || !p.HasMetadata || p.Title != "Künstler - Titel" || p.Response <= 0 {
		t.Errorf("probe = %+v", p)
	}
	if _, err := e.s.Probe(e.ctx, id, 999999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown stream: %v", err)
	}
	if _, err := e.s.Probe(e.ctx, 999999, 0); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown station: %v", err)
	}
}

// Without a test client the catalog goes through the guarded client of the service, which refuses
// the loopback of the test server in the production build.
func TestCatalogUsesGuardedClient(t *testing.T) {
	e := newCatEnv(t)
	e.s.cat.fetch.client = nil
	_, err := e.s.syncCatalog(e.ctx)
	if !errors.Is(err, netguard.ErrRefused) {
		t.Fatalf("sync from 127.0.0.1 through the guarded client: %v, want ErrRefused", err)
	}
}
