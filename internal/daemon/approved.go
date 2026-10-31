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
// the human although they approved them at an earlier head, with the reason the
// human gave for asking to see them again.
type Reask struct {
	PRs    []string `json:"prs,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

func (r Reask) validate() error {
	if len(r.PRs) > 0 && strings.TrimSpace(r.Reason) == "" {
		return errors.New("--owner-reask needs --reason saying why the owner asked to see the pull request again")
	}
	if len(r.PRs) == 0 && r.Reason != "" {
		return errors.New("--reason goes with --owner-reask")
	}
	return nil
}

type approval struct {
	head string
	at   string
}

func approvals(ctx context.Context, db *sql.DB, subID string) (map[string]approval, error) {
	events, err := loadEventsOfType(ctx, db, subID, EventTriageDecided)
	if err != nil {
		return nil, err
	}
	found := map[string]approval{}
	for _, ev := range events {
		var p struct {
			Verdicts map[string]triageEntry `json:"verdicts"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("read %s at seq %d: %w", ev.Type, ev.Seq, err)
		}
		for row, e := range p.Verdicts {
			if e.Verdict == "approved" && e.Head != "" {
				found[row] = approval{head: e.Head, at: e.At}
			}
		}
	}
	return found, nil
}

type approvalGate struct {
	st       *state.State
	approved map[string]approval
	reask    Reask
}

func newApprovalGate(ctx context.Context, db *sql.DB, subID string, st *state.State, reask Reask) (approvalGate, error) {
	if err := reask.validate(); err != nil {
		return approvalGate{}, err
	}
	approved, err := approvals(ctx, db, subID)
	return approvalGate{st: st, approved: approved, reask: reask}, err
}

func (g approvalGate) prior(row string) (approval, bool) {
	a, ok := g.approved[row]
	return a, ok && !slices.Contains(g.reask.PRs, row)
}

func (g approvalGate) checkRows(b doc.Block) error {
	var refusal error
	doc.Walk(b, func(b doc.Block) {
		groups, ok := b.(*doc.PRGroups)
		if !ok || refusal != nil {
			return
		}
		for _, group := range groups.Groups {
			for _, row := range group.PRs {
				id := doc.RowID(row.Number)
				a, ok := g.prior(id)
				if !ok {
					continue
				}
				if live, ok := g.st.Interactions.Triage[groups.ID][id]; ok && live.Verdict == "approved" && row.Stands(live.Head) {
					continue
				}
				refusal = fmt.Errorf("pull request #%s was approved at %s (%s), and an approved pull request never goes back in front of the owner at any head; list %s in the row's carried heads, verdict copy its approval into %q, or drop the row, and pass --owner-reask %s --reason <why> only when the owner asked to see it again",
					id, a.head, a.at, a.head, groups.ID, id)
				return
			}
		}
	})
	return refusal
}

func (g approvalGate) checkVerdicts(payloads []json.RawMessage) error {
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
			a, ok := g.prior(id)
			if !ok {
				continue
			}
			row, ok := groups.Row(id)
			if !ok || e.Verdict == "approved" && row.Stands(e.Head) {
				continue
			}
			return fmt.Errorf("pull request #%s was approved at %s (%s), and this %s verdict at %s would put its row on %q back in front of the owner; keep the approval standing on that row (verdict copy instead of move, or carry it to a head the row stands at), and pass --owner-reask %s --reason <why> only when the owner asked to see it again",
				id, a.head, a.at, e.Verdict, cmp.Or(e.Head, "no head"), p.BlockID, id)
		}
	}
	return nil
}

func refuseAskingAgain(hc ccd.HandlerCtx, subID string, st *state.State, reask Reask, payloads []json.RawMessage) error {
	gate, err := newApprovalGate(hc.Ctx, hc.DB, subID, st, reask)
	if err != nil {
		return err
	}
	return gate.checkVerdicts(payloads)
}
