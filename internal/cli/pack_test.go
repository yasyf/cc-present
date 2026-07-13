package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/app"
	"github.com/yasyf/cc-present/internal/packs"
)

func writePackFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

const goodManifest = `host_api = 1
name = "example"
version = "0.1.0"
description = "Example blocks."
entry = "dist/pack.js"
reference = "reference/blocks.md"

[blocks.callout]
description = "Callout."
schema = "schema/callout.json"
examples = ["examples/callout.json"]

[blocks.rating]
description = "Rating."
schema = "schema/rating.json"
interaction = "schema/rating.interaction.json"
examples = ["examples/rating.json"]
`

func goodPackFiles() map[string]string {
	return map[string]string{
		"cc-present.toml":                goodManifest,
		"dist/pack.js":                   "export default {}",
		"reference/blocks.md":            "# blocks",
		"schema/callout.json":            `{"type":"object"}`,
		"schema/rating.json":             `{"type":"object","required":["value"],"properties":{"value":{"type":"integer"}}}`,
		"schema/rating.interaction.json": `{"type":"object"}`,
		"examples/callout.json":          `{"id":"c","type":"example.callout"}`,
		"examples/rating.json":           `{"id":"r","type":"example.rating","value":3}`,
	}
}

func writeGoodPack(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writePackFiles(t, dir, goodPackFiles())
	return dir
}

