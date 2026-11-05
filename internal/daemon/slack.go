package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

const slackBoardTitle = "Questions for you"

type groupPoster func(ctx context.Context, session, thread string, card slackcards.StatusCard) (string, error)

type slackAnswerer func(ctx context.Context, waits, card, label string, other bool, via string) (slackcards.SlackAnswer, error)

func ccSlackCardAnswer(ctx context.Context, waits, card, label string, other bool, via string) (slackcards.SlackAnswer, error) {
	args := []string{"card-answer", "--waits", waits, "--card", card, "--label", label, "--via", via}
	if other {
		args = append(args, "--other")
	}
	out, err := slackcards.CCSlack(ctx, args...)
	if err != nil {
		return slackcards.SlackAnswer{}, err
	}
	var res slackcards.SlackAnswer
	if err := json.Unmarshal(out, &res); err != nil {
		return slackcards.SlackAnswer{}, fmt.Errorf("decode cc-slack card-answer reply %q: %w", out, err)
	}
	return res, nil
}

func handleSlackCard(hc ccd.HandlerCtx, pt doc.PackTypes, display displayFunc) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	blk, err := doc.DecodeBlock(b.Block)
	if err != nil {
		return errReply(err.Error())
	}
	card, ok := blk.(*doc.Card)
	if !ok || card.Slack == nil {
		return errReply(fmt.Sprintf("block %q is a %s; slack-card takes a card carrying its Slack permalink", blk.BlockID(), blk.BlockType()))
	}
	if _, err := slackcards.Check(card); err != nil {
		return errReply(err.Error())
	}
	if err := validateBlock(blk, pt); err != nil {
		return errReply(err.Error())
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	sub, found, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope)
	if err != nil {
		return errReply(err.Error())
	}
	if !found || sub.Status == statusClosed {
		if sub, _, err = hc.Subjects.Start(hc.Ctx, hc.Window, hc.Scope, slugify(slackBoardTitle)+"--"+randomHex(4), lifecycle, found); err != nil {
			return errReply(err.Error())
		}
	}
	board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	defer board.mu.Unlock()
	st := &board.st
	cardJSON := b.Block
	if b.Posted {
		loc, ok := doc.Locate(st.Doc, card.ID)
		current, isCard := loc.Block.(*doc.Card)
		if !ok || !isCard {
			return errReply(fmt.Sprintf("card %q left the board while its Slack card posted", card.ID))
		}
		bound := *current
		bound.Slack = card.Slack
		card, cardJSON = &bound, mustJSON(&bound)
	}
	if err := admitCard(hc, sub.ID, st, board.retired, card); err != nil {
		return errReply(err.Error())
	}
	if err := fitsBoard(st.Doc, card); err != nil {
		return errReply(err.Error())
	}
	w := b.Writer
	w.keepOwner = !w.TakeOver && postsCurrent(st.Doc, card)
	if b.Posted {
		w.Rev = 0
	}
	if err := w.admit(st, board.seq, card.ID); err != nil {
		return errReply(err.Error())
	}
	var seq int64
	if st.Doc.Title == "" && len(st.Doc.Blocks) == 0 {
		raw, err := json.Marshal(&doc.Doc{Version: 1, Title: slackBoardTitle, Blocks: []doc.Block{card}})
		if err != nil {
			return errReply(err.Error())
		}
		rev, err := nextRevision(hc.Ctx, hc.DB, sub.ID)
		if err != nil {
			return errReply(err.Error())
		}
		if seq, err = appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
			SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventDocReplaced,
			Payload: docReplacedPayload(raw, rev, nil, w),
		}); err != nil {
			return errReply(err.Error())
		}
	} else if seq, err = appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginAgent, Type: EventBlockUpserted,
		Payload: blockUpsertedPayload(cardJSON, "", w),
	}); err != nil {
		return errReply(err.Error())
	}
	res := result{Rev: seq, URL: artifactURL(hc.HTTPPort, sub.Slug), TailnetURLs: display(hc.Ctx, sub.Slug, hc.HTTPPort), Slug: sub.Slug}
	raw, _ := json.Marshal(res)
	return ccd.Reply{OK: true, SubjectID: sub.ID, HTTPPort: hc.HTTPPort, Body: raw}
}

