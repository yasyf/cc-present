package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"
	"github.com/yasyf/cc-interact/subject"

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

func syncedQuestion(id, control string) string {
	return `{"id":"` + id + `","type":"card","title":"Ask?","slack":{"url":"` + slackPermalink + `"},"children":[` + control + `]}`
}

func TestASlackAnswerResolvesAnApprovalOrAnInputOnTheBoard(t *testing.T) {
	const approval, input = `{"id":"q","type":"approval"}`, `{"id":"q","type":"input","label":"Why?"}`
	tests := []struct {
		name    string
		control string
		answer  string
		check   func(st state.State) bool
	}{
		{"an approve tap", approval, `{"waits":"c","label":"Approve","at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, func(st state.State) bool {
			return st.Interactions.Decisions["q"].Verdict == "approved"
		}},
		{"a reject tap", approval, `{"waits":"c","label":"Reject","at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, func(st state.State) bool {
			return st.Interactions.Decisions["q"].Verdict == "rejected"
		}},
		{"words written in on an approval", approval, `{"waits":"c","label":"After the freeze","other":true,"at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, func(st state.State) bool {
			d := st.Interactions.Decisions["q"]
			return d.Verdict == "rejected" && d.Note == "After the freeze"
		}},
		{"a written answer to an input", input, `{"waits":"c","label":"Because of the freeze","other":true,"at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`, func(st state.State) bool {
			return st.Interactions.Inputs["q"].Text == "Because of the freeze"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedQuestion("c", tt.control))}), doc.NoPacks, nilDisplay)
			if !r.OK {
				t.Fatal(r.Error)
			}
			if a := handleSlackAnswer(slackWindow(h, body{Answer: json.RawMessage(tt.answer)})); !a.OK {
				t.Fatal(a.Error)
			}
			st := reduceSubject(t, h, r.SubjectID)
			if !tt.check(st) {
				t.Errorf("board interactions %+v do not carry the answer", st.Interactions)
			}
			if got := syncedCardOf(t, st, "c").Status; got != slackcards.Resolved {
				t.Errorf("card status = %q, want resolved", got)
			}
		})
	}
}

func TestABoardExplainMorePickLeavesItsSlackCardOpen(t *testing.T) {
	explained := strings.Replace(syncedCardJSON, `{"id":"answer-2","label":"Hold"}`, `{"id":"answer-2","label":"Hold"},{"id":"more","label":"Explain more"}`, 1)
	h := newRestHarnessWith(t, `{"version":1,"title":"T","blocks":[`+explained+`]}`, emptyPackLoader(t))
	h.rs.answerSlack = func(context.Context, string, string, string, bool, string) (slackcards.SlackAnswer, error) {
		t.Error("an Explain more pick answered the Slack card")
		return slackcards.SlackAnswer{}, nil
	}
	if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"choice.selected","blockId":"step-3:merge-answer","optionIds":["more"]}}`); w.Code != http.StatusOK {
		t.Fatalf("pick = %d %s", w.Code, w.Body)
	}
	if got := syncedCardOf(t, h.reduce(t), "step-3:merge").Status; got == slackcards.Resolved {
		t.Error("an Explain more pick resolved the card")
	}
}

func TestABoardAnswerOnAnApprovalOrAnInputAnswersItsSlackCard(t *testing.T) {
	tests := []struct {
		name        string
		control     string
		interaction string
		label       string
		other       bool
	}{
		{"an approval", `{"id":"q","type":"approval"}`, `{"type":"decision.created","blockId":"q","verdict":"approved"}`, "Approve", false},
		{"a rejection", `{"id":"q","type":"approval"}`, `{"type":"decision.created","blockId":"q","verdict":"rejected","note":"Not yet"}`, "Reject", false},
		{"an input", `{"id":"q","type":"input","label":"Why?"}`, `{"type":"input.submitted","blockId":"q","text":"Because"}`, "Because", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			h := newRestHarnessWith(t, `{"version":1,"title":"T","blocks":[`+syncedQuestion("c", tt.control)+`]}`, emptyPackLoader(t))
			h.rs.answerSlack = func(_ context.Context, waits, _, label string, other bool, _ string) (slackcards.SlackAnswer, error) {
				calls = append(calls, fmt.Sprintf("%s|%s|%v", waits, label, other))
				return slackcards.SlackAnswer{Answered: true, Label: label, Other: other}, nil
			}
			if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":`+tt.interaction+`}`); w.Code != http.StatusOK {
				t.Fatalf("answer = %d %s", w.Code, w.Body)
			}
			if want := []string{fmt.Sprintf("c|%s|%v", tt.label, tt.other)}; !slices.Equal(calls, want) {
				t.Fatalf("cc-slack calls = %q, want %q", calls, want)
			}
			st := h.reduce(t)
			if got := syncedCardOf(t, st, "c").Status; got != slackcards.Resolved {
				t.Errorf("card status = %q, want resolved", got)
			}
			if d := st.Interactions.Decisions["q"]; tt.name == "a rejection" && d.Note != "Not yet" {
				t.Errorf("decision %+v lost the note the human wrote", d)
			}
		})
	}
}