func TestPackLint(t *testing.T) {
	good := writeGoodPack(t)

	badExample := t.TempDir()
	badExampleFiles := goodPackFiles()
	// The rating schema requires an integer value; this example violates it.
	badExampleFiles["examples/rating.json"] = `{"id":"r","type":"example.rating","value":"not-an-int"}`
	writePackFiles(t, badExample, badExampleFiles)

	badManifest := t.TempDir()
	badManifestFiles := goodPackFiles()
	badManifestFiles["cc-present.toml"] = "host_api = 1\nname = \"example\"\nbogus_key = true\n"
	writePackFiles(t, badManifest, badManifestFiles)

	missingEntry := t.TempDir()
	missingEntryFiles := goodPackFiles()
	delete(missingEntryFiles, "dist/pack.js")
	writePackFiles(t, missingEntry, missingEntryFiles)

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{"good pack", good, ""},
		{"example violates schema", badExample, "example"},
		{"unknown manifest key", badManifest, "decode manifest"},
		{"missing entry bundle", missingEntry, "entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newPackLintCmd()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetArgs([]string{tt.dir})
			err := c.Execute()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("lint good pack: %v", err)
				}
				if !strings.Contains(out.String(), "ok:") {
					t.Fatalf("out = %q, want ok summary", out.String())
				}
				return
			}
			if err == nil {
				t.Fatalf("lint %s: err = nil, want %q", tt.name, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("lint %s: err = %q, want substring %q", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestPackList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))

	good := writeGoodPack(t)

	dropped := t.TempDir()
	droppedFiles := goodPackFiles()
	delete(droppedFiles, "dist/pack.js") // missing entry → dropped fail-soft
	writePackFiles(t, dropped, droppedFiles)

	if err := app.WriteConfig(app.Config{PackDirs: []string{good, dropped}}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c := newPackListCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs(nil)
	if err := c.Execute(); err != nil {
		t.Fatalf("pack list: %v", err)
	}
	s := out.String()
	for _, want := range []string{"example 0.1.0", "example.callout", "example.rating (interactive)", good, "dropped:", dropped, "entry"} {
		if !strings.Contains(s, want) {
			t.Fatalf("list output missing %q:\n%s", want, s)
		}
	}
	// The reference fragment is printed as an absolute path the model can Read.
	if !strings.Contains(s, filepath.Join(good, "reference", "blocks.md")) {
		t.Fatalf("list output missing reference path:\n%s", s)
	}
}

func runPackInit(t *testing.T, args []string) (string, error) {
	t.Helper()
	c := newPackInitCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

func readScaffoldFile(t *testing.T, path string) []byte {
	t.Helper()
	//nolint:gosec // G304: reading a file the test just scaffolded under t.TempDir().
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPackInit(t *testing.T) {
	t.Run("happy path renames pack", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "scratch")
		out, err := runPackInit(t, []string{"--name", "demo", dir})
		if err != nil {
			t.Fatalf("pack init: %v", err)
		}
		if !strings.Contains(out, `scaffolded pack "demo"`) {
			t.Fatalf("out = %q, want scaffolded summary", out)
		}

		m, err := packs.ParseManifest(dir)
		if err != nil {
			t.Fatalf("re-parse manifest: %v", err)
		}
		if m.Name != "demo" {
			t.Fatalf("manifest name = %q, want demo", m.Name)
		}

		callout := readScaffoldFile(t, filepath.Join(dir, "schema", "callout.json"))
		if !bytes.Contains(callout, []byte("demo.callout")) {
			t.Fatalf("callout schema missing demo.callout:\n%s", callout)
		}

		err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			//nolint:gosec // G304: reading a file the test just scaffolded under t.TempDir().
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if bytes.Contains(b, []byte("example.")) {
				t.Errorf("%s still contains example.", p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		exCallout := readScaffoldFile(t, filepath.Join(dir, "examples", "callout.json"))
		if !bytes.Contains(exCallout, []byte(`"demo.callout"`)) {
			t.Fatalf("example callout missing renamed type:\n%s", exCallout)
		}

		gitignore := readScaffoldFile(t, filepath.Join(dir, ".gitignore"))
		if !bytes.Contains(gitignore, []byte("node_modules/")) {
			t.Fatalf(".gitignore missing node_modules/:\n%s", gitignore)
		}
		if bytes.Contains(gitignore, []byte("dist/")) {
			t.Fatalf(".gitignore must not ignore dist/:\n%s", gitignore)
		}

		pkg := readScaffoldFile(t, filepath.Join(dir, "package.json"))
		if !bytes.Contains(pkg, []byte("@cc-present/demo-pack")) {
			t.Fatalf("package.json missing @cc-present/demo-pack:\n%s", pkg)
		}
	})

	t.Run("default name from dir basename", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "mypack")
		if _, err := runPackInit(t, []string{dir}); err != nil {
			t.Fatalf("pack init: %v", err)
		}
		m, err := packs.ParseManifest(dir)
		if err != nil {
			t.Fatal(err)
		}
		if m.Name != "mypack" {
			t.Fatalf("manifest name = %q, want mypack", m.Name)
		}
	})

	t.Run("lint passes on stubbed scaffold", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "linted")
		if _, err := runPackInit(t, []string{"--name", "linted", dir}); err != nil {
			t.Fatalf("pack init: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "dist", "pack.js"), []byte("export default {}"), 0o600); err != nil {
			t.Fatal(err)
		}
		p, err := packs.Lint(dir)
		if err != nil {
			t.Fatalf("lint scaffold: %v", err)
		}
		if p.Name != "linted" {
			t.Fatalf("lint name = %q, want linted", p.Name)
		}
		if len(p.Blocks) != 3 {
			t.Fatalf("lint blocks = %d, want 3", len(p.Blocks))
		}
	})

	t.Run("bad name rejected", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "x")
		_, err := runPackInit(t, []string{"--name", "Bad", dir})
		if err == nil || !strings.Contains(err.Error(), "must match") {
			t.Fatalf("err = %v, want 'must match'", err)
		}
	})

	t.Run("name too long rejected", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "x")
		_, err := runPackInit(t, []string{"--name", strings.Repeat("a", 33), dir})
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("err = %v, want 'exceeds'", err)
		}
	})

	t.Run("non-empty dir refused", func(t *testing.T) {
		dir := t.TempDir()
		stray := filepath.Join(dir, "keep.txt")
		if err := os.WriteFile(stray, []byte("stay"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := runPackInit(t, []string{"--name", "demo", dir})
		if err == nil || !strings.Contains(err.Error(), "not empty") {
			t.Fatalf("err = %v, want 'not empty'", err)
		}
		//nolint:gosec // G304: reading the stray file under t.TempDir() to prove it's untouched.
		b, err := os.ReadFile(stray)
		if err != nil || string(b) != "stay" {
			t.Fatalf("stray file altered: %q, %v", b, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "cc-present.toml")); !os.IsNotExist(err) {
			t.Fatalf("scaffold wrote into non-empty dir: %v", err)
		}
	})

	t.Run("invalid derived name instructs --name", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "Bad_Name")
		_, err := runPackInit(t, []string{dir})
		if err == nil || !strings.Contains(err.Error(), "--name") {
			t.Fatalf("err = %v, want '--name'", err)
		}
	})
}

func TestPushDryRunPackBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))

	good := writeGoodPack(t)
	if err := app.WriteConfig(app.Config{PackDirs: []string{good}}); err != nil {
		t.Fatalf("write config: %v", err)
	}

	docPath := filepath.Join(t.TempDir(), "doc.json")
	docJSON := `{"version":1,"title":"T","blocks":[{"id":"c1","type":"example.callout","tone":"warn"}]}`
	if err := os.WriteFile(docPath, []byte(docJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	c := newPushCmd(cmd.Deps{})
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{docPath, "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("push --dry-run: %v", err)
	}
	if strings.TrimSpace(out.String()) != "ok" {
		t.Fatalf("out = %q, want ok", out.String())
	}
}
