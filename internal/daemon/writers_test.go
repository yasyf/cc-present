package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	"golang.org/x/sync/errgroup"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

func readRev(t *testing.T, h *harness) int64 {
	t.Helper()
	reply := handleOutcomes(h.hc(body{}), h.cursors)
	if !reply.OK {
		t.Fatalf("outcomes: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatal(err)
	}
	return res.Rev
}

func startAs(t *testing.T, h *harness, lane, docJSON string) string {
	t.Helper()
	start := handleStart(h.hc(body{Writer: Writer{Lane: lane}, Doc: json.RawMessage(docJSON)}), doc.NoPacks, nilDisplay, noChannel)
	if !start.OK {
		t.Fatalf("start: %s", start.Error)
	}
	return start.SubjectID
}

func upsertAs(h *harness, w Writer, block string) ccd.Reply {
	return handleUpsertBlock(h.hc(body{Writer: w, Block: json.RawMessage(block), Round: "current"}), doc.NoPacks)
}

func TestTwoWritesRacingFromOneReadLandOnceAndTheLoserNamesTheWinner(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "", cardDoc)
	rev := readRev(t, h)
	lanes := []string{"board-refresher-6", "card-mirror-2"}
	replies := make([]ccd.Reply, len(lanes))
	var g errgroup.Group
	for i, lane := range lanes {
		g.Go(func() error {
			replies[i] = upsertAs(h, Writer{Lane: lane, Rev: rev}, fmt.Sprintf(`{"id":"m1","type":"markdown","md":%q}`, lane))
			return nil
		})
	}
	_ = g.Wait()
	var won, lost []int
	for i, r := range replies {
		if r.OK {
			won = append(won, i)
		} else {
			lost = append(lost, i)
		}
	}
	if len(won) != 1 || len(lost) != 1 {
		t.Fatalf("replies = %+v, want one success and one refusal", replies)
	}
	refusal := replies[lost[0]].Error
	if want := "lane " + lanes[won[0]] + " wrote it at rev"; !strings.Contains(refusal, want) || !strings.Contains(refusal, fmt.Sprintf("after rev %d that you read", rev)) {
		t.Fatalf("refusal = %q, want it to name the winner (%q) and the rev the loser read", refusal, want)
	}
	if got := reduceSubject(t, h, subjectOf(t, h)).Doc.Blocks[1].(*doc.Markdown).Md; got != lanes[won[0]] {
		t.Fatalf("m1 = %q, want the winner's write %q", got, lanes[won[0]])
	}
	if retry := upsertAs(h, Writer{Lane: lanes[lost[0]], Rev: readRev(t, h)}, `{"id":"m1","type":"markdown","md":"merged"}`); !retry.OK {
		t.Fatalf("retry after a fresh read = %s, want ok", retry.Error)
	}
}

func subjectOf(t *testing.T, h *harness) string {
	t.Helper()
	sub, err := resolve(h.hc(body{}))
	if err != nil {
		t.Fatal(err)
	}
	return sub.ID
}