func TestClearingAnApprovalOrBlankingAnInputAsksSlackNothing(t *testing.T) {
	for name, tt := range map[string]struct{ control, interaction string }{
		"a cleared approval": {`{"id":"q","type":"approval"}`, `{"type":"decision.created","blockId":"q","verdict":"cleared"}`},
		"a blank input":      {`{"id":"q","type":"input","label":"Why?"}`, `{"type":"input.submitted","blockId":"q","text":"  "}`},
	} {
		t.Run(name, func(t *testing.T) {
			h := newRestHarnessWith(t, `{"version":1,"title":"T","blocks":[`+syncedQuestion("c", tt.control)+`]}`, emptyPackLoader(t))
			h.rs.answerSlack = func(context.Context, string, string, string, bool, string) (slackcards.SlackAnswer, error) {
				t.Fatal("cc-slack was asked to answer")
				return slackcards.SlackAnswer{}, nil
			}
			if w := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":`+tt.interaction+`}`); w.Code != http.StatusOK {
				t.Fatalf("answer = %d %s", w.Code, w.Body)
			}
			if got := syncedCardOf(t, h.reduce(t), "c").Status; got == slackcards.Resolved {
				t.Error("the card resolved on no answer")
			}
		})
	}
}

func TestRewritingACardKeepsItsSlackPost(t *testing.T) {
	h := newHarness(t)
	r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedCardJSON)}), doc.NoPacks, nilDisplay)
	if !r.OK {
		t.Fatal(r.Error)
	}
	unlinked := strings.Replace(syncedCardJSON, `"slack":{"url":"`+slackPermalink+`"},`, "", 1)
	reworded := strings.Replace(unlinked, "Merge #41 now?", "Merge #41 today?", 1)
	if u := handleUpsertBlock(slackWindow(h, body{Block: json.RawMessage(reworded)}), doc.NoPacks); !u.OK {
		t.Fatal(u.Error)
	}
	card := syncedCardOf(t, reduceSubject(t, h, r.SubjectID), "step-3:merge")
	if card.Title != "Merge #41 today?" || card.Slack == nil || card.Slack.URL != slackPermalink {
		t.Fatalf("card = %+v, want the new title and its Slack post kept", card)
	}
}

