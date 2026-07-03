// Package state reduces the append-only event log into the document plus the
// human interaction state. Doc content and human verdicts are held separately
// and keyed by block id, so an agent re-upserting a block never clobbers a
// human's decision. The reduction is pure: replaying the log from seq 0
// reconstructs a fresh tab's state.
package state

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/yasyf/cc-present/internal/doc"
)

var validVerdict = map[string]bool{"approved": true, "rejected": true, "cleared": true}

// Event is one entry in a subject's log. Type is the reduction discriminant;
// Payload is the type-specific JSON. Reduce orders events by Seq.
type Event struct {
	Origin  string          `json:"origin"`
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload"`
}

// Decision is a human's last-write-wins verdict on a block, with an optional note.
type Decision struct {
	Verdict string `json:"verdict"`
	Note    string `json:"note,omitempty"`
}

// Selection is a human's last-write-wins option selection on a choice block.
type Selection struct {
	OptionIDs []string `json:"optionIds"`
}

// InputValue is a human's last-write-wins text entry on an input block.
type InputValue struct {
	Text string `json:"text"`
}

// Feedback is one entry in a block's append-only feedback list.
type Feedback struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Reply is one entry in a block's append-only agent reply thread.
type Reply struct {
	ID string `json:"id"`
	Md string `json:"md"`
}

// Submitted records whether a human has submitted and the last revision submitted.
type Submitted struct {
	Value    bool `json:"value"`
	Revision int  `json:"revision"`
}

// Closed records whether the agent has closed the presentation and its summary.
type Closed struct {
	Value   bool   `json:"value"`
	Summary string `json:"summary,omitempty"`
}

// Interactions holds every human interaction, keyed by block id, plus the
// submit and close signals. Decisions, choices, and inputs are last-write-wins;
// feedback and replies are append-only.
type Interactions struct {
	Decisions map[string]Decision   `json:"decisions"`
	Choices   map[string]Selection  `json:"choices"`
	Inputs    map[string]InputValue `json:"inputs"`
	Feedback  map[string][]Feedback `json:"feedback"`
	Replies   map[string][]Reply    `json:"replies"`
	Submitted Submitted             `json:"submitted"`
	Closed    Closed                `json:"closed"`
}

// State is the full reduction: the current document and the human interactions.
type State struct {
	Doc          *doc.Doc     `json:"doc"`
	Interactions Interactions `json:"interactions"`
}

// Reduce folds the log into a State. Events are processed in ascending Seq
// order; last-write-wins interactions resolve by that order. The document
// starts empty, so a block.upserted before any doc.replaced appends to it. A
// present.closed is terminal: any event ordered after it is an error. Unknown
// event types are an error.
func Reduce(events []Event) (State, error) {
	s := State{
		Doc: &doc.Doc{Version: 1, Blocks: []doc.Block{}},
		Interactions: Interactions{
			Decisions: map[string]Decision{},
			Choices:   map[string]Selection{},
			Inputs:    map[string]InputValue{},
			Feedback:  map[string][]Feedback{},
			Replies:   map[string][]Reply{},
		},
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	for _, ev := range ordered {
		if s.Interactions.Closed.Value {
			return State{}, fmt.Errorf("event %q at seq %d after present.closed: log is terminal", ev.Type, ev.Seq)
		}
		if err := s.apply(ev); err != nil {
			return State{}, fmt.Errorf("apply %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
	}
	return s, nil
}

func (s *State) apply(ev Event) error {
	switch ev.Type {
	case "doc.replaced":
		var p struct {
			Doc      *doc.Doc `json:"doc"`
			Revision int      `json:"revision"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Doc = p.Doc
		return nil
	case "block.upserted":
		var p struct {
			Block json.RawMessage `json:"block"`
			After *string         `json:"after"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		b, err := doc.DecodeBlock(p.Block)
		if err != nil {
			return err
		}
		s.upsert(b, p.After)
		return nil
	case "block.removed":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.remove(p.ID)
		return nil
	case "reply.created":
		var p struct {
			ID      string `json:"id"`
			BlockID string `json:"blockId"`
			Md      string `json:"md"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Replies[p.BlockID] = append(s.Interactions.Replies[p.BlockID], Reply{ID: p.ID, Md: p.Md})
		return nil
	case "present.closed":
		var p struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Closed = Closed{Value: true, Summary: p.Summary}
		return nil
	case "decision.created":
		var p struct {
			BlockID string `json:"blockId"`
			Verdict string `json:"verdict"`
			Note    string `json:"note"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		if !validVerdict[p.Verdict] {
			return fmt.Errorf("invalid verdict %q", p.Verdict)
		}
		if p.Verdict == "cleared" {
			delete(s.Interactions.Decisions, p.BlockID)
			return nil
		}
		s.Interactions.Decisions[p.BlockID] = Decision{Verdict: p.Verdict, Note: p.Note}
		return nil
	case "choice.selected":
		var p struct {
			BlockID   string   `json:"blockId"`
			OptionIDs []string `json:"optionIds"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		if p.OptionIDs == nil {
			p.OptionIDs = []string{}
		}
		s.Interactions.Choices[p.BlockID] = Selection{OptionIDs: p.OptionIDs}
		return nil
	case "feedback.created":
		var p struct {
			ID      string `json:"id"`
			BlockID string `json:"blockId"`
			Text    string `json:"text"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Feedback[p.BlockID] = append(s.Interactions.Feedback[p.BlockID], Feedback{ID: p.ID, Text: p.Text})
		return nil
	case "input.submitted":
		var p struct {
			BlockID string `json:"blockId"`
			Text    string `json:"text"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Inputs[p.BlockID] = InputValue{Text: p.Text}
		return nil
	case "submit":
		var p struct {
			Revision int `json:"revision"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Submitted = Submitted{Value: true, Revision: p.Revision}
		return nil
	default:
		return fmt.Errorf("unknown event type %q", ev.Type)
	}
}

func (s *State) upsert(b doc.Block, after *string) {
	id := b.BlockID()
	for i, existing := range s.Doc.Blocks {
		if existing.BlockID() == id {
			s.Doc.Blocks[i] = b
			return
		}
	}
	if after != nil {
		for i, existing := range s.Doc.Blocks {
			if existing.BlockID() == *after {
				s.insertAt(i+1, b)
				return
			}
		}
	}
	s.Doc.Blocks = append(s.Doc.Blocks, b)
}

func (s *State) insertAt(idx int, b doc.Block) {
	s.Doc.Blocks = append(s.Doc.Blocks, nil)
	copy(s.Doc.Blocks[idx+1:], s.Doc.Blocks[idx:])
	s.Doc.Blocks[idx] = b
}

func (s *State) remove(id string) {
	for i, b := range s.Doc.Blocks {
		if b.BlockID() == id {
			s.Doc.Blocks = append(s.Doc.Blocks[:i], s.Doc.Blocks[i+1:]...)
			return
		}
	}
}
