package packs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

func TestBuildPackSchemaSymlinkEscape(t *testing.T) {
	dir := writeTree(t, validFiles())
	secret := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(secret, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(dir, "schema", "callout.json")
	if err := os.Remove(schemaPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, schemaPath); err != nil {
		t.Fatal(err)
	}

	// scan: the pack drops with a reason naming the escaping schema path.
	reg := buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil)
	if names := packNames(reg); len(names) != 0 {
		t.Fatalf("packs = %v, want none (symlinked schema dropped)", names)
	}
	if !hasDropReason(reg, dir, "schema/callout.json") {
		t.Fatalf("dropped = %+v, want reason naming schema/callout.json", reg.Dropped)
	}

	// lint: the same escape fails loud.
	if _, err := Lint(dir); err == nil || !strings.Contains(err.Error(), "callout.json") {
		t.Fatalf("Lint() = %v, want error naming callout.json", err)
	}
}

func TestRegistryValidateBlock(t *testing.T) {
	dir := writeTree(t, validFiles())
	reg := buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil)
	if names := packNames(reg); len(names) != 1 || names[0] != "example" {
		t.Fatalf("packs = %v, want [example]", names)
	}
	if _, ok := reg.Type("example.callout"); !ok {
		t.Fatalf("example.callout not registered")
	}
	if _, ok := reg.Type("example.missing"); ok {
		t.Fatalf("example.missing unexpectedly registered")
	}

	tests := []struct {
		name    string
		typ     string
		payload string
		wantErr bool
	}{
		{"valid object", "example.callout", `{"id":"c","type":"example.callout"}`, false},
		{"non-object rejected", "example.callout", `"nope"`, true},
		{"uninstalled rejected", "other.thing", `{}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.ValidateBlock(tt.typ, json.RawMessage(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateBlock() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistryValidateInteraction(t *testing.T) {
	dir := writeTree(t, validFiles())
	reg := buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil)

	tests := []struct {
		name    string
		typ     string
		payload string
		wantErr string
	}{
		{"valid", "example.rating", `{"value":3}`, ""},
		{"schema violation", "example.rating", `{"value":"x"}`, "pack interaction"},
		{"non-interactive", "example.callout", `{"value":3}`, "not interactive"},
		{"uninstalled", "other.thing", `{}`, "not installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.ValidateInteraction(tt.typ, json.RawMessage(tt.payload))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateInteraction() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateInteraction() = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestRegistrySatisfiesPackTypes(t *testing.T) {
	dir := writeTree(t, validFiles())
	var pt doc.PackTypes = buildRegistry([]packRoot{{dir: dir, tier: tierDev}}, nil)

	var ok doc.Doc
	if err := json.Unmarshal([]byte(`{"version":1,"title":"T","blocks":[{"id":"c","type":"example.callout"}]}`), &ok); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ok.Validate(pt); err != nil {
		t.Fatalf("Validate declared pack block: %v", err)
	}

	var bad doc.Doc
	if err := json.Unmarshal([]byte(`{"version":1,"title":"T","blocks":[{"id":"x","type":"example.nope"}]}`), &bad); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := bad.Validate(pt); err == nil {
		t.Fatal("Validate undeclared pack type = nil, want error")
	}
}
