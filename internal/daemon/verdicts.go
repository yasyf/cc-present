package daemon

import (
	"encoding/json"
	"fmt"
	"slices"
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

// ImportResult counts what the import-verdicts op recorded and what it left alone
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

// VerdictRef names one recorded verdict: a triage item or a pr-groups row.
type VerdictRef struct {
	Block string `json:"block"`
	Row   string `json:"row"`
}

// VerdictMove re-keys the verdict at From onto To; Keep leaves the source in
// place, making it a copy.
type VerdictMove struct {
	From VerdictRef `json:"from"`
	To   VerdictRef `json:"to"`
	Keep bool       `json:"keep,omitempty"`
}

func handleMoveVerdicts(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	var moves []VerdictMove
	if err := json.Unmarshal(b.Verdicts, &moves); err != nil {
		return errReply("decode moves: " + err.Error())
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
	payloads, err := moveVerdictPayloads(&st, moves, time.Now())
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
	return okReply(result{})
}

func moveVerdictPayloads(st *state.State, moves []VerdictMove, now time.Time) ([]json.RawMessage, error) {
	targets := map[VerdictRef]bool{}
	var payloads []json.RawMessage
	var clears []json.RawMessage
	for _, m := range moves {
		if m.From == m.To {
			return nil, fmt.Errorf("verdict %s/%s: source and target are the same", m.From.Block, m.From.Row)
		}
		d, ok := recordedVerdict(st, m.From)
		if !ok {
			return nil, fmt.Errorf("no verdict recorded at %s/%s", m.From.Block, m.From.Row)
		}
		if targets[m.To] {
			return nil, fmt.Errorf("verdict target %s/%s named twice", m.To.Block, m.To.Row)
		}
		targets[m.To] = true
		if _, taken := st.Interactions.Triage[m.To.Block][m.To.Row]; taken {
			return nil, fmt.Errorf("%s/%s already carries a verdict; it is never overwritten", m.To.Block, m.To.Row)
		}
		entry, err := movedEntry(st, m.To, d, now)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, mustJSON(map[string]any{"blockId": m.To.Block, "verdicts": map[string]triageEntry{m.To.Row: entry}}))
		if !m.Keep {
			clears = append(clears, mustJSON(map[string]any{"blockId": m.From.Block, "verdicts": map[string]triageEntry{m.From.Row: {Verdict: "cleared"}}}))
		}
	}
	return append(clears, payloads...), nil
}

func recordedVerdict(st *state.State, ref VerdictRef) (state.Decision, bool) {
	if d, ok := st.Interactions.Triage[ref.Block][ref.Row]; ok {
		return d, true
	}
	for i := len(st.Rounds.History) - 1; i >= 0; i-- {
		if d, ok := st.Rounds.History[i].Triage[ref.Block][ref.Row]; ok {
			return d, true
		}
	}
	return state.Decision{}, false
}

func movedEntry(st *state.State, to VerdictRef, d state.Decision, now time.Time) (triageEntry, error) {
	b, topID, err := findBlock(st.Doc, to.Block)
	if err != nil {
		return triageEntry{}, err
	}
	if err := requireCurrentRound(st, to.Block, topID); err != nil {
		return triageEntry{}, err
	}
	switch blk := b.(type) {
	case *doc.Triage:
		if !slices.ContainsFunc(blk.Items, func(it doc.Item) bool { return it.ID == to.Row }) {
			return triageEntry{}, fmt.Errorf("triage %q has no item %q", to.Block, to.Row)
		}
		if d.Note != "" && !allowsNotes(blk) {
			return triageEntry{}, fmt.Errorf("triage %q does not allow notes, and the verdict at the source has one", to.Block)
		}
		return triageEntry{Verdict: d.Verdict, Note: d.Note}, nil
	case *doc.PRGroups:
		row, ok := blk.Row(to.Row)
		if !ok {
			return triageEntry{}, fmt.Errorf("pr-groups %q has no pull request %q", to.Block, to.Row)
		}
		head, at := d.Head, d.At
		if head == "" {
			head = row.Head
		}
		if at == "" {
			at = now.UTC().Format(time.RFC3339)
		}
		return triageEntry{Verdict: d.Verdict, Note: d.Note, Head: head, At: at}, nil
	default:
		return triageEntry{}, fmt.Errorf("block %q is a %s, not a triage or pr-groups", to.Block, b.BlockType())
	}
}