func TestACardTakesWritesOnlyFromTheLaneThatOwnsItUntilASuccessorTakesItOver(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "card-mirror-2", cardDoc)
	child := `{"id":"m2","type":"markdown","md":"edited"}`
	const owned = `lane card-mirror-2 owns it`

	if r := upsertAs(h, Writer{Lane: "board-refresher-6"}, child); r.OK || !strings.Contains(r.Error, owned) {
		t.Fatalf("another lane's child write = %+v, want a refusal naming %q", r, owned)
	}
	if r := upsertAs(h, Writer{}, child); r.OK || !strings.Contains(r.Error, owned) {
		t.Fatalf("an unnamed write = %+v, want a refusal naming %q", r, owned)
	}
	if r := handleRemoveBlock(h.hc(body{Writer: Writer{Lane: "board-refresher-6"}, ID: "c1"})); r.OK || !strings.Contains(r.Error, owned) {
		t.Fatalf("another lane's removal = %+v, want a refusal naming %q", r, owned)
	}
	dropsCard := `{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"note"}]}`
	if r := handlePush(h.hc(body{Writer: Writer{Lane: "board-refresher-6"}, Doc: json.RawMessage(dropsCard)}), doc.NoPacks, nilDisplay); r.OK || !strings.Contains(r.Error, owned) {
		t.Fatalf("another lane's push dropping the card = %+v, want a refusal naming %q", r, owned)
	}
	keepsCard := strings.Replace(cardDoc, `"md":"note"`, `"md":"refreshed"`, 1)
	if r := handlePush(h.hc(body{Writer: Writer{Lane: "board-refresher-6"}, Doc: json.RawMessage(keepsCard)}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("another lane's push carrying the card unchanged = %s, want ok", r.Error)
	}

	if r := upsertAs(h, Writer{Lane: "card-mirror-3", TakeOver: true}, child); !r.OK {
		t.Fatalf("the successor's take-over = %s, want ok", r.Error)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-2"}, `{"id":"m2","type":"markdown","md":"late"}`); r.OK || !strings.Contains(r.Error, "lane card-mirror-3 owns it") {
		t.Fatalf("the predecessor's write after the take-over = %+v, want a refusal naming card-mirror-3", r)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-3"}, `{"id":"m2","type":"markdown","md":"next"}`); !r.OK {
		t.Fatalf("the new owner's write = %s, want ok", r.Error)
	}
	reply := handleBlockShow(h.hc(body{ID: "m2"}))
	if !reply.OK {
		t.Fatalf("block show: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatal(err)
	}
	var show BlockShow
	if err := json.Unmarshal(res.State, &show); err != nil {
		t.Fatal(err)
	}
	if show.Write == nil || show.Write.Owner != "card-mirror-3" || show.Write.TakenFrom != "card-mirror-2" || show.Write.Lane != "card-mirror-3" {
		t.Fatalf("block show write = %+v, want c1 owned by card-mirror-3, taken over from card-mirror-2", show.Write)
	}
	if show.Rev < show.Write.Rev {
		t.Fatalf("block show rev %d is before its last write %d", show.Rev, show.Write.Rev)
	}
}

func TestATakeOverNamesTheLaneTakingTheCard(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "card-mirror-2", cardDoc)
	if r := upsertAs(h, Writer{TakeOver: true}, `{"id":"m2","type":"markdown","md":"x"}`); r.OK || !strings.Contains(r.Error, "--take-over needs --lane") {
		t.Fatalf("an unnamed take-over = %+v, want a refusal asking for --lane", r)
	}
}

