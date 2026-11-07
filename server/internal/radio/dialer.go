//go:build !radiotest

package radio

import "github.com/slouchcat/slouchcat/server/internal/netguard"

// controlFunc is the dial control of every outgoing radio connection (streams, playlists, HLS
// segments, the catalog and logos): the production build refuses every local address
// (netguard.RefuseLocal, concept 16 section 7.1). The e2e build (tag radiotest) has a twin in
// dialer_radiotest.go that allows loopback only.
var controlFunc = netguard.RefuseLocal
