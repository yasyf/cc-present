package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

type answerGate struct {
	ctx     context.Context
	db      *sql.DB
	subID   string
	st      *state.State
	events  []state.Event
	retired map[string]bool
	reopen  []string
}

func newAnswerGate(ctx context.Context, db *sql.DB, subID string, st *state.State, events []state.Event, reopen []string) (answerGate, error) {
	retired, err := state.Retired(events)
	if err != nil {
		return answerGate{}, err
	}
	return answerGate{ctx: ctx, db: db, subID: subID, st: st, events: events, retired: retired, reopen: reopen}, nil
}

func (g answerGate) strip(blocks []doc.Block) ([]doc.Block, []string) {
	warnings := make([]string, 0, len(g.retired))
	d := &doc.Doc{Blocks: blocks}
	for id := range g.retired {
		if slices.Contains(g.reopen, id) {
			continue
		}
		if _, ok := doc.Locate(d, id); !ok {
			continue
		}
		d.Blocks, _ = doc.RemoveBlock(d.Blocks, id)
		warnings = append(warnings, fmt.Sprintf("dropped block %q: it was answered and then removed; pass --reopen %s to put it back", id, id))
	}
	slices.Sort(warnings)
	return d.Blocks, warnings
}

func (g answerGate) check(b doc.Block) error {
	decidable, answered := g.st.AnsweredBy(b)
	if !answered || slices.Contains(g.reopen, b.BlockID()) || slices.Contains(g.reopen, decidable) {
		return nil
	}
	at, err := g.answeredAt(decidable)
	if err != nil {
		return err
	}
	return fmt.Errorf("block %q was answered at %s (%q carries the human's verdict) and answered blocks never come back; pass --reopen %s to ask again",
		b.BlockID(), at, decidable, b.BlockID())
}

func (g answerGate) answeredAt(id string) (string, error) {
	for i := len(g.events) - 1; i >= 0; i-- {
		ev := g.events[i]
		if ev.Type != EventDecisionCreated && ev.Type != EventChoiceSelected && ev.Type != EventInputSubmitted {
			continue
		}
		var p struct {
			BlockID string `json:"blockId"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return "", fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		if p.BlockID != id {
			continue
		}
		var created int64
		if err := g.db.QueryRowContext(g.ctx, `SELECT created_at FROM events WHERE subject_id=? AND seq=?`, g.subID, ev.Seq).Scan(&created); err != nil {
			return "", fmt.Errorf("read the time of seq %d: %w", ev.Seq, err)
		}
		return time.Unix(created, 0).Format("Jan 2 3:04 PM MST"), nil
	}
	return "", fmt.Errorf("block %q carries a verdict with no event in the log", id)
}

func (g answerGate) retiredIDs() []string {
	ids := make([]string, 0, len(g.retired))
	for id := range g.retired {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (g answerGate) admitDoc(d *doc.Doc, raw json.RawMessage) (json.RawMessage, []string, error) {
	blocks, warnings := g.strip(d.Blocks)
	if len(warnings) > 0 {
		d.Blocks = blocks
		var err error
		if raw, err = json.Marshal(d); err != nil {
			return nil, nil, err
		}
	}
	retained := retainedTopIDs(g.st, d)
	for _, b := range d.Blocks {
		if slices.Contains(retained, b.BlockID()) {
			continue
		}
		if err := g.check(b); err != nil {
			return nil, nil, err
		}
	}
	return raw, warnings, nil
}
