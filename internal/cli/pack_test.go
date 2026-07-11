package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-interact/cmd"

	"github.com/yasyf/cc-present/internal/app"
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
