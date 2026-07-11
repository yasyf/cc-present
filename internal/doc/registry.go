package doc

import (
	"encoding/json"
	"strings"
)

// spec is the per-type behavior a block registers: how to decode it, how to
// validate it, its nested children (card only), pointers to its asset srcs
// (image only), and whether it is confined to the top level (section, card).
type spec struct {
	decode    func(json.RawMessage) (Block, error)
	validate  func(Block) error
	children  func(Block) []Block
	assetSrcs func(Block) []*string
	topOnly   bool
}

var registry = map[string]spec{
	"section": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Section{}) },
		validate: func(b Block) error { return validateSection(b.(*Section)) },
		topOnly:  true,
	},
	"card": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Card{}) },
		validate: func(b Block) error { return validateCard(b.(*Card)) },
		children: func(b Block) []Block { return b.(*Card).Children },
		topOnly:  true,
	},
	"approval": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Approval{}) },
		validate: func(b Block) error { return validateApproval(b.(*Approval)) },
	},
	"choice": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Choice{}) },
		validate: func(b Block) error { return validateChoice(b.(*Choice)) },
	},
	"input": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Input{}) },
		validate: func(b Block) error { return validateInput(b.(*Input)) },
	},
	"markdown": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Markdown{}) },
		validate: func(b Block) error { return validateMarkdown(b.(*Markdown)) },
	},
	"code": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Code{}) },
		validate: func(b Block) error { return validateCode(b.(*Code)) },
	},
	"diff": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Diff{}) },
		validate: func(b Block) error { return validateDiff(b.(*Diff)) },
	},
	"image": {
		decode:    func(data json.RawMessage) (Block, error) { return decodeInto(data, &Image{}) },
		validate:  func(b Block) error { return validateImage(b.(*Image)) },
		assetSrcs: func(b Block) []*string { return []*string{&b.(*Image).Src} },
	},
	"table": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Table{}) },
		validate: func(b Block) error { return validateTable(b.(*Table)) },
	},
	"progress": {
		decode:   func(data json.RawMessage) (Block, error) { return decodeInto(data, &Progress{}) },
		validate: func(b Block) error { return validateProgress(b.(*Progress)) },
	},
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

func assetSrcPtrs(b Block) []*string {
	sp := registry[b.BlockType()]
	var ptrs []*string
	if sp.assetSrcs != nil {
		ptrs = append(ptrs, sp.assetSrcs(b)...)
	}
	for _, child := range Children(b) {
		ptrs = append(ptrs, assetSrcPtrs(child)...)
	}
	return ptrs
}
