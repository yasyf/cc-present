package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

const defaultWithdrawReason = "removed from the board"

var mirrors sync.Map

type slackWithdrawer func(ctx context.Context, waits, card, reason string) error

func ccSlackCardWithdraw(ctx context.Context, waits, card, reason string) error {
	_, err := slackcards.CCSlack(ctx, "card-answer", "--waits", waits, "--card", card, "--label", reason, "--withdrawn", "--via", card)
	return err
}

type slackMirror struct {
	append   ccd.AppendFunc
	withdraw slackWithdrawer
}

type withdrawal struct {
	card, slack, reason string
}

func withdrawnCopy(loc doc.Location) string {
	card, ok := loc.Block.(*doc.Card)
	if !ok || loc.Kind != doc.TopLevel || card.Slack == nil || card.Status == slackcards.Resolved {
		return ""
	}
	q, ok := slackcards.Question(card)
	if _, rows := q.(*doc.Triage); !ok || rows {
		return ""
	}
	return card.Slack.URL
}

func trackWithdrawals(events []state.Event, owed map[int64]withdrawal) error {
	for _, ev := range events {
		var p struct {
			ID       string `json:"id"`
			BlockID  string `json:"blockId"`
			Slack    string `json:"slack"`
			Reason   string `json:"reason"`
			Withdrew int64  `json:"withdrew"`
		}
		switch ev.Type {
		case EventBlockRemoved, EventReplyCreated:
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				return fmt.Errorf("decode %s at seq %d: %w", ev.Type, ev.Seq, err)
			}
		default:
			continue
		}
		switch {
		case ev.Type == EventBlockRemoved && p.Slack != "":
			owed[ev.Seq] = withdrawal{card: p.ID, slack: p.Slack, reason: p.Reason}
		case ev.Type == EventReplyCreated && p.Withdrew != 0:
			delete(owed, p.Withdrew)
		}
	}
	return nil
}

func (m *slackMirror) settle(ctx context.Context, db *sql.DB, subID string, owed map[int64]withdrawal) {
	for _, seq := range slices.Sorted(maps.Keys(owed)) {
		w := owed[seq]
		reason := w.reason
		if reason == "" {
			reason = defaultWithdrawReason
		}
		if err := m.withdraw(ctx, w.card, w.slack, reason); err != nil {
			slog.Warn("slack card still open after its card left the board; retrying on the board's next event", "subject", subID, "card", w.card, "slack", w.slack, "err", err)
			continue
		}
		if _, err := appendEvent(ctx, m.append, &ccevent.Event{
			SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventReplyCreated,
			Payload: mustJSON(map[string]any{"id": randomHex(4), "blockId": w.card, "md": fmt.Sprintf("Withdrawn on [Slack](%s): %s", w.slack, reason), "withdrew": seq}),
		}); err != nil {
			slog.Warn("slack card withdrawn but not recorded on the board", "subject", subID, "card", w.card, "err", err)
		}
	}
	board := boardFor(db, subID)
	board.mu.Lock()
	defer board.mu.Unlock()
	board.settling = false
	if slices.ContainsFunc(slices.Collect(maps.Keys(board.owed)), func(seq int64) bool { _, tried := owed[seq]; return !tried }) {
		board.settleOwed(ctx, db, subID)
	}
}
