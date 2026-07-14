package doc_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yasyf/cc-present/internal/doc"
)

func parse(data string) (*doc.Doc, error) {
	var d doc.Doc
	if err := json.Unmarshal([]byte(data), &d); err != nil {
		return nil, err
	}
	return &d, d.Validate(doc.NoPacks)
}

func card(id, children string) string {
	return fmt.Sprintf(`{"id":%q,"type":"card","children":[%s]}`, id, children)
}

func docWith(blocks string) string {
	return fmt.Sprintf(`{"version":1,"title":"T","blocks":[%s]}`, blocks)
}

const richDoc = `{
  "version": 1,
  "title": "Opener approvals",
  "intro": "Review each proposed opener.",
  "stats": [{"label": "Repos", "value": "26"}],
  "submit": {"label": "Apply approved", "note": "Undecided stay as-is."},
  "blocks": [
    {"id": "sec1", "type": "section", "title": "Batch one", "md": "First ten."},
    {"id": "c1", "type": "card", "title": "acme", "flagged": true, "status": "open",
     "chips": [{"label": "demo", "tone": "demo"}, {"label": "flagged", "tone": "flag"}],
     "children": [
       {"id": "a1", "type": "approval", "prompt": "Use this opener?", "allowFeedback": false},
       {"id": "ch1", "type": "choice", "prompt": "Pick a variant", "multi": true,
        "options": [{"id": "o1", "label": "Terse", "md": "A terse line."}, {"id": "o2", "label": "Warm"}]},
       {"id": "in1", "type": "input", "label": "Notes", "placeholder": "optional", "multiline": true},
       {"id": "md1", "type": "markdown", "md": "Was: old opener", "struck": true},
       {"id": "cd1", "type": "code", "lang": "go", "code": "package main", "title": "main.go"},
       {"id": "df1", "type": "diff", "diff": "@@ -1 +1 @@\n-a\n+b", "title": "README.md"},
       {"id": "img1", "type": "image", "src": "asset:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "alt": "logo", "caption": "the mark"},
       {"id": "tb1", "type": "table", "columns": [{"key": "k", "label": "Key", "align": "right"}], "rows": [{"k": "v"}]},
       {"id": "pr1", "type": "progress", "label": "Drafting", "value": 3, "max": 10, "state": "active"}
     ]}
  ]
}`

