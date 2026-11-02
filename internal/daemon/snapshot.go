package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

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
	rs.serveBoard(w, r, func(b *reducedBoard) (encoded, error) {
		return b.encode(&b.snapshot, func() (any, error) { return snapshotOf(&b.st, b.seq, b.revision), nil })
	})
}

func (rs *restServer) handleRounds(w http.ResponseWriter, r *http.Request) {
	rs.serveBoard(w, r, func(b *reducedBoard) (encoded, error) {
		return b.encode(&b.rounds, func() (any, error) {
			return map[string][]state.RoundRecord{"history": b.st.Rounds.History}, nil
		})
	})
}

func (rs *restServer) serveBoard(w http.ResponseWriter, r *http.Request, encode func(*reducedBoard) (encoded, error)) {
	ref := r.URL.Query().Get("session")
	id, ok, err := rs.resolve(r.Context(), ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unknown subject: "+ref, http.StatusNotFound)
		return
	}
	board, err := caughtUp(r.Context(), rs.db(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	e, err := encode(board)
	board.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Vary", "Accept-Encoding")
	w.Header().Set("ETag", fmt.Sprintf(`W/"%s.%d"`, rs.version, e.seq))
	body := e.body
	if accepts(r, "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		body = e.gz
	}
	r.Header.Del("Range")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
}

func snapshotOf(st *state.State, seq int64, revision int) snapshot {
	history := make([]roundStub, len(st.Rounds.History))
	for i, record := range st.Rounds.History {
		outline := make([]outlineBlock, len(record.Blocks))
		for j, b := range record.Blocks {
			outline[j] = outlineBlock{ID: b.BlockID(), Type: b.BlockType(), Label: outlineLabel(b)}
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
	}
}

var labelKeys = []string{"prompt", "question", "title", "label"}

func outlineLabel(b doc.Block) string {
	if pb, ok := b.(*doc.PackBlock); ok {
		for _, key := range labelKeys {
			var s string
			if json.Unmarshal(pb.Fields[key], &s) == nil && strings.TrimSpace(s) != "" {
				return s
			}
		}
		return b.BlockType()
	}
	v := reflect.ValueOf(b).Elem()
	for _, key := range labelKeys {
		for _, f := range reflect.VisibleFields(v.Type()) {
			if f.IsExported() && f.Type.Kind() == reflect.String && strings.Split(f.Tag.Get("json"), ",")[0] == key {
				if s := v.FieldByIndex(f.Index).String(); strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
	}
	t := b.BlockType()
	return strings.ToUpper(t[:1]) + t[1:]
}
