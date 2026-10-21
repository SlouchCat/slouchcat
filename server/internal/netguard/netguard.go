// Package netguard guards the server's fetches of URLs a podcast feed names: the feed itself, its
// enclosures and its cover images (M6, ADR 0007). The feed's publisher chooses those URLs, not the
// admin, so a fetch must not reach the household's own network: after DNS resolution (which covers
// redirects and DNS rebinding) the dialer refuses loopback, private (RFC 1918, unique-local),
// link-local and unspecified addresses, unless the feed URL's own host is such an address (a feed
// the household runs on its LAN on purpose; its enclosures and covers may then be local too).
//
// Feed and enclosure URLs may carry credentials in their path or query (private feeds), so fetch
// errors name only the scheme and host of a URL (Clean, CLAUDE.md rule 10). The package has no
// database access.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"syscall"
	"time"
)

// ErrRefused is the dial error of a guarded fetch whose address lies in the local network.
var ErrRefused = errors.New("address in the local network refused")

// Local reports whether ip is an address a guarded fetch refuses: loopback, private (10/8,
// 172.16/12, 192.168/16, fc00::/7), link-local (169.254/16, fe80::/10) or unspecified.
func Local(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// LocalHost reports whether the host of rawURL is, or resolves to, a local address. A feed on such
// a host is one the household runs itself; an unparsable URL or a failed lookup is not local.
func LocalHost(ctx context.Context, rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return false
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		return Local(ip)
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	return err == nil && slices.ContainsFunc(addrs, Local)
}

// RefuseLocal is the Control function of a guarded dialer: it sees the resolved address of every
// connection, including those of redirects, and refuses a Local one with ErrRefused. The push
// requests of package notify/push use it as well (an endpoint is chosen by a browser).
func RefuseLocal(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: unparsable address %q", ErrRefused, address)
	}
	if Local(ap.Addr()) {
		return fmt.Errorf("%w: %s", ErrRefused, ap.Addr())
	}
	return nil
}

// Clients are two HTTP clients with one policy (timeout and redirect rule of the caller): public
// refuses local addresses, local does not. Neither uses a proxy from the environment, because the
// guard checks the address it dials.
type Clients struct {
	public, local *http.Client
}

// NewClients returns the clients of one kind of fetch: base gives the overall Timeout and the
// CheckRedirect policy, headerTimeout bounds the wait for the response headers (0: none).
func NewClients(base http.Client, headerTimeout time.Duration) Clients {
	client := func(guarded bool) *http.Client {
		d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		if guarded {
			d.Control = RefuseLocal
		}
		c := base
		c.Transport = &http.Transport{
			DialContext:           d.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: headerTimeout,
		}
		return &c
	}
	return Clients{public: client(true), local: client(false)}
}

// For returns the client of a fetch on behalf of the feed at feedURL: the unguarded one when the
// feed's own host is local, else the guarded one.
func (c Clients) For(ctx context.Context, feedURL string) *http.Client {
	if LocalHost(ctx, feedURL) {
		return c.local
	}
	return c.public
}

// Site returns the scheme and host of rawURL ("https://feeds.example.org"), without user info,
// path and query; "" when it does not parse.
func Site(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// Clean reduces the URL of the *url.Error in err (every error of http.Client.Do is one) to its
// scheme and host, in place, and returns err, so it can be logged and stored. Call it before the
// error is wrapped: fmt.Errorf formats the text right away. errors.Is and errors.As still reach the
// cause.
func Clean(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		ue.URL = Site(ue.URL)
	}
	return err
}
