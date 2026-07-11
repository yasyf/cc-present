package packs

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestLintExamplePack lints the committed reference pack in examples/packs/example
// with a stubbed bundle, so go test needs no JS toolchain. It is the Go half of
// the pack's CI gate: the real manifest, schemas, and examples must validate.
func TestLintExamplePack(t *testing.T) {
	src := filepath.Join("..", "..", "examples", "packs", "example")
	dir := t.TempDir()
	copyPackTree(t, src, dir)
	// dist/ is not committed; stub the bundle lint only checks for existence.
	writeTreeInto(t, dir, map[string]string{"dist/pack.js": "0"})

	p, err := Lint(dir)
	if err != nil {
		t.Fatalf("Lint(example pack): %v", err)
	}
	if p.Name != "example" {
		t.Errorf("pack name = %q, want %q", p.Name, "example")
	}
	byName := map[string]*BlockType{}
	for _, bt := range p.Blocks {
		byName[bt.Name] = bt
	}
	if byName["callout"] == nil || byName["rating"] == nil {
		t.Fatalf("blocks = %v, want callout and rating", blockNames(p))
	}
	if byName["callout"].Interactive() {
		t.Errorf("callout should be content-only, got interactive")
	}
	if !byName["rating"].Interactive() {
		t.Errorf("rating should be interactive")
	}
}

func blockNames(p *Pack) []string {
	out := make([]string, 0, len(p.Blocks))
	for _, bt := range p.Blocks {
		out = append(out, bt.Name)
	}
	return out
}

func copyPackTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel == "node_modules" || rel == "dist" {
				return fs.SkipDir
			}
			return nil
		}
		//nolint:gosec // G304: reading the repo's own committed example pack tree in a test.
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			return err
		}
		//nolint:gosec // G703: out is rooted at t.TempDir(); rel comes from the trusted source tree.
		return os.WriteFile(out, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy pack tree: %v", err)
	}
}
