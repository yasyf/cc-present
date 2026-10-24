package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/slackcards"
	"github.com/yasyf/cc-present/internal/state"
)

const (
	slackPermalink = "https://example.slack.com/archives/D1/p1800000001000100"
	syncedCardJSON = `{"id":"step-3:merge","type":"card","title":"Merge #41 now?","slack":{"url":"` + slackPermalink + `"},"children":[{"id":"step-3:merge-answer","type":"choice","options":[{"id":"answer-1","label":"Merge","recommended":true},{"id":"answer-2","label":"Hold"}]}]}`
)

func slackWindow(h *harness, b body) ccd.HandlerCtx {
	hc := h.hc(b)
	hc.Env.ClaudePID = 0
	hc.Window.ClaudePID = 0
	return hc
}

func syncedCardOf(t *testing.T, st state.State, id string) *doc.Card {
	t.Helper()
	card, _, err := syncedCard(st.Doc, id)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func TestSlackCardStartsTheSessionsBoardThenJoinsIt(t *testing.T) {
	h := newHarness(t)
	first := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
	if !first.OK {
		t.Fatalf("first card: %s", first.Error)
	}
	st := reduceSubject(t, h, first.SubjectID)
	if st.Doc.Title != slackBoardTitle || len(st.Doc.Blocks) != 1 || st.Doc.Blocks[0].BlockID() != "step-3:merge" {
		t.Fatalf("board after the first card = %q with %d blocks", st.Doc.Title, len(st.Doc.Blocks))
	}
	second := strings.ReplaceAll(syncedCardJSON, "step-3:merge", "step-4:release")
	again := handleSlackCard(h.hc(body{Block: json.RawMessage(second)}), doc.NoPacks, nilDisplay)
	if !again.OK || again.SubjectID != first.SubjectID {
		t.Fatalf("second card went to %q (%s), want the board %q", again.SubjectID, again.Error, first.SubjectID)
	}
	st = reduceSubject(t, h, first.SubjectID)
	if got := len(st.Doc.Blocks); got != 2 {
		t.Fatalf("board holds %d cards, want 2", got)
	}
}

func TestSlackCardRefusesACardWithoutItsSlackPost(t *testing.T) {
	h := newHarness(t)
	unlinked := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	if r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(unlinked)}), doc.NoPacks, nilDisplay); r.OK {
		t.Fatal("a card with no Slack permalink was put on the board as a Slack card")
	}
}

func TestSlackAnswerSelectsResolvesAndRepliesOnce(t *testing.T) {
	tests := []struct {
		name     string
		answer   string
		wantIDs  []string
		wantText string
	}{
		{"an option", `{"waits":"step-3:merge","label":"Hold","at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, []string{"answer-2"}, ""},
		{"a written-in answer", `{"waits":"step-3:merge","label":"Merge after the freeze","other":true,"at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, []string{}, "Merge after the freeze"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
			if !r.OK {
				t.Fatal(r.Error)
			}
			for range 2 {
				if a := handleSlackAnswer(slackWindow(h, body{Answer: json.RawMessage(tt.answer)})); !a.OK {
					t.Fatal(a.Error)
				}
			}
			st := reduceSubject(t, h, r.SubjectID)
			sel := st.Interactions.Choices["step-3:merge-answer"]
			if !slices.Equal(sel.OptionIDs, tt.wantIDs) || sel.Other != tt.wantText {
				t.Errorf("board selection = %v %q, want %v %q", sel.OptionIDs, sel.Other, tt.wantIDs, tt.wantText)
			}
			if got := syncedCardOf(t, st, "step-3:merge").Status; got != slackcards.Resolved {
				t.Errorf("card status = %q, want resolved", got)
			}
			replies := st.Interactions.Replies["step-3:merge"]
			if len(replies) != 1 || !strings.Contains(replies[0].Md, "Answered on [Slack]("+slackPermalink+")") {
				t.Errorf("replies = %+v, want one naming the Slack answer", replies)
			}
		})
	}
}

func newSyncedRestHarness(t *testing.T, answer slackAnswerer) *restHarness {
	t.Helper()
	h := newRestHarnessWith(t, `{"version":1,"title":"T","blocks":[`+syncedCardJSON+`]}`, emptyPackLoader(t))
	h.rs.answerSlack = answer
	return h
}

func (h *restHarness) reduce(t *testing.T) state.State {
	t.Helper()
	events, err := loadEvents(context.Background(), h.cc.DB(), h.id)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Reduce(events)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestBoardPickAnswersTheSlackCardOnce(t *testing.T) {
	var calls []string
	h := newSyncedRestHarness(t, func(_ context.Context, waits, card, label string, other bool, via string) (slackcards.SlackAnswer, error) {
		calls = append(calls, fmt.Sprintf("%s|%s|%s|%v|%s", waits, card, label, other, via))
		return slackcards.SlackAnswer{Answered: true, Label: label}, nil
	})
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":["answer-1"]}}`); w.Code != http.StatusOK {
		t.Fatalf("pick = %d %s", w.Code, w.Body)
	}
	want := []string{"step-3:merge|" + slackPermalink + "|Merge|false|http://example.com/p/board--abcd0000"}
	if !slices.Equal(calls, want) {
		t.Fatalf("cc-slack calls = %q, want %q", calls, want)
	}
	st := h.reduce(t)
	if got := syncedCardOf(t, st, "step-3:merge").Status; got != slackcards.Resolved {
		t.Fatalf("card status = %q, want resolved", got)
	}
	if sel := st.Interactions.Choices["step-3:merge-answer"]; !slices.Equal(sel.OptionIDs, []string{"answer-1"}) {
		t.Fatalf("board selection = %v, want the winning Merge recorded with the resolution", sel.OptionIDs)
	}
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n2","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":["answer-2"]}}`); w.Code != http.StatusConflict {
		t.Fatalf("second pick = %d, want 409", w.Code)
	}
	if len(calls) != 1 {
		t.Fatalf("cc-slack was called %d times, want once", len(calls))
	}
}

