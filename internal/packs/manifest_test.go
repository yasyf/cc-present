package packs

import (
	"strings"
	"testing"
)

const validManifest = `host_api = 1
name = "example"
version = "0.1.0"
description = "Example blocks."
entry = "dist/pack.js"
styles = "dist/pack.css"
reference = "reference/blocks.md"

[blocks.callout]
description = "Toned admonition."
schema = "schema/callout.json"
examples = ["examples/callout.json"]

[blocks.rating]
description = "1-5 rating."
schema = "schema/rating.json"
interaction = "schema/rating.interaction.json"
examples = ["examples/rating.json"]
`

func validFiles() map[string]string {
	return map[string]string{
		"cc-present.toml":                validManifest,
		"dist/pack.js":                   "export default {}",
		"dist/pack.css":                  ".x{}",
		"reference/blocks.md":            "# blocks",
		"schema/callout.json":            `{"type":"object"}`,
		"schema/rating.json":             `{"type":"object"}`,
		"schema/rating.interaction.json": `{"type":"object","properties":{"value":{"type":"integer"}}}`,
		"examples/callout.json":          `{"id":"c","type":"example.callout"}`,
		"examples/rating.json":           `{"id":"r","type":"example.rating","value":3}`,
	}
}

func TestBuildPackValid(t *testing.T) {
	p, err := buildPack(writeTree(t, validFiles()))
	if err != nil {
		t.Fatalf("buildPack: %v", err)
	}
	if p.Name != "example" || p.Version != "0.1.0" {
		t.Fatalf("name/version = %q/%q", p.Name, p.Version)
	}
	if len(p.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(p.Blocks))
	}
	// blocks are name-sorted: callout, rating
	if p.Blocks[0].FullType() != "example.callout" || p.Blocks[1].FullType() != "example.rating" {
		t.Fatalf("block types = %q, %q", p.Blocks[0].FullType(), p.Blocks[1].FullType())
	}
	if p.Blocks[0].Interactive() {
		t.Fatalf("callout should not be interactive")
	}
	if !p.Blocks[1].Interactive() {
		t.Fatalf("rating should be interactive")
	}
	if len(p.Blocks[1].InteractionBytes) == 0 {
		t.Fatalf("rating interaction bytes empty")
	}
}

func TestBuildPackErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f map[string]string)
		wantErr string
	}{
		{"unknown key", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, "host_api = 1\n", "host_api = 1\nbogus = 2\n", 1)
		}, "strict mode"},
		{"bad name", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `name = "example"`, `name = "Example"`, 1)
		}, "must match"},
		{"name too long", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `name = "example"`, `name = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, 1)
		}, "exceeds"},
		{"empty entry", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `entry = "dist/pack.js"`, `entry = ""`, 1)
		}, "entry must not be empty"},
		{"entry escapes root", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `entry = "dist/pack.js"`, `entry = "../pack.js"`, 1)
		}, "resolve inside the pack root"},
		{"entry not under dist", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `entry = "dist/pack.js"`, `entry = "pack.js"`, 1)
		}, "must be under dist/"},
		{"styles not under dist", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `styles = "dist/pack.css"`, `styles = "pack.css"`, 1)
		}, `styles "pack.css" must be under dist/`},
		{"schema escapes root", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, `schema = "schema/callout.json"`, `schema = "../../etc/passwd"`, 1)
		}, "resolve inside the pack root"},
		{"block name not dot-free", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, "[blocks.callout]", `[blocks."bad.name"]`, 1)
		}, "block name"},
		{"zero blocks", func(f map[string]string) {
			f["cc-present.toml"] = `host_api = 1
name = "example"
version = "0.1.0"
description = "d"
entry = "dist/pack.js"
`
		}, "declares no blocks"},
		{"wrong host_api", func(f map[string]string) {
			f["cc-present.toml"] = strings.Replace(validManifest, "host_api = 1", "host_api = 2", 1)
		}, "host_api 2, want 1"},
		{"entry file missing", func(f map[string]string) {
			delete(f, "dist/pack.js")
		}, "entry"},
		{"schema file missing", func(f map[string]string) {
			delete(f, "schema/callout.json")
		}, "read"},
		{"example file missing", func(f map[string]string) {
			delete(f, "examples/callout.json")
		}, "example"},
		{"non-compiling schema", func(f map[string]string) {
			f["schema/callout.json"] = `{"type": 123}`
		}, "metaschema"},
		{"remote ref schema", func(f map[string]string) {
			f["schema/callout.json"] = `{"$ref":"https://example.com/x.json"}`
		}, "external schema reference not allowed"},
		{"manifest over limit", func(f map[string]string) {
			f["cc-present.toml"] = validManifest + "\n#" + strings.Repeat("a", maxFileBytes)
		}, "manifest exceeds"},
		{"schema over limit", func(f map[string]string) {
			f["schema/callout.json"] = strings.Repeat("a", maxFileBytes+1)
		}, "exceeds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validFiles()
			tt.mutate(f)
			_, err := buildPack(writeTree(t, f))
			if err == nil {
				t.Fatalf("buildPack() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
