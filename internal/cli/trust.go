package cli

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"
	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/synckit/meshtrust"
)

// newTrustCmd reports the synckit mesh trust the daemon derives automatically:
// which hosts are trusted, the tailnet addresses they resolve to, and the
// tailnet listeners the running daemon serves. Read-only — enablement follows
// synckit's presence, with nothing to configure.
func newTrustCmd(d cmd.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "trust",
		Short: "Show which synckit mesh hosts can reach the HTTP plane without a token",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return runTrust(c.Context(), c.OutOrStdout(), d)
		},
	}
}

func runTrust(ctx context.Context, out io.Writer, d cmd.Deps) error {
	path, err := meshtrust.StatePath()
	if err != nil {
		return err
	}
	tp := meshtrust.Detect()
	if tp == nil {
		_, _ = fmt.Fprintf(out, "synckit not detected (no %s); tailnet trust off\n", path)
		return nil
	}
	renderTrust(out, path, tp.Mesh(ctx), readHTTPInfo(d.Paths), d.NewClient().Available())
	return nil
}

func renderTrust(out io.Writer, statePath string, m meshtrust.Mesh, info ccd.HTTPInfo, live bool) {
	_, _ = fmt.Fprintf(out, "synckit mesh: %s\n", statePath)
	_, _ = fmt.Fprintf(out, "self: %s\n", m.Self)
	_, _ = fmt.Fprintln(out, "trusted hosts:")
	if len(m.Hosts) == 0 {
		_, _ = fmt.Fprintln(out, "  (none registered)")
	}
	for _, h := range m.Hosts {
		if len(h.Addrs) == 0 {
			_, _ = fmt.Fprintf(out, "  %s → no tailnet IPs — not network-trusted\n", h.Target)
			continue
		}
		addrs := make([]string, len(h.Addrs))
		for i, a := range h.Addrs {
			addrs[i] = a.String()
		}
		_, _ = fmt.Fprintf(out, "  %s → %s\n", h.Target, strings.Join(addrs, ", "))
	}
	_, _ = fmt.Fprint(out, listenerLine(info, live))
}

// listenerLine renders the daemon's tailnet-serving state: live listeners, a
// wide primary bind that already covers the tailnet, last-known handshake data
// when the daemon is down, or none.
func listenerLine(info ccd.HTTPInfo, live bool) string {
	label := "tailnet listeners"
	if !live {
		label = "tailnet listeners (last known; daemon not running)"
	}
	if a, err := netip.ParseAddr(info.Bind); err == nil && !a.IsLoopback() {
		return fmt.Sprintf("%s: primary bind %s serves the tailnet IPs on port %d\n", label, info.Bind, info.Port)
	}
	if len(info.ExtraAddrs) > 0 {
		return fmt.Sprintf("%s: %s\n", label, strings.Join(info.ExtraAddrs, ", "))
	}
	if !live {
		return "tailnet listeners: daemon not running\n"
	}
	return "tailnet listeners: none published — tailscale down, or daemon started before trust\n"
}
