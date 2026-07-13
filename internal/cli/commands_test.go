package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

func TestLineCol(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		offset   int64
		wantLine int
		wantCol  int
	}{
		{"first line bracket", `{ "a": ]`, 8, 1, 8},
		{"multiline second comma", "{\n  \"version\": 1,\n  \"title\": \"T\",,\n}", 34, 3, 16},
		{"offset one is start", "abc", 1, 1, 1},
		{"offset past end clamps", "ab\ncd", 100, 2, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := lineCol([]byte(tt.data), tt.offset)
			if line != tt.wantLine || col != tt.wantCol {
				t.Fatalf("lineCol() = (%d, %d), want (%d, %d)", line, col, tt.wantLine, tt.wantCol)
			}
		})
	}
}

func TestJSONErrorAt(t *testing.T) {
	syntaxRaw := []byte(`{ "a": ]`)
	var m map[string]any
	syntaxErr := json.Unmarshal(syntaxRaw, &m)

	typeRaw := []byte("{\n  \"version\": \"oops\"\n}")
	var d doc.Doc
	typeErr := json.Unmarshal(typeRaw, &d)

	plain := errors.New("boom")

	tests := []struct {
		name string
		raw  []byte
		err  error
		want string
	}{
		{"syntax error annotated", syntaxRaw, syntaxErr, syntaxErr.Error() + " (line 1, column 8)"},
		{"type error annotated", typeRaw, typeErr, typeErr.Error() + " (line 2, column 19)"},
		{"non-json passthrough", nil, plain, "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("test setup: err is nil")
			}
			got := jsonErrorAt(tt.raw, tt.err)
			if got.Error() != tt.want {
				t.Fatalf("jsonErrorAt() = %q, want %q", got.Error(), tt.want)
			}
		})
	}
}

func TestStripDocKey(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "drops doc key",
			raw:  `{"doc":{"a":1},"interactions":{"b":2},"rounds":{"c":3}}`,
			want: `{"interactions":{"b":2},"rounds":{"c":3}}`,
		},
		{
			name: "absent doc key unchanged",
			raw:  `{"interactions":{},"rounds":{"current":2}}`,
			want: `{"interactions":{},"rounds":{"current":2}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stripDocKey(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatalf("stripDocKey() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("stripDocKey() = %s, want %s", got, tt.want)
			}
		})
	}
}

func mustDoc(t *testing.T, s string) *doc.Doc {
	t.Helper()
	var d doc.Doc
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		t.Fatalf("unmarshal doc: %v", err)
	}
	return &d
}

func mustBlock(t *testing.T, s string) doc.Block {
	t.Helper()
	b, err := doc.DecodeBlock([]byte(s))
	if err != nil {
		t.Fatalf("decode block: %v", err)
	}
	return b
}

func TestDryRunReport(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		asBlock bool
		want    string
		wantOk  bool
	}{
		{
			name:   "valid doc ok",
			raw:    `{"version":1,"title":"T","blocks":[{"id":"m1","type":"markdown","md":"hi"}]}`,
			want:   "ok",
			wantOk: true,
		},
		{
			name: "invalid doc reports every violation",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"s1","type":"section"},{"id":"m1","type":"markdown","md":""}]}`,
			want: `section "s1": title must not be empty
markdown "m1": md must not be empty`,
			wantOk: false,
		},
		{
			name:    "valid single block ok",
			raw:     `{"id":"m1","type":"markdown","md":"hi"}`,
			asBlock: true,
			want:    "ok",
			wantOk:  true,
		},
		{
			name:    "invalid single block reports violation",
			raw:     `{"id":"in1","type":"input"}`,
			asBlock: true,
			want:    `input "in1": label must not be empty`,
			wantOk:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dd *doc.Doc
			if tt.asBlock {
				dd = blockDoc(mustBlock(t, tt.raw))
			} else {
				dd = mustDoc(t, tt.raw)
			}
			msg, ok := dryRunReport(dd, doc.NoPacks)
			if ok != tt.wantOk {
				t.Fatalf("dryRunReport() ok = %v, want %v", ok, tt.wantOk)
			}
			if msg != tt.want {
				t.Fatalf("dryRunReport() msg = %q, want %q", msg, tt.want)
			}
		})
	}
}
