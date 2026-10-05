package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"

	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"

	"github.com/yasyf/cc-interact/cmd"
	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit/paths"
	"github.com/yasyf/synckit/meshtrust"

	"github.com/yasyf/cc-present/internal/app"
)

const (
	bindLAN      = "0.0.0.0"
	bindLoopback = "127.0.0.1"
)

// newPairCmd exposes the daemon to the LAN and prints a QR code the iOS client
// scans to pair. It restarts the daemon when its bind or token must change; open
// browser tabs reconnect on their own.
func newPairCmd(d cmd.Deps) *cobra.Command {
	var resetToken, off bool
	c := &cobra.Command{
		Use:   "pair",
		Short: "Expose the daemon to the LAN and print a QR code to pair the iOS client",
		Long: "Pair rebinds the daemon to the LAN (0.0.0.0), mints a bearer token, and prints a QR " +
			"code the cc-present iOS app scans to connect.\n\n" +
			"Open browser tabs reconnect on their own after the daemon restarts, so pairing never " +
			"interrupts a live session.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if off {
				return runOff(c.Context(), d, c.OutOrStdout())
			}
			return runPair(c.Context(), d, c.OutOrStdout(), resetToken)
		},
	}
	c.Flags().BoolVar(&resetToken, "reset-token", false, "regenerate the bearer token before pairing")
	c.Flags().BoolVar(&off, "off", false, "disable LAN pairing: rebind the daemon to loopback only")
	return c
}

// runPair rebinds the daemon to the LAN, ensures a bearer token, restarts the
// daemon if needed, then prints the tailnet and LAN candidates, a QR code, and
// the pairing payload as copyable text.
func runPair(ctx context.Context, d cmd.Deps, out io.Writer, resetToken bool) error {
	if err := setBind(bindLAN); err != nil {
		return err
	}
	prev, err := app.ReadToken()
	if err != nil {
		return err
	}
	var token string
	if resetToken {
		token, err = app.ResetToken()
	} else {
		token, err = app.EnsureToken()
	}
	if err != nil {
		return err
	}
	tokenChanged := resetToken || prev == ""

	info, err := ensureDaemon(ctx, d, bindLAN, tokenChanged)
	if err != nil {
		return err
	}

	ips, err := lanIPs()
	if err != nil {
		return err
	}
	label := tailnetLabel(ctx)
	host, err := pairHost(label, ips)
	if err != nil {
		return err
	}

	if label != "" {
		_, _ = fmt.Fprintln(out, "Tailnet address:")
		_, _ = fmt.Fprintf(out, "  %s:%d\n\n", label, info.Port)
	}
	if len(ips) > 0 {
		_, _ = fmt.Fprintln(out, "LAN addresses:")
		for _, ip := range ips {
			_, _ = fmt.Fprintf(out, "  %s:%d\n", ip, info.Port)
		}
		_, _ = fmt.Fprintln(out)
	}

	_, payload, err := composePairPayload(host, info.Port, token)
	if err != nil {
		return err
	}
	qrterminal.GenerateWithConfig(payload, qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    out,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	})
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintf(out, "pair payload: %s\n", payload)
	_, _ = fmt.Fprintf(out, "host: %s:%d\n", host, info.Port)
	_, _ = fmt.Fprintln(out, "\nOpen browser tabs reconnect on their own after the daemon restarts.")
	return nil
}

// runOff rebinds the daemon to loopback only and restarts it, taking the plane
// off the LAN.
func runOff(ctx context.Context, d cmd.Deps, out io.Writer) error {
	if err := setBind(bindLoopback); err != nil {
		return err
	}
	if _, err := ensureDaemon(ctx, d, bindLoopback, false); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "LAN pairing disabled; the daemon now binds 127.0.0.1 only.")
	return nil
}

// setBind persists the desired bind to the host config, a no-op when it already
// matches.
func setBind(bind string) error {
	cfg, err := app.ReadConfig()
	if err != nil {
		return err
	}
	if cfg.Bind == bind {
		return nil
	}
	cfg.Bind = bind
	return app.WriteConfig(cfg)
}

