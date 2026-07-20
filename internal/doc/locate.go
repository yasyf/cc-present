package doc

import "slices"

// LocationKind classifies where a block sits in a document: at the top level, as
// a card's child, or as a choice option's visual.
type LocationKind int

const (
	// TopLevel is a block in Doc.Blocks.
	TopLevel LocationKind = iota
	// CardChild is a block nested one level inside a card.
	CardChild
	// OptionVisual is a leaf block carried in a choice option's visual stage.
	OptionVisual
)

// Location is a block's resolved position: its kind, the block itself, and the
// enclosing top-level block's id and index. ChildIndex is the index within its
// card (CardChild only); ChoiceID and OptionID name the carrier (OptionVisual
// only).
type Location struct {
	Kind       LocationKind
	Block      Block
	TopID      string
	TopIndex   int
	ChildIndex int
	ChoiceID   string
	OptionID   string
}

// Locate finds the block with id anywhere Doc.Validate registers one — a
// top-level block, a card child, or a choice option's visual — reporting false
// when no block carries id.
func Locate(d *Doc, id string) (Location, bool) {
	for ti, b := range d.Blocks {
		if b.BlockID() == id {
			return Location{Kind: TopLevel, Block: b, TopID: b.BlockID(), TopIndex: ti}, true
		}
		if loc, ok := locateVisual(b, id, b.BlockID(), ti); ok {
			return loc, true
		}
		for ci, child := range Children(b) {
			if child.BlockID() == id {
				return Location{Kind: CardChild, Block: child, TopID: b.BlockID(), TopIndex: ti, ChildIndex: ci}, true
			}
			if loc, ok := locateVisual(child, id, b.BlockID(), ti); ok {
				return loc, true
			}
		}
	}
	return Location{}, false
}

func locateVisual(b Block, id, topID string, topIndex int) (Location, bool) {
	ch, ok := b.(*Choice)
	if !ok {
		return Location{}, false
	}
	for i := range ch.Options {
		v := ch.Options[i].Visual
		if v != nil && v.BlockID() == id {
			return Location{
				Kind:     OptionVisual,
				Block:    v,
				TopID:    topID,
				TopIndex: topIndex,
				ChoiceID: ch.ID,
				OptionID: ch.Options[i].ID,
			}, true
		}
	}
	return Location{}, false
}

// UpsertBlocks returns blocks with blk inserted or replaced by id, never mutating
// the input. An existing id (top-level or card child) replaces the block where it
// lives, order preserved. For a new id, after names either a top-level block (blk
// lands after it) or a card child (blk becomes a new child after it); an empty or
// unknown after appends at the top level.
func UpsertBlocks(blocks []Block, blk Block, after string) []Block {
	id := blk.BlockID()
	for i, b := range blocks {
		if b.BlockID() == id {
			out := cloneBlocks(blocks)
			out[i] = blk
			return out
		}
	}
	for i, b := range blocks {
		for ci, child := range Children(b) {
			if child.BlockID() == id {
				return replaceChild(blocks, i, ci, blk)
			}
		}
	}
	if after != "" {
		for i, b := range blocks {
			if b.BlockID() == after {
				return slices.Insert(cloneBlocks(blocks), i+1, blk)
			}
		}
		for i, b := range blocks {
			for ci, child := range Children(b) {
				if child.BlockID() == after {
					return insertChild(blocks, i, ci+1, blk)
				}
			}
		}
	}
	return append(cloneBlocks(blocks), blk)
}

// RemoveBlock returns blocks with id removed plus the top-level id whose
// bookkeeping the removal restamps: a card's id when id named one of its children,
// and "" for a top-level removal or an id no block carries. Inputs are never
// mutated.
func RemoveBlock(blocks []Block, id string) ([]Block, string) {
	for i, b := range blocks {
		if b.BlockID() == id {
			return slices.Delete(cloneBlocks(blocks), i, i+1), ""
		}
	}
	for i, b := range blocks {
		for ci, child := range Children(b) {
			if child.BlockID() == id {
				out := cloneBlocks(blocks)
				out[i] = withChildren(b, func(children []Block) []Block {
					return slices.Delete(cloneBlocks(children), ci, ci+1)
				})
				return out, b.BlockID()
			}
		}
	}
	return blocks, ""
}

func replaceChild(blocks []Block, cardIdx, childIdx int, blk Block) []Block {
	out := cloneBlocks(blocks)
	out[cardIdx] = withChildren(blocks[cardIdx], func(children []Block) []Block {
		next := cloneBlocks(children)
		next[childIdx] = blk
		return next
	})
	return out
}

func insertChild(blocks []Block, cardIdx, at int, blk Block) []Block {
	out := cloneBlocks(blocks)
	out[cardIdx] = withChildren(blocks[cardIdx], func(children []Block) []Block {
		return slices.Insert(cloneBlocks(children), at, blk)
	})
	return out
}

// withChildren shallow-copies the card b, swapping its Children for fn's result;
// only a card carries children, so b is always a *Card here.
func withChildren(b Block, fn func([]Block) []Block) Block {
	c := b.(*Card)
	nc := *c
	nc.Children = fn(c.Children)
	return &nc
}

func cloneBlocks(blocks []Block) []Block {
	return append([]Block(nil), blocks...)
}
