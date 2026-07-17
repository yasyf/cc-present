package cli

import (
	"net/netip"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/synckit/meshtrust"
)

func TestRenderTrust(t *testing.T) {
	mesh := meshtrust.Mesh{
		Self: "yasyf@yasyf-home.tail71af5d.ts.net",
		Hosts: []meshtrust.HostTrust{
			{Target: "yasyf@yasyf.tail71af5d.ts.net", Addrs: []netip.Addr{
				netip.MustParseAddr("100.114.101.73"),
				netip.MustParseAddr("fd7a:115c:a1e0::d101:654a"),
			}},
			{Target: "yasyf@nas-local"},
		},
	}
	tests := []struct {
		name string
		mesh meshtrust.Mesh
		info ccd.HTTPInfo
		live bool
		want []string
	}{
		{
			"live listeners",
			mesh,
			ccd.HTTPInfo{Port: 52668, ExtraAddrs: []string{"100.88.252.58:52668"}},
			true,
			[]string{
				"synckit mesh: /tmp/state.json",
				"self: yasyf@yasyf-home.tail71af5d.ts.net",
				"  yasyf@yasyf.tail71af5d.ts.net → 100.114.101.73, fd7a:115c:a1e0::d101:654a",
				"  yasyf@nas-local → no tailnet IPs — not network-trusted",
				"tailnet listeners: 100.88.252.58:52668",
			},
		},
		{
			"daemon down with last-known listeners",
			mesh,
			ccd.HTTPInfo{Port: 52668, ExtraAddrs: []string{"100.88.252.58:52668"}},
			false,
			[]string{"tailnet listeners (last known; daemon not running): 100.88.252.58:52668"},
		},
		{
			"wildcard bind covers the tailnet",
			mesh,
			ccd.HTTPInfo{Port: 52668, Bind: "0.0.0.0"},
			true,
			[]string{"tailnet listeners: primary bind 0.0.0.0 serves the tailnet IPs on port 52668"},
		},
		{
			"live with none published",
			meshtrust.Mesh{Self: "yasyf@solo"},
			ccd.HTTPInfo{},
			true,
			[]string{
				"  (none registered)",
				"tailnet listeners: none published — tailscale down, or daemon started before trust",
			},
		},
		{
			"daemon down with no handshake",
			meshtrust.Mesh{Self: "yasyf@solo"},
			ccd.HTTPInfo{},
			false,
			[]string{"tailnet listeners: daemon not running"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			renderTrust(&b, "/tmp/state.json", tt.mesh, tt.info, tt.live)
			for _, want := range tt.want {
				if !strings.Contains(b.String(), want) {
					t.Errorf("output missing %q:\n%s", want, b.String())
				}
			}
		})
	}
}
