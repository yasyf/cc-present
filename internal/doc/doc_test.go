package doc_test

import (
	"encoding/json"
	"fmt"
	"math"
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
	bigSource := strings.Repeat("x", doc.MaxDiagramBytes+1)
	bigOutput := strings.Repeat("x", doc.MaxTermBytes+1)

	manySeries := make([]string, doc.MaxChartSeries+1)
	for i := range manySeries {
		manySeries[i] = fmt.Sprintf(`{"label":"s%d","values":[1]}`, i)
	}
	overSeriesChart := fmt.Sprintf(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[%s]}`, strings.Join(manySeries, ","))

	manyCats := make([]string, doc.MaxChartPoints+1)
	manyVals := make([]string, doc.MaxChartPoints+1)
	for i := range manyCats {
		manyCats[i] = fmt.Sprintf(`"c%d"`, i)
		manyVals[i] = "1"
	}
	overPointsChart := fmt.Sprintf(`{"id":"cht1","type":"chart","kind":"bar","categories":[%s],"series":[{"label":"S","values":[%s]}]}`, strings.Join(manyCats, ","), strings.Join(manyVals, ","))

	manyEntries := make([]string, doc.MaxTreeEntries+1)
	for i := range manyEntries {
		manyEntries[i] = fmt.Sprintf(`{"path":"f%d"}`, i)
	}
	overEntriesTree := fmt.Sprintf(`{"id":"ft1","type":"filetree","entries":[%s]}`, strings.Join(manyEntries, ","))

	manyFacts := make([]string, doc.MaxRecordFacts+1)
	for i := range manyFacts {
		manyFacts[i] = fmt.Sprintf(`{"label":"L%d","value":"v"}`, i)
	}
	overFactsRecord := fmt.Sprintf(`{"id":"rec1","type":"record","facts":[%s]}`, strings.Join(manyFacts, ","))

	seriesJSON := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = fmt.Sprintf(`{"label":"s%d","values":[1]}`, i)
		}
		return strings.Join(s, ",")
	}
	atSeriesChart := fmt.Sprintf(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[%s]}`, seriesJSON(doc.MaxChartSeries))

	atCats := make([]string, doc.MaxChartPoints)
	atVals := make([]string, doc.MaxChartPoints)
	for i := range atCats {
		atCats[i] = fmt.Sprintf(`"c%d"`, i)
		atVals[i] = "1"
	}
	atPointsChart := fmt.Sprintf(`{"id":"cht1","type":"chart","kind":"bar","categories":[%s],"series":[{"label":"S","values":[%s]}]}`, strings.Join(atCats, ","), strings.Join(atVals, ","))

	atEntries := make([]string, doc.MaxTreeEntries)
	for i := range atEntries {
		atEntries[i] = fmt.Sprintf(`{"path":"f%d"}`, i)
	}
	atEntriesTree := fmt.Sprintf(`{"id":"ft1","type":"filetree","entries":[%s]}`, strings.Join(atEntries, ","))

	atFacts := make([]string, doc.MaxRecordFacts)
	for i := range atFacts {
		atFacts[i] = fmt.Sprintf(`{"label":"L%d","value":"v"}`, i)
	}
	atFactsRecord := fmt.Sprintf(`{"id":"rec1","type":"record","facts":[%s]}`, strings.Join(atFacts, ","))

	chipsJSON := func(n int) string {
		cs := make([]string, n)
		for i := range cs {
			cs[i] = fmt.Sprintf(`{"label":"c%d"}`, i)
		}
		return strings.Join(cs, ",")
	}
	linksJSON := func(n int) string {
		ls := make([]string, n)
		for i := range ls {
			ls[i] = fmt.Sprintf(`{"label":"l%d","url":"https://x.com/%d"}`, i, i)
		}
		return strings.Join(ls, ",")
	}
	atChipsRecord := fmt.Sprintf(`{"id":"rec1","type":"record","chips":[%s],"facts":[{"label":"L","value":"x"}]}`, chipsJSON(doc.MaxRecordChips))
	overChipsRecord := fmt.Sprintf(`{"id":"rec1","type":"record","chips":[%s],"facts":[{"label":"L","value":"x"}]}`, chipsJSON(doc.MaxRecordChips+1))
	atLinksRecord := fmt.Sprintf(`{"id":"rec1","type":"record","links":[%s],"facts":[{"label":"L","value":"x"}]}`, linksJSON(doc.MaxRecordLinks))
	overLinksRecord := fmt.Sprintf(`{"id":"rec1","type":"record","links":[%s],"facts":[{"label":"L","value":"x"}]}`, linksJSON(doc.MaxRecordLinks+1))

	deepPath := strings.Repeat("a/", doc.MaxTreeDepth) + "b"      // MaxTreeDepth+1 segments
	atDepthPath := strings.Repeat("a/", doc.MaxTreeDepth-1) + "b" // MaxTreeDepth segments
	deepTree := fmt.Sprintf(`{"id":"ft1","type":"filetree","entries":[{"path":%q}]}`, deepPath)
	atDepthTree := fmt.Sprintf(`{"id":"ft1","type":"filetree","entries":[{"path":%q}]}`, atDepthPath)

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

		{"diagram at top level", docWith(`{"id":"dg1","type":"diagram","kind":"mermaid","source":"graph TD; A-->B"}`), ""},
		{"diagram in card with title", docWith(card("c1", `{"id":"dg1","type":"diagram","kind":"mermaid","source":"graph LR; A-->B","title":"Flow"}`)), ""},
		{"diagram bad kind", docWith(card("c1", `{"id":"dg1","type":"diagram","kind":"d2","source":"x"}`)), "kind must be mermaid"},
		{"diagram empty source", docWith(card("c1", `{"id":"dg1","type":"diagram","kind":"mermaid","source":""}`)), "source must not be empty"},
		{"diagram oversized source", docWith(card("c1", fmt.Sprintf(`{"id":"dg1","type":"diagram","kind":"mermaid","source":%q}`, bigSource))), "exceeds"},
		{"diagram multiline title", docWith(card("c1", `{"id":"dg1","type":"diagram","kind":"mermaid","source":"x","title":"a\nb"}`)), "title must be a single line"},

		{"choice one recommended single-select", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","recommended":true},{"id":"o2","label":"B"}]}`)), ""},
		{"choice two recommended single-select", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","recommended":true},{"id":"o2","label":"B","recommended":true}]}`)), "at most one recommended"},
		{"choice two recommended multi-select", docWith(card("c1", `{"id":"ch1","type":"choice","multi":true,"options":[{"id":"o1","label":"A","recommended":true},{"id":"o2","label":"B","recommended":true}]}`)), ""},

		{"option visual code", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"code","lang":"go","code":"x"}}]}`)), ""},
		{"option visual diagram", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"diagram","kind":"mermaid","source":"graph TD; A-->B"}}]}`)), ""},
		{"option visual disallowed type", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"approval"}}]}`)), "not an allowed visual"},
		{"option visual unknown type", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"frobnicate"}}]}`)), `unknown type "frobnicate"`},
		{"option visual invalid leaf", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"diagram","kind":"d2","source":"x"}}]}`)), "kind must be mermaid"},
		{"duplicate id via visual", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"ch1","type":"code","lang":"go","code":"x"}}]}`)), `duplicate block id "ch1"`},

		{"card empty chip label rejected", docWith(`{"id":"c1","type":"card","chips":[{"label":""}],"children":[]}`), "chip label must not be empty"},

		{"chart valid bar top-level", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["Q1","Q2"],"series":[{"label":"Rev","values":[10,20]}]}`), ""},
		{"chart valid line with title and unit", docWith(card("c1", `{"id":"cht1","type":"chart","kind":"line","title":"Latency","unit":"ms","categories":["a","b"],"series":[{"label":"p50","values":[1.5,2.5]},{"label":"p99","values":[3,4]}]}`)), ""},
		{"chart negative values allowed", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a","b"],"series":[{"label":"Delta","values":[-5,3]}]}`), ""},
		{"chart bad kind", docWith(`{"id":"cht1","type":"chart","kind":"donut","categories":["a"],"series":[{"label":"S","values":[1]}]}`), "kind must be bar or line"},
		{"chart ragged series", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a","b"],"series":[{"label":"S","values":[1]}]}`), "one per category"},
		{"chart empty categories", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":[],"series":[{"label":"S","values":[]}]}`), "at least one category"},
		{"chart empty series", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[]}`), "at least one series"},
		{"chart empty series label", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"","values":[1]}]}`), "series label must not be empty"},
		{"chart duplicate series label", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[1]},{"label":"S","values":[2]}]}`), "duplicate series label"},
		{"chart empty category", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a",""],"series":[{"label":"S","values":[1,2]}]}`), "category must not be empty"},
		{"chart duplicate category", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a","a"],"series":[{"label":"S","values":[1,2]}]}`), "duplicate category"},
		{"chart over series cap", docWith(overSeriesChart), "exceeds"},
		{"chart over points cap", docWith(overPointsChart), "exceeds"},
		{"chart at series cap accepted", docWith(atSeriesChart), ""},
		{"chart at points cap accepted", docWith(atPointsChart), ""},
		{"chart value magnitude too large", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[1e16]}]}`), "magnitude must be 0 or within"},
		{"chart value magnitude too small", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[1e-16]}]}`), "magnitude must be 0 or within"},
		{"chart value magnitude at bounds accepted", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a","b"],"series":[{"label":"S","values":[1e15,1e-15]}]}`), ""},
		{"chart zero value accepted", docWith(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[0]}]}`), ""},
		{"chart title with carriage return", docWith(`{"id":"cht1","type":"chart","kind":"bar","title":"a\rb","categories":["a"],"series":[{"label":"S","values":[1]}]}`), "title must be a single line"},

		{"term valid", docWith(`{"id":"tm1","type":"term","command":"go test ./...","output":"ok","title":"Tests"}`), ""},
		{"term output empty", docWith(`{"id":"tm1","type":"term","output":""}`), "output must not be empty"},
		{"term multiline command", docWith(`{"id":"tm1","type":"term","command":"line1\nline2","output":"x"}`), "command must be a single line"},
		{"term multiline title", docWith(`{"id":"tm1","type":"term","output":"x","title":"a\nb"}`), "title must be a single line"},
		{"term output too big", docWith(fmt.Sprintf(`{"id":"tm1","type":"term","output":%q}`, bigOutput)), "exceeds"},

		{"filetree valid", docWith(`{"id":"ft1","type":"filetree","title":"Changes","entries":[{"path":"cmd/main.go","badge":"added","note":"new entrypoint"},{"path":"internal/doc/doc.go","badge":"modified"}]}`), ""},
		{"filetree empty entries", docWith(`{"id":"ft1","type":"filetree","entries":[]}`), "at least one entry"},
		{"filetree dotdot segment", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a/../b"}]}`), "empty or dot segment"},
		{"filetree absolute path", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"/a/b"}]}`), "must be relative"},
		{"filetree trailing slash", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a/b/"}]}`), "must not end with a slash"},
		{"filetree duplicate path", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a/b"},{"path":"a/b"}]}`), "duplicate path"},
		{"filetree bad badge", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a","badge":"renamed"}]}`), "invalid badge"},
		{"filetree multiline note", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a","note":"x\ny"}]}`), "note must be a single line"},
		{"filetree over entries cap", docWith(overEntriesTree), "exceeds"},
		{"filetree at entries cap accepted", docWith(atEntriesTree), ""},
		{"filetree path too deep", docWith(deepTree), "exceeds depth"},
		{"filetree path at max depth accepted", docWith(atDepthTree), ""},
		{"filetree windows drive path rejected", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"C:/Users/x/file.txt"}]}`), "must be relative"},
		{"filetree backslash path rejected", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a\\b"}]}`), "must use forward slashes"},
		{"filetree note with carriage return", docWith(`{"id":"ft1","type":"filetree","entries":[{"path":"a","note":"x\ry"}]}`), "note must be a single line"},

		{"record valid", docWith(`{"id":"rec1","type":"record","title":"AA123","chips":[{"label":"Nonstop","tone":"flag"}],"facts":[{"label":"Cabin","value":"Business","tone":"good"},{"label":"Miles","value":"70k"}],"links":[{"label":"Book","url":"https://example.com/book"}]}`), ""},
		{"record missing facts", docWith(`{"id":"rec1","type":"record","facts":[]}`), "at least one fact"},
		{"record label-less fact rejected", docWith(`{"id":"rec1","type":"record","facts":[{"value":"x"}]}`), "fact label must not be empty"},
		{"record duplicate fact labels accepted", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"a"},{"label":"L","value":"b"}]}`), ""},
		{"record empty chip label rejected", docWith(`{"id":"rec1","type":"record","chips":[{"label":""}],"facts":[{"label":"L","value":"x"}]}`), "chip label must not be empty"},
		{"record empty link label rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"","url":"https://x.com"}]}`), "link label must not be empty"},
		{"record http link rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"http://x.com"}]}`), "must be https"},
		{"record relative link rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"/path"}]}`), "must be https"},
		{"record javascript link rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"javascript:alert(1)"}]}`), "must be https"},
		{"record over facts cap", docWith(overFactsRecord), "exceeds"},
		{"record at facts cap accepted", docWith(atFactsRecord), ""},
		{"record at chips cap accepted", docWith(atChipsRecord), ""},
		{"record over chips cap", docWith(overChipsRecord), "exceeds"},
		{"record at links cap accepted", docWith(atLinksRecord), ""},
		{"record over links cap", docWith(overLinksRecord), "exceeds"},
		{"record link out-of-range port rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"https://example.com:99999"}]}`), "invalid port"},
		{"record link valid explicit port accepted", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"https://example.com:8443/x"}]}`), ""},
		{"record link port-only authority rejected", docWith(`{"id":"rec1","type":"record","facts":[{"label":"L","value":"x"}],"links":[{"label":"Book","url":"https://:443/path"}]}`), "must be https with a host"},
		{"record title with carriage return", docWith(`{"id":"rec1","type":"record","title":"a\rb","facts":[{"label":"L","value":"x"}]}`), "title must be a single line"},
		{"term command with carriage return", docWith(`{"id":"tm1","type":"term","command":"first\rsecond","output":"x"}`), "command must be a single line"},

		{"option visual chart", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[1]}]}}]}`)), ""},
		{"option visual term", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"term","output":"ok"}}]}`)), ""},
		{"option visual filetree", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"filetree","entries":[{"path":"a"}]}}]}`)), ""},
		{"option visual record", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"record","facts":[{"label":"L","value":"x"}]}}]}`)), ""},
		{"option visual error names new types", docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"approval"}}]}`)), "chart, term, filetree, record"},

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

