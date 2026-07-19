package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"os"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit/paths"
	"github.com/yasyf/synckit/meshtrust"
)

// reconcileInterval is how often reconcileTailnet re-checks the live tailnet for
// addresses that gained a leg since the last pass.
const reconcileInterval = 30 * time.Second

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

// reconcileTailnet binds a leg for any tailnet address gained since boot, once
// immediately then every reconcileInterval until ctx is cancelled — so a late
// `tailscale up` is picked up without a daemon restart.
func reconcileTailnet(ctx context.Context, srv *ccd.Server, tp *meshtrust.Provider, p paths.Paths, bind string) {
	tick := time.NewTicker(reconcileInterval)
	defer tick.Stop()
	for {
		reconcileTailnetPass(ctx, srv, tp, p, bind)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// reconcileTailnetPass binds one leg per Provider address not already served.
// meshtrust.Listeners binds eagerly, so a served address is filtered by exact IP
// first — re-passing it would bind ip:0 and double-leg on a fresh port; the port
// hint reuses an existing leg's port (else the handshake) to keep every leg on
// one restart-sticky port.
func reconcileTailnetPass(ctx context.Context, srv *ccd.Server, tp *meshtrust.Provider, p paths.Paths, bind string) {
	extra := srv.HTTPExtraAddrs()
	bound := make(map[netip.Addr]bool, len(extra))
	hint := lastHTTPPort(p)
	haveHint := false
	for _, a := range extra {
		ap, err := netip.ParseAddrPort(a)
		if err != nil {
			continue
		}
		bound[ap.Addr().Unmap()] = true
		if !haveHint {
			hint = ap.Port()
			haveHint = true
		}
	}
	var missing []netip.Addr
	for _, a := range tp.SelfAddrs(ctx) {
		if !bound[a.Unmap()] {
			missing = append(missing, a)
			bound[a.Unmap()] = true
		}
	}
	if len(missing) == 0 {
		return
	}
	for _, factory := range meshtrust.Listeners(bind, missing, hint) {
		ln, err := factory(ctx)
		if err != nil {
			slog.Warn("tailnet: bind listener", "err", err)
			continue
		}
		// AddHTTPListener closes ln itself on refusal, so a failure needs no cleanup.
		if err := srv.AddHTTPListener(ln); err != nil {
			slog.Warn("tailnet: add listener", "addr", ln.Addr().String(), "err", err)
		}
	}
}

// displayURLs composes an artifact's tailnet display URLs: the live extra legs
// when any exist, else — under a non-loopback bind, where the primary listener
// already serves the tailnet — the primary port on each self address.
func displayURLs(dns string, extra []string, selfAddrs []netip.Addr, loopbackBind bool, port int, slug string) []string {
	if len(extra) == 0 && !loopbackBind && port >= 1 && port <= math.MaxUint16 {
		for _, a := range selfAddrs {
			extra = append(extra, netip.AddrPortFrom(a.Unmap(), uint16(port)).String())
		}
	}
	return tailnetURLs(dns, extra, slug)
}

// tailnetURLs renders the browsable http URLs for a tailnet-served artifact —
// always http (plaintext behind tailnet encryption). A MagicDNS name yields one
// http://dns:PORT/p/slug per distinct leg port (v4+v6 share a port, so collapse
// to one); no name falls back to one URL per raw leg address. Empty addrs → nil.
func tailnetURLs(dns string, extraAddrs []string, slug string) []string {
	var urls []string
	if dns != "" {
		seen := make(map[uint16]bool, len(extraAddrs))
		for _, a := range extraAddrs {
			ap, err := netip.ParseAddrPort(a)
			if err != nil {
				continue
			}
			if seen[ap.Port()] {
				continue
			}
			seen[ap.Port()] = true
			urls = append(urls, fmt.Sprintf("http://%s:%d/p/%s", dns, ap.Port(), slug))
		}
		return urls
	}
	for _, a := range extraAddrs {
		ap, err := netip.ParseAddrPort(a)
		if err != nil {
			continue
		}
		urls = append(urls, fmt.Sprintf("http://%s/p/%s", ap.String(), slug))
	}
	return urls
}