// postsCurrent reports whether card is the board's card of the same id as it
// stands, with only its Slack binding changed: the write that records a post,
// which takes no ownership of the card.
func postsCurrent(d *doc.Doc, card *doc.Card) bool {
	loc, ok := doc.Locate(d, card.ID)
	if !ok {
		return false
	}
	cur, ok := loc.Block.(*doc.Card)
	if !ok {
		return false
	}
	was, now := *cur, *card
	was.Slack, now.Slack = nil, nil
	return string(mustJSON(&was)) == string(mustJSON(&now))
}

func handleSlackCardCheck(hc ccd.HandlerCtx, pt doc.PackTypes) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	blk, err := doc.DecodeBlock(b.Block)
	if err != nil {
		return errReply(err.Error())
	}
	card, ok := blk.(*doc.Card)
	if !ok {
		return errReply(fmt.Sprintf("block %q is a %s; a Slack card is a card", blk.BlockID(), blk.BlockType()))
	}
	if _, err := slackcards.Check(card); err != nil {
		return errReply(err.Error())
	}
	if err := validateBlock(blk, pt); err != nil {
		return errReply(err.Error())
	}
	sub, found, err := hc.Subjects.Find(hc.Ctx, hc.Window, hc.Scope)
	if err != nil {
		return errReply(err.Error())
	}
	empty := state.New()
	st, retired, head := &empty, map[string]bool{}, int64(0)
	if found && sub.Status != statusClosed {
		board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
		if err != nil {
			return errReply(err.Error())
		}
		defer board.mu.Unlock()
		st, retired, head = &board.st, board.retired, board.seq
	}
	if err := admitCard(hc, sub.ID, st, retired, card); err != nil {
		return errReply(err.Error())
	}
	if err := fitsBoard(st.Doc, card); err != nil {
		return errReply(err.Error())
	}
	w := b.Writer
	w.keepOwner = !w.TakeOver && postsCurrent(st.Doc, card)
	if err := w.admit(st, head, card.ID); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

func admitCard(hc ccd.HandlerCtx, subID string, st *state.State, retired map[string]bool, card *doc.Card) error {
	gate, err := newAdmitGate(hc.Ctx, hc.DB, subID, st, retired, nil, Reask{})
	if err != nil {
		return err
	}
	if _, warnings := gate.strip([]doc.Block{card}); len(warnings) > 0 {
		return errors.New(strings.Join(warnings, "; "))
	}
	return gate.check(card)
}

func fitsBoard(d *doc.Doc, card *doc.Card) error {
	if loc, ok := doc.Locate(d, card.ID); ok && loc.Kind != doc.TopLevel {
		return fmt.Errorf("block %q already sits inside card %q; a Slack card's id is its own", card.ID, loc.TopID)
	}
	return upsertedDoc(d, card, "").ValidateLayout()
}