func TestASlackAnswerReachesAWatcherAsTheHumansOwn(t *testing.T) {
	tests := []struct {
		name    string
		control string
		answer  string
		event   string
		payload string
	}{
		{"a button on a choice", `{"id":"q","type":"choice","options":[{"id":"a","label":"Merge"},{"id":"b","label":"Hold"}]}`, `"label":"Hold"`, EventChoiceSelected, `"optionIds":["b"]`},
		{"words written in on a choice", `{"id":"q","type":"choice","options":[{"id":"a","label":"Merge"}]}`, `"label":"After the freeze","other":true`, EventChoiceSelected, `"other":"After the freeze"`},
		{"a button on an approval", `{"id":"q","type":"approval"}`, `"label":"Approve"`, EventDecisionCreated, `"verdict":"approved"`},
		{"words written in on an approval", `{"id":"q","type":"approval"}`, `"label":"After the freeze","other":true`, EventDecisionCreated, `"note":"After the freeze"`},
		{"words answering an input", `{"id":"q","type":"input","label":"Why?"}`, `"label":"The freeze","other":true`, EventInputSubmitted, `"text":"The freeze"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r := handleSlackCard(slackWindow(h, body{Block: json.RawMessage(syncedQuestion("c", tt.control))}), doc.NoPacks, nilDisplay)
			if !r.OK {
				t.Fatal(r.Error)
			}
			answer := `{"waits":"c",` + tt.answer + `,"at":"2026-10-23T17:21:00Z","permalink":"` + slackPermalink + `"}`
			if a := handleSlackAnswer(slackWindow(h, body{Answer: json.RawMessage(answer)})); !a.OK {
				t.Fatal(a.Error)
			}
			events := h.eventsOfType(t, r.SubjectID, tt.event)
			if len(events) != 1 || events[0].Origin != ccevent.OriginHuman {
				t.Fatalf("%s events = %+v, want one from the human", tt.event, events)
			}
			if !strings.Contains(string(events[0].Payload), tt.payload) {
				t.Errorf("payload %s lacks %s", events[0].Payload, tt.payload)
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

func TestSlackDecidedRecordsTheBoardsOwnVerdict(t *testing.T) {
	tests := []struct {
		name, row, number, label string
		want                     state.Decision
		wantErr                  string
	}{
		{"approve at the head the card showed", "pg:8:abc1234", "8", "Approve", state.Decision{Verdict: "approved", Head: "abc1234"}, ""},
		{"hold", "pg:9:def5678", "9", "Hold", state.Decision{Verdict: "rejected", Head: "def5678"}, ""},
		{"a head that moved is refused", "pg:8:0001111", "8", "Approve", state.Decision{}, `is not the row's head "abc1234"`},
		{"a row not on the board is refused", "pg:77:abc1234", "77", "Approve", state.Decision{}, `has no pull request "77"`},
		{"a markdown block is refused", "nope:8:abc1234", "8", "Approve", state.Decision{}, `"nope" is neither a triage nor a pr-groups block`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			start := handleStart(h.hc(body{Doc: json.RawMessage(prGroupsDoc)}), doc.NoPacks, nilDisplay)
			if !start.OK {
				t.Fatal(start.Error)
			}
			tap := `{"row":"` + tt.row + `","label":"` + tt.label + `","at":"2026-10-27T03:12:00Z","permalink":"` + slackPermalink + `"}`
			r := handleSlackDecided(h.hc(body{Answer: json.RawMessage(tap)}))
			if tt.wantErr != "" {
				if r.OK || !strings.Contains(r.Error, tt.wantErr) {
					t.Fatalf("reply = %+v, want an error with %q", r, tt.wantErr)
				}
				return
			}
			if !r.OK {
				t.Fatal(r.Error)
			}
			if events := h.eventsOfType(t, start.SubjectID, EventTriageDecided); len(events) != 1 || events[0].Origin != ccevent.OriginHuman {
				t.Fatalf("triage.decided events = %+v, want one from the human", events)
			}
			got := reduceSubject(t, h, start.SubjectID).Interactions.Triage["pg"][tt.number]
			if got.Verdict != tt.want.Verdict || got.Head != tt.want.Head || got.At == "" {
				t.Fatalf("verdict = %+v, want %+v stamped with a time", got, tt.want)
			}
		})
	}
}

const triageCardDoc = `{"version":1,"title":"T","blocks":[{"id":"devices","type":"card","title":"Tag or remove each device?","slack":{"url":"` + slackPermalink + `"},"children":[` +
	`{"id":"tr","type":"triage","items":[{"id":"mbp-1","label":"MacBook"},{"id":"ipad-2","label":"iPad"},{"id":"pi-3","label":"Raspberry Pi"}]}]}]}`

func TestASlackRowTapOnATriageCardRecordsThatItemAlone(t *testing.T) {
	tests := []struct {
		name, row, label, wantErr string
		want                      map[string]string
	}{
		{"reject", "tr:ipad-2", "Reject", "", map[string]string{"ipad-2": "rejected"}},
		{"approve", "tr:mbp-1", "Approve", "", map[string]string{"mbp-1": "approved"}},
		{"a pr-groups label is refused", "tr:mbp-1", "Hold", "answers Approve or Reject", nil},
		{"an item not on the board is refused", "tr:tv-9", "Approve", `has no item "tv-9"`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			start := handleStart(h.hc(body{Doc: json.RawMessage(triageCardDoc)}), doc.NoPacks, nilDisplay)
			if !start.OK {
				t.Fatal(start.Error)
			}
			tap := `{"row":"` + tt.row + `","label":"` + tt.label + `","at":"2026-10-27T03:12:00Z","permalink":"` + slackPermalink + `"}`
			r := handleSlackDecided(h.hc(body{Answer: json.RawMessage(tap)}))
			if tt.wantErr != "" {
				if r.OK || !strings.Contains(r.Error, tt.wantErr) {
					t.Fatalf("reply = %+v, want an error with %q", r, tt.wantErr)
				}
				return
			}
			if !r.OK {
				t.Fatal(r.Error)
			}
			if events := h.eventsOfType(t, start.SubjectID, EventTriageDecided); len(events) != 1 || events[0].Origin != ccevent.OriginHuman {
				t.Fatalf("triage.decided events = %+v, want one from the human, as a board tap records it", events)
			}
			got := map[string]string{}
			for id, d := range reduceSubject(t, h, start.SubjectID).Interactions.Triage["tr"] {
				got[id] = d.Verdict
			}
			if !maps.Equal(got, tt.want) {
				t.Fatalf("verdicts = %v, want %v and the other rows still open", got, tt.want)
			}
		})
	}
}

