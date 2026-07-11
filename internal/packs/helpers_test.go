package packs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTreeInto(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeTreeInto(t, dir, files)
	return dir
}

func manifestFor(name string) string {
	return fmt.Sprintf(`host_api = 1
name = %q
version = "0.1.0"
description = "d"
entry = "dist/pack.js"

[blocks.callout]
description = "c"
schema = "schema/callout.json"
examples = ["examples/callout.json"]
`, name)
}

func packFiles(name string) map[string]string {
	return map[string]string{
		"cc-present.toml":       manifestFor(name),
		"dist/pack.js":          "export default {}",
		"schema/callout.json":   `{"type":"object"}`,
		"examples/callout.json": fmt.Sprintf(`{"id":"c","type":%q}`, name+".callout"),
	}
}

func mkPluginInstall(t *testing.T, name string) string {
	t.Helper()
	installPath := t.TempDir()
	writeTreeInto(t, filepath.Join(installPath, ".claude", "components"), packFiles(name))
	return installPath
}

func writeInstalledPlugins(t *testing.T, configDir string, plugins map[string][]string) {
	t.Helper()
	pl := map[string]any{}
	for key, paths := range plugins {
		recs := make([]map[string]string, 0, len(paths))
		for _, p := range paths {
			recs = append(recs, map[string]string{"scope": "project", "installPath": p})
		}
		pl[key] = recs
	}
	data, err := json.Marshal(map[string]any{"version": 2, "plugins": pl})
	if err != nil {
		t.Fatal(err)
	}
	writeTreeInto(t, configDir, map[string]string{
		filepath.Join("plugins", "installed_plugins.json"): string(data),
	})
}

func hasDropReason(r *Registry, dir, substr string) bool {
	for _, d := range r.Dropped {
		if d.Dir == dir && strings.Contains(d.Reason, substr) {
			return true
		}
	}
	return false
}

func packNames(r *Registry) []string {
	packs := r.Packs()
	out := make([]string, 0, len(packs))
	for _, p := range packs {
		out = append(out, p.Name)
	}
	return out
}
