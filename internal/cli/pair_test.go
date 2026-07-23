package cli

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/yasyf/cc-interact/cmd"
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
	p, raw, err := composePairPayload(net.ParseIP("192.168.1.5"), 8765, "deadbeef")
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
