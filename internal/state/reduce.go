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

// InputValue is a human's last-write-wins text entry on an input block. Round is
// the round its enclosing top-level block was in when the entry was committed,
// stamped by the reducer.
type InputValue struct {
	Text  string `json:"text"`
	Round int    `json:"round"`
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

// RoundRecord is a closed round: the top-level blocks live at close (frozen
// copies) plus the interaction values snapshotted to those blocks' ids.
// SubmittedRevision is set only when the round closed on a submit.
type RoundRecord struct {
	Number            int                   `json:"number"`
	Title             string                `json:"title,omitempty"`
	Blocks            doc.BlockList         `json:"blocks"`
	Decisions         map[string]Decision   `json:"decisions"`
	Choices           map[string]Selection  `json:"choices"`
	Inputs            map[string]InputValue `json:"inputs"`
	Feedback          map[string][]Feedback `json:"feedback"`
	SubmittedRevision *int                  `json:"submittedRevision,omitempty"`
}

// Rounds tracks the round partition. Current is 1-based; BlockRounds maps a
// top-level block id to the round of its last agent touch; History holds the
// closed rounds in ascending order. A round is dirty when a live top-level block
// carries the current round.
type Rounds struct {
	Current      int            `json:"current"`
	CurrentTitle string         `json:"currentTitle,omitempty"`
	BlockRounds  map[string]int `json:"blockRounds"`
	History      []RoundRecord  `json:"history"`
}

// State is the full reduction: the current document, the human interactions, and
// the round partition.
type State struct {
	Doc          *doc.Doc     `json:"doc"`
	Interactions Interactions `json:"interactions"`
	Rounds       Rounds       `json:"rounds"`
}

// Reduce folds the log into a State. Events are processed in ascending Seq
// order; last-write-wins interactions resolve by that order. The document
// starts empty, so a block.upserted before any doc.replaced appends to it.
// present.closed is terminal for the reduction: any event ordered after it is a
// no-op, so a human interaction that races the close never poisons replay. The
// framework appends channel.changed presence frames into the same log, so Reduce
// skips them regardless of origin; any other unknown event type is an error.
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
		Rounds: Rounds{
			Current:     1,
			BlockRounds: map[string]int{},
			History:     []RoundRecord{},
		},
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	for _, ev := range ordered {
		if s.Interactions.Closed.Value {
			continue
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
		s.Rounds.BlockRounds = map[string]int{}
		for _, b := range s.Doc.Blocks {
			s.Rounds.BlockRounds[b.BlockID()] = s.Rounds.Current
		}
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
		s.Rounds.BlockRounds[b.BlockID()] = s.Rounds.Current
		return nil
	case "block.removed":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.remove(p.ID)
		delete(s.Rounds.BlockRounds, p.ID)
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
		s.Interactions.Inputs[p.BlockID] = InputValue{Text: p.Text, Round: s.inputRound(p.BlockID)}
		return nil
	case "submit":
		var p struct {
			Revision int `json:"revision"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Submitted = Submitted{Value: true, Revision: p.Revision}
		if s.dirty() {
			rec, err := s.closeRound(&p.Revision)
			if err != nil {
				return err
			}
			s.Rounds.History = append(s.Rounds.History, rec)
			s.Rounds.Current++
			s.Rounds.CurrentTitle = ""
		}
		return nil
	case "round.started":
		var p struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		if s.dirty() {
			rec, err := s.closeRound(nil)
			if err != nil {
				return err
			}
			s.Rounds.History = append(s.Rounds.History, rec)
			s.Rounds.Current++
		}
		s.Rounds.CurrentTitle = p.Title
		return nil
	case "channel.changed":
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

func (s *State) dirty() bool {
	for _, b := range s.Doc.Blocks {
		if s.Rounds.BlockRounds[b.BlockID()] == s.Rounds.Current {
			return true
		}
	}
	return false
}

func (s *State) closeRound(revision *int) (RoundRecord, error) {
	cur := s.Rounds.Current
	var live []doc.Block
	for _, b := range s.Doc.Blocks {
		if s.Rounds.BlockRounds[b.BlockID()] == cur {
			live = append(live, b)
		}
	}
	blocks, err := copyBlocks(live)
	if err != nil {
		return RoundRecord{}, err
	}
	ids := idsOf(blocks)
	return RoundRecord{
		Number:            cur,
		Title:             s.Rounds.CurrentTitle,
		Blocks:            blocks,
		Decisions:         filterMap(s.Interactions.Decisions, ids),
		Choices:           filterMap(s.Interactions.Choices, ids),
		Inputs:            filterMap(s.Interactions.Inputs, ids),
		Feedback:          filterFeedback(s.Interactions.Feedback, ids),
		SubmittedRevision: revision,
	}, nil
}

// inputRound resolves the round an input value belongs to: the round of its
// enclosing top-level block (the block itself when top-level, else the card one
// level up that contains it), mirroring idsOf's one-level child resolution. An
// id with no block in the doc (an orphaned interaction) falls back to the current
// round so the reduction stays total.
func (s *State) inputRound(id string) int {
	for _, b := range s.Doc.Blocks {
		if b.BlockID() == id {
			return s.stampedRound(id)
		}
		for _, child := range doc.Children(b) {
			if child.BlockID() == id {
				return s.stampedRound(b.BlockID())
			}
		}
	}
	return s.Rounds.Current
}

func (s *State) stampedRound(id string) int {
	if r, ok := s.Rounds.BlockRounds[id]; ok {
		return r
	}
	return s.Rounds.Current
}

// idsOf collects the ids of a block slice plus one level of card children,
// mirroring where interactive blocks may nest (see daemon.findBlock).
func idsOf(blocks []doc.Block) map[string]bool {
	ids := map[string]bool{}
	for _, b := range blocks {
		ids[b.BlockID()] = true
		for _, child := range doc.Children(b) {
			ids[child.BlockID()] = true
		}
	}
	return ids
}

func filterMap[T any](m map[string]T, ids map[string]bool) map[string]T {
	out := map[string]T{}
	for id, v := range m {
		if ids[id] {
			out[id] = v
		}
	}
	return out
}

func filterFeedback(m map[string][]Feedback, ids map[string]bool) map[string][]Feedback {
	out := map[string][]Feedback{}
	for id, v := range m {
		if ids[id] {
			out[id] = append([]Feedback(nil), v...)
		}
	}
	return out
}

// copyBlocks deep-copies blocks through the doc decoder so a later in-place
// mutation of the live document cannot reach a round's frozen snapshot.
func copyBlocks(blocks []doc.Block) (doc.BlockList, error) {
	data, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("marshal round blocks: %w", err)
	}
	var out doc.BlockList
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode round blocks: %w", err)
	}
	return out, nil
}
