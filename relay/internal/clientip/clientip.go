// Package clientip finds the address of the caller behind a reverse proxy and turns it into the key
// the limits count by (concept 27, rules D-2 and D-9).
//
// The rule is the server's (clientIP in server/internal/api/session.go): the X-Forwarded-For header
// is believed only when the request comes from a trusted proxy, it is read from the back, and the
// first entry that is not a trusted proxy is the caller. What a caller writes into the header
// itself stands further to the front and is never reached. A malformed chain is not believed.
//
// The address is used for the limits only and lives in memory; nothing here logs it (D-5).
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver knows the trusted proxies.
type Resolver struct {
	trusted []netip.Prefix
	// Untrusted is called when a request carries X-Forwarded-For but comes from a peer that is
	// not a trusted proxy: either a caller forging the header, or a reverse proxy missing from
	// RELAY_TRUSTED_PROXIES, which makes every caller count as one (L-5). The service logs it
	// rate-limited and without the address. May be nil.
	Untrusted func()
}

// ParseTrusted reads a comma- or space-separated list of addresses and prefixes
// ("192.0.2.10, 198.51.100.0/24, 2001:db8::/48"). A bare address is a prefix of its full length.
func ParseTrusted(list string) (*Resolver, error) {
	r := &Resolver{}
	for _, f := range strings.FieldsFunc(list, func(c rune) bool { return c == ',' || c == ' ' || c == '\t' || c == '\n' }) {
		if strings.Contains(f, "/") {
			p, err := netip.ParsePrefix(f)
			if err != nil {
				return nil, fmt.Errorf("trusted proxy %q: %w", f, err)
			}
			r.trusted = append(r.trusted, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", f, err)
		}
		a = a.Unmap()
		r.trusted = append(r.trusted, netip.PrefixFrom(a, a.BitLen()))
	}
	return r, nil
}

// Count is the number of trusted prefixes (for the start-up log line).
func (r *Resolver) Count() int { return len(r.trusted) }

// Addr returns the caller's address. The second result is false when RemoteAddr is unusable.
func (r *Resolver) Addr(req *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	remote = remote.WithZone("").Unmap()
	if !r.isTrusted(remote) {
		if r.Untrusted != nil && req.Header.Get("X-Forwarded-For") != "" {
			r.Untrusted()
		}
		return remote, true
	}
	hops := strings.Split(strings.Join(req.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		h := strings.TrimSpace(hops[i])
		if h == "" && len(hops) == 1 {
			return remote, true // no header at all: the proxy itself is the caller
		}
		ip, err := netip.ParseAddr(h)
		if err != nil {
			return remote, true // a malformed chain is not believed
		}
		ip = ip.WithZone("").Unmap()
		if !r.isTrusted(ip) || i == 0 {
			return ip, true
		}
	}
	return remote, true
}

// Key is what the limits count by: an IPv4 address as itself, an IPv6 address by its /64 network
// (D-2), because one household or one host usually owns a whole /64.
func (r *Resolver) Key(req *http.Request) string {
	k, _ := r.Keys(req)
	return k
}

// Keys returns the caller key (Key) and the key of the larger network around it (NetOf). The
// services limit by both: one /48 holds 65 536 /64 networks and costs an attacker little, so a
// limit per /64 alone does not bound him (security review M-1, M-2).
func (r *Resolver) Keys(req *http.Request) (caller, network string) {
	a, ok := r.Addr(req)
	if !ok {
		return "unknown", "unknown"
	}
	return KeyOf(a), NetOf(a)
}

// NetOf is the larger network of a: the /24 of an IPv4 address, the /48 of an IPv6 address.
func NetOf(a netip.Addr) string {
	bits := 48
	if a.Is4() {
		bits = 24
	}
	p, _ := a.Prefix(bits)
	return p.String()
}

// KeyOf is Key for an address already known.
func KeyOf(a netip.Addr) string {
	if a.Is4() {
		return a.String()
	}
	p, _ := a.Prefix(64)
	return p.String()
}

func (r *Resolver) isTrusted(a netip.Addr) bool {
	for _, p := range r.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
