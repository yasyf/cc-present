package daemon

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	ccd "github.com/yasyf/cc-interact/daemon"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// Reask names the pull requests a push or verdict write puts back in front of
// the human although they approved or held them at an earlier head, with the
// reason: the human asked to see it again, or the new head answers the hold.
type Reask struct {
	PRs    []string `json:"prs,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

func (r Reask) validate() error {
	if len(r.PRs) > 0 && strings.TrimSpace(r.Reason) == "" {
		return errors.New("--owner-reask needs --reason saying why the pull request goes back in front of the owner")
	}
	if len(r.PRs) == 0 && r.Reason != "" {
		return errors.New("--reason goes with --owner-reask")
	}
	return nil
}

type decided struct {
	verdict string
	head    string
	at      string
}

func (d decided) refusal(id string) string {
	if d.verdict == "rejected" {
		return fmt.Sprintf("pull request #%s was held at %s (%s), and a held pull request goes back in front of the owner only at a head that answers the hold; pass --owner-reask %s --reason <the root record naming this head as answering the hold>", id, d.head, d.at, id)
	}
	return fmt.Sprintf("pull request #%s was approved at %s (%s), and an approved pull request never goes back in front of the owner at any head; pass --owner-reask %s --reason <why> only when the owner asked to see it again", id, d.head, d.at, id)
}

func decisions(ctx context.Context, db *sql.DB, subID string) (map[string]decided, error) {
	events, err := loadEventsOfType(ctx, db, subID, EventTriageDecided)
	if err != nil {
		return nil, err
	}
	found := map[string]decided{}
	for _, ev := range events {
		var p struct {
			Verdicts map[string]triageEntry `json:"verdicts"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		for row, e := range p.Verdicts {
			if (e.Verdict == "approved" || e.Verdict == "rejected") && e.Head != "" {
				found[row] = decided{verdict: e.Verdict, head: e.Head, at: e.At}
			}
		}
	}
	return found, nil
}

type decidedGate struct {
	st      *state.State
	decided map[string]decided
	reask   Reask
}

func newDecidedGate(ctx context.Context, db *sql.DB, subID string, st *state.State, reask Reask) (decidedGate, error) {
	if err := reask.validate(); err != nil {
		return decidedGate{}, err
	}
	found, err := decisions(ctx, db, subID)
	return decidedGate{st: st, decided: found, reask: reask}, err
}

func (g decidedGate) prior(row string) (decided, bool) {
	d, ok := g.decided[row]
	return d, ok && !slices.Contains(g.reask.PRs, row)
}

func (g decidedGate) checkRows(b doc.Block) error {
	var refusal error
	doc.Walk(b, func(b doc.Block) {
		groups, ok := b.(*doc.PRGroups)
		if !ok || refusal != nil {
			return
		}
		for _, group := range groups.Groups {
			for _, row := range group.PRs {
				id := doc.RowID(row.Number)
				d, ok := g.prior(id)
				if !ok {
					continue
				}
				if live, ok := g.st.Interactions.Triage[groups.ID][id]; ok && live.Verdict == d.verdict && row.Stands(live.Head) {
					continue
				}
				refusal = fmt.Errorf("%s; to keep the row, list %s in its carried heads or verdict copy its verdict into %q, or drop the row", d.refusal(id), d.head, groups.ID)
				return
			}
		}
	})
	return refusal
}

func (g decidedGate) checkVerdicts(payloads []json.RawMessage) error {
	for _, raw := range payloads {
		var p struct {
			BlockID  string                 `json:"blockId"`
			Verdicts map[string]triageEntry `json:"verdicts"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		loc, onBoard := doc.Locate(g.st.Doc, p.BlockID)
		if !onBoard {
			continue
		}
		groups, ok := loc.Block.(*doc.PRGroups)
		if !ok {
			continue
		}
		for id, e := range p.Verdicts {
			d, ok := g.prior(id)
			if !ok {
				continue
			}
			row, ok := groups.Row(id)
			if !ok || e.Verdict == d.verdict && row.Stands(e.Head) {
				continue
			}
			return fmt.Errorf("%s; this %s verdict at %s would put its row on %q back in front of the owner, so keep the verdict standing on that row (verdict copy instead of move, or carry it to a head the row stands at)",
				d.refusal(id), e.Verdict, cmp.Or(e.Head, "no head"), p.BlockID)
		}
	}
	return nil
}

func refuseAskingAgain(hc ccd.HandlerCtx, subID string, st *state.State, reask Reask, payloads []json.RawMessage) error {
	gate, err := newDecidedGate(hc.Ctx, hc.DB, subID, st, reask)
	if err != nil {
		return err
	}
	return gate.checkVerdicts(payloads)
}