// ensureDaemon brings a daemon bound to desiredBind online and returns its
// published handshake. A running daemon is restarted when its effective bind
// differs or the token changed; otherwise a stopped daemon is cold-started.
func ensureDaemon(ctx context.Context, d cmd.Deps, desiredBind string, tokenChanged bool) (ccd.HTTPInfo, error) {
	if err := d.EnsureCurrent(ctx); err != nil {
		return ccd.HTTPInfo{}, err
	}
	info := readHTTPInfo(d.Paths)
	if tokenChanged || effectiveBind(info.Bind) != desiredBind {
		if err := restartDaemon(ctx, d); err != nil {
			return ccd.HTTPInfo{}, err
		}
		info = readHTTPInfo(d.Paths)
	}
	if info.Port == 0 {
		return ccd.HTTPInfo{}, errors.New("daemon did not publish its HTTP port")
	}
	return info, nil
}

// restartDaemon stops the running daemon through its exact-role controller,
// then lets EnsureCurrent replace it with a daemon that re-reads host config.
func restartDaemon(ctx context.Context, d cmd.Deps) error {
	if err := d.Stop(ctx); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	return d.EnsureCurrent(ctx)
}

// effectiveBind resolves an empty bind to the loopback default the daemon
// applies, so it compares equal to a handshake that recorded "127.0.0.1".
func effectiveBind(bind string) string {
	if bind == "" {
		return bindLoopback
	}
	return bind
}

// readHTTPInfo reads the daemon's published handshake, returning the zero value
// when it is absent or unreadable.
func readHTTPInfo(p paths.Paths) ccd.HTTPInfo {
	b, err := os.ReadFile(p.HTTPInfoPath())
	if err != nil {
		return ccd.HTTPInfo{}
	}
	var info ccd.HTTPInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return ccd.HTTPInfo{}
	}
	return info
}

// pairPayload is the compact JSON the QR encodes and the command also prints as
// copyable text: the protocol version, the daemon URL, and the bearer token the
// iOS client presents.
type pairPayload struct {
	V     int    `json:"v"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

// tailnetLabel is this machine's bare MagicDNS label when the synckit mesh and
// tailscale are up, else empty.
func tailnetLabel(ctx context.Context) string {
	tp := meshtrust.Detect()
	if tp == nil {
		return ""
	}
	return tp.SelfHostLabel(ctx)
}

// pairHost picks the host the pairing URL names: the tailnet label, which the
// phone reaches on and off the LAN, else the first LAN address.
func pairHost(label string, ips []net.IP) (string, error) {
	if label != "" {
		return label, nil
	}
	if len(ips) == 0 {
		return "", errors.New("no tailnet name or LAN IPv4 address found on any up, non-loopback interface")
	}
	return ips[0].String(), nil
}

// composePairPayload builds the pairing payload and its compact JSON encoding.
func composePairPayload(host string, port int, token string) (pairPayload, string, error) {
	p := pairPayload{V: 1, URL: "http://" + net.JoinHostPort(host, strconv.Itoa(port)), Token: token}
	raw, err := json.Marshal(p)
	if err != nil {
		return pairPayload{}, "", err
	}
	return p, string(raw), nil
}

// netIface is the subset of a network interface the LAN-IP picker reads, so a
// test drives it with fabricated data.
type netIface struct {
	Up       bool
	Loopback bool
	Addrs    []net.Addr
}

// pickLANIPs returns the usable LAN IPv4 addresses across ifaces, private-range
// (real LAN) addresses first, skipping down and loopback interfaces and any
// non-IPv4, loopback, or link-local address.
func pickLANIPs(ifaces []netIface) []net.IP {
	var private, other []net.IP
	for _, ifc := range ifaces {
		if !ifc.Up || ifc.Loopback {
			continue
		}
		for _, a := range ifc.Addrs {
			v4 := addrIP(a).To4()
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
				continue
			}
			if v4.IsPrivate() {
				private = append(private, v4)
			} else {
				other = append(other, v4)
			}
		}
	}
	return append(private, other...)
}

// addrIP extracts the IP from a network address, ignoring the mask.
func addrIP(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

// lanIPs picks the host's LAN IPv4 addresses from its real network interfaces.
func lanIPs() ([]net.IP, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}
	out := make([]netIface, 0, len(ifs))
	for _, ifc := range ifs {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		out = append(out, netIface{
			Up:       ifc.Flags&net.FlagUp != 0,
			Loopback: ifc.Flags&net.FlagLoopback != 0,
			Addrs:    addrs,
		})
	}
	return pickLANIPs(out), nil
}