func TestABoardTapOnATriageCardRedrawsItsSlackRows(t *testing.T) {
	calls := make(chan slackcards.StatusCard, 1)
	h := newRestHarnessWith(t, triageCardDoc, emptyPackLoader(t))
	h.rs.postGroup = func(_ context.Context, _, thread string, card slackcards.StatusCard) (string, error) {
		if thread != "" {
			t.Errorf("thread = %q, want the session's own", thread)
		}
		calls <- card
		return slackPermalink, nil
	}
	if code := h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"triage.decided","blockId":"tr","verdicts":{"ipad-2":{"verdict":"rejected"}}}}`).Code; code != http.StatusOK {
		t.Fatalf("tap = %d, want 200", code)
	}
	var card slackcards.StatusCard
	select {
	case card = <-calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the Slack card never redrew after the tap")
	}
	var rows []string
	for _, r := range card.Rows {
		rows = append(rows, r.ID)
	}
	if card.Group != "devices" || card.Status != "needs_you" || !slices.Equal(rows, []string{"tr:mbp-1", "tr:pi-3"}) || !strings.Contains(card.Body, "*iPad* · Rejected") {
		t.Fatalf("card = %+v, want the two open rows still asking and the iPad rejected in the body", card)
	}
}

func TestABoundPRGroupsBlockKeepsItsThreadUntilAPushUnbindsIt(t *testing.T) {
	const thread = "1792842193.708029"
	withSlack := func(slack string) string {
		return strings.Replace(prGroupsDoc, `"repo":"a/b",`, `"repo":"a/b",`+slack, 1)
	}
	var d doc.Doc
	if err := json.Unmarshal([]byte(prGroupsDoc), &d); err != nil {
		t.Fatal(err)
	}
	unstated, err := json.Marshal(d.Blocks[0])
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(withSlack(`"slack":"` + thread + `",`))}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatal(start.Error)
	}
	steps := []struct {
		name string
		send func() ccd.Reply
		want string
	}{
		{"a push stating no thread keeps it", func() ccd.Reply {
			return handlePush(h.hc(body{Doc: json.RawMessage(prGroupsDoc)}), doc.NoPacks, nilDisplay)
		}, thread},
		{"an update-block stating no thread keeps it", func() ccd.Reply { return handleUpsertBlock(h.hc(body{Block: unstated}), doc.NoPacks) }, thread},
		{"an empty slack unbinds it", func() ccd.Reply {
			return handlePush(h.hc(body{Doc: json.RawMessage(withSlack(`"slack":"",`))}), doc.NoPacks, nilDisplay)
		}, ""},
		{"a later push stating no thread leaves it unbound", func() ccd.Reply {
			return handlePush(h.hc(body{Doc: json.RawMessage(prGroupsDoc)}), doc.NoPacks, nilDisplay)
		}, ""},
	}
	for _, step := range steps {
		if r := step.send(); !r.OK {
			t.Fatalf("%s: %s", step.name, r.Error)
		}
		if got := reduceSubject(t, h, start.SubjectID).Doc.Blocks[0].(*doc.PRGroups).Thread(); got != step.want {
			t.Fatalf("%s: thread = %q, want %q", step.name, got, step.want)
		}
	}
}

func TestABoardCannotBindASlackThreadAnotherOpenBoardDrives(t *testing.T) {
	bound := json.RawMessage(strings.Replace(prGroupsDoc, `"repo":"a/b",`, `"repo":"a/b","slack":"1792842193.708029",`, 1))
	h := newHarness(t)
	if r := handleStart(h.hc(body{Doc: bound}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatal(r.Error)
	}
	other := func(b body) ccd.HandlerCtx {
		hc := h.hc(b)
		hc.Env.Session, hc.Env.ClaudePID = "s2", 200
		hc.Window = subject.Window{Session: "s2", ClaudePID: 200}
		return hc
	}
	if r := handleStart(other(body{Doc: bound}), doc.NoPacks, nilDisplay); r.OK || !strings.Contains(r.Error, "belongs to the open board") {
		t.Fatalf("seeding a second board with the bound thread = %+v, want refused", r)
	}
	if r := handlePush(other(body{Doc: bound}), doc.NoPacks, nilDisplay); r.OK || !strings.Contains(r.Error, "belongs to the open board") {
		t.Fatalf("pushing the bound thread to a second board = %+v, want refused", r)
	}
	if r := handlePush(h.hc(body{Doc: bound}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("the owning board re-pushing its own thread: %s", r.Error)
	}
	if r := handleClose(h.hc(body{}), h.assets); !r.OK {
		t.Fatal(r.Error)
	}
	if r := handlePush(other(body{Doc: bound}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("binding the thread once its board closed: %s", r.Error)
	}
}

func TestBoardVerdictAnswersBeforeItsSlackGroupCardRedraws(t *testing.T) {
	type posted struct {
		session, thread string
		card            slackcards.StatusCard
	}
	calls := make(chan posted, 1)
	release := make(chan struct{})
	h := newRestHarnessWith(t, strings.Replace(prGroupsDoc, `"repo":"a/b",`, `"repo":"a/b","slack":"1792842193.708029",`, 1), emptyPackLoader(t))
	h.rs.postGroup = func(_ context.Context, session, thread string, card slackcards.StatusCard) (string, error) {
		<-release
		calls <- posted{session, thread, card}
		return slackPermalink, nil
	}
	tapped := make(chan int, 1)
	go func() {
		tapped <- h.post(t, `{"subject":"board--abcd0000","nonce":"n1","interaction":{"type":"triage.decided","blockId":"pg","verdicts":{"8":{"verdict":"approved","head":"abc1234"}}}}`).Code
	}()
	select {
	case code := <-tapped:
		if code != http.StatusOK {
			t.Fatalf("tap = %d, want 200", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tap waited on the Slack card redraw")
	}
	close(release)
	var call posted
	select {
	case call = <-calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the Slack card never redrew after the tap")
	}
	if call.session != "s1" || call.thread != "1792842193.708029" || call.card.Group != "pg:g1" {
		t.Fatalf("cc-slack call = %+v, want an edit of pg:g1 in the bound thread", call)
	}
	if len(call.card.Rows) != 1 || call.card.Rows[0].ID != "pg:9:def5678" || !strings.Contains(call.card.Body, "`abc1234` · Approved at abc1234") {
		t.Fatalf("card = %+v, want row 9 still asking and row 8 approved in the body", call.card)
	}
}

func TestAPRGroupsVerdictLandsAfterItsRoundCloses(t *testing.T) {
	h := newRestHarnessWith(t, prGroupsDoc, emptyPackLoader(t))
	st := h.reduce(t)
	st.Rounds.Current = st.Rounds.BlockRounds["pg"] + 1
	merge := interaction{Type: EventTriageDecided, BlockID: "pg", Verdicts: map[string]triageEntry{"8": {Verdict: "approved", Head: "abc1234"}}}
	if _, err := validateInteraction(&st, 0, &merge, nil, time.Now()); err != nil {
		t.Fatalf("a merge verdict pinned to its head was refused in a closed round: %v", err)
	}
	item := interaction{Type: EventTriageDecided, BlockID: "tr1", Verdicts: map[string]triageEntry{"i1": {Verdict: "approved"}}}
	if _, err := validateInteraction(&st, 0, &item, nil, time.Now()); err == nil || !strings.Contains(err.Error(), "closed round") {
		t.Fatalf("a triage verdict in a closed round = %v, want refused", err)
	}
}

func TestASlackThreadIsFreeOnceItsBoardUnbindsIt(t *testing.T) {
	bound := json.RawMessage(strings.Replace(prGroupsDoc, `"repo":"a/b",`, `"repo":"a/b","slack":"1792842193.708029",`, 1))
	unbound := json.RawMessage(strings.Replace(prGroupsDoc, `"repo":"a/b",`, `"repo":"a/b","slack":"",`, 1))
	h := newHarness(t)
	if r := handleStart(h.hc(body{Doc: bound}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatal(r.Error)
	}
	if r := handlePush(h.hc(body{Doc: unbound}), doc.NoPacks, nilDisplay); !r.OK {
		t.Fatal(r.Error)
	}
	other := h.hc(body{Doc: bound})
	other.Env.Session, other.Env.ClaudePID = "s2", 200
	other.Window = subject.Window{Session: "s2", ClaudePID: 200}
	if r := handleStart(other, doc.NoPacks, nilDisplay); !r.OK {
		t.Fatalf("binding a thread its first board unbound: %s", r.Error)
	}
}
