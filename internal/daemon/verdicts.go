package daemon

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

// PriorVerdict is a verdict the human gave on an earlier board, carried onto a
// pr-groups row so they are not asked again. Head defaults to the row's head; a
// different head records the verdict at that commit, which the board shows as
// needing a fresh tap. CarriedFrom names the head the human tapped when it
// differs from Head, and CarryNote says why the tap still stands there.
type PriorVerdict struct {
	Block       string    `json:"block"`
	Row         string    `json:"row"`
	Verdict     string    `json:"verdict"`
	At          time.Time `json:"at"`
	Note        string    `json:"note,omitempty"`
	Head        string    `json:"head,omitempty"`
	CarriedFrom string    `json:"carriedFrom,omitempty"`
	CarryNote   string    `json:"carryNote,omitempty"`
}

// ImportResult counts what the import-verdicts op recorded, the rows it left
// alone because they already carried a verdict, and, under replace, the rows
// whose verdict it overwrote.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  []string `json:"skipped,omitempty"`
	Replaced []string `json:"replaced,omitempty"`
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
	payloads, res, err := priorVerdictPayloads(&st, priors, b.Replace)
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

func priorVerdictPayloads(st *state.State, priors []PriorVerdict, replace bool) ([]json.RawMessage, ImportResult, error) {
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
		if (p.CarriedFrom == "") != (p.CarryNote == "") {
			return nil, res, fmt.Errorf("pr-groups %q #%s: carriedFrom and carryNote come together", p.Block, p.Row)
		}
		if p.CarriedFrom != "" {
			if _, err := doc.ParseHead(p.CarriedFrom); err != nil {
				return nil, res, fmt.Errorf("pr-groups %q #%s carriedFrom: %w", p.Block, p.Row, err)
			}
		}
		if len(p.Note) > maxHumanTextBytes || len(p.CarryNote) > maxHumanTextBytes {
			return nil, res, fmt.Errorf("pr-groups %q #%s note exceeds %d bytes", p.Block, p.Row, maxHumanTextBytes)
		}
		if _, decided := st.Interactions.Triage[p.Block][p.Row]; decided {
			if !replace {
				res.Skipped = append(res.Skipped, p.Block+"/"+p.Row)
				continue
			}
			res.Replaced = append(res.Replaced, p.Block+"/"+p.Row)
		}
		if byBlock[p.Block] == nil {
			byBlock[p.Block] = map[string]triageEntry{}
		}
		byBlock[p.Block][p.Row] = triageEntry{
			Verdict: p.Verdict, Note: p.Note, Head: head, At: p.At.UTC().Format(time.RFC3339),
			CarriedFrom: p.CarriedFrom, CarryNote: p.CarryNote,
		}
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

// VerdictCarry moves a pr-groups row's recorded verdict to Head, a commit the
// agent judged the same change as the one the human tapped, with Note saying why.
type VerdictCarry struct {
	Block string `json:"block"`
	Row   string `json:"row"`
	Head  string `json:"head"`
	Note  string `json:"note"`
}

// CarryResult is what the carry-verdict op recorded: the verdict, the head the
// human tapped, when, and the head it now stands at. Already reports a verdict
// that stood at Head before the carry, which leaves the log untouched.
type CarryResult struct {
	Verdict string `json:"verdict"`
	From    string `json:"from"`
	At      string `json:"at"`
	To      string `json:"to"`
	Already bool   `json:"already,omitempty"`
}

func handleCarryVerdict(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	var c VerdictCarry
	if err := json.Unmarshal(b.Verdicts, &c); err != nil {
		return errReply("decode carry: " + err.Error())
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
	payload, res, err := carryVerdictPayload(&st, c)
	if err != nil {
		return errReply(err.Error())
	}
	if payload != nil {
		if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
			SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventTriageDecided, Payload: payload,
		}); err != nil {
			return errReply(err.Error())
		}
	}
	raw, _ := json.Marshal(res)
	return okReply(result{State: raw})
}

func carryVerdictPayload(st *state.State, c VerdictCarry) (json.RawMessage, CarryResult, error) {
	g, topID, err := requirePRGroups(st, c.Block)
	if err != nil {
		return nil, CarryResult{}, err
	}
	if err := requireCurrentRound(st, c.Block, topID); err != nil {
		return nil, CarryResult{}, err
	}
	head, err := doc.ParseHead(c.Head)
	if err != nil {
		return nil, CarryResult{}, fmt.Errorf("pr-groups %q #%s: %w", c.Block, c.Row, err)
	}
	if row, ok := g.Row(c.Row); ok && doc.SameHead(head, row.Head) {
		head = row.Head
	}
	if strings.TrimSpace(c.Note) == "" {
		return nil, CarryResult{}, fmt.Errorf("pr-groups %q #%s: a carry needs a note saying why the tap still stands", c.Block, c.Row)
	}
	if len(c.Note) > maxHumanTextBytes {
		return nil, CarryResult{}, fmt.Errorf("pr-groups %q #%s note exceeds %d bytes", c.Block, c.Row, maxHumanTextBytes)
	}
	d, ok := recordedVerdict(st, VerdictRef{Block: c.Block, Row: c.Row})
	if !ok {
		return nil, CarryResult{}, fmt.Errorf("pr-groups %q #%s has no recorded verdict to carry", c.Block, c.Row)
	}
	tapped := cmp.Or(d.CarriedFrom, d.Head)
	res := CarryResult{Verdict: d.Verdict, From: tapped, At: d.At, To: head}
	if live, ok := st.Interactions.Triage[c.Block][c.Row]; ok && doc.SameHead(live.Head, head) {
		res.Already = true
		return nil, res, nil
	}
	entry := triageEntry{Verdict: d.Verdict, Note: d.Note, Head: head, At: d.At}
	if !doc.SameHead(tapped, head) {
		entry.CarriedFrom, entry.CarryNote = tapped, c.Note
	}
	return mustJSON(map[string]any{"blockId": c.Block, "verdicts": map[string]triageEntry{c.Row: entry}}), res, nil
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
		return triageEntry{Verdict: d.Verdict, Note: d.Note, Head: head, At: at, CarriedFrom: d.CarriedFrom, CarryNote: d.CarryNote}, nil
	default:
		return triageEntry{}, fmt.Errorf("block %q is a %s, not a triage or pr-groups", to.Block, b.BlockType())
	}
}
