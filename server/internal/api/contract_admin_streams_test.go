package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 18, part D (contract 1.23.0, section 17.4): the stream overview names what each stream
// plays, and the radio relays name their logo and each listener's device. All fields are additive.

// TestContractStreamOverviewFields checks the new fields in api/openapi.yaml: kind and source are
// required enums, the other stream fields optional, the listener's device and the relay's logo
// optional, and the relay state knows failed.
func TestContractStreamOverviewFields(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	schema := func(name string) (props map[string]any, required []any) {
		s, _ := resolve(doc, "#/components/schemas/"+name).(map[string]any)
		props, _ = s["properties"].(map[string]any)
		required, _ = s["required"].([]any)
		return props, required
	}
	enum := func(s map[string]any) []string {
		var out []string
		for _, v := range s["enum"].([]any) {
			out = append(out, v.(string))
		}
		return out
	}
	stream, required := schema("AdminStream")
	kind, _ := stream["kind"].(map[string]any)
	if kind["$ref"] != "#/components/schemas/AdminStreamKind" || !slices.Contains(required, any("kind")) {
		t.Errorf("AdminStream.kind = %v, want a required AdminStreamKind", kind)
	}
	kinds, _ := resolve(doc, "#/components/schemas/AdminStreamKind").(map[string]any)
	if got := enum(kinds); !slices.Equal(got, []string{"movie", "episode", "extra", "audiobook", "podcast_episode"}) {
		t.Errorf("AdminStreamKind = %v", got)
	}
	source, _ := stream["source"].(map[string]any)
	if source == nil || !slices.Equal(enum(source), []string{"file", "feed"}) || !slices.Contains(required, any("source")) {
		t.Errorf("AdminStream.source = %v, want a required enum file, feed", source)
	}
	for field, typ := range map[string]string{"parentTitle": "string", "chapterTitle": "string", "audioCodec": "string",
		"partCount": "integer", "audioBitrate": "integer"} {
		p, _ := stream[field].(map[string]any)
		if p == nil || p["type"] != typ || slices.Contains(required, any(field)) {
			t.Errorf("AdminStream.%s = %v, want an optional %s", field, p, typ)
		}
	}
	listener, required := schema("RadioRelayListener")
	device, _ := listener["device"].(map[string]any)
	if device == nil || device["type"] != "object" || slices.Contains(required, any("device")) {
		t.Errorf("RadioRelayListener.device = %v, want an optional object", device)
	}
	relay, required := schema("RadioRelay")
	logo, _ := relay["logo"].(map[string]any)
	if logo["$ref"] != "#/components/schemas/RadioLogo" || slices.Contains(required, any("logo")) {
		t.Errorf("RadioRelay.logo = %v, want an optional RadioLogo", logo)
	}
	states, _ := resolve(doc, "#/components/schemas/RadioRelayState").(map[string]any)
	if !slices.Contains(enum(states), "failed") {
		t.Errorf("RadioRelayState = %v, want failed among them", enum(states))
	}
}

// The new stream fields reach the JSON: kind, source, parent, chapter, parts and audio; empty ones
// are absent.
func TestAdminStreamKindFieldsJSON(t *testing.T) {
	t.Parallel()
	pos, dur := int64(2000), int64(9000)
	book := toAdminStream(model.AdminStream{ID: "01JAAAAAAAAAAAAAAAAAAAAAAA", ItemID: "01JBBBBBBBBBBBBBBBBBBBBBBB", ItemTitle: "Buch",
		Mode: "remux", StartedAt: time.Unix(1, 0), LastActivityAt: time.Unix(2, 0), PositionMs: &pos, DurationMs: &dur,
		Kind: "audiobook", Source: model.StreamSourceFile, ChapterTitle: "Kapitel 2", PartCount: 3, AudioCodec: "aac", AudioBitrate: 64000})
	feed := toAdminStream(model.AdminStream{ID: "01JCCCCCCCCCCCCCCCCCCCCCCC", ItemID: "01JDDDDDDDDDDDDDDDDDDDDDDD", ItemTitle: "Folge",
		Mode: "direct", StartedAt: time.Unix(1, 0), LastActivityAt: time.Unix(2, 0),
		Kind: "podcast_episode", Source: model.StreamSourceFeed, ParentTitle: "Podcast", AudioCodec: "mp3"})
	for _, c := range []struct {
		st   gen.AdminStream
		want map[string]any
		none []string
	}{
		{book, map[string]any{"kind": "audiobook", "source": "file", "chapterTitle": "Kapitel 2", "partCount": 3.0, "audioCodec": "aac",
			"audioBitrate": 64000.0}, []string{"parentTitle"}},
		{feed, map[string]any{"kind": "podcast_episode", "source": "feed", "parentTitle": "Podcast", "audioCodec": "mp3"},
			[]string{"chapterTitle", "partCount", "audioBitrate"}},
	} {
		b, err := json.Marshal(c.st)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%s: %s = %v, want %v", c.st.ItemTitle, k, got[k], v)
			}
		}
		for _, k := range c.none {
			if _, ok := got[k]; ok {
				t.Errorf("%s: %s = %v, want absent", c.st.ItemTitle, k, got[k])
			}
		}
	}
}

