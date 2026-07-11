package doc

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// packTypePattern matches a pack block's dotted wire type `<pack>.<name>`, where
// both segments are lowercase, dot-free, and start with a letter. Built-in types
// never contain a dot, so the dotted namespace is reserved for packs permanently.
var packTypePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$`)

// PackBlock is a block whose type is supplied by an installed pack. Its type is
// dotted; every field other than id and type is preserved verbatim in Fields so
// the block round-trips through the append-only log even after the pack is
// uninstalled. Schema validation happens only at authoring edges via PackTypes.
type PackBlock struct {
	base
	Fields map[string]json.RawMessage
}

// UnmarshalJSON strips id and type into the embedded base and keeps every other
// key byte-for-byte in Fields.
func (p *PackBlock) UnmarshalJSON(data []byte) error {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("unmarshal pack block: %w", err)
	}
	if raw, ok := fields["id"]; ok {
		if err := json.Unmarshal(raw, &p.ID); err != nil {
			return fmt.Errorf("pack block id: %w", err)
		}
		delete(fields, "id")
	}
	if raw, ok := fields["type"]; ok {
		if err := json.Unmarshal(raw, &p.Type); err != nil {
			return fmt.Errorf("pack block type: %w", err)
		}
		delete(fields, "type")
	}
	p.Fields = fields
	return nil
}

// MarshalJSON merges the base id and type back over the preserved fields. A
// map encodes its keys in sorted order, so the output is deterministic.
func (p *PackBlock) MarshalJSON() ([]byte, error) {
	out := make(map[string]json.RawMessage, len(p.Fields)+2)
	for k, v := range p.Fields {
		out[k] = v
	}
	id, err := json.Marshal(p.ID)
	if err != nil {
		return nil, fmt.Errorf("marshal pack block id: %w", err)
	}
	typ, err := json.Marshal(p.Type)
	if err != nil {
		return nil, fmt.Errorf("marshal pack block type: %w", err)
	}
	out["id"] = id
	out["type"] = typ
	return json.Marshal(out)
}

// PayloadJSON returns the whole block object — id, type, and every preserved
// field — as canonical JSON for schema validation against the pack's declared
// block schema, which validates the entire block object.
func (p *PackBlock) PayloadJSON() json.RawMessage {
	data, err := p.MarshalJSON()
	if err != nil {
		panic(fmt.Sprintf("marshal pack block %q: %v", p.ID, err))
	}
	return data
}

// PackTypes validates a pack block's payload against the installed pack's declared
// schema for the given dotted type. It is the firewall that keeps the JSON Schema
// dependency out of package doc: the daemon and CLI supply a real implementation
// from package packs, while reducers, CI fixtures, and not-yet-wired callers use
// NoPacks.
type PackTypes interface {
	ValidateBlock(typeName string, payload json.RawMessage) error
}

// NoPacks is a PackTypes with no packs installed; it rejects every pack block.
var NoPacks PackTypes = noPacks{}

type noPacks struct{}

func (noPacks) ValidateBlock(typeName string, _ json.RawMessage) error {
	return fmt.Errorf("pack block type %q is not installed", typeName)
}
