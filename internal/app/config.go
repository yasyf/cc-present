package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Config is cc-present's optional host configuration at ~/.cc-present/config.json.
// The zero value is the loopback-only default, so an absent file yields it.
type Config struct {
	// Bind is the HTTP plane's bind address. Empty means 127.0.0.1 (loopback
	// only); "0.0.0.0" exposes the plane to the LAN.
	Bind string `json:"bind,omitempty"`
}

// ConfigPath is the host config file (~/.cc-present/config.json).
func ConfigPath() string { return filepath.Join(Paths().StateDir(), "config.json") }

// TokenPath is the LAN bearer-token file (~/.cc-present/token).
func TokenPath() string { return filepath.Join(Paths().StateDir(), "token") }

// ReadConfig loads the host config. An absent file is the loopback-only zero
// value; a present-but-corrupt file fails loudly.
func ReadConfig() (Config, error) {
	b, err := os.ReadFile(ConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", ConfigPath(), err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", ConfigPath(), err)
	}
	return c, nil
}

// WriteConfig persists the host config (0600), creating the state dir if needed.
func WriteConfig(c Config) error {
	if err := Paths().EnsureStateDir(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(ConfigPath(), b, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", ConfigPath(), err)
	}
	return nil
}

// ReadToken returns the LAN bearer token, or "" when no token file exists. An
// empty token leaves the HTTP plane's auth off — exactly today's behavior.
func ReadToken() (string, error) {
	b, err := os.ReadFile(TokenPath())
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read token %q: %w", TokenPath(), err)
	}
	return strings.TrimSpace(string(b)), nil
}

// EnsureToken returns the LAN bearer token, generating and persisting a fresh
// one the first time. It is idempotent: a second call returns the same token.
func EnsureToken() (string, error) {
	tok, err := ReadToken()
	if err != nil {
		return "", err
	}
	if tok != "" {
		return tok, nil
	}
	return writeToken()
}

// ResetToken generates a fresh token, overwriting any existing one, and returns
// it. The daemon must be restarted to pick the new token up.
func ResetToken() (string, error) { return writeToken() }

// writeToken generates 32 crypto-random bytes as hex and writes them to the
// token file (0600), creating the state dir if needed.
func writeToken() (string, error) {
	if err := Paths().EnsureStateDir(); err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	tok := hex.EncodeToString(buf)
	if err := os.WriteFile(TokenPath(), []byte(tok), 0o600); err != nil {
		return "", fmt.Errorf("write token %q: %w", TokenPath(), err)
	}
	return tok, nil
}
