package daemon

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	ccd "github.com/yasyf/cc-interact/daemon"
	ccevent "github.com/yasyf/cc-interact/event"

	"github.com/yasyf/cc-present/internal/doc"
	"github.com/yasyf/cc-present/internal/state"
)

const answeredCardDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Merge?","children":[{"id":"a1","type":"approval"}]},{"id":"m1","type":"markdown","md":"note"}]}`

const editedCardDoc = `{"version":1,"title":"Board","blocks":[{"id":"c1","type":"card","title":"Merge now?","children":[{"id":"a1","type":"approval"}]},{"id":"m1","type":"markdown","md":"note"}]}`

const answeredCard = `{"id":"c1","type":"card","title":"Merge?","children":[{"id":"a1","type":"approval"}]}`

const reaskedCard = `{"id":"c1","type":"card","title":"Merge?","children":[{"id":"a1","type":"approval","prompt":"Merge it now?"}]}`

const portTitle = "Port, old tree to new: dev #41: Merge?"

const portedCard = `{"id":"c1","type":"card","title":"` + portTitle + `","children":[{"id":"a1","type":"approval"}]}`

func answer(t *testing.T, h *harness, subjectID, blockID string) {
	t.Helper()
	if _, err := h.cc.AppendEvent(context.Background(), &ccevent.Event{
		SubjectID: subjectID, Origin: ccevent.OriginHuman, Type: EventDecisionCreated,
		Payload: injectIdentity(EventDecisionCreated, json.RawMessage(`{"blockId":"`+blockID+`","verdict":"approved"}`)),
	}); err != nil {
		t.Fatalf("append decision: %v", err)
	}
}

func answeredBoard(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	start := handleStart(h.hc(body{Doc: json.RawMessage(answeredCardDoc)}), doc.NoPacks, nilDisplay)
	if !start.OK {
		t.Fatalf("start not ok: %s", start.Error)
	}
	answer(t, h, start.SubjectID, "a1")
	ageEvents(t, h, start.SubjectID)
	return h, start.SubjectID
}

func outcome(t *testing.T, h *harness) result {
	t.Helper()
	reply := handleOutcomes(h.hc(body{}), h.cursors)
	if !reply.OK {
		t.Fatalf("outcomes not ok: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return res
}

func TestAnsweredBlocksNeverComeBack(t *testing.T) {
	t.Run("update-block refuses an answered card until reopened", func(t *testing.T) {
		h, sub := answeredBoard(t)
		reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(reaskedCard)}), doc.NoPacks)
		if reply.OK {
			t.Fatal("update-block re-asked an answered card")
		}
		for _, want := range []string{`"c1"`, `"a1"`, "answered at", "--reopen c1"} {
			if !strings.Contains(reply.Error, want) {
				t.Fatalf("refusal %q does not name %q", reply.Error, want)
			}
		}
		if got := h.eventsOfType(t, sub, EventBlockUpserted); len(got) != 0 {
			t.Fatalf("refused update-block appended %d block.upserted", len(got))
		}
		if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(reaskedCard), Reopen: []string{"c1"}}), doc.NoPacks); !reply.OK {
			t.Fatalf("reopened update-block not ok: %s", reply.Error)
		}
	})

	t.Run("push keeps an unchanged answered card but refuses to re-ask it", func(t *testing.T) {
		h, _ := answeredBoard(t)
		if rev := pushRev(t, h, answeredCardDoc); rev != 2 {
			t.Fatalf("unchanged push revision = %d, want 2", rev)
		}
		reply := handlePush(h.hc(body{Doc: json.RawMessage(editedCardDoc)}), doc.NoPacks, nilDisplay)
		if reply.OK || !strings.Contains(reply.Error, "--reopen c1") {
			t.Fatalf("edited push: ok=%v err=%q, want a --reopen c1 refusal", reply.OK, reply.Error)
		}
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(editedCardDoc), Reopen: []string{"a1"}}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("reopened push not ok: %s", reply.Error)
		}
	})

	t.Run("remove-block on an answered card retires it from later writes", func(t *testing.T) {
		h, sub := answeredBoard(t)
		if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
			t.Fatalf("remove-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); !slices.Equal(res.Retired, []string{"c1"}) {
			t.Fatalf("retired = %v, want [c1]", res.Retired)
		}
		reply := handlePush(h.hc(body{Doc: json.RawMessage(answeredCardDoc)}), doc.NoPacks, nilDisplay)
		if !reply.OK {
			t.Fatalf("push over a retired card not ok: %s", reply.Error)
		}
		var res result
		if err := json.Unmarshal(reply.Body, &res); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `"c1"`) {
			t.Fatalf("warnings = %v, want one naming c1", res.Warnings)
		}
		docs := h.eventsOfType(t, sub, EventDocReplaced)
		if strings.Contains(string(docs[len(docs)-1].Payload), `"c1"`) {
			t.Fatal("the pushed document kept the retired card")
		}
		removals := len(h.eventsOfType(t, sub, EventBlockRemoved))
		upsert := handleUpsertBlock(h.hc(body{Block: json.RawMessage(reaskedCard)}), doc.NoPacks)
		var dropped result
		if err := json.Unmarshal(upsert.Body, &dropped); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if !upsert.OK || len(dropped.Warnings) != 1 || !strings.Contains(dropped.Warnings[0], "--reopen c1") ||
			len(h.eventsOfType(t, sub, EventBlockUpserted)) != 0 || len(h.eventsOfType(t, sub, EventBlockRemoved)) != removals {
			t.Fatalf("update-block asking a retired card again = %+v, want a warning and no write", upsert)
		}
		if reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(answeredCard), Reopen: []string{"c1"}, Round: "current"}), doc.NoPacks); !reply.OK {
			t.Fatalf("reopened update-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); len(res.Retired) != 0 {
			t.Fatalf("retired after reopen = %v, want none", res.Retired)
		}
	})

	t.Run("removing an unanswered block leaves no tombstone", func(t *testing.T) {
		h, _ := answeredBoard(t)
		if reply := handleRemoveBlock(h.hc(body{ID: "m1"})); !reply.OK {
			t.Fatalf("remove-block not ok: %s", reply.Error)
		}
		if res := outcome(t, h); len(res.Retired) != 0 {
			t.Fatalf("retired = %v, want none", res.Retired)
		}
		if reply := handlePush(h.hc(body{Doc: json.RawMessage(answeredCardDoc), Round: "current"}), doc.NoPacks, nilDisplay); !reply.OK {
			t.Fatalf("push restoring m1 not ok: %s", reply.Error)
		}
	})
}

