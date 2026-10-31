package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
)

const touchHold = 30 * time.Minute

type touch struct {
	typ string
	at  time.Time
}

type touches map[string]touch

func recentTouches(ctx context.Context, db *sql.DB, subID string) (touches, error) {
	args := append([]any{subID, ccevent.OriginHuman, time.Now().Add(-touchHold).Unix()}, deltaTypes...)
	rows, err := db.QueryContext(ctx, `SELECT seq, type, payload, created_at FROM events WHERE subject_id=? AND origin=? AND created_at>=? AND type IN (?`+strings.Repeat(",?", len(deltaTypes)-1)+`) ORDER BY seq ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("load recent interactions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	held := touches{}
	for rows.Next() {
		var (
			seq     int64
			typ     string
			payload string
			created int64
		)
		if err := rows.Scan(&seq, &typ, &payload, &created); err != nil {
			return nil, fmt.Errorf("scan interaction: %w", err)
		}
		if typ == EventSubmit {
			clear(held)
			continue
		}
		var p struct {
			BlockID string `json:"blockId"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, fmt.Errorf("read %s at seq %d: %w", typ, seq, err)
		}
		held[p.BlockID] = touch{typ: typ, at: time.Unix(created, 0)}
	}
	return held, rows.Err()
}

func (t touches) refuseReplacing(cur, next doc.Block) error {
	if string(mustJSON(cur)) == string(mustJSON(next)) {
		return nil
	}
	return t.refuse(cur, "replace", false)
}

func (t touches) refuse(b doc.Block, op string, removing bool) error {
	var refusal error
	doc.Walk(b, func(inner doc.Block) {
		got, ok := t[inner.BlockID()]
		if refusal != nil || !ok || got.typ == EventTriageDecided && !removing {
			return
		}
		refusal = fmt.Errorf("refusing to %s block %q: the human's %s on %q at %s, %s ago, holds it for %d minutes unless the round is submitted; retry after %s or once the human submits",
			op, b.BlockID(), got.typ, inner.BlockID(), got.at.Format("3:04 PM MST"), time.Since(got.at).Round(time.Second), int(touchHold.Minutes()), got.at.Add(touchHold).Format("3:04 PM MST"))
	})
	return refusal
}
