package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yasyf/cc-present/internal/doc"
)

type withdrawCall struct {
	waits, card, reason, via string
}

type fakeWithdrawer struct {
	mu    sync.Mutex
	calls []withdrawCall
	fail  int
}

func (f *fakeWithdrawer) withdraw(_ context.Context, waits, card, reason, via string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, withdrawCall{waits, card, reason, via})
	if f.fail > 0 {
		f.fail--
		return errors.New("cc-slack is not running")
	}
	return nil
}

func (f *fakeWithdrawer) made() []withdrawCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]withdrawCall(nil), f.calls...)
}

func mirrorTo(t *testing.T, h *harness, f *fakeWithdrawer) {
	t.Helper()
	mirrors.Store(h.cc.DB(), &slackMirror{append: h.cc.AppendEvent, withdraw: f.withdraw})
	t.Cleanup(func() { mirrors.Delete(h.cc.DB()) })
}

func settled(t *testing.T, h *harness, subjectID string) {
	t.Helper()
	board := boardFor(h.cc.DB(), subjectID)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		board.mu.Lock()
		busy := board.settling
		board.mu.Unlock()
		if !busy {
			return
		}
	}
	t.Fatal("the Slack mirror never finished")
}

func withdrawnReplies(t *testing.T, h *harness, subjectID string) []string {
	t.Helper()
	var mds []string
	for _, ev := range h.eventsOfType(t, subjectID, EventReplyCreated) {
		var p struct {
			Md       string `json:"md"`
			Withdrew int64  `json:"withdrew"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p.Withdrew != 0 {
			mds = append(mds, p.Md)
		}
	}
	return mds
}

func postSynced(t *testing.T, h *harness, card string) string {
	t.Helper()
	r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(card)}), doc.NoPacks, nilDisplay)
	if !r.OK {
		t.Fatal(r.Error)
	}
	return r.SubjectID
}

func TestRemovingASlackCardWithdrawsItsSlackCardOnceAcrossARestart(t *testing.T) {
	h := newHarness(t)
	f := &fakeWithdrawer{}
	mirrorTo(t, h, f)
	sub := postSynced(t, h, syncedCardJSON)
	if r := handleUpsertBlock(slackWindow(h, body{Block: json.RawMessage(`{"id":"note","type":"markdown","md":"context"}`)}), doc.NoPacks); !r.OK {
		t.Fatal(r.Error)
	}
	if r := handleRemoveBlock(slackWindow(h, body{ID: "note", Reason: "stale"})); !r.OK {
		t.Fatal(r.Error)
	}
	if r := handleRemoveBlock(slackWindow(h, body{ID: "step-3:merge", Reason: "superseded by step 4"})); !r.OK {
		t.Fatal(r.Error)
	}
	settled(t, h, sub)
	want := withdrawCall{"step-3:merge", slackPermalink, "superseded by step 4", slackPermalink}
	if got := f.made(); len(got) != 1 || got[0] != want {
		t.Fatalf("withdrawals = %+v, want only %+v", got, want)
	}
	if got := withdrawnReplies(t, h, sub); len(got) != 1 || !strings.Contains(got[0], "superseded by step 4") {
		t.Fatalf("withdrawn replies = %q, want one naming the reason", got)
	}

	forgetBoardsExcept(h.cc.DB(), nil)
	board, err := caughtUp(context.Background(), h.cc.DB(), sub)
	if err != nil {
		t.Fatal(err)
	}
	board.mu.Unlock()
	settled(t, h, sub)
	if got := f.made(); len(got) != 1 {
		t.Fatalf("withdrawals after a restart = %+v, want the card edited once", got)
	}
}

func TestACardReplacedByANewOneNamesItAsWhatReplacedIt(t *testing.T) {
	h := newHarness(t)
	f := &fakeWithdrawer{}
	mirrorTo(t, h, f)
	sub := postSynced(t, h, syncedCardJSON)
	const next = "https://example.slack.com/archives/D1/p1800000009000100"
	if r := handleRemoveBlock(slackWindow(h, body{ID: "step-3:merge", Reason: "a new version asks again", ReplacedBy: "slack.com/no-scheme"})); r.OK {
		t.Fatal("remove-block took a --replaced-by that is no https permalink")
	}
	if r := handleRemoveBlock(slackWindow(h, body{ID: "step-3:merge", Reason: "a new version asks again", ReplacedBy: next})); !r.OK {
		t.Fatal(r.Error)
	}
	settled(t, h, sub)
	want := withdrawCall{"step-3:merge", slackPermalink, "a new version asks again", next}
	if got := f.made(); len(got) != 1 || got[0] != want {
		t.Fatalf("withdrawals = %+v, want %+v", got, want)
	}
}

func TestAFailedWithdrawalRetriesOnTheBoardsNextEvent(t *testing.T) {
	h := newHarness(t)
	f := &fakeWithdrawer{fail: 1}
	mirrorTo(t, h, f)
	sub := postSynced(t, h, syncedCardJSON)
	if r := handleRemoveBlock(slackWindow(h, body{ID: "step-3:merge"})); !r.OK {
		t.Fatalf("a failed Slack edit blocked the removal: %s", r.Error)
	}
	settled(t, h, sub)
	if got := withdrawnReplies(t, h, sub); len(got) != 0 {
		t.Fatalf("a failed edit was recorded as withdrawn: %q", got)
	}
	postSynced(t, h, strings.ReplaceAll(syncedCardJSON, "step-3:merge", "step-4:release"))
	board, err := caughtUp(context.Background(), h.cc.DB(), sub)
	if err != nil {
		t.Fatal(err)
	}
	board.mu.Unlock()
	settled(t, h, sub)
	want := withdrawCall{"step-3:merge", slackPermalink, defaultWithdrawReason, slackPermalink}
	if got := f.made(); len(got) != 2 || got[1] != want {
		t.Fatalf("withdrawals = %+v, want a retry of %+v", got, want)
	}
	if got := withdrawnReplies(t, h, sub); len(got) != 1 {
		t.Fatalf("withdrawn replies = %q, want one after the retry", got)
	}
}

func TestAnAnsweredSlackCardLeavesTheBoardWithoutAWithdrawal(t *testing.T) {
	h := newHarness(t)
	f := &fakeWithdrawer{}
	mirrorTo(t, h, f)
	sub := postSynced(t, h, syncedCardJSON)
	answer := `{"waits":"step-3:merge","label":"Hold","at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`
	if r := handleSlackAnswer(slackWindow(h, body{Answer: json.RawMessage(answer)})); !r.OK {
		t.Fatal(r.Error)
	}
	submitRevision(t, h, sub, 1)
	if r := handleRemoveBlock(slackWindow(h, body{ID: "step-3:merge", Reason: "done"})); !r.OK {
		t.Fatal(r.Error)
	}
	settled(t, h, sub)
	if got := f.made(); len(got) != 0 {
		t.Fatalf("withdrawals = %+v, want none: Slack already shows the answer", got)
	}
}
