package daemon

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

type snapshot struct {
	Seq      int64         `json:"seq"`
	Revision int           `json:"revision"`
	State    snapshotState `json:"state"`
}

type snapshotState struct {
	SchemaVersion int                `json:"schemaVersion"`
	Doc           *doc.Doc           `json:"doc"`
	Interactions  state.Interactions `json:"interactions"`
	Rounds        snapshotRounds     `json:"rounds"`
	Revising      state.Revising     `json:"revising"`
}

type snapshotRounds struct {
	Current      int            `json:"current"`
	CurrentTitle string         `json:"currentTitle,omitempty"`
	BlockRounds  map[string]int `json:"blockRounds"`
	History      []roundStub    `json:"history"`
}

type roundStub struct {
	Number            int                         `json:"number"`
	Title             string                      `json:"title,omitempty"`
	SubmittedRevision *int                        `json:"submittedRevision,omitempty"`
	Outline           []outlineBlock              `json:"outline"`
	Feedback          map[string][]state.Feedback `json:"feedback"`
}

type outlineBlock struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Label    string   `json:"label"`
	Children []string `json:"children,omitempty"`
}

func (rs *restServer) handleState(w http.ResponseWriter, r *http.Request) {
	st, events, ok := rs.reduceSubject(w, r)
	if !ok {
		return
	}
	snap, err := snapshotOf(&st, events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (rs *restServer) handleRounds(w http.ResponseWriter, r *http.Request) {
	st, _, ok := rs.reduceSubject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string][]state.RoundRecord{"history": st.Rounds.History})
}

func (rs *restServer) reduceSubject(w http.ResponseWriter, r *http.Request) (state.State, []state.Event, bool) {
	ref := r.URL.Query().Get("session")
	id, ok, err := rs.resolve(r.Context(), ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return state.State{}, nil, false
	}
	if !ok {
		http.Error(w, "unknown subject: "+ref, http.StatusNotFound)
		return state.State{}, nil, false
	}
	events, err := loadEvents(r.Context(), rs.db(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return state.State{}, nil, false
	}
	st, err := state.Reduce(events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return state.State{}, nil, false
	}
	return st, events, true
}

func snapshotOf(st *state.State, events []state.Event) (snapshot, error) {
	var seq int64
	revision := 0
	for _, ev := range events {
		seq = ev.Seq
		if ev.Type == EventDocReplaced {
			revision++
		}
	}
	history := make([]roundStub, len(st.Rounds.History))
	for i, record := range st.Rounds.History {
		outline := make([]outlineBlock, len(record.Blocks))
		for j, b := range record.Blocks {
			label, err := outlineLabel(b)
			if err != nil {
				return snapshot{}, err
			}
			outline[j] = outlineBlock{ID: b.BlockID(), Type: b.BlockType(), Label: label}
			if card, ok := b.(*doc.Card); ok {
				for _, child := range card.Children {
					outline[j].Children = append(outline[j].Children, child.BlockID())
				}
			}
		}
		history[i] = roundStub{
			Number:            record.Number,
			Title:             record.Title,
			SubmittedRevision: record.SubmittedRevision,
			Outline:           outline,
			Feedback:          record.Feedback,
		}
	}
	return snapshot{
		Seq:      seq,
		Revision: revision,
		State: snapshotState{
			SchemaVersion: st.SchemaVersion,
			Doc:           st.Doc,
			Interactions:  st.Interactions,
			Rounds: snapshotRounds{
				Current:      st.Rounds.Current,
				CurrentTitle: st.Rounds.CurrentTitle,
				BlockRounds:  st.Rounds.BlockRounds,
				History:      history,
			},
			Revising: st.Revising,
		},
	}, nil
}

func outlineLabel(b doc.Block) (string, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", err
	}
	for _, key := range []string{"prompt", "question", "title", "label"} {
		var s string
		if json.Unmarshal(fields[key], &s) == nil && strings.TrimSpace(s) != "" {
			return s, nil
		}
	}
	if _, ok := b.(*doc.PackBlock); ok {
		return b.BlockType(), nil
	}
	t := b.BlockType()
	return strings.ToUpper(t[:1]) + t[1:], nil
}
