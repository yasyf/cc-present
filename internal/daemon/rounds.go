package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// roundIntent is the agent's declared intent when an edit adds a new top-level
// block to a round the human has already engaged: unspecified (the default,
// which the guard may reject), current (add to the round in progress), or new
// (close the round into history and open the next).
type roundIntent int

const (
	roundUnspecified roundIntent = iota
	roundCurrent
	roundNew
)

// parseRoundIntent maps the wire round declaration to a roundIntent; an empty
// string is unspecified, and any other unknown value is an error.
func parseRoundIntent(s string) (roundIntent, error) {
	switch s {
	case "":
		return roundUnspecified, nil
	case "current":
		return roundCurrent, nil
	case "new":
		return roundNew, nil
	default:
		return roundUnspecified, fmt.Errorf("unknown round intent %q; want \"current\" or \"new\"", s)
	}
}

// currentRoundTops returns the live top-level blocks stamped with the current
// round, in document order.
func currentRoundTops(st *state.State) []doc.Block {
	var tops []doc.Block
	for _, b := range st.Doc.Blocks {
		if st.Rounds.BlockRounds[b.BlockID()] == st.Rounds.Current {
			tops = append(tops, b)
		}
	}
	return tops
}

// roundDirty reports whether any live top-level block carries the current round.
func roundDirty(st *state.State) bool {
	return len(currentRoundTops(st)) > 0
}

// roundEngaged reports whether the human has recorded any interaction on a
// current-round top-level block or one of its one-level children. Feedback and
// annotations count as engagement even though they never complete a block.
func roundEngaged(st *state.State) bool {
	for _, b := range currentRoundTops(st) {
		if hasInteraction(&st.Interactions, b.BlockID()) {
			return true
		}
		for _, child := range doc.Children(b) {
			if hasInteraction(&st.Interactions, child.BlockID()) {
				return true
			}
		}
	}
	return false
}

// hasInteraction reports whether id keys any of the seven human-interaction maps.
func hasInteraction(in *state.Interactions, id string) bool {
	if _, ok := in.Decisions[id]; ok {
		return true
	}
	if _, ok := in.Choices[id]; ok {
		return true
	}
	if _, ok := in.Inputs[id]; ok {
		return true
	}
	if _, ok := in.Packs[id]; ok {
		return true
	}
	if _, ok := in.Triage[id]; ok {
		return true
	}
	if _, ok := in.Feedback[id]; ok {
		return true
	}
	_, ok := in.Annotations[id]
	return ok
}

// carryIDs returns, in document order, the id of every live current-round
// top-level block still awaiting a human interaction — the blocks that ride
// forward into the next round instead of freezing into the closed one.
func carryIDs(st *state.State, pt doc.PackTypes) []string {
	var ids []string
	for _, b := range currentRoundTops(st) {
		if awaiting(b, st, pt) {
			ids = append(ids, b.BlockID())
		}
	}
	return ids
}

// awaiting reports whether b still needs a human interaction: an approval,
// choice, or input without its recorded value; a triage until every item is
// decided; a draft always; an interactive pack block without its recorded
// interaction; and a card whenever any child is awaiting. Pure content types
// never await.
func awaiting(b doc.Block, st *state.State, pt doc.PackTypes) bool {
	id := b.BlockID()
	switch b.BlockType() {
	case "approval":
		_, ok := st.Interactions.Decisions[id]
		return !ok
	case "choice":
		selection, ok := st.Interactions.Choices[id]
		return !ok || len(selection.OptionIDs) == 0 && selection.Other == ""
	case "input":
		value, ok := st.Interactions.Inputs[id]
		return !ok || value.Text == ""
	case "triage":
		return triageAwaiting(b.(*doc.Triage), st.Interactions.Triage[id])
	case "draft":
		return true
	case "card":
		for _, child := range doc.Children(b) {
			if awaiting(child, st, pt) {
				return true
			}
		}
		return false
	default:
		if pt.Interactive(b.BlockType()) {
			_, ok := st.Interactions.Packs[id]
			return !ok
		}
		return false
	}
}

// triageAwaiting reports whether any item in t lacks a recorded verdict.
func triageAwaiting(t *doc.Triage, verdicts map[string]state.Decision) bool {
	for i := range t.Items {
		if _, ok := verdicts[t.Items[i].ID]; !ok {
			return true
		}
	}
	return false
}

// newTopIDs returns the ids of the incoming document's top-level blocks that no
// top-level block in the current document carries — the new tops a push introduces.
func newTopIDs(st *state.State, d *doc.Doc) []string {
	var ids []string
	for _, b := range d.Blocks {
		if loc, ok := doc.Locate(st.Doc, b.BlockID()); !ok || loc.Kind != doc.TopLevel {
			ids = append(ids, b.BlockID())
		}
	}
	return ids
}

// upsertNewTop reports whether upserting blk (optionally after `after`) adds a
// new top-level block, mirroring the reducer's topID computation: a Locate miss
// on the block's id that is not a card-child insertion via `after`.
func upsertNewTop(st *state.State, blk doc.Block, after string) bool {
	if _, ok := doc.Locate(st.Doc, blk.BlockID()); ok {
		return false
	}
	if loc, ok := doc.Locate(st.Doc, after); ok && loc.Kind == doc.CardChild {
		return false
	}
	return true
}

// roundGuard rejects an unspecified round intent when the op adds a new
// top-level block into a dirty round the human has already engaged, naming both
// escape hatches. It returns nil when the op may proceed as-is.
func roundGuard(st *state.State, intent roundIntent, op string, newTops []string) error {
	if intent != roundUnspecified || len(newTops) == 0 || !roundDirty(st) || !roundEngaged(st) {
		return nil
	}
	cur := st.Rounds.Current
	return fmt.Errorf("round %d is mid-review: the human has interacted with its blocks, and this %s adds new top-level block(s) %v. "+
		"Pass --round current to add them to the round in progress, or --round new to close round %d into history and open round %d (unanswered blocks carry forward and stay actionable)",
		cur, op, newTops, cur, cur+1)
}

// roundStartedPayload builds a round.started event payload with an optional
// title and the ids to carry forward into the advanced round.
func roundStartedPayload(title string, carry []string) json.RawMessage {
	return mustJSON(struct {
		Title string   `json:"title,omitempty"`
		Carry []string `json:"carry,omitempty"`
	}{Title: title, Carry: carry})
}
