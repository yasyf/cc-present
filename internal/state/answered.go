package state

import (
	"fmt"

	"github.com/yasyf/cc-present/internal/doc"
)

// AnsweredBy reports the first decidable in b or its children (an approval,
// choice, or input) that carries a human verdict.
func (s *State) AnsweredBy(b doc.Block) (string, bool) {
	for _, id := range subtreeIDs(b) {
		_, decided := s.Interactions.Decisions[id]
		_, chosen := s.Interactions.Choices[id]
		_, entered := s.Interactions.Inputs[id]
		if decided || chosen || entered {
			return id, true
		}
	}
	return "", false
}

func subtreeIDs(b doc.Block) []string {
	children := doc.Children(b)
	ids := make([]string, 0, len(children)+1)
	ids = append(ids, b.BlockID())
	for _, c := range children {
		ids = append(ids, c.BlockID())
	}
	return ids
}

// ApplyRetiring is Apply that also keeps retired current: a block.removed adds
// the block it takes off the board when the block carries a human verdict, and
// a later write that puts the block back drops it. Applying a log in pieces with
// one retired set leaves the set Retired returns for the whole log.
func (s *State) ApplyRetiring(events []Event, retired map[string]bool) error {
	for _, ev := range ordered(events) {
		if s.Interactions.Closed.Value {
			continue
		}
		if ev.Type == "block.removed" {
			var p struct {
				payloadIdentity
				ID string `json:"id"`
			}
			if err := decodeEventPayload(ev, &p); err != nil {
				return fmt.Errorf("apply %s at seq %d: %w", ev.Type, ev.Seq, err)
			}
			if loc, ok := doc.Locate(s.Doc, p.ID); ok && !loc.Visual() {
				if _, answered := s.AnsweredBy(loc.Block); answered {
					retired[p.ID] = true
				}
			}
		}
		if err := s.apply(ev); err != nil {
			return fmt.Errorf("apply %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		for id := range retired {
			if _, ok := doc.Locate(s.Doc, id); ok {
				delete(retired, id)
			}
		}
	}
	return nil
}
