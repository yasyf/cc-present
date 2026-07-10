package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withHome points HOME at a fresh temp dir so Paths() resolves ~/.cc-present
// under it, isolating each test from the real state directory.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestReadConfigAbsentIsZero(t *testing.T) {
	withHome(t)
	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.Bind != "" {
		t.Fatalf("absent config Bind = %q, want empty (loopback default)", cfg.Bind)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	withHome(t)
	if err := WriteConfig(Config{Bind: "0.0.0.0"}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.Bind != "0.0.0.0" {
		t.Fatalf("round-trip Bind = %q, want 0.0.0.0", cfg.Bind)
	}
	info, err := os.Stat(ConfigPath())
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config perms = %o, want 600", perm)
	}
}

func TestReadConfigCorruptFailsLoud(t *testing.T) {
	withHome(t)
	if err := Paths().EnsureStateDir(); err != nil {
		t.Fatalf("ensure state dir: %v", err)
	}
	if err := os.WriteFile(ConfigPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt config: %v", err)
	}
	if _, err := ReadConfig(); err == nil {
		t.Fatal("ReadConfig on corrupt file returned nil error, want failure")
	}
}

func TestReadTokenAbsentIsEmpty(t *testing.T) {
	withHome(t)
	tok, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if tok != "" {
		t.Fatalf("absent token = %q, want empty (auth off)", tok)
	}
}

func TestEnsureTokenGeneratesAndIsIdempotent(t *testing.T) {
	withHome(t)
	first, err := EnsureToken()
	if err != nil {
		t.Fatalf("EnsureToken: %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("token length = %d, want 64 (32 bytes hex)", len(first))
	}
	info, err := os.Stat(TokenPath())
	if err != nil {
		t.Fatalf("stat token: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("token perms = %o, want 600", perm)
	}
	second, err := EnsureToken()
	if err != nil {
		t.Fatalf("EnsureToken second: %v", err)
	}
	if second != first {
		t.Fatalf("EnsureToken not idempotent: %q then %q", first, second)
	}
	got, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got != first {
		t.Fatalf("ReadToken = %q, want %q", got, first)
	}
}

func TestResetTokenChangesIt(t *testing.T) {
	withHome(t)
	first, err := EnsureToken()
	if err != nil {
		t.Fatalf("EnsureToken: %v", err)
	}
	reset, err := ResetToken()
	if err != nil {
		t.Fatalf("ResetToken: %v", err)
	}
	if reset == first {
		t.Fatalf("ResetToken returned the same token %q", reset)
	}
	if len(reset) != 64 {
		t.Fatalf("reset token length = %d, want 64", len(reset))
	}
	got, err := ReadToken()
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got != reset {
		t.Fatalf("ReadToken = %q, want reset token %q", got, reset)
	}
}

// TestConfigPathUnderStateDir pins the config and token to ~/.cc-present so a
// stray relocation is caught.
func TestConfigPathUnderStateDir(t *testing.T) {
	home := withHome(t)
	wantDir := filepath.Join(home, ".cc-present")
	if got := filepath.Dir(ConfigPath()); got != wantDir {
		t.Fatalf("config dir = %q, want %q", got, wantDir)
	}
	if got := filepath.Dir(TokenPath()); got != wantDir {
		t.Fatalf("token dir = %q, want %q", got, wantDir)
	}
	if filepath.Base(ConfigPath()) != "config.json" {
		t.Fatalf("config base = %q, want config.json", filepath.Base(ConfigPath()))
	}
	// A written config parses back as JSON, catching a non-JSON encoder swap.
	if err := WriteConfig(Config{Bind: "127.0.0.1"}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	b, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("config is not JSON: %v", err)
	}
}
