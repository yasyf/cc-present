package packs

import (
	"context"
	"sync"
	"time"
)

const scanTTL = 2 * time.Second

// Loader scans for packs eagerly at construction, then re-scans on access after a
// short TTL so installing a pack and pushing a doc that uses it just works. A
// source pack whose bundle is stale builds in the background and reads as
// dropped with reason "building" until it finishes. It is safe for concurrent use.
type Loader struct {
	devDirs   []string
	disabled  []string
	configDir string
	ttl       time.Duration
	builds    *backgroundBuilds

	mu        sync.Mutex
	current   *Registry
	scannedAt time.Time
}

// Load performs a single scan over the given dev dirs and disabled pack names
// and returns the resulting registry — the one-shot form the CLI uses once per
// invocation, where a long-lived loader with its re-scan TTL earns nothing. It
// waits for any stale source pack's build.
func Load(ctx context.Context, devDirs, disabled []string) *Registry {
	roots, dropped := discoverRoots(devDirs, ClaudeConfigDir())
	return buildRegistry(roots, dropped, disabled, syncBuilds{ctx: ctx})
}

// NewLoader builds a loader over the given dev dirs and disabled pack names and
// performs the first scan; ctx bounds its background builds.
func NewLoader(ctx context.Context, devDirs, disabled []string) *Loader {
	l := &Loader{
		devDirs:   append([]string(nil), devDirs...),
		disabled:  append([]string(nil), disabled...),
		configDir: ClaudeConfigDir(),
		ttl:       scanTTL,
		builds:    newBackgroundBuilds(ctx),
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
	l.current = buildRegistry(roots, dropped, l.disabled, l.builds)
	l.scannedAt = time.Now()
}