func upserted(t *testing.T, reply ccd.Reply) result {
	t.Helper()
	if !reply.OK {
		t.Fatalf("update-block not ok: %s", reply.Error)
	}
	var res result
	if err := json.Unmarshal(reply.Body, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return res
}

func TestNewWordsOnAnAnsweredCardKeepItsVerdictAndItsPlace(t *testing.T) {
	t.Run("a retired card stays off the board in the round it left", func(t *testing.T) {
		h, sub := answeredBoard(t)
		if reply := handleRemoveBlock(h.hc(body{ID: "c1"})); !reply.OK {
			t.Fatalf("remove-block not ok: %s", reply.Error)
		}
		submitRevision(t, h, sub, 1)
		docs := len(h.eventsOfType(t, sub, EventDocReplaced))
		res := upserted(t, handleUpsertBlock(h.hc(body{Block: json.RawMessage(portedCard)}), doc.NoPacks))
		if len(res.Warnings) != 0 || res.Rev == 0 || !strings.Contains(string(res.Reworded), portTitle) {
			t.Fatalf("result = %+v, want the new words recorded with no warning", res)
		}
		if len(h.eventsOfType(t, sub, EventBlockUpserted)) != 0 || len(h.eventsOfType(t, sub, EventDocReplaced)) != docs || len(h.eventsOfType(t, sub, EventRoundStarted)) != 0 {
			t.Fatal("new words on a retired card wrote it back onto the board")
		}
		if res := outcome(t, h); !slices.Equal(res.Retired, []string{"c1"}) {
			t.Fatalf("retired = %v, want [c1]", res.Retired)
		}
		st := reduceSubject(t, h, sub)
		if _, on := doc.Locate(st.Doc, "c1"); on {
			t.Fatal("the retired card is back on the board")
		}
		if got := st.Interactions.Decisions["a1"].Verdict; got != "approved" {
			t.Fatalf("verdict = %q, want approved kept", got)
		}
		if got, want := st.RemovedAnswers()["a1"], (state.Removed{Top: "c1", Title: portTitle, Round: 1}); got != want || st.Rounds.Current != 2 {
			t.Fatalf("removed a1 = %+v in round %d, want %+v in round 2", got, st.Rounds.Current, want)
		}
		if show := blockShow(t, h, "c1"); show.OnBoard || !strings.Contains(string(show.Top), portTitle) {
			t.Fatalf("block show = %s on board %v, want the new title off the board", show.Top, show.OnBoard)
		}
	})

	t.Run("a card on the board keeps its closed round", func(t *testing.T) {
		h, sub := answeredBoard(t)
		submitRevision(t, h, sub, 1)
		read := outcome(t, h).Rev
		res := upserted(t, handleUpsertBlock(h.hc(body{Writer: Writer{Rev: read}, Block: json.RawMessage(portedCard)}), doc.NoPacks))
		if !strings.Contains(string(res.Reworded), portTitle) {
			t.Fatalf("reworded = %s, want the card with its new title", res.Reworded)
		}
		if stale := handleUpsertBlock(h.hc(body{Writer: Writer{Rev: read}, Block: json.RawMessage(answeredCard)}), doc.NoPacks); stale.OK || !strings.Contains(stale.Error, `refusing to write "c1"`) {
			t.Fatalf("a second redraw from rev %d = ok %v err %q, want it refused as stale", read, stale.OK, stale.Error)
		}
		if len(h.eventsOfType(t, sub, EventBlockUpserted)) != 0 {
			t.Fatal("new words on an answered card upserted it into the current round")
		}
		st := reduceSubject(t, h, sub)
		loc, on := doc.Locate(st.Doc, "c1")
		if card, ok := loc.Block.(*doc.Card); !on || !ok || card.Title != portTitle {
			t.Fatalf("c1 = %+v on board %v, want it in place with the new title", loc.Block, on)
		}
		if st.Rounds.BlockRounds["c1"] != 1 || st.Rounds.Current != 2 {
			t.Fatalf("c1 in round %d of %d, want it left in closed round 1", st.Rounds.BlockRounds["c1"], st.Rounds.Current)
		}
		if got := st.Interactions.Decisions["a1"].Verdict; got != "approved" {
			t.Fatalf("verdict = %q, want approved kept", got)
		}
	})
}

func TestAnAnsweredCardTakesNewWordsAndNothingElse(t *testing.T) {
	card := func(title, prTitle, head, options string) string {
		return `{"id":"c2","type":"card","title":"` + title + `","children":[{"id":"pr41","type":"pr","repo":"o/r","number":41,"head":"` + head + `","title":"` + prTitle + `"},` +
			`{"id":"q2","type":"choice","options":[` + options + `]}]}`
	}
	const options = `{"id":"merge","label":"Merge","recommended":true},{"id":"hold","label":"Hold"}`
	asked := card("Merge #41?", "Fix the thing", "abc1234", options)
	for name, tc := range map[string]struct {
		answered bool
		card     string
		refused  bool
	}{
		"new words on the card and its PR":  {true, card(portTitle, "Fix the other thing", "abc1234", options), false},
		"a relabeled option":                {true, card("Merge #41?", "Fix the thing", "abc1234", `{"id":"merge","label":"Merge","recommended":true},{"id":"hold","label":"Wait"}`), true},
		"an added option":                   {true, card("Merge #41?", "Fix the thing", "abc1234", options+`,{"id":"later","label":"Later"}`), true},
		"a moved PR head":                   {true, card("Merge #41?", "Fix the thing", "def5678", options), true},
		"an unanswered card's added option": {false, card("Merge #41?", "Fix the thing", "abc1234", options+`,{"id":"later","label":"Later"}`), false},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			start := handleStart(h.hc(body{Doc: json.RawMessage(`{"version":1,"title":"Board","blocks":[` + asked + `]}`)}), doc.NoPacks, nilDisplay)
			if !start.OK {
				t.Fatalf("start not ok: %s", start.Error)
			}
			if tc.answered {
				appendFrom(t, h, start.SubjectID, ccevent.OriginHuman, EventChoiceSelected, `{"blockId":"q2","optionIds":["hold"]}`)
			}
			reply := handleUpsertBlock(h.hc(body{Block: json.RawMessage(tc.card)}), doc.NoPacks)
			if tc.refused {
				if reply.OK || !strings.HasPrefix(reply.Error, `block "c2" was answered at `) ||
					!strings.HasSuffix(reply.Error, `("q2" carries the human's verdict) and answered blocks never come back; pass --reopen c2 to ask again`) {
					t.Fatalf("update-block = ok %v err %q, want the answered refusal", reply.OK, reply.Error)
				}
				return
			}
			res := upserted(t, reply)
			upserts := len(h.eventsOfType(t, start.SubjectID, EventBlockUpserted))
			if (len(res.Reworded) > 0) != tc.answered || (upserts == 0) != tc.answered {
				t.Fatalf("reworded %s with %d block.upserted; answered %v wants a redraw in place, else an upsert", res.Reworded, upserts, tc.answered)
			}
		})
	}
}
