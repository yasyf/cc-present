package packs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

type tier int

const (
	tierPlugin tier = iota
	tierDev
)

// packRoot is a candidate pack directory tagged with its discovery tier. Dev
// dirs are emitted unconditionally; plugin dirs only when their manifest exists.
type packRoot struct {
	dir  string
	tier tier
}

type installedPlugins struct {
	Version int                          `json:"version"`
	Plugins map[string][]installedRecord `json:"plugins"`
}

type installedRecord struct {
	Scope       string `json:"scope"`
	InstallPath string `json:"installPath"`
}

// ClaudeConfigDir returns the Claude config directory: $CLAUDE_CONFIG_DIR when
// set, else ~/.claude.
func ClaudeConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// discoverRoots lists pack roots: the configured dev dirs in order, then the
// installed plugins whose components dir holds a manifest, sorted by plugin key.
func discoverRoots(devDirs []string, configDir string) []packRoot {
	roots := make([]packRoot, 0, len(devDirs))
	for _, d := range devDirs {
		roots = append(roots, packRoot{dir: d, tier: tierDev})
	}
	return append(roots, pluginRoots(configDir)...)
}

func pluginRoots(configDir string) []packRoot {
	path := filepath.Join(configDir, "plugins", "installed_plugins.json")
	//nolint:gosec // G304: reading the Claude config's installed_plugins.json is discovery's purpose.
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		slog.Warn("read installed plugins", "path", path, "err", err)
		return nil
	}
	var ip installedPlugins
	if err := json.Unmarshal(data, &ip); err != nil {
		slog.Warn("parse installed plugins", "path", path, "err", err)
		return nil
	}
	if ip.Version != 2 {
		slog.Warn("unsupported installed_plugins.json version", "path", path, "version", ip.Version)
		return nil
	}
	keys := make([]string, 0, len(ip.Plugins))
	for k := range ip.Plugins {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	seen := map[string]bool{}
	var roots []packRoot
	for _, k := range keys {
		installPaths := make([]string, 0, len(ip.Plugins[k]))
		for _, rec := range ip.Plugins[k] {
			installPaths = append(installPaths, rec.InstallPath)
		}
		sort.Strings(installPaths)
		for _, p := range installPaths {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			components := filepath.Join(p, ".claude", "components")
			if !fileExists(filepath.Join(components, ManifestName)) {
				continue
			}
			roots = append(roots, packRoot{dir: components, tier: tierPlugin})
		}
	}
	return roots
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
