package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
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
