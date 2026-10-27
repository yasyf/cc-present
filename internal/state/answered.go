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

// Retired replays the log and returns the ids of blocks a block.removed took
// off the board while they carried a human verdict and that no later write has
// put back.
func Retired(events []Event) (map[string]bool, error) {
	s := newState()
	retired := map[string]bool{}
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
				return nil, fmt.Errorf("apply %s at seq %d: %w", ev.Type, ev.Seq, err)
			}
			if loc, ok := doc.Locate(s.Doc, p.ID); ok && !loc.Visual() {
				if _, answered := s.AnsweredBy(loc.Block); answered {
					retired[p.ID] = true
				}
			}
		}
		if err := s.apply(ev); err != nil {
			return nil, fmt.Errorf("apply %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		for id := range retired {
			if _, ok := doc.Locate(s.Doc, id); ok {
				delete(retired, id)
			}
		}
	}
	return retired, nil
}
