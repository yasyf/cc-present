package web

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

var assetReferenceRE = regexp.MustCompile(`(?i)<(?:script\b[^>]*\bsrc|link\b[^>]*\bhref)\s*=\s*["'](/assets/[^"'?#\s>]+)(?:[?#][^"']*)?["']`)

// Validate checks that the embedded SPA is a complete Vite build output.
func Validate() error {
	return validateDist(Dist())
}

func validateDist(dist fs.FS) error {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return fmt.Errorf("no web build embedded — run 'task build': %w", err)
	}

	references := assetReferenceRE.FindAllSubmatch(index, -1)
	if len(references) == 0 {
		return fmt.Errorf("embedded web build is not a Vite build output")
	}

	for _, reference := range references {
		name := strings.TrimPrefix(string(reference[1]), "/")
		if _, err := fs.Stat(dist, name); err != nil {
			return fmt.Errorf("embedded web build is internally inconsistent: missing %q: %w", name, err)
		}
	}

	return nil
}
