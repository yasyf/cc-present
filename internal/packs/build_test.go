package packs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yasyf/cc-present/examples"
)

var buildEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func sourcePackFiles(build string) map[string]string {
	files := packFiles("src-pack")
	delete(files, "dist/pack.js")
	files["package.json"] = `{"name":"src-pack","private":true,"scripts":{"build":` + build + `}}`
	files["src/pack.ts"] = "export default { hostApi: 1, blocks: {} };\n"
	return files
}

func setModTimes(t *testing.T, dir string, at time.Time, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		if err := os.Chtimes(filepath.Join(dir, rel), at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func mustDigest(t *testing.T, dir string) string {
	t.Helper()
	files, err := listSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := digestSources(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func requireBun(t *testing.T) {
	t.Helper()
	if _, err := findBun(); err != nil {
		t.Skipf("bun unavailable: %v", err)
	}
}

func TestDigestSources(t *testing.T) {
	base := sourcePackFiles(`"true"`)
	baseDigest := mustDigest(t, writeTree(t, base))

	tests := []struct {
		name    string
		mutate  func(map[string]string)
		changed bool
	}{
		{"identical tree", func(map[string]string) {}, false},
		{"src edit", func(f map[string]string) { f["src/pack.ts"] += "// edit\n" }, true},
		{"schema edit", func(f map[string]string) { f["schema/callout.json"] = `{"type":"string"}` }, true},
		{"package.json edit", func(f map[string]string) { f["package.json"] = `{"name":"other"}` }, true},
		{"tsconfig added", func(f map[string]string) { f["tsconfig.json"] = "{}" }, true},
		{"vite config added", func(f map[string]string) { f["vite.config.ts"] = "export default {}" }, true},
		{"src rename", func(f map[string]string) {
			f["src/entry.ts"] = f["src/pack.ts"]
			delete(f, "src/pack.ts")
		}, true},
		{"nested src file added", func(f map[string]string) { f["src/host/react.ts"] = "export {}" }, true},
		{"example edit ignored", func(f map[string]string) { f["examples/callout.json"] = `{}` }, false},
		{"manifest edit ignored", func(f map[string]string) { f["cc-present.toml"] += "\n" }, false},
		{"bundle edit ignored", func(f map[string]string) { f["dist/pack.js"] = "export default {}" }, false},
		{"readme added ignored", func(f map[string]string) { f["README.md"] = "# hi" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{}
			for k, v := range base {
				files[k] = v
			}
			tt.mutate(files)
			got := mustDigest(t, writeTree(t, files))
			if (got != baseDigest) != tt.changed {
				t.Fatalf("digest changed = %v, want %v", got != baseDigest, tt.changed)
			}
		})
	}
}

func TestListSources(t *testing.T) {
	files := sourcePackFiles(`"true"`)
	files["src/host/react.ts"] = "export {}"
	files["tsconfig.json"] = "{}"
	files["dist/pack.js"] = "x"
	files["node_modules/vite/index.js"] = "x"
	got, err := listSources(writeTree(t, files))
	if err != nil {
		t.Fatal(err)
	}
	want := "package.json schema/callout.json src/host/react.ts src/pack.ts tsconfig.json"
	if strings.Join(got, " ") != want {
		t.Fatalf("listSources = %v, want %s", got, want)
	}
}

func TestInspectBundle(t *testing.T) {
	older := buildEpoch
	newer := buildEpoch.Add(time.Hour)
	sources := []string{"package.json", "src/pack.ts", "schema/callout.json"}

	tests := []struct {
		name      string
		entry     bool
		record    string
		srcTime   time.Time
		touched   string
		wantStale bool
	}{
		{name: "entry missing", entry: false, srcTime: older, wantStale: true},
		{name: "entry newer than sources, no record", entry: true, srcTime: older, wantStale: false},
		{name: "entry newer than sources, stale record", entry: true, record: "stale", srcTime: older, wantStale: false},
		{name: "source newer, no record", entry: true, srcTime: newer, wantStale: true},
		{name: "source newer, record differs", entry: true, record: "stale", srcTime: newer, wantStale: true},
		{name: "source newer, record matches", entry: true, record: "match", srcTime: newer, wantStale: false},
		{name: "only a non-source file newer", entry: true, srcTime: older, touched: "examples/callout.json", wantStale: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := sourcePackFiles(`"true"`)
			if tt.entry {
				files["dist/pack.js"] = "export default {}"
			}
			dir := writeTree(t, files)
			digest := mustDigest(t, dir)
			record := tt.record
			if record == "match" {
				record = digest
			}
			if record != "" {
				if err := writeBuildRecord(dir, record); err != nil {
					t.Fatal(err)
				}
			}
			setModTimes(t, dir, tt.srcTime, sources...)
			setModTimes(t, dir, older, "examples/callout.json")
			if tt.entry {
				setModTimes(t, dir, buildEpoch.Add(time.Minute), "dist/pack.js")
			}
			if tt.touched != "" {
				setModTimes(t, dir, newer, tt.touched)
			}

			st, err := inspectBundle(dir, "dist/pack.js")
			if err != nil {
				t.Fatal(err)
			}
			if st.stale != tt.wantStale {
				t.Fatalf("stale = %v, want %v", st.stale, tt.wantStale)
			}
			if st.recorded != record {
				t.Fatalf("recorded = %q, want %q", st.recorded, record)
			}
			if st.stale && st.digest != digest {
				t.Fatalf("digest = %q, want %q", st.digest, digest)
			}
		})
	}
}

func TestFindBun(t *testing.T) {
	t.Run("missing names the tool", func(t *testing.T) {
		t.Setenv("PATH", "")
		t.Setenv("HOME", t.TempDir())
		t.Setenv("MISE_DATA_DIR", "")
		t.Setenv("XDG_DATA_HOME", "")
		_, err := findBun()
		if err == nil || !strings.Contains(err.Error(), "build needs bun") {
			t.Fatalf("findBun() err = %v, want 'build needs bun'", err)
		}
	})
	t.Run("mise shim", func(t *testing.T) {
		mise := t.TempDir()
		t.Setenv("PATH", "")
		t.Setenv("HOME", t.TempDir())
		t.Setenv("MISE_DATA_DIR", mise)
		t.Setenv("XDG_DATA_HOME", "")
		shim := filepath.Join(mise, "shims", "bun")
		if err := os.MkdirAll(filepath.Dir(shim), 0o700); err != nil {
			t.Fatal(err)
		}
		//nolint:gosec // G306: the fake shim must be executable to resolve.
		if err := os.WriteFile(shim, []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		got, err := findBun()
		if err != nil || got != shim {
			t.Fatalf("findBun() = %q, %v, want %q", got, err, shim)
		}
	})
}

func TestLoadDropsSourcePackWithoutBun(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	dir := writeTree(t, sourcePackFiles(`"true"`))

	reg := Load(t.Context(), []string{dir}, nil)
	if len(reg.Packs()) != 0 {
		t.Fatalf("packs = %v, want none", packNames(reg))
	}
	if !hasDropReason(reg, dir, "build needs bun: not found on PATH or in mise shims") {
		t.Fatalf("dropped = %+v, want bun-missing reason", reg.Dropped)
	}
}

func TestLoaderBuildsScaffoldedPack(t *testing.T) {
	requireBun(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	dir := filepath.Join(t.TempDir(), "scaffolded")
	if _, err := Scaffold(dir, "scaffolded", examples.ExamplePack, examples.ExamplePackRoot); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(dir, "dist", "pack.js")
	if fileExists(entry) {
		t.Fatalf("scaffold wrote %s, want source only", entry)
	}

	l := NewLoader(t.Context(), []string{dir}, nil)
	if !hasDropReason(l.Current(), dir, "building") {
		t.Fatalf("first scan dropped = %+v, want building", l.Current().Dropped)
	}
	l.builds.wg.Wait()
	l.ttl = 0
	reg := l.Current()
	if names := packNames(reg); len(names) != 1 || names[0] != "scaffolded" {
		t.Fatalf("packs = %v, dropped = %+v, want scaffolded", names, reg.Dropped)
	}
	digest := mustDigest(t, dir)
	if got := reg.Packs()[0].Built; got != digest {
		t.Fatalf("Built = %q, want source digest %q", got, digest)
	}
	if recorded, err := readBuildRecord(dir); err != nil || recorded != digest {
		t.Fatalf("build record = %q, %v, want %q", recorded, err, digest)
	}
	built, err := os.Stat(entry)
	if err != nil {
		t.Fatal(err)
	}

	if p, err := Lint(t.Context(), dir); err != nil || p.Built != digest {
		t.Fatalf("Lint() = %v, %v, want built %q without rebuilding", p, err, digest)
	}
	after, err := os.Stat(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(built.ModTime()) {
		t.Fatalf("unchanged tree rebuilt: entry mtime %v -> %v", built.ModTime(), after.ModTime())
	}

	if err := os.RemoveAll(filepath.Join(dir, "dist")); err != nil {
		t.Fatal(err)
	}
	reg = Load(t.Context(), []string{dir}, nil)
	if names := packNames(reg); len(names) != 1 || reg.Packs()[0].Built != digest {
		t.Fatalf("after deleting dist: packs = %v, dropped = %+v, want rebuilt scaffolded", names, reg.Dropped)
	}
	if !fileExists(entry) {
		t.Fatalf("entry %s not rebuilt", entry)
	}
}

func TestFailedBuildKeepsPreviousBundle(t *testing.T) {
	requireBun(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	files := sourcePackFiles(`"echo build-broke-marker && exit 3"`)
	files["dist/pack.js"] = "previous bundle"
	dir := writeTree(t, files)
	if err := writeBuildRecord(dir, "previous-digest"); err != nil {
		t.Fatal(err)
	}
	setModTimes(t, dir, buildEpoch, "dist/pack.js")

	_, err := Lint(t.Context(), dir)
	if err == nil || !strings.Contains(err.Error(), "build failed: bun run build") || !strings.Contains(err.Error(), "build-broke-marker") {
		t.Fatalf("Lint() err = %v, want build failure with the log tail", err)
	}
	//nolint:gosec // G304: reading the bundle under t.TempDir().
	got, err := os.ReadFile(filepath.Join(dir, "dist", "pack.js"))
	if err != nil || string(got) != "previous bundle" {
		t.Fatalf("dist/pack.js = %q, %v, want the previous bundle kept", got, err)
	}
	if recorded, err := readBuildRecord(dir); err != nil || recorded != "previous-digest" {
		t.Fatalf("build record = %q, %v, want previous-digest kept", recorded, err)
	}

	l := NewLoader(t.Context(), []string{dir}, nil)
	l.builds.wg.Wait()
	l.ttl = 0
	for range 2 {
		if !hasDropReason(l.Current(), dir, "build-broke-marker") {
			t.Fatalf("dropped = %+v, want build failure reason", l.Current().Dropped)
		}
		if l.builds.running[dir] {
			t.Fatal("unchanged broken tree rebuilt on rescan")
		}
	}
}
