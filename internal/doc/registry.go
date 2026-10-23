package doc

import (
	"encoding/json"
	"strings"
)

// spec is the per-type behavior a block registers: how to decode it, how to
// validate it, its nested children (card only), its per-slot visuals (choice
// options, triage items), pointers to its asset srcs (image only), whether it
// is confined to the top level (section, card), and whether it asks the human
// for a decision.
type spec struct {
	decode    func(json.RawMessage, unmarshalFunc) (Block, error)
	validate  func(Block) error
	children  func(Block) []Block
	visuals   func(Block) []Block
	assetSrcs func(Block) []*string
	topOnly   bool
	decidable bool
}

var registry map[string]spec

func init() {
	registry = map[string]spec{
		"section": {
			decode:   decodeLeaf[Section],
			validate: func(b Block) error { return validateSection(b.(*Section)) },
			topOnly:  true,
		},
		"card": {
			decode:   decodeCard,
			validate: func(b Block) error { return validateCard(b.(*Card)) },
			children: func(b Block) []Block { return b.(*Card).Children },
			topOnly:  true,
		},
		"approval": {
			decode:    decodeLeaf[Approval],
			validate:  func(b Block) error { return validateApproval(b.(*Approval)) },
			decidable: true,
		},
		"choice": {
			decode:    decodeChoice,
			validate:  func(b Block) error { return validateChoice(b.(*Choice)) },
			visuals:   func(b Block) []Block { return choiceVisuals(b.(*Choice)) },
			decidable: true,
		},
		"draft": {
			decode:    decodeLeaf[Draft],
			validate:  func(b Block) error { return validateDraft(b.(*Draft)) },
			decidable: true,
		},
		"triage": {
			decode:    decodeTriage,
			validate:  func(b Block) error { return validateTriage(b.(*Triage)) },
			visuals:   func(b Block) []Block { return triageVisuals(b.(*Triage)) },
			decidable: true,
		},
		"input": {
			decode:    decodeLeaf[Input],
			validate:  func(b Block) error { return validateInput(b.(*Input)) },
			decidable: true,
		},
		"markdown": {
			decode:   decodeLeaf[Markdown],
			validate: func(b Block) error { return validateMarkdown(b.(*Markdown)) },
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

// Decidable reports whether b asks the human for a decision: an approval,
// choice, draft, triage, or input, or a pack block whose type pt marks
// interactive.
func Decidable(b Block, pt PackTypes) bool {
	if pb, ok := b.(*PackBlock); ok {
		return pt.Interactive(pb.Type)
	}
	return registry[b.BlockType()].decidable
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
