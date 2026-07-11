package packs

import (
	"path/filepath"
	"testing"
)

func TestDiscoverDisableList(t *testing.T) {
	devDir := writeTree(t, packFiles("foo"))
	roots, dropped := discoverRoots([]string{devDir}, t.TempDir())
	reg := buildRegistry(roots, dropped, []string{"foo"})
	if names := packNames(reg); len(names) != 0 {
		t.Fatalf("packs = %v, want none", names)
	}
	if !hasDropReason(reg, devDir, "disabled") {
		t.Fatalf("dropped = %+v, want foo disabled", reg.Dropped)
	}
}

func TestDiscoverDevShadowsInstalled(t *testing.T) {
	devDir := writeTree(t, packFiles("foo"))
	install := mkPluginInstall(t, "foo")
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{"foo@mkt": {install}})

	roots, dropped := discoverRoots([]string{devDir}, configDir)
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 1 || names[0] != "foo" {
		t.Fatalf("packs = %v, want [foo]", names)
	}
	if reg.Packs()[0].Dir != devDir {
		t.Fatalf("winner dir = %q, want dev dir %q", reg.Packs()[0].Dir, devDir)
	}
	components := filepath.Join(install, ".claude", "components")
	if !hasDropReason(reg, components, "shadowed by dev dir") {
		t.Fatalf("dropped = %+v, want plugin shadowed", reg.Dropped)
	}
}

func TestDiscoverSameTierDupes(t *testing.T) {
	dev1 := writeTree(t, packFiles("dup"))
	dev2 := writeTree(t, packFiles("dup"))
	roots, dropped := discoverRoots([]string{dev1, dev2}, t.TempDir())
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 0 {
		t.Fatalf("packs = %v, want none", names)
	}
	if !hasDropReason(reg, dev1, "duplicate") || !hasDropReason(reg, dev2, "duplicate") {
		t.Fatalf("dropped = %+v, want both dup dropped", reg.Dropped)
	}
}

func TestDiscoverRottedAmongTwo(t *testing.T) {
	good := writeTree(t, packFiles("good"))
	badFiles := packFiles("bad")
	delete(badFiles, "schema/callout.json")
	bad := writeTree(t, badFiles)

	roots, dropped := discoverRoots([]string{good, bad}, t.TempDir())
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 1 || names[0] != "good" {
		t.Fatalf("packs = %v, want [good]", names)
	}
	if len(reg.Dropped) != 1 || reg.Dropped[0].Dir != bad {
		t.Fatalf("dropped = %+v, want [%s]", reg.Dropped, bad)
	}
}

func TestDiscoverPluginPack(t *testing.T) {
	install := mkPluginInstall(t, "plug")
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{"plug@mkt": {install}})

	roots, dropped := discoverRoots(nil, configDir)
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 1 || names[0] != "plug" {
		t.Fatalf("packs = %v, want [plug]", names)
	}
}

func TestDiscoverPluginWithoutComponentsSkipped(t *testing.T) {
	install := t.TempDir() // no .claude/components
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{"empty@mkt": {install}})

	roots, _ := discoverRoots(nil, configDir)
	if len(roots) != 0 {
		t.Fatalf("roots = %v, want none (silently skipped)", roots)
	}
}

func TestDiscoverNoPluginsFile(t *testing.T) {
	roots, dropped := discoverRoots(nil, t.TempDir())
	if len(roots) != 0 || len(dropped) != 0 {
		t.Fatalf("roots = %v, dropped = %v, want both empty", roots, dropped)
	}
}

func TestDiscoverMalformedPluginsFile(t *testing.T) {
	configDir := t.TempDir()
	writeTreeInto(t, configDir, map[string]string{
		filepath.Join("plugins", "installed_plugins.json"): "{not valid json",
	})
	path := filepath.Join(configDir, "plugins", "installed_plugins.json")

	roots, dropped := discoverRoots(nil, configDir)
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 0 {
		t.Fatalf("packs = %v, want none", names)
	}
	if !hasDropReason(reg, path, "parse error") {
		t.Fatalf("dropped = %+v, want parse error naming %s", reg.Dropped, path)
	}
}

func TestDiscoverUnsupportedPluginsVersion(t *testing.T) {
	configDir := t.TempDir()
	writeTreeInto(t, configDir, map[string]string{
		filepath.Join("plugins", "installed_plugins.json"): `{"version":3,"plugins":{}}`,
	})
	path := filepath.Join(configDir, "plugins", "installed_plugins.json")

	roots, dropped := discoverRoots(nil, configDir)
	reg := buildRegistry(roots, dropped, nil)
	if names := packNames(reg); len(names) != 0 {
		t.Fatalf("packs = %v, want none", names)
	}
	if !hasDropReason(reg, path, "version 3 unsupported") {
		t.Fatalf("dropped = %+v, want version 3 unsupported naming %s", reg.Dropped, path)
	}
}

func TestDiscoverDedupeByInstallPath(t *testing.T) {
	install := mkPluginInstall(t, "shared")
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{
		"a@mkt": {install},
		"b@mkt": {install},
	})
	roots, _ := discoverRoots(nil, configDir)
	if len(roots) != 1 {
		t.Fatalf("roots = %d, want 1 (deduped by installPath)", len(roots))
	}
}
