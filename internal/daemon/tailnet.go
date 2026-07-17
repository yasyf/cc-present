package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"os"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/paths"
)

type listenFunc func(address string) (net.Listener, error)

func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}

// tailnetListeners eagerly binds one extra listener per tailnet address, so a
// loopback-bound daemon also serves the mesh, returning them as ccd factories.
// An unbindable address is skipped with a warning (loopback must survive); nil
// for a non-loopback primary bind (it covers the tailnet; a second bind would
// collide) or empty addrs. Binds try the last handshake's port, then ephemeral.
func tailnetListeners(p paths.Paths, bind string, addrs []netip.Addr) []func(context.Context) (net.Listener, error) {
	return bindTailnet(netListen, p, bind, addrs)
}

func bindTailnet(listen listenFunc, p paths.Paths, bind string, addrs []netip.Addr) []func(context.Context) (net.Listener, error) {
	if !isLoopbackBind(bind) {
		return nil
	}
	if len(addrs) == 0 {
		slog.Warn("trust: no tailnet addresses; serving loopback only")
		return nil
	}
	hint := lastHTTPPort(p)
	factories := make([]func(context.Context) (net.Listener, error), 0, len(addrs))
	for _, addr := range addrs {
		var ln net.Listener
		var err error
		if hint != 0 {
			ln, err = listen(netip.AddrPortFrom(addr, hint).String())
		}
		if ln == nil {
			ln, err = listen(netip.AddrPortFrom(addr, 0).String())
		}
		if err != nil {
			slog.Warn("trust: cannot bind tailnet address; skipping", "addr", addr, "err", err)
			continue
		}
		factories = append(factories, func(context.Context) (net.Listener, error) { return ln, nil })
	}
	return factories
}

// lastHTTPPort reads the previous boot's handshake as a port-reuse hint, zero
// when absent, unreadable, or out of port range.
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
