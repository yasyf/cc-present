// Package state reduces the append-only event log into the document plus the
// human interaction state. Doc content and human verdicts are held separately
// and keyed by block id, so an agent re-upserting a block never clobbers a
// human's decision. The reduction is pure: replaying the log from seq 0
// reconstructs a fresh tab's state.
package state

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

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
// Other is a free-text write-in outside the authored option set; it may stand
// alone (single-select write-in) or coexist with OptionIDs (multi-select).
type Selection struct {
	OptionIDs []string `json:"optionIds"`
	Other     string   `json:"other,omitempty"`
}

// InputValue is a human's last-write-wins text entry on an input block. Round is
// the round its enclosing top-level block was in when the entry was committed,
// stamped by the reducer.
type InputValue struct {
	Text  string `json:"text"`
	Round int    `json:"round"`
}

// PackValue is a human's last-write-wins interaction on a pack block: the
// payload bytes exactly as the REST edge validated them. The reducer stays
// pack-blind — it never inspects a pack payload's shape.
type PackValue struct {
	Payload json.RawMessage `json:"payload"`
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

// Annotation is one anchored mark a human placed on a draft block. Anchor is an
// opaque content-anchor string the reducer never parses; Quote is the server-
// stamped text of the anchored lines.
type Annotation struct {
	ID     string `json:"id"`
	Anchor string `json:"anchor"`
	Text   string `json:"text"`
	Quote  string `json:"quote"`
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
// submit and close signals. Decisions, choices, inputs, and packs are
// last-write-wins; triage is last-write-wins per item; feedback and replies are
// append-only; annotations are an ordered per-block list with last-write-wins
// upsert by annotation id.
type Interactions struct {
	Decisions   map[string]Decision            `json:"decisions"`
	Choices     map[string]Selection           `json:"choices"`
	Inputs      map[string]InputValue          `json:"inputs"`
	Packs       map[string]PackValue           `json:"packs"`
	Feedback    map[string][]Feedback          `json:"feedback"`
	Replies     map[string][]Reply             `json:"replies"`
	Annotations map[string][]Annotation        `json:"annotations"`
	Triage      map[string]map[string]Decision `json:"triage"`
	Submitted   Submitted                      `json:"submitted"`
	Closed      Closed                         `json:"closed"`
}

// RoundRecord is a closed round: the top-level blocks live at close (frozen
// copies) plus the interaction values snapshotted to those blocks' ids.
// SubmittedRevision is set only when the round closed on a submit.
type RoundRecord struct {
	Number            int                            `json:"number"`
	Title             string                         `json:"title,omitempty"`
	Blocks            doc.BlockList                  `json:"blocks"`
	Decisions         map[string]Decision            `json:"decisions"`
	Choices           map[string]Selection           `json:"choices"`
	Inputs            map[string]InputValue          `json:"inputs"`
	Packs             map[string]PackValue           `json:"packs"`
	Feedback          map[string][]Feedback          `json:"feedback"`
	Annotations       map[string][]Annotation        `json:"annotations"`
	Triage            map[string]map[string]Decision `json:"triage"`
	SubmittedRevision *int                           `json:"submittedRevision,omitempty"`
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

// Revising is the agent's declared working set: the top-level block ids being
// rewritten plus an optional shared note. revising.changed replaces it wholesale;
// a block.upserted or block.removed drops its id, draining the note when the set
// empties; doc.replaced, submit, and round.started clear everything, note
// included — no announcement crosses a round boundary. It never stamps rounds.
type Revising struct {
	BlockIDs []string `json:"blockIds"`
	Note     string   `json:"note,omitempty"`
}

// State is the full reduction: the current document, the human interactions, the
// round partition, and the agent's declared revising working set.
type State struct {
	Doc          *doc.Doc     `json:"doc"`
	Interactions Interactions `json:"interactions"`
	Rounds       Rounds       `json:"rounds"`
	Revising     Revising     `json:"revising"`
}

// Reduce folds the log into a State in ascending Seq order; present.closed is
// terminal, so a racing interaction never poisons replay. Framework events in
// the shared log — channel.changed and the agent.* lifecycle — are skipped;
// any other unknown event type is an error.
func Reduce(events []Event) (State, error) {
	s := State{
		Doc: &doc.Doc{Version: 1, Blocks: []doc.Block{}},
		Interactions: Interactions{
			Decisions:   map[string]Decision{},
			Choices:     map[string]Selection{},
			Inputs:      map[string]InputValue{},
			Packs:       map[string]PackValue{},
			Feedback:    map[string][]Feedback{},
			Replies:     map[string][]Reply{},
			Annotations: map[string][]Annotation{},
			Triage:      map[string]map[string]Decision{},
		},
		Rounds: Rounds{
			Current:     1,
			BlockRounds: map[string]int{},
			History:     []RoundRecord{},
		},
		Revising: Revising{BlockIDs: []string{}},
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
	if strings.HasPrefix(ev.Type, "agent.") {
		return nil
	}
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
		s.Revising = Revising{BlockIDs: []string{}}
		return nil
	case "block.upserted":
		var p struct {
			Block json.RawMessage `json:"block"`
			After string          `json:"after"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		b, err := doc.DecodeBlock(p.Block)
		if err != nil {
			return err
		}
		topID := b.BlockID()
		if loc, ok := doc.Locate(s.Doc, b.BlockID()); ok {
			topID = loc.TopID
		} else if loc, ok := doc.Locate(s.Doc, p.After); ok && loc.Kind == doc.CardChild {
			topID = loc.TopID
		}
		s.Doc.Blocks = doc.UpsertBlocks(s.Doc.Blocks, b, p.After)
		s.Rounds.BlockRounds[topID] = s.Rounds.Current
		s.revisingOnUpsert(topID)
		return nil
	case "block.removed":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		loc, ok := doc.Locate(s.Doc, p.ID)
		if !ok || loc.Visual() {
			return nil
		}
		blocks, topID := doc.RemoveBlock(s.Doc.Blocks, p.ID)
		s.Doc.Blocks = blocks
		if loc.Kind == doc.CardChild {
			s.Rounds.BlockRounds[topID] = s.Rounds.Current
			s.revisingOnRemove(topID)
			return nil
		}
		delete(s.Rounds.BlockRounds, loc.TopID)
		s.revisingOnRemove(loc.TopID)
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
			Other     string   `json:"other"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		if p.OptionIDs == nil {
			p.OptionIDs = []string{}
		}
		s.Interactions.Choices[p.BlockID] = Selection{OptionIDs: p.OptionIDs, Other: p.Other}
		return nil
	case "revising.changed":
		var p struct {
			BlockIDs []string `json:"blockIds"`
			Note     string   `json:"note"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		if p.BlockIDs == nil {
			p.BlockIDs = []string{}
		}
		s.Revising = Revising{BlockIDs: p.BlockIDs, Note: p.Note}
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
	case "pack.interaction":
		var p struct {
			BlockID string          `json:"blockId"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Packs[p.BlockID] = PackValue{Payload: p.Payload}
		return nil
	case "annotation.created":
		var p struct {
			ID      string `json:"id"`
			BlockID string `json:"blockId"`
			Anchor  string `json:"anchor"`
			Text    string `json:"text"`
			Quote   string `json:"quote"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		ann := Annotation{ID: p.ID, Anchor: p.Anchor, Text: p.Text, Quote: p.Quote}
		list := s.Interactions.Annotations[p.BlockID]
		for i := range list {
			if list[i].ID == p.ID {
				list[i] = ann
				s.Interactions.Annotations[p.BlockID] = list
				return nil
			}
		}
		s.Interactions.Annotations[p.BlockID] = append(list, ann)
		return nil
	case "annotation.removed":
		var p struct {
			ID      string `json:"id"`
			BlockID string `json:"blockId"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		list := s.Interactions.Annotations[p.BlockID]
		for i := range list {
			if list[i].ID == p.ID {
				s.Interactions.Annotations[p.BlockID] = append(list[:i:i], list[i+1:]...)
				return nil
			}
		}
		return nil
	case "triage.decided":
		var p struct {
			BlockID  string `json:"blockId"`
			Verdicts map[string]struct {
				Verdict string `json:"verdict"`
				Note    string `json:"note"`
			} `json:"verdicts"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		block := s.Interactions.Triage[p.BlockID]
		if block == nil {
			block = map[string]Decision{}
		}
		for itemID, entry := range p.Verdicts {
			if !validVerdict[entry.Verdict] {
				return fmt.Errorf("invalid verdict %q", entry.Verdict)
			}
			if entry.Verdict == "cleared" {
				delete(block, itemID)
				continue
			}
			block[itemID] = Decision{Verdict: entry.Verdict, Note: entry.Note}
		}
		if len(block) == 0 {
			delete(s.Interactions.Triage, p.BlockID)
		} else {
			s.Interactions.Triage[p.BlockID] = block
		}
		return nil
	case "submit":
		var p struct {
			Revision int `json:"revision"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Interactions.Submitted = Submitted{Value: true, Revision: p.Revision}
		s.Revising = Revising{BlockIDs: []string{}}
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
			Title string   `json:"title"`
			Carry []string `json:"carry,omitempty"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		s.Revising = Revising{BlockIDs: []string{}}
		if s.dirty() {
			rec, err := s.closeRound(nil)
			if err != nil {
				return err
			}
			s.Rounds.History = append(s.Rounds.History, rec)
			s.Rounds.Current++
			// Skip, never error: carry comes from a daemon snapshot a concurrent
			// append may have outrun, and a reduction error is permanent replay
			// failure for the subject.
			for _, id := range p.Carry {
				if loc, ok := doc.Locate(s.Doc, id); ok && loc.Kind == doc.TopLevel {
					s.Rounds.BlockRounds[id] = s.Rounds.Current
				}
			}
		}
		s.Rounds.CurrentTitle = p.Title
		return nil
	case "channel.changed":
		return nil
	default:
		return fmt.Errorf("unknown event type %q", ev.Type)
	}
}

// revisingOnUpsert drops id from the working set as its revision lands. When the
// set empties the shared note drains too — including an upsert landing while the
// set is already empty, the doc-level note's completion signal.
func (s *State) revisingOnUpsert(id string) {
	s.dropRevising(id)
	if len(s.Revising.BlockIDs) == 0 {
		s.Revising = Revising{BlockIDs: []string{}}
	}
}

// revisingOnRemove drops id from the working set as its block is removed, draining
// the shared note only when removing a tracked id empties the set (a removal while
// the set is already empty leaves a doc-level note untouched).
func (s *State) revisingOnRemove(id string) {
	had := slices.Contains(s.Revising.BlockIDs, id)
	s.dropRevising(id)
	if had && len(s.Revising.BlockIDs) == 0 {
		s.Revising = Revising{BlockIDs: []string{}}
	}
}

func (s *State) dropRevising(id string) {
	filtered := make([]string, 0, len(s.Revising.BlockIDs))
	for _, bid := range s.Revising.BlockIDs {
		if bid != id {
			filtered = append(filtered, bid)
		}
	}
	s.Revising.BlockIDs = filtered
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
		Decisions:         filterMap(s.Interactions.Decisions, ids, identity[Decision]),
		Choices:           filterMap(s.Interactions.Choices, ids, identity[Selection]),
		Inputs:            filterMap(s.Interactions.Inputs, ids, identity[InputValue]),
		Packs:             filterMap(s.Interactions.Packs, ids, identity[PackValue]),
		Feedback:          filterMap(s.Interactions.Feedback, ids, cloneSlice[Feedback]),
		Annotations:       filterMap(s.Interactions.Annotations, ids, cloneSlice[Annotation]),
		Triage:            filterMap(s.Interactions.Triage, ids, cloneVerdicts),
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

// filterMap projects m to the block ids in ids, cloning each retained value
// through clone so a later mutation of the live interaction state cannot reach a
// round's frozen snapshot.
func filterMap[T any](m map[string]T, ids map[string]bool, clone func(T) T) map[string]T {
	out := map[string]T{}
	for id, v := range m {
		if ids[id] {
			out[id] = clone(v)
		}
	}
	return out
}

func identity[T any](v T) T { return v }

func cloneSlice[T any](s []T) []T { return append([]T(nil), s...) }

// cloneVerdicts copies a triage block's per-item verdict map; Decision is a value
// type, so a shallow copy fully detaches it from the live state.
func cloneVerdicts(m map[string]Decision) map[string]Decision {
	out := make(map[string]Decision, len(m))
	for id, d := range m {
		out[id] = d
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
