package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

type answerGate struct {
	ctx      context.Context
	db       *sql.DB
	subID    string
	st       *state.State
	retired  map[string]bool
	reopen   []string
	prior    decidedGate
	touches  touches
	touching touching
}

func newAnswerGate(ctx context.Context, db *sql.DB, subID string, st *state.State, retired map[string]bool, reopen []string) answerGate {
	return answerGate{ctx: ctx, db: db, subID: subID, st: st, retired: retired, reopen: reopen}
}

func newAdmitGate(ctx context.Context, db *sql.DB, subID string, st *state.State, retired map[string]bool, reopen []string, reask Reask) (answerGate, error) {
	gate := newAnswerGate(ctx, db, subID, st, retired, reopen)
	var err error
	if gate.prior, err = newDecidedGate(ctx, db, subID, st, reask); err != nil {
		return gate, err
	}
	gate.touches, err = recentTouches(ctx, db, subID)
	return gate, err
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
	if err := g.answered(b); err != nil {
		return err
	}
	if err := g.prior.checkRows(b); err != nil {
		return err
	}
	if loc, ok := doc.Locate(g.st.Doc, b.BlockID()); ok {
		return g.touches.refuseReplacing(loc.Block, b)
	}
	return nil
}

func (g answerGate) answered(b doc.Block) error {
	decidable, held := g.holding(b)
	if !held {
		return nil
	}
	at, err := g.answeredAt(decidable)
	if err != nil {
		return err
	}
	return fmt.Errorf("block %q was answered at %s (%q carries the human's verdict) and answered blocks never come back; pass --reopen %s to ask again",
		b.BlockID(), at, decidable, b.BlockID())
}

func (g answerGate) holding(b doc.Block) (string, bool) {
	decidable, answered := g.st.AnsweredBy(b)
	return decidable, answered && !slices.Contains(g.reopen, b.BlockID()) && !slices.Contains(g.reopen, decidable)
}

func (g answerGate) redraws(shown, b doc.Block) bool {
	_, held := g.holding(b)
	return held && wordsOnly(shown, b)
}

func wordsOnly(shown, b doc.Block) bool {
	return string(mustJSON(unworded(shown))) == string(mustJSON(unworded(b)))
}

func unworded(b doc.Block) doc.Block {
	switch b := b.(type) {
	case *doc.Card:
		c := *b
		c.Title, c.Summary, c.Children = "", "", make([]doc.Block, len(b.Children))
		for i, child := range b.Children {
			c.Children[i] = unworded(child)
		}
		return &c
	case *doc.Markdown:
		m := *b
		m.Md = ""
		return &m
	case *doc.PR:
		p := *b
		p.Title, p.Change, p.Plan, p.Risk, p.Blocker = "", "", "", "", ""
		return &p
	case *doc.Image:
		i := *b
		i.Alt = ""
		return &i
	}
	return b
}

func redrawRemoved(hc ccd.HandlerCtx, subID string, board *reducedBoard, gate answerGate, b body, blk doc.Block) (ccd.Reply, bool) {
	st := &board.st
	shown, top, removed := st.RemovedBlock(blk.BlockID())
	if _, held := gate.holding(blk); !removed || top != blk.BlockID() || b.After != "" || !held {
		return ccd.Reply{}, false
	}
	doc.KeepSlack(&doc.Doc{Blocks: []doc.Block{shown}}, []doc.Block{blk})
	if !wordsOnly(shown, blk) {
		return ccd.Reply{}, false
	}
	if err := b.admit(st, board.seq, blk.BlockID()); err != nil {
		return errReply(err.Error()), true
	}
	raw := mustJSON(blk)
	seq, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventBlockRemoved,
		Payload: blockRewordedPayload(blk.BlockID(), raw, b.Writer),
	})
	if err != nil {
		return errReply(err.Error()), true
	}
	return okReply(result{Rev: seq, Reworded: raw}), true
}

func appendRedraw(hc ccd.HandlerCtx, subID string, cur *doc.Doc, blk doc.Block, w Writer) (int64, error) {
	d := *cur
	d.Blocks = doc.UpsertBlocks(cur.Blocks, blk, "")
	raw, err := json.Marshal(&d)
	if err != nil {
		return 0, err
	}
	rev, err := nextRevision(hc.Ctx, hc.DB, subID)
	if err != nil {
		return 0, err
	}
	retained := make([]string, len(d.Blocks))
	for i, top := range d.Blocks {
		retained[i] = top.BlockID()
	}
	return appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
		Payload: docRedrawnPayload(raw, rev, retained, blk.BlockID(), w),
	})
}

func (g answerGate) answeredAt(id string) (string, error) {
	events, err := loadEventsOfType(g.ctx, g.db, g.subID, EventDecisionCreated, EventChoiceSelected, EventInputSubmitted)
	if err != nil {
		return "", err
	}
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
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
	for _, b := range g.st.Doc.Blocks {
		if _, kept := doc.Locate(d, b.BlockID()); kept {
			continue
		}
		if err := g.touching.refuse(b.BlockID(), "drop"); err != nil {
			return nil, nil, err
		}
		if _, answered := g.st.AnsweredBy(b); answered {
			continue
		}
		if err := g.touches.refuse(b, "drop", true); err != nil {
			return nil, nil, err
		}
	}
	return raw, warnings, nil
}
