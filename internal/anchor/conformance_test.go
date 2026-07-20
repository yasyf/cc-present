// Package anchor_test pins the upstream github.com/yasyf/cc-context/anchor
// package to the corpus contract in testdata/anchors.json, shared with the web
// and iOS surfaces. Parse error rows only assert the acceptance predicate
// (ok && err == nil) is false, since upstream passes non-anchor-shaped input
// through (ok false, nil err) yet the REST edge rejects both buckets alike.
package anchor_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yasyf/cc-context/anchor"
)

type corpus struct {
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

func loadCorpus(t *testing.T) corpus {
	t.Helper()
	b, err := os.ReadFile("testdata/anchors.json")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var c corpus
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("decode corpus: %v", err)
	}
	return c
}

func TestOf(t *testing.T) {
	for _, tt := range loadCorpus(t).Hash {
		t.Run(fmt.Sprintf("%q", tt.Line), func(t *testing.T) {
			if got := string(anchor.Of(tt.Line)); got != tt.Hash {
				t.Fatalf("Of(%q) = %q, want %q", tt.Line, got, tt.Hash)
			}
		})
	}
}

func TestParse(t *testing.T) {
	for _, tt := range loadCorpus(t).Parse {
		t.Run(tt.Ref, func(t *testing.T) {
			ref, ok, err := anchor.Parse(tt.Ref)
			if tt.Error {
				if ok && err == nil {
					t.Fatalf("Parse(%q) accepted %+v, want rejection", tt.Ref, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.Ref, err)
			}
			if !ok {
				t.Fatalf("Parse(%q) ok = false, want true", tt.Ref)
			}
			if ref.Line != tt.Line {
				t.Fatalf("Parse(%q) line = %d, want %d", tt.Ref, ref.Line, tt.Line)
			}
			if string(ref.Hash) != tt.Hash {
				t.Fatalf("Parse(%q) hash = %q, want %q", tt.Ref, ref.Hash, tt.Hash)
			}
			wantEnd := 0
			if tt.End > tt.Line {
				wantEnd = tt.End
			}
			if ref.End != wantEnd {
				t.Fatalf("Parse(%q) end = %d, want %d", tt.Ref, ref.End, wantEnd)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	for _, tt := range loadCorpus(t).Resolve {
		t.Run(tt.Ref+fmt.Sprint(tt.Lines), func(t *testing.T) {
			ref, ok, err := anchor.Parse(tt.Ref)
			if err != nil || !ok {
				t.Fatalf("Parse(%q) = %+v, ok %v, err %v", tt.Ref, ref, ok, err)
			}
			f := anchor.FromBytes("corpus", []byte(strings.Join(tt.Lines, "\n")))
			rng, move, err := f.Resolve(ref)
			if tt.Error != "" {
				if err == nil {
					t.Fatalf("Resolve(%q) error = nil, want error", tt.Ref)
				}
				if tt.Error == "not found" && !strings.Contains(err.Error(), "not found") {
					t.Fatalf("Resolve(%q) error = %q, want to contain %q", tt.Ref, err, "not found")
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tt.Ref, err)
			}
			if rng.Start != tt.Start || rng.End != tt.End {
				t.Fatalf("Resolve(%q) range = %+v, want {Start:%d End:%d}", tt.Ref, rng, tt.Start, tt.End)
			}
			if tt.Moved {
				if move == nil {
					t.Fatalf("Resolve(%q) move = nil, want move from %d", tt.Ref, tt.From)
				}
				if move.From != tt.From {
					t.Fatalf("Resolve(%q) move.From = %d, want %d", tt.Ref, move.From, tt.From)
				}
				return
			}
			if move != nil {
				t.Fatalf("Resolve(%q) move = %+v, want nil", tt.Ref, move)
			}
		})
	}
}