func handleSlackAnswer(hc ccd.HandlerCtx) ccd.Reply {
	b := decodeBody(hc.Env.Body)
	var a slackcards.Answered
	if err := json.Unmarshal(b.Answer, &a); err != nil {
		return errReply("decode answer: " + err.Error())
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	defer board.mu.Unlock()
	st := &board.st
	card, q, err := syncedCard(st.Doc, a.Waits)
	if err != nil {
		return errReply(err.Error())
	}
	if card.Status == slackcards.Resolved {
		return okReply(result{})
	}
	if err := recordAnswer(hc.Ctx, hc.Append, sub.ID, card, q, a.Label, a.Other); err != nil {
		return errReply(err.Error())
	}
	note := fmt.Sprintf("Answered on [Slack](%s) at %s: **%s**", a.Permalink, a.At.Local().Format(time.Kitchen), a.Label)
	if err := settleCard(hc.Ctx, hc.Append, sub.ID, card, note); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

func handleSlackDecided(hc ccd.HandlerCtx) ccd.Reply {
	var d slackcards.Decided
	if err := json.Unmarshal(decodeBody(hc.Env.Body).Answer, &d); err != nil {
		return errReply("decode row tap: " + err.Error())
	}
	sub, err := resolveOpen(hc)
	if err != nil {
		return errReply(err.Error())
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	board, err := caughtUp(hc.Ctx, hc.DB, sub.ID)
	if err != nil {
		return errReply(err.Error())
	}
	defer board.mu.Unlock()
	st := &board.st
	if st.Interactions.Closed.Value {
		return errReply("the board is closed")
	}
	blockID, rowID, head, verdict, err := tappedRow(st.Doc, d)
	if err != nil {
		return errReply(err.Error())
	}
	note := st.Interactions.Triage[blockID][rowID].Note
	it := interaction{Type: EventTriageDecided, BlockID: blockID, Verdicts: map[string]triageEntry{rowID: {Verdict: verdict, Head: head, Note: note}}}
	payload, err := validateInteraction(st, 0, &it, nil, time.Now())
	if err != nil {
		return errReply(err.Error())
	}
	if _, err := appendEvent(hc.Ctx, hc.Append, &ccevent.Event{
		SubjectID: sub.ID, Origin: ccevent.OriginHuman, Type: EventTriageDecided, Payload: payload,
		DedupKey: EventTriageDecided + ":" + blockID + ":slack:" + d.Row + ":" + d.At.Format(time.RFC3339Nano),
	}); err != nil {
		return errReply(err.Error())
	}
	return okReply(result{})
}

func tappedRow(d *doc.Doc, tap slackcards.Decided) (block, row, head, verdict string, err error) {
	block, row, verdict, err = tap.Item()
	if b, _, ferr := findBlock(d, block); ferr == nil && b.BlockType() == "triage" {
		return block, row, "", verdict, err
	}
	if block, row, head, verdict, err = tap.Verdict(); err != nil {
		return "", "", "", "", err
	}
	if b, _, err := findBlock(d, block); err != nil || b.BlockType() != "pr-groups" {
		return "", "", "", "", fmt.Errorf("block %q is neither a triage nor a pr-groups block on this board", block)
	}
	return block, row, head, verdict, nil
}

func syncedCard(d *doc.Doc, id string) (*doc.Card, doc.Block, error) {
	loc, ok := doc.Locate(d, id)
	if !ok || loc.Kind != doc.TopLevel {
		return nil, nil, fmt.Errorf("no card %q on this board", id)
	}
	card, ok := loc.Block.(*doc.Card)
	if !ok || card.Slack == nil {
		return nil, nil, fmt.Errorf("block %q is not a card synced with Slack", id)
	}
	q, ok := slackcards.Question(card)
	if !ok {
		return nil, nil, fmt.Errorf("card %q asks no single question", id)
	}
	if _, rows := q.(*doc.Triage); rows {
		return nil, nil, fmt.Errorf("card %q asks per row; its Slack cards take row taps", id)
	}
	return card, q, nil
}

func recordAnswer(ctx context.Context, appendFn ccd.AppendFunc, subID string, card *doc.Card, q doc.Block, label string, other bool) error {
	p := map[string]any{"blockId": q.BlockID()}
	var eventType string
	switch q := q.(type) {
	case *doc.Choice:
		ids, written := slackcards.Selection(q, label, other)
		eventType, p["optionIds"] = EventChoiceSelected, ids
		if written != "" {
			p["other"] = written
		}
		if heads := askedHeads(card); len(heads) > 0 {
			p["heads"] = heads
		}
		if approved := approvedHeads(q, ids); len(approved) > 0 {
			p["approves"] = approved
		}
		if words := shownWords(card); words != "" {
			p["shown"], p["picked"] = words, pickedLabels(q, ids)
		}
	case *doc.Approval:
		verdict, note := slackcards.ApprovalVerdict(label, other)
		eventType, p["verdict"] = EventDecisionCreated, verdict
		if note != "" {
			p["note"] = note
		}
		if words := shownWords(card); words != "" {
			p["shown"] = words
		}
	case *doc.Input:
		eventType, p["text"] = EventInputSubmitted, label
	}
	_, err := appendEvent(ctx, appendFn, &ccevent.Event{
		SubjectID: subID, Origin: ccevent.OriginHuman, Type: eventType, Payload: mustJSON(p),
	})
	return err
}

const slackAnswerLane = "slack-answer"

func settleCard(ctx context.Context, appendFn ccd.AppendFunc, subID string, card *doc.Card, note string) error {
	resolved := *card
	resolved.Status = slackcards.Resolved
	raw, err := json.Marshal(&resolved)
	if err != nil {
		return err
	}
	if _, err := appendEvent(ctx, appendFn, &ccevent.Event{
		SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventBlockUpserted, Payload: blockUpsertedPayload(raw, "", Writer{Lane: slackAnswerLane, keepOwner: true}),
	}); err != nil {
		return err
	}
	if note == "" {
		return nil
	}
	_, err = appendEvent(ctx, appendFn, &ccevent.Event{
		SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventReplyCreated,
		Payload: mustJSON(map[string]string{"id": randomHex(4), "blockId": card.ID, "md": note}),
	})
	return err
}

func answersQuestion(it *interaction) bool {
	switch it.Type {
	case EventChoiceSelected:
		return true
	case EventDecisionCreated:
		return it.Verdict != "cleared"
	case EventInputSubmitted:
		return !visuallyEmpty(it.Text)
	}
	return false
}

func slackQuestion(st *state.State, it *interaction) (*doc.Card, doc.Block) {
	if !answersQuestion(it) {
		return nil, nil
	}
	loc, ok := doc.Locate(st.Doc, it.BlockID)
	if !ok || loc.Kind != doc.CardChild {
		return nil, nil
	}
	card, q, err := syncedCard(st.Doc, loc.TopID)
	if err != nil || q.BlockID() != it.BlockID {
		return nil, nil
	}
	if ch, ok := q.(*doc.Choice); ok && slackcards.ExplainsMore(ch, it.OptionIDs) {
		return nil, nil
	}
	return card, q
}

func boardLabel(q doc.Block, it *interaction) (string, bool) {
	switch q := q.(type) {
	case *doc.Choice:
		return slackcards.Label(q, it.OptionIDs, it.Other)
	case *doc.Approval:
		return slackcards.ApprovalLabel(it.Verdict), false
	}
	return it.Text, true
}

func (rs *restServer) answerOnSlack(ctx context.Context, board *reducedBoard, subID string, card *doc.Card, q doc.Block, it *interaction, via string) error {
	label, other := boardLabel(q, it)
	res, err := rs.answerSlack(ctx, card.ID, card.Slack.URL, label, other, via)
	if lerr := board.lock(ctx, rs.db(), subID); lerr != nil {
		return lerr
	}
	defer board.mu.Unlock()
	current, currentQ, lerr := syncedCard(board.st.Doc, card.ID)
	if lerr != nil || current.Status == slackcards.Resolved || current.Slack.URL != card.Slack.URL {
		return nil
	}
	switch {
	case err != nil:
		_, err = appendEvent(ctx, rs.append, &ccevent.Event{
			SubjectID: subID, Origin: ccevent.OriginAgent, Type: EventReplyCreated,
			Payload: mustJSON(map[string]string{"id": randomHex(4), "blockId": card.ID, "md": fmt.Sprintf("This answer did not reach the [Slack card](%s): %s", card.Slack.URL, err)}),
		})
		return err
	case res.Answered:
		return settleCard(ctx, rs.append, subID, current, "")
	default:
		if err := recordAnswer(ctx, rs.append, subID, current, currentQ, res.Label, res.Other); err != nil {
			return err
		}
		return settleCard(ctx, rs.append, subID, current, fmt.Sprintf("The [Slack card](%s) was answered first: **%s**. This pick did not count.", card.Slack.URL, res.Label))
	}
}

var redrawMu sync.Mutex

func (rs *restServer) redrawGroupCards(ctx context.Context, subID string, it *interaction) error {
	redrawMu.Lock()
	defer redrawMu.Unlock()
	board, err := caughtUp(ctx, rs.db(), subID)
	if err != nil {
		return err
	}
	thread, cards, err := groupCards(&board.st, it)
	board.mu.Unlock()
	if err != nil || len(cards) == 0 {
		return err
	}
	var session sql.NullString
	if err := rs.db().QueryRowContext(ctx, `SELECT session_id FROM subjects WHERE id=?`, subID).Scan(&session); err != nil {
		return fmt.Errorf("read subject session: %w", err)
	}
	var errs []error
	for _, card := range cards {
		if _, err := rs.postGroup(ctx, session.String, thread, card); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func groupCards(st *state.State, it *interaction) (string, []slackcards.StatusCard, error) {
	b, top, err := findBlock(st.Doc, it.BlockID)
	if err != nil {
		return "", nil, err
	}
	if tr, ok := b.(*doc.Triage); ok {
		return "", triageCards(st, top, tr), nil
	}
	g, ok := b.(*doc.PRGroups)
	if !ok || g.Thread() == "" {
		return "", nil, nil
	}
	var cards []slackcards.StatusCard
	for _, group := range g.Groups {
		if slices.ContainsFunc(group.PRs, func(r doc.PRRow) bool { _, ok := it.Verdicts[doc.RowID(r.Number)]; return ok }) {
			cards = append(cards, slackcards.GroupCard(g, group, st.Interactions.Triage[g.ID], true, 0)...)
		}
	}
	return g.Thread(), cards, nil
}

func triageCards(st *state.State, top string, tr *doc.Triage) []slackcards.StatusCard {
	loc, _ := doc.Locate(st.Doc, top)
	card, ok := loc.Block.(*doc.Card)
	if !ok || card.Slack == nil {
		return nil
	}
	if _, ok := slackcards.Question(card); !ok {
		return nil
	}
	return slackcards.Cards(card, tr, st.Interactions.Triage[tr.ID])
}

func subjectSlug(ctx context.Context, db *sql.DB, subID string) (string, error) {
	var slug string
	if err := db.QueryRowContext(ctx, `SELECT slug FROM subjects WHERE id=?`, subID).Scan(&slug); err != nil {
		return "", fmt.Errorf("read subject slug: %w", err)
	}
	return slug, nil
}

// claimThreads refuses a pr-groups block that newly binds a Slack thread an
// open board other than subID already binds, so a board seeded from another
// board's document cannot redraw that board's group cards.
func claimThreads(ctx context.Context, db *sql.DB, subID string, cur *doc.Doc, blocks []doc.Block) error {
	claims := map[string]*doc.PRGroups{}
	for _, b := range blocks {
		doc.Walk(b, func(b doc.Block) {
			g, ok := b.(*doc.PRGroups)
			if !ok || g.Thread() == "" {
				return
			}
			if loc, found := doc.Locate(cur, g.ID); found {
				if prev, ok := loc.Block.(*doc.PRGroups); ok && prev.Thread() == g.Thread() {
					return
				}
			}
			claims[g.Thread()] = g
		})
	}
	if len(claims) == 0 {
		return nil
	}
	ids, err := openSubjectIDs(ctx, db)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == subID {
			continue
		}
		board, err := caughtUp(ctx, db, id)
		if err != nil {
			return fmt.Errorf("read the slack threads open board %s binds: %w", id, err)
		}
		var claimed *doc.PRGroups
		for _, b := range board.st.Doc.Blocks {
			doc.Walk(b, func(b doc.Block) {
				if g, ok := b.(*doc.PRGroups); ok && claimed == nil {
					claimed = claims[g.Thread()]
				}
			})
		}
		board.mu.Unlock()
		if claimed == nil {
			continue
		}
		slug, err := subjectSlug(ctx, db, id)
		if err != nil {
			return err
		}
		return fmt.Errorf("pr-groups %q: slack thread %s belongs to the open board %s; a board drives only its own thread, so drop slack from this block or close that board", claimed.ID, claimed.Thread(), slug)
	}
	return nil
}
