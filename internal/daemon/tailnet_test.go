package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"github.com/yasyf/daemonkit/paths"
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

func TestTailnetListenersUsesHandshakeHint(t *testing.T) {
	p := testPaths(t)

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hint := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	writeHandshake(t, p, hint)

	factories := tailnetListeners(p, "", []netip.Addr{netip.MustParseAddr("127.0.0.1")})
	if len(factories) != 1 {
		t.Fatalf("len(factories) = %d, want 1", len(factories))
	}
	ln, err := factories[0](context.Background())
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if got := ln.Addr().(*net.TCPAddr).Port; got != hint {
		t.Errorf("bound port = %d, want handshake hint %d", got, hint)
	}
}

func TestTailnetURLs(t *testing.T) {
	tests := []struct {
		name  string
		dns   string
		addrs []string
		want  []string
	}{
		{
			"dns dedupes v4 and v6 legs sharing a port",
			"host.ts.net",
			[]string{"100.1.2.3:8080", "[fd7a:115c:a1e0::1]:8080"},
			[]string{"http://host.ts.net:8080/p/board"},
		},
		{
			"dns emits one url per distinct port",
			"host.ts.net",
			[]string{"100.1.2.3:8080", "100.1.2.3:9090"},
			[]string{"http://host.ts.net:8080/p/board", "http://host.ts.net:9090/p/board"},
		},
		{
			"no dns falls back to raw ips, bracketing v6",
			"",
			[]string{"100.1.2.3:8080", "[fd7a:115c:a1e0::1]:9090"},
			[]string{"http://100.1.2.3:8080/p/board", "http://[fd7a:115c:a1e0::1]:9090/p/board"},
		},
		{"empty addrs with dns yields nil", "host.ts.net", nil, nil},
		{"empty addrs and no dns yields nil", "", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tailnetURLs(tt.dns, tt.addrs, "board")
			if !slices.Equal(got, tt.want) {
				t.Fatalf("tailnetURLs(%q, %v) = %v, want %v", tt.dns, tt.addrs, got, tt.want)
			}
		})
	}
}

func TestDisplayURLs(t *testing.T) {
	self := []netip.Addr{netip.MustParseAddr("100.64.0.7"), netip.MustParseAddr("fd7a::7")}
	tests := []struct {
		name     string
		extra    []string
		loopback bool
		want     []string
	}{
		{"extra legs win", []string{"100.64.0.7:62520"}, true, []string{"http://ts.example.ts.net:62520/p/s"}},
		{"loopback bind without legs", nil, true, nil},
		{"wildcard bind serves the primary port", nil, false, []string{"http://ts.example.ts.net:8080/p/s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayURLs("ts.example.ts.net", tt.extra, self, tt.loopback, 8080, "s")
			if !slices.Equal(got, tt.want) {
				t.Errorf("displayURLs() = %v, want %v", got, tt.want)
			}
		})
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
