package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/packs"
)

func TestPlanBoardExample(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"dist/pack.js": "0", "dist/pack.css": ""}
	plan := os.DirFS(filepath.Join(repo, "plugin", ".claude", "components", "plan"))
	for _, pattern := range []string{"cc-present.toml", "reference/*.md", "schema/*.json", "examples/*.json"} {
		matches, err := fs.Glob(plan, pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range matches {
			data, err := fs.ReadFile(plan, rel)
			if err != nil {
				t.Fatal(err)
			}
			files[rel] = string(data)
		}
	}
	pack := t.TempDir()
	writePackFiles(t, pack, files)
	reg := packs.Load(t.Context(), []string{pack}, nil)
	if len(reg.Dropped) != 0 {
		t.Fatalf("dropped = %+v", reg.Dropped)
	}

	//nolint:gosec // G304: reading the repo's own committed example board in a test.
	raw, err := os.ReadFile(filepath.Join(repo, "examples", "plan-board.json"))
	if err != nil {
		t.Fatal(err)
	}
	dd := &doc.Doc{}
	if err := json.Unmarshal(raw, dd); err != nil {
		t.Fatalf("decode plan-board.json: %v", jsonErrorAt(raw, err))
	}
	if msg, ok := dryRunReport(t.Context(), dd, reg, repo); !ok {
		t.Fatalf("dry-run plan-board.json = %q, want ok", msg)
	}
}
