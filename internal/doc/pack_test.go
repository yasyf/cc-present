package doc_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

type stubPacks struct {
	valid       map[string]bool
	lastType    string
	lastPayload json.RawMessage
}

func (s *stubPacks) ValidateBlock(typeName string, payload json.RawMessage) error {
	s.lastType = typeName
	s.lastPayload = payload
	if s.valid[typeName] {
		return nil
	}
	return &stubErr{typeName}
}

func (s *stubPacks) Interactive(typeName string) bool { return s.valid[typeName] }

type stubErr struct{ typ string }

func (e *stubErr) Error() string { return "stub rejects " + e.typ }

func TestDecodePackBlock(t *testing.T) {
	raw := `{"id":"cal1","type":"example.callout","tone": "warn","config":{"a": 1,"b": [2,3]}}`
	blk, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("DecodeBlock: %v", err)
	}
	pb, ok := blk.(*doc.PackBlock)
	if !ok {
		t.Fatalf("block type = %T, want *doc.PackBlock", blk)
	}
	if pb.BlockID() != "cal1" || pb.BlockType() != "example.callout" {
		t.Fatalf("id/type = %q/%q, want cal1/example.callout", pb.BlockID(), pb.BlockType())
	}
	if _, present := pb.Fields["id"]; present {
		t.Fatalf("Fields kept id key")
	}
	if _, present := pb.Fields["type"]; present {
		t.Fatalf("Fields kept type key")
	}
	// byte-for-byte: the value's exact bytes (with source whitespace) survive.
	if got := string(pb.Fields["config"]); got != `{"a": 1,"b": [2,3]}` {
		t.Fatalf("config bytes = %q, want %q", got, `{"a": 1,"b": [2,3]}`)
	}
	if got := string(pb.Fields["tone"]); got != `"warn"` {
		t.Fatalf("tone bytes = %q, want %q", got, `"warn"`)
	}
}

func TestPackBlockRoundTrip(t *testing.T) {
	inputs := []string{
		`{"id":"r1","type":"example.rating","value":4,"labels":["a","b"],"nested":{"x":{"y":1}}}`,
		`{"type":"a.b","id":"z","only":true}`,
		`{"id":"e1","type":"pack.empty"}`,
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			blk, err := doc.DecodeBlock(json.RawMessage(in))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			out, err := json.Marshal(blk)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !sameJSON(t, []byte(in), out) {
				t.Fatalf("round-trip changed meaning:\n in  = %s\n out = %s", in, out)
			}
			// re-decode is idempotent
			blk2, err := doc.DecodeBlock(out)
			if err != nil {
				t.Fatalf("re-decode: %v", err)
			}
			out2, err := json.Marshal(blk2)
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}
			if string(out) != string(out2) {
				t.Fatalf("marshal not stable:\n %s\n %s", out, out2)
			}
		})
	}
}

func TestDecodePackBlockMalformed(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		wantErr string
	}{
		{"trailing dot", "example.", "malformed pack type"},
		{"leading dot", ".callout", "malformed pack type"},
		{"double dot", "example.callout.extra", "malformed pack type"},
		{"uppercase pack", "Example.callout", "malformed pack type"},
		{"uppercase name", "example.Callout", "malformed pack type"},
		{"empty segment mid", "example..callout", "malformed pack type"},
		{"dot free unknown", "frobnicate", `unknown type "frobnicate"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"id":"x1","type":"` + tt.typ + `"}`
			_, err := doc.DecodeBlock(json.RawMessage(raw))
			if err == nil {
				t.Fatalf("DecodeBlock(%q) = nil, want error", tt.typ)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPackBlockLeafOnly(t *testing.T) {
	// A pack block carrying nested structure and an asset-looking field exposes
	// no children and no asset refs — it is opaque, leaf-only data.
	raw := `{"id":"p1","type":"example.callout","children":[{"id":"n","type":"markdown","md":"x"}],"src":"asset:` + strings.Repeat("a", 64) + `"}`
	blk, err := doc.DecodeBlock(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if children := doc.Children(blk); children != nil {
		t.Fatalf("Children = %v, want nil", children)
	}
	if refs := doc.AssetRefs(blk); refs != nil {
		t.Fatalf("AssetRefs = %v, want nil", refs)
	}
}

func TestValidatePackBlock(t *testing.T) {
	packLeaf := `{"id":"p1","type":"example.callout","tone":"warn"}`
	tests := []struct {
		name    string
		doc     string
		pt      doc.PackTypes
		wantErr string
	}{
		{"declared valid top level", docWith(packLeaf), &stubPacks{valid: map[string]bool{"example.callout": true}}, ""},
		{"declared valid card child", docWith(card("c1", packLeaf)), &stubPacks{valid: map[string]bool{"example.callout": true}}, ""},
		{"declared invalid", docWith(packLeaf), &stubPacks{valid: map[string]bool{}}, "stub rejects example.callout"},
		{"undeclared", docWith(packLeaf), &stubPacks{valid: map[string]bool{"other.thing": true}}, "stub rejects example.callout"},
		{"no packs", docWith(packLeaf), doc.NoPacks, `"example.callout" is not installed`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d doc.Doc
			if err := json.Unmarshal([]byte(tt.doc), &d); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			err := d.Validate(tt.pt)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePackBlockPayload(t *testing.T) {
	stub := &stubPacks{valid: map[string]bool{"example.callout": true}}
	var d doc.Doc
	src := docWith(`{"id":"p1","type":"example.callout","tone":"warn"}`)
	if err := json.Unmarshal([]byte(src), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := d.Validate(stub); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if stub.lastType != "example.callout" {
		t.Fatalf("lastType = %q", stub.lastType)
	}
	var got map[string]any
	if err := json.Unmarshal(stub.lastPayload, &got); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
	if got["id"] != "p1" || got["type"] != "example.callout" || got["tone"] != "warn" {
		t.Fatalf("payload = %v, want whole block object with id/type/tone", got)
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	return reflect.DeepEqual(av, bv)
}