func TestNoRedrawTouchesAVerdictANoteOrAReply(t *testing.T) {
	h := newHarness(t)
	const board = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Ask","children":[{"id":"a1","type":"approval"}]},{"id":"m1","type":"markdown","md":"note"}]}`
	subID := startAs(t, h, "communicator", board)
	stale := readRev(t, h)
	engageDecision(t, h, subID, "a1")
	fileNote(t, h, subID, "a1", "f1")
	if r := handleReply(h.hc(body{BlockID: "a1", Md: "on it"})); !r.OK {
		t.Fatalf("reply: %s", r.Error)
	}
	ageEvents(t, h, subID)
	answers := func() string {
		in := reduceSubject(t, h, subID).Interactions
		return string(mustJSON([]any{in.Decisions, in.Feedback, in.Replies}))
	}
	want := answers()

	redrawn := strings.Replace(board, `"title":"Ask"`, `"title":"Ask again"`, 1)
	reopen := []string{"c1"}
	writes := []struct {
		name  string
		write func() ccd.Reply
		ok    bool
	}{
		{"another lane's write after the read", func() ccd.Reply {
			return upsertAs(h, Writer{Lane: "board-refresher-6"}, `{"id":"m1","type":"markdown","md":"moved on"}`)
		}, true},
		{"a stale push", func() ccd.Reply {
			return handlePush(h.hc(body{Writer: Writer{Lane: "communicator", Rev: stale}, Doc: json.RawMessage(redrawn), Reopen: reopen, Round: "current"}), doc.NoPacks, nilDisplay)
		}, false},
		{"a push redrawing the card", func() ccd.Reply {
			return handlePush(h.hc(body{Writer: Writer{Lane: "communicator", Rev: readRev(t, h)}, Doc: json.RawMessage(redrawn), Reopen: reopen, Round: "current"}), doc.NoPacks, nilDisplay)
		}, true},
		{"an update-block of the card", func() ccd.Reply {
			return handleUpsertBlock(h.hc(body{Writer: Writer{Lane: "communicator"}, Block: json.RawMessage(`{"id":"c1","type":"card","title":"Ask a third time","children":[{"id":"a1","type":"approval"}]}`), Reopen: reopen}), doc.NoPacks)
		}, true},
		{"a push opening a new round", func() ccd.Reply {
			return handlePush(h.hc(body{Writer: Writer{Lane: "communicator"}, Doc: json.RawMessage(board), Reopen: reopen, Round: "new"}), doc.NoPacks, nilDisplay)
		}, true},
		{"a remove-block of the answered approval", func() ccd.Reply {
			return handleRemoveBlock(h.hc(body{Writer: Writer{Lane: "communicator"}, ID: "a1"}))
		}, true},
		{"a push dropping the card", func() ccd.Reply {
			return handlePush(h.hc(body{Writer: Writer{Lane: "communicator"}, Doc: json.RawMessage(`{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"gone"}]}`), Round: "current"}), doc.NoPacks, nilDisplay)
		}, true},
	}
	for _, w := range writes {
		if r := w.write(); r.OK != w.ok || !r.OK && !strings.Contains(r.Error, "that you read") {
			t.Fatalf("%s = %+v, want ok %v", w.name, r, w.ok)
		}
		if got := answers(); got != want {
			t.Fatalf("after %s, verdicts, notes and replies = %s, want them byte-identical to %s", w.name, got, want)
		}
	}
}

func TestAnyLaneMovesAPRBlockToANewHeadAndNothingElse(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "card-mirror-2", `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","children":[{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"abc1234"},{"id":"q1","type":"choice","options":[{"id":"go","label":"Go"}]}]}]}`)
	rev := readRev(t, h)

	if r := upsertAs(h, Writer{Lane: "board-refresher-7", Rev: rev}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"def5678","summary":"also new words"}`); r.OK || !strings.Contains(r.Error, "lane card-mirror-2 owns it") {
		t.Fatalf("another lane's head move that also changes text = %+v, want the owner refusal", r)
	}
	if r := upsertAs(h, Writer{Lane: "board-refresher-7", Rev: rev}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"def5678"}`); !r.OK {
		t.Fatalf("another lane's head-only move = %s, want ok", r.Error)
	}
	if r := upsertAs(h, Writer{Lane: "board-refresher-7", Rev: rev}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"0123abc"}`); r.OK || !strings.Contains(r.Error, "after rev") {
		t.Fatalf("a head-only move from a stale read = %+v, want the rev refusal", r)
	}
	st := reduceSubject(t, h, subjectOf(t, h))
	if got := st.Doc.Blocks[0].(*doc.Card).Children[0].(*doc.PR).Head; got != "def5678" {
		t.Fatalf("p1 head = %q, want def5678", got)
	}
	if w, _ := st.Written("c1"); w.Owner != "card-mirror-2" || w.Lane != "board-refresher-7" {
		t.Fatalf("c1 write = %+v, want card-mirror-2 still owning it and board-refresher-7 named as the writer", w)
	}
}

func TestAWriteThatChangesNothingAnOwnerDrewLeavesAnUnownedCardUnowned(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "", `{"version":1,"title":"T","blocks":[{"id":"c1","type":"card","title":"C","children":[{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"abc1234"}]}]}`)
	if r := upsertAs(h, Writer{Lane: "board-refresher-6"}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"def5678"}`); !r.OK {
		t.Fatalf("head-only write: %s", r.Error)
	}
	hc := h.hc(body{})
	board, err := caughtUp(hc.Ctx, hc.DB, subjectOf(t, h))
	if err != nil {
		t.Fatal(err)
	}
	last, _ := board.st.Written("c1")
	board.mu.Unlock()
	if last.Lane != "board-refresher-6" || last.Owner != "" {
		t.Fatalf("after a head-only write the last write is %+v, want the writer named and no owner", last)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-2"}, `{"id":"c1","type":"card","title":"Redrawn","children":[{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"def5678"}]}`); !r.OK {
		t.Fatalf("a redraw of the still unowned card: %s", r.Error)
	}
}

