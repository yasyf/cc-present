package cli

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

// reducedFixture is a reduced-state document with a top-level markdown, a card
// nesting a choice and an approval, and a top-level input, plus interactions on
// each interactive block — the input for filterBlock's client-side filter.
const reducedFixture = `{
  "doc": {
    "version": 1,
    "title": "T",
    "blocks": [
      {"id":"m1","type":"markdown","md":"hi"},
      {"id":"card1","type":"card","children":[
        {"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]},
        {"id":"ap1","type":"approval","prompt":"ok?"}
      ]},
      {"id":"in1","type":"input","label":"Notes"}
    ]
  },
  "interactions": {
    "decisions": {"ap1": {"verdict":"approved","round":1}},
    "choices": {"ch1": {"optionIds":["o1"],"round":1}},
    "inputs": {"in1": {"text":"hello","round":1}},
    "packs": {},
    "feedback": {"ap1": [{"id":"f1","text":"x","round":1}], "ch1": [{"id":"f2","text":"y","round":1}]},
    "replies": {},
    "annotations": {"m1": [{"id":"an1","anchor":"L1","text":"note","quote":"hi"}], "in1": [{"id":"an2","anchor":"L2","text":"other","quote":"q"}]},
    "triage": {"m1": {"t1": {"verdict":"approved"}}, "in1": {"t2": {"verdict":"rejected"}}},
    "submitted": {"value":true,"revision":0},
    "closed": {"value":false}
  },
  "rounds": {"current":1,"blockRounds":{},"history":[]},
  "revising": {"blockIds":[]}
}`

func docBlockIDs(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	var dd doc.Doc
	if err := json.Unmarshal(m["doc"], &dd); err != nil {
		t.Fatalf("unmarshal doc: %v", err)
	}
	ids := make([]string, len(dd.Blocks))
	for i, b := range dd.Blocks {
		ids[i] = b.BlockID()
	}
	return ids
}

func cardChildIDs(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	var dd doc.Doc
	if err := json.Unmarshal(m["doc"], &dd); err != nil {
		t.Fatalf("unmarshal doc: %v", err)
	}
	card, ok := dd.Blocks[0].(*doc.Card)
	if !ok {
		t.Fatalf("first block is %T, want *doc.Card", dd.Blocks[0])
	}
	ids := make([]string, len(card.Children))
	for i, b := range card.Children {
		ids[i] = b.BlockID()
	}
	return ids
}

