package daemon

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/netip"
	"os"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/paths"
	"github.com/yasyf/synckit/meshtrust"
)

// tailnetListeners composes meshtrust.Listeners with the persisted handshake's
// port-reuse hint, so a restarting daemon reclaims its previous tailnet port.
func tailnetListeners(p paths.Paths, bind string, addrs []netip.Addr) []func(context.Context) (net.Listener, error) {
	return meshtrust.Listeners(bind, addrs, lastHTTPPort(p))
}

// lastHTTPPort reads the previous boot's handshake as a port-reuse hint for
// meshtrust.Listeners, zero when absent, unreadable, or out of port range.
func lastHTTPPort(p paths.Paths) uint16 {
	b, err := os.ReadFile(p.HTTPInfoPath())
	if err != nil {
		return 0
	}
	var info ccd.HTTPInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return 0
	}
	if info.Port < 1 || info.Port > math.MaxUint16 {
		return 0
	}
	return uint16(info.Port)
}
