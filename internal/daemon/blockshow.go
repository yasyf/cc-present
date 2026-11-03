package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// BlockShow is one block as the board last showed it and the interactions on
// it, newest first. Top is the block's enclosing top-level block, a card for a
// card child; it is null when no write ever carried the block. OnBoard reports
// whether the block is in the current document. Rev is the board's revision at
// the read, and Write the last agent write to the enclosing top-level block.
type BlockShow struct {
	Top     json.RawMessage `json:"top"`
	OnBoard bool            `json:"onBoard"`
	Rev     int64           `json:"rev"`
	Write   *state.Write    `json:"write,omitempty"`
	Events  []BlockEvent    `json:"events"`
}

// BlockEvent is one interaction on a block: its event type, who made it, when,
// the round it landed in, and the event payload.
type BlockEvent struct {
	Type    string          `json:"type"`
	Origin  string          `json:"origin"`
	At      time.Time       `json:"at"`
	Round   int             `json:"round"`
	Payload json.RawMessage `json:"payload"`
}

func handleBlockShow(hc ccd.HandlerCtx) ccd.Reply {
	id := decodeBody(hc.Env.Body).ID
	sub, err := resolve(hc)
	if err != nil {
		return errReply(err.Error())
	}
	board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	var top doc.Block
	loc, onBoard := doc.Locate(board.st.Doc, id)
	topID := id
	if onBoard {
		top, topID = board.st.Doc.Blocks[loc.TopIndex], loc.TopID
	}
	show := BlockShow{OnBoard: onBoard, Rev: board.seq}
	if w, ok := board.st.Written(topID); ok {
		show.Write = &w
	}
	show.Top, err = json.Marshal(top)
	rounds := board.st.RoundStarts()
	board.mu.Unlock()
	if err == nil && !onBoard {
		if top, err = lastShown(hc.Ctx, hc.DB, sub.ID, id); err == nil {
			show.Top, err = json.Marshal(top)
		}
	}
	if err != nil {
		return errReply(err.Error())
	}
	ids := []string{id}
	if top != nil && top.BlockID() == id {
		for _, c := range doc.Children(top) {
			ids = append(ids, c.BlockID())
		}
	}
	if show.Events, err = blockEvents(hc.Ctx, hc.DB, sub.ID, ids, rounds); err != nil {
		return errReply(err.Error())
	}
	if top == nil && len(show.Events) == 0 {
		return errReply(fmt.Sprintf("block %q was never on this board", id))
	}
	raw, _ := json.Marshal(show)
	return okReply(result{State: raw})
}

// lastShown finds the newest write that carried id and returns the top-level
// block that held it. An update-block of a lone leaf does not say whether it
// sat in a card, so the newest such leaf is set into the newest document or
// card that held it.
func lastShown(ctx context.Context, db *sql.DB, subjectID, id string) (doc.Block, error) {
	quoted, _ := json.Marshal(id)
	events, err := queryEvents(ctx, db, `SELECT origin, type, seq, payload FROM events WHERE subject_id=? AND type IN (?,?) AND instr(payload, ?) > 0 ORDER BY seq DESC`,
		subjectID, EventDocReplaced, EventBlockUpserted, string(quoted))
	if err != nil {
		return nil, err
	}
	var leaf doc.Block
	for _, ev := range events {
		var p struct {
			Doc   json.RawMessage `json:"doc"`
			Block json.RawMessage `json:"block"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		d := &doc.Doc{}
		if ev.Type == EventDocReplaced {
			if d, err = doc.DecodeStored(p.Doc); err != nil {
				return nil, fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
			}
		} else {
			b, err := doc.DecodeStoredBlock(p.Block)
			if err != nil {
				return nil, fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
			}
			d.Blocks = []doc.Block{b}
		}
		loc, ok := doc.Locate(d, id)
		if !ok {
			continue
		}
		top := d.Blocks[loc.TopIndex]
		card, isCard := top.(*doc.Card)
		if !isCard && loc.Kind == doc.TopLevel && ev.Type == EventBlockUpserted {
			if leaf == nil {
				leaf = top
			}
			continue
		}
		switch {
		case leaf == nil:
		case loc.Kind == doc.CardChild:
			card.Children[loc.ChildIndex] = leaf
		case loc.Kind == doc.TopLevel:
			top = leaf
		}
		return top, nil
	}
	return leaf, nil
}

func blockEvents(ctx context.Context, db *sql.DB, subjectID string, ids []string, rounds state.RoundStarts) ([]BlockEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT seq, origin, type, payload, created_at FROM events WHERE subject_id=? AND type IN (?,?,?,?,?,?,?) ORDER BY seq DESC`,
		subjectID, EventDecisionCreated, EventChoiceSelected, EventFeedbackCreated, EventInputSubmitted, EventPackInteraction, EventAnnotationCreated, EventTriageDecided)
	if err != nil {
		return nil, fmt.Errorf("load interactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []BlockEvent{}
	for rows.Next() {
		var (
			e       BlockEvent
			seq     int64
			payload string
			created int64
		)
		if err := rows.Scan(&seq, &e.Origin, &e.Type, &payload, &created); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}
		var p struct {
			BlockID string `json:"blockId"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Type, err)
		}
		if !slices.Contains(ids, p.BlockID) {
			continue
		}
		e.At, e.Round, e.Payload = time.Unix(created, 0), rounds.At(seq), json.RawMessage(payload)
		out = append(out, e)
	}
	return out, rows.Err()
}
