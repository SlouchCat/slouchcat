package radio

import (
	"net/netip"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// The provider of a station (concept 19 part B, B7.4, stage B1): the registrable domain of the
// host of its best stream, the first row of ListRadioStreamsRanked (its resolved address, else its
// address): srg-ssr.ch, streamtheworld.com, zeno.fm. The sync sets it whenever it derives a
// station again, an own station when it is created or gets a new stream address, the first start
// after migration 00048 for every station.

// bestURL is the address of a stream the provider is read from: the resolved one, else the one
// the catalog names.
func bestURL(rawURL, resolved string) string {
	if resolved != "" {
		return resolved
	}
	return rawURL
}

// providerOf returns the registrable domain of an address' host; "" for an IP address, a host
// without a registrable domain or an address that does not parse.
func providerOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return ""
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return ""
	}
	d, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}
	return d
}
