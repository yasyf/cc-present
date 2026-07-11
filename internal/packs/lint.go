package packs

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Lint runs discovery's fail-loud checks over a pack root (strict manifest,
// host_api, files present, schemas compile) plus the one discovery skips: every
// declared example must validate against its block schema. It returns the built
// pack, or the first violation.
func Lint(dir string) (*Pack, error) {
	m, err := ParseManifest(dir)
	if err != nil {
		return nil, err
	}
	p, err := buildPack(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open pack root: %w", err)
	}
	defer func() { _ = root.Close() }()
	byName := make(map[string]*BlockType, len(p.Blocks))
	for _, bt := range p.Blocks {
		byName[bt.Name] = bt
	}
	names := make([]string, 0, len(m.Blocks))
	for name := range m.Blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		bt := byName[name]
		for _, ex := range m.Blocks[name].Examples {
			if err := validateExample(root, ex, bt.Schema); err != nil {
				return nil, fmt.Errorf("block %q example %q: %w", name, ex, err)
			}
		}
	}
	return p, nil
}

func validateExample(root *os.Root, rel string, schema *jsonschema.Schema) error {
	data, err := readCapped(root, rel)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return schema.Validate(inst)
}
