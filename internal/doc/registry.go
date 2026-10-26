package doc

import (
	"encoding/json"
	"regexp"
	"strings"
)

// spec is the per-type behavior a block registers: how to decode it, how to
// validate it, its nested children (card only), its per-slot visuals (choice
// options, triage items), pointers to its asset srcs (image only), and whether it
// is confined to the top level (section, card), and the markdown fields it
// renders.
type spec struct {
	decode    func(json.RawMessage, unmarshalFunc) (Block, error)
	validate  func(Block) error
	children  func(Block) []Block
	visuals   func(Block) []Block
	assetSrcs func(Block) []*string
	markdown  func(Block, func(*string))
	topOnly   bool
}

var registry map[string]spec

func init() {
	registry = map[string]spec{
		"section": {
			decode:   decodeLeaf[Section],
			validate: func(b Block) error { return validateSection(b.(*Section)) },
			topOnly:  true,
			markdown: func(b Block, f func(*string)) { f(&b.(*Section).Md) },
		},
		"card": {
			decode:   decodeCard,
			validate: func(b Block) error { return validateCard(b.(*Card)) },
			children: func(b Block) []Block { return b.(*Card).Children },
			topOnly:  true,
			markdown: func(b Block, f func(*string)) { f(&b.(*Card).Summary) },
		},
		"approval": {
			decode:   decodeLeaf[Approval],
			validate: func(b Block) error { return validateApproval(b.(*Approval)) },
			markdown: func(b Block, f func(*string)) { detailMarkdown(b.(*Approval).Detail, f) },
		},
		"choice": {
			decode:   decodeChoice,
			validate: func(b Block) error { return validateChoice(b.(*Choice)) },
			visuals:  func(b Block) []Block { return choiceVisuals(b.(*Choice)) },
			markdown: func(b Block, f func(*string)) { choiceMarkdown(b.(*Choice), f) },
		},
		"draft": {
			decode:   decodeLeaf[Draft],
			validate: func(b Block) error { return validateDraft(b.(*Draft)) },
		},
		"triage": {
			decode:   decodeTriage,
			validate: func(b Block) error { return validateTriage(b.(*Triage)) },
			visuals:  func(b Block) []Block { return triageVisuals(b.(*Triage)) },
			markdown: func(b Block, f func(*string)) { triageMarkdown(b.(*Triage), f) },
		},
		"input": {
			decode:   decodeLeaf[Input],
			validate: func(b Block) error { return validateInput(b.(*Input)) },
		},
		"markdown": {
			decode:   decodeLeaf[Markdown],
			validate: func(b Block) error { return validateMarkdown(b.(*Markdown)) },
			markdown: func(b Block, f func(*string)) { f(&b.(*Markdown).Md) },
		},
		"code": {
			decode:   decodeLeaf[Code],
			validate: func(b Block) error { return validateCode(b.(*Code)) },
		},
		"diff": {
			decode:   decodeLeaf[Diff],
			validate: func(b Block) error { return validateDiff(b.(*Diff)) },
		},
		"diagram": {
			decode:   decodeLeaf[Diagram],
			validate: func(b Block) error { return validateDiagram(b.(*Diagram)) },
		},
		"image": {
			decode:    decodeLeaf[Image],
			validate:  func(b Block) error { return validateImage(b.(*Image)) },
			assetSrcs: func(b Block) []*string { return []*string{&b.(*Image).Src} },
		},
		"table": {
			decode:   decodeLeaf[Table],
			validate: func(b Block) error { return validateTable(b.(*Table)) },
			markdown: func(b Block, f func(*string)) { tableMarkdown(b.(*Table), f) },
		},
		"progress": {
			decode:   decodeLeaf[Progress],
			validate: func(b Block) error { return validateProgress(b.(*Progress)) },
		},
		"chart": {
			decode:   decodeLeaf[Chart],
			validate: func(b Block) error { return validateChart(b.(*Chart)) },
		},
		"term": {
			decode:   decodeLeaf[Term],
			validate: func(b Block) error { return validateTerm(b.(*Term)) },
		},
		"filetree": {
			decode:   decodeLeaf[FileTree],
			validate: func(b Block) error { return validateFileTree(b.(*FileTree)) },
		},
		"record": {
			decode:   decodeLeaf[Record],
			validate: func(b Block) error { return validateRecord(b.(*Record)) },
		},
		"pr": {
			decode:   decodeLeaf[PR],
			validate: func(b Block) error { return validatePR(b.(*PR)) },
			markdown: func(b Block, f func(*string)) { f(&b.(*PR).Summary) },
		},
		"pr-groups": {
			decode:   decodeLeaf[PRGroups],
			validate: func(b Block) error { return validatePRGroups(b.(*PRGroups)) },
			markdown: func(b Block, f func(*string)) { prGroupsMarkdown(b.(*PRGroups), f) },
		},
	}
}

