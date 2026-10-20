package netguard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestLocal(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1": true, "127.8.9.10": true, "::1": true, "10.1.2.3": true, "172.16.0.1": true, "172.31.255.255": true,
		"192.168.1.43": true, "169.254.169.254": true, "fe80::1": true, "fd00::1": true, "0.0.0.0": true, "::": true,
		"::ffff:192.168.1.1": true, "::ffff:127.0.0.1": true,
		"203.0.113.5": false, "8.8.8.8": false, "172.32.0.1": false, "2001:db8::1": false, "100.64.0.1": false,
	} {
		if got := Local(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Local(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestLocalHost(t *testing.T) {
	ctx := context.Background()
	for raw, want := range map[string]bool{
		"http://127.0.0.1:8095/f/abc/feed.xml": true, "http://192.168.1.10/podcast.rss": true, "http://[::1]:80/x": true,
		"http://localhost:8080/feed": true, "https://203.0.113.5/feed.xml": false, "not a url\x00": false, "": false,
	} {
		if got := LocalHost(ctx, raw); got != want {
			t.Errorf("LocalHost(%q) = %v, want %v", raw, got, want)
		}
	}
}

// A feed on the loopback address (the household's own, like the e2e mock feed) may fetch from the
// loopback address; a fetch on behalf of a public feed is refused after DNS resolution, also when a
// redirect leads there.
func TestClientsFor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/episode.mp3", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("audio"))
	}))
	defer srv.Close()
	ctx := context.Background()
	c := NewClients(http.Client{}, 0)
	get := func(client *http.Client, url string) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}
	if err := get(c.For(ctx, srv.URL+"/feed.xml"), srv.URL+"/redirect"); err != nil {
		t.Errorf("loopback feed: %v", err)
	}
	public := c.For(ctx, "https://203.0.113.5/feed.xml?token=abc")
	for _, path := range []string{"/episode.mp3", "/redirect"} {
		if err := get(public, srv.URL+path); !errors.Is(err, ErrRefused) {
			t.Errorf("public feed, local enclosure %s: %v", path, err)
		}
	}
	// A host name resolving to the loopback address is refused as well.
	if err := get(public, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)); !errors.Is(err, ErrRefused) {
		t.Errorf("public feed, localhost: %v", err)
	}
}

// Fetch errors keep scheme and host only: path, query and user info of a private feed carry its
// credentials.
func TestClean(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/feeds/s3cr3t-path.xml?auth=s3cr3t-query", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("closed server answered")
	}
	wrapped := fmt.Errorf("fetch podcast: %w", Clean(err))
	if msg := wrapped.Error(); strings.Contains(msg, "s3cr3t") || !strings.Contains(msg, srv.URL) {
		t.Errorf("cleaned error %q", msg)
	}
	if Clean(nil) != nil {
		t.Error("Clean(nil) is not nil")
	}
	if got := Site("https://user:pw@feeds.example.org:8443/a/b?token=x"); got != "https://feeds.example.org:8443" {
		t.Errorf("Site = %q", got)
	}
}
