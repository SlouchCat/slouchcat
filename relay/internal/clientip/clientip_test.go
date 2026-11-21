package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D-9: the header counts only from a trusted proxy, read from the back; the first entry that is
// not a trusted proxy is the caller, whatever stands in front of it.
func TestAddrBehindProxies(t *testing.T) {
	r, err := ParseTrusted("192.0.2.10, 198.51.100.0/24")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote string
		xff          []string
		want         string
	}{
		{"direct caller, no header", "203.0.113.5:4000", nil, "203.0.113.5"},
		{"direct caller forges the header", "203.0.113.5:4000", []string{"192.0.2.99"}, "203.0.113.5"},
		{"trusted proxy, one hop", "192.0.2.10:4000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"caller's own entry in front is not believed", "192.0.2.10:4000", []string{"203.0.113.66, 203.0.113.7"}, "203.0.113.7"},
		{"two trusted proxies", "192.0.2.10:4000", []string{"203.0.113.7, 198.51.100.20"}, "203.0.113.7"},
		{"header split over two lines", "192.0.2.10:4000", []string{"203.0.113.66", "203.0.113.8"}, "203.0.113.8"},
		{"all hops trusted: the first one", "192.0.2.10:4000", []string{"198.51.100.1, 198.51.100.2"}, "198.51.100.1"},
		{"malformed chain is not believed", "192.0.2.10:4000", []string{"203.0.113.7, garbage"}, "192.0.2.10"},
		{"trusted proxy without header", "192.0.2.10:4000", nil, "192.0.2.10"},
		{"IPv4-mapped proxy address", "[::ffff:192.0.2.10]:4000", []string{"203.0.113.9"}, "203.0.113.9"},
		{"IPv6 caller", "192.0.2.10:4000", []string{"2001:db8:1:2::5"}, "2001:db8:1:2::5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = c.remote
			for _, v := range c.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			got, ok := r.Addr(req)
			if !ok || got.String() != c.want {
				t.Fatalf("Addr = %v, %v; want %s", got, ok, c.want)
			}
		})
	}
}

func TestNoTrustedProxyIgnoresHeader(t *testing.T) {
	r, _ := ParseTrusted("")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got, _ := r.Addr(req); got.String() != "192.0.2.10" {
		t.Fatalf("Addr = %s; the header was believed without a trusted proxy", got)
	}
}

// D-2: IPv6 callers count by their /64.
func TestKeyGroupsIPv6By64(t *testing.T) {
	r, _ := ParseTrusted("")
	key := func(remote string) string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		return r.Key(req)
	}
	if a, b := key("[2001:db8:1:2::5]:1"), key("[2001:db8:1:2:ffff::9]:1"); a != b || a != "2001:db8:1:2::/64" {
		t.Fatalf("same /64 gave %q and %q", a, b)
	}
	if a, b := key("[2001:db8:1:2::5]:1"), key("[2001:db8:1:3::5]:1"); a == b {
		t.Fatal("different /64 gave the same key")
	}
	if got := key("192.0.2.1:1"); got != "192.0.2.1" {
		t.Fatalf("IPv4 key = %q", got)
	}
}

func TestParseTrustedRejectsGarbage(t *testing.T) {
	for _, s := range []string{"proxy.example.test", "192.0.2.0/33"} {
		if _, err := ParseTrusted(s); err == nil {
			t.Errorf("ParseTrusted(%q) accepted", s)
		}
	}
}
