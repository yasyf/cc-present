package packs

import (
	"sync"
	"time"
)

const scanTTL = 2 * time.Second

// Loader scans for packs eagerly at construction, then re-scans on access after a
// short TTL so installing a pack and pushing a doc that uses it just works. It is
// safe for concurrent use.
type Loader struct {
	devDirs   []string
	disabled  []string
	configDir string
	ttl       time.Duration

	mu        sync.Mutex
	current   *Registry
	scannedAt time.Time
}

// Load performs a single scan over the given dev dirs and disabled pack names
// and returns the resulting registry — the one-shot form the CLI uses once per
// invocation, where a long-lived loader with its re-scan TTL earns nothing.
func Load(devDirs, disabled []string) *Registry {
	return NewLoader(devDirs, disabled).Current()
}

// NewLoader builds a loader over the given dev dirs and disabled pack names and
// performs the first scan.
func NewLoader(devDirs, disabled []string) *Loader {
	l := &Loader{
		devDirs:   append([]string(nil), devDirs...),
		disabled:  append([]string(nil), disabled...),
		configDir: ClaudeConfigDir(),
		ttl:       scanTTL,
	}
	l.mu.Lock()
	l.scanLocked()
	l.mu.Unlock()
	return l
}

// Current returns the latest registry, re-scanning when the TTL has elapsed.
func (l *Loader) Current() *Registry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.scannedAt) >= l.ttl {
		l.scanLocked()
	}
	return l.current
}

func (l *Loader) scanLocked() {
	roots, dropped := discoverRoots(l.devDirs, l.configDir)
	l.current = buildRegistry(roots, dropped, l.disabled)
	l.scannedAt = time.Now()
}
