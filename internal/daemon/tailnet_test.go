package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/cc-interact/paths"
)

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	p := paths.Paths{App: ".cc-present-test"}
	if err := p.EnsureStateDir(); err != nil {
		t.Fatalf("EnsureStateDir: %v", err)
	}
	return p
}

func writeHandshake(t *testing.T, p paths.Paths, port int) {
	t.Helper()
	b, err := json.Marshal(ccd.HTTPInfo{Port: port})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.HTTPInfoPath(), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

type stubListener struct{ addr string }

func (s stubListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (s stubListener) Close() error              { return nil }
func (s stubListener) Addr() net.Addr            { return stubAddr(s.addr) }

// stubListen records every requested address and fails the ones in reject.
func stubListen(requested *[]string, reject func(string) bool) listenFunc {
	return func(address string) (net.Listener, error) {
		*requested = append(*requested, address)
		if reject != nil && reject(address) {
			return nil, errors.New("stub: rejected")
		}
		return stubListener{addr: address}, nil
	}
}

func TestBindTailnet(t *testing.T) {
	v4 := netip.MustParseAddr("100.88.252.58")
	v6 := netip.MustParseAddr("fd7a:115c:a1e0::6d33:fc3c")
	tests := []struct {
		name      string
		bind      string
		addrs     []netip.Addr
		hint      int
		reject    func(string) bool
		wantAddrs []string
	}{
		{"non-loopback bind yields none", "0.0.0.0", []netip.Addr{v4}, 0, nil, nil},
		{"lan bind yields none", "192.168.1.9", []netip.Addr{v4}, 0, nil, nil},
		{"no addrs yields none", "", nil, 0, nil, nil},
		{
			"one listener per addr on ephemeral ports",
			"",
			[]netip.Addr{v4, v6},
			0, nil,
			[]string{"100.88.252.58:0", "[fd7a:115c:a1e0::6d33:fc3c]:0"},
		},
		{
			"hint tried first",
			"",
			[]netip.Addr{v4},
			4321, nil,
			[]string{"100.88.252.58:4321"},
		},
		{
			"hint failure falls back to ephemeral",
			"",
			[]netip.Addr{v4},
			4321,
			func(a string) bool { return strings.HasSuffix(a, ":4321") },
			[]string{"100.88.252.58:0"},
		},
		{
			"unbindable addr skipped, rest survive",
			"",
			[]netip.Addr{v6, v4},
			0,
			func(a string) bool { return strings.HasPrefix(a, "[fd7a") },
			[]string{"100.88.252.58:0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPaths(t)
			if tt.hint != 0 {
				writeHandshake(t, p, tt.hint)
			}
			var requested []string
			factories := bindTailnet(stubListen(&requested, tt.reject), p, tt.bind, tt.addrs)
			if got, want := len(factories), len(tt.wantAddrs); got != want {
				t.Fatalf("len(factories) = %d, want %d (requested: %v)", got, want, requested)
			}
			for i, factory := range factories {
				ln, err := factory(context.Background())
				if err != nil {
					t.Fatalf("factory %d: %v", i, err)
				}
				if got := ln.Addr().String(); got != tt.wantAddrs[i] {
					t.Errorf("listener %d bound %q, want %q", i, got, tt.wantAddrs[i])
				}
			}
		})
	}
}

func TestTailnetListenersRealBindFallback(t *testing.T) {
	p := testPaths(t)

	holder, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Close() }()
	occupied := holder.Addr().(*net.TCPAddr).Port
	writeHandshake(t, p, occupied)

	factories := tailnetListeners(p, "", []netip.Addr{netip.MustParseAddr("127.0.0.1")})
	if len(factories) != 1 {
		t.Fatalf("len(factories) = %d, want 1", len(factories))
	}
	ln, err := factories[0](context.Background())
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if got := ln.Addr().(*net.TCPAddr).Port; got == occupied || got == 0 {
		t.Errorf("bound port = %d, want a live ephemeral port != %d", got, occupied)
	}
}

func TestLastHTTPPort(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, p paths.Paths)
		want  uint16
	}{
		{"absent handshake", func(*testing.T, paths.Paths) {}, 0},
		{"corrupt handshake", func(t *testing.T, p paths.Paths) {
			if err := os.WriteFile(filepath.Join(p.StateDir(), "http.json"), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, 0},
		{"out-of-range port", func(t *testing.T, p paths.Paths) { writeHandshake(t, p, 99999) }, 0},
		{"negative port", func(t *testing.T, p paths.Paths) { writeHandshake(t, p, -1) }, 0},
		{"valid port", func(t *testing.T, p paths.Paths) { writeHandshake(t, p, 4321) }, 4321},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPaths(t)
			tt.setup(t, p)
			if got := lastHTTPPort(p); got != tt.want {
				t.Errorf("lastHTTPPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
