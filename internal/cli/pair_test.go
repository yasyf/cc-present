package cli

import (
	"context"
	"net"
	"os"
	"reflect"
	"testing"

	"github.com/yasyf/cc-interact/cmd"
	"github.com/yasyf/daemonkit/paths"
)

// ipNet wraps a parsed IP as a *net.IPNet, the concrete net.Addr the real
// net.Interface.Addrs returns, so the picker sees production-shaped input.
func ipNet(s string) net.Addr {
	return &net.IPNet{IP: net.ParseIP(s), Mask: net.CIDRMask(24, 32)}
}

func TestPickLANIPs(t *testing.T) {
	tests := []struct {
		name   string
		ifaces []netIface
		want   []string
	}{
		{
			name:   "private ipv4 included",
			ifaces: []netIface{{Up: true, Addrs: []net.Addr{ipNet("192.168.1.5")}}},
			want:   []string{"192.168.1.5"},
		},
		{
			name:   "down interface skipped",
			ifaces: []netIface{{Up: false, Addrs: []net.Addr{ipNet("192.168.1.5")}}},
			want:   nil,
		},
		{
			name:   "loopback interface skipped",
			ifaces: []netIface{{Up: true, Loopback: true, Addrs: []net.Addr{ipNet("192.168.1.5")}}},
			want:   nil,
		},
		{
			name:   "ipv6 skipped",
			ifaces: []netIface{{Up: true, Addrs: []net.Addr{ipNet("fe80::1"), ipNet("2001:db8::1")}}},
			want:   nil,
		},
		{
			name:   "link-local ipv4 skipped",
			ifaces: []netIface{{Up: true, Addrs: []net.Addr{ipNet("169.254.10.1")}}},
			want:   nil,
		},
		{
			name:   "loopback ipv4 skipped",
			ifaces: []netIface{{Up: true, Addrs: []net.Addr{ipNet("127.0.0.1")}}},
			want:   nil,
		},
		{
			name:   "private ranked before public",
			ifaces: []netIface{{Up: true, Addrs: []net.Addr{ipNet("8.8.8.8"), ipNet("10.0.0.2")}}},
			want:   []string{"10.0.0.2", "8.8.8.8"},
		},
		{
			name: "across interfaces, private first",
			ifaces: []netIface{
				{Up: true, Addrs: []net.Addr{ipNet("203.0.113.7")}},
				{Up: false, Addrs: []net.Addr{ipNet("192.168.9.9")}},
				{Up: true, Addrs: []net.Addr{ipNet("172.16.0.4")}},
			},
			want: []string{"172.16.0.4", "203.0.113.7"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickLANIPs(tt.ifaces)
			var gotStr []string
			for _, ip := range got {
				gotStr = append(gotStr, ip.String())
			}
			if len(gotStr) != len(tt.want) {
				t.Fatalf("pickLANIPs = %v, want %v", gotStr, tt.want)
			}
			for i := range tt.want {
				if gotStr[i] != tt.want[i] {
					t.Fatalf("pickLANIPs = %v, want %v", gotStr, tt.want)
				}
			}
		})
	}
}

func TestComposePairPayload(t *testing.T) {
	p, raw, err := composePairPayload("192.168.1.5", 8765, "deadbeef")
	if err != nil {
		t.Fatalf("composePairPayload: %v", err)
	}
	if p.V != 1 {
		t.Fatalf("V = %d, want 1", p.V)
	}
	if p.URL != "http://192.168.1.5:8765" {
		t.Fatalf("URL = %q, want http://192.168.1.5:8765", p.URL)
	}
	if p.Token != "deadbeef" {
		t.Fatalf("Token = %q, want deadbeef", p.Token)
	}
	// Compact JSON in declaration order — the shape the QR encodes.
	want := `{"v":1,"url":"http://192.168.1.5:8765","token":"deadbeef"}`
	if raw != want {
		t.Fatalf("payload = %q, want %q", raw, want)
	}
}

func TestComposePairPayloadTailnetLabel(t *testing.T) {
	p, _, err := composePairPayload("yasyf-forge-studio", 61118, "deadbeef")
	if err != nil {
		t.Fatalf("composePairPayload: %v", err)
	}
	if p.URL != "http://yasyf-forge-studio:61118" {
		t.Fatalf("URL = %q, want http://yasyf-forge-studio:61118", p.URL)
	}
}

