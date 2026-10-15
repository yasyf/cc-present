// Package toolpath resolves an external tool the daemon runs. A daemon launched
// outside a login shell lacks mise's shims on PATH, so the lookup falls back to
// them.
package toolpath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Find returns the absolute path of the named executable from PATH or, failing
// that, from mise's shim directories.
func Find(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	dirs := miseShimDirs()
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("not found on PATH or in mise shims (%s)", strings.Join(dirs, ", "))
}

func miseShimDirs() []string {
	var dirs []string
	if d := os.Getenv("MISE_DATA_DIR"); d != "" {
		dirs = append(dirs, filepath.Join(d, "shims"))
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		dirs = append(dirs, filepath.Join(d, "mise", "shims"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "mise", "shims"))
	}
	return dirs
}