// Children returns the blocks nested one level inside b — a card's children, and
// nil for every other block type.
func Children(b Block) []Block {
	sp := registry[b.BlockType()]
	if sp.children == nil {
		return nil
	}
	return sp.children(b)
}

// Visuals returns the visual leaf blocks carried by b — a choice's per-option
// visuals or a triage's per-item visuals, and nil for every other block type.
func Visuals(b Block) []Block {
	sp := registry[b.BlockType()]
	if sp.visuals == nil {
		return nil
	}
	return sp.visuals(b)
}

func choiceVisuals(c *Choice) []Block {
	var vs []Block
	for i := range c.Options {
		if c.Options[i].Visual != nil {
			vs = append(vs, c.Options[i].Visual)
		}
	}
	return vs
}

func triageVisuals(t *Triage) []Block {
	var vs []Block
	for i := range t.Items {
		if t.Items[i].Visual != nil {
			vs = append(vs, t.Items[i].Visual)
		}
	}
	return vs
}

func detailMarkdown(d *Detail, f func(*string)) {
	if d != nil {
		f(&d.Md)
	}
}

func choiceMarkdown(c *Choice, f func(*string)) {
	for i := range c.Options {
		o := &c.Options[i]
		f(&o.Hint)
		f(&o.Md)
		detailMarkdown(o.Detail, f)
	}
}

func prGroupsMarkdown(g *PRGroups, f func(*string)) {
	for i := range g.Groups {
		gr := &g.Groups[i]
		f(&gr.Summary)
		for j := range gr.PRs {
			r := &gr.PRs[j]
			for _, s := range []*string{&r.Change, &r.Plan, &r.Risk, &r.Review, &r.Blocker, &r.Md} {
				f(s)
			}
		}
	}
}

func triageMarkdown(t *Triage, f func(*string)) {
	for i := range t.Items {
		it := &t.Items[i]
		f(&it.Hint)
		f(&it.Md)
		detailMarkdown(it.Detail, f)
	}
}

func tableMarkdown(t *Table, f func(*string)) {
	for _, row := range t.Rows {
		for k, v := range row {
			f(&v)
			row[k] = v
		}
	}
}

var refTitlePattern = regexp.MustCompile(`ref:([0-9a-f]{64})`)

// RefTitle is the link title that marks a markdown link as a resolved reference
// whose hover snippet is the stored asset sha.
func RefTitle(sha string) string { return "ref:" + sha }

// RefShas returns the snippet sha of every resolved reference link in b's
// markdown and its nested blocks' markdown.
func RefShas(b Block) []string {
	var shas []string
	markdownPtrs(b, func(p *string) {
		for _, m := range refTitlePattern.FindAllStringSubmatch(*p, -1) {
			shas = append(shas, m[1])
		}
	})
	return shas
}

// VisitMarkdown calls fn with every non-empty markdown field in b and its
// nested blocks.
func VisitMarkdown(b Block, fn func(string)) {
	markdownPtrs(b, func(p *string) {
		if *p != "" {
			fn(*p)
		}
	})
}

// RewriteMarkdown replaces every non-empty markdown field in b and its nested
// blocks with the result of fn, stopping at and returning the first error.
func RewriteMarkdown(b Block, fn func(string) (string, error)) error {
	var err error
	markdownPtrs(b, func(p *string) {
		if err != nil || *p == "" {
			return
		}
		var v string
		if v, err = fn(*p); err == nil {
			*p = v
		}
	})
	return err
}

func markdownPtrs(b Block, f func(*string)) {
	Walk(b, func(b Block) {
		if sp := registry[b.BlockType()]; sp.markdown != nil {
			sp.markdown(b, f)
		}
	})
}

// AssetRefs returns the sha256 digest of every asset:<sha256> image reference in
// b and its children.
func AssetRefs(b Block) []string {
	var refs []string
	for _, p := range assetSrcPtrs(b) {
		if sha, ok := strings.CutPrefix(*p, "asset:"); ok {
			refs = append(refs, sha)
		}
	}
	return refs
}

// RewriteAssetSrcs replaces every image src in b and its children with the result
// of fn, stopping at and returning the first error fn yields.
func RewriteAssetSrcs(b Block, fn func(string) (string, error)) error {
	for _, p := range assetSrcPtrs(b) {
		v, err := fn(*p)
		if err != nil {
			return err
		}
		*p = v
	}
	return nil
}

// Walk calls fn on b, then depth-first on each of its visuals and children.
func Walk(b Block, fn func(Block)) {
	fn(b)
	for _, v := range Visuals(b) {
		Walk(v, fn)
	}
	for _, child := range Children(b) {
		Walk(child, fn)
	}
}

func assetSrcPtrs(b Block) []*string {
	var ptrs []*string
	Walk(b, func(b Block) {
		if sp := registry[b.BlockType()]; sp.assetSrcs != nil {
			ptrs = append(ptrs, sp.assetSrcs(b)...)
		}
	})
	return ptrs
}
