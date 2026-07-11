package packs

import (
	"path/filepath"
	"testing"
)

func TestDiscoverDisableList(t *testing.T) {
	devDir := writeTree(t, packFiles("foo"))
	reg := buildRegistry(discoverRoots([]string{devDir}, t.TempDir()), []string{"foo"})
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

	reg := buildRegistry(discoverRoots([]string{devDir}, configDir), nil)
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
	reg := buildRegistry(discoverRoots([]string{dev1, dev2}, t.TempDir()), nil)
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

	reg := buildRegistry(discoverRoots([]string{good, bad}, t.TempDir()), nil)
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

	reg := buildRegistry(discoverRoots(nil, configDir), nil)
	if names := packNames(reg); len(names) != 1 || names[0] != "plug" {
		t.Fatalf("packs = %v, want [plug]", names)
	}
}

func TestDiscoverPluginWithoutComponentsSkipped(t *testing.T) {
	install := t.TempDir() // no .claude/components
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{"empty@mkt": {install}})

	roots := discoverRoots(nil, configDir)
	if len(roots) != 0 {
		t.Fatalf("roots = %v, want none (silently skipped)", roots)
	}
}

func TestDiscoverNoPluginsFile(t *testing.T) {
	if roots := discoverRoots(nil, t.TempDir()); len(roots) != 0 {
		t.Fatalf("roots = %v, want none", roots)
	}
}

func TestDiscoverDedupeByInstallPath(t *testing.T) {
	install := mkPluginInstall(t, "shared")
	configDir := t.TempDir()
	writeInstalledPlugins(t, configDir, map[string][]string{
		"a@mkt": {install},
		"b@mkt": {install},
	})
	if roots := discoverRoots(nil, configDir); len(roots) != 1 {
		t.Fatalf("roots = %d, want 1 (deduped by installPath)", len(roots))
	}
}
