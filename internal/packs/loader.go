package packs

import (
	"context"
	"sync"
	"time"
)

const scanTTL = 2 * time.Second

// Loader scans for packs eagerly at construction, then re-scans on access after a
// short TTL so installing a pack, or adding its directory to the config, and
// pushing a doc that uses it just works. A
// source pack whose bundle is stale builds in the background and reads as
// dropped with reason "building" until it finishes; Settled waits for it instead.
// It is safe for concurrent use.
type Loader struct {
	dirs      Dirs
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

// Dirs returns the dev dirs and disabled pack names a scan uses.
type Dirs func() (devDirs, disabled []string)

// NewLoader builds a loader over fixed dev dirs and disabled pack names and
// performs the first scan; ctx bounds its background builds.
func NewLoader(ctx context.Context, devDirs, disabled []string) *Loader {
	devDirs = append([]string(nil), devDirs...)
	disabled = append([]string(nil), disabled...)
	return NewLoaderFrom(ctx, func() ([]string, []string) { return devDirs, disabled })
}

// NewLoaderFrom builds a loader that asks dirs for the dev dirs and disabled
// pack names on every scan, and performs the first scan.
func NewLoaderFrom(ctx context.Context, dirs Dirs) *Loader {
	l := &Loader{
		dirs:      dirs,
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

// Settled returns the latest registry for an authoring request, waiting for any
// pack the current scan dropped as building: it re-scans with synchronous builds,
// which queue on the build lock behind the running build. ctx bounds those builds.
func (l *Loader) Settled(ctx context.Context) *Registry {
	if reg := l.Current(); !reg.building {
		return reg
	}
	devDirs, disabled := l.dirs()
	roots, dropped := discoverRoots(devDirs, l.configDir)
	reg := buildRegistry(roots, dropped, disabled, syncBuilds{ctx: ctx})
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current, l.scannedAt = reg, time.Now()
	return reg
}

func (l *Loader) scanLocked() {
	devDirs, disabled := l.dirs()
	roots, dropped := discoverRoots(devDirs, l.configDir)
	l.current = buildRegistry(roots, dropped, disabled, l.builds)
	l.scannedAt = time.Now()
}