func TestBoardWriteInLosesToAnEarlierSlackAnswer(t *testing.T) {
	var sent []string
	h := newSyncedRestHarness(t, func(_ context.Context, _, _, label string, other bool, _ string) (slackcards.SlackAnswer, error) {
		sent = append(sent, fmt.Sprintf("%s|%v", label, other))
		return slackcards.SlackAnswer{Answered: false, Label: "Hold"}, nil
	})
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":[],"other":"Merge after the freeze"}}`); w.Code != http.StatusOK {
		t.Fatalf("write-in = %d %s", w.Code, w.Body)
	}
	if !slices.Equal(sent, []string{"Merge after the freeze|true"}) {
		t.Fatalf("sent %q, want the write-in", sent)
	}
	st := h.reduce(t)
	if sel := st.Interactions.Choices["step-3:merge-answer"]; !slices.Equal(sel.OptionIDs, []string{"answer-2"}) || sel.Other != "" {
		t.Errorf("board selection = %v %q, want Slack's Hold", sel.OptionIDs, sel.Other)
	}
	if got := syncedCardOf(t, st, "step-3:merge").Status; got != slackcards.Resolved {
		t.Errorf("card status = %q, want resolved", got)
	}
	if replies := st.Interactions.Replies["step-3:merge"]; len(replies) != 1 || !strings.Contains(replies[0].Md, "answered first: **Hold**") {
		t.Errorf("replies = %+v, want one naming Slack's answer", replies)
	}
}

func TestBoardPickThatCannotReachSlackLeavesTheCardOpen(t *testing.T) {
	h := newSyncedRestHarness(t, func(context.Context, string, string, string, bool, string) (slackcards.SlackAnswer, error) {
		return slackcards.SlackAnswer{}, errors.New("cc-slack card-answer: no waiting card")
	})
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":["answer-1"]}}`); w.Code != http.StatusOK {
		t.Fatalf("pick = %d %s", w.Code, w.Body)
	}
	st := h.reduce(t)
	if got := syncedCardOf(t, st, "step-3:merge").Status; got == slackcards.Resolved {
		t.Error("a pick Slack never took resolved the card")
	}
	if replies := st.Interactions.Replies["step-3:merge"]; len(replies) != 1 || !strings.Contains(replies[0].Md, "no waiting card") {
		t.Errorf("replies = %+v, want one carrying the cc-slack error", replies)
	}
}

func TestAReplyOnASyncedCardNamesItsSlackCard(t *testing.T) {
	h := newHarness(t)
	if r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("card: %s", r.Error)
	}
	for _, block := range []string{"step-3:merge", "step-3:merge-answer"} {
		r := handleReply(slackWindow(h, body{BlockID: block, Md: "Merging lands #41 before the freeze."}))
		var res result
		if err := json.Unmarshal(r.Body, &res); err != nil || !r.OK || res.Waits != "step-3:merge" {
			t.Fatalf("reply on %s = %+v (%s), want the Slack card's waits id", block, res, r.Error)
		}
	}
}

func TestSlackCardRefusesACardWhoseChildIDsCollideWithTheBoard(t *testing.T) {
	h := newHarness(t)
	if r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("card: %s", r.Error)
	}
	clash := strings.Replace(syncedCardJSON, `{"id":"step-3:merge","type":"card"`, `{"id":"step-4:release","type":"card"`, 1)
	if r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(clash)}), doc.NoPacks, nilDisplay); r.OK {
		t.Fatal("a second card reusing the first card's choice id joined the board")
	}
}

func TestBoardPickLosingToASlackWriteInRecordsTheWriteIn(t *testing.T) {
	h := newSyncedRestHarness(t, func(context.Context, string, string, string, bool, string) (slackcards.SlackAnswer, error) {
		return slackcards.SlackAnswer{Answered: false, Label: "Hold", Other: true}, nil
	})
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":["answer-1"]}}`); w.Code != http.StatusOK {
		t.Fatalf("pick = %d %s", w.Code, w.Body)
	}
	if sel := h.reduce(t).Interactions.Choices["step-3:merge-answer"]; len(sel.OptionIDs) != 0 || sel.Other != "Hold" {
		t.Errorf("board selection = %v %q, want Slack's written-in Hold", sel.OptionIDs, sel.Other)
	}
}

func TestSlackCardCheckRefusesWhatTheBoardWouldBeforeAnythingIsPosted(t *testing.T) {
	h := newHarness(t)
	unposted := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	multiline := strings.Replace(unposted, `"type":"card",`, `"type":"card","summary":"Merge it.\nThen release.",`, 1)
	if r := handleSlackCardCheck(slackWindow(h, body{Block: json.RawMessage(multiline)}), doc.NoPacks); r.OK || !strings.Contains(r.Error, "summary must be a single line") {
		t.Fatalf("check on a multi-line summary = ok %v, error %q", r.OK, r.Error)
	}
	if r := handleSlackCardCheck(slackWindow(h, body{Block: json.RawMessage(unposted)}), doc.NoPacks); !r.OK {
		t.Fatalf("check refused a card the board takes: %s", r.Error)
	}
	if _, found, err := h.hc(body{}).Subjects.Find(context.Background(), slackWindow(h, body{}).Window, slackWindow(h, body{}).Scope); err != nil || found {
		t.Fatalf("check started a board: found %v, err %v", found, err)
	}
}
