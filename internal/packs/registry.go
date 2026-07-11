package packs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/yasyf/cc-present/internal/doc"
)

// HostAPIVersion is the pack host API this daemon implements; a manifest
// declaring a different host_api is skipped, and the /api/packs surface echoes
// it as the compat gate the SPA and packs check against.
const HostAPIVersion = 1

// maxFileBytes caps the manifest and every manifest-declared file so a malicious
// pack can't force an unbounded read at scan.
const maxFileBytes = 512 << 10

var _ doc.PackTypes = (*Registry)(nil)

// Pack is a validated block pack: its manifest metadata and compiled block types.
type Pack struct {
	Name        string
	Version     string
	Description string
	Dir         string
	Entry       string
	Styles      string
	Reference   string
	Blocks      []*BlockType
	tier        tier
}

// BlockType is one compiled pack block: its dotted-type halves, the compiled
// block and interaction schemas, and their raw bytes for the /api/packs surface.
type BlockType struct {
	Pack             *Pack
	Name             string
	Schema           *jsonschema.Schema
	Interaction      *jsonschema.Schema
	SchemaBytes      json.RawMessage
	InteractionBytes json.RawMessage
}

// FullType is the block's dotted wire type `<pack>.<name>`.
func (b *BlockType) FullType() string { return b.Pack.Name + "." + b.Name }

// Interactive reports whether the block declares an interaction schema.
func (b *BlockType) Interactive() bool { return b.Interaction != nil }

// DroppedPack records a pack skipped at scan and why.
type DroppedPack struct {
	Dir    string
	Reason string
}

// Registry is an immutable snapshot of the installed packs. It implements
// doc.PackTypes so authoring edges validate pack blocks against it.
type Registry struct {
	Dropped []DroppedPack
	packs   []*Pack
	byType  map[string]*BlockType
}

// Type returns the block type for a dotted wire type, if installed.
func (r *Registry) Type(full string) (*BlockType, bool) {
	bt, ok := r.byType[full]
	return bt, ok
}

// Packs returns the installed packs in name order.
func (r *Registry) Packs() []*Pack { return append([]*Pack(nil), r.packs...) }

// ValidateBlock validates a pack block's payload against the installed schema for
// its dotted type; an uninstalled type is rejected.
func (r *Registry) ValidateBlock(typeName string, payload json.RawMessage) error {
	bt, ok := r.byType[typeName]
	if !ok {
		return fmt.Errorf("pack block type %q is not installed", typeName)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pack block %q: decode payload: %w", typeName, err)
	}
	if err := bt.Schema.Validate(inst); err != nil {
		return fmt.Errorf("pack block %q: %w", typeName, err)
	}
	return nil
}

// ValidateInteraction validates a pack interaction payload against the installed
// interaction schema for its dotted type. An uninstalled type or a
// non-interactive one (no declared interaction schema) is rejected.
func (r *Registry) ValidateInteraction(typeName string, payload json.RawMessage) error {
	bt, ok := r.byType[typeName]
	if !ok {
		return fmt.Errorf("pack block type %q is not installed", typeName)
	}
	if bt.Interaction == nil {
		return fmt.Errorf("pack block type %q is not interactive", typeName)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pack interaction %q: decode payload: %w", typeName, err)
	}
	if err := bt.Interaction.Validate(inst); err != nil {
		return fmt.Errorf("pack interaction %q: %w", typeName, err)
	}
	return nil
}

type builtPack struct {
	pack *Pack
	root packRoot
}

func buildRegistry(roots []packRoot, disabled []string) *Registry {
	r := &Registry{byType: map[string]*BlockType{}}
	disabledSet := map[string]bool{}
	for _, d := range disabled {
		disabledSet[d] = true
	}
	var built []builtPack
	for _, root := range roots {
		p, err := buildPack(root.dir)
		if err != nil {
			r.drop(root.dir, err.Error())
			slog.Warn("dropped pack", "dir", root.dir, "reason", err.Error())
			continue
		}
		p.tier = root.tier
		if disabledSet[p.Name] {
			r.drop(root.dir, fmt.Sprintf("pack %q disabled by config", p.Name))
			continue
		}
		built = append(built, builtPack{pack: p, root: root})
	}
	byName := map[string][]builtPack{}
	var names []string
	for _, b := range built {
		if _, seen := byName[b.pack.Name]; !seen {
			names = append(names, b.pack.Name)
		}
		byName[b.pack.Name] = append(byName[b.pack.Name], b)
	}
	sort.Strings(names)
	for _, name := range names {
		if winner := r.resolveGroup(name, byName[name]); winner != nil {
			r.packs = append(r.packs, winner)
			for _, bt := range winner.Blocks {
				r.byType[bt.FullType()] = bt
			}
		}
	}
	sort.Slice(r.packs, func(i, j int) bool { return r.packs[i].Name < r.packs[j].Name })
	sort.Slice(r.Dropped, func(i, j int) bool { return r.Dropped[i].Dir < r.Dropped[j].Dir })
	return r
}