func TestASlackCardRefusedByTheBoardIsRefusedBeforeItIsPosted(t *testing.T) {
	h := newHarness(t)
	unposted := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	startAs(t, h, "", `{"version":1,"title":"Board","blocks":[`+unposted+`]}`)
	rev := readRev(t, h)
	if r := upsertAs(h, Writer{Lane: "card-mirror-2", Rev: rev}, unposted); !r.OK {
		t.Fatalf("the owner's write: %s", r.Error)
	}
	check := func(w Writer, card string) ccd.Reply {
		return handleSlackCardCheck(h.hc(body{Writer: w, Block: json.RawMessage(card)}), doc.NoPacks)
	}
	redrawn := strings.Replace(unposted, `"type":"card",`, `"type":"card","summary":"Redrawn.",`, 1)
	if r := check(Writer{Lane: "board-refresher-6"}, redrawn); r.OK || !strings.Contains(r.Error, "lane card-mirror-2 owns it") {
		t.Fatalf("another lane's redraw: %+v", r)
	}
	if r := check(Writer{Lane: "board-refresher-6", Rev: rev}, unposted); r.OK || !strings.Contains(r.Error, "lane card-mirror-2 wrote it") {
		t.Fatalf("a stale send: %+v", r)
	}
	if r := check(Writer{Lane: "board-refresher-6", Rev: readRev(t, h)}, unposted); !r.OK {
		t.Fatalf("a fresh send of the card as it stands: %s", r.Error)
	}
}

