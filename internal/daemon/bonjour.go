package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/grandcat/zeroconf"
)

// bonjourService is the mDNS service type cc-present advertises so an iOS client
// can discover a LAN-exposed daemon without a typed-in address.
const bonjourService = "_cc-present._tcp"

// bonjourHook returns the OnHTTPStart hook that advertises the HTTP plane over
// mDNS once it is bound. A loopback bind is unreachable off-host, so it
// advertises nothing and returns nil (no hook). The token is never advertised —
// the TXT records carry only the protocol version and the host name.
func bonjourHook(bind string) func(ctx context.Context, port int) {
	if isLoopbackBind(bind) {
		return nil
	}
	return func(ctx context.Context, port int) {
		host, err := os.Hostname()
		if err != nil {
			slog.Error("bonjour: resolve hostname", "err", err)
			return
		}
		server, err := zeroconf.Register(host, bonjourService, "local.", port,
			[]string{"v=1", "name=" + host}, nil)
		if err != nil {
			slog.Error("bonjour: register service", "service", bonjourService, "err", err)
			return
		}
		slog.Info("bonjour: advertising", "service", bonjourService, "instance", host, "port", port)
		<-ctx.Done()
		server.Shutdown()
	}
}

// isLoopbackBind reports whether bind keeps the HTTP plane on loopback. An empty
// bind is the loopback default; a loopback IP is loopback; any other address
// (0.0.0.0 or a LAN IP) exposes the plane.
func isLoopbackBind(bind string) bool {
	if bind == "" {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}
