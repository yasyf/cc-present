package state

import (
	"slices"

	"github.com/yasyf/cc-present/internal/doc"
)

// Removed is where a block stood when the agent took it off the board, by a
// remove-block or by a push that left it out: the top-level block that held it,
// that block's card title, and the round it left in.
type Removed struct {
	Top   string `json:"top"`
	Title string `json:"title,omitempty"`
	Round int    `json:"round"`
}

// RoundStarts holds the seq of the event that opened each round after the
// first, ascending.
type RoundStarts []int64

// At is the round an event at seq landed in; the submit that closes a round
// belongs to it.
func (r RoundStarts) At(seq int64) int {
	n, _ := slices.BinarySearch(r, seq)
	return n + 1
}

// RoundStarts returns the seqs that opened each round after the first.
func (s *State) RoundStarts() RoundStarts {
	return s.roundStarts
}

// RemovedAnswers maps each block no longer on the board that carries a human
// verdict, choice, write-in, note or annotation to where it last stood.
func (s *State) RemovedAnswers() map[string]Removed {
	out := map[string]Removed{}
	for id, r := range s.removed {
		if s.Interactions.carries(id) {
			out[id] = r
		}
	}
	return out
}

func (in *Interactions) carries(id string) bool {
	_, decided := in.Decisions[id]
	_, chosen := in.Choices[id]
	_, entered := in.Inputs[id]
	_, packed := in.Packs[id]
	return decided || chosen || entered || packed || len(in.Feedback[id]) > 0 || len(in.Annotations[id]) > 0 || len(in.Triage[id]) > 0
}

// RemovedBlock returns a block no longer on the board and the top-level block
// that held it.
func (s *State) RemovedBlock(id string) (doc.Block, string, bool) {
	b, ok := s.removedBlocks[id]
	if !ok {
		return nil, "", false
	}
	return b, s.removed[id].Top, true
}

func (s *State) markRemoved(top, b doc.Block) {
	at := removedFrom(top, s.Rounds.Current)
	doc.Walk(b, func(c doc.Block) {
		s.removed[c.BlockID()] = at
		s.removedBlocks[c.BlockID()] = c
	})
}

func (s *State) rewordRemoved(top doc.Block) {
	doc.Walk(top, func(c doc.Block) {
		at, held := s.removed[c.BlockID()]
		if !held || at.Top != top.BlockID() {
			return
		}
		s.removed[c.BlockID()] = removedFrom(top, at.Round)
		s.removedBlocks[c.BlockID()] = c
	})
}

func (s *State) dropMissing(before, after []doc.Block) {
	shown := map[string]bool{}
	for _, b := range after {
		doc.Walk(b, func(c doc.Block) { shown[c.BlockID()] = true })
	}
	for _, top := range before {
		at := removedFrom(top, s.Rounds.Current)
		doc.Walk(top, func(c doc.Block) {
			if !shown[c.BlockID()] {
				s.removed[c.BlockID()] = at
				s.removedBlocks[c.BlockID()] = c
			}
		})
	}
	for id := range shown {
		delete(s.removed, id)
		delete(s.removedBlocks, id)
	}
}

func removedFrom(top doc.Block, round int) Removed {
	at := Removed{Top: top.BlockID(), Round: round}
	if card, ok := top.(*doc.Card); ok {
		at.Title = card.Title
	}
	return at
}

func (s *State) startRound(seq int64) {
	s.roundStarts = append(s.roundStarts, seq)
	s.Rounds.Current++
}