func interactionKeys(t *testing.T, raw json.RawMessage, group string) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	var inter map[string]json.RawMessage
	if err := json.Unmarshal(m["interactions"], &inter); err != nil {
		t.Fatalf("unmarshal interactions: %v", err)
	}
	var g map[string]json.RawMessage
	if err := json.Unmarshal(inter[group], &g); err != nil {
		t.Fatalf("unmarshal %s: %v", group, err)
	}
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestFilterBlockNarrowsAnnotationsAndTriage(t *testing.T) {
	got, err := filterBlock(json.RawMessage(reducedFixture), "in1")
	if err != nil {
		t.Fatalf("filterBlock() error = %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	var itx map[string]json.RawMessage
	if err := json.Unmarshal(m["interactions"], &itx); err != nil {
		t.Fatalf("unmarshal interactions: %v", err)
	}
	for _, key := range []string{"annotations", "triage"} {
		var byBlock map[string]json.RawMessage
		if err := json.Unmarshal(itx[key], &byBlock); err != nil {
			t.Fatalf("unmarshal %s: %v", key, err)
		}
		if len(byBlock) != 1 {
			t.Fatalf("%s keys = %d, want only in1", key, len(byBlock))
		}
		if _, ok := byBlock["in1"]; !ok {
			t.Fatalf("%s lost the filtered block's entries", key)
		}
	}
}

func TestFilterBlockUnknownID(t *testing.T) {
	_, err := filterBlock(json.RawMessage(reducedFixture), "nope")
	if err == nil || err.Error() != `no block "nope" in the current document` {
		t.Fatalf("filterBlock() error = %v, want unknown-block error", err)
	}
}

func TestFilterBlock(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		wantDoc       []string
		wantChildren  []string // children of the kept card, nil to skip
		wantDecisions []string
		wantChoices   []string
		wantInputs    []string
		wantFeedback  []string
	}{
		{
			name:       "top-level input keeps its own subtree and interactions",
			id:         "in1",
			wantDoc:    []string{"in1"},
			wantInputs: []string{"in1"},
		},
		{
			name:         "child choice keeps the enclosing card and its own interactions",
			id:           "ch1",
			wantDoc:      []string{"card1"},
			wantChildren: []string{"ch1", "ap1"},
			wantChoices:  []string{"ch1"},
			wantFeedback: []string{"ch1"},
		},
		{
			name:         "top-level card keeps the card but no child interactions",
			id:           "card1",
			wantDoc:      []string{"card1"},
			wantChildren: []string{"ch1", "ap1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterBlock(json.RawMessage(reducedFixture), tt.id)
			if err != nil {
				t.Fatalf("filterBlock() error = %v", err)
			}
			if ids := docBlockIDs(t, got); !slices.Equal(ids, tt.wantDoc) {
				t.Fatalf("doc block ids = %v, want %v", ids, tt.wantDoc)
			}
			if tt.wantChildren != nil {
				if ids := cardChildIDs(t, got); !slices.Equal(ids, tt.wantChildren) {
					t.Fatalf("card child ids = %v, want %v", ids, tt.wantChildren)
				}
			}
			for _, g := range []struct {
				group string
				want  []string
			}{
				{"decisions", tt.wantDecisions},
				{"choices", tt.wantChoices},
				{"inputs", tt.wantInputs},
				{"feedback", tt.wantFeedback},
			} {
				if keys := interactionKeys(t, got, g.group); !slices.Equal(keys, g.want) {
					t.Fatalf("%s keys = %v, want %v", g.group, keys, g.want)
				}
			}
		})
	}
}

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

	// A nested block/child type mismatch offsets a RawMessage sub-slice, not raw.
	nestedRaw := []byte("{\n  \"version\": 1,\n  \"title\": \"T\",\n  \"blocks\": [\n    { \"id\": \"p1\", \"type\": \"progress\", \"label\": \"x\", \"value\": \"oops\", \"max\": 10 }\n  ]\n}")
	var nd doc.Doc
	nestedErr := json.Unmarshal(nestedRaw, &nd)

	childRaw := []byte("{\n  \"version\": 1,\n  \"title\": \"T\",\n  \"blocks\": [\n    { \"id\": \"c1\", \"type\": \"card\", \"children\": [\n      { \"id\": \"p1\", \"type\": \"progress\", \"label\": \"x\", \"value\": \"oops\", \"max\": 5 }\n    ] }\n  ]\n}")
	var cd doc.Doc
	childErr := json.Unmarshal(childRaw, &cd)

	plain := errors.New("boom")

	tests := []struct {
		name string
		raw  []byte
		err  error
		want string
	}{
		{"syntax error annotated", syntaxRaw, syntaxErr, syntaxErr.Error() + " (line 1, column 8)"},
		{"envelope type error passthrough", typeRaw, typeErr, typeErr.Error()},
		{"nested block type error passthrough", nestedRaw, nestedErr, nestedErr.Error()},
		{"nested card child type error passthrough", childRaw, childErr, childErr.Error()},
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

func TestVisualNudge(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "no choices",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"m1","type":"markdown","md":"hi"}]}`,
			want: "",
		},
		{
			name: "every option carries a visual",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"code","lang":"go","code":"x"}}]}]}`,
			want: "",
		},
		{
			name: "one option with a visual is enough",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"code","lang":"go","code":"x"}},{"id":"o2","label":"B"}]}]}`,
			want: "",
		},
		{
			name: "prose-only top-level choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "hint: 1 choice ships without a visual (ch1); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "prose-only choices at top level and inside a card",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]},{"id":"c1","type":"card","children":[{"id":"ch2","type":"choice","options":[{"id":"o2","label":"B"}]}]}]}`,
			want: "hint: 2 choices ship without a visual (ch1, ch2); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "top-level diagram lead-in satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"d1","type":"diagram","kind":"mermaid","source":"graph LR\n a-->b"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "",
		},
		{
			name: "top-level image lead-in satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"i1","type":"image","src":"https://x/y.png","alt":"x"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "",
		},
		{
			name: "a context block keeps the top-level lead-in",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"d1","type":"diagram","kind":"mermaid","source":"graph LR\n a-->b"},{"id":"m1","type":"markdown","md":"hi"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "",
		},
		{
			name: "a section breaks the top-level lead-in",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"d1","type":"diagram","kind":"mermaid","source":"graph LR\n a-->b"},{"id":"s1","type":"section","title":"S"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "hint: 1 choice ships without a visual (ch1); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "a trailing diagram does not satisfy the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]},{"id":"d1","type":"diagram","kind":"mermaid","source":"graph LR\n a-->b"}]}`,
			want: "hint: 1 choice ships without a visual (ch1); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "a diagram sibling satisfies a choice inside a card",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"c1","type":"card","children":[{"id":"d1","type":"diagram","kind":"mermaid","source":"graph LR\n a-->b"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}]}`,
			want: "",
		},
		{
			name: "top-level chart lead-in satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[1]}]},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "",
		},
		{
			name: "top-level filetree lead-in satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ft1","type":"filetree","entries":[{"path":"a/b.go"}]},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "",
		},
		{
			name: "a term sibling does not satisfy the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"tm1","type":"term","output":"ok"},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "hint: 1 choice ships without a visual (ch1); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "a record sibling does not satisfy the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}]},{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A"}]}]}`,
			want: "hint: 1 choice ships without a visual (ch1); attach an option.visual or lead the card with a diagram",
		},
		{
			name: "a term option.visual satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"term","output":"ok"}}]}]}`,
			want: "",
		},
		{
			name: "a record option.visual satisfies the choice",
			raw:  `{"version":1,"title":"T","blocks":[{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"record","facts":[{"label":"L","value":"x"}]}}]}]}`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visualNudge(mustDoc(t, tt.raw)); got != tt.want {
				t.Fatalf("visualNudge() = %q, want %q", got, tt.want)
			}
		})
	}
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
