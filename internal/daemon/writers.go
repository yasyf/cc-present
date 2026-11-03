package daemon

import (
	"fmt"
	"reflect"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// Writer is who makes a board write and what it read: the writing lane, the
// board revision the write was derived from (0 when it read nothing), and
// whether the lane takes over the cards it writes from the lane that owns them.
// A keepOwner write, which the daemon alone decides on, changes nothing an owner
// drew, so any lane may make it and it never makes that lane the owner.
type Writer struct {
	Lane      string `json:"lane,omitempty"`
	Rev       int64  `json:"rev,omitempty"`
	TakeOver  bool   `json:"takeOver,omitempty"`
	keepOwner bool
}

// admit refuses w's write to the top-level blocks ids when a write landed on
// one after the revision w read, or, unless w is keepOwner, when one is a card
// another lane owns and w does not take it over. head is the board's revision
// now.
func (w Writer) admit(st *state.State, head int64, ids ...string) error {
	if w.TakeOver && w.Lane == "" {
		return fmt.Errorf("--take-over needs --lane naming the lane that takes the card over")
	}
	if w.Rev > head {
		return fmt.Errorf("--rev %d is past this board's revision %d; pass the rev a read of this board printed", w.Rev, head)
	}
	for _, id := range ids {
		if last, ok := st.Written(id); ok && w.Rev > 0 && last.Rev > w.Rev {
			return fmt.Errorf("refusing to write %q: %s wrote it at rev %d, after rev %d that you read; read the board again, rebuild this write from that read, and retry with the rev it prints",
				id, LaneName(last.Lane), last.Rev, w.Rev)
		}
	}
	if w.keepOwner {
		return nil
	}
	for _, id := range ids {
		last, _ := st.Written(id)
		if last.Owner != "" && last.Owner != w.Lane && !w.TakeOver {
			return fmt.Errorf("refusing to write card %q: lane %s owns it; %s writes it, or the lane that succeeds %s takes it over with --take-over --lane <successor>",
				id, last.Owner, last.Owner, last.Owner)
		}
	}
	return nil
}

// headOnly reports whether b rewrites a pr block already on the board with
// nothing changed but its head, the redraw any lane may make of a moved PR on a
// card another lane owns.
func headOnly(st *state.State, b doc.Block) bool {
	next, ok := b.(*doc.PR)
	if !ok {
		return false
	}
	loc, ok := doc.Locate(st.Doc, next.BlockID())
	if !ok {
		return false
	}
	prior, ok := loc.Block.(*doc.PR)
	if !ok {
		return false
	}
	moved := *prior
	moved.Head = next.Head
	return reflect.DeepEqual(&moved, next)
}

// LaneName names the lane a write recorded, or says the writer named none.
func LaneName(lane string) string {
	if lane == "" {
		return "a writer that named no lane"
	}
	return "lane " + lane
}

// changedTops is every top-level block a push of d writes over st: each block
// on either side whose subtree the push does not leave byte-identical.
func changedTops(st *state.State, d *doc.Doc, retained []string) []string {
	kept := map[string]bool{}
	for _, id := range retained {
		kept[id] = true
	}
	var ids []string
	for _, blocks := range [][]doc.Block{st.Doc.Blocks, d.Blocks} {
		for _, b := range blocks {
			if id := b.BlockID(); !kept[id] {
				kept[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}