func TestValidate(t *testing.T) {
	bigData := "data:image/png;base64," + strings.Repeat("A", doc.MaxDataURIBytes)
	okData := "data:image/png;base64," + strings.Repeat("A", 100)
	bigMd := strings.Repeat("x", doc.MaxDocBytes+1)

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{"happy path rich doc", richDoc, ""},
		{"empty card children", docWith(card("c1", "")), ""},
		{"table with no rows", docWith(card("c1", `{"id":"t1","type":"table","columns":[{"key":"k","label":"K"}],"rows":[]}`)), ""},
		{"https image", docWith(card("c1", `{"id":"i1","type":"image","src":"https://x/y.png","alt":"a"}`)), ""},
		{"small data uri image", docWith(card("c1", fmt.Sprintf(`{"id":"i1","type":"image","src":%q,"alt":"a"}`, okData))), ""},
		{"leaf at top level", docWith(`{"id":"m1","type":"markdown","md":"hi"}`), ""},

		{"version not 1", `{"version":2,"title":"T","blocks":[]}`, "version must be 1"},
		{"empty title", `{"version":1,"title":"","blocks":[]}`, "title must not be empty"},
		{"presentation focus", `{"version":1,"title":"T","presentation":"focus","blocks":[]}`, ""},
		{"presentation board", `{"version":1,"title":"T","presentation":"board","blocks":[]}`, ""},
		{"presentation empty", `{"version":1,"title":"T","presentation":"","blocks":[]}`, "presentation must be"},
		{"presentation junk", `{"version":1,"title":"T","presentation":"carousel","blocks":[]}`, "presentation must be"},
		{"unknown block type", docWith(`{"id":"x1","type":"frobnicate"}`), `unknown type "frobnicate"`},
		{"empty block id", docWith(`{"id":"","type":"section","title":"S"}`), "block id must not be empty"},
		{"duplicate top-level id", docWith(`{"id":"d","type":"section","title":"A"},{"id":"d","type":"section","title":"B"}`), `duplicate block id "d"`},
		{"duplicate child vs card id", docWith(card("c1", `{"id":"c1","type":"markdown","md":"m"}`)), `duplicate block id "c1"`},
		{"card nested in card", docWith(card("c1", card("c2", ""))), `may not be a card`},
		{"section nested in card", docWith(card("c1", `{"id":"s2","type":"section","title":"S"}`)), `may not be a section`},

		{"section missing title", docWith(`{"id":"s1","type":"section"}`), "title must not be empty"},
		{"input missing label", docWith(card("c1", `{"id":"in1","type":"input"}`)), "label must not be empty"},
		{"markdown missing md", docWith(card("c1", `{"id":"m1","type":"markdown"}`)), "md must not be empty"},
		{"code missing lang", docWith(card("c1", `{"id":"cd1","type":"code","code":"x"}`)), "lang must not be empty"},
		{"code missing code", docWith(card("c1", `{"id":"cd1","type":"code","lang":"go"}`)), "code must not be empty"},
		{"diff missing diff", docWith(card("c1", `{"id":"df1","type":"diff"}`)), "diff must not be empty"},

		{"image missing alt", docWith(card("c1", `{"id":"i1","type":"image","src":"https://x/y.png"}`)), "alt must not be empty"},
		{"image bad scheme", docWith(card("c1", `{"id":"i1","type":"image","src":"ftp://x","alt":"a"}`)), "src must be https"},
		{"image bad asset sha", docWith(card("c1", `{"id":"i1","type":"image","src":"asset:zzz","alt":"a"}`)), "asset src must be"},
		{"image data uri too big", docWith(card("c1", fmt.Sprintf(`{"id":"i1","type":"image","src":%q,"alt":"a"}`, bigData))), "data URI is"},

		{"table no columns", docWith(card("c1", `{"id":"t1","type":"table","columns":[],"rows":[]}`)), "at least one column"},
		{"table column missing key", docWith(card("c1", `{"id":"t1","type":"table","columns":[{"key":"","label":"K"}],"rows":[]}`)), "column key must not be empty"},
		{"table column missing label", docWith(card("c1", `{"id":"t1","type":"table","columns":[{"key":"k","label":""}],"rows":[]}`)), "label must not be empty"},
		{"table bad align", docWith(card("c1", `{"id":"t1","type":"table","columns":[{"key":"k","label":"K","align":"center"}],"rows":[]}`)), "invalid align"},

		{"progress max zero", docWith(card("c1", `{"id":"p1","type":"progress","label":"L","value":0,"max":0}`)), "max must be > 0"},
		{"progress value out of range", docWith(card("c1", `{"id":"p1","type":"progress","label":"L","value":5,"max":3}`)), "out of range"},
		{"progress missing label", docWith(card("c1", `{"id":"p1","type":"progress","value":1,"max":3}`)), "label must not be empty"},
		{"progress bad state", docWith(card("c1", `{"id":"p1","type":"progress","label":"L","value":1,"max":3,"state":"paused"}`)), "invalid state"},

		{"choice no options", docWith(card("c1", `{"id":"ch1","type":"choice","options":[]}`)), "at least one option"},
		{"choice duplicate option id", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A"},{"id":"o","label":"B"}]}`)), `duplicate option id "o"`},
		{"choice option missing id", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"","label":"A"}]}`)), "option id must not be empty"},
		{"choice option missing label", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":""}]}`)), "label must not be empty"},
		{"choice option with hint", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","hint":"a few words"}]}`)), ""},
		{"choice option hint with newline", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","hint":"line1\nline2"}]}`)), "hint must be a single line"},

		{"choice option with facts and detail", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","facts":[{"label":"Cost","value":"$5/mo","tone":"good"},{"value":"no label"}],"detail":{"pros":["fast","cheap"],"cons":["locks in"],"md":"Full rationale.","mode":"modal"}}]}`)), ""},
		{"choice option nil detail omitted facts", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A"}]}`)), ""},
		{"fact empty value", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","facts":[{"label":"Cost","value":""}]}]}`)), `choice "ch1": option "o": fact value must not be empty`},
		{"fact value with newline", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","facts":[{"value":"line1\nline2"}]}]}`)), "fact value must be a single line"},
		{"fact label with newline", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","facts":[{"label":"a\nb","value":"x"}]}]}`)), "fact label must be a single line"},
		{"fact bad tone", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","facts":[{"value":"x","tone":"loud"}]}]}`)), `fact tone must be default, good, warn, or bad, got "loud"`},
		{"detail all empty", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{}}]}`)), `choice "ch1": option "o": detail must set at least one of pros, cons, or md`},
		{"detail empty pro", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{"pros":[""]}}]}`)), "detail pro must not be empty"},
		{"detail pro with newline", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{"pros":["a\nb"]}}]}`)), "detail pro must be a single line"},
		{"detail empty con", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{"cons":[""]}}]}`)), "detail con must not be empty"},
		{"detail con with newline", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{"cons":["a\nb"]}}]}`)), "detail con must be a single line"},
		{"detail bad mode", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o","label":"A","detail":{"md":"x","mode":"popover"}}]}`)), `detail mode must be inline or modal, got "popover"`},

		{"approval with detail", docWith(card("c1", `{"id":"a1","type":"approval","prompt":"OK?","detail":{"pros":["safe"],"md":"why"}}`)), ""},
		{"approval nil detail", docWith(card("c1", `{"id":"a1","type":"approval","prompt":"OK?"}`)), ""},
		{"approval all-empty detail", docWith(card("c1", `{"id":"a1","type":"approval","detail":{}}`)), `approval "a1": detail must set at least one of pros, cons, or md`},
		{"approval detail bad mode", docWith(card("c1", `{"id":"a1","type":"approval","detail":{"md":"x","mode":"popover"}}`)), `approval "a1": detail mode must be inline or modal, got "popover"`},

		{"card with summary", docWith(`{"id":"c1","type":"card","summary":"One crisp line.","children":[]}`), ""},
		{"card summary with newline", docWith(`{"id":"c1","type":"card","summary":"line1\nline2","children":[]}`), "summary must be a single line"},
		{"card bad status", docWith(`{"id":"c1","type":"card","status":"pending","children":[]}`), "invalid status"},
		{"card bad chip tone", docWith(`{"id":"c1","type":"card","chips":[{"label":"x","tone":"loud"}],"children":[]}`), "invalid chip tone"},

		{"doc too big", docWith(card("c1", fmt.Sprintf(`{"id":"m1","type":"markdown","md":%q}`, bigMd))), "exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(tt.doc)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("parse() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("parse() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parse() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestValidateJoinsViolations pins the all-violations behavior: three distinct
// problems across three blocks surface together as one errors.Join string, in
// document order, rather than fail-fasting on the first.
func TestValidateJoinsViolations(t *testing.T) {
	d := docWith(strings.Join([]string{
		`{"id":"s1","type":"section"}`,
		`{"id":"c1","type":"card","status":"bogus","children":[]}`,
		`{"id":"p1","type":"progress","label":"L","value":9,"max":3}`,
	}, ","))
	_, err := parse(d)
	if err == nil {
		t.Fatal("parse() error = nil, want three joined violations")
	}
	want := `section "s1": title must not be empty
card "c1": invalid status "bogus"
progress "p1": value 9 out of range [0,3]`
	if err.Error() != want {
		t.Fatalf("parse() error =\n%s\nwant\n%s", err.Error(), want)
	}
	if got := strings.Count(err.Error(), "\n"); got != 2 {
		t.Fatalf("joined error has %d newlines, want 2 (one per violation, newline-joined)", got)
	}
}

func TestRoundTrip(t *testing.T) {
	var d doc.Doc
	if err := json.Unmarshal([]byte(richDoc), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(&d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var reparsed doc.Doc
	if err := json.Unmarshal(out, &reparsed); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if err := reparsed.Validate(doc.NoPacks); err != nil {
		t.Fatalf("validate re-parsed: %v", err)
	}
	if got := len(reparsed.Blocks); got != 2 {
		t.Fatalf("blocks = %d, want 2", got)
	}
	c, ok := reparsed.Blocks[1].(*doc.Card)
	if !ok {
		t.Fatalf("block[1] type = %T, want *doc.Card", reparsed.Blocks[1])
	}
	if got := len(c.Children); got != 9 {
		t.Fatalf("card children = %d, want 9", got)
	}
}

// TestFieldRoundTrip guards the tier fields against silent loss on the marshal
// path: card.summary must survive Card.UnmarshalJSON's raw struct (which decodes
// into a shadow type and copies fields across), and option.hint must survive the
// plain Option struct's tags.
func TestFieldRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		check func(t *testing.T, c *doc.Card)
	}{
		{
			name: "card summary survives marshal",
			doc:  docWith(`{"id":"c1","type":"card","title":"acme","summary":"One crisp line.","children":[]}`),
			check: func(t *testing.T, c *doc.Card) {
				if c.Summary != "One crisp line." {
					t.Fatalf("summary = %q, want %q", c.Summary, "One crisp line.")
				}
			},
		},
		{
			name: "option hint survives marshal",
			doc:  docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"Terse","hint":"fewest words"}]}`)),
			check: func(t *testing.T, c *doc.Card) {
				ch, ok := c.Children[0].(*doc.Choice)
				if !ok {
					t.Fatalf("child[0] type = %T, want *doc.Choice", c.Children[0])
				}
				if got := ch.Options[0].Hint; got != "fewest words" {
					t.Fatalf("hint = %q, want %q", got, "fewest words")
				}
			},
		},
		{
			name: "option facts and detail survive marshal",
			doc:  docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"Terse","facts":[{"label":"Cost","value":"$5/mo","tone":"good"}],"detail":{"pros":["fast"],"cons":["locks in"],"md":"why","mode":"modal"}}]}`)),
			check: func(t *testing.T, c *doc.Card) {
				ch, ok := c.Children[0].(*doc.Choice)
				if !ok {
					t.Fatalf("child[0] type = %T, want *doc.Choice", c.Children[0])
				}
				o := ch.Options[0]
				if len(o.Facts) != 1 {
					t.Fatalf("facts = %d, want 1", len(o.Facts))
				}
				if o.Facts[0] != (doc.Fact{Label: "Cost", Value: "$5/mo", Tone: "good"}) {
					t.Fatalf("fact = %+v, want {Cost $5/mo good}", o.Facts[0])
				}
				if o.Detail == nil {
					t.Fatal("detail = nil, want set")
				}
				if got := o.Detail.Pros; len(got) != 1 || got[0] != "fast" {
					t.Fatalf("detail pros = %v, want [fast]", got)
				}
				if got := o.Detail.Cons; len(got) != 1 || got[0] != "locks in" {
					t.Fatalf("detail cons = %v, want [locks in]", got)
				}
				if o.Detail.Md != "why" || o.Detail.Mode != "modal" {
					t.Fatalf("detail md/mode = %q/%q, want why/modal", o.Detail.Md, o.Detail.Mode)
				}
			},
		},
		{
			name: "approval detail survives marshal",
			doc:  docWith(card("c1", `{"id":"a1","type":"approval","prompt":"OK?","detail":{"pros":["safe"],"md":"why"}}`)),
			check: func(t *testing.T, c *doc.Card) {
				a, ok := c.Children[0].(*doc.Approval)
				if !ok {
					t.Fatalf("child[0] type = %T, want *doc.Approval", c.Children[0])
				}
				if a.Detail == nil {
					t.Fatal("detail = nil, want set")
				}
				if got := a.Detail.Pros; len(got) != 1 || got[0] != "safe" {
					t.Fatalf("detail pros = %v, want [safe]", got)
				}
				if a.Detail.Md != "why" {
					t.Fatalf("detail md = %q, want why", a.Detail.Md)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d doc.Doc
			if err := json.Unmarshal([]byte(tt.doc), &d); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out, err := json.Marshal(&d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var reparsed doc.Doc
			if err := json.Unmarshal(out, &reparsed); err != nil {
				t.Fatalf("re-unmarshal: %v", err)
			}
			c, ok := reparsed.Blocks[0].(*doc.Card)
			if !ok {
				t.Fatalf("block[0] type = %T, want *doc.Card", reparsed.Blocks[0])
			}
			tt.check(t, c)
		})
	}
}

