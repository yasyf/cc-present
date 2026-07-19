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
// Every leg is dual-mode: sniffFactories wraps it to serve TLS and plaintext
// on the same port.
func tailnetListeners(p paths.Paths, bind string, addrs []netip.Addr, mgr *certManager) []func(context.Context) (net.Listener, error) {
	return sniffFactories(meshtrust.Listeners(bind, addrs, lastHTTPPort(p)), mgr)
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
func reconcileTailnet(ctx context.Context, srv *ccd.Server, tp *meshtrust.Provider, p paths.Paths, bind string, mgr *certManager) {
	tick := time.NewTicker(reconcileInterval)
	defer tick.Stop()
	for {
		// Each pass doubles as the cert renewal tick; ensure runs off-loop so a
		// slow `tailscale cert` issuance never delays leg binding.
		go func() { mgr.ensure(ctx, tp.SelfCertDomain(ctx)) }()
		reconcileTailnetPass(ctx, srv, tp, p, bind, mgr)
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
func reconcileTailnetPass(ctx context.Context, srv *ccd.Server, tp *meshtrust.Provider, p paths.Paths, bind string, mgr *certManager) {
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
	for _, factory := range sniffFactories(meshtrust.Listeners(bind, missing, hint), mgr) {
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
// when any exist, else — under an unspecified bind — the primary port on each
// self address, always as http on raw IPs (the primary listener has no TLS
// sniffer; https URLs only ever name sniffer-wrapped extra legs). A specific
// non-loopback bind serves only that address, so it advertises none.
func displayURLs(certDomain string, minted bool, extra []string, selfAddrs []netip.Addr, bind string, port int, slug string) []string {
	if len(extra) > 0 {
		return tailnetURLs(certDomain, minted, extra, slug)
	}
	if !isUnspecifiedBind(bind) || port < 1 || port > math.MaxUint16 {
		return nil
	}
	addrs := make([]string, 0, len(selfAddrs))
	for _, a := range selfAddrs {
		addrs = append(addrs, netip.AddrPortFrom(a.Unmap(), uint16(port)).String())
	}
	return tailnetURLs(certDomain, false, addrs, slug)
}

// isUnspecifiedBind reports whether bind is 0.0.0.0 or ::; the empty bind is
// the loopback default, not unspecified.
func isUnspecifiedBind(bind string) bool {
	ip, err := netip.ParseAddr(bind)
	return err == nil && ip.IsUnspecified()
}

// tailnetURLs renders the browsable URLs for a tailnet-served artifact. With a
// minted cert: https on the cert domain, one URL per distinct leg port (v4+v6
// share a port, so collapse to one). Without: http on the raw leg addresses —
// IP literals escape ts.net's HSTS preload, and every leg serves plaintext.
// A MagicDNS name is never composed into an http URL. Empty addrs → nil.
func tailnetURLs(certDomain string, minted bool, extraAddrs []string, slug string) []string {
	var urls []string
	if minted && certDomain != "" {
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
			urls = append(urls, fmt.Sprintf("https://%s:%d/p/%s", certDomain, ap.Port(), slug))
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
