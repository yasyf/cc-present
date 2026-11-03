package state

import "github.com/yasyf/cc-present/internal/doc"

// Write is the agent write that last landed on a top-level block: the board
// revision (event seq) it landed at, the lane that wrote it, whether it took
// the block off the board, and for a card the lane that owns it and the lane it
// last took the card over from.
type Write struct {
	Rev       int64  `json:"rev"`
	Lane      string `json:"lane,omitempty"`
	Removed   bool   `json:"removed,omitempty"`
	Owner     string `json:"owner,omitempty"`
	TakenFrom string `json:"takenFrom,omitempty"`
}

// Written returns the last agent write to top-level block id, including the
// write that removed it.
func (s *State) Written(id string) (Write, bool) {
	w, ok := s.writes[id]
	return w, ok
}

// UpsertTop is the top-level block an upsert of b after `after` lands in: the
// block that holds b now, else the card `after` names a child of, else b itself.
func (s *State) UpsertTop(b doc.Block, after string) string {
	if loc, ok := doc.Locate(s.Doc, b.BlockID()); ok {
		return loc.TopID
	}
	if loc, ok := doc.Locate(s.Doc, after); ok && loc.Kind == doc.CardChild {
		return loc.TopID
	}
	return b.BlockID()
}

type writer struct {
	Lane      string `json:"lane"`
	TakeOver  bool   `json:"takeOver"`
	KeepOwner bool   `json:"keepOwner"`
}

func (s *State) wrote(id string, seq int64, by writer, top doc.Block) {
	w := Write{Rev: seq, Lane: by.Lane}
	if _, card := top.(*doc.Card); card {
		prior := s.writes[id]
		w.Owner, w.TakenFrom = prior.Owner, prior.TakenFrom
		switch {
		case by.KeepOwner:
		case prior.Owner == "":
			w.Owner = by.Lane
		case by.TakeOver && by.Lane != prior.Owner:
			w.Owner, w.TakenFrom = by.Lane, prior.Owner
		}
	}
	s.writes[id] = w
}

func (s *State) wroteDoc(seq int64, by writer, before, after []doc.Block, retained []string) {
	kept := map[string]bool{}
	for _, id := range retained {
		kept[id] = true
	}
	shown := map[string]bool{}
	for _, b := range after {
		shown[b.BlockID()] = true
		if !kept[b.BlockID()] {
			s.wrote(b.BlockID(), seq, by, b)
		}
	}
	for _, b := range before {
		if !shown[b.BlockID()] {
			s.writes[b.BlockID()] = Write{Rev: seq, Lane: by.Lane, Removed: true}
		}
	}
}