// Over HTTP a film and an episode name their kind and come from a file.
func TestAdminStreamKindHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	for _, id := range []string{f.Show.E1.ID, f.Rated(6).ID} {
		if _, code, p := play(t, h, "p18", id, `{}`); code != http.StatusOK {
			t.Fatalf("play %s = %d %+v", id, code, p)
		}
	}
	rec := send(t, h, http.MethodGet, BasePath+"/admin/streams", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var list []gen.AdminStream
	decode(t, rec, &list)
	kinds := map[string]gen.AdminStreamKind{}
	for _, st := range list {
		kinds[st.ItemId] = st.Kind
		if st.Source != gen.AdminStreamSourceFile || st.ParentTitle != nil || st.AudioCodec != nil {
			t.Errorf("stream %s: source %q parent %v audio %v, want a file without parent and audio", st.ItemTitle, st.Source, st.ParentTitle, st.AudioCodec)
		}
	}
	if kinds[f.Show.E1.ID] != gen.AdminStreamKindEpisode || kinds[f.Rated(6).ID] != gen.AdminStreamKindMovie {
		t.Errorf("kinds = %v", kinds)
	}
}

// relaysRadio is the contract radio with canned relays.
type relaysRadio struct {
	*contractRadio
	relays model.RadioRelays
}

func (r relaysRadio) Relays(context.Context) (model.RadioRelays, error) { return r.relays, nil }

// listRadioRelays carries the logo of the station (getAdminRadioImage, w256, content hash) and the
// device of each listener; a failed relay keeps its state.
func TestRadioRelaysDeviceLogoHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	since := time.UnixMilli(1_700_000_000_000)
	o := pf.opts
	o.Radio = relaysRadio{contractRadio: pf.radio, relays: model.RadioRelays{Relays: []model.RadioRelayInfo{
		{StationID: 7, Name: "Test FM", Codec: "MP3", BitrateKbps: 128, StartedAt: since, State: "live", LogoHash: "abc",
			Listeners: []model.RadioListenerInfo{
				{ListenID: "L1", ProfileName: "Marco", AccountEmail: "m@example.org", Playing: true, Since: since,
					Device: &model.DeviceRef{ID: "dev1", Name: "Küche (iPhone)"}},
				{ListenID: "L2", ProfileName: "Lena", AccountEmail: "l@example.org", Since: since},
			}},
		{StationID: 8, Name: "Ohne Logo", StartedAt: since, State: "failed", Listeners: []model.RadioListenerInfo{}},
	}}}
	rec := send(t, NewHandler(o), http.MethodGet, BasePath+"/admin/radio/relays", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var got gen.RadioRelays
	decode(t, rec, &got)
	if len(got.Relays) != 2 {
		t.Fatalf("relays = %+v", got.Relays)
	}
	r := got.Relays[0]
	if want := BasePath + "/admin/radio/images/" + strconv.Itoa(7) + "/logo/w256?v=abc"; r.Logo == nil || r.Logo.Url != want {
		t.Errorf("logo = %+v, want %q", r.Logo, want)
	}
	if d := r.Listeners[0].Device; d == nil || d.Id != "dev1" || deref(d.Name) != "Küche (iPhone)" {
		t.Errorf("device = %+v", d)
	}
	if r.Listeners[1].Device != nil {
		t.Errorf("listener without device = %+v", r.Listeners[1].Device)
	}
	if got.Relays[1].Logo != nil || got.Relays[1].State != gen.RadioRelayStateFailed {
		t.Errorf("second relay = %+v, want failed without logo", got.Relays[1])
	}
}
