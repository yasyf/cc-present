package anchor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type anchorCorpus struct {
	Hash []struct {
		Line string `json:"line"`
		Hash string `json:"hash"`
	} `json:"hash"`
	Parse []struct {
		Ref   string `json:"ref"`
		Line  int    `json:"line"`
		End   int    `json:"end"`
		Hash  string `json:"hash"`
		Error bool   `json:"error"`
	} `json:"parse"`
	Resolve []struct {
		Ref   string   `json:"ref"`
		Lines []string `json:"lines"`
		Start int      `json:"start"`
		End   int      `json:"end"`
		Moved bool     `json:"moved"`
		From  int      `json:"from"`
		Error string   `json:"error"`
	} `json:"resolve"`
}

func loadCorpus(t *testing.T) anchorCorpus {
	t.Helper()
	b, err := os.ReadFile("testdata/anchors.json")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var corpus anchorCorpus
	if err := json.Unmarshal(b, &corpus); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	return corpus
}

func TestOf(t *testing.T) {
	for _, tt := range loadCorpus(t).Hash {
		t.Run(fmt.Sprintf("%q", tt.Line), func(t *testing.T) {
			if got := Of(tt.Line); got != tt.Hash {
				t.Fatalf("Of(%q) = %q, want %q", tt.Line, got, tt.Hash)
			}
		})
	}
}

func TestParse(t *testing.T) {
	for _, tt := range loadCorpus(t).Parse {
		t.Run(tt.Ref, func(t *testing.T) {
			got, err := Parse(tt.Ref)
			if tt.Error {
				if err == nil {
					t.Fatalf("Parse(%q) error = nil, want failure", tt.Ref)
				}
				if !strings.Contains(err.Error(), tt.Ref) {
					t.Fatalf("Parse(%q) error = %q, want offending reference", tt.Ref, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.Ref, err)
			}
			want := Ref{Line: tt.Line, End: tt.End, Hash: tt.Hash}
			if got != want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tt.Ref, got, want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "line", got: Format(12, "xrkm"), want: "12#xrkm"},
		{name: "range", got: FormatRange(2, 4, "tj58"), want: "2-4#tj58"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("format = %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	for _, tt := range loadCorpus(t).Resolve {
		t.Run(tt.Ref+fmt.Sprint(tt.Lines), func(t *testing.T) {
			r, err := Parse(tt.Ref)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.Ref, err)
			}
			got, err := Resolve(r, tt.Lines)
			if tt.Error != "" {
				if err == nil {
					t.Fatalf("Resolve(%q) error = nil, want %q", tt.Ref, tt.Error)
				}
				if !strings.Contains(err.Error(), tt.Error) {
					t.Fatalf("Resolve(%q) error = %q, want category %q", tt.Ref, err, tt.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tt.Ref, err)
			}
			want := Resolution{Start: tt.Start, End: tt.End, Moved: tt.Moved, From: tt.From}
			if got != want {
				t.Fatalf("Resolve(%q) = %+v, want %+v", tt.Ref, got, want)
			}
		})
	}
}
