package radio

import "testing"

// The provider is the registrable domain of the stream host (B7.4); an IP address, a bare public
// suffix or a broken address give none.
func TestProviderOf(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		"https://stream.srg-ssr.ch/m/drs3/mp3_128":                "srg-ssr.ch",
		"http://playerservices.streamtheworld.com/api/livestream": "streamtheworld.com",
		"https://stream.zeno.fm/abc":                              "zeno.fm",
		"http://ice.infomaniak.ch:8000/x.mp3":                     "infomaniak.ch",
		"https://radio.example.co.uk/live":                        "example.co.uk",
		"HTTPS://Stream.SRG-SSR.CH./x":                            "srg-ssr.ch",
		"http://203.0.113.10/stream":                              "",
		"http://[2001:db8::1]:8000/stream":                        "",
		"http://co.uk/stream":                                     "",
		"not a url %zz":                                           "",
		"":                                                        "",
	} {
		if got := providerOf(raw); got != want {
			t.Errorf("providerOf(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := bestURL("http://a.example/x", "http://b.example/y"); got != "http://b.example/y" {
		t.Errorf("bestURL takes the resolved address: %q", got)
	}
	if got := bestURL("http://a.example/x", ""); got != "http://a.example/x" {
		t.Errorf("bestURL without a resolved address: %q", got)
	}
}