func TestAWriteFromTheRevASlackPostPrintedIsNotRefusedByThatPost(t *testing.T) {
	h := newHarness(t)
	unposted := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	startAs(t, h, "card-mirror-2", `{"version":1,"title":"Board","blocks":[`+unposted+`]}`)
	reply := handleSlackCard(h.hc(body{Writer: Writer{Lane: "card-mirror-2", Rev: readRev(t, h)}, Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
	if !reply.OK {
		t.Fatalf("the post: %s", reply.Error)
	}
	var posted result
	if err := json.Unmarshal(reply.Body, &posted); err != nil {
		t.Fatal(err)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-2", Rev: posted.Rev}, syncedCardJSON); !r.OK {
		t.Fatalf("a write from the rev the post replied %d: %s", posted.Rev, r.Error)
	}
}

func TestASendLeavesTheCardUnownedAndAnotherLanesRedrawOfItsCardIsRefused(t *testing.T) {
	h := newHarness(t)
	unposted := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	startAs(t, h, "", `{"version":1,"title":"Board","blocks":[`+unposted+`]}`)
	post := func(w Writer, posted bool, card string) ccd.Reply {
		return handleSlackCard(h.hc(body{Writer: w, Posted: posted, Block: json.RawMessage(card)}), doc.NoPacks, nilDisplay)
	}
	if r := post(Writer{Lane: "card-mirror-2"}, true, syncedCardJSON); !r.OK {
		t.Fatalf("the send: %s", r.Error)
	}
	st := reduceSubject(t, h, subjectOf(t, h))
	if last, _ := st.Written("step-3:merge"); last.Lane != "card-mirror-2" || last.Owner != "" {
		t.Fatalf("after a send the last write is %+v, want the sender named and no owner", last)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-3"}, syncedCardJSON); !r.OK {
		t.Fatalf("the write that claims the card: %s", r.Error)
	}
	redrawn := strings.Replace(syncedCardJSON, `"type":"card",`, `"type":"card","summary":"Redrawn.",`, 1)
	if r := post(Writer{Lane: "card-mirror-2"}, false, redrawn); r.OK || !strings.Contains(r.Error, "lane card-mirror-3 owns it") {
		t.Fatalf("another lane's Slack write of a changed card: %+v", r)
	}
}

func TestAPostBindsToTheCardAsItStandsAfterAWriteLandedDuringThePost(t *testing.T) {
	h := newHarness(t)
	unposted := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	startAs(t, h, "card-mirror-2", `{"version":1,"title":"Board","blocks":[`+unposted+`]}`)
	read := readRev(t, h)
	redrawn := strings.Replace(unposted, `"type":"card",`, `"type":"card","summary":"Redrawn during the post.",`, 1)
	if r := upsertAs(h, Writer{Lane: "card-mirror-2", Rev: read}, redrawn); !r.OK {
		t.Fatalf("the write during the post: %s", r.Error)
	}
	r := handleSlackCard(h.hc(body{Writer: Writer{Lane: "board-refresher-6", Rev: read}, Posted: true, Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
	if !r.OK {
		t.Fatalf("recording the post: %s", r.Error)
	}
	card := reduceSubject(t, h, subjectOf(t, h)).Doc.Blocks[0].(*doc.Card)
	if card.Slack == nil || card.Slack.URL != slackPermalink || card.Summary != "Redrawn during the post." {
		t.Fatalf("card = %+v, want the redraw kept and the post bound to it", card)
	}
	if r := handleRemoveBlock(h.hc(body{Writer: Writer{Lane: "card-mirror-2"}, ID: card.ID})); !r.OK {
		t.Fatalf("remove: %s", r.Error)
	}
	gone := handleSlackCard(h.hc(body{Writer: Writer{Lane: "board-refresher-6"}, Posted: true, Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
	if gone.OK || !strings.Contains(gone.Error, "left the board") {
		t.Fatalf("recording a post for a removed card: %+v", gone)
	}
}

func TestAReplaceRefusedByAnOwnerLeavesTheBoardOpenAndATakeOverReplacesIt(t *testing.T) {
	h := newHarness(t)
	sub := startAs(t, h, "card-mirror-2", cardDoc)
	replace := func(w Writer) ccd.Reply {
		return handleStart(h.hc(body{Writer: w, Replace: true, Doc: json.RawMessage(`{"version":1,"title":"New","blocks":[]}`)}), doc.NoPacks, nilDisplay, noChannel)
	}
	if r := replace(Writer{Lane: "board-refresher-6", Rev: 100000}); r.OK {
		t.Fatalf("a replace from a rev past the board = ok, want a refusal")
	}
	if r := replace(Writer{Lane: "board-refresher-6"}); r.OK || !strings.Contains(r.Error, "lane card-mirror-2 owns it") {
		t.Fatalf("another lane's replace: %+v", r)
	}
	if got := subjectOf(t, h); got != sub {
		t.Fatalf("a refused replace moved the window to subject %s, want %s still open", got, sub)
	}
	if r := replace(Writer{Lane: "card-mirror-3", TakeOver: true}); !r.OK {
		t.Fatalf("the successor's replace: %s", r.Error)
	}
	if got := subjectOf(t, h); got == sub {
		t.Fatalf("the successor's replace left the old subject open")
	}
}

func TestAStalePushCannotRevertATitleOrOrderAnotherLaneWrote(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "", `{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"a"},{"id":"m2","type":"markdown","md":"b"}]}`)
	read := readRev(t, h)
	push := func(w Writer, d string) ccd.Reply {
		return handlePush(h.hc(body{Writer: w, Doc: json.RawMessage(d), Round: "current"}), doc.NoPacks, nilDisplay)
	}
	if r := push(Writer{Lane: "card-mirror-2", Rev: read}, `{"version":1,"title":"Renamed","blocks":[{"id":"m2","type":"markdown","md":"b"},{"id":"m1","type":"markdown","md":"a"}]}`); !r.OK {
		t.Fatalf("the rename: %s", r.Error)
	}
	if r := push(Writer{Lane: "board-refresher-6", Rev: read}, `{"version":1,"title":"Board","blocks":[{"id":"m1","type":"markdown","md":"a"},{"id":"m2","type":"markdown","md":"b"}]}`); r.OK || !strings.Contains(r.Error, "lane card-mirror-2 wrote them") {
		t.Fatalf("a stale push of the old title and order: %+v", r)
	}
	if r := push(Writer{Lane: "board-refresher-6", Rev: read}, `{"version":1,"title":"Renamed","blocks":[{"id":"m2","type":"markdown","md":"b"},{"id":"m1","type":"markdown","md":"a"}]}`); !r.OK {
		t.Fatalf("a stale push that keeps the title and order: %s", r.Error)
	}
}

func TestAClosedBoardRefusesAWriteThatReachesTheLockAfterTheClose(t *testing.T) {
	st := state.New()
	st.Interactions.Closed.Value = true
	if err := (Writer{Lane: "card-mirror-2"}).admit(&st, 1, "m1"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("admit on a closed board = %v", err)
	}
}

func TestATakeOverIsKeptOnAHeadOnlyWriteAndEmptyFilesCountAsNoChange(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "card-mirror-2", `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","children":[{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"abc1234","files":[]}]}]}`)
	if r := upsertAs(h, Writer{Lane: "board-refresher-7"}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"def5678"}`); !r.OK {
		t.Fatalf("a head move that omits empty files: %s", r.Error)
	}
	if r := upsertAs(h, Writer{Lane: "card-mirror-3", TakeOver: true}, `{"id":"p1","type":"pr","repo":"o/r","number":7,"head":"0123456"}`); !r.OK {
		t.Fatalf("a head move that takes the card over: %s", r.Error)
	}
	st := reduceSubject(t, h, subjectOf(t, h))
	if last, _ := st.Written("c1"); last.Owner != "card-mirror-3" || last.TakenFrom != "card-mirror-2" {
		t.Fatalf("last write = %+v, want card-mirror-3 owning it, taken from card-mirror-2", last)
	}
}

func TestAProofBoardKeepsItsMarkThroughStartAndAPush(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "card-mirror-2", `{"version":1,"title":"Board","proof":true,"blocks":[]}`)
	if !reduceSubject(t, h, subjectOf(t, h)).Doc.Proof {
		t.Fatal("start dropped the proof mark")
	}
	r := handlePush(h.hc(body{Doc: json.RawMessage(`{"version":1,"title":"Board","proof":true,"blocks":[{"id":"m1","type":"markdown","md":"x"}]}`), Round: "current"}), doc.NoPacks, nilDisplay)
	if !r.OK {
		t.Fatalf("push: %s", r.Error)
	}
	if !reduceSubject(t, h, subjectOf(t, h)).Doc.Proof {
		t.Fatal("push dropped the proof mark")
	}
}

func TestOutcomesNameTheLaneThatOwnsEachCardOnTheBoard(t *testing.T) {
	h := newHarness(t)
	startAs(t, h, "", cardDoc)
	if r := upsertAs(h, Writer{Lane: "dashboard-proof-1"}, `{"id":"c2","type":"card","children":[{"id":"m3","type":"markdown","md":"proof"}]}`); !r.OK {
		t.Fatalf("upsert c2: %s", r.Error)
	}
	if got := outcome(t, h).Owners; len(got) != 1 || got["c2"] != "dashboard-proof-1" {
		t.Fatalf("owners = %v, want only c2 owned by dashboard-proof-1", got)
	}
	if r := handleRemoveBlock(h.hc(body{Writer: Writer{Lane: "dashboard-proof-1"}, ID: "c2"})); !r.OK {
		t.Fatalf("remove c2: %s", r.Error)
	}
	if got := outcome(t, h).Owners; len(got) != 0 {
		t.Fatalf("owners after c2 left the board = %v, want none", got)
	}
}
