package daemon

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// PriorVerdict is a verdict the human gave on an earlier board, carried onto a
// pr-groups row so they are not asked again. Head defaults to the row's head; a
// different head records the verdict at that commit, which the board shows as
// needing a fresh tap.
type PriorVerdict struct {
	Block   string    `json:"block"`
	Row     string    `json:"row"`
	Verdict string    `json:"verdict"`
	At      time.Time `json:"at"`
	Note    string    `json:"note,omitempty"`
	Head    string    `json:"head,omitempty"`
}

// ImportResult counts what import-verdicts recorded and what it left alone
// because the row already carried a verdict.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  []string `json:"skipped,omitempty"`
}

func handleImportVerdicts(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	var priors []PriorVerdict
	if err := json.Unmarshal(b.Verdicts, &priors); err != nil {
		return errReply("decode verdicts: " + err.Error())
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	events, err := loadEvents(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	st, err := state.Reduce(events)
	if err != nil {
		return errReply(err.Error())
	}
	payloads, res, err := priorVerdictPayloads(&st, priors)
	if err != nil {
		return errReply(err.Error())
	}
	for _, p := range payloads {
		if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
			SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventTriageDecided, Payload: p,
		}); err != nil {
			return errReply(err.Error())
		}
	}
	raw, _ := json.Marshal(res)
	return okReply(result{State: raw})
}

func priorVerdictPayloads(st *state.State, priors []PriorVerdict) ([]json.RawMessage, ImportResult, error) {
	byBlock := map[string]map[string]triageEntry{}
	var res ImportResult
	for _, p := range priors {
		g, topID, err := requirePRGroups(st, p.Block)
		if err != nil {
			return nil, res, err
		}
		if err := requireCurrentRound(st, p.Block, topID); err != nil {
			return nil, res, err
		}
		row, ok := g.Row(p.Row)
		if !ok {
			return nil, res, fmt.Errorf("pr-groups %q has no pull request %q", p.Block, p.Row)
		}
		if p.Verdict != "approved" && p.Verdict != "rejected" {
			return nil, res, fmt.Errorf("pr-groups %q #%s: a prior verdict is approved or rejected, got %q", p.Block, p.Row, p.Verdict)
		}
		if p.At.IsZero() {
			return nil, res, fmt.Errorf("pr-groups %q #%s: a prior verdict needs the time it was given", p.Block, p.Row)
		}
		head := p.Head
		if head == "" {
			head = row.Head
		}
		if _, err := doc.ParseHead(head); err != nil {
			return nil, res, fmt.Errorf("pr-groups %q #%s: %w", p.Block, p.Row, err)
		}
		if len(p.Note) > maxHumanTextBytes {
			return nil, res, fmt.Errorf("pr-groups %q #%s note exceeds %d bytes", p.Block, p.Row, maxHumanTextBytes)
		}
		if _, decided := st.Interactions.Triage[p.Block][p.Row]; decided {
			res.Skipped = append(res.Skipped, p.Block+"/"+p.Row)
			continue
		}
		if byBlock[p.Block] == nil {
			byBlock[p.Block] = map[string]triageEntry{}
		}
		byBlock[p.Block][p.Row] = triageEntry{Verdict: p.Verdict, Note: p.Note, Head: head, At: p.At.UTC().Format(time.RFC3339)}
		res.Imported++
	}
	blocks := make([]string, 0, len(byBlock))
	for id := range byBlock {
		blocks = append(blocks, id)
	}
	sort.Strings(blocks)
	payloads := make([]json.RawMessage, 0, len(blocks))
	for _, id := range blocks {
		payloads = append(payloads, mustJSON(map[string]any{"blockId": id, "verdicts": byBlock[id]}))
	}
	return payloads, res, nil
}

func requirePRGroups(st *state.State, id string) (*doc.PRGroups, string, error) {
	b, topID, err := findBlock(st.Doc, id)
	if err != nil {
		return nil, "", err
	}
	g, ok := b.(*doc.PRGroups)
	if !ok {
		return nil, "", fmt.Errorf("block %q is a %s, not a pr-groups", id, b.BlockType())
	}
	return g, topID, nil
}
