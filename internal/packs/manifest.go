// Package packs discovers, validates, and serves block packs — plugin-supplied
// block primitives declared by a TOML manifest and JSON Schemas. It is the
// firewall that keeps the JSON Schema dependency out of package doc: its Registry
// implements doc.PackTypes.
package packs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ManifestName is the well-known pack manifest filename at a pack root.
const ManifestName = "cc-present.toml"

const maxPackNameLen = 32

var packNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Manifest is the parsed cc-present.toml at a pack root. Every path field is
// manifest-relative and constrained to the pack root; block names are dot-free.
type Manifest struct {
	HostAPI     int                      `toml:"host_api"`
	Name        string                   `toml:"name"`
	Version     string                   `toml:"version"`
	Description string                   `toml:"description"`
	Entry       string                   `toml:"entry"`
	Styles      string                   `toml:"styles"`
	Reference   string                   `toml:"reference"`
	Blocks      map[string]BlockManifest `toml:"blocks"`
}

// BlockManifest is one [blocks.<name>] table. Interaction's presence marks the
// block interactive.
type BlockManifest struct {
	Description string   `toml:"description"`
	Schema      string   `toml:"schema"`
	Interaction string   `toml:"interaction"`
	Examples    []string `toml:"examples"`
}

// ParseManifest reads and strictly decodes the manifest at a pack root, then
// validates field constraints and path containment. Unknown keys, malformed
// names, escaping paths, and empty block sets are each an error.
func ParseManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, ManifestName)
	//nolint:gosec // G304: opening the pack's own manifest at a discovered pack root is the parser's purpose.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat manifest: %w", err)
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxFileBytes)
	}
	dec := toml.NewDecoder(f)
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	if m.Name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if !packNamePattern.MatchString(m.Name) {
		return fmt.Errorf("name %q must match %s", m.Name, packNamePattern)
	}
	if len(m.Name) > maxPackNameLen {
		return fmt.Errorf("name %q exceeds %d characters", m.Name, maxPackNameLen)
	}
	if m.Version == "" {
		return fmt.Errorf("version must not be empty")
	}
	if err := requireContained("entry", m.Entry); err != nil {
		return err
	}
	if !underDist(m.Entry) {
		return fmt.Errorf("entry %q must be under dist/", m.Entry)
	}
	if err := optionalContained("styles", m.Styles); err != nil {
		return err
	}
	if m.Styles != "" && !underDist(m.Styles) {
		return fmt.Errorf("styles %q must be under dist/", m.Styles)
	}
	if err := optionalContained("reference", m.Reference); err != nil {
		return err
	}
	if len(m.Blocks) == 0 {
		return fmt.Errorf("manifest declares no blocks")
	}
	for name, blk := range m.Blocks {
		if err := blk.validate(name); err != nil {
			return err
		}
	}
	return nil
}

func (b BlockManifest) validate(name string) error {
	if !packNamePattern.MatchString(name) {
		return fmt.Errorf("block name %q must match %s", name, packNamePattern)
	}
	if b.Description == "" {
		return fmt.Errorf("block %q: description must not be empty", name)
	}
	if err := requireContained(fmt.Sprintf("block %q schema", name), b.Schema); err != nil {
		return err
	}
	if err := optionalContained(fmt.Sprintf("block %q interaction", name), b.Interaction); err != nil {
		return err
	}
	if len(b.Examples) == 0 {
		return fmt.Errorf("block %q: must declare at least one example", name)
	}
	for _, ex := range b.Examples {
		if err := requireContained(fmt.Sprintf("block %q example", name), ex); err != nil {
			return err
		}
	}
	return nil
}

// Interactive reports whether the block declares an interaction schema.
func (b BlockManifest) Interactive() bool { return b.Interaction != "" }

func requireContained(field, p string) error {
	if p == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return optionalContained(field, p)
}

func optionalContained(field, p string) error {
	if p == "" {
		return nil
	}
	if !filepath.IsLocal(p) {
		return fmt.Errorf("%s path %q must resolve inside the pack root", field, p)
	}
	return nil
}

func underDist(p string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	return strings.HasPrefix(clean, "dist/")
}
