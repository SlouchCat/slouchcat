//go:build radiotest

package radio

import (
	"net/netip"
	"syscall"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// controlFunc of the e2e build (bin/slouchcat-e2e) allows loopback, where the e2e test station
// runs, and still refuses every other local address. No environment variable switches it.
var controlFunc = func(network, address string, c syscall.RawConn) error {
	if ap, err := netip.ParseAddrPort(address); err == nil && ap.Addr().Unmap().IsLoopback() {
		return nil
	}
	return netguard.RefuseLocal(network, address, c)
}