func TestPairHost(t *testing.T) {
	lan := []net.IP{net.ParseIP("192.168.4.21").To4(), net.ParseIP("10.0.0.2").To4()}
	tests := []struct {
		name    string
		label   string
		ips     []net.IP
		want    string
		wantErr bool
	}{
		{name: "tailnet label wins over LAN", label: "studio", ips: lan, want: "studio"},
		{name: "tailnet label without LAN", label: "studio", want: "studio"},
		{name: "first LAN address without tailnet", ips: lan, want: "192.168.4.21"},
		{name: "neither", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pairHost(tt.label, tt.ips)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pairHost err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("pairHost = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPairBindOpensTheLANOnlyWithoutATailnet(t *testing.T) {
	if got := pairBind("studio"); got != bindLoopback {
		t.Errorf("pairBind with a tailnet label = %q, want %q", got, bindLoopback)
	}
	if got := pairBind(""); got != bindLAN {
		t.Errorf("pairBind with no tailnet = %q, want %q", got, bindLAN)
	}
}

func tempPaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	p := paths.Paths{App: ".cc-present-test"}
	if err := os.MkdirAll(p.StateDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnsureDaemonOnATailnet(t *testing.T) {
	const onPrimary = `{"port":61118,"bind":"127.0.0.1","extra_addrs":["100.113.56.52:61118","[fd7a:115c:a1e0::1]:61118"]}`
	const apart = `{"port":61118,"bind":"127.0.0.1","extra_addrs":["100.113.56.52:52668"]}`
	const none = `{"port":61118,"bind":"127.0.0.1"}`
	tests := []struct {
		name         string
		before       string
		afterRestart string
		wantRestart  bool
		wantErr      bool
	}{
		{name: "listener on the primary port needs no restart", before: onPrimary, wantRestart: false},
		{name: "a first start's split ports converge", before: apart, afterRestart: onPrimary, wantRestart: true},
		{name: "a daemon from before the mesh picks up its listeners", before: none, afterRestart: onPrimary, wantRestart: true},
		{name: "still no listener after the restart refuses to print a dead code", before: none, afterRestart: none, wantRestart: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tempPaths(t)
			write := func(body string) {
				t.Helper()
				if err := os.WriteFile(p.HTTPInfoPath(), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(tt.before)
			restarted := false
			d := cmd.Deps{
				Paths:         p,
				EnsureCurrent: func(context.Context) error { return nil },
				Stop: func(context.Context) error {
					restarted = true
					write(tt.afterRestart)
					return nil
				},
			}
			info, err := ensureDaemon(context.Background(), d, bindLoopback, false, true)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ensureDaemon err = %v, wantErr %v", err, tt.wantErr)
			}
			if restarted != tt.wantRestart {
				t.Fatalf("restarted = %v, want %v", restarted, tt.wantRestart)
			}
			if !tt.wantErr && !tailnetOnPrimaryPort(info) {
				t.Fatalf("info = %+v has no tailnet listener on the primary port", info)
			}
		})
	}
}

func TestEnsureDaemonOffATailnetIgnoresListeners(t *testing.T) {
	p := tempPaths(t)
	if err := os.WriteFile(p.HTTPInfoPath(), []byte(`{"port":61118,"bind":"0.0.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := cmd.Deps{
		Paths:         p,
		EnsureCurrent: func(context.Context) error { return nil },
		Stop: func(context.Context) error {
			t.Fatal("restarted a daemon already on the wanted bind")
			return nil
		},
	}
	if _, err := ensureDaemon(context.Background(), d, bindLAN, false, false); err != nil {
		t.Fatalf("ensureDaemon: %v", err)
	}
}

func TestEffectiveBind(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", bindLoopback},
		{"0.0.0.0", "0.0.0.0"},
		{"127.0.0.1", "127.0.0.1"},
	}
	for _, tt := range tests {
		if got := effectiveBind(tt.in); got != tt.want {
			t.Errorf("effectiveBind(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRestartDaemonStopsBeforeEnsureCurrent(t *testing.T) {
	var calls []string
	d := cmd.Deps{
		Stop: func(context.Context) error {
			calls = append(calls, "stop")
			return nil
		},
		EnsureCurrent: func(context.Context) error {
			calls = append(calls, "ensure")
			return nil
		},
	}
	if err := restartDaemon(context.Background(), d); err != nil {
		t.Fatalf("restartDaemon: %v", err)
	}
	if want := []string{"stop", "ensure"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}