// TestChartNonFiniteValueRejected pins the finite-value guard on a constructed
// struct, since JSON literals cannot carry NaN or Inf. The block is decoded from
// a valid chart so its base (id/type) is set, then a single value is poisoned.
func TestChartNonFiniteValueRejected(t *testing.T) {
	tests := []struct {
		name string
		v    float64
	}{
		{"NaN", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := doc.DecodeBlock([]byte(`{"id":"cht1","type":"chart","kind":"bar","categories":["a"],"series":[{"label":"S","values":[0]}]}`))
			if err != nil {
				t.Fatalf("decode chart: %v", err)
			}
			c, ok := b.(*doc.Chart)
			if !ok {
				t.Fatalf("decoded block type = %T, want *doc.Chart", b)
			}
			c.Series[0].Values[0] = tt.v
			d := &doc.Doc{Version: 1, Title: "T", Blocks: []doc.Block{c}}
			err = d.Validate(doc.NoPacks)
			if err == nil || !strings.Contains(err.Error(), "non-finite") {
				t.Fatalf("Validate() = %v, want error containing %q", err, "non-finite")
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
		{
			name: "option recommended and visual survive marshal",
			doc:  docWith(card("c1", `{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","recommended":true,"visual":{"id":"v1","type":"diagram","kind":"mermaid","source":"graph TD; A-->B"}}]}`)),
			check: func(t *testing.T, c *doc.Card) {
				ch, ok := c.Children[0].(*doc.Choice)
				if !ok {
					t.Fatalf("child[0] type = %T, want *doc.Choice", c.Children[0])
				}
				o := ch.Options[0]
				if !o.Recommended {
					t.Fatal("recommended = false, want true")
				}
				dg, ok := o.Visual.(*doc.Diagram)
				if !ok {
					t.Fatalf("visual type = %T, want *doc.Diagram", o.Visual)
				}
				if dg.ID != "v1" || dg.Kind != "mermaid" || dg.Source != "graph TD; A-->B" {
					t.Fatalf("visual = %+v, want {v1 mermaid graph TD; A-->B}", dg)
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

// TestVisualAssetRefs pins the registry visuals hook: an asset image carried as an
// option.visual is reachable by AssetRefs, so CLI image inlining and the GC keep
// set see it. Missing the hook silently strands the ref.
func TestVisualAssetRefs(t *testing.T) {
	const sha = "asset:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	raw := card("c1", fmt.Sprintf(`{"id":"ch1","type":"choice","options":[{"id":"o1","label":"A","visual":{"id":"v1","type":"image","src":%q,"alt":"x"}}]}`, sha))
	var d doc.Doc
	if err := json.Unmarshal([]byte(docWith(raw)), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := doc.AssetRefs(d.Blocks[0])
	want := sha[len("asset:"):]
	if len(refs) != 1 || refs[0] != want {
		t.Fatalf("AssetRefs = %v, want [%s]", refs, want)
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