// TestPresentationRoundTrip guards Doc.UnmarshalJSON's pointer copy of the
// presentation hint: a present value survives decode and re-marshal, and an
// absent field stays nil and re-marshals without the key.
func TestPresentationRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		doc   string
		check func(t *testing.T, d *doc.Doc, marshaled string)
	}{
		{
			name: "board value survives decode and marshal",
			doc:  `{"version":1,"title":"T","presentation":"board","blocks":[]}`,
			check: func(t *testing.T, d *doc.Doc, marshaled string) {
				if d.Presentation == nil {
					t.Fatalf("presentation = nil, want %q", "board")
				}
				if *d.Presentation != "board" {
					t.Fatalf("presentation = %q, want %q", *d.Presentation, "board")
				}
				if !strings.Contains(marshaled, `"presentation":"board"`) {
					t.Fatalf("marshal = %s, want substring %q", marshaled, `"presentation":"board"`)
				}
			},
		},
		{
			name: "absent field stays nil and omits the key",
			doc:  `{"version":1,"title":"T","blocks":[]}`,
			check: func(t *testing.T, d *doc.Doc, marshaled string) {
				if d.Presentation != nil {
					t.Fatalf("presentation = %q, want nil", *d.Presentation)
				}
				if strings.Contains(marshaled, "presentation") {
					t.Fatalf("marshal = %s, want no presentation key", marshaled)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d doc.Doc
			if err := json.Unmarshal([]byte(tt.doc), &d); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out, err := json.Marshal(&d)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			tt.check(t, &d, string(out))
		})
	}
}
