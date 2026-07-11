package packs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ValidateName reports whether name is a legal pack name: non-empty, matching
// packNamePattern, and at most maxPackNameLen characters.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if !packNamePattern.MatchString(name) {
		return fmt.Errorf("name %q must match %s", name, packNamePattern)
	}
	if len(name) > maxPackNameLen {
		return fmt.Errorf("name %q exceeds %d characters", name, maxPackNameLen)
	}
	return nil
}

// Scaffold copies the reference pack rooted at srcRoot within src into destDir,
// renaming the pack name to name in every file and writing a generated
// .gitignore. name and destDir emptiness are validated before any write. It
// returns the written paths relative to destDir.
func Scaffold(destDir, name string, src fs.FS, srcRoot string) ([]string, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if err := requireEmptyDir(destDir); err != nil {
		return nil, err
	}
	//nolint:gosec // G301: a scaffolded project dir uses standard 0o755 for the human to edit and commit.
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("create dest dir: %w", err)
	}
	var written []string
	err := fs.WalkDir(src, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, srcRoot+"/")
		content, err := fs.ReadFile(src, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		if err := writeScaffoldFile(destDir, rel, rename(content, name)); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := writeScaffoldFile(destDir, ".gitignore", []byte("node_modules/\n")); err != nil {
		return nil, err
	}
	written = append(written, ".gitignore")
	return written, nil
}

// Targeted, not a global "example" replace: the examples/ paths and the
// manifest examples = [...] key must stay literal.
func rename(content []byte, name string) []byte {
	out := content
	out = bytes.ReplaceAll(out, []byte("example."), []byte(name+"."))
	out = bytes.ReplaceAll(out, []byte(`name = "example"`), []byte(`name = "`+name+`"`))
	out = bytes.ReplaceAll(out, []byte("@cc-present/example-pack"), []byte("@cc-present/"+name+"-pack"))
	out = bytes.ReplaceAll(out, []byte("# example pack blocks"), []byte("# "+name+" pack blocks"))
	out = bytes.ReplaceAll(out, []byte("`example`"), []byte("`"+name+"`"))
	return out
}

func requireEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dest dir: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("destination %q is not empty", dir)
	}
	return nil
}

func writeScaffoldFile(destDir, rel string, content []byte) error {
	full := filepath.Join(destDir, filepath.FromSlash(rel))
	//nolint:gosec // G301: a scaffolded project dir uses standard 0o755 for the human to edit and commit.
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", rel, err)
	}
	//nolint:gosec // G306: scaffolded project files use standard 0o644 for the human to edit and commit.
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}