// resolveGroup applies the conflict rules for a set of same-named packs: a dev
// pack shadows an installed one (logged), and two same-tier packs drop each other.
func (r *Registry) resolveGroup(name string, group []builtPack) *Pack {
	var dev, plugin []builtPack
	for _, b := range group {
		if b.root.tier == tierDev {
			dev = append(dev, b)
		} else {
			plugin = append(plugin, b)
		}
	}
	winners := dev
	if len(dev) == 0 {
		winners = plugin
	} else {
		for _, b := range plugin {
			r.drop(b.root.dir, fmt.Sprintf("pack %q shadowed by dev dir", name))
			slog.Info("pack shadowed by dev dir", "pack", name, "dir", b.root.dir)
		}
	}
	if len(winners) == 1 {
		return winners[0].pack
	}
	for _, b := range winners {
		r.drop(b.root.dir, fmt.Sprintf("duplicate pack name %q in same tier", name))
		slog.Warn("duplicate pack name", "pack", name, "dir", b.root.dir)
	}
	return nil
}

func (r *Registry) drop(dir, reason string) {
	r.Dropped = append(r.Dropped, DroppedPack{Dir: dir, Reason: reason})
}

// buildPack validates a pack root fail-loud: strict manifest, host_api match,
// declared files present, and every schema compiling as Draft 2020-12.
func buildPack(root string) (*Pack, error) {
	m, err := ParseManifest(root)
	if err != nil {
		return nil, err
	}
	if m.HostAPI != HostAPIVersion {
		return nil, fmt.Errorf("host_api %d, want %d", m.HostAPI, HostAPIVersion)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open pack root: %w", err)
	}
	defer func() { _ = r.Close() }()
	if !fileExistsIn(r, m.Entry) {
		return nil, fmt.Errorf("entry %q not found", m.Entry)
	}
	if m.Styles != "" && !fileExistsIn(r, m.Styles) {
		return nil, fmt.Errorf("styles %q not found", m.Styles)
	}
	if m.Reference != "" && !fileExistsIn(r, m.Reference) {
		return nil, fmt.Errorf("reference %q not found", m.Reference)
	}
	p := &Pack{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Dir:         root,
		Entry:       m.Entry,
		Styles:      m.Styles,
		Reference:   m.Reference,
	}
	names := make([]string, 0, len(m.Blocks))
	for name := range m.Blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		bt, err := buildBlock(r, p, name, m.Blocks[name])
		if err != nil {
			return nil, err
		}
		p.Blocks = append(p.Blocks, bt)
	}
	return p, nil
}

func buildBlock(root *os.Root, p *Pack, name string, bm BlockManifest) (*BlockType, error) {
	schemaBytes, sch, err := compileSchema(root, bm.Schema)
	if err != nil {
		return nil, fmt.Errorf("block %q schema: %w", name, err)
	}
	bt := &BlockType{Pack: p, Name: name, Schema: sch, SchemaBytes: schemaBytes}
	if bm.Interaction != "" {
		ib, isch, err := compileSchema(root, bm.Interaction)
		if err != nil {
			return nil, fmt.Errorf("block %q interaction schema: %w", name, err)
		}
		bt.Interaction = isch
		bt.InteractionBytes = ib
	}
	for _, ex := range bm.Examples {
		if !fileExistsIn(root, ex) {
			return nil, fmt.Errorf("block %q example %q not found", name, ex)
		}
	}
	return bt, nil
}

// rejectLoader refuses every external schema reference so a malicious or errant
// $ref cannot reach the network or the filesystem; Draft 2020-12 meta-schemas
// still resolve from the library's embedded copies.
type rejectLoader struct{}

func (rejectLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema reference not allowed: %s", url)
}

func compileSchema(root *os.Root, rel string) (json.RawMessage, *jsonschema.Schema, error) {
	data, err := readCapped(root, rel)
	if err != nil {
		return nil, nil, fmt.Errorf("read %q: %w", rel, err)
	}
	loaded, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("parse %q: %w", rel, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(rejectLoader{})
	if err := c.AddResource("schema.json", loaded); err != nil {
		return nil, nil, fmt.Errorf("add %q: %w", rel, err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		return nil, nil, fmt.Errorf("compile %q: %w", rel, err)
	}
	return json.RawMessage(data), sch, nil
}

// readCapped reads a manifest-declared file through the pack's os.Root, which
// refuses any symlink or .. escape structurally, and rejects one past
// maxFileBytes so a pack can't force an unbounded read.
func readCapped(root *os.Root, rel string) ([]byte, error) {
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("exceeds %d bytes", maxFileBytes)
	}
	return data, nil
}

// fileExistsIn reports whether rel names a regular file inside the pack's
// os.Root; a symlink or path escaping the root is not found.
func fileExistsIn(root *os.Root, rel string) bool {
	info, err := root.Stat(rel)
	return err == nil && !info.IsDir()
}
